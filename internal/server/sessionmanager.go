package server

import (
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

	req, err := request.UnmarshalMessage(msg, request.ReadRootSessionOpenRequest)
	if err != nil {
		return
	}

	dialAddr, err := net.ResolveUDPAddr("udp", req.DestAddr)
	if err != nil {
		return
	}

	log.Printf("Request to dial udp://%s", dialAddr)

	sess, err := createAndDialSession(dialAddr)
	if err != nil {
		log.Printf("Unable to dial %s: %v", req.DestAddr, err)
		err = nil
		response = request.MarshalMessage(request.RES_HEADER_DIAL_ERROR, nil)
		return
	}

	mgr.storeSession(sess)

	response = request.MarshalMessage(request.RES_HEADER_POLL_OK, request.SessionOpenResponse{
		ID: sess.id,
	})

	return
}

func (mgr *SessionManager) handlePoll(msg []byte) (response []byte, err error) {
	req, err := request.UnmarshalMessage[request.SessionPollRequest](msg)
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
	req, err := request.UnmarshalMessage[request.SessionWriteRequest](msg)
	if err != nil {
		return
	}

	log.Printf("Write request received for %d\n", req.ID)

	sess, ok := mgr.getSession(req.ID)

	if !ok {
		response = []byte{request.RES_HEADER_CLOSED}
		return
	}

	data, err := req.Data()

	if err != nil {
		return
	}

	response = sess.Write(data)
	return
}

func (mgr *SessionManager) handleMessage(msg []byte) (response []byte, err error) {
	headerByte := msg[0]
	data := msg[1:]

	log.Printf("%d and %s", headerByte, data)

	switch headerByte {
	case request.REQ_HEADER_SESSION_OPEN:
		response, err = mgr.handleOpen(data)
	case request.REQ_HEADER_SESSION_POLL:
		response, err = mgr.handlePoll(data)
	case request.REQ_HEADER_SESSION_WRITE:
		response, err = mgr.handleWrite(data)
		// case request.REQ_HEADER_SESSION_CLOSE:
		// 	response, err = mgr.handleClose(data)
	}

	return
}
