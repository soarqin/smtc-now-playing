//go:build windows

package smtc

import (
	"unsafe"

	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/media/control"
)

// handleMediaPropertiesChanged reads artist/title from the current session and fires OnInfo
// callback when values change. Called when MediaPropertiesChanged event fires or on initial
// session connect. Replicates C++ getMediaProperties() at c/smtc.cpp:236-261.
func (s *Smtc) handleMediaPropertiesChanged() {
	if s.currentSession == nil {
		return
	}

	op, err := s.currentSession.TryGetMediaPropertiesAsync()
	if err != nil {
		s.clearMediaInfo()
		return
	}

	result, status := waitForAsync(op, iidMediaPropertiesCompletedHandler)
	if status != foundation.AsyncStatusCompleted || result == nil {
		s.clearMediaInfo()
		return
	}

	props := (*control.GlobalSystemMediaTransportControlsSessionMediaProperties)(unsafe.Pointer(result))

	artist, _ := props.GetArtist()
	title, _ := props.GetTitle()

	escapedArtist := escape(artist)
	escapedTitle := escape(title)

	// Store properties for thumbnail access.
	s.currentProperties = props

	// Only fire callback if artist, title, or thumbnail actually changed.
	artistChanged := escapedArtist != s.currentArtist || escapedTitle != s.currentTitle
	// Reset dedup state when song changes so readThumbnail always does a fresh read.
	if artistChanged {
		s.currentThumbnailSize = 0
		s.currentThumbnailData = nil
		s.currentThumbnailContentType = ""
	}
	// Snapshot BEFORE readThumbnail() mutates s.currentThumbnailData.
	oldThumbLen := len(s.currentThumbnailData)
	// Always read thumbnail — it may have changed independently of artist/title.
	contentType, thumbData := s.readThumbnail()
	// readThumbnail returns stored data on dedup hit, so compare by length as a proxy.
	thumbChanged := len(thumbData) != oldThumbLen
	if !artistChanged && !thumbChanged {
		return
	}
	s.currentArtist = escapedArtist
	s.currentTitle = escapedTitle

	if s.opts.OnInfo != nil {
		s.opts.OnInfo(InfoData{
			Artist:               s.currentArtist,
			Title:                s.currentTitle,
			ThumbnailContentType: contentType,
			ThumbnailData:        thumbData,
		})
	}
}

// clearMediaInfo clears artist/title/properties state and fires an empty OnInfo callback.
// Called when TryGetMediaPropertiesAsync fails or returns nil, mirroring C++ null properties
// handling at c/smtc.cpp:248-255.
func (s *Smtc) clearMediaInfo() {
	if s.currentArtist == "" && s.currentTitle == "" {
		return
	}
	s.currentArtist = ""
	s.currentTitle = ""
	s.currentProperties = nil
	s.currentThumbnailSize = 0
	s.currentThumbnailData = nil
	s.currentThumbnailContentType = ""
	if s.opts.OnInfo != nil {
		s.opts.OnInfo(InfoData{})
	}
}

// handlePlaybackInfoChanged reads playback status from the current session and fires
// OnProgress callback when the status changes. Position and duration are handled by
// readTimelineAndProgress. Replicates C++ playback status handling at c/smtc.cpp.
func (s *Smtc) handlePlaybackInfoChanged() {
	if s.currentSession == nil {
		return
	}

	playbackInfo, err := s.currentSession.GetPlaybackInfo()
	if err != nil || playbackInfo == nil {
		return
	}

	// Map WinRT GlobalSystemMediaTransportControlsSessionPlaybackStatus to our int constants.
	// WinRT enum: Closed=0, Opened=1, Changing=2, Stopped=3, Playing=4, Paused=5.
	// Our StatusClosed..StatusPaused constants match exactly.
	status, err := playbackInfo.GetPlaybackStatus()
	if err != nil {
		return
	}

	newStatus := int(status)
	s.mu.Lock()
	if newStatus == s.currentStatus {
		s.mu.Unlock()
		return
	}
	s.currentStatus = newStatus
	s.mu.Unlock()

	if s.opts.OnProgress != nil {
		s.opts.OnProgress(ProgressData{
			Status: newStatus,
			// Position and Duration filled by readTimelineAndProgress.
		})
	}
}
