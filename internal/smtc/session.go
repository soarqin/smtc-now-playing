//go:build windows

package smtc

import (
	"fmt"
	"log/slog"
	"unsafe"

	"github.com/go-ole/go-ole"
	winrt "github.com/saltosystems/winrt-go"
	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/media/control"
)

// Parameterized IIDs for TypedEventHandler<T,T> and AsyncOperationCompletedHandler<T>.
// Computed once at package init to avoid repeated SHA-1 hashing on every subscription.
var (
	iidSessionManagerCompletedHandler  *ole.GUID
	iidSessionsChangedHandler          *ole.GUID
	iidMediaPropertiesChangedHandler   *ole.GUID
	iidPlaybackInfoChangedHandler      *ole.GUID
	iidMediaPropertiesCompletedHandler *ole.GUID
)

func init() {
	// IAsyncOperationCompletedHandler<GlobalSystemMediaTransportControlsSessionManager>
	iidSessionManagerCompletedHandler = ole.NewGUID(winrt.ParameterizedInstanceGUID(
		foundation.GUIDAsyncOperationCompletedHandler,
		control.SignatureGlobalSystemMediaTransportControlsSessionManager,
	))
	// TypedEventHandler<GlobalSystemMediaTransportControlsSessionManager, SessionsChangedEventArgs>
	iidSessionsChangedHandler = ole.NewGUID(winrt.ParameterizedInstanceGUID(
		foundation.GUIDTypedEventHandler,
		control.SignatureGlobalSystemMediaTransportControlsSessionManager,
		control.SignatureSessionsChangedEventArgs,
	))
	// TypedEventHandler<GlobalSystemMediaTransportControlsSession, MediaPropertiesChangedEventArgs>
	iidMediaPropertiesChangedHandler = ole.NewGUID(winrt.ParameterizedInstanceGUID(
		foundation.GUIDTypedEventHandler,
		control.SignatureGlobalSystemMediaTransportControlsSession,
		control.SignatureMediaPropertiesChangedEventArgs,
	))
	// TypedEventHandler<GlobalSystemMediaTransportControlsSession, PlaybackInfoChangedEventArgs>
	iidPlaybackInfoChangedHandler = ole.NewGUID(winrt.ParameterizedInstanceGUID(
		foundation.GUIDTypedEventHandler,
		control.SignatureGlobalSystemMediaTransportControlsSession,
		control.SignaturePlaybackInfoChangedEventArgs,
	))
	// IAsyncOperationCompletedHandler<GlobalSystemMediaTransportControlsSessionMediaProperties>
	iidMediaPropertiesCompletedHandler = ole.NewGUID(winrt.ParameterizedInstanceGUID(
		foundation.GUIDAsyncOperationCompletedHandler,
		control.SignatureGlobalSystemMediaTransportControlsSessionMediaProperties,
	))
}

// isDeadSessionError returns true if err represents a dead or unavailable WinRT session.
// Sessions can become unavailable when the corresponding media application exits.
// 0x800706BA: RPC server unavailable. 0x80070015: Device not ready.
func isDeadSessionError(err error) bool {
	if err == nil {
		return false
	}
	if oleErr, ok := err.(*ole.OleError); ok {
		hr := oleErr.Code()
		return hr == 0x800706BA || hr == 0x80070015
	}
	return false
}

// initSessionManager initializes the SMTC session manager via WinRT async call.
// Must be called after RoInitialize (from the smtc goroutine).
func (s *Smtc) initSessionManager() error {
	op, err := control.GlobalSystemMediaTransportControlsSessionManagerRequestAsync()
	if err != nil {
		return err
	}
	result, status := waitForAsync(op, iidSessionManagerCompletedHandler)
	if status != foundation.AsyncStatusCompleted {
		return fmt.Errorf("session manager RequestAsync failed: status %d", status)
	}
	s.sessionManager = (*control.GlobalSystemMediaTransportControlsSessionManager)(result)

	s.enumerateSessions()
	s.subscribeSessionsChanged()
	return nil
}

// subscribeSessionsChanged registers the SessionsChanged event on the session manager.
// The token is stored in s.sessionsChangedToken for later removal.
func (s *Smtc) subscribeSessionsChanged() {
	handler := foundation.NewTypedEventHandler(iidSessionsChangedHandler, func(
		_ *foundation.TypedEventHandler,
		_ unsafe.Pointer,
		_ unsafe.Pointer,
	) {
		select {
		case s.cmdChan <- func() { s.enumerateSessions() }:
		default:
			s.droppedEvents.Add(1)
			slog.Warn("SMTC event dropped", "type", "SessionsChanged", "dropped_total", s.droppedEvents.Load())
		}
	})
	token, _ := s.sessionManager.AddSessionsChanged(handler)
	s.sessionsChangedToken = token
	// Release our initial ref; WinRT holds its own reference for the subscription lifetime.
	handler.Release()
}

// enumerateSessions queries all current SMTC sessions, builds the session list with
// duplicate-AppID disambiguation, and switches to the appropriate session.
// Called on startup and whenever the SessionsChanged event fires.
// Must be called from the smtc goroutine.
func (s *Smtc) enumerateSessions() {
	vectorView, err := s.sessionManager.GetSessions()
	if err != nil {
		// Failed to get sessions — treat as empty.
		s.applySessionList(nil, nil)
		return
	}

	size, err := vectorView.GetSize()
	if err != nil || size == 0 {
		s.applySessionList(nil, nil)
		return
	}

	// Build filtered lists, skipping dead or unreachable sessions.
	appIDs := make([]string, 0, size)
	objects := make([]*control.GlobalSystemMediaTransportControlsSession, 0, size)

	for i := uint32(0); i < size; i++ {
		ptr, err := vectorView.GetAt(i)
		if err != nil || ptr == nil {
			continue
		}
		session := (*control.GlobalSystemMediaTransportControlsSession)(ptr)
		appID, err := session.GetSourceAppUserModelId()
		if err != nil {
			if isDeadSessionError(err) {
				// Session died between enumeration and query — skip it gracefully.
				continue
			}
			appID = ""
		}
		appIDs = append(appIDs, appID)
		objects = append(objects, session)
	}

	// Build SessionInfo list with duplicate AppID disambiguation.
	counts := make(map[string]int)
	for _, appID := range appIDs {
		counts[appID]++
	}
	indices := make(map[string]int)
	sessions := make([]SessionInfo, len(appIDs))
	for i, appID := range appIDs {
		name := friendlyAppName(appID)
		if counts[appID] > 1 {
			indices[appID]++
			name = fmt.Sprintf("%s (%d)", name, indices[appID])
		}
		sessions[i] = SessionInfo{AppID: appID, Name: name}
	}

	s.applySessionList(sessions, objects)
}

// applySessionList stores the new session list under the mutex, fires OnSessionsChanged,
// and switches to the appropriate session. Called only from the smtc goroutine.
func (s *Smtc) applySessionList(sessions []SessionInfo, objects []*control.GlobalSystemMediaTransportControlsSession) {
	// Determine which session to select: keep selectedAppID if still present,
	// otherwise default to index 0.
	selectedIndex := -1
	if s.selectedAppID != "" {
		for i, sess := range sessions {
			if sess.AppID == s.selectedAppID {
				selectedIndex = i
				break
			}
		}
	}
	if selectedIndex == -1 && len(sessions) > 0 {
		selectedIndex = 0
	}

	s.mu.Lock()
	s.sessions = sessions
	s.sessionObjects = objects
	s.mu.Unlock()

	if s.opts.OnSessionsChanged != nil {
		s.opts.OnSessionsChanged(sessions)
	}

	if len(sessions) == 0 {
		// No active sessions: clear all state and fire empty callbacks.
		s.currentSession = nil
		s.mu.Lock()
		s.currentArtist = ""
		s.currentTitle = ""
		s.currentStatus = StatusClosed
		s.currentThumbnailSize = 0
		s.mu.Unlock()
		if s.opts.OnInfo != nil {
			s.opts.OnInfo(InfoData{})
		}
		if s.opts.OnProgress != nil {
			s.opts.OnProgress(ProgressData{Status: StatusClosed})
		}
		return
	}

	s.switchToSession(selectedIndex)
}

// selectDevice sets the selected device by appID and switches to it if found.
// Must be called from the smtc goroutine (via cmdChan).
func (s *Smtc) selectDevice(appID string) {
	s.selectedAppID = appID
	s.mu.Lock()
	sessions := s.sessions
	objects := s.sessionObjects
	s.mu.Unlock()
	for i, sess := range sessions {
		if sess.AppID == appID && i < len(objects) {
			s.switchToSession(i)
			return
		}
	}
}

// switchToSession switches SMTC monitoring to the session at the given index.
// Unsubscribes old property events, subscribes new ones, reads initial media properties,
// and fires OnSelectedDeviceChange. Must be called from the smtc goroutine.
func (s *Smtc) switchToSession(index int) {
	if s.currentSession != nil {
		s.unsubscribePropertyEvents()
	}

	s.mu.Lock()
	objects := s.sessionObjects
	sessions := s.sessions
	s.mu.Unlock()

	if index < 0 || index >= len(objects) {
		return
	}

	s.currentSession = objects[index]
	s.subscribePropertyEvents()
	s.handleMediaPropertiesChanged()

	if s.opts.OnSelectedDeviceChange != nil && index < len(sessions) {
		appID := sessions[index].AppID
		slog.Info("SMTC session changed", "app", appID)
		s.opts.OnSelectedDeviceChange(appID)
	}
}

// subscribePropertyEvents subscribes MediaPropertiesChanged and PlaybackInfoChanged events
// on the current session. Tokens are stored in s.mediaPropertiesChangedToken and
// s.playbackInfoChangedToken for cleanup in unsubscribePropertyEvents.
func (s *Smtc) subscribePropertyEvents() {
	mediaHandler := foundation.NewTypedEventHandler(iidMediaPropertiesChangedHandler, func(
		_ *foundation.TypedEventHandler,
		_ unsafe.Pointer,
		_ unsafe.Pointer,
	) {
		select {
		case s.cmdChan <- func() { s.handleMediaPropertiesChanged() }:
		default:
			s.droppedEvents.Add(1)
			slog.Warn("SMTC event dropped", "type", "MediaPropertiesChanged", "dropped_total", s.droppedEvents.Load())
		}
	})
	token, _ := s.currentSession.AddMediaPropertiesChanged(mediaHandler)
	s.mediaPropertiesChangedToken = token
	// Release our initial ref; WinRT holds its own reference for the subscription lifetime.
	mediaHandler.Release()

	playbackHandler := foundation.NewTypedEventHandler(iidPlaybackInfoChangedHandler, func(
		_ *foundation.TypedEventHandler,
		_ unsafe.Pointer,
		_ unsafe.Pointer,
	) {
		select {
		case s.cmdChan <- func() { s.handlePlaybackInfoChanged() }:
		default:
			s.droppedEvents.Add(1)
			slog.Warn("SMTC event dropped", "type", "PlaybackInfoChanged", "dropped_total", s.droppedEvents.Load())
		}
	})
	token, _ = s.currentSession.AddPlaybackInfoChanged(playbackHandler)
	s.playbackInfoChangedToken = token
	// Release our initial ref; WinRT holds its own reference for the subscription lifetime.
	playbackHandler.Release()
}

// unsubscribePropertyEvents removes MediaPropertiesChanged and PlaybackInfoChanged handlers
// using the stored tokens, then zeroes the tokens to prevent double-removal.
func (s *Smtc) unsubscribePropertyEvents() {
	_ = s.currentSession.RemoveMediaPropertiesChanged(s.mediaPropertiesChangedToken)
	s.mediaPropertiesChangedToken = foundation.EventRegistrationToken{}
	_ = s.currentSession.RemovePlaybackInfoChanged(s.playbackInfoChangedToken)
	s.playbackInfoChangedToken = foundation.EventRegistrationToken{}
}
