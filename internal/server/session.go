package server

import (
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/lachlan2k/dns-tunnel/internal/fragmentation"
	"github.com/lachlan2k/dns-tunnel/internal/request"
)

type Session struct {
	startTime    time.Time
	lastPollTime time.Time
	id           request.SessionID
	conn         *net.UDPConn
	readLock     sync.Mutex
	fragTable    fragmentation.FragmentationTable
}

func createAndDialSession(dialAddr *net.UDPAddr) (sess *Session, err error) {
	sess = &Session{
		id:        rand.Uint64(),
		fragTable: fragmentation.NewFragTable(),
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

func (sess *Session) Write(req request.WriteRequest) []byte {
	log.Printf("Ingesting fragment for session %d: %v", sess.id, req)

	completePacket, err := sess.fragTable.FeedFragment(req.FragmentationHeader, req.Data)
	if err != nil {
		log.Printf("Error feeding packet fragment: %v", err)
		return []byte{request.RES_HEADER_WRITE_OK}
	}

	if completePacket != nil {
		log.Printf("reconstructed complete packet: %v", completePacket)
		_, err := sess.conn.Write(completePacket)
		if err != nil {
			log.Printf("write failed (sess %d): %v", sess.id, err)
			return []byte{request.RES_HEADER_CLOSED}
		}
	}

	return []byte{request.RES_HEADER_WRITE_OK}
}
