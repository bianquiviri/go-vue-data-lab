package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// GormJobExecutionRepository persists job executions in lab_target via GORM.
type GormJobExecutionRepository struct {
	db *gorm.DB
}

// NewJobExecutionRepository builds a job-execution repository on db (lab_target).
func NewJobExecutionRepository(db *gorm.DB) *GormJobExecutionRepository {
	return &GormJobExecutionRepository{db: db}
}

// jobExecutionListQuery builds the newest-first, paged listing query.
func jobExecutionListQuery(db *gorm.DB, opts ListOptions) *gorm.DB {
	return applyPaging(db.Model(&models.JobExecution{}).Order("started_at DESC, id DESC"), opts)
}

// Create inserts a new execution row.
func (r *GormJobExecutionRepository) Create(ctx context.Context, exec *models.JobExecution) error {
	if err := r.db.WithContext(ctx).Create(exec).Error; err != nil {
		return fmt.Errorf("repository: create job execution: %w", err)
	}
	return nil
}

// Update saves the terminal state of an execution row.
func (r *GormJobExecutionRepository) Update(ctx context.Context, exec *models.JobExecution) error {
	if err := r.db.WithContext(ctx).Save(exec).Error; err != nil {
		return fmt.Errorf("repository: update job execution: %w", err)
	}
	return nil
}

// List returns a page of executions (newest first) plus the total count.
func (r *GormJobExecutionRepository) List(ctx context.Context, limit, offset int) ([]models.JobExecution, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.JobExecution{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("repository: count job executions: %w", err)
	}

	rows := make([]models.JobExecution, 0)
	q := jobExecutionListQuery(r.db.WithContext(ctx), ListOptions{Limit: limit, Offset: offset})
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("repository: list job executions: %w", err)
	}
	return rows, total, nil
}
