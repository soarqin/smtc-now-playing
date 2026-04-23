// Package wsproto defines the WebSocket v2 protocol message types and helpers.
package wsproto

import (
	"encoding/json"
	"fmt"
	"time"

	"smtc-now-playing/internal/domain"
)

// Protocol version
const ProtocolVersion = 2

// Heartbeat timing
const (
	HeartbeatInterval = 30 * time.Second
	HeartbeatTimeout  = 90 * time.Second
)

// MessageType represents the type of a WebSocket message
type MessageType string

// Message type constants
const (
	MsgHello    MessageType = "hello"
	MsgInfo     MessageType = "info"
	MsgProgress MessageType = "progress"
	MsgSessions MessageType = "sessions"
	MsgReload   MessageType = "reload"
	MsgControl  MessageType = "control"
	MsgPing     MessageType = "ping"
	MsgPong     MessageType = "pong"
	MsgAck      MessageType = "ack"
)

// Envelope is the top-level WebSocket message container
type Envelope struct {
	Type MessageType     `json:"type"`
	V    int             `json:"v"`
	ID   string          `json:"id,omitempty"`
	TS   int64           `json:"ts"`
	Data json.RawMessage `json:"data,omitempty"`
}

// HelloPayload is the data for a hello message
type HelloPayload struct {
	ServerVersion     string        `json:"serverVersion"`
	SupportedMessages []MessageType `json:"supportedMessages"`
	Capabilities      map[string]bool `json:"capabilities"`
}

// InfoPayload is the data for an info message
type InfoPayload struct {
	Artist       string `json:"artist"`
	Title        string `json:"title"`
	AlbumTitle   string `json:"albumTitle"`
	AlbumArtist  string `json:"albumArtist"`
	PlaybackType int    `json:"playbackType"`
	SourceApp    string `json:"sourceApp"`
	AlbumArt     string `json:"albumArt"`
}

// ProgressPayload is the data for a progress message
type ProgressPayload struct {
	Position        int     `json:"position"`
	Duration        int     `json:"duration"`
	Status          int     `json:"status"`
	PlaybackRate    float64 `json:"playbackRate"`
	IsShuffleActive *bool   `json:"isShuffleActive"`
	AutoRepeatMode  int     `json:"autoRepeatMode"`
	LastUpdatedTime int64   `json:"lastUpdatedTime"`
}

// SessionInfo holds metadata about an available SMTC session
type SessionInfo struct {
	AppID       string `json:"appId"`
	Name        string `json:"name"`
	SourceAppID string `json:"sourceAppId"`
}

// SessionsPayload is the data for a sessions message
type SessionsPayload struct {
	Sessions []SessionInfo `json:"sessions"`
}

// ControlPayload is the data for a control message
type ControlPayload struct {
	Action string          `json:"action"`
	Args   json.RawMessage `json:"args,omitempty"`
}

// AckPayload is the data for an ack message
type AckPayload struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// NewHello creates a hello message
func NewHello(version string, caps map[string]bool) Envelope {
	payload := HelloPayload{
		ServerVersion: version,
		SupportedMessages: []MessageType{
			MsgHello, MsgInfo, MsgProgress, MsgSessions,
			MsgReload, MsgControl, MsgPing, MsgPong, MsgAck,
		},
		Capabilities: caps,
	}
	data, _ := json.Marshal(payload)
	return Envelope{
		Type: MsgHello,
		V:    ProtocolVersion,
		TS:   time.Now().UnixMilli(),
		Data: data,
	}
}

// NewInfo creates an info message
func NewInfo(d domain.InfoData, albumArtURL string) Envelope {
	payload := InfoPayload{
		Artist:       d.Artist,
		Title:        d.Title,
		AlbumTitle:   d.AlbumTitle,
		AlbumArtist:  d.AlbumArtist,
		PlaybackType: d.PlaybackType,
		SourceApp:    d.SourceApp,
		AlbumArt:     albumArtURL,
	}
	data, _ := json.Marshal(payload)
	return Envelope{
		Type: MsgInfo,
		V:    ProtocolVersion,
		TS:   time.Now().UnixMilli(),
		Data: data,
	}
}

// NewProgress creates a progress message
func NewProgress(d domain.ProgressData) Envelope {
	payload := ProgressPayload{
		Position:        d.Position,
		Duration:        d.Duration,
		Status:          d.Status,
		PlaybackRate:    d.PlaybackRate,
		IsShuffleActive: d.IsShuffleActive,
		AutoRepeatMode:  d.AutoRepeatMode,
		LastUpdatedTime: d.LastUpdatedTime,
	}
	data, _ := json.Marshal(payload)
	return Envelope{
		Type: MsgProgress,
		V:    ProtocolVersion,
		TS:   time.Now().UnixMilli(),
		Data: data,
	}
}

// NewSessions creates a sessions message
func NewSessions(sessions []domain.SessionInfo) Envelope {
	sessionInfos := make([]SessionInfo, len(sessions))
	for i, s := range sessions {
		sessionInfos[i] = SessionInfo{
			AppID:       s.AppID,
			Name:        s.Name,
			SourceAppID: s.SourceAppID,
		}
	}
	payload := SessionsPayload{
		Sessions: sessionInfos,
	}
	data, _ := json.Marshal(payload)
	return Envelope{
		Type: MsgSessions,
		V:    ProtocolVersion,
		TS:   time.Now().UnixMilli(),
		Data: data,
	}
}

// NewReload creates a reload message
func NewReload() Envelope {
	return Envelope{
		Type: MsgReload,
		V:    ProtocolVersion,
		TS:   time.Now().UnixMilli(),
	}
}

// NewPing creates a ping message
func NewPing() Envelope {
	return Envelope{
		Type: MsgPing,
		V:    ProtocolVersion,
		TS:   time.Now().UnixMilli(),
	}
}

// NewPong creates a pong message echoing the ping's timestamp
func NewPong(ts int64) Envelope {
	return Envelope{
		Type: MsgPong,
		V:    ProtocolVersion,
		TS:   ts,
	}
}

// NewAck creates an ack message
func NewAck(id string, err error) Envelope {
	payload := AckPayload{
		Success: err == nil,
	}
	if err != nil {
		payload.Error = err.Error()
	}
	data, _ := json.Marshal(payload)
	return Envelope{
		Type: MsgAck,
		V:    ProtocolVersion,
		ID:   id,
		TS:   time.Now().UnixMilli(),
		Data: data,
	}
}

// ParseEnvelope unmarshals and validates a WebSocket message
func ParseEnvelope(data []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Envelope{}, err
	}

	if env.V != ProtocolVersion {
		return Envelope{}, fmt.Errorf("unsupported protocol version: %d", env.V)
	}

	if env.Type == "" {
		return Envelope{}, fmt.Errorf("missing message type")
	}

	return env, nil
}

// ParseControl decodes the control payload from an envelope
func (e Envelope) ParseControl() (ControlPayload, error) {
	var payload ControlPayload
	if err := json.Unmarshal(e.Data, &payload); err != nil {
		return ControlPayload{}, err
	}
	return payload, nil
}
