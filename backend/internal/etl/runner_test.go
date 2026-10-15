package etl

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// seqUUID returns a deterministic, ordered UUID for tests.
func seqUUID(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012d", n))
}

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type fakeSource struct {
	mu        sync.Mutex
	customers []models.Customer
	products  []models.Product
	orders    []models.Order
	items     []models.OrderItem

	fetchErr map[Entity]error
	block    map[Entity]chan struct{}
}

func newFakeSource() *fakeSource {
	return &fakeSource{fetchErr: map[Entity]error{}, block: map[Entity]chan struct{}{}}
}

func (f *fakeSource) Count(_ context.Context, e Entity) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch e {
	case EntityCustomers:
		return int64(len(f.customers)), nil
	case EntityProducts:
		return int64(len(f.products)), nil
	case EntityOrders:
		return int64(len(f.orders)), nil
	case EntityOrderItems:
		return int64(len(f.items)), nil
	}
	return 0, fmt.Errorf("fake: entidad desconocida %q", e)
}

func (f *fakeSource) wait(ctx context.Context, e Entity) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := f.fetchErr[e]; err != nil {
		return err
	}
	if ch, ok := f.block[e]; ok {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		}
	}
	return nil
}

func (f *fakeSource) FetchCustomers(ctx context.Context, after uuid.UUID, limit int) ([]models.Customer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.wait(ctx, EntityCustomers); err != nil {
		return nil, err
	}
	out := make([]models.Customer, 0, limit)
	for _, r := range f.customers {
		if bytesGreaterUUID(r.ID, after) && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeSource) FetchProducts(ctx context.Context, after uuid.UUID, limit int) ([]models.Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.wait(ctx, EntityProducts); err != nil {
		return nil, err
	}
	out := make([]models.Product, 0, limit)
	for _, r := range f.products {
		if bytesGreaterUUID(r.ID, after) && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeSource) FetchOrders(ctx context.Context, after uuid.UUID, limit int) ([]models.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.wait(ctx, EntityOrders); err != nil {
		return nil, err
	}
	out := make([]models.Order, 0, limit)
	for _, r := range f.orders {
		if bytesGreaterUUID(r.ID, after) && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeSource) FetchOrderItems(ctx context.Context, after int64, limit int) ([]models.OrderItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.wait(ctx, EntityOrderItems); err != nil {
		return nil, err
	}
	out := make([]models.OrderItem, 0, limit)
	for _, r := range f.items {
		if r.ID > after && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeSource) OrderItemSums(_ context.Context, orderIDs []uuid.UUID) (map[uuid.UUID]float64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	want := make(map[uuid.UUID]struct{}, len(orderIDs))
	for _, id := range orderIDs {
		want[id] = struct{}{}
	}
	sums := map[uuid.UUID]float64{}
	for _, it := range f.items {
		if _, ok := want[it.OrderID]; ok {
			sums[it.OrderID] += it.Subtotal
		}
	}
	return sums, nil
}

// sortAll keeps the fake keyset pagination deterministic.
func (f *fakeSource) sortAll() {
	f.mu.Lock()
	defer f.mu.Unlock()
	sort.Slice(f.customers, func(i, j int) bool { return bytesGreaterUUID(f.customers[j].ID, f.customers[i].ID) })
	sort.Slice(f.products, func(i, j int) bool { return bytesGreaterUUID(f.products[j].ID, f.products[i].ID) })
	sort.Slice(f.orders, func(i, j int) bool { return bytesGreaterUUID(f.orders[j].ID, f.orders[i].ID) })
	sort.Slice(f.items, func(i, j int) bool { return f.items[i].ID < f.items[j].ID })
}

// bytesGreaterUUID reports whether a sorts strictly after b.
func bytesGreaterUUID(a, b uuid.UUID) bool {
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// fakeLoader records the batches it receives and simulates insert/update by
// tracking which primary keys it has already seen. failOn[batchIndex] forces a
// failure for that entity's N-th batch (0-based).
type fakeLoader struct {
	mu        sync.Mutex
	seen      map[Entity]map[string]struct{}
	batches   map[Entity][][]string
	failOn    map[Entity]map[int]error
	batchSeq  map[Entity]int
	callCount int

	orders []models.Order
}

func newFakeLoader() *fakeLoader {
	return &fakeLoader{
		seen:     map[Entity]map[string]struct{}{},
		batches:  map[Entity][][]string{},
		failOn:   map[Entity]map[int]error{},
		batchSeq: map[Entity]int{},
	}
}

func (l *fakeLoader) failNext(entity Entity, index int, err error) {
	if l.failOn[entity] == nil {
		l.failOn[entity] = map[int]error{}
	}
	l.failOn[entity][index] = err
}

func (l *fakeLoader) record(entity Entity, keys []string) (LoadResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.callCount++
	idx := l.batchSeq[entity]
	l.batchSeq[entity]++
	if errs := l.failOn[entity]; errs != nil {
		if err, ok := errs[idx]; ok {
			return LoadResult{}, err
		}
	}
	l.batches[entity] = append(l.batches[entity], keys)
	if l.seen[entity] == nil {
		l.seen[entity] = map[string]struct{}{}
	}
	var res LoadResult
	for _, k := range keys {
		if _, ok := l.seen[entity][k]; ok {
			res.Updated++
		} else {
			l.seen[entity][k] = struct{}{}
			res.Inserted++
		}
	}
	return res, nil
}

func (l *fakeLoader) UpsertCustomers(_ context.Context, rows []models.Customer) (LoadResult, error) {
	keys := make([]string, len(rows))
	for i, r := range rows {
		keys[i] = r.ID.String()
	}
	return l.record(EntityCustomers, keys)
}

func (l *fakeLoader) UpsertProducts(_ context.Context, rows []models.Product) (LoadResult, error) {
	keys := make([]string, len(rows))
	for i, r := range rows {
		keys[i] = r.ID.String()
	}
	return l.record(EntityProducts, keys)
}

func (l *fakeLoader) UpsertOrders(_ context.Context, rows []models.Order) (LoadResult, error) {
	keys := make([]string, len(rows))
	for i, r := range rows {
		keys[i] = r.ID.String()
	}
	l.mu.Lock()
	l.orders = append(l.orders, rows...)
	l.mu.Unlock()
	return l.record(EntityOrders, keys)
}

func (l *fakeLoader) UpsertOrderItems(_ context.Context, rows []models.OrderItem) (LoadResult, error) {
	keys := make([]string, len(rows))
	for i, r := range rows {
		keys[i] = fmt.Sprintf("%d", r.ID)
	}
	return l.record(EntityOrderItems, keys)
}

func (l *fakeLoader) batchCount(entity Entity) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.batches[entity])
}

func (l *fakeLoader) loadedKeys(entity Entity) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, b := range l.batches[entity] {
		out = append(out, b...)
	}
	return out
}

// loadedOrders returns the actual order rows received by the loader.
func (l *fakeLoader) loadedOrders() []models.Order {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.orders
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// dataset builds a small, valid source dataset.
func dataset(customers, products, orders int) (*fakeSource, map[uuid.UUID]float64) {
	src := newFakeSource()
	for i := 1; i <= customers; i++ {
		src.customers = append(src.customers, models.Customer{
			ID: seqUUID(i), FirstName: "First", LastName: "Last",
			Email: fmt.Sprintf("user%d@example.com", i), CountryCode: "AR", City: "CBA",
		})
	}
	for i := 1; i <= products; i++ {
		src.products = append(src.products, models.Product{
			ID: seqUUID(100 + i), SKU: fmt.Sprintf("SKU-%d", i), Name: "Product",
			Category: "cat", Price: float64(i) * 1.5, Stock: 10, Status: "active",
		})
	}
	sums := map[uuid.UUID]float64{}
	itemID := int64(1)
	for i := 1; i <= orders; i++ {
		orderID := seqUUID(200 + i)
		var total float64
		for q := 1; q <= 2; q++ {
			unit := 5.0
			sub := unit * float64(q)
			src.items = append(src.items, models.OrderItem{
				ID: itemID, OrderID: orderID, ProductID: seqUUID(101),
				Quantity: q, UnitPrice: unit, Subtotal: sub,
			})
			total += sub
			itemID++
		}
		sums[orderID] = total
		src.orders = append(src.orders, models.Order{
			ID: orderID, OrderNumber: fmt.Sprintf("ORD-%d", i),
			CustomerID: seqUUID(1), Status: "pending",
			// Deliberately wrong to exercise total recomputation.
			TotalAmount: 99999, Currency: "USD",
		})
	}
	src.sortAll()
	return src, sums
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestNew_ValidatesDependencies(t *testing.T) {
	src := newFakeSource()
	ldr := newFakeLoader()
	if _, err := New(Config{}, nil, ldr); err == nil {
		t.Error("expected error for nil source")
	}
	if _, err := New(Config{}, src, nil); err == nil {
		t.Error("expected error for nil loader")
	}
	if _, err := New(Config{Entities: []Entity{"nope"}}, src, ldr); err == nil {
		t.Error("expected error for unknown entity")
	}
}

func TestConfig_ApplicationDefaults(t *testing.T) {
	cfg := Config{}
	cfg.applyDefaults()
	if cfg.Workers <= 0 {
		t.Errorf("Workers = %d, want > 0", cfg.Workers)
	}
	if cfg.BatchSize != DefaultBatchSize {
		t.Errorf("BatchSize = %d, want %d", cfg.BatchSize, DefaultBatchSize)
	}
	if cfg.ExtractSize != DefaultExtractSize {
		t.Errorf("ExtractSize = %d, want %d", cfg.ExtractSize, DefaultExtractSize)
	}
	if cfg.MaxSkipped != DefaultMaxSkipped {
		t.Errorf("MaxSkipped = %d, want %d", cfg.MaxSkipped, DefaultMaxSkipped)
	}
}

func TestConfig_EntitiesCanonicalOrder(t *testing.T) {
	cfg := Config{Entities: []Entity{EntityOrderItems, EntityCustomers}}
	cfg.applyDefaults()
	got := cfg.entities()
	want := []Entity{EntityCustomers, EntityOrderItems}
	if len(got) != len(want) {
		t.Fatalf("entities = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entities[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRunner_MigratesAllEntities(t *testing.T) {
	src, _ := dataset(5, 4, 3)
	ldr := newFakeLoader()
	runner, err := New(Config{
		Workers: 3, BatchSize: 3, ExtractSize: 2, MaxSkipped: 100,
	}, src, ldr)
	if err != nil {
		t.Fatal(err)
	}

	res, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	total := res.Metrics.Total()
	if total.Read != int64(5+4+3+6) {
		t.Errorf("Read = %d, want 18", total.Read)
	}
	if total.Inserted != 18 {
		t.Errorf("Inserted = %d, want 18", total.Inserted)
	}
	if total.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", total.Skipped)
	}
	if total.Failed != 0 {
		t.Errorf("Failed = %d, want 0", total.Failed)
	}
	if len(res.Errors) != 0 {
		t.Errorf("Errors = %v, want none", res.Errors)
	}

	for _, e := range AllEntities {
		if !res.Checkpoint.IsCompleted(e) {
			t.Errorf("entity %s not completed", e)
		}
	}

	// Total amounts must have been recomputed from the items.
	for _, o := range ldr.loadedOrders() {
		if o.TotalAmount == 99999 {
			t.Errorf("order %s kept the invalid total", o.OrderNumber)
		}
		if o.TotalAmount != 15 {
			t.Errorf("order %s total = %v, want 15", o.OrderNumber, o.TotalAmount)
		}
	}
}

func TestRunner_ReportsUpdatesOnRerun(t *testing.T) {
	src, _ := dataset(3, 0, 0)
	ldr := newFakeLoader()
	ldr.seen[EntityCustomers] = map[string]struct{}{
		seqUUID(1).String(): {}, seqUUID(2).String(): {}, seqUUID(3).String(): {},
	}

	runner, _ := New(Config{Workers: 1, BatchSize: 2, ExtractSize: 2}, src, ldr)
	res, err := runner.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Metrics[EntityCustomers].Updated; got != 3 {
		t.Errorf("Updated = %d, want 3", got)
	}
	if got := res.Metrics[EntityCustomers].Inserted; got != 0 {
		t.Errorf("Inserted = %d, want 0", got)
	}
}

func TestRunner_SkipsInvalidRows(t *testing.T) {
	src, _ := dataset(3, 2, 0)
	// Corrupt one customer (bad email) and one product (negative price).
	src.customers[1].Email = "not-an-email"
	src.products[0].Price = -1
	src.sortAll()

	ldr := newFakeLoader()
	runner, _ := New(Config{Workers: 2, BatchSize: 2, ExtractSize: 2}, src, ldr)

	res, err := runner.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Metrics[EntityCustomers].Skipped != 1 {
		t.Errorf("customer skipped = %d, want 1", res.Metrics[EntityCustomers].Skipped)
	}
	if res.Metrics[EntityProducts].Skipped != 1 {
		t.Errorf("product skipped = %d, want 1", res.Metrics[EntityProducts].Skipped)
	}
	if len(res.Skipped) != 2 {
		t.Fatalf("skip records = %d, want 2 (%+v)", len(res.Skipped), res.Skipped)
	}
	for _, s := range res.Skipped {
		if s.Reason == "" {
			t.Errorf("skip record without reason: %+v", s)
		}
		if s.Entity == "" || s.Key == "" {
			t.Errorf("incomplete skip record: %+v", s)
		}
	}
	// Only valid rows reach the loader.
	if got := len(ldr.loadedKeys(EntityCustomers)); got != 2 {
		t.Errorf("loaded customers = %d, want 2", got)
	}
	if got := len(ldr.loadedKeys(EntityProducts)); got != 1 {
		t.Errorf("loaded products = %d, want 1", got)
	}
}

func TestRunner_DeduplicatesWithinBatch(t *testing.T) {
	src := newFakeSource()
	c := models.Customer{
		ID: seqUUID(1), FirstName: "A", LastName: "B",
		Email: "a@x.io", CountryCode: "AR", City: "CBA",
	}
	src.customers = []models.Customer{c, c, c}
	src.sortAll()

	ldr := newFakeLoader()
	runner, _ := New(Config{Workers: 1, BatchSize: 10, ExtractSize: 10}, src, ldr)
	res, err := runner.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := len(ldr.loadedKeys(EntityCustomers)); got != 1 {
		t.Errorf("loaded customers = %d, want 1 (deduplicated)", got)
	}
	if res.Metrics[EntityCustomers].Skipped != 2 {
		t.Errorf("Skipped = %d, want 2 duplicates", res.Metrics[EntityCustomers].Skipped)
	}
}

func TestRunner_FailFastStopsRun(t *testing.T) {
	src, _ := dataset(4, 2, 0)
	ldr := newFakeLoader()
	ldr.failNext(EntityCustomers, 0, errors.New("boom"))

	runner, _ := New(Config{
		Workers: 2, BatchSize: 2, ExtractSize: 2, FailFast: true,
	}, src, ldr)
	res, err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("expected error with fail-fast")
	}
	if len(res.Errors) == 0 {
		t.Error("expected recorded errors")
	}
	if res.Metrics.Total().Failed == 0 {
		t.Error("expected failed rows")
	}
	if res.Checkpoint.IsCompleted(EntityCustomers) {
		t.Error("customers must not be marked completed after failure")
	}
	// Downstream entities must not have been loaded (FK order + fail-fast).
	if ldr.batchCount(EntityProducts) != 0 {
		t.Errorf("products loaded after fail-fast: %d batches", ldr.batchCount(EntityProducts))
	}
}

func TestRunner_ContinuesOnFailureWithoutFailFast(t *testing.T) {
	src, _ := dataset(2, 4, 0)
	ldr := newFakeLoader()
	ldr.failNext(EntityProducts, 0, errors.New("transient"))

	runner, _ := New(Config{Workers: 1, BatchSize: 2, ExtractSize: 2}, src, ldr)
	res, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error without fail-fast: %v", err)
	}
	if len(res.Errors) == 0 {
		t.Error("expected the failed batch to be recorded")
	}
	if !res.Checkpoint.IsCompleted(EntityCustomers) {
		t.Error("customers should complete")
	}
	if res.Checkpoint.IsCompleted(EntityProducts) {
		t.Error("products should NOT complete when a batch failed")
	}
	// The non-failing product batch must still have been loaded.
	if res.Metrics[EntityProducts].Failed == 0 {
		t.Error("expected failed product rows")
	}
}

func TestRunner_ResumeFromCheckpoint(t *testing.T) {
	src, _ := dataset(3, 2, 3)

	store := NewMemoryStore()
	cp := NewCheckpoint()
	cp.MarkCompleted(EntityCustomers)
	cp.MarkCompleted(EntityProducts)
	// Only the first two orders were processed: resume starts after seqUUID(202).
	cp.SetKey(EntityOrders, seqUUID(202).String())
	if err := store.Save(context.Background(), cp); err != nil {
		t.Fatal(err)
	}

	ldr := newFakeLoader()
	runner, _ := New(Config{Workers: 1, BatchSize: 2, ExtractSize: 2}, src, ldr,
		WithCheckpointStore(store))
	res, err := runner.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if ldr.batchCount(EntityCustomers) != 0 {
		t.Error("completed customers entity must be skipped on resume")
	}
	if ldr.batchCount(EntityProducts) != 0 {
		t.Error("completed products entity must be skipped on resume")
	}
	orders := ldr.loadedKeys(EntityOrders)
	if len(orders) != 1 || orders[0] != seqUUID(203).String() {
		t.Errorf("resumed orders = %v, want only order 203", orders)
	}
	if !res.Checkpoint.IsCompleted(EntityOrders) {
		t.Error("orders should complete after resuming")
	}
	// Order items start from scratch.
	if got := len(ldr.loadedKeys(EntityOrderItems)); got != 6 {
		t.Errorf("order items loaded = %d, want 6", got)
	}
}

func TestRunner_ProgressCallback(t *testing.T) {
	src, _ := dataset(2, 2, 1)
	ldr := newFakeLoader()

	var mu sync.Mutex
	seen := map[Entity]bool{}
	runner, _ := New(Config{
		Workers: 1, BatchSize: 2, ExtractSize: 2,
		OnProgress: func(p Progress) {
			mu.Lock()
			seen[p.Entity] = true
			mu.Unlock()
		},
	}, src, ldr)

	if _, err := runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, e := range AllEntities {
		if !seen[e] {
			t.Errorf("no progress reported for %s", e)
		}
	}
}

func TestRunner_StrictTotalsSkipsMismatch(t *testing.T) {
	src, _ := dataset(1, 1, 2)
	ldr := newFakeLoader()
	runner, _ := New(Config{
		Workers: 1, BatchSize: 2, ExtractSize: 2, StrictTotals: true,
	}, src, ldr)
	res, err := runner.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Metrics[EntityOrders].Skipped != 2 {
		t.Errorf("orders skipped = %d, want 2", res.Metrics[EntityOrders].Skipped)
	}
	if got := len(ldr.loadedKeys(EntityOrders)); got != 0 {
		t.Errorf("orders loaded = %d, want 0 in strict mode", got)
	}
}

func TestRunner_ContextCancelled(t *testing.T) {
	src, _ := dataset(10, 0, 0)
	ldr := newFakeLoader()
	runner, _ := New(Config{Workers: 1, BatchSize: 2, ExtractSize: 2}, src, ldr)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runner.Run(ctx)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want wrapping context.Canceled", err)
	}
}

func TestRunner_StateAndMetricsAccessors(t *testing.T) {
	src, _ := dataset(2, 0, 0)
	ldr := newFakeLoader()
	runner, _ := New(Config{Workers: 1, BatchSize: 1, ExtractSize: 1}, src, ldr)
	if _, err := runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !runner.State().IsCompleted(EntityCustomers) {
		t.Error("State() should report completed entity")
	}
	if runner.Metrics()[EntityCustomers].Inserted != 2 {
		t.Errorf("Metrics() = %+v", runner.Metrics()[EntityCustomers])
	}
}

func TestRunner_SourceFetchError(t *testing.T) {
	src, _ := dataset(0, 0, 0)
	src.fetchErr[EntityProducts] = errors.New("db down")
	src.products = append(src.products, models.Product{ID: seqUUID(1), SKU: "S", Name: "N", Category: "C", Price: 1, Stock: 1, Status: "active"})

	ldr := newFakeLoader()
	runner, _ := New(Config{Workers: 1, BatchSize: 2, ExtractSize: 2}, src, ldr)
	_, err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("expected fetch error to abort the run")
	}
	if !errors.Is(err, src.fetchErr[EntityProducts]) {
		t.Errorf("error = %v, want to wrap the source error", err)
	}
}
