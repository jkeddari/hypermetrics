// Package server provides HTTP server functionality for the hypermetrics API.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

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
	config          *Config
	leaderboardLive *leaderboard.LiveLeaderboard
	httpServer      *http.Server
	logger          *slog.Logger
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

	s := &Server{
		config:          config,
		leaderboardLive: lb,
		logger:          config.Logger,
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
