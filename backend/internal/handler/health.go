// Package handler contains the HTTP transport layer: Gin handlers that decode
// requests, delegate to services and encode responses.
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthResponse is the JSON payload returned by the health endpoint.
type HealthResponse struct {
	Status  string `json:"status"`
	Time    string `json:"time"`
	Version string `json:"version"`
}

// HealthHandler serves liveness/readiness probes.
type HealthHandler struct {
	version string
	now     func() time.Time
}

// NewHealthHandler builds a HealthHandler reporting the given version.
func NewHealthHandler(version string) *HealthHandler {
	return &HealthHandler{version: version, now: time.Now}
}

// Health handles GET /api/v1/health.
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:  "ok",
		Time:    h.now().UTC().Format(time.RFC3339),
		Version: h.version,
	})
}
