package api

import (
	"net/http"

	"github.com/jkeddari/hypermetrics/internal/app"
	"github.com/jkeddari/hypermetrics/internal/middleware"
)

func SetupRoutes(app *app.APIApp) http.Handler {
	requireAPIKey := middleware.RequireAPIKey(app.APIKeyService)
	hyperliquidAPI := newHyperliquidAPIHandler(app.HypercoreStore, app.CoreClient, app.Cfg.HypercoreWhaleThresholdUSD)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/hyperliquid/whale-alert", requireAPIKey(hyperliquidAPI.whaleAlert))
	mux.HandleFunc("GET /api/hyperliquid/whale-position", requireAPIKey(hyperliquidAPI.whalePosition))
	mux.HandleFunc("GET /api/hyperliquid/position", requireAPIKey(hyperliquidAPI.position))
	mux.HandleFunc("GET /api/hyperliquid/user-position", requireAPIKey(hyperliquidAPI.userPosition))
	mux.HandleFunc("GET /api/hyperliquid/wallet/overview", requireAPIKey(hyperliquidAPI.walletOverview))
	mux.HandleFunc("GET /api/hyperliquid/wallets", requireAPIKey(hyperliquidAPI.wallets))
	mux.HandleFunc("GET /api/hyperliquid/wallet/position-distribution", requireAPIKey(hyperliquidAPI.walletPositionDistribution))
	mux.HandleFunc("GET /api/hyperliquid/wallet/pnl-distribution", requireAPIKey(hyperliquidAPI.walletPnLDistribution))
	mux.HandleFunc("GET /api/hyperliquid/global-long-short-account-ratio/history", requireAPIKey(hyperliquidAPI.longShortAccountRatioHistory))

	return middleware.Chain(
		mux,
		middleware.CORS(app.Cfg.AppURL),
		middleware.RequestLogging,
	)
}
