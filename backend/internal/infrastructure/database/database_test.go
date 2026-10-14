package database

import (
	"strings"
	"testing"

	"github.com/josebianco/go-vue-data-lab/internal/config"
)

func TestBuildDSN(t *testing.T) {
	cfg := &config.Config{
		DBHost:     "db.internal",
		DBPort:     "5432",
		DBUser:     "lab",
		DBPassword: "secret",
	}

	dsn := buildDSN(cfg, "lab_source")

	wants := []string{
		"host=db.internal",
		"port=5432",
		"user=lab",
		"password=secret",
		"dbname=lab_source",
		"sslmode=disable",
	}
	for _, want := range wants {
		if !strings.Contains(dsn, want) {
			t.Errorf("dsn %q missing %q", dsn, want)
		}
	}
}

func TestOpenNilConfig(t *testing.T) {
	if _, err := Open(nil, "lab_source"); err == nil {
		t.Fatal("expected error for nil config")
	}
}
