package request

import (
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
