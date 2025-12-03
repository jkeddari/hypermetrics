# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Development
```bash
# Run all tests with race detector
go test -v -race -timeout 30s ./...

# Run tests for a specific package
go test -v ./internal/metrics/leaderboard/...

# Run tests with coverage
make test-coverage

# Run linters (go fmt, go vet, gosec, staticcheck)
make lint

# Install development tools (staticcheck, gosec)
make install-tools
```

### Building
```bash
# Build binary with optimizations
make build

# Build manually
go build -ldflags="-s -w" -o hypermetrics ./cmd/server

# Run server locally (default port 8080)
make run

# Run on custom port
./hypermetrics -addr :3000
```

### Docker
```bash
# Build Docker image
make docker-build

# Run Docker container
make docker-run

# Manual Docker build
docker build -f Dockerfile.backend -t hypermetrics:latest .
```

### CI Pipeline
```bash
# Run full CI pipeline (deps, lint, test, build)
make ci

# Run complete build with cleanup
make all
```

## Architecture

### High-Level Design

Hypermetrics is a REST API service that provides real-time metrics and analytics for the Hyperliquid DEX. The architecture follows a lazy-loading cache pattern optimized for serverless environments (Google Cloud Run) with scale-to-zero capability.

**Key Design Principles:**
- **Lazy Loading**: Data fetched only when requested, not on startup or background
- **TTL-based Caching**: 60-second cache expiration for fresh data without constant API calls
- **Thread-Safe**: RWMutex protection with single-fetch guarantee during cache miss
- **Scale-to-Zero Friendly**: No background goroutines or timers

### Package Structure

```
internal/
├── metrics/
│   ├── leaderboard/    # Leaderboard tracking with lazy-loading cache
│   ├── user/           # User info fetching (perp, spot, orders, funding)
│   └── positions/      # Large position tracker (in progress)
├── server/             # HTTP server and route handlers
└── utils/              # Rate limiting and HTTP utilities
```

### Core Components

#### 1. Leaderboard (`internal/metrics/leaderboard/`)

Manages Hyperliquid's trading leaderboard with lazy-loading cache.

**Cache Strategy:**
- First request OR after 60s TTL: Fetch from Hyperliquid API (2-5s)
- Within TTL: Return cached data (<50ms)
- Concurrent requests during fetch: Single fetch, others wait

**Thread Safety:**
- `mu (RWMutex)`: Protects `board` and `lastRefreshed`
- `fetchMutex`: Ensures only one fetch at a time
- Double-check locking in `ensureFreshData()` prevents redundant fetches

**Sorting Capabilities:**
- Sort by account value, or by window+metric (e.g., "dailypnl", "weeklyroi")
- Time windows: daily, weekly, monthly, alltime
- Metrics: PNL, ROI, VLM (volume)

#### 2. User Info (`internal/metrics/user/`)

Fetches comprehensive user data from Hyperliquid API.

**Data Aggregation:**
- `InfoUser()` combines multiple API calls:
  - Perpetual positions (clearinghouseState)
  - Spot balances (spotClearinghouseState)
  - Open orders (openOrders)
  - Funding payments (userFunding, last 30 days)

**Batch Operations:**
- `BatchPerpStates()`: Fetches multiple users in single request
- Tries `batchClearinghouseStates` endpoint first
- Falls back to sequential requests if batch fails
- Returns map[address]*PerpState

**Rate Limiting:**
- Default: 10 req/s via `utils.NewRoundTripper`
- 30-second timeout per request

#### 3. Server (`internal/server/`)

HTTP server with RESTful API endpoints.

**Endpoints:**
- `GET /health` - Health check with last refresh timestamp
- `GET /api/info/leaderboard` - Leaderboard with query params (window, metric, order)
- `GET /api/info/user/{address}` - Comprehensive user info
- `GET /api/positions/large` - Large position tracker (in progress)

**Middleware:**
- Logging middleware: Captures method, path, status, duration
- Uses structured logging (slog)

**Timeouts:**
- Read: 10s, Write: 10s, Idle: 60s
- User info context: 30s
- Large positions context: 2m (for batch fallback)

**Graceful Shutdown:**
- Listens for SIGINT/SIGTERM
- 10-second shutdown timeout

### Data Flow

```
┌─────────────────┐
│  HTTP Client    │
└────────┬────────┘
         │ GET /api/info/leaderboard?window=daily&metric=pnl
         ▼
┌─────────────────┐
│  HTTP Server    │
│  (server.go)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ handleLeaderboard│
│  - Parse params  │
│  - Build sortCode│
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│LiveLeaderboard  │
│ .SortBoards()   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ ensureFreshData │ ◄── Thread-safe cache check
│  - Check TTL    │     (RWMutex + fetchMutex)
│  - Fetch if old │
└────────┬────────┘
         │
         │ (cache miss)
         ▼
┌─────────────────┐
│ Hyperliquid API │
│   (external)    │
└─────────────────┘
```

### Concurrency Patterns

**Lazy Loading Cache (leaderboard)**
1. Fast path: RLock → check TTL → RUnlock (concurrent reads OK)
2. Cache miss: Lock fetchMutex → double-check with RLock → fetch → Lock → update → Unlock
3. Result: Only one fetch during cache miss, others wait and reuse result

**Batch Operations (user)**
1. Try batch endpoint first (fast, single request for N users)
2. If batch fails: Fall back to sequential requests
3. Context timeout: 2 minutes to allow for fallback

### Important Implementation Notes

**When Adding New Metrics:**
1. Consider lazy-loading pattern for scale-to-zero compatibility
2. Use TTL-based caching (60s default) to balance freshness vs API load
3. Implement double-check locking for thread-safe single-fetch guarantee
4. Never use background goroutines (breaks scale-to-zero)

**When Adding New Endpoints:**
1. Follow RESTful conventions in `server.go`
2. Add route to `setupRoutes()`
3. Use structured logging with context (address, filters, etc.)
4. Set appropriate context timeouts based on expected latency
5. Return proper HTTP status codes and error messages

**JSON Field Naming:**
- External API responses: camelCase (e.g., `accountValue`)
- Hyperliquid API: Matches their format (e.g., `entryPx`, `szi`)
- Use json struct tags to map between Go names and JSON

**Error Handling:**
- Wrap errors with context: `fmt.Errorf("context: %w", err)`
- Log errors with structured fields before returning
- Return user-friendly error messages in API responses
- Don't expose internal error details to API clients

**Testing:**
- Tests run with `-race` flag to catch race conditions
- Use parallel tests (`t.Parallel()`) for faster execution
- Mock HTTP clients for testing without external dependencies
- 30-second timeout per test suite

### Dependencies

- `golang.org/x/time/rate`: Rate limiting for API calls
- `github.com/stretchr/testify`: Test assertions and mocking
- Standard library: `net/http`, `encoding/json`, `log/slog`, `sync`

### Deployment Context

Designed for **Google Cloud Run** with:
- True scale-to-zero (no background processes)
- Automatic HTTPS and load balancing
- PORT environment variable (defaults to 8080)
- Health check endpoint for orchestration
- Efficient memory usage (~50MB idle, ~200-300MB with cache)

**Performance Characteristics:**
- Startup time: <1 second (no initial data fetch)
- Cache HIT: <50ms
- Cache MISS: 2-5 seconds (Hyperliquid API latency)
- Handles 1000+ req/s concurrent load
