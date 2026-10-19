package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// TargetHandler serves read endpoints backed by lab_target. It only depends
// on the service.TargetData port, so tests use fakes.
type TargetHandler struct {
	svc service.TargetData
}

// NewTargetHandler builds a TargetHandler over the given target data service.
func NewTargetHandler(svc service.TargetData) *TargetHandler {
	return &TargetHandler{svc: svc}
}

// LimitMeta is the metadata block of a top-N analytics response.
type LimitMeta struct {
	Limit int `json:"limit"`
}

// AnalyticsListResponse is the stable envelope of the top-N analytics
// endpoints:
//
//	{"data": [...], "meta": {"limit":10}}
type AnalyticsListResponse struct {
	Data any       `json:"data"`
	Meta LimitMeta `json:"meta"`
}

// topQuery is the bound query string of the top-N analytics endpoints.
type topQuery struct {
	Limit int `form:"limit" binding:"omitempty,min=1,max=100"`
}

// ListCustomers handles GET /api/v1/target/customers.
//
//	@Summary		List target customers
//	@Description	Paginated customers from lab_target, searchable by name/email and filterable by country.
//	@Tags			target
//	@Produce		json
//	@Param			page		query		int		false	"Page number (1-based)"	minimum(1)
//	@Param			per_page	query		int		false	"Items per page (max 100)"	minimum(1)	maximum(100)
//	@Param			search		query		string	false	"Search by first name, last name or email"
//	@Param			country		query		string	false	"ISO 3166-1 alpha-2 country code"
//	@Success		200			{object}	PaginatedResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/target/customers [get]
func (h *TargetHandler) ListCustomers(c *gin.Context) {
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

// GetCustomer handles GET /api/v1/target/customers/:id.
//
//	@Summary		Get a target customer
//	@Description	Returns a single customer from lab_target by id.
//	@Tags			target
//	@Produce		json
//	@Param			id	path		string	true	"Customer id (opaque token)"
//	@Success		200	{object}	models.Customer
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/target/customers/{id} [get]
func (h *TargetHandler) GetCustomer(c *gin.Context) {
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

// ListProducts handles GET /api/v1/target/products.
//
//	@Summary		List target products
//	@Description	Paginated products from lab_target, searchable by name/SKU and filterable by category.
//	@Tags			target
//	@Produce		json
//	@Param			page		query		int		false	"Page number (1-based)"	minimum(1)
//	@Param			per_page	query		int		false	"Items per page (max 100)"	minimum(1)	maximum(100)
//	@Param			search		query		string	false	"Search by name or SKU"
//	@Param			category	query		string	false	"Filter by category"
//	@Success		200			{object}	PaginatedResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/target/products [get]
func (h *TargetHandler) ListProducts(c *gin.Context) {
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

// GetProduct handles GET /api/v1/target/products/:id.
//
//	@Summary		Get a target product
//	@Description	Returns a single product from lab_target by id.
//	@Tags			target
//	@Produce		json
//	@Param			id	path		string	true	"Product id (opaque token)"
//	@Success		200	{object}	models.Product
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/target/products/{id} [get]
func (h *TargetHandler) GetProduct(c *gin.Context) {
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

// ListOrders handles GET /api/v1/target/orders.
//
//	@Summary		List target orders
//	@Description	Paginated orders from lab_target, filterable by customer, status and order_date range.
//	@Tags			target
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
//	@Router			/target/orders [get]
func (h *TargetHandler) ListOrders(c *gin.Context) {
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

// GetOrder handles GET /api/v1/target/orders/:id, including its order items.
//
//	@Summary		Get a target order with items
//	@Description	Returns a single order from lab_target by id, including its order items. Every id (and each item id) is returned as an opaque token string.
//	@Tags			target
//	@Produce		json
//	@Param			id	path		string	true	"Order id (opaque token)"
//	@Success		200	{object}	models.Order
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/target/orders/{id} [get]
func (h *TargetHandler) GetOrder(c *gin.Context) {
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

// AnalyticsSummary handles GET /api/v1/target/analytics/summary.
//
//	@Summary		Analytics summary
//	@Description	Aggregate snapshot of lab_target: entity counts, revenue totals, average order value and orders grouped by status.
//	@Tags			analytics
//	@Produce		json
//	@Success		200	{object}	models.AnalyticsSummary
//	@Failure		500	{object}	ErrorResponse
//	@Router			/target/analytics/summary [get]
func (h *TargetHandler) AnalyticsSummary(c *gin.Context) {
	summary, err := h.svc.Summary(c.Request.Context())
	if err != nil {
		RespondInternalError(c, err)
		return
	}
	RespondJSON(c, http.StatusOK, summary)
}

// TopProducts handles GET /api/v1/target/analytics/top-products.
//
//	@Summary		Top products by revenue
//	@Description	Ranks products by total revenue and units sold in lab_target. Supports a limit query param (default 10, max 100).
//	@Tags			analytics
//	@Produce		json
//	@Param			limit	query		int		false	"Maximum number of products (default 10, max 100)"	minimum(1)	maximum(100)
//	@Success		200		{object}	handler.AnalyticsListResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/target/analytics/top-products [get]
func (h *TargetHandler) TopProducts(c *gin.Context) {
	var q topQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		RespondValidationError(c, err)
		return
	}
	limit := service.NormalizeAnalyticsLimit(q.Limit)

	rows, err := h.svc.TopProducts(c.Request.Context(), limit)
	if err != nil {
		RespondInternalError(c, err)
		return
	}
	RespondJSON(c, http.StatusOK, AnalyticsListResponse{
		Data: rows,
		Meta: LimitMeta{Limit: limit},
	})
}

// TopCustomers handles GET /api/v1/target/analytics/top-customers.
//
//	@Summary		Top customers by spend
//	@Description	Ranks customers by total spent and order count in lab_target. Supports a limit query param (default 10, max 100).
//	@Tags			analytics
//	@Produce		json
//	@Param			limit	query		int		false	"Maximum number of customers (default 10, max 100)"	minimum(1)	maximum(100)
//	@Success		200		{object}	handler.AnalyticsListResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/target/analytics/top-customers [get]
func (h *TargetHandler) TopCustomers(c *gin.Context) {
	var q topQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		RespondValidationError(c, err)
		return
	}
	limit := service.NormalizeAnalyticsLimit(q.Limit)

	rows, err := h.svc.TopCustomers(c.Request.Context(), limit)
	if err != nil {
		RespondInternalError(c, err)
		return
	}
	RespondJSON(c, http.StatusOK, AnalyticsListResponse{
		Data: rows,
		Meta: LimitMeta{Limit: limit},
	})
}
