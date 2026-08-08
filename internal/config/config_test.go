package config

import "testing"

func TestLoadUsesDatabaseURLWithoutEnvironmentMode(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/hypermetrics")
	t.Setenv("HYPERCORE_RUN_COLLECTORS", "")
	t.Setenv("PORT", "9876")
	t.Setenv("STRIPE_BUILDER_PRICE_ID", "price_builder")

	cfg := Load(".env.test")
	if cfg.DatabaseURL != "postgres://localhost/hypermetrics" {
		t.Fatalf("unexpected database URL: %q", cfg.DatabaseURL)
	}
	if !cfg.HypercoreRunCollectors {
		t.Fatal("expected collectors to run by default")
	}
	if cfg.Port != "9876" {
		t.Fatalf("unexpected port: %q", cfg.Port)
	}
	if cfg.StripeBuilderPriceID != "price_builder" {
		t.Fatalf("unexpected Stripe builder price: %q", cfg.StripeBuilderPriceID)
	}
}
