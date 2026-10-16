package jobs

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/etl"
	"github.com/josebianco/go-vue-data-lab/internal/generator"
)

// fakeStore is an in-memory generator.Store.
type fakeStore struct {
	mu        sync.Mutex
	customers int
	products  int
	orders    int
	items     int
	reset     bool
}

func (s *fakeStore) EnsureSchema(context.Context) error { return nil }
func (s *fakeStore) Reset(context.Context) error        { s.reset = true; return nil }
func (s *fakeStore) SaveCustomers(_ context.Context, rows []models.Customer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.customers += len(rows)
	return nil
}
func (s *fakeStore) SaveProducts(_ context.Context, rows []models.Product) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.products += len(rows)
	return nil
}
func (s *fakeStore) SaveOrders(_ context.Context, rows []models.Order) ([]uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders += len(rows)
	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids, nil
}
func (s *fakeStore) SaveOrderItems(_ context.Context, rows []models.OrderItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items += len(rows)
	return nil
}
func (s *fakeStore) DatasetExists(context.Context, string) (bool, error) { return false, nil }
func (s *fakeStore) SaveDatasetMarker(context.Context, generator.DatasetMarker) error {
	return nil
}

var _ generator.Store = (*fakeStore)(nil)

func TestGenerateParamsNormalizeDefaults(t *testing.T) {
	p := GenerateParams{}.Normalize()

	if p.Seed != 1 || p.Customers != 1000 || p.Products != 500 || p.Orders != 2000 {
		t.Errorf("unexpected defaults: %+v", p)
	}
	if p.MinItems != 1 || p.MaxItems != 5 || p.BatchSize != generator.DefaultBatchSize {
		t.Errorf("unexpected item/batch defaults: %+v", p)
	}
	if p.Reset == nil || !*p.Reset {
		t.Errorf("expected reset default true, got %v", p.Reset)
	}
}

func TestGenerateParamsValidateRejectsNegative(t *testing.T) {
	if err := (GenerateParams{Customers: -1}).Validate(); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}
	if err := (GenerateParams{MinItems: 5, MaxItems: 2}).Validate(); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams for max<min, got %v", err)
	}
	if err := (GenerateParams{}).Validate(); err != nil {
		t.Fatalf("expected defaults to validate, got %v", err)
	}
}

func TestGenerateRunnerRunsGenerator(t *testing.T) {
	store := &fakeStore{}
	runner := NewGenerateRunner(store)

	job, err := runner(GenerateParams{Seed: 7, Customers: 3, Products: 2, Orders: 2, MinItems: 1, MaxItems: 2, BatchSize: 2})
	if err != nil {
		t.Fatalf("build job: %v", err)
	}

	var reports []EntityProgress
	metrics, err := job(context.Background(), func(p EntityProgress) { reports = append(reports, p) })
	if err != nil {
		t.Fatalf("run job: %v", err)
	}
	if metrics["customers"] != int64(3) || metrics["products"] != int64(2) || metrics["orders"] != int64(2) {
		t.Errorf("unexpected metrics: %v", metrics)
	}
	if store.customers != 3 || store.products != 2 || store.orders != 2 {
		t.Errorf("unexpected store counts: %+v", store)
	}
	if len(reports) == 0 {
		t.Error("expected progress reports")
	}
}

func TestGenerateRunnerAcceptsNilParams(t *testing.T) {
	runner := NewGenerateRunner(&fakeStore{})
	job, err := runner(nil)
	if err != nil {
		t.Fatalf("expected nil params to use defaults, got %v", err)
	}
	if job == nil {
		t.Fatal("expected a job func")
	}
}

func TestGenerateRunnerRejectsInvalidParams(t *testing.T) {
	runner := NewGenerateRunner(&fakeStore{})
	if _, err := runner(GenerateParams{Customers: -1}); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}
	if _, err := runner("wrong type"); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams for wrong type, got %v", err)
	}
}

// fakeSource is a zero-row etl.Source.
type fakeSource struct{}

func (fakeSource) Count(context.Context, etl.Entity) (int64, error) { return 0, nil }
func (fakeSource) FetchCustomers(context.Context, uuid.UUID, int) ([]models.Customer, error) {
	return nil, nil
}
func (fakeSource) FetchProducts(context.Context, uuid.UUID, int) ([]models.Product, error) {
	return nil, nil
}
func (fakeSource) FetchOrders(context.Context, uuid.UUID, int) ([]models.Order, error) {
	return nil, nil
}
func (fakeSource) FetchOrderItems(context.Context, int64, int) ([]models.OrderItem, error) {
	return nil, nil
}
func (fakeSource) OrderItemSums(context.Context, []uuid.UUID) (map[uuid.UUID]float64, error) {
	return map[uuid.UUID]float64{}, nil
}

// fakeLoader is a no-op etl.Loader.
type fakeLoader struct{}

func (fakeLoader) UpsertCustomers(context.Context, []models.Customer) (etl.LoadResult, error) {
	return etl.LoadResult{}, nil
}
func (fakeLoader) UpsertProducts(context.Context, []models.Product) (etl.LoadResult, error) {
	return etl.LoadResult{}, nil
}
func (fakeLoader) UpsertOrders(context.Context, []models.Order) (etl.LoadResult, error) {
	return etl.LoadResult{}, nil
}
func (fakeLoader) UpsertOrderItems(context.Context, []models.OrderItem) (etl.LoadResult, error) {
	return etl.LoadResult{}, nil
}

func TestMigrateParamsValidateRejectsUnknownEntity(t *testing.T) {
	if err := (MigrateParams{Entities: []string{"nope"}}).Validate(); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}
	if err := (MigrateParams{Workers: -1}).Validate(); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams for negative workers, got %v", err)
	}
	if err := (MigrateParams{Entities: []string{"customers", "orders"}}).Validate(); err != nil {
		t.Fatalf("expected valid params, got %v", err)
	}
}

func TestMigrateRunnerRunsETL(t *testing.T) {
	runner := NewMigrateRunner(fakeSource{}, fakeLoader{})

	job, err := runner(MigrateParams{Entities: []string{"customers"}})
	if err != nil {
		t.Fatalf("build job: %v", err)
	}
	metrics, err := job(context.Background(), func(EntityProgress) {})
	if err != nil {
		t.Fatalf("run job: %v", err)
	}
	if _, ok := metrics["customers"]; !ok {
		t.Errorf("expected per-entity metrics, got %v", metrics)
	}
	if _, ok := metrics["total"]; !ok {
		t.Errorf("expected total metrics, got %v", metrics)
	}
}

func TestMigrateRunnerRejectsInvalidParams(t *testing.T) {
	runner := NewMigrateRunner(fakeSource{}, fakeLoader{})
	if _, err := runner(MigrateParams{Entities: []string{"nope"}}); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}
	if _, err := runner(42); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams for wrong type, got %v", err)
	}
}
