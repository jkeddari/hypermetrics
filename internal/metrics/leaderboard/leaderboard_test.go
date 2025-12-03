package leaderboard

import (
	"encoding/json"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testDataCache holds the loaded leaderboard data, initialized once in init().
var testDataCache []LeaderBoardRow

// init loads the test data once at package initialization.
func init() {
	data, err := os.ReadFile("leaderboard.json")
	if err != nil {
		log.Fatalf("failed to read test fixture: %v", err)
	}

	var raw rawLeaderboard
	if err := json.Unmarshal(data, &raw); err != nil {
		log.Fatalf("failed to unmarshal test data: %v", err)
	}

	testDataCache = raw.Rows
}

// loadTestData returns a deep copy of the cached test data to ensure test isolation.
func loadTestData(t *testing.T) []LeaderBoardRow {
	t.Helper()

	if len(testDataCache) == 0 {
		t.Fatal("test data cache is empty")
	}

	// Create a deep copy to prevent test interference
	copied := make([]LeaderBoardRow, len(testDataCache))
	copy(copied, testDataCache)

	return copied
}

func TestLeaderBoardRow_UnmarshalJSON(t *testing.T) {
	t.Parallel()
	rows := loadTestData(t)
	require.NotEmpty(t, rows, "test data should not be empty")

	// Test first row has expected structure
	row := rows[0]
	assert.NotEmpty(t, row.EthAddress, "ethAddress should not be empty")
	assert.NotEmpty(t, row.AccountValue, "accountValue should not be empty")
	assert.NotEmpty(t, row.DailyPerformance.PNL, "daily PNL should not be empty")
	assert.NotEmpty(t, row.WeekPerformance.PNL, "weekly PNL should not be empty")
	assert.NotEmpty(t, row.MonthPerformance.PNL, "monthly PNL should not be empty")
	assert.NotEmpty(t, row.AllPerformance.PNL, "all-time PNL should not be empty")

	// Verify all required performance windows are present
	assert.NotEmpty(t, row.DailyPerformance.ROI, "daily ROI should not be empty")
	assert.NotEmpty(t, row.DailyPerformance.VLM, "daily VLM should not be empty")
	assert.NotEmpty(t, row.WeekPerformance.ROI, "weekly ROI should not be empty")
	assert.NotEmpty(t, row.WeekPerformance.VLM, "weekly VLM should not be empty")
	assert.NotEmpty(t, row.MonthPerformance.ROI, "monthly ROI should not be empty")
	assert.NotEmpty(t, row.MonthPerformance.VLM, "monthly VLM should not be empty")
	assert.NotEmpty(t, row.AllPerformance.ROI, "all-time ROI should not be empty")
	assert.NotEmpty(t, row.AllPerformance.VLM, "all-time VLM should not be empty")
}

func TestSortBoards_Value(t *testing.T) {
	t.Parallel()
	lb := &LiveLeaderboard{
		logger:        slog.Default(),
		cacheTTL:      time.Minute,
		board:         loadTestData(t),
		lastRefreshed: time.Now(),
	}

	// Test descending sort
	sorted, err := lb.SortBoards("value", true)
	require.NoError(t, err)
	require.NotEmpty(t, sorted)

	// Verify descending order
	for i := 0; i < len(sorted)-1; i++ {
		curr := parseFloat(sorted[i].AccountValue)
		next := parseFloat(sorted[i+1].AccountValue)
		assert.GreaterOrEqual(t, curr, next, "values should be in descending order")
	}

	// Test ascending sort
	sorted, err = lb.SortBoards("value", false)
	require.NoError(t, err)

	// Verify ascending order
	for i := 0; i < len(sorted)-1; i++ {
		curr := parseFloat(sorted[i].AccountValue)
		next := parseFloat(sorted[i+1].AccountValue)
		assert.LessOrEqual(t, curr, next, "values should be in ascending order")
	}
}

func TestSortBoards_DailyMetrics(t *testing.T) {
	t.Parallel()
	lb := &LiveLeaderboard{
		logger:        slog.Default(),
		cacheTTL:      time.Minute,
		board:         loadTestData(t),
		lastRefreshed: time.Now(),
	}

	tests := []struct {
		name     string
		sortCode string
		extract  func(LeaderBoardRow) string
	}{
		{"daily PNL", "dailypnl", func(r LeaderBoardRow) string { return r.DailyPerformance.PNL }},
		{"daily ROI", "dailyroi", func(r LeaderBoardRow) string { return r.DailyPerformance.ROI }},
		{"daily VLM", "dailyvlm", func(r LeaderBoardRow) string { return r.DailyPerformance.VLM }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Test descending
			sorted, err := lb.SortBoards(tt.sortCode, true)
			require.NoError(t, err)
			require.NotEmpty(t, sorted)

			for i := 0; i < len(sorted)-1; i++ {
				curr := parseFloat(tt.extract(sorted[i]))
				next := parseFloat(tt.extract(sorted[i+1]))
				assert.GreaterOrEqual(t, curr, next, "should be in descending order")
			}

			// Test ascending
			sorted, err = lb.SortBoards(tt.sortCode, false)
			require.NoError(t, err)

			for i := 0; i < len(sorted)-1; i++ {
				curr := parseFloat(tt.extract(sorted[i]))
				next := parseFloat(tt.extract(sorted[i+1]))
				assert.LessOrEqual(t, curr, next, "should be in ascending order")
			}
		})
	}
}

func TestSortBoards_WeeklyMetrics(t *testing.T) {
	t.Parallel()
	lb := &LiveLeaderboard{
		logger:        slog.Default(),
		cacheTTL:      time.Minute,
		board:         loadTestData(t),
		lastRefreshed: time.Now(),
	}

	tests := []struct {
		name     string
		sortCode string
		extract  func(LeaderBoardRow) string
	}{
		{"weekly PNL", "weeklypnl", func(r LeaderBoardRow) string { return r.WeekPerformance.PNL }},
		{"weekly ROI", "weeklyroi", func(r LeaderBoardRow) string { return r.WeekPerformance.ROI }},
		{"weekly VLM", "weeklyvlm", func(r LeaderBoardRow) string { return r.WeekPerformance.VLM }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sorted, err := lb.SortBoards(tt.sortCode, true)
			require.NoError(t, err)
			require.NotEmpty(t, sorted)

			for i := 0; i < len(sorted)-1; i++ {
				curr := parseFloat(tt.extract(sorted[i]))
				next := parseFloat(tt.extract(sorted[i+1]))
				assert.GreaterOrEqual(t, curr, next)
			}
		})
	}
}

func TestSortBoards_MonthlyMetrics(t *testing.T) {
	t.Parallel()
	lb := &LiveLeaderboard{
		logger:        slog.Default(),
		cacheTTL:      time.Minute,
		board:         loadTestData(t),
		lastRefreshed: time.Now(),
	}

	tests := []struct {
		name     string
		sortCode string
		extract  func(LeaderBoardRow) string
	}{
		{"monthly PNL", "monthlypnl", func(r LeaderBoardRow) string { return r.MonthPerformance.PNL }},
		{"monthly ROI", "monthlyroi", func(r LeaderBoardRow) string { return r.MonthPerformance.ROI }},
		{"monthly VLM", "monthlyvlm", func(r LeaderBoardRow) string { return r.MonthPerformance.VLM }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sorted, err := lb.SortBoards(tt.sortCode, true)
			require.NoError(t, err)
			require.NotEmpty(t, sorted)

			for i := 0; i < len(sorted)-1; i++ {
				curr := parseFloat(tt.extract(sorted[i]))
				next := parseFloat(tt.extract(sorted[i+1]))
				assert.GreaterOrEqual(t, curr, next)
			}
		})
	}
}

func TestSortBoards_AllTimeMetrics(t *testing.T) {
	t.Parallel()
	lb := &LiveLeaderboard{
		logger:        slog.Default(),
		cacheTTL:      time.Minute,
		board:         loadTestData(t),
		lastRefreshed: time.Now(),
	}

	tests := []struct {
		name     string
		sortCode string
		extract  func(LeaderBoardRow) string
	}{
		{"all-time PNL", "alltimepnl", func(r LeaderBoardRow) string { return r.AllPerformance.PNL }},
		{"all-time ROI", "alltimeroi", func(r LeaderBoardRow) string { return r.AllPerformance.ROI }},
		{"all-time VLM", "alltimevlm", func(r LeaderBoardRow) string { return r.AllPerformance.VLM }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sorted, err := lb.SortBoards(tt.sortCode, true)
			require.NoError(t, err)
			require.NotEmpty(t, sorted)

			for i := 0; i < len(sorted)-1; i++ {
				curr := parseFloat(tt.extract(sorted[i]))
				next := parseFloat(tt.extract(sorted[i+1]))
				assert.GreaterOrEqual(t, curr, next)
			}
		})
	}
}

func TestSortBoards_InvalidSortCode(t *testing.T) {
	t.Parallel()
	lb := &LiveLeaderboard{
		logger:        slog.Default(),
		cacheTTL:      time.Minute,
		board:         loadTestData(t),
		lastRefreshed: time.Now(),
	}

	_, err := lb.SortBoards("invalid", true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sort error code unknown")
}

func TestSortBoards_DoesNotMutateOriginal(t *testing.T) {
	t.Parallel()
	rows := loadTestData(t)
	lb := &LiveLeaderboard{
		logger:        slog.Default(),
		cacheTTL:      time.Minute,
		board:         rows,
		lastRefreshed: time.Now(),
	}

	originalFirst := rows[0].EthAddress
	originalLast := rows[len(rows)-1].EthAddress

	// Sort by value descending
	sorted, err := lb.SortBoards("value", true)
	require.NoError(t, err)

	// Verify original board is unchanged
	assert.Equal(t, originalFirst, lb.board[0].EthAddress)
	assert.Equal(t, originalLast, lb.board[len(lb.board)-1].EthAddress)

	// Verify sorted is different
	assert.NotEqual(t, sorted[0].EthAddress, originalFirst)
}

func TestAddressList(t *testing.T) {
	t.Parallel()
	rows := loadTestData(t)
	lb := &LiveLeaderboard{
		logger:        slog.Default(),
		cacheTTL:      time.Minute,
		board:         rows,
		lastRefreshed: time.Now(),
	}

	addresses := lb.AddressList(0)

	// Verify addresses start with 0x
	for _, addr := range addresses {
		assert.True(t, len(addr) > 2 && addr[:2] == "0x", "address should start with 0x")
	}
}

func TestLastRefresh(t *testing.T) {
	t.Parallel()
	lb := &LiveLeaderboard{
		logger:        slog.Default(),
		cacheTTL:      time.Minute,
		board:         loadTestData(t),
		lastRefreshed: time.Now(),
	}

	refreshTime := lb.LastRefresh()
	assert.False(t, refreshTime.IsZero(), "last refresh time should be set")
	assert.WithinDuration(t, time.Now(), refreshTime, 5*time.Second)
}

func TestLazyLoading(t *testing.T) {
	t.Parallel()
	rows := loadTestData(t)
	lb := &LiveLeaderboard{
		logger:   slog.Default(),
		cacheTTL: time.Minute,
	}

	// Initially, cache should be empty
	assert.Zero(t, lb.LastRefresh(), "last refresh should be zero initially")

	// Manually populate cache to simulate lazy load
	lb.mu.Lock()
	lb.board = rows
	lb.lastRefreshed = time.Now()
	lb.mu.Unlock()

	// Now cache should be populated
	assert.NotZero(t, lb.LastRefresh(), "last refresh should be set after cache population")

	// Access should work without fetch
	addresses := lb.AddressList(0)
	assert.NotEmpty(t, addresses, "should return addresses from cache")
}

func TestCacheTTLExpiration(t *testing.T) {
	t.Parallel()
	rows := loadTestData(t)
	lb := &LiveLeaderboard{
		logger:   slog.Default(),
		cacheTTL: 100 * time.Millisecond, // Short TTL for testing
	}

	// Populate cache
	lb.mu.Lock()
	lb.board = rows
	lb.lastRefreshed = time.Now().Add(-200 * time.Millisecond) // Set to expired
	lb.mu.Unlock()

	// Cache should be considered expired
	assert.True(t, time.Since(lb.LastRefresh()) > lb.cacheTTL, "cache should be expired")
}

func TestParseFloat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"positive number", "123.456", 123.456},
		{"negative number", "-789.012", -789.012},
		{"with whitespace", "  456.789  ", 456.789},
		{"zero", "0", 0},
		{"invalid input", "invalid", 0},
		{"empty string", "", 0},
		{"scientific notation", "1.23e5", 123000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseFloat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPerformanceMetrics_AllFieldsPresent(t *testing.T) {
	t.Parallel()
	rows := loadTestData(t)
	require.NotEmpty(t, rows)

	// Check a sample of rows to ensure all metrics are populated
	sampleSize := 100
	if len(rows) < sampleSize {
		sampleSize = len(rows)
	}

	for i := 0; i < sampleSize; i++ {
		row := rows[i]

		// Daily metrics
		assert.NotEmpty(t, row.DailyPerformance.PNL, "row %d daily PNL should not be empty", i)
		assert.NotEmpty(t, row.DailyPerformance.ROI, "row %d daily ROI should not be empty", i)
		assert.NotEmpty(t, row.DailyPerformance.VLM, "row %d daily VLM should not be empty", i)

		// Weekly metrics
		assert.NotEmpty(t, row.WeekPerformance.PNL, "row %d weekly PNL should not be empty", i)
		assert.NotEmpty(t, row.WeekPerformance.ROI, "row %d weekly ROI should not be empty", i)
		assert.NotEmpty(t, row.WeekPerformance.VLM, "row %d weekly VLM should not be empty", i)

		// Monthly metrics
		assert.NotEmpty(t, row.MonthPerformance.PNL, "row %d monthly PNL should not be empty", i)
		assert.NotEmpty(t, row.MonthPerformance.ROI, "row %d monthly ROI should not be empty", i)
		assert.NotEmpty(t, row.MonthPerformance.VLM, "row %d monthly VLM should not be empty", i)

		// All-time metrics
		assert.NotEmpty(t, row.AllPerformance.PNL, "row %d all-time PNL should not be empty", i)
		assert.NotEmpty(t, row.AllPerformance.ROI, "row %d all-time ROI should not be empty", i)
		assert.NotEmpty(t, row.AllPerformance.VLM, "row %d all-time VLM should not be empty", i)
	}
}
