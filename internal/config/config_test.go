package config

import "testing"

func TestLoadUsesDatabaseURLWithoutEnvironmentMode(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/hypermetrics")
	t.Setenv("HYPERCORE_RUN_COLLECTORS", "")

	cfg := Load()
	if cfg.DatabaseURL != "postgres://localhost/hypermetrics" {
		t.Fatalf("unexpected database URL: %q", cfg.DatabaseURL)
	}
	if !cfg.HypercoreRunCollectors {
		t.Fatal("expected collectors to run by default")
	}
}
