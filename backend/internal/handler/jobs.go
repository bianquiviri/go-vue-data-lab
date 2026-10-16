package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/jobs"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// JobsHandler serves the job control endpoints. It only depends on the
// service.JobControl port, so tests use fakes.
type JobsHandler struct {
	svc service.JobControl
}

// NewJobsHandler builds a JobsHandler over the given job control service.
func NewJobsHandler(svc service.JobControl) *JobsHandler {
	return &JobsHandler{svc: svc}
}

// JobStartResponse is the payload returned when a job is accepted.
type JobStartResponse struct {
	JobID  uuid.UUID `json:"job_id"`
	Type   string    `json:"type"`
	Status string    `json:"status"`
}

// JobStartEnvelope wraps JobStartResponse in the standard success envelope.
type JobStartEnvelope struct {
	Data JobStartResponse `json:"data"`
}

// JobStopResponse is the payload returned when a job is stopped.
type JobStopResponse struct {
	Stopped bool `json:"stopped"`
}

// JobStopEnvelope wraps JobStopResponse in the standard success envelope.
type JobStopEnvelope struct {
	Data JobStopResponse `json:"data"`
}

// JobStatusEnvelope wraps a job status in the standard success envelope.
type JobStatusEnvelope struct {
	Data jobs.JobStatus `json:"data"`
}

// JobProgressEnvelope wraps job progress in the standard success envelope.
type JobProgressEnvelope struct {
	Data jobs.Progress `json:"data"`
}

// respondJobError maps a job control error to the API error envelope.
func respondJobError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrJobRunning):
		RespondConflict(c, "a job is already running")
	case errors.Is(err, service.ErrNoRunningJob):
		RespondNotFound(c, "no job is running")
	case errors.Is(err, service.ErrUnknownJobType), errors.Is(err, service.ErrInvalidParams):
		RespondError(c, http.StatusBadRequest, CodeValidation, err.Error())
	default:
		RespondInternalError(c, err)
	}
}

// StartGenerate handles POST /api/v1/jobs/generate/start.
//
//	@Summary		Start a data generation job
//	@Description	Launches synthetic data generation into lab_source. Only one job may run at a time.
//	@Tags			jobs
//	@Accept			json
//	@Produce		json
//	@Param			request	body		jobs.GenerateParams	false	"Generation parameters (defaults applied when omitted)"
//	@Success		202		{object}	JobStartEnvelope
//	@Failure		400		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/jobs/generate/start [post]
func (h *JobsHandler) StartGenerate(c *gin.Context) {
	var p jobs.GenerateParams
	if err := c.ShouldBindJSON(&p); err != nil {
		RespondValidationError(c, err)
		return
	}
	id, err := h.svc.Start(c.Request.Context(), jobs.TypeGenerate, p)
	if err != nil {
		respondJobError(c, err)
		return
	}
	RespondData(c, http.StatusAccepted, JobStartResponse{JobID: id, Type: jobs.TypeGenerate, Status: string(jobs.StatusRunning)})
}

// StartMigrate handles POST /api/v1/jobs/migrate/start.
//
//	@Summary		Start an ETL migration job
//	@Description	Launches the lab_source -> lab_target migration. Only one job may run at a time.
//	@Tags			jobs
//	@Accept			json
//	@Produce		json
//	@Param			request	body		jobs.MigrateParams	false	"Migration parameters (defaults applied when omitted)"
//	@Success		202		{object}	JobStartEnvelope
//	@Failure		400		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/jobs/migrate/start [post]
func (h *JobsHandler) StartMigrate(c *gin.Context) {
	var p jobs.MigrateParams
	if err := c.ShouldBindJSON(&p); err != nil {
		RespondValidationError(c, err)
		return
	}
	id, err := h.svc.Start(c.Request.Context(), jobs.TypeMigrate, p)
	if err != nil {
		respondJobError(c, err)
		return
	}
	RespondData(c, http.StatusAccepted, JobStartResponse{JobID: id, Type: jobs.TypeMigrate, Status: string(jobs.StatusRunning)})
}

// Stop handles POST /api/v1/jobs/stop.
//
//	@Summary		Stop the running job
//	@Description	Cancels the currently running job; its status becomes stopped.
//	@Tags			jobs
//	@Produce		json
//	@Success		200	{object}	JobStopEnvelope
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/jobs/stop [post]
func (h *JobsHandler) Stop(c *gin.Context) {
	if err := h.svc.Stop(); err != nil {
		respondJobError(c, err)
		return
	}
	RespondData(c, http.StatusOK, JobStopResponse{Stopped: true})
}

// Status handles GET /api/v1/jobs/status.
//
//	@Summary		Current job status
//	@Description	Returns the snapshot of the current (or last) job, or an idle status.
//	@Tags			jobs
//	@Produce		json
//	@Success		200	{object}	JobStatusEnvelope
//	@Failure		500	{object}	ErrorResponse
//	@Router			/jobs/status [get]
func (h *JobsHandler) Status(c *gin.Context) {
	st, err := h.svc.Status()
	if err != nil {
		respondJobError(c, err)
		return
	}
	RespondData(c, http.StatusOK, st)
}

// Progress handles GET /api/v1/jobs/progress.
//
//	@Summary		Real-time job progress
//	@Description	Returns per-entity progress, overall percent and processed/failed counters.
//	@Tags			jobs
//	@Produce		json
//	@Success		200	{object}	JobProgressEnvelope
//	@Failure		500	{object}	ErrorResponse
//	@Router			/jobs/progress [get]
func (h *JobsHandler) Progress(c *gin.Context) {
	pr, err := h.svc.Progress()
	if err != nil {
		respondJobError(c, err)
		return
	}
	RespondData(c, http.StatusOK, pr)
}

// History handles GET /api/v1/jobs/history.
//
//	@Summary		Job execution history
//	@Description	Paginated job executions from lab_target, newest first.
//	@Tags			jobs
//	@Produce		json
//	@Param			page		query		int	false	"Page number (1-based)"	minimum(1)
//	@Param			per_page	query		int	false	"Items per page (max 100)"	minimum(1)	maximum(100)
//	@Success		200			{object}	PaginatedResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/jobs/history [get]
func (h *JobsHandler) History(c *gin.Context) {
	var q Pagination
	if err := c.ShouldBindQuery(&q); err != nil {
		RespondValidationError(c, err)
		return
	}
	page := q.Normalized()

	rows, total, err := h.svc.History(c.Request.Context(), page.Page, page.PerPage)
	if err != nil {
		respondJobError(c, err)
		return
	}
	RespondPaginated(c, http.StatusOK, rows, page, total)
}
