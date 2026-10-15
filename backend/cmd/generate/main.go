// Command generate produces synthetic e-commerce data (customers, products,
// orders and order items) in the lab_source schema.
//
// Examples:
//
//	go run ./cmd/generate -customers 10000 -products 2000 -orders 50000 -seed 42
//	go run ./cmd/generate -customers 100 -products 50 -orders 200 -items-per-order 1-5 -reset=false
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/josebianco/go-vue-data-lab/internal/config"
	"github.com/josebianco/go-vue-data-lab/internal/generator"
	"github.com/josebianco/go-vue-data-lab/internal/generator/postgres"
	"github.com/josebianco/go-vue-data-lab/internal/infrastructure/database"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		customers    = fs.Int("customers", generator.DefaultConfig().Customers, "number of customers to generate")
		products     = fs.Int("products", generator.DefaultConfig().Products, "number of products to generate")
		orders       = fs.Int("orders", generator.DefaultConfig().Orders, "number of orders to generate")
		itemsPerOrdr = fs.String("items-per-order", "1-5", "items per order: N or MIN-MAX (inclusive)")
		seed         = fs.Int64("seed", generator.DefaultConfig().Seed, "deterministic seed (same seed + counts = same data)")
		batchSize    = fs.Int("batch-size", generator.DefaultBatchSize, "rows per bulk insert")
		reset        = fs.Bool("reset", true, "truncate the source tables before generating (recommended)")
		skipIfExists = fs.Bool("skip-if-exists", false, "skip entirely if this exact dataset was already generated")
		quiet        = fs.Bool("quiet", false, "do not print progress updates")
	)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse flags: %w", err)
	}

	minItems, maxItems, err := parseItemsPerOrder(*itemsPerOrdr)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	gen, err := generator.New(generator.Config{
		Seed:         *seed,
		Customers:    *customers,
		Products:     *products,
		Orders:       *orders,
		MinItems:     minItems,
		MaxItems:     maxItems,
		BatchSize:    *batchSize,
		Reset:        *reset,
		SkipIfExists: *skipIfExists,
	})
	if err != nil {
		return fmt.Errorf("build generator: %w", err)
	}

	db, err := database.OpenSource(cfg)
	if err != nil {
		return fmt.Errorf("open source database: %w", err)
	}
	defer func() { _ = database.Close(db) }()

	store, err := postgres.NewPostgresStore(db, *batchSize)
	if err != nil {
		return fmt.Errorf("build store: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var progress generator.ProgressFunc
	var printer *progressPrinter
	if !*quiet {
		printer = newProgressPrinter(os.Stderr)
		progress = printer.update
	}

	started := time.Now()
	summary, err := gen.Generate(ctx, store, progress)
	if err != nil {
		return fmt.Errorf("generate dataset: %w", err)
	}
	if printer != nil {
		printer.finish()
	}

	printSummary(os.Stdout, summary, time.Since(started))
	return nil
}

// parseItemsPerOrder parses "N" or "MIN-MAX" (both integers, inclusive).
func parseItemsPerOrder(spec string) (int, int, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return 0, 0, fmt.Errorf("items-per-order must not be empty")
	}
	if i := strings.IndexByte(spec, '-'); i >= 0 {
		minStr := strings.TrimSpace(spec[:i])
		maxStr := strings.TrimSpace(spec[i+1:])
		minItems, err := strconv.Atoi(minStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid items-per-order min %q: %w", minStr, err)
		}
		maxItems, err := strconv.Atoi(maxStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid items-per-order max %q: %w", maxStr, err)
		}
		return minItems, maxItems, nil
	}
	n, err := strconv.Atoi(spec)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid items-per-order %q: %w", spec, err)
	}
	return n, n, nil
}

func printSummary(w io.Writer, s generator.Summary, elapsed time.Duration) {
	fmt.Fprintln(w, "generation complete")
	fmt.Fprintf(w, "  dataset key : %s\n", s.DatasetKey)
	fmt.Fprintf(w, "  seed        : %d\n", s.Seed)
	if s.Skipped {
		fmt.Fprintln(w, "  status      : skipped (dataset already present)")
		return
	}
	fmt.Fprintf(w, "  customers   : %d\n", s.Customers)
	fmt.Fprintf(w, "  products    : %d\n", s.Products)
	fmt.Fprintf(w, "  orders      : %d\n", s.Orders)
	fmt.Fprintf(w, "  order items : %d\n", s.OrderItems)
	fmt.Fprintf(w, "  elapsed     : %s\n", elapsed.Round(time.Millisecond))
}

// progressPrinter renders a compact, single-line-per-entity progress display.
type progressPrinter struct {
	w        io.Writer
	lastLine string
}

func newProgressPrinter(w io.Writer) *progressPrinter {
	return &progressPrinter{w: w}
}

func (p *progressPrinter) update(entity string, done, total int64) {
	line := fmt.Sprintf("%-12s %d/%d", entity, done, total)
	// Pad to overwrite the previous, possibly longer, line.
	if len(line) < len(p.lastLine) {
		line += strings.Repeat(" ", len(p.lastLine)-len(line))
	}
	fmt.Fprintf(p.w, "\r%s", line)
	p.lastLine = line
}

func (p *progressPrinter) finish() {
	if p.lastLine != "" {
		fmt.Fprintln(p.w)
	}
}
