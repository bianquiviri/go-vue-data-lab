package repository_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/config"
	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/infrastructure/database"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
)

// openSourceDB opens lab_source for integration tests, skipping cleanly when
// PostgreSQL is not configured/reachable or the schema is not migrated.
func openSourceDB(t *testing.T) *gorm.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database integration test in short mode")
	}
	if os.Getenv("DB_HOST") == "" && os.Getenv("TEST_DATABASE_URL") == "" {
		t.Skip("set DB_* or TEST_DATABASE_URL to run repository integration tests")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("could not load config: %v", err)
	}
	db, err := database.OpenSource(cfg)
	if err != nil {
		t.Skipf("PostgreSQL not reachable: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Skipf("no *sql.DB handle: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Skipf("PostgreSQL not reachable: %v", err)
	}
	if !db.Migrator().HasTable("customers") || !db.Migrator().HasTable("order_items") {
		t.Skip("lab_source schema not migrated (customers/order_items missing)")
	}
	return db
}

func TestGormRepositoriesIntegration(t *testing.T) {
	db := openSourceDB(t)
	ctx := context.Background()

	suffix := uuid.NewString()
	customer := models.Customer{
		ID:          uuid.New(),
		FirstName:   "Integration",
		LastName:    "Tester",
		Email:       fmt.Sprintf("it-%s@example.test", suffix),
		CountryCode: "US",
		City:        "Testville",
		CreatedAt:   time.Now().UTC(),
	}
	product := models.Product{
		ID:        uuid.New(),
		SKU:       fmt.Sprintf("IT-%s", suffix[:8]),
		Name:      fmt.Sprintf("Integration Widget %s", suffix[:8]),
		Category:  "IntegrationCategory",
		Price:     9.99,
		Stock:     10,
		Status:    "active",
		CreatedAt: time.Now().UTC(),
	}
	orderDate := time.Now().UTC().Truncate(time.Second)
	order := models.Order{
		ID:          uuid.New(),
		OrderNumber: fmt.Sprintf("IT-%s", suffix),
		CustomerID:  customer.ID,
		Status:      "pending",
		TotalAmount: 19.98,
		Currency:    "USD",
		OrderDate:   orderDate,
	}
	item := models.OrderItem{
		OrderID:   order.ID,
		ProductID: product.ID,
		Quantity:  2,
		UnitPrice: 9.99,
		Subtotal:  19.98,
	}

	for _, row := range []any{&customer, &product, &order, &item} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM order_items WHERE order_id = ?", order.ID)
		db.Exec("DELETE FROM orders WHERE id = ?", order.ID)
		db.Exec("DELETE FROM products WHERE id = ?", product.ID)
		db.Exec("DELETE FROM customers WHERE id = ?", customer.ID)
	})

	repos := repository.NewSourceRepositories(db)

	t.Run("customers list/search/get", func(t *testing.T) {
		rows, total, err := repos.Customers.List(ctx, repository.ListOptions{Limit: 5}, repository.CustomerFilter{
			Search:  suffix[:8],
			Country: "us",
		})
		if err != nil {
			t.Fatalf("list customers: %v", err)
		}
		if total < 1 || len(rows) < 1 {
			t.Fatalf("expected seeded customer in results (total=%d, rows=%d)", total, len(rows))
		}
		if rows[0].ID != customer.ID {
			t.Errorf("expected seeded customer first, got %s", rows[0].ID)
		}

		got, err := repos.Customers.GetByID(ctx, customer.ID)
		if err != nil {
			t.Fatalf("get customer: %v", err)
		}
		if got.Email != customer.Email {
			t.Errorf("email = %q, want %q", got.Email, customer.Email)
		}
	})

	t.Run("customer not found", func(t *testing.T) {
		_, err := repos.Customers.GetByID(ctx, uuid.New())
		if !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("products filter/get", func(t *testing.T) {
		rows, total, err := repos.Products.List(ctx, repository.ListOptions{Limit: 10}, repository.ProductFilter{
			Category: "IntegrationCategory",
			Search:   suffix[:8],
		})
		if err != nil {
			t.Fatalf("list products: %v", err)
		}
		if total != 1 || len(rows) != 1 {
			t.Fatalf("expected exactly the seeded product (total=%d, rows=%d)", total, len(rows))
		}

		got, err := repos.Products.GetByID(ctx, product.ID)
		if err != nil {
			t.Fatalf("get product: %v", err)
		}
		if got.SKU != product.SKU {
			t.Errorf("sku = %q, want %q", got.SKU, product.SKU)
		}
	})

	t.Run("orders filter/get with items", func(t *testing.T) {
		from := orderDate.Add(-time.Hour)
		to := orderDate.Add(time.Hour)
		rows, total, err := repos.Orders.List(ctx, repository.ListOptions{Limit: 10}, repository.OrderFilter{
			CustomerID: &customer.ID,
			Status:     "PENDING",
			DateFrom:   &from,
			DateTo:     &to,
		})
		if err != nil {
			t.Fatalf("list orders: %v", err)
		}
		if total != 1 || len(rows) != 1 {
			t.Fatalf("expected exactly the seeded order (total=%d, rows=%d)", total, len(rows))
		}
		if len(rows[0].OrderItems) != 0 {
			t.Errorf("list must not preload order items, got %d", len(rows[0].OrderItems))
		}

		got, err := repos.Orders.GetByID(ctx, order.ID)
		if err != nil {
			t.Fatalf("get order: %v", err)
		}
		if len(got.OrderItems) != 1 {
			t.Fatalf("expected 1 order item preloaded, got %d", len(got.OrderItems))
		}
		if got.OrderItems[0].ProductID != product.ID {
			t.Errorf("item product = %s, want %s", got.OrderItems[0].ProductID, product.ID)
		}
	})

	t.Run("order items filter/get", func(t *testing.T) {
		rows, total, err := repos.OrderItems.List(ctx, repository.ListOptions{Limit: 10}, repository.OrderItemFilter{
			OrderID:   &order.ID,
			ProductID: &product.ID,
		})
		if err != nil {
			t.Fatalf("list order items: %v", err)
		}
		if total != 1 || len(rows) != 1 {
			t.Fatalf("expected exactly the seeded item (total=%d, rows=%d)", total, len(rows))
		}

		got, err := repos.OrderItems.GetByID(ctx, rows[0].ID)
		if err != nil {
			t.Fatalf("get order item: %v", err)
		}
		if got.OrderID != order.ID {
			t.Errorf("order id = %s, want %s", got.OrderID, order.ID)
		}

		_, err = repos.OrderItems.GetByID(ctx, 9_999_999_999)
		if !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("paging", func(t *testing.T) {
		page1, _, err := repos.Products.List(ctx, repository.ListOptions{Limit: 1, Offset: 0}, repository.ProductFilter{Category: "IntegrationCategory"})
		if err != nil {
			t.Fatalf("page 1: %v", err)
		}
		page2, _, err := repos.Products.List(ctx, repository.ListOptions{Limit: 1, Offset: 1}, repository.ProductFilter{Category: "IntegrationCategory"})
		if err != nil {
			t.Fatalf("page 2: %v", err)
		}
		if len(page1) != 1 {
			t.Fatalf("expected 1 row on page 1, got %d", len(page1))
		}
		if len(page2) != 0 {
			t.Errorf("expected empty page 2 (only one seeded row), got %d", len(page2))
		}
	})
}
