package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected missing database URL error")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://gigi:gigi@localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_HTTP_ADDR", "")
	t.Setenv("GIGI_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.Env != "development" {
		t.Fatalf("Env = %q, want development", cfg.Env)
	}
}
