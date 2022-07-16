package request

import (
	"bytes"
	"errors"
	"strconv"
)

const (
	REQ_HEADER_SESSION_OPEN  = iota
	REQ_HEADER_SESSION_POLL  = iota
	REQ_HEADER_SESSION_WRITE = iota
	REQ_HEADER_SESSION_CLOSE = iota
)

const (
	RES_HEADER_WRITE_OK   = iota
	RES_HEADER_POLL_OK    = iota
	RES_HEADER_CLOSED     = iota
	RES_HEADER_DIAL_ERROR = iota
)

type SessionID = uint64

func marshalSessionID(id SessionID) []byte {
	return []byte(strconv.FormatUint(id, 10))
}

func unmarshalSessionID(msg []byte) (id SessionID, err error) {
	return strconv.ParseUint(string(msg), 10, 64)
}

type Message interface {
	Marshal() []byte
	Unmarshal([]byte) error
}

// Request to create a new session
type SessionOpenRequest struct {
	DestAddr string
}

func (r *SessionOpenRequest) Unmarshal(msg []byte) error {
	r.DestAddr = string(msg)
	return nil
}

func (r SessionOpenRequest) Marshal() []byte {
	var buff bytes.Buffer
	buff.WriteByte(REQ_HEADER_SESSION_OPEN)
	buff.WriteString(r.DestAddr)
	return buff.Bytes()
}

type SessionOpenResponse struct {
	ID SessionID
}

func (r *SessionOpenResponse) Unmarshal(msg []byte) (err error) {
	r.ID, err = unmarshalSessionID(msg)
	return
}

func (r SessionOpenResponse) Marshal() []byte {
	return marshalSessionID(r.ID)
}

// Request to ask for new data
type PollRequest struct {
	ID SessionID
}

func (r *PollRequest) Unmarshal(msg []byte) (err error) {
	r.ID, err = unmarshalSessionID(msg)
	return
}

func (r PollRequest) Marshal() []byte {
	return marshalSessionID(r.ID)
}

// Request to kill session
type SessionCloseRequest struct {
	ID SessionID
}

func (r *SessionCloseRequest) Unmarshal(msg []byte) (err error) {
	r.ID, err = unmarshalSessionID(msg)
	return
}

func (r SessionCloseRequest) Marshal() []byte {
	return marshalSessionID(r.ID)
}

// Request to write data
type WriteRequest struct {
	ID   SessionID
	Data []byte
}

func (r *WriteRequest) Unmarshal(msg []byte) (err error) {
	if len(msg) < 8 {
		return errors.New("Session write request too small")
	}

	r.ID, err = unmarshalSessionID(msg[:8])
	r.Data = msg[8:]
	return
}

func (r WriteRequest) Marshal() []byte {
	var buff bytes.Buffer
	buff.Write(marshalSessionID(r.ID))
	buff.Write(r.Data)
	return buff.Bytes()
}
