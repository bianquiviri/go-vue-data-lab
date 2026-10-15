package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// schemaMigrationsDDL creates the per-database tracking table. It is safe to
// run repeatedly (IF NOT EXISTS), which keeps `up`/`down` idempotent.
const schemaMigrationsDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

// SQLTracker is the PostgreSQL-backed Tracker implementation.
type SQLTracker struct {
	db *sql.DB
}

// NewSQLTracker builds a Tracker that persists to db.
func NewSQLTracker(db *sql.DB) *SQLTracker {
	return &SQLTracker{db: db}
}

// EnsureTable creates schema_migrations when missing.
func (t *SQLTracker) EnsureTable(ctx context.Context) error {
	if _, err := t.db.ExecContext(ctx, schemaMigrationsDDL); err != nil {
		return fmt.Errorf("crear schema_migrations: %w", err)
	}
	return nil
}

// Applied returns applied migrations ordered by ascending version.
func (t *SQLTracker) Applied(ctx context.Context) ([]Applied, error) {
	rows, err := t.db.QueryContext(ctx,
		`SELECT version, name, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("consultar schema_migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var applied []Applied
	for rows.Next() {
		var (
			a         Applied
			appliedAt time.Time
		)
		if err := rows.Scan(&a.Version, &a.Name, &appliedAt); err != nil {
			return nil, fmt.Errorf("leer fila de schema_migrations: %w", err)
		}
		a.AppliedAt = appliedAt.UTC().Format(time.RFC3339)
		applied = append(applied, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar schema_migrations: %w", err)
	}
	return applied, nil
}

// Record inserts a migration as applied inside tx. The version is the primary
// key, so a concurrent double-apply fails loudly instead of duplicating rows.
func (t *SQLTracker) Record(ctx context.Context, tx Tx, m Migration) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		m.Version, m.Name); err != nil {
		return fmt.Errorf("insertar versión %d: %w", m.Version, err)
	}
	return nil
}

// Delete removes a migration from the tracking table inside tx.
func (t *SQLTracker) Delete(ctx context.Context, tx Tx, version int) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM schema_migrations WHERE version = $1`, version); err != nil {
		return fmt.Errorf("eliminar versión %d: %w", version, err)
	}
	return nil
}
