package main

import (
	"context"
	"errors"
	"log/slog"
	"os/signal"
	"syscall"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jkeddari/hypermetrics/internal/config"
	"github.com/jkeddari/hypermetrics/internal/db"
	"github.com/jkeddari/hypermetrics/internal/hypercore"
	"github.com/jkeddari/hypermetrics/internal/logger"
	"github.com/jkeddari/hypermetrics/internal/s3ingest"
)

func main() {
	cfg := config.Load()
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

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("ap-northeast-1"))
	if err != nil {
		panic(err)
	}

	store := hypercore.OpenStore(database, hypercore.PriorityConfig{
		WhaleThresholdUSD:    cfg.HypercoreWhaleThresholdUSD,
		RejectedCandidateTTL: cfg.HypercoreRejectedCandidateTTL,
	})
	ingester := s3ingest.New(s3.NewFromConfig(awsCfg), store, s3ingest.Config{
		Lookback:     cfg.S3IngestLookback,
		PollInterval: cfg.S3IngestPollInterval,
	})

	slog.Info("Hyperliquid S3 ingestion service starting",
		"bucket", s3ingest.Bucket,
		"prefix", s3ingest.Prefix,
		"lookback", cfg.S3IngestLookback,
		"poll_interval", cfg.S3IngestPollInterval,
	)
	if err := ingester.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		panic(err)
	}
}
