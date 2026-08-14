package hypercore

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"
)

type Config struct {
	DB                   *sql.DB
	LeaderboardURL       string
	LeaderboardInterval  time.Duration
	RefreshRatePerSecond float64
	StatsLogInterval     time.Duration
	DistributionInterval time.Duration
	WhaleThresholdUSD    float64
	RejectedCandidateTTL time.Duration
}

type Service struct {
	cfg       Config
	store     *Store
	collector *collector
	queue     *RefreshQueue
}

func Open(cfg Config) (*Service, error) {
	if cfg.DB == nil {
		return nil, ErrStoreUnavailable
	}
	if cfg.LeaderboardInterval <= 0 {
		cfg.LeaderboardInterval = 10 * time.Minute
	}
	if cfg.RefreshRatePerSecond <= 0 {
		cfg.RefreshRatePerSecond = 7
	}
	if cfg.DistributionInterval <= 0 {
		cfg.DistributionInterval = 15 * time.Minute
	}

	priorityCfg := PriorityConfig{
		WhaleThresholdUSD:    cfg.WhaleThresholdUSD,
		RejectedCandidateTTL: cfg.RejectedCandidateTTL,
	}

	store := OpenStore(cfg.DB, priorityCfg)

	client := newHyperliquidClient("", nil)
	collector := newCollector(store, &http.Client{Timeout: 20 * time.Second})
	queue := newRefreshQueue(store, client, QueueConfig{
		RefreshRatePerSecond:  cfg.RefreshRatePerSecond,
		UpstreamRequestWeight: 24,
		StatsLogInterval:      cfg.StatsLogInterval,
		Priority:              priorityCfg,
	})

	return &Service{
		cfg:       cfg,
		store:     store,
		collector: collector,
		queue:     queue,
	}, nil
}

func (s *Service) RefreshWallet(ctx context.Context, address string) (WalletState, error) {
	return s.queue.refreshWallet(ctx, address)
}

func (s *Service) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.close()
}

func (s *Service) RunCollectors(ctx context.Context) error {
	errs := make(chan error, 3)

	go func() {
		errs <- s.collector.runLeaderboardPoller(ctx, s.cfg.LeaderboardURL, s.cfg.LeaderboardInterval)
	}()
	go func() {
		errs <- s.queue.run(ctx)
	}()
	go func() {
		errs <- s.runDistributionPoller(ctx)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errs:
		return err
	}
}

func (s *Service) runDistributionPoller(ctx context.Context) error {
	refresh := func() {
		if err := s.store.RefreshDistributionSnapshot(ctx); err != nil {
			slog.Warn("distribution snapshot refresh failed", "error", err)
			return
		}
		slog.Info("distribution snapshot refreshed")
	}

	refresh()
	ticker := time.NewTicker(s.cfg.DistributionInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			refresh()
		}
	}
}
