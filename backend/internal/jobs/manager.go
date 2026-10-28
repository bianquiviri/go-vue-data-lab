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

// Manager owns jobs. It is safe for concurrent use.
type Manager struct {
	repo    JobRepository
	runners map[string]RunnerFunc
	log     *zap.Logger
	now     func() time.Time

	mu    sync.RWMutex
	jobs  map[uuid.UUID]*activeJob

	// subs holds the live SSE subscribers. It has its own mutex so a slow
	// broadcast never contends with mu (the job-state lock); this separation is
	// what guarantees a slow client can never block the job.
	subs subscribeState
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
		jobs:    make(map[uuid.UUID]*activeJob),
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
// it. ErrUnknownJobType is returned for an unregistered type and ErrInvalidParams
// when the parameters are invalid.
func (m *Manager) Start(ctx context.Context, jobType string, params any) (uuid.UUID, error) {
	m.mu.RLock()
	runner, ok := m.runners[jobType]
	m.mu.RUnlock()
	if !ok {
		return uuid.Nil, fmt.Errorf("jobs: start %s: %w", jobType, ErrUnknownJobType)
	}

	job, err := runner(params)
	if err != nil {
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
		return uuid.Nil, fmt.Errorf("jobs: persist execution: %w", err)
	}

	jobCtx, cancel := context.WithCancel(context.Background())

	cur := &activeJob{
		id:        id,
		jobType:   jobType,
		status:    StatusRunning,
		startedAt: startedAt,
		cancel:    cancel,
		entities:  make(map[string]EntityProgress),
	}

	m.mu.Lock()
	m.jobs[id] = cur
	running := cur.progress()
	m.mu.Unlock()

	// Announce the running state before the first progress tick so subscribers
	// see the job start.
	m.broadcast(running)

	go m.run(jobCtx, cur, job)
	return id, nil
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
	terminal := cur.progress()
	m.mu.Unlock()

	// Tell subscribers the job ended, then close their streams.
	m.broadcastTerminal(terminal)

	if uerr := m.repo.Update(context.Background(), exec); uerr != nil {
		m.log.Error("no se pudo actualizar la ejecución",
			zap.String("id", cur.id.String()), zap.Error(uerr))
	}
}

// applyProgress merges a per-entity report into the job's progress and
// broadcasts the new snapshot to every live subscriber.
func (m *Manager) applyProgress(cur *activeJob, p EntityProgress) {
	m.mu.Lock()

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

	snapshot := cur.progress()
	m.mu.Unlock()

	// Broadcast outside m.mu: a slow subscriber must never hold up the job.
	m.broadcast(snapshot)
}

// Stop cancels the running job. Its status becomes stopped once the job
// goroutine returns. It returns ErrNoRunningJob when nothing is active.
func (m *Manager) Stop() error {
	m.mu.RLock()
	var cur *activeJob
	for _, j := range m.jobs {
		if j.isActive() {
			cur = j
			break
		}
	}
	m.mu.RUnlock()

	if cur == nil {
		return fmt.Errorf("jobs: stop: %w", ErrNoRunningJob)
	}
	cur.stopReq = true
	cancel := cur.cancel
	if cancel != nil {
		cancel()
	}
	return nil
}

// Status returns the snapshot of the most recent/active job, or an idle status
// when no job has been started.
func (m *Manager) Status() (JobStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.jobs) == 0 {
		return JobStatus{Status: StatusIdle, Progress: Progress{Status: StatusIdle}}, nil
	}
	var latest *activeJob
	for _, j := range m.jobs {
		if latest == nil {
			latest = j
			continue
		}
		if j.startedAt.After(latest.startedAt) {
			latest = j
		}
	}
	if latest == nil {
		return JobStatus{Status: StatusIdle, Progress: Progress{Status: StatusIdle}}, nil
	}
	return latest.snapshot(), nil
}

// Progress returns the real-time progress of the most recent/active job.
func (m *Manager) Progress() (Progress, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.jobs) == 0 {
		return Progress{Status: StatusIdle}, nil
	}
	var latest *activeJob
	for _, j := range m.jobs {
		if latest == nil {
			latest = j
			continue
		}
		if j.startedAt.After(latest.startedAt) {
			latest = j
		}
	}
	if latest == nil {
		return Progress{Status: StatusIdle}, nil
	}
	return latest.progress(), nil
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
