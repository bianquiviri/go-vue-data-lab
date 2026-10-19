package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/josebianco/go-vue-data-lab/internal/config"
	"github.com/josebianco/go-vue-data-lab/internal/infrastructure/database"
)

// openTestDBs returns source and target connections for integration tests, or
// skips the test when no database is configured/reachable. It never fails the
// suite just because a local Postgres is absent.
//
// Sources, in order: TEST_DATABASE_URL / TEST_TARGET_DATABASE_URL explicit
// DSNs, else the DB_* configuration (lab_source and lab_target).
func openTestDBs(t *testing.T) (src, tgt *gorm.DB) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database integration test in short mode")
	}

	openDSN := func(dsn string) *gorm.DB {
		db, err := gorm.Open(pgdriver.Open(dsn), &gorm.Config{
			Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		})
		if err != nil {
			t.Skipf("DSN set but connection failed: %v", err)
		}
		return db
	}

	explicitSrc := os.Getenv("TEST_DATABASE_URL")
	explicitTgt := os.Getenv("TEST_TARGET_DATABASE_URL")
	cfgEnv := os.Getenv("DB_HOST") != ""

	switch {
	case explicitSrc != "" || explicitTgt != "":
		if explicitSrc != "" {
			src = openDSN(explicitSrc)
		}
		if explicitTgt != "" {
			tgt = openDSN(explicitTgt)
		}
		if src == nil || tgt == nil {
			cfg, err := config.Load()
			if err != nil {
				t.Skipf("could not complete config: %v", err)
			}
			if src == nil {
				if src, err = database.OpenSource(cfg); err != nil {
					t.Skipf("source connection failed: %v", err)
				}
			}
			if tgt == nil {
				if tgt, err = database.OpenTarget(cfg); err != nil {
					t.Skipf("target connection failed: %v", err)
				}
			}
		}
	case cfgEnv:
		cfg, err := config.Load()
		if err != nil {
			t.Skipf("could not load config: %v", err)
		}
		if src, err = database.OpenSource(cfg); err != nil {
			t.Skipf("DB_HOST set but source connection failed: %v", err)
		}
		if tgt, err = database.OpenTarget(cfg); err != nil {
			t.Skipf("DB_HOST set but target connection failed: %v", err)
		}
	default:
		t.Skip("set TEST_DATABASE_URL/TEST_TARGET_DATABASE_URL or DB_* to run database integration tests")
	}

	for name, db := range map[string]*gorm.DB{"source": src, "target": tgt} {
		sqlDB, err := db.DB()
		if err != nil {
			t.Skipf("could not access sql.DB (%s): %v", name, err)
		}
		if err := sqlDB.Ping(); err != nil {
			t.Skipf("%s database not reachable: %v", name, err)
		}
		if !tableExists(t, db, "customers") {
			t.Skipf("%s schema not migrated (customers table missing)", name)
		}
	}
	return src, tgt
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

func countWhere(t *testing.T, db *gorm.DB, table, where string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := db.Raw("SELECT COUNT(*) FROM "+table+" WHERE "+where, args...).Scan(&n).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// Fixture identifiers. They use an extreme low UUID prefix so source
// pagination tests find them at the head of the table, and high BIGINTs so
// order item ids never collide with sequence-generated rows.
var (
	srcCustomerID = uuid.MustParse("00000000-0000-4000-8000-0000000000c1")
	srcProductID  = uuid.MustParse("00000000-0000-4000-8000-0000000000c2")
	srcOrderID    = uuid.MustParse("00000000-0000-4000-8000-0000000000c3")
	srcItemID1    = int64(9000000001)
	srcItemID2    = int64(9000000002)

	tgtCustomerID    = uuid.MustParse("00000000-0000-4000-8000-0000000000d1")
	tgtCustomer2ID   = uuid.MustParse("00000000-0000-4000-8000-0000000000d2")
	tgtProductID     = uuid.MustParse("00000000-0000-4000-8000-0000000000d3")
	tgtProductGoodID = uuid.MustParse("00000000-0000-4000-8000-0000000000d4")
	tgtProductBadID  = uuid.MustParse("00000000-0000-4000-8000-0000000000d5")
	tgtProductModID  = uuid.MustParse("00000000-0000-4000-8000-0000000000d6")
	tgtOrderID       = uuid.MustParse("00000000-0000-4000-8000-0000000000d7")
	tgtOrderItemID1  = int64(9100000001)
	tgtOrderItemID2  = int64(9100000002)
)

// cleanupFixtures removes every fixture row from both schemas. Children go
// first so foreign keys are respected.
func cleanupFixtures(t *testing.T, dbs ...*gorm.DB) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		for _, db := range dbs {
			_ = db.WithContext(ctx).Exec(
				"DELETE FROM order_items WHERE id IN ? OR order_id IN ?",
				[]int64{srcItemID1, srcItemID2, tgtOrderItemID1, tgtOrderItemID2},
				[]uuid.UUID{srcOrderID, tgtOrderID},
			).Error
			_ = db.WithContext(ctx).Exec("DELETE FROM orders WHERE id IN ?", []uuid.UUID{srcOrderID, tgtOrderID}).Error
			_ = db.WithContext(ctx).Exec("DELETE FROM products WHERE id IN ?",
				[]uuid.UUID{srcProductID, tgtProductID, tgtProductGoodID, tgtProductBadID, tgtProductModID}).Error
			_ = db.WithContext(ctx).Exec("DELETE FROM customers WHERE id IN ?",
				[]uuid.UUID{srcCustomerID, tgtCustomerID, tgtCustomer2ID}).Error
		}
	})
}
