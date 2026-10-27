package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/auth"
	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
)

// Admin user management sentinels. Handlers translate them into the API error
// envelope: ErrNotFound → 404, ErrConflict/ErrSelfMutation/ErrLastAdmin → 409,
// ErrNoChanges/ErrInvalidInput → 400.
var (
	// ErrNoChanges is returned when an update carries no mutable field.
	ErrNoChanges = errors.New("service: no changes")
	// ErrSelfMutation is returned when an admin tries to delete themselves,
	// disable themselves or drop their own admin role. It prevents locking
	// the caller out of the panel by accident.
	ErrSelfMutation = errors.New("service: cannot remove own access")
	// ErrLastAdmin is returned when an operation would leave the system with
	// no active admin at all.
	ErrLastAdmin = errors.New("service: cannot remove the last active admin")
	// ErrInvalidInput is returned when a payload fails service-side
	// validation (defense in depth behind the HTTP binding rules).
	ErrInvalidInput = errors.New("service: invalid input")
)

// AdminUserListParams is the service-level query for the admin user listing.
type AdminUserListParams struct {
	Page    int
	PerPage int
	// Search matches the email case-insensitively.
	Search string
	// Role restricts the listing to a single RBAC role; empty means all.
	Role string
	// Active filters by account state; nil means both.
	Active *bool
}

// AdminUserCreateInput is the payload of POST /admin/users. The binding tags
// are the HTTP contract (Gin validates them before the handler reaches the
// service); the service re-validates so it is safe when called directly.
type AdminUserCreateInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Role     string `json:"role" binding:"required,oneof=admin operator viewer"`
	// Active defaults to true when omitted.
	Active *bool `json:"active"`
}

// UnmarshalJSON trims the email before the HTTP binding rules run: a valid
// address never starts or ends with whitespace, so a padded payload such as
// `" ada@example.com "` would otherwise be rejected before the service can
// normalize it.
func (in *AdminUserCreateInput) UnmarshalJSON(data []byte) error {
	type plain AdminUserCreateInput
	var raw plain
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	raw.Email = strings.TrimSpace(raw.Email)
	*in = AdminUserCreateInput(raw)
	return nil
}

// AdminUserUpdateInput is the payload of PUT /admin/users/:id. Every field is
// optional: nil means "leave unchanged". A non-nil password is a reset.
type AdminUserUpdateInput struct {
	Email    *string `json:"email" binding:"omitempty,email"`
	Password *string `json:"password" binding:"omitempty,min=8"`
	Role     *string `json:"role" binding:"omitempty,oneof=admin operator viewer"`
	Active   *bool   `json:"active"`
}

// UnmarshalJSON trims the e-mail before the binding rules run, mirroring
// AdminUserCreateInput.
func (in *AdminUserUpdateInput) UnmarshalJSON(data []byte) error {
	type plain AdminUserUpdateInput
	var raw plain
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Email != nil {
		trimmed := strings.TrimSpace(*raw.Email)
		raw.Email = &trimmed
	}
	*in = AdminUserUpdateInput(raw)
	return nil
}

// AdminUsers is the admin user management port consumed by the HTTP layer. It
// is satisfied by *AdminUsersService and by fakes in tests.
type AdminUsers interface {
	// List returns a filtered page of users.
	List(ctx context.Context, p AdminUserListParams) (PageResult[models.User], error)
	// Get returns a single user or ErrNotFound.
	Get(ctx context.Context, id uuid.UUID) (models.User, error)
	// Create hashes the password and inserts a new user (ErrConflict on a
	// duplicate email).
	Create(ctx context.Context, in AdminUserCreateInput) (models.User, error)
	// Update applies a partial update. actorID is the authenticated caller
	// and drives the self-mutation guard.
	Update(ctx context.Context, actorID, id uuid.UUID, in AdminUserUpdateInput) (models.User, error)
	// Delete removes a user. actorID is the authenticated caller and drives
	// the self-deletion and last-admin guards.
	Delete(ctx context.Context, actorID, id uuid.UUID) error
}

// AdminUsersService implements AdminUsers over the user admin repository and
// the Argon2id password hasher.
type AdminUsersService struct {
	users  repository.UserAdminRepository
	hasher auth.PasswordHasher
}

// NewAdminUsersService wires the admin user service with its collaborators.
func NewAdminUsersService(users repository.UserAdminRepository, hasher auth.PasswordHasher) *AdminUsersService {
	return &AdminUsersService{users: users, hasher: hasher}
}

// ValidRole reports whether role is one of the RBAC roles accepted by the API.
func ValidRole(role string) bool {
	switch models.UserRole(strings.ToLower(strings.TrimSpace(role))) {
	case models.RoleAdmin, models.RoleOperator, models.RoleViewer:
		return true
	default:
		return false
	}
}

// parseRole normalizes and validates an inbound role, mapping an unknown value
// to ErrInvalidInput.
func parseRole(raw string) (models.UserRole, error) {
	role := models.UserRole(strings.ToLower(strings.TrimSpace(raw)))
	if !ValidRole(string(role)) {
		return "", fmt.Errorf("service: role %q: %w", raw, ErrInvalidInput)
	}
	return role, nil
}

// normalizeEmail lower-cases and trims an email address.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// validEmail reports whether email is a plausible single address. It mirrors
// the loose contract of the `email` validator used at the HTTP boundary.
func validEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

// wrapAdminUserErr adds operation context and maps the repository sentinels to
// their service counterparts.
func wrapAdminUserErr(op string, err error) error {
	if err == nil {
		return nil
	}
	msg := "service: " + op
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return fmt.Errorf("%s: %w", msg, ErrNotFound)
	case errors.Is(err, repository.ErrConflict):
		return fmt.Errorf("%s: %w", msg, ErrConflict)
	default:
		return fmt.Errorf("%s: %w", msg, err)
	}
}

// List returns a filtered page of users.
func (s *AdminUsersService) List(ctx context.Context, p AdminUserListParams) (PageResult[models.User], error) {
	role := strings.ToLower(strings.TrimSpace(p.Role))
	if role != "" && !ValidRole(role) {
		return PageResult[models.User]{}, fmt.Errorf("service: list users: role %q: %w", p.Role, ErrInvalidInput)
	}

	page, perPage, opts := listOptions(p.Page, p.PerPage)
	rows, total, err := s.users.List(ctx, opts, repository.UserFilter{
		Search: p.Search,
		Role:   models.UserRole(role),
		Active: p.Active,
	})
	if err != nil {
		return PageResult[models.User]{}, wrapAdminUserErr("list users", err)
	}
	return PageResult[models.User]{Items: rows, Total: total, Page: page, PerPage: perPage}, nil
}

// Get returns a single user or ErrNotFound.
func (s *AdminUsersService) Get(ctx context.Context, id uuid.UUID) (models.User, error) {
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		return models.User{}, wrapAdminUserErr("get user", err)
	}
	return *user, nil
}

// Create hashes the password and inserts a new user.
func (s *AdminUsersService) Create(ctx context.Context, in AdminUserCreateInput) (models.User, error) {
	email := normalizeEmail(in.Email)
	if !validEmail(email) {
		return models.User{}, fmt.Errorf("service: create user: invalid email %q: %w", email, ErrInvalidInput)
	}
	if len(in.Password) < minPasswordLength {
		return models.User{}, fmt.Errorf("service: create user: password must be at least %d characters: %w",
			minPasswordLength, ErrInvalidInput)
	}
	role, err := parseRole(in.Role)
	if err != nil {
		return models.User{}, fmt.Errorf("service: create user: %w", err)
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return models.User{}, fmt.Errorf("service: create user: hash password: %w", err)
	}

	// Accounts are enabled unless the caller explicitly disables them.
	active := true
	if in.Active != nil {
		active = *in.Active
	}

	user := models.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: hash,
		Role:         role,
		Active:       active,
	}
	if err := s.users.Create(ctx, &user); err != nil {
		return models.User{}, wrapAdminUserErr("create user", err)
	}
	return user, nil
}

// Update applies a partial update to an existing user.
//
// Order of checks: an empty payload is rejected (ErrNoChanges), provided
// fields are validated (ErrInvalidInput), the caller may not disable themselves
// or strip their own admin role (ErrSelfMutation), and the last active admin
// may not be disabled or demoted (ErrLastAdmin).
func (s *AdminUsersService) Update(ctx context.Context, actorID, id uuid.UUID, in AdminUserUpdateInput) (models.User, error) {
	if in.Email == nil && in.Password == nil && in.Role == nil && in.Active == nil {
		return models.User{}, fmt.Errorf("service: update user %w", ErrNoChanges)
	}

	current, err := s.users.FindByID(ctx, id)
	if err != nil {
		return models.User{}, wrapAdminUserErr("get user", err)
	}

	// Validate the provided fields before applying the business guards so a
	// malformed payload always answers 400, never 409.
	role := current.Role
	if in.Role != nil {
		role, err = parseRole(*in.Role)
		if err != nil {
			return models.User{}, fmt.Errorf("service: update user: %w", err)
		}
	}

	email := current.Email
	if in.Email != nil {
		email = normalizeEmail(*in.Email)
		if !validEmail(email) {
			return models.User{}, fmt.Errorf("service: update user: invalid email %q: %w", email, ErrInvalidInput)
		}
	}

	if in.Password != nil && len(*in.Password) < minPasswordLength {
		return models.User{}, fmt.Errorf("service: update user: password must be at least %d characters: %w",
			minPasswordLength, ErrInvalidInput)
	}

	active := current.Active
	if in.Active != nil {
		active = *in.Active
	}

	if id == actorID {
		if !active {
			return models.User{}, fmt.Errorf("service: update user: self-deactivation: %w", ErrSelfMutation)
		}
		if role != models.RoleAdmin {
			return models.User{}, fmt.Errorf("service: update user: self-demotion: %w", ErrSelfMutation)
		}
	}

	// Only an active admin losing that status can empty the admin pool.
	stripsAdmin := current.Role == models.RoleAdmin && current.Active && (!active || role != models.RoleAdmin)
	if stripsAdmin {
		if err := s.ensureAnotherActiveAdmin(ctx); err != nil {
			return models.User{}, err
		}
	}

	user := *current
	user.Email = email
	user.Role = role
	user.Active = active
	if in.Password != nil {
		hash, err := s.hasher.Hash(*in.Password)
		if err != nil {
			return models.User{}, fmt.Errorf("service: update user: hash password: %w", err)
		}
		user.PasswordHash = hash
	}

	if err := s.users.Update(ctx, &user); err != nil {
		return models.User{}, wrapAdminUserErr("update user", err)
	}
	return user, nil
}

// Delete removes a user. An admin can neither delete themselves
// (ErrSelfMutation) nor the last active admin (ErrLastAdmin).
func (s *AdminUsersService) Delete(ctx context.Context, actorID, id uuid.UUID) error {
	if id == actorID {
		return fmt.Errorf("service: delete user: self-deletion: %w", ErrSelfMutation)
	}

	current, err := s.users.FindByID(ctx, id)
	if err != nil {
		return wrapAdminUserErr("get user", err)
	}

	if current.Role == models.RoleAdmin && current.Active {
		if err := s.ensureAnotherActiveAdmin(ctx); err != nil {
			return err
		}
	}

	if err := s.users.Delete(ctx, id); err != nil {
		return wrapAdminUserErr("delete user", err)
	}
	return nil
}

// ensureAnotherActiveAdmin rejects the mutation when the system currently has
// at most one active admin: the caller is about to remove the last one.
func (s *AdminUsersService) ensureAnotherActiveAdmin(ctx context.Context) error {
	total, err := s.users.CountByRole(ctx, models.RoleAdmin, true)
	if err != nil {
		return wrapAdminUserErr("count active admins", err)
	}
	if total <= 1 {
		return fmt.Errorf("service: guard last admin: %w", ErrLastAdmin)
	}
	return nil
}
