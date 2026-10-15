package repository_test

import (
	"context"
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

// openTargetDB opens lab_target for integration tests, skipping cleanly when
// PostgreSQL is not configured/reachable or the schema is not migrated.
func openTargetDB(t *testing.T) *gorm.DB {
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
	db, err := database.OpenTarget(cfg)
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
		t.Skip("lab_target schema not migrated (customers/order_items missing)")
	}
	return db
}

// seedAnalyticsFixture inserts a small customer/product/order/items graph for
// the analytics queries and removes it afterwards.
func seedAnalyticsFixture(t *testing.T, db *gorm.DB) (customerID, productID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	customerID = uuid.New()
	productID = uuid.New()
	orderID := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	tx := db.WithContext(ctx)
	if err := tx.Create(&models.Customer{
		ID:          customerID,
		FirstName:   "Analytics",
		LastName:    "Fixture",
		Email:       fmt.Sprintf("analytics-%s@example.test", suffix),
		CountryCode: "US",
		City:        "Testville",
		CreatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	if err := tx.Create(&models.Product{
		ID:        productID,
		SKU:       fmt.Sprintf("AN-%s", suffix),
		Name:      fmt.Sprintf("Analytics Widget %s", suffix),
		Category:  "AnalyticsCategory",
		Price:     25.50,
		Stock:     100,
		Status:    "active",
		CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	if err := tx.Create(&models.Order{
		ID:          orderID,
		OrderNumber: fmt.Sprintf("AN-ORD-%s", suffix),
		CustomerID:  customerID,
		Status:      "paid",
		TotalAmount: 51.00,
		Currency:    "USD",
		OrderDate:   now,
	}).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}
	if err := tx.Create(&models.OrderItem{
		ID:        now.UnixNano(),
		OrderID:   orderID,
		ProductID: productID,
		Quantity:  2,
		UnitPrice: 25.50,
		Subtotal:  51.00,
	}).Error; err != nil {
		t.Fatalf("seed order item: %v", err)
	}

	t.Cleanup(func() {
		d := db.WithContext(context.Background())
		_ = d.Where("order_id = ?", orderID).Delete(&models.OrderItem{}).Error
		_ = d.Where("id = ?", orderID).Delete(&models.Order{}).Error
		_ = d.Where("id = ?", productID).Delete(&models.Product{}).Error
		_ = d.Where("id = ?", customerID).Delete(&models.Customer{}).Error
	})
	return customerID, productID
}

func TestAnalyticsRepositoryIntegration(t *testing.T) {
	db := openTargetDB(t)
	ctx := context.Background()
	repo := repository.NewAnalyticsRepository(db)

	customerID, productID := seedAnalyticsFixture(t, db)

	t.Run("summary", func(t *testing.T) {
		s, err := repo.Summary(ctx)
		if err != nil {
			t.Fatalf("summary: %v", err)
		}
		if s.TotalCustomers < 1 || s.TotalProducts < 1 || s.TotalOrders < 1 {
			t.Errorf("expected non-empty counts, got %+v", s)
		}
		if s.TotalRevenue <= 0 || s.AverageOrderValue <= 0 {
			t.Errorf("expected positive revenue, got %+v", s)
		}
		if len(s.OrdersByStatus) == 0 {
			t.Errorf("expected orders grouped by status, got %+v", s.OrdersByStatus)
		}
		found := false
		for _, b := range s.OrdersByStatus {
			if b.Status == "paid" && b.Count >= 1 {
				found = true
			}
		}
		if !found {
			t.Errorf("expected a paid bucket, got %+v", s.OrdersByStatus)
		}
	})

	t.Run("top products", func(t *testing.T) {
		rows, err := repo.TopProducts(ctx, 100)
		if err != nil {
			t.Fatalf("top products: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("expected at least one top product")
		}
		var found bool
		for _, r := range rows {
			if r.ID == productID {
				found = true
				if r.TotalQuantity < 2 || r.TotalRevenue <= 0 {
					t.Errorf("expected quantity >= 2 and positive revenue, got %+v", r)
				}
				if r.SKU == "" || r.Name == "" || r.Category == "" {
					t.Errorf("expected product fields populated, got %+v", r)
				}
			}
		}
		if !found {
			t.Errorf("seeded product %s not in top products", productID)
		}
	})

	t.Run("top customers", func(t *testing.T) {
		rows, err := repo.TopCustomers(ctx, 100)
		if err != nil {
			t.Fatalf("top customers: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("expected at least one top customer")
		}
		var found bool
		for _, r := range rows {
			if r.ID == customerID {
				found = true
				if r.TotalOrders < 1 || r.TotalSpent <= 0 {
					t.Errorf("expected at least one order and positive spend, got %+v", r)
				}
				if r.Email == "" || r.FirstName == "" {
					t.Errorf("expected customer fields populated, got %+v", r)
				}
			}
		}
		if !found {
			t.Errorf("seeded customer %s not in top customers", customerID)
		}
	})

	t.Run("limit respected", func(t *testing.T) {
		rows, err := repo.TopProducts(ctx, 1)
		if err != nil {
			t.Fatalf("top products limit: %v", err)
		}
		if len(rows) > 1 {
			t.Errorf("expected at most 1 row, got %d", len(rows))
		}
	})
}

func TestTargetRepositoriesIntegration(t *testing.T) {
	db := openTargetDB(t)
	ctx := context.Background()
	repos := repository.NewTargetRepositories(db)

	// Sanity: the target repositories serve the same reads as the source ones.
	if _, _, err := repos.Customers.List(ctx, repository.ListOptions{Limit: 1}, repository.CustomerFilter{}); err != nil {
		t.Errorf("target customers list: %v", err)
	}
	if _, _, err := repos.Products.List(ctx, repository.ListOptions{Limit: 1}, repository.ProductFilter{}); err != nil {
		t.Errorf("target products list: %v", err)
	}
	if _, _, err := repos.Orders.List(ctx, repository.ListOptions{Limit: 1}, repository.OrderFilter{}); err != nil {
		t.Errorf("target orders list: %v", err)
	}
}

func TestAnalyticsRepositoryErrorPaths(t *testing.T) {
	// A closed handle exercises the error wrapping without a live database.
	db := openTargetDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	_ = sqlDB.Close()

	repo := repository.NewAnalyticsRepository(db)
	ctx := context.Background()

	if _, err := repo.Summary(ctx); err == nil {
		t.Error("expected summary error on closed db")
	}
	if _, err := repo.TopProducts(ctx, 10); err == nil {
		t.Error("expected top products error on closed db")
	}
	if _, err := repo.TopCustomers(ctx, 10); err == nil {
		t.Error("expected top customers error on closed db")
	}
}
