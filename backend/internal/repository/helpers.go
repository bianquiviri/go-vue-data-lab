package repository

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// ilikePattern wraps s in a case-insensitive LIKE pattern, escaping the
// wildcard characters so user input is matched literally.
func ilikePattern(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)
	return "%" + s + "%"
}

// applyPaging applies LIMIT/OFFSET only when a positive limit is set.
func applyPaging(q *gorm.DB, opts ListOptions) *gorm.DB {
	if opts.Limit > 0 {
		q = q.Limit(opts.Limit)
	}
	if opts.Offset > 0 {
		q = q.Offset(opts.Offset)
	}
	return q
}

// wrapNotFound converts a GORM lookup failure into either a wrapped
// ErrNotFound or a wrapped unexpected error, both with operation context.
func wrapNotFound(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("repository: %s: %w", op, ErrNotFound)
	}
	return fmt.Errorf("repository: %s: %w", op, err)
}
