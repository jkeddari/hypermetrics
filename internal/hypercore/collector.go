package hypercore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"time"
)

const defaultLeaderboardURL = "https://stats-data.hyperliquid.xyz/Mainnet/leaderboard"

var addressPattern = regexp.MustCompile(`(?i)^0x[0-9a-f]{40}$`)

type collector struct {
	store      *Store
	httpClient *http.Client
	cfg        PriorityConfig
}

func newCollector(store *Store, httpClient *http.Client) *collector {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	var cfg PriorityConfig
	if store != nil {
		cfg = store.cfg
	}
	return &collector{
		store:      store,
		httpClient: httpClient,
		cfg:        cfg,
	}
}

func (c *collector) collectLeaderboard(ctx context.Context, url string) (LeaderboardCollectStats, error) {
	if url == "" {
		url = defaultLeaderboardURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return LeaderboardCollectStats{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return LeaderboardCollectStats{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return LeaderboardCollectStats{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return LeaderboardCollectStats{}, fmt.Errorf("leaderboard status %d: %s", resp.StatusCode, string(body))
	}

	signals, err := extractLeaderboardSignals(body, time.Now().UTC())
	if err != nil {
		return LeaderboardCollectStats{}, err
	}

	var stats LeaderboardCollectStats
	for _, signal := range signals {
		if signal.LeaderboardAccountValue < c.cfg.whaleThreshold() {
			continue
		}
		stats.EligibleAddresses++
		_, isNew, isExistingWallet, isRejected, err := c.store.upsertLeaderboardCandidate(signal)
		if err != nil {
			slog.Warn("failed to upsert leaderboard candidate", "address", signal.Address, "error", err)
			continue
		}
		switch {
		case isNew:
			stats.NewPotentialAddresses++
		case isExistingWallet:
			stats.ExistingWallets++
		case isRejected:
			stats.RejectedAddresses++
		default:
			stats.UpdatedCandidates++
		}
	}
	return stats, nil
}

func (c *collector) runLeaderboardPoller(ctx context.Context, url string, interval time.Duration) error {
	if interval <= 0 {
		interval = 10 * time.Minute
	}

	if stats, err := c.collectLeaderboard(ctx, url); err != nil {
		slog.Warn("leaderboard collection failed", "error", err)
	} else {
		slog.Info("leaderboard potential addresses collected",
			"new_addresses_potential", stats.NewPotentialAddresses,
			"eligible_addresses", stats.EligibleAddresses,
			"updated_candidates", stats.UpdatedCandidates,
			"existing_wallets", stats.ExistingWallets,
			"rejected_addresses", stats.RejectedAddresses,
		)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			stats, err := c.collectLeaderboard(ctx, url)
			if err != nil {
				slog.Warn("leaderboard collection failed", "error", err)
				continue
			}
			slog.Info("leaderboard potential addresses collected",
				"new_addresses_potential", stats.NewPotentialAddresses,
				"eligible_addresses", stats.EligibleAddresses,
				"updated_candidates", stats.UpdatedCandidates,
				"existing_wallets", stats.ExistingWallets,
				"rejected_addresses", stats.RejectedAddresses,
			)
		}
	}
}

func extractLeaderboardSignals(data []byte, seenAt time.Time) ([]WalletSignal, error) {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	signals := make([]WalletSignal, 0)
	seen := make(map[string]struct{})
	walkLeaderboard(payload, seenAt, &signals, seen)
	for i := range signals {
		if signals[i].LeaderboardRank == 0 {
			signals[i].LeaderboardRank = i + 1
		}
	}
	return signals, nil
}

func walkLeaderboard(value any, seenAt time.Time, signals *[]WalletSignal, seen map[string]struct{}) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			walkLeaderboard(item, seenAt, signals, seen)
		}
	case map[string]any:
		address := findAddressInMap(typed)
		if address != "" {
			address = NormalizeAddress(address)
			if _, exists := seen[address]; !exists {
				seen[address] = struct{}{}
				*signals = append(*signals, WalletSignal{
					Address:                 address,
					Source:                  SourceLeaderboard,
					SeenAt:                  seenAt,
					LeaderboardRank:         int(numberFromMap(typed, "rank")),
					LeaderboardAccountValue: numberFromMap(typed, "accountValue", "account_value", "account_value_usd"),
					LeaderboardPNL:          numberFromMap(typed, "pnl", "profit", "profit_loss"),
					LeaderboardROI:          numberFromMap(typed, "roi"),
				})
			}
		}
		for _, child := range typed {
			walkLeaderboard(child, seenAt, signals, seen)
		}
	}
}

func findAddressInMap(values map[string]any) string {
	for _, key := range []string{"ethAddress", "eth_address", "address", "user", "user_address", "wallet"} {
		if value, ok := values[key].(string); ok && addressPattern.MatchString(value) {
			return value
		}
	}
	for _, value := range values {
		if text, ok := value.(string); ok && addressPattern.MatchString(text) {
			return text
		}
	}
	return ""
}

func numberFromMap(values map[string]any, keys ...string) float64 {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed
		case string:
			return parseFloat(typed)
		case json.Number:
			return parseFloat(typed.String())
		}
	}
	return 0
}
