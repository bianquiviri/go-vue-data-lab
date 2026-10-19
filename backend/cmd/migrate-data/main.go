// Command migrate-data copies the synthetic e-commerce data from lab_source
// into lab_target using the etl package.
//
// It performs an idempotent, resumable, concurrent migration: batches are
// upserted with INSERT ... ON CONFLICT, the checkpoint is persisted to a JSON
// file, and a failed batch never aborts the whole run unless -fail-fast is set.
//
// Examples:
//
//	go run ./cmd/migrate-data -workers 8 -batch-size 3500
//	go run ./cmd/migrate-data -entities customers,products -strict-totals
//	go run ./cmd/migrate-data -checkpoint logs/etl.json -reset-checkpoint
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/josebianco/go-vue-data-lab/internal/config"
	"github.com/josebianco/go-vue-data-lab/internal/etl"
	etlpostgres "github.com/josebianco/go-vue-data-lab/internal/etl/postgres"
	"github.com/josebianco/go-vue-data-lab/internal/infrastructure/database"
	"github.com/josebianco/go-vue-data-lab/internal/infrastructure/logger"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "migrate-data: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("migrate-data", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		workers         = fs.Int("workers", 0, "concurrent load workers (0 = GOMAXPROCS)")
		batchSize       = fs.Int("batch-size", etl.DefaultBatchSize, "rows per bulk upsert")
		extractSize     = fs.Int("extract-size", etl.DefaultExtractSize, "rows per source query")
		failFast        = fs.Bool("fail-fast", false, "abort on the first failed batch")
		strictTotals    = fs.Bool("strict-totals", false, "skip orders/items whose totals do not match instead of correcting")
		entitiesFlag    = fs.String("entities", "", "comma-separated subset: customers,products,orders,order_items (default all)")
		checkpointPath  = fs.String("checkpoint", "logs/etl-checkpoint.json", "checkpoint file path")
		resetCheckpoint = fs.Bool("reset-checkpoint", false, "delete the checkpoint and start from scratch")
		quiet           = fs.Bool("quiet", false, "do not print progress updates")
	)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse flags: %w", err)
	}

	entities, err := parseEntities(*entitiesFlag)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := buildLogger(cfg, *quiet)
	if err != nil {
		return err
	}
	defer func() { _ = log.Sync() }()

	if *resetCheckpoint {
		if err := os.Remove(*checkpointPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("reset checkpoint: %w", err)
		}
	}

	srcDB, err := database.OpenSource(cfg)
	if err != nil {
		return fmt.Errorf("open source database: %w", err)
	}
	defer func() { _ = database.Close(srcDB) }()

	tgtDB, err := database.OpenTarget(cfg)
	if err != nil {
		return fmt.Errorf("open target database: %w", err)
	}
	defer func() { _ = database.Close(tgtDB) }()

	src, err := etlpostgres.NewSource(srcDB)
	if err != nil {
		return fmt.Errorf("build source adapter: %w", err)
	}
	loader, err := etlpostgres.NewLoader(tgtDB)
	if err != nil {
		return fmt.Errorf("build loader adapter: %w", err)
	}
	store := etl.NewFileStore(*checkpointPath)

	var printer *progressPrinter
	var onProgress func(etl.Progress)
	if !*quiet {
		printer = &progressPrinter{}
		onProgress = printer.update
	}

	runner, err := etl.New(etl.Config{
		Workers:      *workers,
		BatchSize:    *batchSize,
		ExtractSize:  *extractSize,
		FailFast:     *failFast,
		StrictTotals: *strictTotals,
		Entities:     entities,
		OnProgress:   onProgress,
	}, src, loader, etl.WithLogger(log), etl.WithCheckpointStore(store))
	if err != nil {
		return fmt.Errorf("build etl runner: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	started := time.Now()
	res, runErr := runner.Run(ctx)
	if printer != nil {
		printer.finish()
	}

	if res != nil {
		printSummary(os.Stdout, res, time.Since(started))
	}
	if runErr != nil {
		return fmt.Errorf("run etl: %w", runErr)
	}
	if res != nil && res.HasErrors() {
		return fmt.Errorf("%d lote(s) fallaron; revisa los logs y reintenta (el checkpoint no avanzó más allá del último lote correcto)", len(res.Errors))
	}
	return nil
}

// parseEntities converts the -entities flag into etl.Entity values.
func parseEntities(spec string) ([]etl.Entity, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	parts := strings.Split(spec, ",")
	out := make([]etl.Entity, 0, len(parts))
	for _, p := range parts {
		e, err := etl.ParseEntity(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// buildLogger returns the structured logger, or a no-op logger when quiet.
func buildLogger(cfg *config.Config, quiet bool) (*zap.Logger, error) {
	if quiet {
		return zap.NewNop(), nil
	}
	log, err := logger.New(cfg.LogLevel, cfg.LogPath)
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}
	return log, nil
}

// printSummary renders the final per-entity report.
func printSummary(w io.Writer, res *etl.Result, elapsed time.Duration) {
	fmt.Fprintln(w, "migración completa")
	fmt.Fprintf(w, "  %-12s %10s %10s %10s %8s %8s %8s\n", "entity", "read", "inserted", "updated", "skipped", "failed", "corrected")
	for _, e := range etl.AllEntities {
		m, ok := res.Metrics[e]
		if !ok {
			continue
		}
		fmt.Fprintf(w, "  %-12s %10d %10d %10d %8d %8d %8d\n",
			e, m.Read, m.Inserted, m.Updated, m.Skipped, m.Failed, m.Corrected)
	}
	total := res.Metrics.Total()
	fmt.Fprintf(w, "  %-12s %10d %10d %10d %8d %8d %8d\n",
		"TOTAL", total.Read, total.Inserted, total.Updated, total.Skipped, total.Failed, total.Corrected)
	fmt.Fprintf(w, "  elapsed      : %s\n", elapsed.Round(time.Millisecond))

	if len(res.Skipped) > 0 {
		const maxPrint = 10
		fmt.Fprintf(w, "  skipped rows (showing %d of %d):\n", min(len(res.Skipped), maxPrint), len(res.Skipped))
		for i, s := range res.Skipped {
			if i >= maxPrint {
				break
			}
			fmt.Fprintf(w, "    - %s %s: %s\n", s.Entity, s.Key, s.Reason)
		}
	}
	if len(res.Errors) > 0 {
		fmt.Fprintf(w, "  failed batches: %d\n", len(res.Errors))
	}
}

// progressPrinter renders a compact per-entity progress line.
type progressPrinter struct {
	mu       sync.Mutex
	lastLine string
}

func (p *progressPrinter) update(pr etl.Progress) {
	p.mu.Lock()
	defer p.mu.Unlock()
	line := fmt.Sprintf("%-12s inserted=%d updated=%d skipped=%d failed=%d",
		pr.Entity, pr.Metrics.Inserted, pr.Metrics.Updated, pr.Metrics.Skipped, pr.Metrics.Failed)
	if len(line) < len(p.lastLine) {
		line += strings.Repeat(" ", len(p.lastLine)-len(line))
	}
	fmt.Fprintf(os.Stderr, "\r%s", line)
	p.lastLine = line
}

func (p *progressPrinter) finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lastLine != "" {
		fmt.Fprintln(os.Stderr)
	}
}
