package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName    string
	AppEnv     string
	AppURL     string
	APIBaseURL string
	Port       string

	DatabaseURL string

	SentryDSN string

	APIKeys []string

	SessionSecret string
	SessionExpiry time.Duration

	StripeSecretKey             string
	StripeWebhookSecret         string
	StripeBuilderPriceID        string
	StripeProPriceID            string
	StripePortalConfigurationID string
	ResendAPIKey                string
	ResendFromEmail             string

	HypercoreRunCollectors        bool
	HypercoreLeaderboardURL       string
	HypercoreRefreshRate          float64
	HypercoreStatsLogInterval     time.Duration
	HypercoreWhaleThresholdUSD    float64
	HypercoreRejectedCandidateTTL time.Duration

	S3IngestLookback     time.Duration
	S3IngestPollInterval time.Duration
}

func Load(envFile string) *Config {
	if err := godotenv.Load(envFile); err != nil {
		slog.Info("environment file not found, using environment variables", "file", envFile)
	}

	apiKeys := envCSV("HM_API_KEYS", nil)

	return &Config{
		AppName:     envString("APP_NAME", "Hypermetrics"),
		AppEnv:      envString("APP_ENV", "development"),
		AppURL:      envString("APP_URL", "http://localhost:3000"),
		APIBaseURL:  envString("API_BASE_URL", "http://localhost:8080"),
		Port:        envString("PORT", "8080"),
		DatabaseURL: envString("DATABASE_URL", ""),
		SentryDSN:   envString("SENTRY_DSN", ""),
		APIKeys:     apiKeys,

		SessionSecret: envString("SESSION_SECRET", "dev-only-change-me-before-production"),
		SessionExpiry: envDuration("SESSION_EXPIRY", 7*24*time.Hour),

		StripeSecretKey:             envString("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret:         envString("STRIPE_WEBHOOK_SECRET", ""),
		StripeBuilderPriceID:        envString("STRIPE_BUILDER_PRICE_ID", ""),
		StripeProPriceID:            envString("STRIPE_PRO_PRICE_ID", ""),
		StripePortalConfigurationID: envString("STRIPE_PORTAL_CONFIGURATION_ID", ""),
		ResendAPIKey:                envString("RESEND_API_KEY", ""),
		ResendFromEmail:             envString("RESEND_FROM_EMAIL", ""),

		HypercoreRunCollectors:        envBool("HYPERCORE_RUN_COLLECTORS", true),
		HypercoreLeaderboardURL:       envString("HYPERCORE_LEADERBOARD_URL", ""),
		HypercoreRefreshRate:          envFloat("HYPERCORE_REFRESH_RATE", 7),
		HypercoreStatsLogInterval:     envDuration("HYPERCORE_STATS_LOG_INTERVAL", time.Minute),
		HypercoreWhaleThresholdUSD:    envFloat("HYPERCORE_WHALE_THRESHOLD_USD", 1_000_000),
		HypercoreRejectedCandidateTTL: envDuration("HYPERCORE_REJECTED_CANDIDATE_TTL", 12*time.Hour),

		S3IngestLookback:     envDuration("S3_INGEST_LOOKBACK", 30*24*time.Hour),
		S3IngestPollInterval: envDuration("S3_INGEST_POLL_INTERVAL", 15*time.Minute),
	}
}

func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

func envString(key, def string) string {
	value := os.Getenv(key)
	if value == "" {
		return def
	}
	return value
}

func envCSV(key string, def []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return def
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

func envBool(key string, def bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return def
	}
	switch value {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	case "0", "false", "FALSE", "no", "NO", "off", "OFF":
		return false
	default:
		return def
	}
}

func envFloat(key string, def float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return def
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return def
	}
	return parsed
}

func envDuration(key string, def time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return def
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return def
	}
	return parsed
}
