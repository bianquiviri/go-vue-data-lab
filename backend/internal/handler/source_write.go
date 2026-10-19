package handler

import (
	"errors"
	"net/http"

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
//	@Param			id		path		string					true	"Customer UUID"
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
//	@Param			id	path	string	true	"Customer UUID"
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
//	@Param			id		path		string				true	"Product UUID"
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
//	@Param			id	path	string	true	"Product UUID"
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

// CreateOrder handles POST /api/v1/source/orders.
//
//	@Summary		Create a source order
//	@Description	Creates an order in lab_source. Items are optional; total_amount is computed from them.
//	@Tags			source
//	@Accept			json
//	@Produce		json
//	@Param			request	body		service.OrderCreateInput	true	"Order payload"
//	@Success		201		{object}	DataResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/source/orders [post]
func (h *SourceHandler) CreateOrder(c *gin.Context) {
	var in service.OrderCreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		RespondValidationError(c, err)
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
//	@Param			id		path		string						true	"Order UUID"
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
//	@Param			id	path	string	true	"Order UUID"
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
