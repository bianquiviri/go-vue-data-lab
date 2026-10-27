package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// EntityCounts holds the row counts of the four lab schema entities. Both
// lab_source and lab_target expose the same tables (lab_target mirrors
// lab_source), so the same SQL is reused on both handles.
type EntityCounts struct {
	Customers  int64
	Products   int64
	Orders     int64
	OrderItems int64
}

// AdminCounts is the raw counter snapshot behind the admin dashboard. Every
// map is filled from the database as-is; normalizing missing keys to zero is
// the service's responsibility.
type AdminCounts struct {
	UsersTotal       int64
	UsersActive      int64
	UsersByRole      map[string]int64
	Source           EntityCounts
	Target           EntityCounts
	JobsTotal        int64
	JobsByStatus     map[string]int64
	SchedulesTotal   int64
	SchedulesEnabled int64
}

// AdminStatsRepository reads the admin dashboard counters from both database
// handles: the entity counts span lab_source and lab_target, while the users
// and control tables live in lab_target.
type AdminStatsRepository interface {
	Counts(ctx context.Context) (AdminCounts, error)
}

// adminStatsRepository is the two-handle implementation of
// AdminStatsRepository. Each GORM handle keeps its own search_path, so the
// entity counts are unqualified table names evaluated against the right
// schema.
type adminStatsRepository struct {
	source *gorm.DB
	target *gorm.DB
}

// NewAdminStatsRepository builds the admin stats repository over the
// lab_source and lab_target handles.
func NewAdminStatsRepository(source, target *gorm.DB) AdminStatsRepository {
	return &adminStatsRepository{source: source, target: target}
}

// entityCountsSQL counts the four mirrored entities in one round trip, per
// database handle. Every subquery returns 0 on an empty table (never NULL).
const entityCountsSQL = `
SELECT
	(SELECT COUNT(*) FROM customers) AS customers,
	(SELECT COUNT(*) FROM products) AS products,
	(SELECT COUNT(*) FROM orders) AS orders,
	(SELECT COUNT(*) FROM order_items) AS order_items`

// usersCountsSQL returns the total and the enabled account count.
const usersCountsSQL = `
SELECT
	(SELECT COUNT(*) FROM users) AS total,
	(SELECT COUNT(*) FROM users WHERE active) AS active`

// usersByRoleSQL groups users by role so unknown roles are also visible.
const usersByRoleSQL = `SELECT role AS key, COUNT(*) AS count FROM users GROUP BY role`

// jobsCountsSQL returns the total number of job executions.
const jobsCountsSQL = `SELECT COUNT(*) FROM job_executions`

// jobsByStatusSQL groups job executions by status.
const jobsByStatusSQL = `SELECT status AS key, COUNT(*) AS count FROM job_executions GROUP BY status`

// schedulesCountsSQL returns the total and the enabled schedule count.
const schedulesCountsSQL = `
SELECT
	(SELECT COUNT(*) FROM cron_schedules) AS total,
	(SELECT COUNT(*) FROM cron_schedules WHERE enabled) AS enabled`

// countValue is the scan target of the grouped COUNT(*) queries.
type countValue struct {
	Key   string `gorm:"column:key"`
	Count int64  `gorm:"column:count"`
}

// Counts gathers every dashboard counter. Each step is wrapped with the
// failing aggregation so 500s stay attributable in the logs.
func (r *adminStatsRepository) Counts(ctx context.Context) (AdminCounts, error) {
	counts := AdminCounts{
		UsersByRole:  map[string]int64{},
		JobsByStatus: map[string]int64{},
	}

	if err := r.countEntities(ctx, r.source, &counts.Source); err != nil {
		return AdminCounts{}, fmt.Errorf("repository: count lab_source entities: %w", err)
	}
	if err := r.countEntities(ctx, r.target, &counts.Target); err != nil {
		return AdminCounts{}, fmt.Errorf("repository: count lab_target entities: %w", err)
	}

	if err := r.target.WithContext(ctx).Raw(usersCountsSQL).
		Row().Scan(&counts.UsersTotal, &counts.UsersActive); err != nil {
		return AdminCounts{}, fmt.Errorf("repository: count users: %w", err)
	}
	if err := r.groupCounts(ctx, r.target, usersByRoleSQL, counts.UsersByRole); err != nil {
		return AdminCounts{}, fmt.Errorf("repository: count users by role: %w", err)
	}

	if err := r.target.WithContext(ctx).Raw(jobsCountsSQL).Row().Scan(&counts.JobsTotal); err != nil {
		return AdminCounts{}, fmt.Errorf("repository: count job executions: %w", err)
	}
	if err := r.groupCounts(ctx, r.target, jobsByStatusSQL, counts.JobsByStatus); err != nil {
		return AdminCounts{}, fmt.Errorf("repository: count job executions by status: %w", err)
	}

	if err := r.target.WithContext(ctx).Raw(schedulesCountsSQL).
		Row().Scan(&counts.SchedulesTotal, &counts.SchedulesEnabled); err != nil {
		return AdminCounts{}, fmt.Errorf("repository: count cron schedules: %w", err)
	}

	return counts, nil
}

// countEntities fills out with the four entity counts of one handle.
func (r *adminStatsRepository) countEntities(ctx context.Context, db *gorm.DB, out *EntityCounts) error {
	return db.WithContext(ctx).Raw(entityCountsSQL).
		Row().Scan(&out.Customers, &out.Products, &out.Orders, &out.OrderItems)
}

// groupCounts runs a "key, COUNT(*)" query and stores the result in dst.
func (r *adminStatsRepository) groupCounts(ctx context.Context, db *gorm.DB, query string, dst map[string]int64) error {
	rows := make([]countValue, 0)
	if err := db.WithContext(ctx).Raw(query).Scan(&rows).Error; err != nil {
		return fmt.Errorf("scan grouped counts: %w", err)
	}
	for _, row := range rows {
		dst[row.Key] = row.Count
	}
	return nil
}
