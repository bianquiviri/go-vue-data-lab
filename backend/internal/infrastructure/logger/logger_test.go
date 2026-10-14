package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWritesToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "app.log")

	log, err := New("debug", path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	log.Info("hello from test")
	// Sync on stdout may fail when redirected; ignore it and assert on the file.
	_ = log.Sync()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "hello from test") {
		t.Errorf("log file does not contain message, got: %s", data)
	}
}

func TestNewInvalidLevel(t *testing.T) {
	if _, err := New("not-a-level", filepath.Join(t.TempDir(), "app.log")); err == nil {
		t.Fatal("expected error for invalid log level")
	}
}
