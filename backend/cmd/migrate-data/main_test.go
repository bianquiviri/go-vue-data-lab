package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/josebianco/go-vue-data-lab/internal/etl"
)

func TestParseEntities(t *testing.T) {
	got, err := parseEntities("")
	if err != nil || got != nil {
		t.Fatalf("empty spec = %v, %v; want nil, nil", got, err)
	}

	got, err = parseEntities(" customers , orders ")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 || got[0] != etl.EntityCustomers || got[1] != etl.EntityOrders {
		t.Errorf("got %v", got)
	}

	if _, err := parseEntities("customers,nope"); err == nil {
		t.Error("expected error for unknown entity")
	}
}

func TestPrintSummary(t *testing.T) {
	res := &etl.Result{
		Metrics: etl.MetricsSnapshot{
			etl.EntityCustomers: {Read: 10, Inserted: 8, Updated: 2, Skipped: 1, Failed: 0},
		},
		Skipped: []etl.SkipRecord{
			{Entity: etl.EntityCustomers, Key: "abc", Reason: "email: formato inválido"},
		},
	}
	var buf bytes.Buffer
	printSummary(&buf, res, 123*time.Millisecond)

	out := buf.String()
	for _, want := range []string{"migración completa", "customers", "TOTAL", "email: formato inválido", "elapsed"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestProgressPrinter(t *testing.T) {
	p := &progressPrinter{}
	p.update(etl.Progress{Entity: etl.EntityOrders, Metrics: etl.EntityMetrics{Inserted: 5, Updated: 1}})
	p.finish()
	if p.lastLine == "" {
		t.Error("progress line not recorded")
	}
}
