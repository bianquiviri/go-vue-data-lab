package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/router"
	"github.com/josebianco/go-vue-data-lab/internal/secureid"
)

// routerTestCodec is built on a fixed 32-byte key shared by the router
// integration tests. Sealing is deterministic, so tests can compare tokens
// byte for byte.
var routerTestCodec = mustRouterTestCodec()

func mustRouterTestCodec() *secureid.Codec {
	codec, err := secureid.New(bytes.Repeat([]byte{0x5a}, 32))
	if err != nil {
		panic("router tests: invalid secureid test key: " + err.Error())
	}
	return codec
}

// stubSourceDataWithCustomer extends the permissive stub so GetCustomer
// returns a record with a known id and records the id the handler opened from
// the path, letting the test assert both directions of the codec.
type stubSourceDataWithCustomer struct {
	stubSourceData
	customer models.Customer
	openedID uuid.UUID
}

func (s *stubSourceDataWithCustomer) GetCustomer(_ context.Context, id uuid.UUID) (models.Customer, error) {
	s.openedID = id
	return s.customer, nil
}

// TestRouterSecureIDEndToEnd verifies the full opaque-id contract through the
// real router: with the codec wired, raw UUID path params are rejected with a
// 400 while sealed tokens resolve, are opened back into UUIDs and responses
// come back sealed.
func TestRouterSecureIDEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const rawUUID = "7c9e6679-7425-40de-944b-e07fc1f90ae7"

	customerID := uuid.MustParse(rawUUID)
	token := routerTestCodec.SealUUID(customerID)

	data := &stubSourceDataWithCustomer{
		customer: models.Customer{
			ID:        customerID,
			Email:     "alice@example.com",
			FirstName: "Alice",
			LastName:  "Johnson",
		},
	}

	engine := router.New(router.Dependencies{
		Logger: zap.NewNop(),
		Data:   data,
		Codec:  routerTestCodec,
	})

	t.Run("raw uuid path param is rejected", func(t *testing.T) {
		rec := get(t, engine, "/api/v1/source/customers/"+rawUUID, nil)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for a raw UUID path param, got %d (body: %s)",
				http.StatusBadRequest, rec.Code, rec.Body.String())
		}

		var body struct {
			Error struct {
				Code    string `json:"code"`
				Details struct {
					Fields map[string]any `json:"fields"`
				} `json:"details"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode error envelope: %v (body: %s)", err, rec.Body.String())
		}
		if body.Error.Code != "validation_failed" {
			t.Errorf("expected code validation_failed, got %q", body.Error.Code)
		}
		if got := body.Error.Details.Fields["id"]; got != "must be an opaque id" {
			t.Errorf("id hint = %v, want %q", got, "must be an opaque id")
		}
		if data.openedID != (uuid.UUID{}) {
			t.Errorf("the service must not be reached on rejection, opened id %v", data.openedID)
		}
	})

	t.Run("token path param resolves and response id is sealed", func(t *testing.T) {
		data.openedID = uuid.UUID{}

		rec := get(t, engine, "/api/v1/source/customers/"+token, nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d for a sealed token, got %d (body: %s)",
				http.StatusOK, rec.Code, rec.Body.String())
		}

		if data.openedID != customerID {
			t.Errorf("expected the handler to open the token back into %v, got %v", customerID, data.openedID)
		}

		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v (body: %s)", err, rec.Body.String())
		}

		id, ok := body["id"].(string)
		if !ok {
			t.Fatalf("expected a string id in the response, got %v (%T)", body["id"], body["id"])
		}
		if id != token {
			t.Errorf("expected the response id to be the sealed token %q, got %q", token, id)
		}
		if bytes.Contains(rec.Body.Bytes(), []byte(rawUUID)) {
			t.Errorf("the raw UUID must never leak in the response: %s", rec.Body.String())
		}
	})
}

// TestRouterWithoutCodecKeepsLegacyPassThrough verifies that leaving the codec
// unset preserves the legacy raw-UUID behaviour: no sealing and no rejection.
func TestRouterWithoutCodecKeepsLegacyPassThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const rawUUID = "7c9e6679-7425-40de-944b-e07fc1f90ae7"

	customerID := uuid.MustParse(rawUUID)
	data := &stubSourceDataWithCustomer{
		customer: models.Customer{ID: customerID, Email: "bob@example.com"},
	}

	engine := router.New(router.Dependencies{
		Logger: zap.NewNop(),
		Data:   data,
		// Codec intentionally nil: legacy raw-UUID pass-through mode.
	})

	rec := get(t, engine, "/api/v1/source/customers/"+rawUUID, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d without a codec, got %d (body: %s)",
			http.StatusOK, rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v (body: %s)", err, rec.Body.String())
	}
	if body["id"] != rawUUID {
		t.Errorf("expected the id to pass through raw as %q, got %v", rawUUID, body["id"])
	}
}
