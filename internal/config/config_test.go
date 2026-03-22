package config

import "testing"

func TestLoadUsesEnvironmentOverrides(t *testing.T) {
	t.Setenv("OPENCLAW_DATABASE_URL", "postgres://example")
	t.Setenv("OPENCLAW_SECURITY_MASTER_KEY", "top-secret")
	t.Setenv("OPENCLAW_API_LISTEN_ADDR", ":9090")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.API.ListenAddr != ":9090" {
		t.Fatalf("expected listen addr override, got %q", cfg.API.ListenAddr)
	}
	if cfg.Database.URL != "postgres://example" {
		t.Fatalf("expected database override, got %q", cfg.Database.URL)
	}
	if cfg.Security.MasterKey != "top-secret" {
		t.Fatalf("expected master key override, got %q", cfg.Security.MasterKey)
	}
}
