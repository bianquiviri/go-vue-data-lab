package repository

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapPgError(t *testing.T) {
	t.Run("nil passthrough", func(t *testing.T) {
		if err := mapPgError("op", nil); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("unique violation to ErrConflict", func(t *testing.T) {
		err := mapPgError("create customer", &pgconn.PgError{Code: "23505"})
		if !errors.Is(err, ErrConflict) {
			t.Fatalf("expected ErrConflict, got %v", err)
		}
	})

	t.Run("foreign key violation to ErrInvalidReference", func(t *testing.T) {
		err := mapPgError("create order", &pgconn.PgError{Code: "23503"})
		if !errors.Is(err, ErrInvalidReference) {
			t.Fatalf("expected ErrInvalidReference, got %v", err)
		}
	})

	t.Run("wrapped pg error is detected", func(t *testing.T) {
		base := errors.New("db")
		wrapped := errors.Join(base, &pgconn.PgError{Code: "23505"})
		err := mapPgError("op", wrapped)
		if !errors.Is(err, ErrConflict) {
			t.Fatalf("expected ErrConflict, got %v", err)
		}
	})

	t.Run("other pg code wraps original", func(t *testing.T) {
		pg := &pgconn.PgError{Code: "42P01"}
		err := mapPgError("op", pg)
		if errors.Is(err, ErrConflict) || errors.Is(err, ErrInvalidReference) {
			t.Fatalf("unexpected sentinel mapping: %v", err)
		}
		if !errors.Is(err, pg) {
			t.Fatalf("expected original error wrapped, got %v", err)
		}
	})

	t.Run("plain error wraps original", func(t *testing.T) {
		base := errors.New("boom")
		err := mapPgError("op", base)
		if !errors.Is(err, base) {
			t.Fatalf("expected original error wrapped, got %v", err)
		}
	})
}
