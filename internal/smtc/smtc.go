//go:build windows

package smtc

import (
	"runtime"
	"sync"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/media/control"
)

// Smtc manages Windows System Media Transport Controls with callback-based updates
type Smtc struct {
	opts     Options
	quitChan chan struct{}
	cmdChan  chan func()
	mu       sync.Mutex // protects sessions, sessionObjects, currentStatus, currentPosition, currentDuration, currentArtist, currentTitle, currentThumbnailSize, currentProperties

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
}

// New creates a new Smtc instance with the given options
func New(opts Options) *Smtc {
	return &Smtc{
		opts:          opts,
		quitChan:      make(chan struct{}),
		cmdChan:       make(chan func(), 8),
		selectedAppID: opts.InitialDevice,
	}
}

// Start begins monitoring SMTC for media changes.
// Launches a dedicated goroutine that initializes COM (MTA), creates the session manager,
// subscribes to events, and runs the progress ticker event loop.
func (s *Smtc) Start() error {
	go func() {
		// Lock this goroutine to its OS thread so WinRT COM objects stay on a single thread.
		runtime.LockOSThread()

		// Initialize WinRT apartment as MTA (1 = COINIT_MULTITHREADED).
		// Must be called on the locked OS thread before any WinRT calls.
		if err := ole.RoInitialize(1); err != nil {
			// MTA initialization failed — this is fatal, bail out.
			return
		}
		defer roUninitialize()

		if err := s.initSessionManager(); err != nil {
			return
		}

		s.startProgressTimer()
		defer s.stopProgressTimer()

		// Event loop: drive the progress ticker, handle commands, and respond to quit signal.
		for {
			select {
			case <-s.quitChan:
				// Cleanup: remove all WinRT event subscriptions before exiting.
				if s.currentSession != nil {
					s.unsubscribePropertyEvents()
				}
				if s.sessionManager != nil {
					_ = s.sessionManager.RemoveSessionsChanged(s.sessionsChangedToken)
				}
				return
			case cmd := <-s.cmdChan:
				cmd()
			case <-s.progressTicker.C:
				s.readTimelineAndProgress()
			}
		}
	}()
	return nil
}

// Stop stops monitoring SMTC by signalling the dedicated goroutine to exit.
func (s *Smtc) Stop() {
	close(s.quitChan)
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
