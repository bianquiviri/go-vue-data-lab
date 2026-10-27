package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// --- Test doubles -----------------------------------------------------------

// fakeAdminUserRepo is an in-memory repository.UserAdminRepository double. It
// emulates the unique email constraint so the service's ErrConflict mapping is
// exercised without a database.
type fakeAdminUserRepo struct {
	users map[uuid.UUID]*models.User

	listErr   error
	findErr   error
	createErr error
	updateErr error
	deleteErr error
	countErr  error

	gotOpts         repository.ListOptions
	gotFilter       repository.UserFilter
	created         *models.User
	updated         *models.User
	deleted         []uuid.UUID
	countRole       models.UserRole
	countActiveOnly bool
}

func newFakeAdminUserRepo(users ...*models.User) *fakeAdminUserRepo {
	r := &fakeAdminUserRepo{users: make(map[uuid.UUID]*models.User, len(users))}
	for _, u := range users {
		r.users[u.ID] = u
	}
	return r
}

func (r *fakeAdminUserRepo) List(_ context.Context, opts repository.ListOptions, f repository.UserFilter) ([]models.User, int64, error) {
	r.gotOpts, r.gotFilter = opts, f
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	rows := make([]models.User, 0, len(r.users))
	for _, u := range r.users {
		rows = append(rows, *u)
	}
	return rows, int64(len(rows)), nil
}

func (r *fakeAdminUserRepo) FindByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	u, ok := r.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	clone := *u
	return &clone, nil
}

func (r *fakeAdminUserRepo) Create(_ context.Context, user *models.User) error {
	if r.createErr != nil {
		return r.createErr
	}
	for _, existing := range r.users {
		if existing.Email == user.Email {
			return fmt.Errorf("fake: unique email: %w", repository.ErrConflict)
		}
	}
	clone := *user
	r.created = &clone
	r.users[user.ID] = user
	return nil
}

func (r *fakeAdminUserRepo) Update(_ context.Context, user *models.User) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	if _, ok := r.users[user.ID]; !ok {
		return fmt.Errorf("fake: update: %w", repository.ErrNotFound)
	}
	clone := *user
	r.updated = &clone
	r.users[user.ID] = user
	return nil
}

func (r *fakeAdminUserRepo) Delete(_ context.Context, id uuid.UUID) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if _, ok := r.users[id]; !ok {
		return fmt.Errorf("fake: delete: %w", repository.ErrNotFound)
	}
	r.deleted = append(r.deleted, id)
	delete(r.users, id)
	return nil
}

func (r *fakeAdminUserRepo) CountByRole(_ context.Context, role models.UserRole, activeOnly bool) (int64, error) {
	r.countRole, r.countActiveOnly = role, activeOnly
	if r.countErr != nil {
		return 0, r.countErr
	}
	var total int64
	for _, u := range r.users {
		if u.Role == role && (!activeOnly || u.Active) {
			total++
		}
	}
	return total, nil
}

var _ repository.UserAdminRepository = (*fakeAdminUserRepo)(nil)

func adminUsersSvc(repo *fakeAdminUserRepo) *service.AdminUsersService {
	return service.NewAdminUsersService(repo, fakeHasher{})
}

func newAdminUser(email string, role models.UserRole, active bool) *models.User {
	return &models.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: "hashed:current",
		Role:         role,
		Active:       active,
	}
}

func boolPtr(v bool) *bool       { return &v }
func stringPtr(v string) *string { return &v }

// --- Create ----------------------------------------------------------------

func TestAdminUsersCreateNormalizesAndHashes(t *testing.T) {
	repo := newFakeAdminUserRepo()
	svc := adminUsersSvc(repo)

	user, err := svc.Create(context.Background(), service.AdminUserCreateInput{
		Email:    "  Ada@Example.COM ",
		Password: "s3cret-pass",
		Role:     "ADMIN",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if user.Email != "ada@example.com" {
		t.Errorf("email = %q, want normalized lower-case", user.Email)
	}
	if user.Role != models.RoleAdmin {
		t.Errorf("role = %q, want %q", user.Role, models.RoleAdmin)
	}
	if !user.Active {
		t.Error("role defaults to active when omitted")
	}
	if user.PasswordHash != "hashed:s3cret-pass" {
		t.Errorf("password not hashed: %q", user.PasswordHash)
	}
	if user.ID == uuid.Nil {
		t.Error("expected a generated id")
	}
	if repo.created == nil || repo.created.Email != "ada@example.com" {
		t.Errorf("repository did not receive the user: %+v", repo.created)
	}
}

func TestAdminUsersCreateHonorsActiveFalse(t *testing.T) {
	repo := newFakeAdminUserRepo()
	svc := adminUsersSvc(repo)

	user, err := svc.Create(context.Background(), service.AdminUserCreateInput{
		Email:    "off@example.com",
		Password: "s3cret-pass",
		Role:     "viewer",
		Active:   boolPtr(false),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if user.Active {
		t.Error("active=false must be persisted")
	}
}

func TestAdminUsersCreateRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		in   service.AdminUserCreateInput
	}{
		{"invalid email", service.AdminUserCreateInput{Email: "not-an-email", Password: "longenough", Role: "admin"}},
		{"short password", service.AdminUserCreateInput{Email: "ada@example.com", Password: "short", Role: "admin"}},
		{"invalid role", service.AdminUserCreateInput{Email: "ada@example.com", Password: "longenough", Role: "root"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeAdminUserRepo()
			if _, err := adminUsersSvc(repo).Create(context.Background(), tc.in); !errors.Is(err, service.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
			if repo.created != nil {
				t.Error("repository must not be called on invalid input")
			}
		})
	}
}

func TestAdminUsersCreateDuplicateEmailConflicts(t *testing.T) {
	repo := newFakeAdminUserRepo(newAdminUser("ada@example.com", models.RoleViewer, true))
	svc := adminUsersSvc(repo)

	_, err := svc.Create(context.Background(), service.AdminUserCreateInput{
		Email:    "ADA@example.com",
		Password: "s3cret-pass",
		Role:     "viewer",
	})
	if !errors.Is(err, service.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestAdminUsersCreateWrapsHashFailure(t *testing.T) {
	repo := newFakeAdminUserRepo()
	svc := service.NewAdminUsersService(repo, fakeHasher{hashErr: errors.New("argon boom")})

	_, err := svc.Create(context.Background(), service.AdminUserCreateInput{
		Email:    "ada@example.com",
		Password: "s3cret-pass",
		Role:     "viewer",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, service.ErrInvalidInput) || errors.Is(err, service.ErrConflict) {
		t.Fatalf("hash failure must not be mapped to a business sentinel: %v", err)
	}
}

// --- List / Get ------------------------------------------------------------

func TestAdminUsersListPassesFiltersAndPaging(t *testing.T) {
	repo := newFakeAdminUserRepo(newAdminUser("ada@example.com", models.RoleAdmin, true))
	svc := adminUsersSvc(repo)

	active := true
	result, err := svc.List(context.Background(), service.AdminUserListParams{
		Page: 2, PerPage: 5, Search: " ada ", Role: "ADMIN", Active: &active,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if result.Page != 2 || result.PerPage != 5 {
		t.Errorf("paging = %d/%d, want 2/5", result.Page, result.PerPage)
	}
	if repo.gotOpts.Limit != 5 || repo.gotOpts.Offset != 5 {
		t.Errorf("repo paging = %+v, want limit 5 offset 5", repo.gotOpts)
	}
	if repo.gotFilter.Search != " ada " {
		t.Errorf("search = %q", repo.gotFilter.Search)
	}
	if repo.gotFilter.Role != models.RoleAdmin {
		t.Errorf("role = %q, want admin", repo.gotFilter.Role)
	}
	if repo.gotFilter.Active == nil || !*repo.gotFilter.Active {
		t.Error("active filter not forwarded")
	}
}

func TestAdminUsersListDefaultsPaging(t *testing.T) {
	repo := newFakeAdminUserRepo()
	result, err := adminUsersSvc(repo).List(context.Background(), service.AdminUserListParams{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if result.Page != 1 || result.PerPage != 20 {
		t.Errorf("paging = %d/%d, want defaults 1/20", result.Page, result.PerPage)
	}
	if repo.gotFilter.Role != "" || repo.gotFilter.Active != nil {
		t.Errorf("unexpected filter: %+v", repo.gotFilter)
	}
}

func TestAdminUsersListRejectsInvalidRole(t *testing.T) {
	repo := newFakeAdminUserRepo()
	_, err := adminUsersSvc(repo).List(context.Background(), service.AdminUserListParams{Role: "root"})
	if !errors.Is(err, service.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestAdminUsersListWrapsRepositoryError(t *testing.T) {
	repo := newFakeAdminUserRepo()
	repo.listErr = errors.New("db down")
	if _, err := adminUsersSvc(repo).List(context.Background(), service.AdminUserListParams{}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestAdminUsersGet(t *testing.T) {
	user := newAdminUser("ada@example.com", models.RoleAdmin, true)
	svc := adminUsersSvc(newFakeAdminUserRepo(user))

	got, err := svc.Get(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != user.ID || got.Email != user.Email {
		t.Errorf("got %+v, want %+v", got, user)
	}

	if _, err := svc.Get(context.Background(), uuid.New()); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- Update ----------------------------------------------------------------

func TestAdminUsersUpdateRejectsEmptyPayload(t *testing.T) {
	user := newAdminUser("ada@example.com", models.RoleViewer, true)
	repo := newFakeAdminUserRepo(user)

	_, err := adminUsersSvc(repo).Update(context.Background(), uuid.New(), user.ID, service.AdminUserUpdateInput{})
	if !errors.Is(err, service.ErrNoChanges) {
		t.Fatalf("expected ErrNoChanges, got %v", err)
	}
	if repo.updated != nil {
		t.Error("repository must not be called with no changes")
	}
}

func TestAdminUsersUpdateAppliesPartialFields(t *testing.T) {
	user := newAdminUser("ada@example.com", models.RoleViewer, true)
	repo := newFakeAdminUserRepo(user)

	got, err := adminUsersSvc(repo).Update(context.Background(), uuid.New(), user.ID, service.AdminUserUpdateInput{
		Email:    stringPtr("  Grace@Example.COM "),
		Role:     stringPtr("operator"),
		Active:   boolPtr(false),
		Password: stringPtr("brand-new-pass"),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Email != "grace@example.com" {
		t.Errorf("email = %q", got.Email)
	}
	if got.Role != models.RoleOperator {
		t.Errorf("role = %q", got.Role)
	}
	if got.Active {
		t.Error("active should be false")
	}
	if got.PasswordHash != "hashed:brand-new-pass" {
		t.Errorf("password not re-hashed: %q", got.PasswordHash)
	}
	if repo.updated == nil || repo.updated.Email != "grace@example.com" {
		t.Errorf("repository did not receive the update: %+v", repo.updated)
	}
}

func TestAdminUsersUpdateKeepsUnsetFields(t *testing.T) {
	user := newAdminUser("ada@example.com", models.RoleViewer, true)
	repo := newFakeAdminUserRepo(user)

	got, err := adminUsersSvc(repo).Update(context.Background(), uuid.New(), user.ID, service.AdminUserUpdateInput{
		Password: stringPtr("brand-new-pass"),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Email != "ada@example.com" || got.Role != models.RoleViewer || !got.Active {
		t.Errorf("unset fields must be preserved: %+v", got)
	}
	if got.PasswordHash != "hashed:brand-new-pass" {
		t.Errorf("password not reset: %q", got.PasswordHash)
	}
}

func TestAdminUsersUpdateSelfGuards(t *testing.T) {
	cases := []struct {
		name string
		in   service.AdminUserUpdateInput
	}{
		{"self-deactivation", service.AdminUserUpdateInput{Active: boolPtr(false)}},
		{"self-demotion", service.AdminUserUpdateInput{Role: stringPtr("operator")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := newAdminUser("root@example.com", models.RoleAdmin, true)
			repo := newFakeAdminUserRepo(user)

			_, err := adminUsersSvc(repo).Update(context.Background(), user.ID, user.ID, tc.in)
			if !errors.Is(err, service.ErrSelfMutation) {
				t.Fatalf("expected ErrSelfMutation, got %v", err)
			}
			if repo.updated != nil {
				t.Error("repository must not be called on a self mutation")
			}
		})
	}
}

func TestAdminUsersUpdateSelfKeepingAdminIsAllowed(t *testing.T) {
	user := newAdminUser("root@example.com", models.RoleAdmin, true)
	repo := newFakeAdminUserRepo(user)

	got, err := adminUsersSvc(repo).Update(context.Background(), user.ID, user.ID, service.AdminUserUpdateInput{
		Email:  stringPtr("root2@example.com"),
		Role:   stringPtr("admin"),
		Active: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Email != "root2@example.com" || got.Role != models.RoleAdmin {
		t.Errorf("unexpected user: %+v", got)
	}
}

func TestAdminUsersUpdateBlocksLastActiveAdmin(t *testing.T) {
	cases := []struct {
		name string
		in   service.AdminUserUpdateInput
	}{
		{"deactivate", service.AdminUserUpdateInput{Active: boolPtr(false)}},
		{"demote", service.AdminUserUpdateInput{Role: stringPtr("viewer")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			admin := newAdminUser("root@example.com", models.RoleAdmin, true)
			repo := newFakeAdminUserRepo(admin)

			_, err := adminUsersSvc(repo).Update(context.Background(), uuid.New(), admin.ID, tc.in)
			if !errors.Is(err, service.ErrLastAdmin) {
				t.Fatalf("expected ErrLastAdmin, got %v", err)
			}
			if repo.updated != nil {
				t.Error("repository must not be called when the last admin guard trips")
			}
		})
	}
}

func TestAdminUsersUpdateAllowsDemotionWithAnotherAdmin(t *testing.T) {
	admin := newAdminUser("root@example.com", models.RoleAdmin, true)
	other := newAdminUser("second@example.com", models.RoleAdmin, true)
	repo := newFakeAdminUserRepo(admin, other)

	got, err := adminUsersSvc(repo).Update(context.Background(), uuid.New(), admin.ID, service.AdminUserUpdateInput{
		Role: stringPtr("viewer"),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Role != models.RoleViewer {
		t.Errorf("role = %q, want viewer", got.Role)
	}
	if repo.countRole != models.RoleAdmin || !repo.countActiveOnly {
		t.Errorf("guard counted role=%q activeOnly=%v", repo.countRole, repo.countActiveOnly)
	}
}

func TestAdminUsersUpdateInactiveAdminNeedsNoGuard(t *testing.T) {
	inactive := newAdminUser("old@example.com", models.RoleAdmin, false)
	active := newAdminUser("root@example.com", models.RoleAdmin, true)
	repo := newFakeAdminUserRepo(inactive, active)

	if _, err := adminUsersSvc(repo).Update(context.Background(), uuid.New(), inactive.ID, service.AdminUserUpdateInput{
		Role: stringPtr("viewer"),
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.countRole != "" {
		t.Error("guard must not run for an already inactive admin")
	}
}

func TestAdminUsersUpdateValidatesFieldsBeforeGuards(t *testing.T) {
	user := newAdminUser("ada@example.com", models.RoleViewer, true)
	repo := newFakeAdminUserRepo(user)
	svc := adminUsersSvc(repo)

	cases := []struct {
		name string
		in   service.AdminUserUpdateInput
	}{
		{"invalid email", service.AdminUserUpdateInput{Email: stringPtr("nope")}},
		{"invalid role", service.AdminUserUpdateInput{Role: stringPtr("root")}},
		{"short password", service.AdminUserUpdateInput{Password: stringPtr("short")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Update(context.Background(), uuid.New(), user.ID, tc.in); !errors.Is(err, service.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestAdminUsersUpdateNotFound(t *testing.T) {
	repo := newFakeAdminUserRepo()
	_, err := adminUsersSvc(repo).Update(context.Background(), uuid.New(), uuid.New(), service.AdminUserUpdateInput{
		Role: stringPtr("viewer"),
	})
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAdminUsersUpdateMapsUniqueConflict(t *testing.T) {
	user := newAdminUser("ada@example.com", models.RoleViewer, true)
	repo := newFakeAdminUserRepo(user)
	repo.updateErr = fmt.Errorf("fake: unique: %w", repository.ErrConflict)

	_, err := adminUsersSvc(repo).Update(context.Background(), uuid.New(), user.ID, service.AdminUserUpdateInput{
		Email: stringPtr("taken@example.com"),
	})
	if !errors.Is(err, service.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestAdminUsersUpdateWrapsGuardCountError(t *testing.T) {
	admin := newAdminUser("root@example.com", models.RoleAdmin, true)
	repo := newFakeAdminUserRepo(admin)
	repo.countErr = errors.New("db down")

	_, err := adminUsersSvc(repo).Update(context.Background(), uuid.New(), admin.ID, service.AdminUserUpdateInput{
		Active: boolPtr(false),
	})
	if err == nil || errors.Is(err, service.ErrLastAdmin) {
		t.Fatalf("expected a wrapped repository error, got %v", err)
	}
}

// --- Delete ----------------------------------------------------------------

func TestAdminUsersDeleteRejectsSelf(t *testing.T) {
	user := newAdminUser("root@example.com", models.RoleAdmin, true)
	repo := newFakeAdminUserRepo(user)

	err := adminUsersSvc(repo).Delete(context.Background(), user.ID, user.ID)
	if !errors.Is(err, service.ErrSelfMutation) {
		t.Fatalf("expected ErrSelfMutation, got %v", err)
	}
	if len(repo.deleted) != 0 {
		t.Error("repository must not delete on a self mutation")
	}
}

func TestAdminUsersDeleteBlocksLastActiveAdmin(t *testing.T) {
	admin := newAdminUser("root@example.com", models.RoleAdmin, true)
	repo := newFakeAdminUserRepo(admin)

	err := adminUsersSvc(repo).Delete(context.Background(), uuid.New(), admin.ID)
	if !errors.Is(err, service.ErrLastAdmin) {
		t.Fatalf("expected ErrLastAdmin, got %v", err)
	}
	if len(repo.deleted) != 0 {
		t.Error("the last admin must not be deleted")
	}
}

func TestAdminUsersDeleteSucceedsWithAnotherAdmin(t *testing.T) {
	admin := newAdminUser("root@example.com", models.RoleAdmin, true)
	other := newAdminUser("second@example.com", models.RoleAdmin, true)
	repo := newFakeAdminUserRepo(admin, other)

	if err := adminUsersSvc(repo).Delete(context.Background(), admin.ID, other.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != other.ID {
		t.Errorf("deleted = %v, want [%s]", repo.deleted, other.ID)
	}
}

func TestAdminUsersDeleteInactiveAdminNeedsNoGuard(t *testing.T) {
	inactive := newAdminUser("old@example.com", models.RoleAdmin, false)
	repo := newFakeAdminUserRepo(inactive)

	if err := adminUsersSvc(repo).Delete(context.Background(), uuid.New(), inactive.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.countRole != "" {
		t.Error("guard must not run for an already inactive admin")
	}
}

func TestAdminUsersDeleteNotFound(t *testing.T) {
	repo := newFakeAdminUserRepo()
	if err := adminUsersSvc(repo).Delete(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAdminUsersDeleteWrapsRepositoryError(t *testing.T) {
	user := newAdminUser("ada@example.com", models.RoleViewer, true)
	repo := newFakeAdminUserRepo(user)
	repo.deleteErr = errors.New("db down")

	if err := adminUsersSvc(repo).Delete(context.Background(), uuid.New(), user.ID); err == nil {
		t.Fatal("expected an error")
	}
}

func TestValidRole(t *testing.T) {
	cases := map[string]bool{
		"admin": true, "operator": true, "viewer": true,
		" ADMIN ": true, "root": false, "": false,
	}
	for role, want := range cases {
		if got := service.ValidRole(role); got != want {
			t.Errorf("ValidRole(%q) = %v, want %v", role, got, want)
		}
	}
}

// --- JSON decoding ----------------------------------------------------------

// The Gin binding rules validate the raw string, so the payloads trim the email
// on decode: a valid address never carries surrounding whitespace and the
// documented normalization (lower/trim) must not depend on the caller.
func TestAdminUserCreateInputUnmarshalTrimsEmail(t *testing.T) {
	var in service.AdminUserCreateInput
	raw := []byte(`{"email":"  Ada@Example.com  ","password":"s3cret-pass","role":"viewer","active":false}`)
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.Email != "Ada@Example.com" {
		t.Errorf("email = %q, want trimmed %q", in.Email, "Ada@Example.com")
	}
	if in.Password != "s3cret-pass" || in.Role != "viewer" {
		t.Errorf("other fields lost: %+v", in)
	}
	if in.Active == nil || *in.Active {
		t.Errorf("active = %v, want explicit false", in.Active)
	}
}

func TestAdminUserCreateInputUnmarshalRejectsInvalidJSON(t *testing.T) {
	var in service.AdminUserCreateInput
	if err := json.Unmarshal([]byte(`{"email":`), &in); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

func TestAdminUserUpdateInputUnmarshalTrimsEmail(t *testing.T) {
	var in service.AdminUserUpdateInput
	raw := []byte(`{"email":"  New@Example.com  ","active":true}`)
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.Email == nil || *in.Email != "New@Example.com" {
		t.Errorf("email = %v, want trimmed %q", in.Email, "New@Example.com")
	}
	if in.Password != nil || in.Role != nil {
		t.Errorf("unset fields must stay nil: %+v", in)
	}
	if in.Active == nil || !*in.Active {
		t.Errorf("active = %v, want true", in.Active)
	}
}

func TestAdminUserUpdateInputUnmarshalKeepsAbsentAndNullEmail(t *testing.T) {
	for name, raw := range map[string]string{
		"absent": `{"role":"viewer"}`,
		"null":   `{"email":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			var in service.AdminUserUpdateInput
			if err := json.Unmarshal([]byte(raw), &in); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if in.Email != nil {
				t.Errorf("email = %v, want nil", *in.Email)
			}
		})
	}
}

func TestAdminUserUpdateInputUnmarshalRejectsInvalidJSON(t *testing.T) {
	var in service.AdminUserUpdateInput
	if err := json.Unmarshal([]byte(`{"email":`), &in); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}
