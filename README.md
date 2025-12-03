# Hypermetrics

> 📊 Comprehensive analytics and insights platform for Hyperliquid DEX

[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://go.dev/dl/)
[![Private](https://img.shields.io/badge/status-private-red.svg)]()

---

## 🎯 Overview

**Hypermetrics** is a high-performance analytics platform that provides real-time insights about the Hyperliquid decentralized exchange. Built with Go for speed and reliability, it delivers comprehensive visibility into trader activity and performance metrics.

The platform answers critical questions about Hyperliquid's ecosystem:
- Who are the top traders and how are they performing?
- How is any specific address performing across perpetuals and spot markets?
- What are the real-time positions, balances, and P&L for any trader?

## ✨ Current Features

### 🏆 Leaderboard Tracking

Track and analyze top traders on Hyperliquid with comprehensive sorting capabilities:
- **Flexible Sorting** - Rank by account value, PNL, ROI, or trading volume
- **Time Windows** - View performance across daily, weekly, monthly, or all-time periods
- **Real-time Updates** - Fresh data with intelligent caching (60-second TTL)
- **27,000+ Traders** - Complete coverage of the Hyperliquid trading community
- **13+ Metric Combinations** - Sort by value, or any combination of window × metric (e.g., daily PNL, weekly ROI, monthly volume)

**Performance:**
- Cache HIT: <50ms response time
- Cache MISS: 2-5 seconds (fetches fresh data from Hyperliquid)
- Thread-safe concurrent access with single-fetch guarantee

**API Endpoint:** `GET /api/info/leaderboard?window={daily|weekly|monthly|alltime}&metric={pnl|roi|vlm}&order={asc|desc}`

---

### 👤 Address Analytics

Deep dive into any Ethereum address on Hyperliquid with comprehensive data aggregation:

**Account Overview:**
- Total account value
- Withdrawable balance
- Total margin used (cross and isolated)

**Perpetual Positions:**
- Active positions with entry price and current size
- Leverage (cross or isolated) and liquidation price
- Unrealized P&L and position value
- Margin requirements per position

**Spot Holdings:**
- Token balances across all available markets
- Total and held amounts per coin

**Open Orders:**
- Current limit orders for both perp and spot markets
- Order details: side, price, size, timestamp

**Funding Payments:**
- Historical funding payment data (30-day history)
- Per-coin funding rates and USDC amounts

**Performance:**
- Response time: 2-5 seconds (aggregates 4 separate API calls)
- Rate-limited at 10 requests/second for API protection
- Fresh data on every request (no caching)

**API Endpoint:** `GET /api/info/user/{address}`

---

### 🏥 Health Monitoring

System health check endpoint for monitoring and orchestration:
- Server status indicator
- Current timestamp
- Last leaderboard cache refresh time

**API Endpoint:** `GET /health`

---

## 🏗️ Technical Architecture

### Design Principles

**Serverless-First Approach:**
- **Lazy Loading** - Data fetched on-demand, not at startup
- **Smart Caching** - 60-second TTL cache for leaderboard data
- **Scale-to-Zero** - No background processes or goroutines
- **Cost-Efficient** - Perfect for Google Cloud Run and similar platforms

**Performance Characteristics:**
- Startup time: <1 second (no initial data load)
- Memory usage: ~50MB idle, ~200-300MB with populated cache
- Concurrent requests: Handles 1000+ req/s
- Thread-safe: RWMutex protection with double-check locking pattern

### Technology Stack

- **Language:** Go 1.25+
- **API:** RESTful HTTP endpoints with JSON responses
- **Caching:** In-memory with TTL-based expiration (leaderboard only)
- **Concurrency:** Thread-safe with RWMutex and fetch mutex
- **Deployment:** Docker-ready, optimized for Google Cloud Run
- **Logging:** Structured JSON logging with request middleware

### Key Features

- ✅ Thread-safe concurrent access patterns
- ✅ Intelligent caching with single-fetch guarantee
- ✅ Rate limiting for external API calls (10-20 req/s)
- ✅ Graceful shutdown with signal handling
- ✅ Production-ready error handling and logging
- ✅ Comprehensive test coverage (95%+)

---

## 🗺️ Roadmap

### Phase 1: Position Tracking 🚧 *In Progress*

Real-time whale monitoring and large position tracking:
- [ ] **Position Aggregation** - Aggregate positions across top traders
- [ ] **Large Position Detection** - Identify and track positions above configurable thresholds
- [ ] **Flexible Filtering** - Filter by minimum value, asset type, and leverage
- [ ] **Batch Optimization** - Efficient batch fetching with sequential fallback
- [ ] **API Endpoint** - `GET /api/positions/large` with query parameters

**Use Cases:**
- Identify large concentrated positions in specific assets
- Monitor high-leverage trades (≥10x)
- Track whale entry/exit activity
- Detect potential liquidation risks

---

### Phase 2: Protocol Metrics 📋 *Planned*

Track core Hyperliquid protocol indicators:
- [ ] **Total HYPE Staked** - Monitor staking participation and protocol security
- [ ] **Funding Rates** - Real-time funding rates across all perpetual markets
- [ ] **HYPE Buyback Activity** - Protocol buyback volume and frequency
- [ ] **Daily Generated Fees** - Revenue metrics and protocol health
- [ ] **Global Trading Volume** - Aggregate trading activity across markets
- [ ] **Open Interest** - Total notional value of open positions
- [ ] **API Endpoint** - `GET /api/protocol/metrics`

---

### Phase 3: Advanced Features 🔮 *Future*

**Near-Term:**
- [ ] **Historical Data** - Time-series data for positions, balances, and performance
- [ ] **WebSocket Support** - Real-time streaming updates for positions and leaderboard
- [ ] **Advanced Filtering** - Complex queries across multiple dimensions
- [ ] **Position Change Detection** - Track and alert on significant position changes

**Mid-Term:**
- [ ] **Alert System** - Customizable notifications for whale activity and liquidations
- [ ] **Trade History Analysis** - Comprehensive fill history and performance attribution
- [ ] **Data Export** - CSV/JSON exports for external analysis

**Long-Term:**
- [ ] **Predictive Analytics** - ML-powered insights on trader behavior
- [ ] **Cross-Protocol Integration** - Comparative analytics across multiple DEXs
- [ ] **Social Features** - Trader following and community insights
- [ ] **Performance Benchmarking** - Compare performance against market segments

---

## 📊 Current Status

### Implemented & Production-Ready ✅
- Live leaderboard tracking with flexible sorting (13+ metrics)
- Comprehensive wallet analytics for any address
- RESTful API with health checks
- Thread-safe concurrent access with smart caching
- Structured logging and error handling
- Graceful shutdown and signal handling
- Rate limiting for API protection

### In Active Development 🚧
- Large position tracking and whale monitoring
- Position aggregation across top traders
- Batch API optimization with fallback strategies

### Planned for Future Releases 📋
- Protocol-level metrics (staking, fees, buybacks)
- Historical data and time-series analysis
- Real-time alerting infrastructure
- WebSocket streaming
- Advanced data export and visualization

---

## 🔒 License

This is a **private project**. All rights reserved.

Unauthorized copying, distribution, or use of this software is strictly prohibited.

---

**Built with ⚡ by [@jkeddari](https://github.com/jkeddari)**
