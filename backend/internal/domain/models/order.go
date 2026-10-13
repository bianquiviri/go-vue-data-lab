package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID              uuid.UUID       `json:"id" gorm:"type:uuid;primaryKey"`
	OrderNumber     string          `json:"order_number" gorm:"size:50;uniqueIndex;not null"`
	CustomerID      uuid.UUID       `json:"customer_id" gorm:"type:uuid;not null;index"`
	Customer        *Customer       `json:"customer,omitempty" gorm:"foreignKey:CustomerID"`
	Status          string          `json:"status" gorm:"size:30;not null;default:pending"`
	TotalAmount     float64         `json:"total_amount" gorm:"type:decimal(10,2);not null"`
	Currency        string          `json:"currency" gorm:"size:3;not null;default:USD"`
	ShippingAddress json.RawMessage `json:"shipping_address" gorm:"type:jsonb"`
	OrderDate       time.Time       `json:"order_date"`
	OrderItems      []OrderItem     `json:"order_items,omitempty" gorm:"foreignKey:OrderID"`
}
