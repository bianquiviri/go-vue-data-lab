package migrate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/josebianco/go-vue-data-lab/internal/migrate"
)

func TestToRevertReturnsLatestAppliedDescending(t *testing.T) {
	all := []migrate.Migration{
		{Version: 1, Name: "a"},
		{Version: 2, Name: "b"},
		{Version: 3, Name: "c"},
	}
	applied := []migrate.Applied{
		{Version: 1, Name: "a"},
		{Version: 2, Name: "b"},
		{Version: 3, Name: "c"},
	}

	revert, err := migrate.ToRevert(all, applied, 2)
	if err != nil {
		t.Fatalf("ToRevert: %v", err)
	}
	if len(revert) != 2 || revert[0].Version != 3 || revert[1].Version != 2 {
		t.Fatalf("revert = %+v, want versions [3 2]", revert)
	}
}

func TestToRevertStepsGreaterThanApplied(t *testing.T) {
	all := []migrate.Migration{{Version: 1, Name: "a"}}
	applied := []migrate.Applied{{Version: 1, Name: "a"}}

	revert, err := migrate.ToRevert(all, applied, 99)
	if err != nil {
		t.Fatalf("ToRevert: %v", err)
	}
	if len(revert) != 1 || revert[0].Version != 1 {
		t.Fatalf("revert = %+v, want [1]", revert)
	}
}

func TestToRevertNothingApplied(t *testing.T) {
	revert, err := migrate.ToRevert([]migrate.Migration{{Version: 1}}, nil, 1)
	if err != nil {
		t.Fatalf("ToRevert: %v", err)
	}
	if len(revert) != 0 {
		t.Fatalf("revert = %+v, want empty", revert)
	}
}

func TestToRevertErrorsWhenAppliedFileMissing(t *testing.T) {
	all := []migrate.Migration{{Version: 1, Name: "a"}}
	applied := []migrate.Applied{
		{Version: 1, Name: "a"},
		{Version: 9, Name: "ghost"},
	}

	_, err := migrate.ToRevert(all, applied, 1)
	if err == nil {
		t.Fatal("expected error for applied migration without files")
	}
	if !strings.Contains(err.Error(), "009_ghost") {
		t.Errorf("error %q should mention the missing version", err)
	}
}

func TestToRevertErrorsOnInvalidSteps(t *testing.T) {
	if _, err := migrate.ToRevert(nil, nil, 0); err == nil {
		t.Fatal("expected error for steps=0")
	}
	if _, err := migrate.ToRevert(nil, nil, -1); err == nil {
		t.Fatal("expected error for steps=-1")
	}
}

func TestFindDirExplicit(t *testing.T) {
	dir := writeFiles(t, map[string]string{})
	if err := os.MkdirAll(filepath.Join(dir, "source"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "target"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := migrate.FindDir(dir)
	if err != nil {
		t.Fatalf("FindDir: %v", err)
	}
	if got != dir {
		t.Fatalf("FindDir = %q, want %q", got, dir)
	}
}

func TestFindDirExplicitInvalid(t *testing.T) {
	if _, err := migrate.FindDir(t.TempDir()); err == nil {
		t.Fatal("expected error for dir without source/target subdirs")
	}
}

func TestFindDirAutoDetectsFromDescendant(t *testing.T) {
	root := t.TempDir()
	migDir := filepath.Join(root, "backend", "migrations")
	for _, sub := range []string{"source", "target"} {
		if err := os.MkdirAll(filepath.Join(migDir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	deep := filepath.Join(root, "backend", "cmd", "migrate")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(deep); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	got, err := migrate.FindDir("")
	if err != nil {
		t.Fatalf("FindDir: %v", err)
	}
	// os.Getwd() resolves symlinks (e.g. /private/var on macOS), so compare
	// against the evaluated path.
	want, err := filepath.EvalSymlinks(migDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("FindDir = %q, want %q", got, want)
	}
}

func TestFindDirNotFound(t *testing.T) {
	empty := t.TempDir()

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(empty); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	// Walk up from a temp dir; the real repo migrations must not be found
	// because temp dirs live outside the repository.
	if _, err := migrate.FindDir(""); err == nil {
		t.Fatal("expected error when no migrations dir exists in ancestors")
	}
}
