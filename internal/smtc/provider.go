package smtc

import "context"

// Provider abstracts the SMTC integration for testing.
// The real implementation is *Smtc; tests use MockProvider.
type Provider interface {
	// Run begins monitoring SMTC for media changes, blocking until ctx canceled.
	Run(ctx context.Context) error
	// Subscribe creates an event channel. Caller must Unsubscribe when done.
	Subscribe(bufSize int) <-chan Event
	// Unsubscribe removes and closes the channel.
	Unsubscribe(ch <-chan Event)
	// GetSessions returns all currently available SMTC sessions.
	GetSessions() []SessionInfo
	// SelectDevice switches monitoring to the session identified by appID.
	SelectDevice(appID string)
	// GetCapabilities returns which controls the current session supports.
	GetCapabilities() ControlCapabilities
	// Control methods.
	Play() error
	Pause() error
	StopPlayback() error
	TogglePlayPause() error
	SkipNext() error
	SkipPrevious() error
	SeekTo(positionMs int64) error
	SetShuffle(active bool) error
	SetRepeat(mode int) error
}
