package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	defaultIter   = 100000
	saltSize      = 16
	derivedKeyLen = 32 // AES-256
)

// EncryptString derives a key from passphrase and returns base64(salt|nonce|ciphertext).
func EncryptString(plain, passphrase string) (string, error) {
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("crypt: salt: %w", err)
	}
	key := pbkdf2.Key([]byte(passphrase), salt, defaultIter, derivedKeyLen, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypt: new cipher: %w", err)
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypt: new gcm: %w", err)
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypt: nonce: %w", err)
	}
	ct := g.Seal(nil, nonce, []byte(plain), nil)
	out := make([]byte, 0, len(salt)+len(nonce)+len(ct))
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ct...)
	return base64.StdEncoding.EncodeToString(out), nil
}

// DecryptString expects base64(salt|nonce|ciphertext) and returns plaintext.
func DecryptString(encoded, passphrase string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("crypt: decode: %w", err)
	}
	if len(data) < saltSize {
		return "", fmt.Errorf("crypt: data too short")
	}
	salt := data[:saltSize]
	// derive key
	key := pbkdf2.Key([]byte(passphrase), salt, defaultIter, derivedKeyLen, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypt: new cipher: %w", err)
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypt: new gcm: %w", err)
	}
	nonceSize := g.NonceSize()
	if len(data) < saltSize+nonceSize {
		return "", fmt.Errorf("crypt: data too short for nonce")
	}
	nonce := data[saltSize : saltSize+nonceSize]
	ct := data[saltSize+nonceSize:]
	pt, err := g.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("crypt: open: %w", err)
	}
	return string(pt), nil
}
