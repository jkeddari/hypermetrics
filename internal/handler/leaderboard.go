package handler

import (
	"log/slog"
	"net/http"

	"github.com/jkeddari/hypermetrics/internal/service"
	"github.com/jkeddari/hypermetrics/internal/ui"
	"github.com/jkeddari/hypermetrics/internal/ui/pages"
)

type LeaderboardHandler struct {
	leaderboardService *service.LeaderboardS3Service
}

func NewLeaderboardHandler(leaderboardService *service.LeaderboardS3Service) *LeaderboardHandler {
	return &LeaderboardHandler{
		leaderboardService: leaderboardService,
	}
}

func (h *LeaderboardHandler) LeaderboardPage(w http.ResponseWriter, r *http.Request) {
	// Get sort parameters from query string
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "dailypnl" // Default sort
	}

	// Get period from query string (for sorting by value)
	period := r.URL.Query().Get("period")
	if period == "" {
		// Extract period from sortBy if not explicitly provided
		period = extractPeriod(sortBy)
	}

	// Get leaderboard data sorted by the specified metric
	data, err := h.leaderboardService.GetSortedLeaderboard(sortBy, true) // true = descending
	if err != nil {
		slog.Error("failed to fetch leaderboard", "error", err)
		http.Error(w, "Failed to load leaderboard", http.StatusInternalServerError)
		return
	}

	// Limit to top 100 to improve performance
	limit := 100
	if len(data) > limit {
		data = data[:limit]
	}

	// Get last refresh time
	lastRefresh := h.leaderboardService.GetLastRefresh()

	ui.Render(w, r, pages.Leaderboard(data, sortBy, period, lastRefresh))
}

func extractPeriod(sortBy string) string {
	if len(sortBy) >= 5 {
		if sortBy[:5] == "daily" {
			return "daily"
		}
	}
	if len(sortBy) >= 6 {
		if sortBy[:6] == "weekly" {
			return "weekly"
		}
	}
	if len(sortBy) >= 7 {
		if sortBy[:7] == "monthly" {
			return "monthly"
		}
		if sortBy[:7] == "alltime" {
			return "alltime"
		}
	}
	return "daily" // default
}
