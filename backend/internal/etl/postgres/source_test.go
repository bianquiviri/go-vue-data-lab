package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/etl"
)

// seedSourceFixture inserts a small, FK-valid fixture into lab_source.
func seedSourceFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	cleanupFixtures(t, db)

	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.WithContext(ctx).Exec(query, args...).Error; err != nil {
			t.Fatalf("seed fixture: %v", err)
		}
	}

	exec(`INSERT INTO customers (id, first_name, last_name, email, country_code, city, state, created_at)
	      VALUES (?, 'Etl', 'Source', 'etl.source.fixture@example.com', 'AR', 'Cba', NULL, NOW())
	      ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email`, srcCustomerID)

	exec(`INSERT INTO products (id, sku, name, category, price, stock, status, created_at)
	      VALUES (?, 'ETL-SRC-1', 'Fixture', 'fixtures', 10.00, 5, 'active', NOW())
	      ON CONFLICT (id) DO UPDATE SET sku = EXCLUDED.sku`, srcProductID)

	exec(`INSERT INTO orders (id, order_number, customer_id, status, total_amount, currency, shipping_address, order_date)
	      VALUES (?, 'ETL-SRC-ORD-1', ?, 'pending', 999.00, 'USD', '{"city":"Cba"}', NOW())
	      ON CONFLICT (id) DO UPDATE SET total_amount = EXCLUDED.total_amount`, srcOrderID, srcCustomerID)

	exec(`INSERT INTO order_items (id, order_id, product_id, quantity, unit_price, subtotal)
	      VALUES (?, ?, ?, 1, 5.00, 5.00), (?, ?, ?, 2, 5.00, 10.00)
	      ON CONFLICT (id) DO UPDATE SET subtotal = EXCLUDED.subtotal`,
		srcItemID1, srcOrderID, srcProductID, srcItemID2, srcOrderID, srcProductID)
}

// uuidLess compares UUIDs bytewise, matching PostgreSQL's uuid ordering.
func uuidLess(a, b uuid.UUID) bool { return bytes.Compare(a[:], b[:]) < 0 }

func TestSourceIntegration(t *testing.T) {
	src, _ := openTestDBs(t)
	seedSourceFixture(t, src)

	s, err := NewSource(src)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// --- Count --------------------------------------------------------------
	total, err := s.Count(ctx, etl.EntityCustomers)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if total < 1 {
		t.Errorf("Count(customers) = %d, want >= 1 (fixture)", total)
	}
	if _, err := s.Count(ctx, "nope"); err == nil {
		t.Error("Count with unknown entity must fail")
	}

	// --- Customers keyset pagination ---------------------------------------
	page1, err := s.FetchCustomers(ctx, uuid.Nil, 50)
	if err != nil {
		t.Fatalf("FetchCustomers: %v", err)
	}
	if !containsCustomer(page1, srcCustomerID) {
		t.Fatal("fixture customer missing from first page (low prefix should sort first)")
	}
	if len(page1) > 0 {
		last := page1[len(page1)-1].ID
		page2, err := s.FetchCustomers(ctx, last, 50)
		if err != nil {
			t.Fatalf("FetchCustomers page2: %v", err)
		}
		for _, c := range page2 {
			if !uuidLess(last, c.ID) {
				t.Errorf("page2 row %s not strictly after %s", c.ID, last)
			}
			if containsCustomer(page1, c.ID) {
				t.Errorf("row %s duplicated across pages", c.ID)
			}
		}
	}

	// --- Orders -------------------------------------------------------------
	orders, err := s.FetchOrders(ctx, uuid.Nil, 50)
	if err != nil {
		t.Fatalf("FetchOrders: %v", err)
	}
	found := false
	for _, o := range orders {
		if o.ID == srcOrderID {
			found = true
			if o.TotalAmount != 999 {
				t.Errorf("order total = %v, want 999 (raw source value)", o.TotalAmount)
			}
			var addr map[string]any
			if err := json.Unmarshal(o.ShippingAddress, &addr); err != nil {
				t.Errorf("shipping_address not valid JSON: %v", err)
			} else if addr["city"] != "Cba" {
				t.Errorf("shipping_address city = %v, want Cba", addr["city"])
			}
		}
	}
	if !found {
		t.Error("fixture order missing from first page")
	}

	// --- Order items + sums -------------------------------------------------
	items, err := s.FetchOrderItems(ctx, srcItemID1-1, 100)
	if err != nil {
		t.Fatalf("FetchOrderItems: %v", err)
	}
	var got1, got2 bool
	for _, it := range items {
		switch it.ID {
		case srcItemID1:
			got1 = true
		case srcItemID2:
			got2 = true
		}
	}
	if !got1 || !got2 {
		t.Errorf("fixture items found: %v %v", got1, got2)
	}

	sums, err := s.OrderItemSums(ctx, []uuid.UUID{srcOrderID})
	if err != nil {
		t.Fatalf("OrderItemSums: %v", err)
	}
	if sums[srcOrderID] != 15 {
		t.Errorf("sum = %v, want 15", sums[srcOrderID])
	}

	sums, err = s.OrderItemSums(ctx, nil)
	if err != nil || len(sums) != 0 {
		t.Errorf("empty OrderItemSums = %v, %v; want empty map, nil", sums, err)
	}

	// --- Limit edge cases ---------------------------------------------------
	if rows, err := s.FetchProducts(ctx, uuid.Nil, 0); err != nil || len(rows) != 0 {
		t.Errorf("FetchProducts(limit=0) = %v, %v; want empty, nil", rows, err)
	}
}

func containsCustomer(rows []models.Customer, id uuid.UUID) bool {
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}
