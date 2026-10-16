package migrate_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/josebianco/go-vue-data-lab/internal/migrate"
)

// --- fakes -----------------------------------------------------------------

// fakeDB records transactions opened against it and lets tests fail specific
// ExecContext calls (1-based global counter).
type fakeDB struct {
	mu          sync.Mutex
	beginErr    error
	failOnExec  map[int]error // global exec-call number -> error
	execCalls   int
	txs         []*fakeTx
	commitErrAt map[int]error // tx number -> commit error
}

type fakeTx struct {
	db         *fakeDB
	num        int // 1-based tx number
	execs      []string
	committed  bool
	rolledBack bool
}

func (d *fakeDB) BeginTx(context.Context) (migrate.Tx, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.beginErr != nil {
		return nil, d.beginErr
	}
	d.txs = append(d.txs, &fakeTx{db: d, num: len(d.txs) + 1})
	return d.txs[len(d.txs)-1], nil
}

func (t *fakeTx) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	t.execs = append(t.execs, query)
	t.db.execCalls++
	if err, ok := t.db.failOnExec[t.db.execCalls]; ok {
		return nil, err
	}
	return nil, nil
}

func (t *fakeTx) Commit() error {
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	if err, ok := t.db.commitErrAt[t.num]; ok {
		return err
	}
	t.committed = true
	return nil
}

func (t *fakeTx) Rollback() error {
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	t.rolledBack = true
	return nil
}

// fakeTracker is an in-memory Tracker.
type fakeTracker struct {
	mu         sync.Mutex
	rows       map[int]migrate.Applied
	ensureErr  error
	appliedErr error
	recordErr  error
	deleteErr  error
}

func newFakeTracker(applied ...int) *fakeTracker {
	rows := make(map[int]migrate.Applied, len(applied))
	for _, v := range applied {
		rows[v] = migrate.Applied{Version: v, Name: fmt.Sprintf("m%d", v)}
	}
	return &fakeTracker{rows: rows}
}

func (f *fakeTracker) EnsureTable(context.Context) error { return f.ensureErr }

func (f *fakeTracker) Applied(context.Context) ([]migrate.Applied, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.appliedErr != nil {
		return nil, f.appliedErr
	}
	out := make([]migrate.Applied, 0, len(f.rows))
	for _, a := range f.rows {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func (f *fakeTracker) Record(_ context.Context, _ migrate.Tx, m migrate.Migration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recordErr != nil {
		return f.recordErr
	}
	f.rows[m.Version] = migrate.Applied{Version: m.Version, Name: m.Name}
	return nil
}

func (f *fakeTracker) Delete(_ context.Context, _ migrate.Tx, version int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.rows, version)
	return nil
}

func (f *fakeTracker) versions() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]int, 0, len(f.rows))
	for v := range f.rows {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}

func testMigrations(n int) []migrate.Migration {
	ms := make([]migrate.Migration, n)
	for i := range ms {
		ms[i] = migrate.Migration{
			Version: i + 1,
			Name:    fmt.Sprintf("m%d", i+1),
			UpSQL:   fmt.Sprintf("-- up %d", i+1),
			DownSQL: fmt.Sprintf("-- down %d", i+1),
		}
	}
	return ms
}

// --- Runner.Up --------------------------------------------------------------

func TestRunnerUpAppliesPendingInOrder(t *testing.T) {
	db := &fakeDB{}
	tr := newFakeTracker(1) // 001 already applied
	r := migrate.NewRunner(db, tr, nil)

	done, err := r.Up(context.Background(), testMigrations(3))
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if len(done) != 2 || done[0].Version != 2 || done[1].Version != 3 {
		t.Fatalf("done = %+v, want [2 3]", done)
	}

	if len(db.txs) != 2 {
		t.Fatalf("got %d transactions, want 2", len(db.txs))
	}
	if got := db.txs[0].execs; len(got) != 1 || got[0] != "-- up 2" {
		t.Errorf("tx1 execs = %v, want [-- up 2]", got)
	}
	if got := db.txs[1].execs; len(got) != 1 || got[0] != "-- up 3" {
		t.Errorf("tx2 execs = %v, want [-- up 3]", got)
	}
	if !db.txs[0].committed || !db.txs[1].committed {
		t.Error("all transactions must be committed")
	}
	if got := tr.versions(); len(got) != 3 {
		t.Errorf("tracker versions = %v, want [1 2 3]", got)
	}
}

func TestRunnerUpIsIdempotent(t *testing.T) {
	db := &fakeDB{}
	tr := newFakeTracker(1, 2, 3)
	r := migrate.NewRunner(db, tr, nil)

	done, err := r.Up(context.Background(), testMigrations(3))
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if len(done) != 0 {
		t.Fatalf("done = %+v, want empty", done)
	}
	if len(db.txs) != 0 {
		t.Fatalf("got %d transactions, want 0", len(db.txs))
	}
}

func TestRunnerUpStopsAndRollsBackOnError(t *testing.T) {
	execErr := errors.New("boom")
	db := &fakeDB{failOnExec: map[int]error{2: execErr}} // 2nd migration fails
	tr := newFakeTracker()
	r := migrate.NewRunner(db, tr, nil)

	done, err := r.Up(context.Background(), testMigrations(3))
	if err == nil {
		t.Fatal("expected error from Up")
	}
	if !errors.Is(err, execErr) {
		t.Fatalf("error %q should wrap the exec error", err)
	}
	if !strings.Contains(err.Error(), "002_m2") {
		t.Errorf("error %q should mention the failing migration", err)
	}
	if len(done) != 1 || done[0].Version != 1 {
		t.Fatalf("done = %+v, want [1] (only the committed one)", done)
	}
	if len(db.txs) != 2 {
		t.Fatalf("got %d transactions, want 2 (third never attempted)", len(db.txs))
	}
	if !db.txs[0].committed {
		t.Error("first transaction should be committed")
	}
	if !db.txs[1].rolledBack {
		t.Error("failing transaction should be rolled back")
	}
	// Tracker must only know about the committed migration.
	if got := tr.versions(); len(got) != 1 || got[0] != 1 {
		t.Errorf("tracker versions = %v, want [1]", got)
	}
}

func TestRunnerUpBeginError(t *testing.T) {
	beginErr := errors.New("connection refused")
	db := &fakeDB{beginErr: beginErr}
	r := migrate.NewRunner(db, newFakeTracker(), nil)

	_, err := r.Up(context.Background(), testMigrations(1))
	if !errors.Is(err, beginErr) {
		t.Fatalf("err = %v, want wrapped begin error", err)
	}
}

func TestRunnerUpEnsureTableError(t *testing.T) {
	ensureErr := errors.New("no permission")
	r := migrate.NewRunner(&fakeDB{}, &fakeTracker{ensureErr: ensureErr}, nil)

	_, err := r.Up(context.Background(), testMigrations(1))
	if !errors.Is(err, ensureErr) {
		t.Fatalf("err = %v, want wrapped ensure error", err)
	}
}

// --- Runner.Down ------------------------------------------------------------

func TestRunnerDownRevertsLatestSteps(t *testing.T) {
	db := &fakeDB{}
	tr := newFakeTracker(1, 2, 3)
	r := migrate.NewRunner(db, tr, nil)

	done, err := r.Down(context.Background(), testMigrations(3), 2)
	if err != nil {
		t.Fatalf("Down: %v", err)
	}
	if len(done) != 2 || done[0].Version != 3 || done[1].Version != 2 {
		t.Fatalf("done = %+v, want [3 2]", done)
	}
	if len(db.txs) != 2 {
		t.Fatalf("got %d transactions, want 2", len(db.txs))
	}
	if got := db.txs[0].execs; len(got) != 1 || got[0] != "-- down 3" {
		t.Errorf("tx1 execs = %v, want [-- down 3]", got)
	}
	if got := db.txs[1].execs; len(got) != 1 || got[0] != "-- down 2" {
		t.Errorf("tx2 execs = %v, want [-- down 2]", got)
	}
	if got := tr.versions(); len(got) != 1 || got[0] != 1 {
		t.Errorf("tracker versions = %v, want [1]", got)
	}
}

func TestRunnerDownStopsOnError(t *testing.T) {
	execErr := errors.New("down boom")
	db := &fakeDB{failOnExec: map[int]error{1: execErr}}
	tr := newFakeTracker(1, 2, 3)
	r := migrate.NewRunner(db, tr, nil)

	done, err := r.Down(context.Background(), testMigrations(3), 2)
	if !errors.Is(err, execErr) {
		t.Fatalf("err = %v, want wrapped exec error", err)
	}
	if len(done) != 0 {
		t.Fatalf("done = %+v, want empty", done)
	}
	if !db.txs[0].rolledBack {
		t.Error("failing transaction should be rolled back")
	}
	// Nothing was reverted in the tracker.
	if got := tr.versions(); len(got) != 3 {
		t.Errorf("tracker versions = %v, want [1 2 3]", got)
	}
}

func TestRunnerDownNothingApplied(t *testing.T) {
	db := &fakeDB{}
	r := migrate.NewRunner(db, newFakeTracker(), nil)

	done, err := r.Down(context.Background(), testMigrations(3), 1)
	if err != nil {
		t.Fatalf("Down: %v", err)
	}
	if len(done) != 0 || len(db.txs) != 0 {
		t.Fatalf("done=%+v txs=%d, want nothing applied", done, len(db.txs))
	}
}

// --- Runner.Status ----------------------------------------------------------

func TestRunnerStatus(t *testing.T) {
	tr := newFakeTracker(1, 3)
	r := migrate.NewRunner(&fakeDB{}, tr, nil)

	rows, err := r.Status(context.Background(), testMigrations(3))
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	want := []struct {
		version int
		applied bool
	}{
		{1, true},
		{2, false},
		{3, true},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d", len(rows), len(want))
	}
	for i, w := range want {
		if rows[i].Version != w.version || rows[i].Applied != w.applied {
			t.Errorf("row[%d] = %03d applied=%v, want %03d applied=%v",
				i, rows[i].Version, rows[i].Applied, w.version, w.applied)
		}
	}
}
