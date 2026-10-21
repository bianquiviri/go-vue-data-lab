package secureid

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNew_ValidKey32Bytes(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	if _, err := New(key); err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
}

func TestNew_InvalidKeyLengths(t *testing.T) {
	cases := [][]byte{
		make([]byte, 0),
		make([]byte, 31),
		make([]byte, 33),
	}
	for i, k := range cases {
		_, err := New(k)
		if !errors.Is(err, ErrInvalidKey) {
			t.Errorf("case %d: New() error = %v, want ErrInvalidKey", i, err)
		}
	}
}

func TestSealOpenUUID_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	codec, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	cases := []uuid.UUID{
		uuid.Nil,
		uuid.New(),
		{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
	}
	for _, id := range cases {
		tok := codec.SealUUID(id)
		got, err := codec.OpenUUID(tok)
		if err != nil {
			t.Fatalf("OpenUUID(%q) error = %v", tok, err)
		}
		if got != id {
			t.Errorf("got %v, want %v", got, id)
		}
	}
}

func TestSealOpenInt64_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	codec, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	cases := []int64{0, 1, -1, 42, -9223372036854775808, 9223372036854775807}
	for _, id := range cases {
		tok := codec.SealInt64(id)
		got, err := codec.OpenInt64(tok)
		if err != nil {
			t.Fatalf("OpenInt64(%q) error = %v", tok, err)
		}
		if got != id {
			t.Errorf("got %d, want %d", got, id)
		}
	}
}

func TestSeal_Deterministic(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	codec, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	t1 := codec.SealUUID(id)
	t2 := codec.SealUUID(id)
	t3 := codec.SealUUID(id)
	if t1 != t2 || t2 != t3 {
		t.Errorf("not deterministic: %q, %q, %q", t1, t2, t3)
	}

	var n int64 = 123456789
	s1 := codec.SealInt64(n)
	s2 := codec.SealInt64(n)
	if s1 != s2 {
		t.Errorf("not deterministic: %q, %q", s1, s2)
	}
}

func TestOpen_InvalidPrefix(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	codec, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	tok := codec.SealUUID(id)
	bad := strings.Replace(tok, "v1.", "v2.", 1)
	if _, err := codec.OpenUUID(bad); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("OpenUUID(%q) error = %v, want ErrInvalidToken", bad, err)
	}
	if _, err := codec.OpenInt64(bad); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("OpenInt64(%q) error = %v, want ErrInvalidToken", bad, err)
	}
}

func TestOpen_EmptyToken(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	codec, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := codec.OpenUUID(""); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("OpenUUID(\"\") error = %v, want ErrInvalidToken", err)
	}
	if _, err := codec.OpenInt64(""); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("OpenInt64(\"\") error = %v, want ErrInvalidToken", err)
	}
}

func TestOpen_CorruptedTokenMutations(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	codec, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	tok := codec.SealUUID(id)
	if !strings.HasPrefix(tok, "v1.") {
		t.Fatal("bad prefix")
	}
	enc := strings.TrimPrefix(tok, "v1.")
	raw, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 12+16+1 {
		t.Fatal("unexpected raw len")
	}
	bad := make([]byte, len(raw))
	copy(bad, raw)
	bad[0] ^= 0xFF
	if _, err := codec.OpenUUID("v1." + base64.RawURLEncoding.EncodeToString(bad)); err == nil {
		t.Errorf("mutating nonce should fail")
	}
	copy(bad, raw)
	bad[12] ^= 0x01
	if _, err := codec.OpenUUID("v1." + base64.RawURLEncoding.EncodeToString(bad)); err == nil {
		t.Errorf("mutating ct should fail")
	}
	copy(bad, raw)
	bad[len(bad)-1] ^= 0xAA
	if _, err := codec.OpenUUID("v1." + base64.RawURLEncoding.EncodeToString(bad)); err == nil {
		t.Errorf("mutating tag should fail")
	}
}

func TestOpen_DifferentKeyFails(t *testing.T) {
	key1 := make([]byte, 32)
	if _, err := rand.Read(key1); err != nil {
		t.Fatal(err)
	}
	key2 := make([]byte, 32)
	if _, err := rand.Read(key2); err != nil {
		t.Fatal(err)
	}
	c1, _ := New(key1)
	c2, _ := New(key2)
	id := uuid.New()
	tok := c1.SealUUID(id)
	if _, err := c2.OpenUUID(tok); err == nil {
		t.Errorf("opening with different key should fail")
	}
}

func TestOpen_BadBase64(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	codec, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := codec.OpenUUID("v1.not-base64!!!"); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("error = %v, want ErrInvalidToken", err)
	}
}

func TestNew_WrongLengthErrorsWrapped(t *testing.T) {
	_, err := New(make([]byte, 31))
	if err == nil || !errors.Is(err, ErrInvalidKey) {
		t.Fatal("want ErrInvalidKey")
	}
	if !strings.Contains(err.Error(), "secureid:") {
		t.Errorf("error should wrap with secureid context: %v", err)
	}
}

func TestOpenUUID_RejectsUnexpectedPlaintextLength(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	c, _ := New(key)
	tok := c.SealInt64(42)
	if _, err := c.OpenUUID(tok); err == nil {
		t.Errorf("should reject opening int64 token as UUID")
	} else if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("want ErrInvalidToken, got %v", err)
	}
}

func TestOpenInt64_RejectsUnexpectedPlaintextLength(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	c, _ := New(key)
	tok := c.SealUUID(uuid.New())
	if _, err := c.OpenInt64(tok); err == nil {
		t.Errorf("should reject opening UUID token as int64")
	} else if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("want ErrInvalidToken, got %v", err)
	}
}
