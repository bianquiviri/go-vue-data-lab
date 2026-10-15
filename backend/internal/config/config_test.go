package config

import (
	"testing"
	"time"
)

func TestGetEnvDuration(t *testing.T) {
	const key = "TEST_DURATION_VAR"
	const fallback = 42 * time.Second

	t.Run("unset returns default", func(t *testing.T) {
		if got := getEnvDuration(key, fallback); got != fallback {
			t.Errorf("got %v, want %v", got, fallback)
		}
	})

	t.Run("valid value is parsed", func(t *testing.T) {
		t.Setenv(key, "15m")
		if got := getEnvDuration(key, fallback); got != 15*time.Minute {
			t.Errorf("got %v, want 15m", got)
		}
	})

	t.Run("invalid value falls back", func(t *testing.T) {
		t.Setenv(key, "not-a-duration")
		if got := getEnvDuration(key, fallback); got != fallback {
			t.Errorf("got %v, want %v", got, fallback)
		}
	})

	t.Run("non-positive value falls back", func(t *testing.T) {
		t.Setenv(key, "-5m")
		if got := getEnvDuration(key, fallback); got != fallback {
			t.Errorf("got %v, want %v", got, fallback)
		}
	})
}

func TestLoadJWTTTLs(t *testing.T) {
	t.Setenv("JWT_ACCESS_TTL", "30m")
	t.Setenv("JWT_REFRESH_TTL", "72h")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.JWTAccessTTL != 30*time.Minute {
		t.Errorf("JWTAccessTTL = %v, want 30m", cfg.JWTAccessTTL)
	}
	if cfg.JWTRefreshTTL != 72*time.Hour {
		t.Errorf("JWTRefreshTTL = %v, want 72h", cfg.JWTRefreshTTL)
	}
}

func TestLoadJWTTTLDefaults(t *testing.T) {
	t.Setenv("JWT_ACCESS_TTL", "")
	t.Setenv("JWT_REFRESH_TTL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.JWTAccessTTL != DefaultJWTAccessTTL {
		t.Errorf("JWTAccessTTL = %v, want %v", cfg.JWTAccessTTL, DefaultJWTAccessTTL)
	}
	if cfg.JWTRefreshTTL != DefaultJWTRefreshTTL {
		t.Errorf("JWTRefreshTTL = %v, want %v", cfg.JWTRefreshTTL, DefaultJWTRefreshTTL)
	}
}
