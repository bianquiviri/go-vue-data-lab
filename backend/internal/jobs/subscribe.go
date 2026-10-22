package jobs

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// subscriberBuffer is the per-subscriber channel capacity. A job may emit
// thousands of progress ticks per second while a client reads far slower, so
// the buffer is deliberately small: an overwhelmed subscriber loses the oldest
// events instead of ever blocking the job.
const subscriberBuffer = 8

// subscriber is one live progress stream. A Manager owns the map and is the
// only writer/owner of ch; removal happens either when the client's context is
// cancelled or when the job reaches a terminal state. Every write to ch and
// every close of ch happens while the manager's subscriber lock is held, so the
// two are always serialised against each other.
type subscriber struct {
	ch     chan Progress
	cancel context.CancelFunc
}

// subscribeState groups the pub/sub bookkeeping under its own mutex so a slow
// broadcast never contends with the job-state mutex (m.mu). Holding them
// separately is what keeps the job from being blocked by a subscriber.
//
// The mutex also serialises every write and every close of a subscriber
// channel: a channel is only ever written or closed while this mutex is held,
// so a broadcast can never send on a channel another goroutine already closed
// (which would panic with "send on closed channel" and take the process down).
type subscribeState struct {
	mu   sync.Mutex
	subs map[*subscriber]struct{}
}

// Subscribe returns a channel that receives the job's progress events until the
// job reaches a terminal state (completed/failed/stopped) or ctx is cancelled
// (the client disconnected).
//
// The returned channel is buffered, and a snapshot of the current progress is
// delivered immediately (an idle Progress when no job has been started), so a
// client never has to wait for the first tick. Broadcasts are non-blocking: a
// subscriber whose buffer is full silently drops the event, which means a slow
// client can never slow down or block the running job.
//
// The channel is owned by the Manager: the caller must only read from it. It is
// closed when ctx is done or the job terminates (the terminal event is emitted
// before closing). Subscribers are removed from the Manager in both cases, so
// cancelled subscriptions leave no leak.
func (m *Manager) Subscribe(ctx context.Context) <-chan Progress {
	if ctx == nil {
		ctx = context.Background()
	}
	subCtx, cancel := context.WithCancel(ctx)
	s := &subscriber{ch: make(chan Progress, subscriberBuffer), cancel: cancel}

	// Take the snapshot before the subscriber is registered, so the job-state
	// lock (m.mu) is never held together with the subscriber lock (m.subs.mu).
	snapshot := m.snapshotProgress()

	// Register the subscriber and deliver the snapshot atomically: holding the
	// subscriber lock across both steps is what guarantees that no broadcast
	// can close the channel in between (the snapshot write would panic with
	// "send on closed channel") and that the snapshot is the first event the
	// client sees.
	m.subs.mu.Lock()
	if m.subs.subs == nil {
		m.subs.subs = make(map[*subscriber]struct{})
	}
	m.subs.subs[s] = struct{}{}
	s.ch <- snapshot // never blocks: the channel is brand new, empty and buffered
	m.subs.mu.Unlock()

	// If the job already finished, close immediately with the terminal event.
	m.closeIfTerminal(s)

	go m.awaitCancel(subCtx, s)
	return s.ch
}

// snapshotProgress returns the current progress without blocking on m.mu while
// the channel is written; safe to call before or after a job starts.
func (m *Manager) snapshotProgress() Progress {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil {
		return Progress{Status: StatusIdle}
	}
	return m.current.progress()
}

// closeIfTerminal enforces the terminal contract at subscribe time: when the
// job has already finished (for example a client connecting after the job
// ended), the subscriber gets the final event and a closed channel immediately
// instead of hanging forever. Broadcasts from the run loop handle the case
// where the job finishes while the subscriber is live.
func (m *Manager) closeIfTerminal(s *subscriber) {
	m.mu.Lock()
	if m.current == nil || !isTerminal(m.current.status) {
		m.mu.Unlock()
		return
	}
	p := m.current.progress()
	m.mu.Unlock()

	m.closeSubscriber(s, p)
}

// awaitCancel removes the subscriber once the client's context is done.
func (m *Manager) awaitCancel(ctx context.Context, s *subscriber) {
	<-ctx.Done()
	m.removeSubscriber(s)
}

// broadcast delivers a progress event to every live subscriber without ever
// blocking. It is called from the job goroutine, so a full buffer drops the
// event for that subscriber only.
//
// The subscriber lock is held across the writes: it serialises them with the
// close of the very same channels, so a subscriber can never be closed while
// this broadcast writes to it. The writes are non-blocking, so holding the
// lock never stalls the running job.
func (m *Manager) broadcast(p Progress) {
	m.subs.mu.Lock()
	dropped := 0
	for s := range m.subs.subs {
		select {
		case s.ch <- p:
		default:
			// Buffer full: drop the event for this subscriber. Never block the
			// job on a slow reader.
			dropped++
		}
	}
	m.subs.mu.Unlock()

	if dropped > 0 {
		m.log.Debug("progress event dropped for slow subscriber", zap.Int("subscribers", dropped))
	}
}

// broadcastTerminal delivers the final event to every live subscriber and
// closes their streams, so clients observe the end of the job without polling.
func (m *Manager) broadcastTerminal(final Progress) {
	m.subs.mu.Lock()
	closed := make([]*subscriber, 0, len(m.subs.subs))
	for s := range m.subs.subs {
		if m.closeSubscriberLocked(s, final) {
			closed = append(closed, s)
		}
	}
	m.subs.mu.Unlock()

	// Cancel the subscription contexts outside the lock: cancelling wakes the
	// awaitCancel goroutine, which calls back into removeSubscriber and would
	// otherwise contend for the very same lock.
	for _, s := range closed {
		s.cancel()
	}
}

// closeSubscriber emits the terminal event (best effort) and closes the
// channel, removing the subscriber from the set.
func (m *Manager) closeSubscriber(s *subscriber, p Progress) {
	m.subs.mu.Lock()
	closed := m.closeSubscriberLocked(s, p)
	m.subs.mu.Unlock()

	if closed {
		s.cancel()
	}
}

// closeSubscriberLocked removes the subscriber from the set, emits the terminal
// event (best effort) and closes its channel. It reports whether the subscriber
// was still registered, i.e. whether this call is the one that closed it.
//
// The subscriber lock must be held: it serialises the write and the close of
// the channel with every other write/close of the same channel, so nobody can
// send on a closed channel. The terminal write is best effort (a full buffer
// drops the event) and never blocks.
func (m *Manager) closeSubscriberLocked(s *subscriber, p Progress) bool {
	if _, ok := m.subs.subs[s]; !ok {
		return false
	}
	delete(m.subs.subs, s)

	select {
	case s.ch <- p:
	default:
		// Best effort: the buffer is full, the terminal event is dropped.
	}
	close(s.ch)
	return true
}

// removeSubscriber drops a subscriber from the set (cancelled client) and
// closes its channel so readers observe the disconnect.
//
// The close happens under the subscriber lock, so it is serialised with the
// broadcasts writing to the same channel: nobody can send on a closed channel.
func (m *Manager) removeSubscriber(s *subscriber) {
	m.subs.mu.Lock()
	defer m.subs.mu.Unlock()
	if _, ok := m.subs.subs[s]; !ok {
		return
	}
	delete(m.subs.subs, s)
	close(s.ch)
}

// subscriberCount reports how many live subscribers are registered. It exists
// so tests can assert that cancelled subscriptions leave no leak.
func (m *Manager) subscriberCount() int {
	m.subs.mu.Lock()
	defer m.subs.mu.Unlock()
	return len(m.subs.subs)
}

// isTerminal reports whether a job status is a terminal lifecycle state. It is
// the predicate the progress-stream tests use to spot the final event.
func isTerminal(s Status) bool {
	switch s {
	case StatusCompleted, StatusFailed, StatusStopped:
		return true
	default:
		return false
	}
}
