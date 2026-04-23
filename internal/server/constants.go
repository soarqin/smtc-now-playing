package server

import "time"

const (
	// hotReloadDebounce is the debounce period for file system change events.
	hotReloadDebounce = 500 * time.Millisecond
	// wsHeartbeatInterval is the interval between server heartbeat pings.
	wsHeartbeatInterval = 30 * time.Second
	// wsHeartbeatTimeout is the maximum time to wait for a client pong.
	wsHeartbeatTimeout = 90 * time.Second
	// shutdownGracePeriod is the maximum time for graceful server shutdown.
	shutdownGracePeriod = 10 * time.Second
	// subscribeBufSize is the default event channel buffer size for server subscription.
	subscribeBufSize = 64
	// hubChanCapacity is the size of the hub command channel.
	hubChanCapacity = 64
)
