package server

import (
	"context"
	"log/slog"

	"github.com/lxzan/gws"
)

type hubCmd interface{ isHubCmd() }

type addConnCmd struct{ conn *gws.Conn }

func (addConnCmd) isHubCmd() {}

type removeConnCmd struct{ conn *gws.Conn }

func (removeConnCmd) isHubCmd() {}

type broadcastCmd struct{ msg []byte }

func (broadcastCmd) isHubCmd() {}

type shutdownHubCmd struct{}

func (shutdownHubCmd) isHubCmd() {}

type hub struct {
	ch          chan hubCmd
	connections map[*gws.Conn]struct{}
}

func newHub() *hub {
	return &hub{
		ch:          make(chan hubCmd, hubChanCapacity),
		connections: make(map[*gws.Conn]struct{}),
	}
}

func (h *hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			for conn := range h.connections {
				_ = conn.WriteClose(1000, nil)
				if netConn := conn.NetConn(); netConn != nil {
					_ = netConn.Close()
				}
			}
			return
		case cmd := <-h.ch:
			switch c := cmd.(type) {
			case addConnCmd:
				h.connections[c.conn] = struct{}{}
			case removeConnCmd:
				delete(h.connections, c.conn)
			case broadcastCmd:
				for conn := range h.connections {
					if err := conn.WriteMessage(gws.OpcodeText, c.msg); err != nil {
						slog.Debug("websocket write failed", "err", err)
					}
				}
			case shutdownHubCmd:
				for conn := range h.connections {
					_ = conn.WriteClose(1000, nil)
					if netConn := conn.NetConn(); netConn != nil {
						_ = netConn.Close()
					}
				}
				return
			}
		}
	}
}

func (h *hub) Add(conn *gws.Conn) {
	h.ch <- addConnCmd{conn: conn}
}

func (h *hub) Remove(conn *gws.Conn) {
	select {
	case h.ch <- removeConnCmd{conn: conn}:
	default:
	}
}

func (h *hub) Broadcast(msg []byte) {
	select {
	case h.ch <- broadcastCmd{msg: msg}:
	default:
		slog.Warn("hub broadcast channel full, dropping message")
	}
}

func (h *hub) Shutdown() {
	select {
	case h.ch <- shutdownHubCmd{}:
	default:
	}
}
