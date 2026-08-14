package main

import (
	"context"
	"errors"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/jkeddari/hypermetrics/internal/config"
	"github.com/jkeddari/hypermetrics/internal/corebus"
	"github.com/jkeddari/hypermetrics/internal/db"
	"github.com/jkeddari/hypermetrics/internal/hypercore"
	"github.com/jkeddari/hypermetrics/internal/logger"
)

func main() {
	cfg := config.Load(".env.core")
	logger.Init(cfg.SentryDSN)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	database, err := db.InitPostgres(cfg.DatabaseURL)
	if err != nil {
		panic(err)
	}
	defer database.Close()
	if err := db.RunMigrations(database); err != nil {
		panic(err)
	}

	service, err := hypercore.Open(hypercore.Config{
		DB:                   database,
		LeaderboardURL:       cfg.HypercoreLeaderboardURL,
		RefreshRatePerSecond: cfg.HypercoreRefreshRate,
		StatsLogInterval:     cfg.HypercoreStatsLogInterval,
		DistributionInterval: cfg.HypercoreDistributionInterval,
		WhaleThresholdUSD:    cfg.HypercoreWhaleThresholdUSD,
		RejectedCandidateTTL: cfg.HypercoreRejectedCandidateTTL,
	})
	if err != nil {
		panic(err)
	}
	defer service.Close()

	natsConn, err := corebus.Connect(cfg.NATSURL, "hypermetrics-core")
	if err != nil {
		panic(err)
	}
	defer natsConn.Close()
	if _, err := corebus.Subscribe(natsConn, service); err != nil {
		panic(err)
	}

	slog.Info("core service starting", "nats_subject", corebus.Subject, "refresh_rate_per_second", cfg.HypercoreRefreshRate)
	if err := service.RunCollectors(ctx); err != nil && !errors.Is(err, context.Canceled) {
		panic(err)
	}
}
