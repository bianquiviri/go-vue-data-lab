package etl

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewCheckpoint_Zero(t *testing.T) {
	cp := NewCheckpoint()
	if cp.Entities == nil {
		t.Fatal("Entities map must be initialised")
	}
	if cp.Key(EntityCustomers) != "" {
		t.Errorf("Key(customers) = %q, want empty", cp.Key(EntityCustomers))
	}
	if cp.IsCompleted(EntityCustomers) {
		t.Error("new checkpoint must not be completed")
	}
}

func TestCheckpoint_SetKeyMarkCompleted(t *testing.T) {
	cp := NewCheckpoint()
	cp.SetKey(EntityCustomers, "abc")
	if got := cp.Key(EntityCustomers); got != "abc" {
		t.Errorf("Key = %q, want abc", got)
	}
	cp.MarkCompleted(EntityCustomers)
	if !cp.IsCompleted(EntityCustomers) {
		t.Error("expected completed")
	}
}

func TestCheckpoint_Clone(t *testing.T) {
	cp := NewCheckpoint()
	cp.SetKey(EntityOrders, "42")
	clone := cp.Clone()
	clone.SetKey(EntityOrders, "99")
	if cp.Key(EntityOrders) != "42" {
		t.Errorf("clone mutated the original: %q", cp.Key(EntityOrders))
	}
}

func TestMemoryStore_RoundTrip(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	cp, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if cp.Entities == nil {
		t.Fatal("Load must return an initialised checkpoint")
	}

	cp.SetKey(EntityProducts, "p1")
	cp.MarkCompleted(EntityCustomers)
	if err := store.Save(ctx, cp); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Key(EntityProducts) != "p1" || !got.IsCompleted(EntityCustomers) {
		t.Errorf("round trip mismatch: %+v", got.Entities)
	}
}

func TestFileStore_RoundTrip(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "nested", "checkpoint.json")
	store := NewFileStore(path)

	cp, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load missing file: %v", err)
	}
	if cp.Entities == nil {
		t.Fatal("Load must return an initialised checkpoint when file is missing")
	}

	cp.SetKey(EntityOrderItems, "12345")
	if err := store.Save(ctx, cp); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not written: %v", err)
	}

	got, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Key(EntityOrderItems) != "12345" {
		t.Errorf("Key = %q, want 12345", got.Key(EntityOrderItems))
	}
}

func TestFileStore_Overwrite(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "checkpoint.json")
	store := NewFileStore(path)

	cp := NewCheckpoint()
	cp.SetKey(EntityCustomers, "first")
	if err := store.Save(ctx, cp); err != nil {
		t.Fatal(err)
	}
	cp.SetKey(EntityCustomers, "second")
	if err := store.Save(ctx, cp); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Key(EntityCustomers) != "second" {
		t.Errorf("Key = %q, want second", got.Key(EntityCustomers))
	}
}

func TestFileStore_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "checkpoint.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewFileStore(path)
	if _, err := store.Load(ctx); err == nil {
		t.Fatal("expected error for malformed checkpoint")
	}
}

func TestParseUUIDKey(t *testing.T) {
	if got, err := parseUUIDKey(""); err != nil || got.String() != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("parseUUIDKey(\"\") = %v, %v", got, err)
	}
	id := "11111111-1111-1111-1111-111111111111"
	if got, err := parseUUIDKey(id); err != nil || got.String() != id {
		t.Errorf("parseUUIDKey(%q) = %v, %v", id, got, err)
	}
	if _, err := parseUUIDKey("nope"); err == nil {
		t.Error("expected error for invalid uuid")
	}
}

func TestParseInt64Key(t *testing.T) {
	if got, err := parseInt64Key(""); err != nil || got != 0 {
		t.Errorf("parseInt64Key(\"\") = %v, %v", got, err)
	}
	if got, err := parseInt64Key("9007199254740993"); err != nil || got != 9007199254740993 {
		t.Errorf("parseInt64Key large = %v, %v", got, err)
	}
	if _, err := parseInt64Key("x"); err == nil {
		t.Error("expected error for invalid int64")
	}
}
