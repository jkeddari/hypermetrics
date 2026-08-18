package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jkeddari/hypermetrics/internal/corebus"
	"github.com/jkeddari/hypermetrics/internal/db"
	"github.com/jkeddari/hypermetrics/internal/hypercore"
	apimodel "github.com/jkeddari/hypermetrics/internal/model/api"
)

func TestHyperliquidCurrentEndpoints(t *testing.T) {
	store := newTestHypercoreStore(t)
	handler := newHyperliquidAPIHandler(store, nil, 1_000_000)

	user := "0x0000000000000000000000000000000000000001"
	now := time.Now().UTC()

	state := hypercore.WalletState{
		Account: hypercore.WalletAccount{
			Address:      user,
			AccountValue: 500_000,
			MarginUsed:   100_000,
			Withdrawable: 400_000,
			RefreshedAt:  now,
		},
		Positions: []hypercore.WalletPosition{
			{
				Address:          user,
				Symbol:           "BTC",
				PositionSize:     20,
				EntryPrice:       50_000,
				MarkPrice:        60_000,
				LiqPrice:         45_000,
				Leverage:         5,
				MarginBalance:    100_000,
				PositionValueUSD: 1_200_000,
				UnrealizedPnL:    200_000,
				RefreshedAt:      now,
			},
		},
		SpotBalances: []hypercore.SpotBalance{
			{
				Address: user, Token: 0, Coin: "USDC", Total: 2500,
				MarkPrice: 1, ValueUSD: 2500, RefreshedAt: now,
			},
		},
		OpenOrders: []hypercore.WalletOpenOrder{
			{Address: user, OID: 1, Coin: "BTC", MarketType: "perpetual", Side: "B", OrderType: "Limit", Size: 1, OriginalSize: 1},
			{Address: user, OID: 2, Coin: "PURR/USDC", MarketType: "spot", Side: "A", OrderType: "Limit", Size: 5, OriginalSize: 5},
		},
	}

	wallet := walletFromTestState(state)
	if err := store.SaveWalletState(wallet, state); err != nil {
		t.Fatal(err)
	}
	if err := store.RefreshDistributionSnapshot(context.Background()); err != nil {
		t.Fatal(err)
	}

	t.Run("user position", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/hyperliquid/user-position?user_address="+user, nil)
		rec := httptest.NewRecorder()

		handler.userPosition(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var envelope apimodel.ResponseEnvelope[apimodel.UserPositionData]
		if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Code != "0" || envelope.Data.User != user || len(envelope.Data.AssetPosition) != 1 {
			t.Fatalf("unexpected envelope: %+v", envelope)
		}
	})

	t.Run("wallet overview", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/hyperliquid/wallet/overview?user_address="+user, nil)
		rec := httptest.NewRecorder()

		handler.walletOverview(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var envelope apimodel.ResponseEnvelope[apimodel.WalletOverviewData]
		if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Code != "0" || envelope.Data.Totals.TotalValueUSD != 502500 {
			t.Fatalf("unexpected overview totals: %+v", envelope.Data.Totals)
		}
		if len(envelope.Data.Spot.Balances) != 1 || len(envelope.Data.Spot.OpenOrders) != 1 || len(envelope.Data.Perpetuals.OpenOrders) != 1 {
			t.Fatalf("unexpected overview details: %+v", envelope.Data)
		}
	})

	t.Run("position by symbol", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/hyperliquid/position?symbol=BTC&current_page=1", nil)
		rec := httptest.NewRecorder()

		handler.position(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var envelope apimodel.ResponseEnvelope[[]apimodel.PositionItem]
		if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Code != "0" || len(envelope.Data) != 1 || envelope.Data[0].Symbol != "BTC" {
			t.Fatalf("unexpected envelope: %+v", envelope)
		}
	})

	t.Run("whale positions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/hyperliquid/whale-position", nil)
		rec := httptest.NewRecorder()

		handler.whalePosition(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var envelope apimodel.ResponseEnvelope[[]apimodel.WhalePositionItem]
		if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Code != "0" || len(envelope.Data) != 1 || envelope.Data[0].PositionValueUSD != 1_200_000 {
			t.Fatalf("unexpected envelope: %+v", envelope)
		}
	})

	t.Run("wallets", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/hyperliquid/wallets?current_page=1&page_size=10", nil)
		rec := httptest.NewRecorder()

		handler.wallets(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var envelope apimodel.ResponseEnvelope[[]apimodel.WalletItem]
		if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Code != "0" || len(envelope.Data) != 1 || envelope.Data[0].Address != user {
			t.Fatalf("unexpected envelope: %+v", envelope)
		}
	})

	t.Run("wallet position distribution", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/hyperliquid/wallet/position-distribution", nil)
		rec := httptest.NewRecorder()

		handler.walletPositionDistribution(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var response struct {
			Code string                        `json:"code"`
			Data []apimodel.DistributionBucket `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Code != "0" || len(response.Data) != 8 {
			t.Fatalf("unexpected response: %+v", response)
		}
		smallWhale := response.Data[4]
		if smallWhale.GroupName != "small_whale" || smallWhale.AllAddressCount != 1 || smallWhale.LongPositionUSD != 1_200_000 {
			t.Fatalf("unexpected small whale bucket: %+v", smallWhale)
		}

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		if len(fields) != 2 || fields["code"] == nil || fields["data"] == nil {
			t.Fatalf("expected exact CoinGlass top-level fields, got %v", fields)
		}
	})

	t.Run("wallet pnl distribution", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/hyperliquid/wallet/pnl-distribution", nil)
		rec := httptest.NewRecorder()

		handler.walletPnLDistribution(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var response struct {
			Code string                        `json:"code"`
			Data []apimodel.DistributionBucket `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Code != "0" || len(response.Data) != 8 || response.Data[0].GroupName != "money_printer" {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("long short account ratio history", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/hyperliquid/global-long-short-account-ratio/history?symbol=BTC&interval=1h", nil)
		rec := httptest.NewRecorder()

		handler.longShortAccountRatioHistory(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var response apimodel.ResponseEnvelope[[]apimodel.LongShortAccountRatioPoint]
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Code != "0" || len(response.Data) != 1 {
			t.Fatalf("unexpected response: %+v", response)
		}
		point := response.Data[0]
		if point.Symbol != "BTC" || point.LongAccount != 1 || point.ShortAccount != 0 || point.LongShortRatio != 0 {
			t.Fatalf("unexpected ratio point: %+v", point)
		}
	})

	t.Run("whale alerts", func(t *testing.T) {
		alertAt := now.Add(time.Minute)
		storedWallet, err := store.GetWallet(user)
		if err != nil {
			t.Fatal(err)
		}
		closedState := hypercore.WalletState{Account: hypercore.WalletAccount{
			Address: user, AccountValue: 500_000, RefreshedAt: alertAt, RawReceivedAt: alertAt,
		}}
		storedWallet.AccountValue = closedState.Account.AccountValue
		storedWallet.LastRefreshedAt = alertAt
		storedWallet.LastSuccessfulRefresh = alertAt
		storedWallet.RefreshAttempts++
		storedWallet.HasOpenPosition = false
		storedWallet.MaxPositionValueUSD = 0
		storedWallet.TotalPositionValueUSD = 0
		storedWallet.NextRefreshAt = alertAt
		if err := store.SaveWalletState(storedWallet, closedState); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/hyperliquid/whale-alert", nil)
		rec := httptest.NewRecorder()
		handler.whaleAlert(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var response apimodel.ResponseEnvelope[[]apimodel.WhaleAlertItem]
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Code != "0" || len(response.Data) != 1 || response.Data[0].PositionAction != hypercore.WhalePositionClosed {
			t.Fatalf("unexpected response: %+v", response)
		}
		if response.Data[0].CreateTime != alertAt.UnixMilli() || response.Data[0].PositionValueUSD != 1_200_000 {
			t.Fatalf("unexpected CoinGlass alert fields: %+v", response.Data[0])
		}
	})
}

func TestLongShortAccountRatioHistoryValidatesQuery(t *testing.T) {
	handler := &hyperliquidAPIHandler{}
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "missing symbol", path: "?interval=1h"},
		{name: "missing interval", path: "?symbol=BTC"},
		{name: "unsupported interval", path: "?symbol=BTC&interval=15m"},
		{name: "limit too high", path: "?symbol=BTC&interval=1h&limit=1001"},
		{name: "invalid start time", path: "?symbol=BTC&interval=1h&start_time=now"},
		{name: "reversed time range", path: "?symbol=BTC&interval=1h&start_time=2000&end_time=1000"},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/hyperliquid/global-long-short-account-ratio/history"+test.path, nil)
			rec := httptest.NewRecorder()

			handler.longShortAccountRatioHistory(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestUserPositionRefreshesAndStoresMissingWallet(t *testing.T) {
	store := newTestHypercoreStore(t)

	user := "0x0000000000000000000000000000000000000002"
	state := hypercore.WalletState{
		Account: hypercore.WalletAccount{
			Address:      user,
			AccountValue: 250_000,
			MarginUsed:   50_000,
			Withdrawable: 200_000,
		},
		Positions: []hypercore.WalletPosition{
			{
				Address:          user,
				Symbol:           "ETH",
				PositionSize:     -100,
				EntryPrice:       3_000,
				MarkPrice:        3_100,
				PositionValueUSD: 310_000,
				UnrealizedPnL:    -10_000,
			},
		},
	}

	handler := newHyperliquidAPIHandler(store, fakeWalletRefreshRequester{
		store: store,
		state: state,
	}, 1_000_000)

	req := httptest.NewRequest(http.MethodGet, "/api/hyperliquid/user-position?user_address="+user, nil)
	rec := httptest.NewRecorder()

	handler.userPosition(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	stored, err := store.GetWalletState(user)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Account.Address != user || len(stored.Positions) != 1 || stored.Positions[0].Symbol != "ETH" {
		t.Fatalf("unexpected stored state: %+v", stored)
	}
}

func TestWalletStateFreshForFiveMinutes(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	state := hypercore.WalletState{
		Account: hypercore.WalletAccount{RefreshedAt: now.Add(-4*time.Minute - 59*time.Second)},
	}
	if !walletStateFresh(state, now) {
		t.Fatal("expected state younger than five minutes to be fresh")
	}
	state.Account.RefreshedAt = now.Add(-5 * time.Minute)
	if walletStateFresh(state, now) {
		t.Fatal("expected state five minutes old to be stale")
	}
}

func TestSpotOrderSymbolResolvesAtCoin(t *testing.T) {
	_, spotOrders := mapOpenOrders(
		[]hypercore.WalletOpenOrder{{Coin: "@107", MarketType: "spot"}},
		[]hypercore.SpotMarket{{Index: 107, Name: "@107", BaseTokenIndex: 1, QuoteTokenIndex: 0}},
		[]hypercore.SpotToken{{Index: 0, Name: "USDC"}, {Index: 1, Name: "APE"}},
	)
	if len(spotOrders) != 1 || spotOrders[0].Symbol != "APE/USDC" {
		t.Fatalf("unexpected spot order symbol: %+v", spotOrders)
	}
}

func TestWriteRefreshErrorUsesServiceStatuses(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{corebus.ErrCoreUnavailable, http.StatusServiceUnavailable},
		{corebus.ErrCoreTimeout, http.StatusGatewayTimeout},
		{corebus.ErrRefreshFailed, http.StatusBadGateway},
	}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		writeRefreshError(recorder, test.err)
		if recorder.Code != test.want {
			t.Fatalf("expected %d, got %d", test.want, recorder.Code)
		}
	}
}

func TestWriteDistributionSnapshotErrorUsesServiceStatuses(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{hypercore.ErrDistributionSnapshotNotFound, http.StatusServiceUnavailable},
		{errors.New("database unavailable"), http.StatusInternalServerError},
	}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		writeDistributionSnapshotError(recorder, "wallet position", test.err)
		if recorder.Code != test.want {
			t.Fatalf("expected %d, got %d", test.want, recorder.Code)
		}
	}
}

func newTestHypercoreStore(t *testing.T) *hypercore.Store {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	base, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("api_test_%d_%d", os.Getpid(), apiTestSchemaSequence.Add(1))
	if _, err := base.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()

	database, err := sql.Open("pgx", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RunMigrations(database); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.Close()
		if _, err := base.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
		base.Close()
	})
	return hypercore.OpenStore(database, hypercore.PriorityConfig{WhaleThresholdUSD: 1_000_000})
}

var apiTestSchemaSequence atomic.Uint64

type fakeWalletRefreshRequester struct {
	store *hypercore.Store
	state hypercore.WalletState
	err   error
}

func (f fakeWalletRefreshRequester) RequestWalletRefresh(context.Context, string) error {
	if f.err != nil {
		return f.err
	}

	wallet := walletFromTestState(f.state)
	if err := f.store.SaveWalletState(wallet, f.state); err != nil {
		return err
	}
	return nil
}

func walletFromTestState(state hypercore.WalletState) hypercore.Wallet {
	firstSeen := state.Account.RefreshedAt
	if firstSeen.IsZero() {
		firstSeen = time.Now().UTC()
	}
	wallet := hypercore.Wallet{
		Address:               state.Account.Address,
		FirstSeenAt:           firstSeen,
		LastRefreshedAt:       firstSeen,
		LastSuccessfulRefresh: firstSeen,
		RefreshAttempts:       1,
		AccountValue:          state.Account.AccountValue,
		NextRefreshAt:         firstSeen,
	}
	for _, position := range state.Positions {
		value := math.Abs(position.PositionValueUSD)
		wallet.TotalPositionValueUSD += value
		if value > wallet.MaxPositionValueUSD {
			wallet.MaxPositionValueUSD = value
		}
	}
	wallet.HasOpenPosition = len(state.Positions) > 0
	wallet.KnownWhale = wallet.MaxPositionValueUSD >= 1_000_000
	return wallet
}
