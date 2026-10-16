package repository

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

func timeAt(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return v
}

// dryRunDB returns a GORM handle that builds SQL without connecting to a
// database, so query construction can be unit tested.
func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		postgres.New(postgres.Config{DSN: "host=localhost port=5432 user=test password=test dbname=test sslmode=disable"}),
		&gorm.Config{
			DryRun:               true,
			DisableAutomaticPing: true,
			Logger:               gormlogger.Discard,
		},
	)
	if err != nil {
		t.Fatalf("open dry-run gorm: %v", err)
	}
	return db
}

func TestIlikePattern(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"  alice ", "%alice%"},
		{"100%", `%100\%%`},
		{"a_b", `%a\_b%`},
		{`back\slash`, `%back\\slash%`},
		{"", "%%"},
	}
	for _, tc := range cases {
		if got := ilikePattern(tc.in); got != tc.want {
			t.Errorf("ilikePattern(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCustomerQuery(t *testing.T) {
	db := dryRunDB(t)

	t.Run("no filter", func(t *testing.T) {
		stmt := customerQuery(db, CustomerFilter{}).Find(&[]models.Customer{}).Statement
		if strings.Contains(stmt.SQL.String(), "WHERE") {
			t.Errorf("expected no WHERE clause, got %s", stmt.SQL.String())
		}
	})

	t.Run("country normalized to upper", func(t *testing.T) {
		stmt := customerQuery(db, CustomerFilter{Country: " us "}).Find(&[]models.Customer{}).Statement
		if !strings.Contains(stmt.SQL.String(), "country_code =") {
			t.Errorf("expected country filter, got %s", stmt.SQL.String())
		}
		if got := fmt.Sprintf("%v", stmt.Vars); !strings.Contains(got, "US") {
			t.Errorf("expected uppercased country var, got %v", stmt.Vars)
		}
	})

	t.Run("search hits name and email", func(t *testing.T) {
		stmt := customerQuery(db, CustomerFilter{Search: "bob"}).Find(&[]models.Customer{}).Statement
		sql := stmt.SQL.String()
		for _, col := range []string{"first_name ILIKE", "last_name ILIKE", "email ILIKE"} {
			if !strings.Contains(sql, col) {
				t.Errorf("expected %q in %s", col, sql)
			}
		}
		if got := fmt.Sprintf("%v", stmt.Vars); !strings.Contains(got, "%bob%") {
			t.Errorf("expected %%bob%% pattern, got %v", stmt.Vars)
		}
	})
}

func TestProductQuery(t *testing.T) {
	db := dryRunDB(t)

	stmt := productQuery(db, ProductFilter{Category: "Electronics", Search: "widget"}).
		Find(&[]models.Product{}).Statement
	sql := stmt.SQL.String()
	if !strings.Contains(sql, "category =") {
		t.Errorf("expected category filter, got %s", sql)
	}
	if !strings.Contains(sql, "name ILIKE") || !strings.Contains(sql, "sku ILIKE") {
		t.Errorf("expected name/sku search, got %s", sql)
	}
}

func TestOrderQuery(t *testing.T) {
	db := dryRunDB(t)
	customerID := uuid.New()

	t.Run("customer and status", func(t *testing.T) {
		stmt := orderQuery(db, OrderFilter{CustomerID: &customerID, Status: " PENDING "}).
			Find(&[]models.Order{}).Statement
		sql := stmt.SQL.String()
		if !strings.Contains(sql, "customer_id =") || !strings.Contains(sql, "status =") {
			t.Errorf("expected customer/status filters, got %s", sql)
		}
		if got := fmt.Sprintf("%v", stmt.Vars); !strings.Contains(got, "pending") {
			t.Errorf("expected lowercased status var, got %v", stmt.Vars)
		}
	})

	t.Run("date range", func(t *testing.T) {
		from := timeAt(t, "2024-01-01T00:00:00Z")
		to := timeAt(t, "2024-02-01T00:00:00Z")
		stmt := orderQuery(db, OrderFilter{DateFrom: &from, DateTo: &to}).
			Find(&[]models.Order{}).Statement
		sql := stmt.SQL.String()
		if !strings.Contains(sql, "order_date >=") || !strings.Contains(sql, "order_date <=") {
			t.Errorf("expected date range filters, got %s", sql)
		}
	})
}

func TestOrderItemQuery(t *testing.T) {
	db := dryRunDB(t)
	orderID := uuid.New()
	productID := uuid.New()

	stmt := orderItemQuery(db, OrderItemFilter{OrderID: &orderID, ProductID: &productID}).
		Find(&[]models.OrderItem{}).Statement
	sql := stmt.SQL.String()
	if !strings.Contains(sql, "order_id =") || !strings.Contains(sql, "product_id =") {
		t.Errorf("expected order/product filters, got %s", sql)
	}
}

func TestApplyPaging(t *testing.T) {
	db := dryRunDB(t)

	t.Run("limit and offset", func(t *testing.T) {
		stmt := applyPaging(db.Model(&models.Customer{}), ListOptions{Limit: 25, Offset: 50}).
			Find(&[]models.Customer{}).Statement
		sql := stmt.SQL.String()
		if !strings.Contains(sql, "LIMIT") || !strings.Contains(sql, "OFFSET") {
			t.Errorf("expected LIMIT/OFFSET, got %s", sql)
		}
	})

	t.Run("zero options are a no-op", func(t *testing.T) {
		stmt := applyPaging(db.Model(&models.Customer{}), ListOptions{}).
			Find(&[]models.Customer{}).Statement
		if strings.Contains(stmt.SQL.String(), "LIMIT") {
			t.Errorf("expected no LIMIT for zero options, got %s", stmt.SQL.String())
		}
	})
}

func TestWrapNotFound(t *testing.T) {
	if err := wrapNotFound("op", nil); err != nil {
		t.Errorf("expected nil for nil error, got %v", err)
	}

	err := wrapNotFound("get customer", gorm.ErrRecordNotFound)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "get customer") {
		t.Errorf("expected operation context, got %v", err)
	}

	boom := errors.New("boom")
	err = wrapNotFound("get customer", boom)
	if !errors.Is(err, boom) {
		t.Errorf("expected wrapped original error, got %v", err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("unexpected ErrNotFound for generic failure: %v", err)
	}
}
