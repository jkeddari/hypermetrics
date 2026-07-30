package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jkeddari/hypermetrics/internal/hypercore"
	apimodel "github.com/jkeddari/hypermetrics/internal/model/api"
)

const defaultPositionPageSize = 100
const defaultWalletPageSize = 100

type HyperliquidAPIHandler struct {
	store             *hypercore.Store
	refresher         WalletRefresher
	whaleThresholdUSD float64
}

type WalletRefresher interface {
	RefreshWallet(ctx context.Context, address string) (hypercore.WalletState, error)
}

func NewHyperliquidAPIHandler(store *hypercore.Store, refresher WalletRefresher, whaleThresholdUSD float64) *HyperliquidAPIHandler {
	if whaleThresholdUSD <= 0 {
		whaleThresholdUSD = 1_000_000
	}
	return &HyperliquidAPIHandler{
		store:             store,
		refresher:         refresher,
		whaleThresholdUSD: whaleThresholdUSD,
	}
}

func (h *HyperliquidAPIHandler) WhaleAlert(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, apimodel.ResponseEnvelope[[]apimodel.WhaleAlertItem]{
		Code: "1006",
		Msg:  "hyperliquid whale alert not implemented",
		Data: nil,
	})
}

func (h *HyperliquidAPIHandler) WhalePosition(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusInternalServerError, apimodel.ResponseEnvelope[any]{
			Code: "1006",
			Msg:  "hypercore store unavailable",
			Data: nil,
		})
		return
	}

	positions, err := h.store.ListWhalePositions(h.whaleThresholdUSD)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apimodel.ResponseEnvelope[any]{
			Code: "1006",
			Msg:  "failed to load whale positions",
			Data: nil,
		})
		return
	}

	writeJSON(w, http.StatusOK, apimodel.ResponseEnvelope[[]apimodel.WhalePositionItem]{
		Code: "0",
		Msg:  "success",
		Data: mapWhalePositions(positions),
	})
}

func (h *HyperliquidAPIHandler) Position(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusInternalServerError, apimodel.ResponseEnvelope[any]{
			Code: "1006",
			Msg:  "hypercore store unavailable",
			Data: nil,
		})
		return
	}

	query := r.URL.Query()
	symbol := strings.ToUpper(strings.TrimSpace(query.Get("symbol")))
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, apimodel.ResponseEnvelope[any]{
			Code: "1001",
			Msg:  "missing required parameter: symbol",
			Data: nil,
		})
		return
	}

	currentPage := 1
	if rawPage := query.Get("current_page"); rawPage != "" {
		parsed, err := strconv.Atoi(rawPage)
		if err != nil || parsed <= 0 {
			writeJSON(w, http.StatusBadRequest, apimodel.ResponseEnvelope[any]{
				Code: "1002",
				Msg:  "invalid parameter: current_page",
				Data: nil,
			})
			return
		}
		currentPage = parsed
	}

	positions, err := h.store.ListPositionsBySymbol(symbol, currentPage, defaultPositionPageSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apimodel.ResponseEnvelope[any]{
			Code: "1006",
			Msg:  "failed to load positions",
			Data: nil,
		})
		return
	}

	writeJSON(w, http.StatusOK, apimodel.ResponseEnvelope[[]apimodel.PositionItem]{
		Code: "0",
		Msg:  "success",
		Data: mapPositions(positions),
	})
}

func (h *HyperliquidAPIHandler) UserPosition(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusInternalServerError, apimodel.ResponseEnvelope[any]{
			Code: "1006",
			Msg:  "hypercore store unavailable",
			Data: nil,
		})
		return
	}

	query := r.URL.Query()
	user := strings.TrimSpace(query.Get("user_address"))
	if user == "" {
		user = strings.TrimSpace(query.Get("user"))
	}
	user = hypercore.NormalizeAddress(user)
	if user == "" {
		writeJSON(w, http.StatusBadRequest, apimodel.ResponseEnvelope[any]{
			Code: "1001",
			Msg:  "missing required parameter: user_address",
			Data: nil,
		})
		return
	}
	if !hypercore.IsAddress(user) {
		writeJSON(w, http.StatusBadRequest, apimodel.ResponseEnvelope[any]{
			Code: "1002",
			Msg:  "invalid parameter: user_address",
			Data: nil,
		})
		return
	}

	state, err := h.store.GetWalletState(user)
	if errors.Is(err, hypercore.ErrWalletNotFound) || err == nil && !walletStateFresh(state, time.Now().UTC()) {
		state, err = h.refreshUserPosition(r.Context(), user)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, apimodel.ResponseEnvelope[any]{
				Code: "1005",
				Msg:  "upstream unavailable while refreshing wallet position",
				Data: nil,
			})
			return
		}
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apimodel.ResponseEnvelope[any]{
			Code: "1006",
			Msg:  "failed to load user position",
			Data: nil,
		})
		return
	}

	writeJSON(w, http.StatusOK, apimodel.ResponseEnvelope[apimodel.UserPositionData]{
		Code: "0",
		Msg:  "success",
		Data: mapUserPosition(state),
	})
}

func walletStateFresh(state hypercore.WalletState, now time.Time) bool {
	refreshedAt := state.Account.RefreshedAt
	return !refreshedAt.IsZero() && now.Sub(refreshedAt) < hypercore.OnDemandFreshness
}

func (h *HyperliquidAPIHandler) refreshUserPosition(ctx context.Context, user string) (hypercore.WalletState, error) {
	if h.refresher == nil {
		return hypercore.WalletState{}, hypercore.ErrWalletNotFound
	}
	return h.refresher.RefreshWallet(ctx, user)
}

func (h *HyperliquidAPIHandler) Wallets(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusInternalServerError, apimodel.ResponseEnvelope[any]{
			Code: "1006",
			Msg:  "hypercore store unavailable",
			Data: nil,
		})
		return
	}

	currentPage, ok := parsePositiveIntQuery(w, r, "current_page", 1)
	if !ok {
		return
	}
	pageSize, ok := parsePositiveIntQuery(w, r, "page_size", defaultWalletPageSize)
	if !ok {
		return
	}

	wallets, err := h.store.ListWalletsPage(currentPage, pageSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apimodel.ResponseEnvelope[any]{
			Code: "1006",
			Msg:  "failed to load wallets",
			Data: nil,
		})
		return
	}

	writeJSON(w, http.StatusOK, apimodel.ResponseEnvelope[[]apimodel.WalletItem]{
		Code: "0",
		Msg:  "success",
		Data: mapWallets(wallets),
	})
}

func (h *HyperliquidAPIHandler) WalletPositionDistribution(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, apimodel.ResponseEnvelope[[]apimodel.DistributionBucket]{
		Code: "1006",
		Msg:  "hyperliquid wallet position distribution not implemented",
		Data: nil,
	})
}

func (h *HyperliquidAPIHandler) WalletPnLDistribution(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, apimodel.ResponseEnvelope[[]apimodel.DistributionBucket]{
		Code: "1006",
		Msg:  "hyperliquid wallet pnl distribution not implemented",
		Data: nil,
	})
}

func (h *HyperliquidAPIHandler) LongShortAccountRatioHistory(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if query.Get("symbol") == "" {
		writeJSON(w, http.StatusBadRequest, apimodel.ResponseEnvelope[any]{
			Code: "1001",
			Msg:  "missing required parameter: symbol",
			Data: nil,
		})
		return
	}
	if query.Get("interval") == "" {
		writeJSON(w, http.StatusBadRequest, apimodel.ResponseEnvelope[any]{
			Code: "1001",
			Msg:  "missing required parameter: interval",
			Data: nil,
		})
		return
	}
	if limit := query.Get("limit"); limit != "" {
		if _, err := strconv.Atoi(limit); err != nil {
			writeJSON(w, http.StatusBadRequest, apimodel.ResponseEnvelope[any]{
				Code: "1002",
				Msg:  "invalid parameter: limit",
				Data: nil,
			})
			return
		}
	}

	writeJSON(w, http.StatusNotImplemented, apimodel.ResponseEnvelope[[]apimodel.LongShortAccountRatioPoint]{
		Code: "1006",
		Msg:  "hyperliquid long short account ratio history not implemented",
		Data: nil,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func parsePositiveIntQuery(w http.ResponseWriter, r *http.Request, key string, defaultValue int) (int, bool) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultValue, true
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		writeJSON(w, http.StatusBadRequest, apimodel.ResponseEnvelope[any]{
			Code: "1002",
			Msg:  "invalid parameter: " + key,
			Data: nil,
		})
		return 0, false
	}
	return parsed, true
}

func mapPositions(positions []hypercore.WalletPosition) []apimodel.PositionItem {
	items := make([]apimodel.PositionItem, 0, len(positions))
	for _, position := range positions {
		items = append(items, apimodel.PositionItem{
			User:             position.Address,
			Symbol:           position.Symbol,
			PositionSize:     position.PositionSize,
			EntryPrice:       position.EntryPrice,
			MarkPrice:        position.MarkPrice,
			LiqPrice:         position.LiqPrice,
			Leverage:         position.Leverage,
			MarginBalance:    position.MarginBalance,
			PositionValueUSD: position.PositionValueUSD,
			UnrealizedPnL:    position.UnrealizedPnL,
		})
	}
	return items
}

func mapWhalePositions(positions []hypercore.WalletPosition) []apimodel.WhalePositionItem {
	items := make([]apimodel.WhalePositionItem, 0, len(positions))
	for _, position := range positions {
		items = append(items, apimodel.WhalePositionItem{
			User:             position.Address,
			Symbol:           position.Symbol,
			PositionSize:     position.PositionSize,
			EntryPrice:       position.EntryPrice,
			MarkPrice:        position.MarkPrice,
			LiqPrice:         position.LiqPrice,
			Leverage:         position.Leverage,
			MarginBalance:    position.MarginBalance,
			PositionValueUSD: position.PositionValueUSD,
			UnrealizedPnL:    position.UnrealizedPnL,
		})
	}
	return items
}

func mapUserPosition(state hypercore.WalletState) apimodel.UserPositionData {
	positions := make([]apimodel.AssetPosition, 0, len(state.Positions))
	for _, position := range state.Positions {
		positions = append(positions, apimodel.AssetPosition{
			Symbol:           position.Symbol,
			PositionSize:     position.PositionSize,
			EntryPrice:       position.EntryPrice,
			MarkPrice:        position.MarkPrice,
			LiqPrice:         position.LiqPrice,
			Leverage:         position.Leverage,
			PositionValueUSD: position.PositionValueUSD,
			UnrealizedPnL:    position.UnrealizedPnL,
		})
	}

	return apimodel.UserPositionData{
		User: state.Account.Address,
		MarginSummary: apimodel.MarginSummary{
			AccountValue: state.Account.AccountValue,
			MarginUsed:   state.Account.MarginUsed,
			Withdrawable: state.Account.Withdrawable,
		},
		AssetPosition: positions,
	}
}

func mapWallets(wallets []hypercore.Wallet) []apimodel.WalletItem {
	items := make([]apimodel.WalletItem, 0, len(wallets))
	for _, wallet := range wallets {
		items = append(items, apimodel.WalletItem{
			Address:               wallet.Address,
			Sources:               wallet.Sources,
			Tier:                  wallet.Tier,
			KnownWhale:            wallet.KnownWhale,
			HasOpenPosition:       wallet.HasOpenPosition,
			AccountValue:          wallet.AccountValue,
			TotalPositionValueUSD: wallet.TotalPositionValueUSD,
			MaxPositionValueUSD:   wallet.MaxPositionValueUSD,
			LastSeenLeaderboardAt: millis(wallet.LastSeenLeaderboardAt),
			LastRefreshedAt:       millis(wallet.LastRefreshedAt),
			NextRefreshAt:         millis(wallet.NextRefreshAt),
			PriorityScore:         wallet.PriorityScore,
		})
	}
	return items
}

func millis(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}
