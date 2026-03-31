//go:build windows

package smtc

import (
	"log/slog"
	"time"
)

// readTimelineAndProgress reads position/duration/status from the current session
// and fires OnProgress callback when values change.
// Called on every 200ms timer tick.
// Replicates C++ update() timeline handling at c/smtc.cpp:180-212.
func (s *Smtc) readTimelineAndProgress() {
	if s.currentSession == nil {
		return
	}

	// Get timeline properties
	timeline, err := s.currentSession.GetTimelineProperties()
	if err != nil || timeline == nil {
		if err != nil {
			slog.Debug("failed to get timeline properties", "err", err)
		}
		return
	}

	// Get playback info
	playbackInfo, err := s.currentSession.GetPlaybackInfo()
	if err != nil || playbackInfo == nil {
		if err != nil {
			slog.Debug("failed to get playback info", "err", err)
		}
		return
	}

	// Get playback status (WinRT enum: Closed=0, Opened=1, Changing=2, Stopped=3, Playing=4, Paused=5)
	status, err := playbackInfo.GetPlaybackStatus()
	if err != nil {
		slog.Debug("failed to get playback status", "err", err)
		return
	}
	newStatus := int(status)

	// Get position (WinRT TimeSpan.Duration = 100ns ticks)
	positionSpan, err := timeline.GetPosition()
	if err != nil {
		slog.Debug("failed to get timeline position", "err", err)
		return
	}

	// Get lastUpdatedTime (WinRT DateTime.UniversalTime = 100ns ticks since 1601-01-01)
	lastUpdated, err := timeline.GetLastUpdatedTime()
	if err != nil {
		slog.Debug("failed to get last updated time", "err", err)
		return
	}

	// Get end time / duration (WinRT TimeSpan.Duration = 100ns ticks)
	endTimeSpan, err := timeline.GetEndTime()
	if err != nil {
		slog.Debug("failed to get end time", "err", err)
		return
	}

	// Read playback rate via helper (default 1.0 if unavailable).
	// Matches C++: playbackRatePtr ? playbackRatePtr.Value() : 1.0
	newPlaybackRate := 1.0
	playbackRateRef, _ := playbackInfo.GetPlaybackRate()
	if val, ok := readNullableFloat64(playbackRateRef); ok {
		newPlaybackRate = val
	}

	// Read shuffle active status (nil = unavailable, &true = on, &false = off).
	var newIsShuffleActive *bool
	shuffleRef, _ := playbackInfo.GetIsShuffleActive()
	if val, ok := readNullableBool(shuffleRef); ok {
		v := val
		newIsShuffleActive = &v
	}

	// Read auto-repeat mode (0=None, 1=Track, 2=List).
	newAutoRepeatMode := 0
	repeatRef, _ := playbackInfo.GetAutoRepeatMode()
	if val, ok := readNullableInt32(repeatRef); ok {
		newAutoRepeatMode = int(val)
	}

	// Convert Windows DateTime (100ns ticks since 1601-01-01) to Unix milliseconds.
	// Windows-to-Unix epoch offset: 116,444,736,000,000,000 * 100ns ticks.
	const windowsToUnixEpochTicks = int64(116444736000000000)
	var newLastUpdatedMs int64
	if lastUpdated.UniversalTime != 0 {
		newLastUpdatedMs = (lastUpdated.UniversalTime - windowsToUnixEpochTicks) / 10000
	}

	var newPosition, newDuration int

	if lastUpdated.UniversalTime == 0 {
		// Edge case: no valid timestamp — position and duration are unknown.
		// Matches C++ smtc.cpp:206-211: sets position=0, duration=0.
		newPosition = 0
		newDuration = 0
	} else {
		// Position interpolation:
		// C++: position += (now - lastUpdatedTime) * playbackRate
		// Convert 100ns ticks to seconds: ticks / 10_000_000
		positionTicks := positionSpan.Duration

		if newStatus == StatusPlaying {
			// Convert lastUpdatedTime (Windows FILETIME, 100ns ticks since 1601-01-01)
			// to Go time.Time for delta calculation.
			lastUpdatedNano := (lastUpdated.UniversalTime - windowsToUnixEpochTicks) * 100
			lastUpdatedGoTime := time.Unix(0, lastUpdatedNano)

			// Delta in 100ns ticks: time.Since returns nanoseconds, divide by 100.
			deltaTicks := int64(time.Since(lastUpdatedGoTime)) / 100
			interpolatedTicks := positionTicks + int64(float64(deltaTicks)*newPlaybackRate)
			newPosition = int(interpolatedTicks / 10_000_000)
		} else {
			newPosition = int(positionTicks / 10_000_000)
		}

		newDuration = int(endTimeSpan.Duration / 10_000_000)
	}

	// Only fire callback if position/duration/status changed.
	s.mu.Lock()
	if newPosition == s.currentPosition && newDuration == s.currentDuration && newStatus == s.currentStatus {
		s.mu.Unlock()
		return
	}
	s.currentPosition = newPosition
	s.currentDuration = newDuration
	s.currentStatus = newStatus
	s.mu.Unlock()

	if s.opts.OnProgress != nil {
		s.opts.OnProgress(ProgressData{
			Position:        newPosition,
			Duration:        newDuration,
			Status:          newStatus,
			PlaybackRate:    newPlaybackRate,
			IsShuffleActive: newIsShuffleActive,
			AutoRepeatMode:  newAutoRepeatMode,
			LastUpdatedTime: newLastUpdatedMs,
		})
	}
}

// startProgressTimer starts a 200ms ticker that calls readTimelineAndProgress on each tick.
// Must be called from the smtc goroutine.
func (s *Smtc) startProgressTimer() {
	s.progressTicker = time.NewTicker(200 * time.Millisecond)
}

// stopProgressTimer stops the progress ticker.
func (s *Smtc) stopProgressTimer() {
	if s.progressTicker != nil {
		s.progressTicker.Stop()
		s.progressTicker = nil
	}
}
