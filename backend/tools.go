//go:build tools

// Package tools pins dependencies that are required by the project but are not
// yet imported by production code (they arrive in later issues: scheduler,
// auth/JWT and Argon2id hashing). Keeping them here ensures `go mod tidy`
// retains them in go.mod.
package tools

import (
	_ "github.com/golang-jwt/jwt/v5"
	_ "github.com/robfig/cron/v3"
	_ "golang.org/x/crypto/argon2"
)
