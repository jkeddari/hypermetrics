package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jkeddari/hypermetrics/internal/db"
	"github.com/jkeddari/hypermetrics/internal/hypercore"
	apimodel "github.com/jkeddari/hypermetrics/internal/model/api"
)

func TestHyperliquidCurrentEndpoints(t *testing.T) {
	store := newTestHypercoreStore(t)
	handler := NewHyperliquidAPIHandler(store, nil, 1_000_000)

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
	}

	wallet := hypercore.ApplyRefreshSuccess(hypercore.Wallet{Address: user}, state, hypercore.PriorityConfig{
		WhaleThresholdUSD: 1_000_000,
		Now: func() time.Time {
			return now
		},
	})
	if err := store.SaveWalletState(wallet, state); err != nil {
		t.Fatal(err)
	}

	t.Run("user position", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/hyperliquid/user-position?user_address="+user, nil)
		rec := httptest.NewRecorder()

		handler.UserPosition(rec, req)

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

	t.Run("position by symbol", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/hyperliquid/position?symbol=BTC&current_page=1", nil)
		rec := httptest.NewRecorder()

		handler.Position(rec, req)

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

		handler.WhalePosition(rec, req)

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

		handler.Wallets(rec, req)

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

	handler := NewHyperliquidAPIHandler(store, fakeWalletRefresher{
		store: store,
		state: state,
	}, 1_000_000)

	req := httptest.NewRequest(http.MethodGet, "/api/hyperliquid/user-position?user_address="+user, nil)
	rec := httptest.NewRecorder()

	handler.UserPosition(rec, req)

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

type fakeWalletRefresher struct {
	store *hypercore.Store
	state hypercore.WalletState
	err   error
}

func (f fakeWalletRefresher) RefreshWallet(context.Context, string) (hypercore.WalletState, error) {
	if f.err != nil {
		return hypercore.WalletState{}, f.err
	}

	wallet := hypercore.ApplyRefreshSuccess(hypercore.Wallet{Address: f.state.Account.Address}, f.state, hypercore.PriorityConfig{
		WhaleThresholdUSD: 1_000_000,
	})
	if err := f.store.SaveWalletState(wallet, f.state); err != nil {
		return hypercore.WalletState{}, err
	}
	return f.state, nil
}
