package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// fakeRepo is an in-memory JobRepository used by the manager tests.
type fakeRepo struct {
	mu        sync.Mutex
	rows      map[uuid.UUID]*models.JobExecution
	order     []uuid.UUID
	listFn    func(limit, offset int) ([]models.JobExecution, int64, error)
	createErr error
	updateErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{rows: make(map[uuid.UUID]*models.JobExecution)}
}

func (r *fakeRepo) Create(_ context.Context, exec *models.JobExecution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	cp := *exec
	r.rows[exec.ID] = &cp
	r.order = append(r.order, exec.ID)
	return nil
}

func (r *fakeRepo) Update(_ context.Context, exec *models.JobExecution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateErr != nil {
		return r.updateErr
	}
	cp := *exec
	r.rows[exec.ID] = &cp
	return nil
}

func (r *fakeRepo) List(_ context.Context, limit, offset int) ([]models.JobExecution, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listFn != nil {
		return r.listFn(limit, offset)
	}
	out := make([]models.JobExecution, 0, len(r.order))
	for i := len(r.order) - 1; i >= 0; i-- {
		out = append(out, *r.rows[r.order[i]])
	}
	total := int64(len(out))
	if offset > len(out) {
		offset = len(out)
	}
	out = out[offset:]
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, total, nil
}

func (r *fakeRepo) get(id uuid.UUID) (models.JobExecution, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[id]
	if !ok {
		return models.JobExecution{}, false
	}
	return *row, true
}

// waitFor polls cond until it holds or the timeout elapses.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

func mustRegister(t *testing.T, m *Manager, jobType string, fn JobFunc) {
	t.Helper()
	if err := m.Register(jobType, func(any) (JobFunc, error) { return fn, nil }); err != nil {
		t.Fatalf("register %s: %v", jobType, err)
	}
}

func newManager(t *testing.T, repo JobRepository) *Manager {
	t.Helper()
	m, err := New(repo)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	return m
}

func TestNewRejectsNilRepository(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected error for nil repository")
	}
}

func TestRegisterRejectsInvalidRunner(t *testing.T) {
	m := newManager(t, newFakeRepo())
	if err := m.Register("", func(any) (JobFunc, error) { return nil, nil }); err == nil {
		t.Error("expected error for empty job type")
	}
	if err := m.Register(TypeGenerate, nil); err == nil {
		t.Error("expected error for nil runner")
	}
}

func TestStatusIdleWhenNoJobStarted(t *testing.T) {
	m := newManager(t, newFakeRepo())

	st, err := m.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Status != StatusIdle {
		t.Errorf("expected idle status, got %q", st.Status)
	}
	if st.Running {
		t.Error("expected Running=false when idle")
	}

	pr, err := m.Progress()
	if err != nil {
		t.Fatalf("progress: %v", err)
	}
	if pr.Status != StatusIdle {
		t.Errorf("expected idle progress status, got %q", pr.Status)
	}
}

func TestStartUnknownJobType(t *testing.T) {
	m := newManager(t, newFakeRepo())

	_, err := m.Start(context.Background(), "nope", nil)
	if !errors.Is(err, ErrUnknownJobType) {
		t.Fatalf("expected ErrUnknownJobType, got %v", err)
	}
}

func TestStartInvalidParamsClearsSlot(t *testing.T) {
	m := newManager(t, newFakeRepo())
	if err := m.Register(TypeGenerate, func(any) (JobFunc, error) {
		return nil, ErrInvalidParams
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	if _, err := m.Start(context.Background(), TypeGenerate, nil); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}

	// The reserved slot must be released so a valid start can proceed.
	mustRegister(t, m, TypeGenerate, func(context.Context, func(EntityProgress)) (map[string]any, error) {
		return nil, nil
	})
	if _, err := m.Start(context.Background(), TypeGenerate, nil); err != nil {
		t.Fatalf("expected start to succeed after invalid params, got %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		st, _ := m.Status()
		return st.Status == StatusCompleted
	})
}

func TestStartCompletesAndPersistsMetrics(t *testing.T) {
	repo := newFakeRepo()
	m := newManager(t, repo)
	mustRegister(t, m, TypeGenerate, func(_ context.Context, report func(EntityProgress)) (map[string]any, error) {
		report(EntityProgress{Entity: "customers", Done: 5, Total: 10, Processed: 5})
		return map[string]any{"customers": 10}, nil
	})

	id, err := m.Start(context.Background(), TypeGenerate, nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if id == uuid.Nil {
		t.Fatal("expected a job id")
	}

	waitFor(t, 2*time.Second, func() bool {
		st, _ := m.Status()
		return st.Status == StatusCompleted
	})

	st, _ := m.Status()
	if st.ID != id || st.Type != TypeGenerate {
		t.Errorf("unexpected status: %+v", st)
	}
	if st.Running {
		t.Error("expected Running=false after completion")
	}
	if st.FinishedAt == nil {
		t.Error("expected FinishedAt to be set")
	}

	row, ok := repo.get(id)
	if !ok {
		t.Fatal("expected a persisted execution row")
	}
	if row.Status != string(StatusCompleted) {
		t.Errorf("expected completed row, got %q", row.Status)
	}
	if row.FinishedAt == nil {
		t.Error("expected finished_at persisted")
	}
	var metrics map[string]any
	if err := json.Unmarshal(row.Metrics, &metrics); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	if metrics["customers"] != float64(10) {
		t.Errorf("unexpected metrics: %v", metrics)
	}
}

func TestStartConflictWhileRunning(t *testing.T) {
	repo := newFakeRepo()
	m := newManager(t, repo)
	release := make(chan struct{})
	mustRegister(t, m, TypeGenerate, func(context.Context, func(EntityProgress)) (map[string]any, error) {
		<-release
		return nil, nil
	})

	if _, err := m.Start(context.Background(), TypeGenerate, nil); err != nil {
		t.Fatalf("first start: %v", err)
	}
	if _, err := m.Start(context.Background(), TypeGenerate, nil); !errors.Is(err, ErrJobRunning) {
		t.Fatalf("expected ErrJobRunning, got %v", err)
	}

	close(release)
	waitFor(t, 2*time.Second, func() bool {
		st, _ := m.Status()
		return st.Status == StatusCompleted
	})
}

func TestStopMarksJobStopped(t *testing.T) {
	repo := newFakeRepo()
	m := newManager(t, repo)
	mustRegister(t, m, TypeMigrate, func(ctx context.Context, _ func(EntityProgress)) (map[string]any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})

	id, err := m.Start(context.Background(), TypeMigrate, nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		st, _ := m.Status()
		return st.Running
	})

	if err := m.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		st, _ := m.Status()
		return st.Status == StatusStopped
	})

	row, _ := repo.get(id)
	if row.Status != string(StatusStopped) {
		t.Errorf("expected stopped row, got %q", row.Status)
	}
}

func TestStopWithoutRunningJob(t *testing.T) {
	m := newManager(t, newFakeRepo())
	if err := m.Stop(); !errors.Is(err, ErrNoRunningJob) {
		t.Fatalf("expected ErrNoRunningJob, got %v", err)
	}
}

func TestPanicIsRecoveredAndJobFails(t *testing.T) {
	repo := newFakeRepo()
	m := newManager(t, repo)
	mustRegister(t, m, TypeGenerate, func(context.Context, func(EntityProgress)) (map[string]any, error) {
		panic("boom")
	})

	id, err := m.Start(context.Background(), TypeGenerate, nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		st, _ := m.Status()
		return st.Status == StatusFailed
	})

	st, _ := m.Status()
	if st.Error == nil || !contains(*st.Error, "panic") {
		t.Errorf("expected panic error, got %v", st.Error)
	}
	row, _ := repo.get(id)
	if row.Status != string(StatusFailed) {
		t.Errorf("expected failed row, got %q", row.Status)
	}
	if row.Error == nil || !contains(*row.Error, "boom") {
		t.Errorf("expected persisted panic error, got %v", row.Error)
	}
}

func TestProgressAggregatesEntities(t *testing.T) {
	m := newManager(t, newFakeRepo())
	done := make(chan struct{})
	mustRegister(t, m, TypeMigrate, func(_ context.Context, report func(EntityProgress)) (map[string]any, error) {
		report(EntityProgress{Entity: "customers", Done: 5, Total: 10, Processed: 5})
		report(EntityProgress{Entity: "orders", Done: 1, Total: 10, Processed: 1, Failed: 2, Skipped: 3})
		close(done)
		return nil, nil
	})

	if _, err := m.Start(context.Background(), TypeMigrate, nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	<-done
	waitFor(t, 2*time.Second, func() bool {
		pr, _ := m.Progress()
		return len(pr.Entities) == 2
	})

	pr, _ := m.Progress()
	if pr.Processed != 6 {
		t.Errorf("expected processed=6, got %d", pr.Processed)
	}
	if pr.Failed != 2 || pr.Skipped != 3 {
		t.Errorf("unexpected failed/skipped: %d/%d", pr.Failed, pr.Skipped)
	}
	if pr.Percent != 30 {
		t.Errorf("expected percent=30, got %v", pr.Percent)
	}
	if pr.Entities[0].Entity != "customers" || pr.Entities[1].Entity != "orders" {
		t.Errorf("expected stable entity order, got %+v", pr.Entities)
	}
	if pr.Entities[0].Percent != 50 {
		t.Errorf("expected per-entity percent=50, got %v", pr.Entities[0].Percent)
	}
}

func TestHistoryNormalizesPaging(t *testing.T) {
	repo := newFakeRepo()
	var gotLimit, gotOffset int
	repo.listFn = func(limit, offset int) ([]models.JobExecution, int64, error) {
		gotLimit, gotOffset = limit, offset
		return []models.JobExecution{}, 42, nil
	}
	m := newManager(t, repo)

	if _, total, err := m.History(context.Background(), 0, 0); err != nil || total != 42 {
		t.Fatalf("history: total=%d err=%v", total, err)
	}
	if gotLimit != DefaultPerPage || gotOffset != 0 {
		t.Errorf("expected defaults limit=%d offset=0, got %d/%d", DefaultPerPage, gotLimit, gotOffset)
	}

	if _, _, err := m.History(context.Background(), 3, 5); err != nil {
		t.Fatalf("history: %v", err)
	}
	if gotLimit != 5 || gotOffset != 10 {
		t.Errorf("expected limit=5 offset=10, got %d/%d", gotLimit, gotOffset)
	}

	if _, _, err := m.History(context.Background(), 1, 1000); err != nil {
		t.Fatalf("history: %v", err)
	}
	if gotLimit != MaxPerPage {
		t.Errorf("expected limit clamped to %d, got %d", MaxPerPage, gotLimit)
	}
}

func TestHistoryWrapsRepositoryError(t *testing.T) {
	repo := newFakeRepo()
	repo.listFn = func(int, int) ([]models.JobExecution, int64, error) {
		return nil, 0, errors.New("db down")
	}
	m := newManager(t, repo)

	if _, _, err := m.History(context.Background(), 1, 20); err == nil {
		t.Fatal("expected error")
	}
}

func TestStartPersistFailureReleasesSlot(t *testing.T) {
	repo := newFakeRepo()
	repo.createErr = errors.New("db down")
	m := newManager(t, repo)
	mustRegister(t, m, TypeGenerate, func(context.Context, func(EntityProgress)) (map[string]any, error) {
		return nil, nil
	})

	if _, err := m.Start(context.Background(), TypeGenerate, nil); err == nil {
		t.Fatal("expected persist error")
	}
	repo.createErr = nil
	if _, err := m.Start(context.Background(), TypeGenerate, nil); err != nil {
		t.Fatalf("expected start to succeed after persist failure, got %v", err)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
