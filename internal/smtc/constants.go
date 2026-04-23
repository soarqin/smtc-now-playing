//go:build windows

package smtc

import "time"

const (
	// progressTickInterval is the period between progress timeline reads.
	progressTickInterval = 200 * time.Millisecond
	// thumbnailRetryDelay is the wait before retrying thumbnail reads on song change.
	thumbnailRetryDelay = 50 * time.Millisecond
	// thumbnailRetryMaxAttempts caps how many times readThumbnail is retried
	// per song change before the InfoEvent is fired with the current (possibly
	// empty) thumbnail state. Total retry window: thumbnailRetryDelay * thumbnailRetryMaxAttempts.
	thumbnailRetryMaxAttempts = 10
	// cmdChanCapacity is the size of the internal command channel.
	cmdChanCapacity = 32
)
