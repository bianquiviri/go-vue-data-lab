package jobs

import (
	"context"
	"sync"
)

// subscribeState holds live SSE subscribers for job progress.
type subscribeState struct {
	mu   sync.RWMutex
	subs map[chan Progress]struct{}
}

// Subscribe registers a subscriber and returns its channel. The caller must
// drain the channel; the manager closes it when the current job ends.
func (m *Manager) Subscribe(ctx context.Context) <-chan Progress {
	ch := make(chan Progress, 8)
	m.subs.mu.Lock()
	if m.subs.subs == nil {
		m.subs.subs = make(map[chan Progress]struct{})
	}
	m.subs.subs[ch] = struct{}{}
	m.subs.mu.Unlock()

	// Snapshot of the most recent job on subscription.
	m.mu.RLock()
	var latest *activeJob
	for _, j := range m.jobs {
		if latest == nil || j.startedAt.After(latest.startedAt) {
			latest = j
		}
	}
	m.mu.RUnlock()
	if latest != nil {
		select {
		case ch <- latest.progress():
		default:
		}
	}

	go func() {
		<-ctx.Done()
		m.unsubscribe(ch)
	}()

	return ch
}

func (m *Manager) unsubscribe(ch chan Progress) {
	m.subs.mu.Lock()
	delete(m.subs.subs, ch)
	m.subs.mu.Unlock()
}

// broadcast sends a snapshot to every subscriber without blocking the caller.
// A slow subscriber drops the event.
func (m *Manager) broadcast(p Progress) {
	m.subs.mu.RLock()
	defer m.subs.mu.RUnlock()
	for ch := range m.subs.subs {
		select {
		case ch <- p:
		default:
		}
	}
}

// broadcastTerminal sends the terminal snapshot and closes all subscriber
// channels. It must not be called concurrently in a way that races with
// Subscribe/unsubscribe; the channels are removed on send/close under the
// subs mutex.
func (m *Manager) broadcastTerminal(p Progress) {
	m.subs.mu.Lock()
	subs := m.subs.subs
	m.subs.subs = make(map[chan Progress]struct{})
	m.subs.mu.Unlock()

	for ch := range subs {
		select {
		case ch <- p:
		default:
		}
		close(ch)
	}
}
