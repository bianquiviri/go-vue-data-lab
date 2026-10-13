package models

import (
	"time"

	"github.com/google/uuid"
)

type Customer struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	FirstName   string    `json:"first_name" gorm:"size:100;not null"`
	LastName    string    `json:"last_name" gorm:"size:100;not null"`
	Email       string    `json:"email" gorm:"size:255;uniqueIndex;not null"`
	CountryCode string    `json:"country_code" gorm:"size:2;not null"`
	City        string    `json:"city" gorm:"size:100;not null"`
	State       *string   `json:"state" gorm:"size:100"`
	CreatedAt   time.Time `json:"created_at"`
}
