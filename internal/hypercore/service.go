package hypercore

import (
	"context"
	"database/sql"
	"net/http"
	"time"
)

type Config struct {
	DB                   *sql.DB
	LeaderboardURL       string
	LeaderboardInterval  time.Duration
	RefreshRatePerSecond float64
	StatsLogInterval     time.Duration
	WhaleThresholdUSD    float64
	RejectedCandidateTTL time.Duration
}

type Service struct {
	cfg       Config
	store     *Store
	collector *Collector
	queue     *RefreshQueue
}

func New(cfg Config) *Service {
	return &Service{
		cfg: cfg,
	}
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

	priorityCfg := PriorityConfig{
		WhaleThresholdUSD:    cfg.WhaleThresholdUSD,
		RejectedCandidateTTL: cfg.RejectedCandidateTTL,
	}

	store := OpenStore(cfg.DB, priorityCfg)

	client := NewHyperliquidClient("", nil)
	collector := NewCollector(store, &http.Client{Timeout: 20 * time.Second})
	queue := NewRefreshQueue(store, client, QueueConfig{
		RefreshRatePerSecond: cfg.RefreshRatePerSecond,
		StatsLogInterval:     cfg.StatsLogInterval,
		Priority:             priorityCfg,
	})

	return &Service{
		cfg:       cfg,
		store:     store,
		collector: collector,
		queue:     queue,
	}, nil
}

func (s *Service) Store() *Store {
	return s.store
}

func (s *Service) Collector() *Collector {
	return s.collector
}

func (s *Service) Queue() *RefreshQueue {
	return s.queue
}

func (s *Service) RefreshWallet(ctx context.Context, address string) (WalletState, error) {
	return s.queue.RefreshWallet(ctx, address)
}

func (s *Service) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Close()
}

func (s *Service) RunCollectors(ctx context.Context) error {
	errs := make(chan error, 2)

	go func() {
		errs <- s.collector.RunLeaderboardPoller(ctx, s.cfg.LeaderboardURL, s.cfg.LeaderboardInterval)
	}()
	go func() {
		errs <- s.queue.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errs:
		return err
	}
}
