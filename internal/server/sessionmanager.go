package server

import (
	"errors"
	"log"
	"net"

	"github.com/lachlan2k/dns-tunnel/internal/request"
)

type SessionManager struct {
	store map[request.SessionID](*Session)
}

func (mgr *SessionManager) getSession(id request.SessionID) (sess *Session, ok bool) {
	sess, ok = mgr.store[id]
	return
}

func (mgr *SessionManager) storeSession(sess *Session) {
	mgr.store[sess.id] = sess
}

func (mgr *SessionManager) handleOpen(msg []byte) (response []byte, err error) {
	log.Printf("Received new session request\n")

	req := request.UnmarshalSessionOpenRequest(msg)
	dialAddr, err := net.ResolveUDPAddr("udp", req.DestAddr)
	if err != nil {
		return
	}

	log.Printf("Request to dial udp://%s", dialAddr)

	sess, err := createAndDialSession(dialAddr)
	if err != nil {
		log.Printf("Unable to dial %s: %v", req.DestAddr, err)
		err = nil
		reply := request.SessionOpenResponse{
			Status: request.SESSION_OPEN_DIAL_FAIL,
		}
		response = reply.Marshal()
		return
	} else {
		log.Printf("Opened session %d to %s", sess.id, dialAddr)
	}

	mgr.storeSession(sess)

	reply := request.SessionOpenResponse{
		Status: request.SESSION_OPEN_OK,
		ID:     sess.id,
	}
	response = reply.Marshal()

	return
}

func (mgr *SessionManager) handlePoll(msg []byte) (response []byte, err error) {
	req, err := request.UnmarshalPollRequest(msg)
	if err != nil {
		return
	}

	sess, ok := mgr.getSession(req.ID)

	if !ok {
		response = []byte{request.RES_HEADER_CLOSED}
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

	log.Printf("Write request received for %d: %s\n", req.ID, req.Data)

	sess, ok := mgr.getSession(req.ID)

	if !ok {
		response = []byte{request.RES_HEADER_CLOSED}
		return
	}

	response = sess.Write(req)
	return
}

func (mgr *SessionManager) handleControlMessage(msg []byte) (response []byte, err error) {
	headerByte := msg[0]
	data := msg[1:]

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
