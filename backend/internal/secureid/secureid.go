// Package secureid provides encrypted, deterministic opaque IDs (style Laravel Crypt::encrypt).
// The encryption is AES-256-GCM with a deterministically derived nonce.
package secureid

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var (
	// ErrInvalidKey is returned when the provided key is not valid.
	ErrInvalidKey = errors.New("secureid: invalid key")
	// ErrInvalidToken is returned when a token is malformed or fails verification.
	ErrInvalidToken = errors.New("secureid: invalid token")
)

const (
	versionPrefix = "v1."
	uuidLen       = 16
	int64Len      = 8
	nonceLen      = 12
	gcmTagLen     = 16
)

// Codec encrypts and decrypts fixed-length values deterministically.
// Determinism is achieved by deriving the GCM nonce from the plaintext
// using HMAC-SHA256 with a key derived from the master key (HMAC-based
// key separation). This ensures Seal(x)==Seal(x) for the same input.
// Security note: the nonce is predictable for a given plaintext, but it is
// never chosen randomly and is bound to the secret key via the derivation;
// without knowledge of the secret key an attacker cannot forge valid
// (nonce, ciphertext) pairs or exploit nonce reuse across different keys.
type Codec struct {
	key []byte
}

// New creates a new Codec. The key must be exactly 32 bytes (AES-256).
func New(key []byte) (*Codec, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("secureid: invalid key length: %w", ErrInvalidKey)
	}
	c := &Codec{key: make([]byte, len(key))}
	copy(c.key, key)
	return c, nil
}

// SealUUID encrypts a UUID to a token string in the format: v1.<base64url(nonce||ct||tag)>.
func (c *Codec) SealUUID(id uuid.UUID) string {
	return c.seal(id[:])
}

// OpenUUID decrypts a token and returns the UUID. It rejects plaintexts of unexpected length.
func (c *Codec) OpenUUID(token string) (uuid.UUID, error) {
	pt, err := c.open(token, uuidLen)
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	copy(id[:], pt)
	return id, nil
}

// SealInt64 encrypts an int64 (big-endian) to a token string.
func (c *Codec) SealInt64(id int64) string {
	b := make([]byte, int64Len)
	binary.BigEndian.PutUint64(b, uint64(id))
	return c.seal(b)
}

// OpenInt64 decrypts a token and returns the int64. It rejects plaintexts of unexpected length.
func (c *Codec) OpenInt64(token string) (int64, error) {
	pt, err := c.open(token, int64Len)
	if err != nil {
		return 0, err
	}
	if len(pt) != int64Len {
		return 0, fmt.Errorf("secureid: unexpected plaintext length: %w", ErrInvalidToken)
	}
	v := binary.BigEndian.Uint64(pt)
	return int64(v), nil
}

func (c *Codec) seal(plain []byte) string {
	nonce := c.deriveNonce(plain)
	aead, err := c.aead()
	if err != nil {
		// Should not happen if key is valid; but return empty on error path
		return ""
	}
	ct := aead.Seal(nil, nonce, plain, nil)
	raw := make([]byte, len(nonce)+len(ct))
	copy(raw, nonce)
	copy(raw[len(nonce):], ct)
	return versionPrefix + base64.RawURLEncoding.EncodeToString(raw)
}

func (c *Codec) open(token string, wantLen int) ([]byte, error) {
	if token == "" || !strings.HasPrefix(token, versionPrefix) {
		return nil, fmt.Errorf("secureid: invalid token: %w", ErrInvalidToken)
	}
	enc := strings.TrimPrefix(token, versionPrefix)
	raw, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return nil, fmt.Errorf("secureid: invalid token encoding: %w", ErrInvalidToken)
	}
	if len(raw) < nonceLen+gcmTagLen+wantLen {
		// minimum: nonce+tag + at least wantLen ct? ct len = wantLen (plaintext same len) + tag appended
		// aead.Seal appends tag; so ct includes tag
	}
	// GCM: raw = nonce||ct||tag
	if len(raw) < nonceLen+gcmTagLen {
		return nil, fmt.Errorf("secureid: invalid token length: %w", ErrInvalidToken)
	}
	nonce := raw[:nonceLen]
	ct := raw[nonceLen:]
	aead, err := c.aead()
	if err != nil {
		return nil, fmt.Errorf("secureid: aead init: %w", err)
	}
	pt, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("secureid: decrypt failed: %w", ErrInvalidToken)
	}
	if len(pt) != wantLen {
		return nil, fmt.Errorf("secureid: unexpected plaintext length: %w", ErrInvalidToken)
	}
	return pt, nil
}

func (c *Codec) deriveNonce(plain []byte) []byte {
	// Derive nonce key: HMAC-SHA256(masterKey, "nonce") to separate contexts.
	nonceKey := hmac.New(sha256.New, c.key)
	nonceKey.Write([]byte("secureid-nonce"))
	nk := nonceKey.Sum(nil)
	// Derive nonce: first 12 bytes of HMAC-SHA256(nonceKey, plaintext)
	m := hmac.New(sha256.New, nk)
	m.Write(plain)
	sum := m.Sum(nil)
	nonce := make([]byte, nonceLen)
	copy(nonce, sum[:nonceLen])
	return nonce
}

func (c *Codec) aead() (cipher.AEAD, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead, nil
}
