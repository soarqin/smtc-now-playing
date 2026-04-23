//go:build windows

package smtc

import (
	"context"
	"errors"
	"testing"
	"time"
)

var _ func(*Smtc, context.Context) error = (*Smtc).Run

// TestSmtcContextShutdown verifies that a canceled context causes Run to return promptly.
func TestSmtcContextShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := New(Options{})
	done := make(chan error, 1)

	go func() {
		done <- s.Run(ctx)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Run returned nil after cancellation")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not return within 500ms after context cancellation")
	}
}
