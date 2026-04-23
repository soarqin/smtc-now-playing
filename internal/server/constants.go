package server

import "time"

const (
	// hotReloadDebounce is the debounce period for file system change events.
	hotReloadDebounce = 500 * time.Millisecond
	// shutdownGracePeriod is the maximum time for graceful server shutdown.
	shutdownGracePeriod = 10 * time.Second
	// subscribeBufSize is the default event channel buffer size for server subscription.
	subscribeBufSize = 64
	// hubChanCapacity is the size of the hub command channel.
	hubChanCapacity = 64
)
