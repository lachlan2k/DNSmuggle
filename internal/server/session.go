package server

import (
	"crypto/rand"
	"encoding/binary"
	"log"
	"math"
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
	pollChan     chan *request.PollResponse
	fragId       uint16
}

func createAndDialSession(dialAddr *net.UDPAddr) (sess *Session, err error) {
	var idb [8]byte
	rand.Read(idb[:])

	sess = &Session{
		id:        binary.BigEndian.Uint64(idb[:]),
		fragTable: fragmentation.NewFragTable(),
		pollChan:  make(chan *request.PollResponse, 8000),
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

func (sess *Session) pollAndChunk() (firstResponse *request.PollResponse, err error) {
	sess.readLock.Lock()
	defer sess.readLock.Unlock()

	buff := make([]byte, 65535)

	sess.conn.SetReadDeadline(time.Now().Add(time.Second))
	n, err := sess.conn.Read(buff)
	if err != nil {
		return
	}

	chunkSize := request.GetMaxResponseSize()
	chunkCount := int(math.Ceil(float64(n) / float64(chunkSize)))

	log.Printf("Chunking into %d chunks", chunkCount)

	id := sess.fragId
	sess.fragId++

	for i := 0; i < chunkCount; i++ {
		start := chunkSize * i
		end := chunkSize * (i + 1)
		if end > n {
			end = n
		}

		if sess.fragId > request.MAX_FRAG_ID {
			sess.fragId = 0
		}

		res := &request.PollResponse{
			Status: request.POLL_OK,
			FragmentationHeader: request.FragmentationHeader{
				ID:              id,
				Index:           uint8(i),
				IsFinalFragment: i == (chunkCount - 1),
			},
			Data: buff[start:end],
		}

		if i == 0 {
			firstResponse = res
		} else {
			go func() {
				sess.pollChan <- res
			}()
		}
	}

	return
}

func (sess *Session) Poll() []byte {
	sess.lastPollTime = time.Now()

	var res *request.PollResponse
	var err error

	select {
	case res = <-sess.pollChan:
	default:
		res, err = sess.pollAndChunk()
	}

	if err != nil {
		// todo: handle timeout errros
		// handle situations where the connection has closed
		// handle general error
		res = &request.PollResponse{
			Status: request.POLL_NO_DATA,
		}
	}

	log.Printf("sending poll response (%d bytes): %v", len(res.Marshal()), *res)

	return res.Marshal()
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
