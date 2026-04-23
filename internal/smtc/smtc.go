//go:build windows

package smtc

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/media/control"
)

// Smtc manages Windows System Media Transport Controls.
type Smtc struct {
	opts          Options
	cmdChan       chan func()
	droppedEvents atomic.Int64
	mu            sync.Mutex // protects sessions, sessionObjects, currentStatus, currentPosition, currentDuration, currentArtist, currentTitle, currentThumbnailSize, currentProperties

	// Subscriber state — protected by subsMu.
	subsMu      sync.Mutex
	subscribers []*subscriber

	// Session management
	sessionManager *control.GlobalSystemMediaTransportControlsSessionManager
	currentSession *control.GlobalSystemMediaTransportControlsSession

	// Multi-session state
	sessions       []SessionInfo
	sessionObjects []*control.GlobalSystemMediaTransportControlsSession
	selectedAppID  string

	// Event tokens for cleanup
	sessionsChangedToken        foundation.EventRegistrationToken
	mediaPropertiesChangedToken foundation.EventRegistrationToken
	playbackInfoChangedToken    foundation.EventRegistrationToken

	// Current state (for change detection and deduplication)
	currentArtist               string
	currentTitle                string
	currentStatus               int
	currentThumbnailSize        uint64
	currentThumbnailContentType string
	currentThumbnailData        []byte

	// Progress tracking
	currentPosition int
	currentDuration int
	progressTicker  *time.Ticker

	// currentProperties holds the latest media properties object for thumbnail reading.
	currentProperties *control.GlobalSystemMediaTransportControlsSessionMediaProperties

	// thumbnailRetryTimer is used to delay thumbnail reading when a song changes
	// but the thumbnail is not yet available. Prevents flickering by waiting for
	// the thumbnail to be ready before firing OnInfo.
	// Access serialised by timerMu — it's manipulated from both the SMTC
	// goroutine (via handleMediaPropertiesChanged) and Run() cleanup.
	thumbnailRetryTimer *time.Timer
	timerMu             sync.Mutex
}

type subscriber struct {
	ch      chan Event
	dropped atomic.Int64
}

// New creates a new Smtc instance with the given options
func New(opts Options) *Smtc {
	return &Smtc{
		opts:          opts,
		cmdChan:       make(chan func(), 32),
		selectedAppID: opts.InitialDevice,
	}
}

// Run begins monitoring SMTC for media changes. It blocks until ctx is canceled.
// Must be called from a dedicated goroutine. Initializes COM (MTA), subscribes
// to SMTC events, runs the event loop, then cleans up.
func (s *Smtc) Run(ctx context.Context) error {
	// Lock this goroutine to its OS thread so WinRT COM objects stay on a single thread.
	runtime.LockOSThread()

	// Initialize WinRT apartment as MTA (1 = COINIT_MULTITHREADED).
	if err := ole.RoInitialize(1); err != nil {
		return fmt.Errorf("smtc: RoInitialize: %w", err)
	}
	defer roUninitialize()

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := s.initSessionManager(); err != nil {
		return fmt.Errorf("smtc: init session manager: %w", err)
	}

	s.startProgressTimer()
	defer s.stopProgressTimer()

	for {
		select {
		case <-ctx.Done():
			s.cleanupRun()
			return ctx.Err()
		case cmd := <-s.cmdChan:
			cmd()
		case <-s.progressTicker.C:
			s.readTimelineAndProgress()
		}
	}
}

func (s *Smtc) cleanupRun() {
	// Cancel any pending thumbnail retry timer under its own mutex so we don't
	// race with the SMTC goroutine that sets/resets it.
	s.timerMu.Lock()
	if s.thumbnailRetryTimer != nil {
		s.thumbnailRetryTimer.Stop()
		s.thumbnailRetryTimer = nil
	}
	s.timerMu.Unlock()

	if s.currentSession != nil {
		s.unsubscribePropertyEvents()
		s.currentSession = nil
	}
	if s.sessionManager != nil {
		_ = s.sessionManager.RemoveSessionsChanged(s.sessionsChangedToken)
		s.sessionManager = nil
	}
	s.sessionsChangedToken = foundation.EventRegistrationToken{}
	s.currentProperties = nil

	s.subsMu.Lock()
	for _, sub := range s.subscribers {
		close(sub.ch)
	}
	s.subscribers = nil
	s.subsMu.Unlock()
}

// SelectDevice selects the SMTC session identified by appID for monitoring.
// Safe to call from any goroutine; the actual selection runs on the SMTC goroutine via cmdChan.
func (s *Smtc) SelectDevice(appID string) {
	s.cmdChan <- func() { s.selectDevice(appID) }
}

// GetSessions returns a copy of the current list of available SMTC sessions.
// Safe to call from any goroutine.
func (s *Smtc) GetSessions() []SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sessions) == 0 {
		return nil
	}
	result := make([]SessionInfo, len(s.sessions))
	copy(result, s.sessions)
	return result
}

// Subscribe creates a new event channel with the given buffer size.
// Caller must call Unsubscribe when done to avoid channel leaks.
func (s *Smtc) Subscribe(bufSize int) <-chan Event {
	if bufSize < 0 {
		bufSize = 0
	}
	sub := &subscriber{ch: make(chan Event, bufSize)}
	s.subsMu.Lock()
	s.subscribers = append(s.subscribers, sub)
	s.subsMu.Unlock()
	return sub.ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (s *Smtc) Unsubscribe(ch <-chan Event) {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	for i, sub := range s.subscribers {
		if (<-chan Event)(sub.ch) == ch {
			copy(s.subscribers[i:], s.subscribers[i+1:])
			s.subscribers[len(s.subscribers)-1] = nil
			s.subscribers = s.subscribers[:len(s.subscribers)-1]
			close(sub.ch)
			return
		}
	}
}

// fanout sends ev to all subscribers non-blocking, dropping and counting if full.
func (s *Smtc) fanout(ev Event) {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	for _, sub := range s.subscribers {
		select {
		case sub.ch <- ev:
		default:
			sub.dropped.Add(1)
		}
	}
}
