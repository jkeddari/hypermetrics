# Wallet Refresh Queue

## Purpose

The refresh queue decides which wallets should be refreshed through Hyperliquid `clearinghouseState`.

This is the central backend component for the product. Hypermetrics cannot refresh every wallet uniformly at high frequency, so the queue must prioritize wallets that are most likely to matter.

## Constraints

Hyperliquid public REST limit:

```txt
1200 weight / minute / IP
```

`clearinghouseState` weight:

```txt
2
```

Maximum theoretical throughput:

```txt
600 wallets / minute / IP
10 wallets / second / IP
```

Recommended operational throughput:

```txt
6-8 wallets / second / IP
```

The queue should be built around the safe rate, not the theoretical maximum.

## Queue Inputs

Wallets and wallet candidates enter or update the queue from several signals:

- leaderboard poller, as validation candidates first
- manual seed list
- known whale refresh loop
- stale wallet scanner
- failed refresh retries
- user API request cache miss

Leaderboard candidates are scanned with `clearinghouseState` before becoming indexed wallets. A candidate is retained when either its real account value or the absolute value of one of its positions reaches the whale threshold.

Each signal should update wallet or candidate metadata and then recompute priority.

## Wallet Metadata Used For Scoring

Recommended fields:

- `wallet_address`
- `first_seen_at`
- `last_seen_leaderboard_at`
- `last_refreshed_at`
- `last_successful_refresh_at`
- `last_failed_refresh_at`
- `refresh_attempts`
- `consecutive_failures`
- `known_whale`
- `has_open_position`
- `max_position_value_usd`
- `total_position_value_usd`
- `account_value`
- `leaderboard_rank`
- `leaderboard_account_value`
- `leaderboard_pnl`
- `leaderboard_roi`
- `manual_priority_boost`
- `next_refresh_at`
- `priority_score`

## Priority Tiers

Use hard tiers first, then a score inside each tier.

### Tier 0: On-Demand

Triggered by a user calling `user-position` for a wallet that is missing or older than five minutes.

Target refresh:

```txt
immediate, unless refreshed very recently
```

The refresh is protected by the same PostgreSQL lease as background work. Concurrent requests for one stale wallet therefore produce one upstream call and wait for the same stored result.

### Tier 1: Known Whales

Wallets with recent known whale positions:

```txt
max_position_value_usd >= 1000000
```

Target refresh:

```txt
10-60 seconds
```

Use shorter intervals for larger positions.

### Tier 2: Leaderboard Wallets

Wallets recently seen on the leaderboard.

Target refresh:

```txt
1-5 minutes
```

Prioritize by:

- account value
- rank
- recent PnL
- ROI
- volume when available

### Tier 3: Warm Wallets

Wallets previously seen with positions, but not recently active.

Target refresh:

```txt
15-60 minutes
```

### Tier 4: Cold Wallets

Wallets discovered once but never important, or wallets with no recent position.

Target refresh:

```txt
1-24 hours
```

Cold wallets should not consume much quota.

## Priority Score

A simple first version can use additive scoring:

```txt
priority_score =
  stale_score
+ known_whale_score
+ leaderboard_score
+ open_position_score
+ account_value_score
+ manual_boost
- recent_refresh_penalty
- failure_penalty
```

Higher score means higher priority.

## Suggested Scoring Formula

### Staleness

```txt
age_seconds = now - last_refreshed_at
target_interval_seconds = interval_for_tier(wallet)
stale_ratio = age_seconds / target_interval_seconds
stale_score = min(stale_ratio, 5) * 20
```

Examples:

- just refreshed: near `0`
- due now: `20`
- very stale: up to `100`

### Known Whale Score

```txt
known_whale_score =
  if max_position_value_usd >= 10000000 then 100
  else if max_position_value_usd >= 5000000 then 75
  else if max_position_value_usd >= 1000000 then 50
  else 0
```

### Leaderboard Score

```txt
leaderboard_score =
  if leaderboard_rank <= 100 then 80
  else if leaderboard_rank <= 500 then 50
  else if leaderboard_rank <= 1000 then 30
  else if last_seen_leaderboard_at within 24h then 15
  else 0
```

Add account value influence:

```txt
account_value_score = min(log10(max(account_value, 1)) * 8, 80)
```

### Open Position Score

```txt
open_position_score =
  if has_open_position then 25
  else 0
```

### Manual Boost

Manual boost should be explicit and bounded:

```txt
manual_boost = min(manual_priority_boost, 100)
```

### Recent Refresh Penalty

Avoid refreshing the same wallet too often:

```txt
seconds_since_refresh = now - last_refreshed_at
recent_refresh_penalty =
  if seconds_since_refresh < 10 then 1000
  else if seconds_since_refresh < 30 then 200
  else if seconds_since_refresh < 60 then 75
  else 0
```

### Failure Penalty

Back off failing wallets:

```txt
failure_penalty = consecutive_failures * 50
```

Also set `next_refresh_at` with exponential backoff:

```txt
backoff = min(2 ^ consecutive_failures minutes, 60 minutes)
next_refresh_at = now + backoff
```

## Tier Target Intervals

Recommended starting values:

```txt
on_demand:       immediate, with per-key and per-wallet guardrails
known_whale:     10-60 seconds
leaderboard:     1-5 minutes
recent_active:   1-15 minutes
warm:            15-60 minutes
cold:            1-24 hours
```

The interval should shrink as wallet importance increases.

Example:

```txt
if max_position_value_usd >= 10000000:
  target_interval = 10s
else if max_position_value_usd >= 5000000:
  target_interval = 30s
else if max_position_value_usd >= 1000000:
  target_interval = 60s
```

## Queue Selection Algorithm

The implemented worker runs at seven scans per second and alternates two SQL orderings:

```txt
scans 1-4: priority lane (highest score first)
scan 5:   coverage lane (earliest deadline / stalest first)
repeat
```

Both lanes borrow unused capacity from the other because they select from the same due set. Jobs are atomically leased with:

```sql
WITH next_job AS (
    SELECT wallet_address
    FROM wallet_refresh_queue
    WHERE next_refresh_at <= now()
      AND (locked_until IS NULL OR locked_until <= now())
    ORDER BY priority_score DESC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE wallet_refresh_queue
SET locked_until = now() + interval '20 seconds',
    locked_by = $worker
FROM next_job
WHERE wallet_refresh_queue.wallet_address = next_job.wallet_address;
```

The coverage deadline cannot be pushed forward by a leaderboard re-poll. Initial caps are one minute for positions above $10M, two minutes above $5M, five minutes above $1M, 24 hours for leaderboard wallets and 48 hours for cold wallets.

## Rate Limiter

The API process uses a fixed ticker. The default is:

```txt
rate: 7 clearinghouseState calls / second / process
```

PostgreSQL leases make multiple workers safe for deduplication, but the rate limit is process-local. Before deploying multiple API replicas behind the same egress IP, the global seven-per-second budget must be divided between replicas or moved to a shared limiter.

## Refresh Result Handling

### Success

On success:

- reset `consecutive_failures`
- update `last_successful_refresh_at`
- update account current row
- replace current positions for the wallet
- append the account snapshot
- append position snapshots
- detect position changes for whale alerts
- recompute tier and score
- set `next_refresh_at`

### No Open Positions

If a wallet has no open positions:

- set `has_open_position = false`
- reduce priority unless it is leaderboard or manually boosted
- increase target interval

### Failure

On failure:

- increment `consecutive_failures`
- store error class
- apply exponential backoff
- do not delete current position data immediately
- mark stale data if it exceeds freshness thresholds

## Whale Alert Detection

Run detection after a successful refresh by comparing previous current positions with new current positions.

Actions:

- `open`: previous size `0`, new size non-zero
- `close`: previous size non-zero, new size `0`
- `increase`: same sign, absolute size increased
- `decrease`: same sign, absolute size decreased
- `flip`: sign changed

Only emit whale alerts when:

```txt
abs(new_position_value_usd) >= 1000000
or abs(delta_position_value_usd) >= 1000000
```

Avoid false positives from mark price movement:

```txt
delta_position_size = abs(new_size - old_size)
delta_notional = delta_position_size * mark_price
```

Use `delta_notional`, not just the difference between old and new `position_value_usd`.

## Deduplication

Use a deterministic alert key:

```txt
wallet_address
symbol
action
rounded_position_size_delta
time_bucket
```

Recommended first time bucket:

```txt
60 seconds
```

This prevents repeated alerts when the same wallet is refreshed several times after one change.

## Freshness Classes

Expose or track freshness internally:

```txt
fresh:      refreshed <= 60 seconds ago
warm:       refreshed <= 15 minutes ago
stale:      refreshed <= 1 hour ago
very_stale: refreshed > 1 hour ago
```

Endpoints can decide whether to include stale data. For example, `whale-position` should probably exclude very stale rows.

## Scale Estimates

At `7 wallets / second / IP`:

```txt
1,000 wallets:     ~2.4 minutes
10,000 wallets:    ~23.8 minutes
100,000 wallets:   ~4 hours
1,000,000 wallets: ~39.7 hours
```

This is why the queue must not refresh every wallet uniformly.

## Local Development

`task db:up` starts PostgreSQL from `compose.yml`. `task dev` starts PostgreSQL and the API. Unit tests run with `task test`; `task test:integration` additionally supplies `TEST_DATABASE_URL` and exercises migrations, SQL queries, leases and concurrent on-demand refreshes against PostgreSQL.

## Metrics To Track

Required operational metrics:

- wallets known
- wallets refreshed per minute
- queue size by tier
- average wallet freshness by tier
- max wallet staleness by tier
- successful refreshes
- failed refreshes
- Hyperliquid rate limit errors
- refresh latency
- whale alerts generated
- stale rows served by API
- skipped refreshes due to recent refresh penalty

If these metrics are missing, it will be hard to know whether the API is trustworthy.
