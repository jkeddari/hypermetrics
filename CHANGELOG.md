# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Phase 2: Position Tracking
- Large position tracker implementation
- Real-time position monitoring
- Position change alerts
- Historical position data

### Phase 3: Wallet Analytics
- Wallet-level analytics
- Individual address tracking
- Trading history and P&L
- Position portfolio tracking

### Phase 4: Advanced Features
- WebSocket support for real-time updates
- Advanced filtering and search
- Data export capabilities
- Custom alerts system
- Performance benchmarking

## [0.1.0] - 2024-11-23

### Added
- **Lazy-loading cache architecture** with 60s TTL for optimal serverless deployment
- REST API with flexible sorting capabilities (13 sort combinations)
- Support for multiple time windows (daily, weekly, monthly, all-time)
- Support for multiple metrics (PNL, ROI, Volume)
- In-memory caching with sub-50ms response times on cache HIT
- Thread-safe concurrent access with RWMutex
- **Single-fetch guarantee**: Multiple concurrent requests during cache miss result in single fetch
- Health check endpoint
- Docker support with multi-stage builds
- Comprehensive test suite with 95%+ coverage
- Production-ready logging and error handling
- Graceful shutdown
- Google Cloud Run deployment ready
- Complete documentation (README, QUICKSTART, DEPLOYMENT guides)

### Architecture
- **Lazy Loading Strategy**: 
  - Data fetched only when requested (on-demand)
  - No background goroutines → true scale-to-zero
  - TTL-based cache expiration (60 seconds)
  - Double-check locking pattern for thread safety
- **Performance Characteristics**:
  - Cache HIT: <50ms response time
  - Cache MISS: 2-5 seconds (Hyperliquid API latency)
  - Memory: ~200-300 MB when cached, ~50 MB when idle
  - Startup: <1 second (no initial data fetch)

### Technical
- Go 1.25 support
- Makefile with automated linting and testing
- gosec security scanning
- go vet static analysis
- Parallel test execution with cached test data
- Rate limiting for external API calls (20 req/s)

## Project Vision

Hypermetrics aims to be a comprehensive real-time metrics and analytics platform for Hyperliquid, providing traders and analysts with powerful tools to track performance, monitor positions, and analyze market activity.
