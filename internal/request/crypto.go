package request

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"

	"golang.org/x/crypto/chacha20poly1305"
)

func EncryptMessage(msg []byte, psk string) (encryptedMsg []byte, err error) {
	key := sha256.Sum256([]byte(psk))
	aead, err := chacha20poly1305.NewX(key[:])

	if err != nil {
		return
	}

	nonce := make([]byte, aead.NonceSize(), aead.NonceSize()+len(msg)+aead.Overhead())
	_, err = rand.Read(nonce)

	if err != nil {
		return
	}

	encryptedMsg = aead.Seal(nonce, nonce, msg, nil)
	return
}

func DecryptMessage(encryptedMsg []byte, psk string) (msg []byte, err error) {
	key := sha256.Sum256([]byte(psk))
	aead, err := chacha20poly1305.NewX(key[:])

	if err != nil {
		return
	}

	if len(encryptedMsg) < aead.NonceSize() {
		err = errors.New("encrypted message was too short (< nonce size)")
		return
	}

	nonce, ciphertext := encryptedMsg[:aead.NonceSize()], encryptedMsg[aead.NonceSize():]

	return aead.Open(nil, nonce, ciphertext, nil)
}
