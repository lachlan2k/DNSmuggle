package request

import (
	"capnproto.org/go/capnp/v3"
	"github.com/miekg/dns"
)

func GetCtrlFQDN(domain string) string {
	return dns.Fqdn("c." + domain)
}

// TODO:
func GetMaxRequestSize(domain string) int {
	return 90
}

func GetMaxResponseSize() int {
	return 100
}

func CreateMessage[T any](createFunc func(s *capnp.Segment) (T, error)) (out T, msg *capnp.Message, err error) {
	msg, s, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return
	}

	out, err = createFunc(s)
	if err != nil {
		return
	}

	return
}

// func MarshalMessage[T any]() (data []byte, err error) {
// 	msg, s, err := capnp.NewMessage(capnp.SingleSegment(nil))
// 	if err != nil {
// 		return
// 	}

// 	data, err = msg.MarshalPacked()
// 	return
// }

func UnmarshalMessage[T any](data []byte, readFunc func(msg *capnp.Message) (T, error)) (out T, err error) {
	msg, err := capnp.UnmarshalPacked(data)
	if err != nil {
		return
	}

	out, err = readFunc(msg)
	return
}

// func MarshalMessage() {
// msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
// }

// func UnmarshalMessage[T any](msg []byte) (out T, err error) {
// 	r := bytes.NewReader(msg)
// 	decoded, err := capnp.NewDecoder(r).Decode()

// 	if err != nil {
// 		return
// 	}

// }

// Need this per https://go.googlesource.com/proposal/+/refs/heads/master/design/43651-type-parameters.md#pointer-method-example
// type Marshalable[T any] interface {
// 	proto.Message
// 	*T
// }

// func MarshalMessage[T any, PT Marshalable[T]](msg T) []byte {
// 	data, err := proto.Marshal(PT(&msg))

// 	if err != nil {
// 		log.Printf("!!!! Error marshalling message: %v", err)
// 	}

// 	return data
// }

// func MarshalMessageWithHeader[T any, PT Marshalable[T]](header byte, msg T) []byte {
// 	var buff bytes.Buffer
// 	buff.WriteByte(header)

// 	data, err := proto.Marshal(PT(&msg))

// 	if err != nil {
// 		log.Printf("!!!! Error marshalling message: %v", err)
// 	} else {
// 		buff.Write(data)
// 	}

// 	return buff.Bytes()
// }

// func UnmarshalMessage[T any, PT Marshalable[T]](msg []byte) (out T, err error) {
// 	err = proto.Unmarshal(msg, PT(&out))
// 	return
// }
