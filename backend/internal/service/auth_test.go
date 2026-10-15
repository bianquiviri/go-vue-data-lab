package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/auth"
	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// --- Test doubles -----------------------------------------------------------

type fakeUserRepo struct {
	byEmail map[string]*models.User
	byID    map[uuid.UUID]*models.User

	findByEmailErr error
	findByIDErr    error
	updatePassErr  error
	ensureErr      error

	updatedPasswordHash string
	updatePasswordCalls int
}

func newFakeUserRepo(users ...*models.User) *fakeUserRepo {
	r := &fakeUserRepo{
		byEmail: make(map[string]*models.User),
		byID:    make(map[uuid.UUID]*models.User),
	}
	for _, u := range users {
		r.byEmail[u.Email] = u
		r.byID[u.ID] = u
	}
	return r
}

func (r *fakeUserRepo) FindByEmail(_ context.Context, email string) (*models.User, error) {
	if r.findByEmailErr != nil {
		return nil, r.findByEmailErr
	}
	u, ok := r.byEmail[email]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (r *fakeUserRepo) FindByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	if r.findByIDErr != nil {
		return nil, r.findByIDErr
	}
	u, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (r *fakeUserRepo) Create(_ context.Context, user *models.User) error {
	r.byEmail[user.Email] = user
	r.byID[user.ID] = user
	return nil
}

func (r *fakeUserRepo) Update(_ context.Context, user *models.User) error {
	r.byEmail[user.Email] = user
	r.byID[user.ID] = user
	return nil
}

func (r *fakeUserRepo) UpdatePassword(_ context.Context, id uuid.UUID, passwordHash string) error {
	r.updatePasswordCalls++
	if r.updatePassErr != nil {
		return r.updatePassErr
	}
	u, ok := r.byID[id]
	if !ok {
		return repository.ErrNotFound
	}
	u.PasswordHash = passwordHash
	r.updatedPasswordHash = passwordHash
	return nil
}

func (r *fakeUserRepo) EnsureUser(_ context.Context, user *models.User) (bool, error) {
	if r.ensureErr != nil {
		return false, r.ensureErr
	}
	if existing, ok := r.byEmail[user.Email]; ok {
		*user = *existing
		return false, nil
	}
	r.byEmail[user.Email] = user
	r.byID[user.ID] = user
	return true, nil
}

type fakeHasher struct {
	hashErr   error
	verifyErr error
}

func (h fakeHasher) Hash(password string) (string, error) {
	if h.hashErr != nil {
		return "", h.hashErr
	}
	return "hashed:" + password, nil
}

func (h fakeHasher) Verify(password, hash string) (bool, error) {
	if h.verifyErr != nil {
		return false, h.verifyErr
	}
	return hash == "hashed:"+password, nil
}

func newTestService(repo repository.UserRepository, hasher auth.PasswordHasher) service.AuthService {
	tokens := auth.NewJWTManager("service-test-secret", 15*time.Minute, time.Hour)
	return service.NewAuthService(repo, tokens, hasher)
}

func activeUser(email, password string, role models.UserRole) *models.User {
	return &models.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: "hashed:" + password,
		Role:         role,
		Active:       true,
	}
}

// --- Login ------------------------------------------------------------------

func TestLoginSuccess(t *testing.T) {
	user := activeUser("user@example.com", "correct-password", models.RoleAdmin)
	svc := newTestService(newFakeUserRepo(user), fakeHasher{})

	result, err := svc.Login(context.Background(), "user@example.com", "correct-password")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.User.ID != user.ID {
		t.Errorf("user id = %v, want %v", result.User.ID, user.ID)
	}
	if result.Tokens.AccessToken == "" || result.Tokens.RefreshToken == "" {
		t.Error("Login() returned empty tokens")
	}
	if result.Tokens.AccessExpiresIn != 15*time.Minute {
		t.Errorf("AccessExpiresIn = %v, want 15m", result.Tokens.AccessExpiresIn)
	}
}

func TestLoginNormalizesEmail(t *testing.T) {
	user := activeUser("user@example.com", "pw", models.RoleViewer)
	svc := newTestService(newFakeUserRepo(user), fakeHasher{})

	if _, err := svc.Login(context.Background(), "  USER@example.com  ", "pw"); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
}

func TestLoginUnknownEmail(t *testing.T) {
	svc := newTestService(newFakeUserRepo(), fakeHasher{})

	_, err := svc.Login(context.Background(), "missing@example.com", "pw")
	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	user := activeUser("user@example.com", "correct", models.RoleViewer)
	svc := newTestService(newFakeUserRepo(user), fakeHasher{})

	_, err := svc.Login(context.Background(), "user@example.com", "wrong")
	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginInactiveUser(t *testing.T) {
	user := activeUser("user@example.com", "pw", models.RoleViewer)
	user.Active = false
	svc := newTestService(newFakeUserRepo(user), fakeHasher{})

	_, err := svc.Login(context.Background(), "user@example.com", "pw")
	if !errors.Is(err, service.ErrUserInactive) {
		t.Fatalf("error = %v, want ErrUserInactive", err)
	}
}

func TestLoginRepositoryError(t *testing.T) {
	repo := newFakeUserRepo()
	repo.findByEmailErr = errors.New("boom")
	svc := newTestService(repo, fakeHasher{})

	_, err := svc.Login(context.Background(), "user@example.com", "pw")
	if err == nil || errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want a wrapped repository error", err)
	}
}

func TestLoginVerifyError(t *testing.T) {
	user := activeUser("user@example.com", "pw", models.RoleViewer)
	svc := newTestService(newFakeUserRepo(user), fakeHasher{verifyErr: errors.New("boom")})

	_, err := svc.Login(context.Background(), "user@example.com", "pw")
	if err == nil || errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want a wrapped verify error", err)
	}
}

// --- Refresh ----------------------------------------------------------------

func TestRefreshSuccess(t *testing.T) {
	user := activeUser("user@example.com", "pw", models.RoleOperator)
	repo := newFakeUserRepo(user)
	svc := newTestService(repo, fakeHasher{})

	login, err := svc.Login(context.Background(), "user@example.com", "pw")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	result, err := svc.Refresh(context.Background(), login.Tokens.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if result.User.ID != user.ID {
		t.Errorf("user id = %v, want %v", result.User.ID, user.ID)
	}
	if result.Tokens.AccessToken == "" {
		t.Error("Refresh() returned an empty access token")
	}
}

func TestRefreshRejectsAccessToken(t *testing.T) {
	user := activeUser("user@example.com", "pw", models.RoleViewer)
	svc := newTestService(newFakeUserRepo(user), fakeHasher{})

	login, err := svc.Login(context.Background(), "user@example.com", "pw")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	_, err = svc.Refresh(context.Background(), login.Tokens.AccessToken)
	if !errors.Is(err, service.ErrInvalidToken) {
		t.Fatalf("error = %v, want ErrInvalidToken", err)
	}
}

func TestRefreshInvalidToken(t *testing.T) {
	svc := newTestService(newFakeUserRepo(), fakeHasher{})

	_, err := svc.Refresh(context.Background(), "garbage")
	if !errors.Is(err, service.ErrInvalidToken) {
		t.Fatalf("error = %v, want ErrInvalidToken", err)
	}
}

func TestRefreshUserDeleted(t *testing.T) {
	user := activeUser("user@example.com", "pw", models.RoleViewer)
	repo := newFakeUserRepo(user)
	svc := newTestService(repo, fakeHasher{})

	login, err := svc.Login(context.Background(), "user@example.com", "pw")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	delete(repo.byID, user.ID)

	_, err = svc.Refresh(context.Background(), login.Tokens.RefreshToken)
	if !errors.Is(err, service.ErrInvalidToken) {
		t.Fatalf("error = %v, want ErrInvalidToken", err)
	}
}

func TestRefreshInactiveUser(t *testing.T) {
	user := activeUser("user@example.com", "pw", models.RoleViewer)
	repo := newFakeUserRepo(user)
	svc := newTestService(repo, fakeHasher{})

	login, err := svc.Login(context.Background(), "user@example.com", "pw")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	user.Active = false

	_, err = svc.Refresh(context.Background(), login.Tokens.RefreshToken)
	if !errors.Is(err, service.ErrUserInactive) {
		t.Fatalf("error = %v, want ErrUserInactive", err)
	}
}

// --- Me ---------------------------------------------------------------------

func TestMeSuccess(t *testing.T) {
	user := activeUser("user@example.com", "pw", models.RoleViewer)
	svc := newTestService(newFakeUserRepo(user), fakeHasher{})

	got, err := svc.Me(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Me() error = %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("id = %v, want %v", got.ID, user.ID)
	}
}

func TestMeNotFound(t *testing.T) {
	svc := newTestService(newFakeUserRepo(), fakeHasher{})

	_, err := svc.Me(context.Background(), uuid.New())
	if !errors.Is(err, service.ErrUserNotFound) {
		t.Fatalf("error = %v, want ErrUserNotFound", err)
	}
}

// --- ChangePassword ---------------------------------------------------------

func TestChangePasswordSuccess(t *testing.T) {
	user := activeUser("user@example.com", "old-password", models.RoleViewer)
	repo := newFakeUserRepo(user)
	svc := newTestService(repo, fakeHasher{})

	if err := svc.ChangePassword(context.Background(), user.ID, "old-password", "new-password"); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if repo.updatedPasswordHash != "hashed:new-password" {
		t.Errorf("stored hash = %q, want %q", repo.updatedPasswordHash, "hashed:new-password")
	}
}

func TestChangePasswordWrongOldPassword(t *testing.T) {
	user := activeUser("user@example.com", "old-password", models.RoleViewer)
	repo := newFakeUserRepo(user)
	svc := newTestService(repo, fakeHasher{})

	err := svc.ChangePassword(context.Background(), user.ID, "wrong", "new-password")
	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want ErrInvalidCredentials", err)
	}
	if repo.updatePasswordCalls != 0 {
		t.Error("UpdatePassword was called despite a bad old password")
	}
}

func TestChangePasswordWeakNewPassword(t *testing.T) {
	user := activeUser("user@example.com", "old-password", models.RoleViewer)
	svc := newTestService(newFakeUserRepo(user), fakeHasher{})

	err := svc.ChangePassword(context.Background(), user.ID, "old-password", "short")
	if !errors.Is(err, service.ErrWeakPassword) {
		t.Fatalf("error = %v, want ErrWeakPassword", err)
	}
}

func TestChangePasswordUserNotFound(t *testing.T) {
	svc := newTestService(newFakeUserRepo(), fakeHasher{})

	err := svc.ChangePassword(context.Background(), uuid.New(), "old-password", "new-password")
	if !errors.Is(err, service.ErrUserNotFound) {
		t.Fatalf("error = %v, want ErrUserNotFound", err)
	}
}

func TestChangePasswordUpdateError(t *testing.T) {
	user := activeUser("user@example.com", "old-password", models.RoleViewer)
	repo := newFakeUserRepo(user)
	repo.updatePassErr = errors.New("boom")
	svc := newTestService(repo, fakeHasher{})

	err := svc.ChangePassword(context.Background(), user.ID, "old-password", "new-password")
	if err == nil || errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want a wrapped update error", err)
	}
}

// --- Logout -----------------------------------------------------------------

func TestLogoutIsStateless(t *testing.T) {
	svc := newTestService(newFakeUserRepo(), fakeHasher{})

	if err := svc.Logout(context.Background(), uuid.New()); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
}
