// Package leaderboard provides functionality to track and analyze the Hyperliquid trading leaderboard.
//
// This package implements a lazy-loading cache strategy with Time-To-Live (TTL) expiration.
// Data is fetched from Hyperliquid's API only when requested and cached for a configurable duration.
// This approach optimizes for both performance and cost-efficiency, particularly in serverless
// environments like Google Cloud Run where scale-to-zero is desired.
//
// Architecture - Lazy Loading with TTL Cache:
//
//	First Request (or after TTL expiration):
//	  User Request → Fetch from Hyperliquid API (2-5s) → Cache (60s TTL) → Response
//
//	Subsequent Requests (within TTL):
//	  User Request → Read from Cache (<50ms) → Response
//
// Key Features:
//   - Lazy loading: Data fetched only when needed
//   - TTL-based cache: Automatic expiration after configurable duration (default: 60s)
//   - Thread-safe: Concurrent requests handled safely with RWMutex
//   - Single-fetch guarantee: Multiple concurrent requests during cache miss result in single fetch
//   - Scale-to-zero friendly: No background goroutines, perfect for serverless
//   - Sort capabilities across multiple metrics (PNL, ROI, Volume)
//   - Support for multiple time windows (daily, weekly, monthly, all-time)
//
// Performance Characteristics:
//   - Cache HIT: <50ms response time
//   - Cache MISS: 2-5 seconds (Hyperliquid API latency)
//   - Memory usage: ~200-300 MB when cache is populated
//   - TTL duration: 60 seconds (configurable)
//
// Performance Metrics:
//   - PNL (Profit and Loss): Trading profit or loss in USD
//   - ROI (Return on Investment): Percentage return on investment
//   - VLM (Volume): Total trading volume in USD
//
// Time Windows:
//   - Daily: Last 24 hours of trading activity
//   - Weekly: Last 7 days of trading activity
//   - Monthly: Last 30 days of trading activity
//   - All-time: Complete trading history
//
// Usage Example:
//
//	// Create a new leaderboard tracker with default settings
//	lb, err := leaderboard.NewLiveLeaderboard(nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Get all addresses on the leaderboard
//	// First call: fetches from API (2-5s)
//	// Subsequent calls within 60s: served from cache (<50ms)
//	addresses := lb.AddressList()
//
//	// Sort by daily PNL (descending)
//	sorted, err := lb.SortBoards("dailypnl", true)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Check when data was last refreshed
//	lastUpdate := lb.LastRefresh()
//
// Thread Safety:
//
// The package uses a combination of RWMutex and a separate fetch mutex to ensure:
//   - Multiple readers can access the cache concurrently
//   - Only one fetch operation occurs during cache miss, even with concurrent requests
//   - No race conditions during cache updates
//
// Custom HTTP clients can be provided for advanced configuration such as custom
// rate limits, timeouts, or proxy settings.
package leaderboard

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jkeddari/hypermetrics/internal/utils"
	"golang.org/x/time/rate"
)

const (
	statsLeaderboardURL = "https://stats-data.hyperliquid.xyz/Mainnet/leaderboard"
	defaultHTTPTimeout  = 20 * time.Second
	defaultCacheTTL     = time.Minute // Cache duration before re-fetch
	defaultRateLimit    = 20
)

// LiveLeaderboard maintains an in-memory cached snapshot of Hyperliquid's leaderboard.
//
// This implementation uses a lazy-loading strategy with TTL-based cache expiration.
// Data is fetched from the Hyperliquid API only when requested and the cache has expired.
//
// Cache Behavior:
//   - On first access or after TTL expiration: Fetches fresh data from Hyperliquid API
//   - Within TTL window: Returns cached data immediately
//   - Concurrent requests during fetch: Only one fetch occurs, others wait for result
//
// The structure is thread-safe and can handle high concurrent read load efficiently.
type LiveLeaderboard struct {
	logger        *slog.Logger
	client        *http.Client
	cacheTTL      time.Duration
	mu            sync.RWMutex // Protects board and lastRefreshed
	fetchMutex    sync.Mutex   // Ensures only one fetch at a time
	board         []LeaderBoardRow
	lastRefreshed time.Time
}

// NewLiveLeaderboard creates a new leaderboard tracker with lazy-loading cache.
//
// Parameters:
//   - client: HTTP client for API requests. If nil, a default client with rate limiting is used.
//
// The returned LiveLeaderboard uses a lazy-loading strategy:
//   - No data is fetched at initialization
//   - First access triggers data fetch from Hyperliquid API
//   - Subsequent accesses within TTL (60s) use cached data
//   - After TTL expiration, next access triggers fresh fetch
//
// This design is optimal for serverless environments (e.g., Cloud Run) as it:
//   - Enables true scale-to-zero (no background goroutines)
//   - Minimizes memory usage when idle
//   - Provides fast response times for cached data (<50ms)
//   - Controls API request rate (max 1 per TTL period under load)
//
// Example:
//
//	lb, err := NewLiveLeaderboard(nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// First call: may take 2-5 seconds (fetches from API)
//	data, err := lb.SortBoards("dailypnl", true)
//
//	// Second call within 60s: <50ms (from cache)
//	data, err = lb.SortBoards("weeklyroi", true)
func NewLiveLeaderboard(client *http.Client) (*LiveLeaderboard, error) {
	if client == nil {
		client = &http.Client{
			Timeout:   defaultHTTPTimeout,
			Transport: utils.NewRoundTripper(rate.NewLimiter(defaultRateLimit, 2), defaultHTTPTimeout),
		}
	}

	lb := &LiveLeaderboard{
		logger:   slog.Default(),
		client:   client,
		cacheTTL: defaultCacheTTL,
	}

	return lb, nil
}

// ensureFreshData ensures the cache is populated and fresh.
//
// This method implements the lazy-loading cache strategy:
//  1. First checks if cached data is still valid (within TTL)
//  2. If valid, returns immediately (fast path)
//  3. If expired or empty, acquires fetch mutex and fetches new data
//  4. Uses double-check locking to prevent redundant fetches
//
// Thread Safety:
//   - Multiple concurrent calls during cache HIT: All proceed without blocking
//   - Multiple concurrent calls during cache MISS: First call fetches, others wait
//   - No race conditions during cache update
//
// This method is called internally before any data access operation.
func (l *LiveLeaderboard) ensureFreshData() error {
	// Fast path: check if cache is still valid (read lock only)
	l.mu.RLock()
	if time.Since(l.lastRefreshed) < l.cacheTTL && len(l.board) > 0 {
		l.mu.RUnlock()
		return nil // Cache is fresh
	}
	l.mu.RUnlock()

	// Cache expired or empty, need to fetch
	// Use fetch mutex to ensure only one fetch happens
	l.fetchMutex.Lock()
	defer l.fetchMutex.Unlock()

	// Double-check: another goroutine might have fetched while we waited
	l.mu.RLock()
	if time.Since(l.lastRefreshed) < l.cacheTTL && len(l.board) > 0 {
		l.mu.RUnlock()
		return nil // Another goroutine already fetched
	}
	l.mu.RUnlock()

	// Fetch fresh data
	l.logger.Info("fetching leaderboard data", "reason", "cache expired or empty")
	return l.fetchAndCache()
}

// fetchAndCache fetches the latest leaderboard data from Hyperliquid API and updates the cache.
//
// This method performs the actual HTTP request to fetch leaderboard data.
// It should only be called by ensureFreshData() which handles proper locking.
//
// Returns an error if the fetch fails for any reason (network, parsing, etc).
func (l *LiveLeaderboard) fetchAndCache() error {
	req, err := http.NewRequest(http.MethodGet, statsLeaderboardURL, nil)
	if err != nil {
		return fmt.Errorf("metrics: create leaderboard request: %w", err)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return fmt.Errorf("metrics: fetch leaderboard: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("metrics: leaderboard request failed: %s", resp.Status)
	}

	var board rawLeaderboard
	if err := json.NewDecoder(resp.Body).Decode(&board); err != nil {
		return fmt.Errorf("metrics: decode leaderboard response: %w", err)
	}

	// Update cache
	l.mu.Lock()
	l.board = board.Rows
	l.lastRefreshed = time.Now()
	l.mu.Unlock()

	l.logger.Info("leaderboard data cached", "rows", len(board.Rows), "ttl_seconds", l.cacheTTL.Seconds())
	return nil
}

// LastRefresh returns when the leaderboard cache was last updated.
//
// Returns zero time if no data has been fetched yet.
// The returned time indicates when the cached data was retrieved from Hyperliquid's API.
func (l *LiveLeaderboard) LastRefresh() time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastRefreshed
}

// SortBoards returns a sorted copy of the leaderboard based on the specified sortCode and order.
//
// This method triggers lazy loading: if the cache is expired or empty, it fetches fresh data
// from Hyperliquid's API before sorting. Subsequent calls within the TTL window use cached data.
//
// Performance:
//   - Cache HIT (within TTL): <50ms
//   - Cache MISS (expired or first call): 2-5 seconds (includes API fetch)
//
// The sortCode parameter accepts the following values:
//   - "value": sort by account value
//   - "dailypnl", "dailyroi", "dailyvlm": sort by daily performance metrics
//   - "weeklypnl", "weeklyroi", "weeklyvlm": sort by weekly performance metrics
//   - "monthlypnl", "monthlyroi", "monthlyvlm": sort by monthly performance metrics
//   - "alltimepnl", "alltimeroi", "alltimevlm": sort by all-time performance metrics
//
// The desc parameter controls sort order: true for descending, false for ascending.
//
// Returns an error if:
//   - sortCode is not recognized
//   - Data fetch fails (network error, API error, etc.)
//
// Example:
//
//	// First call may take 2-5s (fetches from API)
//	sorted, err := lb.SortBoards("dailypnl", true)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Second call within 60s is fast (<50ms, from cache)
//	sorted, err = lb.SortBoards("weeklyroi", false)
func (l *LiveLeaderboard) SortBoards(sortCode string, desc bool) ([]LeaderBoardRow, error) {
	// Ensure we have fresh data (lazy load if needed)
	if err := l.ensureFreshData(); err != nil {
		return nil, fmt.Errorf("failed to fetch leaderboard data: %w", err)
	}

	// Read from cache
	l.mu.RLock()
	board := make([]LeaderBoardRow, len(l.board))
	copy(board, l.board)
	l.mu.RUnlock()

	// Define a value extractor function based on the sort code
	var valueExtractor func(LeaderBoardRow) float64

	switch sortCode {
	case "value":
		valueExtractor = func(row LeaderBoardRow) float64 {
			return parseFloat(row.AccountValue)
		}
	case "dailypnl":
		valueExtractor = func(row LeaderBoardRow) float64 {
			return parseFloat(row.DailyPerformance.PNL)
		}
	case "dailyroi":
		valueExtractor = func(row LeaderBoardRow) float64 {
			return parseFloat(row.DailyPerformance.ROI)
		}
	case "dailyvlm":
		valueExtractor = func(row LeaderBoardRow) float64 {
			return parseFloat(row.DailyPerformance.VLM)
		}
	case "weeklypnl":
		valueExtractor = func(row LeaderBoardRow) float64 {
			return parseFloat(row.WeekPerformance.PNL)
		}
	case "weeklyroi":
		valueExtractor = func(row LeaderBoardRow) float64 {
			return parseFloat(row.WeekPerformance.ROI)
		}
	case "weeklyvlm":
		valueExtractor = func(row LeaderBoardRow) float64 {
			return parseFloat(row.WeekPerformance.VLM)
		}
	case "monthlypnl":
		valueExtractor = func(row LeaderBoardRow) float64 {
			return parseFloat(row.MonthPerformance.PNL)
		}
	case "monthlyroi":
		valueExtractor = func(row LeaderBoardRow) float64 {
			return parseFloat(row.MonthPerformance.ROI)
		}
	case "monthlyvlm":
		valueExtractor = func(row LeaderBoardRow) float64 {
			return parseFloat(row.MonthPerformance.VLM)
		}
	case "alltimepnl":
		valueExtractor = func(row LeaderBoardRow) float64 {
			return parseFloat(row.AllPerformance.PNL)
		}
	case "alltimeroi":
		valueExtractor = func(row LeaderBoardRow) float64 {
			return parseFloat(row.AllPerformance.ROI)
		}
	case "alltimevlm":
		valueExtractor = func(row LeaderBoardRow) float64 {
			return parseFloat(row.AllPerformance.VLM)
		}
	default:
		return nil, fmt.Errorf("liveboard: sort error code unknown %s", sortCode)
	}

	// Perform the sort with the extracted values
	sort.Slice(board, func(i, j int) bool {
		left := valueExtractor(board[i])
		right := valueExtractor(board[j])
		if desc {
			return left > right
		}
		return left < right
	})

	return board, nil
}

// AddressList returns all Ethereum addresses from the leaderboard
// which total value is more than value in parameters.
//
// This method triggers lazy loading: if the cache is expired or empty, it fetches fresh data
// from Hyperliquid's API before extracting addresses.
//
// Performance:
//   - Cache HIT: <50ms
//   - Cache MISS: 2-5 seconds (includes API fetch)
//
// Returns an empty slice if data fetch fails. Check logs for errors.
func (l *LiveLeaderboard) AddressList(value float64) []string {
	// Ensure we have fresh data (lazy load if needed)
	if err := l.ensureFreshData(); err != nil {
		l.logger.Error("failed to fetch leaderboard for address list", "error", err)
		return []string{}
	}

	// Read from cache
	l.mu.RLock()
	board := make([]LeaderBoardRow, len(l.board))
	copy(board, l.board)
	l.mu.RUnlock()

	var addresses []string
	for _, row := range board {
		if parseFloat(row.AccountValue) >= value {
			addresses = append(addresses, row.EthAddress)
		}
	}
	return addresses
}

// parseFloat converts a string to float64, returning 0.0 if parsing fails.
// It trims whitespace before parsing.
func parseFloat(value string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return f
}
