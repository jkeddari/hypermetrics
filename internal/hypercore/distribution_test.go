package hypercore

import (
	"context"
	"testing"
	"time"
)

func TestFinalizePositionDistribution(t *testing.T) {
	bucket := finalizePositionDistribution(PositionDistributionBucket{
		AllAddressCount:      4,
		PositionAddressCount: 3,
		LongPositionUSD:      300,
		ShortPositionUSD:     100,
		ProfitAddressCount:   2,
		LossAddressCount:     1,
	}, 2, 1)

	if bucket.PositionAddressPercent != 75 {
		t.Fatalf("expected 75%% active addresses, got %v", bucket.PositionAddressPercent)
	}
	if bucket.PositionUSD != 400 || bucket.LongPositionUSDPercent != 75 || bucket.ShortPositionUSDPercent != 25 {
		t.Fatalf("unexpected position totals: %+v", bucket)
	}
	if bucket.ProfitAddressPercent != 66.67 || bucket.LossAddressPercent != 33.33 {
		t.Fatalf("unexpected PnL percentages: %+v", bucket)
	}
	if bucket.BiasScore != 0.33 || bucket.BiasRemark != "bullish" {
		t.Fatalf("unexpected bias: %+v", bucket)
	}

	empty := finalizePositionDistribution(PositionDistributionBucket{}, 0, 0)
	if empty.PositionAddressPercent != 0 || empty.LongPositionUSDPercent != 0 || empty.BiasRemark != "indecisive" {
		t.Fatalf("expected zero-safe empty bucket, got %+v", empty)
	}
}

func TestListWalletPositionDistribution(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{Now: func() time.Time { return now }}
	store := openTestStore(t, cfg)

	saveDistributionWallet(t, store, cfg, "0x0000000000000000000000000000000000000101", 100, nil)
	saveDistributionWallet(t, store, cfg, "0x0000000000000000000000000000000000000102", 249.99, []WalletPosition{
		{Symbol: "BTC", PositionSize: 1, PositionValueUSD: 100, UnrealizedPnL: 10},
	})
	saveDistributionWallet(t, store, cfg, "0x0000000000000000000000000000000000000103", 250, []WalletPosition{
		{Symbol: "ETH", PositionSize: -1, PositionValueUSD: 500, UnrealizedPnL: -20},
	})
	saveDistributionWallet(t, store, cfg, "0x0000000000000000000000000000000000000104", 1_000_000, []WalletPosition{
		{Symbol: "BTC", PositionSize: 1, PositionValueUSD: 600, UnrealizedPnL: 20},
		{Symbol: "ETH", PositionSize: -1, PositionValueUSD: 100, UnrealizedPnL: -5},
	})
	saveDistributionWallet(t, store, cfg, "0x0000000000000000000000000000000000000105", 100_000_000, nil)
	saveDistributionWallet(t, store, cfg, "0x0000000000000000000000000000000000000106", 10_000, []WalletPosition{
		{Symbol: "BTC", PositionSize: 1, PositionValueUSD: 200_000, UnrealizedPnL: 150_000},
	})
	saveDistributionWallet(t, store, cfg, "0x0000000000000000000000000000000000000107", 10_000, []WalletPosition{
		{Symbol: "ETH", PositionSize: -1, PositionValueUSD: 300_000, UnrealizedPnL: -150_000},
	})

	buckets, err := store.ListWalletPositionDistribution()
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 8 {
		t.Fatalf("expected 8 stable buckets, got %d", len(buckets))
	}

	shrimp := buckets[0]
	if shrimp.GroupName != "shrimp" || shrimp.AllAddressCount != 2 || shrimp.PositionAddressCount != 1 {
		t.Fatalf("unexpected shrimp bucket: %+v", shrimp)
	}
	if shrimp.PositionAddressPercent != 50 || shrimp.LongPositionUSD != 100 || shrimp.ProfitAddressCount != 1 {
		t.Fatalf("unexpected shrimp metrics: %+v", shrimp)
	}

	fish := buckets[1]
	if fish.MinimumAmount != 250 || fish.AllAddressCount != 1 || fish.ShortPositionUSD != 500 || fish.BiasRemark != "bearish" {
		t.Fatalf("expected boundary wallet in fish bucket, got %+v", fish)
	}

	whale := buckets[5]
	if whale.AllAddressCount != 1 || whale.PositionAddressCount != 1 || whale.PositionUSD != 700 {
		t.Fatalf("expected multi-position wallet to count once, got %+v", whale)
	}

	leviathan := buckets[7]
	if leviathan.MinimumAmount != 100_000_000 || leviathan.MaximumAmount != 0 || leviathan.AllAddressCount != 1 {
		t.Fatalf("unexpected unbounded leviathan bucket: %+v", leviathan)
	}
}

func TestListWalletPnLDistribution(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{Now: func() time.Time { return now }}
	store := openTestStore(t, cfg)

	saveDistributionWallet(t, store, cfg, "0x0000000000000000000000000000000000000201", 500_000, []WalletPosition{
		{Symbol: "BTC", PositionSize: 1, PositionValueUSD: 200_000, UnrealizedPnL: 150_000},
	})
	saveDistributionWallet(t, store, cfg, "0x0000000000000000000000000000000000000202", 500_000, []WalletPosition{
		{Symbol: "ETH", PositionSize: -1, PositionValueUSD: 50_000, UnrealizedPnL: 50_000},
	})
	saveDistributionWallet(t, store, cfg, "0x0000000000000000000000000000000000000203", 500_000, []WalletPosition{
		{Symbol: "SOL", PositionSize: -1, PositionValueUSD: 75_000, UnrealizedPnL: -150_000},
	})
	saveDistributionWallet(t, store, cfg, "0x0000000000000000000000000000000000000204", 500_000, nil)

	buckets, err := store.ListWalletPnLDistribution()
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 8 {
		t.Fatalf("expected 8 stable PnL buckets, got %d", len(buckets))
	}

	moneyPrinter := buckets[0]
	if moneyPrinter.GroupName != "money_printer" || moneyPrinter.AllAddressCount != 1 || moneyPrinter.LongPositionUSD != 200_000 {
		t.Fatalf("unexpected money printer bucket: %+v", moneyPrinter)
	}
	smartMoney := buckets[1]
	if smartMoney.GroupName != "smart_money" || smartMoney.AllAddressCount != 1 || smartMoney.ProfitAddressCount != 1 {
		t.Fatalf("unexpected smart money bucket: %+v", smartMoney)
	}
	humbleEarner := buckets[3]
	if humbleEarner.AllAddressCount != 1 || humbleEarner.PositionAddressCount != 0 {
		t.Fatalf("expected inactive zero-PnL wallet in humble earner: %+v", humbleEarner)
	}
	gigaRekt := buckets[7]
	if gigaRekt.GroupName != "giga_rekt" || gigaRekt.AllAddressCount != 1 || gigaRekt.LossAddressCount != 1 {
		t.Fatalf("unexpected giga rekt bucket: %+v", gigaRekt)
	}
}

func TestDistributionSnapshotRoundTrip(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{Now: func() time.Time { return now }}
	store := openTestStore(t, cfg)

	saveDistributionWallet(t, store, cfg, "0x0000000000000000000000000000000000000301", 500_000, []WalletPosition{
		{Symbol: "BTC", PositionSize: 1, PositionValueUSD: 200_000, UnrealizedPnL: 150_000},
	})

	if err := store.RefreshDistributionSnapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	position, pnl, computedAt, err := store.GetDistributionSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if computedAt.IsZero() || len(position) != 8 || len(pnl) != 8 {
		t.Fatalf("unexpected snapshot: computed_at=%v position=%d pnl=%d", computedAt, len(position), len(pnl))
	}
	if position[4].PositionUSD != 200_000 || pnl[0].LongPositionUSD != 200_000 {
		t.Fatalf("unexpected snapshot values: position=%+v pnl=%+v", position[4], pnl[0])
	}
}

func TestLongShortAccountRatioHistory(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{Now: func() time.Time { return now }}
	store := openTestStore(t, cfg)

	saveDistributionWallet(t, store, cfg, "0x0000000000000000000000000000000000000401", 500_000, []WalletPosition{
		{Symbol: "BTC", PositionSize: 1, PositionValueUSD: 100_000},
	})
	saveDistributionWallet(t, store, cfg, "0x0000000000000000000000000000000000000402", 500_000, []WalletPosition{
		{Symbol: "BTC", PositionSize: -1, PositionValueUSD: 100_000},
	})
	saveDistributionWallet(t, store, cfg, "0x0000000000000000000000000000000000000403", 500_000, []WalletPosition{
		{Symbol: "BTC", PositionSize: 0, PositionValueUSD: 0},
	})

	if err := store.RefreshDistributionSnapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	snapshots, err := store.ListLongShortAccountRatioHistory(context.Background(), "btc", "1h", 1000, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected one BTC snapshot, got %d", len(snapshots))
	}
	snapshot := snapshots[0]
	if snapshot.Symbol != "BTC" || snapshot.PositionedWalletCount != 2 || snapshot.LongWalletCount != 1 || snapshot.ShortWalletCount != 1 {
		t.Fatalf("unexpected ratio snapshot: %+v", snapshot)
	}
}

func saveDistributionWallet(
	t *testing.T,
	store *Store,
	cfg PriorityConfig,
	address string,
	accountValue float64,
	positions []WalletPosition,
) {
	t.Helper()
	now := cfg.now()
	for i := range positions {
		positions[i].Address = address
		positions[i].RefreshedAt = now
	}
	state := WalletState{
		Account: WalletAccount{
			Address:       address,
			AccountValue:  accountValue,
			RefreshedAt:   now,
			RawReceivedAt: now,
		},
		Positions: positions,
	}
	wallet := applyRefreshSuccess(Wallet{
		Address:     address,
		FirstSeenAt: now,
	}, state, cfg)
	if err := store.SaveWalletState(wallet, state); err != nil {
		t.Fatal(err)
	}
}
