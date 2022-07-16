package request

import (
	"encoding/base32"
	"encoding/base64"
	"strings"
)

var b32Encoding = base32.HexEncoding.WithPadding(base32.NoPadding)

func EncodeRequest(msg []byte) string {
	encodedMsg := b32Encoding.EncodeToString(msg)

	// Subdomains can only be 63 chars long
	for i := 63; i < len(encodedMsg); i += 63 {
		encodedMsg = encodedMsg[:i] + "." + encodedMsg[i:]
	}

	return encodedMsg
}

func DecodeRequest(encodedMsg string) ([]byte, error) {
	// Remove periods from subdomains (per encoding above), and uppercase it all
	upperWithoutPeriods := strings.ToUpper(strings.Replace(encodedMsg, ".", "", -1))
	return b32Encoding.DecodeString(upperWithoutPeriods)
}

func EncodeResponse(msg []byte) string {
	return base64.RawURLEncoding.EncodeToString(msg)
}

func DecodeResponse(encodedMsg string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(encodedMsg)
}
