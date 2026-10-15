package repository

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

func TestNewTargetRepositories(t *testing.T) {
	db := dryRunDB(t)

	repos := NewTargetRepositories(db)

	if repos == nil {
		t.Fatal("expected non-nil target repositories")
	}
	if repos.Customers == nil || repos.Products == nil || repos.Orders == nil || repos.OrderItems == nil {
		t.Errorf("expected every repository to be set, got %+v", repos)
	}
}

func TestSummarySQL(t *testing.T) {
	fragments := []string{
		"COUNT(*) FROM customers",
		"COUNT(*) FROM products",
		"COUNT(*) FROM orders",
		"SUM(total_amount)",
		"AVG(total_amount)",
		"total_customers",
		"total_products",
		"total_orders",
		"total_revenue",
		"average_order_value",
	}
	for _, f := range fragments {
		if !strings.Contains(summarySQL, f) {
			t.Errorf("summarySQL missing %q", f)
		}
	}
}

func TestOrdersByStatusSQL(t *testing.T) {
	fragments := []string{
		"SELECT status, COUNT(*) AS count",
		"GROUP BY status",
		"ORDER BY count DESC",
	}
	for _, f := range fragments {
		if !strings.Contains(ordersByStatusSQL, f) {
			t.Errorf("ordersByStatusSQL missing %q", f)
		}
	}
}

func TestTopProductsSQL(t *testing.T) {
	fragments := []string{
		"FROM products p",
		"JOIN order_items oi ON oi.product_id = p.id",
		"SUM(oi.quantity)",
		"SUM(oi.subtotal)",
		"total_quantity",
		"total_revenue",
		"GROUP BY p.id",
		"ORDER BY total_revenue DESC, total_quantity DESC",
		"LIMIT ?",
	}
	for _, f := range fragments {
		if !strings.Contains(topProductsSQL, f) {
			t.Errorf("topProductsSQL missing %q", f)
		}
	}
}

func TestTopCustomersSQL(t *testing.T) {
	fragments := []string{
		"FROM customers c",
		"JOIN orders o ON o.customer_id = c.id",
		"COUNT(o.id)",
		"SUM(o.total_amount)",
		"total_orders",
		"total_spent",
		"GROUP BY c.id",
		"ORDER BY total_spent DESC, total_orders DESC",
		"LIMIT ?",
	}
	for _, f := range fragments {
		if !strings.Contains(topCustomersSQL, f) {
			t.Errorf("topCustomersSQL missing %q", f)
		}
	}
}

func TestAnalyticsSQLDryRun(t *testing.T) {
	db := dryRunDB(t)

	t.Run("top products binds limit", func(t *testing.T) {
		got := db.ToSQL(func(tx *gorm.DB) *gorm.DB { return tx.Raw(topProductsSQL, 25).Scan(&[]models.TopProduct{}) })
		if !strings.Contains(got, "LIMIT") || !strings.Contains(got, "25") {
			t.Errorf("expected LIMIT 25 in %s", got)
		}
	})

	t.Run("top customers binds limit", func(t *testing.T) {
		got := db.ToSQL(func(tx *gorm.DB) *gorm.DB { return tx.Raw(topCustomersSQL, 7).Scan(&[]models.TopCustomer{}) })
		if !strings.Contains(got, "LIMIT") || !strings.Contains(got, "7") {
			t.Errorf("expected LIMIT 7 in %s", got)
		}
	})
}
