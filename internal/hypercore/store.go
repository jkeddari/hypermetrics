package hypercore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrWalletNotFound   = errors.New("wallet not found")
	ErrStoreUnavailable = errors.New("postgres store unavailable")
)

const (
	jobCandidate int16 = iota
	jobWallet

	candidatePending int16 = iota
	candidateRejected
)

type Store struct {
	db  *sql.DB
	cfg PriorityConfig
}

type RefreshJob struct {
	Address           string
	Kind              int16
	NextRefreshAt     time.Time
	RefreshDeadlineAt time.Time
	PriorityScore     float64
	LastRefreshedAt   time.Time
	LockedUntil       time.Time
	LockedBy          string
}

func OpenStore(db *sql.DB, cfg PriorityConfig) *Store {
	return &Store{db: db, cfg: cfg}
}

func (s *Store) Close() error {
	return nil
}

func (s *Store) UpsertWalletSignal(signal WalletSignal) (Wallet, error) {
	if s == nil || s.db == nil {
		return Wallet{}, ErrStoreUnavailable
	}
	signal.Address = NormalizeAddress(signal.Address)
	if !IsAddress(signal.Address) {
		return Wallet{}, fmt.Errorf("invalid wallet address: %q", signal.Address)
	}

	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Wallet{}, err
	}
	defer tx.Rollback()
	if err := lockWallet(ctx, tx, signal.Address); err != nil {
		return Wallet{}, err
	}

	wallet, err := getWallet(ctx, tx, signal.Address)
	if err != nil && !errors.Is(err, ErrWalletNotFound) {
		return Wallet{}, err
	}
	wallet = MergeWalletSignal(wallet, signal, s.cfg)
	if err := upsertWallet(ctx, tx, wallet); err != nil {
		return Wallet{}, err
	}
	if err := upsertWalletSources(ctx, tx, wallet); err != nil {
		return Wallet{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM wallet_candidates WHERE address = $1`, wallet.Address); err != nil {
		return Wallet{}, err
	}
	if err := enqueueWallet(ctx, tx, wallet, s.cfg); err != nil {
		return Wallet{}, err
	}
	if err := tx.Commit(); err != nil {
		return Wallet{}, err
	}
	return wallet, nil
}

func (s *Store) UpsertLeaderboardCandidate(signal WalletSignal) (WalletCandidate, bool, bool, bool, error) {
	if s == nil || s.db == nil {
		return WalletCandidate{}, false, false, false, ErrStoreUnavailable
	}
	signal.Address = NormalizeAddress(signal.Address)
	if !IsAddress(signal.Address) {
		return WalletCandidate{}, false, false, false, fmt.Errorf("invalid wallet address: %q", signal.Address)
	}
	if signal.LeaderboardAccountValue < s.cfg.whaleThreshold() {
		return WalletCandidate{}, false, false, false, nil
	}

	now := signal.SeenAt
	if now.IsZero() {
		now = s.cfg.now()
	}
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WalletCandidate{}, false, false, false, err
	}
	defer tx.Rollback()
	if err := lockWallet(ctx, tx, signal.Address); err != nil {
		return WalletCandidate{}, false, false, false, err
	}

	wallet, err := getWallet(ctx, tx, signal.Address)
	if err == nil {
		wallet = MergeWalletSignal(wallet, signal, s.cfg)
		if err := upsertWallet(ctx, tx, wallet); err != nil {
			return WalletCandidate{}, false, false, false, err
		}
		if err := upsertWalletSources(ctx, tx, wallet); err != nil {
			return WalletCandidate{}, false, false, false, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM wallet_candidates WHERE address = $1`, signal.Address); err != nil {
			return WalletCandidate{}, false, false, false, err
		}
		if err := enqueueWallet(ctx, tx, wallet, s.cfg); err != nil {
			return WalletCandidate{}, false, false, false, err
		}
		if err := tx.Commit(); err != nil {
			return WalletCandidate{}, false, false, false, err
		}
		return candidateFromSignal(signal, now, s.cfg), false, true, false, nil
	}
	if !errors.Is(err, ErrWalletNotFound) {
		return WalletCandidate{}, false, false, false, err
	}

	candidate, status, rejectedUntil, err := getCandidate(ctx, tx, signal.Address)
	isNew := errors.Is(err, ErrWalletNotFound)
	if err != nil && !isNew {
		return WalletCandidate{}, false, false, false, err
	}
	if !isNew && status == candidateRejected && rejectedUntil.After(now) {
		candidate.LastSeenLeaderboardAt = now
		candidate.LeaderboardRank = signal.LeaderboardRank
		candidate.LeaderboardAccountValue = signal.LeaderboardAccountValue
		candidate.LeaderboardPNL = signal.LeaderboardPNL
		candidate.LeaderboardROI = signal.LeaderboardROI
		if err := upsertCandidate(ctx, tx, candidate, candidateRejected, rejectedUntil.Add(-s.cfg.rejectedCandidateTTL()), rejectedUntil, 0, ""); err != nil {
			return WalletCandidate{}, false, false, false, err
		}
		if err := tx.Commit(); err != nil {
			return WalletCandidate{}, false, false, false, err
		}
		return candidate, false, false, true, nil
	}

	if isNew || status == candidateRejected {
		candidate = candidateFromSignal(signal, now, s.cfg)
		isNew = true
	} else {
		candidate.LastSeenLeaderboardAt = now
		candidate.LeaderboardRank = signal.LeaderboardRank
		candidate.LeaderboardAccountValue = signal.LeaderboardAccountValue
		candidate.LeaderboardPNL = signal.LeaderboardPNL
		candidate.LeaderboardROI = signal.LeaderboardROI
		candidate.PriorityScore = ComputeCandidatePriorityScore(candidate, s.cfg)
	}
	if err := upsertCandidate(ctx, tx, candidate, candidatePending, time.Time{}, time.Time{}, 0, ""); err != nil {
		return WalletCandidate{}, false, false, false, err
	}
	if err := enqueueCandidate(ctx, tx, candidate); err != nil {
		return WalletCandidate{}, false, false, false, err
	}
	if err := tx.Commit(); err != nil {
		return WalletCandidate{}, false, false, false, err
	}
	return candidate, isNew, false, false, nil
}

func (s *Store) GetWallet(address string) (Wallet, error) {
	if s == nil || s.db == nil {
		return Wallet{}, ErrStoreUnavailable
	}
	return getWallet(context.Background(), s.db, NormalizeAddress(address))
}

func (s *Store) GetCandidate(address string) (WalletCandidate, error) {
	if s == nil || s.db == nil {
		return WalletCandidate{}, ErrStoreUnavailable
	}
	candidate, status, _, err := getCandidate(context.Background(), s.db, NormalizeAddress(address))
	if err != nil {
		return WalletCandidate{}, err
	}
	if status != candidatePending {
		return WalletCandidate{}, ErrWalletNotFound
	}
	return candidate, nil
}

func (s *Store) SaveWallet(wallet Wallet) error {
	if s == nil || s.db == nil {
		return ErrStoreUnavailable
	}
	wallet.Address = NormalizeAddress(wallet.Address)
	if !IsAddress(wallet.Address) {
		return fmt.Errorf("invalid wallet address: %q", wallet.Address)
	}
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockWallet(ctx, tx, wallet.Address); err != nil {
		return err
	}
	if err := upsertWallet(ctx, tx, wallet); err != nil {
		return err
	}
	if err := upsertWalletSources(ctx, tx, wallet); err != nil {
		return err
	}
	if err := enqueueWallet(ctx, tx, wallet, s.cfg); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SaveWalletState(wallet Wallet, state WalletState) error {
	if s == nil || s.db == nil {
		return ErrStoreUnavailable
	}
	wallet.Address = NormalizeAddress(wallet.Address)
	if !IsAddress(wallet.Address) {
		return fmt.Errorf("invalid wallet address: %q", wallet.Address)
	}
	refreshedAt := state.Account.RefreshedAt
	if refreshedAt.IsZero() {
		refreshedAt = wallet.LastRefreshedAt
	}
	if refreshedAt.IsZero() {
		refreshedAt = s.cfg.now()
	}
	state.Account.Address = wallet.Address
	state.Account.RefreshedAt = refreshedAt
	if state.Account.RawReceivedAt.IsZero() {
		state.Account.RawReceivedAt = refreshedAt
	}
	for i := range state.Positions {
		state.Positions[i].Address = wallet.Address
		if state.Positions[i].RefreshedAt.IsZero() {
			state.Positions[i].RefreshedAt = refreshedAt
		}
	}
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockWallet(ctx, tx, wallet.Address); err != nil {
		return err
	}
	if candidate, status, _, err := getCandidate(ctx, tx, wallet.Address); err == nil && status == candidatePending {
		wallet = MergeWalletSignal(wallet, CandidateWalletSignal(candidate), s.cfg)
	} else if err != nil && !errors.Is(err, ErrWalletNotFound) {
		return err
	}

	if err := upsertWallet(ctx, tx, wallet); err != nil {
		return err
	}
	if err := upsertWalletSources(ctx, tx, wallet); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO wallet_accounts_current (
			wallet_address, account_value, margin_used, withdrawable, refreshed_at, source_received_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (wallet_address) DO UPDATE SET
			account_value = EXCLUDED.account_value,
			margin_used = EXCLUDED.margin_used,
			withdrawable = EXCLUDED.withdrawable,
			refreshed_at = EXCLUDED.refreshed_at,
			source_received_at = EXCLUDED.source_received_at`,
		state.Account.Address, state.Account.AccountValue, state.Account.MarginUsed,
		state.Account.Withdrawable, state.Account.RefreshedAt, state.Account.RawReceivedAt,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO wallet_account_history (
			wallet_address, account_value, margin_used, withdrawable, refreshed_at, source_received_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT DO NOTHING`,
		state.Account.Address, state.Account.AccountValue, state.Account.MarginUsed,
		state.Account.Withdrawable, state.Account.RefreshedAt, state.Account.RawReceivedAt,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM wallet_positions_current WHERE wallet_address = $1`, wallet.Address); err != nil {
		return err
	}
	for _, position := range state.Positions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO wallet_positions_current (
				wallet_address, symbol, position_size, entry_price, mark_price, liquidation_price,
				leverage, margin_balance, position_value_usd, unrealized_pnl, refreshed_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			wallet.Address, strings.ToUpper(position.Symbol), position.PositionSize, position.EntryPrice,
			position.MarkPrice, position.LiqPrice, position.Leverage, position.MarginBalance,
			position.PositionValueUSD, position.UnrealizedPnL, position.RefreshedAt,
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO wallet_position_history (
				wallet_address, symbol, position_size, entry_price, mark_price, liquidation_price,
				leverage, margin_balance, position_value_usd, unrealized_pnl, refreshed_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT DO NOTHING`,
			wallet.Address, strings.ToUpper(position.Symbol), position.PositionSize, position.EntryPrice,
			position.MarkPrice, position.LiqPrice, position.Leverage, position.MarginBalance,
			position.PositionValueUSD, position.UnrealizedPnL, position.RefreshedAt,
		); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM wallet_candidates WHERE address = $1`, wallet.Address); err != nil {
		return err
	}
	if err := enqueueWallet(ctx, tx, wallet, s.cfg); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RejectCandidate(candidate WalletCandidate, realAccountValue float64, reason string) error {
	if s == nil || s.db == nil {
		return ErrStoreUnavailable
	}
	candidate.Address = NormalizeAddress(candidate.Address)
	if !IsAddress(candidate.Address) {
		return fmt.Errorf("invalid wallet address: %q", candidate.Address)
	}
	now := s.cfg.now()
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockWallet(ctx, tx, candidate.Address); err != nil {
		return err
	}
	if stored, status, _, err := getCandidate(ctx, tx, candidate.Address); err == nil && status == candidatePending {
		candidate = mergeCandidateDiscovery(candidate, stored)
	} else if err != nil && !errors.Is(err, ErrWalletNotFound) {
		return err
	}
	if err := upsertCandidate(ctx, tx, candidate, candidateRejected, now, now.Add(s.cfg.rejectedCandidateTTL()), realAccountValue, reason); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM wallet_refresh_queue WHERE wallet_address = $1 AND job_kind = $2`, candidate.Address, jobCandidate); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SaveCandidate(candidate WalletCandidate) error {
	if s == nil || s.db == nil {
		return ErrStoreUnavailable
	}
	candidate.Address = NormalizeAddress(candidate.Address)
	if !IsAddress(candidate.Address) {
		return fmt.Errorf("invalid wallet address: %q", candidate.Address)
	}
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockWallet(ctx, tx, candidate.Address); err != nil {
		return err
	}
	if stored, status, _, err := getCandidate(ctx, tx, candidate.Address); err == nil && status == candidatePending {
		candidate = mergeCandidateDiscovery(candidate, stored)
	} else if err != nil && !errors.Is(err, ErrWalletNotFound) {
		return err
	}
	candidate.PriorityScore = ComputeCandidatePriorityScore(candidate, s.cfg)
	if err := upsertCandidate(ctx, tx, candidate, candidatePending, time.Time{}, time.Time{}, 0, ""); err != nil {
		return err
	}
	if err := enqueueCandidate(ctx, tx, candidate); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetWalletState(address string) (WalletState, error) {
	if s == nil || s.db == nil {
		return WalletState{}, ErrStoreUnavailable
	}
	address = NormalizeAddress(address)
	if !IsAddress(address) {
		return WalletState{}, fmt.Errorf("invalid wallet address: %q", address)
	}
	ctx := context.Background()
	var state WalletState
	err := s.db.QueryRowContext(ctx, `
		SELECT wallet_address, account_value, margin_used, withdrawable, refreshed_at, source_received_at
		FROM wallet_accounts_current WHERE wallet_address = $1`, address,
	).Scan(
		&state.Account.Address, &state.Account.AccountValue, &state.Account.MarginUsed,
		&state.Account.Withdrawable, &state.Account.RefreshedAt, &state.Account.RawReceivedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return WalletState{}, ErrWalletNotFound
	}
	if err != nil {
		return WalletState{}, err
	}
	positions, err := s.listPositions(ctx, `WHERE wallet_address = $1 ORDER BY symbol`, address)
	if err != nil {
		return WalletState{}, err
	}
	state.Positions = positions
	return state, nil
}

func (s *Store) ListPositionsBySymbol(symbol string, currentPage, pageSize int) ([]WalletPosition, error) {
	if pageSize <= 0 {
		pageSize = 100
	}
	if currentPage <= 0 {
		currentPage = 1
	}
	return s.listPositions(context.Background(), `
		WHERE symbol = $1
		ORDER BY position_value_usd DESC, wallet_address
		LIMIT $2 OFFSET $3`,
		strings.ToUpper(strings.TrimSpace(symbol)), pageSize, (currentPage-1)*pageSize,
	)
}

func (s *Store) ListWhalePositions(thresholdUSD float64) ([]WalletPosition, error) {
	if thresholdUSD <= 0 {
		thresholdUSD = 1_000_000
	}
	return s.listPositions(context.Background(), `
		WHERE position_value_usd >= $1
		ORDER BY position_value_usd DESC, wallet_address`, thresholdUSD,
	)
}

func (s *Store) ListWalletPositionDistribution() ([]PositionDistributionBucket, error) {
	if s == nil || s.db == nil {
		return nil, ErrStoreUnavailable
	}
	rows, err := s.db.Query(`
		WITH tiers (ordinal, group_name, minimum_amount, maximum_amount) AS (
			VALUES
				(1, 'shrimp',          0::float8,       250::float8),
				(2, 'fish',          250::float8,      2500::float8),
				(3, 'dolphin',      2500::float8,     25000::float8),
				(4, 'apex_predator',25000::float8,    100000::float8),
				(5, 'small_whale', 100000::float8,   1000000::float8),
				(6, 'whale',      1000000::float8,  10000000::float8),
				(7, 'tidal_whale',10000000::float8, 100000000::float8),
				(8, 'leviathan', 100000000::float8,         0::float8)
		),
		wallet_stats AS (
			SELECT
				w.address,
				GREATEST(w.account_value, 0) AS account_value,
				count(p.symbol) AS position_count,
				COALESCE(sum(CASE WHEN p.position_size > 0 THEN abs(p.position_value_usd) ELSE 0 END), 0) AS long_usd,
				COALESCE(sum(CASE WHEN p.position_size < 0 THEN abs(p.position_value_usd) ELSE 0 END), 0) AS short_usd,
				COALESCE(sum(p.unrealized_pnl), 0) AS unrealized_pnl,
				COALESCE(sum(CASE
					WHEN p.position_size > 0 THEN abs(p.position_value_usd)
					WHEN p.position_size < 0 THEN -abs(p.position_value_usd)
					ELSE 0
				END), 0) AS net_position_usd
			FROM wallets w
			LEFT JOIN wallet_positions_current p ON p.wallet_address = w.address
			GROUP BY w.address, w.account_value
		)
		SELECT
			t.group_name,
			t.minimum_amount,
			t.maximum_amount,
			count(ws.address),
			count(ws.address) FILTER (WHERE ws.position_count > 0),
			COALESCE(sum(ws.long_usd), 0),
			COALESCE(sum(ws.short_usd), 0),
			count(ws.address) FILTER (WHERE ws.position_count > 0 AND ws.unrealized_pnl >= 0),
			count(ws.address) FILTER (WHERE ws.position_count > 0 AND ws.unrealized_pnl < 0),
			count(ws.address) FILTER (WHERE ws.net_position_usd > 0),
			count(ws.address) FILTER (WHERE ws.net_position_usd < 0)
		FROM tiers t
		LEFT JOIN wallet_stats ws
			ON ws.account_value >= t.minimum_amount
			AND (t.maximum_amount = 0 OR ws.account_value < t.maximum_amount)
		GROUP BY t.ordinal, t.group_name, t.minimum_amount, t.maximum_amount
		ORDER BY t.ordinal`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	buckets := make([]PositionDistributionBucket, 0, 8)
	for rows.Next() {
		var bucket PositionDistributionBucket
		var longWallets, shortWallets int64
		if err := rows.Scan(
			&bucket.GroupName,
			&bucket.MinimumAmount,
			&bucket.MaximumAmount,
			&bucket.AllAddressCount,
			&bucket.PositionAddressCount,
			&bucket.LongPositionUSD,
			&bucket.ShortPositionUSD,
			&bucket.ProfitAddressCount,
			&bucket.LossAddressCount,
			&longWallets,
			&shortWallets,
		); err != nil {
			return nil, err
		}
		buckets = append(buckets, finalizePositionDistribution(bucket, longWallets, shortWallets))
	}
	return buckets, rows.Err()
}

func (s *Store) ListPositions() ([]WalletPosition, error) {
	return s.listPositions(context.Background(), `ORDER BY position_value_usd DESC, wallet_address`)
}

func (s *Store) ListCandidates() ([]WalletCandidate, error) {
	if s == nil || s.db == nil {
		return nil, ErrStoreUnavailable
	}
	rows, err := s.db.Query(`
		SELECT address, first_seen_at, last_seen_leaderboard_at, next_scan_at,
			scan_attempts, consecutive_failures, last_error, leaderboard_rank,
			leaderboard_account_value, leaderboard_pnl, leaderboard_roi, priority_score
		FROM wallet_candidates
		WHERE status = $1
		ORDER BY priority_score DESC, leaderboard_account_value DESC, address`, candidatePending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []WalletCandidate
	for rows.Next() {
		var candidate WalletCandidate
		if err := rows.Scan(
			&candidate.Address, &candidate.FirstSeenAt, &candidate.LastSeenLeaderboardAt, &candidate.NextScanAt,
			&candidate.ScanAttempts, &candidate.ConsecutiveFailure, &candidate.LastError, &candidate.LeaderboardRank,
			&candidate.LeaderboardAccountValue, &candidate.LeaderboardPNL, &candidate.LeaderboardROI, &candidate.PriorityScore,
		); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

func (s *Store) ListWallets() ([]Wallet, error) {
	return s.listWallets(context.Background(), "")
}

func (s *Store) ListWalletsPage(currentPage, pageSize int) ([]Wallet, error) {
	if pageSize <= 0 {
		pageSize = 100
	}
	if currentPage <= 0 {
		currentPage = 1
	}
	return s.listWallets(context.Background(), `LIMIT $1 OFFSET $2`, pageSize, (currentPage-1)*pageSize)
}

func (s *Store) CountWallets() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT count(*) FROM wallets`).Scan(&count)
	return count, err
}

func (s *Store) ClaimNextJob(workerID string, coverage bool, lease time.Duration) (RefreshJob, bool, error) {
	if s == nil || s.db == nil {
		return RefreshJob{}, false, ErrStoreUnavailable
	}
	now := s.cfg.now()
	order := "priority_score DESC, next_refresh_at, created_at"
	if coverage {
		order = "refresh_deadline_at, last_refreshed_at ASC NULLS FIRST, priority_score DESC"
	}
	query := fmt.Sprintf(`
		WITH next_job AS (
			SELECT wallet_address
			FROM wallet_refresh_queue
			WHERE next_refresh_at <= $1
			  AND (locked_until IS NULL OR locked_until <= $1)
			ORDER BY %s
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE wallet_refresh_queue q
		SET locked_until = $2, locked_by = $3, attempts = attempts + 1, updated_at = $1
		FROM next_job
		WHERE q.wallet_address = next_job.wallet_address
		RETURNING q.wallet_address, q.job_kind, q.next_refresh_at, q.refresh_deadline_at,
			q.priority_score, q.last_refreshed_at, q.locked_until, q.locked_by`, order)
	var job RefreshJob
	var lastRefreshed sql.NullTime
	err := s.db.QueryRow(query, now, now.Add(lease), workerID).Scan(
		&job.Address, &job.Kind, &job.NextRefreshAt, &job.RefreshDeadlineAt,
		&job.PriorityScore, &lastRefreshed, &job.LockedUntil, &job.LockedBy,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RefreshJob{}, false, nil
	}
	if err != nil {
		return RefreshJob{}, false, err
	}
	if lastRefreshed.Valid {
		job.LastRefreshedAt = lastRefreshed.Time
	}
	return job, true, nil
}

func (s *Store) ClaimOnDemand(address, workerID string, lease time.Duration) (bool, error) {
	now := s.cfg.now()
	result, err := s.db.Exec(`
		UPDATE wallet_refresh_queue
		SET locked_until = $1, locked_by = $2, updated_at = $3
		WHERE wallet_address = $4
		  AND (locked_until IS NULL OR locked_until <= $3)`,
		now.Add(lease), workerID, now, NormalizeAddress(address),
	)
	if err != nil {
		return false, err
	}
	claimed, err := result.RowsAffected()
	return claimed == 1, err
}

func (s *Store) ReleaseJob(address, workerID string) error {
	_, err := s.db.Exec(`
		UPDATE wallet_refresh_queue
		SET locked_until = NULL, locked_by = NULL, updated_at = $1
		WHERE wallet_address = $2 AND locked_by = $3`,
		s.cfg.now(), NormalizeAddress(address), workerID,
	)
	return err
}

func (s *Store) listPositions(ctx context.Context, suffix string, args ...any) ([]WalletPosition, error) {
	if s == nil || s.db == nil {
		return nil, ErrStoreUnavailable
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT wallet_address, symbol, position_size, entry_price, mark_price, liquidation_price,
			leverage, margin_balance, position_value_usd, unrealized_pnl, refreshed_at
		FROM wallet_positions_current `+suffix, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	positions := make([]WalletPosition, 0)
	for rows.Next() {
		var position WalletPosition
		if err := rows.Scan(
			&position.Address, &position.Symbol, &position.PositionSize, &position.EntryPrice,
			&position.MarkPrice, &position.LiqPrice, &position.Leverage, &position.MarginBalance,
			&position.PositionValueUSD, &position.UnrealizedPnL, &position.RefreshedAt,
		); err != nil {
			return nil, err
		}
		positions = append(positions, position)
	}
	return positions, rows.Err()
}

func (s *Store) listWallets(ctx context.Context, suffix string, args ...any) ([]Wallet, error) {
	rows, err := s.db.QueryContext(ctx, walletSelect+`
		ORDER BY w.tier, w.priority_score DESC, w.last_refreshed_at DESC NULLS LAST, w.address `+suffix, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	wallets := make([]Wallet, 0)
	for rows.Next() {
		wallet, err := scanWallet(rows)
		if err != nil {
			return nil, err
		}
		wallets = append(wallets, wallet)
	}
	return wallets, rows.Err()
}

const walletSelect = `
	SELECT w.address,
		COALESCE((SELECT string_agg(ws.source, ',' ORDER BY ws.source)
			FROM wallet_sources ws WHERE ws.wallet_address = w.address), ''),
		w.first_seen_at, w.last_seen_leaderboard_at, w.last_refreshed_at,
		w.last_successful_refresh_at, w.last_failed_refresh_at,
		w.refresh_attempts, w.consecutive_failures, w.last_error,
		w.known_whale, w.has_open_position, w.max_position_value_usd,
		w.total_position_value_usd, w.account_value, w.leaderboard_rank,
		w.leaderboard_account_value, w.leaderboard_pnl, w.leaderboard_roi,
		w.manual_priority_boost, w.next_refresh_at, w.priority_score, w.tier
	FROM wallets w `

type scanner interface {
	Scan(dest ...any) error
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func lockWallet(ctx context.Context, tx *sql.Tx, address string) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, address)
	return err
}

func getWallet(ctx context.Context, q queryer, address string) (Wallet, error) {
	if !IsAddress(address) {
		return Wallet{}, ErrWalletNotFound
	}
	wallet, err := scanWallet(q.QueryRowContext(ctx, walletSelect+` WHERE w.address = $1`, address))
	if errors.Is(err, sql.ErrNoRows) {
		return Wallet{}, ErrWalletNotFound
	}
	return wallet, err
}

func scanWallet(row scanner) (Wallet, error) {
	var wallet Wallet
	var sources string
	var lastLeaderboard, lastRefreshed, lastSuccess, lastFailure, nextRefresh sql.NullTime
	err := row.Scan(
		&wallet.Address, &sources, &wallet.FirstSeenAt, &lastLeaderboard, &lastRefreshed,
		&lastSuccess, &lastFailure, &wallet.RefreshAttempts, &wallet.ConsecutiveFailure,
		&wallet.LastError, &wallet.KnownWhale, &wallet.HasOpenPosition,
		&wallet.MaxPositionValueUSD, &wallet.TotalPositionValueUSD, &wallet.AccountValue,
		&wallet.LeaderboardRank, &wallet.LeaderboardAccountValue, &wallet.LeaderboardPNL,
		&wallet.LeaderboardROI, &wallet.ManualPriorityBoost, &nextRefresh,
		&wallet.PriorityScore, &wallet.Tier,
	)
	if err != nil {
		return Wallet{}, err
	}
	if sources != "" {
		wallet.Sources = strings.Split(sources, ",")
	}
	wallet.LastSeenLeaderboardAt = nullTime(lastLeaderboard)
	wallet.LastRefreshedAt = nullTime(lastRefreshed)
	wallet.LastSuccessfulRefresh = nullTime(lastSuccess)
	wallet.LastFailedRefreshAt = nullTime(lastFailure)
	wallet.NextRefreshAt = nullTime(nextRefresh)
	return wallet, nil
}

func upsertWallet(ctx context.Context, tx *sql.Tx, wallet Wallet) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO wallets (
			address, first_seen_at, last_seen_leaderboard_at, last_refreshed_at,
			last_successful_refresh_at, last_failed_refresh_at, refresh_attempts,
			consecutive_failures, last_error, known_whale, has_open_position,
			max_position_value_usd, total_position_value_usd, account_value,
			leaderboard_rank, leaderboard_account_value, leaderboard_pnl,
			leaderboard_roi, manual_priority_boost, next_refresh_at, priority_score, tier
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19, $20, $21, $22
		)
		ON CONFLICT (address) DO UPDATE SET
			first_seen_at = EXCLUDED.first_seen_at,
			last_seen_leaderboard_at = EXCLUDED.last_seen_leaderboard_at,
			last_refreshed_at = EXCLUDED.last_refreshed_at,
			last_successful_refresh_at = EXCLUDED.last_successful_refresh_at,
			last_failed_refresh_at = EXCLUDED.last_failed_refresh_at,
			refresh_attempts = EXCLUDED.refresh_attempts,
			consecutive_failures = EXCLUDED.consecutive_failures,
			last_error = EXCLUDED.last_error,
			known_whale = EXCLUDED.known_whale,
			has_open_position = EXCLUDED.has_open_position,
			max_position_value_usd = EXCLUDED.max_position_value_usd,
			total_position_value_usd = EXCLUDED.total_position_value_usd,
			account_value = EXCLUDED.account_value,
			leaderboard_rank = EXCLUDED.leaderboard_rank,
			leaderboard_account_value = EXCLUDED.leaderboard_account_value,
			leaderboard_pnl = EXCLUDED.leaderboard_pnl,
			leaderboard_roi = EXCLUDED.leaderboard_roi,
			manual_priority_boost = EXCLUDED.manual_priority_boost,
			next_refresh_at = EXCLUDED.next_refresh_at,
			priority_score = EXCLUDED.priority_score,
			tier = EXCLUDED.tier,
			updated_at = now()`,
		wallet.Address, wallet.FirstSeenAt, nullableTime(wallet.LastSeenLeaderboardAt),
		nullableTime(wallet.LastRefreshedAt), nullableTime(wallet.LastSuccessfulRefresh),
		nullableTime(wallet.LastFailedRefreshAt), wallet.RefreshAttempts, wallet.ConsecutiveFailure,
		wallet.LastError, wallet.KnownWhale, wallet.HasOpenPosition, wallet.MaxPositionValueUSD,
		wallet.TotalPositionValueUSD, wallet.AccountValue, wallet.LeaderboardRank,
		wallet.LeaderboardAccountValue, wallet.LeaderboardPNL, wallet.LeaderboardROI,
		wallet.ManualPriorityBoost, nullableTime(wallet.NextRefreshAt), wallet.PriorityScore, wallet.Tier,
	)
	return err
}

func upsertWalletSources(ctx context.Context, tx *sql.Tx, wallet Wallet) error {
	for _, source := range wallet.Sources {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO wallet_sources (wallet_address, source, first_seen_at, last_seen_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (wallet_address, source) DO UPDATE SET last_seen_at = EXCLUDED.last_seen_at`,
			wallet.Address, source, wallet.FirstSeenAt, sourceSeenAt(wallet, source),
		); err != nil {
			return err
		}
	}
	return nil
}

func sourceSeenAt(wallet Wallet, source string) time.Time {
	if source == SourceLeaderboard && !wallet.LastSeenLeaderboardAt.IsZero() {
		return wallet.LastSeenLeaderboardAt
	}
	if !wallet.LastRefreshedAt.IsZero() {
		return wallet.LastRefreshedAt
	}
	return wallet.FirstSeenAt
}

func mergeCandidateDiscovery(candidate, latest WalletCandidate) WalletCandidate {
	if !latest.LastSeenLeaderboardAt.After(candidate.LastSeenLeaderboardAt) {
		return candidate
	}
	candidate.LastSeenLeaderboardAt = latest.LastSeenLeaderboardAt
	candidate.LeaderboardRank = latest.LeaderboardRank
	candidate.LeaderboardAccountValue = latest.LeaderboardAccountValue
	candidate.LeaderboardPNL = latest.LeaderboardPNL
	candidate.LeaderboardROI = latest.LeaderboardROI
	return candidate
}

func enqueueWallet(ctx context.Context, tx *sql.Tx, wallet Wallet, cfg PriorityConfig) error {
	next := wallet.NextRefreshAt
	if next.IsZero() {
		next = cfg.now()
	}
	deadline := next.Add(MaxRefreshInterval(wallet, cfg))
	_, err := tx.ExecContext(ctx, `
		INSERT INTO wallet_refresh_queue (
			wallet_address, job_kind, next_refresh_at, refresh_deadline_at,
			priority_score, last_refreshed_at, consecutive_failures, last_error
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (wallet_address) DO UPDATE SET
			job_kind = EXCLUDED.job_kind,
			next_refresh_at = CASE
				WHEN EXCLUDED.last_refreshed_at IS DISTINCT FROM wallet_refresh_queue.last_refreshed_at
				THEN EXCLUDED.next_refresh_at
				ELSE LEAST(wallet_refresh_queue.next_refresh_at, EXCLUDED.next_refresh_at)
			END,
			refresh_deadline_at = CASE
				WHEN EXCLUDED.last_refreshed_at IS DISTINCT FROM wallet_refresh_queue.last_refreshed_at
				THEN EXCLUDED.refresh_deadline_at
				ELSE LEAST(wallet_refresh_queue.refresh_deadline_at, EXCLUDED.refresh_deadline_at)
			END,
			priority_score = EXCLUDED.priority_score,
			last_refreshed_at = EXCLUDED.last_refreshed_at,
			consecutive_failures = EXCLUDED.consecutive_failures,
			last_error = EXCLUDED.last_error,
			updated_at = now()`,
		wallet.Address, jobWallet, next, deadline, wallet.PriorityScore,
		nullableTime(wallet.LastRefreshedAt), wallet.ConsecutiveFailure, wallet.LastError,
	)
	return err
}

func enqueueCandidate(ctx context.Context, tx *sql.Tx, candidate WalletCandidate) error {
	next := candidate.NextScanAt
	if next.IsZero() {
		next = candidate.FirstSeenAt
	}
	deadline := candidate.FirstSeenAt.Add(24 * time.Hour)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO wallet_refresh_queue (
			wallet_address, job_kind, next_refresh_at, refresh_deadline_at,
			priority_score, last_refreshed_at, consecutive_failures, last_error
		) VALUES ($1, $2, $3, $4, $5, NULL, $6, $7)
		ON CONFLICT (wallet_address) DO UPDATE SET
			job_kind = EXCLUDED.job_kind,
			next_refresh_at = CASE
				WHEN EXCLUDED.consecutive_failures > wallet_refresh_queue.consecutive_failures
				THEN EXCLUDED.next_refresh_at
				ELSE LEAST(wallet_refresh_queue.next_refresh_at, EXCLUDED.next_refresh_at)
			END,
			refresh_deadline_at = LEAST(wallet_refresh_queue.refresh_deadline_at, EXCLUDED.refresh_deadline_at),
			priority_score = EXCLUDED.priority_score,
			consecutive_failures = EXCLUDED.consecutive_failures,
			last_error = EXCLUDED.last_error,
			updated_at = now()`,
		candidate.Address, jobCandidate, next, deadline, candidate.PriorityScore,
		candidate.ConsecutiveFailure, candidate.LastError,
	)
	return err
}

func getCandidate(ctx context.Context, q queryer, address string) (WalletCandidate, int16, time.Time, error) {
	var candidate WalletCandidate
	var status int16
	var rejectedUntil sql.NullTime
	err := q.QueryRowContext(ctx, `
		SELECT address, status, first_seen_at, last_seen_leaderboard_at, next_scan_at,
			scan_attempts, consecutive_failures, last_error, leaderboard_rank,
			leaderboard_account_value, leaderboard_pnl, leaderboard_roi, priority_score,
			rejected_until
		FROM wallet_candidates WHERE address = $1`, address,
	).Scan(
		&candidate.Address, &status, &candidate.FirstSeenAt, &candidate.LastSeenLeaderboardAt,
		&candidate.NextScanAt, &candidate.ScanAttempts, &candidate.ConsecutiveFailure,
		&candidate.LastError, &candidate.LeaderboardRank, &candidate.LeaderboardAccountValue,
		&candidate.LeaderboardPNL, &candidate.LeaderboardROI, &candidate.PriorityScore, &rejectedUntil,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return WalletCandidate{}, 0, time.Time{}, ErrWalletNotFound
	}
	return candidate, status, nullTime(rejectedUntil), err
}

func upsertCandidate(
	ctx context.Context,
	tx *sql.Tx,
	candidate WalletCandidate,
	status int16,
	rejectedAt time.Time,
	rejectedUntil time.Time,
	realAccountValue float64,
	reason string,
) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO wallet_candidates (
			address, status, first_seen_at, last_seen_leaderboard_at, next_scan_at,
			scan_attempts, consecutive_failures, last_error, leaderboard_rank,
			leaderboard_account_value, leaderboard_pnl, leaderboard_roi, priority_score,
			rejected_at, rejected_until, real_account_value, rejection_reason
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (address) DO UPDATE SET
			status = EXCLUDED.status,
			first_seen_at = EXCLUDED.first_seen_at,
			last_seen_leaderboard_at = EXCLUDED.last_seen_leaderboard_at,
			next_scan_at = EXCLUDED.next_scan_at,
			scan_attempts = EXCLUDED.scan_attempts,
			consecutive_failures = EXCLUDED.consecutive_failures,
			last_error = EXCLUDED.last_error,
			leaderboard_rank = EXCLUDED.leaderboard_rank,
			leaderboard_account_value = EXCLUDED.leaderboard_account_value,
			leaderboard_pnl = EXCLUDED.leaderboard_pnl,
			leaderboard_roi = EXCLUDED.leaderboard_roi,
			priority_score = EXCLUDED.priority_score,
			rejected_at = EXCLUDED.rejected_at,
			rejected_until = EXCLUDED.rejected_until,
			real_account_value = EXCLUDED.real_account_value,
			rejection_reason = EXCLUDED.rejection_reason,
			updated_at = now()`,
		candidate.Address, status, candidate.FirstSeenAt, candidate.LastSeenLeaderboardAt,
		nullableTime(candidate.NextScanAt), candidate.ScanAttempts, candidate.ConsecutiveFailure,
		candidate.LastError, candidate.LeaderboardRank, candidate.LeaderboardAccountValue,
		candidate.LeaderboardPNL, candidate.LeaderboardROI, candidate.PriorityScore,
		nullableTime(rejectedAt), nullableTime(rejectedUntil), realAccountValue, reason,
	)
	return err
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func nullTime(value sql.NullTime) time.Time {
	if value.Valid {
		return value.Time
	}
	return time.Time{}
}
