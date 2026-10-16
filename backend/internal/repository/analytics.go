package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// AnalyticsRepository exposes the aggregate read queries over lab_target.
type AnalyticsRepository interface {
	// Summary returns entity counts, revenue totals and the orders-by-status
	// breakdown.
	Summary(ctx context.Context) (models.AnalyticsSummary, error)
	// TopProducts ranks products by revenue (then units sold), limited to
	// limit rows.
	TopProducts(ctx context.Context, limit int) ([]models.TopProduct, error)
	// TopCustomers ranks customers by amount spent (then order count),
	// limited to limit rows.
	TopCustomers(ctx context.Context, limit int) ([]models.TopCustomer, error)
}

// GormAnalyticsRepository runs the analytics SQL against lab_target via GORM
// raw queries.
type GormAnalyticsRepository struct {
	db *gorm.DB
}

// NewAnalyticsRepository builds an AnalyticsRepository on db (lab_target).
func NewAnalyticsRepository(db *gorm.DB) AnalyticsRepository {
	return &GormAnalyticsRepository{db: db}
}

// summarySQL computes the scalar aggregates in a single round trip.
const summarySQL = `
SELECT
	(SELECT COUNT(*) FROM customers) AS total_customers,
	(SELECT COUNT(*) FROM products) AS total_products,
	(SELECT COUNT(*) FROM orders) AS total_orders,
	COALESCE((SELECT SUM(total_amount) FROM orders), 0) AS total_revenue,
	COALESCE((SELECT AVG(total_amount) FROM orders), 0) AS average_order_value`

// ordersByStatusSQL groups orders by status, most frequent first.
const ordersByStatusSQL = `
SELECT status, COUNT(*) AS count
FROM orders
GROUP BY status
ORDER BY count DESC, status ASC`

// topProductsSQL ranks products by revenue and units sold.
const topProductsSQL = `
SELECT
	p.id,
	p.name,
	p.sku,
	p.category,
	COALESCE(SUM(oi.quantity), 0) AS total_quantity,
	COALESCE(SUM(oi.subtotal), 0) AS total_revenue
FROM products p
JOIN order_items oi ON oi.product_id = p.id
GROUP BY p.id, p.name, p.sku, p.category
ORDER BY total_revenue DESC, total_quantity DESC, p.id ASC
LIMIT ?`

// topCustomersSQL ranks customers by amount spent and order count.
const topCustomersSQL = `
SELECT
	c.id,
	c.first_name,
	c.last_name,
	c.email,
	COUNT(o.id) AS total_orders,
	COALESCE(SUM(o.total_amount), 0) AS total_spent
FROM customers c
JOIN orders o ON o.customer_id = c.id
GROUP BY c.id, c.first_name, c.last_name, c.email
ORDER BY total_spent DESC, total_orders DESC, c.id ASC
LIMIT ?`

// Summary returns the lab_target aggregate snapshot.
func (r *GormAnalyticsRepository) Summary(ctx context.Context) (models.AnalyticsSummary, error) {
	var summary models.AnalyticsSummary
	if err := r.db.WithContext(ctx).Raw(summarySQL).Scan(&summary).Error; err != nil {
		return models.AnalyticsSummary{}, fmt.Errorf("repository: analytics summary: %w", err)
	}

	statuses := make([]models.OrderStatusCount, 0)
	if err := r.db.WithContext(ctx).Raw(ordersByStatusSQL).Scan(&statuses).Error; err != nil {
		return models.AnalyticsSummary{}, fmt.Errorf("repository: analytics orders by status: %w", err)
	}
	summary.OrdersByStatus = statuses
	return summary, nil
}

// TopProducts returns the top-selling products, limited to limit rows.
func (r *GormAnalyticsRepository) TopProducts(ctx context.Context, limit int) ([]models.TopProduct, error) {
	rows := make([]models.TopProduct, 0)
	if err := r.db.WithContext(ctx).Raw(topProductsSQL, limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: analytics top products: %w", err)
	}
	return rows, nil
}

// TopCustomers returns the highest-spending customers, limited to limit rows.
func (r *GormAnalyticsRepository) TopCustomers(ctx context.Context, limit int) ([]models.TopCustomer, error) {
	rows := make([]models.TopCustomer, 0)
	if err := r.db.WithContext(ctx).Raw(topCustomersSQL, limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: analytics top customers: %w", err)
	}
	return rows, nil
}
