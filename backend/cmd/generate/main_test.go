package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/josebianco/go-vue-data-lab/internal/generator"
)

func TestParseItemsPerOrder(t *testing.T) {
	cases := []struct {
		spec    string
		wantMin int
		wantMax int
		wantErr bool
	}{
		{"3", 3, 3, false},
		{"1-5", 1, 5, false},
		{" 2 - 4 ", 2, 4, false},
		{"", 0, 0, true},
		{"abc", 0, 0, true},
		{"1-", 0, 0, true},
		{"-5", 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.spec, func(t *testing.T) {
			min, max, err := parseItemsPerOrder(tc.spec)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.spec)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if min != tc.wantMin || max != tc.wantMax {
				t.Errorf("got %d-%d, want %d-%d", min, max, tc.wantMin, tc.wantMax)
			}
		})
	}
}

func TestPrintSummary(t *testing.T) {
	var buf bytes.Buffer
	printSummary(&buf, generator.Summary{
		DatasetKey: "abc123",
		Seed:       7,
		Customers:  10,
		Products:   5,
		Orders:     3,
		OrderItems: 9,
	}, 1500*time.Millisecond)

	out := buf.String()
	for _, want := range []string{"abc123", "customers", "10", "order items", "9", "elapsed"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestPrintSummarySkipped(t *testing.T) {
	var buf bytes.Buffer
	printSummary(&buf, generator.Summary{DatasetKey: "k", Skipped: true}, time.Second)
	if !strings.Contains(buf.String(), "skipped") {
		t.Errorf("expected skipped status, got:\n%s", buf.String())
	}
}

func TestProgressPrinter(t *testing.T) {
	var buf bytes.Buffer
	p := newProgressPrinter(&buf)
	p.update(generator.EntityCustomers, 50, 100)
	p.update(generator.EntityCustomers, 100, 100)
	p.finish()
	out := buf.String()
	if !strings.Contains(out, "100/100") {
		t.Errorf("unexpected output: %q", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Error("finish should emit a trailing newline")
	}
}
