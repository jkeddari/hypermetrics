package hypercore

import (
	"context"
	"testing"
	"time"
)

func TestImportS3WalletSignalsIsAtomicAndIdempotent(t *testing.T) {
	now := time.Date(2025, 7, 28, 14, 0, 0, 0, time.UTC)
	store := openTestStore(t, PriorityConfig{Now: func() time.Time { return now }})
	signals := []WalletSignal{
		{
			Address: "0x1111111111111111111111111111111111111111",
			Source:  SourceS3,
			SeenAt:  now.Add(-time.Minute),
		},
		{
			Address: "0x2222222222222222222222222222222222222222",
			Source:  SourceS3,
			SeenAt:  now,
		},
	}

	newWallets, err := store.ImportS3WalletSignals(
		context.Background(),
		"node_fills_by_block/hourly/20250728/14.lz4",
		`"etag-1"`,
		signals,
	)
	if err != nil {
		t.Fatal(err)
	}
	if newWallets != 2 {
		t.Fatalf("expected 2 new wallets, got %d", newWallets)
	}

	wallet, err := store.GetWallet(signals[0].Address)
	if err != nil {
		t.Fatal(err)
	}
	if len(wallet.Sources) != 1 || wallet.Sources[0] != SourceS3 {
		t.Fatalf("unexpected wallet sources: %v", wallet.Sources)
	}

	var queued int
	if err := store.db.QueryRow(`
		SELECT count(*)
		FROM wallet_refresh_queue
		WHERE wallet_address = ANY($1::text[])`,
		`{`+signals[0].Address+`,`+signals[1].Address+`}`,
	).Scan(&queued); err != nil {
		t.Fatal(err)
	}
	if queued != 2 {
		t.Fatalf("expected 2 queued wallets, got %d", queued)
	}

	newWallets, err = store.ImportS3WalletSignals(
		context.Background(),
		"node_fills_by_block/hourly/20250728/14.lz4",
		`"etag-1"`,
		signals,
	)
	if err != nil {
		t.Fatal(err)
	}
	if newWallets != 0 {
		t.Fatalf("expected idempotent replay, got %d new wallets", newWallets)
	}

	processed, err := store.S3ObjectProcessed(
		context.Background(),
		"node_fills_by_block/hourly/20250728/14.lz4",
		`"etag-1"`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("expected object to be marked processed")
	}
}
