package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/secureid"
)

// internalTestCodec is the codec shared by the white-box tests. Sealing is
// deterministic, so the expected tokens can be computed in the assertions.
var internalTestCodec = mustInternalTestCodec()

func mustInternalTestCodec() *secureid.Codec {
	codec, err := secureid.New(bytes.Repeat([]byte{0x5a}, 32))
	if err != nil {
		panic("handler internal tests: invalid secureid test key: " + err.Error())
	}
	return codec
}

// newInternalContext returns a gin context backed by a recorder, optionally
// carrying the opaque id codec.
func newInternalContext(codec *secureid.Codec) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if codec != nil {
		c.Set(codecContextKey, codec)
	}
	return c, rec
}

// --- middleware / context helpers -------------------------------------------

func TestCodecFromWithoutCodec(t *testing.T) {
	if got := codecFrom(nil); got != nil {
		t.Errorf("codecFrom(nil) = %v, want nil", got)
	}

	c, _ := newInternalContext(nil)
	if got := codecFrom(c); got != nil {
		t.Errorf("codecFrom(ctx without codec) = %v, want nil", got)
	}
}

func TestCodecFromWithCodec(t *testing.T) {
	c, _ := newInternalContext(internalTestCodec)

	if got := codecFrom(c); got != internalTestCodec {
		t.Errorf("codecFrom(ctx) = %v, want the installed codec", got)
	}
}

// --- JSON sealing walk ------------------------------------------------------

func TestSealJSONValueKeepsEmptyAndScalarInput(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"empty", ""},
		{"whitespace", "  \n\t "},
		{"string scalar", `"plain"`},
		{"number scalar", "42"},
		{"null scalar", "null"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sealJSONValue([]byte(tc.raw), internalTestCodec)
			if err != nil {
				t.Fatalf("sealJSONValue(%q): %v", tc.raw, err)
			}
			if string(got) != tc.raw {
				t.Errorf("sealJSONValue(%q) = %q, want it unchanged", tc.raw, got)
			}
		})
	}
}

func TestSealJSONValueSealsIDShapes(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	customerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"uuid string id", `{"id":"11111111-1111-1111-1111-111111111111"}`,
			`{"id":"` + internalTestCodec.SealUUID(id) + `"}`},
		{"uuid foreign key", `{"customer_id":"22222222-2222-2222-2222-222222222222"}`,
			`{"customer_id":"` + internalTestCodec.SealUUID(customerID) + `"}`},
		{"numeric id", `{"id":7}`, `{"id":"` + internalTestCodec.SealInt64(7) + `"}`},
		{"negative numeric id", `{"id":-42}`, `{"id":"` + internalTestCodec.SealInt64(-42) + `"}`},
		{"numeric string id", `{"id":"12"}`, `{"id":"` + internalTestCodec.SealInt64(12) + `"}`},
		{"third party id", `{"third_party_id":"abc"}`,
			`{"third_party_id":"abc"}`},
		{"non-id key untouched", `{"valid":true,"product_id":"11111111-1111-1111-1111-111111111111"}`,
			`{"valid":true,"product_id":"` + internalTestCodec.SealUUID(id) + `"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sealJSONValue([]byte(tc.raw), internalTestCodec)
			if err != nil {
				t.Fatalf("sealJSONValue(%q): %v", tc.raw, err)
			}
			if string(got) != tc.want {
				t.Errorf("sealJSONValue(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestSealJSONValueCopiesNonIdentifierIDValues(t *testing.T) {
	// Null, booleans, containers and out-of-range/opaque scalars under an id
	// key must be copied verbatim so responses never break.
	cases := []string{
		`{"id":null}`,
		`{"id":true}`,
		`{"id":false}`,
		`{"id":{}}`,
		`{"id":[1,2]}`,
		`{"id":"not-a-uuid"}`,
		`{"id":12345678901234567890}`,
	}

	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			got, err := sealJSONValue([]byte(raw), internalTestCodec)
			if err != nil {
				t.Fatalf("sealJSONValue(%q): %v", raw, err)
			}
			if string(got) != raw {
				t.Errorf("sealJSONValue(%q) = %q, want it unchanged", raw, got)
			}
		})
	}
}

func TestSealJSONValueRecursesIntoContainers(t *testing.T) {
	id := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	itemID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	raw := `{"data":[{"id":"33333333-3333-3333-3333-333333333333","items":[{"id":"44444444-4444-4444-4444-444444444444"}]}],` +
		`"meta":{"page":1}}`
	want := `{"data":[{"id":"` + internalTestCodec.SealUUID(id) + `","items":[{"id":"` +
		internalTestCodec.SealUUID(itemID) + `"}]}],"meta":{"page":1}}`

	got, err := sealJSONValue([]byte(raw), internalTestCodec)
	if err != nil {
		t.Fatalf("sealJSONValue: %v", err)
	}
	if string(got) != want {
		t.Errorf("sealJSONValue() = %q, want %q", got, want)
	}
}

func TestSealJSONValueReportsMalformedJSON(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"unterminated key", `{"a"`},
		{"missing value", `{"a":}`},
		{"non string key", `{[1]:2}`},
		{"missing element", `[,1]`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := sealJSONValue([]byte(tc.raw), internalTestCodec); err == nil {
				t.Errorf("sealJSONValue(%q) = nil error, want a wrapped error", tc.raw)
			}
		})
	}
}

// TestSealJSONValueLeavesTruncatedContainersValid documents the lenient edge:
// a bare "{" or "[" tokenizes as an empty container, so the walk emits "{}" and
// "[]" instead of failing. Only reachable for input that never comes from
// json.Marshal, and the output stays valid JSON.
func TestSealJSONValueLeavesTruncatedContainersValid(t *testing.T) {
	cases := map[string]string{
		`{`: `{}`,
		`[`: `[]`,
	}

	for raw, want := range cases {
		got, err := sealJSONValue([]byte(raw), internalTestCodec)
		if err != nil {
			t.Fatalf("sealJSONValue(%q): %v", raw, err)
		}
		if string(got) != want {
			t.Errorf("sealJSONValue(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestSealWalkReportsUntokenizableInput(t *testing.T) {
	t.Run("object", func(t *testing.T) {
		var out bytes.Buffer

		err := sealObjectInto(&out, []byte(`{x`), internalTestCodec)
		if err == nil {
			t.Fatal("expected an error for an untokenizable object")
		}
		if !strings.Contains(err.Error(), "decode json object") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("array", func(t *testing.T) {
		var out bytes.Buffer

		err := sealArrayInto(&out, []byte(`x`), internalTestCodec)
		if err == nil {
			t.Fatal("expected an error for an untokenizable array")
		}
		if !strings.Contains(err.Error(), "decode json array") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestSealIntoRejectsEmptyValue(t *testing.T) {
	var out bytes.Buffer

	err := sealInto(&out, []byte("  \t"), internalTestCodec)
	if err == nil {
		t.Fatal("expected an error for an empty JSON value")
	}
	if !strings.Contains(err.Error(), "empty json value") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSealObjectIntoRejectsNonObject(t *testing.T) {
	var out bytes.Buffer

	err := sealObjectInto(&out, []byte(`[1]`), internalTestCodec)
	if err == nil {
		t.Fatal("expected an error for a non-object value")
	}
	if !strings.Contains(err.Error(), "expected json object") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSealArrayIntoRejectsNonArray(t *testing.T) {
	var out bytes.Buffer

	err := sealArrayInto(&out, []byte(`{}`), internalTestCodec)
	if err == nil {
		t.Fatal("expected an error for a non-array value")
	}
	if !strings.Contains(err.Error(), "expected json array") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSealIDIntoRejectsMalformedString(t *testing.T) {
	var out bytes.Buffer

	err := sealIDInto(&out, json.RawMessage(`"unterminated`), internalTestCodec)
	if err == nil {
		t.Fatal("expected an error for a malformed id string")
	}
	if !strings.Contains(err.Error(), "decode id string") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestIsOpaqueIDKey(t *testing.T) {
	cases := map[string]bool{
		"id":          true,
		"customer_id": true,
		"valid":       false,
		"video":       false,
		"identity":    false,
	}

	for key, want := range cases {
		if got := isOpaqueIDKey(key); got != want {
			t.Errorf("isOpaqueIDKey(%q) = %v, want %v", key, got, want)
		}
	}
}

// --- inbound id helpers -----------------------------------------------------

func TestOpenInboundIDWithoutCodecParsesRawUUID(t *testing.T) {
	id := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	c, _ := newInternalContext(nil)

	got, err := openInboundID(c, id.String())
	if err != nil {
		t.Fatalf("openInboundID(raw uuid, legacy mode): %v", err)
	}
	if got != id {
		t.Errorf("openInboundID() = %v, want %v", got, id)
	}
}

func TestOpenInboundIDWithoutCodecRejectsGarbage(t *testing.T) {
	c, _ := newInternalContext(nil)

	_, err := openInboundID(c, "not-a-uuid")
	if err == nil {
		t.Fatal("expected an error for a non-UUID value")
	}
	if errors.Is(err, secureid.ErrInvalidToken) {
		t.Errorf("legacy mode must not report ErrInvalidToken: %v", err)
	}
	if !strings.Contains(err.Error(), "parse uuid") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOpenInboundIDWithCodecAcceptsToken(t *testing.T) {
	id := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	c, _ := newInternalContext(internalTestCodec)

	got, err := openInboundID(c, internalTestCodec.SealUUID(id))
	if err != nil {
		t.Fatalf("openInboundID(token): %v", err)
	}
	if got != id {
		t.Errorf("openInboundID() = %v, want %v", got, id)
	}
}

func TestOpenInboundIDWithCodecRejectsRawUUID(t *testing.T) {
	id := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	c, _ := newInternalContext(internalTestCodec)

	_, err := openInboundID(c, id.String())
	if !errors.Is(err, secureid.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
	if !strings.Contains(err.Error(), "raw uuid rejected") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOpenInboundIDWithCodecRejectsInvalidToken(t *testing.T) {
	c, _ := newInternalContext(internalTestCodec)

	_, err := openInboundID(c, "definitely-not-a-token")
	if !errors.Is(err, secureid.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
	if !strings.Contains(err.Error(), "open opaque id") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInvalidIDHintFollowsContract(t *testing.T) {
	withCodec, _ := newInternalContext(internalTestCodec)
	if got := invalidIDHint(withCodec); got != "must be an opaque id" {
		t.Errorf("hint with codec = %q, want %q", got, "must be an opaque id")
	}

	withoutCodec, _ := newInternalContext(nil)
	if got := invalidIDHint(withoutCodec); got != "must be a valid UUID" {
		t.Errorf("hint without codec = %q, want %q", got, "must be a valid UUID")
	}
}

func TestRespondInvalidIDEnvelopeAndCause(t *testing.T) {
	t.Run("with cause", func(t *testing.T) {
		c, rec := newInternalContext(nil)

		respondInvalidID(c, "invalid path parameter", "id", "must be an opaque id",
			secureid.ErrInvalidToken)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
		if len(c.Errors) != 1 {
			t.Fatalf("expected the cause to be attached, got %d errors", len(c.Errors))
		}
		if !errors.Is(c.Errors[0], secureid.ErrInvalidToken) {
			t.Errorf("attached error = %v, want it to wrap ErrInvalidToken", c.Errors[0])
		}
	})

	t.Run("without cause", func(t *testing.T) {
		c, rec := newInternalContext(nil)

		respondInvalidID(c, "invalid path parameter", "id", "must be an opaque id", nil)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
		if len(c.Errors) != 0 {
			t.Errorf("expected no attached errors, got %d", len(c.Errors))
		}

		var body struct {
			Error struct {
				Code   string         `json:"code"`
				Fields map[string]any `json:"details"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode envelope: %v (body: %s)", err, rec.Body.String())
		}
		if body.Error.Code != CodeValidation {
			t.Errorf("code = %q, want %q", body.Error.Code, CodeValidation)
		}
	})
}
