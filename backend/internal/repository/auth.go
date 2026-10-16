// Package repository holds the persistence layer. This file implements the
// user store used for authentication.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// UserRepository is the persistence contract for application users.
type UserRepository interface {
	// FindByEmail returns the user with the given email (case-sensitive).
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	// FindByID returns the user with the given primary key.
	FindByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	// Create inserts a new user.
	Create(ctx context.Context, user *models.User) error
	// Update persists changes to an existing user.
	Update(ctx context.Context, user *models.User) error
	// UpdatePassword sets a new password hash for the given user id.
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	// EnsureUser inserts user only if no row with the same email exists. It
	// runs inside a transaction and returns true when a row was created.
	EnsureUser(ctx context.Context, user *models.User) (bool, error)
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository builds a GORM-backed UserRepository.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, fmt.Errorf("repository: find user by email %q: %w", email, mapError(err))
	}
	return &user, nil
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return nil, fmt.Errorf("repository: find user by id %q: %w", id, mapError(err))
	}
	return &user, nil
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return fmt.Errorf("repository: create user %q: %w", user.Email, err)
	}
	return nil
}

func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	if err := r.db.WithContext(ctx).Save(user).Error; err != nil {
		return fmt.Errorf("repository: update user %q: %w", user.Email, err)
	}
	return nil
}

func (r *userRepository) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	result := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		Update("password_hash", passwordHash)
	if result.Error != nil {
		return fmt.Errorf("repository: update password for user %q: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("repository: update password for user %q: %w", id, ErrNotFound)
	}
	return nil
}

func (r *userRepository) EnsureUser(ctx context.Context, user *models.User) (bool, error) {
	created := false

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.User
		err := tx.Where("email = ?", user.Email).First(&existing).Error
		if err == nil {
			*user = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("repository: lookup user %q in transaction: %w", user.Email, err)
		}

		if err := tx.Create(user).Error; err != nil {
			return fmt.Errorf("repository: create user %q in transaction: %w", user.Email, err)
		}
		created = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("repository: ensure user %q: %w", user.Email, err)
	}

	return created, nil
}

// mapError converts GORM's sentinel not-found error into ErrNotFound while
// letting any other error pass through unchanged.
func mapError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
