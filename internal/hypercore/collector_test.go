package hypercore

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCollectLeaderboardStoresOnlyCandidatesAtOrAboveWhaleThreshold(t *testing.T) {
	store := openTestStore(t, PriorityConfig{WhaleThresholdUSD: 1_000_000})

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `[
			{"address":"0x0000000000000000000000000000000000000001","accountValue":1500000},
			{"address":"0x0000000000000000000000000000000000000002","accountValue":1000000},
			{"address":"0x0000000000000000000000000000000000000003","accountValue":999999},
			{"address":"0x0000000000000000000000000000000000000004"}
		]`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})}

	collector := NewCollector(store, client)
	stats, err := collector.CollectLeaderboard(context.Background(), "https://example.test/leaderboard")
	if err != nil {
		t.Fatal(err)
	}
	if stats.EligibleAddresses != 2 {
		t.Fatalf("expected 2 eligible addresses, got %d", stats.EligibleAddresses)
	}
	if stats.NewPotentialAddresses != 2 {
		t.Fatalf("expected 2 new potential addresses, got %d", stats.NewPotentialAddresses)
	}

	wallets, err := store.ListWallets()
	if err != nil {
		t.Fatal(err)
	}
	if len(wallets) != 0 {
		t.Fatalf("expected no stored wallets before validation scan, got %d", len(wallets))
	}

	candidates, err := store.ListCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 stored candidates, got %d", len(candidates))
	}

	stored := make(map[string]WalletCandidate, len(candidates))
	for _, candidate := range candidates {
		stored[candidate.Address] = candidate
	}

	if stored["0x0000000000000000000000000000000000000001"].LeaderboardAccountValue != 1_500_000 {
		t.Fatalf("expected candidate above threshold to be stored")
	}
	if stored["0x0000000000000000000000000000000000000002"].LeaderboardAccountValue != 1_000_000 {
		t.Fatalf("expected candidate at threshold to be stored")
	}
	if _, ok := stored["0x0000000000000000000000000000000000000003"]; ok {
		t.Fatalf("expected candidate below threshold to be ignored")
	}
	if _, ok := stored["0x0000000000000000000000000000000000000004"]; ok {
		t.Fatalf("expected candidate without account value to be ignored")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
