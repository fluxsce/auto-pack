package crypt

import (
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	c, err := NewFromPassphrase("test-master-key-for-unit-tests")
	if err != nil {
		t.Fatal(err)
	}
	plain := "ssh-secret-password"
	enc, err := c.EncryptString(plain)
	if err != nil {
		t.Fatal(err)
	}
	if !IsCiphertext(enc) {
		t.Fatalf("expected ciphertext prefix, got %q", enc)
	}
	if strings.Contains(enc, plain) {
		t.Fatal("ciphertext must not contain plaintext")
	}
	got, err := c.DecryptString(enc)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("got %q want %q", got, plain)
	}
}

func TestDecryptPassthrough(t *testing.T) {
	c, err := NewFromPassphrase("key")
	if err != nil {
		t.Fatal(err)
	}
	legacy := "plain-stored-value"
	got, err := c.DecryptString(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if got != legacy {
		t.Fatalf("got %q want %q", got, legacy)
	}
}

func TestWrongKeyFails(t *testing.T) {
	a, _ := NewFromPassphrase("key-a")
	b, _ := NewFromPassphrase("key-b")
	enc, err := a.EncryptString("secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.DecryptString(enc); err == nil {
		t.Fatal("expected decrypt failure with wrong key")
	}
}

func TestTamperFails(t *testing.T) {
	c, _ := NewFromPassphrase("key")
	enc, _ := c.EncryptString("secret")
	tampered := enc[:len(enc)-2] + "XX"
	if _, err := c.DecryptString(tampered); err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestGlobalHelpers(t *testing.T) {
	if err := InitFromPassphrase("global-test-key"); err != nil {
		t.Fatal(err)
	}
	enc, err := EncryptString("x")
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecryptString(enc)
	if err != nil || got != "x" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestEmptyInputs(t *testing.T) {
	c, _ := NewFromPassphrase("key")
	enc, err := c.Encrypt(nil)
	if err != nil || enc != "" {
		t.Fatalf("encrypt empty: enc=%q err=%v", enc, err)
	}
	got, err := c.Decrypt("")
	if err != nil || len(got) != 0 {
		t.Fatalf("decrypt empty: got %q err=%v", got, err)
	}
}

func TestNewInvalidKey(t *testing.T) {
	if _, err := New([]byte("short")); err == nil {
		t.Fatal("expected invalid key error")
	}
}
