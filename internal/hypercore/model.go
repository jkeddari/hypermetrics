package hypercore

import (
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	SourceLeaderboard = "leaderboard"
	SourceManual      = "manual"

	OnDemandFreshness = 5 * time.Minute

	TierOnDemand = iota
	TierKnownWhale
	TierLeaderboard
	TierWarm
	TierCold
)

type Wallet struct {
	Address string   `json:"address"`
	Sources []string `json:"sources"`

	FirstSeenAt           time.Time `json:"first_seen_at"`
	LastSeenLeaderboardAt time.Time `json:"last_seen_leaderboard_at,omitempty"`
	LastRefreshedAt       time.Time `json:"last_refreshed_at,omitempty"`
	LastSuccessfulRefresh time.Time `json:"last_successful_refresh_at,omitempty"`
	LastFailedRefreshAt   time.Time `json:"last_failed_refresh_at,omitempty"`

	RefreshAttempts    int    `json:"refresh_attempts"`
	ConsecutiveFailure int    `json:"consecutive_failures"`
	LastError          string `json:"last_error,omitempty"`

	KnownWhale            bool    `json:"known_whale"`
	HasOpenPosition       bool    `json:"has_open_position"`
	MaxPositionValueUSD   float64 `json:"max_position_value_usd"`
	TotalPositionValueUSD float64 `json:"total_position_value_usd"`
	AccountValue          float64 `json:"account_value"`

	LeaderboardRank         int     `json:"leaderboard_rank,omitempty"`
	LeaderboardAccountValue float64 `json:"leaderboard_account_value,omitempty"`
	LeaderboardPNL          float64 `json:"leaderboard_pnl,omitempty"`
	LeaderboardROI          float64 `json:"leaderboard_roi,omitempty"`

	ManualPriorityBoost float64   `json:"manual_priority_boost"`
	NextRefreshAt       time.Time `json:"next_refresh_at,omitempty"`
	PriorityScore       float64   `json:"priority_score"`
	Tier                int       `json:"tier"`
}

type WalletSignal struct {
	Address string
	Source  string
	SeenAt  time.Time

	LeaderboardRank         int
	LeaderboardAccountValue float64
	LeaderboardPNL          float64
	LeaderboardROI          float64

	ManualPriorityBoost float64
}

type WalletCandidate struct {
	Address string `json:"address"`

	FirstSeenAt           time.Time `json:"first_seen_at"`
	LastSeenLeaderboardAt time.Time `json:"last_seen_leaderboard_at"`
	NextScanAt            time.Time `json:"next_scan_at,omitempty"`

	ScanAttempts       int    `json:"scan_attempts"`
	ConsecutiveFailure int    `json:"consecutive_failures"`
	LastError          string `json:"last_error,omitempty"`

	LeaderboardRank         int     `json:"leaderboard_rank,omitempty"`
	LeaderboardAccountValue float64 `json:"leaderboard_account_value"`
	LeaderboardPNL          float64 `json:"leaderboard_pnl,omitempty"`
	LeaderboardROI          float64 `json:"leaderboard_roi,omitempty"`
	PriorityScore           float64 `json:"priority_score"`
}

type RejectedWalletCandidate struct {
	Address string `json:"address"`

	RejectedAt              time.Time `json:"rejected_at"`
	LastSeenLeaderboardAt   time.Time `json:"last_seen_leaderboard_at"`
	LeaderboardRank         int       `json:"leaderboard_rank,omitempty"`
	LeaderboardAccountValue float64   `json:"leaderboard_account_value"`
	RealAccountValue        float64   `json:"real_account_value"`
	Reason                  string    `json:"reason"`
}

type LeaderboardCollectStats struct {
	EligibleAddresses     int
	NewPotentialAddresses int
	UpdatedCandidates     int
	ExistingWallets       int
	RejectedAddresses     int
}

type WalletAccount struct {
	Address       string    `json:"address"`
	AccountValue  float64   `json:"account_value"`
	MarginUsed    float64   `json:"margin_used"`
	Withdrawable  float64   `json:"withdrawable"`
	RefreshedAt   time.Time `json:"refreshed_at"`
	RawReceivedAt time.Time `json:"raw_received_at"`
}

type WalletPosition struct {
	Address          string    `json:"address"`
	Symbol           string    `json:"symbol"`
	PositionSize     float64   `json:"position_size"`
	EntryPrice       float64   `json:"entry_price"`
	MarkPrice        float64   `json:"mark_price"`
	LiqPrice         float64   `json:"liq_price"`
	Leverage         float64   `json:"leverage"`
	MarginBalance    float64   `json:"margin_balance"`
	PositionValueUSD float64   `json:"position_value_usd"`
	UnrealizedPnL    float64   `json:"unrealized_pnl"`
	RefreshedAt      time.Time `json:"refreshed_at"`
}

type WalletState struct {
	Account   WalletAccount    `json:"account"`
	Positions []WalletPosition `json:"positions"`
}

type PriorityConfig struct {
	WhaleThresholdUSD    float64
	RejectedCandidateTTL time.Duration
	Now                  func() time.Time
}

func (c PriorityConfig) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now().UTC()
}

func (c PriorityConfig) whaleThreshold() float64 {
	if c.WhaleThresholdUSD > 0 {
		return c.WhaleThresholdUSD
	}
	return 1_000_000
}

func (c PriorityConfig) rejectedCandidateTTL() time.Duration {
	if c.RejectedCandidateTTL > 0 {
		return c.RejectedCandidateTTL
	}
	return 12 * time.Hour
}

func NormalizeAddress(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

func IsAddress(address string) bool {
	address = NormalizeAddress(address)
	if len(address) != 42 || !strings.HasPrefix(address, "0x") {
		return false
	}
	for _, r := range address[2:] {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}
	return true
}

func ParseFloat(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0
	}
	return parsed
}

func MergeWalletSignal(wallet Wallet, signal WalletSignal, cfg PriorityConfig) Wallet {
	now := signal.SeenAt
	if now.IsZero() {
		now = cfg.now()
	}

	signal.Address = NormalizeAddress(signal.Address)
	if wallet.Address == "" {
		wallet.Address = signal.Address
		wallet.FirstSeenAt = now
		wallet.Tier = TierCold
	}
	if wallet.FirstSeenAt.IsZero() {
		wallet.FirstSeenAt = now
	}

	wallet.Sources = appendSource(wallet.Sources, signal.Source)

	switch signal.Source {
	case SourceLeaderboard:
		wallet.LastSeenLeaderboardAt = now
		if signal.LeaderboardRank > 0 {
			wallet.LeaderboardRank = signal.LeaderboardRank
		}
		if signal.LeaderboardAccountValue > 0 {
			wallet.LeaderboardAccountValue = signal.LeaderboardAccountValue
			if wallet.AccountValue == 0 {
				wallet.AccountValue = signal.LeaderboardAccountValue
			}
		}
		wallet.LeaderboardPNL = signal.LeaderboardPNL
		wallet.LeaderboardROI = signal.LeaderboardROI
		if wallet.LastRefreshedAt.IsZero() && wallet.ConsecutiveFailure == 0 && EffectiveAccountValue(wallet) >= 100_000 {
			wallet.NextRefreshAt = now
		}
	case SourceManual:
		if signal.ManualPriorityBoost > wallet.ManualPriorityBoost {
			wallet.ManualPriorityBoost = signal.ManualPriorityBoost
		}
	}

	wallet.Tier = ComputeTier(wallet, cfg)
	wallet.PriorityScore = ComputePriorityScore(wallet, cfg)
	if wallet.NextRefreshAt.IsZero() {
		wallet.NextRefreshAt = now
	}
	return wallet
}

func ApplyRefreshSuccess(wallet Wallet, state WalletState, cfg PriorityConfig) Wallet {
	now := cfg.now()
	wallet.AccountValue = state.Account.AccountValue
	wallet.LastRefreshedAt = now
	wallet.LastSuccessfulRefresh = now
	wallet.RefreshAttempts++
	wallet.ConsecutiveFailure = 0
	wallet.LastError = ""

	totalValue := 0.0
	maxValue := 0.0
	for _, position := range state.Positions {
		value := math.Abs(position.PositionValueUSD)
		totalValue += value
		if value > maxValue {
			maxValue = value
		}
	}

	wallet.HasOpenPosition = len(state.Positions) > 0
	wallet.TotalPositionValueUSD = totalValue
	wallet.MaxPositionValueUSD = maxValue
	wallet.KnownWhale = maxValue >= cfg.whaleThreshold()
	wallet.Tier = ComputeTier(wallet, cfg)
	wallet.PriorityScore = ComputePriorityScore(wallet, cfg)
	wallet.NextRefreshAt = now.Add(TargetInterval(wallet, cfg))
	return wallet
}

func ApplyRefreshFailure(wallet Wallet, err error, cfg PriorityConfig) Wallet {
	now := cfg.now()
	wallet.LastRefreshedAt = now
	wallet.LastFailedRefreshAt = now
	wallet.RefreshAttempts++
	wallet.ConsecutiveFailure++
	if err != nil {
		wallet.LastError = err.Error()
	}
	backoff := time.Duration(1<<min(wallet.ConsecutiveFailure, 6)) * time.Minute
	wallet.NextRefreshAt = now.Add(backoff)
	wallet.PriorityScore = ComputePriorityScore(wallet, cfg)
	return wallet
}

func ComputeTier(wallet Wallet, cfg PriorityConfig) int {
	if wallet.KnownWhale || wallet.MaxPositionValueUSD >= cfg.whaleThreshold() {
		return TierKnownWhale
	}
	if !wallet.LastSeenLeaderboardAt.IsZero() && cfg.now().Sub(wallet.LastSeenLeaderboardAt) <= 24*time.Hour {
		return TierLeaderboard
	}
	if wallet.HasOpenPosition {
		return TierWarm
	}
	return TierCold
}

func ComputePriorityScore(wallet Wallet, cfg PriorityConfig) float64 {
	now := cfg.now()
	score := staleScore(wallet, now, cfg)
	score += knownWhaleScore(wallet)
	score += leaderboardScore(wallet, now)
	score += accountValueScore(wallet)
	score += unscannedValueScore(wallet)

	if wallet.HasOpenPosition {
		score += 25
	}
	score += math.Min(wallet.ManualPriorityBoost, 100)
	score -= recentRefreshPenalty(wallet, now)
	score -= float64(wallet.ConsecutiveFailure * 50)
	return score
}

func TargetInterval(wallet Wallet, cfg PriorityConfig) time.Duration {
	switch ComputeTier(wallet, cfg) {
	case TierKnownWhale:
		if wallet.MaxPositionValueUSD >= 10_000_000 {
			return 10 * time.Second
		}
		if wallet.MaxPositionValueUSD >= 5_000_000 {
			return 30 * time.Second
		}
		return time.Minute
	case TierLeaderboard:
		return 3 * time.Minute
	case TierWarm:
		return 30 * time.Minute
	default:
		return 6 * time.Hour
	}
}

// MaxRefreshInterval caps how long the coverage lane may postpone a due wallet.
func MaxRefreshInterval(wallet Wallet, cfg PriorityConfig) time.Duration {
	switch {
	case wallet.MaxPositionValueUSD >= 10_000_000:
		return time.Minute
	case wallet.MaxPositionValueUSD >= 5_000_000:
		return 2 * time.Minute
	case wallet.MaxPositionValueUSD >= cfg.whaleThreshold():
		return 5 * time.Minute
	case !wallet.LastSeenLeaderboardAt.IsZero():
		return 24 * time.Hour
	default:
		return 48 * time.Hour
	}
}

func IsTrackableState(state WalletState, thresholdUSD float64) bool {
	if thresholdUSD <= 0 {
		thresholdUSD = 1_000_000
	}
	if state.Account.AccountValue >= thresholdUSD {
		return true
	}
	for _, position := range state.Positions {
		if math.Abs(position.PositionValueUSD) >= thresholdUSD {
			return true
		}
	}
	return false
}

func appendSource(sources []string, source string) []string {
	if source == "" {
		return sources
	}
	for _, existing := range sources {
		if existing == source {
			return sources
		}
	}
	return append(sources, source)
}

func staleScore(wallet Wallet, now time.Time, cfg PriorityConfig) float64 {
	if wallet.LastRefreshedAt.IsZero() {
		return 100
	}
	interval := TargetInterval(wallet, cfg)
	if interval <= 0 {
		return 0
	}
	ratio := now.Sub(wallet.LastRefreshedAt).Seconds() / interval.Seconds()
	return math.Min(ratio, 5) * 20
}

func knownWhaleScore(wallet Wallet) float64 {
	switch {
	case wallet.MaxPositionValueUSD >= 10_000_000:
		return 100
	case wallet.MaxPositionValueUSD >= 5_000_000:
		return 75
	case wallet.MaxPositionValueUSD >= 1_000_000:
		return 50
	default:
		return 0
	}
}

func leaderboardScore(wallet Wallet, now time.Time) float64 {
	switch {
	case wallet.LeaderboardRank > 0 && wallet.LeaderboardRank <= 100:
		return 80
	case wallet.LeaderboardRank > 0 && wallet.LeaderboardRank <= 500:
		return 50
	case wallet.LeaderboardRank > 0 && wallet.LeaderboardRank <= 1000:
		return 30
	case !wallet.LastSeenLeaderboardAt.IsZero() && now.Sub(wallet.LastSeenLeaderboardAt) <= 24*time.Hour:
		return 15
	default:
		return 0
	}
}

func accountValueScore(wallet Wallet) float64 {
	value := EffectiveAccountValue(wallet)
	if value <= 0 {
		return 0
	}
	return math.Min(math.Log10(value)*8, 80)
}

func EffectiveAccountValue(wallet Wallet) float64 {
	if wallet.AccountValue > 0 {
		return wallet.AccountValue
	}
	return wallet.LeaderboardAccountValue
}

func unscannedValueScore(wallet Wallet) float64 {
	if !wallet.LastRefreshedAt.IsZero() {
		return 0
	}

	value := EffectiveAccountValue(wallet)
	if value < 100_000 {
		return 0
	}
	return 10_000 + math.Min(value/1_000, 100_000)
}

func ComputeCandidatePriorityScore(candidate WalletCandidate, cfg PriorityConfig) float64 {
	score := 10_000 + math.Min(candidate.LeaderboardAccountValue/1_000, 100_000)
	switch {
	case candidate.LeaderboardRank > 0 && candidate.LeaderboardRank <= 100:
		score += 80
	case candidate.LeaderboardRank > 0 && candidate.LeaderboardRank <= 500:
		score += 50
	case candidate.LeaderboardRank > 0 && candidate.LeaderboardRank <= 1000:
		score += 30
	}
	score -= float64(candidate.ConsecutiveFailure * 50)
	return score
}

func CandidateWalletSignal(candidate WalletCandidate) WalletSignal {
	return WalletSignal{
		Address:                 candidate.Address,
		Source:                  SourceLeaderboard,
		SeenAt:                  candidate.LastSeenLeaderboardAt,
		LeaderboardRank:         candidate.LeaderboardRank,
		LeaderboardAccountValue: candidate.LeaderboardAccountValue,
		LeaderboardPNL:          candidate.LeaderboardPNL,
		LeaderboardROI:          candidate.LeaderboardROI,
	}
}

func candidateFromSignal(signal WalletSignal, now time.Time, cfg PriorityConfig) WalletCandidate {
	candidate := WalletCandidate{
		Address:                 signal.Address,
		FirstSeenAt:             now,
		LastSeenLeaderboardAt:   now,
		NextScanAt:              now,
		LeaderboardRank:         signal.LeaderboardRank,
		LeaderboardAccountValue: signal.LeaderboardAccountValue,
		LeaderboardPNL:          signal.LeaderboardPNL,
		LeaderboardROI:          signal.LeaderboardROI,
	}
	candidate.PriorityScore = ComputeCandidatePriorityScore(candidate, cfg)
	return candidate
}

func recentRefreshPenalty(wallet Wallet, now time.Time) float64 {
	if wallet.LastRefreshedAt.IsZero() {
		return 0
	}
	seconds := now.Sub(wallet.LastRefreshedAt).Seconds()
	switch {
	case seconds < 10:
		return 1000
	case seconds < 30:
		return 200
	case seconds < 60:
		return 75
	default:
		return 0
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
