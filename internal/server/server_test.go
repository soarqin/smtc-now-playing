package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lxzan/gws"
	"smtc-now-playing/internal/config"
	"smtc-now-playing/internal/domain"
	"smtc-now-playing/internal/smtc"
	"smtc-now-playing/internal/version"
	"smtc-now-playing/internal/wsproto"
)

func boolPtr(b bool) *bool { return &b }

type fakeSMTCService struct {
	events       chan smtc.Event
	sessions     []smtc.SessionInfo
	capabilities smtc.ControlCapabilities
	playErr      error
	pauseErr     error
	stopErr      error
	toggleErr    error
	nextErr      error
	previousErr  error
	seekErr      error
	shuffleErr   error
	repeatErr    error
	seekCalls    []int64
	shuffleCalls []bool
	repeatCalls  []int
}

func newFakeSMTCService() *fakeSMTCService {
	return &fakeSMTCService{events: make(chan smtc.Event, 16)}
}

func (f *fakeSMTCService) Subscribe(buf int) <-chan smtc.Event { return f.events }
func (f *fakeSMTCService) Unsubscribe(ch <-chan smtc.Event)    {}
func (f *fakeSMTCService) GetSessions() []smtc.SessionInfo     { return f.sessions }
func (f *fakeSMTCService) SelectDevice(appID string)           {}
func (f *fakeSMTCService) GetCapabilities() smtc.ControlCapabilities {
	return f.capabilities
}
func (f *fakeSMTCService) Play() error {
	return f.playErr
}
func (f *fakeSMTCService) Pause() error {
	return f.pauseErr
}
func (f *fakeSMTCService) StopPlayback() error {
	return f.stopErr
}
func (f *fakeSMTCService) TogglePlayPause() error {
	return f.toggleErr
}
func (f *fakeSMTCService) SkipNext() error {
	return f.nextErr
}
func (f *fakeSMTCService) SkipPrevious() error {
	return f.previousErr
}
func (f *fakeSMTCService) SeekTo(positionMs int64) error {
	f.seekCalls = append(f.seekCalls, positionMs)
	return f.seekErr
}
func (f *fakeSMTCService) SetShuffle(active bool) error {
	f.shuffleCalls = append(f.shuffleCalls, active)
	return f.shuffleErr
}
func (f *fakeSMTCService) SetRepeat(mode int) error {
	f.repeatCalls = append(f.repeatCalls, mode)
	return f.repeatErr
}

func newTestServer(t *testing.T) (*Server, *fakeSMTCService, context.CancelFunc) {
	t.Helper()
	svc := newFakeSMTCService()
	srv, err := New(&config.Config{
		Server: config.ServerConfig{Port: 11451},
		UI:     config.UIConfig{Theme: "default"},
	}, svc)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go srv.hub.Run(ctx)
	t.Cleanup(cancel)
	return srv, svc, cancel
}

func snapshotForTest(t *testing.T, srv *Server) *stateSnapshot {
	t.Helper()
	snapshot := srv.snapshot()
	return snapshot
}

type testWSClientHandler struct {
	mu      sync.Mutex
	msgs    chan []byte
	closed  chan error
	closeCh chan struct{}
	closeFn sync.Once
}

func newTestWSClientHandler() *testWSClientHandler {
	return &testWSClientHandler{
		msgs:    make(chan []byte, 16),
		closed:  make(chan error, 1),
		closeCh: make(chan struct{}),
	}
}

func (h *testWSClientHandler) OnOpen(socket *gws.Conn) {}

func (h *testWSClientHandler) OnClose(socket *gws.Conn, err error) {
	select {
	case h.closed <- err:
	default:
	}
	h.closeFn.Do(func() {
		close(h.closeCh)
	})
}

func (h *testWSClientHandler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (h *testWSClientHandler) OnPong(socket *gws.Conn, payload []byte) {}

func (h *testWSClientHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	payload := append([]byte(nil), message.Bytes()...)
	select {
	case h.msgs <- payload:
	default:
	}
}

func startWSTestServer(t *testing.T, srv *Server) *httptest.Server {
	t.Helper()
	httpSrv := httptest.NewServer(srv.setupRoutes())
	t.Cleanup(httpSrv.Close)
	return httpSrv
}

func connectWSClient(t *testing.T, baseURL string) (*gws.Conn, *testWSClientHandler) {
	t.Helper()
	handler := newTestWSClientHandler()
	conn, _, err := gws.NewClient(handler, &gws.ClientOption{Addr: "ws" + strings.TrimPrefix(baseURL, "http") + "/ws"})
	if err != nil {
		t.Fatalf("gws.NewClient returned error: %v", err)
	}
	go conn.ReadLoop()
	t.Cleanup(func() {
		_ = conn.WriteClose(1000, nil)
		<-handler.closeCh
	})
	return conn, handler
}

func mustReadEnvelope(t *testing.T, msgs <-chan []byte) wsproto.Envelope {
	t.Helper()
	select {
	case payload := <-msgs:
		var env wsproto.Envelope
		if err := json.Unmarshal(payload, &env); err != nil {
			t.Fatalf("failed to decode envelope: %v", err)
		}
		return env
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for websocket message")
		return wsproto.Envelope{}
	}
}

func TestNew_InitializesServer(t *testing.T) {
	svc := newFakeSMTCService()
	srv, err := New(&config.Config{
		Server: config.ServerConfig{Port: 4321, AllowRemote: true},
		UI:     config.UIConfig{Theme: "default"},
	}, svc)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if srv == nil {
		t.Fatal("New returned nil")
	}
	if got := srv.Address(); got != ":4321" {
		t.Fatalf("Address() = %q, want %q", got, ":4321")
	}
	if srv.httpSrv.ReadTimeout != httpReadTimeout {
		t.Fatalf("ReadTimeout = %v, want %v", srv.httpSrv.ReadTimeout, httpReadTimeout)
	}
	if srv.httpSrv.WriteTimeout != httpWriteTimeout {
		t.Fatalf("WriteTimeout = %v, want %v", srv.httpSrv.WriteTimeout, httpWriteTimeout)
	}
	if srv.httpSrv.IdleTimeout != httpIdleTimeout {
		t.Fatalf("IdleTimeout = %v, want %v", srv.httpSrv.IdleTimeout, httpIdleTimeout)
	}
	if srv.httpSrv.ReadHeaderTimeout != 3*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v, want %v", srv.httpSrv.ReadHeaderTimeout, 3*time.Second)
	}
}

func TestHandleInfoEvent_MessageFormattingAndDeduplication(t *testing.T) {
	srv, _, _ := newTestServer(t)

	data := domain.InfoData{Artist: "Artist", Title: "Title"}
	srv.handleInfoEvent(data)
	first := snapshotForTest(t, srv)
	if first.info == nil {
		t.Fatal("expected info snapshot")
	}
	if first.info.Artist != "Artist" || first.info.Title != "Title" {
		t.Fatalf("unexpected info snapshot: %+v", *first.info)
	}

	var env wsproto.Envelope
	if err := json.Unmarshal(first.infoJSON, &env); err != nil {
		t.Fatalf("failed to decode info message: %v", err)
	}
	if env.Type != wsproto.MsgInfo {
		t.Fatalf("info envelope type = %q, want %q", env.Type, wsproto.MsgInfo)
	}

	srv.handleInfoEvent(data)
	second := snapshotForTest(t, srv)
	if string(first.infoJSON) != string(second.infoJSON) {
		t.Fatal("identical info event changed stored message")
	}
}

func TestHandleProgressEvent_MessageFormattingAndDeduplication(t *testing.T) {
	srv, _, _ := newTestServer(t)

	data := domain.ProgressData{
		Position:        60,
		Duration:        180,
		Status:          4,
		PlaybackRate:    1,
		IsShuffleActive: boolPtr(true),
		AutoRepeatMode:  1,
		LastUpdatedTime: 1700000000000,
	}
	srv.handleProgressEvent(data)
	first := snapshotForTest(t, srv)
	if first.progress == nil {
		t.Fatal("expected progress snapshot")
	}
	if first.progress.Position != 60 || first.progress.Duration != 180 {
		t.Fatalf("unexpected progress snapshot: %+v", *first.progress)
	}

	srv.handleProgressEvent(data)
	second := snapshotForTest(t, srv)
	if string(first.progressJSON) != string(second.progressJSON) {
		t.Fatal("identical progress event changed stored message")
	}
}

func TestHandleAlbumArt_WithStoredHash(t *testing.T) {
	srv, _, _ := newTestServer(t)

	srv.handleInfoEvent(domain.InfoData{
		Artist:               "Artist",
		Title:                "Title",
		ThumbnailData:        []byte{0xFF, 0xD8, 0xFF},
		ThumbnailContentType: "image/jpeg",
	})
	snapshot := snapshotForTest(t, srv)

	var env wsproto.Envelope
	if err := json.Unmarshal(snapshot.infoJSON, &env); err != nil {
		t.Fatalf("decode info envelope: %v", err)
	}
	var payload wsproto.InfoPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("decode info payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, payload.AlbumArt, nil)
	req.SetPathValue("hash", payload.AlbumArt[len("/albumArt/"):])
	w := httptest.NewRecorder()
	srv.handleAlbumArt(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("handleAlbumArt status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("Content-Type = %q, want %q", got, "image/jpeg")
	}
}

func TestHandleAlbumArt_EmptyContentTypeFallsBack(t *testing.T) {
	srv, _, _ := newTestServer(t)

	srv.handleInfoEvent(domain.InfoData{
		Artist:        "Artist",
		Title:         "Title",
		ThumbnailData: []byte{0x01, 0x02},
	})
	snapshot := snapshotForTest(t, srv)
	var env wsproto.Envelope
	_ = json.Unmarshal(snapshot.infoJSON, &env)
	var payload wsproto.InfoPayload
	_ = json.Unmarshal(env.Data, &payload)

	req := httptest.NewRequest(http.MethodGet, payload.AlbumArt, nil)
	req.SetPathValue("hash", payload.AlbumArt[len("/albumArt/"):])
	w := httptest.NewRecorder()
	srv.handleAlbumArt(w, req)

	if got := w.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/octet-stream")
	}
}

func TestHandleWebSocket_HelloOnConnect(t *testing.T) {
	srv, _, _ := newTestServer(t)
	srv.handleInfoEvent(domain.InfoData{Artist: "Artist", Title: "Track"})
	srv.handleProgressEvent(domain.ProgressData{Position: 12, Duration: 30, Status: 4, PlaybackRate: 1, LastUpdatedTime: 1700000000000})

	httpSrv := startWSTestServer(t, srv)
	_, handler := connectWSClient(t, httpSrv.URL)

	hello := mustReadEnvelope(t, handler.msgs)
	if hello.Type != wsproto.MsgHello {
		t.Fatalf("first websocket message type = %q, want %q", hello.Type, wsproto.MsgHello)
	}
	if hello.V != wsproto.ProtocolVersion {
		t.Fatalf("hello version = %d, want %d", hello.V, wsproto.ProtocolVersion)
	}
	var payload wsproto.HelloPayload
	if err := json.Unmarshal(hello.Data, &payload); err != nil {
		t.Fatalf("failed to decode hello payload: %v", err)
	}
	if payload.ServerVersion != version.Version {
		t.Fatalf("hello serverVersion = %q, want %q", payload.ServerVersion, version.Version)
	}
	if !payload.Capabilities["control"] || !payload.Capabilities["heartbeat"] {
		t.Fatalf("hello capabilities = %+v, want control+heartbeat", payload.Capabilities)
	}

	info := mustReadEnvelope(t, handler.msgs)
	if info.Type != wsproto.MsgInfo {
		t.Fatalf("second websocket message type = %q, want %q", info.Type, wsproto.MsgInfo)
	}
	progress := mustReadEnvelope(t, handler.msgs)
	if progress.Type != wsproto.MsgProgress {
		t.Fatalf("third websocket message type = %q, want %q", progress.Type, wsproto.MsgProgress)
	}
}

func TestHandleWebSocket_PingAndControlMessages(t *testing.T) {
	srv, svc, _ := newTestServer(t)
	httpSrv := startWSTestServer(t, srv)
	conn, handler := connectWSClient(t, httpSrv.URL)
	_ = mustReadEnvelope(t, handler.msgs)

	pingPayload, err := json.Marshal(wsproto.Envelope{Type: wsproto.MsgPing, V: wsproto.ProtocolVersion, TS: 12345})
	if err != nil {
		t.Fatalf("marshal ping: %v", err)
	}
	if err := conn.WriteMessage(gws.OpcodeText, pingPayload); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	pong := mustReadEnvelope(t, handler.msgs)
	if pong.Type != wsproto.MsgPong || pong.TS != 12345 {
		t.Fatalf("pong = %+v, want type=%q ts=12345", pong, wsproto.MsgPong)
	}

	control := wsproto.Envelope{
		Type: wsproto.MsgControl,
		V:    wsproto.ProtocolVersion,
		ID:   "req-1",
		TS:   time.Now().UnixMilli(),
	}
	control.Data, err = json.Marshal(wsproto.ControlPayload{Action: "seek", Args: json.RawMessage(`{"position":321}`)})
	if err != nil {
		t.Fatalf("marshal control payload: %v", err)
	}
	controlMsg, err := json.Marshal(control)
	if err != nil {
		t.Fatalf("marshal control envelope: %v", err)
	}
	if err := conn.WriteMessage(gws.OpcodeText, controlMsg); err != nil {
		t.Fatalf("write control: %v", err)
	}
	ack := mustReadEnvelope(t, handler.msgs)
	if ack.Type != wsproto.MsgAck || ack.ID != "req-1" {
		t.Fatalf("ack envelope = %+v", ack)
	}
	var ackPayload wsproto.AckPayload
	if err := json.Unmarshal(ack.Data, &ackPayload); err != nil {
		t.Fatalf("decode ack payload: %v", err)
	}
	if !ackPayload.Success {
		t.Fatalf("ack success = false, error = %q", ackPayload.Error)
	}
	if len(svc.seekCalls) != 1 || svc.seekCalls[0] != 321 {
		t.Fatalf("seek calls = %v, want [321]", svc.seekCalls)
	}
}

func TestHandleWebSocket_UnsupportedVersionClosesConnection(t *testing.T) {
	srv, _, _ := newTestServer(t)
	httpSrv := startWSTestServer(t, srv)
	conn, handler := connectWSClient(t, httpSrv.URL)
	_ = mustReadEnvelope(t, handler.msgs)

	badPayload := []byte(`{"type":"ping","v":1,"ts":1}`)
	if err := conn.WriteMessage(gws.OpcodeText, badPayload); err != nil {
		t.Fatalf("write invalid version message: %v", err)
	}

	select {
	case err := <-handler.closed:
		closeErr, ok := err.(*gws.CloseError)
		if !ok {
			t.Fatalf("close error type = %T, want *gws.CloseError", err)
		}
		if closeErr.Code != 4002 {
			t.Fatalf("close code = %d, want 4002", closeErr.Code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for websocket close")
	}
}

func TestRun_ShutsDownOnContextCancel(t *testing.T) {
	svc := newFakeSMTCService()
	srv, err := New(&config.Config{
		Server: config.ServerConfig{Port: 0},
		UI:     config.UIConfig{Theme: "default"},
	}, svc)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- srv.Run(ctx)
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run returned error: %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}

func TestHandleTheme_TraversalRejected(t *testing.T) {
	srv, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/../../../main.go", nil)
	req.URL.Path = "/../../../main.go"
	w := httptest.NewRecorder()
	srv.handleTheme(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("handleTheme status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestControlHelpersRecordArguments(t *testing.T) {
	srv, svc, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/control/seek", strings.NewReader(`{"position":123}`))
	req.SetPathValue("action", "seek")
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	localhostOnly(srv.handleControl, false)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("seek status = %d, want %d", w.Code, http.StatusOK)
	}
	if len(svc.seekCalls) != 1 || svc.seekCalls[0] != 123 {
		t.Fatalf("seek calls = %v, want [123]", svc.seekCalls)
	}
}

func TestHandleControl_ServiceErrorReturnsSuccessFalse(t *testing.T) {
	srv, svc, _ := newTestServer(t)
	svc.playErr = errors.New("boom")
	req := httptest.NewRequest(http.MethodPost, "/api/control/play", nil)
	req.SetPathValue("action", "play")
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	localhostOnly(srv.handleControl, false)(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["success"] != false {
		t.Fatalf("success = %v, want false", body["success"])
	}
}
