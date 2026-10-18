package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// zapLine renders a record the way the production JSON encoder does.
func zapLine(level, ts, msg string, extra map[string]any) string {
	record := map[string]any{"level": level, "ts": ts, "msg": msg, "caller": "internal/api/main.go:42"}
	for k, v := range extra {
		record[k] = v
	}
	b, err := json.Marshal(record)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func writeLog(t *testing.T, name string, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	return path
}

func TestFileErrorLogsFiltersAndOrdersNewestFirst(t *testing.T) {
	path := writeLog(t, "app.log", []string{
		zapLine("INFO", "2026-10-16T10:00:00.000Z", "boot", nil),
		zapLine("ERROR", "2026-10-16T10:01:00.000Z", "oldest error", map[string]any{"error": "a"}),
		zapLine("DEBUG", "2026-10-16T10:02:00.000Z", "trace", nil),
		zapLine("WARN", "2026-10-16T10:03:00.000Z", "careful", nil),
		zapLine("ERROR", "2026-10-16T10:04:00.000Z", "newest error", map[string]any{"error": "b"}),
		zapLine("FATAL", "2026-10-16T10:05:00.000Z", "boom", nil),
	})

	entries, total, err := service.NewFileErrorLogs(path).RecentErrors(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("RecentErrors: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	want := []string{"boom", "newest error", "oldest error"}
	if len(entries) != len(want) {
		t.Fatalf("entries = %d, want %d", len(entries), len(want))
	}
	for i, m := range want {
		if entries[i].Message != m {
			t.Errorf("entries[%d].Message = %q, want %q", i, entries[i].Message, m)
		}
	}

	if entries[1].Fields["error"] != "b" {
		t.Errorf("fields not preserved: %+v", entries[1].Fields)
	}
	if entries[1].Caller == "" {
		t.Error("expected caller to be captured")
	}
	wantTime := time.Date(2026, 10, 16, 10, 4, 0, 0, time.UTC)
	if !entries[1].Time.Equal(wantTime) {
		t.Errorf("time = %v, want %v", entries[1].Time, wantTime)
	}
}

func TestFileErrorLogsPaginates(t *testing.T) {
	lines := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		lines = append(lines, zapLine("ERROR", "2026-10-16T10:00:00.000Z", fmt.Sprintf("err-%d", i), nil))
	}
	path := writeLog(t, "app.log", lines)

	// Newest first => err-4, err-3, err-2, err-1, err-0.
	page, total, err := service.NewFileErrorLogs(path).RecentErrors(context.Background(), 2, 2)
	if err != nil {
		t.Fatalf("RecentErrors: %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	if len(page) != 2 || page[0].Message != "err-2" || page[1].Message != "err-1" {
		t.Fatalf("unexpected page: %+v", page)
	}

	// Offset past the end returns an empty (non-nil) page, not an error.
	page, total, err = service.NewFileErrorLogs(path).RecentErrors(context.Background(), 10, 99)
	if err != nil {
		t.Fatalf("RecentErrors: %v", err)
	}
	if total != 5 || len(page) != 0 {
		t.Errorf("got total=%d len=%d, want 5/0", total, len(page))
	}
}

func TestFileErrorLogsMissingFileIsEmptyNotError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.log")

	entries, total, err := service.NewFileErrorLogs(path).RecentErrors(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if total != 0 || len(entries) != 0 {
		t.Errorf("expected empty result, got total=%d entries=%d", total, len(entries))
	}
}

func TestFileErrorLogsSkipsMalformedLines(t *testing.T) {
	path := writeLog(t, "app.log", []string{
		"not json at all",
		zapLine("ERROR", "2026-10-16T10:01:00.000Z", "kept", nil),
		`{"level":"ERROR"}`,
	})

	entries, total, err := service.NewFileErrorLogs(path).RecentErrors(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("RecentErrors: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if entries[0].Message != "kept" {
		t.Errorf("message = %q, want kept", entries[0].Message)
	}
}

func TestFileErrorLogsRespectsCancelledContext(t *testing.T) {
	path := writeLog(t, "app.log", []string{zapLine("ERROR", "2026-10-16T10:00:00.000Z", "x", nil)})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, _, err := service.NewFileErrorLogs(path).RecentErrors(ctx, 10, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestFileErrorLogsScansOnlyTheTailWindow(t *testing.T) {
	// A head error far from the end must be excluded: only maxTailBytes are
	// scanned, which keeps the endpoint cheap on large rotated files.
	head := zapLine("ERROR", "2026-10-16T09:00:00.000Z", "head error", nil)

	filler := make([]string, 0, 1400)
	fadeline := zapLine("INFO", "2026-10-16T09:30:00.000Z", strings.Repeat("f", 3000), nil)
	for i := 0; i < 1500; i++ {
		filler = append(filler, fadeline)
	}
	tail := []string{
		zapLine("ERROR", "2026-10-16T11:00:00.000Z", "tail error", nil),
	}

	lines := append([]string{head}, filler...)
	lines = append(lines, tail...)
	path := writeLog(t, "app.log", lines)

	entries, total, err := service.NewFileErrorLogs(path).RecentErrors(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("RecentErrors: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1 (head error must be outside the scanned tail)", total)
	}
	if entries[0].Message != "tail error" {
		t.Errorf("message = %q, want tail error", entries[0].Message)
	}
}
