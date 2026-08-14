# Hypermetrics API Spec

## Overview

Hypermetrics is a Hyperliquid-only data product.

The public v1 API intentionally follows the CoinGlass Hyperliquid API surface as closely as possible so builders, bots, scripts, and internal tools can adopt it with minimal mapping.

Compatibility target:

- same endpoint families
- same `GET` style
- same response envelope
- same query parameter names where possible
- same timestamp convention: milliseconds since Unix epoch

Base URL:

```txt
https://api.hypermetrics.xyz
```

Local development:

```txt
http://localhost:8080
```

## Authentication

All API endpoints are private and require an API key.

Clients must send:

```http
HM-API-KEY: <api_key>
```

CoinGlass uses `CG-API-KEY`. Hypermetrics intentionally uses `HM-API-KEY`.

## Plans

- `Free`: no API access
- `Builder`: access to all v1 REST endpoints
- `Pro`: all Builder endpoints plus future realtime features such as WebSocket and alerts

## Response Envelope

All endpoints return a common JSON envelope:

```json
{
  "code": "0",
  "msg": "success",
  "data": {}
}
```

Error shape:

```json
{
  "code": "1000",
  "msg": "invalid api key",
  "data": null
}
```

## Standard Error Codes

- `0`: success
- `1000`: authentication failed
- `1001`: missing required parameter
- `1002`: invalid parameter
- `1003`: resource not found
- `1004`: rate limit exceeded
- `1005`: upstream unavailable
- `1006`: internal error

## Endpoint Conventions

- All v1 endpoints are `GET`
- Query params use snake_case
- Timestamps are returned in milliseconds since Unix epoch
- Numeric fields are returned as JSON numbers
- Wallet addresses are returned as lowercase hex strings when available
- Position size sign follows Hyperliquid convention: positive is long, negative is short
- `position_value_usd` is absolute notional value in USD unless stated otherwise

---

## 1. Hyperliquid Whale Alert

### Endpoint

```http
GET /api/hyperliquid/whale-alert
```

### CoinGlass Compatibility

Reference endpoint:

```txt
https://open-api-v4.coinglass.com/api/hyperliquid/whale-alert
```

This endpoint returns recent large Hyperliquid position changes, prioritizing events with notional value above USD 1,000,000. It should return up to the 200 most recent records.

### Query Params

- none in v1

### Response

```json
{
  "code": "0",
  "msg": "success",
  "data": [
    {
      "user": "0x3fd4444154242720c0d0c61c74a240d90c127d33",
      "symbol": "ETH",
      "position_size": 12700,
      "entry_price": 1611.62,
      "liq_price": 527.2521,
      "position_value_usd": 21003260,
      "position_action": 1,
      "create_time": 1745219517000
    }
  ]
}
```

### Fields

- `user`: wallet address
- `symbol`: Hyperliquid perp coin
- `position_size`: signed position size, positive long and negative short
- `entry_price`: position entry price
- `liq_price`: liquidation price when available
- `position_value_usd`: absolute notional value
- `position_action`: CoinGlass-compatible action code
  - `1`: open or increase
  - `2`: close or decrease
- `create_time`: event timestamp in milliseconds

### Hypermetrics Notes

Internally, Hypermetrics can classify richer actions (`open`, `increase`, `decrease`, `close`, `flip`, `liquidation`). Public v1 returns the CoinGlass-compatible `position_action` code. A future non-compatible endpoint can expose the richer classification.

---

## 2. Hyperliquid Whale Position

### Endpoint

```http
GET /api/hyperliquid/whale-position
```

### CoinGlass Compatibility

Reference endpoint:

```txt
https://open-api-v4.coinglass.com/api/hyperliquid/whale-position
```

This endpoint returns current Hyperliquid positions with notional value above USD 1,000,000.

### Query Params

- none in v1

### Response

```json
{
  "code": "0",
  "msg": "success",
  "data": [
    {
      "user": "0x20c2d95a3dfdca9e9ad12794d5fa6fad99da44f5",
      "symbol": "ETH",
      "position_size": -44727.1273,
      "entry_price": 2249.7,
      "mark_price": 1645.8,
      "liq_price": 2358.2766,
      "leverage": 25,
      "margin_balance": 2943581.7019,
      "position_value_usd": 73589542.5467,
      "unrealized_pnl": -27002174.34
    }
  ]
}
```

---

## 3. Hyperliquid Wallet Positions By Coin

### Endpoint

```http
GET /api/hyperliquid/position
```

### CoinGlass Compatibility

Reference endpoint:

```txt
https://open-api-v4.coinglass.com/api/hyperliquid/position
```

This endpoint returns current wallet positions for a single Hyperliquid coin.

### Query Params

- `symbol` required, for example `BTC`
- `current_page` optional, default `1`

### Example

```http
GET /api/hyperliquid/position?symbol=ETH&current_page=1
```

### Response

```json
{
  "code": "0",
  "msg": "success",
  "data": [
    {
      "user": "0xabc0000000000000000000000000000000000000",
      "symbol": "ETH",
      "position_size": 1023.4,
      "entry_price": 3100.0,
      "mark_price": 3150.0,
      "liq_price": 2800.0,
      "leverage": 5,
      "margin_balance": 45678.9,
      "position_value_usd": 3222450.0,
      "unrealized_pnl": 3750.0
    }
  ]
}
```

### Pagination

`current_page` is CoinGlass-compatible. Page size is implementation-defined in v1 and should remain stable. Recommended v1 page size: `100`.

---

## 4. Hyperliquid Wallet Positions By Address

### Endpoint

```http
GET /api/hyperliquid/user-position
```

### CoinGlass Compatibility

Reference endpoint:

```txt
https://open-api-v4.coinglass.com/api/hyperliquid/user-position
```

This endpoint returns margin summary and open positions for a single Hyperliquid wallet.

### Query Params

- `user_address` required

Compatibility alias:

- `user` may be accepted for backward compatibility with early Hypermetrics clients, but `user_address` is canonical.

### Example

```http
GET /api/hyperliquid/user-position?user_address=0x20c2d95a3dfdca9e9ad12794d5fa6fad99da44f5
```

### Response

```json
{
  "code": "0",
  "msg": "success",
  "data": {
    "user": "0x20c2d95a3dfdca9e9ad12794d5fa6fad99da44f5",
    "margin_summary": {
      "account_value": 123456.78,
      "margin_used": 45678.12,
      "withdrawable": 77778.66
    },
    "asset_positions": [
      {
        "symbol": "BTC",
        "position_size": 2.5,
        "entry_price": 82000,
        "mark_price": 83500,
        "liq_price": 70200,
        "leverage": 5,
        "position_value_usd": 208750,
        "unrealized_pnl": 3750
      }
    ]
  }
}
```

### Hyperliquid Source

This endpoint is served from PostgreSQL. On a cache miss or stale row, the API asks the Core service to refresh the wallet through NATS request/reply, then reads the committed state from PostgreSQL.

---

## 5. Hyperliquid Wallet Position Distribution

### Endpoint

```http
GET /api/hyperliquid/wallet/position-distribution
```

### CoinGlass Compatibility

Reference endpoint:

```txt
https://open-api-v4.coinglass.com/api/hyperliquid/wallet/position-distribution
```

This endpoint returns current wallet distribution grouped by position size tiers.

### Query Params

- none in v1

### Response

```json
{
  "code": "0",
  "data": [
    {
      "group_name": "shrimp",
      "all_address_count": 1200,
      "position_address_count": 700,
      "position_address_percent": 58.33,
      "bias_score": 0.32,
      "bias_remark": "bullish",
      "minimum_amount": 0,
      "maximum_amount": 250,
      "long_position_usd": 15200000,
      "short_position_usd": 9800000,
      "long_position_usd_percent": 60.8,
      "short_position_usd_percent": 39.2,
      "position_usd": 25000000,
      "profit_address_count": 420,
      "loss_address_count": 280,
      "profit_address_percent": 60,
      "loss_address_percent": 40
    }
  ]
}
```

### Calculation

Wallets are grouped by current account value:

- `shrimp`: `[0, 250)`
- `fish`: `[250, 2,500)`
- `dolphin`: `[2,500, 25,000)`
- `apex_predator`: `[25,000, 100,000)`
- `small_whale`: `[100,000, 1,000,000)`
- `whale`: `[1,000,000, 10,000,000)`
- `tidal_whale`: `[10,000,000, 100,000,000)`
- `leviathan`: `[100,000,000, +∞)`; `maximum_amount` is `0`

Each wallet is counted once. Position values and unrealized PnL are aggregated across all its open positions. `bias_score` is:

```txt
(net-long wallet count - net-short wallet count)
/
(net-long wallet count + net-short wallet count)
```

All percentage fields are rounded to two decimals and return `0` for an empty denominator.

---

## 6. Hyperliquid Wallet PnL Distribution

### Endpoint

```http
GET /api/hyperliquid/wallet/pnl-distribution
```

### CoinGlass Compatibility

Reference endpoint:

```txt
https://open-api-v4.coinglass.com/api/hyperliquid/wallet/pnl-distribution
```

This endpoint returns current wallet distribution grouped by unrealized PnL tiers.

### Query Params

- none in v1

### Response

```json
{
  "code": "0",
  "msg": "success",
  "data": [
    {
      "tier": "-100k to -10k",
      "address_count": 340,
      "long_position_value": 4200000,
      "short_position_value": 6100000,
      "sentiment": "bearish",
      "profit_loss_distribution": {
        "profit": 120,
        "loss": 220
      }
    }
  ]
}
```

### Calculation

For each wallet, bucket by total unrealized PnL:

- `< -1m`
- `-1m to -100k`
- `-100k to -10k`
- `-10k to 0`
- `0 to 10k`
- `10k to 100k`
- `100k to 1m`
- `1m+`

---

## 7. Hyperliquid Global Long Short Account Ratio History

### Endpoint

```http
GET /api/hyperliquid/global-long-short-account-ratio/history
```

### CoinGlass Compatibility

Reference endpoint:

```txt
https://open-api-v4.coinglass.com/api/hyperliquid/global-long-short-account-ratio/history
```

This endpoint returns historical long/short account ratio data for Hyperliquid symbols.

### Query Params

- `symbol` required, for example `BTC`
- `interval` required
- `limit` optional, default `1000`, maximum `1000`
- `start_time` optional
- `end_time` optional

### Supported Intervals

CoinGlass Hyperliquid-compatible intervals:

- `5m`
- `1h`
- `1d`

### Example

```http
GET /api/hyperliquid/global-long-short-account-ratio/history?symbol=BTC&interval=1h&limit=500
```

### Response

```json
{
  "code": "0",
  "msg": "success",
  "data": [
    {
      "time": 1740000000000,
      "symbol": "BTC",
      "long_account": 0.61,
      "short_account": 0.39,
      "long_short_ratio": 1.56
    }
  ]
}
```

### Calculation

For each symbol and interval:

- `long_account = long_wallet_count / positioned_wallet_count`
- `short_account = short_wallet_count / positioned_wallet_count`
- `long_short_ratio = long_wallet_count / short_wallet_count`

Only wallets with a non-zero open position for the symbol are counted.

---

## Backend Data Requirements

The API cannot be fully served as a stateless proxy over Hyperliquid. Hyperliquid can return account state for a known wallet, but it does not expose a single canonical endpoint for all open positions across all wallets.

Hypermetrics therefore needs a local data pipeline:

1. Market ingestion
   - Source: Hyperliquid `meta`, `metaAndAssetCtxs`, `allMids`
   - Stores supported coins, mark prices, funding, open interest, and market metadata

2. Wallet discovery
   - Source A: Hyperliquid public leaderboard
   - Source B: manual allowlist or customer watchlists

3. Wallet state refresh
   - Source: Hyperliquid `clearinghouseState`
   - Stores account value, withdrawable, margin used, open positions, liquidation prices, and unrealized PnL

4. Current position serving
   - Reads from `wallet_positions_current`
   - Powers `position`, `user-position`, and `whale-position`

5. Snapshot aggregation
   - Stores periodic `wallet_position_snapshots`
   - Powers `position-distribution`, `pnl-distribution`, and `global-long-short-account-ratio/history`

6. Whale alert generation
   - Compares current and previous wallet position state
   - Emits events when notional or notional delta is above the configured whale threshold

## Hyperliquid Leaderboard Seed

The Hyperliquid leaderboard can be used as the initial source for high-value or high-performing wallets.

Observed public stats endpoint:

```txt
https://stats-data.hyperliquid.xyz/Mainnet/leaderboard
```

Expected useful fields:

- wallet address, commonly exposed as `eth_address`
- account value
- display name when available
- window performances such as day, week, month, and all-time
- PnL, ROI, and volume per performance window

The leaderboard is useful to bootstrap high-signal wallet candidates. It is not enough by itself to reproduce CoinGlass-style position endpoints because the leaderboard is performance-oriented, not a full open-position feed. Each discovered address still needs a `clearinghouseState` validation refresh before it is promoted into `wallets`.

Recommended ingestion behavior:

- poll leaderboard periodically, for example every 5 to 15 minutes
- store discovered high-value addresses as validation candidates
- promote candidates into `wallets` only when real `clearinghouseState.accountValue` is above the whale threshold
- tag source as `leaderboard`
- prioritize refreshes by account value, recent PnL, ROI, or volume

## Recommended Storage Model

Minimum tables:

- `api_keys`
- `markets`
- `asset_contexts_current`
- `wallets`
- `wallet_leaderboard_snapshots`
- `wallet_account_snapshots`
- `wallet_positions_current`
- `wallet_position_snapshots`
- `whale_alerts`
- `position_distribution_snapshots`
- `pnl_distribution_snapshots`
- `long_short_ratio_snapshots`
- `ingestion_runs`

## Implementation Priority

1. API key validation
2. Hyperliquid REST client
3. `user-position` via `clearinghouseState`
4. Market metadata via `metaAndAssetCtxs`
5. Leaderboard ingestion to seed large wallets
6. Wallet refresh queue with rate limiting
7. `position?symbol=...&current_page=...`
8. `whale-position`
9. Position snapshots
10. `whale-alert`
11. Distribution snapshots
12. Long/short account ratio history

## Source References

- https://docs.coinglass.com/reference/hyperliquid-whale-alert
- https://docs.coinglass.com/reference/hyperliquid-whale-position
- https://docs.coinglass.com/reference/hyperliquid-position
- https://docs.coinglass.com/reference/hyperliquid-user-position
- https://docs.coinglass.com/reference/hyperliquid-wallet-position-distribution
- https://docs.coinglass.com/reference/hyperliquid-wallet-pnl-distribution
- https://docs.coinglass.com/reference/hyperliquid-long-short-account-ratio-history
- https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/info-endpoint
- https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/info-endpoint/perpetuals
- https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/websocket/subscriptions
- https://stats-data.hyperliquid.xyz/Mainnet/leaderboard
