package etl

import (
	"encoding/json"
	"fmt"
	"math"
	"net/mail"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// ValidationError describes why a row was skipped. It is returned by every
// Transform* function and recorded in Result.Skipped.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface.
func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }

// invalid builds a *ValidationError.
func invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

// round2 rounds a money value to two decimal places.
func round2(v float64) float64 { return math.Round(v*100) / 100 }

// TransformCustomer normalizes and validates a customer row.
//
// Normalization: trims all strings, lowercases the email, uppercases the
// country code and turns a blank state into nil.
func TransformCustomer(in models.Customer) (models.Customer, error) {
	out := in
	out.FirstName = strings.TrimSpace(in.FirstName)
	out.LastName = strings.TrimSpace(in.LastName)
	out.Email = strings.ToLower(strings.TrimSpace(in.Email))
	out.CountryCode = strings.ToUpper(strings.TrimSpace(in.CountryCode))
	out.City = strings.TrimSpace(in.City)
	out.State = nil
	if in.State != nil {
		if s := strings.TrimSpace(*in.State); s != "" {
			out.State = &s
		}
	}

	if out.ID == uuid.Nil {
		return out, invalid("id", "requerido")
	}
	if out.FirstName == "" {
		return out, invalid("first_name", "requerido")
	}
	if out.LastName == "" {
		return out, invalid("last_name", "requerido")
	}
	if out.Email == "" {
		return out, invalid("email", "requerido")
	}
	if addr, err := mail.ParseAddress(out.Email); err != nil || addr.Address != out.Email {
		return out, invalid("email", "formato inválido")
	}
	if len(out.CountryCode) != 2 {
		return out, invalid("country_code", "debe tener 2 caracteres")
	}
	if out.City == "" {
		return out, invalid("city", "requerido")
	}
	return out, nil
}

// TransformProduct normalizes and validates a product row.
//
// Normalization: trims strings, uppercases the SKU, lowercases the status
// (defaulting to "active") and rounds the price.
func TransformProduct(in models.Product) (models.Product, error) {
	out := in
	out.SKU = strings.ToUpper(strings.TrimSpace(in.SKU))
	out.Name = strings.TrimSpace(in.Name)
	out.Category = strings.TrimSpace(in.Category)
	out.Status = strings.ToLower(strings.TrimSpace(in.Status))
	if out.Status == "" {
		out.Status = "active"
	}
	out.Price = round2(in.Price)

	if out.ID == uuid.Nil {
		return out, invalid("id", "requerido")
	}
	if out.SKU == "" {
		return out, invalid("sku", "requerido")
	}
	if out.Name == "" {
		return out, invalid("name", "requerido")
	}
	if out.Category == "" {
		return out, invalid("category", "requerido")
	}
	if out.Price < 0 {
		return out, invalid("price", "no puede ser negativo")
	}
	if out.Stock < 0 {
		return out, invalid("stock", "no puede ser negativo")
	}
	return out, nil
}

// OrderOptions carries the cross-row context needed to transform an order: the
// sum of its items' subtotals and whether it has any items at all.
type OrderOptions struct {
	ItemSubtotalSum float64
	HasItems        bool
	Strict          bool
}

// TransformOrder normalizes and validates an order row.
//
// The order total is always recomputed from the sum of its order_items'
// subtotals. When Strict is true a mismatch between the stored and computed
// total is an error instead of a silent correction.
func TransformOrder(in models.Order, opts OrderOptions) (models.Order, error) {
	out := in
	out.OrderNumber = strings.TrimSpace(in.OrderNumber)
	out.Status = strings.ToLower(strings.TrimSpace(in.Status))
	if out.Status == "" {
		out.Status = "pending"
	}
	out.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if out.Currency == "" {
		out.Currency = "USD"
	}

	raw := strings.TrimSpace(string(in.ShippingAddress))
	if raw == "" {
		out.ShippingAddress = nil
	} else {
		if !json.Valid([]byte(raw)) {
			return out, invalid("shipping_address", "JSON inválido")
		}
		out.ShippingAddress = json.RawMessage(raw)
	}

	if out.ID == uuid.Nil {
		return out, invalid("id", "requerido")
	}
	if out.OrderNumber == "" {
		return out, invalid("order_number", "requerido")
	}
	if out.CustomerID == uuid.Nil {
		return out, invalid("customer_id", "requerido")
	}
	if len(out.Currency) != 3 {
		return out, invalid("currency", "debe tener 3 caracteres")
	}

	computed := 0.0
	if opts.HasItems {
		computed = round2(opts.ItemSubtotalSum)
	}
	if opts.Strict && round2(out.TotalAmount) != computed {
		return out, invalid("total_amount", fmt.Sprintf(
			"no coincide con la suma de items (%v != %v)", round2(out.TotalAmount), computed))
	}
	out.TotalAmount = computed
	if out.TotalAmount < 0 {
		return out, invalid("total_amount", "no puede ser negativo")
	}
	return out, nil
}

// TransformOrderItem normalizes and validates an order item row.
//
// The subtotal is always recomputed as round2(quantity × unit_price). When
// strict is true a mismatch with the stored subtotal is an error.
func TransformOrderItem(in models.OrderItem, strict bool) (models.OrderItem, error) {
	out := in
	out.UnitPrice = round2(in.UnitPrice)

	if out.ID <= 0 {
		return out, invalid("id", "requerido")
	}
	if out.OrderID == uuid.Nil {
		return out, invalid("order_id", "requerido")
	}
	if out.ProductID == uuid.Nil {
		return out, invalid("product_id", "requerido")
	}
	if out.Quantity <= 0 {
		return out, invalid("quantity", "debe ser > 0")
	}
	if out.UnitPrice < 0 {
		return out, invalid("unit_price", "no puede ser negativo")
	}

	computed := round2(float64(out.Quantity) * out.UnitPrice)
	if strict && round2(in.Subtotal) != computed {
		return out, invalid("subtotal", "no coincide con quantity × unit_price")
	}
	out.Subtotal = computed
	if out.Subtotal < 0 {
		return out, invalid("subtotal", "no puede ser negativo")
	}
	return out, nil
}

// dedupeBy keeps the first occurrence of a row (per its natural keys) and
// returns the discarded duplicates in order.
func dedupeBy[T any](rows []T, keysOf func(T) []string) (kept, dropped []T) {
	seen := make(map[string]struct{}, len(rows))
	kept = make([]T, 0, len(rows))
	for _, row := range rows {
		keys := keysOf(row)
		dup := false
		for _, k := range keys {
			if _, ok := seen[k]; ok {
				dup = true
				break
			}
		}
		if dup {
			dropped = append(dropped, row)
			continue
		}
		for _, k := range keys {
			seen[k] = struct{}{}
		}
		kept = append(kept, row)
	}
	return kept, dropped
}

func customerKeys(c models.Customer) []string {
	return []string{"id:" + c.ID.String(), "email:" + strings.ToLower(c.Email)}
}

func productKeys(p models.Product) []string {
	return []string{"id:" + p.ID.String(), "sku:" + strings.ToUpper(p.SKU)}
}

func orderKeys(o models.Order) []string {
	return []string{"id:" + o.ID.String(), "num:" + strings.ToUpper(o.OrderNumber)}
}

func orderItemKeys(i models.OrderItem) []string {
	return []string{"id:" + strconv.FormatInt(i.ID, 10)}
}

// DedupeCustomers keeps the first row per id and per email.
func DedupeCustomers(rows []models.Customer) []models.Customer {
	kept, _ := dedupeBy(rows, customerKeys)
	return kept
}

// DedupeProducts keeps the first row per id and per SKU.
func DedupeProducts(rows []models.Product) []models.Product {
	kept, _ := dedupeBy(rows, productKeys)
	return kept
}

// DedupeOrders keeps the first row per id and per order number.
func DedupeOrders(rows []models.Order) []models.Order {
	kept, _ := dedupeBy(rows, orderKeys)
	return kept
}

// DedupeOrderItems keeps the first row per id.
func DedupeOrderItems(rows []models.OrderItem) []models.OrderItem {
	kept, _ := dedupeBy(rows, orderItemKeys)
	return kept
}
