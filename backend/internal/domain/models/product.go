package models

import (
	"time"

	"github.com/google/uuid"
)

type Product struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	SKU       string    `json:"sku" gorm:"size:50;uniqueIndex;not null"`
	Name      string    `json:"name" gorm:"size:255;not null"`
	Category  string    `json:"category" gorm:"size:100;not null"`
	Price     float64   `json:"price" gorm:"type:decimal(10,2);not null"`
	Stock     int       `json:"stock" gorm:"not null"`
	Status    string    `json:"status" gorm:"size:20;not null;default:active"`
	CreatedAt time.Time `json:"created_at"`
}
