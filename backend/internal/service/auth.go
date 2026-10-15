// Package service holds the application's business logic layer. This file
// implements the authentication service.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/auth"
	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
)

// Minimum acceptable password length enforced on change/reset.
const minPasswordLength = 8

// Domain errors returned by the auth service. Handlers map these to HTTP
// status codes.
var (
	// ErrInvalidCredentials signals a bad email/password combination.
	ErrInvalidCredentials = errors.New("service: invalid credentials")
	// ErrUserInactive signals the account is disabled.
	ErrUserInactive = errors.New("service: user inactive")
	// ErrInvalidToken signals an unusable/expired refresh token.
	ErrInvalidToken = errors.New("service: invalid token")
	// ErrUserNotFound signals the referenced user does not exist.
	ErrUserNotFound = errors.New("service: user not found")
	// ErrWeakPassword signals a password that fails the minimum policy.
	ErrWeakPassword = errors.New("service: password too weak")
)

// TokenPair bundles freshly minted access and refresh tokens with their TTLs.
type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresIn  time.Duration
	RefreshExpiresIn time.Duration
}

// LoginResult is the outcome of a successful login or refresh.
type LoginResult struct {
	User   *models.User
	Tokens TokenPair
}

// AuthService is the application-facing authentication contract.
type AuthService interface {
	// Login authenticates an email/password pair and issues tokens.
	Login(ctx context.Context, email, password string) (*LoginResult, error)
	// Refresh exchanges a valid refresh token for a new token pair.
	Refresh(ctx context.Context, refreshToken string) (*LoginResult, error)
	// Me returns the user identified by userID.
	Me(ctx context.Context, userID uuid.UUID) (*models.User, error)
	// ChangePassword verifies oldPassword then stores a hash of newPassword.
	ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error
	// Logout invalidates the caller's session. It is stateless today.
	Logout(ctx context.Context, userID uuid.UUID) error
}

type authService struct {
	users  repository.UserRepository
	tokens *auth.JWTManager
	hasher auth.PasswordHasher
}

// NewAuthService wires the auth service with its collaborators.
func NewAuthService(users repository.UserRepository, tokens *auth.JWTManager, hasher auth.PasswordHasher) AuthService {
	return &authService{users: users, tokens: tokens, hasher: hasher}
}

func (s *authService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("service: login: %w", ErrInvalidCredentials)
		}
		return nil, fmt.Errorf("service: login: %w", err)
	}

	if !user.Active {
		return nil, fmt.Errorf("service: login: %w", ErrUserInactive)
	}

	ok, err := s.hasher.Verify(password, user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("service: login: verify password: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("service: login: %w", ErrInvalidCredentials)
	}

	result, err := s.issueTokens(user)
	if err != nil {
		return nil, fmt.Errorf("service: login: %w", err)
	}
	return result, nil
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	claims, err := s.tokens.ValidateToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("service: refresh: %w", ErrInvalidToken)
	}
	if claims.TokenType != auth.TokenTypeRefresh {
		return nil, fmt.Errorf("service: refresh: %w", ErrInvalidToken)
	}

	user, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("service: refresh: %w", ErrInvalidToken)
		}
		return nil, fmt.Errorf("service: refresh: %w", err)
	}
	if !user.Active {
		return nil, fmt.Errorf("service: refresh: %w", ErrUserInactive)
	}

	result, err := s.issueTokens(user)
	if err != nil {
		return nil, fmt.Errorf("service: refresh: %w", err)
	}
	return result, nil
}

func (s *authService) Me(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("service: me: %w", ErrUserNotFound)
		}
		return nil, fmt.Errorf("service: me: %w", err)
	}
	return user, nil
}

func (s *authService) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	if len(newPassword) < minPasswordLength {
		return fmt.Errorf("service: change password: %w", ErrWeakPassword)
	}

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("service: change password: %w", ErrUserNotFound)
		}
		return fmt.Errorf("service: change password: %w", err)
	}

	ok, err := s.hasher.Verify(oldPassword, user.PasswordHash)
	if err != nil {
		return fmt.Errorf("service: change password: verify old password: %w", err)
	}
	if !ok {
		return fmt.Errorf("service: change password: %w", ErrInvalidCredentials)
	}

	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("service: change password: hash new password: %w", err)
	}

	if err := s.users.UpdatePassword(ctx, user.ID, hash); err != nil {
		return fmt.Errorf("service: change password: %w", err)
	}
	return nil
}

// Logout is stateless for now: since tokens are not persisted or denylisted,
// the client simply discards them. It will become stateful once a token store
// (or token version counter) is introduced.
func (s *authService) Logout(_ context.Context, _ uuid.UUID) error {
	return nil
}

// issueTokens mints a token pair for user and records the configured TTLs.
func (s *authService) issueTokens(user *models.User) (*LoginResult, error) {
	access, refresh, err := s.tokens.GenerateTokens(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	return &LoginResult{
		User: user,
		Tokens: TokenPair{
			AccessToken:      access,
			RefreshToken:     refresh,
			AccessExpiresIn:  s.tokens.AccessTTL(),
			RefreshExpiresIn: s.tokens.RefreshTTL(),
		},
	}, nil
}
