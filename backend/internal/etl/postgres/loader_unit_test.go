package postgres

import (
	"context"
	"testing"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/etl"
)

func TestNewSource_NilDB(t *testing.T) {
	if _, err := NewSource(nil); err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestNewLoader_NilDB(t *testing.T) {
	if _, err := NewLoader(nil); err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestSource_CountUnknownEntity(t *testing.T) {
	// The entity lookup happens before any database access, so a nil *Source
	// value with a nil db is enough to exercise the guard.
	s := &Source{}
	if _, err := s.Count(context.Background(), "nope"); err == nil {
		t.Fatal("expected error for unknown entity")
	}
}

func TestLoader_EmptyBatch(t *testing.T) {
	// n == 0 short-circuits before touching the database.
	l := &Loader{}
	res, err := l.UpsertCustomers(context.Background(), nil)
	if err != nil {
		t.Fatalf("UpsertCustomers(nil): %v", err)
	}
	if res.Inserted != 0 || res.Updated != 0 {
		t.Errorf("res = %+v, want zero", res)
	}
	if _, err := l.UpsertOrderItems(context.Background(), []models.OrderItem{}); err != nil {
		t.Fatalf("UpsertOrderItems(empty): %v", err)
	}
}

func TestPlaceholders(t *testing.T) {
	cases := []struct {
		rows, cols int
		want       string
	}{
		{1, 1, "(?)"},
		{1, 3, "(?,?,?)"},
		{2, 2, "(?,?),(?,?)"},
		{3, 1, "(?),(?),(?)"},
	}
	for _, c := range cases {
		if got := placeholders(c.rows, c.cols); got != c.want {
			t.Errorf("placeholders(%d,%d) = %q, want %q", c.rows, c.cols, got, c.want)
		}
	}
}

func TestEntityTables_Coverage(t *testing.T) {
	// Every canonical entity must resolve to a table name.
	for _, e := range etl.AllEntities {
		if entityTables[e] == "" {
			t.Errorf("missing table mapping for %q", e)
		}
	}
}
