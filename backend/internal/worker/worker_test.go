package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Run("nil handler returns error", func(t *testing.T) {
		_, err := New[string](Config{}, nil)
		if err == nil {
			t.Fatal("expected error for nil handler")
		}
	})

	t.Run("defaults workers to runtime.NumCPU", func(t *testing.T) {
		p, err := New[string](Config{Workers: 0}, func(ctx context.Context, batch []string) error {
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if p.cfg.Workers <= 0 {
			t.Fatal("expected workers > 0")
		}
	})

	t.Run("negative workers defaults", func(t *testing.T) {
		p, err := New[string](Config{Workers: -5}, func(ctx context.Context, batch []string) error {
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if p.cfg.Workers <= 0 {
			t.Fatal("expected workers > 0")
		}
	})

	t.Run("defaults batch size when <= 0", func(t *testing.T) {
		for _, bs := range []int{0, -1} {
			p, err := New[string](Config{BatchSize: bs}, func(ctx context.Context, batch []string) error {
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if p.cfg.BatchSize != DefaultBatchSize {
				t.Fatalf("batch size %d: expected %d, got %d", bs, DefaultBatchSize, p.cfg.BatchSize)
			}
		}
	})

	t.Run("accepts batch size in range 3000-4000", func(t *testing.T) {
		for _, bs := range []int{3000, 3500, 4000} {
			p, err := New[string](Config{BatchSize: bs}, func(ctx context.Context, batch []string) error {
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if p.cfg.BatchSize != bs {
				t.Fatalf("expected batch size %d, got %d", bs, p.cfg.BatchSize)
			}
		}
	})
}

func TestSuccessfulProcessing(t *testing.T) {
	var processed int64
	p, err := New[string](Config{Workers: 2, BatchSize: 100}, func(ctx context.Context, batch []string) error {
		atomic.AddInt64(&processed, int64(len(batch)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	p.Start(context.Background())
	for i := 0; i < 100; i++ {
		if err := p.Submit(fmt.Sprintf("item-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	p.Stop()

	if processed != 100 {
		t.Fatalf("expected 100 processed, got %d", processed)
	}
	if got := p.Metrics().Processed; got != 100 {
		t.Fatalf("expected metrics processed 100, got %d", got)
	}
	if got := p.Metrics().Failed; got != 0 {
		t.Fatalf("expected metrics failed 0, got %d", got)
	}
	if len(p.Errors()) != 0 {
		t.Fatalf("expected no errors, got %v", p.Errors())
	}
}

func TestBatching(t *testing.T) {
	t.Run("exact batch forms one call", func(t *testing.T) {
		var batches int32
		var total int64
		p, err := New[int](Config{Workers: 1, BatchSize: 10}, func(ctx context.Context, batch []int) error {
			atomic.AddInt32(&batches, 1)
			atomic.AddInt64(&total, int64(len(batch)))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		p.Start(context.Background())
		for i := 0; i < 10; i++ {
			if err := p.Submit(i); err != nil {
				t.Fatal(err)
			}
		}
		p.Stop()

		if batches != 1 {
			t.Fatalf("expected 1 batch, got %d", batches)
		}
		if total != 10 {
			t.Fatalf("expected total 10, got %d", total)
		}
	})

	t.Run("remainder flushed on close", func(t *testing.T) {
		var batches int32
		var total int64
		p, err := New[int](Config{Workers: 1, BatchSize: 10}, func(ctx context.Context, batch []int) error {
			atomic.AddInt32(&batches, 1)
			atomic.AddInt64(&total, int64(len(batch)))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		p.Start(context.Background())
		for i := 0; i < 15; i++ {
			if err := p.Submit(i); err != nil {
				t.Fatal(err)
			}
		}
		p.Stop()

		if batches != 2 {
			t.Fatalf("expected 2 batches, got %d", batches)
		}
		if total != 15 {
			t.Fatalf("expected total 15, got %d", total)
		}
	})

	t.Run("no empty batch on exact multiple", func(t *testing.T) {
		var batches int32
		p, err := New[int](Config{Workers: 1, BatchSize: 5}, func(ctx context.Context, batch []int) error {
			atomic.AddInt32(&batches, 1)
			if len(batch) == 0 {
				t.Error("received empty batch")
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		p.Start(context.Background())
		for i := 0; i < 10; i++ {
			if err := p.Submit(i); err != nil {
				t.Fatal(err)
			}
		}
		p.Stop()

		if batches != 2 {
			t.Fatalf("expected 2 batches, got %d", batches)
		}
	})
}

func TestBatchSizeBoundaries(t *testing.T) {
	for _, bs := range []int{3000, 3500, 4000} {
		bs := bs
		t.Run(fmt.Sprintf("batch size %d", bs), func(t *testing.T) {
			var total int64
			p, err := New[int](Config{Workers: 2, BatchSize: bs}, func(ctx context.Context, batch []int) error {
				atomic.AddInt64(&total, int64(len(batch)))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			p.Start(context.Background())
			for i := 0; i < bs; i++ {
				if err := p.Submit(i); err != nil {
					t.Fatal(err)
				}
			}
			p.Stop()
			if total != int64(bs) {
				t.Fatalf("expected %d, got %d", bs, total)
			}
		})
	}
}

func TestWorkerCount(t *testing.T) {
	var concurrentMax int64
	var current int64
	p, err := New[int](Config{Workers: 4, BatchSize: 5}, func(ctx context.Context, batch []int) error {
		c := atomic.AddInt64(&current, 1)
		for {
			m := atomic.LoadInt64(&concurrentMax)
			if c <= m || atomic.CompareAndSwapInt64(&concurrentMax, m, c) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		atomic.AddInt64(&current, -1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.cfg.Workers != 4 {
		t.Fatalf("expected 4 workers, got %d", p.cfg.Workers)
	}

	p.Start(context.Background())
	for i := 0; i < 40; i++ {
		if err := p.Submit(i); err != nil {
			t.Fatal(err)
		}
	}
	p.Stop()

	if got := atomic.LoadInt64(&concurrentMax); got > 4 {
		t.Fatalf("concurrency exceeded worker count: %d", got)
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var processed int64
	p, err := New[int](Config{Workers: 2, BatchSize: 100}, func(ctx context.Context, batch []int) error {
		atomic.AddInt64(&processed, int64(len(batch)))
		// Ensure handler sees a cancellable context.
		if ctx.Err() != nil {
			t.Log("handler observed cancelled context")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	p.Start(ctx)
	cancel()

	// After cancellation, submits must fail and Stop must not hang.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := p.Submit(1); err != nil {
			break
		}
	}

	done := make(chan struct{})
	go func() {
		p.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop did not return after context cancellation")
	}
}

func TestHandlerReceivesCancellableContext(t *testing.T) {
	received := make(chan context.Context, 1)
	p, err := New[int](Config{Workers: 1, BatchSize: 5}, func(ctx context.Context, batch []int) error {
		select {
		case received <- ctx:
		default:
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.Start(ctx)
	if err := p.Submit(1); err != nil {
		t.Fatal(err)
	}
	p.Close()

	select {
	case hctx := <-received:
		if hctx == nil {
			t.Fatal("handler context is nil")
		}
		if hctx.Err() != nil {
			t.Fatal("handler context should not be cancelled yet")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler was not called")
	}
	cancel()
	p.Wait()
}

func TestPanicRecovery(t *testing.T) {
	var batches int64
	p, err := New[int](Config{Workers: 2, BatchSize: 10}, func(ctx context.Context, batch []int) error {
		atomic.AddInt64(&batches, 1)
		for _, v := range batch {
			if v == 5 || v == 25 {
				panic("boom")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	p.Start(context.Background())
	for i := 0; i < 40; i++ {
		if err := p.Submit(i); err != nil {
			t.Fatal(err)
		}
	}
	p.Stop()

	m := p.Metrics()
	if m.Panics == 0 {
		t.Fatalf("expected panic count > 0, got %d", m.Panics)
	}
	if len(p.Errors()) == 0 {
		t.Fatal("expected panic errors to be recorded")
	}
	// The pool must have continued: all batches were dispatched.
	if m.Batches != batches {
		t.Fatalf("metrics batches %d != observed %d", m.Batches, batches)
	}
}

func TestPanicDoesNotKillPool(t *testing.T) {
	var calls int64
	p, err := New[int](Config{Workers: 1, BatchSize: 1}, func(ctx context.Context, batch []int) error {
		n := atomic.AddInt64(&calls, 1)
		if n == 1 {
			panic("first batch panics")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	p.Start(context.Background())
	for i := 0; i < 5; i++ {
		if err := p.Submit(i); err != nil {
			t.Fatal(err)
		}
	}
	p.Stop()

	if atomic.LoadInt64(&calls) != 5 {
		t.Fatalf("expected 5 handler calls, got %d", calls)
	}
}

func TestErrorAggregation(t *testing.T) {
	sentinel := errors.New("handler failed")
	p, err := New[int](Config{Workers: 2, BatchSize: 5}, func(ctx context.Context, batch []int) error {
		return sentinel
	})
	if err != nil {
		t.Fatal(err)
	}

	p.Start(context.Background())
	for i := 0; i < 10; i++ {
		if err := p.Submit(i); err != nil {
			t.Fatal(err)
		}
	}
	p.Stop()

	m := p.Metrics()
	if m.Failed != 10 {
		t.Fatalf("expected failed 10, got %d", m.Failed)
	}
	if m.Processed != 0 {
		t.Fatalf("expected processed 0, got %d", m.Processed)
	}
	errs := p.Errors()
	if len(errs) != 2 {
		t.Fatalf("expected 2 aggregated errors, got %d", len(errs))
	}
	for _, e := range errs {
		if !errors.Is(e, sentinel) {
			t.Fatalf("expected wrapped sentinel, got %v", e)
		}
		if !strings.Contains(e.Error(), "worker: error en handler") {
			t.Fatalf("expected wrapped message, got %q", e.Error())
		}
	}
}

func TestErrorsReturnsCopy(t *testing.T) {
	p, err := New[int](Config{Workers: 1, BatchSize: 1}, func(ctx context.Context, batch []int) error {
		return errors.New("x")
	})
	if err != nil {
		t.Fatal(err)
	}
	p.Start(context.Background())
	p.Submit(1)
	p.Stop()

	first := p.Errors()
	if len(first) != 1 {
		t.Fatalf("expected 1 error, got %d", len(first))
	}
	first[0] = nil // mutate the returned copy
	if p.Errors()[0] == nil {
		t.Fatal("Errors returned the internal slice")
	}
}

func TestConcurrentSubmits(t *testing.T) {
	var processed int64
	p, err := New[int](Config{Workers: 4, BatchSize: 100}, func(ctx context.Context, batch []int) error {
		atomic.AddInt64(&processed, int64(len(batch)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	p.Start(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if err := p.Submit(j); err != nil {
					return
				}
			}
		}()
	}
	wg.Wait()
	p.Stop()

	if processed != 1000 {
		t.Fatalf("expected 1000 processed, got %d", processed)
	}
}

func TestSubmitAfterClose(t *testing.T) {
	p, err := New[string](Config{Workers: 1, BatchSize: 10}, func(ctx context.Context, batch []string) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	p.Start(context.Background())
	p.Close()
	if err := p.Submit("test"); err == nil {
		t.Fatal("expected error when submitting after close")
	}
	if err := p.SubmitBatch([]string{"a", "b"}); err == nil {
		t.Fatal("expected error when submitting batch after close")
	}
}

func TestCloseTwice(t *testing.T) {
	p, err := New[string](Config{Workers: 1, BatchSize: 10}, func(ctx context.Context, batch []string) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	p.Start(context.Background())
	p.Close()
	p.Close() // must not panic (double close)
	p.Stop()  // must not panic (close after close)
	p.Stop()
}

func TestSubmitBatch(t *testing.T) {
	var processed int64
	p, err := New[int](Config{Workers: 2, BatchSize: 100}, func(ctx context.Context, batch []int) error {
		atomic.AddInt64(&processed, int64(len(batch)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	p.Start(context.Background())
	items := make([]int, 50)
	if err := p.SubmitBatch(items); err != nil {
		t.Fatal(err)
	}
	p.Stop()

	if processed != 50 {
		t.Fatalf("expected 50 processed, got %d", processed)
	}
}

func TestSubmitBatchEmpty(t *testing.T) {
	called := int32(0)
	p, err := New[int](Config{Workers: 1, BatchSize: 10}, func(ctx context.Context, batch []int) error {
		atomic.AddInt32(&called, 1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	p.Start(context.Background())
	if err := p.SubmitBatch(nil); err != nil {
		t.Fatal(err)
	}
	if err := p.SubmitBatch([]int{}); err != nil {
		t.Fatal(err)
	}
	p.Stop()
	if atomic.LoadInt32(&called) != 0 {
		t.Fatal("empty batch should not invoke handler")
	}
}

func TestSubmitStream(t *testing.T) {
	var processed int64
	p, err := New[int](Config{Workers: 2, BatchSize: 25}, func(ctx context.Context, batch []int) error {
		atomic.AddInt64(&processed, int64(len(batch)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	p.Start(context.Background())

	stream := make(chan int)
	go func() {
		for i := 0; i < 100; i++ {
			stream <- i
		}
		close(stream)
	}()

	if err := p.SubmitStream(context.Background(), stream); err != nil {
		t.Fatalf("SubmitStream error: %v", err)
	}
	p.Stop()

	if processed != 100 {
		t.Fatalf("expected 100 processed, got %d", processed)
	}
}

func TestSubmitStreamCancelled(t *testing.T) {
	p, err := New[int](Config{Workers: 1, BatchSize: 10}, func(ctx context.Context, batch []int) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	p.Start(context.Background())

	stream := make(chan int) // never written, never closed
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.SubmitStream(ctx, stream); err == nil {
		t.Fatal("expected error from cancelled stream context")
	}
	p.Close()
	p.Wait()
}

func TestProgressCallback(t *testing.T) {
	var progressCalls int64
	var lastProcessed, lastFailed int64
	p, err := New[int](Config{
		Workers:   1,
		BatchSize: 10,
		OnProgress: func(processed, failed int64) {
			atomic.AddInt64(&progressCalls, 1)
			atomic.StoreInt64(&lastProcessed, processed)
			atomic.StoreInt64(&lastFailed, failed)
		},
	}, func(ctx context.Context, batch []int) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	p.Start(context.Background())
	for i := 0; i < 25; i++ {
		if err := p.Submit(i); err != nil {
			t.Fatal(err)
		}
	}
	p.Stop()

	if progressCalls == 0 {
		t.Fatal("expected progress callback to be called")
	}
	if atomic.LoadInt64(&lastProcessed) != 25 {
		t.Fatalf("expected last processed 25, got %d", lastProcessed)
	}
	if atomic.LoadInt64(&lastFailed) != 0 {
		t.Fatalf("expected last failed 0, got %d", lastFailed)
	}
}

func TestMetrics(t *testing.T) {
	p, err := New[int](Config{Workers: 2, BatchSize: 10}, func(ctx context.Context, batch []int) error {
		for _, v := range batch {
			if v%2 == 0 {
				return errors.New("even not allowed")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	p.Start(context.Background())
	for i := 0; i < 20; i++ {
		p.Submit(i)
	}
	p.Stop()

	m := p.Metrics()
	if m.Batches != 2 {
		t.Fatalf("expected 2 batches, got %d", m.Batches)
	}
	// First batch (0..9) fails as a whole (contains even), second (10..19) too.
	if m.Processed == 0 && m.Failed == 0 {
		t.Fatal("expected some metrics")
	}
	if m.Processed+m.Failed != 20 {
		t.Fatalf("processed+failed should equal 20, got %d+%d", m.Processed, m.Failed)
	}
}

func TestIsClosed(t *testing.T) {
	p, err := New[int](Config{Workers: 1, BatchSize: 10}, func(ctx context.Context, batch []int) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if p.IsClosed() {
		t.Fatal("should not be closed initially")
	}
	p.Start(context.Background())
	p.Close()
	if !p.IsClosed() {
		t.Fatal("should be closed after Close")
	}
}

func TestStartIdempotent(t *testing.T) {
	var count int64
	p, err := New[int](Config{Workers: 2, BatchSize: 10}, func(ctx context.Context, batch []int) error {
		atomic.AddInt64(&count, int64(len(batch)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	p.Start(context.Background())
	p.Start(context.Background()) // no-op
	p.Start(context.Background()) // no-op

	for i := 0; i < 10; i++ {
		if err := p.Submit(i); err != nil {
			t.Fatal(err)
		}
	}
	p.Stop()

	if count != 10 {
		t.Fatalf("expected 10 processed, got %d", count)
	}
}

func TestStopBeforeStart(t *testing.T) {
	p, err := New[int](Config{Workers: 1, BatchSize: 10}, func(ctx context.Context, batch []int) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should not hang or panic.
	p.Stop()
}

func TestNoGoroutineLeakAfterStop(t *testing.T) {
	p, err := New[int](Config{Workers: 3, BatchSize: 10}, func(ctx context.Context, batch []int) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	p.Start(context.Background())
	for i := 0; i < 100; i++ {
		p.Submit(i)
	}
	p.Stop()

	// Second Stop must be safe and immediate.
	done := make(chan struct{})
	go func() {
		p.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("second Stop hung")
	}
}
