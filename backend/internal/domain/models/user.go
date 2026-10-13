package models

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleAdmin    UserRole = "admin"
	RoleOperator UserRole = "operator"
	RoleViewer   UserRole = "viewer"
)

type User struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Email        string    `json:"email" gorm:"size:255;uniqueIndex;not null"`
	PasswordHash string    `json:"-" gorm:"size:255;not null"`
	Role         UserRole  `json:"role" gorm:"size:50;not null;default:viewer"`
	Active       bool      `json:"active" gorm:"not null;default:true"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
