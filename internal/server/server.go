// Package server provides HTTP server functionality for the hypermetrics API.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jkeddari/hypermetrics/internal/metrics/hyperliquid"
	"github.com/jkeddari/hypermetrics/internal/metrics/leaderboard"
)

// Config holds the server configuration.
type Config struct {
	// Address is the server listen address (e.g., ":8080" or "localhost:8080")
	Address string
	// ReadTimeout is the maximum duration for reading the entire request
	ReadTimeout time.Duration
	// WriteTimeout is the maximum duration before timing out writes of the response
	WriteTimeout time.Duration
	// IdleTimeout is the maximum amount of time to wait for the next request
	IdleTimeout time.Duration
	// Logger is the structured logger for the server
	Logger *slog.Logger
}

// DefaultConfig returns a Config with sensible default values.
func DefaultConfig() *Config {
	return &Config{
		Address:      ":8080",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
		Logger:       slog.Default(),
	}
}

// Server represents the HTTP server with its dependencies.
type Server struct {
	config            *Config
	leaderboardLive   *leaderboard.LiveLeaderboard
	hyperliquidClient *hyperliquid.Client
	httpServer        *http.Server
	logger            *slog.Logger
}

// NewServer creates a new Server instance with the given configuration.
// It initializes the leaderboard tracker and sets up HTTP routes.
func NewServer(config *Config) (*Server, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	// Initialize leaderboard tracker
	lb, err := leaderboard.NewLiveLeaderboard(nil)
	if err != nil {
		return nil, fmt.Errorf("server: failed to initialize leaderboard: %w", err)
	}

	// Initialize Hyperliquid client
	hlClient := hyperliquid.NewClient(nil)

	s := &Server{
		config:            config,
		leaderboardLive:   lb,
		hyperliquidClient: hlClient,
		logger:            config.Logger,
	}

	// Setup HTTP server
	mux := http.NewServeMux()
	s.setupRoutes(mux)

	s.httpServer = &http.Server{
		Addr:         config.Address,
		Handler:      s.loggingMiddleware(mux),
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return s, nil
}

// setupRoutes configures all HTTP routes for the server.
func (s *Server) setupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/info/leaderboard", s.handleLeaderboard)
	mux.HandleFunc("/api/info/user/", s.handleUserInfo)
	mux.HandleFunc("/health", s.handleHealth)
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.logger.Info("starting server", "address", s.config.Address)
	return s.httpServer.ListenAndServe()
}

// Stop gracefully stops the HTTP server and cleans up resources.
func (s *Server) Stop() error {
	s.logger.Info("stopping server")

	// Shutdown HTTP server
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}

	return nil
}

// handleLeaderboard handles GET /api/info/leaderboard
// Query parameters:
//   - window: time window (daily, weekly, monthly, alltime) - optional
//   - metric: metric to sort by (pnl, roi, vlm) - optional
//   - order: sort order (asc, desc) - optional, defaults to desc
//
// By default, returns leaderboard sorted by account value (descending).
//
// Examples:
//   - /api/info/leaderboard                                    -> sorted by value desc
//   - /api/info/leaderboard?window=daily&metric=pnl           -> sorted by daily PNL desc
//   - /api/info/leaderboard?window=weekly&metric=roi&order=asc -> sorted by weekly ROI asc
func (s *Server) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse query parameters
	window := r.URL.Query().Get("window") // daily, weekly, monthly, alltime
	metric := r.URL.Query().Get("metric") // pnl, roi, vlm
	order := r.URL.Query().Get("order")   // asc, desc

	// Default to descending order
	desc := true
	if order == "asc" {
		desc = false
	}

	// Build sort code
	sortCode := s.buildSortCode(window, metric)

	// Get sorted leaderboard
	rows, err := s.leaderboardLive.SortBoards(sortCode, desc)
	if err != nil {
		s.logger.Error("failed to sort leaderboard", "error", err, "sortCode", sortCode)
		s.respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid sort parameters: %v", err))
		return
	}

	// Prepare response
	response := LeaderboardResponse{
		Rows:        rows,
		Count:       len(rows),
		SortBy:      sortCode,
		Order:       getOrderString(desc),
		LastRefresh: s.leaderboardLive.LastRefresh(),
	}

	s.respondJSON(w, http.StatusOK, response)
}

// buildSortCode constructs the sort code from window and metric parameters.
// Returns "value" by default if no parameters are specified.
func (s *Server) buildSortCode(window, metric string) string {
	// Default to sorting by account value
	if window == "" || metric == "" {
		return "value"
	}

	// Validate and normalize window
	switch window {
	case "daily", "day":
		window = "daily"
	case "weekly", "week":
		window = "weekly"
	case "monthly", "month":
		window = "monthly"
	case "alltime", "all":
		window = "alltime"
	default:
		return "value"
	}

	// Validate and normalize metric
	switch metric {
	case "pnl", "PNL":
		metric = "pnl"
	case "roi", "ROI":
		metric = "roi"
	case "vlm", "volume", "VLM":
		metric = "vlm"
	default:
		return "value"
	}

	return window + metric
}

// handleUserInfo handles GET /api/info/user/{address}
//
// Returns comprehensive user information from Hyperliquid including:
//   - Perpetual positions and margin summary
//   - Spot balances
//   - Open orders (perp and spot)
//   - Historical funding payments
//
// Example: GET /api/info/user/0x5d2f4460ac3514ada79f5d9838916e508ab39bb7
func (s *Server) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract address from URL path: /api/info/user/{address}
	path := strings.TrimPrefix(r.URL.Path, "/api/info/user/")
	address := strings.TrimSpace(path)

	if address == "" {
		s.respondError(w, http.StatusBadRequest, "address is required")
		return
	}

	// Basic validation: Ethereum address should start with 0x and be 42 chars
	if !strings.HasPrefix(address, "0x") || len(address) != 42 {
		s.respondError(w, http.StatusBadRequest, "invalid Ethereum address format (must be 0x followed by 40 hex characters)")
		return
	}

	// Fetch user info from Hyperliquid API
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	s.logger.Info("fetching user info", "address", address)

	userInfo, err := s.hyperliquidClient.InfoUser(ctx, address)
	if err != nil {
		s.logger.Error("failed to fetch user info", "address", address, "error", err)
		s.respondError(w, http.StatusInternalServerError, "failed to fetch user information from Hyperliquid")
		return
	}

	// Build API response
	response := s.buildUserInfoResponse(userInfo)

	s.respondJSON(w, http.StatusOK, response)
}

// buildUserInfoResponse converts internal UserInfo to API response format
func (s *Server) buildUserInfoResponse(info *hyperliquid.UserInfo) UserInfoResponse {
	response := UserInfoResponse{
		Address:   info.Address,
		UpdatedAt: info.UpdatedAt.Format(time.RFC3339),
	}

	// Build Perp response
	if info.Perp != nil {
		perpResp := PerpResponse{
			AccountValue:    info.Perp.MarginSummary.AccountValue,
			Withdrawable:    info.Perp.Withdrawable,
			TotalMarginUsed: info.Perp.MarginSummary.TotalMarginUsed,
			Positions:       make([]PositionResponse, 0, len(info.Perp.AssetPositions)),
		}

		for _, ap := range info.Perp.AssetPositions {
			pos := ap.Position
			perpResp.Positions = append(perpResp.Positions, PositionResponse{
				Coin:          pos.Coin,
				Size:          pos.Szi,
				EntryPrice:    pos.EntryPx,
				LiquidationPx: pos.LiquidationPx,
				UnrealizedPnl: pos.UnrealizedPnl,
				Leverage:      pos.Leverage.Value,
				LeverageType:  pos.Leverage.Type,
				PositionValue: pos.PositionValue,
				MarginUsed:    pos.MarginUsed,
			})
		}

		response.Perp = &perpResp
	}

	// Build Spot response
	if info.Spot != nil {
		spotResp := SpotResponse{
			Balances: make([]BalanceResponse, 0, len(info.Spot.Balances)),
		}

		for _, bal := range info.Spot.Balances {
			spotResp.Balances = append(spotResp.Balances, BalanceResponse{
				Coin:  bal.Coin,
				Total: bal.Total,
				Hold:  bal.Hold,
			})
		}

		response.Spot = &spotResp
	}

	// Build Open Orders response
	if info.OpenOrders != nil {
		ordersResp := OpenOrdersResponse{
			Perp: make([]OrderResponse, 0, len(info.OpenOrders.Perp)),
			Spot: make([]OrderResponse, 0, len(info.OpenOrders.Spot)),
		}

		for _, order := range info.OpenOrders.Perp {
			ordersResp.Perp = append(ordersResp.Perp, OrderResponse{
				Coin:      order.Coin,
				Side:      order.Side,
				LimitPx:   order.LimitPx,
				Size:      order.Sz,
				OrderID:   order.Oid,
				Timestamp: order.Timestamp,
			})
		}

		for _, order := range info.OpenOrders.Spot {
			ordersResp.Spot = append(ordersResp.Spot, OrderResponse{
				Coin:      order.Coin,
				Side:      order.Side,
				LimitPx:   order.LimitPx,
				Size:      order.Sz,
				OrderID:   order.Oid,
				Timestamp: order.Timestamp,
			})
		}

		response.OpenOrders = &ordersResp
	}

	// Build Funding response
	response.Funding = make([]FundingPaymentResponse, 0, len(info.Funding))
	for _, funding := range info.Funding {
		response.Funding = append(response.Funding, FundingPaymentResponse{
			Time:        funding.Time,
			Coin:        funding.Coin,
			AmountUSDC:  funding.UsedC,
			FundingRate: funding.FundingRate,
		})
	}

	return response
}

// handleHealth handles GET /health for health checks.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	health := HealthResponse{
		Status:      "ok",
		Timestamp:   time.Now(),
		LastRefresh: s.leaderboardLive.LastRefresh(),
	}

	s.respondJSON(w, http.StatusOK, health)
}

// loggingMiddleware logs HTTP requests.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a custom response writer to capture status code
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		duration := time.Since(start)
		s.logger.Info("request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", sw.status,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// respondJSON sends a JSON response.
func (s *Server) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("failed to encode JSON response", "error", err)
	}
}

// respondError sends an error response.
func (s *Server) respondError(w http.ResponseWriter, status int, message string) {
	s.respondJSON(w, status, ErrorResponse{
		Error:  message,
		Status: status,
	})
}

// getOrderString returns the string representation of the sort order.
func getOrderString(desc bool) string {
	if desc {
		return "desc"
	}
	return "asc"
}

// LeaderboardResponse represents the response for the leaderboard endpoint.
type LeaderboardResponse struct {
	Rows        []leaderboard.LeaderBoardRow `json:"rows"`
	Count       int                          `json:"count"`
	SortBy      string                       `json:"sort_by"`
	Order       string                       `json:"order"`
	LastRefresh time.Time                    `json:"last_refresh"`
}

// HealthResponse represents the response for the health check endpoint.
type HealthResponse struct {
	Status      string    `json:"status"`
	Timestamp   time.Time `json:"timestamp"`
	LastRefresh time.Time `json:"last_refresh"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error  string `json:"error"`
	Status int    `json:"status"`
}

// UserInfoResponse represents the response for /api/info/user/{address}
type UserInfoResponse struct {
	Address    string                   `json:"address"`
	Perp       *PerpResponse            `json:"perp"`
	Spot       *SpotResponse            `json:"spot"`
	OpenOrders *OpenOrdersResponse      `json:"open_orders"`
	Funding    []FundingPaymentResponse `json:"funding_payments"`
	UpdatedAt  string                   `json:"updated_at"`
}

// PerpResponse represents perpetual futures state in the API response
type PerpResponse struct {
	AccountValue    string             `json:"account_value"`
	Withdrawable    string             `json:"withdrawable"`
	TotalMarginUsed string             `json:"total_margin_used"`
	Positions       []PositionResponse `json:"positions"`
}

// PositionResponse represents a position in the API response
type PositionResponse struct {
	Coin          string `json:"coin"`
	Size          string `json:"size"`
	EntryPrice    string `json:"entry_price"`
	LiquidationPx string `json:"liquidation_px,omitempty"`
	UnrealizedPnl string `json:"unrealized_pnl"`
	Leverage      int    `json:"leverage"`
	LeverageType  string `json:"leverage_type"`
	PositionValue string `json:"position_value"`
	MarginUsed    string `json:"margin_used"`
}

// SpotResponse represents spot state in the API response
type SpotResponse struct {
	Balances []BalanceResponse `json:"balances"`
}

// BalanceResponse represents a token balance in the API response
type BalanceResponse struct {
	Coin  string `json:"coin"`
	Total string `json:"total"`
	Hold  string `json:"hold"`
}

// OpenOrdersResponse represents open orders in the API response
type OpenOrdersResponse struct {
	Perp []OrderResponse `json:"perp"`
	Spot []OrderResponse `json:"spot"`
}

// OrderResponse represents an order in the API response
type OrderResponse struct {
	Coin      string `json:"coin"`
	Side      string `json:"side"`
	LimitPx   string `json:"limit_px"`
	Size      string `json:"size"`
	OrderID   int64  `json:"order_id"`
	Timestamp int64  `json:"timestamp"`
}

// FundingPaymentResponse represents a funding payment in the API response
type FundingPaymentResponse struct {
	Time        int64  `json:"time"`
	Coin        string `json:"coin"`
	AmountUSDC  string `json:"amount_usdc"`
	FundingRate string `json:"funding_rate"`
}
