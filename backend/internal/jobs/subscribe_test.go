package jobs

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// recvProgress receives a Progress from ch, failing the test if it does not
// arrive within the timeout.
func recvProgress(t *testing.T, ch <-chan Progress, timeout time.Duration) Progress {
	t.Helper()
	select {
	case p, ok := <-ch:
		if !ok {
			t.Fatal("progress channel closed unexpectedly")
		}
		return p
	case <-time.After(timeout):
		t.Fatal("timed out waiting for progress")
		return Progress{}
	}
}

func TestSubscribeSendsIdleSnapshotWhenNoJob(t *testing.T) {
	m := newManager(t, newFakeRepo())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := m.Subscribe(ctx)
	p := recvProgress(t, ch, time.Second)
	if p.Status != StatusIdle {
		t.Errorf("expected idle snapshot, got %q", p.Status)
	}
	if p.JobID != uuid.Nil {
		t.Errorf("expected nil job id on idle snapshot, got %v", p.JobID)
	}
}

func TestSubscribeSendsCurrentSnapshotWhenJobRunning(t *testing.T) {
	repo := newFakeRepo()
	m := newManager(t, repo)
	release := make(chan struct{})
	mustRegister(t, m, TypeGenerate, func(context.Context, func(EntityProgress)) (map[string]any, error) {
		<-release
		return nil, nil
	})

	id, err := m.Start(context.Background(), TypeGenerate, nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	// Wait for the job to actually be running before subscribing.
	waitFor(t, 2*time.Second, func() bool {
		st, _ := m.Status()
		return st.Running
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := m.Subscribe(ctx)
	p := recvProgress(t, ch, time.Second)
	if p.JobID != id {
		t.Errorf("expected snapshot job id %v, got %v", id, p.JobID)
	}
	if p.Status != StatusRunning {
		t.Errorf("expected running snapshot, got %q", p.Status)
	}

	close(release)
}

func TestSubscribeBroadcastsProgressToAllSubscribers(t *testing.T) {
	repo := newFakeRepo()
	m := newManager(t, repo)
	progressed := make(chan struct{}, 1)
	mustRegister(t, m, TypeGenerate, func(_ context.Context, report func(EntityProgress)) (map[string]any, error) {
		report(EntityProgress{Entity: "customers", Done: 7, Total: 10, Processed: 7})
		progressed <- struct{}{}
		<-make(chan struct{}) // keep the job alive so subscribers observe running state
		return nil, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := m.Subscribe(ctx)
	b := m.Subscribe(ctx)

	// Drain the initial idle snapshots.
	recvProgress(t, a, time.Second)
	recvProgress(t, b, time.Second)

	if _, err := m.Start(context.Background(), TypeGenerate, nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	<-progressed

	aSeen := waitForProcessed(t, a, 7, 2*time.Second)
	bSeen := waitForProcessed(t, b, 7, 2*time.Second)
	if !aSeen || !bSeen {
		t.Fatalf("expected both subscribers to see processed=7 (a=%v b=%v)", aSeen, bSeen)
	}
}

// waitForProcessed drains ch until it observes the wanted processed count.
func waitForProcessed(t *testing.T, ch <-chan Progress, want int64, timeout time.Duration) bool {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case p, ok := <-ch:
			if !ok {
				return false
			}
			if p.Processed == want {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

func TestSubscribeSlowSubscriberDoesNotBlockJob(t *testing.T) {
	repo := newFakeRepo()
	m := newManager(t, repo)

	// The job reports more events than the subscriber buffer can hold; none of
	// them must block, so the job completes.
	done := make(chan struct{})
	mustRegister(t, m, TypeGenerate, func(_ context.Context, report func(EntityProgress)) (map[string]any, error) {
		for i := 0; i < subscriberBuffer*4; i++ {
			report(EntityProgress{Entity: "customers", Done: int64(i + 1), Total: 100, Processed: int64(i + 1)})
		}
		close(done)
		return nil, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := m.Subscribe(ctx)
	recvProgress(t, ch, time.Second) // initial snapshot; then never drain again

	if _, err := m.Start(context.Background(), TypeGenerate, nil); err != nil {
		t.Fatalf("start: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("slow subscriber blocked the job")
	}
	waitFor(t, 2*time.Second, func() bool {
		st, _ := m.Status()
		return st.Status == StatusCompleted
	})
}

func TestSubscribeClosesChannelWhenJobTerminates(t *testing.T) {
	repo := newFakeRepo()
	m := newManager(t, repo)
	mustRegister(t, m, TypeGenerate, func(_ context.Context, report func(EntityProgress)) (map[string]any, error) {
		report(EntityProgress{Entity: "customers", Done: 1, Total: 1, Processed: 1})
		return nil, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := m.Subscribe(ctx)
	recvProgress(t, ch, time.Second) // initial snapshot

	if _, err := m.Start(context.Background(), TypeGenerate, nil); err != nil {
		t.Fatalf("start: %v", err)
	}

	terminal := waitForTerminal(t, ch, 3*time.Second)
	if terminal == nil {
		t.Fatal("expected a terminal progress event before the channel closed")
	}
	if terminal.Status != StatusCompleted {
		t.Errorf("expected completed as final event, got %q", terminal.Status)
	}

	// After the terminal event the channel must be closed.
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed after the terminal event")
		}
	case <-time.After(time.Second):
		t.Error("channel not closed after the job terminated")
	}
}

// waitForTerminal drains ch until a terminal status arrives or the channel
// closes; it returns the terminal progress, or nil when closed without one.
func waitForTerminal(t *testing.T, ch <-chan Progress, timeout time.Duration) *Progress {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case p, ok := <-ch:
			if !ok {
				return nil
			}
			if isTerminal(p.Status) {
				return &p
			}
		case <-deadline:
			t.Fatal("timed out waiting for a terminal event")
			return nil
		}
	}
}

func TestSubscribeClosesChannelOnContextCancel(t *testing.T) {
	m := newManager(t, newFakeRepo())

	ctx, cancel := context.WithCancel(context.Background())
	ch := m.Subscribe(ctx)
	recvProgress(t, ch, time.Second) // initial snapshot

	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed after the subscriber context was cancelled")
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed after ctx cancellation")
	}

	// Cancellation must remove the subscriber so it leaves no leak.
	waitFor(t, time.Second, func() bool { return m.subscriberCount() == 0 })
}

func TestSubscribeUnsubscribeRemovesSubscriber(t *testing.T) {
	m := newManager(t, newFakeRepo())

	ctx, cancel := context.WithCancel(context.Background())
	_ = m.Subscribe(ctx)
	_ = m.Subscribe(ctx)
	if got := m.subscriberCount(); got != 2 {
		t.Fatalf("expected 2 subscribers, got %d", got)
	}

	cancel()
	waitFor(t, time.Second, func() bool { return m.subscriberCount() == 0 })
}

func TestSubscribeConcurrentSubscribersRaceFree(t *testing.T) {
	m := newManager(t, newFakeRepo())
	release := make(chan struct{})
	mustRegister(t, m, TypeGenerate, func(_ context.Context, report func(EntityProgress)) (map[string]any, error) {
		for i := 0; i < 50; i++ {
			report(EntityProgress{Entity: "customers", Done: int64(i + 1), Total: 50, Processed: int64(i + 1)})
		}
		return nil, nil
	})

	for i := 0; i < 8; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		ch := m.Subscribe(ctx)
		go func() {
			<-release
			cancel()
			for range ch { //nolint:revive // drain until closed
			}
		}()
	}

	if _, err := m.Start(context.Background(), TypeGenerate, nil); err != nil {
		t.Fatalf("start: %v", err)
	}

	waitFor(t, 3*time.Second, func() bool {
		st, _ := m.Status()
		return !st.Running
	})
	close(release)
	waitFor(t, time.Second, func() bool { return m.subscriberCount() == 0 })
}

func TestStartNotifiesRunningState(t *testing.T) {
	repo := newFakeRepo()
	m := newManager(t, repo)
	release := make(chan struct{})
	mustRegister(t, m, TypeGenerate, func(context.Context, func(EntityProgress)) (map[string]any, error) {
		<-release
		return nil, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := m.Subscribe(ctx)
	recvProgress(t, ch, time.Second) // idle snapshot

	if _, err := m.Start(context.Background(), TypeGenerate, nil); err != nil {
		t.Fatalf("start: %v", err)
	}

	sawRunning := false
	deadline := time.After(2 * time.Second)
	for !sawRunning {
		select {
		case p := <-ch:
			if p.Status == StatusRunning {
				sawRunning = true
			}
		case <-deadline:
			t.Fatal("expected a running event after Start")
		}
	}
	close(release)
}

func TestStopNotifiesStoppedState(t *testing.T) {
	repo := newFakeRepo()
	m := newManager(t, repo)
	mustRegister(t, m, TypeMigrate, func(ctx context.Context, _ func(EntityProgress)) (map[string]any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := m.Subscribe(ctx)
	recvProgress(t, ch, time.Second)

	if _, err := m.Start(context.Background(), TypeMigrate, nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		st, _ := m.Status()
		return st.Running
	})

	if err := m.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	final := waitForTerminal(t, ch, 3*time.Second)
	if final == nil {
		t.Fatal("expected a terminal event after Stop")
	}
	if final.Status != StatusStopped {
		t.Errorf("expected stopped as final event, got %q", final.Status)
	}
}

// panicGuard collects the panics raised by the goroutines of a concurrency
// test, so a "send on closed channel" crash is reported as a test failure
// instead of taking down the whole test binary.
type panicGuard struct {
	mu     sync.Mutex
	panics []any
}

// catch must be deferred at the top of a goroutine body: it turns a panic into
// a recorded failure.
func (g *panicGuard) catch() {
	if r := recover(); r != nil {
		g.mu.Lock()
		g.panics = append(g.panics, r)
		g.mu.Unlock()
	}
}

// report fails the test when any panic was recorded.
func (g *panicGuard) report(t *testing.T) {
	t.Helper()
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, r := range g.panics {
		t.Errorf("unexpected panic in concurrent subscriber test: %v", r)
	}
}

// addRawSubscriber registers a subscriber directly in the manager's subscriber
// set, bypassing Subscribe, so the test controls exactly when the channel is
// closed.
func addRawSubscriber(m *Manager) *subscriber {
	s := &subscriber{ch: make(chan Progress, subscriberBuffer), cancel: func() {}}
	m.subs.mu.Lock()
	if m.subs.subs == nil {
		m.subs.subs = make(map[*subscriber]struct{})
	}
	m.subs.subs[s] = struct{}{}
	m.subs.mu.Unlock()
	return s
}

// TestBroadcastConcurrentWithSubscriberClose hammers the broadcast path while
// other goroutines close subscriber channels through both removal paths
// (client disconnect and job termination).
//
// The broadcast must never send on a channel that another goroutine already
// closed: that panic ("send on closed channel") would take down the whole API
// process, not just the offending request.
func TestBroadcastConcurrentWithSubscriberClose(t *testing.T) {
	m := newManager(t, newFakeRepo())

	const (
		poolSize = 32
		rounds   = 200
		removers = 4
	)

	// Each remover owns its own pool of live subscribers, registered before
	// the broadcast starts, so the broadcast always copies live channels into
	// its target list.
	pools := make([][]*subscriber, removers)
	for w := range pools {
		pool := make([]*subscriber, poolSize)
		for i := range pool {
			pool[i] = addRawSubscriber(m)
		}
		pools[w] = pool
	}

	var (
		wg    sync.WaitGroup
		guard panicGuard
	)

	// Broadcaster: pushes progress events to every subscriber.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer guard.catch()
		for i := 0; i < rounds; i++ {
			m.broadcast(Progress{Status: StatusRunning, Processed: int64(i)})
		}
	}()

	// Removers: close live channels while the broadcast is in flight.
	for w := 0; w < removers; w++ {
		wg.Add(1)
		go func(pool []*subscriber) {
			defer wg.Done()
			defer guard.catch()
			for i := 0; i < rounds; i++ {
				idx := i % len(pool)
				s := pool[idx]
				if i%2 == 0 {
					m.removeSubscriber(s)
				} else {
					m.closeSubscriber(s, Progress{Status: StatusCompleted})
				}
				// Keep the pool populated so the broadcaster keeps finding live
				// subscribers to write to.
				pool[idx] = addRawSubscriber(m)
			}
		}(pools[w])
	}

	wg.Wait()
	guard.report(t)

	// The pools must not leak: every subscriber is either still live or was
	// removed exactly once.
	if got := m.subscriberCount(); got != removers*poolSize {
		t.Errorf("expected %d live subscribers, got %d", removers*poolSize, got)
	}
}

// TestSubscribeConcurrentWithTerminalBroadcast subscribes while the manager
// closes every stream (job termination). The initial snapshot must never be
// written to a channel that was closed in the meantime.
func TestSubscribeConcurrentWithTerminalBroadcast(t *testing.T) {
	m := newManager(t, newFakeRepo())

	// A finished job makes closeIfTerminal close the stream right away.
	m.mu.Lock()
	m.current = &activeJob{status: StatusCompleted, entities: map[string]EntityProgress{}}
	m.mu.Unlock()

	const (
		subscribers = 8
		rounds      = 100
	)

	var (
		wg    sync.WaitGroup
		guard panicGuard
	)

	for i := 0; i < subscribers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer guard.catch()
			for j := 0; j < rounds; j++ {
				ch := m.Subscribe(context.Background())
				for range ch { //nolint:revive // drain until the stream closes
				}
			}
		}()
	}

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer guard.catch()
			for j := 0; j < rounds; j++ {
				m.broadcastTerminal(Progress{Status: StatusCompleted})
			}
		}()
	}

	wg.Wait()
	guard.report(t)

	if got := m.subscriberCount(); got != 0 {
		t.Errorf("expected no live subscribers, got %d", got)
	}
}
