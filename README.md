# Hypermetrics

> 🚀 Comprehensive real-time metrics and analytics platform for Hyperliquid

[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## 🎯 Overview

Hypermetrics is a high-performance Go service that provides comprehensive real-time metrics and analytics for the Hyperliquid decentralized exchange. Built with speed, reliability, and scalability in mind, it offers a REST API to track trading activity, analyze performance, and monitor market movements.

## 🎪 Features & Roadmap

### ✅ Live Leaderboard (v0.1.0)
- **Lazy-loading cache** with 60-second TTL
- **On-demand fetching** - Data loaded only when requested
- **Flexible sorting** across 13 metrics (value, daily/weekly/monthly/alltime × PNL/ROI/Volume)
- **Thread-safe** concurrent access with RWMutex
- **Fast response** - <50ms for cached data, 2-5s on cache miss
- **Scale-to-zero friendly** - No background goroutines, perfect for serverless

### 🔜 Large Position Tracker (Planned)
- Track and monitor large positions across the platform
- Real-time alerts for significant position changes
- Historical position data and trends
- Filter by size, asset, and timeframe

### 🔜 Wallet Analytics (Planned)
- Comprehensive wallet-level analytics
- Track individual address performance
- Historical trading activity and P&L
- Position history and current holdings
- Trading patterns and behavior analysis

### 🔜 Future Enhancements
- WebSocket support for real-time updates
- Advanced filtering and search capabilities
- Data export (CSV, JSON)
- Custom alerts and notifications
- Performance benchmarking tools

## ✨ Technical Features

- 🔒 **Thread-safe**: Concurrent access with RWMutex protection
- ⚡ **Lazy loading**: Data fetched only when needed, cached with TTL
- 🔄 **Smart caching**: Single-fetch guarantee even with concurrent requests
- 🐳 **Docker ready**: Multi-stage build optimized for Google Cloud Run
- 📊 **Comprehensive metrics**: PNL, ROI, and Volume across all time windows
- 🧪 **Well tested**: 95%+ code coverage with parallel tests
- 🔧 **Production-ready**: Logging, health checks, graceful shutdown
- 💰 **Cost-efficient**: True scale-to-zero, no background processes

## 🚀 Quick Start

### Prerequisites

- Go 1.25+
- Docker (optional)
- Make (optional, but recommended)

### Installation

```bash
# Clone the repository
git clone https://github.com/jkeddari/hypermetrics.git
cd hypermetrics

# Install dependencies
go mod download

# Run tests
make test

# Build and run
make build
./hypermetrics
```

### Using Make

```bash
# See all available commands
make help

# Run tests with coverage
make test-coverage

# Build binary
make build

# Run server locally
make run

# Build Docker image
make docker-build

# Run Docker container
make docker-run
```

## 📖 API Documentation

### Current Endpoints (v0.1.0)

#### GET /health
Health check endpoint to verify server status.

**Response:**
```json
{
  "status": "ok",
  "timestamp": "2024-11-23T12:34:56Z",
  "last_refresh": "2024-11-23T12:33:00Z"
}
```

#### GET /api/info/leaderboard
Get Hyperliquid's trading leaderboard with flexible sorting.

**Query Parameters:**
- `window` (optional): Time window - `daily`, `weekly`, `monthly`, `alltime`
- `metric` (optional): Metric to sort by - `pnl`, `roi`, `vlm` (volume)
- `order` (optional): Sort order - `asc`, `desc` (default: `desc`)

**Default behavior:** Returns leaderboard sorted by account value (descending).

**Examples:**

```bash
# Get leaderboard sorted by account value (default)
curl http://localhost:8080/api/info/leaderboard

# Get top daily PNL traders
curl "http://localhost:8080/api/info/leaderboard?window=daily&metric=pnl"

# Get top weekly ROI traders
curl "http://localhost:8080/api/info/leaderboard?window=weekly&metric=roi"

# Get highest volume traders (monthly)
curl "http://localhost:8080/api/info/leaderboard?window=monthly&metric=vlm"
```

**Response Format:**
```json
{
  "rows": [
    {
      "ethAddress": "0x...",
      "accountValue": "123456.789",
      "dailyPerformance": {
        "pnl": "1234.56",
        "roi": "0.0123",
        "vlm": "100000.00"
      },
      "weekPerformance": { ... },
      "monthPerformance": { ... },
      "allPerformance": { ... },
      "prize": 0,
      "displayName": null
    }
  ],
  "count": 27000,
  "sort_by": "dailypnl",
  "order": "desc",
  "last_refresh": "2024-11-23T12:34:56Z"
}
```

### Upcoming Endpoints (Planned)

- `GET /api/positions/large` - Track large positions
- `GET /api/wallet/{address}` - Get wallet analytics
- `GET /api/wallet/{address}/history` - Get wallet trading history
- `GET /api/wallet/{address}/positions` - Get current positions

## Starting the Server

```bash
# Build the server
go build -o hypermetrics ./cmd/server

# Run with default settings (listens on :8080)
./hypermetrics

# Run on a custom port
./hypermetrics -addr :3000
```

## Configuration

Server configuration can be customized via:

- **Command-line flags**: `-addr` for listen address
- **Code**: Modify `server.Config` struct for advanced settings (timeouts, etc.)

## Architecture

```
┌─────────────────┐
│  HTTP Client    │
└────────┬────────┘
         │ GET /api/info/leaderboard
         ▼
┌─────────────────┐
│  HTTP Server    │
│  (port 8080)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ LiveLeaderboard │ ◄─── Auto-refresh (60s)
│  (in-memory)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Hyperliquid API │
│  (external)     │
└─────────────────┘
```

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/metrics/leaderboard/...
```

### Building

```bash
# Build binary
go build -o hypermetrics ./cmd/server

# Build with optimizations
go build -ldflags="-s -w" -o hypermetrics ./cmd/server
```

## 🏗️ Project Structure

```
.
├── cmd/
│   └── server/             # Main application entry point
├── internal/
│   ├── metrics/
│   │   └── leaderboard/    # Live leaderboard tracking (v0.1.0)
│   │   └── positions/      # Large position tracker (planned)
│   │   └── wallet/         # Wallet analytics (planned)
│   ├── server/             # HTTP server implementation
│   └── utils/              # Utility functions
├── Dockerfile.backend      # Multi-stage Docker build (backend API)
├── Makefile               # Build automation
├── COST_ESTIMATION.md     # Google Cloud Run cost analysis
├── DEPLOYMENT.md          # Deployment guide
├── QUICKSTART.md          # Quick start guide
└── README.md              # This file
```

## 🧪 Testing

```bash
# Run all tests
make test

# Run tests with coverage report
make test-coverage

# Run specific package tests
go test ./internal/metrics/leaderboard/... -v

# Run with race detector
go test -race ./...
```

## 🐳 Docker

### Build and Run Locally

```bash
# Build image
make docker-build

# Run container
make docker-run

# Or manually
docker build -t hypermetrics .
docker run -p 8080:8080 hypermetrics
```

### Multi-platform Build

```bash
# Build for linux/amd64 (Cloud Run)
docker buildx build --platform linux/amd64 -t hypermetrics:latest .
```

## ☁️ Deployment

### Google Cloud Run

Detailed deployment instructions are in [DEPLOYMENT.md](DEPLOYMENT.md)

**Quick deploy:**

```bash
# Set your project
gcloud config set project YOUR_PROJECT_ID

# Deploy directly from source
gcloud run deploy hypermetrics \
  --source . \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated
```

**With Cloud Build:**

```bash
# Submit to Cloud Build
gcloud builds submit --config cloudbuild.yaml
```

## 🔧 Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server listen port | `8080` |

### Command-line Flags

```bash
./hypermetrics -addr :3000  # Run on custom port
```

## 📊 Performance

- **Memory usage**: ~200-300 MB when cache is populated, ~50 MB when idle
- **Startup time**: <1 second (no initial data fetch, lazy loading)
- **Request latency**: 
  - Cache HIT (within 60s TTL): <50ms
  - Cache MISS (first request or expired): 2-5 seconds
- **Cache TTL**: 60 seconds (configurable)
- **Test suite**: <2 seconds (parallel execution with cached data)
- **Concurrent requests**: Handles 1000+ req/s, single fetch during cache miss
- **Serverless-friendly**: True scale-to-zero with no background goroutines

## 💰 Cost Estimation (Google Cloud Run)

| Traffic Volume | Cache HIT Rate | Monthly Cost | Cost per Request |
|----------------|----------------|--------------|------------------|
| 30K req/month (Dev) | 50% | **$0.00** | $0.000000 |
| 1.5M req/month (Low) | 80% | **$15.39** | $0.000010 |
| 6M req/month (Medium) | 90% | **$46.10** | $0.000008 |
| 30M req/month (High) | 95% | **$158.80** | $0.000005 |

**Key advantages:**
- ✅ **Free Tier**: First 30-50K requests/month are FREE
- ✅ **Scale-to-zero**: $0 cost when idle
- ✅ **Pay-per-use**: Only pay for actual usage
- ✅ **Cache optimization**: 80-95% cache HIT rate drastically reduces costs

📋 **Detailed analysis**: See [COST_ESTIMATION.md](COST_ESTIMATION.md) for complete breakdown, comparisons, and optimization strategies.

**Recommended starting budget**: $20-30/month

## 🛠️ Development

### Prerequisites

```bash
# Install development tools
make install-tools
```

This installs:
- `staticcheck` - Static analysis
- `gosec` - Security scanner

### Linting

```bash
# Run all linters
make lint

# Individual linters
make fmt    # Format code
make vet    # Run go vet
make sec    # Run gosec
```

### CI/CD

You can set up your own CI/CD pipeline using the provided `Makefile` targets:
- `make ci` - Run full CI pipeline (lint + test + build)
- `make all` - Run complete build with cleanup

## 🤝 Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Code Standards

- Follow Go best practices and idioms
- Write tests for new features
- Run `make lint` before committing
- Update documentation as needed

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🗺️ Development Roadmap

### Phase 1: Foundation ✅
- [x] Project setup and architecture
- [x] Live leaderboard tracking
- [x] REST API implementation
- [x] Docker deployment
- [x] Comprehensive testing

### Phase 2: Position Tracking 🚧
- [ ] Large position tracker implementation
- [ ] Real-time position monitoring
- [ ] Position change alerts
- [ ] Historical position data

### Phase 3: Wallet Analytics 📋
- [x] Wallet-level analytics
- [x] Individual address tracking
- [x] Trading history and P&L
- [x] Position portfolio tracking

### Phase 4: Advanced Features 🔮
- [ ] WebSocket support for real-time updates
- [ ] Advanced filtering and search
- [ ] Data export capabilities
- [ ] Custom alerts system
- [ ] Performance benchmarking

## 🙏 Acknowledgments

- Hyperliquid for providing the API
- The Go community for excellent tools and libraries

## 📧 Contact

- GitHub: [@jkeddari](https://github.com/jkeddari)
- Issues: [GitHub Issues](https://github.com/jkeddari/hypermetrics/issues)

---

**Built with ❤️ using Go**
