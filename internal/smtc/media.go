//go:build windows

package smtc

import (
	"smtc-now-playing/internal/domain"
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
		log.Debug("failed to get media properties async", "err", err)
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

	// Transient stream failure: readThumbnail() returned nil (e.g. size=0
	// reported by the WinRT IRandomAccessStreamReference on a spurious
	// MediaPropertiesChanged event — common on pause/resume or app state
	// hiccups for Spotify / browser media). If the song hasn't changed and
	// we already have valid cached thumbnail bytes, keep using the cache
	// instead of broadcasting a nil thumbnail, which would cause the client
	// cover art to flicker off.
	//
	// When the song DID change, currentThumbnailData was reset to nil above
	// (in the `if artistChanged` block), so this fallback intentionally
	// does not fire — the retry mechanism below handles song-change cases.
	if !artistChanged && thumbData == nil && s.currentThumbnailData != nil {
		contentType = s.currentThumbnailContentType
		thumbData = s.currentThumbnailData
	}

	// When song changes but thumbnail is not yet available, delay the OnInfo callback
	// to allow SMTC time to write the thumbnail. This prevents the cover art from
	// briefly disappearing when the track changes.
	if artistChanged && thumbData == nil {
		s.currentArtist = escapedArtist
		s.currentTitle = escapedTitle
		s.thumbnailRetryCount = 0
		s.scheduleThumbnailRetry(escapedArtist, escapedTitle, props)
		return
	}

	// readThumbnail returns stored data on dedup hit, so compare by length as a proxy.
	thumbChanged := len(thumbData) != oldThumbLen
	if !artistChanged && !thumbChanged {
		return
	}
	s.currentArtist = escapedArtist
	s.currentTitle = escapedTitle

	// Extract album info and playback type just before firing the event.
	albumTitle, _ := props.GetAlbumTitle()
	albumArtist, _ := props.GetAlbumArtist()
	playbackTypeRef, _ := props.GetPlaybackType()
	playbackType, _ := readNullableInt32(playbackTypeRef)
	s.fanOut(InfoEvent{Data: domain.InfoData{
		Artist:               s.currentArtist,
		Title:                s.currentTitle,
		ThumbnailContentType: contentType,
		ThumbnailData:        thumbData,
		AlbumTitle:           domain.Escape(albumTitle),
		AlbumArtist:          domain.Escape(albumArtist),
		PlaybackType:         int(playbackType),
		SourceApp:            s.selectedAppID,
	}})
}

// scheduleThumbnailRetry arms a one-shot timer that retries readThumbnail
// on the SMTC goroutine. Called from both handleMediaPropertiesChanged
// (initial scheduling on song change) and retryThumbnailAndFireInfo
// (follow-up retries when thumbnail still unavailable).
func (s *Smtc) scheduleThumbnailRetry(artist, title string, props *control.GlobalSystemMediaTransportControlsSessionMediaProperties) {
	s.timerMu.Lock()
	if s.thumbnailRetryTimer != nil {
		s.thumbnailRetryTimer.Stop()
	}
	s.thumbnailRetryTimer = time.AfterFunc(thumbnailRetryDelay, func() {
		select {
		case s.cmdChan <- func() { s.retryThumbnailAndFireInfo(artist, title, props) }:
		default:
			s.droppedEvents.Add(1)
			log.Warn("SMTC event dropped", "type", "ThumbnailRetry", "dropped_total", s.droppedEvents.Load())
		}
	})
	s.timerMu.Unlock()
}

// retryThumbnailAndFireInfo is called ~50ms after a song change when the initial
// readThumbnail returned nil. By this time SMTC has usually written the thumbnail.
// Fires OnInfo regardless of whether a thumbnail is available, so the client
// always receives the new track info (with or without cover art).
// Must be called from the smtc goroutine (via cmdChan).
func (s *Smtc) retryThumbnailAndFireInfo(artist, title string, props *control.GlobalSystemMediaTransportControlsSessionMediaProperties) {
	s.timerMu.Lock()
	s.thumbnailRetryTimer = nil
	s.timerMu.Unlock()
	// If the song has already changed again, this retry is stale — discard it.
	if s.currentArtist != artist || s.currentTitle != title {
		return
	}
	contentType, thumbData := s.readThumbnail()
	if thumbData == nil && s.thumbnailRetryCount < thumbnailRetryMaxAttempts-1 {
		s.thumbnailRetryCount++
		s.scheduleThumbnailRetry(artist, title, props)
		return
	}
	albumTitle, _ := props.GetAlbumTitle()
	albumArtist, _ := props.GetAlbumArtist()
	playbackTypeRef, _ := props.GetPlaybackType()
	playbackType, _ := readNullableInt32(playbackTypeRef)
	s.fanOut(InfoEvent{Data: domain.InfoData{
		Artist:               artist,
		Title:                title,
		ThumbnailContentType: contentType,
		ThumbnailData:        thumbData,
		AlbumTitle:           domain.Escape(albumTitle),
		AlbumArtist:          domain.Escape(albumArtist),
		PlaybackType:         int(playbackType),
		SourceApp:            s.selectedAppID,
	}})
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
	s.fanOut(InfoEvent{Data: domain.InfoData{}})
}

// handlePlaybackInfoChanged responds to WinRT PlaybackInfoChanged events by delegating
// to readTimelineAndProgress, which reads the full timeline state and fires OnProgress
// with complete data (position, duration, status, rate, lastUpdatedTime).
//
// Previously this function fired OnProgress with only Status set — all other fields
// defaulted to zero. That clobbered the client's interpolation state (position=0,
// duration=0, lastUpdatedTime=0) and, when the next 200ms readTimelineAndProgress
// tick happened to dedup (same position/duration/status as server state — e.g. when
// a user replays a song that was already at position 0), left the client stuck
// showing "0:00/" until the song's position advanced by a whole second.
//
// Delegating to readTimelineAndProgress guarantees the client always receives a
// consistent, complete snapshot on status transitions (play/pause/stop/replay).
func (s *Smtc) handlePlaybackInfoChanged() {
	if s.currentSession == nil {
		return
	}
	// readTimelineAndProgress has its own dedup (position+duration+status) that
	// fires whenever any of those change, which is exactly what we want on a
	// playback-info event: the status has almost always changed, and even if it
	// didn't (e.g. rate/shuffle/repeat tweaks), the next progress tick will pick
	// the delta up within 200ms.
	s.readTimelineAndProgress()
}
