package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/jobs"
	"github.com/josebianco/go-vue-data-lab/internal/scheduler"
)

// Job control sentinels re-exported for the HTTP layer so handlers do not need
// to import the jobs package for error matching.
var (
	// ErrJobRunning is returned when a job is already active.
	ErrJobRunning = jobs.ErrJobRunning
	// ErrNoRunningJob is returned when there is no job to stop.
	ErrNoRunningJob = jobs.ErrNoRunningJob
	// ErrUnknownJobType is returned for an unregistered job type.
	ErrUnknownJobType = jobs.ErrUnknownJobType
	// ErrInvalidParams is returned when job parameters fail validation.
	ErrInvalidParams = jobs.ErrInvalidParams
)

// JobControl is the HTTP-facing job control port. It is satisfied by
// (*jobs.Manager).
type JobControl interface {
	// Start launches a job of the given type and returns its id.
	Start(ctx context.Context, jobType string, params any) (uuid.UUID, error)
	// Stop cancels the running job.
	Stop() error
	// Status returns the current job snapshot.
	Status() (jobs.JobStatus, error)
	// Progress returns the real-time progress of the current job.
	Progress() (jobs.Progress, error)
	// Subscribe returns a channel that streams progress events until the job
	// reaches a terminal state or ctx is cancelled (client disconnected).
	// It powers the SSE endpoint GET /api/v1/jobs/events.
	Subscribe(ctx context.Context) <-chan jobs.Progress
	// History returns a page of past executions plus the total count.
	History(ctx context.Context, page, perPage int) ([]models.JobExecution, int64, error)
}

// Scheduler sentinels re-exported for the HTTP layer.
var (
	// ErrScheduleNotFound is returned when a schedule does not exist.
	ErrScheduleNotFound = scheduler.ErrNotFound
	// ErrInvalidCron is returned when a cron expression cannot be parsed.
	ErrInvalidCron = scheduler.ErrInvalidCron
	// ErrInvalidSchedule is returned when schedule fields are invalid.
	ErrInvalidSchedule = scheduler.ErrInvalidParams
	// ErrUnknownCommand is returned when the schedule command is not allowed.
	ErrUnknownCommand = scheduler.ErrUnknownCommand
)

// SchedulerControl is the HTTP-facing scheduler port. It is satisfied by
// (*scheduler.Scheduler).
type SchedulerControl interface {
	// List returns every schedule.
	List(ctx context.Context) ([]models.CronSchedule, error)
	// Create validates and persists a new schedule.
	Create(ctx context.Context, p scheduler.CreateParams) (models.CronSchedule, error)
	// Update replaces an existing schedule.
	Update(ctx context.Context, id uuid.UUID, p scheduler.UpdateParams) (models.CronSchedule, error)
	// Delete removes a schedule.
	Delete(ctx context.Context, id uuid.UUID) error
	// Enable turns a schedule on.
	Enable(ctx context.Context, id uuid.UUID) (models.CronSchedule, error)
	// Disable turns a schedule off.
	Disable(ctx context.Context, id uuid.UUID) (models.CronSchedule, error)
	// RunNow triggers a schedule immediately and returns the started job id.
	RunNow(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
}
