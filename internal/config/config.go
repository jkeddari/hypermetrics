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
	APIPort string

	DatabaseURL string

	SentryDSN string

	APIKeys []string

	HypercoreRunCollectors        bool
	HypercoreLeaderboardURL       string
	HypercoreRefreshRate          float64
	HypercoreStatsLogInterval     time.Duration
	HypercoreWhaleThresholdUSD    float64
	HypercoreRejectedCandidateTTL time.Duration
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file found, using environment variables")
	}

	apiKeys := envCSV("HM_API_KEYS", nil)

	return &Config{
		APIPort:     envString("API_PORT", envString("PORT", "8080")),
		DatabaseURL: envString("DATABASE_URL", ""),
		SentryDSN:   envString("SENTRY_DSN", ""),
		APIKeys:     apiKeys,

		HypercoreRunCollectors:        envBool("HYPERCORE_RUN_COLLECTORS", true),
		HypercoreLeaderboardURL:       envString("HYPERCORE_LEADERBOARD_URL", ""),
		HypercoreRefreshRate:          envFloat("HYPERCORE_REFRESH_RATE", 7),
		HypercoreStatsLogInterval:     envDuration("HYPERCORE_STATS_LOG_INTERVAL", time.Minute),
		HypercoreWhaleThresholdUSD:    envFloat("HYPERCORE_WHALE_THRESHOLD_USD", 1_000_000),
		HypercoreRejectedCandidateTTL: envDuration("HYPERCORE_REJECTED_CANDIDATE_TTL", 12*time.Hour),
	}
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
