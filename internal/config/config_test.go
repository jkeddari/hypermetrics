package config

import "testing"

func TestLoadUsesDatabaseURLWithoutEnvironmentMode(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/hypermetrics")
	t.Setenv("NATS_URL", "nats://nats:4222")
	t.Setenv("PORT", "9876")
	t.Setenv("STRIPE_BUILDER_PRICE_ID", "price_builder")

	cfg := Load(".env.test")
	if cfg.DatabaseURL != "postgres://localhost/hypermetrics" {
		t.Fatalf("unexpected database URL: %q", cfg.DatabaseURL)
	}
	if cfg.NATSURL != "nats://nats:4222" {
		t.Fatalf("unexpected NATS URL: %q", cfg.NATSURL)
	}
	if cfg.Port != "9876" {
		t.Fatalf("unexpected port: %q", cfg.Port)
	}
	if cfg.StripeBuilderPriceID != "price_builder" {
		t.Fatalf("unexpected Stripe builder price: %q", cfg.StripeBuilderPriceID)
	}
}
