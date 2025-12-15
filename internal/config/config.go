package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Application
	AppName      string
	AppEnv       string
	AppURL       string
	Port         string
	AppTagline   string
	SupportEmail string
	ContentPath  string

	// Database (optional driver switch via ENV, default: sqlite)
	DBSupabasePassword string // Supabase PostgreSQL password

	// Security
	JWTSecret                string
	JWTExpiry                time.Duration
	TokenEmailVerifyExpiry   time.Duration
	TokenPasswordResetExpiry time.Duration
	TokenEmailChangeExpiry   time.Duration
	TokenMagicLinkExpiry     time.Duration

	// OAuth
	GoogleClientID     string
	GoogleClientSecret string
	GitHubClientID     string
	GitHubClientSecret string

	// Email
	EmailFrom        string
	ResendAPIKey     string
	ResendAudienceID string

	// Payment
	PaymentProvider string // "polar" or "stripe"
	// Payment - Polar
	PolarAPIKey                string
	PolarWebhookSecret         string
	PolarSandboxMode           bool
	PolarProductIDFreePlan     string
	PolarProductIDBasicMonthly string
	PolarProductIDBasicYearly  string
	PolarProductIDProMonthly   string
	PolarProductIDProYearly    string
	// Payment - Stripe
	StripeSecretKey                string
	StripeWebhookSecret            string
	StripePriceIDProMonthly        string
	StripePriceIDProYearly         string
	StripePriceIDEnterpriseMonthly string
	StripePriceIDEnterpriseYearly  string

	// Analytics (all optional, can be used simultaneously)
	UmamiWebsiteID    string
	UmamiHost         string // Default: cloud.umami.is, can be self-hosted
	PlausibleDomain   string
	PlausibleHost     string // Default: plausible.io, can be self-hosted
	GoogleAnalyticsID string

	// Observability (optional)
	SentryDSN string

	// Storage (S3-compatible: MinIO, AWS S3, Cloudflare R2, DigitalOcean Spaces, etc.)
	S3Region               string
	S3Bucket               string
	S3AccessKey            string
	S3SecretKey            string
	S3Endpoint             string        // Optional: for S3-compatible services (MinIO, DO Spaces, R2, etc.)
	S3PresignExpiryPublic  time.Duration // Expiry for public files (avatars, profile pics) - default: 7 days
	S3PresignExpiryPrivate time.Duration // Expiry for private files (documents, uploads) - default: 1 hour
}

func Load() *Config {
	// Load .env file if it exists
	err := godotenv.Load()
	if err != nil {
		slog.Info("no .env file found, using environment variables")
	}

	cfg := &Config{
		// Application
		AppName:      envString("APP_NAME", "Acme"),
		AppEnv:       envRequired("APP_ENV"), // Required: 'development' or 'production'
		AppURL:       envRequired("APP_URL"), // Required: base URL for email links and OAuth redirects
		Port:         envString("PORT", "8080"),
		AppTagline:   envString("APP_TAGLINE", "Build better products faster"),
		SupportEmail: envString("SUPPORT_EMAIL", "hello@example.com"),
		ContentPath:  envString("CONTENT_PATH", "content"),

		// Database
		DBSupabasePassword: envString("DB_PASSWORD", ""),

		// Security
		JWTSecret:                envRequired("JWT_SECRET"),
		JWTExpiry:                envDuration("JWT_EXPIRY", 168*time.Hour),                // 7 days
		TokenEmailVerifyExpiry:   envDuration("TOKEN_EMAIL_VERIFY_EXPIRY", 24*time.Hour),  // 24 hours
		TokenPasswordResetExpiry: envDuration("TOKEN_PASSWORD_RESET_EXPIRY", 1*time.Hour), // 1 hour
		TokenEmailChangeExpiry:   envDuration("TOKEN_EMAIL_CHANGE_EXPIRY", 24*time.Hour),  // 24 hours
		TokenMagicLinkExpiry:     envDuration("TOKEN_MAGIC_LINK_EXPIRY", 10*time.Minute),  // 10 minutes

		// OAuth
		GoogleClientID:     envString("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: envString("GOOGLE_CLIENT_SECRET", ""),
		GitHubClientID:     envString("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: envString("GITHUB_CLIENT_SECRET", ""),

		// Email (RESEND_API_KEY optional in development, required in production)
		EmailFrom:        envString("EMAIL_FROM", "noreply@example.com"),
		ResendAPIKey:     envString("RESEND_API_KEY", ""),
		ResendAudienceID: envString("RESEND_AUDIENCE_ID", ""),

		// Payment (provider selection and configuration)
		PaymentProvider:                envString("PAYMENT_PROVIDER", "polar"), // Default: polar
		PolarAPIKey:                    envString("POLAR_API_KEY", ""),
		PolarWebhookSecret:             envString("POLAR_WEBHOOK_SECRET", ""),
		PolarSandboxMode:               envBool("POLAR_SANDBOX_MODE", envString("APP_ENV", "development") == "development"),
		PolarProductIDFreePlan:         envString("POLAR_PRODUCT_ID_FREE", ""),
		PolarProductIDBasicMonthly:     envString("POLAR_PRODUCT_ID_BASIC_MONTHLY", ""),
		PolarProductIDBasicYearly:      envString("POLAR_PRODUCT_ID_BASIC_YEARLY", ""),
		PolarProductIDProMonthly:       envString("POLAR_PRODUCT_ID_PRO_MONTHLY", ""),
		PolarProductIDProYearly:        envString("POLAR_PRODUCT_ID_PRO_YEARLY", ""),
		StripeSecretKey:                envString("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret:            envString("STRIPE_WEBHOOK_SECRET", ""),
		StripePriceIDProMonthly:        envString("STRIPE_PRICE_ID_PRO_MONTHLY", ""),
		StripePriceIDProYearly:         envString("STRIPE_PRICE_ID_PRO_YEARLY", ""),
		StripePriceIDEnterpriseMonthly: envString("STRIPE_PRICE_ID_ENTERPRISE_MONTHLY", ""),
		StripePriceIDEnterpriseYearly:  envString("STRIPE_PRICE_ID_ENTERPRISE_YEARLY", ""),

		// Analytics
		UmamiWebsiteID:    envString("UMAMI_WEBSITE_ID", ""),
		UmamiHost:         envString("UMAMI_HOST", "cloud.umami.is"),
		PlausibleDomain:   envString("PLAUSIBLE_DOMAIN", ""),
		PlausibleHost:     envString("PLAUSIBLE_HOST", "plausible.io"),
		GoogleAnalyticsID: envString("GOOGLE_ANALYTICS_ID", ""),

		// Observability
		SentryDSN: envString("SENTRY_DSN", ""),

		// Storage (S3-compatible - required for avatar uploads)
		S3Region:               envRequired("S3_REGION"),
		S3Bucket:               envRequired("S3_BUCKET"),
		S3AccessKey:            envRequired("S3_ACCESS_KEY"),
		S3SecretKey:            envRequired("S3_SECRET_KEY"),
		S3Endpoint:             envString("S3_ENDPOINT", ""),                           // Optional: for non-AWS providers
		S3PresignExpiryPublic:  envDuration("S3_PRESIGN_EXPIRY_PUBLIC", 168*time.Hour), // Default: 7 days for public files
		S3PresignExpiryPrivate: envDuration("S3_PRESIGN_EXPIRY_PRIVATE", 1*time.Hour),  // Default: 1 hour for private files
	}

	// Production: validate required services
	if cfg.IsProduction() {
		validateProduction(cfg)
	}

	return cfg
}

// validateProduction ensures all required services are configured for production deployments.
// Development allows some services (like email) to use fallback modes for easier local testing.
func validateProduction(cfg *Config) {
	if cfg.ResendAPIKey == "" {
		slog.Error("production deployment requires RESEND_API_KEY",
			"hint", "set APP_ENV=development for local testing with email log mode")
		os.Exit(1)
	}
}

func envString(key, def string) string {
	value := os.Getenv(key)
	if value == "" {
		value = def
	}
	return value
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		slog.Warn("config invalid bool, using default", "key", key, "value", v, "default", def)
		return def
	}
	return b
}

func envDuration(key string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		slog.Warn("config invalid duration, using default", "key", key, "value", v, "default", def)
		return def
	}
	return d
}

func envRequired(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	slog.Error("config required env var missing", "key", key)
	os.Exit(1)
	return ""
}

func (c *Config) IsDevelopment() bool {
	return c.AppEnv == "development"
}

func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

// Sanitized returns a copy of the config with only public/safe fields.
// All secrets, credentials, and sensitive data are excluded.
// Safe to expose in ctx, templates and client-facing contexts.
func (c *Config) Sanitized() *Config {
	return &Config{
		AppName:      c.AppName,
		AppEnv:       c.AppEnv,
		AppURL:       c.AppURL,
		Port:         c.Port,
		AppTagline:   c.AppTagline,
		SupportEmail: c.SupportEmail,

		EmailFrom: c.EmailFrom,

		GoogleClientID: c.GoogleClientID,
		GitHubClientID: c.GitHubClientID,

		UmamiWebsiteID:    c.UmamiWebsiteID,
		UmamiHost:         c.UmamiHost,
		PlausibleDomain:   c.PlausibleDomain,
		PlausibleHost:     c.PlausibleHost,
		GoogleAnalyticsID: c.GoogleAnalyticsID,

		S3Endpoint: c.S3Endpoint, // Needed for CSP policies
	}
}
