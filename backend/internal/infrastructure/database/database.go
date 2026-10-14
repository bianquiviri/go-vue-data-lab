// Package database opens and manages the GORM connections to the two
// independent PostgreSQL schemas used by the lab: lab_source and lab_target.
//
// Schema management is handled exclusively by versioned SQL migrations; this
// package never calls AutoMigrate.
package database

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/josebianco/go-vue-data-lab/internal/config"
)

// Connection pool defaults.
const (
	maxOpenConns    = 25
	maxIdleConns    = 5
	connMaxLifetime = 30 * time.Minute
	connMaxIdleTime = 5 * time.Minute
)

// Open opens a GORM connection to the given database name using cfg.
func Open(cfg *config.Config, dbName string) (*gorm.DB, error) {
	if cfg == nil {
		return nil, fmt.Errorf("database: nil config")
	}

	db, err := gorm.Open(postgres.Open(buildDSN(cfg, dbName)), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("database: open %q: %w", dbName, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("database: get sql.DB for %q: %w", dbName, err)
	}

	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

	return db, nil
}

// OpenSource opens the connection to the lab_source schema.
func OpenSource(cfg *config.Config) (*gorm.DB, error) {
	return Open(cfg, cfg.DBSourceName)
}

// OpenTarget opens the connection to the lab_target schema.
func OpenTarget(cfg *config.Config) (*gorm.DB, error) {
	return Open(cfg, cfg.DBTargetName)
}

// Close releases the underlying database/sql connection pool.
func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("database: get sql.DB for close: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("database: close: %w", err)
	}
	return nil
}

// buildDSN assembles a PostgreSQL DSN from the configuration.
func buildDSN(cfg *config.Config, dbName string) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, dbName,
	)
}
