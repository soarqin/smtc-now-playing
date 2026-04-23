//go:build smtc_test

package smtc

import (
	"context"
	"sync"
)

// MockProvider implements Provider for testing without real WinRT.
type MockProvider struct {
	mu            sync.Mutex
	subscribers   map[chan Event]*struct{}
	sessions      []SessionInfo
	SelectedAppID string
}

// Run blocks until ctx is canceled, then closes all subscriber channels.
func (m *MockProvider) Run(ctx context.Context) error {
	<-ctx.Done()
	m.mu.Lock()
	for ch := range m.subscribers {
		close(ch)
	}
	m.subscribers = nil
	m.mu.Unlock()
	return ctx.Err()
}

// Subscribe registers a new event subscriber.
func (m *MockProvider) Subscribe(bufSize int) <-chan Event {
	if bufSize < 0 {
		bufSize = 0
	}
	ch := make(chan Event, bufSize)
	m.mu.Lock()
	if m.subscribers == nil {
		m.subscribers = make(map[chan Event]*struct{})
	}
	m.subscribers[ch] = &struct{}{}
	m.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (m *MockProvider) Unsubscribe(ch <-chan Event) {
	m.mu.Lock()
	for writeCh := range m.subscribers {
		if (<-chan Event)(writeCh) == ch {
			delete(m.subscribers, writeCh)
			close(writeCh)
			break
		}
	}
	m.mu.Unlock()
}

// Inject sends an event to all subscribers.
func (m *MockProvider) Inject(e Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for ch := range m.subscribers {
		select {
		case ch <- e:
		default:
		}
	}
}

// GetSessions returns the configured session list.
func (m *MockProvider) GetSessions() []SessionInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sessions) == 0 {
		return nil
	}
	out := make([]SessionInfo, len(m.sessions))
	copy(out, m.sessions)
	return out
}

// SelectDevice records the selected app ID.
func (m *MockProvider) SelectDevice(appID string) {
	m.mu.Lock()
	m.SelectedAppID = appID
	m.mu.Unlock()
}

// GetCapabilities returns an empty capability set for tests.
func (m *MockProvider) GetCapabilities() ControlCapabilities { return ControlCapabilities{} }

func (m *MockProvider) Play() error                   { return ErrNoSession }
func (m *MockProvider) Pause() error                  { return ErrNoSession }
func (m *MockProvider) StopPlayback() error           { return ErrNoSession }
func (m *MockProvider) TogglePlayPause() error        { return ErrNoSession }
func (m *MockProvider) SkipNext() error               { return ErrNoSession }
func (m *MockProvider) SkipPrevious() error           { return ErrNoSession }
func (m *MockProvider) SeekTo(positionMs int64) error { return ErrNoSession }
func (m *MockProvider) SetShuffle(active bool) error  { return ErrNoSession }
func (m *MockProvider) SetRepeat(mode int) error      { return ErrNoSession }
