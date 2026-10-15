package generator

import "testing"

func TestNormalize(t *testing.T) {
	got := (Config{}).Normalize()
	if got.BatchSize != DefaultBatchSize {
		t.Errorf("BatchSize = %d, want %d", got.BatchSize, DefaultBatchSize)
	}
	if got.MinItems != 1 {
		t.Errorf("MinItems = %d, want 1", got.MinItems)
	}
	if got.MaxItems != 1 {
		t.Errorf("MaxItems = %d, want 1", got.MaxItems)
	}

	// Explicit values are preserved.
	in := Config{BatchSize: 42, MinItems: 2, MaxItems: 7}
	out := in.Normalize()
	if out != in {
		t.Errorf("Normalize changed explicit values: %+v -> %+v", in, out)
	}
}

func TestValidate(t *testing.T) {
	if err := (DefaultConfig()).Validate(); err != nil {
		t.Fatalf("DefaultConfig should be valid: %v", err)
	}
	if err := (Config{MinItems: 1, MaxItems: 1, BatchSize: 1}).Validate(); err != nil {
		t.Fatalf("zero-count config should be valid: %v", err)
	}
}

func TestDatasetKeyStable(t *testing.T) {
	a := Config{Seed: 7, Customers: 10, Products: 5, Orders: 3, MinItems: 1, MaxItems: 2}
	b := a
	if a.DatasetKey() != b.DatasetKey() {
		t.Fatal("same config should produce the same key")
	}

	b.Seed = 8
	if a.DatasetKey() == b.DatasetKey() {
		t.Fatal("different seed should produce a different key")
	}

	c := a
	c.Orders = 4
	if a.DatasetKey() == c.DatasetKey() {
		t.Fatal("different counts should produce a different key")
	}

	// Key must not depend on fields that do not affect the data shape.
	d := a
	d.BatchSize = 999
	d.Reset = true
	if a.DatasetKey() != d.DatasetKey() {
		t.Fatal("BatchSize/Reset should not affect the dataset key")
	}
}
