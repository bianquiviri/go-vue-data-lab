package migrate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/josebianco/go-vue-data-lab/internal/migrate"
)

// writeFiles creates a temp dir populated with the given file contents.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

func TestDiscoverOrdersByVersion(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"002_seeders.up.sql":           "-- up 2",
		"002_seeders.down.sql":         "-- down 2",
		"001_initial.up.sql":           "-- up 1",
		"001_initial.down.sql":         "-- down 1",
		"010_generator_seeds.up.sql":   "-- up 10",
		"010_generator_seeds.down.sql": "-- down 10",
	})

	ms, err := migrate.Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	want := []struct {
		version int
		name    string
		up      string
		down    string
	}{
		{1, "initial", "-- up 1", "-- down 1"},
		{2, "seeders", "-- up 2", "-- down 2"},
		{10, "generator_seeds", "-- up 10", "-- down 10"},
	}
	if len(ms) != len(want) {
		t.Fatalf("got %d migrations, want %d: %+v", len(ms), len(want), ms)
	}
	for i, w := range want {
		if ms[i].Version != w.version || ms[i].Name != w.name {
			t.Errorf("migration[%d] = %03d_%s, want %03d_%s", i, ms[i].Version, ms[i].Name, w.version, w.name)
		}
		if ms[i].UpSQL != w.up {
			t.Errorf("migration[%d] UpSQL = %q, want %q", i, ms[i].UpSQL, w.up)
		}
		if ms[i].DownSQL != w.down {
			t.Errorf("migration[%d] DownSQL = %q, want %q", i, ms[i].DownSQL, w.down)
		}
	}
}

func TestDiscoverIgnoresNonMigrationFiles(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"001_initial.up.sql":   "up",
		"001_initial.down.sql": "down",
		"README.md":            "# docs",
		"001_broken.sql":       "not a migration pair",
		".DS_Store":            "",
	})

	ms, err := migrate.Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(ms) != 1 || ms[0].Version != 1 {
		t.Fatalf("got %+v, want only migration 1", ms)
	}
}

func TestDiscoverErrorsOnMissingDown(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"001_initial.up.sql": "up",
	})

	if _, err := migrate.Discover(dir); err == nil {
		t.Fatal("expected error for up file without down counterpart")
	}
}

func TestDiscoverErrorsOnDuplicateVersion(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"001_initial.up.sql":   "up a",
		"001_initial.down.sql": "down a",
		"001_other.up.sql":     "up b",
		"001_other.down.sql":   "down b",
	})

	if _, err := migrate.Discover(dir); err == nil {
		t.Fatal("expected error for duplicate version 001")
	}
}

func TestDiscoverErrorsOnInvalidFileName(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"initial.up.sql":   "up", // no numeric prefix
		"initial.down.sql": "down",
	})

	if _, err := migrate.Discover(dir); err == nil {
		t.Fatal("expected error for file without numeric version prefix")
	}
}

func TestDiscoverMissingDir(t *testing.T) {
	if _, err := migrate.Discover(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestDiscoverEmptyDirReturnsEmpty(t *testing.T) {
	ms, err := migrate.Discover(t.TempDir())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(ms) != 0 {
		t.Fatalf("got %d migrations, want 0", len(ms))
	}
}

func TestPendingSelectsOnlyNotAppliedInOrder(t *testing.T) {
	all := []migrate.Migration{
		{Version: 1, Name: "a"},
		{Version: 2, Name: "b"},
		{Version: 3, Name: "c"},
		{Version: 4, Name: "d"},
	}

	pending := migrate.Pending(all, []int{2, 4})
	if len(pending) != 2 {
		t.Fatalf("got %d pending, want 2: %+v", len(pending), pending)
	}
	if pending[0].Version != 1 || pending[1].Version != 3 {
		t.Fatalf("pending = [%d %d], want [1 3]", pending[0].Version, pending[1].Version)
	}
}

func TestPendingWithNoneApplied(t *testing.T) {
	all := []migrate.Migration{{Version: 1}, {Version: 2}}
	pending := migrate.Pending(all, nil)
	if len(pending) != 2 {
		t.Fatalf("got %d pending, want 2", len(pending))
	}
}

func TestPendingWithAllApplied(t *testing.T) {
	all := []migrate.Migration{{Version: 1}, {Version: 2}}
	pending := migrate.Pending(all, []int{1, 2})
	if len(pending) != 0 {
		t.Fatalf("got %d pending, want 0", len(pending))
	}
}
