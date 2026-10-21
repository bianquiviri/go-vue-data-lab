package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/secureid"
)

// codecContextKey is the gin context key holding the opaque id codec installed
// by WithCodec.
const codecContextKey = "secureid.codec"

// WithCodec returns a gin middleware that stores the opaque id codec in the
// request context. RespondJSON/RespondData seal every outbound id through it
// and the id parsers accept only opaque tokens. A nil codec keeps the legacy
// raw-UUID pass-through behavior (used by tests and bootstraps).
func WithCodec(codec *secureid.Codec) gin.HandlerFunc {
	return func(c *gin.Context) {
		if codec != nil {
			c.Set(codecContextKey, codec)
		}
		c.Next()
	}
}

// codecFrom returns the codec installed by WithCodec, or nil when the request
// runs without one.
func codecFrom(c *gin.Context) *secureid.Codec {
	if c == nil {
		return nil
	}
	codec, _ := c.Value(codecContextKey).(*secureid.Codec)
	return codec
}

// sealJSONValue rewrites every opaque id of an already-marshaled JSON document
// in place. Object key order and HTML escaping are preserved, so the sealed
// payload is byte-identical to the unsealed one except for the id values.
func sealJSONValue(raw []byte, codec *secureid.Codec) ([]byte, error) {
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	if len(trimmed) == 0 {
		return raw, nil
	}

	var out bytes.Buffer
	if err := sealInto(&out, trimmed, codec); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// sealInto appends the sealed form of one JSON value to out. Values are raw
// JSON, so non-id leaves are copied verbatim.
func sealInto(out *bytes.Buffer, raw []byte, codec *secureid.Codec) error {
	raw = bytes.TrimLeft(raw, " \t\r\n")
	if len(raw) == 0 {
		return fmt.Errorf("handler: empty json value")
	}

	switch raw[0] {
	case '{':
		return sealObjectInto(out, raw, codec)
	case '[':
		return sealArrayInto(out, raw, codec)
	default:
		// Scalars (strings, numbers, booleans, null) are never rewritten here;
		// id scalars are handled by sealIDInto under their id key.
		out.Write(raw)
		return nil
	}
}

// sealObjectInto rewrites a JSON object, sealing the values of every id key
// and recursing into nested objects/arrays.
func sealObjectInto(out *bytes.Buffer, raw []byte, codec *secureid.Codec) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("handler: decode json object: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("handler: expected json object, got %v", tok)
	}

	out.WriteByte('{')
	first := true
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("handler: decode json object key: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return fmt.Errorf("handler: expected json object key, got %v", keyTok)
		}

		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return fmt.Errorf("handler: decode json object value for key %q: %w", key, err)
		}

		if !first {
			out.WriteByte(',')
		}
		first = false

		keyJSON, err := json.Marshal(key)
		if err != nil {
			return fmt.Errorf("handler: marshal json object key %q: %w", key, err)
		}
		out.Write(keyJSON)
		out.WriteByte(':')

		if isOpaqueIDKey(key) {
			if err := sealIDInto(out, value, codec); err != nil {
				return err
			}
			continue
		}
		if err := sealInto(out, value, codec); err != nil {
			return err
		}
	}

	out.WriteByte('}')
	return nil
}

// sealArrayInto rewrites a JSON array, recursing into every element.
func sealArrayInto(out *bytes.Buffer, raw []byte, codec *secureid.Codec) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("handler: decode json array: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return fmt.Errorf("handler: expected json array, got %v", tok)
	}

	out.WriteByte('[')
	first := true
	for dec.More() {
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return fmt.Errorf("handler: decode json array element: %w", err)
		}
		if !first {
			out.WriteByte(',')
		}
		first = false
		if err := sealInto(out, value, codec); err != nil {
			return err
		}
	}

	out.WriteByte(']')
	return nil
}

// sealIDInto rewrites the value of an id key: UUID strings become UUID tokens,
// int64 strings/numbers become int64 tokens, and anything else (null, objects,
// unparseable values) is copied verbatim so responses never break.
func sealIDInto(out *bytes.Buffer, raw json.RawMessage, codec *secureid.Codec) error {
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	if len(trimmed) == 0 || (trimmed[0] != '"' && trimmed[0] != '-' && (trimmed[0] < '0' || trimmed[0] > '9')) {
		// null, booleans, objects and arrays are not identifiers.
		out.Write(raw)
		return nil
	}

	// Numeric id (e.g. order_items.id): seal only when it parses as int64.
	if trimmed[0] != '"' {
		if n, err := strconv.ParseInt(string(trimmed), 10, 64); err == nil {
			return writeSealed(out, codec.SealInt64(n))
		}
		out.Write(raw)
		return nil
	}

	var value string
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return fmt.Errorf("handler: decode id string: %w", err)
	}

	// String id: UUID first, then int64 (both shapes reach the API today).
	if id, err := uuid.Parse(value); err == nil {
		return writeSealed(out, codec.SealUUID(id))
	}
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		return writeSealed(out, codec.SealInt64(n))
	}

	out.Write(raw)
	return nil
}

// writeSealed appends a sealed token as a JSON string.
func writeSealed(out *bytes.Buffer, token string) error {
	encoded, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("handler: marshal sealed id: %w", err)
	}
	out.Write(encoded)
	return nil
}

// isOpaqueIDKey reports whether a JSON key carries an identifier that must be
// sealed on the way out: the generic "id" primary key or any "*_id" foreign
// key. Non-identifier keys that merely end in "id" (e.g. "valid") do not
// match, since the check is on the exact key or the "_id" suffix.
func isOpaqueIDKey(key string) bool {
	return key == "id" || strings.HasSuffix(key, "_id")
}

// openInboundID converts an inbound id (path parameter, query parameter or
// request body field) into a UUID. With a codec wired, only opaque tokens are
// accepted and raw UUIDs are rejected with a wrapped secureid.ErrInvalidToken;
// without a codec the legacy plain-UUID parsing is kept.
func openInboundID(c *gin.Context, raw string) (uuid.UUID, error) {
	codec := codecFrom(c)
	if codec == nil {
		id, err := uuid.Parse(raw)
		if err != nil {
			return uuid.Nil, fmt.Errorf("handler: parse uuid: %w", err)
		}
		return id, nil
	}

	// Reject raw UUIDs explicitly: with the codec wired the API contract is
	// token-only, and the explicit cause is logged for observability.
	if _, err := uuid.Parse(raw); err == nil {
		return uuid.Nil, fmt.Errorf("handler: raw uuid rejected: %w", secureid.ErrInvalidToken)
	}

	id, err := codec.OpenUUID(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("handler: open opaque id: %w", err)
	}
	return id, nil
}

// invalidIDHint returns the field-level hint for an invalid id value, matching
// the contract in force: opaque tokens with a codec, plain UUIDs without one.
func invalidIDHint(c *gin.Context) string {
	if codecFrom(c) != nil {
		return "must be an opaque id"
	}
	return "must be a valid UUID"
}

// respondInvalidID writes the 400 envelope for an id that could not be opened
// and attaches the underlying cause to the context so the request logger can
// record it.
func respondInvalidID(c *gin.Context, message, field, hint string, cause error) {
	if cause != nil {
		_ = c.Error(fmt.Errorf("handler: invalid id %q: %w", field, cause))
	}
	RespondErrorDetails(c, http.StatusBadRequest, CodeValidation, message,
		map[string]any{"fields": map[string]string{field: hint}})
}
