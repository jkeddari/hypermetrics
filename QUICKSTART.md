# 🚀 Quick Start Guide

## In 3 Minutes

### 1. Run Locally

```bash
# Clone and enter directory
cd hypermetrics

# Run all tests
make test

# Build and run
make build
./hypermetrics
```

Server starts on `http://localhost:8080` 🎉

### 2. Test the API

**Health Check:**
```bash
curl http://localhost:8080/health
```

**Live Leaderboard (Current Feature - v0.1.0):**
```bash
# Get leaderboard sorted by account value (default)
curl http://localhost:8080/api/info/leaderboard

# Get top daily PNL traders
curl "http://localhost:8080/api/info/leaderboard?window=daily&metric=pnl"

# Get top weekly ROI traders
curl "http://localhost:8080/api/info/leaderboard?window=weekly&metric=roi"

# Get highest volume traders
curl "http://localhost:8080/api/info/leaderboard?window=monthly&metric=vlm"
```

**Coming Soon:**
- Large position tracking
- Wallet-level analytics
- Real-time alerts

### 3. Deploy to Google Cloud Run

```bash
# Set your project
gcloud config set project YOUR_PROJECT_ID

# Deploy
gcloud run deploy hypermetrics \
  --source . \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated
```

Done! Your API is live 🌐

---

## Common Commands

```bash
# Development
make run              # Run locally
make test             # Run tests
make test-coverage    # Generate coverage report
make lint             # Run all linters

# Building
make build            # Build binary
make clean            # Clean artifacts

# Docker
make docker-build     # Build image
make docker-run       # Run container

# Help
make help             # Show all commands
```

---

## Project Structure

```
hypermetrics/
├── cmd/server/               # Main application
├── internal/
│   ├── metrics/
│   │   └── leaderboard/      # ✅ Live leaderboard (v0.1.0)
│   │   └── positions/        # 🔜 Position tracker (planned)
│   │   └── wallet/           # 🔜 Wallet analytics (planned)
│   └── server/               # HTTP server
├── Makefile                 # Build automation
├── Dockerfile.backend       # Backend container definition
├── README.md                # Full documentation
└── DEPLOYMENT.md            # Cloud deployment guide
```

---

## Current Features (v0.1.0)

✅ **Live Leaderboard**
- Real-time tracking of Hyperliquid's trading leaderboard
- Flexible sorting by PNL, ROI, Volume
- Multiple time windows (daily, weekly, monthly, all-time)
- Sub-50ms response times with in-memory caching

## Upcoming Features

🔜 **Large Position Tracker**
- Monitor significant positions across the platform
- Real-time position change alerts
- Historical data and trends

🔜 **Wallet Analytics**
- Comprehensive address-level analytics
- Trading history and P&L tracking
- Portfolio monitoring

---

## What's Next?

- 📖 Read [README.md](README.md) for full documentation and roadmap
- ☁️ See [DEPLOYMENT.md](DEPLOYMENT.md) for production deployment
- 🐛 Found a bug? Open an [issue](https://github.com/jkeddari/hypermetrics/issues)
- 💡 Have an idea? Submit a [pull request](https://github.com/jkeddari/hypermetrics/pulls)

---

**Pro Tips:**

- Use `make help` to see all available commands
- Tests run in parallel for speed (< 2 seconds!)
- Binary is optimized with `-ldflags="-s -w"` (6.3 MB)
- Docker image uses multi-stage build (~20 MB final image)
- Cloud Run auto-scales from 0 to 10 instances
- Check the [roadmap](README.md#-development-roadmap) for upcoming features

Happy coding! 🎉
