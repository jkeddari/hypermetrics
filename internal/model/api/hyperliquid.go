package api

type WhaleAlertQuery struct{}

type WhaleAlertItem struct {
	User             string  `json:"user"`
	Symbol           string  `json:"symbol"`
	PositionSize     float64 `json:"position_size"`
	EntryPrice       float64 `json:"entry_price"`
	LiqPrice         float64 `json:"liq_price"`
	PositionValueUSD float64 `json:"position_value_usd"`
	PositionAction   int16   `json:"position_action"`
	CreateTime       int64   `json:"create_time"`
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

type SpotBalanceItem struct {
	Coin          string  `json:"coin"`
	Token         int     `json:"token"`
	Hold          float64 `json:"hold"`
	Total         float64 `json:"total"`
	EntryNtl      float64 `json:"entry_ntl"`
	MarkPrice     float64 `json:"mark_price"`
	ValueUSD      float64 `json:"value_usd"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
}

type OpenOrderItem struct {
	OID              int64   `json:"oid"`
	ClientOID        string  `json:"client_oid,omitempty"`
	Coin             string  `json:"coin"`
	Symbol           string  `json:"symbol,omitempty"`
	MarketType       string  `json:"market_type"`
	Side             string  `json:"side"`
	OrderType        string  `json:"order_type"`
	LimitPrice       float64 `json:"limit_price"`
	Size             float64 `json:"size"`
	OriginalSize     float64 `json:"original_size"`
	ReduceOnly       bool    `json:"reduce_only"`
	IsTrigger        bool    `json:"is_trigger"`
	IsPositionTPSL   bool    `json:"is_position_tpsl"`
	TriggerCondition string  `json:"trigger_condition,omitempty"`
	TriggerPrice     float64 `json:"trigger_price"`
	Timestamp        int64   `json:"timestamp"`
}

type PerpetualOverview struct {
	MarginSummary MarginSummary   `json:"margin_summary"`
	Positions     []AssetPosition `json:"positions"`
	OpenOrders    []OpenOrderItem `json:"open_orders"`
	UnrealizedPnL float64         `json:"unrealized_pnl"`
}

type SpotOverview struct {
	Balances      []SpotBalanceItem `json:"balances"`
	OpenOrders    []OpenOrderItem   `json:"open_orders"`
	ValueUSD      float64           `json:"value_usd"`
	UnrealizedPnL float64           `json:"unrealized_pnl"`
}

type WalletOverviewTotals struct {
	PerpetualAccountValue float64 `json:"perpetual_account_value"`
	SpotValueUSD          float64 `json:"spot_value_usd"`
	TotalValueUSD         float64 `json:"total_value_usd"`
	UnrealizedPnL         float64 `json:"unrealized_pnl"`
}

type WalletOverviewData struct {
	User        string               `json:"user"`
	RefreshedAt int64                `json:"refreshed_at"`
	Totals      WalletOverviewTotals `json:"totals"`
	Perpetuals  PerpetualOverview    `json:"perpetuals"`
	Spot        SpotOverview         `json:"spot"`
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

type DistributionBucket struct {
	GroupName               string  `json:"group_name"`
	AllAddressCount         int64   `json:"all_address_count"`
	PositionAddressCount    int64   `json:"position_address_count"`
	PositionAddressPercent  float64 `json:"position_address_percent"`
	BiasScore               float64 `json:"bias_score"`
	BiasRemark              string  `json:"bias_remark"`
	MinimumAmount           float64 `json:"minimum_amount"`
	MaximumAmount           float64 `json:"maximum_amount"`
	LongPositionUSD         float64 `json:"long_position_usd"`
	ShortPositionUSD        float64 `json:"short_position_usd"`
	LongPositionUSDPercent  float64 `json:"long_position_usd_percent"`
	ShortPositionUSDPercent float64 `json:"short_position_usd_percent"`
	PositionUSD             float64 `json:"position_usd"`
	ProfitAddressCount      int64   `json:"profit_address_count"`
	LossAddressCount        int64   `json:"loss_address_count"`
	ProfitAddressPercent    float64 `json:"profit_address_percent"`
	LossAddressPercent      float64 `json:"loss_address_percent"`
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
