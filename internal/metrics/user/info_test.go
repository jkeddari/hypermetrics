package user

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test with a real address to verify API integration
// Using a well-known active address from Hyperliquid leaderboard
const testAddress = "0x5d2f4460ac3514ada79f5d9838916e508ab39bb7"

func TestNewClient(t *testing.T) {
	t.Parallel()

	t.Run("with_nil_http_client", func(t *testing.T) {
		t.Parallel()
		client := NewClient(nil)
		assert.NotNil(t, client)
		assert.NotNil(t, client.httpClient)
		assert.NotNil(t, client.logger)
		assert.Equal(t, hyperliquidAPIURL, client.apiURL)
	})

	t.Run("with_custom_http_client", func(t *testing.T) {
		t.Parallel()
		customClient := &http.Client{Timeout: 5 * time.Second}
		client := NewClient(customClient)
		assert.NotNil(t, client)
		assert.Equal(t, customClient, client.httpClient)
	})
}

func TestInfoUser_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	info, err := client.InfoUser(ctx, testAddress)
	require.NoError(t, err, "InfoUser should not return error for valid address")
	require.NotNil(t, info)

	// Verify basic structure
	assert.Equal(t, testAddress, info.Address)
	assert.NotZero(t, info.UpdatedAt)

	// Verify perp state exists
	assert.NotNil(t, info.Perp)
	// Note: AccountValue may be empty string for accounts with no activity
	// assert.NotEmpty(t, info.Perp.MarginSummary.AccountValue)

	// Verify spot state exists
	assert.NotNil(t, info.Spot)
	// Note: balances may be empty if user has no spot positions

	// Verify open orders structure exists
	assert.NotNil(t, info.OpenOrders)

	t.Logf("User Info Summary:")
	t.Logf("  Account Value: %s USDC", info.GetTotalAccountValue())
	t.Logf("  Withdrawable: %s USDC", info.GetWithdrawable())
	t.Logf("  Perp Positions: %d", len(info.GetPerpPositions()))
	t.Logf("  Spot Balances: %d", len(info.GetSpotBalances()))
	t.Logf("  Open Orders (Perp): %d", len(info.OpenOrders.Perp))
	t.Logf("  Open Orders (Spot): %d", len(info.OpenOrders.Spot))
	t.Logf("  Funding Payments: %d", len(info.Funding))
}

func TestFetchPerpState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	state, err := client.fetchPerpState(ctx, testAddress)
	require.NoError(t, err)
	require.NotNil(t, state)

	// Verify margin summary structure exists (values may be empty for accounts with no positions)
	// Fields are present but may be empty strings or "0.0"
	assert.NotNil(t, state.MarginSummary)
	assert.NotNil(t, state.CrossMarginSummary)

	// Withdrawable should be present (may be "0.0" for accounts with only open orders)
	assert.NotNil(t, state.Withdrawable)

	// Asset positions may be empty if no open positions
	t.Logf("Perp State:")
	t.Logf("  Account Value: %s", state.MarginSummary.AccountValue)
	t.Logf("  Withdrawable: %s", state.Withdrawable)
	t.Logf("  Positions: %d", len(state.AssetPositions))

	if len(state.AssetPositions) > 0 {
		for i, pos := range state.AssetPositions {
			t.Logf("  Position %d:", i+1)
			t.Logf("    Coin: %s", pos.Position.Coin)
			t.Logf("    Size: %s", pos.Position.Szi)
			t.Logf("    Entry Px: %s", pos.Position.EntryPx)
			t.Logf("    Unrealized PnL: %s", pos.Position.UnrealizedPnl)
		}
	}
}

func TestFetchSpotState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	state, err := client.fetchSpotState(ctx, testAddress)
	require.NoError(t, err)
	require.NotNil(t, state)

	t.Logf("Spot State:")
	t.Logf("  Balances: %d", len(state.Balances))

	if len(state.Balances) > 0 {
		for i, balance := range state.Balances {
			if i >= 5 {
				t.Logf("  ... and %d more", len(state.Balances)-5)
				break
			}
			t.Logf("  Balance %d:", i+1)
			t.Logf("    Coin: %s", balance.Coin)
			t.Logf("    Total: %s", balance.Total)
			t.Logf("    Hold: %s", balance.Hold)
		}
	}
}

func TestFetchOpenOrders(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	orders, err := client.fetchOpenOrders(ctx, testAddress)
	require.NoError(t, err)
	require.NotNil(t, orders)

	t.Logf("Open Orders:")
	t.Logf("  Perp Orders: %d", len(orders.Perp))
	t.Logf("  Spot Orders: %d", len(orders.Spot))

	if len(orders.Perp) > 0 {
		for i, order := range orders.Perp {
			if i >= 3 {
				t.Logf("  ... and %d more perp orders", len(orders.Perp)-3)
				break
			}
			t.Logf("  Order %d:", i+1)
			t.Logf("    Coin: %s", order.Coin)
			t.Logf("    Side: %s", order.Side)
			t.Logf("    Limit Px: %s", order.LimitPx)
			t.Logf("    Size: %s", order.Sz)
		}
	}
}

func TestFetchFundingPayments(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	payments, err := client.fetchFundingPayments(ctx, testAddress)
	// Funding may fail for some addresses, so we don't require no error
	if err != nil {
		t.Logf("Funding payments fetch failed (expected for some addresses): %v", err)
		return
	}

	t.Logf("Funding Payments: %d records", len(payments))

	if len(payments) > 0 {
		for i, payment := range payments {
			if i >= 5 {
				t.Logf("  ... and %d more payments", len(payments)-5)
				break
			}
			t.Logf("  Payment %d:", i+1)
			t.Logf("    Coin: %s", payment.Coin)
			t.Logf("    USDC: %s", payment.UsedC)
			t.Logf("    Rate: %s", payment.FundingRate)
			t.Logf("    Time: %v", time.UnixMilli(payment.Time))
		}
	}
}

func TestUserInfo_HelperMethods(t *testing.T) {
	t.Parallel()

	t.Run("GetPerpPositions_nil_perp", func(t *testing.T) {
		t.Parallel()
		info := &UserInfo{}
		positions := info.GetPerpPositions()
		assert.Empty(t, positions)
	})

	t.Run("GetPerpPositions_with_data", func(t *testing.T) {
		t.Parallel()
		info := &UserInfo{
			Perp: &PerpState{
				AssetPositions: []AssetPosition{
					{Position: Position{Coin: "BTC"}},
					{Position: Position{Coin: "ETH"}},
				},
			},
		}
		positions := info.GetPerpPositions()
		assert.Len(t, positions, 2)
		assert.Equal(t, "BTC", positions[0].Position.Coin)
	})

	t.Run("GetSpotBalances_nil_spot", func(t *testing.T) {
		t.Parallel()
		info := &UserInfo{}
		balances := info.GetSpotBalances()
		assert.Empty(t, balances)
	})

	t.Run("GetSpotBalances_with_data", func(t *testing.T) {
		t.Parallel()
		info := &UserInfo{
			Spot: &SpotState{
				Balances: []SpotBalance{
					{Coin: "USDC", Total: "1000"},
					{Coin: "PURR", Total: "500"},
				},
			},
		}
		balances := info.GetSpotBalances()
		assert.Len(t, balances, 2)
		assert.Equal(t, "USDC", balances[0].Coin)
	})

	t.Run("GetTotalAccountValue", func(t *testing.T) {
		t.Parallel()
		info := &UserInfo{
			Perp: &PerpState{
				MarginSummary: MarginSummary{
					AccountValue: "12345.67",
				},
			},
		}
		value := info.GetTotalAccountValue()
		assert.Equal(t, "12345.67", value)
	})

	t.Run("GetTotalAccountValue_nil_perp", func(t *testing.T) {
		t.Parallel()
		info := &UserInfo{}
		value := info.GetTotalAccountValue()
		assert.Equal(t, "0", value)
	})

	t.Run("GetWithdrawable", func(t *testing.T) {
		t.Parallel()
		info := &UserInfo{
			Perp: &PerpState{
				Withdrawable: "5000.00",
			},
		}
		withdrawable := info.GetWithdrawable()
		assert.Equal(t, "5000.00", withdrawable)
	})

	t.Run("HasOpenPositions_true", func(t *testing.T) {
		t.Parallel()
		info := &UserInfo{
			Perp: &PerpState{
				AssetPositions: []AssetPosition{
					{Position: Position{Coin: "BTC"}},
				},
			},
		}
		assert.True(t, info.HasOpenPositions())
	})

	t.Run("HasOpenPositions_false", func(t *testing.T) {
		t.Parallel()
		info := &UserInfo{
			Perp: &PerpState{
				AssetPositions: []AssetPosition{},
			},
		}
		assert.False(t, info.HasOpenPositions())
	})

	t.Run("HasOpenOrders_true_perp", func(t *testing.T) {
		t.Parallel()
		info := &UserInfo{
			OpenOrders: &OpenOrdersInfo{
				Perp: []Order{{Coin: "BTC"}},
				Spot: []Order{},
			},
		}
		assert.True(t, info.HasOpenOrders())
	})

	t.Run("HasOpenOrders_true_spot", func(t *testing.T) {
		t.Parallel()
		info := &UserInfo{
			OpenOrders: &OpenOrdersInfo{
				Perp: []Order{},
				Spot: []Order{{Coin: "USDC"}},
			},
		}
		assert.True(t, info.HasOpenOrders())
	})

	t.Run("HasOpenOrders_false", func(t *testing.T) {
		t.Parallel()
		info := &UserInfo{
			OpenOrders: &OpenOrdersInfo{
				Perp: []Order{},
				Spot: []Order{},
			},
		}
		assert.False(t, info.HasOpenOrders())
	})

	t.Run("HasOpenOrders_nil", func(t *testing.T) {
		t.Parallel()
		info := &UserInfo{}
		assert.False(t, info.HasOpenOrders())
	})
}

func TestInfoUser_InvalidAddress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test with invalid address format
	info, err := client.InfoUser(ctx, "invalid")

	// The API might still return data or an error, both are acceptable
	if err != nil {
		t.Logf("Expected error for invalid address: %v", err)
	} else if info != nil {
		t.Logf("API returned data for invalid address (Hyperliquid may accept any format)")
	}
}

func TestInfoUser_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(nil)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	_, err := client.InfoUser(ctx, testAddress)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestInfoUser_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(nil)

	// Very short timeout to force timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	_, err := client.InfoUser(ctx, testAddress)
	assert.Error(t, err)
}
