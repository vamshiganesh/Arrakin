package config_test

import (
	"os"
	"testing"

	"github.com/vamshiganesh/arrakin/internal/config"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when DATABASE_URL is missing")
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://arrakin:arrakin@localhost:5432/arrakin?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("APP_ENV", "development")
	os.Unsetenv("HTTP_PORT")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.HTTPPort != 8080 {
		t.Fatalf("expected default HTTP port 8080, got %d", cfg.HTTPPort)
	}
	if !cfg.IsDevelopment() {
		t.Fatal("expected development environment")
	}
}
