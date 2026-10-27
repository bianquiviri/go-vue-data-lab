package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// AdminStatsHandler serves the admin dashboard counters endpoint. It only
// depends on the service.AdminStats port, so tests use fakes.
type AdminStatsHandler struct {
	svc service.AdminStats
}

// NewAdminStatsHandler builds an AdminStatsHandler over the given stats
// service.
func NewAdminStatsHandler(svc service.AdminStats) *AdminStatsHandler {
	return &AdminStatsHandler{svc: svc}
}

// AdminStatsEnvelope wraps the dashboard snapshot in the standard success
// envelope.
type AdminStatsEnvelope struct {
	Data service.AdminStatsSnapshot `json:"data"`
}

// Stats handles GET /api/v1/admin/stats.
//
//	@Summary		Admin dashboard counters
//	@Description	Row counts of both lab schemas, user/job/schedule aggregates and process metadata. Empty tables report 0, never null.
//	@Tags			admin
//	@Produce		json
//	@Success		200	{object}	AdminStatsEnvelope
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/admin/stats [get]
func (h *AdminStatsHandler) Stats(c *gin.Context) {
	snapshot, err := h.svc.Stats(c.Request.Context())
	if err != nil {
		RespondInternalError(c, err)
		return
	}
	RespondData(c, http.StatusOK, snapshot)
}
