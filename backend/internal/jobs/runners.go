package jobs

import (
	"context"
	"fmt"
	"strings"

	"github.com/josebianco/go-vue-data-lab/internal/etl"
	"github.com/josebianco/go-vue-data-lab/internal/generator"
)

// GenerateParams are the parameters of a "generate" job. Zero values fall back
// to the generator defaults (see Normalize); negative values are rejected.
type GenerateParams struct {
	Seed         int64 `json:"seed"`
	Customers    int   `json:"customers"`
	Products     int   `json:"products"`
	Orders       int   `json:"orders"`
	MinItems     int   `json:"min_items"`
	MaxItems     int   `json:"max_items"`
	BatchSize    int   `json:"batch_size"`
	Reset        *bool `json:"reset"`
	SkipIfExists bool  `json:"skip_if_exists"`
}

// Normalize fills zero-valued fields with the generator defaults. Reset is
// tri-state: nil means "use the default" (true).
func (p GenerateParams) Normalize() GenerateParams {
	d := generator.DefaultConfig()
	if p.Seed == 0 {
		p.Seed = d.Seed
	}
	if p.Customers == 0 {
		p.Customers = d.Customers
	}
	if p.Products == 0 {
		p.Products = d.Products
	}
	if p.Orders == 0 {
		p.Orders = d.Orders
	}
	if p.MinItems == 0 {
		p.MinItems = d.MinItems
	}
	if p.MaxItems == 0 {
		p.MaxItems = d.MaxItems
	}
	if p.BatchSize == 0 {
		p.BatchSize = d.BatchSize
	}
	if p.Reset == nil {
		reset := d.Reset
		p.Reset = &reset
	}
	return p
}

// toConfig converts the normalized parameters into a generator.Config.
func (p GenerateParams) toConfig() generator.Config {
	return generator.Config{
		Seed:         p.Seed,
		Customers:    p.Customers,
		Products:     p.Products,
		Orders:       p.Orders,
		MinItems:     p.MinItems,
		MaxItems:     p.MaxItems,
		BatchSize:    p.BatchSize,
		Reset:        p.Reset != nil && *p.Reset,
		SkipIfExists: p.SkipIfExists,
	}
}

// Validate reports whether the parameters can produce a consistent dataset.
func (p GenerateParams) Validate() error {
	if err := p.Normalize().toConfig().Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidParams, err)
	}
	return nil
}

// NewGenerateRunner builds a RunnerFunc that generates synthetic data into the
// given store.
func NewGenerateRunner(store generator.Store) RunnerFunc {
	return func(params any) (JobFunc, error) {
		p, err := decodeParams[GenerateParams](params)
		if err != nil {
			return nil, err
		}
		p = p.Normalize()
		if err := p.Validate(); err != nil {
			return nil, err
		}
		gen, err := generator.New(p.toConfig())
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidParams, err)
		}
		return func(ctx context.Context, report func(EntityProgress)) (map[string]any, error) {
			summary, err := gen.Generate(ctx, store, func(entity string, done, total int64) {
				report(EntityProgress{Entity: entity, Done: done, Total: total, Processed: done})
			})
			if err != nil {
				return nil, fmt.Errorf("jobs: generate: %w", err)
			}
			return map[string]any{
				"dataset_key": summary.DatasetKey,
				"seed":        summary.Seed,
				"customers":   summary.Customers,
				"products":    summary.Products,
				"orders":      summary.Orders,
				"order_items": summary.OrderItems,
				"skipped":     summary.Skipped,
			}, nil
		}, nil
	}
}

// MigrateParams are the parameters of a "migrate" job. Zero values fall back to
// the ETL defaults; an empty Entities list migrates every entity.
type MigrateParams struct {
	Entities     []string `json:"entities"`
	Workers      int      `json:"workers"`
	BatchSize    int      `json:"batch_size"`
	ExtractSize  int      `json:"extract_size"`
	FailFast     bool     `json:"fail_fast"`
	StrictTotals bool     `json:"strict_totals"`
}

// Validate reports whether the parameters reference known entities and use
// non-negative sizes.
func (p MigrateParams) Validate() error {
	if p.Workers < 0 {
		return fmt.Errorf("%w: workers must be >= 0", ErrInvalidParams)
	}
	if p.BatchSize < 0 {
		return fmt.Errorf("%w: batch_size must be >= 0", ErrInvalidParams)
	}
	if p.ExtractSize < 0 {
		return fmt.Errorf("%w: extract_size must be >= 0", ErrInvalidParams)
	}
	for _, e := range p.Entities {
		if _, err := etl.ParseEntity(strings.TrimSpace(e)); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidParams, err)
		}
	}
	return nil
}

// toConfig converts the parameters into an etl.Config.
func (p MigrateParams) toConfig() etl.Config {
	entities := make([]etl.Entity, 0, len(p.Entities))
	for _, e := range p.Entities {
		if ent, err := etl.ParseEntity(strings.TrimSpace(e)); err == nil {
			entities = append(entities, ent)
		}
	}
	return etl.Config{
		Workers:      p.Workers,
		BatchSize:    p.BatchSize,
		ExtractSize:  p.ExtractSize,
		FailFast:     p.FailFast,
		StrictTotals: p.StrictTotals,
		Entities:     entities,
	}
}

// NewMigrateRunner builds a RunnerFunc that migrates lab_source into lab_target
// through the ETL runner.
func NewMigrateRunner(src etl.Source, loader etl.Loader, opts ...etl.Option) RunnerFunc {
	return func(params any) (JobFunc, error) {
		p, err := decodeParams[MigrateParams](params)
		if err != nil {
			return nil, err
		}
		if err := p.Validate(); err != nil {
			return nil, err
		}
		return func(ctx context.Context, report func(EntityProgress)) (map[string]any, error) {
			cfg := p.toConfig()
			cfg.OnProgress = func(pr etl.Progress) {
				report(EntityProgress{
					Entity:    string(pr.Entity),
					Done:      pr.Metrics.Processed,
					Total:     pr.Total,
					Processed: pr.Metrics.Processed,
					Failed:    pr.Metrics.Failed,
					Skipped:   pr.Metrics.Skipped,
				})
			}
			runner, err := etl.New(cfg, src, loader, opts...)
			if err != nil {
				return nil, fmt.Errorf("jobs: migrate: %w", err)
			}
			res, err := runner.Run(ctx)
			if err != nil {
				return nil, fmt.Errorf("jobs: migrate: %w", err)
			}
			metrics := map[string]any{
				"duration_ms":    res.Duration.Milliseconds(),
				"failed_batches": len(res.Errors),
				"skipped_rows":   len(res.Skipped),
			}
			for entity, m := range res.Metrics {
				metrics[string(entity)] = m
			}
			metrics["total"] = res.Metrics.Total()
			if res.HasErrors() {
				return metrics, fmt.Errorf("jobs: migrate: %d lote(s) fallaron", len(res.Errors))
			}
			return metrics, nil
		}, nil
	}
}

// decodeParams accepts nil, T or *T and rejects anything else with
// ErrInvalidParams.
func decodeParams[T any](params any) (T, error) {
	var zero T
	switch v := params.(type) {
	case nil:
		return zero, nil
	case T:
		return v, nil
	case *T:
		if v == nil {
			return zero, nil
		}
		return *v, nil
	default:
		return zero, fmt.Errorf("%w: expected %T, got %T", ErrInvalidParams, zero, params)
	}
}
