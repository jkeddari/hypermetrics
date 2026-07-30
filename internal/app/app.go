package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/jkeddari/hypermetrics/internal/config"
	"github.com/jkeddari/hypermetrics/internal/db"
	"github.com/jkeddari/hypermetrics/internal/hypercore"
	"github.com/jkeddari/hypermetrics/internal/service"
)

type APIApp struct {
	Cfg              *config.Config
	DB               *sql.DB
	APIKeyService    *service.APIKeyService
	HypercoreService *hypercore.Service
	HypercoreStore   *hypercore.Store
}

func NewAPI(cfg *config.Config) (*APIApp, error) {
	database, err := db.InitPostgres(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := db.RunMigrations(database); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	hypercoreService, err := hypercore.Open(hypercore.Config{
		DB:                   database,
		LeaderboardURL:       cfg.HypercoreLeaderboardURL,
		RefreshRatePerSecond: cfg.HypercoreRefreshRate,
		StatsLogInterval:     cfg.HypercoreStatsLogInterval,
		WhaleThresholdUSD:    cfg.HypercoreWhaleThresholdUSD,
		RejectedCandidateTTL: cfg.HypercoreRejectedCandidateTTL,
	})
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("failed to initialize hypercore store: %w", err)
	}

	return &APIApp{
		Cfg:              cfg,
		DB:               database,
		APIKeyService:    service.NewAPIKeyService(cfg.APIKeys...),
		HypercoreService: hypercoreService,
		HypercoreStore:   hypercoreService.Store(),
	}, nil
}

func (a *APIApp) RunHypercoreCollectors(ctx context.Context) {
	if a == nil || a.HypercoreService == nil || !a.Cfg.HypercoreRunCollectors {
		return
	}

	go func() {
		slog.Info("hypercore collectors starting inside api process")
		if err := a.HypercoreService.RunCollectors(ctx); err != nil && ctx.Err() == nil {
			slog.Error("hypercore collectors stopped", "error", err)
		}
	}()
}

func (a *APIApp) Close() error {
	if a.HypercoreService != nil {
		if err := a.HypercoreService.Close(); err != nil {
			return err
		}
	}
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}
