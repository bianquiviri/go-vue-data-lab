// Package auth implements authentication primitives: Argon2id password
// hashing and JWT access/refresh token management.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id tuning parameters. The defaults follow the OWASP recommendation for
// interactive authentication: 64 MiB of memory, 3 iterations and 2 threads.
const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024 // 64 MiB
	argonThreads uint8  = 2
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

var (
	// ErrInvalidHash is returned when a stored password hash cannot be parsed.
	ErrInvalidHash = errors.New("auth: invalid password hash format")
	// ErrIncompatibleVersion is returned when a hash uses an unsupported
	// Argon2 version.
	ErrIncompatibleVersion = errors.New("auth: incompatible argon2 version")
)

// PasswordHasher abstracts password hashing so callers can be tested without
// paying the cost of real Argon2id computations.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) (bool, error)
}

// Argon2Hasher is the production PasswordHasher backed by Argon2id.
type Argon2Hasher struct{}

// Hash derives an Argon2id hash of password and returns it in the standard
// PHC string format: $argon2id$v=19$m=...,t=...,p=...$<salt>$<hash>.
func (Argon2Hasher) Hash(password string) (string, error) { return Hash(password) }

// Verify reports whether password matches the supplied PHC-encoded hash.
func (Argon2Hasher) Verify(password, hash string) (bool, error) { return Verify(password, hash) }

// Hash derives an Argon2id hash of password using a random salt.
func Hash(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: generate salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonTime,
		argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
	return encoded, nil
}

// Verify reports whether password matches the supplied Argon2id hash.
//
// A malformed or unsupported hash yields an error; a well-formed hash that
// simply does not match yields (false, nil).
func Verify(password, encodedHash string) (bool, error) {
	params, salt, key, err := decodeHash(encodedHash)
	if err != nil {
		return false, fmt.Errorf("auth: verify password: %w", err)
	}

	computed := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, params.keyLen)

	if subtle.ConstantTimeCompare(key, computed) == 1 {
		return true, nil
	}
	return false, nil
}

// argonParams holds the decoded Argon2id cost parameters.
type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
	keyLen  uint32
}

// decodeHash parses a PHC-encoded Argon2id hash into its parameters, salt and
// derived key.
func decodeHash(encodedHash string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "" {
		return argonParams{}, nil, nil, fmt.Errorf("%w: expected 6 parts", ErrInvalidHash)
	}

	if parts[1] != "argon2id" {
		return argonParams{}, nil, nil, fmt.Errorf("%w: unsupported algorithm %q", ErrInvalidHash, parts[1])
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("%w: parse version: %v", ErrInvalidHash, err)
	}
	if version != argon2.Version {
		return argonParams{}, nil, nil, fmt.Errorf("%w: got v=%d, want v=%d", ErrIncompatibleVersion, version, argon2.Version)
	}

	var (
		memory  uint32
		iters   uint32
		threads uint8
	)
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iters, &threads); err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("%w: parse params: %v", ErrInvalidHash, err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("%w: decode salt: %v", ErrInvalidHash, err)
	}

	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("%w: decode key: %v", ErrInvalidHash, err)
	}
	if len(key) == 0 {
		return argonParams{}, nil, nil, fmt.Errorf("%w: empty key", ErrInvalidHash)
	}

	return argonParams{
		memory:  memory,
		time:    iters,
		threads: threads,
		keyLen:  uint32(len(key)),
	}, salt, key, nil
}
