//go:build windows

package smtc

import (
	"testing"
	"time"

	"smtc-now-playing/internal/domain"
)

func TestSubscribeFanout_TwoSubscribers(t *testing.T) {
	s := New(Options{})
	ch1 := s.Subscribe(1)
	ch2 := s.Subscribe(1)
	t.Cleanup(func() {
		s.Unsubscribe(ch1)
		s.Unsubscribe(ch2)
	})

	event := InfoEvent{Data: domain.InfoData{Artist: "artist", Title: "title"}}
	s.fanOut(event)

	assertEventReceived(t, ch1, event)
	assertEventReceived(t, ch2, event)
}

func TestSubscribe_DropsOnFullBuffer(t *testing.T) {
	s := New(Options{})
	ch := s.Subscribe(1)
	t.Cleanup(func() { s.Unsubscribe(ch) })

	first := InfoEvent{Data: domain.InfoData{Artist: "first"}}
	second := InfoEvent{Data: domain.InfoData{Artist: "second"}}

	s.fanOut(first)
	s.fanOut(second)

	assertEventReceived(t, ch, first)
	if got := s.dropCount(ch); got != 1 {
		t.Fatalf("drop count = %d, want 1", got)
	}
	assertNoEvent(t, ch)
}

func TestClearMediaInfo_DedupsEmptyEvent(t *testing.T) {
	s := New(Options{})
	ch := s.Subscribe(1)
	t.Cleanup(func() { s.Unsubscribe(ch) })

	s.currentArtist = "artist"
	s.currentTitle = "title"

	s.clearMediaInfo()
	assertEventReceived(t, ch, InfoEvent{Data: domain.InfoData{}})

	s.clearMediaInfo()
	assertNoEvent(t, ch)
}

func TestUnsubscribe_ChannelClosed(t *testing.T) {
	s := New(Options{})
	ch := s.Subscribe(1)

	s.Unsubscribe(ch)
	s.Unsubscribe(ch)

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("channel should be closed after unsubscribe")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for closed channel")
	}

	s.fanOut(InfoEvent{Data: domain.InfoData{Artist: "ignored"}})
}

func (s *Smtc) dropCount(ch <-chan Event) int64 {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	for _, sub := range s.subscribers {
		if (<-chan Event)(sub.ch) == ch {
			return sub.dropped.Load()
		}
	}
	return 0
}

func assertEventReceived(t *testing.T, ch <-chan Event, want Event) {
	t.Helper()
	select {
	case got := <-ch:
		compareEvents(t, got, want)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timed out waiting for %T", want)
	}
}

func assertNoEvent(t *testing.T, ch <-chan Event) {
	t.Helper()
	select {
	case got := <-ch:
		t.Fatalf("unexpected event received: %#v", got)
	default:
	}
}

func compareEvents(t *testing.T, got Event, want Event) {
	t.Helper()
	switch wantTyped := want.(type) {
	case InfoEvent:
		gotTyped, ok := got.(InfoEvent)
		if !ok {
			t.Fatalf("got %T, want %T", got, want)
		}
		if !gotTyped.Data.Equal(&wantTyped.Data) {
			t.Fatalf("got %#v, want %#v", gotTyped, wantTyped)
		}
	case ProgressEvent:
		gotTyped, ok := got.(ProgressEvent)
		if !ok {
			t.Fatalf("got %T, want %T", got, want)
		}
		if !gotTyped.Data.Equal(&wantTyped.Data) {
			t.Fatalf("got %#v, want %#v", gotTyped, wantTyped)
		}
	case SessionsChangedEvent:
		gotTyped, ok := got.(SessionsChangedEvent)
		if !ok {
			t.Fatalf("got %T, want %T", got, want)
		}
		if len(gotTyped.Sessions) != len(wantTyped.Sessions) {
			t.Fatalf("got %#v, want %#v", gotTyped, wantTyped)
		}
		for i := range gotTyped.Sessions {
			if gotTyped.Sessions[i] != wantTyped.Sessions[i] {
				t.Fatalf("got %#v, want %#v", gotTyped, wantTyped)
			}
		}
	case DeviceChangedEvent:
		gotTyped, ok := got.(DeviceChangedEvent)
		if !ok {
			t.Fatalf("got %T, want %T", got, want)
		}
		if gotTyped != wantTyped {
			t.Fatalf("got %#v, want %#v", gotTyped, wantTyped)
		}
	default:
		t.Fatalf("unsupported event type %T", want)
	}
}
