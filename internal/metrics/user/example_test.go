package user_test

import (
	"context"
	"fmt"
	"log"

	"github.com/jkeddari/hypermetrics/internal/metrics/user"
)

// ExampleClient_InfoUser demonstrates how to fetch comprehensive user information.
func ExampleClient_InfoUser() {
	// Create a new Hyperliquid client
	client := user.NewClient(nil)

	// Fetch user information
	ctx := context.Background()
	info, err := client.InfoUser(ctx, "0x00c9a8023b6e1f2b761f5c111dd1c785adb0c0b4")
	if err != nil {
		log.Fatal(err)
	}

	// Access account summary
	fmt.Printf("Account Value: %s USDC\n", info.GetTotalAccountValue())
	fmt.Printf("Withdrawable: %s USDC\n", info.GetWithdrawable())

	// Check positions
	if info.HasOpenPositions() {
		fmt.Printf("Open Positions: %d\n", len(info.GetPerpPositions()))
		for _, pos := range info.GetPerpPositions() {
			fmt.Printf("  %s: Size=%s, UnrealizedPnL=%s\n",
				pos.Position.Coin,
				pos.Position.Szi,
				pos.Position.UnrealizedPnl,
			)
		}
	}

	// Check spot balances
	if len(info.GetSpotBalances()) > 0 {
		fmt.Printf("Spot Balances: %d tokens\n", len(info.GetSpotBalances()))
		for _, balance := range info.GetSpotBalances() {
			fmt.Printf("  %s: %s (hold: %s)\n",
				balance.Coin,
				balance.Total,
				balance.Hold,
			)
		}
	}

	// Check open orders
	if info.HasOpenOrders() {
		fmt.Printf("Open Orders: %d\n",
			len(info.OpenOrders.Perp)+len(info.OpenOrders.Spot))
	}
}

// ExampleUserInfo_GetPerpPositions shows how to access perpetual positions.
func ExampleUserInfo_GetPerpPositions() {
	// Assuming you have a UserInfo object
	var info *user.UserInfo

	// Get all perpetual positions
	positions := info.GetPerpPositions()

	for _, pos := range positions {
		fmt.Printf("Position: %s\n", pos.Position.Coin)
		fmt.Printf("  Size: %s\n", pos.Position.Szi)
		fmt.Printf("  Entry Price: %s\n", pos.Position.EntryPx)
		fmt.Printf("  Unrealized PnL: %s\n", pos.Position.UnrealizedPnl)
		fmt.Printf("  Liquidation Price: %s\n", pos.Position.LiquidationPx)
	}
}

// ExampleUserInfo_GetSpotBalances shows how to access spot balances.
func ExampleUserInfo_GetSpotBalances() {
	// Assuming you have a UserInfo object
	var info *user.UserInfo

	// Get all spot balances
	balances := info.GetSpotBalances()

	for _, balance := range balances {
		fmt.Printf("Token: %s\n", balance.Coin)
		fmt.Printf("  Total: %s\n", balance.Total)
		fmt.Printf("  Hold: %s\n", balance.Hold)
		fmt.Printf("  Entry Notional: %s\n", balance.EntryNtl)
	}
}
