package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// GormCronScheduleRepository persists cron schedules in lab_target via GORM.
type GormCronScheduleRepository struct {
	db *gorm.DB
}

// NewCronScheduleRepository builds a cron-schedule repository on db (lab_target).
func NewCronScheduleRepository(db *gorm.DB) *GormCronScheduleRepository {
	return &GormCronScheduleRepository{db: db}
}

// cronScheduleListQuery builds the stable listing query.
func cronScheduleListQuery(db *gorm.DB) *gorm.DB {
	return db.Model(&models.CronSchedule{}).Order("name ASC, id ASC")
}

// List returns every schedule ordered by name.
func (r *GormCronScheduleRepository) List(ctx context.Context) ([]models.CronSchedule, error) {
	rows := make([]models.CronSchedule, 0)
	if err := cronScheduleListQuery(r.db.WithContext(ctx)).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: list cron schedules: %w", err)
	}
	return rows, nil
}

// Get returns the schedule with the given id or ErrNotFound.
func (r *GormCronScheduleRepository) Get(ctx context.Context, id uuid.UUID) (*models.CronSchedule, error) {
	var row models.CronSchedule
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, wrapNotFound(fmt.Sprintf("get cron schedule %s", id), err)
	}
	return &row, nil
}

// Create inserts a new schedule.
func (r *GormCronScheduleRepository) Create(ctx context.Context, s *models.CronSchedule) error {
	if err := r.db.WithContext(ctx).Create(s).Error; err != nil {
		return fmt.Errorf("repository: create cron schedule: %w", err)
	}
	return nil
}

// Update saves an existing schedule.
func (r *GormCronScheduleRepository) Update(ctx context.Context, s *models.CronSchedule) error {
	if err := r.db.WithContext(ctx).Save(s).Error; err != nil {
		return fmt.Errorf("repository: update cron schedule: %w", err)
	}
	return nil
}

// Delete removes a schedule, returning ErrNotFound when it does not exist.
func (r *GormCronScheduleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&models.CronSchedule{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("repository: delete cron schedule: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("repository: delete cron schedule %s: %w", id, ErrNotFound)
	}
	return nil
}
