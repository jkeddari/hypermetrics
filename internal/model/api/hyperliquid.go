package api

type WhaleAlertQuery struct{}

type WhaleAlertItem struct {
	User             string  `json:"user"`
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"`
	Action           string  `json:"action"`
	PositionSize     float64 `json:"position_size"`
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	LiqPrice         float64 `json:"liq_price"`
	Leverage         float64 `json:"leverage"`
	PositionValueUSD float64 `json:"position_value_usd"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	Time             int64   `json:"time"`
}

type WhalePositionQuery struct{}

type WhalePositionItem struct {
	User             string  `json:"user"`
	Symbol           string  `json:"symbol"`
	PositionSize     float64 `json:"position_size"`
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	LiqPrice         float64 `json:"liq_price"`
	Leverage         float64 `json:"leverage"`
	MarginBalance    float64 `json:"margin_balance"`
	PositionValueUSD float64 `json:"position_value_usd"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
}

type PositionQuery struct {
	Symbol string `json:"symbol"`
}

type PositionItem struct {
	User             string  `json:"user"`
	Symbol           string  `json:"symbol"`
	PositionSize     float64 `json:"position_size"`
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	LiqPrice         float64 `json:"liq_price"`
	Leverage         float64 `json:"leverage"`
	MarginBalance    float64 `json:"margin_balance"`
	PositionValueUSD float64 `json:"position_value_usd"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
}

type UserPositionQuery struct {
	User string `json:"user"`
}

type MarginSummary struct {
	AccountValue float64 `json:"account_value"`
	MarginUsed   float64 `json:"margin_used"`
	Withdrawable float64 `json:"withdrawable"`
}

type AssetPosition struct {
	Symbol           string  `json:"symbol"`
	PositionSize     float64 `json:"position_size"`
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	LiqPrice         float64 `json:"liq_price"`
	Leverage         float64 `json:"leverage"`
	PositionValueUSD float64 `json:"position_value_usd"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
}

type UserPositionData struct {
	User          string          `json:"user"`
	MarginSummary MarginSummary   `json:"margin_summary"`
	AssetPosition []AssetPosition `json:"asset_positions"`
}

type WalletItem struct {
	Address               string   `json:"address"`
	Sources               []string `json:"sources"`
	Tier                  int      `json:"tier"`
	KnownWhale            bool     `json:"known_whale"`
	HasOpenPosition       bool     `json:"has_open_position"`
	AccountValue          float64  `json:"account_value"`
	TotalPositionValueUSD float64  `json:"total_position_value_usd"`
	MaxPositionValueUSD   float64  `json:"max_position_value_usd"`
	LastSeenLeaderboardAt int64    `json:"last_seen_leaderboard_at,omitempty"`
	LastRefreshedAt       int64    `json:"last_refreshed_at,omitempty"`
	NextRefreshAt         int64    `json:"next_refresh_at,omitempty"`
	PriorityScore         float64  `json:"priority_score"`
}

type WalletPositionDistributionQuery struct{}

type WalletPnLDistributionQuery struct{}

type DistributionBreakdown struct {
	Profit int `json:"profit"`
	Loss   int `json:"loss"`
}

type DistributionBucket struct {
	Tier                   string                `json:"tier"`
	AddressCount           int                   `json:"address_count"`
	LongPositionValue      float64               `json:"long_position_value"`
	ShortPositionValue     float64               `json:"short_position_value"`
	Sentiment              string                `json:"sentiment"`
	ProfitLossDistribution DistributionBreakdown `json:"profit_loss_distribution"`
}

type LongShortAccountRatioHistoryQuery struct {
	Symbol    string `json:"symbol"`
	Interval  string `json:"interval"`
	Limit     int    `json:"limit"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
}

type LongShortAccountRatioPoint struct {
	Time           int64   `json:"time"`
	Symbol         string  `json:"symbol"`
	LongAccount    float64 `json:"long_account"`
	ShortAccount   float64 `json:"short_account"`
	LongShortRatio float64 `json:"long_short_ratio"`
}
