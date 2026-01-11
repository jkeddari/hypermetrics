package handler

import (
	"log/slog"
	"net/http"

	"github.com/jkeddari/hypermetrics/internal/service"
	"github.com/jkeddari/hypermetrics/internal/ui"
	"github.com/jkeddari/hypermetrics/internal/ui/pages"
)

type DashboardHandler struct {
	dashboardService *service.DashboardService
}

func NewDashboardHandler(dashboardService *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{
		dashboardService: dashboardService,
	}
}

func (h *DashboardHandler) DashboardPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Fetch dashboard data from service
	data, err := h.dashboardService.GetDashboardData(ctx)
	if err != nil {
		slog.Error("failed to fetch dashboard data", "error", err)
		http.Error(w, "Failed to load dashboard", http.StatusInternalServerError)
		return
	}

	ui.Render(w, r, pages.Dashboard(data))
}

// DashboardMetrics handles HTMX polling requests for dashboard metrics
func (h *DashboardHandler) DashboardMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Fetch dashboard data from service
	data, err := h.dashboardService.GetDashboardData(ctx)
	if err != nil {
		slog.Warn("error refreshing dashboard metrics", "error", err)

		// If data is nil, service is completely unavailable
		if data == nil {
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}

		// If we have stale data, serve it (data.IsStale will be true)
	}

	// Render partial for HTMX swap
	ui.Render(w, r, pages.DashboardMetricsPartial(data))
}
