package etl

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

func TestRound2(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{0, 0},
		{1.0049, 1},
		{19.999, 20},
		{2.676, 2.68},
		{-2.676, -2.68},
		{10.006, 10.01},
		{1234.567, 1234.57},
	}
	for _, c := range cases {
		if got := round2(c.in); got != c.want {
			t.Errorf("round2(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func strPtr(s string) *string { return &s }

func TestTransformCustomer_Valid(t *testing.T) {
	in := models.Customer{
		ID:          uuid.New(),
		FirstName:   "  Ana ",
		LastName:    "Pérez ",
		Email:       "  Ana.Perez@Example.COM ",
		CountryCode: "mx",
		City:        " Guadalajara ",
		State:       strPtr("  Jalisco "),
		CreatedAt:   time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
	}

	out, err := TransformCustomer(in)
	if err != nil {
		t.Fatalf("TransformCustomer: %v", err)
	}
	if out.FirstName != "Ana" {
		t.Errorf("FirstName = %q, want %q", out.FirstName, "Ana")
	}
	if out.LastName != "Pérez" {
		t.Errorf("LastName = %q, want %q", out.LastName, "Pérez")
	}
	if out.Email != "ana.perez@example.com" {
		t.Errorf("Email = %q, want %q", out.Email, "ana.perez@example.com")
	}
	if out.CountryCode != "MX" {
		t.Errorf("CountryCode = %q, want %q", out.CountryCode, "MX")
	}
	if out.City != "Guadalajara" {
		t.Errorf("City = %q, want %q", out.City, "Guadalajara")
	}
	if out.State == nil || *out.State != "Jalisco" {
		t.Errorf("State = %v, want pointer to %q", out.State, "Jalisco")
	}
	if !out.CreatedAt.Equal(in.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", out.CreatedAt, in.CreatedAt)
	}
	if out.ID != in.ID {
		t.Errorf("ID changed: %v", out.ID)
	}
}

func TestTransformCustomer_BlankStateBecomesNil(t *testing.T) {
	in := models.Customer{
		ID: uuid.New(), FirstName: "Ana", LastName: "P",
		Email: "a@b.co", CountryCode: "AR", City: "CBA", State: strPtr("   "),
	}
	out, err := TransformCustomer(in)
	if err != nil {
		t.Fatalf("TransformCustomer: %v", err)
	}
	if out.State != nil {
		t.Errorf("State = %v, want nil for blank input", out.State)
	}
}

func TestTransformCustomer_Invalid(t *testing.T) {
	base := func() models.Customer {
		return models.Customer{
			ID: uuid.New(), FirstName: "Ana", LastName: "P",
			Email: "a@b.co", CountryCode: "AR", City: "CBA",
		}
	}

	cases := []struct {
		name   string
		mutate func(*models.Customer)
	}{
		{"zero id", func(c *models.Customer) { c.ID = uuid.Nil }},
		{"empty first name", func(c *models.Customer) { c.FirstName = "  " }},
		{"empty last name", func(c *models.Customer) { c.LastName = "" }},
		{"empty email", func(c *models.Customer) { c.Email = " " }},
		{"malformed email", func(c *models.Customer) { c.Email = "not-an-email" }},
		{"email with name", func(c *models.Customer) { c.Email = "Ana <a@b.co>" }},
		{"country too short", func(c *models.Customer) { c.CountryCode = "A" }},
		{"country too long", func(c *models.Customer) { c.CountryCode = "ARG" }},
		{"empty city", func(c *models.Customer) { c.City = "" }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := base()
			c.mutate(&in)
			_, err := TransformCustomer(in)
			if err == nil {
				t.Fatalf("expected error for %s", c.name)
			}
			if _, ok := err.(*ValidationError); !ok {
				t.Errorf("expected *ValidationError, got %T", err)
			}
		})
	}
}

func TestTransformProduct_Valid(t *testing.T) {
	in := models.Product{
		ID:       uuid.New(),
		SKU:      " sku-001 ",
		Name:     "  Widget  ",
		Category: " tools ",
		Price:    19.999,
		Stock:    10,
		Status:   " Active ",
	}

	out, err := TransformProduct(in)
	if err != nil {
		t.Fatalf("TransformProduct: %v", err)
	}
	if out.SKU != "SKU-001" {
		t.Errorf("SKU = %q, want %q", out.SKU, "SKU-001")
	}
	if out.Name != "Widget" {
		t.Errorf("Name = %q, want %q", out.Name, "Widget")
	}
	if out.Category != "tools" {
		t.Errorf("Category = %q, want %q", out.Category, "tools")
	}
	if out.Price != 20 {
		t.Errorf("Price = %v, want 20", out.Price)
	}
	if out.Status != "active" {
		t.Errorf("Status = %q, want %q", out.Status, "active")
	}
}

func TestTransformProduct_EmptyStatusDefaultsToActive(t *testing.T) {
	in := models.Product{
		ID: uuid.New(), SKU: "S1", Name: "N", Category: "C", Price: 1, Stock: 1, Status: "",
	}
	out, err := TransformProduct(in)
	if err != nil {
		t.Fatalf("TransformProduct: %v", err)
	}
	if out.Status != "active" {
		t.Errorf("Status = %q, want active", out.Status)
	}
}

func TestTransformProduct_Invalid(t *testing.T) {
	base := func() models.Product {
		return models.Product{
			ID: uuid.New(), SKU: "S1", Name: "N", Category: "C", Price: 1, Stock: 1, Status: "active",
		}
	}
	cases := []struct {
		name   string
		mutate func(*models.Product)
	}{
		{"zero id", func(p *models.Product) { p.ID = uuid.Nil }},
		{"empty sku", func(p *models.Product) { p.SKU = "  " }},
		{"empty name", func(p *models.Product) { p.Name = "" }},
		{"empty category", func(p *models.Product) { p.Category = " " }},
		{"negative price", func(p *models.Product) { p.Price = -0.01 }},
		{"negative stock", func(p *models.Product) { p.Stock = -1 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := base()
			c.mutate(&in)
			_, err := TransformProduct(in)
			if err == nil {
				t.Fatalf("expected error for %s", c.name)
			}
		})
	}
}

func TestTransformOrder_RecomputesTotal(t *testing.T) {
	in := models.Order{
		ID:          uuid.New(),
		OrderNumber: "ORD-1",
		CustomerID:  uuid.New(),
		Status:      " PAID ",
		TotalAmount: 99.99, // wrong on purpose
		Currency:    " usd ",
		OrderDate:   time.Now(),
	}

	out, err := TransformOrder(in, OrderOptions{ItemSubtotalSum: 42.5, HasItems: true})
	if err != nil {
		t.Fatalf("TransformOrder: %v", err)
	}
	if out.TotalAmount != 42.5 {
		t.Errorf("TotalAmount = %v, want 42.5 (recomputed from items)", out.TotalAmount)
	}
	if out.Status != "paid" {
		t.Errorf("Status = %q, want paid", out.Status)
	}
	if out.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", out.Currency)
	}
}

func TestTransformOrder_StrictMismatchSkips(t *testing.T) {
	in := models.Order{
		ID: uuid.New(), OrderNumber: "ORD-1", CustomerID: uuid.New(),
		Status: "paid", TotalAmount: 10, Currency: "USD",
	}
	_, err := TransformOrder(in, OrderOptions{ItemSubtotalSum: 11, HasItems: true, Strict: true})
	if err == nil {
		t.Fatal("expected total mismatch error in strict mode")
	}
	ve, ok := err.(*ValidationError)
	if !ok || ve.Field != "total_amount" {
		t.Errorf("expected ValidationError on total_amount, got %v", err)
	}
}

func TestTransformOrder_StrictMatchPasses(t *testing.T) {
	in := models.Order{
		ID: uuid.New(), OrderNumber: "ORD-1", CustomerID: uuid.New(),
		Status: "paid", TotalAmount: 10.01, Currency: "USD",
	}
	out, err := TransformOrder(in, OrderOptions{ItemSubtotalSum: 10.01, HasItems: true, Strict: true})
	if err != nil {
		t.Fatalf("TransformOrder: %v", err)
	}
	if out.TotalAmount != 10.01 {
		t.Errorf("TotalAmount = %v, want 10.01", out.TotalAmount)
	}
}

func TestTransformOrder_NoItemsBecomesZeroTotal(t *testing.T) {
	in := models.Order{
		ID: uuid.New(), OrderNumber: "ORD-1", CustomerID: uuid.New(),
		Status: "pending", TotalAmount: 5, Currency: "USD",
	}
	out, err := TransformOrder(in, OrderOptions{})
	if err != nil {
		t.Fatalf("TransformOrder: %v", err)
	}
	if out.TotalAmount != 0 {
		t.Errorf("TotalAmount = %v, want 0", out.TotalAmount)
	}
}

func TestTransformOrder_DefaultsAndAddress(t *testing.T) {
	in := models.Order{
		ID: uuid.New(), OrderNumber: " ORD-2 ", CustomerID: uuid.New(),
		ShippingAddress: []byte(`{"city":"CBA"}`),
	}
	out, err := TransformOrder(in, OrderOptions{})
	if err != nil {
		t.Fatalf("TransformOrder: %v", err)
	}
	if out.Status != "pending" {
		t.Errorf("Status = %q, want pending default", out.Status)
	}
	if out.Currency != "USD" {
		t.Errorf("Currency = %q, want USD default", out.Currency)
	}
	if out.OrderNumber != "ORD-2" {
		t.Errorf("OrderNumber = %q, want trimmed", out.OrderNumber)
	}
	if string(out.ShippingAddress) != `{"city":"CBA"}` {
		t.Errorf("ShippingAddress = %s", out.ShippingAddress)
	}

	in.ShippingAddress = []byte("   ")
	out, err = TransformOrder(in, OrderOptions{})
	if err != nil {
		t.Fatalf("TransformOrder blank address: %v", err)
	}
	if out.ShippingAddress != nil {
		t.Errorf("blank ShippingAddress = %v, want nil", out.ShippingAddress)
	}
}

func TestTransformOrder_Invalid(t *testing.T) {
	base := func() models.Order {
		return models.Order{
			ID: uuid.New(), OrderNumber: "ORD-1", CustomerID: uuid.New(),
			Status: "pending", Currency: "USD",
		}
	}
	cases := []struct {
		name   string
		mutate func(*models.Order)
	}{
		{"zero id", func(o *models.Order) { o.ID = uuid.Nil }},
		{"empty order number", func(o *models.Order) { o.OrderNumber = "  " }},
		{"zero customer id", func(o *models.Order) { o.CustomerID = uuid.Nil }},
		{"bad currency", func(o *models.Order) { o.Currency = "US" }},
		{"invalid json address", func(o *models.Order) { o.ShippingAddress = []byte("{nope") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := base()
			c.mutate(&in)
			_, err := TransformOrder(in, OrderOptions{})
			if err == nil {
				t.Fatalf("expected error for %s", c.name)
			}
		})
	}
}

func TestTransformOrderItem_RecomputesSubtotal(t *testing.T) {
	in := models.OrderItem{
		ID:        1,
		OrderID:   uuid.New(),
		ProductID: uuid.New(),
		Quantity:  3,
		UnitPrice: 9.999,
		Subtotal:  1, // wrong on purpose
	}
	out, err := TransformOrderItem(in, false)
	if err != nil {
		t.Fatalf("TransformOrderItem: %v", err)
	}
	if out.Subtotal != 30 {
		t.Errorf("Subtotal = %v, want 30 (3 × 10)", out.Subtotal)
	}
	if out.UnitPrice != 10 {
		t.Errorf("UnitPrice = %v, want rounded 10", out.UnitPrice)
	}
}

func TestTransformOrderItem_StrictMismatch(t *testing.T) {
	in := models.OrderItem{
		ID: 1, OrderID: uuid.New(), ProductID: uuid.New(),
		Quantity: 2, UnitPrice: 5, Subtotal: 9,
	}
	_, err := TransformOrderItem(in, true)
	if err == nil {
		t.Fatal("expected subtotal mismatch error in strict mode")
	}
}

func TestTransformOrderItem_Invalid(t *testing.T) {
	base := func() models.OrderItem {
		return models.OrderItem{
			ID: 1, OrderID: uuid.New(), ProductID: uuid.New(),
			Quantity: 1, UnitPrice: 1, Subtotal: 1,
		}
	}
	cases := []struct {
		name   string
		mutate func(*models.OrderItem)
	}{
		{"zero id", func(i *models.OrderItem) { i.ID = 0 }},
		{"zero order id", func(i *models.OrderItem) { i.OrderID = uuid.Nil }},
		{"zero product id", func(i *models.OrderItem) { i.ProductID = uuid.Nil }},
		{"zero quantity", func(i *models.OrderItem) { i.Quantity = 0 }},
		{"negative quantity", func(i *models.OrderItem) { i.Quantity = -2 }},
		{"negative unit price", func(i *models.OrderItem) { i.UnitPrice = -1 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := base()
			c.mutate(&in)
			_, err := TransformOrderItem(in, false)
			if err == nil {
				t.Fatalf("expected error for %s", c.name)
			}
		})
	}
}

func TestDedupeCustomers(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	rows := []models.Customer{
		{ID: id1, Email: "a@x.io", FirstName: "A"},
		{ID: id2, Email: "b@x.io", FirstName: "B"},
		{ID: id1, Email: "a@x.io", FirstName: "A-dup"},        // duplicate id
		{ID: uuid.New(), Email: "B@x.io", FirstName: "B-dup"}, // duplicate email (case-insensitive after transform)
	}
	out := DedupeCustomers(rows)
	if len(out) != 2 {
		t.Fatalf("DedupeCustomers len = %d, want 2", len(out))
	}
	if out[0].FirstName != "A" || out[1].FirstName != "B" {
		t.Errorf("first occurrences not kept: %+v", out)
	}
}

func TestDedupeProducts(t *testing.T) {
	id1 := uuid.New()
	rows := []models.Product{
		{ID: id1, SKU: "S1"},
		{ID: uuid.New(), SKU: "S2"},
		{ID: uuid.New(), SKU: "s1"}, // dup sku case-insensitive
		{ID: id1, SKU: "S1"},        // dup id
	}
	out := DedupeProducts(rows)
	if len(out) != 2 {
		t.Fatalf("DedupeProducts len = %d, want 2", len(out))
	}
}

func TestDedupeOrders(t *testing.T) {
	id1 := uuid.New()
	rows := []models.Order{
		{ID: id1, OrderNumber: "N1"},
		{ID: uuid.New(), OrderNumber: "N2"},
		{ID: uuid.New(), OrderNumber: "n1"}, // dup number
	}
	out := DedupeOrders(rows)
	if len(out) != 2 {
		t.Fatalf("DedupeOrders len = %d, want 2", len(out))
	}
}

func TestDedupeOrderItems(t *testing.T) {
	rows := []models.OrderItem{
		{ID: 1},
		{ID: 2},
		{ID: 1},
	}
	out := DedupeOrderItems(rows)
	if len(out) != 2 {
		t.Fatalf("DedupeOrderItems len = %d, want 2", len(out))
	}
}

func TestValidationError_Error(t *testing.T) {
	err := invalid("email", "formato inválido")
	if err.Error() != "email: formato inválido" {
		t.Errorf("Error() = %q", err.Error())
	}
}
