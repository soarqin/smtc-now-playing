//go:build windows

package smtc

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/media/control"
)

// Smtc manages Windows System Media Transport Controls with callback-based updates
type Smtc struct {
	opts          Options
	quitChan      chan struct{}
	doneChan      chan struct{} // closed by the goroutine when it exits
	cmdChan       chan func()
	droppedEvents atomic.Int64
	mu            sync.Mutex // protects sessions, sessionObjects, currentStatus, currentPosition, currentDuration, currentArtist, currentTitle, currentThumbnailSize, currentProperties

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
	// goroutine (via handleMediaPropertiesChanged) and any caller of Stop().
	thumbnailRetryTimer *time.Timer
	timerMu             sync.Mutex

	// stopped is flipped to true exactly once by Stop() so callers can
	// safely call Stop() multiple times (e.g. in defers).
	stopped atomic.Bool
}

// New creates a new Smtc instance with the given options
func New(opts Options) *Smtc {
	return &Smtc{
		opts:          opts,
		quitChan:      make(chan struct{}),
		doneChan:      make(chan struct{}),
		cmdChan:       make(chan func(), 32),
		selectedAppID: opts.InitialDevice,
	}
}

// Start begins monitoring SMTC for media changes.
// Launches a dedicated goroutine that initializes COM (MTA), creates the session manager,
// subscribes to events, and runs the progress ticker event loop.
func (s *Smtc) Start() error {
	go func() {
		// Always signal doneChan so Stop() can unblock even if we bail
		// out early (RoInitialize / initSessionManager failures).
		defer close(s.doneChan)

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
// Waits up to ~2s for the goroutine to clean up WinRT subscriptions so the
// caller (server.Stop() / app shutdown) can proceed deterministically.
// Safe to call multiple times.
func (s *Smtc) Stop() {
	if !s.stopped.CompareAndSwap(false, true) {
		return
	}

	// Cancel any pending thumbnail retry timer under its own mutex so we
	// don't race with the SMTC goroutine that sets/resets it.
	s.timerMu.Lock()
	if s.thumbnailRetryTimer != nil {
		s.thumbnailRetryTimer.Stop()
		s.thumbnailRetryTimer = nil
	}
	s.timerMu.Unlock()

	close(s.quitChan)

	// Block briefly for a clean shutdown: let the goroutine unwind its
	// WinRT subscriptions on its own thread. Bound the wait so a stuck
	// WinRT call can never hang app exit indefinitely.
	select {
	case <-s.doneChan:
	case <-time.After(2 * time.Second):
	}
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
