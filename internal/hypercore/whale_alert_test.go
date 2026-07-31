package hypercore

import (
	"fmt"
	"testing"
	"time"
)

func TestDetectWhaleAlerts(t *testing.T) {
	address := "0x0000000000000000000000000000000000000301"
	at := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	previous := []WalletPosition{
		{Address: address, Symbol: "BTC", PositionSize: 1, PositionValueUSD: 900_000},
		{Address: address, Symbol: "ETH", PositionSize: -1, PositionValueUSD: 1_200_000},
		{Address: address, Symbol: "SOL", PositionSize: 1, PositionValueUSD: 1_300_000},
		{Address: address, Symbol: "HYPE", PositionSize: 1, PositionValueUSD: 1_400_000},
	}
	current := []WalletPosition{
		{Address: address, Symbol: "BTC", PositionSize: 1, PositionValueUSD: 1_100_000},
		{Address: address, Symbol: "SOL", PositionSize: -1, PositionValueUSD: 1_500_000},
		{Address: address, Symbol: "HYPE", PositionSize: 1, PositionValueUSD: 1_600_000},
	}

	alerts := detectWhaleAlerts(previous, current, 1_000_000, at)
	got := make(map[string]WhaleAlert, len(alerts))
	for _, alert := range alerts {
		got[fmt.Sprintf("%s/%d", alert.Symbol, alert.PositionAction)] = alert
	}
	for _, key := range []string{"BTC/1", "ETH/2", "SOL/1", "SOL/2"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("missing alert %s in %+v", key, alerts)
		}
	}
	if len(alerts) != 4 {
		t.Fatalf("expected four threshold transitions, got %+v", alerts)
	}
}

func TestWhaleAlertsPersistAcrossWalletRefreshes(t *testing.T) {
	times := []time.Time{
		time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 31, 12, 5, 0, 0, time.UTC),
		time.Date(2026, 7, 31, 12, 10, 0, 0, time.UTC),
	}
	now := times[0]
	cfg := PriorityConfig{WhaleThresholdUSD: 1_000_000, Now: func() time.Time { return now }}
	store := openTestStore(t, cfg)
	address := "0x0000000000000000000000000000000000000302"

	saveAlertState(t, store, cfg, address, times[0], []WalletPosition{
		{Symbol: "BTC", PositionSize: 1, EntryPrice: 50_000, LiqPrice: 30_000, PositionValueUSD: 900_000},
	})
	now = times[1]
	saveAlertState(t, store, cfg, address, times[1], []WalletPosition{
		{Symbol: "BTC", PositionSize: 1, EntryPrice: 50_000, LiqPrice: 30_000, PositionValueUSD: 1_200_000},
	})
	now = times[2]
	saveAlertState(t, store, cfg, address, times[2], nil)

	alerts, err := store.ListWhaleAlerts(200)
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 2 {
		t.Fatalf("expected open and close alerts, got %+v", alerts)
	}
	if alerts[0].PositionAction != WhalePositionClosed || !alerts[0].CreatedAt.Equal(times[2]) {
		t.Fatalf("expected newest close first, got %+v", alerts[0])
	}
	if alerts[1].PositionAction != WhalePositionOpened || !alerts[1].CreatedAt.Equal(times[1]) {
		t.Fatalf("expected threshold open second, got %+v", alerts[1])
	}
}

func saveAlertState(t *testing.T, store *Store, cfg PriorityConfig, address string, refreshedAt time.Time, positions []WalletPosition) {
	t.Helper()
	state := WalletState{
		Account:   WalletAccount{Address: address, AccountValue: 500_000, RefreshedAt: refreshedAt, RawReceivedAt: refreshedAt},
		Positions: positions,
	}
	for i := range state.Positions {
		state.Positions[i].Address = address
		state.Positions[i].RefreshedAt = refreshedAt
	}
	wallet, err := store.GetWallet(address)
	if err != nil && err != ErrWalletNotFound {
		t.Fatal(err)
	}
	if err == ErrWalletNotFound {
		wallet = Wallet{Address: address, FirstSeenAt: refreshedAt}
	}
	wallet = ApplyRefreshSuccess(wallet, state, cfg)
	if err := store.SaveWalletState(wallet, state); err != nil {
		t.Fatal(err)
	}
}
