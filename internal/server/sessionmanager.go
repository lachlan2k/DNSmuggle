package server

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/lachlan2k/dns-tunnel/internal/request"
)

const AllowedClockSkew = 5 * time.Minute

type SessionManager struct {
	store  map[request.SessionID](*Session)
	server *Server
	// The seen request map is to prevent replay-attacks, or issues when a resolver sends a request twice.
	// We keep track of all session open requests' sha256sums, and look for something we've seen before, and drop it if we have.
	// The janitor clears out any that run out of time
	seenRequestMap map[[sha256.Size]byte](time.Time)
	requestMapLock sync.Mutex
}

func (mgr *SessionManager) getSession(id request.SessionID) (sess *Session, ok bool) {
	sess, ok = mgr.store[id]
	return
}

func (mgr *SessionManager) storeSession(sess *Session) {
	mgr.store[sess.id] = sess
}

func (mgr *SessionManager) janitor() {
	for {
		mgr.requestMapLock.Lock()

		now := time.Now()
		for checksum, seenTime := range mgr.seenRequestMap {
			if now.After(seenTime.Add(AllowedClockSkew)) {
				delete(mgr.seenRequestMap, checksum)
			}
		}

		mgr.requestMapLock.Unlock()
		time.Sleep(time.Minute)
	}
}

func (mgr *SessionManager) checkReplay(timestamp time.Time, msg []byte) error {
	mgr.requestMapLock.Lock()
	defer mgr.requestMapLock.Unlock()

	now := time.Now()
	if timestamp.After(now.Add(AllowedClockSkew)) || timestamp.Before(now.Add(-AllowedClockSkew)) {
		return errors.New("session open request clock skew error -- possible replay attack, or incorrect time on server/client")
	}

	checksum := sha256.Sum256(msg)
	_, found := mgr.seenRequestMap[checksum]

	if found {
		return errors.New("request open collision detected (possible replay attack, or resolver being funky)")
	}

	mgr.seenRequestMap[checksum] = timestamp

	return nil
}

func (mgr *SessionManager) handleOpen(msg []byte) (response []byte, err error) {
	log.Printf("Received new session request\n")

	req := request.UnmarshalSessionOpenRequest(msg)

	dialAddr, err := net.ResolveUDPAddr("udp", req.DestAddr)
	if err != nil {
		return
	}

	log.Printf("Request to dial udp://%s", dialAddr)

	sess, err := createAndDialSession(dialAddr, mgr.server)
	if err != nil {
		log.Printf("Unable to dial %s: %v", req.DestAddr, err)
		err = nil
		response = request.SessionOpenResponse{
			Status: request.SESSION_OPEN_DIAL_FAIL,
		}.Marshal()
		return
	} else {
		log.Printf("Opened session %d to %s", sess.id, dialAddr)
	}

	mgr.storeSession(sess)

	response = request.SessionOpenResponse{
		Status: request.SESSION_OPEN_OK,
		ID:     sess.id,
	}.Marshal()

	return
}

func (mgr *SessionManager) handlePoll(msg []byte) (response []byte, err error) {
	req, err := request.UnmarshalPollRequest(msg)
	if err != nil {
		return
	}

	sess, ok := mgr.getSession(req.ID)

	if !ok {
		response = request.PollResponse{
			Status: request.RES_HEADER_CLOSED,
		}.Marshal()
		return
	}

	response = sess.Poll()
	return
}

func (mgr *SessionManager) handleWrite(msg []byte) (response []byte, err error) {
	req, err := request.UnmarshalWriteRequest(msg)
	if err != nil {
		return
	}

	sess, ok := mgr.getSession(req.ID)
	if !ok {
		response = request.PollResponse{
			Status: request.RES_HEADER_CLOSED,
		}.Marshal()
		return
	}

	response = sess.Write(req)
	return
}

func (mgr *SessionManager) handleControlMessage(msg []byte, nonce []byte) (response []byte, err error) {
	if len(msg) < 10 {
		err = errors.New("received unusually small control channel message")
		return
	}

	timestampBytes := msg[0:8]
	timestamp := binary.BigEndian.Uint64(timestampBytes)
	headerByte := msg[8]
	data := msg[9:]

	dataToCheck := make([]byte, len(nonce)+len(msg))
	copy(dataToCheck[:len(nonce)], nonce)
	copy(dataToCheck[len(nonce):], msg)

	err = mgr.checkReplay(time.UnixMilli(int64(timestamp)), dataToCheck)

	if err != nil {
		return
	}

	switch headerByte {
	case request.CTRL_HEADER_SESSION_OPEN:
		response, err = mgr.handleOpen(data)
	case request.CTRL_HEADER_SESSION_POLL:
		response, err = mgr.handlePoll(data)
	default:
		err = errors.New("unrecognized header byte")
	}

	return
}

func (mgr *SessionManager) handleDataMessage(msg []byte) (response []byte, err error) {
	response, err = mgr.handleWrite(msg)
	return
}
