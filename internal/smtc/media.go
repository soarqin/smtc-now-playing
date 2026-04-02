//go:build windows

package smtc

import (
	"log/slog"
	"time"
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
		slog.Debug("failed to get media properties async", "err", err)
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

	// When song changes but thumbnail is not yet available, delay the OnInfo callback
	// to allow SMTC time to write the thumbnail. This prevents the cover art from
	// briefly disappearing when the track changes.
	if artistChanged && thumbData == nil {
		s.currentArtist = escapedArtist
		s.currentTitle = escapedTitle
		// Cancel any pending retry from a previous song change.
		if s.thumbnailRetryTimer != nil {
			s.thumbnailRetryTimer.Stop()
		}
		s.thumbnailRetryTimer = time.AfterFunc(50*time.Millisecond, func() {
			select {
			case s.cmdChan <- func() { s.retryThumbnailAndFireInfo(escapedArtist, escapedTitle, props) }:
			default:
				s.droppedEvents.Add(1)
				slog.Warn("SMTC event dropped", "type", "ThumbnailRetry", "dropped_total", s.droppedEvents.Load())
			}
		})
		return
	}

	// readThumbnail returns stored data on dedup hit, so compare by length as a proxy.
	thumbChanged := len(thumbData) != oldThumbLen
	if !artistChanged && !thumbChanged {
		return
	}
	s.currentArtist = escapedArtist
	s.currentTitle = escapedTitle

	if s.opts.OnInfo != nil {
		// Extract album info and playback type just before firing the callback.
		albumTitle, _ := props.GetAlbumTitle()
		albumArtist, _ := props.GetAlbumArtist()
		playbackTypeRef, _ := props.GetPlaybackType()
		playbackType, _ := readNullableInt32(playbackTypeRef)
		s.opts.OnInfo(InfoData{
			Artist:               s.currentArtist,
			Title:                s.currentTitle,
			ThumbnailContentType: contentType,
			ThumbnailData:        thumbData,
			AlbumTitle:           escape(albumTitle),
			AlbumArtist:          escape(albumArtist),
			PlaybackType:         int(playbackType),
		})
	}
}

// retryThumbnailAndFireInfo is called ~50ms after a song change when the initial
// readThumbnail returned nil. By this time SMTC has usually written the thumbnail.
// Fires OnInfo regardless of whether a thumbnail is available, so the client
// always receives the new track info (with or without cover art).
// Must be called from the smtc goroutine (via cmdChan).
func (s *Smtc) retryThumbnailAndFireInfo(artist, title string, props *control.GlobalSystemMediaTransportControlsSessionMediaProperties) {
	s.thumbnailRetryTimer = nil
	// If the song has already changed again, this retry is stale — discard it.
	if s.currentArtist != artist || s.currentTitle != title {
		return
	}
	contentType, thumbData := s.readThumbnail()
	if s.opts.OnInfo != nil {
		albumTitle, _ := props.GetAlbumTitle()
		albumArtist, _ := props.GetAlbumArtist()
		playbackTypeRef, _ := props.GetPlaybackType()
		playbackType, _ := readNullableInt32(playbackTypeRef)
		s.opts.OnInfo(InfoData{
			Artist:               artist,
			Title:                title,
			ThumbnailContentType: contentType,
			ThumbnailData:        thumbData,
			AlbumTitle:           escape(albumTitle),
			AlbumArtist:          escape(albumArtist),
			PlaybackType:         int(playbackType),
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
		if err != nil {
			slog.Debug("failed to get playback info", "err", err)
		}
		return
	}

	// Map WinRT GlobalSystemMediaTransportControlsSessionPlaybackStatus to our int constants.
	// WinRT enum: Closed=0, Opened=1, Changing=2, Stopped=3, Playing=4, Paused=5.
	// Our StatusClosed..StatusPaused constants match exactly.
	status, err := playbackInfo.GetPlaybackStatus()
	if err != nil {
		slog.Debug("failed to get playback status", "err", err)
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
