package postgres

import (
	"testing"

	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/generator"
)

func TestNewPostgresStoreNilDB(t *testing.T) {
	if _, err := NewPostgresStore(nil, 100); err == nil {
		t.Fatal("expected error for nil *gorm.DB")
	}
}

func TestNewPostgresStoreDefaultsBatchSize(t *testing.T) {
	// A non-positive batch size must fall back to the generator default. We use a
	// zero-value *gorm.DB only to exercise the constructor; no query is issued.
	store, err := NewPostgresStore(&gorm.DB{}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.batchSize != generator.DefaultBatchSize {
		t.Errorf("batchSize = %d, want %d", store.batchSize, generator.DefaultBatchSize)
	}
}
