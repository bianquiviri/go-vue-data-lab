package etl

import "sync"

// batchOutcome is the terminal state of one submitted batch.
type batchOutcome struct {
	ok      bool
	lastKey string
}

// batchTracker guarantees the checkpoint only ever advances past a contiguous
// run of successful batches.
//
// Batches are registered in extraction order (monotonic sequence numbers) but
// complete concurrently through the worker pool. The tracker advances a
// "confirmed" pointer over contiguous completed batches; the checkpoint is
// moved to the last key of the last confirmed successful batch. A single failed
// batch freezes the checkpoint forever, so a resume re-processes everything from
// the last good batch onward (upserts are idempotent, so replays are safe).
type batchTracker struct {
	mu        sync.Mutex
	nextSeq   int
	confirmed int
	pending   map[int]batchOutcome
	lastKey   string
	broken    bool
	onConfirm func(lastKey string)
}

// newBatchTracker builds a tracker starting at startKey.
func newBatchTracker(startKey string, onConfirm func(string)) *batchTracker {
	return &batchTracker{
		pending:   make(map[int]batchOutcome),
		lastKey:   startKey,
		onConfirm: onConfirm,
	}
}

// register returns the next sequence number. Callers must register batches in
// extraction order.
func (t *batchTracker) register() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	seq := t.nextSeq
	t.nextSeq++
	return seq
}

// complete records the outcome of batch seq and advances the confirmed pointer
// as far as contiguous outcomes allow.
func (t *batchTracker) complete(seq int, ok bool, lastKey string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if seq < t.confirmed {
		return // late duplicate
	}
	if _, done := t.pending[seq]; done {
		return
	}
	t.pending[seq] = batchOutcome{ok: ok, lastKey: lastKey}

	for {
		outcome, exists := t.pending[t.confirmed]
		if !exists {
			return
		}
		delete(t.pending, t.confirmed)
		if !outcome.ok {
			t.broken = true
		} else if !t.broken {
			t.lastKey = outcome.lastKey
			if t.onConfirm != nil {
				t.onConfirm(outcome.lastKey)
			}
		}
		t.confirmed++
	}
}

// lastGoodKey returns the key up to which the checkpoint is safe.
func (t *batchTracker) lastGoodKey() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastKey
}

// allSucceeded reports whether every registered batch completed successfully.
func (t *batchTracker) allSucceeded() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.broken && t.confirmed == t.nextSeq
}
