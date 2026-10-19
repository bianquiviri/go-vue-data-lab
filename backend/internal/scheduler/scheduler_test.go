package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
)

// fakeRepo is an in-memory ScheduleRepository.
type fakeRepo struct {
	mu        sync.Mutex
	rows      map[uuid.UUID]*models.CronSchedule
	order     []uuid.UUID
	createErr error
	updateErr error
	deleteErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{rows: make(map[uuid.UUID]*models.CronSchedule)}
}

func (r *fakeRepo) seed(rows ...*models.CronSchedule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, row := range rows {
		cp := *row
		r.rows[row.ID] = &cp
		r.order = append(r.order, row.ID)
	}
}

func (r *fakeRepo) List(context.Context) ([]models.CronSchedule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.CronSchedule, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, *r.rows[id])
	}
	return out, nil
}

func (r *fakeRepo) Get(_ context.Context, id uuid.UUID) (*models.CronSchedule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *row
	return &cp, nil
}

func (r *fakeRepo) Create(_ context.Context, s *models.CronSchedule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	cp := *s
	r.rows[s.ID] = &cp
	r.order = append(r.order, s.ID)
	return nil
}

func (r *fakeRepo) Update(_ context.Context, s *models.CronSchedule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateErr != nil {
		return r.updateErr
	}
	cp := *s
	r.rows[s.ID] = &cp
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if _, ok := r.rows[id]; !ok {
		return repository.ErrNotFound
	}
	delete(r.rows, id)
	for i, existing := range r.order {
		if existing == id {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return nil
}

func (r *fakeRepo) get(id uuid.UUID) (models.CronSchedule, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[id]
	if !ok {
		return models.CronSchedule{}, false
	}
	return *row, true
}

// fakeStarter records the jobs started by the scheduler.
type fakeStarter struct {
	mu    sync.Mutex
	calls []starterCall
	id    uuid.UUID
	err   error
}

type starterCall struct {
	jobType string
	params  any
}

func (s *fakeStarter) start(_ context.Context, jobType string, params any) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, starterCall{jobType: jobType, params: params})
	if s.err != nil {
		return uuid.Nil, s.err
	}
	if s.id == uuid.Nil {
		s.id = uuid.New()
	}
	return s.id, nil
}

func (s *fakeStarter) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func newScheduler(t *testing.T, repo ScheduleRepository, starter JobStarter, opts ...Option) *Scheduler {
	t.Helper()
	s, err := New(repo, starter, opts...)
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	return s
}

func TestNewRejectsNilDependencies(t *testing.T) {
	if _, err := New(nil, (&fakeStarter{}).start); err == nil {
		t.Error("expected error for nil repository")
	}
	if _, err := New(newFakeRepo(), nil); err == nil {
		t.Error("expected error for nil starter")
	}
}

func TestCreateValidatesCronExpression(t *testing.T) {
	s := newScheduler(t, newFakeRepo(), (&fakeStarter{}).start)

	_, err := s.Create(context.Background(), CreateParams{Name: "bad", CronExpr: "not a cron", Command: "generate"})
	if !errors.Is(err, ErrInvalidCron) {
		t.Fatalf("expected ErrInvalidCron, got %v", err)
	}
}

func TestCreateValidatesNameAndCommand(t *testing.T) {
	s := newScheduler(t, newFakeRepo(), (&fakeStarter{}).start)

	if _, err := s.Create(context.Background(), CreateParams{CronExpr: "* * * * *", Command: "generate"}); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams for empty name, got %v", err)
	}
	if _, err := s.Create(context.Background(), CreateParams{Name: "x", CronExpr: "* * * * *"}); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams for empty command, got %v", err)
	}
}

func TestCreateRejectsUnknownCommandWhenRestricted(t *testing.T) {
	s := newScheduler(t, newFakeRepo(), (&fakeStarter{}).start, WithValidCommands("generate"))

	_, err := s.Create(context.Background(), CreateParams{Name: "x", CronExpr: "* * * * *", Command: "migrate"})
	if !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("expected ErrUnknownCommand, got %v", err)
	}
}

func TestCreatePersistsAndRegistersEnabledSchedule(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := newScheduler(t, repo, (&fakeStarter{}).start, WithClock(fixedClock(now)))
	defer s.Stop()

	sc, err := s.Create(context.Background(), CreateParams{Name: "nightly", CronExpr: "0 3 * * *", Command: "generate"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !sc.Enabled {
		t.Error("expected enabled by default")
	}
	if sc.NextRunAt == nil {
		t.Fatal("expected next_run_at to be computed")
	}
	want := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	if !sc.NextRunAt.Equal(want) {
		t.Errorf("expected next run %s, got %s", want, sc.NextRunAt)
	}

	row, ok := repo.get(sc.ID)
	if !ok {
		t.Fatal("expected persisted schedule")
	}
	if row.NextRunAt == nil || !row.NextRunAt.Equal(want) {
		t.Errorf("expected persisted next_run_at %s, got %v", want, row.NextRunAt)
	}

	s.mu.Lock()
	entries := len(s.entries)
	s.mu.Unlock()
	if entries != 1 {
		t.Errorf("expected 1 registered entry, got %d", entries)
	}
}

func TestCreateDisabledScheduleIsNotRegistered(t *testing.T) {
	repo := newFakeRepo()
	s := newScheduler(t, repo, (&fakeStarter{}).start)
	defer s.Stop()

	disabled := false
	sc, err := s.Create(context.Background(), CreateParams{Name: "off", CronExpr: "0 3 * * *", Command: "generate", Enabled: &disabled})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sc.Enabled {
		t.Error("expected disabled schedule")
	}
	if sc.NextRunAt != nil {
		t.Error("expected no next_run_at for disabled schedule")
	}
	s.mu.Lock()
	entries := len(s.entries)
	s.mu.Unlock()
	if entries != 0 {
		t.Errorf("expected no registered entries, got %d", entries)
	}
}

func TestListReturnsAllSchedules(t *testing.T) {
	repo := newFakeRepo()
	repo.seed(
		&models.CronSchedule{ID: uuid.New(), Name: "a", CronExpr: "* * * * *", Command: "generate", Enabled: true},
		&models.CronSchedule{ID: uuid.New(), Name: "b", CronExpr: "* * * * *", Command: "migrate", Enabled: false},
	)
	s := newScheduler(t, repo, (&fakeStarter{}).start)

	rows, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 schedules, got %d", len(rows))
	}
}

func TestUpdateReplacesScheduleAndRecomputesNextRun(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	repo.seed(&models.CronSchedule{ID: id, Name: "old", CronExpr: "0 3 * * *", Command: "generate", Enabled: true})
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := newScheduler(t, repo, (&fakeStarter{}).start, WithClock(fixedClock(now)))
	defer s.Stop()

	updated, err := s.Update(context.Background(), id, UpdateParams{Name: "new", CronExpr: "0 4 * * *", Command: "migrate"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "new" || updated.CronExpr != "0 4 * * *" || updated.Command != "migrate" {
		t.Errorf("unexpected updated schedule: %+v", updated)
	}
	want := time.Date(2026, 1, 2, 4, 0, 0, 0, time.UTC)
	if updated.NextRunAt == nil || !updated.NextRunAt.Equal(want) {
		t.Errorf("expected next run %s, got %v", want, updated.NextRunAt)
	}
	row, _ := repo.get(id)
	if row.Name != "new" {
		t.Errorf("expected persisted update, got %+v", row)
	}
}

func TestUpdateInvalidCron(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	repo.seed(&models.CronSchedule{ID: id, Name: "old", CronExpr: "0 3 * * *", Command: "generate", Enabled: true})
	s := newScheduler(t, repo, (&fakeStarter{}).start)

	if _, err := s.Update(context.Background(), id, UpdateParams{Name: "new", CronExpr: "nope", Command: "generate"}); !errors.Is(err, ErrInvalidCron) {
		t.Fatalf("expected ErrInvalidCron, got %v", err)
	}
}

func TestUpdateNotFound(t *testing.T) {
	s := newScheduler(t, newFakeRepo(), (&fakeStarter{}).start)

	_, err := s.Update(context.Background(), uuid.New(), UpdateParams{Name: "x", CronExpr: "* * * * *", Command: "generate"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteRemovesSchedule(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	repo.seed(&models.CronSchedule{ID: id, Name: "x", CronExpr: "* * * * *", Command: "generate", Enabled: true})
	s := newScheduler(t, repo, (&fakeStarter{}).start)
	defer s.Stop()

	if err := s.Delete(context.Background(), id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := repo.get(id); ok {
		t.Error("expected schedule to be removed")
	}
	if err := s.Delete(context.Background(), id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second delete, got %v", err)
	}
}

func TestEnableAndDisable(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	repo.seed(&models.CronSchedule{ID: id, Name: "x", CronExpr: "0 3 * * *", Command: "generate", Enabled: false})
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := newScheduler(t, repo, (&fakeStarter{}).start, WithClock(fixedClock(now)))
	defer s.Stop()

	enabled, err := s.Enable(context.Background(), id)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !enabled.Enabled || enabled.NextRunAt == nil {
		t.Errorf("expected enabled schedule with next_run_at, got %+v", enabled)
	}

	disabled, err := s.Disable(context.Background(), id)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if disabled.Enabled || disabled.NextRunAt != nil {
		t.Errorf("expected disabled schedule without next_run_at, got %+v", disabled)
	}
	s.mu.Lock()
	entries := len(s.entries)
	s.mu.Unlock()
	if entries != 0 {
		t.Errorf("expected no registered entries after disable, got %d", entries)
	}
}

func TestEnableNotFound(t *testing.T) {
	s := newScheduler(t, newFakeRepo(), (&fakeStarter{}).start)
	if _, err := s.Enable(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if _, err := s.Disable(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRunNowTriggersJobAndUpdatesTimestamps(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	repo.seed(&models.CronSchedule{ID: id, Name: "x", CronExpr: "0 3 * * *", Command: "generate", Enabled: true})
	starter := &fakeStarter{}
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := newScheduler(t, repo, starter.start, WithClock(fixedClock(now)))

	jobID, err := s.RunNow(context.Background(), id)
	if err != nil {
		t.Fatalf("run now: %v", err)
	}
	if jobID != starter.id {
		t.Errorf("expected job id %s, got %s", starter.id, jobID)
	}
	if starter.callCount() != 1 {
		t.Fatalf("expected 1 starter call, got %d", starter.callCount())
	}
	if starter.calls[0].jobType != "generate" {
		t.Errorf("expected generate job, got %q", starter.calls[0].jobType)
	}

	row, _ := repo.get(id)
	if row.LastRunAt == nil || !row.LastRunAt.Equal(now) {
		t.Errorf("expected last_run_at %s, got %v", now, row.LastRunAt)
	}
	want := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	if row.NextRunAt == nil || !row.NextRunAt.Equal(want) {
		t.Errorf("expected next_run_at %s, got %v", want, row.NextRunAt)
	}
}

func TestRunNowNotFound(t *testing.T) {
	s := newScheduler(t, newFakeRepo(), (&fakeStarter{}).start)
	if _, err := s.RunNow(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRunNowPropagatesStarterError(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	repo.seed(&models.CronSchedule{ID: id, Name: "x", CronExpr: "0 3 * * *", Command: "generate", Enabled: true})
	starter := &fakeStarter{err: errors.New("busy")}
	s := newScheduler(t, repo, starter.start)

	if _, err := s.RunNow(context.Background(), id); err == nil {
		t.Fatal("expected error from starter")
	}
	// The tick is still recorded even when the job could not start.
	row, _ := repo.get(id)
	if row.LastRunAt == nil {
		t.Error("expected last_run_at to be recorded")
	}
}

func TestStartRegistersOnlyEnabledSchedules(t *testing.T) {
	repo := newFakeRepo()
	enabled := &models.CronSchedule{ID: uuid.New(), Name: "on", CronExpr: "0 3 * * *", Command: "generate", Enabled: true}
	disabled := &models.CronSchedule{ID: uuid.New(), Name: "off", CronExpr: "0 3 * * *", Command: "generate", Enabled: false}
	repo.seed(enabled, disabled)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := newScheduler(t, repo, (&fakeStarter{}).start, WithClock(fixedClock(now)))

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Stop()

	s.mu.Lock()
	entries := len(s.entries)
	s.mu.Unlock()
	if entries != 1 {
		t.Fatalf("expected 1 registered entry, got %d", entries)
	}
	row, _ := repo.get(enabled.ID)
	if row.NextRunAt == nil {
		t.Error("expected next_run_at persisted for the enabled schedule")
	}
}

func TestStartSkipsInvalidPersistedCron(t *testing.T) {
	repo := newFakeRepo()
	repo.seed(&models.CronSchedule{ID: uuid.New(), Name: "bad", CronExpr: "nope", Command: "generate", Enabled: true})
	s := newScheduler(t, repo, (&fakeStarter{}).start)

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("start should tolerate invalid persisted schedules: %v", err)
	}
	defer s.Stop()
	s.mu.Lock()
	entries := len(s.entries)
	s.mu.Unlock()
	if entries != 0 {
		t.Errorf("expected no registered entries, got %d", entries)
	}
}

func TestNextRunRejectsInvalidExpression(t *testing.T) {
	if _, err := nextRun("nope", time.Now()); !errors.Is(err, ErrInvalidCron) {
		t.Fatalf("expected ErrInvalidCron, got %v", err)
	}
}
