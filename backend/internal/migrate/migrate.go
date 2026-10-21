// Package migrate implements a small, dependency-free SQL migration runner
// for the two independent databases of the lab (lab_source and lab_target).
//
// Migrations are plain versioned SQL files (001_name.up.sql / 001_name.down.sql)
// discovered in lexicographic order. Applied versions are tracked in a
// schema_migrations table inside each database, so `up` is idempotent and
// `down` reverts the latest applied migrations in reverse order.
//
// Schema management is handled exclusively by these versioned SQL files;
// this package never calls AutoMigrate.
package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Migration is a single versioned pair of SQL scripts.
type Migration struct {
	Version int    // numeric prefix, e.g. 1 for "001_initial.up.sql"
	Name    string // textual part, e.g. "initial"
	UpSQL   string // contents of the .up.sql file
	DownSQL string // contents of the .down.sql file
}

// Label returns the canonical "NNN_name" identifier of the migration.
func (m Migration) Label() string {
	return fmt.Sprintf("%03d_%s", m.Version, m.Name)
}

// Applied is a row of the schema_migrations tracking table.
type Applied struct {
	Version   int
	Name      string
	AppliedAt string // RFC3339 timestamp as stored by PostgreSQL
}

// migrationFileRe matches "001_name.up.sql" / "001_name.down.sql".
var migrationFileRe = regexp.MustCompile(`^(\d+)_([A-Za-z0-9][A-Za-z0-9_-]*)\.(up|down)\.sql$`)

// Discover reads dir and returns all migration pairs ordered by ascending
// version. Files that do not end in .up.sql / .down.sql are ignored. Every
// .up.sql must have a matching .down.sql (and vice versa), and versions must
// be unique; otherwise an error is returned.
func Discover(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("descubrir migraciones en %q: %w", dir, err)
	}

	byVersion := make(map[int]*Migration)
	var order []int

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		m := migrationFileRe.FindStringSubmatch(entry.Name())
		if m == nil {
			if strings.HasSuffix(entry.Name(), ".up.sql") || strings.HasSuffix(entry.Name(), ".down.sql") {
				return nil, fmt.Errorf("nombre de migración inválido: %q (esperado NNN_nombre.up.sql)", entry.Name())
			}
			continue // not a migration file (README, .DS_Store, ...)
		}

		version, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, fmt.Errorf("versión inválida en %q: %w", entry.Name(), err)
		}

		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("leer migración %q: %w", entry.Name(), err)
		}

		mig, seen := byVersion[version]
		if !seen {
			mig = &Migration{Version: version, Name: m[2]}
			byVersion[version] = mig
			order = append(order, version)
		} else if mig.Name != m[2] {
			return nil, fmt.Errorf("versión duplicada %03d: %q y %q", version, mig.Name, m[2])
		}

		switch m[3] {
		case "up":
			mig.UpSQL = string(content)
		case "down":
			mig.DownSQL = string(content)
		}
	}

	sort.Ints(order)
	migrations := make([]Migration, 0, len(order))
	for _, v := range order {
		mig := byVersion[v]
		if strings.TrimSpace(mig.UpSQL) == "" {
			return nil, fmt.Errorf("migración %03d_%s: falta el archivo .up.sql", v, mig.Name)
		}
		if strings.TrimSpace(mig.DownSQL) == "" {
			return nil, fmt.Errorf("migración %03d_%s: falta el archivo .down.sql", v, mig.Name)
		}
		migrations = append(migrations, *mig)
	}

	return migrations, nil
}

// Pending returns the migrations whose version is not present in applied,
// preserving ascending order.
func Pending(all []Migration, applied []int) []Migration {
	appliedSet := make(map[int]bool, len(applied))
	for _, v := range applied {
		appliedSet[v] = true
	}

	var pending []Migration
	for _, m := range all {
		if !appliedSet[m.Version] {
			pending = append(pending, m)
		}
	}
	return pending
}

// ToRevert selects the migrations to roll back: the applied ones with the
// highest versions first, at most steps of them. It fails if an applied
// version has no corresponding files on disk (the files must never disappear
// before the database has been rolled back).
func ToRevert(all []Migration, applied []Applied, steps int) ([]Migration, error) {
	if steps < 1 {
		return nil, fmt.Errorf("steps debe ser >= 1, got %d", steps)
	}

	byVersion := make(map[int]Migration, len(all))
	for _, m := range all {
		byVersion[m.Version] = m
	}

	sorted := make([]Applied, len(applied))
	copy(sorted, applied)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Version > sorted[j].Version })

	var revert []Migration
	for i, a := range sorted {
		if i >= steps {
			break
		}
		m, ok := byVersion[a.Version]
		if !ok {
			return nil, fmt.Errorf("la migración aplicada %03d_%s no tiene archivos en disco", a.Version, a.Name)
		}
		revert = append(revert, m)
	}
	return revert, nil
}

// FindDir resolves the migrations root directory. An explicit path always
// wins. Otherwise the current working directory and its ancestors are
// searched for a "migrations" directory (or "backend/migrations", so the
// tool works both from backend/ and from the repository root).
func FindDir(explicit string) (string, error) {
	if explicit != "" {
		if !isMigrationsDir(explicit) {
			return "", fmt.Errorf("el directorio %q no contiene subdirectorios source/ ni target/", explicit)
		}
		return explicit, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("obtener directorio de trabajo: %w", err)
	}

	dir := wd
	for {
		for _, candidate := range []string{
			filepath.Join(dir, "migrations"),
			filepath.Join(dir, "backend", "migrations"),
		} {
			if isMigrationsDir(candidate) {
				return candidate, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no se encontró el directorio de migraciones desde %q (usa -dir)", wd)
}

// isMigrationsDir reports whether dir looks like the migrations root.
func isMigrationsDir(dir string) bool {
	for _, sub := range []string{"source", "target"} {
		info, err := os.Stat(filepath.Join(dir, sub))
		if err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}
