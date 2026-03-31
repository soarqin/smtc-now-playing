//go:build windows

package smtc

import (
	"errors"
	"fmt"

	"github.com/go-ole/go-ole"
	winrt "github.com/saltosystems/winrt-go"
	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/media"
)

// iidBoolAsyncCompletedHandler is the parameterized IID for
// IAsyncOperationCompletedHandler<bool>, used when waiting on TryXxxAsync operations.
var iidBoolAsyncCompletedHandler *ole.GUID

func init() {
	// IAsyncOperationCompletedHandler<bool>
	iidBoolAsyncCompletedHandler = ole.NewGUID(winrt.ParameterizedInstanceGUID(
		foundation.GUIDAsyncOperationCompletedHandler,
		winrt.SignatureBool,
	))
}

// ControlAction represents a media control command type.
type ControlAction int

const (
	// ControlPlay requests playback to start.
	ControlPlay ControlAction = iota
	// ControlPause requests playback to pause.
	ControlPause
	// ControlStop requests playback to stop.
	ControlStop
	// ControlTogglePlayPause toggles between play and pause.
	ControlTogglePlayPause
	// ControlSkipNext skips to the next track.
	ControlSkipNext
	// ControlSkipPrevious skips to the previous track.
	ControlSkipPrevious
	// ControlSeek requests a playback position change (uses seekPosition field).
	ControlSeek
	// ControlShuffle requests a shuffle state change (uses shuffleActive field).
	ControlShuffle
	// ControlRepeat requests a repeat mode change (uses repeatMode field).
	ControlRepeat
)

// controlCommand is sent through cmdChan to execute a media control action
// on the SMTC goroutine. ResultChan receives the outcome once the WinRT call completes.
type controlCommand struct {
	action        ControlAction
	seekPosition  int64      // ControlSeek: position in milliseconds
	shuffleActive bool       // ControlShuffle: desired shuffle state
	repeatMode    int        // ControlRepeat: 0=None, 1=Track, 2=List
	resultChan    chan error // receives nil on success or an error
}

// ErrNoSession is returned when a control command is issued but no SMTC session is active.
var ErrNoSession = errors.New("smtc: no active session")

// Play sends a play request to the current SMTC session.
// Blocks until the WinRT async call completes.
func (s *Smtc) Play() error {
	return s.sendControl(controlCommand{action: ControlPlay})
}

// Pause sends a pause request to the current SMTC session.
// Blocks until the WinRT async call completes.
func (s *Smtc) Pause() error {
	return s.sendControl(controlCommand{action: ControlPause})
}

// StopPlayback sends a stop-playback request to the current SMTC session.
// Blocks until the WinRT async call completes.
// Named StopPlayback to avoid collision with Smtc.Stop() which shuts down the goroutine.
func (s *Smtc) StopPlayback() error {
	return s.sendControl(controlCommand{action: ControlStop})
}

// TogglePlayPause toggles playback state on the current SMTC session.
// Blocks until the WinRT async call completes.
func (s *Smtc) TogglePlayPause() error {
	return s.sendControl(controlCommand{action: ControlTogglePlayPause})
}

// SkipNext skips to the next track on the current SMTC session.
// Blocks until the WinRT async call completes.
func (s *Smtc) SkipNext() error {
	return s.sendControl(controlCommand{action: ControlSkipNext})
}

// SkipPrevious skips to the previous track on the current SMTC session.
// Blocks until the WinRT async call completes.
func (s *Smtc) SkipPrevious() error {
	return s.sendControl(controlCommand{action: ControlSkipPrevious})
}

// SeekTo requests the playback position of the current SMTC session to change to positionMs.
// positionMs is the target position in milliseconds.
// Blocks until the WinRT async call completes.
func (s *Smtc) SeekTo(positionMs int64) error {
	return s.sendControl(controlCommand{action: ControlSeek, seekPosition: positionMs})
}

// SetShuffle requests a shuffle state change on the current SMTC session.
// Blocks until the WinRT async call completes.
func (s *Smtc) SetShuffle(active bool) error {
	return s.sendControl(controlCommand{action: ControlShuffle, shuffleActive: active})
}

// SetRepeat requests a repeat mode change on the current SMTC session.
// mode: 0=None, 1=Track, 2=List.
// Blocks until the WinRT async call completes.
func (s *Smtc) SetRepeat(mode int) error {
	return s.sendControl(controlCommand{action: ControlRepeat, repeatMode: mode})
}

// GetCapabilities returns which controls are currently enabled for the active session.
// Blocks briefly while routing through cmdChan for thread safety.
// Returns a zero-value ControlCapabilities when no session is active.
func (s *Smtc) GetCapabilities() ControlCapabilities {
	resultChan := make(chan ControlCapabilities, 1)
	select {
	case s.cmdChan <- func() { resultChan <- s.readCapabilities() }:
	default:
		// cmdChan is full — return zero-value rather than block.
		return ControlCapabilities{}
	}
	return <-resultChan
}

// sendControl routes cmd through cmdChan and blocks until executeControl completes.
// Returns an error if the channel is full (non-blocking send).
func (s *Smtc) sendControl(cmd controlCommand) error {
	cmd.resultChan = make(chan error, 1)
	select {
	case s.cmdChan <- func() { s.executeControl(cmd) }:
	default:
		return fmt.Errorf("smtc: command channel full")
	}
	return <-cmd.resultChan
}

// executeControl performs the WinRT control call on the smtc goroutine.
// Must only be called from within the cmdChan event loop.
func (s *Smtc) executeControl(cmd controlCommand) {
	if s.currentSession == nil {
		cmd.resultChan <- ErrNoSession
		return
	}

	var (
		op  *foundation.IAsyncOperation
		err error
	)

	switch cmd.action {
	case ControlPlay:
		op, err = s.currentSession.TryPlayAsync()
	case ControlPause:
		op, err = s.currentSession.TryPauseAsync()
	case ControlStop:
		op, err = s.currentSession.TryStopAsync()
	case ControlTogglePlayPause:
		op, err = s.currentSession.TryTogglePlayPauseAsync()
	case ControlSkipNext:
		op, err = s.currentSession.TrySkipNextAsync()
	case ControlSkipPrevious:
		op, err = s.currentSession.TrySkipPreviousAsync()
	case ControlSeek:
		// WinRT uses 100-nanosecond ticks; convert ms → ticks.
		ticks := cmd.seekPosition * 10000
		op, err = s.currentSession.TryChangePlaybackPositionAsync(ticks)
	case ControlShuffle:
		op, err = s.currentSession.TryChangeShuffleActiveAsync(cmd.shuffleActive)
	case ControlRepeat:
		op, err = s.currentSession.TryChangeAutoRepeatModeAsync(media.MediaPlaybackAutoRepeatMode(cmd.repeatMode))
	default:
		cmd.resultChan <- fmt.Errorf("smtc: unknown control action %d", cmd.action)
		return
	}

	if err != nil {
		cmd.resultChan <- err
		return
	}

	// Wait for the IAsyncOperation<bool> to complete.
	// Status == AsyncStatusCompleted means the request was accepted by the session.
	_, status := waitForAsync(op, iidBoolAsyncCompletedHandler)
	if status != foundation.AsyncStatusCompleted {
		cmd.resultChan <- fmt.Errorf("smtc: control async failed: status %d", status)
		return
	}

	cmd.resultChan <- nil
}

// readCapabilities reads playback controls from the current session synchronously.
// Must only be called from within the cmdChan event loop.
func (s *Smtc) readCapabilities() ControlCapabilities {
	if s.currentSession == nil {
		return ControlCapabilities{}
	}

	playbackInfo, err := s.currentSession.GetPlaybackInfo()
	if err != nil || playbackInfo == nil {
		return ControlCapabilities{}
	}

	controls, err := playbackInfo.GetControls()
	if err != nil || controls == nil {
		return ControlCapabilities{}
	}

	var caps ControlCapabilities
	caps.IsPlayEnabled, _ = controls.GetIsPlayEnabled()
	caps.IsPauseEnabled, _ = controls.GetIsPauseEnabled()
	caps.IsStopEnabled, _ = controls.GetIsStopEnabled()
	caps.IsNextEnabled, _ = controls.GetIsNextEnabled()
	caps.IsPreviousEnabled, _ = controls.GetIsPreviousEnabled()
	caps.IsSeekEnabled, _ = controls.GetIsPlaybackPositionEnabled()
	caps.IsShuffleEnabled, _ = controls.GetIsShuffleEnabled()
	caps.IsRepeatEnabled, _ = controls.GetIsRepeatEnabled()
	return caps
}
