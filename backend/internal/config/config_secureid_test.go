package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"testing"
)

func TestLoadSecureIDKey_Base64Valid(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SECURE_ID_KEY", base64.StdEncoding.EncodeToString(key))
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.SecureIDKey) != 32 {
		t.Fatalf("len=%d want 32", len(cfg.SecureIDKey))
	}
}

func TestLoadSecureIDKey_HexValid(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SECURE_ID_KEY", hex.EncodeToString(key))
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.SecureIDKey) != 32 {
		t.Fatalf("len=%d want 32", len(cfg.SecureIDKey))
	}
}

func TestLoadSecureIDKey_Missing(t *testing.T) {
	t.Setenv("SECURE_ID_KEY", "")
	_, err := Load()
	if !errors.Is(err, ErrInvalidSecureIDKey) {
		t.Fatalf("Load() error = %v, want ErrInvalidSecureIDKey", err)
	}
}

func TestLoadSecureIDKey_WrongLength(t *testing.T) {
	key := make([]byte, 16)
	t.Setenv("SECURE_ID_KEY", base64.StdEncoding.EncodeToString(key))
	_, err := Load()
	if !errors.Is(err, ErrInvalidSecureIDKey) {
		t.Fatalf("Load() error = %v, want ErrInvalidSecureIDKey", err)
	}
	key = make([]byte, 64)
	t.Setenv("SECURE_ID_KEY", base64.StdEncoding.EncodeToString(key))
	_, err = Load()
	if !errors.Is(err, ErrInvalidSecureIDKey) {
		t.Fatalf("Load() error = %v, want ErrInvalidSecureIDKey", err)
	}
}
