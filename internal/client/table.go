package client

import (
	"net"
	"sync"
	"time"
)

type TableEntry struct {
	sess     *TunnelClientSession
	lastTime time.Time
}

type NATManager struct {
	table  sync.Map
	client *Client
}

func (mgr *NATManager) janitor() {
	// todo: clean up expired sessions
}

func (mgr *NATManager) UpsertSession(addr *net.UDPAddr) (sess *TunnelClientSession) {
	key := addr.String()
	entry, ok := mgr.table.Load(key)

	if !ok {
		sess := newSession(mgr.client, addr)
		mgr.table.Store(key, &TableEntry{
			sess:     sess,
			lastTime: time.Now(),
		})
		sess.Open()
		return sess
	} else {
		return entry.(*TableEntry).sess
	}
}
