package models

import "github.com/google/uuid"

// OrderStatusCount is one bucket of the orders-by-status breakdown returned by
// the analytics summary.
type OrderStatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// AnalyticsSummary is the aggregate snapshot of lab_target: entity counts,
// revenue totals and the orders-by-status breakdown.
type AnalyticsSummary struct {
	TotalCustomers    int64              `json:"total_customers"`
	TotalProducts     int64              `json:"total_products"`
	TotalOrders       int64              `json:"total_orders"`
	TotalRevenue      float64            `json:"total_revenue"`
	AverageOrderValue float64            `json:"average_order_value"`
	OrdersByStatus    []OrderStatusCount `json:"orders_by_status" gorm:"-"`
}

// TopProduct is a product ranked by units sold and revenue.
type TopProduct struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	SKU           string    `json:"sku"`
	Category      string    `json:"category"`
	TotalQuantity int64     `json:"total_quantity"`
	TotalRevenue  float64   `json:"total_revenue"`
}

// TopCustomer is a customer ranked by amount spent and order count.
type TopCustomer struct {
	ID          uuid.UUID `json:"id"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	Email       string    `json:"email"`
	TotalOrders int64     `json:"total_orders"`
	TotalSpent  float64   `json:"total_spent"`
}
