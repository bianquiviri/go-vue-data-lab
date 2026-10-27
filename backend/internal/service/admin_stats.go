package service

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/jobs"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
)

// AdminStats is the read port of the admin dashboard counters. It is
// satisfied by *AdminStatsService and by fakes in tests.
type AdminStats interface {
	// Stats returns the dashboard snapshot.
	Stats(ctx context.Context) (AdminStatsSnapshot, error)
}

// AdminStatsUsers is the user block of the dashboard snapshot.
type AdminStatsUsers struct {
	Total  int64            `json:"total"`
	Active int64            `json:"active"`
	ByRole map[string]int64 `json:"by_role"`
}

// AdminEntityCounts is the entity block of one lab schema.
type AdminEntityCounts struct {
	Customers  int64 `json:"customers"`
	Products   int64 `json:"products"`
	Orders     int64 `json:"orders"`
	OrderItems int64 `json:"order_items"`
}

// AdminJobCounts is the job-execution block of the dashboard snapshot.
type AdminJobCounts struct {
	Total    int64            `json:"total"`
	ByStatus map[string]int64 `json:"by_status"`
}

// AdminScheduleCounts is the cron schedule block of the dashboard snapshot.
type AdminScheduleCounts struct {
	Total   int64 `json:"total"`
	Enabled int64 `json:"enabled"`
}

// AdminServerInfo is the process block of the dashboard snapshot. StartedAt is
// the process start instant injected from the entrypoint, never derived per
// request.
type AdminServerInfo struct {
	UptimeSeconds int64     `json:"uptime_seconds"`
	GoVersion     string    `json:"go_version"`
	StartedAt     time.Time `json:"started_at"`
}

// AdminStatsSnapshot is the flat payload of GET /admin/stats:
//
//	{"users":{...},"source":{...},"target":{...},"jobs":{...},"schedules":{...},"server":{...}}
type AdminStatsSnapshot struct {
	Users     AdminStatsUsers     `json:"users"`
	Source    AdminEntityCounts   `json:"source"`
	Target    AdminEntityCounts   `json:"target"`
	Jobs      AdminJobCounts      `json:"jobs"`
	Schedules AdminScheduleCounts `json:"schedules"`
	Server    AdminServerInfo     `json:"server"`
}

// AdminStatsOption customizes the admin stats service (used by tests to pin the
// clock).
type AdminStatsOption func(*AdminStatsService)

// WithStatsClock overrides the clock used to derive uptime_seconds.
func WithStatsClock(now func() time.Time) AdminStatsOption {
	return func(s *AdminStatsService) {
		if now != nil {
			s.now = now
		}
	}
}

// AdminStatsService assembles the dashboard snapshot from the counts
// repository and the process metadata.
type AdminStatsService struct {
	counts    repository.AdminStatsRepository
	startedAt time.Time
	now       func() time.Time
}

// NewAdminStatsService builds the admin stats service. startedAt is the
// process start instant captured by the entrypoint.
func NewAdminStatsService(counts repository.AdminStatsRepository, startedAt time.Time, opts ...AdminStatsOption) *AdminStatsService {
	s := &AdminStatsService{
		counts:    counts,
		startedAt: startedAt.UTC(),
		now:       time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Stats returns the dashboard snapshot. Missing role/status buckets are
// normalized to 0 so the payload shape is stable (never null, never absent).
func (s *AdminStatsService) Stats(ctx context.Context) (AdminStatsSnapshot, error) {
	raw, err := s.counts.Counts(ctx)
	if err != nil {
		return AdminStatsSnapshot{}, fmt.Errorf("service: admin stats: %w", err)
	}

	return AdminStatsSnapshot{
		Users: AdminStatsUsers{
			Total:  raw.UsersTotal,
			Active: raw.UsersActive,
			ByRole: roleCounts(raw.UsersByRole),
		},
		Source: AdminEntityCounts{
			Customers:  raw.Source.Customers,
			Products:   raw.Source.Products,
			Orders:     raw.Source.Orders,
			OrderItems: raw.Source.OrderItems,
		},
		Target: AdminEntityCounts{
			Customers:  raw.Target.Customers,
			Products:   raw.Target.Products,
			Orders:     raw.Target.Orders,
			OrderItems: raw.Target.OrderItems,
		},
		Jobs: AdminJobCounts{
			Total:    raw.JobsTotal,
			ByStatus: jobStatusCounts(raw.JobsByStatus),
		},
		Schedules: AdminScheduleCounts{
			Total:   raw.SchedulesTotal,
			Enabled: raw.SchedulesEnabled,
		},
		Server: AdminServerInfo{
			UptimeSeconds: uptimeSeconds(s.now(), s.startedAt),
			GoVersion:     runtime.Version(),
			StartedAt:     s.startedAt,
		},
	}, nil
}

// roleCounts normalizes the per-role counters to the three RBAC roles known to
// the API, defaulting missing buckets to zero.
func roleCounts(raw map[string]int64) map[string]int64 {
	return map[string]int64{
		string(models.RoleAdmin):    raw[string(models.RoleAdmin)],
		string(models.RoleOperator): raw[string(models.RoleOperator)],
		string(models.RoleViewer):   raw[string(models.RoleViewer)],
	}
}

// jobStatusCounts normalizes the per-status counters to the job statuses known
// by the jobs package, defaulting missing buckets to zero.
func jobStatusCounts(raw map[string]int64) map[string]int64 {
	statuses := []jobs.Status{
		jobs.StatusPending,
		jobs.StatusRunning,
		jobs.StatusCompleted,
		jobs.StatusFailed,
		jobs.StatusStopped,
	}
	out := make(map[string]int64, len(statuses))
	for _, status := range statuses {
		out[string(status)] = raw[string(status)]
	}
	return out
}

// uptimeSeconds derives the process uptime from the clock and the start
// instant. A clock that ran backwards yields 0, never a negative uptime.
func uptimeSeconds(now, startedAt time.Time) int64 {
	elapsed := now.Sub(startedAt)
	if elapsed < 0 {
		return 0
	}
	return int64(elapsed / time.Second)
}
