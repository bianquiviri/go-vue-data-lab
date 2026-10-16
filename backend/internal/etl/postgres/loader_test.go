package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

func TestLoaderIntegration(t *testing.T) {
	_, tgt := openTestDBs(t)
	cleanupFixtures(t, tgt)

	loader, err := NewLoader(tgt)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC()

	customers := []models.Customer{
		{ID: tgtCustomerID, FirstName: "Test", LastName: "Target", Email: "etl.tgt1@example.com", CountryCode: "AR", City: "Cba", CreatedAt: now},
		{ID: tgtCustomer2ID, FirstName: "Second", LastName: "Target", Email: "etl.tgt2@example.com", CountryCode: "AR", City: "Cba", CreatedAt: now},
	}

	// --- Insert vs update reporting ----------------------------------------
	res, err := loader.UpsertCustomers(ctx, customers)
	if err != nil {
		t.Fatalf("insert customers: %v", err)
	}
	if res.Inserted != 2 || res.Updated != 0 {
		t.Errorf("first upsert = %+v, want inserted 2 updated 0", res)
	}

	customers[0].FirstName = "Updated"
	res, err = loader.UpsertCustomers(ctx, customers[:1])
	if err != nil {
		t.Fatalf("update customer: %v", err)
	}
	if res.Updated != 1 || res.Inserted != 0 {
		t.Errorf("second upsert = %+v, want updated 1 inserted 0", res)
	}
	if got := countWhere(t, tgt, "customers", "id = ? AND first_name = ?", tgtCustomerID, "Updated"); got != 1 {
		t.Errorf("customer not updated in db (matches=%d)", got)
	}

	// --- Internal sub-chunking still reports correctly ---------------------
	loader.maxRowsPerStmt = 1
	res, err = loader.UpsertCustomers(ctx, customers)
	if err != nil {
		t.Fatalf("chunked upsert: %v", err)
	}
	if res.Inserted != 0 || res.Updated != 2 {
		t.Errorf("chunked upsert = %+v, want updated 2", res)
	}
	loader.maxRowsPerStmt = DefaultMaxRowsPerStmt

	// --- Products: atomic rollback on a bad row ----------------------------
	p := models.Product{ID: tgtProductID, SKU: "ETL-TGT-1", Name: "Widget", Category: "tools", Price: 10.5, Stock: 3, Status: "active", CreatedAt: now}
	res, err = loader.UpsertProducts(ctx, []models.Product{p})
	if err != nil {
		t.Fatalf("insert product: %v", err)
	}
	if res.Inserted != 1 {
		t.Errorf("product insert = %+v", res)
	}

	good := models.Product{ID: tgtProductGoodID, SKU: "ETL-TGT-GOOD", Name: "Good", Category: "tools", Price: 1, Stock: 1, Status: "active", CreatedAt: now}
	bad := models.Product{ID: tgtProductBadID, SKU: "ETL-TGT-BAD", Name: "Bad", Category: "tools", Price: -1, Stock: 1, Status: "active", CreatedAt: now}
	if _, err := loader.UpsertProducts(ctx, []models.Product{good, bad}); err == nil {
		t.Fatal("expected CHECK violation for negative price")
	}
	if got := countWhere(t, tgt, "products", "id = ?", tgtProductGoodID); got != 0 {
		t.Errorf("good product persisted despite rollback (count=%d)", got)
	}

	// --- Orders require an existing customer (FK) --------------------------
	order := models.Order{
		ID: tgtOrderID, OrderNumber: "ETL-TGT-ORD-1", CustomerID: tgtCustomerID,
		Status: "pending", TotalAmount: 15, Currency: "USD",
		ShippingAddress: json.RawMessage(`{"city":"Cba"}`), OrderDate: now,
	}
	res, err = loader.UpsertOrders(ctx, []models.Order{order})
	if err != nil {
		t.Fatalf("insert order: %v", err)
	}
	if res.Inserted != 1 {
		t.Errorf("order insert = %+v", res)
	}
	res, err = loader.UpsertOrders(ctx, []models.Order{order})
	if err != nil {
		t.Fatalf("re-upsert order: %v", err)
	}
	if res.Updated != 1 {
		t.Errorf("order re-upsert = %+v, want updated 1", res)
	}

	// --- Order items idempotency -------------------------------------------
	items := []models.OrderItem{
		{ID: tgtOrderItemID1, OrderID: tgtOrderID, ProductID: tgtProductID, Quantity: 1, UnitPrice: 5, Subtotal: 5},
		{ID: tgtOrderItemID2, OrderID: tgtOrderID, ProductID: tgtProductID, Quantity: 2, UnitPrice: 5, Subtotal: 10},
	}
	res, err = loader.UpsertOrderItems(ctx, items)
	if err != nil {
		t.Fatalf("insert items: %v", err)
	}
	if res.Inserted != 2 {
		t.Errorf("items insert = %+v", res)
	}
	res, err = loader.UpsertOrderItems(ctx, items)
	if err != nil {
		t.Fatalf("re-upsert items: %v", err)
	}
	if res.Updated != 2 {
		t.Errorf("items re-upsert = %+v, want updated 2", res)
	}
	if got := countWhere(t, tgt, "order_items", "order_id = ?", tgtOrderID); got != 2 {
		t.Errorf("order_items count = %d, want 2 (no duplicates)", got)
	}

	// --- FK violation rolls back the batch ---------------------------------
	orphan := models.OrderItem{ID: tgtOrderItemID1 + 100, OrderID: tgtOrderID, ProductID: tgtProductBadID, Quantity: 1, UnitPrice: 1, Subtotal: 1}
	if _, err := loader.UpsertOrderItems(ctx, []models.OrderItem{orphan}); err == nil {
		t.Fatal("expected FK violation for missing product")
	}

	// --- Cancelled context aborts ------------------------------------------
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := loader.UpsertCustomers(cancelled, customers); err == nil {
		t.Fatal("expected error on cancelled context")
	} else if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want wrapping context.Canceled", err)
	}
}
