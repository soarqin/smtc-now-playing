//go:build windows

package smtc

import (
	"errors"
	"testing"
)

// TestControlCapabilities_ZeroValue verifies that a zero-value ControlCapabilities
// has all fields false, which is the expected result when no session is active.
func TestControlCapabilities_ZeroValue(t *testing.T) {
	caps := ControlCapabilities{}
	if caps.IsPlayEnabled {
		t.Error("IsPlayEnabled should default to false")
	}
	if caps.IsPauseEnabled {
		t.Error("IsPauseEnabled should default to false")
	}
	if caps.IsStopEnabled {
		t.Error("IsStopEnabled should default to false")
	}
	if caps.IsNextEnabled {
		t.Error("IsNextEnabled should default to false")
	}
	if caps.IsPreviousEnabled {
		t.Error("IsPreviousEnabled should default to false")
	}
	if caps.IsSeekEnabled {
		t.Error("IsSeekEnabled should default to false")
	}
	if caps.IsShuffleEnabled {
		t.Error("IsShuffleEnabled should default to false")
	}
	if caps.IsRepeatEnabled {
		t.Error("IsRepeatEnabled should default to false")
	}
}

// TestControlCapabilities_AllFields verifies all ControlCapabilities fields
// can be set independently — confirming the struct has the expected shape.
func TestControlCapabilities_AllFields(t *testing.T) {
	caps := ControlCapabilities{
		IsPlayEnabled:     true,
		IsPauseEnabled:    true,
		IsStopEnabled:     true,
		IsNextEnabled:     true,
		IsPreviousEnabled: true,
		IsSeekEnabled:     true,
		IsShuffleEnabled:  true,
		IsRepeatEnabled:   true,
	}
	if !caps.IsPlayEnabled || !caps.IsPauseEnabled || !caps.IsStopEnabled ||
		!caps.IsNextEnabled || !caps.IsPreviousEnabled || !caps.IsSeekEnabled ||
		!caps.IsShuffleEnabled || !caps.IsRepeatEnabled {
		t.Error("all ControlCapabilities fields should be settable to true")
	}
}

// TestPlay_NoSession verifies that Play() returns ErrNoSession when
// currentSession is nil — exercising the full cmdChan round-trip.
func TestPlay_NoSession(t *testing.T) {
	s := New(Options{})

	// Drain cmdChan in a separate goroutine so the event-loop func can run.
	go func() {
		fn := <-s.cmdChan
		fn()
	}()

	err := s.Play()
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("Play() with no session: got %v, want ErrNoSession", err)
	}
}

// TestPause_NoSession verifies that Pause() returns ErrNoSession when no session is active.
func TestPause_NoSession(t *testing.T) {
	s := New(Options{})
	go func() { fn := <-s.cmdChan; fn() }()

	err := s.Pause()
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("Pause() with no session: got %v, want ErrNoSession", err)
	}
}

// TestSkipNext_NoSession verifies that SkipNext() returns ErrNoSession when no session is active.
func TestSkipNext_NoSession(t *testing.T) {
	s := New(Options{})
	go func() { fn := <-s.cmdChan; fn() }()

	err := s.SkipNext()
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("SkipNext() with no session: got %v, want ErrNoSession", err)
	}
}

// TestSeek_NoSession verifies that Seek() returns ErrNoSession when no session is active.
func TestSeek_NoSession(t *testing.T) {
	s := New(Options{})
	go func() { fn := <-s.cmdChan; fn() }()

	err := s.Seek(30000)
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("Seek() with no session: got %v, want ErrNoSession", err)
	}
}

// TestGetCapabilities_NoSession verifies that GetCapabilities() returns
// zero-value when no session is active — exercising the cmdChan round-trip.
func TestGetCapabilities_NoSession(t *testing.T) {
	s := New(Options{})
	go func() { fn := <-s.cmdChan; fn() }()

	caps := s.GetCapabilities()
	if caps.IsPlayEnabled || caps.IsPauseEnabled || caps.IsStopEnabled ||
		caps.IsNextEnabled || caps.IsPreviousEnabled || caps.IsSeekEnabled ||
		caps.IsShuffleEnabled || caps.IsRepeatEnabled {
		t.Error("GetCapabilities() with no session: expected all false, got non-zero value")
	}
}

// TestSendControl_ChannelFull verifies that sendControl returns an error
// (not a deadlock) when cmdChan is filled to capacity.
func TestSendControl_ChannelFull(t *testing.T) {
	s := New(Options{})

	// Fill cmdChan to capacity (32) with no-op functions.
	for i := 0; i < 32; i++ {
		s.cmdChan <- func() {}
	}

	// sendControl must return an error without blocking.
	err := s.Play()
	if err == nil {
		t.Error("Play() with full cmdChan: expected error, got nil")
	}
}

// TestControlAction_Constants verifies that ControlAction constants have expected values.
func TestControlAction_Constants(t *testing.T) {
	if ControlPlay != 0 {
		t.Errorf("ControlPlay = %d, want 0", ControlPlay)
	}
	if ControlPause != 1 {
		t.Errorf("ControlPause = %d, want 1", ControlPause)
	}
	if ControlStop != 2 {
		t.Errorf("ControlStop = %d, want 2", ControlStop)
	}
	if ControlSeek != 6 {
		t.Errorf("ControlSeek = %d, want 6", ControlSeek)
	}
	if ControlRepeat != 8 {
		t.Errorf("ControlRepeat = %d, want 8", ControlRepeat)
	}
}
