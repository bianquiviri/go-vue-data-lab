package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
)

// TestAdminUserRepositoryIntegration exercises the admin user queries against
// a real lab_target. It skips cleanly when PostgreSQL is not available (see
// openTargetDB).
func TestAdminUserRepositoryIntegration(t *testing.T) {
	db := openTargetDB(t)
	ctx := context.Background()
	repo := repository.NewUserAdminRepository(db)

	email := "admin-it-" + uuid.NewString()[:8] + "@example.com"
	user := models.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: "hashed:integration",
		Role:         models.RoleViewer,
		Active:       true,
	}
	if err := repo.Create(ctx, &user); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), user.ID) })

	// A second row with the same email must surface as ErrConflict.
	dup := models.User{ID: uuid.New(), Email: email, PasswordHash: "x", Role: models.RoleViewer, Active: true}
	if err := repo.Create(ctx, &dup); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("Create duplicate: expected ErrConflict, got %v", err)
	}

	active := true
	rows, total, err := repo.List(ctx, repository.ListOptions{Limit: 50}, repository.UserFilter{
		Search: email,
		Role:   models.RoleViewer,
		Active: &active,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ID != user.ID {
		t.Fatalf("List returned total=%d rows=%d, want the seeded user", total, len(rows))
	}

	// Wildcards in the search must be matched literally.
	if _, total, err := repo.List(ctx, repository.ListOptions{Limit: 50}, repository.UserFilter{
		Search: "%",
	}); err != nil || total != 0 {
		t.Errorf("literal wildcard search: total=%d err=%v, want 0/nil", total, err)
	}

	// CountByRole must only count the seeded row while it is an active admin.
	adminsBefore, err := repo.CountByRole(ctx, models.RoleAdmin, true)
	if err != nil {
		t.Fatalf("CountByRole: %v", err)
	}

	user.Role = models.RoleAdmin
	if err := repo.Update(ctx, &user); err != nil {
		t.Fatalf("Update: %v", err)
	}
	adminsAfter, err := repo.CountByRole(ctx, models.RoleAdmin, true)
	if err != nil {
		t.Fatalf("CountByRole: %v", err)
	}
	if adminsAfter != adminsBefore+1 {
		t.Errorf("active admins = %d, want %d", adminsAfter, adminsBefore+1)
	}

	user.Active = false
	if err := repo.Update(ctx, &user); err != nil {
		t.Fatalf("Update: %v", err)
	}
	stored, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if stored.Role != models.RoleAdmin || stored.Active {
		t.Errorf("update not persisted: %+v", stored)
	}
	adminsInactive, err := repo.CountByRole(ctx, models.RoleAdmin, true)
	if err != nil {
		t.Fatalf("CountByRole: %v", err)
	}
	if adminsInactive != adminsBefore {
		t.Errorf("active admins after deactivation = %d, want %d", adminsInactive, adminsBefore)
	}

	if err := repo.Delete(ctx, user.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := repo.Delete(ctx, user.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("Delete again: expected ErrNotFound, got %v", err)
	}
}

// TestAdminStatsRepositoryIntegration verifies the two-handle counters against
// real lab_source/lab_target databases.
func TestAdminStatsRepositoryIntegration(t *testing.T) {
	source := openSourceDB(t)
	target := openTargetDB(t)
	ctx := context.Background()

	repo := repository.NewAdminStatsRepository(source, target)
	counts, err := repo.Counts(ctx)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}

	if counts.Source.Customers < 0 || counts.Target.Customers < 0 {
		t.Fatal("counts must never be negative")
	}
	if counts.UsersTotal < 0 || counts.SchedulesTotal < 0 || counts.JobsTotal < 0 {
		t.Fatal("control counters must never be negative")
	}

	// The users total must match a direct count on the target handle.
	var wantUsers int64
	if err := target.WithContext(ctx).Raw(`SELECT COUNT(*) FROM users`).Scan(&wantUsers).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if counts.UsersTotal != wantUsers {
		t.Errorf("users total = %d, want %d", counts.UsersTotal, wantUsers)
	}

	// The per-role buckets must add up to the total.
	var byRole int64
	for _, n := range counts.UsersByRole {
		byRole += n
	}
	if byRole != counts.UsersTotal {
		t.Errorf("by_role sum = %d, want %d", byRole, counts.UsersTotal)
	}

	// The source handle must count lab_source, not lab_target.
	var wantSourceCustomers int64
	if err := source.WithContext(ctx).Raw(`SELECT COUNT(*) FROM customers`).Scan(&wantSourceCustomers).Error; err != nil {
		t.Fatalf("count source customers: %v", err)
	}
	if counts.Source.Customers != wantSourceCustomers {
		t.Errorf("source customers = %d, want %d", counts.Source.Customers, wantSourceCustomers)
	}
}
