package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/handler"
	"github.com/josebianco/go-vue-data-lab/internal/jobs"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// sseEngine mounts the SSE endpoint on a fresh engine with the opaque id codec
// wired, so job ids are sealed in the streamed events.
func sseEngine(svc service.JobControl) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := handler.NewJobsHandler(svc)
	engine := gin.New()
	engine.Use(handler.WithCodec(testCodec))
	engine.GET("/api/v1/jobs/events", h.Events)
	return engine
}

// streamEvents runs a request against the SSE endpoint and returns the recorder
// and a done channel closed once the handler returns.
func streamEvents(t *testing.T, engine *gin.Engine) (*syncRecorder, <-chan struct{}) {
	t.Helper()
	rec := newSyncRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/events", nil)
	done := make(chan struct{})
	go func() {
		engine.ServeHTTP(rec, req)
		close(done)
	}()
	return rec, done
}

// sseProgressChannel builds a channel preloaded with the given events that is
// closed right after, so the handler drains them and terminates.
func sseProgressChannel(events ...jobs.Progress) <-chan jobs.Progress {
	ch := make(chan jobs.Progress, len(events))
	for _, e := range events {
		ch <- e
	}
	close(ch)
	return ch
}

// syncRecorder is a mutex-guarded http.ResponseWriter/Flusher. The standard
// httptest.ResponseRecorder is not safe for concurrent use, and the SSE tests
// need to observe the streamed body while the handler goroutine keeps writing.
type syncRecorder struct {
	mu      sync.Mutex
	header  http.Header
	buf     bytes.Buffer
	status  int
	flushed bool
}

func newSyncRecorder() *syncRecorder {
	return &syncRecorder{header: make(http.Header), status: http.StatusOK}
}

func (r *syncRecorder) Header() http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.header
}

func (r *syncRecorder) WriteHeader(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = status
}

func (r *syncRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.Write(p)
}

// Flush satisfies http.Flusher.
func (r *syncRecorder) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flushed = true
}

func (r *syncRecorder) body() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.String()
}

func (r *syncRecorder) headerSnapshot() http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(http.Header, len(r.header))
	for k, v := range r.header {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// parseSSEFrames splits an SSE body into frames of (event, data) pairs.
func parseSSEFrames(t *testing.T, body string) []struct{ event, data string } {
	t.Helper()
	var frames []struct{ event, data string }
	for _, block := range strings.Split(body, "\n\n") {
		block = strings.TrimRight(block, "\n")
		if block == "" {
			continue
		}
		var frame struct{ event, data string }
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "event: "):
				frame.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				frame.data = strings.TrimPrefix(line, "data: ")
			}
		}
		frames = append(frames, frame)
	}
	return frames
}

func TestJobsEventsStreamsProgressFrames(t *testing.T) {
	svc := newFakeJobControl(t)
	id := uuid.New()
	svc.subscribe = func(context.Context) <-chan jobs.Progress {
		return sseProgressChannel(
			jobs.Progress{JobID: id, JobType: jobs.TypeGenerate, Status: jobs.StatusRunning, Percent: 50},
			jobs.Progress{JobID: id, JobType: jobs.TypeGenerate, Status: jobs.StatusCompleted, Percent: 100},
		)
	}

	rec, done := streamEvents(t, sseEngine(svc))
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not return after the progress channel closed")
	}

	if rec.status != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.status)
	}
	if ct := rec.headerSnapshot().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if cc := rec.headerSnapshot().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
	if xab := rec.headerSnapshot().Get("X-Accel-Buffering"); xab != "no" {
		t.Errorf("X-Accel-Buffering = %q, want no", xab)
	}

	frames := parseSSEFrames(t, rec.body())
	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d (body: %q)", len(frames), rec.body())
	}
	for i, f := range frames {
		if f.event != "progress" {
			t.Errorf("frame %d: event = %q, want progress", i, f.event)
		}
	}
}

func TestJobsEventsSealsJobID(t *testing.T) {
	svc := newFakeJobControl(t)
	id := uuid.New()
	svc.subscribe = func(context.Context) <-chan jobs.Progress {
		return sseProgressChannel(
			jobs.Progress{JobID: id, JobType: jobs.TypeGenerate, Status: jobs.StatusRunning, Percent: 10},
		)
	}

	rec, done := streamEvents(t, sseEngine(svc))
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not return")
	}

	frames := parseSSEFrames(t, rec.body())
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d (body: %q)", len(frames), rec.body())
	}

	var payload struct {
		JobID   string  `json:"job_id"`
		JobType string  `json:"job_type"`
		Status  string  `json:"status"`
		Percent float64 `json:"percent"`
	}
	if err := json.Unmarshal([]byte(frames[0].data), &payload); err != nil {
		t.Fatalf("decode frame data: %v (data: %q)", err, frames[0].data)
	}

	if payload.JobID != sealID(id) {
		t.Errorf("job_id = %q, want sealed token %q", payload.JobID, sealID(id))
	}
	if !strings.HasPrefix(payload.JobID, "v1.") {
		t.Errorf("job_id = %q, want opaque token with %q prefix", payload.JobID, "v1.")
	}
	if payload.JobType != jobs.TypeGenerate || payload.Status != string(jobs.StatusRunning) {
		t.Errorf("unexpected payload: %+v", payload)
	}
	// The raw UUID must never leak into the stream.
	if strings.Contains(rec.body(), id.String()) {
		t.Errorf("raw uuid leaked into stream: %q", rec.body())
	}
}

func TestJobsEventsPassesThroughWithoutCodec(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newFakeJobControl(t)
	id := uuid.New()
	svc.subscribe = func(context.Context) <-chan jobs.Progress {
		return sseProgressChannel(jobs.Progress{JobID: id, Status: jobs.StatusRunning})
	}

	h := handler.NewJobsHandler(svc)
	engine := gin.New()
	engine.GET("/api/v1/jobs/events", h.Events) // no codec wired

	rec, done := streamEvents(t, engine)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not return")
	}

	frames := parseSSEFrames(t, rec.body())
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	var payload struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal([]byte(frames[0].data), &payload); err != nil {
		t.Fatalf("decode frame data: %v", err)
	}
	if payload.JobID != id.String() {
		t.Errorf("job_id = %q, want raw uuid %q without a codec", payload.JobID, id.String())
	}
}

func TestJobsEventsHeartbeat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newFakeJobControl(t)
	blocking := make(chan jobs.Progress) // never receives; forces heartbeats
	release := make(chan struct{})
	svc.subscribe = func(context.Context) <-chan jobs.Progress { return blocking }

	prev := handler.JobSSEHeartbeatInterval
	handler.JobSSEHeartbeatInterval = 20 * time.Millisecond
	t.Cleanup(func() { handler.JobSSEHeartbeatInterval = prev })

	h := handler.NewJobsHandler(svc)
	engine := gin.New()
	engine.GET("/api/v1/jobs/events", h.Events)

	rec := newSyncRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/events", nil)
	done := make(chan struct{})
	go func() {
		engine.ServeHTTP(rec, req)
		close(done)
	}()

	// Wait for at least two heartbeat comment frames.
	deadline := time.After(2 * time.Second)
	for strings.Count(rec.body(), ": ping") < 2 {
		select {
		case <-deadline:
			t.Fatalf("expected heartbeats, got %q", rec.body())
		case <-time.After(5 * time.Millisecond):
		}
	}

	close(release)
	close(blocking)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not return after release")
	}
	if !strings.Contains(rec.body(), ": ping") {
		t.Errorf("expected an SSE comment heartbeat, got %q", rec.body())
	}
}

func TestJobsEventsStopsOnClientDisconnect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newFakeJobControl(t)
	blocking := make(chan jobs.Progress) // never closes on its own
	svc.subscribe = func(context.Context) <-chan jobs.Progress { return blocking }

	h := handler.NewJobsHandler(svc)
	engine := gin.New()
	engine.GET("/api/v1/jobs/events", h.Events)

	ctx, cancel := context.WithCancel(context.Background())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/events", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		engine.ServeHTTP(rec, req)
		close(done)
	}()

	// Give the handler a moment to enter the loop, then disconnect.
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not return after the client disconnected")
	}
}

func TestJobsEventsStopsWhenSubscribeChannelCloses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newFakeJobControl(t)
	svc.subscribe = func(context.Context) <-chan jobs.Progress {
		ch := make(chan jobs.Progress)
		go func() {
			time.Sleep(20 * time.Millisecond)
			ch <- jobs.Progress{Status: jobs.StatusCompleted}
			close(ch)
		}()
		return ch
	}

	h := handler.NewJobsHandler(svc)
	engine := gin.New()
	engine.Use(handler.WithCodec(testCodec))
	engine.GET("/api/v1/jobs/events", h.Events)

	rec, done := streamEvents(t, engine)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not return when the channel closed")
	}
	if !strings.Contains(rec.body(), "event: progress") {
		t.Errorf("expected a progress frame, got %q", rec.body())
	}
}

// TestJobsEventsMultipleFramesAreWellFormed guards the exact wire format:
// "event: progress\ndata: <json>\n\n".
func TestJobsEventsFrameWireFormat(t *testing.T) {
	svc := newFakeJobControl(t)
	svc.subscribe = func(context.Context) <-chan jobs.Progress {
		return sseProgressChannel(jobs.Progress{Status: jobs.StatusRunning, Percent: 1})
	}

	rec, done := streamEvents(t, sseEngine(svc))
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not return")
	}

	body := rec.body()
	if !strings.HasPrefix(body, "event: progress\ndata: ") {
		t.Errorf("unexpected frame prefix: %q", body)
	}
	if !strings.Contains(body, "\n\n") {
		t.Errorf("expected a blank line terminating the frame, got %q", body)
	}
	// Each frame's data must be valid JSON on a single line.
	for _, f := range parseSSEFrames(t, body) {
		if f.event != "progress" {
			continue
		}
		if !json.Valid([]byte(f.data)) {
			t.Errorf("frame data is not valid JSON: %q", f.data)
		}
		if strings.Contains(f.data, "\n") {
			t.Errorf("frame data must be single-line JSON: %q", f.data)
		}
	}
}
