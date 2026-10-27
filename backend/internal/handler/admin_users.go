package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/middleware"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// codeUnauthorized is the error code used when the authenticated caller cannot
// be resolved from the Gin context. The admin group is guard-only, so this is a
// defensive fail-closed branch rather than an expected response.
const codeUnauthorized = "unauthorized"

// AdminUsersHandler serves the admin user management endpoints. It only
// depends on the service.AdminUsers port, so tests use fakes.
type AdminUsersHandler struct {
	svc service.AdminUsers
}

// NewAdminUsersHandler builds an AdminUsersHandler over the given admin user
// service.
func NewAdminUsersHandler(svc service.AdminUsers) *AdminUsersHandler {
	return &AdminUsersHandler{svc: svc}
}

// AdminUserEnvelope wraps a single user in the standard success envelope.
type AdminUserEnvelope struct {
	Data models.User `json:"data"`
}

// AdminUserListResponse is the paginated envelope of the user listing.
type AdminUserListResponse struct {
	Data []models.User `json:"data"`
	Meta PageMeta      `json:"meta"`
}

// adminUserListQuery is the bound query string of GET /admin/users.
type adminUserListQuery struct {
	Pagination
	Query  string `form:"q"`
	Role   string `form:"role" binding:"omitempty,oneof=admin operator viewer"`
	Active *bool  `form:"active"`
}

// respondAdminUserError maps an admin user service error to the API error
// envelope: not-found → 404, conflicts and business guards → 409, invalid
// payloads → 400, anything else → 500.
func respondAdminUserError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		RespondNotFound(c, "user not found")
	case errors.Is(err, service.ErrConflict):
		RespondConflict(c, "email is already registered")
	case errors.Is(err, service.ErrSelfMutation):
		RespondConflict(c, "cannot remove your own access")
	case errors.Is(err, service.ErrLastAdmin):
		RespondConflict(c, "cannot remove the last active admin")
	case errors.Is(err, service.ErrNoChanges):
		RespondBadRequest(c, "no changes")
	case errors.Is(err, service.ErrInvalidInput):
		RespondError(c, http.StatusBadRequest, CodeValidation, err.Error())
	default:
		RespondInternalError(c, err)
	}
}

// adminActorID resolves the authenticated caller from the Gin context. The
// admin group is only registered with an auth guard, so a missing id means the
// request never went through that guard: fail closed with 401.
func adminActorID(c *gin.Context) (uuid.UUID, bool) {
	actorID, ok := middleware.CurrentUserID(c)
	if !ok {
		RespondError(c, http.StatusUnauthorized, codeUnauthorized, "unauthorized")
		return uuid.Nil, false
	}
	return actorID, true
}

// ListUsers handles GET /api/v1/admin/users.
//
//	@Summary		List application users
//	@Description	Returns a page of users (never the password hash). `q` matches the email case-insensitively, `role` filters by RBAC role and `active` by account state.
//	@Tags			admin
//	@Produce		json
//	@Param			page		query		int		false	"Page number (1-based)"		minimum(1)
//	@Param			per_page	query		int		false	"Items per page (max 100)"	minimum(1)	maximum(100)
//	@Param			q			query		string	false	"Email search (case-insensitive)"
//	@Param			role		query		string	false	"RBAC role"	Enums(admin, operator, viewer)
//	@Param			active		query		bool	false	"Account state"
//	@Success		200			{object}	AdminUserListResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/admin/users [get]
func (h *AdminUsersHandler) ListUsers(c *gin.Context) {
	var q adminUserListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		RespondValidationError(c, err)
		return
	}
	page := q.Pagination.Normalized()

	result, err := h.svc.List(c.Request.Context(), service.AdminUserListParams{
		Page:    page.Page,
		PerPage: page.PerPage,
		Search:  q.Query,
		Role:    q.Role,
		Active:  q.Active,
	})
	if err != nil {
		respondAdminUserError(c, err)
		return
	}

	users := result.Items
	if users == nil {
		users = []models.User{}
	}
	RespondPaginated(c, http.StatusOK, users, page, result.Total)
}

// GetUser handles GET /api/v1/admin/users/:id.
//
//	@Summary		Get an application user
//	@Description	Returns a single user by opaque id. The password hash is never serialized.
//	@Tags			admin
//	@Produce		json
//	@Param			id	path		string	true	"User id (opaque token)"
//	@Success		200	{object}	AdminUserEnvelope
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/admin/users/{id} [get]
func (h *AdminUsersHandler) GetUser(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	user, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondAdminUserError(c, err)
		return
	}
	RespondData(c, http.StatusOK, user)
}

// CreateUser handles POST /api/v1/admin/users.
//
//	@Summary		Create an application user
//	@Description	Creates a user with an Argon2id password hash. Email is normalized to lower-case and must be unique (409 on duplicates); password must be at least 8 characters and role must be admin, operator or viewer.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Param			request	body		service.AdminUserCreateInput	true	"User payload"
//	@Success		201		{object}	AdminUserEnvelope
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/admin/users [post]
func (h *AdminUsersHandler) CreateUser(c *gin.Context) {
	var in service.AdminUserCreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		RespondValidationError(c, err)
		return
	}

	user, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		respondAdminUserError(c, err)
		return
	}
	RespondData(c, http.StatusCreated, user)
}

// UpdateUser handles PUT /api/v1/admin/users/:id.
//
//	@Summary		Update an application user
//	@Description	Partially updates a user: every field is optional and nil means unchanged. A provided password is a reset (hashed with Argon2id). An empty payload is rejected with 400 "no changes". Admins can neither disable nor demote themselves (409), and the last active admin can be neither disabled nor demoted (409).
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string							true	"User id (opaque token)"
//	@Param			request	body		service.AdminUserUpdateInput	true	"User patch payload"
//	@Success		200		{object}	AdminUserEnvelope
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/admin/users/{id} [put]
func (h *AdminUsersHandler) UpdateUser(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	actorID, ok := adminActorID(c)
	if !ok {
		return
	}

	var in service.AdminUserUpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		RespondValidationError(c, err)
		return
	}

	user, err := h.svc.Update(c.Request.Context(), actorID, id, in)
	if err != nil {
		respondAdminUserError(c, err)
		return
	}
	RespondData(c, http.StatusOK, user)
}

// DeleteUser handles DELETE /api/v1/admin/users/:id.
//
//	@Summary		Delete an application user
//	@Description	Hard-deletes a user. An admin can neither delete themselves nor the last active admin (409).
//	@Tags			admin
//	@Param			id	path	string	true	"User id (opaque token)"
//	@Success		204
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		409	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/admin/users/{id} [delete]
func (h *AdminUsersHandler) DeleteUser(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	actorID, ok := adminActorID(c)
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), actorID, id); err != nil {
		respondAdminUserError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
