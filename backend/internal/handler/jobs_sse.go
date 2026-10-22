package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/josebianco/go-vue-data-lab/internal/jobs"
)

// JobSSEHeartbeatInterval is how often a comment-only heartbeat frame (": ping")
// is written to an idle stream, so intermediaries (proxies, load balancers,
// CDNs) do not close a connection that has no progress events. It is a variable
// only so tests can shorten it; callers should treat it as a constant.
var JobSSEHeartbeatInterval = 20 * time.Second

// jobSSEEventName is the SSE event name of every progress frame.
const jobSSEEventName = "progress"

// Events handles GET /api/v1/jobs/events, the Server-Sent Events stream of job
// progress. It is guarded by AuthRequiredSSE, so the access token must be passed
// as the "token" query parameter (the browser EventSource API cannot send an
// Authorization header).
//
// Wire format: the server emits one frame per progress update,
//
//	event: progress
//	data: <progress json>
//	<blank line>
//
// where the JSON is the jobs.Progress payload sealed through the opaque id
// codec, so job_id is a v1.* token exactly as in GET /jobs/status. Idle streams
// receive a ": ping" heartbeat comment every JobSSEHeartbeatInterval.
//
// The stream ends when the job reaches a terminal state (the subscription
// channel closes) or when the client disconnects (the request context is done).
//
//	@Summary		Live job progress (Server-Sent Events)
//	@Description	Streams job progress as Server-Sent Events. Each frame is `event: progress` followed by `data: <jobs.Progress JSON>`; `job_id` is an opaque v1.* token. A `: ping` comment heartbeat is sent every ~20s while idle. The stream closes when the job finishes (completed/failed/stopped) or the client disconnects. Because the browser EventSource API cannot set headers, the access token is passed as the `?token=` query parameter; refresh tokens are rejected with 401.
//	@Tags			jobs
//	@Produce		text/event-stream
//	@Param			token	query		string	true	"JWT access token (EventSource cannot send an Authorization header)"
//	@Success		200		{string}	string	"SSE stream"
//	@Failure		401		{object}	ErrorResponse
//	@Router			/jobs/events [get]
func (h *JobsHandler) Events(c *gin.Context) {
	// Subscribe to the manager's broadcast before writing any header, so a job
	// that finishes mid-handshake still delivers its final event.
	progress := h.svc.Subscribe(c.Request.Context())

	ticker := time.NewTicker(JobSSEHeartbeatInterval)
	defer ticker.Stop()

	c.Writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	// Disable proxy buffering (nginx and friends) so frames arrive immediately.
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	for {
		select {
		case <-c.Request.Context().Done():
			// Client disconnected: stop streaming. The subscriber is removed by
			// the manager when its context is cancelled.
			return
		case p, ok := <-progress:
			if !ok {
				// The job reached a terminal state and the manager closed the
				// channel after emitting the final event.
				return
			}
			if err := writeJobProgressFrame(c, p); err != nil {
				// The connection went away mid-write; nothing else to do.
				_ = c.Error(fmt.Errorf("handler: write sse frame: %w", err))
				return
			}
		case <-ticker.C:
			if _, err := c.Writer.WriteString(": ping\n\n"); err != nil {
				_ = c.Error(fmt.Errorf("handler: write sse heartbeat: %w", err))
				return
			}
			c.Writer.Flush()
		}
	}
}

// writeJobProgressFrame marshals one progress payload, seals its opaque ids
// through the request codec (job_id becomes a v1.* token, matching the REST
// endpoints) and writes it as a single SSE frame.
func writeJobProgressFrame(c *gin.Context, p jobs.Progress) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal progress: %w", err)
	}

	if codec := codecFrom(c); codec != nil {
		raw, err = sealJSONValue(raw, codec)
		if err != nil {
			return fmt.Errorf("seal progress payload: %w", err)
		}
	}

	frame := make([]byte, 0, len(raw)+len(jobSSEEventName)+16)
	frame = append(frame, "event: "...)
	frame = append(frame, jobSSEEventName...)
	frame = append(frame, "\ndata: "...)
	frame = append(frame, raw...)
	frame = append(frame, "\n\n"...)

	if _, err := c.Writer.Write(frame); err != nil {
		return fmt.Errorf("write frame: %w", err)
	}
	c.Writer.Flush()
	return nil
}
