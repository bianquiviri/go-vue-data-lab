package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/scheduler"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// SchedulerHandler serves the cron schedule endpoints. It only depends on the
// service.SchedulerControl port, so tests use fakes.
type SchedulerHandler struct {
	svc service.SchedulerControl
}

// NewSchedulerHandler builds a SchedulerHandler over the given scheduler service.
func NewSchedulerHandler(svc service.SchedulerControl) *SchedulerHandler {
	return &SchedulerHandler{svc: svc}
}

// ScheduleEnvelope wraps a single schedule in the standard success envelope.
type ScheduleEnvelope struct {
	Data models.CronSchedule `json:"data"`
}

// ScheduleListEnvelope wraps a schedule list in the standard success envelope.
type ScheduleListEnvelope struct {
	Data []models.CronSchedule `json:"data"`
}

// ScheduleRunResponse is the payload returned when a schedule is triggered.
type ScheduleRunResponse struct {
	JobID uuid.UUID `json:"job_id"`
}

// ScheduleRunEnvelope wraps ScheduleRunResponse in the standard success envelope.
type ScheduleRunEnvelope struct {
	Data ScheduleRunResponse `json:"data"`
}

// respondSchedulerError maps a scheduler error to the API error envelope.
func respondSchedulerError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrScheduleNotFound):
		RespondNotFound(c, "schedule not found")
	case errors.Is(err, service.ErrInvalidCron),
		errors.Is(err, service.ErrInvalidSchedule),
		errors.Is(err, service.ErrUnknownCommand):
		RespondError(c, http.StatusBadRequest, CodeValidation, err.Error())
	case errors.Is(err, service.ErrJobRunning):
		RespondConflict(c, "a job is already running")
	default:
		RespondInternalError(c, err)
	}
}

// List handles GET /api/v1/scheduler/jobs.
//
//	@Summary		List cron schedules
//	@Description	Returns every cron schedule persisted in lab_target.
//	@Tags			scheduler
//	@Produce		json
//	@Success		200	{object}	ScheduleListEnvelope
//	@Failure		500	{object}	ErrorResponse
//	@Router			/scheduler/jobs [get]
func (h *SchedulerHandler) List(c *gin.Context) {
	rows, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondSchedulerError(c, err)
		return
	}
	RespondData(c, http.StatusOK, rows)
}

// Create handles POST /api/v1/scheduler/jobs.
//
//	@Summary		Create a cron schedule
//	@Description	Creates a schedule; the cron expression is validated and next_run_at computed.
//	@Tags			scheduler
//	@Accept			json
//	@Produce		json
//	@Param			request	body		scheduler.CreateParams	true	"Schedule definition"
//	@Success		201		{object}	ScheduleEnvelope
//	@Failure		400		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/scheduler/jobs [post]
func (h *SchedulerHandler) Create(c *gin.Context) {
	var p scheduler.CreateParams
	if err := c.ShouldBindJSON(&p); err != nil {
		RespondValidationError(c, err)
		return
	}
	sc, err := h.svc.Create(c.Request.Context(), p)
	if err != nil {
		respondSchedulerError(c, err)
		return
	}
	RespondData(c, http.StatusCreated, sc)
}

// Update handles PUT /api/v1/scheduler/jobs/:id.
//
//	@Summary		Update a cron schedule
//	@Description	Replaces a schedule's fields and recomputes next_run_at.
//	@Tags			scheduler
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Schedule id (opaque token)"
//	@Param			request	body		scheduler.UpdateParams	true	"Schedule definition"
//	@Success		200		{object}	ScheduleEnvelope
//	@Failure		400		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/scheduler/jobs/{id} [put]
func (h *SchedulerHandler) Update(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	var p scheduler.UpdateParams
	if err := c.ShouldBindJSON(&p); err != nil {
		RespondValidationError(c, err)
		return
	}
	sc, err := h.svc.Update(c.Request.Context(), id, p)
	if err != nil {
		respondSchedulerError(c, err)
		return
	}
	RespondData(c, http.StatusOK, sc)
}

// Delete handles DELETE /api/v1/scheduler/jobs/:id.
//
//	@Summary		Delete a cron schedule
//	@Description	Removes a schedule and unregisters it from the cron runner.
//	@Tags			scheduler
//	@Param			id	path	string	true	"Schedule id (opaque token)"
//	@Success		204	"No Content"
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/scheduler/jobs/{id} [delete]
func (h *SchedulerHandler) Delete(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondSchedulerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Enable handles POST /api/v1/scheduler/jobs/:id/enable.
//
//	@Summary		Enable a cron schedule
//	@Description	Turns a schedule on and registers it with the cron runner.
//	@Tags			scheduler
//	@Produce		json
//	@Param			id	path		string	true	"Schedule id (opaque token)"
//	@Success		200	{object}	ScheduleEnvelope
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/scheduler/jobs/{id}/enable [post]
func (h *SchedulerHandler) Enable(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	sc, err := h.svc.Enable(c.Request.Context(), id)
	if err != nil {
		respondSchedulerError(c, err)
		return
	}
	RespondData(c, http.StatusOK, sc)
}

// Disable handles POST /api/v1/scheduler/jobs/:id/disable.
//
//	@Summary		Disable a cron schedule
//	@Description	Turns a schedule off and unregisters it from the cron runner.
//	@Tags			scheduler
//	@Produce		json
//	@Param			id	path		string	true	"Schedule id (opaque token)"
//	@Success		200	{object}	ScheduleEnvelope
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/scheduler/jobs/{id}/disable [post]
func (h *SchedulerHandler) Disable(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	sc, err := h.svc.Disable(c.Request.Context(), id)
	if err != nil {
		respondSchedulerError(c, err)
		return
	}
	RespondData(c, http.StatusOK, sc)
}

// Run handles POST /api/v1/scheduler/jobs/:id/run.
//
//	@Summary		Run a cron schedule now
//	@Description	Triggers the schedule immediately and returns the started job id.
//	@Tags			scheduler
//	@Produce		json
//	@Param			id	path		string	true	"Schedule id (opaque token)"
//	@Success		200	{object}	ScheduleRunEnvelope
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		409	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/scheduler/jobs/{id}/run [post]
func (h *SchedulerHandler) Run(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	jobID, err := h.svc.RunNow(c.Request.Context(), id)
	if err != nil {
		respondSchedulerError(c, err)
		return
	}
	RespondData(c, http.StatusOK, ScheduleRunResponse{JobID: jobID})
}
