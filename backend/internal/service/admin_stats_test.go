package service_test

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/josebianco/go-vue-data-lab/internal/repository"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// fakeAdminStatsRepo is a controllable repository.AdminStatsRepository double.
type fakeAdminStatsRepo struct {
	counts repository.AdminCounts
	err    error
}

func (f fakeAdminStatsRepo) Counts(context.Context) (repository.AdminCounts, error) {
	if f.err != nil {
		return repository.AdminCounts{}, f.err
	}
	return f.counts, nil
}

var _ repository.AdminStatsRepository = fakeAdminStatsRepo{}

func TestAdminStatsMapsCountersAndDefaults(t *testing.T) {
	startedAt := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	repo := fakeAdminStatsRepo{counts: repository.AdminCounts{
		UsersTotal:  3,
		UsersActive: 2,
		// "auditor" is not an API role: it must not leak into by_role.
		UsersByRole: map[string]int64{"admin": 1, "operator": 1, "viewer": 0, "auditor": 1},
		Source: repository.EntityCounts{
			Customers: 60100, Products: 2050, Orders: 200, OrderItems: 400,
		},
		Target: repository.EntityCounts{
			Customers: 60099, Products: 2050, Orders: 199, OrderItems: 399,
		},
		JobsTotal: 12,
		// "exploded" is not a job status: it must not leak into by_status.
		JobsByStatus:   map[string]int64{"completed": 10, "failed": 1, "stopped": 1, "exploded": 7},
		SchedulesTotal: 2, SchedulesEnabled: 1,
	}}
	svc := service.NewAdminStatsService(repo, startedAt, service.WithStatsClock(func() time.Time {
		return startedAt.Add(90 * time.Second)
	}))

	snapshot, err := svc.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	if snapshot.Users.Total != 3 || snapshot.Users.Active != 2 {
		t.Errorf("users = %+v", snapshot.Users)
	}
	wantRoles := map[string]int64{"admin": 1, "operator": 1, "viewer": 0}
	assertCountMap(t, "by_role", snapshot.Users.ByRole, wantRoles)

	if snapshot.Source.Customers != 60100 || snapshot.Source.OrderItems != 400 {
		t.Errorf("source = %+v", snapshot.Source)
	}
	if snapshot.Target.Customers != 60099 || snapshot.Target.OrderItems != 399 {
		t.Errorf("target = %+v", snapshot.Target)
	}

	if snapshot.Jobs.Total != 12 {
		t.Errorf("jobs total = %d", snapshot.Jobs.Total)
	}
	wantStatuses := map[string]int64{
		"pending": 0, "running": 0, "completed": 10, "failed": 1, "stopped": 1,
	}
	assertCountMap(t, "by_status", snapshot.Jobs.ByStatus, wantStatuses)

	if snapshot.Schedules.Total != 2 || snapshot.Schedules.Enabled != 1 {
		t.Errorf("schedules = %+v", snapshot.Schedules)
	}

	if snapshot.Server.UptimeSeconds != 90 {
		t.Errorf("uptime = %d, want 90", snapshot.Server.UptimeSeconds)
	}
	if snapshot.Server.GoVersion != runtime.Version() {
		t.Errorf("go_version = %q, want %q", snapshot.Server.GoVersion, runtime.Version())
	}
	if !snapshot.Server.StartedAt.Equal(startedAt) {
		t.Errorf("started_at = %v, want %v", snapshot.Server.StartedAt, startedAt)
	}
}

func TestAdminStatsEmptyCountsAreZeroNotNil(t *testing.T) {
	startedAt := time.Now().Add(-time.Minute)
	svc := service.NewAdminStatsService(fakeAdminStatsRepo{}, startedAt)

	snapshot, err := svc.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if snapshot.Users.ByRole == nil || len(snapshot.Users.ByRole) != 3 {
		t.Errorf("by_role must be a 3-key map, got %#v", snapshot.Users.ByRole)
	}
	if snapshot.Jobs.ByStatus == nil || len(snapshot.Jobs.ByStatus) != 5 {
		t.Errorf("by_status must be a 5-key map, got %#v", snapshot.Jobs.ByStatus)
	}
	for key, value := range snapshot.Jobs.ByStatus {
		if value != 0 {
			t.Errorf("by_status[%q] = %d, want 0", key, value)
		}
	}
	if snapshot.Server.GoVersion == "" {
		t.Error("go_version must never be empty")
	}
}

func TestAdminStatsUptimeNeverNegative(t *testing.T) {
	startedAt := time.Now()
	svc := service.NewAdminStatsService(fakeAdminStatsRepo{}, startedAt, service.WithStatsClock(func() time.Time {
		return startedAt.Add(-time.Hour)
	}))

	snapshot, err := svc.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if snapshot.Server.UptimeSeconds != 0 {
		t.Errorf("uptime = %d, want 0", snapshot.Server.UptimeSeconds)
	}
}

func TestAdminStatsWrapsRepositoryError(t *testing.T) {
	svc := service.NewAdminStatsService(fakeAdminStatsRepo{err: errors.New("db down")}, time.Now())
	if _, err := svc.Stats(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
}

// assertCountMap compares a counter map exactly.
func assertCountMap(t *testing.T, name string, got, want map[string]int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s has %d keys, want %d: %#v", name, len(got), len(want), got)
	}
	for key, wantValue := range want {
		if got[key] != wantValue {
			t.Errorf("%s[%q] = %d, want %d", name, key, got[key], wantValue)
		}
	}
}
