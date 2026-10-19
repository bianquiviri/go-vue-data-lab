package migrate

import (
	"context"
	"database/sql"
	"fmt"
)

// Tx is the minimal transaction surface the runner needs. Both *sql.Tx and
// *sql.DB satisfy it (the latter allows callers that do not use transactions),
// which keeps the runner testable without a real database.
type Tx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	Commit() error
	Rollback() error
}

// DB opens transactions. It is implemented by SQLStore for production use and
// by in-memory fakes in tests.
type DB interface {
	BeginTx(ctx context.Context) (Tx, error)
}

// Tracker persists which migrations have been applied for one database.
type Tracker interface {
	// EnsureTable creates the tracking table if it does not exist.
	EnsureTable(ctx context.Context) error
	// Applied returns the applied migrations ordered by ascending version.
	Applied(ctx context.Context) ([]Applied, error)
	// Record marks a migration as applied inside tx.
	Record(ctx context.Context, tx Tx, m Migration) error
	// Delete removes a migration from the tracking table inside tx.
	Delete(ctx context.Context, tx Tx, version int) error
}

// LogFunc receives optional progress messages. A nil LogFunc is a no-op.
type LogFunc func(format string, args ...any)

// StatusRow describes one migration and whether it is applied.
type StatusRow struct {
	Version   int
	Name      string
	Applied   bool
	AppliedAt string
}

// Runner applies and reverts migrations against a single database.
type Runner struct {
	db      DB
	tracker Tracker
	logf    LogFunc
}

// NewRunner builds a Runner. logf may be nil.
func NewRunner(db DB, tracker Tracker, logf LogFunc) *Runner {
	return &Runner{db: db, tracker: tracker, logf: logf}
}

func (r *Runner) log(format string, args ...any) {
	if r.logf != nil {
		r.logf(format, args...)
	}
}

// Up applies every pending migration in ascending version order and returns
// the ones that were applied. It is idempotent: already-applied versions are
// skipped. Each migration runs in its own transaction.
func (r *Runner) Up(ctx context.Context, all []Migration) ([]Migration, error) {
	if err := r.tracker.EnsureTable(ctx); err != nil {
		return nil, fmt.Errorf("asegurar tabla de migraciones: %w", err)
	}

	applied, err := r.tracker.Applied(ctx)
	if err != nil {
		return nil, fmt.Errorf("leer migraciones aplicadas: %w", err)
	}

	versions := make([]int, 0, len(applied))
	for _, a := range applied {
		versions = append(versions, a.Version)
	}

	pending := Pending(all, versions)
	if len(pending) == 0 {
		r.log("sin migraciones pendientes")
		return nil, nil
	}

	done := make([]Migration, 0, len(pending))
	for _, m := range pending {
		if err := r.run(ctx, m, m.UpSQL, func(tx Tx) error {
			return r.tracker.Record(ctx, tx, m)
		}); err != nil {
			return done, fmt.Errorf("aplicar migración %s: %w", m.Label(), err)
		}
		done = append(done, m)
		r.log("aplicada %s", m.Label())
	}
	return done, nil
}

// Down reverts at most steps applied migrations, newest first, and returns the
// ones that were reverted. Each migration runs in its own transaction.
func (r *Runner) Down(ctx context.Context, all []Migration, steps int) ([]Migration, error) {
	if err := r.tracker.EnsureTable(ctx); err != nil {
		return nil, fmt.Errorf("asegurar tabla de migraciones: %w", err)
	}

	applied, err := r.tracker.Applied(ctx)
	if err != nil {
		return nil, fmt.Errorf("leer migraciones aplicadas: %w", err)
	}

	revert, err := ToRevert(all, applied, steps)
	if err != nil {
		return nil, fmt.Errorf("seleccionar migraciones a revertir: %w", err)
	}
	if len(revert) == 0 {
		r.log("no hay migraciones aplicadas para revertir")
		return nil, nil
	}

	done := make([]Migration, 0, len(revert))
	for _, m := range revert {
		if err := r.run(ctx, m, m.DownSQL, func(tx Tx) error {
			return r.tracker.Delete(ctx, tx, m.Version)
		}); err != nil {
			return done, fmt.Errorf("revertir migración %s: %w", m.Label(), err)
		}
		done = append(done, m)
		r.log("revertida %s", m.Label())
	}
	return done, nil
}

// Status reports every discovered migration and whether it is applied.
func (r *Runner) Status(ctx context.Context, all []Migration) ([]StatusRow, error) {
	if err := r.tracker.EnsureTable(ctx); err != nil {
		return nil, fmt.Errorf("asegurar tabla de migraciones: %w", err)
	}

	applied, err := r.tracker.Applied(ctx)
	if err != nil {
		return nil, fmt.Errorf("leer migraciones aplicadas: %w", err)
	}

	byVersion := make(map[int]Applied, len(applied))
	for _, a := range applied {
		byVersion[a.Version] = a
	}

	rows := make([]StatusRow, 0, len(all))
	for _, m := range all {
		row := StatusRow{Version: m.Version, Name: m.Name}
		if a, ok := byVersion[m.Version]; ok {
			row.Applied = true
			row.AppliedAt = a.AppliedAt
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// run executes one migration script and the matching tracker mutation inside a
// single transaction, committing only when both succeed.
func (r *Runner) run(ctx context.Context, m Migration, script string, track func(tx Tx) error) error {
	tx, err := r.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacción para %s: %w", m.Label(), err)
	}

	if _, err := tx.ExecContext(ctx, script); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("ejecutar script: %w", err)
	}

	if err := track(tx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("registrar en schema_migrations: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("confirmar transacción: %w", err)
	}
	return nil
}

// SQLStore adapts a *sql.DB to the DB interface used by the runner.
type SQLStore struct {
	db *sql.DB
}

// NewSQLStore wraps a database/sql handle.
func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

// BeginTx starts a transaction.
func (s *SQLStore) BeginTx(ctx context.Context) (Tx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	return tx, nil
}
