package wsproto

import (
	"encoding/json"
	"errors"
	"testing"

	"smtc-now-playing/internal/domain"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		env  Envelope
	}{
		{
			name: "hello",
			env: NewHello("1.0", map[string]bool{
				"play":  true,
				"pause": true,
			}),
		},
		{
			name: "info",
			env: NewInfo(domain.InfoData{
				Artist:       "Test Artist",
				Title:        "Test Title",
				AlbumTitle:   "Test Album",
				AlbumArtist:  "Album Artist",
				PlaybackType: 1,
				SourceApp:    "TestApp.exe",
			}, "/albumArt/a3f2c1"),
		},
		{
			name: "progress",
			env: NewProgress(domain.ProgressData{
				Position:        120,
				Duration:        240,
				Status:          4,
				PlaybackRate:    1.0,
				IsShuffleActive: boolPtr(true),
				AutoRepeatMode:  0,
				LastUpdatedTime: 1711900000000,
			}),
		},
		{
			name: "sessions",
			env: NewSessions([]domain.SessionInfo{
				{
					AppID:       "Spotify.exe",
					Name:        "Spotify",
					SourceAppID: "Spotify.exe",
				},
			}),
		},
		{
			name: "reload",
			env:  NewReload(),
		},
		{
			name: "ping",
			env:  NewPing(),
		},
		{
			name: "pong",
			env:  NewPong(1711900000000),
		},
		{
			name: "ack_success",
			env:  NewAck("msg-123", nil),
		},
		{
			name: "ack_error",
			env:  NewAck("msg-456", errors.New("test error")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			b, err := json.Marshal(tt.env)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}

			// Unmarshal back
			got, err := ParseEnvelope(b)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}

			// Verify type and version
			if got.Type != tt.env.Type {
				t.Errorf("type mismatch: got %q, want %q", got.Type, tt.env.Type)
			}
			if got.V != ProtocolVersion {
				t.Errorf("version mismatch: got %d, want %d", got.V, ProtocolVersion)
			}

			// Verify data round-trips
			if len(tt.env.Data) > 0 {
				if !json.Valid(got.Data) {
					t.Errorf("data is not valid JSON")
				}
			}
		})
	}
}

func TestParseEnvelopeVersionMismatch(t *testing.T) {
	tests := []struct {
		name    string
		version int
	}{
		{"v1", 1},
		{"v3", 3},
		{"v0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(`{"type":"hello","v":` + string(rune(tt.version+'0')) + `,"ts":0}`)
			_, err := ParseEnvelope(data)
			if err == nil {
				t.Errorf("expected error for version %d, got nil", tt.version)
			}
			if err.Error() != "unsupported protocol version: "+string(rune(tt.version+'0')) {
				t.Errorf("unexpected error message: %v", err)
			}
		})
	}
}

func TestParseEnvelopeMissingType(t *testing.T) {
	data := []byte(`{"v":2,"ts":0}`)
	_, err := ParseEnvelope(data)
	if err == nil {
		t.Errorf("expected error for missing type, got nil")
	}
	if err.Error() != "missing message type" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseEnvelopeMalformedJSON(t *testing.T) {
	data := []byte(`{invalid json}`)
	_, err := ParseEnvelope(data)
	if err == nil {
		t.Errorf("expected error for malformed JSON, got nil")
	}
}

func TestParseControl(t *testing.T) {
	payload := ControlPayload{
		Action: "play",
		Args:   json.RawMessage(`{"speed":1.5}`),
	}
	data, _ := json.Marshal(payload)
	env := Envelope{
		Type: MsgControl,
		V:    ProtocolVersion,
		TS:   0,
		Data: data,
	}

	got, err := env.ParseControl()
	if err != nil {
		t.Fatalf("parse control failed: %v", err)
	}

	if got.Action != "play" {
		t.Errorf("action mismatch: got %q, want %q", got.Action, "play")
	}

	if !json.Valid(got.Args) {
		t.Errorf("args is not valid JSON")
	}
}

func TestNewAckSuccess(t *testing.T) {
	env := NewAck("msg-123", nil)

	if env.Type != MsgAck {
		t.Errorf("type mismatch: got %q, want %q", env.Type, MsgAck)
	}

	var payload AckPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if !payload.Success {
		t.Errorf("success should be true")
	}
	if payload.Error != "" {
		t.Errorf("error should be empty, got %q", payload.Error)
	}
}

func TestNewAckError(t *testing.T) {
	env := NewAck("msg-456", errors.New("test error"))

	var payload AckPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if payload.Success {
		t.Errorf("success should be false")
	}
	if payload.Error != "test error" {
		t.Errorf("error mismatch: got %q, want %q", payload.Error, "test error")
	}
}

func TestNewPong(t *testing.T) {
	pingTS := int64(1711900000000)
	env := NewPong(pingTS)

	if env.Type != MsgPong {
		t.Errorf("type mismatch: got %q, want %q", env.Type, MsgPong)
	}

	if env.TS != pingTS {
		t.Errorf("timestamp mismatch: got %d, want %d", env.TS, pingTS)
	}
}

func TestMessageTypeConstants(t *testing.T) {
	types := []MessageType{
		MsgHello, MsgInfo, MsgProgress, MsgSessions,
		MsgReload, MsgControl, MsgPing, MsgPong, MsgAck,
	}

	expectedCount := 9
	if len(types) != expectedCount {
		t.Errorf("expected %d message types, got %d", expectedCount, len(types))
	}

	// Verify no duplicates
	seen := make(map[MessageType]bool)
	for _, mt := range types {
		if seen[mt] {
			t.Errorf("duplicate message type: %q", mt)
		}
		seen[mt] = true
	}
}

func TestProtocolVersion(t *testing.T) {
	if ProtocolVersion != 2 {
		t.Errorf("protocol version mismatch: got %d, want 2", ProtocolVersion)
	}
}

func TestHeartbeatConstants(t *testing.T) {
	if HeartbeatInterval != 30*1000*1000*1000 { // 30 seconds in nanoseconds
		t.Errorf("heartbeat interval mismatch")
	}
	if HeartbeatTimeout != 90*1000*1000*1000 { // 90 seconds in nanoseconds
		t.Errorf("heartbeat timeout mismatch")
	}
}

// Helper function to create a bool pointer
func boolPtr(b bool) *bool {
	return &b
}
