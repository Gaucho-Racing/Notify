package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

const keySize = 32

var ErrNotConfigured = errors.New("ENCRYPTION_KEY is not configured")

type Box struct {
	aead cipher.AEAD
}

// New builds an AES-256-GCM box from a base64-encoded 32-byte key.
func New(encodedKey string) (*Box, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("key must be %d bytes, got %d", keySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Seal binds the ciphertext to associatedData, so a value copied onto a
// different row fails to open instead of decrypting under the wrong identity.
func (b *Box) Seal(plaintext string, associatedData string) ([]byte, error) {
	if b == nil {
		return nil, ErrNotConfigured
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return b.aead.Seal(nonce, nonce, []byte(plaintext), []byte(associatedData)), nil
}

func (b *Box) Open(ciphertext []byte, associatedData string) (string, error) {
	if b == nil {
		return "", ErrNotConfigured
	}
	nonceSize := b.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext is too short")
	}
	plaintext, err := b.aead.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], []byte(associatedData))
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}
