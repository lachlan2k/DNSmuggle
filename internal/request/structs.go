package request

const (
	REQ_HEADER_SESSION_OPEN  = iota
	REQ_HEADER_SESSION_POLL  = iota
	REQ_HEADER_SESSION_WRITE = iota
	REQ_HEADER_SESSION_CLOSE = iota
)

type SessionID = uint64

const (
	MAX_FRAG_INDEX = 0b1111
	MAX_FRAG_ID    = 0b11111111111
)

type ParsedFragmentationHeader struct {
	ID              uint16
	Index           uint8
	IsFinalFragment bool
}

func (s *ParsedFragmentationHeader) Pack() uint16 {
	final := uint16(0)
	if s.IsFinalFragment {
		final = 1
	}

	index := uint16(s.Index & MAX_FRAG_INDEX)
	id := s.ID & MAX_FRAG_ID

	return index<<5 | id<<9 | final
}

func ParseFragmentationHeader(header uint16) (out ParsedFragmentationHeader) {
	out.ID = header >> 9
	out.Index = uint8((header >> 5) & MAX_FRAG_INDEX)
	out.IsFinalFragment = (header & 1) == 1
	return
}
