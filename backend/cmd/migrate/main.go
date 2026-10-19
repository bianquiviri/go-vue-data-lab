// Command migrate applies and reverts the versioned SQL migrations of the lab
// against the two independent databases (lab_source and lab_target).
//
// Usage:
//
//	migrate up                       apply all pending migrations (both databases)
//	migrate down                     revert the latest applied migration (both databases)
//	migrate status                   show applied/pending migrations (both databases)
//
// Flags (after the subcommand):
//
//	-db    source|target|all   target database(s), default "all"
//	-dir   <path>              migrations root; auto-detected by default
//	-steps N                   migrations to revert (down only), default 1
//
// Examples:
//
//	go run ./cmd/migrate up
//	go run ./cmd/migrate down -steps 2
//	go run ./cmd/migrate status -db source
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/josebianco/go-vue-data-lab/internal/config"
	"github.com/josebianco/go-vue-data-lab/internal/infrastructure/database"
	"github.com/josebianco/go-vue-data-lab/internal/migrate"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

// options holds the validated CLI parameters.
type options struct {
	command string
	db      string
	dir     string
	steps   int
}

// target is one database to migrate, with its schema directory.
type target struct {
	label  string // "source" or "target"
	dbName string
	dir    string
}

// run is the testable entrypoint: it parses args and executes the command.
func run(args []string, stdout, stderr io.Writer) error {
	opts, err := parseArgs(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(stdout)
			return nil
		}
		return err
	}

	dir, err := migrate.FindDir(opts.dir)
	if err != nil {
		return err
	}
	opts.dir = dir

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("cargar configuración: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	targets := buildTargets(opts, cfg)
	var errs []error
	for _, t := range targets {
		if err := migrateTarget(ctx, opts, t, cfg, stdout); err != nil {
			errs = append(errs, fmt.Errorf("db %s: %w", t.label, err))
		}
	}
	return errors.Join(errs...)
}

// parseArgs validates the subcommand and flags without touching the network.
func parseArgs(args []string, stderr io.Writer) (*options, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("falta el subcomando: usa up, down o status")
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		return nil, flag.ErrHelp
	}

	command := args[0]
	switch command {
	case "up", "down", "status":
	default:
		return nil, fmt.Errorf("subcomando desconocido %q: usa up, down o status", command)
	}

	fs := flag.NewFlagSet("migrate "+command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {} // printUsage renders the full help
	db := fs.String("db", "all", "base de datos destino: source, target o all")
	dir := fs.String("dir", "", "directorio raíz de migraciones (autodetectado por defecto)")
	steps := fs.Int("steps", 1, "número de migraciones a revertir (solo down)")

	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, flag.ErrHelp
		}
		return nil, fmt.Errorf("parsear flags: %w", err)
	}

	switch *db {
	case "source", "target", "all":
	default:
		return nil, fmt.Errorf("valor inválido para -db: %q (usa source, target o all)", *db)
	}
	if *steps < 1 {
		return nil, fmt.Errorf("valor inválido para -steps: %d (debe ser >= 1)", *steps)
	}

	return &options{command: command, db: *db, dir: *dir, steps: *steps}, nil
}

// buildTargets resolves which databases the command applies to.
func buildTargets(opts *options, cfg *config.Config) []target {
	source := target{label: "source", dbName: cfg.DBSourceName, dir: filepath.Join(opts.dir, "source")}
	tgt := target{label: "target", dbName: cfg.DBTargetName, dir: filepath.Join(opts.dir, "target")}

	switch opts.db {
	case "source":
		return []target{source}
	case "target":
		return []target{tgt}
	default:
		return []target{source, tgt}
	}
}

// migrateTarget runs the requested command against a single database.
func migrateTarget(ctx context.Context, opts *options, t target, cfg *config.Config, stdout io.Writer) error {
	migrations, err := migrate.Discover(t.dir)
	if err != nil {
		return err
	}
	if len(migrations) == 0 {
		fmt.Fprintf(stdout, "[%s] %s: sin archivos de migración en %s\n", t.label, t.dbName, t.dir)
		return nil
	}

	gormDB, err := database.Open(cfg, t.dbName)
	if err != nil {
		return fmt.Errorf("abrir conexión a %q: %w", t.dbName, err)
	}
	defer func() { _ = database.Close(gormDB) }()

	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("obtener *sql.DB de %q: %w", t.dbName, err)
	}

	runner := migrate.NewRunner(
		migrate.NewSQLStore(sqlDB),
		migrate.NewSQLTracker(sqlDB),
		func(format string, args ...any) {
			fmt.Fprintf(stdout, "[%s] "+format+"\n", append([]any{t.label}, args...)...)
		},
	)

	fmt.Fprintf(stdout, "[%s] %s (%s)\n", t.label, t.dbName, t.dir)

	switch opts.command {
	case "up":
		done, err := runner.Up(ctx, migrations)
		return printResult(stdout, t, "aplicadas", done, err)
	case "down":
		done, err := runner.Down(ctx, migrations, opts.steps)
		return printResult(stdout, t, "revertidas", done, err)
	case "status":
		return printStatus(ctx, stdout, t, runner, migrations)
	default:
		return fmt.Errorf("subcomando no soportado %q", opts.command)
	}
}

// printResult renders the migrations applied or reverted for one database.
func printResult(stdout io.Writer, t target, verb string, done []migrate.Migration, err error) error {
	if err != nil {
		return err
	}
	if len(done) == 0 {
		fmt.Fprintf(stdout, "[%s] sin cambios\n", t.label)
		return nil
	}
	labels := make([]string, len(done))
	for i, m := range done {
		labels[i] = m.Label()
	}
	fmt.Fprintf(stdout, "[%s] %s %d migración(es): %v\n", t.label, verb, len(done), labels)
	return nil
}

// printStatus renders the applied/pending state of every migration.
func printStatus(ctx context.Context, stdout io.Writer, t target, runner *migrate.Runner, migrations []migrate.Migration) error {
	rows, err := runner.Status(ctx, migrations)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Applied {
			fmt.Fprintf(stdout, "[%s]   aplicada  %03d_%s (%s)\n", t.label, row.Version, row.Name, row.AppliedAt)
		} else {
			fmt.Fprintf(stdout, "[%s]   pendiente %03d_%s\n", t.label, row.Version, row.Name)
		}
	}
	return nil
}

// printUsage writes the command help.
func printUsage(w io.Writer) {
	fmt.Fprint(w, `migrate - versioned SQL migration runner (lab_source / lab_target)

Usage:
  migrate up                       apply all pending migrations
  migrate down                     revert the latest applied migration
  migrate status                   show applied/pending migrations

Flags (after the subcommand):
  -db    source|target|all   target database(s), default "all"
  -dir   <path>              migrations root; auto-detected by default
  -steps N                   migrations to revert (down only), default 1

Examples:
  go run ./cmd/migrate up
  go run ./cmd/migrate status -db source
  go run ./cmd/migrate down -steps 2
`)
}
