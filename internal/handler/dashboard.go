package handler

import (
	"net/http"

	"github.com/jkeddari/hypermetrics/internal/ui"
	"github.com/jkeddari/hypermetrics/internal/ui/pages"
)

type DashboardHandler struct{}

// TODO : add all service for dashboard
func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{}
}

func (h *DashboardHandler) DashboardPage(w http.ResponseWriter, r *http.Request) {
	// user := ctxkeys.User(r.Context())

	ui.Render(w, r, pages.Dashboard())
}
