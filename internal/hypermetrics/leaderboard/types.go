package leaderboard

import (
	"encoding/json"
	"strings"
)

// rawLeaderboard is the top-level structure for decoding the Hyperliquid leaderboard API response.
type rawLeaderboard struct {
	Rows []LeaderBoardRow `json:"leaderboardRows"`
}

// LeaderBoardRow captures the performance of an address on the leaderboard.
// It includes account information and performance metrics across multiple time windows.
type LeaderBoardRow struct {
	EthAddress       string             `json:"ethAddress"`
	AccountValue     string             `json:"accountValue"`
	DailyPerformance PerformanceMetrics // Daily performance metrics
	WeekPerformance  PerformanceMetrics // Weekly performance metrics
	MonthPerformance PerformanceMetrics // Monthly performance metrics
	AllPerformance   PerformanceMetrics // All-time performance metrics
	Prize            float64            `json:"prize"`
	DisplayName      *string            `json:"displayName"`
}

// UnmarshalJSON implements custom JSON unmarshaling for LeaderBoardRow.
// It handles the Hyperliquid API's format where windowPerformances is an array of [label, metrics] tuples.
func (l *LeaderBoardRow) UnmarshalJSON(data []byte) error {
	type rawRow struct {
		EthAddress         string            `json:"ethAddress"`
		AccountValue       string            `json:"accountValue"`
		WindowPerformances []json.RawMessage `json:"windowPerformances"`
		Prize              float64           `json:"prize"`
		DisplayName        *string           `json:"displayName"`
	}

	var raw rawRow
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	l.EthAddress = raw.EthAddress
	l.AccountValue = raw.AccountValue
	l.Prize = raw.Prize
	l.DisplayName = raw.DisplayName

	for _, entry := range raw.WindowPerformances {
		var tuple []json.RawMessage
		if err := json.Unmarshal(entry, &tuple); err != nil {
			return err
		}
		if len(tuple) != 2 {
			continue
		}

		var label string
		if err := json.Unmarshal(tuple[0], &label); err != nil {
			return err
		}
		var metrics PerformanceMetrics
		if err := json.Unmarshal(tuple[1], &metrics); err != nil {
			return err
		}

		switch strings.ToLower(label) {
		case "day":
			l.DailyPerformance = metrics
		case "week":
			l.WeekPerformance = metrics
		case "month":
			l.MonthPerformance = metrics
		case "alltime":
			l.AllPerformance = metrics
		}
	}
	return nil
}

// PerformanceMetrics contains trading performance data for a specific time window.
// PNL is profit/loss, ROI is return on investment, and VLM is volume.
// All values are stored as strings as returned by the API.
type PerformanceMetrics struct {
	PNL string `json:"pnl"` // Profit and Loss
	ROI string `json:"roi"` // Return on Investment
	VLM string `json:"vlm"` // Volume
}
