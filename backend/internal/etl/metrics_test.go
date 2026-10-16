package etl

import (
	"sync"
	"testing"
)

func TestRecorder_UpdateAndSnapshot(t *testing.T) {
	rec := newRecorder([]Entity{EntityCustomers, EntityProducts})
	rec.update(EntityCustomers, func(m *EntityMetrics) {
		m.Read += 10
		m.Processed += 8
		m.Inserted += 5
		m.Updated += 3
		m.Skipped += 2
	})

	snap := rec.snapshot()
	if snap[EntityCustomers].Read != 10 {
		t.Errorf("Read = %d, want 10", snap[EntityCustomers].Read)
	}
	if snap[EntityCustomers].Inserted != 5 || snap[EntityCustomers].Updated != 3 {
		t.Errorf("inserted/updated = %+v", snap[EntityCustomers])
	}
	if _, ok := snap[EntityProducts]; !ok {
		t.Error("snapshot must contain all configured entities")
	}
}

func TestRecorder_SnapshotIsCopy(t *testing.T) {
	rec := newRecorder([]Entity{EntityCustomers})
	rec.update(EntityCustomers, func(m *EntityMetrics) { m.Read = 1 })

	snap := rec.snapshot()
	mutated := snap[EntityCustomers]
	mutated.Read = 999
	snap[EntityCustomers] = mutated

	if rec.snapshot()[EntityCustomers].Read != 1 {
		t.Error("snapshot must be a copy, internal state mutated")
	}
}

func TestSnapshot_Total(t *testing.T) {
	snap := MetricsSnapshot{
		EntityCustomers: {Read: 10, Inserted: 8, Updated: 2},
		EntityOrders:    {Read: 4, Inserted: 4},
	}
	total := snap.Total()
	if total.Read != 14 || total.Inserted != 12 || total.Updated != 2 {
		t.Errorf("Total = %+v", total)
	}
}

func TestRecorder_ConcurrentUpdate(t *testing.T) {
	rec := newRecorder([]Entity{EntityOrders})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec.update(EntityOrders, func(m *EntityMetrics) { m.Read++ })
		}()
	}
	wg.Wait()
	if got := rec.snapshot()[EntityOrders].Read; got != 100 {
		t.Errorf("Read = %d, want 100", got)
	}
}
