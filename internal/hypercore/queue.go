package hypercore

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"
)

type QueueConfig struct {
	RefreshRatePerSecond float64
	IdleSleep            time.Duration
	RequestTimeout       time.Duration
	StatsLogInterval     time.Duration
	Priority             PriorityConfig
}

type RefreshQueue struct {
	store       *Store
	client      WalletStateClient
	cfg         QueueConfig
	stats       RefreshQueueStats
	workerID    string
	claimNumber uint64
	leaseNumber uint64
}

type RefreshQueueStats struct {
	WalletRefreshed   int
	CandidatePromoted int
	CandidateRejected int
	RefreshFailed     int
	IdleTicks         int
}

func NewRefreshQueue(store *Store, client WalletStateClient, cfg QueueConfig) *RefreshQueue {
	if cfg.RefreshRatePerSecond <= 0 {
		cfg.RefreshRatePerSecond = 7
	}
	if cfg.IdleSleep <= 0 {
		cfg.IdleSleep = time.Second
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 15 * time.Second
	}
	if cfg.StatsLogInterval <= 0 {
		cfg.StatsLogInterval = time.Minute
	}
	if cfg.Priority.WhaleThresholdUSD == 0 {
		cfg.Priority.WhaleThresholdUSD = store.cfg.WhaleThresholdUSD
	}
	if cfg.Priority.RejectedCandidateTTL == 0 {
		cfg.Priority.RejectedCandidateTTL = store.cfg.RejectedCandidateTTL
	}
	if cfg.Priority.Now == nil {
		cfg.Priority.Now = store.cfg.Now
	}
	return &RefreshQueue{
		store:    store,
		client:   client,
		cfg:      cfg,
		workerID: fmt.Sprintf("%d-%d", os.Getpid(), queueWorkerSequence.Add(1)),
	}
}

var queueWorkerSequence atomic.Uint64

func (q *RefreshQueue) Run(ctx context.Context) error {
	interval := time.Duration(float64(time.Second) / q.cfg.RefreshRatePerSecond)
	if interval <= 0 {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	statsTicker := time.NewTicker(q.cfg.StatsLogInterval)
	defer statsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			q.logAndResetStats()
			return ctx.Err()
		case <-statsTicker.C:
			q.logAndResetStats()
		case <-ticker.C:
			refreshed, err := q.RefreshOne(ctx)
			if err != nil {
				q.stats.RefreshFailed++
				slog.Warn("wallet refresh failed", "error", err)
			}
			if !refreshed {
				q.stats.IdleTicks++
				select {
				case <-ctx.Done():
					q.logAndResetStats()
					return ctx.Err()
				case <-time.After(q.cfg.IdleSleep):
				}
			}
		}
	}
}

func (q *RefreshQueue) RefreshOne(ctx context.Context) (bool, error) {
	claimNumber := atomic.AddUint64(&q.claimNumber, 1)
	coverage := claimNumber%5 == 0
	leaseID := q.nextLeaseID()
	job, found, err := q.store.ClaimNextJob(leaseID, coverage, q.cfg.RequestTimeout+5*time.Second)
	if err != nil || !found {
		return false, err
	}
	defer func() {
		if err := q.store.ReleaseJob(job.Address, leaseID); err != nil {
			slog.Warn("failed to release refresh lease", "address", job.Address, "error", err)
		}
	}()

	if job.Kind == jobCandidate {
		candidate, err := q.store.GetCandidate(job.Address)
		if err != nil {
			return true, err
		}
		return q.refreshCandidate(ctx, candidate)
	}

	requestCtx, cancel := context.WithTimeout(ctx, q.cfg.RequestTimeout)
	defer cancel()

	wallet, err := q.store.GetWallet(job.Address)
	if err != nil {
		return true, err
	}
	state, err := q.client.GetClearinghouseState(requestCtx, wallet.Address)
	if err != nil {
		wallet = ApplyRefreshFailure(wallet, err, q.cfg.Priority)
		if saveErr := q.store.SaveWallet(wallet); saveErr != nil {
			return true, saveErr
		}
		return true, err
	}

	wallet = ApplyRefreshSuccess(wallet, state, q.cfg.Priority)
	if err := q.store.SaveWalletState(wallet, state); err != nil {
		return true, err
	}
	q.stats.WalletRefreshed++

	return true, nil
}

func (q *RefreshQueue) refreshCandidate(ctx context.Context, candidate WalletCandidate) (bool, error) {
	requestCtx, cancel := context.WithTimeout(ctx, q.cfg.RequestTimeout)
	defer cancel()

	state, err := q.client.GetClearinghouseState(requestCtx, candidate.Address)
	if err != nil {
		candidate.ScanAttempts++
		candidate.ConsecutiveFailure++
		candidate.LastError = err.Error()
		candidate.NextScanAt = q.cfg.Priority.now().Add(time.Duration(1<<min(candidate.ConsecutiveFailure, 6)) * time.Minute)
		if saveErr := q.store.SaveCandidate(candidate); saveErr != nil {
			return true, saveErr
		}
		return true, err
	}

	if !IsTrackableState(state, q.cfg.Priority.whaleThreshold()) {
		if err := q.store.RejectCandidate(candidate, state.Account.AccountValue, "account_and_positions_below_threshold"); err != nil {
			return true, err
		}
		q.stats.CandidateRejected++
		return true, nil
	}

	wallet := MergeWalletSignal(Wallet{}, CandidateWalletSignal(candidate), q.cfg.Priority)
	wallet = ApplyRefreshSuccess(wallet, state, q.cfg.Priority)
	if err := q.store.SaveWalletState(wallet, state); err != nil {
		return true, err
	}
	q.stats.CandidatePromoted++

	slog.Info("new wallet added to DB",
		"address", wallet.Address,
		"tier", wallet.Tier,
		"positions", len(state.Positions),
		"account_value", wallet.AccountValue,
		"max_position_value_usd", wallet.MaxPositionValueUSD,
		"next_refresh_at", wallet.NextRefreshAt,
	)
	return true, nil
}

func (q *RefreshQueue) logAndResetStats() {
	stats := q.stats
	if stats == (RefreshQueueStats{}) {
		return
	}
	slog.Info("wallet refresh stats",
		"wallet_refreshed", stats.WalletRefreshed,
		"candidate_promoted", stats.CandidatePromoted,
		"candidate_rejected", stats.CandidateRejected,
		"refresh_failed", stats.RefreshFailed,
		"idle_ticks", stats.IdleTicks,
	)
	q.stats = RefreshQueueStats{}
}

func (q *RefreshQueue) RefreshWallet(ctx context.Context, address string) (WalletState, error) {
	address = NormalizeAddress(address)
	cached, cachedErr := q.store.GetWalletState(address)
	if cachedErr == nil && q.cfg.Priority.now().Sub(cached.Account.RefreshedAt) < OnDemandFreshness {
		return cached, nil
	}
	if cachedErr != nil && cachedErr != ErrWalletNotFound {
		return WalletState{}, cachedErr
	}

	wallet, err := q.store.UpsertWalletSignal(WalletSignal{
		Address: address,
		Source:  SourceManual,
		SeenAt:  q.cfg.Priority.now(),
	})
	if err != nil {
		return WalletState{}, err
	}

	leaseID := q.nextLeaseID()
	claimed, err := q.store.ClaimOnDemand(address, leaseID, q.cfg.RequestTimeout+5*time.Second)
	if err != nil {
		return WalletState{}, err
	}
	if !claimed {
		waitCtx, cancel := context.WithTimeout(ctx, q.cfg.RequestTimeout+time.Second)
		defer cancel()
		return q.waitForWalletRefresh(waitCtx, address, cached, cachedErr == nil)
	}
	defer func() {
		if err := q.store.ReleaseJob(address, leaseID); err != nil {
			slog.Warn("failed to release on-demand lease", "address", address, "error", err)
		}
	}()

	requestCtx, cancel := context.WithTimeout(ctx, q.cfg.RequestTimeout)
	defer cancel()

	state, err := q.client.GetClearinghouseState(requestCtx, address)
	if err != nil {
		wallet = ApplyRefreshFailure(wallet, err, q.cfg.Priority)
		if saveErr := q.store.SaveWallet(wallet); saveErr != nil {
			return WalletState{}, saveErr
		}
		return WalletState{}, err
	}

	wallet = ApplyRefreshSuccess(wallet, state, q.cfg.Priority)
	if err := q.store.SaveWalletState(wallet, state); err != nil {
		return WalletState{}, err
	}

	return state, nil
}

func (q *RefreshQueue) nextLeaseID() string {
	return fmt.Sprintf("%s-%d", q.workerID, atomic.AddUint64(&q.leaseNumber, 1))
}

func (q *RefreshQueue) waitForWalletRefresh(ctx context.Context, address string, previous WalletState, hadPrevious bool) (WalletState, error) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return WalletState{}, ctx.Err()
		case <-ticker.C:
			state, err := q.store.GetWalletState(address)
			if err != nil {
				if err == ErrWalletNotFound {
					continue
				}
				return WalletState{}, err
			}
			if !hadPrevious || state.Account.RefreshedAt.After(previous.Account.RefreshedAt) {
				return state, nil
			}
		}
	}
}
