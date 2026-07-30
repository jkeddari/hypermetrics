package api

import (
	"net/http"

	"github.com/jkeddari/hypermetrics/internal/app"
	"github.com/jkeddari/hypermetrics/internal/middleware"
)

func SetupRoutes(app *app.APIApp) http.Handler {
	requireAPIKey := middleware.RequireAPIKey(app.APIKeyService)
	hyperliquidAPI := NewHyperliquidAPIHandler(app.HypercoreStore, app.HypercoreService, app.Cfg.HypercoreWhaleThresholdUSD)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/hyperliquid/whale-alert", requireAPIKey(hyperliquidAPI.WhaleAlert))
	mux.HandleFunc("GET /api/hyperliquid/whale-position", requireAPIKey(hyperliquidAPI.WhalePosition))
	mux.HandleFunc("GET /api/hyperliquid/position", requireAPIKey(hyperliquidAPI.Position))
	mux.HandleFunc("GET /api/hyperliquid/user-position", requireAPIKey(hyperliquidAPI.UserPosition))
	mux.HandleFunc("GET /api/hyperliquid/wallets", requireAPIKey(hyperliquidAPI.Wallets))
	mux.HandleFunc("GET /api/hyperliquid/wallet/position-distribution", requireAPIKey(hyperliquidAPI.WalletPositionDistribution))
	mux.HandleFunc("GET /api/hyperliquid/wallet/pnl-distribution", requireAPIKey(hyperliquidAPI.WalletPnLDistribution))
	mux.HandleFunc("GET /api/hyperliquid/global-long-short-account-ratio/history", requireAPIKey(hyperliquidAPI.LongShortAccountRatioHistory))

	return middleware.Chain(
		mux,
		middleware.RequestLogging,
	)
}
