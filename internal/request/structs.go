package request

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log"
)

const (
	REQ_HEADER_CTRL = iota // encrypted message
	REQ_HEADER_DATA = iota // normal data flow
)

const (
	CTRL_HEADER_SESSION_OPEN  = iota
	CTRL_HEADER_SESSION_POLL  = iota
	CTRL_HEADER_SESSION_WRITE = iota
)

const (
	RES_HEADER_WRITE_OK   = iota
	RES_HEADER_POLL_OK    = iota
	RES_HEADER_CLOSED     = iota
	RES_HEADER_DIAL_ERROR = iota
)

type SessionID = uint64

func fixedSizeMarshal(m any) []byte {
	var buff bytes.Buffer
	err := binary.Write(&buff, binary.BigEndian, m)
	if err != nil {
		log.Printf("Marshal error: %v", err)
	}
	return buff.Bytes()
}

func fixedSizeUnmarshal[T any](msg []byte) (out T, err error) {
	r := bytes.NewReader(msg)
	err = binary.Read(r, binary.BigEndian, &out)
	return
}

// Request to create a new session
type SessionOpenRequest struct {
	DestAddr string
}

func UnmarshalSessionOpenRequest(msg []byte) SessionOpenRequest {
	return SessionOpenRequest{
		DestAddr: string(msg),
	}
}

func (r SessionOpenRequest) Marshal() []byte {
	var buff bytes.Buffer
	buff.WriteByte(CTRL_HEADER_SESSION_OPEN)
	buff.WriteString(r.DestAddr)
	return buff.Bytes()
}

const (
	SESSION_OPEN_OK        = iota
	SESSION_OPEN_DIAL_FAIL = iota
	SESSION_OPEN_ERROR     = iota
)

type SessionOpenResponse struct {
	Status uint8
	ID     SessionID
}

func UnmarshalSessionOpenResponse(msg []byte) (SessionOpenResponse, error) {
	return fixedSizeUnmarshal[SessionOpenResponse](msg)
}

func (r SessionOpenResponse) Marshal() []byte {
	var buff bytes.Buffer
	buff.WriteByte(CTRL_HEADER_SESSION_OPEN)
	buff.Write(fixedSizeMarshal(r))
	return buff.Bytes()
}

// Request to ask for new data
type PollRequest struct {
	ID SessionID
}

func UnmarshalPollRequest(msg []byte) (PollRequest, error) {
	return fixedSizeUnmarshal[PollRequest](msg)
}

func (r PollRequest) Marshal() []byte {
	var buff bytes.Buffer
	buff.WriteByte(CTRL_HEADER_SESSION_POLL)
	buff.Write(fixedSizeMarshal(r))
	return buff.Bytes()
}

// Header for fragmentation
const (
	MAX_FRAG_INDEX = 0b1111
	MAX_FRAG_ID    = 0b11111111111
)

type FragmentationHeader struct {
	ID              uint16
	Index           uint8
	IsFinalFragment bool
}

func UnmarshalFragmentationHeader(header uint16) FragmentationHeader {
	return FragmentationHeader{
		ID:              header >> 9,
		Index:           uint8((header >> 5) & MAX_FRAG_INDEX),
		IsFinalFragment: (header & 1) == 1,
	}
}

func (s FragmentationHeader) Marshal() uint16 {
	final := uint16(0)
	if s.IsFinalFragment {
		final = 1
	}

	index := uint16(s.Index & MAX_FRAG_INDEX)
	id := s.ID & MAX_FRAG_ID

	return index<<5 | id<<9 | final
}

// Request to write data
type WriteRequest struct {
	ID                  SessionID
	FragmentationHeader FragmentationHeader
	Data                []byte
}

func UnmarshalWriteRequest(msg []byte) (req WriteRequest, err error) {
	if len(msg) < 10 {
		err = errors.New("session write request too small")
		return
	}

	// log.Printf("Unarshalling id %d as %s", binary.BigEndian.Uint64(msg[:8]), hex.EncodeToString(msg))

	req = WriteRequest{
		ID:                  binary.BigEndian.Uint64(msg[:8]),
		FragmentationHeader: UnmarshalFragmentationHeader(binary.BigEndian.Uint16(msg[8:10])),
		Data:                msg[10:],
	}
	return
}

func (r WriteRequest) Marshal() []byte {
	buff := make([]byte, 8+2+len(r.Data))
	// todo: rejig how i do some things
	// so that this isn't a ctrl header session
	binary.BigEndian.PutUint64(buff[0:8], r.ID)
	binary.BigEndian.PutUint16(buff[8:10], r.FragmentationHeader.Marshal())
	copy(buff[10:], r.Data)

	return buff
}
