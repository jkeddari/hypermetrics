package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/jkeddari/hypermetrics/internal/model"
	"github.com/sonirico/go-hyperliquid"
	"golang.org/x/time/rate"
)

const topSymbolLimit = 10

// cachedDashboardData wraps dashboard data with a timestamp for TTL management
type cachedDashboardData struct {
	data     *model.DashboardData
	cachedAt time.Time
}

// DashboardService handles dashboard data fetching from Hyperliquid API
type DashboardService struct {
	client      *hyperliquid.Info
	cache       *lru.Cache[string, *cachedDashboardData]
	cacheTTL    time.Duration
	rateLimiter *rate.Limiter
}

// NewDashboardService creates a new dashboard service
func NewDashboardService() *DashboardService {
	ctx := context.Background()

	// Initialize Hyperliquid Info client
	info := hyperliquid.NewInfo(
		ctx,
		"https://api.hyperliquid.xyz", // Mainnet
		true,                          // skipWS - no WebSocket needed
		nil,                           // meta - auto-fetch
		nil,                           // spotMeta - auto-fetch
		nil,                           // perpDexs - auto-fetch
	)

	// Initialize LRU cache with size 1 (only one dashboard entry needed)
	cache, _ := lru.New[string, *cachedDashboardData](1)

	return &DashboardService{
		client:      info,
		cache:       cache,
		cacheTTL:    30 * time.Second,
		rateLimiter: rate.NewLimiter(rate.Every(time.Minute/60), 60),
	}
}

// GetDashboardData returns dashboard data from cache or fetches fresh data
func (svc *DashboardService) GetDashboardData(ctx context.Context) (*model.DashboardData, error) {
	const cacheKey = "dashboard"

	// Try cache first
	if cached, ok := svc.cache.Get(cacheKey); ok {
		// Check if cache is still valid (within TTL)
		if time.Since(cached.cachedAt) <= svc.cacheTTL {
			return cached.data, nil
		}
	}

	// Cache miss or expired - fetch fresh data
	freshData, err := svc.fetchFreshData(ctx)
	if err != nil {
		slog.Warn("Failed to fetch fresh dashboard data", "error", err)

		// Try to return stale data as fallback
		if cached, ok := svc.cache.Get(cacheKey); ok {
			staleData := *cached.data // Copy to avoid mutation
			staleData.IsStale = true
			return &staleData, nil
		}

		return nil, fmt.Errorf("fetch dashboard data: %w", err)
	}

	// Update cache with fresh data
	svc.cache.Add(cacheKey, &cachedDashboardData{
		data:     freshData,
		cachedAt: time.Now(),
	})

	return freshData, nil
}

// fetchFreshData fetches dashboard data from Hyperliquid API
func (svc *DashboardService) fetchFreshData(ctx context.Context) (*model.DashboardData, error) {
	// Wait for rate limiter
	if err := svc.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	// Call Hyperliquid API
	metaAndCtxs, err := svc.client.MetaAndAssetCtxs(ctx)
	if err != nil {
		return nil, fmt.Errorf("API call failed: %w", err)
	}

	// Aggregate metrics across all perpetuals
	metrics := svc.aggregateMetrics(metaAndCtxs.Ctxs)

	// Get top 10 symbols by open interest
	topSymbols := svc.getTopSymbols(metaAndCtxs.Universe, metaAndCtxs.Ctxs)

	return &model.DashboardData{
		Metrics:     metrics,
		TopSymbols:  topSymbols,
		LastUpdated: time.Now(),
		IsStale:     false,
	}, nil
}

// aggregateMetrics sums up open interest in USD, volume, and averages funding rate
func (svc *DashboardService) aggregateMetrics(ctxs []hyperliquid.AssetCtx) model.DashboardMetrics {
	var totalOI float64
	var totalVolume float64

	for _, ctx := range ctxs {
		// Parse open interest and calculate in USD
		if oi, err := strconv.ParseFloat(ctx.OpenInterest, 64); err == nil {
			// Get price to calculate OI in USD
			if price, err := strconv.ParseFloat(ctx.MarkPx, 64); err == nil {
				totalOI += oi * price // OI in USD = contracts × price
			}
		}

		// Parse daily volume (already in USD from API)
		if vol, err := strconv.ParseFloat(ctx.DayNtlVlm, 64); err == nil {
			totalVolume += vol
		}

	}

	return model.DashboardMetrics{
		TotalOpenInterest: totalOI,
		TotalDailyVolume:  totalVolume,
		// TODO: add real alert number
		AlertNumber: 0,
	}
}

// getTopSymbols returns top N symbols sorted by open interest in USD
func (svc *DashboardService) getTopSymbols(universe []hyperliquid.AssetInfo, ctxs []hyperliquid.AssetCtx) []model.Symbol {
	type symbolData struct {
		name             string
		openInterest     float64 // In contracts
		openInterestUSD  float64 // In USD (OI × Price)
		dailyVolume      float64
		price            float64
		diffPrice        float64
		diffPricePercent float64
		funding          float64
	}

	// Collect and parse data
	var symbols []symbolData
	for i, assetInfo := range universe {
		if i >= len(ctxs) {
			break
		}

		ctx := ctxs[i]

		oi, err := strconv.ParseFloat(ctx.OpenInterest, 64)
		if err != nil {
			continue
		}

		vol, _ := strconv.ParseFloat(ctx.DayNtlVlm, 64)
		price, _ := strconv.ParseFloat(ctx.MarkPx, 64)
		prevPrice, _ := strconv.ParseFloat(ctx.PrevDayPx, 64)
		funding, _ := strconv.ParseFloat(ctx.Funding, 64)

		// Calculate open interest in USD (contracts × price)
		openInterestUSD := oi * price

		// Calculate 24H price change
		var diffPricePercent float64
		if prevPrice > 0 {
			diffPricePercent = (price - prevPrice) / prevPrice * 100
		}

		symbols = append(symbols, symbolData{
			name:             assetInfo.Name,
			openInterest:     oi,
			openInterestUSD:  openInterestUSD,
			dailyVolume:      vol,
			price:            price,
			diffPrice:        price - prevPrice,
			diffPricePercent: diffPricePercent,
			funding:          funding,
		})
	}

	// Sort by open interest in USD descending
	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i].openInterestUSD > symbols[j].openInterestUSD
	})

	limit := len(symbols)
	if limit > topSymbolLimit {
		limit = topSymbolLimit
	}

	// Build and return top symbols
	topSymbols := make([]model.Symbol, limit)
	for i := 0; i < limit; i++ {
		sym := symbols[i]
		topSymbols[i] = model.Symbol{
			Name:              sym.name,
			Price:             sym.price,
			PriceChange24h:    sym.diffPrice,
			PriceChange24hPct: sym.diffPricePercent,
			FundingRate:       sym.funding,
			DailyVolume:       sym.dailyVolume,
			OpenInterest:      sym.openInterestUSD, // OI in USD, not contracts
		}
	}

	return topSymbols
}
