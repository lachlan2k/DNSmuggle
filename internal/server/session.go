package server

import (
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/lachlan2k/dns-tunnel/internal/request"
)

type Session struct {
	startTime    time.Time
	lastPollTime time.Time
	id           request.SessionID
	conn         *net.UDPConn
	readLock     sync.Mutex
}

func createAndDialSession(dialAddr *net.UDPAddr) (sess *Session, err error) {
	sess = &Session{
		id: rand.Uint64(),
	}

	err = sess.Open(dialAddr)

	if err != nil {
		return nil, err
	}

	return
}

func (sess *Session) Open(dialAddr *net.UDPAddr) (err error) {
	sess.conn, err = net.DialUDP("udp", nil, dialAddr)

	if err != nil {
		return err
	}

	sess.startTime = time.Now()
	sess.lastPollTime = sess.startTime

	return
}

func (sess *Session) Poll() []byte {
	sess.lastPollTime = time.Now()

	buff := make([]byte, request.GetMaxResponseSize())

	sess.readLock.Lock()
	sess.conn.SetReadDeadline(time.Now().Add(time.Second))
	n, err := sess.conn.Read(buff)
	sess.readLock.Unlock()

	if err != nil {
		return []byte{request.RES_HEADER_CLOSED}
	}

	return append([]byte{request.RES_HEADER_POLL_OK}, buff[:n]...)
}

func (sess *Session) Write(data []byte) []byte {
	_, err := sess.conn.Write(data)

	if err != nil {
		log.Printf("write failed (sess %d): %v", sess.id, err)
		return []byte{request.RES_HEADER_CLOSED}
	}

	return []byte{request.RES_HEADER_WRITE_OK}
}
