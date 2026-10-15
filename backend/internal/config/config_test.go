package config_test

import (
	"reflect"
	"testing"

	"github.com/josebianco/go-vue-data-lab/internal/config"
)

func TestLoadCORSDefaultOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("APP_URL", "https://godata.josebianco.local")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := []string{
		"https://godata.josebianco.local",
		"http://localhost:5173",
		"http://127.0.0.1:5173",
	}
	if !reflect.DeepEqual(cfg.CORSAllowedOrigins, want) {
		t.Errorf("CORSAllowedOrigins = %v, want %v", cfg.CORSAllowedOrigins, want)
	}
}

func TestLoadCORSExplicitOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", " https://a.example ,, https://b.example ,https://a.example")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := []string{"https://a.example", "https://b.example"}
	if !reflect.DeepEqual(cfg.CORSAllowedOrigins, want) {
		t.Errorf("CORSAllowedOrigins = %v, want %v (trimmed, deduped)", cfg.CORSAllowedOrigins, want)
	}
}

func TestLoadCORSDefaultsFollowAppURL(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("APP_URL", "https://custom.local")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.CORSAllowedOrigins[0] != "https://custom.local" {
		t.Errorf("expected app URL first, got %v", cfg.CORSAllowedOrigins)
	}
}
