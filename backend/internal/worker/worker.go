// Package worker provides a reusable, generic, concurrency-safe worker pool
// used as the concurrency core for batch generators and ETL jobs.
//
// A Pool groups a stream of individual items into fixed-size batches and
// dispatches them to a configurable number of worker goroutines. Each worker
// invokes the user supplied handler with the current (cancellable) context and
// the batch. Panics inside the handler are recovered, recorded and do not kill
// the pool.
//
// Typical usage:
//
//	pool, err := worker.New[Order](worker.Config{Workers: 8, BatchSize: 3500},
//		func(ctx context.Context, batch []Order) error {
//			return repo.BulkInsert(ctx, batch)
//		})
//	if err != nil {
//		return err
//	}
//	pool.Start(ctx)
//	defer pool.Stop()
//
//	for _, o := range orders {
//		if err := pool.Submit(o); err != nil {
//			break
//		}
//	}
package worker

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

// DefaultBatchSize is the batch size used when Config.BatchSize is not set.
// It sits in the middle of the recommended 3000-4000 range used by bulk
// inserts/upserts against PostgreSQL.
const DefaultBatchSize = 3500

// Config holds the configuration of a Pool.
type Config struct {
	// Workers is the number of worker goroutines. When <= 0 it defaults to
	// runtime.NumCPU().
	Workers int
	// BatchSize is the number of items grouped into a single handler call.
	// When <= 0 it defaults to DefaultBatchSize (3500). Production jobs should
	// use a value between 3000 and 4000; smaller values are allowed (and are
	// handy in tests).
	BatchSize int
	// OnProgress, when set, is invoked after every batch with a snapshot of the
	// processed and failed item counters. It must be safe for concurrent use.
	OnProgress func(processed, failed int64)
}

// Metrics is an immutable snapshot of a Pool's counters.
type Metrics struct {
	// Processed is the number of items successfully handled.
	Processed int64
	// Failed is the number of items that could not be handled (handler error or
	// panic).
	Failed int64
	// Batches is the number of batches dispatched to the handler.
	Batches int64
	// Panics is the number of panics recovered from the handler.
	Panics int64
}

// Pool is a generic, concurrency-safe worker pool for items of type T.
type Pool[T any] struct {
	cfg     Config
	handler func(ctx context.Context, batch []T) error

	jobs    chan T
	batches chan []T

	// mu guards closed and serialises channel closure against in-flight sends.
	mu     sync.RWMutex
	closed bool

	started    bool
	wg         sync.WaitGroup
	cancel     context.CancelFunc
	cancelOnce sync.Once

	ctx context.Context

	errMu sync.Mutex
	errs  []error

	processed    int64
	failed       int64
	batchesCount int64
	panicCount   int64
}

// New builds a Pool. The handler must not be nil, otherwise an error wrapping
// the cause is returned. The pool is not started until Start is called.
func New[T any](cfg Config, handler func(ctx context.Context, batch []T) error) (*Pool[T], error) {
	if handler == nil {
		return nil, fmt.Errorf("worker: el handler no puede ser nil")
	}
	if cfg.Workers <= 0 {
		cfg.Workers = runtime.NumCPU()
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultBatchSize
	}

	ctx, cancel := context.WithCancel(context.Background())

	queueSize := cfg.BatchSize * 2
	if queueSize < 1 {
		queueSize = 1
	}

	return &Pool[T]{
		cfg:     cfg,
		handler: handler,
		// jobs is buffered so producers are not blocked on every single item.
		jobs: make(chan T, queueSize),
		// batches is buffered so workers can pull while the batcher fills.
		batches: make(chan []T, cfg.Workers),
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

// Start launches the batcher and the worker goroutines bound to ctx. Calling
// Start more than once is a no-op. When ctx is cancelled the workers stop and
// the handler's context is cancelled as well.
func (p *Pool[T]) Start(ctx context.Context) {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return
	}
	p.started = true
	// Derive the pool context from the caller's context so cancellation
	// propagates to workers and handlers. Guarded by mu so concurrent Submit
	// calls observe the new context.
	derived, cancel := context.WithCancel(ctx)
	p.ctx = derived
	p.cancel = cancel

	// Register goroutines directly on the pool WaitGroup before spawning so
	// Wait (possibly called right after Start) cannot observe a partial count.
	p.wg.Add(1)
	go p.batcher()
	for i := 0; i < p.cfg.Workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	p.mu.Unlock()
}

// Submit enqueues a single item. It is safe for concurrent use. It returns an
// error if the pool is closed, cancelled or the context completes before the
// item could be enqueued.
func (p *Pool[T]) Submit(item T) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return fmt.Errorf("worker: submit: pool cerrado")
	}
	select {
	case <-p.ctx.Done():
		return fmt.Errorf("worker: submit: %w", p.ctx.Err())
	case p.jobs <- item:
		return nil
	}
}

// SubmitBatch enqueues a slice of items one by one. Empty slices are ignored.
// It is safe for concurrent use.
func (p *Pool[T]) SubmitBatch(items []T) error {
	for _, item := range items {
		if err := p.Submit(item); err != nil {
			return fmt.Errorf("worker: submit batch: %w", err)
		}
	}
	return nil
}

// SubmitStream consumes items from the given channel until it is closed, the
// pool is closed/cancelled or ctx is done. It blocks the calling goroutine and
// is intended to be run in its own goroutine.
func (p *Pool[T]) SubmitStream(ctx context.Context, items <-chan T) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("worker: submit stream: %w", ctx.Err())
		case <-p.ctx.Done():
			return fmt.Errorf("worker: submit stream: %w", p.ctx.Err())
		case item, ok := <-items:
			if !ok {
				return nil
			}
			if err := p.Submit(item); err != nil {
				return fmt.Errorf("worker: submit stream: %w", err)
			}
		}
	}
}

// Close stops accepting new items and signals the batcher to flush any partial
// batch. It is idempotent and safe for concurrent use. It does not block until
// the workers finish; use Wait or Stop for that.
func (p *Pool[T]) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	// Take the write lock so no Submit call is mid-send; this makes closing
	// jobs free of "send on closed channel" races.
	close(p.jobs)
	p.mu.Unlock()
}

// Stop performs a graceful shutdown: it stops accepting items and waits for
// the workers to drain the queue. It is safe to call multiple times.
func (p *Pool[T]) Stop() {
	p.Close()
	p.Wait()
}

// Wait blocks until every worker goroutine has finished. It also releases the
// pool context once workers are done.
func (p *Pool[T]) Wait() {
	p.wg.Wait()
	p.cancelOnce.Do(p.cancel)
}

// Errors returns a copy of the aggregated handler/panic errors. The returned
// slice can be safely mutated by the caller.
func (p *Pool[T]) Errors() []error {
	p.errMu.Lock()
	defer p.errMu.Unlock()
	out := make([]error, len(p.errs))
	copy(out, p.errs)
	return out
}

// Metrics returns a snapshot of the pool counters.
func (p *Pool[T]) Metrics() Metrics {
	return Metrics{
		Processed: atomic.LoadInt64(&p.processed),
		Failed:    atomic.LoadInt64(&p.failed),
		Batches:   atomic.LoadInt64(&p.batchesCount),
		Panics:    atomic.LoadInt64(&p.panicCount),
	}
}

// IsClosed reports whether Close has been called.
func (p *Pool[T]) IsClosed() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.closed
}

// batcher collects individual jobs into fixed-size batches.
func (p *Pool[T]) batcher() {
	defer p.wg.Done()
	// Closing batches here is what eventually makes the workers exit.
	defer close(p.batches)

	batch := make([]T, 0, p.cfg.BatchSize)
	flush := func() bool {
		if len(batch) == 0 {
			return true
		}
		select {
		case p.batches <- batch:
			batch = make([]T, 0, p.cfg.BatchSize)
			return true
		case <-p.ctx.Done():
			return false
		}
	}

	for {
		select {
		case <-p.ctx.Done():
			return
		case item, ok := <-p.jobs:
			if !ok {
				// Graceful close: flush the remainder and exit.
				_ = flush()
				return
			}
			batch = append(batch, item)
			if len(batch) >= p.cfg.BatchSize {
				if !flush() {
					return
				}
			}
		}
	}
}

// worker consumes batches and invokes the handler.
func (p *Pool[T]) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case batch, ok := <-p.batches:
			if !ok {
				return
			}
			p.processBatch(batch)
		}
	}
}

// processBatch runs the handler for one batch, recovering panics and recording
// errors and metrics. Failures never terminate the pool.
func (p *Pool[T]) processBatch(batch []T) {
	if err := p.safeHandle(batch); err != nil {
		atomic.AddInt64(&p.failed, int64(len(batch)))
		p.record(err)
	} else {
		atomic.AddInt64(&p.processed, int64(len(batch)))
	}
	atomic.AddInt64(&p.batchesCount, 1)

	if p.cfg.OnProgress != nil {
		p.cfg.OnProgress(atomic.LoadInt64(&p.processed), atomic.LoadInt64(&p.failed))
	}
}

// safeHandle isolates the handler call so panics are converted into errors.
func (p *Pool[T]) safeHandle(batch []T) (err error) {
	defer func() {
		if r := recover(); r != nil {
			atomic.AddInt64(&p.panicCount, 1)
			err = fmt.Errorf("worker: pánico recuperado en handler: %v", r)
		}
	}()
	if hErr := p.handler(p.ctx, batch); hErr != nil {
		return fmt.Errorf("worker: error en handler: %w", hErr)
	}
	return nil
}

// record appends an error to the aggregate under the error mutex.
func (p *Pool[T]) record(err error) {
	p.errMu.Lock()
	p.errs = append(p.errs, err)
	p.errMu.Unlock()
}
