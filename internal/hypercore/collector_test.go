package hypercore

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCollectLeaderboardStoresOnlyCandidatesAtOrAboveWhaleThreshold(t *testing.T) {
	store := openTestStore(t, PriorityConfig{WhaleThresholdUSD: 1_000_000})

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `[
			{"address":"0x0000000000000000000000000000000000000001","accountValue":1500000},
			{"address":"0x0000000000000000000000000000000000000002","accountValue":1000000},
			{"address":"0x0000000000000000000000000000000000000003","accountValue":999999},
			{"address":"0x0000000000000000000000000000000000000004"}
		]`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})}

	collector := NewCollector(store, client)
	stats, err := collector.CollectLeaderboard(context.Background(), "https://example.test/leaderboard")
	if err != nil {
		t.Fatal(err)
	}
	if stats.EligibleAddresses != 2 {
		t.Fatalf("expected 2 eligible addresses, got %d", stats.EligibleAddresses)
	}
	if stats.NewPotentialAddresses != 2 {
		t.Fatalf("expected 2 new potential addresses, got %d", stats.NewPotentialAddresses)
	}

	wallets, err := store.ListWallets()
	if err != nil {
		t.Fatal(err)
	}
	if len(wallets) != 0 {
		t.Fatalf("expected no stored wallets before validation scan, got %d", len(wallets))
	}

	candidates, err := store.ListCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 stored candidates, got %d", len(candidates))
	}

	stored := make(map[string]WalletCandidate, len(candidates))
	for _, candidate := range candidates {
		stored[candidate.Address] = candidate
	}

	if stored["0x0000000000000000000000000000000000000001"].LeaderboardAccountValue != 1_500_000 {
		t.Fatalf("expected candidate above threshold to be stored")
	}
	if stored["0x0000000000000000000000000000000000000002"].LeaderboardAccountValue != 1_000_000 {
		t.Fatalf("expected candidate at threshold to be stored")
	}
	if _, ok := stored["0x0000000000000000000000000000000000000003"]; ok {
		t.Fatalf("expected candidate below threshold to be ignored")
	}
	if _, ok := stored["0x0000000000000000000000000000000000000004"]; ok {
		t.Fatalf("expected candidate without account value to be ignored")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func NewCollector(store *Store, httpClient *http.Client) *collector {
	return newCollector(store, httpClient)
}

func (c *collector) CollectLeaderboard(ctx context.Context, url string) (LeaderboardCollectStats, error) {
	return c.collectLeaderboard(ctx, url)
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

func (s *Store) SaveWallet(wallet Wallet) error {
	return s.saveWallet(wallet)
}

func (s *Store) UpsertWalletSignal(signal WalletSignal) (Wallet, error) {
	return s.upsertWalletSignal(signal)
}

func (s *Store) UpsertLeaderboardCandidate(signal WalletSignal) (WalletCandidate, bool, bool, bool, error) {
	return s.upsertLeaderboardCandidate(signal)
}

func (s *Store) ClaimNextJob(workerID string, coverage bool, lease time.Duration) (RefreshJob, bool, error) {
	return s.claimNextJob(workerID, coverage, lease)
}

func (s *Store) ClaimOnDemand(address, workerID string, lease time.Duration) (bool, error) {
	return s.claimOnDemand(address, workerID, lease)
}

func (s *Store) ReleaseJob(address, workerID string) error {
	return s.releaseJob(address, workerID)
}
