package secrets

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
)

func randKeyB64(t *testing.T) string {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(k)
}

func TestSealOpenRoundtrip(t *testing.T) {
	s, err := NewSealer(randKeyB64(t))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := s.Seal("glpat-my-token")
	if err != nil {
		t.Fatal(err)
	}
	if sealed.KeyVersion != currentKeyVersion {
		t.Errorf("keyVersion = %d, want %d", sealed.KeyVersion, currentKeyVersion)
	}
	if len(sealed.Nonce) == 0 || len(sealed.Ciphertext) == 0 {
		t.Fatal("empty nonce/ciphertext")
	}
	if string(sealed.Ciphertext) == "glpat-my-token" {
		t.Fatal("ciphertext must not equal plaintext")
	}
	got, err := s.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if got != "glpat-my-token" {
		t.Errorf("Open = %q, want %q", got, "glpat-my-token")
	}
}

func TestSealUsesAFreshNonceEachTime(t *testing.T) {
	s, _ := NewSealer(randKeyB64(t))
	a, _ := s.Seal("same")
	b, _ := s.Seal("same")
	if string(a.Nonce) == string(b.Nonce) {
		t.Fatal("two seals reused a nonce — a nonce must never repeat under one key")
	}
	if string(a.Ciphertext) == string(b.Ciphertext) {
		t.Fatal("two seals of the same plaintext produced identical ciphertext")
	}
}

func TestOpenWithWrongKeyFails(t *testing.T) {
	s1, _ := NewSealer(randKeyB64(t))
	s2, _ := NewSealer(randKeyB64(t))
	sealed, err := s1.Seal("secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s2.Open(sealed); err == nil {
		t.Fatal("expected decryption failure with the wrong key")
	}
}

func TestNoKeyFailsClosed(t *testing.T) {
	s, err := NewSealer("")
	if err != nil {
		t.Fatalf("empty key should not error: %v", err)
	}
	if s != nil {
		t.Fatal("expected a nil Sealer for an empty key")
	}
	if _, err := s.Seal("x"); !errors.Is(err, ErrNoKey) {
		t.Errorf("Seal err = %v, want ErrNoKey", err)
	}
	if _, err := s.Open(SealedValue{}); !errors.Is(err, ErrNoKey) {
		t.Errorf("Open err = %v, want ErrNoKey", err)
	}
}

func TestBadKeyRejected(t *testing.T) {
	if _, err := NewSealer("not valid base64 %%%"); err == nil {
		t.Error("expected a base64 error")
	}
	if _, err := NewSealer(base64.StdEncoding.EncodeToString(make([]byte, 16))); err == nil {
		t.Error("expected a length error for a 16-byte key")
	}
}
