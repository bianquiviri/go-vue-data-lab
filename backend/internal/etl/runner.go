package etl

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/worker"
)

// errDuplicate is the reason recorded for rows dropped by deduplication.
var errDuplicate = errors.New("duplicado en el lote")

// SkipRecord documents a single row that was not loaded.
type SkipRecord struct {
	Entity Entity
	Key    string
	Reason string
}

// Result is the outcome of a Run.
type Result struct {
	// Metrics is the final metrics snapshot.
	Metrics MetricsSnapshot
	// Skipped lists the skipped rows (capped by Config.MaxSkipped).
	Skipped []SkipRecord
	// Errors lists the per-batch load errors (empty on full success).
	Errors []error
	// Checkpoint is the resumable state at the end of the run.
	Checkpoint Checkpoint
	// Duration is the wall-clock time spent.
	Duration time.Duration
}

// HasErrors reports whether any batch failed.
func (r *Result) HasErrors() bool { return len(r.Errors) > 0 }

// Option customises a Runner.
type Option func(*Runner)

// WithLogger sets the structured logger. A nil logger is ignored.
func WithLogger(l *zap.Logger) Option {
	return func(r *Runner) {
		if l != nil {
			r.log = l
		}
	}
}

// WithCheckpointStore sets the checkpoint store. A nil store is ignored.
func WithCheckpointStore(s CheckpointStore) Option {
	return func(r *Runner) {
		if s != nil {
			r.store = s
		}
	}
}

// Runner orchestrates the extract -> transform -> load pipeline for every
// configured entity, migrating rows from a Source into a Loader.
type Runner struct {
	cfg    Config
	src    Source
	loader Loader
	store  CheckpointStore
	log    *zap.Logger

	rec *recorder

	stateMu sync.Mutex
	state   Checkpoint

	totalsMu sync.Mutex
	totals   map[Entity]int64

	skipMu sync.Mutex
	skips  []SkipRecord

	errMu sync.Mutex
	errs  []error
}

// New builds a Runner. It fails when src or loader is nil, or when an unknown
// entity is requested.
func New(cfg Config, src Source, loader Loader, opts ...Option) (*Runner, error) {
	if src == nil {
		return nil, fmt.Errorf("etl: el source no puede ser nil")
	}
	if loader == nil {
		return nil, fmt.Errorf("etl: el loader no puede ser nil")
	}
	cfg.applyDefaults()
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	r := &Runner{
		cfg:    cfg,
		src:    src,
		loader: loader,
		store:  NewMemoryStore(),
		log:    zap.NewNop(),
		rec:    newRecorder(cfg.entities()),
		state:  NewCheckpoint(),
		totals: make(map[Entity]int64),
		skips:  make([]SkipRecord, 0),
		errs:   make([]error, 0),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

// Run migrates the configured entities. It returns a Result even when a
// per-batch error was recorded (unless FailFast is set, in which case it also
// returns an error). Fatal errors (checkpoint, source, context) abort the run
// and are returned.
func (r *Runner) Run(ctx context.Context) (*Result, error) {
	started := time.Now()

	cp, err := r.store.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("etl: cargar checkpoint: %w", err)
	}
	r.stateMu.Lock()
	r.state = cp
	r.stateMu.Unlock()

	runCtx := ctx
	cancel := func() {}
	if r.cfg.FailFast {
		runCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	var runErr error
	for _, entity := range r.cfg.entities() {
		if err := ctx.Err(); err != nil {
			runErr = fmt.Errorf("etl: contexto cancelado antes de %s: %w", entity, err)
			break
		}
		if r.State().IsCompleted(entity) {
			r.log.Info("entidad omitida (ya completada)", zap.String("entity", string(entity)))
			continue
		}
		if err := r.runEntity(runCtx, entity, cancel); err != nil {
			runErr = err
			r.log.Error("entidad fallida",
				zap.String("entity", string(entity)), zap.Error(err))
			break
		}
	}

	return r.result(time.Since(started)), runErr
}

// State returns a copy of the current checkpoint.
func (r *Runner) State() Checkpoint {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	return r.state.Clone()
}

// Metrics returns a snapshot of the run metrics.
func (r *Runner) Metrics() MetricsSnapshot { return r.rec.snapshot() }

// runEntity migrates a single entity end to end.
func (r *Runner) runEntity(ctx context.Context, entity Entity, cancel context.CancelFunc) error {
	total, err := r.src.Count(ctx, entity)
	if err != nil {
		return fmt.Errorf("etl: contar %s: %w", entity, err)
	}
	r.setTotal(entity, total)

	r.log.Info("entidad iniciada",
		zap.String("entity", string(entity)),
		zap.Int64("source_total", total),
		zap.String("from_key", r.State().Key(entity)))

	afterKey := r.State().Key(entity)

	switch entity {
	case EntityCustomers:
		after, err := parseUUIDKey(afterKey)
		if err != nil {
			return fmt.Errorf("etl: checkpoint de %s inválido: %w", entity, err)
		}
		return runPipeline[uuid.UUID, models.Customer](ctx, r, entity, cancel,
			pipelineArgs[uuid.UUID, models.Customer]{
				after:     after,
				fetch:     r.src.FetchCustomers,
				keyOf:     func(c models.Customer) uuid.UUID { return c.ID },
				keyString: func(id uuid.UUID) string { return id.String() },
				transform: func(c models.Customer) (models.Customer, error) { return TransformCustomer(c) },
				dedupe: func(rows []models.Customer) ([]models.Customer, []models.Customer) {
					return dedupeBy(rows, customerKeys)
				},
				load: r.loader.UpsertCustomers,
			})

	case EntityProducts:
		after, err := parseUUIDKey(afterKey)
		if err != nil {
			return fmt.Errorf("etl: checkpoint de %s inválido: %w", entity, err)
		}
		return runPipeline[uuid.UUID, models.Product](ctx, r, entity, cancel,
			pipelineArgs[uuid.UUID, models.Product]{
				after:     after,
				fetch:     r.src.FetchProducts,
				keyOf:     func(p models.Product) uuid.UUID { return p.ID },
				keyString: func(id uuid.UUID) string { return id.String() },
				transform: func(p models.Product) (models.Product, error) { return TransformProduct(p) },
				dedupe:    func(rows []models.Product) ([]models.Product, []models.Product) { return dedupeBy(rows, productKeys) },
				load:      r.loader.UpsertProducts,
			})

	case EntityOrders:
		after, err := parseUUIDKey(afterKey)
		if err != nil {
			return fmt.Errorf("etl: checkpoint de %s inválido: %w", entity, err)
		}
		var sums map[uuid.UUID]float64
		return runPipeline[uuid.UUID, models.Order](ctx, r, entity, cancel,
			pipelineArgs[uuid.UUID, models.Order]{
				after:     after,
				fetch:     r.src.FetchOrders,
				keyOf:     func(o models.Order) uuid.UUID { return o.ID },
				keyString: func(id uuid.UUID) string { return id.String() },
				prepare: func(pctx context.Context, rows []models.Order) error {
					ids := make([]uuid.UUID, len(rows))
					for i, o := range rows {
						ids[i] = o.ID
					}
					var err error
					sums, err = r.src.OrderItemSums(pctx, ids)
					if err != nil {
						return fmt.Errorf("etl: sumar items de %s: %w", entity, err)
					}
					return nil
				},
				transform: func(o models.Order) (models.Order, error) {
					sum, has := sums[o.ID]
					return TransformOrder(o, OrderOptions{
						ItemSubtotalSum: sum,
						HasItems:        has,
						Strict:          r.cfg.StrictTotals,
					})
				},
				corrected: func(in, out models.Order) bool { return round2(in.TotalAmount) != out.TotalAmount },
				dedupe:    func(rows []models.Order) ([]models.Order, []models.Order) { return dedupeBy(rows, orderKeys) },
				load:      r.loader.UpsertOrders,
			})

	case EntityOrderItems:
		after, err := parseInt64Key(afterKey)
		if err != nil {
			return fmt.Errorf("etl: checkpoint de %s inválido: %w", entity, err)
		}
		return runPipeline[int64, models.OrderItem](ctx, r, entity, cancel,
			pipelineArgs[int64, models.OrderItem]{
				after:     after,
				fetch:     r.src.FetchOrderItems,
				keyOf:     func(i models.OrderItem) int64 { return i.ID },
				keyString: func(id int64) string { return strconv.FormatInt(id, 10) },
				transform: func(i models.OrderItem) (models.OrderItem, error) {
					return TransformOrderItem(i, r.cfg.StrictTotals)
				},
				corrected: func(in, out models.OrderItem) bool { return round2(in.Subtotal) != out.Subtotal },
				dedupe: func(rows []models.OrderItem) ([]models.OrderItem, []models.OrderItem) {
					return dedupeBy(rows, orderItemKeys)
				},
				load: r.loader.UpsertOrderItems,
			})
	}

	return fmt.Errorf("etl: entidad no soportada %q", entity)
}

// tracked is a row queued in the worker pool together with the sequence number
// and checkpoint key of the extraction batch it came from.
type tracked[T any] struct {
	seq     int
	lastKey string
	row     T
}

// pipelineArgs parameterises the generic entity pipeline.
type pipelineArgs[K uuid.UUID | int64, T any] struct {
	after     K
	fetch     func(ctx context.Context, after K, limit int) ([]T, error)
	keyOf     func(T) K
	keyString func(K) string
	// prepare, when set, runs once per extracted batch before transformation
	// (used to fetch order item sums).
	prepare func(ctx context.Context, rows []T) error
	// transform normalizes/validates a single row.
	transform func(T) (T, error)
	// corrected, when set, reports whether transform changed a computed total.
	corrected func(in, out T) bool
	// dedupe splits a batch into kept and dropped (duplicate) rows.
	dedupe func([]T) (kept, dropped []T)
	// load upserts a batch atomically.
	load func(ctx context.Context, rows []T) (LoadResult, error)
}

// runPipeline extracts, transforms and loads one entity concurrently.
func runPipeline[K uuid.UUID | int64, T any](
	ctx context.Context,
	r *Runner,
	entity Entity,
	cancel context.CancelFunc,
	p pipelineArgs[K, T],
) error {
	tracker := newBatchTracker(p.keyString(p.after), func(lastKey string) {
		r.stateMu.Lock()
		r.state.SetKey(entity, lastKey)
		r.stateMu.Unlock()
		// Persist with a detached context: even during a fail-fast cancellation
		// we want the last good key on disk.
		if err := r.persist(context.Background()); err != nil {
			r.log.Warn("no se pudo persistir el checkpoint",
				zap.String("entity", string(entity)), zap.Error(err))
		}
	})

	handler := func(hctx context.Context, batch []tracked[T]) error {
		return loadBatch(hctx, r, entity, p, tracker, batch, cancel)
	}

	pool, err := worker.New(worker.Config{Workers: r.cfg.Workers, BatchSize: r.cfg.BatchSize}, handler)
	if err != nil {
		return fmt.Errorf("etl: crear worker pool: %w", err)
	}
	pool.Start(ctx)
	defer pool.Stop()

	after := p.after
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("etl: %s: %w", entity, err)
		}

		rows, err := p.fetch(ctx, after, r.cfg.ExtractSize)
		if err != nil {
			return fmt.Errorf("etl: extraer %s: %w", entity, err)
		}
		if len(rows) == 0 {
			break
		}

		r.rec.update(entity, func(m *EntityMetrics) { m.Read += int64(len(rows)) })

		if p.prepare != nil {
			if err := p.prepare(ctx, rows); err != nil {
				return fmt.Errorf("etl: preparar lote de %s: %w", entity, err)
			}
		}

		lastRaw := rows[len(rows)-1]
		seq := tracker.register()
		lastKey := p.keyString(p.keyOf(lastRaw))

		valid := make([]T, 0, len(rows))
		for _, row := range rows {
			out, err := p.transform(row)
			if err != nil {
				r.recordSkip(entity, p.keyString(p.keyOf(row)), err)
				continue
			}
			if p.corrected != nil && p.corrected(row, out) {
				r.rec.update(entity, func(m *EntityMetrics) { m.Corrected++ })
			}
			valid = append(valid, out)
		}

		unique, dropped := p.dedupe(valid)
		for _, d := range dropped {
			r.recordSkip(entity, p.keyString(p.keyOf(d)), errDuplicate)
		}
		r.rec.update(entity, func(m *EntityMetrics) { m.Processed += int64(len(unique)) })

		for _, row := range unique {
			if err := pool.Submit(tracked[T]{seq: seq, lastKey: lastKey, row: row}); err != nil {
				return fmt.Errorf("etl: enviar %s al pool: %w", entity, err)
			}
		}

		after = p.keyOf(lastRaw)
	}

	pool.Close()
	pool.Wait()

	if !tracker.allSucceeded() {
		err := fmt.Errorf("etl: %s: uno o más lotes fallaron; checkpoint detenido en %q",
			entity, tracker.lastGoodKey())
		if r.cfg.FailFast {
			return err
		}
		r.recordError(err)
		return nil
	}

	r.stateMu.Lock()
	r.state.SetKey(entity, tracker.lastGoodKey())
	r.state.MarkCompleted(entity)
	r.stateMu.Unlock()
	if err := r.persist(ctx); err != nil {
		return fmt.Errorf("etl: guardar checkpoint final de %s: %w", entity, err)
	}

	r.log.Info("entidad completada", zap.String("entity", string(entity)))
	return nil
}

// loadBatch upserts every sequence group present in a worker batch. Each group
// corresponds to one extraction batch and is loaded atomically.
func loadBatch[K uuid.UUID | int64, T any](
	ctx context.Context,
	r *Runner,
	entity Entity,
	p pipelineArgs[K, T],
	tracker *batchTracker,
	batch []tracked[T],
	cancel context.CancelFunc,
) error {
	groups := make(map[int][]T)
	lastKeys := make(map[int]string)
	seqs := make([]int, 0, len(batch))
	for _, it := range batch {
		if _, ok := groups[it.seq]; !ok {
			seqs = append(seqs, it.seq)
		}
		groups[it.seq] = append(groups[it.seq], it.row)
		lastKeys[it.seq] = it.lastKey
	}

	completed := make(map[int]bool, len(seqs))
	var errs []error
	// Panic safety: any group the loader did not finish is completed as failed
	// so the tracker never stalls waiting for it.
	defer func() {
		for _, seq := range seqs {
			if completed[seq] {
				continue
			}
			completed[seq] = true
			rows := len(groups[seq])
			tracker.complete(seq, false, lastKeys[seq])
			r.rec.update(entity, func(m *EntityMetrics) {
				m.Batches++
				m.Failed += int64(rows)
			})
		}
	}()

	for _, seq := range seqs {
		rows := groups[seq]
		res, err := p.load(ctx, rows)
		completed[seq] = true
		tracker.complete(seq, err == nil, lastKeys[seq])

		r.rec.update(entity, func(m *EntityMetrics) {
			m.Batches++
			m.Inserted += res.Inserted
			m.Updated += res.Updated
			if err != nil {
				m.Failed += int64(len(rows))
			}
		})

		if err != nil {
			wrapped := fmt.Errorf("etl: cargar %s (lote %d): %w", entity, seq, err)
			r.recordError(wrapped)
			errs = append(errs, wrapped)
			r.log.Error("lote fallido",
				zap.String("entity", string(entity)),
				zap.Int("batch", seq),
				zap.Int("rows", len(rows)),
				zap.Error(err))
			if r.cfg.FailFast {
				cancel()
			}
		} else {
			r.log.Debug("lote cargado",
				zap.String("entity", string(entity)),
				zap.Int("batch", seq),
				zap.Int("rows", len(rows)),
				zap.Int64("inserted", res.Inserted),
				zap.Int64("updated", res.Updated))
		}

		if r.cfg.OnProgress != nil {
			r.cfg.OnProgress(Progress{
				Entity:  entity,
				Metrics: r.rec.snapshot()[entity],
				Total:   r.totalFor(entity),
			})
		}
	}
	return errors.Join(errs...)
}

// recordSkip accounts for a skipped row and stores its reason (capped).
func (r *Runner) recordSkip(entity Entity, key string, err error) {
	r.rec.update(entity, func(m *EntityMetrics) { m.Skipped++ })

	r.skipMu.Lock()
	if len(r.skips) < r.cfg.MaxSkipped {
		r.skips = append(r.skips, SkipRecord{Entity: entity, Key: key, Reason: err.Error()})
	}
	r.skipMu.Unlock()

	r.log.Debug("fila omitida",
		zap.String("entity", string(entity)),
		zap.String("key", key),
		zap.String("reason", err.Error()))
}

// recordError appends a batch error.
func (r *Runner) recordError(err error) {
	r.errMu.Lock()
	r.errs = append(r.errs, err)
	r.errMu.Unlock()
}

// persist saves the current checkpoint to the store.
func (r *Runner) persist(ctx context.Context) error {
	if err := r.store.Save(ctx, r.State()); err != nil {
		return fmt.Errorf("etl: guardar checkpoint: %w", err)
	}
	return nil
}

// setTotal records the source row count for an entity.
func (r *Runner) setTotal(entity Entity, n int64) {
	r.totalsMu.Lock()
	r.totals[entity] = n
	r.totalsMu.Unlock()
}

// totalFor returns the source row count for an entity.
func (r *Runner) totalFor(entity Entity) int64 {
	r.totalsMu.Lock()
	defer r.totalsMu.Unlock()
	return r.totals[entity]
}

// result builds the immutable Result.
func (r *Runner) result(d time.Duration) *Result {
	r.skipMu.Lock()
	skips := append([]SkipRecord(nil), r.skips...)
	r.skipMu.Unlock()

	r.errMu.Lock()
	errs := append([]error(nil), r.errs...)
	r.errMu.Unlock()

	return &Result{
		Metrics:    r.rec.snapshot(),
		Skipped:    skips,
		Errors:     errs,
		Checkpoint: r.State(),
		Duration:   d,
	}
}
