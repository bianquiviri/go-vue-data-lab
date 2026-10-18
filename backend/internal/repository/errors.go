package repository

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// Sentinel errors returned by repositories for constraint violations.
var (
	// ErrConflict is returned when a unique constraint is violated.
	ErrConflict = errors.New("record conflict")
	// ErrInvalidReference is returned when a foreign key constraint is violated.
	ErrInvalidReference = errors.New("invalid reference")
)

// mapPgError inspects the underlying Postgres error and maps common constraint
// violations to repository sentinel errors. It returns the mapped error wrapped
// with context, or the original wrapped error if no mapping applies.
func mapPgError(op string, err error) error {
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("repository: %s: %w", op, ErrConflict)
		case "23503": // foreign_key_violation
			return fmt.Errorf("repository: %s: %w", op, ErrInvalidReference)
		}
	}

	return fmt.Errorf("repository: %s: %w", op, err)
}
