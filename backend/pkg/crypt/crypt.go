package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	// KeySize is the required AES-256 key length in bytes.
	KeySize = 32
	// nonceSize is the GCM nonce length.
	nonceSize = 12
	// wirePrefix marks stored ciphertext (version 1).
	wirePrefix = "APENC1:"
)

var (
	ErrEmptyCiphertext   = errors.New("crypt: ciphertext is empty")
	ErrInvalidKey        = errors.New("crypt: key must be 32 bytes or a non-empty passphrase")
	ErrInvalidCiphertext = errors.New("crypt: invalid ciphertext")
	ErrNotInitialized    = errors.New("crypt: package not initialized")
)

// Cipher encrypts and decrypts values with AES-256-GCM.
type Cipher struct {
	aead cipher.AEAD
}

// New creates a Cipher from a 32-byte key.
func New(key []byte) (*Cipher, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("%w (got %d bytes)", ErrInvalidKey, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// NewFromPassphrase derives a 32-byte key with SHA-256.
func NewFromPassphrase(passphrase string) (*Cipher, error) {
	passphrase = strings.TrimSpace(passphrase)
	if passphrase == "" {
		return nil, ErrInvalidKey
	}
	return New(DeriveKey(passphrase))
}

// DeriveKey hashes passphrase into a 32-byte AES key.
func DeriveKey(passphrase string) []byte {
	sum := sha256.Sum256([]byte(passphrase))
	return sum[:]
}

// Encrypt seals plaintext and returns a prefixed base64 string for storage.
// Empty plaintext is stored as an empty string without encryption.
func (c *Cipher) Encrypt(plaintext []byte) (string, error) {
	if len(plaintext) == 0 {
		return "", nil
	}
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nil, nonce, plaintext, nil)
	payload := append(nonce, sealed...)
	return wirePrefix + base64.StdEncoding.EncodeToString(payload), nil
}

// EncryptString is a convenience wrapper for string plaintext.
func (c *Cipher) EncryptString(plaintext string) (string, error) {
	return c.Encrypt([]byte(plaintext))
}

// Decrypt opens a value produced by Encrypt. Non-prefixed input is returned as-is
// so callers can read legacy plaintext during migration.
func (c *Cipher) Decrypt(encoded string) ([]byte, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, nil
	}
	if !IsCiphertext(encoded) {
		return []byte(encoded), nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, wirePrefix))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCiphertext, err)
	}
	if len(raw) < nonceSize {
		return nil, ErrInvalidCiphertext
	}
	nonce, sealed := raw[:nonceSize], raw[nonceSize:]
	plain, err := c.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCiphertext, err)
	}
	return plain, nil
}

// DecryptString is a convenience wrapper for string storage.
func (c *Cipher) DecryptString(encoded string) (string, error) {
	b, err := c.Decrypt(encoded)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// IsCiphertext reports whether s looks like a value from Encrypt.
func IsCiphertext(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), wirePrefix)
}
