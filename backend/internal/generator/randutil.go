package generator

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Deterministic helpers.
//
// Every generated value is a pure function of (seed, namespace, index) so that:
//   - re-running with the same seed produces byte-for-byte identical data, and
//   - records can be built on demand (streaming) without sharing mutable state.
//
// The namespace string "scopes" the random stream, so a customer's names and an
// order's status never interfere with each other.

// pick is a package-level generic helper (methods cannot declare type
// parameters) that draws a random element from a slice.
func pick[T any](r *rand.Rand, slice []T) T {
	if len(slice) == 0 {
		var zero T
		return zero
	}
	return slice[r.Intn(len(slice))]
}

func intRange(r *rand.Rand, min, max int) int {
	if max < min {
		max = min
	}
	return r.Intn(max-min+1) + min
}

func float64Range(r *rand.Rand, min, max float64) float64 {
	if max < min {
		max = min
	}
	return min + r.Float64()*(max-min)
}

// round2 rounds to two decimal places (the precision of DECIMAL(10,2) columns).
func round2(v float64) float64 {
	return math.Round(v*100.0) / 100.0
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// deterministicSeed derives a 64-bit seed from the base seed, a namespace and an
// index using SHA-256. Hashing decorrelates the streams.
func (g *Generator) deterministicSeed(namespace string, index int) int64 {
	h := sha256.New()
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(g.cfg.Seed))
	_, _ = h.Write(b[:])
	_, _ = h.Write([]byte(namespace))
	_, _ = h.Write([]byte(":"))
	binary.BigEndian.PutUint64(b[:], uint64(index))
	_, _ = h.Write(b[:])
	sum := h.Sum(nil)
	return int64(binary.BigEndian.Uint64(sum[:8]))
}

// deterministicRand returns a rand.Rand scoped to (namespace, index).
func (g *Generator) deterministicRand(namespace string, index int) *rand.Rand {
	return rand.New(rand.NewSource(g.deterministicSeed(namespace, index)))
}

// deterministicUUID returns a stable, version-5 (SHA-1) UUID derived from the
// dataset namespace and the given key. The dataset namespace itself depends on
// the seed, so different seeds yield disjoint ID spaces.
func (g *Generator) deterministicUUID(namespace string, index int) uuid.UUID {
	return uuid.NewSHA1(g.ns, []byte(fmt.Sprintf("%s:%d", namespace, index)))
}

// deterministicTime produces a stable timestamp within a plausible window
// (2020-01-01 … 2024-12-31 UTC), avoiding any wall-clock coupling.
func (g *Generator) deterministicTime(namespace string, index int) time.Time {
	r := g.deterministicRand(namespace, index)
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	end := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC).Unix()
	sec := r.Int63n(end-start+1) + start
	return time.Unix(sec, 0).UTC()
}

// deterministicEmail builds a unique email. The record index guarantees
// uniqueness; the domain is chosen deterministically.
func (g *Generator) deterministicEmail(index int, fn, ln string) string {
	r := g.deterministicRand("email", index)
	domain := pick(r, emailDomains)
	local := fmt.Sprintf("%s.%s%d", strings.ToLower(fn), strings.ToLower(ln), index)
	return local + "@" + domain
}

// deterministicSKU builds a unique SKU: a prefix derived from the product name
// plus the record index.
func (g *Generator) deterministicSKU(index int, name string) string {
	prefix := ""
	for _, p := range strings.Fields(name) {
		if len(p) > 0 {
			prefix += strings.ToUpper(p[:1])
		}
	}
	if prefix == "" {
		prefix = "SKU"
	}
	return fmt.Sprintf("%s-%08d", prefix, index)
}

// deterministicOrderNumber builds a unique order number.
func (g *Generator) deterministicOrderNumber(index int) string {
	return fmt.Sprintf("ORD-%09d", index)
}
