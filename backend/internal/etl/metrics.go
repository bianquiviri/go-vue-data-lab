package etl

import "sync"

// EntityMetrics accumulates the counters of a single entity.
type EntityMetrics struct {
	// Read is the number of raw rows extracted.
	Read int64
	// Processed is the number of valid, deduplicated rows submitted for load.
	Processed int64
	// Inserted and Updated are the outcomes reported by the loader.
	Inserted int64
	Updated  int64
	// Skipped is the number of rows discarded by validation or deduplication.
	Skipped int64
	// Corrected is the number of rows whose computed totals were fixed.
	Corrected int64
	// Failed is the number of rows in batches that failed to load.
	Failed int64
	// Batches is the number of load batches executed.
	Batches int64
}

// MetricsSnapshot is an immutable copy of the run metrics, keyed by entity.
type MetricsSnapshot map[Entity]EntityMetrics

// Total sums every entity's counters.
func (s MetricsSnapshot) Total() EntityMetrics {
	var t EntityMetrics
	for _, m := range s {
		t.Read += m.Read
		t.Processed += m.Processed
		t.Inserted += m.Inserted
		t.Updated += m.Updated
		t.Skipped += m.Skipped
		t.Corrected += m.Corrected
		t.Failed += m.Failed
		t.Batches += m.Batches
	}
	return t
}

// Progress is emitted after each loaded batch.
type Progress struct {
	Entity  Entity
	Metrics EntityMetrics
	// Total is the best-effort source row count for the entity.
	Total int64
}

// recorder is the concurrency-safe metrics accumulator used by the Runner.
type recorder struct {
	mu sync.Mutex
	m  MetricsSnapshot
}

// newRecorder builds a recorder pre-populated with every entity.
func newRecorder(entities []Entity) *recorder {
	m := make(MetricsSnapshot, len(entities))
	for _, e := range entities {
		m[e] = EntityMetrics{}
	}
	return &recorder{m: m}
}

// update applies fn to the entity's counters under the lock.
func (r *recorder) update(entity Entity, fn func(*EntityMetrics)) {
	r.mu.Lock()
	m := r.m[entity]
	fn(&m)
	r.m[entity] = m
	r.mu.Unlock()
}

// snapshot returns a deep copy of the current metrics.
func (r *recorder) snapshot() MetricsSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(MetricsSnapshot, len(r.m))
	for k, v := range r.m {
		out[k] = v
	}
	return out
}
