package request

const (
	REQ_HEADER_SESSION_OPEN  = iota
	REQ_HEADER_SESSION_POLL  = iota
	REQ_HEADER_SESSION_WRITE = iota
	REQ_HEADER_SESSION_CLOSE = iota
)

// const (
// 	RES_HEADER_WRITE_OK   = iota
// 	RES_HEADER_POLL_OK    = iota
// 	RES_HEADER_CLOSED     = iota
// 	RES_HEADER_DIAL_ERROR = iota
// )

type SessionID = uint64

// type SessionOpenRequest struct {
// 	DestAddr string
// }

// type SessionOpenResponse struct {
// 	ID SessionID
// }

// type PollRequest struct {
// 	ID SessionID
// }

// type SessionCloseRequest struct {
// 	ID SessionID
// }

// type WriteRequest struct {
// 	ID   SessionID
// 	Data []byte
// }
