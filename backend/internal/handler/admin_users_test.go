package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/handler"
	"github.com/josebianco/go-vue-data-lab/internal/middleware"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// fakeAdminUsers is a controllable service.AdminUsers double.
type fakeAdminUsers struct {
	listResult service.PageResult[models.User]
	listErr    error
	listParams service.AdminUserListParams

	getUser models.User
	getID   uuid.UUID
	getErr  error

	createUser models.User
	createIn   service.AdminUserCreateInput
	createErr  error

	updateUser  models.User
	updateActor uuid.UUID
	updateID    uuid.UUID
	updateIn    service.AdminUserUpdateInput
	updateErr   error

	deleteActor uuid.UUID
	deleteID    uuid.UUID
	deleteErr   error
	deleteCalls int
}

func (f *fakeAdminUsers) List(_ context.Context, p service.AdminUserListParams) (service.PageResult[models.User], error) {
	f.listParams = p
	if f.listErr != nil {
		return service.PageResult[models.User]{}, f.listErr
	}
	return f.listResult, nil
}

func (f *fakeAdminUsers) Get(_ context.Context, id uuid.UUID) (models.User, error) {
	f.getID = id
	if f.getErr != nil {
		return models.User{}, f.getErr
	}
	return f.getUser, nil
}

func (f *fakeAdminUsers) Create(_ context.Context, in service.AdminUserCreateInput) (models.User, error) {
	f.createIn = in
	if f.createErr != nil {
		return models.User{}, f.createErr
	}
	return f.createUser, nil
}

func (f *fakeAdminUsers) Update(_ context.Context, actorID, id uuid.UUID, in service.AdminUserUpdateInput) (models.User, error) {
	f.updateActor, f.updateID, f.updateIn = actorID, id, in
	if f.updateErr != nil {
		return models.User{}, f.updateErr
	}
	return f.updateUser, nil
}

func (f *fakeAdminUsers) Delete(_ context.Context, actorID, id uuid.UUID) error {
	f.deleteActor, f.deleteID = actorID, id
	f.deleteCalls++
	return f.deleteErr
}

var _ service.AdminUsers = (*fakeAdminUsers)(nil)

// adminUsersEngine mounts the admin user routes behind a fake auth guard that
// stores actor in the Gin context (the real guard is middleware.AuthRequired).
func adminUsersEngine(svc service.AdminUsers, actor uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	admin := engine.Group("/api/v1/admin")
	admin.Use(func(c *gin.Context) {
		if actor != uuid.Nil {
			c.Set(middleware.ContextUserID, actor.String())
		}
		c.Next()
	})

	h := handler.NewAdminUsersHandler(svc)
	admin.GET("/users", h.ListUsers)
	admin.GET("/users/:id", h.GetUser)
	admin.POST("/users", h.CreateUser)
	admin.PUT("/users/:id", h.UpdateUser)
	admin.DELETE("/users/:id", h.DeleteUser)
	return engine
}

// adminRequest performs an HTTP request against engine, JSON-encoding body when
// it is not nil.
func adminRequest(t *testing.T, engine http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var payload []byte
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		payload = raw
	}

	req := httptest.NewRequest(method, target, bytes.NewReader(payload))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// --- List -------------------------------------------------------------------

func TestAdminListUsersReturnsPaginatedEnvelope(t *testing.T) {
	id := uuid.New()
	svc := &fakeAdminUsers{listResult: service.PageResult[models.User]{
		Items: []models.User{{
			ID: id, Email: "ada@example.com", PasswordHash: "super-secret-hash",
			Role: models.RoleAdmin, Active: true,
		}},
		Total: 41,
	}}

	rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodGet,
		"/api/v1/admin/users?page=2&per_page=10&q=ada&role=admin&active=true", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	if svc.listParams.Page != 2 || svc.listParams.PerPage != 10 {
		t.Errorf("paging = %d/%d, want 2/10", svc.listParams.Page, svc.listParams.PerPage)
	}
	if svc.listParams.Search != "ada" || svc.listParams.Role != "admin" {
		t.Errorf("filters = %+v", svc.listParams)
	}
	if svc.listParams.Active == nil || !*svc.listParams.Active {
		t.Error("active filter not forwarded")
	}

	var body handler.PaginatedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (body: %s)", err, rec.Body.String())
	}
	if body.Meta.Page != 2 || body.Meta.PerPage != 10 || body.Meta.Total != 41 || body.Meta.TotalPages != 5 {
		t.Errorf("unexpected meta: %+v", body.Meta)
	}

	items, ok := body.Data.([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 user, got %#v", body.Data)
	}
	user, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected object, got %#v", items[0])
	}
	if user["id"] != id.String() || user["email"] != "ada@example.com" {
		t.Errorf("unexpected user payload: %+v", user)
	}
	if _, leaked := user["password_hash"]; leaked {
		t.Error("password hash must never be serialized")
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("super-secret-hash")) {
		t.Error("password hash leaked into the response body")
	}
}

func TestAdminListUsersEmptyIsArrayNotNull(t *testing.T) {
	svc := &fakeAdminUsers{}
	rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodGet, "/api/v1/admin/users", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body struct {
		Data []models.User `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data == nil {
		t.Error("data must be an empty array, not null")
	}
}

func TestAdminListUsersRejectsInvalidQuery(t *testing.T) {
	cases := []string{
		"/api/v1/admin/users?role=root",
		"/api/v1/admin/users?active=maybe",
		"/api/v1/admin/users?per_page=500",
	}
	for _, target := range cases {
		t.Run(target, func(t *testing.T) {
			svc := &fakeAdminUsers{}
			rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodGet, target, nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d (body: %s)", rec.Code, rec.Body.String())
			}
			if got := decodeError(t, rec).Error.Code; got != handler.CodeValidation {
				t.Errorf("code = %v, want %s", got, handler.CodeValidation)
			}
			if svc.listParams != (service.AdminUserListParams{}) {
				t.Error("service must not be called on a validation failure")
			}
		})
	}
}

func TestAdminListUsersMapsServiceErrorTo500(t *testing.T) {
	svc := &fakeAdminUsers{listErr: context.Canceled}
	rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodGet, "/api/v1/admin/users", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

// --- Get --------------------------------------------------------------------

func TestAdminGetUser(t *testing.T) {
	id := uuid.New()
	svc := &fakeAdminUsers{getUser: models.User{ID: id, Email: "ada@example.com", Role: models.RoleOperator}}

	rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodGet,
		"/api/v1/admin/users/"+id.String(), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if svc.getID != id {
		t.Errorf("service got id %s, want %s", svc.getID, id)
	}

	var body handler.AdminUserEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Email != "ada@example.com" {
		t.Errorf("user = %+v", body.Data)
	}
}

func TestAdminGetUserRejectsInvalidID(t *testing.T) {
	svc := &fakeAdminUsers{}
	rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodGet, "/api/v1/admin/users/not-an-id", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	details := decodeError(t, rec).Error.Details
	fields, _ := details["fields"].(map[string]any)
	if _, ok := fields["id"]; !ok {
		t.Errorf("expected fields.id, got %#v", details)
	}
	if svc.getID != uuid.Nil {
		t.Error("service must not be called for an invalid id")
	}
}

func TestAdminGetUserNotFound(t *testing.T) {
	svc := &fakeAdminUsers{getErr: service.ErrNotFound}
	rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodGet,
		"/api/v1/admin/users/"+uuid.NewString(), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// --- Create -----------------------------------------------------------------

func TestAdminCreateUser(t *testing.T) {
	svc := &fakeAdminUsers{createUser: models.User{ID: uuid.New(), Email: "ada@example.com", Role: models.RoleOperator}}

	rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodPost, "/api/v1/admin/users",
		map[string]any{"email": "ada@example.com", "password": "s3cret-pass", "role": "operator"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if svc.createIn.Email != "ada@example.com" || svc.createIn.Password != "s3cret-pass" || svc.createIn.Role != "operator" {
		t.Errorf("service input = %+v", svc.createIn)
	}
}

func TestAdminCreateUserTrimsEmailWhitespace(t *testing.T) {
	svc := &fakeAdminUsers{createUser: models.User{ID: uuid.New(), Email: "ada@example.com", Role: models.RoleOperator}}

	rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodPost, "/api/v1/admin/users",
		map[string]any{"email": "  Ada@Example.com  ", "password": "s3cret-pass", "role": "operator"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	// The handler trims, the service lower-cases: the contract is
	// "normalized lower/trim" without a round-trip of 400s.
	if svc.createIn.Email != "Ada@Example.com" {
		t.Errorf("forwarded email = %q, want trimmed %q", svc.createIn.Email, "Ada@Example.com")
	}
}

func TestAdminCreateUserValidationFailure(t *testing.T) {
	cases := map[string]map[string]any{
		"missing email":  {"password": "s3cret-pass", "role": "admin"},
		"invalid email":  {"email": "nope", "password": "s3cret-pass", "role": "admin"},
		"short password": {"email": "ada@example.com", "password": "short", "role": "admin"},
		"invalid role":   {"email": "ada@example.com", "password": "s3cret-pass", "role": "root"},
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			svc := &fakeAdminUsers{}
			rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodPost, "/api/v1/admin/users", payload)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d (body: %s)", rec.Code, rec.Body.String())
			}
			if svc.createIn.Email != "" {
				t.Error("service must not be called on a validation failure")
			}
		})
	}
}

func TestAdminCreateUserDuplicateEmailConflicts(t *testing.T) {
	svc := &fakeAdminUsers{createErr: service.ErrConflict}
	rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodPost, "/api/v1/admin/users",
		map[string]any{"email": "ada@example.com", "password": "s3cret-pass", "role": "viewer"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if got := decodeError(t, rec).Error.Code; got != handler.CodeConflict {
		t.Errorf("code = %v, want %s", got, handler.CodeConflict)
	}
}

// --- Update -----------------------------------------------------------------

func TestAdminUpdateUserForwardsActor(t *testing.T) {
	actor := uuid.New()
	id := uuid.New()
	svc := &fakeAdminUsers{updateUser: models.User{ID: id, Email: "new@example.com", Role: models.RoleAdmin}}

	rec := adminRequest(t, adminUsersEngine(svc, actor), http.MethodPut, "/api/v1/admin/users/"+id.String(),
		map[string]any{"email": "new@example.com", "active": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if svc.updateActor != actor || svc.updateID != id {
		t.Errorf("service got actor=%s id=%s", svc.updateActor, svc.updateID)
	}
	if svc.updateIn.Email == nil || *svc.updateIn.Email != "new@example.com" {
		t.Errorf("service input = %+v", svc.updateIn)
	}
	if svc.updateIn.Active == nil || !*svc.updateIn.Active {
		t.Errorf("active not forwarded: %+v", svc.updateIn)
	}
	if svc.updateIn.Role != nil || svc.updateIn.Password != nil {
		t.Errorf("unset fields must stay nil: %+v", svc.updateIn)
	}
}

func TestAdminUpdateUserTrimsEmailWhitespace(t *testing.T) {
	id := uuid.New()
	svc := &fakeAdminUsers{updateUser: models.User{ID: id, Email: "new@example.com", Role: models.RoleAdmin}}

	rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodPut, "/api/v1/admin/users/"+id.String(),
		map[string]any{"email": "  New@Example.com  "})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if svc.updateIn.Email == nil || *svc.updateIn.Email != "New@Example.com" {
		t.Errorf("forwarded email = %v", svc.updateIn.Email)
	}
}

func TestAdminUpdateUserErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"not found", service.ErrNotFound, http.StatusNotFound},
		{"conflict", service.ErrConflict, http.StatusConflict},
		{"self mutation", service.ErrSelfMutation, http.StatusConflict},
		{"last admin", service.ErrLastAdmin, http.StatusConflict},
		{"no changes", service.ErrNoChanges, http.StatusBadRequest},
		{"invalid input", service.ErrInvalidInput, http.StatusBadRequest},
		{"internal", context.Canceled, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeAdminUsers{updateErr: tc.err}
			rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodPut,
				"/api/v1/admin/users/"+uuid.NewString(), map[string]any{"active": false})
			if rec.Code != tc.want {
				t.Fatalf("expected %d, got %d (body: %s)", tc.want, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAdminUpdateUserNoChangesMessage(t *testing.T) {
	svc := &fakeAdminUsers{updateErr: service.ErrNoChanges}
	rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodPut,
		"/api/v1/admin/users/"+uuid.NewString(), map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if got := decodeError(t, rec).Error.Message; got != "no changes" {
		t.Errorf("message = %v, want %q", got, "no changes")
	}
}

func TestAdminUpdateUserRejectsInvalidBody(t *testing.T) {
	svc := &fakeAdminUsers{}
	rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodPut,
		"/api/v1/admin/users/"+uuid.NewString(), map[string]any{"password": "short"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if svc.updateID != uuid.Nil {
		t.Error("service must not be called on a validation failure")
	}
}

func TestAdminUpdateUserRequiresActor(t *testing.T) {
	svc := &fakeAdminUsers{}
	rec := adminRequest(t, adminUsersEngine(svc, uuid.Nil), http.MethodPut,
		"/api/v1/admin/users/"+uuid.NewString(), map[string]any{"active": true})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if svc.updateID != uuid.Nil {
		t.Error("service must not be called without an authenticated actor")
	}
}

func TestAdminUpdateUserRejectsInvalidID(t *testing.T) {
	svc := &fakeAdminUsers{}
	rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodPut,
		"/api/v1/admin/users/nope", map[string]any{"active": true})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// --- Delete -----------------------------------------------------------------

func TestAdminDeleteUser(t *testing.T) {
	actor := uuid.New()
	id := uuid.New()
	svc := &fakeAdminUsers{}

	rec := adminRequest(t, adminUsersEngine(svc, actor), http.MethodDelete, "/api/v1/admin/users/"+id.String(), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 must have an empty body, got %q", rec.Body.String())
	}
	if svc.deleteActor != actor || svc.deleteID != id {
		t.Errorf("service got actor=%s id=%s", svc.deleteActor, svc.deleteID)
	}
}

func TestAdminDeleteUserErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"not found", service.ErrNotFound, http.StatusNotFound},
		{"self mutation", service.ErrSelfMutation, http.StatusConflict},
		{"last admin", service.ErrLastAdmin, http.StatusConflict},
		{"internal", context.DeadlineExceeded, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeAdminUsers{deleteErr: tc.err}
			rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodDelete,
				"/api/v1/admin/users/"+uuid.NewString(), nil)
			if rec.Code != tc.want {
				t.Fatalf("expected %d, got %d (body: %s)", tc.want, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAdminDeleteUserRequiresActor(t *testing.T) {
	svc := &fakeAdminUsers{}
	rec := adminRequest(t, adminUsersEngine(svc, uuid.Nil), http.MethodDelete,
		"/api/v1/admin/users/"+uuid.NewString(), nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if svc.deleteCalls != 0 {
		t.Error("service must not be called without an authenticated actor")
	}
}

func TestAdminDeleteUserRejectsInvalidID(t *testing.T) {
	svc := &fakeAdminUsers{}
	rec := adminRequest(t, adminUsersEngine(svc, uuid.New()), http.MethodDelete, "/api/v1/admin/users/raw-uuid", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if svc.deleteCalls != 0 {
		t.Error("service must not be called for an invalid id")
	}
}
