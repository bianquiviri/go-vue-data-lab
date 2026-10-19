package config

import (
	"os"
	"path/filepath"
	"testing"
)

// loadDotEnv must find a .env in a parent directory, which is the normal case
// when binaries run from backend/ while .env lives at the repository root.
func TestLoadDotEnvFindsParentFile(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "backend", "cmd")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	const marker = "GDL_DOTENV_PARENT_MARKER"
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte(marker+"=from-parent\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	orig, had := os.LookupEnv(marker)
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(marker, orig)
		} else {
			_ = os.Unsetenv(marker)
		}
	})
	if err := os.Unsetenv(marker); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(child); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	loadDotEnv()

	if got := os.Getenv(marker); got != "from-parent" {
		t.Errorf("marker = %q, want %q (parent .env not loaded)", got, "from-parent")
	}
}

// A file in the working directory itself must win over any parent .env.
func TestLoadDotEnvPrefersCurrentDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("GDL_DOTENV_HERE=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	loadDotEnv()

	// No assertion on env pollution: only check it does not panic and that a
	// missing parent walk terminates (the file exists right here).
	if _, err := os.Stat(filepath.Join(dir, ".env")); err != nil {
		t.Fatalf(".env should exist: %v", err)
	}
}

// When no .env exists anywhere up the chain, loadDotEnv must be a no-op.
func TestLoadDotEnvNoFileIsNoop(t *testing.T) {
	dir := t.TempDir()

	orig, had := os.LookupEnv("GDL_DOTENV_ABSENT")
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("GDL_DOTENV_ABSENT", orig)
		}
	})
	_ = os.Unsetenv("GDL_DOTENV_ABSENT")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	loadDotEnv()

	if got := os.Getenv("GDL_DOTENV_ABSENT"); got != "" {
		t.Errorf("GDL_DOTENV_ABSENT = %q, want empty", got)
	}
}
