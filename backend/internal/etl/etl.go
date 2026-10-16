// Package etl implements the data migration (ETL) pipeline that copies the
// synthetic e-commerce data from the lab_source schema into lab_target.
//
// The package is split into pure, testable pieces and explicit ports:
//
//   - transform.go holds the pure normalize/validate/deduplicate logic.
//   - checkpoint.go models resumable state and the stores that persist it.
//   - tracker.go guarantees the checkpoint only advances past contiguous,
//     fully successful batches.
//   - runner.go orchestrates extract -> transform -> load using the generic
//     worker pool from internal/worker.
//   - ports.go defines the Source and Loader interfaces; the GORM/SQL-Raw
//     adapters live in the etl/postgres subpackage.
//
// No stage depends on GORM, so orchestration is unit tested with fakes.
package etl

import (
	"fmt"
	"runtime"
	"sort"

	"github.com/josebianco/go-vue-data-lab/internal/worker"
)

// Entity identifies one of the four migratable tables.
type Entity string

// The migratable entities. The string values are used in checkpoints, logs and
// progress reports.
const (
	EntityCustomers  Entity = "customers"
	EntityProducts   Entity = "products"
	EntityOrders     Entity = "orders"
	EntityOrderItems Entity = "order_items"
)

// AllEntities is the canonical migration order. It respects foreign keys:
// customers and products precede orders, and orders precede order_items.
var AllEntities = []Entity{EntityCustomers, EntityProducts, EntityOrders, EntityOrderItems}

// entityOrder maps an entity to its canonical position.
var entityOrder = map[Entity]int{
	EntityCustomers:  0,
	EntityProducts:   1,
	EntityOrders:     2,
	EntityOrderItems: 3,
}

// Defaults. Batch sizes stay inside the 3000-4000 range recommended for bulk
// PostgreSQL statements.
const (
	// DefaultBatchSize is the number of rows pushed per bulk upsert.
	DefaultBatchSize = worker.DefaultBatchSize
	// DefaultExtractSize is the number of rows read per source query.
	DefaultExtractSize = worker.DefaultBatchSize
	// DefaultMaxSkipped caps how many skip details are retained in a Result.
	DefaultMaxSkipped = 10000
)

// ParseEntity converts a string into a known Entity.
func ParseEntity(s string) (Entity, error) {
	e := Entity(s)
	if _, ok := entityOrder[e]; !ok {
		return "", fmt.Errorf("etl: entidad desconocida %q", s)
	}
	return e, nil
}

// Config controls a Runner.
type Config struct {
	// Workers is the number of concurrent load workers. Defaults to GOMAXPROCS.
	Workers int
	// BatchSize is the number of rows per bulk upsert. Defaults to
	// DefaultBatchSize (3500).
	BatchSize int
	// ExtractSize is the number of rows read per source query. Defaults to
	// DefaultExtractSize (3500).
	ExtractSize int
	// FailFast aborts the whole run on the first failed batch. When false a
	// failed batch is rolled back and recorded, and the run continues.
	FailFast bool
	// StrictTotals skips orders/order_items whose stored totals do not match
	// the recomputed values instead of correcting them.
	StrictTotals bool
	// Entities restricts the run to a subset (in any order; the canonical order
	// is always enforced). Empty means AllEntities.
	Entities []Entity
	// MaxSkipped caps the number of detailed SkipRecord entries kept in the
	// Result. Metrics always count every skipped row. Defaults to
	// DefaultMaxSkipped.
	MaxSkipped int
	// OnProgress, when set, is invoked after each loaded batch. It must be safe
	// for concurrent use.
	OnProgress func(Progress)
}

// applyDefaults fills in zero-valued options.
func (c *Config) applyDefaults() {
	if c.Workers <= 0 {
		c.Workers = runtime.NumCPU()
	}
	if c.BatchSize <= 0 {
		c.BatchSize = DefaultBatchSize
	}
	if c.ExtractSize <= 0 {
		c.ExtractSize = DefaultExtractSize
	}
	if c.MaxSkipped <= 0 {
		c.MaxSkipped = DefaultMaxSkipped
	}
}

// entities returns the configured entities in canonical order, or all of them.
func (c Config) entities() []Entity {
	if len(c.Entities) == 0 {
		return append([]Entity(nil), AllEntities...)
	}
	seen := make(map[Entity]bool, len(c.Entities))
	out := make([]Entity, 0, len(c.Entities))
	for _, e := range c.Entities {
		if !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return entityOrder[out[i]] < entityOrder[out[j]] })
	return out
}

// validateConfig rejects unknown entity names.
func validateConfig(c Config) error {
	for _, e := range c.Entities {
		if _, ok := entityOrder[e]; !ok {
			return fmt.Errorf("etl: entidad desconocida %q", e)
		}
	}
	return nil
}
