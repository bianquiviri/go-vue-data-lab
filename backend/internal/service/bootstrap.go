package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/auth"
	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
)

// BootstrapAdmin ensures the configured administrator account exists.
//
// It is idempotent: if a user with the given email already exists it is left
// untouched and (false, nil) is returned. Otherwise the password is hashed with
// the supplied hasher and the admin row is created transactionally, returning
// (true, nil). All errors are wrapped with %w.
func BootstrapAdmin(ctx context.Context, users repository.UserRepository, hasher auth.PasswordHasher, email, password string) (bool, error) {
	if email == "" {
		return false, fmt.Errorf("service: bootstrap admin: empty email")
	}
	if password == "" {
		return false, fmt.Errorf("service: bootstrap admin: empty password")
	}
	if users == nil {
		return false, fmt.Errorf("service: bootstrap admin: nil user repository")
	}
	if hasher == nil {
		return false, fmt.Errorf("service: bootstrap admin: nil password hasher")
	}

	hash, err := hasher.Hash(password)
	if err != nil {
		return false, fmt.Errorf("service: bootstrap admin: hash password: %w", err)
	}

	user := &models.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: hash,
		Role:         models.RoleAdmin,
		Active:       true,
	}

	created, err := users.EnsureUser(ctx, user)
	if err != nil {
		return false, fmt.Errorf("service: bootstrap admin: %w", err)
	}
	return created, nil
}
