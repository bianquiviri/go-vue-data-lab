package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// AdminHandler exposes the admin-only operational endpoints.
type AdminHandler struct {
	logs service.ErrorLogs
}

// NewAdminHandler builds the admin handler over a log read port.
func NewAdminHandler(logs service.ErrorLogs) *AdminHandler {
	return &AdminHandler{logs: logs}
}

// Errors handles GET /api/v1/admin/logs/errors.
//
//	@Summary		List recent application error logs
//	@Description	Error-level records from the rotating JSON log (ERROR/FATAL/PANIC), newest first. The file tail is scanned so the cost stays bounded on large logs.
//	@Tags			admin
//	@Produce		json
//	@Param			page		query		int		false	"Page number (1-based)"	minimum(1)
//	@Param			per_page	query		int		false	"Items per page (max 100)"	minimum(1)	maximum(100)
//	@Success		200			{object}	PaginatedResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/admin/logs/errors [get]
func (h *AdminHandler) Errors(c *gin.Context) {
	var p Pagination
	if err := c.ShouldBindQuery(&p); err != nil {
		RespondValidationError(c, err)
		return
	}
	page := p.Normalized()

	entries, total, err := h.logs.RecentErrors(c.Request.Context(), page.PerPage, page.Offset())
	if err != nil {
		RespondInternalError(c, err)
		return
	}
	if entries == nil {
		entries = []service.LogEntry{}
	}

	RespondPaginated(c, http.StatusOK, entries, page, int64(total))
}
