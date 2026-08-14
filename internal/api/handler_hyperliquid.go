package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jkeddari/hypermetrics/internal/corebus"
	"github.com/jkeddari/hypermetrics/internal/hypercore"
	apimodel "github.com/jkeddari/hypermetrics/internal/model/api"
)

const defaultPositionPageSize = 100
const defaultWalletPageSize = 100

type hyperliquidAPIHandler struct {
	store             *hypercore.Store
	refreshRequester  walletRefreshRequester
	whaleThresholdUSD float64
}

type walletRefreshRequester interface {
	RequestWalletRefresh(ctx context.Context, address string) error
}

func newHyperliquidAPIHandler(store *hypercore.Store, refreshRequester walletRefreshRequester, whaleThresholdUSD float64) *hyperliquidAPIHandler {
	if whaleThresholdUSD <= 0 {
		whaleThresholdUSD = 1_000_000
	}
	return &hyperliquidAPIHandler{
		store:             store,
		refreshRequester:  refreshRequester,
		whaleThresholdUSD: whaleThresholdUSD,
	}
}

func (h *hyperliquidAPIHandler) whaleAlert(w http.ResponseWriter, r *http.Request) {
	alerts, err := h.store.ListWhaleAlerts(200)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apimodel.ResponseEnvelope[any]{
			Code: "1006",
			Msg:  "failed to load whale alerts",
			Data: nil,
		})
		return
	}
	writeJSON(w, http.StatusOK, apimodel.ResponseEnvelope[[]apimodel.WhaleAlertItem]{
		Code: "0",
		Msg:  "success",
		Data: mapWhaleAlerts(alerts),
	})
}

func (h *hyperliquidAPIHandler) whalePosition(w http.ResponseWriter, r *http.Request) {
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

func (h *hyperliquidAPIHandler) position(w http.ResponseWriter, r *http.Request) {
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

func (h *hyperliquidAPIHandler) userPosition(w http.ResponseWriter, r *http.Request) {
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

	state, refreshErr, err := h.loadWalletState(r.Context(), user)
	if refreshErr != nil {
		writeRefreshError(w, refreshErr)
		return
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

func (h *hyperliquidAPIHandler) walletOverview(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusInternalServerError, apimodel.ResponseEnvelope[any]{
			Code: "1006",
			Msg:  "hypercore store unavailable",
			Data: nil,
		})
		return
	}

	query := r.URL.Query()
	user := hypercore.NormalizeAddress(query.Get("user_address"))
	if user == "" {
		user = hypercore.NormalizeAddress(query.Get("user"))
	}
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

	state, refreshErr, err := h.loadWalletState(r.Context(), user)
	if refreshErr != nil {
		writeRefreshError(w, refreshErr)
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apimodel.ResponseEnvelope[any]{
			Code: "1006",
			Msg:  "failed to load wallet overview",
			Data: nil,
		})
		return
	}

	writeJSON(w, http.StatusOK, apimodel.ResponseEnvelope[apimodel.WalletOverviewData]{
		Code: "0",
		Msg:  "success",
		Data: mapWalletOverview(state),
	})
}

func (h *hyperliquidAPIHandler) loadWalletState(ctx context.Context, user string) (hypercore.WalletState, error, error) {
	state, err := h.store.GetWalletState(user)
	if !errors.Is(err, hypercore.ErrWalletNotFound) && (err != nil || walletStateFresh(state, time.Now().UTC())) {
		return state, nil, err
	}

	refreshErr := h.refreshUserPosition(ctx, user)
	refreshedState, refreshedErr := h.store.GetWalletState(user)
	if refreshedErr == nil && walletStateFresh(refreshedState, time.Now().UTC()) {
		return refreshedState, nil, nil
	}
	if refreshErr != nil {
		return state, refreshErr, nil
	}
	return state, corebus.ErrRefreshFailed, nil
}

func walletStateFresh(state hypercore.WalletState, now time.Time) bool {
	refreshedAt := state.Account.RefreshedAt
	return !refreshedAt.IsZero() && now.Sub(refreshedAt) < hypercore.OnDemandFreshness
}

func (h *hyperliquidAPIHandler) refreshUserPosition(ctx context.Context, user string) error {
	if h.refreshRequester == nil {
		return corebus.ErrCoreUnavailable
	}
	return h.refreshRequester.RequestWalletRefresh(ctx, user)
}

func mapWalletOverview(state hypercore.WalletState) apimodel.WalletOverviewData {
	perpetualPositions := make([]apimodel.AssetPosition, 0, len(state.Positions))
	perpetualPnL := 0.0
	for _, position := range state.Positions {
		perpetualPnL += position.UnrealizedPnL
		perpetualPositions = append(perpetualPositions, apimodel.AssetPosition{
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

	spotBalances := make([]apimodel.SpotBalanceItem, 0, len(state.SpotBalances))
	spotValueUSD := 0.0
	spotPnL := 0.0
	for _, balance := range state.SpotBalances {
		spotValueUSD += balance.ValueUSD
		spotPnL += balance.UnrealizedPnL
		spotBalances = append(spotBalances, apimodel.SpotBalanceItem{
			Coin: balance.Coin, Token: balance.Token, Hold: balance.Hold, Total: balance.Total,
			EntryNtl: balance.EntryNtl, MarkPrice: balance.MarkPrice, ValueUSD: balance.ValueUSD,
			UnrealizedPnL: balance.UnrealizedPnL,
		})
	}

	perpetualOrders, spotOrders := mapOpenOrders(state.OpenOrders, state.SpotMarkets, state.SpotTokens)
	totalPnL := perpetualPnL + spotPnL
	return apimodel.WalletOverviewData{
		User:        state.Account.Address,
		RefreshedAt: state.Account.RefreshedAt.UnixMilli(),
		Totals: apimodel.WalletOverviewTotals{
			PerpetualAccountValue: state.Account.AccountValue,
			SpotValueUSD:          spotValueUSD,
			TotalValueUSD:         state.Account.AccountValue + spotValueUSD,
			UnrealizedPnL:         totalPnL,
		},
		Perpetuals: apimodel.PerpetualOverview{
			MarginSummary: apimodel.MarginSummary{
				AccountValue: state.Account.AccountValue,
				MarginUsed:   state.Account.MarginUsed,
				Withdrawable: state.Account.Withdrawable,
			},
			Positions: perpetualPositions, OpenOrders: perpetualOrders, UnrealizedPnL: perpetualPnL,
		},
		Spot: apimodel.SpotOverview{
			Balances: spotBalances, OpenOrders: spotOrders, ValueUSD: spotValueUSD, UnrealizedPnL: spotPnL,
		},
	}
}

func mapOpenOrders(orders []hypercore.WalletOpenOrder, markets []hypercore.SpotMarket, tokens []hypercore.SpotToken) ([]apimodel.OpenOrderItem, []apimodel.OpenOrderItem) {
	perpetuals := make([]apimodel.OpenOrderItem, 0)
	spot := make([]apimodel.OpenOrderItem, 0)
	for _, order := range orders {
		item := apimodel.OpenOrderItem{
			OID: order.OID, ClientOID: order.ClientOID, Coin: order.Coin, MarketType: order.MarketType,
			Side: order.Side, OrderType: order.OrderType, LimitPrice: order.LimitPrice, Size: order.Size,
			OriginalSize: order.OriginalSize, ReduceOnly: order.ReduceOnly, IsTrigger: order.IsTrigger,
			IsPositionTPSL: order.IsPositionTPSL, TriggerCondition: order.TriggerCondition,
			TriggerPrice: order.TriggerPrice, Timestamp: order.OrderTimestamp,
		}
		if order.MarketType == "spot" {
			item.Symbol = spotOrderSymbol(order.Coin, markets, tokens)
			spot = append(spot, item)
		} else {
			perpetuals = append(perpetuals, item)
		}
	}
	return perpetuals, spot
}

func spotOrderSymbol(coin string, markets []hypercore.SpotMarket, tokens []hypercore.SpotToken) string {
	if !strings.HasPrefix(coin, "@") {
		return coin
	}
	marketIndex, err := strconv.Atoi(strings.TrimPrefix(coin, "@"))
	if err != nil {
		return coin
	}
	tokenNames := make(map[int]string, len(tokens))
	for _, token := range tokens {
		tokenNames[token.Index] = token.Name
	}
	for _, market := range markets {
		if market.Index != marketIndex {
			continue
		}
		if market.Name != "" && !strings.HasPrefix(market.Name, "@") {
			return market.Name
		}
		base := tokenNames[market.BaseTokenIndex]
		quote := tokenNames[market.QuoteTokenIndex]
		if base != "" && quote != "" {
			return base + "/" + quote
		}
		if base != "" {
			return base
		}
		return coin
	}
	return coin
}

func writeRefreshError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	message := "upstream unavailable while refreshing wallet position"
	switch {
	case errors.Is(err, corebus.ErrCoreUnavailable):
		status = http.StatusServiceUnavailable
		message = "core service unavailable"
	case errors.Is(err, corebus.ErrCoreTimeout):
		status = http.StatusGatewayTimeout
		message = "core service timed out while refreshing wallet position"
	}
	writeJSON(w, status, apimodel.ResponseEnvelope[any]{Code: "1005", Msg: message, Data: nil})
}

func (h *hyperliquidAPIHandler) wallets(w http.ResponseWriter, r *http.Request) {
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

func (h *hyperliquidAPIHandler) walletPositionDistribution(w http.ResponseWriter, r *http.Request) {
	buckets, _, _, err := h.store.GetDistributionSnapshot(r.Context())
	if err != nil {
		writeDistributionSnapshotError(w, "wallet position", err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Code string                        `json:"code"`
		Data []apimodel.DistributionBucket `json:"data"`
	}{
		Code: "0",
		Data: mapPositionDistribution(buckets),
	})
}

func (h *hyperliquidAPIHandler) walletPnLDistribution(w http.ResponseWriter, r *http.Request) {
	_, buckets, _, err := h.store.GetDistributionSnapshot(r.Context())
	if err != nil {
		writeDistributionSnapshotError(w, "wallet pnl", err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Code string                        `json:"code"`
		Data []apimodel.DistributionBucket `json:"data"`
	}{
		Code: "0",
		Data: mapPositionDistribution(buckets),
	})
}

func writeDistributionSnapshotError(w http.ResponseWriter, name string, err error) {
	status := http.StatusInternalServerError
	message := "failed to load " + name + " distribution"
	if errors.Is(err, hypercore.ErrDistributionSnapshotNotFound) {
		status = http.StatusServiceUnavailable
		message = name + " distribution snapshot unavailable"
	}
	writeJSON(w, status, apimodel.ResponseEnvelope[any]{
		Code: "1006",
		Msg:  message,
		Data: nil,
	})
}

func (h *hyperliquidAPIHandler) longShortAccountRatioHistory(w http.ResponseWriter, r *http.Request) {
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

func mapWhaleAlerts(alerts []hypercore.WhaleAlert) []apimodel.WhaleAlertItem {
	items := make([]apimodel.WhaleAlertItem, 0, len(alerts))
	for _, alert := range alerts {
		items = append(items, apimodel.WhaleAlertItem{
			User:             alert.Address,
			Symbol:           alert.Symbol,
			PositionSize:     alert.PositionSize,
			EntryPrice:       alert.EntryPrice,
			LiqPrice:         alert.LiqPrice,
			PositionValueUSD: alert.PositionValueUSD,
			PositionAction:   alert.PositionAction,
			CreateTime:       alert.CreatedAt.UnixMilli(),
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

func mapPositionDistribution(buckets []hypercore.PositionDistributionBucket) []apimodel.DistributionBucket {
	out := make([]apimodel.DistributionBucket, 0, len(buckets))
	for _, bucket := range buckets {
		out = append(out, apimodel.DistributionBucket{
			GroupName:               bucket.GroupName,
			AllAddressCount:         bucket.AllAddressCount,
			PositionAddressCount:    bucket.PositionAddressCount,
			PositionAddressPercent:  bucket.PositionAddressPercent,
			BiasScore:               bucket.BiasScore,
			BiasRemark:              bucket.BiasRemark,
			MinimumAmount:           bucket.MinimumAmount,
			MaximumAmount:           bucket.MaximumAmount,
			LongPositionUSD:         bucket.LongPositionUSD,
			ShortPositionUSD:        bucket.ShortPositionUSD,
			LongPositionUSDPercent:  bucket.LongPositionUSDPercent,
			ShortPositionUSDPercent: bucket.ShortPositionUSDPercent,
			PositionUSD:             bucket.PositionUSD,
			ProfitAddressCount:      bucket.ProfitAddressCount,
			LossAddressCount:        bucket.LossAddressCount,
			ProfitAddressPercent:    bucket.ProfitAddressPercent,
			LossAddressPercent:      bucket.LossAddressPercent,
		})
	}
	return out
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
