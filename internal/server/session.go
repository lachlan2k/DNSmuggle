package server

import (
	"crypto/rand"
	"encoding/binary"
	"log"
	"math"
	"net"
	"time"

	"github.com/lachlan2k/dns-tunnel/internal/fragmentation"
	"github.com/lachlan2k/dns-tunnel/internal/request"
)

type Session struct {
	server       *Server
	startTime    time.Time
	lastPollTime time.Time
	id           request.SessionID
	conn         *net.UDPConn
	fragTable    fragmentation.FragmentationTable
	responseChan chan *request.PollResponse
	fragId       uint16
}

func createAndDialSession(dialAddr *net.UDPAddr, server *Server) (sess *Session, err error) {
	var idb [8]byte
	rand.Read(idb[:])

	sess = &Session{
		server:       server,
		id:           binary.BigEndian.Uint64(idb[:]),
		fragTable:    fragmentation.NewFragTable(),
		responseChan: make(chan *request.PollResponse),
	}

	err = sess.Open(dialAddr)

	if err != nil {
		return nil, err
	}

	go sess.feeder()

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

func (sess *Session) feeder() {
	buff := make([]byte, 65507)

	for {
		n, err := sess.conn.Read(buff)
		data := make([]byte, n)
		copy(data, buff[:n])

		if err != nil {
			continue
		}

		chunkSize := request.GetMaxResponseSize()
		chunkCount := int(math.Ceil(float64(n) / float64(chunkSize)))

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
				Data: data[start:end],
			}

			sess.responseChan <- res
		}
	}
}

func (sess *Session) Poll() []byte {
	sess.lastPollTime = time.Now()

	var res *request.PollResponse

	select {
	case <-time.After(100 * time.Millisecond):
		res = &request.PollResponse{
			Status: request.POLL_NO_DATA,
		}
	case res = <-sess.responseChan:
	}

	return res.Marshal()
}

func (sess *Session) Write(req request.WriteRequest) []byte {
	sess.lastPollTime = time.Now()
	// log.Printf("Ingesting fragment for session %d: %v", sess.id, req)

	completePacket, err := sess.fragTable.FeedFragment(req.FragmentationHeader, req.Data)
	if err != nil {
		log.Printf("Error feeding packet fragment: %v", err)
		res := request.WriteResponse{
			Status: request.POLL_ERROR,
		}
		return res.Marshal()
	}

	if completePacket != nil {
		// log.Printf("reconstructed complete packet: %v", completePacket)
		_, err := sess.conn.Write(completePacket)
		if err != nil {
			log.Printf("write failed (sess %d): %v", sess.id, err)
			res := request.WriteResponse{
				Status: request.POLL_ERROR,
			}
			return res.Marshal()
		}
	}

	if sess.server.config.PollOnWrite {
		return sess.Poll()
	}

	res := request.WriteResponse{
		Status: request.POLL_NO_DATA,
	}
	return res.Marshal()
}
