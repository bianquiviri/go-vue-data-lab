package auth_test

import (
	"strings"
	"testing"

	"github.com/josebianco/go-vue-data-lab/internal/auth"
)

func TestHashAndVerifyRoundTrip(t *testing.T) {
	const password = "s3cr3t-P@ssw0rd!"

	hash, err := auth.Hash(password)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("hash %q does not use the argon2id PHC prefix", hash)
	}

	ok, err := auth.Verify(password, hash)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !ok {
		t.Error("Verify() = false, want true for the correct password")
	}
}

func TestVerifyWrongPassword(t *testing.T) {
	hash, err := auth.Hash("correct-horse")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	ok, err := auth.Verify("battery-staple", hash)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if ok {
		t.Error("Verify() = true, want false for an incorrect password")
	}
}

func TestHashUsesUniqueSalt(t *testing.T) {
	first, err := auth.Hash("same-password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	second, err := auth.Hash("same-password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	if first == second {
		t.Error("two hashes of the same password are identical; salt is not random")
	}
}

func TestVerifyMalformedHash(t *testing.T) {
	cases := map[string]string{
		"empty":           "",
		"too few parts":   "$argon2id$v=19$m=65536,t=3,p=2$abc",
		"bad algorithm":   "$argon2i$v=19$m=65536,t=3,p=2$YWJj$YWJj",
		"bad version":     "$argon2id$v=13$m=65536,t=3,p=2$YWJj$YWJj",
		"bad params":      "$argon2id$v=19$m=x,t=3,p=2$YWJj$YWJj",
		"bad base64 salt": "$argon2id$v=19$m=65536,t=3,p=2$!!!!$YWJj",
		"bad base64 key":  "$argon2id$v=19$m=65536,t=3,p=2$YWJj$!!!!",
		"empty key":       "$argon2id$v=19$m=65536,t=3,p=2$YWJj$",
		"wrong separator": "argon2id$v=19$m=65536,t=3,p=2$YWJj$YWJj",
		"too many parts":  "$argon2id$v=19$m=65536,t=3,p=2$YWJj$YWJj$extra",
	}

	for name, hash := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := auth.Verify("whatever", hash); err == nil {
				t.Fatalf("Verify(%q) error = nil, want error", hash)
			}
		})
	}
}

func TestArgon2HasherImplementsInterface(t *testing.T) {
	var hasher auth.PasswordHasher = auth.Argon2Hasher{}

	hash, err := hasher.Hash("interface-check")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	ok, err := hasher.Verify("interface-check", hash)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !ok {
		t.Error("Verify() = false, want true")
	}
}
