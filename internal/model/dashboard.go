package model

import "time"

// DashboardMetrics represents aggregated metrics across all perpetuals
type DashboardMetrics struct {
	TotalOpenInterest float64 // Total open interest in USD
	TotalDailyVolume  float64 // Total daily volume in USD
	AlertNumber       int     // Number of active alerts
}

// Symbol represents a single perpetual symbol in the top 10
type Symbol struct {
	Name              string  // "BTC"
	Price             float64 // Current price in USD
	PriceChange24h    float64 // 24h price change in USD
	PriceChange24hPct float64 // 24h price change in percentage
	FundingRate       float64 // Funding rate (decimal, e.g., 0.0000125)
	DailyVolume       float64 // Daily volume in USD
	OpenInterest      float64 // Open interest in USD
}

// DashboardData contains all data needed for dashboard rendering
type DashboardData struct {
	Metrics     DashboardMetrics
	TopSymbols  []Symbol
	LastUpdated time.Time
	IsStale     bool // true when displaying cached data after API error
}
