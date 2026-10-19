package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// SourceHandler serves read endpoints backed by lab_source. It only depends on
// the service.SourceData port, so tests use fakes.
type SourceHandler struct {
	svc service.SourceData
}

// NewSourceHandler builds a SourceHandler over the given data service.
func NewSourceHandler(svc service.SourceData) *SourceHandler {
	return &SourceHandler{svc: svc}
}

// listCustomersQuery is the bound query string of GET /source/customers.
type listCustomersQuery struct {
	Pagination
	Search  string `form:"search"`
	Country string `form:"country"`
}

// listProductsQuery is the bound query string of GET /source/products.
type listProductsQuery struct {
	Pagination
	Search   string `form:"search"`
	Category string `form:"category"`
}

// listOrdersQuery is the bound query string of GET /source/orders.
type listOrdersQuery struct {
	Pagination
	CustomerID string `form:"customer_id"`
	Status     string `form:"status"`
	DateFrom   string `form:"date_from"`
	DateTo     string `form:"date_to"`
}

// listOrderItemsQuery is the bound query string of GET /source/order-items.
type listOrderItemsQuery struct {
	Pagination
	OrderID   string `form:"order_id"`
	ProductID string `form:"product_id"`
}

// dateLayouts supported for date_from/date_to: RFC3339 or date-only (UTC).
var dateLayouts = []string{time.RFC3339, "2006-01-02"}

// parseDate accepts RFC3339 or date-only values. Date-only values are
// interpreted as UTC; when endOfDay is set the date is extended to the last
// instant of that day so ranges are inclusive.
func parseDate(value string, endOfDay bool) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, value); err == nil {
			t = t.UTC()
			if endOfDay && layout == "2006-01-02" {
				t = t.Add(24*time.Hour - time.Nanosecond)
			}
			return t, true
		}
	}
	return time.Time{}, false
}

// parseOptionalUUID parses an optional UUID query parameter. When the request
// carries an opaque id codec the value must be an opaque token; raw UUIDs are
// rejected. It returns ok=false after writing a 400 envelope when the value is
// present but invalid.
func parseOptionalUUID(c *gin.Context, name string) (*uuid.UUID, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, true
	}
	id, err := openInboundID(c, raw)
	if err != nil {
		respondInvalidID(c, "invalid query parameter", name, invalidIDHint(c), err)
		return nil, false
	}
	return &id, true
}

// parseUUIDParam parses a required UUID path parameter. When the request
// carries an opaque id codec the value must be an opaque token; raw UUIDs are
// rejected. It returns ok=false after writing a 400 envelope when the value is
// not a valid id.
func parseUUIDParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := openInboundID(c, c.Param(name))
	if err != nil {
		respondInvalidID(c, "invalid path parameter", name, invalidIDHint(c), err)
		return uuid.Nil, false
	}
	return id, true
}

// respondServiceError maps a service error to the API error envelope: known
// not-found to 404, anything else to 500 (logged, details hidden).
func respondServiceError(c *gin.Context, entity string, err error) {
	if errors.Is(err, service.ErrNotFound) {
		RespondNotFound(c, entity+" not found")
		return
	}
	RespondInternalError(c, err)
}

// ListCustomers handles GET /api/v1/source/customers.
//
//	@Summary		List source customers
//	@Description	Paginated customers from lab_source, searchable by name/email and filterable by country.
//	@Tags			source
//	@Produce		json
//	@Param			page		query		int		false	"Page number (1-based)"	minimum(1)
//	@Param			per_page	query		int		false	"Items per page (max 100)"	minimum(1)	maximum(100)
//	@Param			search		query		string	false	"Search by first name, last name or email"
//	@Param			country		query		string	false	"ISO 3166-1 alpha-2 country code"
//	@Success		200			{object}	PaginatedResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/source/customers [get]
func (h *SourceHandler) ListCustomers(c *gin.Context) {
	var q listCustomersQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		RespondValidationError(c, err)
		return
	}
	page := q.Pagination.Normalized()

	res, err := h.svc.ListCustomers(c.Request.Context(), service.CustomerListParams{
		Page:    page.Page,
		PerPage: page.PerPage,
		Search:  strings.TrimSpace(q.Search),
		Country: strings.TrimSpace(q.Country),
	})
	if err != nil {
		respondServiceError(c, "customer", err)
		return
	}

	RespondPaginated(c, http.StatusOK, res.Items,
		Pagination{Page: res.Page, PerPage: res.PerPage}, res.Total)
}

// GetCustomer handles GET /api/v1/source/customers/:id.
//
//	@Summary		Get a source customer
//	@Description	Returns a single customer from lab_source by id.
//	@Tags			source
//	@Produce		json
//	@Param			id	path		string	true	"Customer id (opaque token)"
//	@Success		200	{object}	models.Customer
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/source/customers/{id} [get]
func (h *SourceHandler) GetCustomer(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	row, err := h.svc.GetCustomer(c.Request.Context(), id)
	if err != nil {
		respondServiceError(c, "customer", err)
		return
	}
	RespondJSON(c, http.StatusOK, row)
}

// ListProducts handles GET /api/v1/source/products.
//
//	@Summary		List source products
//	@Description	Paginated products from lab_source, searchable by name/SKU and filterable by category.
//	@Tags			source
//	@Produce		json
//	@Param			page		query		int		false	"Page number (1-based)"	minimum(1)
//	@Param			per_page	query		int		false	"Items per page (max 100)"	minimum(1)	maximum(100)
//	@Param			search		query		string	false	"Search by name or SKU"
//	@Param			category	query		string	false	"Filter by category"
//	@Success		200			{object}	PaginatedResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/source/products [get]
func (h *SourceHandler) ListProducts(c *gin.Context) {
	var q listProductsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		RespondValidationError(c, err)
		return
	}
	page := q.Pagination.Normalized()

	res, err := h.svc.ListProducts(c.Request.Context(), service.ProductListParams{
		Page:     page.Page,
		PerPage:  page.PerPage,
		Search:   strings.TrimSpace(q.Search),
		Category: strings.TrimSpace(q.Category),
	})
	if err != nil {
		respondServiceError(c, "product", err)
		return
	}

	RespondPaginated(c, http.StatusOK, res.Items,
		Pagination{Page: res.Page, PerPage: res.PerPage}, res.Total)
}

// GetProduct handles GET /api/v1/source/products/:id.
//
//	@Summary		Get a source product
//	@Description	Returns a single product from lab_source by id.
//	@Tags			source
//	@Produce		json
//	@Param			id	path		string	true	"Product id (opaque token)"
//	@Success		200	{object}	models.Product
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/source/products/{id} [get]
func (h *SourceHandler) GetProduct(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	row, err := h.svc.GetProduct(c.Request.Context(), id)
	if err != nil {
		respondServiceError(c, "product", err)
		return
	}
	RespondJSON(c, http.StatusOK, row)
}

// ListOrders handles GET /api/v1/source/orders.
//
//	@Summary		List source orders
//	@Description	Paginated orders from lab_source, filterable by customer, status and order_date range.
//	@Tags			source
//	@Produce		json
//	@Param			page		query		int		false	"Page number (1-based)"	minimum(1)
//	@Param			per_page	query		int		false	"Items per page (max 100)"	minimum(1)	maximum(100)
//	@Param			customer_id	query		string	false	"Filter by customer id (opaque token)"
//	@Param			status		query		string	false	"Filter by order status"
//	@Param			date_from	query		string	false	"Order date >= (RFC3339 or YYYY-MM-DD)"
//	@Param			date_to		query		string	false	"Order date <= (RFC3339 or YYYY-MM-DD)"
//	@Success		200			{object}	PaginatedResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/source/orders [get]
func (h *SourceHandler) ListOrders(c *gin.Context) {
	var q listOrdersQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		RespondValidationError(c, err)
		return
	}
	page := q.Pagination.Normalized()

	customerID, ok := parseOptionalUUID(c, "customer_id")
	if !ok {
		return
	}

	dateFrom, ok := parseDateRangeParam(c, "date_from", q.DateFrom, false)
	if !ok {
		return
	}
	dateTo, ok := parseDateRangeParam(c, "date_to", q.DateTo, true)
	if !ok {
		return
	}
	if dateFrom != nil && dateTo != nil && dateFrom.After(*dateTo) {
		RespondErrorDetails(c, http.StatusBadRequest, CodeValidation, "invalid date range",
			map[string]any{"fields": map[string]string{"date_from": "must be before or equal to date_to"}})
		return
	}

	res, err := h.svc.ListOrders(c.Request.Context(), service.OrderListParams{
		Page:       page.Page,
		PerPage:    page.PerPage,
		CustomerID: customerID,
		Status:     strings.TrimSpace(q.Status),
		DateFrom:   dateFrom,
		DateTo:     dateTo,
	})
	if err != nil {
		respondServiceError(c, "order", err)
		return
	}

	RespondPaginated(c, http.StatusOK, res.Items,
		Pagination{Page: res.Page, PerPage: res.PerPage}, res.Total)
}

// parseDateRangeParam parses an optional date query parameter, writing a 400
// envelope and returning ok=false when the value is present but invalid.
func parseDateRangeParam(c *gin.Context, name, raw string, endOfDay bool) (*time.Time, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	t, valid := parseDate(raw, endOfDay)
	if !valid {
		RespondErrorDetails(c, http.StatusBadRequest, CodeValidation,
			"invalid query parameter",
			map[string]any{"fields": map[string]string{name: "must be an RFC3339 timestamp or YYYY-MM-DD date"}})
		return nil, false
	}
	return &t, true
}

// GetOrder handles GET /api/v1/source/orders/:id, including its order items.
//
//	@Summary		Get a source order with items
//	@Description	Returns a single order from lab_source by id, including its order items. Every id (and each item id) is returned as an opaque token string.
//	@Tags			source
//	@Produce		json
//	@Param			id	path		string	true	"Order id (opaque token)"
//	@Success		200	{object}	models.Order
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/source/orders/{id} [get]
func (h *SourceHandler) GetOrder(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	row, err := h.svc.GetOrder(c.Request.Context(), id)
	if err != nil {
		respondServiceError(c, "order", err)
		return
	}
	RespondJSON(c, http.StatusOK, row)
}

// ListOrderItems handles GET /api/v1/source/order-items.
//
//	@Summary		List source order items
//	@Description	Paginated order items from lab_source, filterable by order and product. Each item id (int64) is returned as an opaque token string.
//	@Tags			source
//	@Produce		json
//	@Param			page		query		int		false	"Page number (1-based)"	minimum(1)
//	@Param			per_page	query		int		false	"Items per page (max 100)"	minimum(1)	maximum(100)
//	@Param			order_id	query		string	false	"Filter by order id (opaque token)"
//	@Param			product_id	query		string	false	"Filter by product id (opaque token)"
//	@Success		200			{object}	PaginatedResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/source/order-items [get]
func (h *SourceHandler) ListOrderItems(c *gin.Context) {
	var q listOrderItemsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		RespondValidationError(c, err)
		return
	}
	page := q.Pagination.Normalized()

	orderID, ok := parseOptionalUUID(c, "order_id")
	if !ok {
		return
	}
	productID, ok := parseOptionalUUID(c, "product_id")
	if !ok {
		return
	}

	res, err := h.svc.ListOrderItems(c.Request.Context(), service.OrderItemListParams{
		Page:      page.Page,
		PerPage:   page.PerPage,
		OrderID:   orderID,
		ProductID: productID,
	})
	if err != nil {
		respondServiceError(c, "order item", err)
		return
	}

	RespondPaginated(c, http.StatusOK, res.Items,
		Pagination{Page: res.Page, PerPage: res.PerPage}, res.Total)
}
