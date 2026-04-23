//go:build windows

package smtc

import "time"

const (
	// progressTickInterval is the period between progress timeline reads.
	progressTickInterval = 200 * time.Millisecond
	// thumbnailRetryDelay is the wait before retrying thumbnail reads on song change.
	thumbnailRetryDelay = 50 * time.Millisecond
	// cmdChanCapacity is the size of the internal command channel.
	cmdChanCapacity = 32
)
