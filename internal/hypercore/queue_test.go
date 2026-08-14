package hypercore

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPriorityScorePrefersKnownWhale(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{
		WhaleThresholdUSD: 1_000_000,
		Now: func() time.Time {
			return now
		},
	}

	cold := Wallet{
		Address:         "0x0000000000000000000000000000000000000001",
		FirstSeenAt:     now.Add(-24 * time.Hour),
		LastRefreshedAt: now.Add(-24 * time.Hour),
		Tier:            TierCold,
	}
	whale := Wallet{
		Address:             "0x0000000000000000000000000000000000000002",
		FirstSeenAt:         now.Add(-24 * time.Hour),
		LastRefreshedAt:     now.Add(-2 * time.Minute),
		MaxPositionValueUSD: 2_000_000,
		KnownWhale:          true,
		HasOpenPosition:     true,
	}

	if computeTier(whale, cfg) != TierKnownWhale {
		t.Fatalf("expected known whale tier")
	}
	if computePriorityScore(whale, cfg) <= computePriorityScore(cold, cfg) {
		t.Fatalf("expected known whale score to beat cold wallet")
	}
}

func TestPriorityScorePrefersUnscannedHighAccountValue(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{
		WhaleThresholdUSD: 1_000_000,
		Now: func() time.Time {
			return now
		},
	}

	knownWhale := Wallet{
		Address:               "0x0000000000000000000000000000000000000001",
		FirstSeenAt:           now.Add(-24 * time.Hour),
		LastRefreshedAt:       now.Add(-2 * time.Minute),
		LastSuccessfulRefresh: now.Add(-2 * time.Minute),
		MaxPositionValueUSD:   2_000_000,
		KnownWhale:            true,
		HasOpenPosition:       true,
	}
	unscannedHighValue := Wallet{
		Address:                 "0x0000000000000000000000000000000000000002",
		FirstSeenAt:             now.Add(-time.Minute),
		LastSeenLeaderboardAt:   now.Add(-time.Minute),
		LeaderboardRank:         300,
		LeaderboardAccountValue: 1_500_000,
		AccountValue:            1_500_000,
	}

	if computePriorityScore(unscannedHighValue, cfg) <= computePriorityScore(knownWhale, cfg) {
		t.Fatalf("expected unscanned high account value wallet to beat known whale")
	}

	expectedMinimum := 10_000 + 1_500
	if computePriorityScore(unscannedHighValue, cfg) < float64(expectedMinimum) {
		t.Fatalf("expected proportional unscanned value score to be included")
	}
}

func TestSelectNextDueWalletPrefersUnscannedHighValue(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{
		WhaleThresholdUSD: 1_000_000,
		Now: func() time.Time {
			return now
		},
	}

	store := openTestStore(t, cfg)

	knownWhale := Wallet{
		Address:             "0x0000000000000000000000000000000000000001",
		FirstSeenAt:         now.Add(-24 * time.Hour),
		LastRefreshedAt:     now.Add(-2 * time.Minute),
		MaxPositionValueUSD: 2_000_000,
		KnownWhale:          true,
		HasOpenPosition:     true,
		NextRefreshAt:       now.Add(-time.Second),
	}
	if err := store.SaveWallet(knownWhale); err != nil {
		t.Fatal(err)
	}

	if _, err := store.UpsertWalletSignal(WalletSignal{
		Address:                 "0x0000000000000000000000000000000000000002",
		Source:                  SourceLeaderboard,
		SeenAt:                  now.Add(-time.Minute),
		LeaderboardRank:         300,
		LeaderboardAccountValue: 1_500_000,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := store.UpsertWalletSignal(WalletSignal{
		Address:                 "0x0000000000000000000000000000000000000003",
		Source:                  SourceLeaderboard,
		SeenAt:                  now.Add(-time.Minute),
		LeaderboardRank:         10,
		LeaderboardAccountValue: 50_000,
	}); err != nil {
		t.Fatal(err)
	}

	job, found, err := store.ClaimNextJob("test-worker", false, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected due wallet")
	}
	selected, err := store.GetWallet(job.Address)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Address != "0x0000000000000000000000000000000000000002" {
		t.Fatalf("expected high value unscanned wallet, got %s", selected.Address)
	}
}

func TestFailedUnscannedHighValueWalletBacksOff(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{
		WhaleThresholdUSD: 1_000_000,
		Now: func() time.Time {
			return now
		},
	}

	store := openTestStore(t, cfg)

	wallet, err := store.UpsertWalletSignal(WalletSignal{
		Address:                 "0x0000000000000000000000000000000000000001",
		Source:                  SourceLeaderboard,
		SeenAt:                  now.Add(-time.Minute),
		LeaderboardAccountValue: 10_000_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	wallet = applyRefreshFailure(wallet, context.DeadlineExceeded, cfg)
	if err := store.SaveWallet(wallet); err != nil {
		t.Fatal(err)
	}

	if _, err := store.UpsertWalletSignal(WalletSignal{
		Address:                 "0x0000000000000000000000000000000000000002",
		Source:                  SourceLeaderboard,
		SeenAt:                  now.Add(-time.Minute),
		LeaderboardAccountValue: 500_000,
	}); err != nil {
		t.Fatal(err)
	}

	job, found, err := store.ClaimNextJob("test-worker", false, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected due wallet")
	}
	selected, err := store.GetWallet(job.Address)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Address != "0x0000000000000000000000000000000000000002" {
		t.Fatalf("expected backed off wallet to be skipped, got %s", selected.Address)
	}
}

func TestLeaderboardRepollMakesUnscannedHighValueWalletDue(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{
		WhaleThresholdUSD: 1_000_000,
		Now: func() time.Time {
			return now
		},
	}

	wallet := Wallet{
		Address:       "0x0000000000000000000000000000000000000001",
		FirstSeenAt:   now.Add(-time.Hour),
		NextRefreshAt: now.Add(time.Hour),
	}

	wallet = mergeWalletSignal(wallet, WalletSignal{
		Address:                 wallet.Address,
		Source:                  SourceLeaderboard,
		SeenAt:                  now,
		LeaderboardAccountValue: 1_000_000,
	}, cfg)

	if !wallet.NextRefreshAt.Equal(now) {
		t.Fatalf("expected next refresh to be reset to now, got %s", wallet.NextRefreshAt)
	}
}

func TestLeaderboardRepollKeepsFailedHighValueWalletBackoff(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{
		WhaleThresholdUSD: 1_000_000,
		Now: func() time.Time {
			return now
		},
	}

	nextRefreshAt := now.Add(10 * time.Minute)
	wallet := Wallet{
		Address:            "0x0000000000000000000000000000000000000001",
		FirstSeenAt:        now.Add(-time.Hour),
		ConsecutiveFailure: 2,
		NextRefreshAt:      nextRefreshAt,
	}

	wallet = mergeWalletSignal(wallet, WalletSignal{
		Address:                 wallet.Address,
		Source:                  SourceLeaderboard,
		SeenAt:                  now,
		LeaderboardAccountValue: 1_000_000,
	}, cfg)

	if !wallet.NextRefreshAt.Equal(nextRefreshAt) {
		t.Fatalf("expected backoff to be preserved, got %s", wallet.NextRefreshAt)
	}
}

func TestMaxRefreshIntervalCapsCoverageDelay(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{WhaleThresholdUSD: 1_000_000}

	tests := []struct {
		name   string
		wallet Wallet
		want   time.Duration
	}{
		{"ten million", Wallet{MaxPositionValueUSD: 10_000_000}, time.Minute},
		{"five million", Wallet{MaxPositionValueUSD: 5_000_000}, 2 * time.Minute},
		{"whale", Wallet{MaxPositionValueUSD: 1_000_000}, 5 * time.Minute},
		{"leaderboard", Wallet{LastSeenLeaderboardAt: now}, 24 * time.Hour},
		{"cold", Wallet{}, 48 * time.Hour},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := maxRefreshInterval(test.wallet, cfg); got != test.want {
				t.Fatalf("expected %s, got %s", test.want, got)
			}
		})
	}
}

func TestIsTrackableStateAcceptsWhalePositionWithSmallAccount(t *testing.T) {
	state := WalletState{
		Account: WalletAccount{AccountValue: 10_000},
		Positions: []WalletPosition{
			{PositionValueUSD: -1_200_000},
		},
	}
	if !isTrackableState(state, 1_000_000) {
		t.Fatal("expected absolute whale position value to make wallet trackable")
	}
}

func TestRefreshQueueRefreshOne(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{
		WhaleThresholdUSD: 1_000_000,
		Now: func() time.Time {
			return now
		},
	}

	store := openTestStore(t, cfg)

	address := "0x0000000000000000000000000000000000000001"
	if _, err := store.UpsertWalletSignal(WalletSignal{
		Address: address,
		Source:  SourceManual,
		SeenAt:  now.Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	queue := NewRefreshQueue(store, fakeWalletStateClient{
		state: WalletState{
			Account: WalletAccount{
				Address:      address,
				AccountValue: 2_000_000,
			},
			Positions: []WalletPosition{
				{
					Address:          address,
					Symbol:           "BTC",
					PositionSize:     20,
					PositionValueUSD: 2_000_000,
				},
			},
		},
	}, QueueConfig{Priority: cfg})

	refreshed, err := queue.RefreshOne(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed {
		t.Fatal("expected wallet to be refreshed")
	}

	wallet, err := store.GetWallet(address)
	if err != nil {
		t.Fatal(err)
	}
	if !wallet.KnownWhale {
		t.Fatal("expected wallet to be marked as known whale")
	}
}

func TestRefreshQueueRefreshOneRetainsIndexedWalletBelowThreshold(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{
		WhaleThresholdUSD: 1_000_000,
		Now: func() time.Time {
			return now
		},
	}

	store := openTestStore(t, cfg)

	address := "0x0000000000000000000000000000000000000001"
	wallet, err := store.UpsertWalletSignal(WalletSignal{
		Address:                 address,
		Source:                  SourceLeaderboard,
		SeenAt:                  now.Add(-time.Minute),
		LeaderboardAccountValue: 1_500_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	wallet = applyRefreshSuccess(wallet, WalletState{
		Account: WalletAccount{
			Address:      address,
			AccountValue: 1_500_000,
		},
		Positions: []WalletPosition{
			{
				Address:          address,
				Symbol:           "BTC",
				PositionSize:     1,
				PositionValueUSD: 1_500_000,
			},
		},
	}, cfg)
	wallet.NextRefreshAt = now.Add(-time.Second)
	if err := store.SaveWalletState(wallet, WalletState{
		Account: WalletAccount{
			Address:      address,
			AccountValue: 1_500_000,
		},
		Positions: []WalletPosition{
			{
				Address:          address,
				Symbol:           "BTC",
				PositionSize:     1,
				PositionValueUSD: 1_500_000,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	queue := NewRefreshQueue(store, fakeWalletStateClient{
		state: WalletState{
			Account: WalletAccount{
				Address:      address,
				AccountValue: 999_999,
			},
		},
	}, QueueConfig{Priority: cfg})

	refreshed, err := queue.RefreshOne(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed {
		t.Fatal("expected wallet to be refreshed")
	}

	storedWallet, err := store.GetWallet(address)
	if err != nil {
		t.Fatalf("expected wallet to be retained, got %v", err)
	}
	if storedWallet.AccountValue != 999_999 {
		t.Fatalf("expected refreshed account value, got %f", storedWallet.AccountValue)
	}
	storedState, err := store.GetWalletState(address)
	if err != nil {
		t.Fatalf("expected wallet state to be retained, got %v", err)
	}
	if storedState.Account.AccountValue != 999_999 {
		t.Fatalf("expected current state to be replaced, got %f", storedState.Account.AccountValue)
	}
	positions, err := store.ListPositions()
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 0 {
		t.Fatalf("expected wallet positions to be deleted, got %d", len(positions))
	}
}

func TestRefreshQueuePromotesLeaderboardCandidateAboveRealAccountValueThreshold(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{
		WhaleThresholdUSD: 1_000_000,
		Now: func() time.Time {
			return now
		},
	}

	store := openTestStore(t, cfg)

	address := "0x0000000000000000000000000000000000000001"
	if _, _, _, _, err := store.UpsertLeaderboardCandidate(WalletSignal{
		Address:                 address,
		Source:                  SourceLeaderboard,
		SeenAt:                  now.Add(-time.Minute),
		LeaderboardRank:         12,
		LeaderboardAccountValue: 1_500_000,
	}); err != nil {
		t.Fatal(err)
	}

	queue := NewRefreshQueue(store, fakeWalletStateClient{
		state: WalletState{
			Account: WalletAccount{
				Address:      address,
				AccountValue: 1_400_000,
			},
			Positions: []WalletPosition{
				{
					Address:          address,
					Symbol:           "ETH",
					PositionSize:     10,
					PositionValueUSD: 1_400_000,
				},
			},
		},
	}, QueueConfig{Priority: cfg})

	refreshed, err := queue.RefreshOne(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed {
		t.Fatal("expected candidate to be refreshed")
	}

	candidates, err := store.ListCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected candidate to be removed after promotion, got %d", len(candidates))
	}

	wallet, err := store.GetWallet(address)
	if err != nil {
		t.Fatal(err)
	}
	if wallet.AccountValue != 1_400_000 {
		t.Fatalf("expected promoted wallet account value 1400000, got %f", wallet.AccountValue)
	}
	if wallet.LeaderboardAccountValue != 1_500_000 {
		t.Fatalf("expected leaderboard metadata to be preserved")
	}
}

func TestRefreshQueueDiscardsLeaderboardCandidateBelowRealAccountValueThreshold(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{
		WhaleThresholdUSD: 1_000_000,
		Now: func() time.Time {
			return now
		},
	}

	store := openTestStore(t, cfg)

	address := "0x0000000000000000000000000000000000000001"
	if _, _, _, _, err := store.UpsertLeaderboardCandidate(WalletSignal{
		Address:                 address,
		Source:                  SourceLeaderboard,
		SeenAt:                  now.Add(-time.Minute),
		LeaderboardRank:         12,
		LeaderboardAccountValue: 1_500_000,
	}); err != nil {
		t.Fatal(err)
	}

	queue := NewRefreshQueue(store, fakeWalletStateClient{
		state: WalletState{
			Account: WalletAccount{
				Address:      address,
				AccountValue: 999_999,
			},
		},
	}, QueueConfig{Priority: cfg})

	refreshed, err := queue.RefreshOne(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed {
		t.Fatal("expected candidate to be refreshed")
	}

	candidates, err := store.ListCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected candidate to be discarded, got %d", len(candidates))
	}
	if _, err := store.GetWallet(address); err != ErrWalletNotFound {
		t.Fatalf("expected wallet not to be created, got %v", err)
	}

	if _, isNew, isExistingWallet, isRejected, err := store.UpsertLeaderboardCandidate(WalletSignal{
		Address:                 address,
		Source:                  SourceLeaderboard,
		SeenAt:                  now.Add(time.Minute),
		LeaderboardRank:         12,
		LeaderboardAccountValue: 1_500_000,
	}); err != nil {
		t.Fatal(err)
	} else if isNew || isExistingWallet || !isRejected {
		t.Fatalf("expected rejected candidate to be skipped, got isNew=%v isExistingWallet=%v isRejected=%v", isNew, isExistingWallet, isRejected)
	}

	candidates, err = store.ListCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected rejected candidate not to be requeued, got %d", len(candidates))
	}
}

func TestRejectedLeaderboardCandidateCanBeRequeuedAfterTTL(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{
		WhaleThresholdUSD:    1_000_000,
		RejectedCandidateTTL: 12 * time.Hour,
		Now: func() time.Time {
			return now
		},
	}

	store := openTestStore(t, cfg)

	address := "0x0000000000000000000000000000000000000001"
	if _, _, _, _, err := store.UpsertLeaderboardCandidate(WalletSignal{
		Address:                 address,
		Source:                  SourceLeaderboard,
		SeenAt:                  now,
		LeaderboardRank:         12,
		LeaderboardAccountValue: 1_500_000,
	}); err != nil {
		t.Fatal(err)
	}

	queue := NewRefreshQueue(store, fakeWalletStateClient{
		state: WalletState{
			Account: WalletAccount{
				Address:      address,
				AccountValue: 999_999,
			},
		},
	}, QueueConfig{Priority: cfg})

	if _, err := queue.RefreshOne(context.Background()); err != nil {
		t.Fatal(err)
	}

	now = now.Add(11*time.Hour + 59*time.Minute)
	if _, isNew, _, isRejected, err := store.UpsertLeaderboardCandidate(WalletSignal{
		Address:                 address,
		Source:                  SourceLeaderboard,
		SeenAt:                  now,
		LeaderboardRank:         12,
		LeaderboardAccountValue: 1_500_000,
	}); err != nil {
		t.Fatal(err)
	} else if isNew || !isRejected {
		t.Fatalf("expected rejected candidate to stay skipped before TTL, got isNew=%v isRejected=%v", isNew, isRejected)
	}

	now = now.Add(time.Minute)
	if _, isNew, _, isRejected, err := store.UpsertLeaderboardCandidate(WalletSignal{
		Address:                 address,
		Source:                  SourceLeaderboard,
		SeenAt:                  now,
		LeaderboardRank:         12,
		LeaderboardAccountValue: 1_500_000,
	}); err != nil {
		t.Fatal(err)
	} else if !isNew || isRejected {
		t.Fatalf("expected rejected candidate to be requeued after TTL, got isNew=%v isRejected=%v", isNew, isRejected)
	}
}

func TestRefreshQueueCoverageLanePreventsStarvation(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{
		WhaleThresholdUSD: 1_000_000,
		Now:               func() time.Time { return now },
	}
	store := openTestStore(t, cfg)

	priorityAddresses := []string{
		"0x0000000000000000000000000000000000000011",
		"0x0000000000000000000000000000000000000012",
		"0x0000000000000000000000000000000000000013",
		"0x0000000000000000000000000000000000000014",
	}
	for _, address := range priorityAddresses {
		if err := store.SaveWallet(Wallet{
			Address:             address,
			FirstSeenAt:         now.Add(-time.Hour),
			LastRefreshedAt:     now.Add(-time.Hour),
			MaxPositionValueUSD: 10_000_000,
			KnownWhale:          true,
			NextRefreshAt:       now.Add(-time.Second),
			PriorityScore:       1_000,
		}); err != nil {
			t.Fatal(err)
		}
	}
	coverageAddress := "0x0000000000000000000000000000000000000015"
	if err := store.SaveWallet(Wallet{
		Address:         coverageAddress,
		FirstSeenAt:     now.Add(-72 * time.Hour),
		LastRefreshedAt: now.Add(-72 * time.Hour),
		NextRefreshAt:   now.Add(-49 * time.Hour),
		PriorityScore:   1,
	}); err != nil {
		t.Fatal(err)
	}

	client := &recordingWalletStateClient{now: now}
	queue := NewRefreshQueue(store, client, QueueConfig{Priority: cfg})
	for range 5 {
		refreshed, err := queue.RefreshOne(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !refreshed {
			t.Fatal("expected due refresh")
		}
	}

	addresses := client.calledAddresses()
	if len(addresses) != 5 || addresses[4] != coverageAddress {
		t.Fatalf("expected fifth scan to use coverage lane for %s, got %v", coverageAddress, addresses)
	}
}

func TestClaimNextJobLeasesOneWalletToOneWorker(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	cfg := PriorityConfig{Now: func() time.Time { return now }}
	store := openTestStore(t, cfg)
	if err := store.SaveWallet(Wallet{
		Address:       "0x0000000000000000000000000000000000000021",
		FirstSeenAt:   now,
		NextRefreshAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	var claims atomic.Int64
	var wait sync.WaitGroup
	for worker := range 10 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, found, err := store.ClaimNextJob(string(rune('a'+worker)), false, time.Minute)
			if err != nil {
				t.Errorf("claim: %v", err)
				return
			}
			if found {
				claims.Add(1)
			}
		}()
	}
	wait.Wait()
	if claims.Load() != 1 {
		t.Fatalf("expected exactly one lease, got %d", claims.Load())
	}
}

func TestConcurrentOnDemandRefreshCallsUpstreamOnce(t *testing.T) {
	store := openTestStore(t, PriorityConfig{WhaleThresholdUSD: 1_000_000})
	address := "0x0000000000000000000000000000000000000031"
	client := &countingWalletStateClient{}
	queue := NewRefreshQueue(store, client, QueueConfig{RequestTimeout: 2 * time.Second})

	start := make(chan struct{})
	errors := make(chan error, 10)
	var wait sync.WaitGroup
	for range 10 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := queue.RefreshWallet(context.Background(), address)
			errors <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if client.calls.Load() != 1 {
		t.Fatalf("expected one upstream request, got %d", client.calls.Load())
	}
}

func TestRefreshQueueSharesRateLimitAcrossRequests(t *testing.T) {
	queue := &RefreshQueue{cfg: QueueConfig{RefreshRatePerSecond: 20}}
	if err := queue.waitForRequestSlot(context.Background()); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if err := queue.waitForRequestSlot(context.Background()); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed < 40*time.Millisecond {
		t.Fatalf("expected shared rate limit delay, got %s", elapsed)
	}
}

func TestRefreshQueueAppliesWeightedUpstreamRateLimit(t *testing.T) {
	queue := &RefreshQueue{cfg: QueueConfig{RefreshRatePerSecond: 20, UpstreamRequestWeight: 24}}
	if err := queue.waitForRequestSlot(context.Background()); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if err := queue.waitForRequestSlot(context.Background()); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed < time.Second {
		t.Fatalf("expected weighted rate limit delay, got %s", elapsed)
	}
}

type fakeWalletStateClient struct {
	state WalletState
	err   error
}

func (c fakeWalletStateClient) getClearinghouseState(context.Context, string) (WalletState, error) {
	return c.state, c.err
}

type recordingWalletStateClient struct {
	mu        sync.Mutex
	addresses []string
	now       time.Time
}

func (c *recordingWalletStateClient) getClearinghouseState(_ context.Context, address string) (WalletState, error) {
	c.mu.Lock()
	c.addresses = append(c.addresses, address)
	c.mu.Unlock()
	return WalletState{
		Account: WalletAccount{Address: address, AccountValue: 2_000_000, RefreshedAt: c.now, RawReceivedAt: c.now},
		Positions: []WalletPosition{
			{Address: address, Symbol: "BTC", PositionValueUSD: 2_000_000, RefreshedAt: c.now},
		},
	}, nil
}

func (c *recordingWalletStateClient) calledAddresses() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.addresses...)
}

type countingWalletStateClient struct {
	calls atomic.Int64
}

func (c *countingWalletStateClient) getClearinghouseState(_ context.Context, address string) (WalletState, error) {
	c.calls.Add(1)
	time.Sleep(75 * time.Millisecond)
	now := time.Now().UTC()
	return WalletState{
		Account: WalletAccount{
			Address:       address,
			AccountValue:  2_000_000,
			RefreshedAt:   now,
			RawReceivedAt: now,
		},
	}, nil
}

func NewRefreshQueue(store *Store, client walletStateClient, cfg QueueConfig) *RefreshQueue {
	return newRefreshQueue(store, client, cfg)
}

func (q *RefreshQueue) RefreshOne(ctx context.Context) (bool, error) {
	return q.refreshOne(ctx)
}

func (q *RefreshQueue) RefreshWallet(ctx context.Context, address string) (WalletState, error) {
	return q.refreshWallet(ctx, address)
}
