//go:build windows

package smtc

import (
	"fmt"
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
	iidCurrentSessionChangedHandler    *ole.GUID
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
	// TypedEventHandler<GlobalSystemMediaTransportControlsSessionManager, CurrentSessionChangedEventArgs>
	iidCurrentSessionChangedHandler = ole.NewGUID(winrt.ParameterizedInstanceGUID(
		foundation.GUIDTypedEventHandler,
		control.SignatureGlobalSystemMediaTransportControlsSessionManager,
		control.SignatureCurrentSessionChangedEventArgs,
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

// initSessionManager initializes the SMTC session manager via WinRT async call.
// Must be called after RoInitialize (from the smtc goroutine).
// Replicates C++ Smtc::init() at c/smtc.cpp:136-149.
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

	session, err := s.sessionManager.GetCurrentSession()
	if err != nil {
		return err
	}
	s.currentSession = session
	if s.currentSession != nil {
		s.subscribePropertyEvents()
	}
	s.subscribeSessionChanged()
	return nil
}

// subscribeSessionChanged registers the CurrentSessionChanged event on the session manager.
// The token is stored in s.sessionChangedToken for later removal (Task 8).
func (s *Smtc) subscribeSessionChanged() {
	handler := foundation.NewTypedEventHandler(iidCurrentSessionChangedHandler, func(
		_ *foundation.TypedEventHandler,
		_ unsafe.Pointer,
		_ unsafe.Pointer,
	) {
		s.switchSession()
	})
	token, _ := s.sessionManager.AddCurrentSessionChanged(handler)
	s.sessionChangedToken = token
	// Release our initial ref; WinRT holds its own reference for the subscription lifetime.
	handler.Release()
}

// switchSession handles a session change: unsubscribes old property events, retrieves the new
// session, and either clears state (null session) or subscribes new property events.
// Replicates C++ session change handling at c/smtc.cpp:152-171.
func (s *Smtc) switchSession() {
	if s.currentSession != nil {
		s.unsubscribePropertyEvents()
	}

	session, err := s.sessionManager.GetCurrentSession()
	if err != nil {
		return
	}
	s.currentSession = session

	if s.currentSession == nil {
		// No active media session: clear all state and fire empty callbacks.
		// Matches C++ behavior: clears fields and marks info/progress dirty.
		s.currentArtist = ""
		s.currentTitle = ""
		s.currentStatus = StatusClosed
		s.currentThumbnailSize = 0
		if s.opts.OnInfo != nil {
			s.opts.OnInfo(InfoData{})
		}
		if s.opts.OnProgress != nil {
			s.opts.OnProgress(ProgressData{Status: StatusClosed})
		}
		return
	}

	s.subscribePropertyEvents()
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
		s.handleMediaPropertiesChanged()
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
		s.handlePlaybackInfoChanged()
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
