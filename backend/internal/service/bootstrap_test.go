package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

func TestBootstrapAdminCreatesUser(t *testing.T) {
	repo := newFakeUserRepo()

	created, err := service.BootstrapAdmin(context.Background(), repo, fakeHasher{}, "root@gmail.com", "super-secret")
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	if !created {
		t.Fatal("BootstrapAdmin() created = false, want true")
	}

	user, ok := repo.byEmail["root@gmail.com"]
	if !ok {
		t.Fatal("admin user was not persisted")
	}
	if user.Role != models.RoleAdmin {
		t.Errorf("role = %q, want %q", user.Role, models.RoleAdmin)
	}
	if !user.Active {
		t.Error("admin user is not active")
	}
	if user.PasswordHash != "hashed:super-secret" {
		t.Errorf("password hash = %q, want %q", user.PasswordHash, "hashed:super-secret")
	}
}

func TestBootstrapAdminIsIdempotent(t *testing.T) {
	existing := activeUser("root@gmail.com", "original", models.RoleAdmin)
	repo := newFakeUserRepo(existing)

	created, err := service.BootstrapAdmin(context.Background(), repo, fakeHasher{}, "root@gmail.com", "different")
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	if created {
		t.Fatal("BootstrapAdmin() created = true, want false for an existing user")
	}
	if existing.PasswordHash != "hashed:original" {
		t.Errorf("existing password hash was overwritten: %q", existing.PasswordHash)
	}
	if len(repo.byEmail) != 1 {
		t.Errorf("user count = %d, want 1 (no duplicates)", len(repo.byEmail))
	}
}

func TestBootstrapAdminValidation(t *testing.T) {
	repo := newFakeUserRepo()

	if _, err := service.BootstrapAdmin(context.Background(), repo, fakeHasher{}, "", "pw"); err == nil {
		t.Error("expected error for empty email")
	}
	if _, err := service.BootstrapAdmin(context.Background(), repo, fakeHasher{}, "root@gmail.com", ""); err == nil {
		t.Error("expected error for empty password")
	}
	if _, err := service.BootstrapAdmin(context.Background(), nil, fakeHasher{}, "root@gmail.com", "pw"); err == nil {
		t.Error("expected error for nil repository")
	}
	if _, err := service.BootstrapAdmin(context.Background(), repo, nil, "root@gmail.com", "pw"); err == nil {
		t.Error("expected error for nil hasher")
	}
}

func TestBootstrapAdminHashError(t *testing.T) {
	repo := newFakeUserRepo()

	_, err := service.BootstrapAdmin(context.Background(), repo, fakeHasher{hashErr: errors.New("boom")}, "root@gmail.com", "pw")
	if err == nil {
		t.Fatal("expected wrapped hash error")
	}
}

func TestBootstrapAdminEnsureError(t *testing.T) {
	repo := newFakeUserRepo()
	repo.ensureErr = errors.New("boom")

	_, err := service.BootstrapAdmin(context.Background(), repo, fakeHasher{}, "root@gmail.com", "pw")
	if err == nil {
		t.Fatal("expected wrapped ensure error")
	}
}
