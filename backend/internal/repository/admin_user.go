package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// UserFilter narrows admin user listings.
type UserFilter struct {
	// Search matches the email case-insensitively (wildcards are escaped).
	Search string
	// Role restricts the listing to a single RBAC role; empty means all.
	Role models.UserRole
	// Active filters by account state; nil means both.
	Active *bool
}

// UserAdminRepository is the persistence contract of the admin user
// management endpoints: listing, soft mutations and the aggregate used to
// protect the last active admin.
//
// It is deliberately separate from UserRepository, which authentication keeps
// consuming through its narrow interface: adding methods here never forces the
// auth fakes/tests to grow.
type UserAdminRepository interface {
	// List returns a page of users plus the total count for the same filter.
	List(ctx context.Context, opts ListOptions, f UserFilter) ([]models.User, int64, error)
	// FindByID returns the user with the given id, or ErrNotFound. It is the
	// same lookup UserRepository exposes, reused here so both interfaces share
	// one implementation.
	FindByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	// Create inserts a new user; a duplicate email maps to ErrConflict.
	Create(ctx context.Context, user *models.User) error
	// Update persists every column of an existing user; a duplicate email
	// maps to ErrConflict and a missing row to ErrNotFound.
	Update(ctx context.Context, user *models.User) error
	// Delete removes a user by id; a missing row maps to ErrNotFound.
	Delete(ctx context.Context, id uuid.UUID) error
	// CountByRole returns how many users carry role. When activeOnly is set
	// only enabled accounts are counted.
	CountByRole(ctx context.Context, role models.UserRole, activeOnly bool) (int64, error)
}

// NewUserAdminRepository builds the admin user repository on db (lab_target,
// where the users table lives). It reuses the GORM implementation behind
// NewUserRepository, so both interfaces read and write the same table.
func NewUserAdminRepository(db *gorm.DB) UserAdminRepository {
	return &userRepository{db: db}
}

// adminUserQuery builds the filtered query shared by List and its count.
func adminUserQuery(db *gorm.DB, f UserFilter) *gorm.DB {
	q := db.Model(&models.User{})
	if search := strings.TrimSpace(f.Search); search != "" {
		q = q.Where("email ILIKE ?", ilikePattern(search))
	}
	if role := strings.TrimSpace(string(f.Role)); role != "" {
		q = q.Where("role = ?", role)
	}
	if f.Active != nil {
		q = q.Where("active = ?", *f.Active)
	}
	return q
}

// List returns a filtered page of users (newest first) plus the total count.
func (r *userRepository) List(ctx context.Context, opts ListOptions, f UserFilter) ([]models.User, int64, error) {
	var total int64
	if err := adminUserQuery(r.db.WithContext(ctx), f).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("repository: count users: %w", err)
	}

	rows := make([]models.User, 0)
	q := applyPaging(adminUserQuery(r.db.WithContext(ctx), f), opts).
		Order("created_at DESC, id DESC")
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("repository: list users: %w", err)
	}
	return rows, total, nil
}

// Delete removes a user by id. Returns ErrNotFound if no rows were affected.
func (r *userRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&models.User{}, "id = ?", id)
	if res.Error != nil {
		return mapPgError(fmt.Sprintf("delete user %s", id), res.Error)
	}
	if res.RowsAffected == 0 {
		return wrapNotFound(fmt.Sprintf("delete user %s", id), gorm.ErrRecordNotFound)
	}
	return nil
}

// CountByRole counts users by role, optionally restricted to active accounts.
func (r *userRepository) CountByRole(ctx context.Context, role models.UserRole, activeOnly bool) (int64, error) {
	q := r.db.WithContext(ctx).Model(&models.User{}).Where("role = ?", role)
	if activeOnly {
		q = q.Where("active = ?", true)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, fmt.Errorf("repository: count users by role %q: %w", role, err)
	}
	return total, nil
}
