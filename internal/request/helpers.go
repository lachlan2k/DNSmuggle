package request

import (
	"bytes"
	"encoding/gob"

	"github.com/miekg/dns"
)

func GetCtrlFQDN(domain string) string {
	return dns.Fqdn("c." + domain)
}

// TODO:
func GetMaxRequestSize(domain string) int {
	return 100
}

func GetMaxResponseSize() int {
	return 100
}

func MarshalMessage(header byte, packet any) []byte {
	var buff bytes.Buffer
	buff.WriteByte(header)
	encoder := gob.NewEncoder(&buff)
	encoder.Encode(packet)
	return buff.Bytes()
}

func UnmarshalMessage[T any](msg []byte) (out T, err error) {
	msgReader := bytes.NewReader(msg)
	decoder := gob.NewDecoder(msgReader)
	err = decoder.Decode(&out)
	return
}

// func MarshalMessage(header byte, packet any) []byte {
// 	var buff bytes.Buffer
// 	buff.WriteByte(header)
// 	err := binary.Write(&buff, binary.LittleEndian, packet)
// 	if err != nil {
// 		log.Printf("Error writing message: %v", err)
// 	} else {
// 		log.Printf("Marshalled message (%d bytes) %s", buff.Len(), buff.Bytes())
// 	}

// 	return buff.Bytes()
// }

// func UnmarshalMessage[T any](msg []byte) (out T, err error) {
// 	r := bytes.NewBuffer(msg)
// 	err = binary.Read(r, binary.LittleEndian, &out)
// 	return
// }
