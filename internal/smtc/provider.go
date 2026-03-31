package smtc

// Provider abstracts the SMTC integration for testing.
// The real implementation is *Smtc; tests use MockProvider.
type Provider interface {
	// Start begins monitoring SMTC for media changes.
	Start()
	// Stop halts monitoring and cleans up resources.
	Stop()
	// SetCallbacks registers event callbacks for info and progress updates.
	SetCallbacks(opts Options)
	// ListSessions returns all currently available SMTC sessions.
	ListSessions() []SessionInfo
	// SelectSession switches monitoring to the session identified by appID.
	SelectSession(appID string) error
}
