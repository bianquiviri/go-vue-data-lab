package postgres

import (
	"context"
	"os"
	"testing"

	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/josebianco/go-vue-data-lab/internal/config"
	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/generator"
	"github.com/josebianco/go-vue-data-lab/internal/infrastructure/database"
)

// openTestDB returns a *gorm.DB for integration tests, or skips the test when no
// database is configured/reachable. It never fails the suite just because a
// local Postgres is absent.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database integration test in short mode")
	}

	var (
		db  *gorm.DB
		err error
	)
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		db, err = gorm.Open(pgdriver.Open(dsn), &gorm.Config{
			Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		})
		if err != nil {
			t.Skipf("TEST_DATABASE_URL set but connection failed: %v", err)
		}
	} else if os.Getenv("DB_HOST") != "" {
		cfg, cfgErr := config.Load()
		if cfgErr != nil {
			t.Skipf("could not load config: %v", cfgErr)
		}
		db, err = database.OpenSource(cfg)
		if err != nil {
			t.Skipf("DB_HOST set but connection failed: %v", err)
		}
	} else {
		t.Skip("set TEST_DATABASE_URL or DB_* to run database integration tests")
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Skipf("could not access sql.DB: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Skipf("database not reachable: %v", err)
	}

	if !tableExists(t, db, "customers") {
		t.Skip("lab_source schema not migrated (customers table missing)")
	}
	return db
}

func tableExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var exists bool
	err := db.Raw(
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = ?)",
		name,
	).Scan(&exists).Error
	if err != nil {
		return false
	}
	return exists
}

func countRows(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var n int64
	if err := db.Model(model).Count(&n).Error; err != nil {
		t.Fatalf("count %T: %v", model, err)
	}
	return n
}

func TestPostgresStoreIntegration(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	store, err := NewPostgresStore(db, 500)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Reset(context.Background()) })

	cfg := generator.Config{
		Seed:      424242,
		Customers: 25,
		Products:  15,
		Orders:    40,
		MinItems:  1,
		MaxItems:  3,
		BatchSize: 7,
		Reset:     true,
	}
	gen, err := generator.New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	summary, err := gen.Generate(ctx, store, nil)
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	if summary.Skipped {
		t.Fatal("first run should not skip")
	}

	if got := countRows(t, db, &models.Customer{}); got != int64(cfg.Customers) {
		t.Errorf("customers = %d, want %d", got, cfg.Customers)
	}
	if got := countRows(t, db, &models.Product{}); got != int64(cfg.Products) {
		t.Errorf("products = %d, want %d", got, cfg.Products)
	}
	if got := countRows(t, db, &models.Order{}); got != int64(cfg.Orders) {
		t.Errorf("orders = %d, want %d", got, cfg.Orders)
	}
	if got := countRows(t, db, &models.OrderItem{}); got != summary.OrderItems {
		t.Errorf("order_items = %d, want %d", got, summary.OrderItems)
	}

	// Re-running with reset must not duplicate rows.
	if _, err := gen.Generate(ctx, store, nil); err != nil {
		t.Fatalf("second generate (reset): %v", err)
	}
	if got := countRows(t, db, &models.Customer{}); got != int64(cfg.Customers) {
		t.Errorf("after reset re-run customers = %d, want %d", got, cfg.Customers)
	}
	if got := countRows(t, db, &models.OrderItem{}); got != summary.OrderItems {
		t.Errorf("after reset re-run order_items = %d, want %d", got, summary.OrderItems)
	}

	// Append mode (no reset) with the same deterministic keys must be a no-op.
	appendCfg := cfg
	appendCfg.Reset = false
	appendGen, err := generator.New(appendCfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appendGen.Generate(ctx, store, nil); err != nil {
		t.Fatalf("append generate: %v", err)
	}
	if got := countRows(t, db, &models.Customer{}); got != int64(cfg.Customers) {
		t.Errorf("append customers = %d, want %d (no duplicates)", got, cfg.Customers)
	}
	if got := countRows(t, db, &models.Order{}); got != int64(cfg.Orders) {
		t.Errorf("append orders = %d, want %d (no duplicates)", got, cfg.Orders)
	}
	if got := countRows(t, db, &models.OrderItem{}); got != summary.OrderItems {
		t.Errorf("append order_items = %d, want %d (no duplicates)", got, summary.OrderItems)
	}

	// The marker must be recorded and detectable.
	exists, err := store.DatasetExists(ctx, gen.DatasetKey())
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected dataset marker to exist")
	}
}
