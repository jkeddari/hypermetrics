# Fetch Command

CLI tool to fetch external data and store in S3.

## Project Structure

```
hypermetrics/
├── cmd/fetch/              # Fetch command source code
│   ├── main.go            # CLI entry point
│   └── leaderboard.go     # Leaderboard fetch logic
├── Dockerfile.fetch        # Docker build file (at project root)
└── cloudbuild-fetch.yaml   # Cloud Build configuration
```

## Commands

### `fetch leaderboard`

Fetches Hyperliquid leaderboard data, sorts by all metrics, and uploads to S3.

**What it does:**
1. Fetches ~23MB JSON from Hyperliquid API (`https://stats-data.hyperliquid.xyz/Mainnet/leaderboard`)
2. Parses and sorts data by 13 different metrics using the existing `internal/hypermetrics/leaderboard` package
3. Uploads 13 pre-sorted JSON files to S3:
   - `leaderboard/by-value.json` - Sorted by account value
   - `leaderboard/by-daily-pnl.json` - Sorted by daily PNL
   - `leaderboard/by-daily-roi.json` - Sorted by daily ROI
   - `leaderboard/by-daily-vlm.json` - Sorted by daily volume
   - `leaderboard/by-weekly-pnl.json` - Sorted by weekly PNL
   - `leaderboard/by-weekly-roi.json` - Sorted by weekly ROI
   - `leaderboard/by-weekly-vlm.json` - Sorted by weekly volume
   - `leaderboard/by-monthly-pnl.json` - Sorted by monthly PNL
   - `leaderboard/by-monthly-roi.json` - Sorted by monthly ROI
   - `leaderboard/by-monthly-vlm.json` - Sorted by monthly volume
   - `leaderboard/by-alltime-pnl.json` - Sorted by all-time PNL
   - `leaderboard/by-alltime-roi.json` - Sorted by all-time ROI
   - `leaderboard/by-alltime-vlm.json` - Sorted by all-time volume

## Local Development

### Build

```bash
# From project root
go build -o bin/fetch ./cmd/fetch
```

### Run

```bash
# Set environment variables
export S3_ENDPOINT=https://your-project.storage.supabase.co/storage/v1/s3
export S3_BUCKET=your-bucket-name
export S3_ACCESS_KEY=your-access-key
export S3_SECRET_KEY=your-secret-key
export S3_REGION=eu-west-1
export LEADERBOARD_LIMIT=100  # optional, default is 100

# Run the command
./bin/fetch leaderboard
```

### Docker

```bash
# Build (from project root)
docker build -f Dockerfile.fetch -t hypermetrics-fetch .

# Run
docker run \
  -e S3_ENDPOINT=https://your-project.storage.supabase.co/storage/v1/s3 \
  -e S3_BUCKET=your-bucket-name \
  -e S3_ACCESS_KEY=your-access-key \
  -e S3_SECRET_KEY=your-secret-key \
  -e S3_REGION=eu-west-1 \
  -e LEADERBOARD_LIMIT=100 \
  hypermetrics-fetch leaderboard
```

**Note:** Le Dockerfile est à la racine du projet: `Dockerfile.fetch`

## Environment Variables

**Required variables:**

| Variable | Description | Example |
|----------|-------------|---------|
| `S3_ENDPOINT` | S3-compatible endpoint URL | `https://your-project.storage.supabase.co/storage/v1/s3` |
| `S3_BUCKET` | S3 bucket name | `your-bucket-name` |
| `S3_ACCESS_KEY` | S3 access key | `your-access-key` |
| `S3_SECRET_KEY` | S3 secret key | `your-secret-key` |
| `S3_REGION` | S3 region | `eu-west-1` |

**Optional variables:**

| Variable | Description | Default |
|----------|-------------|---------|
| `LEADERBOARD_LIMIT` | Max number of users per leaderboard (top N) | `100` |

**Note:** This command uses Supabase Storage with S3-compatible API.

## Deployment

Deploy to Cloud Run Job using the `Dockerfile.fetch` at the project root.

```bash
# Build and push image
docker build -f Dockerfile.fetch -t REGISTRY/hypermetrics-fetch-leaderboard:latest .
docker push REGISTRY/hypermetrics-fetch-leaderboard:latest

# Deploy Cloud Run Job
gcloud run jobs deploy hypermetrics-fetch-leaderboard \
  --image=REGISTRY/hypermetrics-fetch-leaderboard:latest \
  --region=europe-west1 \
  --memory=512Mi \
  --cpu=1 \
  --max-retries=2 \
  --task-timeout=10m \
  --args=leaderboard
```

## Output

### Console logs

```
INFO loaded S3 configuration bucket=your-bucket region=eu-west-1 endpoint=https://your-project.storage.supabase.co/storage/v1/s3 limit=100
INFO fetching and processing leaderboard data from Hyperliquid API
INFO processing sort combination code=value metric="Account Value"
INFO sorted leaderboard code=value rows=100
INFO processing sort combination code=dailypnl metric="Daily PNL"
INFO sorted leaderboard code=dailypnl rows=100
...
INFO uploaded leaderboard to S3 filename=leaderboard.json size_kb=485
INFO leaderboard fetch completed successfully sort_combinations=13
```

### S3 Structure

```
s3://your-bucket/
└── leaderboard.json           (~500KB with limit=100)
```

Single JSON file containing all 13 sort combinations (value, dailypnl, dailyroi, dailyvlm, weeklypnl, weeklyroi, weeklyvlm, monthlypnl, monthlyroi, monthlyvlm, alltimepnl, alltimeroi, alltimevlm).

### JSON File Format

The file contains all 13 leaderboards in a single JSON:

```json
{
  "last_update": "2025-12-23T15:30:00Z",
  "leaderboard": {
    "value": [
      {
        "ethAddress": "0x1234567890abcdef1234567890abcdef12345678",
        "displayName": "TopTrader",
        "accountValue": "1234567.89",
        "dailyPerformance": {
          "pnl": "12345.67",
          "roi": "0.0523",
          "vlm": "987654.32"
        },
        "weekPerformance": {
          "pnl": "45678.90",
          "roi": "0.1234",
          "vlm": "3456789.01"
        },
        "monthPerformance": {
          "pnl": "123456.78",
          "roi": "0.4567",
          "vlm": "12345678.90"
        },
        "allPerformance": {
          "pnl": "987654.32",
          "roi": "1.2345",
          "vlm": "98765432.10"
        },
        "prize": 0
      }
    ],
    "dailypnl": [...],
    "dailyroi": [...],
    "dailyvlm": [...],
    "weeklypnl": [...],
    "weeklyroi": [...],
    "weeklyvlm": [...],
    "monthlypnl": [...],
    "monthlyroi": [...],
    "monthlyvlm": [...],
    "alltimepnl": [...],
    "alltimeroi": [...],
    "alltimevlm": [...]
  }
}
```

Each array contains up to `LEADERBOARD_LIMIT` users (default: 100).

## Performance

**Typical execution:**
- Fetch time: ~20-30 seconds (Hyperliquid API latency, ~23MB JSON)
- Processing time: ~2-5 seconds (13 sorts + limit to top 100 each)
- Upload time: ~1-2 seconds (single file ~500KB)
- **Total: ~25-40 seconds**

**Resource usage:**
- Memory: ~200-300MB peak
- CPU: 1 vCPU recommended
- Network: ~23MB download + ~500KB upload

## Troubleshooting

### S3 connection errors

```bash
# Test S3 connectivity
aws s3 ls s3://$S3_BUCKET/leaderboard/ --endpoint-url=$S3_ENDPOINT

# Check credentials
echo $S3_ACCESS_KEY
echo $S3_SECRET_KEY
```

### Hyperliquid API timeout

The command has a 45-second timeout for the API request. If it fails:
- Check network connectivity
- Verify Hyperliquid API is accessible: `curl https://stats-data.hyperliquid.xyz/Mainnet/leaderboard | head`

### Out of memory

If running in constrained environment, increase memory allocation:
```bash
# Cloud Run Job
gcloud run jobs update hypermetrics-leaderboard-fetch \
  --memory=1Gi \
  --region=europe-west1
```

## Adding New Commands

To add a new fetch command:

1. Create a new file in `cmd/fetch/`
2. Define a new cobra command
3. Register it in `main.go`:

```go
func init() {
    rootCmd.AddCommand(leaderboardCmd)
    rootCmd.AddCommand(yourNewCmd)  // Add your command here
}
```
