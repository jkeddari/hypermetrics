package hypercore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

const defaultInfoURL = "https://api.hyperliquid.xyz/info"
const spotMetadataCacheTTL = time.Minute

type walletStateClient interface {
	getClearinghouseState(ctx context.Context, address string) (WalletState, error)
}

type hyperliquidClient struct {
	infoURL        string
	httpClient     *http.Client
	spotMetadataMu sync.Mutex
	spotTokens     []SpotToken
	spotMarkets    []SpotMarket
	spotMetadataAt time.Time
}

func newHyperliquidClient(infoURL string, httpClient *http.Client) *hyperliquidClient {
	if infoURL == "" {
		infoURL = defaultInfoURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &hyperliquidClient{
		infoURL:    infoURL,
		httpClient: httpClient,
	}
}

func (c *hyperliquidClient) getClearinghouseState(ctx context.Context, address string) (WalletState, error) {
	address = NormalizeAddress(address)
	if !IsAddress(address) {
		return WalletState{}, fmt.Errorf("invalid wallet address: %q", address)
	}

	spotTokens, spotMarkets, err := c.getSpotMetadata(ctx)
	if err != nil {
		return WalletState{}, err
	}

	var perps clearinghouseStateResponse
	var spot spotClearinghouseStateResponse
	var orders []frontendOpenOrderResponse
	errs := make(chan error, 3)

	go func() {
		errs <- c.info(ctx, map[string]string{
			"type": "clearinghouseState",
			"user": address,
		}, &perps)
	}()
	go func() {
		errs <- c.info(ctx, map[string]string{
			"type": "spotClearinghouseState",
			"user": address,
		}, &spot)
	}()
	go func() {
		errs <- c.info(ctx, map[string]string{
			"type": "frontendOpenOrders",
			"user": address,
		}, &orders)
	}()
	for range 3 {
		if err := <-errs; err != nil {
			return WalletState{}, err
		}
	}

	receivedAt := time.Now().UTC()
	state := perps.toWalletState(address, receivedAt)
	state.SpotTokens = spotTokens
	state.SpotMarkets = spotMarkets
	state.SpotBalances = mapSpotBalances(address, spot.Balances, spotMarkets, receivedAt)
	state.OpenOrders = mapOpenOrders(address, orders, spotMarkets, receivedAt)
	return state, nil
}

func (c *hyperliquidClient) info(ctx context.Context, payload any, result any) error {
	requestBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.infoURL, bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("hyperliquid info status %d: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, result); err != nil {
		return err
	}
	return nil
}

type spotMetaAndAssetCtxsResponse struct {
	Tokens   []spotTokenResponse  `json:"tokens"`
	Universe []spotMarketResponse `json:"universe"`
}

type spotTokenResponse struct {
	Name        string                   `json:"name"`
	SzDecimals  int                      `json:"szDecimals"`
	WeiDecimals int                      `json:"weiDecimals"`
	Index       int                      `json:"index"`
	TokenID     string                   `json:"tokenId"`
	IsCanonical bool                     `json:"isCanonical"`
	EVMContract *spotEVMContractResponse `json:"evmContract"`
	FullName    string                   `json:"fullName"`
}

type spotEVMContractResponse struct {
	Address string `json:"address"`
}

type spotMarketResponse struct {
	Name        string `json:"name"`
	Tokens      []int  `json:"tokens"`
	Index       int    `json:"index"`
	IsCanonical bool   `json:"isCanonical"`
}

type spotAssetContextResponse struct {
	DayNtlVlm json.RawMessage `json:"dayNtlVlm"`
	MarkPx    json.RawMessage `json:"markPx"`
	MidPx     json.RawMessage `json:"midPx"`
	PrevDayPx json.RawMessage `json:"prevDayPx"`
}

type spotClearinghouseStateResponse struct {
	Balances []spotBalanceResponse `json:"balances"`
}

type spotBalanceResponse struct {
	Coin     string `json:"coin"`
	Token    int    `json:"token"`
	Hold     string `json:"hold"`
	Total    string `json:"total"`
	EntryNtl string `json:"entryNtl"`
}

type frontendOpenOrderResponse struct {
	Coin             string `json:"coin"`
	IsPositionTPSL   bool   `json:"isPositionTpsl"`
	IsTrigger        bool   `json:"isTrigger"`
	LimitPx          string `json:"limitPx"`
	OID              int64  `json:"oid"`
	ClientOID        string `json:"cloid"`
	OrderType        string `json:"orderType"`
	OriginalSize     string `json:"origSz"`
	ReduceOnly       bool   `json:"reduceOnly"`
	Side             string `json:"side"`
	Size             string `json:"sz"`
	Timestamp        int64  `json:"timestamp"`
	TriggerCondition string `json:"triggerCondition"`
	TriggerPx        string `json:"triggerPx"`
}

func (c *hyperliquidClient) getSpotMetadata(ctx context.Context) ([]SpotToken, []SpotMarket, error) {
	c.spotMetadataMu.Lock()
	if !c.spotMetadataAt.IsZero() && time.Since(c.spotMetadataAt) < spotMetadataCacheTTL {
		tokens := append([]SpotToken(nil), c.spotTokens...)
		markets := append([]SpotMarket(nil), c.spotMarkets...)
		c.spotMetadataMu.Unlock()
		return tokens, markets, nil
	}
	c.spotMetadataMu.Unlock()

	var raw []json.RawMessage
	if err := c.info(ctx, map[string]string{"type": "spotMetaAndAssetCtxs"}, &raw); err != nil {
		return nil, nil, err
	}
	if len(raw) != 2 {
		return nil, nil, fmt.Errorf("invalid spot metadata response: expected 2 items, got %d", len(raw))
	}

	var metadata spotMetaAndAssetCtxsResponse
	if err := json.Unmarshal(raw[0], &metadata); err != nil {
		return nil, nil, err
	}
	var contexts []spotAssetContextResponse
	if err := json.Unmarshal(raw[1], &contexts); err != nil {
		return nil, nil, err
	}

	refreshedAt := time.Now().UTC()
	tokens := make([]SpotToken, 0, len(metadata.Tokens))
	for _, token := range metadata.Tokens {
		evmContract := ""
		if token.EVMContract != nil {
			evmContract = token.EVMContract.Address
		}
		tokens = append(tokens, SpotToken{
			Index: token.Index, Name: token.Name, SzDecimals: token.SzDecimals,
			WeiDecimals: token.WeiDecimals, TokenID: token.TokenID,
			IsCanonical: token.IsCanonical, EVMContract: evmContract,
			FullName: token.FullName, UpdatedAt: refreshedAt,
		})
	}

	markets := make([]SpotMarket, 0, len(metadata.Universe))
	for index, market := range metadata.Universe {
		var baseToken, quoteToken int
		if len(market.Tokens) > 0 {
			baseToken = market.Tokens[0]
		}
		if len(market.Tokens) > 1 {
			quoteToken = market.Tokens[1]
		}
		spotMarket := SpotMarket{
			Index: market.Index, Name: market.Name, BaseTokenIndex: baseToken,
			QuoteTokenIndex: quoteToken, IsCanonical: market.IsCanonical,
			UpdatedAt: refreshedAt,
		}
		if index < len(contexts) {
			spotMarket.DayNotionalVolume = parseJSONFloat(contexts[index].DayNtlVlm)
			spotMarket.MarkPrice = parseJSONFloat(contexts[index].MarkPx)
			spotMarket.MidPrice = parseJSONFloat(contexts[index].MidPx)
			spotMarket.PreviousDayPrice = parseJSONFloat(contexts[index].PrevDayPx)
		}
		markets = append(markets, spotMarket)
	}

	c.spotMetadataMu.Lock()
	c.spotTokens = tokens
	c.spotMarkets = markets
	c.spotMetadataAt = refreshedAt
	c.spotMetadataMu.Unlock()
	return append([]SpotToken(nil), tokens...), append([]SpotMarket(nil), markets...), nil
}

func mapSpotBalances(address string, balances []spotBalanceResponse, markets []SpotMarket, refreshedAt time.Time) []SpotBalance {
	prices := make(map[int]float64)
	for _, market := range markets {
		if market.QuoteTokenIndex != 0 || market.MarkPrice <= 0 {
			continue
		}
		if _, exists := prices[market.BaseTokenIndex]; !exists {
			prices[market.BaseTokenIndex] = market.MarkPrice
		}
	}
	prices[0] = 1

	result := make([]SpotBalance, 0, len(balances))
	for _, balance := range balances {
		total := parseFloat(balance.Total)
		entryNtl := parseFloat(balance.EntryNtl)
		markPrice := prices[balance.Token]
		valueUSD := total * markPrice
		unrealizedPnL := 0.0
		if entryNtl != 0 && markPrice > 0 {
			unrealizedPnL = valueUSD - entryNtl
		}
		result = append(result, SpotBalance{
			Address: address, Token: balance.Token, Coin: balance.Coin,
			Hold: parseFloat(balance.Hold), Total: total, EntryNtl: entryNtl,
			MarkPrice: markPrice, ValueUSD: valueUSD,
			UnrealizedPnL: unrealizedPnL, RefreshedAt: refreshedAt,
		})
	}
	return result
}

func mapOpenOrders(address string, orders []frontendOpenOrderResponse, markets []SpotMarket, refreshedAt time.Time) []WalletOpenOrder {
	spotNames := make(map[string]struct{}, len(markets))
	for _, market := range markets {
		spotNames[market.Name] = struct{}{}
	}

	result := make([]WalletOpenOrder, 0, len(orders))
	for _, order := range orders {
		marketType := "perpetual"
		if _, ok := spotNames[order.Coin]; ok || strings.HasPrefix(order.Coin, "@") || strings.Contains(order.Coin, "/") {
			marketType = "spot"
		}
		result = append(result, WalletOpenOrder{
			Address: address, OID: order.OID, ClientOID: order.ClientOID,
			Coin: order.Coin, MarketType: marketType, Side: order.Side,
			OrderType: order.OrderType, LimitPrice: parseFloat(order.LimitPx),
			Size: parseFloat(order.Size), OriginalSize: parseFloat(order.OriginalSize),
			ReduceOnly: order.ReduceOnly, IsTrigger: order.IsTrigger,
			IsPositionTPSL: order.IsPositionTPSL, TriggerCondition: order.TriggerCondition,
			TriggerPrice: parseFloat(order.TriggerPx), OrderTimestamp: order.Timestamp,
			RefreshedAt: refreshedAt,
		})
	}
	return result
}

func parseJSONFloat(raw json.RawMessage) float64 {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var stringValue string
	if err := json.Unmarshal(raw, &stringValue); err == nil {
		return parseFloat(stringValue)
	}
	var numberValue float64
	if err := json.Unmarshal(raw, &numberValue); err == nil {
		return numberValue
	}
	return 0
}

type clearinghouseStateResponse struct {
	MarginSummary struct {
		AccountValue    string `json:"accountValue"`
		TotalNtlPos     string `json:"totalNtlPos"`
		TotalRawUsd     string `json:"totalRawUsd"`
		TotalMarginUsed string `json:"totalMarginUsed"`
	} `json:"marginSummary"`
	CrossMarginSummary struct {
		AccountValue    string `json:"accountValue"`
		TotalNtlPos     string `json:"totalNtlPos"`
		TotalRawUsd     string `json:"totalRawUsd"`
		TotalMarginUsed string `json:"totalMarginUsed"`
	} `json:"crossMarginSummary"`
	Withdrawable   string                       `json:"withdrawable"`
	AssetPositions []clearinghouseAssetPosition `json:"assetPositions"`
	Time           int64                        `json:"time"`
}

type clearinghouseAssetPosition struct {
	Type     string `json:"type"`
	Position struct {
		Coin          string `json:"coin"`
		Szi           string `json:"szi"`
		EntryPx       string `json:"entryPx"`
		PositionValue string `json:"positionValue"`
		UnrealizedPnL string `json:"unrealizedPnl"`
		LiquidationPx string `json:"liquidationPx"`
		MarginUsed    string `json:"marginUsed"`
		Leverage      struct {
			Type  string  `json:"type"`
			Value float64 `json:"value"`
		} `json:"leverage"`
	} `json:"position"`
}

func (r clearinghouseStateResponse) toWalletState(address string, receivedAt time.Time) WalletState {
	accountValue := parseFloat(r.MarginSummary.AccountValue)
	marginUsed := parseFloat(r.MarginSummary.TotalMarginUsed)
	if accountValue == 0 && r.CrossMarginSummary.AccountValue != "" {
		accountValue = parseFloat(r.CrossMarginSummary.AccountValue)
	}
	if marginUsed == 0 && r.CrossMarginSummary.TotalMarginUsed != "" {
		marginUsed = parseFloat(r.CrossMarginSummary.TotalMarginUsed)
	}

	state := WalletState{
		Account: WalletAccount{
			Address:       address,
			AccountValue:  accountValue,
			MarginUsed:    marginUsed,
			Withdrawable:  parseFloat(r.Withdrawable),
			RefreshedAt:   receivedAt,
			RawReceivedAt: receivedAt,
		},
		Positions: make([]WalletPosition, 0, len(r.AssetPositions)),
	}

	for _, asset := range r.AssetPositions {
		size := parseFloat(asset.Position.Szi)
		if size == 0 {
			continue
		}

		positionValue := math.Abs(parseFloat(asset.Position.PositionValue))
		markPrice := 0.0
		if size != 0 {
			markPrice = positionValue / math.Abs(size)
		}

		state.Positions = append(state.Positions, WalletPosition{
			Address:          address,
			Symbol:           strings.ToUpper(asset.Position.Coin),
			PositionSize:     size,
			EntryPrice:       parseFloat(asset.Position.EntryPx),
			MarkPrice:        markPrice,
			LiqPrice:         parseFloat(asset.Position.LiquidationPx),
			Leverage:         asset.Position.Leverage.Value,
			MarginBalance:    parseFloat(asset.Position.MarginUsed),
			PositionValueUSD: positionValue,
			UnrealizedPnL:    parseFloat(asset.Position.UnrealizedPnL),
			RefreshedAt:      receivedAt,
		})
	}
	return state
}
