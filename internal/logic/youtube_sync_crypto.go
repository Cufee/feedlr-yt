package logic

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

const youtubeSyncCipherVersion byte = 1

type youtubeSyncCrypto struct {
	key        [32]byte
	secretHash string
}

func newYouTubeSyncCrypto(secret string) *youtubeSyncCrypto {
	return &youtubeSyncCrypto{
		key:        sha256.Sum256([]byte(secret)),
		secretHash: HashString(secret),
	}
}

func (c *youtubeSyncCrypto) Encrypt(plaintext []byte, aad string) ([]byte, error) {
	block, err := aes.NewCipher(c.key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	encrypted := gcm.Seal(nil, nonce, plaintext, []byte(aad))
	payload := make([]byte, 1+len(nonce)+len(encrypted))
	payload[0] = youtubeSyncCipherVersion
	copy(payload[1:], nonce)
	copy(payload[1+len(nonce):], encrypted)
	return payload, nil
}

func (c *youtubeSyncCrypto) Decrypt(payload []byte, aad string) ([]byte, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("ciphertext payload is empty")
	}
	if payload[0] != youtubeSyncCipherVersion {
		return nil, fmt.Errorf("unsupported ciphertext version")
	}

	block, err := aes.NewCipher(c.key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(payload) < 1+gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext payload is malformed")
	}

	nonce := payload[1 : 1+gcm.NonceSize()]
	ciphertext := payload[1+gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, []byte(aad))
}
