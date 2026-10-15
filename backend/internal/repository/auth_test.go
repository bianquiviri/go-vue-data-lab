package repository

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestMapErrorTranslatesRecordNotFound(t *testing.T) {
	if err := mapError(gorm.ErrRecordNotFound); !errors.Is(err, ErrNotFound) {
		t.Errorf("mapError(ErrRecordNotFound) = %v, want ErrNotFound", err)
	}
}

func TestMapErrorPassesThroughOtherErrors(t *testing.T) {
	sentinel := errors.New("boom")
	if err := mapError(sentinel); !errors.Is(err, sentinel) {
		t.Errorf("mapError(sentinel) = %v, want sentinel", err)
	}
}

func TestNewUserRepositoryReturnsNonNil(t *testing.T) {
	if NewUserRepository(nil) == nil {
		t.Error("NewUserRepository(nil) = nil, want non-nil repository")
	}
}
