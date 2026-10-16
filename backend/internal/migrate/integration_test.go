package migrate_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/josebianco/go-vue-data-lab/internal/config"
	"github.com/josebianco/go-vue-data-lab/internal/infrastructure/database"
	"github.com/josebianco/go-vue-data-lab/internal/migrate"
)

// requirePostgres returns the config plus a maintenance connection to the
// default "postgres" database. When PostgreSQL is not reachable it skips the
// calling test instead of failing.
func requirePostgres(t *testing.T) (*config.Config, *sql.DB) {
	t.Helper()

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("no config available: %v", err)
	}

	gormDB, err := database.Open(cfg, "postgres")
	if err != nil {
		t.Skipf("PostgreSQL not available (%v); skipping integration test", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		_ = database.Close(gormDB)
		t.Skipf("no *sql.DB handle: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = database.Close(gormDB)
		t.Skipf("PostgreSQL not reachable (%v); skipping integration test", err)
	}

	t.Cleanup(func() { _ = database.Close(gormDB) })
	return cfg, sqlDB
}

func createDatabase(ctx context.Context, admin *sql.DB, name string) error {
	if _, err := admin.ExecContext(ctx, `CREATE DATABASE "`+name+`"`); err != nil {
		return fmt.Errorf("create database %s: %w", name, err)
	}
	return nil
}

func dropDatabase(admin *sql.DB, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// WITH (FORCE) disconnects remaining sessions (PostgreSQL 13+).
	if _, err := admin.ExecContext(ctx, `DROP DATABASE IF EXISTS "`+name+`" WITH (FORCE)`); err != nil {
		return fmt.Errorf("drop database %s: %w", name, err)
	}
	return nil
}

func TestRunnerIntegrationUpDown(t *testing.T) {
	cfg, admin := requirePostgres(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	schemas := []struct {
		dir    string
		marker string // table that must exist after `up`
	}{
		{dir: "source", marker: "customers"},
		{dir: "target", marker: "users"},
	}

	for _, sc := range schemas {
		sc := sc
		t.Run(sc.dir, func(t *testing.T) {
			migrations, err := migrate.Discover(filepath.Join("..", "..", "migrations", sc.dir))
			if err != nil {
				t.Fatalf("Discover: %v", err)
			}
			if len(migrations) == 0 {
				t.Fatal("no migrations discovered")
			}

			dbName := fmt.Sprintf("mig_it_%s_%d", sc.dir, time.Now().UnixNano())
			if err := createDatabase(ctx, admin, dbName); err != nil {
				t.Fatalf("create database: %v", err)
			}
			t.Cleanup(func() { _ = dropDatabase(admin, dbName) })

			gormDB, err := database.Open(cfg, dbName)
			if err != nil {
				t.Fatalf("open %s: %v", dbName, err)
			}
			t.Cleanup(func() { _ = database.Close(gormDB) })

			sqlDB, err := gormDB.DB()
			if err != nil {
				t.Fatalf("sql.DB: %v", err)
			}
			runner := migrate.NewRunner(migrate.NewSQLStore(sqlDB), migrate.NewSQLTracker(sqlDB), nil)

			// up: applies everything
			done, err := runner.Up(ctx, migrations)
			if err != nil {
				t.Fatalf("Up: %v", err)
			}
			if len(done) != len(migrations) {
				t.Fatalf("applied %d migrations, want %d", len(done), len(migrations))
			}
			if !tableExists(t, ctx, sqlDB, "schema_migrations") {
				t.Error("schema_migrations table must exist after up")
			}
			if !tableExists(t, ctx, sqlDB, sc.marker) {
				t.Errorf("%s table must exist after up", sc.marker)
			}

			// up again: idempotent
			again, err := runner.Up(ctx, migrations)
			if err != nil {
				t.Fatalf("second Up: %v", err)
			}
			if len(again) != 0 {
				t.Fatalf("second Up applied %d migrations, want 0", len(again))
			}

			// down all: reverts in reverse order
			reverted, err := runner.Down(ctx, migrations, len(migrations))
			if err != nil {
				t.Fatalf("Down: %v", err)
			}
			if len(reverted) != len(migrations) {
				t.Fatalf("reverted %d migrations, want %d", len(reverted), len(migrations))
			}
			newest := migrations[len(migrations)-1].Version
			if reverted[0].Version != newest {
				t.Errorf("first revert = %d, want %d (newest first)", reverted[0].Version, newest)
			}
			if tableExists(t, ctx, sqlDB, sc.marker) {
				t.Errorf("%s table must be gone after down", sc.marker)
			}

			applied, err := migrate.NewSQLTracker(sqlDB).Applied(ctx)
			if err != nil {
				t.Fatalf("Applied: %v", err)
			}
			if len(applied) != 0 {
				t.Fatalf("%d rows left in schema_migrations, want 0", len(applied))
			}
		})
	}
}

// tableExists reports whether the given relation exists in the current schema.
func tableExists(t *testing.T, ctx context.Context, db *sql.DB, name string) bool {
	t.Helper()
	var reg sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT to_regclass($1)`, name).Scan(&reg); err != nil {
		t.Fatalf("to_regclass(%q): %v", name, err)
	}
	return reg.Valid && strings.TrimSpace(reg.String) != ""
}
