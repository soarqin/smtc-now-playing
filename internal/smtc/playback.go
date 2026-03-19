//go:build windows

package smtc

import (
	"time"
	"unsafe"
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
		return
	}

	// Get playback info
	playbackInfo, err := s.currentSession.GetPlaybackInfo()
	if err != nil || playbackInfo == nil {
		return
	}

	// Get playback status (WinRT enum: Closed=0, Opened=1, Changing=2, Stopped=3, Playing=4, Paused=5)
	status, err := playbackInfo.GetPlaybackStatus()
	if err != nil {
		return
	}
	newStatus := int(status)

	// Get position (WinRT TimeSpan.Duration = 100ns ticks)
	positionSpan, err := timeline.GetPosition()
	if err != nil {
		return
	}

	// Get lastUpdatedTime (WinRT DateTime.UniversalTime = 100ns ticks since 1601-01-01)
	lastUpdated, err := timeline.GetLastUpdatedTime()
	if err != nil {
		return
	}

	// Get end time / duration (WinRT TimeSpan.Duration = 100ns ticks)
	endTimeSpan, err := timeline.GetEndTime()
	if err != nil {
		return
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
			// Get playback rate (default 1.0 if nil or error).
			// Matches C++: playbackRatePtr ? playbackRatePtr.Value() : 1.0
			rate := 1.0
			playbackRateRef, rateErr := playbackInfo.GetPlaybackRate()
			if rateErr == nil && playbackRateRef != nil {
				if ptr, valErr := playbackRateRef.GetValue(); valErr == nil {
					// IReference<Double>.GetValue() writes the float64 bits into an
					// unsafe.Pointer-sized variable. Reinterpret the storage as float64.
					rate = *(*float64)(unsafe.Pointer(&ptr))
				}
			}

			// Convert lastUpdatedTime (Windows FILETIME, 100ns ticks since 1601-01-01)
			// to Go time.Time for delta calculation.
			// Windows-to-Unix epoch offset: 116,444,736,000,000,000 * 100ns ticks.
			const windowsToUnixEpochTicks = int64(116444736000000000)
			lastUpdatedNano := (lastUpdated.UniversalTime - windowsToUnixEpochTicks) * 100
			lastUpdatedTime := time.Unix(0, lastUpdatedNano)

			// Delta in 100ns ticks: time.Since returns nanoseconds, divide by 100.
			deltaTicks := int64(time.Since(lastUpdatedTime)) / 100
			interpolatedTicks := positionTicks + int64(float64(deltaTicks)*rate)
			newPosition = int(interpolatedTicks / 10_000_000)
		} else {
			newPosition = int(positionTicks / 10_000_000)
		}

		newDuration = int(endTimeSpan.Duration / 10_000_000)
	}

	// Only fire callback if values changed.
	if newPosition == s.currentPosition && newDuration == s.currentDuration && newStatus == s.currentStatus {
		return
	}
	s.currentPosition = newPosition
	s.currentDuration = newDuration
	s.currentStatus = newStatus

	if s.opts.OnProgress != nil {
		s.opts.OnProgress(ProgressData{
			Position: s.currentPosition,
			Duration: s.currentDuration,
			Status:   s.currentStatus,
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
