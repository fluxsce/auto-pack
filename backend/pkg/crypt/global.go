package crypt

import (
	"sync"
)

var (
	globalMu sync.RWMutex
	global   *Cipher
)

// Init sets the process-wide cipher used by package-level helpers.
func Init(key []byte) error {
	c, err := New(key)
	if err != nil {
		return err
	}
	globalMu.Lock()
	global = c
	globalMu.Unlock()
	return nil
}

// InitFromPassphrase derives a key and calls Init.
func InitFromPassphrase(passphrase string) error {
	c, err := NewFromPassphrase(passphrase)
	if err != nil {
		return err
	}
	globalMu.Lock()
	global = c
	globalMu.Unlock()
	return nil
}

// Default returns the initialized global cipher.
func Default() (*Cipher, error) {
	globalMu.RLock()
	c := global
	globalMu.RUnlock()
	if c == nil {
		return nil, ErrNotInitialized
	}
	return c, nil
}

// Encrypt uses the global cipher.
func Encrypt(plaintext []byte) (string, error) {
	c, err := Default()
	if err != nil {
		return "", err
	}
	return c.Encrypt(plaintext)
}

// EncryptString uses the global cipher.
func EncryptString(plaintext string) (string, error) {
	c, err := Default()
	if err != nil {
		return "", err
	}
	return c.EncryptString(plaintext)
}

// Decrypt uses the global cipher.
func Decrypt(encoded string) ([]byte, error) {
	c, err := Default()
	if err != nil {
		return nil, err
	}
	return c.Decrypt(encoded)
}

// DecryptString uses the global cipher.
func DecryptString(encoded string) (string, error) {
	c, err := Default()
	if err != nil {
		return "", err
	}
	return c.DecryptString(encoded)
}
