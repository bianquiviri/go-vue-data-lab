//go:build tools

// Package tools pins dependencies that are required by the project but are not
// yet imported by production code (the scheduler arrives in a later issue).
// JWT and Argon2id are now imported directly by internal/auth, but are kept
// here so `go mod tidy` retains them even if that code is temporarily removed.
package tools

import (
	_ "github.com/golang-jwt/jwt/v5"
	_ "github.com/robfig/cron/v3"
	_ "golang.org/x/crypto/argon2"
)
