//go:build smtc_test

package smtc

// MockProvider implements Provider for testing without real WinRT.
// Tests set InfoCallback / ProgressCallback directly to simulate events.
type MockProvider struct {
	// InfoCallback stores the OnInfo callback registered via SetCallbacks.
	// Tests can invoke it directly: mock.InfoCallback(smtc.InfoData{...})
	InfoCallback InfoCallback

	// ProgressCallback stores the OnProgress callback registered via SetCallbacks.
	// Tests can invoke it directly: mock.ProgressCallback(smtc.ProgressData{...})
	ProgressCallback ProgressCallback

	// MockSessions is the list returned by ListSessions.
	MockSessions []SessionInfo

	// SelectedAppID is set by SelectSession for test assertions.
	SelectedAppID string

	// storedOpts retains all callbacks from the last SetCallbacks call.
	storedOpts Options
}

// Start is a no-op — no WinRT initialisation in tests.
func (m *MockProvider) Start() {}

// Stop is a no-op.
func (m *MockProvider) Stop() {}

// SetCallbacks stores opts and exposes the info/progress callbacks as public fields
// so tests can trigger them directly.
func (m *MockProvider) SetCallbacks(opts Options) {
	m.storedOpts = opts
	m.InfoCallback = opts.OnInfo
	m.ProgressCallback = opts.OnProgress
}

// ListSessions returns the pre-configured MockSessions slice.
func (m *MockProvider) ListSessions() []SessionInfo {
	return m.MockSessions
}

// SelectSession records the chosen appID and returns nil (no error).
func (m *MockProvider) SelectSession(appID string) error {
	m.SelectedAppID = appID
	return nil
}

// TriggerSessionsChanged fires the OnSessionsChanged callback if registered.
func (m *MockProvider) TriggerSessionsChanged(sessions []SessionInfo) {
	if m.storedOpts.OnSessionsChanged != nil {
		m.storedOpts.OnSessionsChanged(sessions)
	}
}

// TriggerDeviceChange fires the OnSelectedDeviceChange callback if registered.
func (m *MockProvider) TriggerDeviceChange(appID string) {
	if m.storedOpts.OnSelectedDeviceChange != nil {
		m.storedOpts.OnSelectedDeviceChange(appID)
	}
}
