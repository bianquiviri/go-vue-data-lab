package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type CronSchedule struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name      string     `json:"name" gorm:"size:100;not null"`
	CronExpr  string     `json:"cron_expr" gorm:"size:100;not null"`
	Command   string     `json:"command" gorm:"size:255;not null"`
	Enabled   bool       `json:"enabled" gorm:"not null;default:true"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	LastRunAt *time.Time `json:"last_run_at"`
	NextRunAt *time.Time `json:"next_run_at"`
}

type JobExecution struct {
	ID         uuid.UUID       `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ScheduleID *uuid.UUID      `json:"schedule_id" gorm:"type:uuid;index"`
	Schedule   *CronSchedule   `json:"schedule,omitempty" gorm:"foreignKey:ScheduleID"`
	JobName    string          `json:"job_name" gorm:"size:100;not null"`
	Status     string          `json:"status" gorm:"size:30;not null"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt *time.Time      `json:"finished_at"`
	Error      *string         `json:"error"`
	Metrics    json.RawMessage `json:"metrics" gorm:"type:jsonb"`
}
