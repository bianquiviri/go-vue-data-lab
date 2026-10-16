package etl

import (
	"sync"
	"testing"
)

func TestBatchTracker_RegisterIsMonotonic(t *testing.T) {
	tr := newBatchTracker("start", nil)
	if got := tr.register(); got != 0 {
		t.Errorf("first register = %d, want 0", got)
	}
	if got := tr.register(); got != 1 {
		t.Errorf("second register = %d, want 1", got)
	}
}

func TestBatchTracker_ContiguousConfirm(t *testing.T) {
	var confirmed []string
	tr := newBatchTracker("start", func(key string) { confirmed = append(confirmed, key) })

	s0 := tr.register()
	s1 := tr.register()
	tr.complete(s0, true, "k0")
	tr.complete(s1, true, "k1")

	if tr.lastGoodKey() != "k1" {
		t.Errorf("lastGoodKey = %q, want k1", tr.lastGoodKey())
	}
	if !tr.allSucceeded() {
		t.Error("allSucceeded = false, want true")
	}
	if len(confirmed) != 2 || confirmed[0] != "k0" || confirmed[1] != "k1" {
		t.Errorf("confirmed = %v, want [k0 k1]", confirmed)
	}
}

func TestBatchTracker_OutOfOrderConfirm(t *testing.T) {
	var confirmed []string
	tr := newBatchTracker("start", func(key string) { confirmed = append(confirmed, key) })

	s0 := tr.register()
	s1 := tr.register()
	s2 := tr.register()

	tr.complete(s2, true, "k2")
	if tr.lastGoodKey() != "start" {
		t.Errorf("lastGoodKey = %q, want start (nothing contiguous yet)", tr.lastGoodKey())
	}
	tr.complete(s0, true, "k0")
	if tr.lastGoodKey() != "k0" {
		t.Errorf("lastGoodKey = %q, want k0", tr.lastGoodKey())
	}
	tr.complete(s1, true, "k1")
	// With s1 done the whole contiguous chain (0,1,2) has succeeded, so the
	// checkpoint may advance all the way to k2.
	if tr.lastGoodKey() != "k2" {
		t.Errorf("lastGoodKey = %q, want k2 (contiguous chain completed)", tr.lastGoodKey())
	}
	if len(confirmed) != 3 || confirmed[0] != "k0" || confirmed[1] != "k1" || confirmed[2] != "k2" {
		t.Errorf("confirmed = %v, want [k0 k1 k2]", confirmed)
	}
	if !tr.allSucceeded() {
		t.Error("allSucceeded = false, want true")
	}
}

func TestBatchTracker_FailureFreezesCheckpoint(t *testing.T) {
	var confirmed []string
	tr := newBatchTracker("start", func(key string) { confirmed = append(confirmed, key) })

	s0 := tr.register()
	s1 := tr.register()
	s2 := tr.register()

	tr.complete(s0, true, "k0")
	tr.complete(s1, false, "k1")
	tr.complete(s2, true, "k2")

	if tr.lastGoodKey() != "k0" {
		t.Errorf("lastGoodKey = %q, want k0 (must not advance past failure)", tr.lastGoodKey())
	}
	if tr.allSucceeded() {
		t.Error("allSucceeded = true, want false")
	}
	if len(confirmed) != 1 || confirmed[0] != "k0" {
		t.Errorf("confirmed = %v, want [k0]", confirmed)
	}
}

func TestBatchTracker_AllSucceededFalseWhenPending(t *testing.T) {
	tr := newBatchTracker("start", nil)
	tr.register()
	if tr.allSucceeded() {
		t.Error("allSucceeded = true with an uncompleted batch, want false")
	}
}

func TestBatchTracker_ConcurrentComplete(t *testing.T) {
	tr := newBatchTracker("start", nil)
	const n = 200
	seqs := make([]int, n)
	for i := range seqs {
		seqs[i] = tr.register()
	}

	var wg sync.WaitGroup
	for i := n - 1; i >= 0; i-- { // submit in reverse to stress reordering
		wg.Add(1)
		go func(seq int) {
			defer wg.Done()
			tr.complete(seq, true, "k")
		}(seqs[i])
	}
	wg.Wait()

	if !tr.allSucceeded() {
		t.Error("allSucceeded = false, want true")
	}
	if tr.lastGoodKey() != "k" {
		t.Errorf("lastGoodKey = %q, want k", tr.lastGoodKey())
	}
}
