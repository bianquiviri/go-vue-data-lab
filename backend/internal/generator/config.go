// Package generator creates synthetic e-commerce data (customers, products,
// orders and order items) for the lab_source schema.
//
// The package is deliberately standalone: the pure generation logic depends
// only on an explicit math/rand source and an injectable Store, so it can be
// unit tested without a database and can be reused by the HTTP layer, the
// scheduler or the CLI without pulling in the worker pool.
//
// # Determinism
//
// Given the same seed and counts, Generate always produces exactly the same
// records (including UUIDs and timestamps). Every value is a pure function of
// (seed, namespace, index): identifiers are derived with SHA-1 UUIDs, random
// choices come from a per-record math/rand source seeded by a SHA-256 hash, and
// the wall clock is never read. This also lets records be generated in batches
// without holding the whole dataset in memory.
//
// # Idempotency
//
// Re-running the generator never creates duplicates. Two complementary
// mechanisms are used:
//
//  1. Reset (default, Config.Reset=true): the four source tables are truncated
//     (RESTART IDENTITY CASCADE) before generation, so a run always yields
//     exactly the requested dataset.
//  2. Deterministic keys + ON CONFLICT DO NOTHING (append mode, Reset=false):
//     customers/products/orders use deterministic primary keys and natural keys
//     (email, sku, order_number). Inserts ignore rows that already exist, and
//     order items are only written for orders that were actually inserted on
//     this run. Re-running the same seed/counts is therefore a no-op.
//
// In addition, a generator_seeds marker table records each completed dataset
// (keyed by Config.DatasetKey). With Config.SkipIfExists the whole run is
// skipped when the dataset is already present.
package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Batch-size bounds. Large volumes are always written in chunks.
const (
	// DefaultBatchSize is used when Config.BatchSize is not set. It sits inside
	// the 3,000–4,000 range recommended for bulk inserts.
	DefaultBatchSize = 3000
	// MaxBatchSize guards against the caller asking for absurdly large chunks.
	MaxBatchSize = 10000
)

// Entity names passed to the progress callback.
const (
	EntityCustomers  = "customers"
	EntityProducts   = "products"
	EntityOrders     = "orders"
	EntityOrderItems = "order_items"
)

// Config controls how much data is generated and how it is persisted.
type Config struct {
	// Seed makes a run reproducible. Same seed + same counts ⇒ same bytes.
	Seed int64
	// Customers is the number of customer rows to generate.
	Customers int
	// Products is the number of product rows to generate.
	Products int
	// Orders is the number of order rows to generate (each with its items).
	Orders int
	// MinItems / MaxItems bound the number of items per order (inclusive).
	MinItems int
	MaxItems int
	// BatchSize is the number of rows per bulk insert.
	BatchSize int
	// Reset, when true (the default), truncates the four source tables before
	// generating so a re-run never duplicates rows.
	Reset bool
	// SkipIfExists, when true, makes Generate a no-op if the exact dataset
	// (seed + counts) has already been recorded in the marker table.
	SkipIfExists bool
}

// DefaultConfig returns a sane starting point for a local run.
func DefaultConfig() Config {
	return Config{
		Seed:      1,
		Customers: 1000,
		Products:  500,
		Orders:    2000,
		MinItems:  1,
		MaxItems:  5,
		BatchSize: DefaultBatchSize,
		Reset:     true,
	}
}

// Normalize fills zero-valued fields with defaults so callers can start from a
// zero Config. Negative values are left untouched for Validate to reject.
func (c Config) Normalize() Config {
	if c.BatchSize == 0 {
		c.BatchSize = DefaultBatchSize
	}
	if c.MinItems == 0 {
		c.MinItems = 1
	}
	if c.MaxItems == 0 {
		c.MaxItems = c.MinItems
	}
	return c
}

// Validate reports whether the configuration can produce a consistent dataset.
func (c Config) Validate() error {
	if c.Customers < 0 {
		return fmt.Errorf("generator: customers must be >= 0, got %d", c.Customers)
	}
	if c.Products < 0 {
		return fmt.Errorf("generator: products must be >= 0, got %d", c.Products)
	}
	if c.Orders < 0 {
		return fmt.Errorf("generator: orders must be >= 0, got %d", c.Orders)
	}
	if c.MinItems < 1 {
		return fmt.Errorf("generator: min items per order must be >= 1, got %d", c.MinItems)
	}
	if c.MaxItems < c.MinItems {
		return fmt.Errorf("generator: max items per order (%d) must be >= min items (%d)", c.MaxItems, c.MinItems)
	}
	if c.BatchSize < 1 {
		return fmt.Errorf("generator: batch size must be >= 1, got %d", c.BatchSize)
	}
	if c.BatchSize > MaxBatchSize {
		return fmt.Errorf("generator: batch size must be <= %d, got %d", MaxBatchSize, c.BatchSize)
	}
	// Referential integrity: orders need customers and items need products.
	if c.Orders > 0 && c.Customers < 1 {
		return fmt.Errorf("generator: at least 1 customer is required to generate orders")
	}
	if c.Orders > 0 && c.Products < 1 {
		return fmt.Errorf("generator: at least 1 product is required to generate order items")
	}
	return nil
}

// DatasetKey returns a stable fingerprint of the dataset shape (seed + counts).
// Re-running with the same key produces exactly the same rows.
func (c Config) DatasetKey() string {
	canonical := fmt.Sprintf("v1|seed=%d|customers=%d|products=%d|orders=%d|min_items=%d|max_items=%d",
		c.Seed, c.Customers, c.Products, c.Orders, c.MinItems, c.MaxItems)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

// ProgressFunc is invoked while data is generated. "done" is the number of
// records processed so far for "entity" and "total" is the planned total. It is
// always called with done <= total and total == 0 only when the total is
// genuinely unknown (never with the built-in generators).
type ProgressFunc func(entity string, done, total int64)

// Summary describes the outcome of a generation run.
type Summary struct {
	DatasetKey string
	Seed       int64
	Customers  int64
	Products   int64
	Orders     int64
	OrderItems int64
	// Skipped is true when SkipIfExists found the dataset already recorded.
	Skipped bool
}
