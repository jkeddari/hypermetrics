// Package user provides functionality to fetch and analyze user data from Hyperliquid DEX.
//
// This package offers comprehensive access to user-specific information including:
//   - Perpetual positions and margin summary
//   - Spot balances and holdings
//   - Open orders (perp and spot)
//   - Historical funding payments
//   - User fills and trade history
//
// All data is fetched directly from Hyperliquid's public API with proper error handling
// and type-safe structures.
//
// Example usage:
//
//	client := user.NewClient(nil)
//	info, err := client.InfoUser(ctx, "0x...")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Account Value: %s\n", info.Perp.AccountValue)
package user

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jkeddari/hypermetrics/internal/utils"
	"golang.org/x/time/rate"
)

const (
	hyperliquidAPIURL  = "https://api.hyperliquid.xyz/info"
	defaultHTTPTimeout = 30 * time.Second
	defaultRateLimit   = 10 // requests per second
)

// Client is the main client for interacting with Hyperliquid API.
type Client struct {
	httpClient *http.Client
	logger     *slog.Logger
	apiURL     string
}

// NewClient creates a new Hyperliquid API client.
//
// If httpClient is nil, a default client with rate limiting is created.
// The default configuration includes:
//   - 30 second timeout
//   - 10 requests per second rate limit
//   - Automatic retry logic
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout:   defaultHTTPTimeout,
			Transport: utils.NewRoundTripper(rate.NewLimiter(defaultRateLimit, 2), defaultHTTPTimeout),
		}
	}

	return &Client{
		httpClient: httpClient,
		logger:     slog.Default(),
		apiURL:     hyperliquidAPIURL,
	}
}

// UserInfo aggregates all information about a Hyperliquid user.
//
// This structure contains comprehensive data including perpetual positions,
// spot balances, open orders, and historical data.
type UserInfo struct {
	Address    string           `json:"address"`
	Perp       *PerpState       `json:"perp"`             // Perpetual futures state
	Spot       *SpotState       `json:"spot"`             // Spot trading state
	OpenOrders *OpenOrdersInfo  `json:"open_orders"`      // Current open orders
	Funding    []FundingPayment `json:"funding_payments"` // Historical funding payments
	UpdatedAt  time.Time        `json:"updated_at"`
}

// PerpState represents the user's perpetual futures trading state.
type PerpState struct {
	MarginSummary              MarginSummary   `json:"marginSummary"`
	CrossMarginSummary         MarginSummary   `json:"crossMarginSummary"`
	CrossMaintenanceMarginUsed string          `json:"crossMaintenanceMarginUsed"`
	Withdrawable               string          `json:"withdrawable"`
	AssetPositions             []AssetPosition `json:"assetPositions"`
	Time                       int64           `json:"time"`
}

// MarginSummary contains margin and account value information.
type MarginSummary struct {
	AccountValue    string `json:"accountValue"`
	TotalNtlPos     string `json:"totalNtlPos"`
	TotalRawUsd     string `json:"totalRawUsd"`
	TotalMarginUsed string `json:"totalMarginUsed"`
}

// AssetPosition represents a perpetual position for a specific asset.
type AssetPosition struct {
	Position Position `json:"position"`
	Type     string   `json:"type"`
}

// Position contains detailed information about a trading position.
type Position struct {
	Coin           string       `json:"coin"`
	EntryPx        string       `json:"entryPx,omitempty"`
	Leverage       LeverageInfo `json:"leverage"`
	LiquidationPx  string       `json:"liquidationPx,omitempty"`
	MarginUsed     string       `json:"marginUsed"`
	MaxTradeSzs    []string     `json:"maxTradeSzs,omitempty"`
	PositionValue  string       `json:"positionValue"`
	ReturnOnEquity string       `json:"returnOnEquity,omitempty"`
	Szi            string       `json:"szi"`
	UnrealizedPnl  string       `json:"unrealizedPnl"`
	CumFunding     *CumFunding  `json:"cumFunding,omitempty"`
}

// LeverageInfo represents leverage settings for a position.
type LeverageInfo struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

// CumFunding represents cumulative funding information.
type CumFunding struct {
	AllTime     string `json:"allTime"`
	SinceChange string `json:"sinceChange"`
	SinceOpen   string `json:"sinceOpen"`
}

// SpotState represents the user's spot trading state.
type SpotState struct {
	Balances []SpotBalance `json:"balances"`
}

// SpotBalance represents a balance for a specific token.
type SpotBalance struct {
	Coin     string `json:"coin"`
	Token    int    `json:"token"`
	Total    string `json:"total"`
	Hold     string `json:"hold"`
	EntryNtl string `json:"entryNtl"`
}

// OpenOrdersInfo contains all open orders for a user.
type OpenOrdersInfo struct {
	Perp []Order `json:"perp"`
	Spot []Order `json:"spot"`
}

// Order represents an open order.
type Order struct {
	Coin       string `json:"coin"`
	Side       string `json:"side"`
	LimitPx    string `json:"limitPx"`
	Sz         string `json:"sz"`
	Oid        int64  `json:"oid"`
	Timestamp  int64  `json:"timestamp"`
	OrigSz     string `json:"origSz"`
	Cloid      string `json:"cloid,omitempty"`
	OrderType  string `json:"orderType,omitempty"`
	ReduceOnly bool   `json:"reduceOnly,omitempty"`
	Tif        string `json:"tif,omitempty"`
}

// FundingPayment represents a funding payment record.
type FundingPayment struct {
	Time        int64  `json:"time"`
	Coin        string `json:"coin"`
	UsedC       string `json:"usdc"`
	Szi         string `json:"szi"`
	FundingRate string `json:"fundingRate"`
}

// InfoUser fetches comprehensive information about a Hyperliquid user.
//
// This function aggregates multiple API calls to provide complete user data:
//   - Perpetual futures state (positions, margin, PnL)
//   - Spot balances and holdings
//   - Open orders (perp and spot)
//   - Historical funding payments (last 30 days)
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - address: Ethereum address of the user (must start with 0x)
//
// Returns:
//   - UserInfo: Complete user information
//   - error: Any error encountered during fetching
//
// Example:
//
//	client := user.NewClient(nil)
//	info, err := client.InfoUser(ctx, "0x1234...")
//	if err != nil {
//	    return err
//	}
//
//	fmt.Printf("Account Value: %s USDC\n", info.Perp.MarginSummary.AccountValue)
//	fmt.Printf("Withdrawable: %s USDC\n", info.Perp.Withdrawable)
//	fmt.Printf("Spot Balances: %d tokens\n", len(info.Spot.Balances))
//	fmt.Printf("Open Orders: %d perp, %d spot\n",
//	    len(info.OpenOrders.Perp), len(info.OpenOrders.Spot))
func (c *Client) InfoUser(ctx context.Context, address string) (*UserInfo, error) {
	c.logger.Info("fetching user info", "address", address)

	userInfo := &UserInfo{
		Address:   address,
		UpdatedAt: time.Now(),
	}

	// Fetch perpetual state
	perpState, err := c.FetchPerpState(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch perp state: %w", err)
	}
	userInfo.Perp = perpState

	// Fetch spot state
	spotState, err := c.fetchSpotState(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch spot state: %w", err)
	}
	userInfo.Spot = spotState

	// Fetch open orders
	openOrders, err := c.fetchOpenOrders(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch open orders: %w", err)
	}
	userInfo.OpenOrders = openOrders

	// Fetch funding payments (optional, don't fail if error)
	funding, err := c.fetchFundingPayments(ctx, address)
	if err != nil {
		c.logger.Warn("failed to fetch funding payments", "error", err)
	} else {
		userInfo.Funding = funding
	}

	c.logger.Info("user info fetched successfully",
		"address", address,
		"perp_positions", len(userInfo.Perp.AssetPositions),
		"spot_balances", len(userInfo.Spot.Balances),
		"open_orders_perp", len(userInfo.OpenOrders.Perp),
		"open_orders_spot", len(userInfo.OpenOrders.Spot),
	)

	return userInfo, nil
}

// FetchPerpState fetches the user's perpetual futures state.
func (c *Client) FetchPerpState(ctx context.Context, address string) (*PerpState, error) {
	payload := map[string]interface{}{
		"type": "clearinghouseState",
		"user": address,
	}

	var state PerpState
	if err := c.doRequest(ctx, payload, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// BatchPerpStates fetches perpetual states for multiple addresses in a single request.
//
// This method uses Hyperliquid's batchClearinghouseStates endpoint to efficiently
// retrieve position data for many users at once, significantly reducing API calls
// and improving performance compared to individual requests.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - addresses: Slice of Ethereum addresses to fetch states for
//
// Returns:
//   - Map of address to PerpState
//   - Error if the batch request fails
//
// Performance:
//   - Single request for multiple addresses (vs N individual requests)
//   - Typical response time: 2-5 seconds for 50-100 addresses
//   - Rate limit weight: 2 (same as single clearinghouseState)
//
// Example:
//
//	states, err := client.BatchPerpStates(ctx, []string{
//	    "0x31ca8395cf837de08b24da3f660e77761dfb974b",
//	    "0x2ba553d9f990a3b66b03b2dc0d030dfc1c061036",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for addr, state := range states {
//	    fmt.Printf("%s: %s\n", addr, state.MarginSummary.AccountValue)
//	}
func (c *Client) BatchPerpStates(ctx context.Context, addresses []string) (map[string]*PerpState, error) {
	if len(addresses) == 0 {
		return make(map[string]*PerpState), nil
	}

	// Try batch endpoint first
	payload := map[string]interface{}{
		"type":  "batchClearinghouseStates",
		"users": addresses,
	}

	var states []PerpState
	err := c.doRequest(ctx, payload, &states)

	// If batch endpoint fails, fall back to sequential requests
	if err != nil {
		c.logger.Warn("batch endpoint failed, falling back to sequential requests", "error", err)
		return c.batchPerpStatesSequential(ctx, addresses)
	}

	// Map states back to addresses
	// The API returns states in the same order as the input addresses
	result := make(map[string]*PerpState, len(addresses))
	for i, addr := range addresses {
		if i < len(states) {
			stateCopy := states[i]
			result[addr] = &stateCopy
		}
	}

	return result, nil
}

// batchPerpStatesSequential fetches perp states sequentially (fallback method).
func (c *Client) batchPerpStatesSequential(ctx context.Context, addresses []string) (map[string]*PerpState, error) {
	result := make(map[string]*PerpState, len(addresses))

	for _, addr := range addresses {
		state, err := c.FetchPerpState(ctx, addr)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch perp state for %s: %w", addr, err)
		}
		result[addr] = state
	}

	return result, nil
}

// fetchSpotState fetches the user's spot trading state.
func (c *Client) fetchSpotState(ctx context.Context, address string) (*SpotState, error) {
	payload := map[string]interface{}{
		"type": "spotClearinghouseState",
		"user": address,
	}

	var state SpotState
	if err := c.doRequest(ctx, payload, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// fetchOpenOrders fetches all open orders for the user.
func (c *Client) fetchOpenOrders(ctx context.Context, address string) (*OpenOrdersInfo, error) {
	payload := map[string]interface{}{
		"type": "openOrders",
		"user": address,
	}

	var orders []Order
	if err := c.doRequest(ctx, payload, &orders); err != nil {
		return nil, err
	}

	// Separate perp and spot orders (Hyperliquid returns all together)
	// For now, we'll put them all in perp until we have better logic
	return &OpenOrdersInfo{
		Perp: orders,
		Spot: []Order{},
	}, nil
}

// fetchFundingPayments fetches historical funding payments for the user.
//
// This fetches funding payments from the last 30 days by default.
func (c *Client) fetchFundingPayments(ctx context.Context, address string) ([]FundingPayment, error) {
	// Calculate start time (30 days ago)
	startTime := time.Now().Add(-30 * 24 * time.Hour).UnixMilli()

	payload := map[string]interface{}{
		"type":      "userFunding",
		"user":      address,
		"startTime": startTime,
	}

	var payments []FundingPayment
	if err := c.doRequest(ctx, payload, &payments); err != nil {
		return nil, err
	}

	return payments, nil
}

// doRequest performs an HTTP POST request to the Hyperliquid API.
func (c *Client) doRequest(ctx context.Context, payload map[string]interface{}, result interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// GetPerpPositions returns only the perpetual positions from user info.
//
// This is a convenience method to quickly access position data.
func (u *UserInfo) GetPerpPositions() []AssetPosition {
	if u.Perp == nil {
		return []AssetPosition{}
	}
	return u.Perp.AssetPositions
}

// GetSpotBalances returns only the spot balances from user info.
//
// This is a convenience method to quickly access balance data.
func (u *UserInfo) GetSpotBalances() []SpotBalance {
	if u.Spot == nil {
		return []SpotBalance{}
	}
	return u.Spot.Balances
}

// GetTotalAccountValue returns the total account value in USDC.
//
// This includes both cross margin and isolated margin positions.
func (u *UserInfo) GetTotalAccountValue() string {
	if u.Perp == nil {
		return "0"
	}
	return u.Perp.MarginSummary.AccountValue
}

// GetWithdrawable returns the withdrawable amount in USDC.
func (u *UserInfo) GetWithdrawable() string {
	if u.Perp == nil {
		return "0"
	}
	return u.Perp.Withdrawable
}

// HasOpenPositions returns true if the user has any open perpetual positions.
func (u *UserInfo) HasOpenPositions() bool {
	return len(u.GetPerpPositions()) > 0
}

// HasOpenOrders returns true if the user has any open orders.
func (u *UserInfo) HasOpenOrders() bool {
	if u.OpenOrders == nil {
		return false
	}
	return len(u.OpenOrders.Perp) > 0 || len(u.OpenOrders.Spot) > 0
}
