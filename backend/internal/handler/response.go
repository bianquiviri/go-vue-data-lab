package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Machine-readable error codes returned in the API error envelope.
const (
	CodeValidation = "validation_failed"
	CodeBadRequest = "bad_request"
	CodeNotFound   = "not_found"
	CodeConflict   = "conflict"
	CodeInternal   = "internal_error"
)

// genericInternalMessage is returned to clients on 500s so internal details
// never leak; the underlying error is attached to the gin context instead and
// surfaced by the request logger.
const genericInternalMessage = "internal server error"

// ErrorBody is the payload of the error envelope:
//
//	{"error":{"code":"not_found","message":"...","details":{...}}}
type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// ErrorResponse is the stable JSON envelope returned for every error.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// DataResponse is the stable success envelope of the control endpoints:
//
//	{"data": ...}
type DataResponse struct {
	Data any `json:"data"`
}

// RespondJSON writes data as JSON with the given status code. When the request
// carries an opaque id codec (see WithCodec), every "id"/"*_id" value in the
// payload is sealed into an opaque token; without a codec the payload is
// written unchanged (legacy pass-through for tests and bootstraps).
func RespondJSON(c *gin.Context, status int, data any) {
	codec := codecFrom(c)
	if codec == nil {
		c.JSON(status, data)
		return
	}

	raw, err := json.Marshal(data)
	if err != nil {
		RespondInternalError(c, fmt.Errorf("handler: marshal response payload: %w", err))
		return
	}

	sealed, err := sealJSONValue(raw, codec)
	if err != nil {
		RespondInternalError(c, fmt.Errorf("handler: seal response payload: %w", err))
		return
	}

	c.Data(status, "application/json; charset=utf-8", sealed)
}

// RespondData writes data wrapped in the standard success envelope.
func RespondData(c *gin.Context, status int, data any) {
	RespondJSON(c, status, DataResponse{Data: data})
}

// RespondError writes the standard error envelope.
func RespondError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorResponse{
		Error: ErrorBody{Code: code, Message: message},
	})
}

// RespondErrorDetails writes the standard error envelope with a details map.
func RespondErrorDetails(c *gin.Context, status int, code, message string, details map[string]any) {
	c.AbortWithStatusJSON(status, ErrorResponse{
		Error: ErrorBody{Code: code, Message: message, Details: details},
	})
}

// RespondBadRequest writes a 400 with the standard error envelope.
func RespondBadRequest(c *gin.Context, message string) {
	RespondError(c, http.StatusBadRequest, CodeBadRequest, message)
}

// RespondNotFound writes a 404 with the standard error envelope.
func RespondNotFound(c *gin.Context, message string) {
	RespondError(c, http.StatusNotFound, CodeNotFound, message)
}

// RespondConflict writes a 409 with the standard error envelope.
func RespondConflict(c *gin.Context, message string) {
	RespondError(c, http.StatusConflict, CodeConflict, message)
}

// RespondInternalError logs err on the gin context (so the request logger can
// record it) and writes a 500 with a generic message.
func RespondInternalError(c *gin.Context, err error) {
	if err != nil {
		_ = c.Error(err)
	}
	RespondError(c, http.StatusInternalServerError, CodeInternal, genericInternalMessage)
}

// RespondValidationError writes a 400 for a binding/validation failure.
//
// Field-level failures (from go-playground/validator via Gin binding) are
// reported under details.fields keyed by struct field name; any other error
// (e.g. malformed JSON) is reported under details.reason.
func RespondValidationError(c *gin.Context, err error) {
	details := map[string]any{}

	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fields := make(map[string]string, len(ve))
		for _, fe := range ve {
			fields[fe.Field()] = fmt.Sprintf("failed on '%s' validation", fe.Tag())
		}
		details["fields"] = fields
	} else {
		details["reason"] = err.Error()
	}

	RespondErrorDetails(c, http.StatusBadRequest, CodeValidation, "validation failed", details)
}
