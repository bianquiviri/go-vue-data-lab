package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// respondWriteError maps a service write error to the API error envelope:
// not-found → 404, conflict → 409, invalid reference → 400, anything else → 500.
func respondWriteError(c *gin.Context, entity string, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		RespondNotFound(c, entity+" not found")
	case errors.Is(err, service.ErrConflict):
		RespondConflict(c, entity+" conflicts with an existing record")
	case errors.Is(err, service.ErrInvalidReference):
		RespondErrorDetails(c, http.StatusBadRequest, CodeValidation,
			"invalid reference",
			map[string]any{"reason": entity + " references a record that does not exist"})
	default:
		RespondInternalError(c, err)
	}
}

// CreateCustomer handles POST /api/v1/source/customers.
//
//	@Summary		Create a source customer
//	@Description	Creates a customer in lab_source. Email must be unique.
//	@Tags			source
//	@Accept			json
//	@Produce		json
//	@Param			request	body		service.CustomerInput	true	"Customer payload"
//	@Success		201		{object}	DataResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/source/customers [post]
func (h *SourceHandler) CreateCustomer(c *gin.Context) {
	var in service.CustomerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		RespondValidationError(c, err)
		return
	}
	row, err := h.svc.CreateCustomer(c.Request.Context(), in)
	if err != nil {
		respondWriteError(c, "customer", err)
		return
	}
	RespondData(c, http.StatusCreated, row)
}

// UpdateCustomer handles PUT /api/v1/source/customers/:id.
//
//	@Summary		Update a source customer
//	@Description	Replaces an existing customer in lab_source (full replacement).
//	@Tags			source
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Customer id (opaque token)"
//	@Param			request	body		service.CustomerInput	true	"Customer payload"
//	@Success		200		{object}	DataResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/source/customers/{id} [put]
func (h *SourceHandler) UpdateCustomer(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	var in service.CustomerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		RespondValidationError(c, err)
		return
	}
	row, err := h.svc.UpdateCustomer(c.Request.Context(), id, in)
	if err != nil {
		respondWriteError(c, "customer", err)
		return
	}
	RespondData(c, http.StatusOK, row)
}

// DeleteCustomer handles DELETE /api/v1/source/customers/:id.
//
//	@Summary		Delete a source customer
//	@Description	Deletes a customer from lab_source by id.
//	@Tags			source
//	@Param			id	path	string	true	"Customer id (opaque token)"
//	@Success		204
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/source/customers/{id} [delete]
func (h *SourceHandler) DeleteCustomer(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteCustomer(c.Request.Context(), id); err != nil {
		respondWriteError(c, "customer", err)
		return
	}
	c.Status(http.StatusNoContent)
}

// CreateProduct handles POST /api/v1/source/products.
//
//	@Summary		Create a source product
//	@Description	Creates a product in lab_source. SKU must be unique.
//	@Tags			source
//	@Accept			json
//	@Produce		json
//	@Param			request	body		service.ProductInput	true	"Product payload"
//	@Success		201		{object}	DataResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/source/products [post]
func (h *SourceHandler) CreateProduct(c *gin.Context) {
	var in service.ProductInput
	if err := c.ShouldBindJSON(&in); err != nil {
		RespondValidationError(c, err)
		return
	}
	row, err := h.svc.CreateProduct(c.Request.Context(), in)
	if err != nil {
		respondWriteError(c, "product", err)
		return
	}
	RespondData(c, http.StatusCreated, row)
}

// UpdateProduct handles PUT /api/v1/source/products/:id.
//
//	@Summary		Update a source product
//	@Description	Replaces an existing product in lab_source (full replacement).
//	@Tags			source
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Product id (opaque token)"
//	@Param			request	body		service.ProductInput	true	"Product payload"
//	@Success		200		{object}	DataResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/source/products/{id} [put]
func (h *SourceHandler) UpdateProduct(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	var in service.ProductInput
	if err := c.ShouldBindJSON(&in); err != nil {
		RespondValidationError(c, err)
		return
	}
	row, err := h.svc.UpdateProduct(c.Request.Context(), id, in)
	if err != nil {
		respondWriteError(c, "product", err)
		return
	}
	RespondData(c, http.StatusOK, row)
}

// DeleteProduct handles DELETE /api/v1/source/products/:id.
//
//	@Summary		Delete a source product
//	@Description	Deletes a product from lab_source by id.
//	@Tags			source
//	@Param			id	path	string	true	"Product id (opaque token)"
//	@Success		204
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/source/products/{id} [delete]
func (h *SourceHandler) DeleteProduct(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteProduct(c.Request.Context(), id); err != nil {
		respondWriteError(c, "product", err)
		return
	}
	c.Status(http.StatusNoContent)
}

// OrderItemRequest is the wire shape of a single order item on create. The
// product id is an opaque token opened by the handler.
type OrderItemRequest struct {
	ProductID string  `json:"product_id" binding:"required"`
	Quantity  int     `json:"quantity" binding:"gt=0"`
	UnitPrice float64 `json:"unit_price" binding:"gte=0"`
}

// OrderCreateRequest is the wire shape of POST /api/v1/source/orders. Ids are
// opaque tokens (strings) opened by the handler; the service layer keeps
// uuid.UUID, so this DTO only exists at the HTTP boundary.
type OrderCreateRequest struct {
	OrderNumber     string             `json:"order_number"`
	CustomerID      string             `json:"customer_id" binding:"required"`
	Status          string             `json:"status"`
	Currency        string             `json:"currency"`
	ShippingAddress json.RawMessage    `json:"shipping_address" swaggertype:"object"`
	OrderDate       *time.Time         `json:"order_date"`
	Items           []OrderItemRequest `json:"items"`
}

// openInboundOrderCreate converts the wire DTO into the service input, opening
// the opaque ids. It returns ok=false after writing a 400 envelope when any id
// fails to open.
func openInboundOrderCreate(c *gin.Context, req OrderCreateRequest) (service.OrderCreateInput, bool) {
	customerID, err := openInboundID(c, req.CustomerID)
	if err != nil {
		respondInvalidID(c, "invalid customer id", "customer_id", invalidIDHint(c), err)
		return service.OrderCreateInput{}, false
	}

	items := make([]service.OrderItemInput, 0, len(req.Items))
	for i, item := range req.Items {
		productID, err := openInboundID(c, item.ProductID)
		if err != nil {
			respondInvalidID(c, "invalid product id", "product_id", invalidIDHint(c),
				fmt.Errorf("handler: open order item %d product_id: %w", i, err))
			return service.OrderCreateInput{}, false
		}
		items = append(items, service.OrderItemInput{
			ProductID: productID,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		})
	}

	return service.OrderCreateInput{
		OrderNumber:     req.OrderNumber,
		CustomerID:      customerID,
		Status:          req.Status,
		Currency:        req.Currency,
		ShippingAddress: req.ShippingAddress,
		OrderDate:       req.OrderDate,
		Items:           items,
	}, true
}

// CreateOrder handles POST /api/v1/source/orders.
//
//	@Summary		Create a source order
//	@Description	Creates an order in lab_source. Items are optional; total_amount is computed from them. customer_id and every item product_id are opaque ids (tokens); raw UUIDs are rejected with a 400. The response returns every id (and each item id) as an opaque token string.
//	@Tags			source
//	@Accept			json
//	@Produce		json
//	@Param			request	body		handler.OrderCreateRequest	true	"Order payload (opaque ids)"
//	@Success		201		{object}	DataResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/source/orders [post]
func (h *SourceHandler) CreateOrder(c *gin.Context) {
	var req OrderCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondValidationError(c, err)
		return
	}
	in, ok := openInboundOrderCreate(c, req)
	if !ok {
		return
	}
	row, err := h.svc.CreateOrder(c.Request.Context(), in)
	if err != nil {
		respondWriteError(c, "order", err)
		return
	}
	RespondData(c, http.StatusCreated, row)
}

// UpdateOrder handles PUT /api/v1/source/orders/:id.
//
//	@Summary		Update a source order
//	@Description	Partially updates an order's header fields (status, currency, shipping_address, order_date). Items are not modified.
//	@Tags			source
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"Order id (opaque token)"
//	@Param			request	body		service.OrderUpdateInput	true	"Order update payload"
//	@Success		200		{object}	DataResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/source/orders/{id} [put]
func (h *SourceHandler) UpdateOrder(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	var in service.OrderUpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		RespondValidationError(c, err)
		return
	}
	row, err := h.svc.UpdateOrder(c.Request.Context(), id, in)
	if err != nil {
		respondWriteError(c, "order", err)
		return
	}
	RespondData(c, http.StatusOK, row)
}

// DeleteOrder handles DELETE /api/v1/source/orders/:id.
//
//	@Summary		Delete a source order
//	@Description	Deletes an order (and its items) from lab_source by id.
//	@Tags			source
//	@Param			id	path	string	true	"Order id (opaque token)"
//	@Success		204
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/source/orders/{id} [delete]
func (h *SourceHandler) DeleteOrder(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteOrder(c.Request.Context(), id); err != nil {
		respondWriteError(c, "order", err)
		return
	}
	c.Status(http.StatusNoContent)
}
