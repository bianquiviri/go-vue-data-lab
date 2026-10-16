// Package jobs implements the single-active-job control plane behind the
// /api/v1/jobs endpoints.
//
// A Manager owns at most one running job at a time. Jobs are launched in a
// goroutine, their lifecycle is persisted in the lab_target job_executions
// table and their live progress is tracked in memory. The manager is decoupled
// from the concrete generator/ETL implementations through the RunnerFunc /
// JobFunc ports, so it is unit tested with fakes and no database.
package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// Job types understood by the manager.
const (
	TypeGenerate = "generate"
	TypeMigrate  = "migrate"
)

// Status is the lifecycle state of a job.
type Status string

// Job lifecycle states. StatusIdle is the absence of a job (nothing has been
// started yet) and is only ever reported by Status/Progress, never persisted.
const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusStopped   Status = "stopped"
	StatusIdle      Status = "idle"
)

// Paging defaults/limits for the history endpoint. They mirror the HTTP-level
// pagination contract.
const (
	DefaultPage    = 1
	DefaultPerPage = 20
	MaxPerPage     = 100
)

// Sentinel errors. Handlers translate them into HTTP status codes.
var (
	// ErrJobRunning is returned by Start when another job is still active.
	ErrJobRunning = errors.New("a job is already running")
	// ErrNoRunningJob is returned by Stop when there is nothing to stop.
	ErrNoRunningJob = errors.New("no job is running")
	// ErrUnknownJobType is returned by Start for an unregistered job type.
	ErrUnknownJobType = errors.New("unknown job type")
	// ErrInvalidParams is returned when the job parameters fail validation.
	ErrInvalidParams = errors.New("invalid job parameters")
)

// EntityProgress is a per-entity progress snapshot reported by a running job.
type EntityProgress struct {
	Entity    string  `json:"entity"`
	Done      int64   `json:"done"`
	Total     int64   `json:"total"`
	Percent   float64 `json:"percent"`
	Processed int64   `json:"processed"`
	Failed    int64   `json:"failed"`
	Skipped   int64   `json:"skipped"`
}

// Progress is the aggregated, real-time progress of the current job.
type Progress struct {
	JobID     uuid.UUID        `json:"job_id"`
	JobType   string           `json:"job_type"`
	Status    Status           `json:"status"`
	Entities  []EntityProgress `json:"entities"`
	Percent   float64          `json:"percent"`
	Processed int64            `json:"processed"`
	Failed    int64            `json:"failed"`
	Skipped   int64            `json:"skipped"`
}

// JobStatus is the current job snapshot.
type JobStatus struct {
	ID         uuid.UUID  `json:"id"`
	Type       string     `json:"type"`
	Status     Status     `json:"status"`
	Running    bool       `json:"running"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error      *string    `json:"error,omitempty"`
	Progress   Progress   `json:"progress"`
}

// JobFunc runs a job. It reports per-entity progress through report and returns
// the final metrics, which are marshalled into job_executions.metrics.
type JobFunc func(ctx context.Context, report func(EntityProgress)) (metrics map[string]any, err error)

// RunnerFunc builds a JobFunc for a job type from (possibly nil) parameters.
// It returns ErrInvalidParams when the parameters are invalid.
type RunnerFunc func(params any) (JobFunc, error)

// JobRepository persists and lists job executions in lab_target.
type JobRepository interface {
	// Create inserts a new execution row.
	Create(ctx context.Context, exec *models.JobExecution) error
	// Update saves the terminal state of an execution row.
	Update(ctx context.Context, exec *models.JobExecution) error
	// List returns a page of executions (newest first) plus the total count.
	List(ctx context.Context, limit, offset int) ([]models.JobExecution, int64, error)
}

// normalizePaging clamps the requested paging to the sane contract.
func normalizePaging(page, perPage int) (int, int) {
	if page < 1 {
		page = DefaultPage
	}
	switch {
	case perPage < 1:
		perPage = DefaultPerPage
	case perPage > MaxPerPage:
		perPage = MaxPerPage
	}
	return page, perPage
}

// percentOf returns done/total as a percentage in [0,100]; 0 when total is 0.
func percentOf(done, total int64) float64 {
	if total <= 0 {
		return 0
	}
	p := float64(done) / float64(total) * 100
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}
