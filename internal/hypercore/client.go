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
	"time"
)

const defaultInfoURL = "https://api.hyperliquid.xyz/info"

type WalletStateClient interface {
	GetClearinghouseState(ctx context.Context, address string) (WalletState, error)
}

type HyperliquidClient struct {
	infoURL    string
	httpClient *http.Client
}

func NewHyperliquidClient(infoURL string, httpClient *http.Client) *HyperliquidClient {
	if infoURL == "" {
		infoURL = defaultInfoURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &HyperliquidClient{
		infoURL:    infoURL,
		httpClient: httpClient,
	}
}

func (c *HyperliquidClient) GetClearinghouseState(ctx context.Context, address string) (WalletState, error) {
	address = NormalizeAddress(address)
	if !IsAddress(address) {
		return WalletState{}, fmt.Errorf("invalid wallet address: %q", address)
	}

	requestBody, err := json.Marshal(map[string]string{
		"type": "clearinghouseState",
		"user": address,
	})
	if err != nil {
		return WalletState{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.infoURL, bytes.NewReader(requestBody))
	if err != nil {
		return WalletState{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return WalletState{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return WalletState{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return WalletState{}, fmt.Errorf("hyperliquid info status %d: %s", resp.StatusCode, string(body))
	}

	var raw clearinghouseStateResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return WalletState{}, err
	}
	return raw.toWalletState(address, time.Now().UTC()), nil
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
	accountValue := ParseFloat(r.MarginSummary.AccountValue)
	marginUsed := ParseFloat(r.MarginSummary.TotalMarginUsed)
	if accountValue == 0 && r.CrossMarginSummary.AccountValue != "" {
		accountValue = ParseFloat(r.CrossMarginSummary.AccountValue)
	}
	if marginUsed == 0 && r.CrossMarginSummary.TotalMarginUsed != "" {
		marginUsed = ParseFloat(r.CrossMarginSummary.TotalMarginUsed)
	}

	state := WalletState{
		Account: WalletAccount{
			Address:       address,
			AccountValue:  accountValue,
			MarginUsed:    marginUsed,
			Withdrawable:  ParseFloat(r.Withdrawable),
			RefreshedAt:   receivedAt,
			RawReceivedAt: receivedAt,
		},
		Positions: make([]WalletPosition, 0, len(r.AssetPositions)),
	}

	for _, asset := range r.AssetPositions {
		size := ParseFloat(asset.Position.Szi)
		if size == 0 {
			continue
		}

		positionValue := math.Abs(ParseFloat(asset.Position.PositionValue))
		markPrice := 0.0
		if size != 0 {
			markPrice = positionValue / math.Abs(size)
		}

		state.Positions = append(state.Positions, WalletPosition{
			Address:          address,
			Symbol:           strings.ToUpper(asset.Position.Coin),
			PositionSize:     size,
			EntryPrice:       ParseFloat(asset.Position.EntryPx),
			MarkPrice:        markPrice,
			LiqPrice:         ParseFloat(asset.Position.LiquidationPx),
			Leverage:         asset.Position.Leverage.Value,
			MarginBalance:    ParseFloat(asset.Position.MarginUsed),
			PositionValueUSD: positionValue,
			UnrealizedPnL:    ParseFloat(asset.Position.UnrealizedPnL),
			RefreshedAt:      receivedAt,
		})
	}
	return state
}
