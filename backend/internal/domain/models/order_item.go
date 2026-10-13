package models

import "github.com/google/uuid"

type OrderItem struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderID   uuid.UUID `json:"order_id" gorm:"type:uuid;not null;index"`
	Order     *Order    `json:"order,omitempty" gorm:"foreignKey:OrderID"`
	ProductID uuid.UUID `json:"product_id" gorm:"type:uuid;not null;index"`
	Product   *Product  `json:"product,omitempty" gorm:"foreignKey:ProductID"`
	Quantity  int       `json:"quantity" gorm:"not null;check:quantity>0"`
	UnitPrice float64   `json:"unit_price" gorm:"type:decimal(10,2);not null;check:unit_price>=0"`
	Subtotal  float64   `json:"subtotal" gorm:"type:decimal(10,2);not null;check:subtotal>=0"`
}
