package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/josebianco/go-vue-data-lab/internal/handler"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// fakeAdminStats is a controllable service.AdminStats double.
type fakeAdminStats struct {
	snapshot service.AdminStatsSnapshot
	err      error
}

func (f *fakeAdminStats) Stats(context.Context) (service.AdminStatsSnapshot, error) {
	if f.err != nil {
		return service.AdminStatsSnapshot{}, f.err
	}
	return f.snapshot, nil
}

var _ service.AdminStats = (*fakeAdminStats)(nil)

// adminStatsEngine mounts GET /api/v1/admin/stats over the fake service.
func adminStatsEngine(svc service.AdminStats) *gin.Engine {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.GET("/api/v1/admin/stats", handler.NewAdminStatsHandler(svc).Stats)
	return engine
}

func TestAdminStatsReturnsFlatSnapshot(t *testing.T) {
	startedAt := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	svc := &fakeAdminStats{snapshot: service.AdminStatsSnapshot{
		Users: service.AdminStatsUsers{
			Total: 3, Active: 3,
			ByRole: map[string]int64{"admin": 1, "operator": 1, "viewer": 1},
		},
		Source: service.AdminEntityCounts{Customers: 60100, Products: 2050, Orders: 200, OrderItems: 400},
		Target: service.AdminEntityCounts{Customers: 60100, Products: 2050, Orders: 200, OrderItems: 400},
		Jobs: service.AdminJobCounts{
			Total: 12,
			ByStatus: map[string]int64{
				"pending": 0, "running": 0, "completed": 10, "failed": 1, "stopped": 1,
			},
		},
		Schedules: service.AdminScheduleCounts{Total: 2, Enabled: 1},
		Server: service.AdminServerInfo{
			UptimeSeconds: 1234, GoVersion: "go1.24.0", StartedAt: startedAt,
		},
	}}

	rec := adminRequest(t, adminStatsEngine(svc), http.MethodGet, "/api/v1/admin/stats", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	// The payload must be a plain data envelope with exactly the documented
	// blocks, and the snapshot itself must be an object (no meta).
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if len(envelope) != 1 {
		t.Fatalf("expected only the data key, got %#v", envelope)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(envelope["data"], &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	assertKeys(t, "snapshot", data, []string{"users", "source", "target", "jobs", "schedules", "server"})

	var users map[string]json.RawMessage
	if err := json.Unmarshal(data["users"], &users); err != nil {
		t.Fatalf("decode users: %v", err)
	}
	assertKeys(t, "users", users, []string{"total", "active", "by_role"})

	var server service.AdminServerInfo
	if err := json.Unmarshal(data["server"], &server); err != nil {
		t.Fatalf("decode server: %v", err)
	}
	if server.UptimeSeconds != 1234 || server.GoVersion != "go1.24.0" || !server.StartedAt.Equal(startedAt) {
		t.Errorf("unexpected server block: %+v", server)
	}

	var jobs service.AdminJobCounts
	if err := json.Unmarshal(data["jobs"], &jobs); err != nil {
		t.Fatalf("decode jobs: %v", err)
	}
	if jobs.Total != 12 || len(jobs.ByStatus) != 5 || jobs.ByStatus["completed"] != 10 {
		t.Errorf("unexpected jobs block: %+v", jobs)
	}

	var target service.AdminEntityCounts
	if err := json.Unmarshal(data["target"], &target); err != nil {
		t.Fatalf("decode target: %v", err)
	}
	if target.Customers != 60100 || target.OrderItems != 400 {
		t.Errorf("unexpected target block: %+v", target)
	}
}

func TestAdminStatsMapsServiceErrorTo500(t *testing.T) {
	svc := &fakeAdminStats{err: context.DeadlineExceeded}
	rec := adminRequest(t, adminStatsEngine(svc), http.MethodGet, "/api/v1/admin/stats", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if got := decodeError(t, rec).Error.Code; got != handler.CodeInternal {
		t.Errorf("code = %v, want %s", got, handler.CodeInternal)
	}
}

// assertKeys fails when the decoded object does not have exactly want keys.
func assertKeys(t *testing.T, name string, got map[string]json.RawMessage, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s has %d keys, want %d: %#v", name, len(got), len(want), got)
	}
	for _, key := range want {
		if _, ok := got[key]; !ok {
			t.Errorf("%s is missing key %q", name, key)
		}
	}
}
