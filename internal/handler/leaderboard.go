package handler

import (
	"net/http"

	"github.com/jkeddari/hypermetrics/internal/ui"
	"github.com/jkeddari/hypermetrics/internal/ui/pages"
)

type LeaderboardHandler struct{}

func NewLeaderboardHandler() *LeaderboardHandler {
	return &LeaderboardHandler{}
}

func (h *LeaderboardHandler) LeaderboardPage(w http.ResponseWriter, r *http.Request) {
	ui.Render(w, r, pages.ComigSoon("Leaderboard"))
}
