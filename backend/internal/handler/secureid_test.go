package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/handler"
	"github.com/josebianco/go-vue-data-lab/internal/secureid"
)

// testCodec is built on a fixed 32-byte key shared by every handler test.
// Sealing is deterministic, so tests can compare tokens byte for byte.
var testCodec = mustTestCodec()

func mustTestCodec() *secureid.Codec {
	codec, err := secureid.New(bytes.Repeat([]byte{0x5a}, 32))
	if err != nil {
		panic("handler tests: invalid secureid test key: " + err.Error())
	}
	return codec
}

// sealID returns the opaque token of a UUID under testCodec.
func sealID(id uuid.UUID) string { return testCodec.SealUUID(id) }

// sealInt64ID returns the opaque token of an int64 under testCodec.
func sealInt64ID(v int64) string { return testCodec.SealInt64(v) }

// sealedRespondEngine returns an engine that applies the opaque id codec to
// every request and answers with handler.RespondJSON(status, data).
func sealedRespondEngine(status int, data any) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(handler.WithCodec(testCodec))
	engine.GET("/", func(c *gin.Context) {
		handler.RespondJSON(c, status, data)
	})
	return engine
}

// decodeObject decodes a JSON object body into a generic map.
func decodeObject(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode object body: %v (body: %s)", err, rec.Body.String())
	}
	return body
}

// --- WithCodec DI -----------------------------------------------------------

func TestWithCodecStoresCodecForHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	id := uuid.New()
	engine := gin.New()
	engine.Use(handler.WithCodec(testCodec))
	// A handler that seals through the codec stored in the context proves the
	// middleware ran and installed it.
	engine.GET("/", func(c *gin.Context) {
		handler.RespondData(c, http.StatusOK, map[string]any{"id": id})
	})

	rec := doJSON(t, engine, http.MethodGet, "/", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if got := decodeObject(t, rec)["data"].(map[string]any)["id"]; got != sealID(id) {
		t.Errorf("id = %v, want sealed token %s", got, sealID(id))
	}
}

func TestWithCodecNilKeepsLegacyPassThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(handler.WithCodec(nil))
	id := uuid.New()
	engine.GET("/", func(c *gin.Context) {
		handler.RespondJSON(c, http.StatusOK, map[string]string{"id": id.String()})
	})

	rec := doJSON(t, engine, http.MethodGet, "/", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if got := decodeObject(t, rec)["id"]; got != id.String() {
		t.Errorf("id = %v, want raw %s (no codec wired)", got, id)
	}
}

// --- outbound sealing -------------------------------------------------------

func TestRespondJSONSealsIDsAndPreservesShape(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	customerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	payload := struct {
		ID         uuid.UUID `json:"id"`
		Name       string    `json:"name"`
		CustomerID uuid.UUID `json:"customer_id"`
	}{ID: id, Name: "<b&c > \"quoted\"", CustomerID: customerID}

	rec := doJSON(t, sealedRespondEngine(http.StatusOK, payload), http.MethodGet, "/", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("expected JSON content type, got %q", ct)
	}
	// gin marshals with encoding/json semantics (HTML escaping on); the sealing
	// walk re-marshals the very same way, so only the sealed ids may differ.
	nameJSON, err := json.Marshal("<b&c > \"quoted\"")
	if err != nil {
		t.Fatalf("marshal name: %v", err)
	}
	want := `{"id":"` + sealID(id) + `","name":` + string(nameJSON) + `,"customer_id":"` + sealID(customerID) + `"}`
	if rec.Body.String() != want {
		t.Errorf("sealed body mismatch:\n got: %s\nwant: %s", rec.Body.String(), want)
	}
	if strings.Contains(rec.Body.String(), id.String()) {
		t.Errorf("raw uuid leaked in response: %s", rec.Body.String())
	}
}

func TestRespondJSONSealsInt64IDs(t *testing.T) {
	payload := struct {
		ID    int64 `json:"id"`
		Total int64 `json:"total"`
	}{ID: 42, Total: 7}

	rec := doJSON(t, sealedRespondEngine(http.StatusOK, payload), http.MethodGet, "/", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	want := `{"id":"` + sealInt64ID(42) + `","total":7}`
	if rec.Body.String() != want {
		t.Errorf("sealed body mismatch:\n got: %s\nwant: %s", rec.Body.String(), want)
	}
}

func TestRespondDataSealsNestedIDs(t *testing.T) {
	orderID := uuid.New()
	customerID := uuid.New()
	productID := uuid.New()

	item := struct {
		ID        int64     `json:"id"`
		OrderID   uuid.UUID `json:"order_id"`
		ProductID uuid.UUID `json:"product_id"`
		Subtotal  float64   `json:"subtotal"`
	}{ID: 42, OrderID: orderID, ProductID: productID, Subtotal: 10}
	payload := struct {
		ID         uuid.UUID `json:"id"`
		CustomerID uuid.UUID `json:"customer_id"`
		OrderItems []struct {
			ID        int64     `json:"id"`
			OrderID   uuid.UUID `json:"order_id"`
			ProductID uuid.UUID `json:"product_id"`
			Subtotal  float64   `json:"subtotal"`
		} `json:"order_items"`
	}{ID: orderID, CustomerID: customerID, OrderItems: []struct {
		ID        int64     `json:"id"`
		OrderID   uuid.UUID `json:"order_id"`
		ProductID uuid.UUID `json:"product_id"`
		Subtotal  float64   `json:"subtotal"`
	}{item}}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(handler.WithCodec(testCodec))
	engine.GET("/", func(c *gin.Context) {
		handler.RespondData(c, http.StatusOK, payload)
	})

	rec := doJSON(t, engine, http.MethodGet, "/", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	data, ok := decodeObject(t, rec)["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %s", rec.Body.String())
	}
	if data["id"] != sealID(orderID) || data["customer_id"] != sealID(customerID) {
		t.Errorf("expected sealed order/customer ids, got %+v", data)
	}
	items, ok := data["order_items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one order item, got %+v", data["order_items"])
	}
	gotItem, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected order item object, got %+v", items[0])
	}
	if gotItem["id"] != sealInt64ID(42) {
		t.Errorf("order item id = %v, want token for 42", gotItem["id"])
	}
	if gotItem["order_id"] != sealID(orderID) || gotItem["product_id"] != sealID(productID) {
		t.Errorf("expected sealed item foreign keys, got %+v", gotItem)
	}
	if gotItem["subtotal"] != 10.0 {
		t.Errorf("subtotal = %v, want 10", gotItem["subtotal"])
	}
}

func TestRespondJSONLeavesNullIDsUntouched(t *testing.T) {
	id := uuid.New()
	payload := struct {
		ID         uuid.UUID  `json:"id"`
		ScheduleID *uuid.UUID `json:"schedule_id"`
	}{ID: id}

	rec := doJSON(t, sealedRespondEngine(http.StatusOK, payload), http.MethodGet, "/", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	want := `{"id":"` + sealID(id) + `","schedule_id":null}`
	if rec.Body.String() != want {
		t.Errorf("sealed body mismatch:\n got: %s\nwant: %s", rec.Body.String(), want)
	}
}

func TestRespondJSONIgnoresNonScalarIDs(t *testing.T) {
	payload := struct {
		ID    any    `json:"id"`
		Label string `json:"label"`
	}{ID: "not-an-identifier", Label: "keep"}

	rec := doJSON(t, sealedRespondEngine(http.StatusOK, payload), http.MethodGet, "/", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	want := `{"id":"not-an-identifier","label":"keep"}`
	if rec.Body.String() != want {
		t.Errorf("sealed body mismatch:\n got: %s\nwant: %s", rec.Body.String(), want)
	}
}

func TestRespondPaginatedSealsItemsAndKeepsMetaRaw(t *testing.T) {
	id := uuid.New()
	items := []struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}{{ID: id, Name: "Ada"}}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(handler.WithCodec(testCodec))
	engine.GET("/", func(c *gin.Context) {
		handler.RespondPaginated(c, http.StatusOK, items, handler.Pagination{Page: 2, PerPage: 5}, 42)
	})

	rec := doJSON(t, engine, http.MethodGet, "/", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	// Meta keeps raw integers; only the data rows carry opaque ids.
	want := `{"data":[{"id":"` + sealID(id) + `","name":"Ada"}],"meta":{"page":2,"per_page":5,"total":42,"total_pages":9}}`
	if rec.Body.String() != want {
		t.Errorf("sealed body mismatch:\n got: %s\nwant: %s", rec.Body.String(), want)
	}
}

func TestRespondJSONMarshalFailureReturns500(t *testing.T) {
	rec := doJSON(t, sealedRespondEngine(http.StatusOK, make(chan int)), http.MethodGet, "/", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}
	if resp := decodeError(t, rec); resp.Error.Code != handler.CodeInternal {
		t.Errorf("expected code %s, got %q", handler.CodeInternal, resp.Error.Code)
	}
	if strings.Contains(rec.Body.String(), "chan int") {
		t.Errorf("marshal failure leaked to the response: %s", rec.Body.String())
	}
}
