// Package scheduler wraps robfig/cron/v3 with PostgreSQL persistence in the
// lab_target cron_schedules table.
//
// Enabled schedules are loaded from the database on Start and registered with
// the cron runner; next_run_at is computed and persisted. When a schedule
// fires, the configured command is started through the injected JobStarter
// (the jobs.Manager) and last_run_at/next_run_at are updated.
//
// The package depends only on the ScheduleRepository and JobStarter ports, so
// it is unit tested with fakes and no database.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
)

// Sentinel errors. Handlers translate them into HTTP status codes.
var (
	// ErrNotFound is returned when a schedule does not exist.
	ErrNotFound = errors.New("schedule not found")
	// ErrInvalidCron is returned when a cron expression cannot be parsed.
	ErrInvalidCron = errors.New("invalid cron expression")
	// ErrInvalidParams is returned when the schedule fields are invalid.
	ErrInvalidParams = errors.New("invalid schedule parameters")
	// ErrUnknownCommand is returned when the command is not an allowed job type.
	ErrUnknownCommand = errors.New("unknown schedule command")
)

// ScheduleRepository is the persistence port over cron_schedules.
type ScheduleRepository interface {
	// List returns every schedule.
	List(ctx context.Context) ([]models.CronSchedule, error)
	// Get returns the schedule with the given id, or repository.ErrNotFound.
	Get(ctx context.Context, id uuid.UUID) (*models.CronSchedule, error)
	// Create inserts a new schedule.
	Create(ctx context.Context, s *models.CronSchedule) error
	// Update saves an existing schedule.
	Update(ctx context.Context, s *models.CronSchedule) error
	// Delete removes a schedule.
	Delete(ctx context.Context, id uuid.UUID) error
}

// JobStarter starts a job of the given type and returns its id. It is satisfied
// by (*jobs.Manager).Start.
type JobStarter func(ctx context.Context, jobType string, params any) (uuid.UUID, error)

// CreateParams are the fields required to create a schedule.
type CreateParams struct {
	Name     string `json:"name" binding:"required"`
	CronExpr string `json:"cron_expr" binding:"required"`
	Command  string `json:"command" binding:"required"`
	// Enabled defaults to true when omitted.
	Enabled *bool `json:"enabled"`
}

// UpdateParams are the fields required to replace a schedule. Enabled is
// optional and keeps the current value when omitted.
type UpdateParams struct {
	Name     string `json:"name" binding:"required"`
	CronExpr string `json:"cron_expr" binding:"required"`
	Command  string `json:"command" binding:"required"`
	Enabled  *bool  `json:"enabled"`
}

// Scheduler owns the cron runner and the schedule registry.
type Scheduler struct {
	repo  ScheduleRepository
	start JobStarter
	cron  *cron.Cron
	log   *zap.Logger
	now   func() time.Time
	valid map[string]bool

	mu      sync.Mutex
	entries map[uuid.UUID]cron.EntryID
	started bool
}

// Option customises a Scheduler.
type Option func(*Scheduler)

// WithLogger sets the structured logger. A nil logger is ignored.
func WithLogger(l *zap.Logger) Option {
	return func(s *Scheduler) {
		if l != nil {
			s.log = l
		}
	}
}

// WithClock overrides the clock used for timestamps (tests).
func WithClock(now func() time.Time) Option {
	return func(s *Scheduler) {
		if now != nil {
			s.now = now
		}
	}
}

// WithValidCommands restricts the accepted command values to the given job
// types. When not set, any non-empty command is accepted.
func WithValidCommands(commands ...string) Option {
	return func(s *Scheduler) {
		s.valid = make(map[string]bool, len(commands))
		for _, c := range commands {
			s.valid[c] = true
		}
	}
}

// New builds a Scheduler over the given repository and job starter.
func New(repo ScheduleRepository, start JobStarter, opts ...Option) (*Scheduler, error) {
	if repo == nil {
		return nil, fmt.Errorf("scheduler: repository is nil")
	}
	if start == nil {
		return nil, fmt.Errorf("scheduler: job starter is nil")
	}
	s := &Scheduler{
		repo:    repo,
		start:   start,
		cron:    cron.New(),
		log:     zap.NewNop(),
		now:     time.Now,
		entries: make(map[uuid.UUID]cron.EntryID),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// Start loads the enabled schedules, registers them and starts the cron runner.
func (s *Scheduler) Start(ctx context.Context) error {
	schedules, err := s.repo.List(ctx)
	if err != nil {
		return fmt.Errorf("scheduler: list schedules: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range schedules {
		sc := schedules[i]
		if !sc.Enabled {
			continue
		}
		if err := s.registerLocked(&sc); err != nil {
			s.log.Warn("no se pudo registrar el schedule",
				zap.String("id", sc.ID.String()), zap.Error(err))
			continue
		}
		s.persistNextLocked(&sc)
	}
	s.cron.Start()
	s.started = true
	return nil
}

// Stop stops the cron runner. It is safe to call multiple times.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	started := s.started
	s.started = false
	s.mu.Unlock()
	if started {
		s.cron.Stop()
	}
}

// List returns every schedule.
func (s *Scheduler) List(ctx context.Context) ([]models.CronSchedule, error) {
	rows, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("scheduler: list: %w", err)
	}
	return rows, nil
}

// Create validates and persists a new schedule, registering it when enabled.
func (s *Scheduler) Create(ctx context.Context, p CreateParams) (models.CronSchedule, error) {
	sc, err := s.buildSchedule(uuid.New(), p.Name, p.CronExpr, p.Command, enabledOrDefault(p.Enabled, true))
	if err != nil {
		return models.CronSchedule{}, err
	}
	if err := s.repo.Create(ctx, &sc); err != nil {
		return models.CronSchedule{}, fmt.Errorf("scheduler: create: %w", err)
	}
	if sc.Enabled {
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := s.registerLocked(&sc); err != nil {
			return models.CronSchedule{}, err
		}
		s.persistNextLocked(&sc)
	}
	return sc, nil
}

// Update replaces an existing schedule and re-registers it.
func (s *Scheduler) Update(ctx context.Context, id uuid.UUID, p UpdateParams) (models.CronSchedule, error) {
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return models.CronSchedule{}, wrapNotFound("get schedule", err)
	}

	enabled := existing.Enabled
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	updated, err := s.buildSchedule(id, p.Name, p.CronExpr, p.Command, enabled)
	if err != nil {
		return models.CronSchedule{}, err
	}
	updated.CreatedAt = existing.CreatedAt
	updated.LastRunAt = existing.LastRunAt

	if err := s.repo.Update(ctx, &updated); err != nil {
		return models.CronSchedule{}, fmt.Errorf("scheduler: update: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.unregisterLocked(id)
	if updated.Enabled {
		if err := s.registerLocked(&updated); err != nil {
			return models.CronSchedule{}, err
		}
		s.persistNextLocked(&updated)
	} else {
		updated.NextRunAt = nil
	}
	return updated, nil
}

// Delete removes a schedule and unregisters it.
func (s *Scheduler) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return wrapNotFound("get schedule", err)
	}
	s.mu.Lock()
	s.unregisterLocked(id)
	s.mu.Unlock()

	if err := s.repo.Delete(ctx, id); err != nil {
		return wrapNotFound("delete schedule", err)
	}
	return nil
}

// Enable turns a schedule on and registers it.
func (s *Scheduler) Enable(ctx context.Context, id uuid.UUID) (models.CronSchedule, error) {
	sc, err := s.repo.Get(ctx, id)
	if err != nil {
		return models.CronSchedule{}, wrapNotFound("get schedule", err)
	}
	if sc.Enabled {
		return *sc, nil
	}
	sc.Enabled = true
	if err := s.repo.Update(ctx, sc); err != nil {
		return models.CronSchedule{}, fmt.Errorf("scheduler: enable: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.registerLocked(sc); err != nil {
		return models.CronSchedule{}, err
	}
	s.persistNextLocked(sc)
	return *sc, nil
}

// Disable turns a schedule off and unregisters it.
func (s *Scheduler) Disable(ctx context.Context, id uuid.UUID) (models.CronSchedule, error) {
	sc, err := s.repo.Get(ctx, id)
	if err != nil {
		return models.CronSchedule{}, wrapNotFound("get schedule", err)
	}
	if !sc.Enabled {
		return *sc, nil
	}
	sc.Enabled = false
	sc.NextRunAt = nil
	if err := s.repo.Update(ctx, sc); err != nil {
		return models.CronSchedule{}, fmt.Errorf("scheduler: disable: %w", err)
	}

	s.mu.Lock()
	s.unregisterLocked(id)
	s.mu.Unlock()
	return *sc, nil
}

// RunNow triggers a schedule immediately and returns the started job id.
func (s *Scheduler) RunNow(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	sc, err := s.repo.Get(ctx, id)
	if err != nil {
		return uuid.Nil, wrapNotFound("get schedule", err)
	}
	return s.trigger(ctx, sc)
}

// fire is the cron callback: it reloads the schedule and triggers it.
func (s *Scheduler) fire(id uuid.UUID) {
	ctx := context.Background()
	sc, err := s.repo.Get(ctx, id)
	if err != nil {
		s.log.Error("schedule no encontrado al disparar",
			zap.String("id", id.String()), zap.Error(err))
		return
	}
	if _, err := s.trigger(ctx, sc); err != nil {
		s.log.Error("no se pudo iniciar el job programado",
			zap.String("id", id.String()), zap.Error(err))
	}
}

// trigger starts the schedule's command and records the run timestamps.
func (s *Scheduler) trigger(ctx context.Context, sc *models.CronSchedule) (uuid.UUID, error) {
	jobID, startErr := s.start(ctx, sc.Command, nil)

	now := s.now()
	sc.LastRunAt = &now
	if next, err := nextRun(sc.CronExpr, now); err == nil {
		sc.NextRunAt = &next
	}
	if err := s.repo.Update(context.Background(), sc); err != nil {
		s.log.Warn("no se pudo actualizar el schedule",
			zap.String("id", sc.ID.String()), zap.Error(err))
	}

	if startErr != nil {
		return uuid.Nil, fmt.Errorf("scheduler: start %q: %w", sc.Command, startErr)
	}
	return jobID, nil
}

// buildSchedule validates the fields and returns a new schedule value.
func (s *Scheduler) buildSchedule(id uuid.UUID, name, expr, command string, enabled bool) (models.CronSchedule, error) {
	name = strings.TrimSpace(name)
	expr = strings.TrimSpace(expr)
	command = strings.TrimSpace(command)

	if name == "" {
		return models.CronSchedule{}, fmt.Errorf("%w: name is required", ErrInvalidParams)
	}
	if len(name) > 100 {
		return models.CronSchedule{}, fmt.Errorf("%w: name must be at most 100 characters", ErrInvalidParams)
	}
	if command == "" {
		return models.CronSchedule{}, fmt.Errorf("%w: command is required", ErrInvalidParams)
	}
	if s.valid != nil && !s.valid[command] {
		return models.CronSchedule{}, fmt.Errorf("%w: %q", ErrUnknownCommand, command)
	}
	if _, err := cron.ParseStandard(expr); err != nil {
		return models.CronSchedule{}, fmt.Errorf("%w: %v", ErrInvalidCron, err)
	}
	return models.CronSchedule{
		ID:       id,
		Name:     name,
		CronExpr: expr,
		Command:  command,
		Enabled:  enabled,
	}, nil
}

// registerLocked registers (or re-registers) a schedule with the cron runner.
// The caller must hold s.mu.
func (s *Scheduler) registerLocked(sc *models.CronSchedule) error {
	if _, err := cron.ParseStandard(sc.CronExpr); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidCron, err)
	}
	s.unregisterLocked(sc.ID)

	id := sc.ID
	entryID, err := s.cron.AddFunc(sc.CronExpr, func() { s.fire(id) })
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidCron, err)
	}
	s.entries[id] = entryID
	return nil
}

// unregisterLocked removes a schedule from the cron runner. The caller must
// hold s.mu.
func (s *Scheduler) unregisterLocked(id uuid.UUID) {
	entryID, ok := s.entries[id]
	if !ok {
		return
	}
	s.cron.Remove(entryID)
	delete(s.entries, id)
}

// persistNextLocked computes and persists next_run_at. The caller must hold
// s.mu.
func (s *Scheduler) persistNextLocked(sc *models.CronSchedule) {
	next, err := nextRun(sc.CronExpr, s.now())
	if err != nil {
		return
	}
	sc.NextRunAt = &next
	if err := s.repo.Update(context.Background(), sc); err != nil {
		s.log.Warn("no se pudo persistir next_run_at",
			zap.String("id", sc.ID.String()), zap.Error(err))
	}
}

// nextRun returns the next activation time of a cron expression.
func nextRun(expr string, from time.Time) (time.Time, error) {
	schedule, err := cron.ParseStandard(expr)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %v", ErrInvalidCron, err)
	}
	return schedule.Next(from), nil
}

// enabledOrDefault resolves the tri-state enabled flag.
func enabledOrDefault(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

// wrapNotFound maps repository.ErrNotFound to scheduler.ErrNotFound.
func wrapNotFound(op string, err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("scheduler: %s: %w", op, ErrNotFound)
	}
	return fmt.Errorf("scheduler: %s: %w", op, err)
}
