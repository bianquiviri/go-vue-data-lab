package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// Manager owns the single active job. It is safe for concurrent use.
type Manager struct {
	repo    JobRepository
	runners map[string]RunnerFunc
	log     *zap.Logger
	now     func() time.Time

	mu      sync.Mutex
	current *activeJob
}

// activeJob is the in-memory state of the current (or last) job.
type activeJob struct {
	id         uuid.UUID
	jobType    string
	status     Status
	startedAt  time.Time
	finishedAt *time.Time
	errMsg     *string
	cancel     context.CancelFunc
	stopReq    bool

	entities  map[string]EntityProgress
	order     []string
	percent   float64
	processed int64
	failed    int64
	skipped   int64
}

// isActive reports whether the job is pending or running.
func (j *activeJob) isActive() bool {
	return j.status == StatusPending || j.status == StatusRunning
}

// Option customises a Manager.
type Option func(*Manager)

// WithLogger sets the structured logger. A nil logger is ignored.
func WithLogger(l *zap.Logger) Option {
	return func(m *Manager) {
		if l != nil {
			m.log = l
		}
	}
}

// WithClock overrides the clock used for timestamps (tests).
func WithClock(now func() time.Time) Option {
	return func(m *Manager) {
		if now != nil {
			m.now = now
		}
	}
}

// New builds a Manager over the given repository.
func New(repo JobRepository, opts ...Option) (*Manager, error) {
	if repo == nil {
		return nil, fmt.Errorf("jobs: repository is nil")
	}
	m := &Manager{
		repo:    repo,
		runners: make(map[string]RunnerFunc),
		log:     zap.NewNop(),
		now:     time.Now,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

// Register adds (or replaces) the runner for a job type.
func (m *Manager) Register(jobType string, runner RunnerFunc) error {
	if jobType == "" {
		return fmt.Errorf("jobs: job type is empty")
	}
	if runner == nil {
		return fmt.Errorf("jobs: runner for %q is nil", jobType)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runners[jobType] = runner
	return nil
}

// Start launches a job of the given type in a goroutine and returns its id.
//
// The passed context is only used for the initial persistence; the job itself
// runs on a manager-owned context so it survives the HTTP request that started
// it. It returns ErrJobRunning when another job is active, ErrUnknownJobType
// for an unregistered type and ErrInvalidParams when the parameters are
// invalid.
func (m *Manager) Start(ctx context.Context, jobType string, params any) (uuid.UUID, error) {
	m.mu.Lock()
	if m.current != nil && m.current.isActive() {
		m.mu.Unlock()
		return uuid.Nil, fmt.Errorf("jobs: start %s: %w", jobType, ErrJobRunning)
	}
	runner, ok := m.runners[jobType]
	if !ok {
		m.mu.Unlock()
		return uuid.Nil, fmt.Errorf("jobs: start %s: %w", jobType, ErrUnknownJobType)
	}
	// Reserve the slot so concurrent Starts conflict.
	m.current = &activeJob{
		jobType:  jobType,
		status:   StatusPending,
		entities: make(map[string]EntityProgress),
	}
	m.mu.Unlock()

	job, err := runner(params)
	if err != nil {
		m.releaseSlot()
		return uuid.Nil, fmt.Errorf("jobs: start %s: %w", jobType, err)
	}

	id := uuid.New()
	startedAt := m.now()
	exec := &models.JobExecution{
		ID:        id,
		JobName:   jobType,
		Status:    string(StatusRunning),
		StartedAt: startedAt,
	}
	if err := m.repo.Create(ctx, exec); err != nil {
		m.releaseSlot()
		return uuid.Nil, fmt.Errorf("jobs: persist execution: %w", err)
	}

	jobCtx, cancel := context.WithCancel(context.Background())

	m.mu.Lock()
	cur := m.current
	cur.id = id
	cur.status = StatusRunning
	cur.startedAt = startedAt
	cur.cancel = cancel
	m.mu.Unlock()

	go m.run(jobCtx, cur, job)
	return id, nil
}

// releaseSlot clears the reserved slot after a failed Start.
func (m *Manager) releaseSlot() {
	m.mu.Lock()
	m.current = nil
	m.mu.Unlock()
}

// run executes the job, recovers panics and persists the terminal state.
func (m *Manager) run(ctx context.Context, cur *activeJob, job JobFunc) {
	var (
		metrics map[string]any
		err     error
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("jobs: panic: %v", r)
				m.log.Error("job panicked", zap.String("type", cur.jobType), zap.Any("panic", r))
			}
		}()
		metrics, err = job(ctx, func(p EntityProgress) { m.applyProgress(cur, p) })
	}()

	m.mu.Lock()
	finished := m.now()
	cur.finishedAt = &finished
	switch {
	case cur.stopReq:
		cur.status = StatusStopped
		if err != nil && ctx.Err() == nil {
			msg := err.Error()
			cur.errMsg = &msg
		}
	case err != nil:
		cur.status = StatusFailed
		msg := err.Error()
		cur.errMsg = &msg
	default:
		cur.status = StatusCompleted
	}
	exec := &models.JobExecution{
		ID:         cur.id,
		JobName:    cur.jobType,
		Status:     string(cur.status),
		StartedAt:  cur.startedAt,
		FinishedAt: &finished,
		Error:      cur.errMsg,
		Metrics:    marshalMetrics(metrics),
	}
	m.mu.Unlock()

	if uerr := m.repo.Update(context.Background(), exec); uerr != nil {
		m.log.Error("no se pudo actualizar la ejecución",
			zap.String("id", cur.id.String()), zap.Error(uerr))
	}
}

// applyProgress merges a per-entity report into the job's progress.
func (m *Manager) applyProgress(cur *activeJob, p EntityProgress) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p.Percent = percentOf(p.Done, p.Total)
	if _, ok := cur.entities[p.Entity]; !ok {
		cur.order = append(cur.order, p.Entity)
	}
	cur.entities[p.Entity] = p

	var done, total, processed, failed, skipped int64
	for _, e := range cur.entities {
		done += e.Done
		total += e.Total
		processed += e.Processed
		failed += e.Failed
		skipped += e.Skipped
	}
	cur.percent = percentOf(done, total)
	cur.processed = processed
	cur.failed = failed
	cur.skipped = skipped
}

// Stop cancels the running job. Its status becomes stopped once the job
// goroutine returns. It returns ErrNoRunningJob when nothing is active.
func (m *Manager) Stop() error {
	m.mu.Lock()
	cur := m.current
	if cur == nil || !cur.isActive() {
		m.mu.Unlock()
		return fmt.Errorf("jobs: stop: %w", ErrNoRunningJob)
	}
	cur.stopReq = true
	cancel := cur.cancel
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return nil
}

// Status returns the current job snapshot, or an idle status when no job has
// been started.
func (m *Manager) Status() (JobStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil {
		return JobStatus{Status: StatusIdle, Progress: Progress{Status: StatusIdle}}, nil
	}
	return m.current.snapshot(), nil
}

// Progress returns the real-time progress of the current job.
func (m *Manager) Progress() (Progress, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil {
		return Progress{Status: StatusIdle}, nil
	}
	return m.current.progress(), nil
}

// History returns a page of past executions (newest first) plus the total.
func (m *Manager) History(ctx context.Context, page, perPage int) ([]models.JobExecution, int64, error) {
	page, perPage = normalizePaging(page, perPage)
	rows, total, err := m.repo.List(ctx, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("jobs: list history: %w", err)
	}
	return rows, total, nil
}

// snapshot builds the immutable status of the job.
func (j *activeJob) snapshot() JobStatus {
	return JobStatus{
		ID:         j.id,
		Type:       j.jobType,
		Status:     j.status,
		Running:    j.isActive(),
		StartedAt:  j.startedAt,
		FinishedAt: j.finishedAt,
		Error:      j.errMsg,
		Progress:   j.progress(),
	}
}

// progress builds the immutable progress of the job.
func (j *activeJob) progress() Progress {
	entities := make([]EntityProgress, 0, len(j.order))
	for _, name := range j.order {
		entities = append(entities, j.entities[name])
	}
	return Progress{
		JobID:     j.id,
		JobType:   j.jobType,
		Status:    j.status,
		Entities:  entities,
		Percent:   j.percent,
		Processed: j.processed,
		Failed:    j.failed,
		Skipped:   j.skipped,
	}
}

// marshalMetrics encodes the job metrics for the jsonb column.
func marshalMetrics(metrics map[string]any) json.RawMessage {
	if len(metrics) == 0 {
		return nil
	}
	b, err := json.Marshal(metrics)
	if err != nil {
		return nil
	}
	return b
}
