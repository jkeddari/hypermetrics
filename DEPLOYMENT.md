# Deployment Guide - Google Cloud Run

## Overview

Hypermetrics is designed to be easily deployed on Google Cloud Run, taking advantage of its auto-scaling capabilities and pay-per-use pricing model. The application is containerized using Docker for consistent deployment across environments.

## Prerequisites

1. **Google Cloud SDK** installed and configured
2. **Docker** installed locally
3. **Google Cloud Project** with billing enabled
4. **Cloud Run API** enabled

## Initial Setup

### 1. Install Google Cloud SDK

```bash
# macOS
brew install --cask google-cloud-sdk

# Or download from: https://cloud.google.com/sdk/docs/install
```

### 2. Authenticate and Configure

```bash
# Login to Google Cloud
gcloud auth login

# Set your project ID
gcloud config set project YOUR_PROJECT_ID

# Enable required APIs
gcloud services enable cloudbuild.googleapis.com
gcloud services enable run.googleapis.com
gcloud services enable containerregistry.googleapis.com
```

## Deployment Options

### Option 1: Direct Docker Deployment (Recommended for testing)

```bash
# Build and test locally first
make docker-build  # Uses Dockerfile.backend
make docker-run

# Or build manually
docker build -f Dockerfile.backend -t hypermetrics:latest .

# Tag for Google Container Registry
docker tag hypermetrics:latest gcr.io/YOUR_PROJECT_ID/hypermetrics:latest

# Push to GCR
docker push gcr.io/YOUR_PROJECT_ID/hypermetrics:latest

# Deploy to Cloud Run
gcloud run deploy hypermetrics \
  --image gcr.io/YOUR_PROJECT_ID/hypermetrics:latest \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --port 8080 \
  --memory 512Mi \
  --cpu 1
```

### Option 2: Continuous Deployment from GitHub

```bash
# Connect your GitHub repository
gcloud run deploy hypermetrics \
  --source . \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated

```

## Configuration

### Environment Variables

Set environment variables in Cloud Run:

```bash
gcloud run services update hypermetrics \
  --region us-central1 \
  --update-env-vars KEY1=VALUE1,KEY2=VALUE2
```

### Custom Domain

```bash
# Map custom domain
gcloud run domain-mappings create \
  --service hypermetrics \
  --domain api.yourdomain.com \
  --region us-central1
```

### Scaling Configuration

```bash
# Update scaling settings
gcloud run services update hypermetrics \
  --region us-central1 \
  --min-instances 0 \
  --max-instances 10 \
  --cpu 1 \
  --memory 512Mi \
  --timeout 300
```

## Local Development & Testing

### Test Docker Build Locally

```bash
# Build (uses Dockerfile.backend)
make docker-build

# Or build manually
docker build -f Dockerfile.backend -t hypermetrics:latest .

# Run locally
docker run -p 8080:8080 hypermetrics:latest

# Test the endpoint
curl http://localhost:8080/health
curl "http://localhost:8080/api/info/leaderboard?window=daily&metric=pnl"
```

### Test with Docker Compose (Optional)

Create `docker-compose.yml`:

```yaml
version: '3.8'
services:
  hypermetrics:
    build: .
    ports:
      - "8080:8080"
    environment:
      - LOG_LEVEL=info
    restart: unless-stopped
```

Run with:
```bash
docker-compose up
```

## Monitoring & Logs

### View Logs

```bash
# Tail logs
gcloud run logs tail hypermetrics --region us-central1

# Read recent logs
gcloud run logs read hypermetrics --region us-central1 --limit 50

# Filter by severity
gcloud run logs read hypermetrics --region us-central1 --log-filter="severity>=ERROR"
```

### Metrics

View in Google Cloud Console:
- Navigate to Cloud Run > hypermetrics
- Click on "Metrics" tab
- Monitor: Request count, Latency, Memory usage, CPU usage

**Key metrics to watch:**
- **Request latency**: Should stay <100ms for leaderboard queries
- **Memory usage**: ~200-300 MB with current features
- **Error rate**: Should be <1%
- **Instance count**: Auto-scales based on traffic

## Cost Optimization

Cloud Run pricing is based on:
- **Requests**: First 2 million requests/month are free
- **Compute time**: Pay only when handling requests
- **Memory**: 512Mi recommended (current leaderboard feature)

**Estimated costs** (assuming moderate traffic):
- ~$5-20/month for small to medium traffic
- Free tier covers up to 2M requests/month
- Additional features (position tracker, wallet analytics) may require more resources

**Tips to reduce costs**:
```bash
# Set minimum instances to 0 (no cold starts charged)
gcloud run services update hypermetrics \
  --min-instances 0 \
  --region us-central1

# Adjust memory based on enabled features
# Current: 512Mi for leaderboard
# Future: May need 1Gi with position tracking + wallet analytics
gcloud run services update hypermetrics \
  --memory 512Mi \
  --region us-central1
```

## Security Best Practices

### 1. Use Secret Manager for sensitive data

```bash
# Create secret
echo -n "secret-value" | gcloud secrets create my-secret --data-file=-

# Grant access to Cloud Run service account
gcloud secrets add-iam-policy-binding my-secret \
  --member=serviceAccount:YOUR_SERVICE_ACCOUNT \
  --role=roles/secretmanager.secretAccessor

# Use in Cloud Run
gcloud run services update hypermetrics \
  --update-secrets=ENV_VAR=my-secret:latest
```

### 2. Enable authentication (if needed)

```bash
# Require authentication
gcloud run services update hypermetrics \
  --no-allow-unauthenticated \
  --region us-central1

# Generate auth token
gcloud auth print-identity-token
```

### 3. Set up VPC connector (for private resources)

```bash
# Create VPC connector
gcloud compute networks vpc-access connectors create my-connector \
  --network default \
  --region us-central1 \
  --range 10.8.0.0/28

# Connect Cloud Run to VPC
gcloud run services update hypermetrics \
  --vpc-connector my-connector \
  --region us-central1
```

## Troubleshooting

### Container fails to start

```bash
# Check logs
gcloud run logs read hypermetrics --region us-central1 --limit 100

# Verify container locally
docker run -it hypermetrics:latest /bin/sh
```

### High memory usage

```bash
# Increase memory
gcloud run services update hypermetrics \
  --memory 1Gi \
  --region us-central1
```

### Cold starts are slow

```bash
# Keep warm with min instances
gcloud run services update hypermetrics \
  --min-instances 1 \
  --region us-central1
```

## Rollback

```bash
# List revisions
gcloud run revisions list --service hypermetrics --region us-central1

# Rollback to specific revision
gcloud run services update-traffic hypermetrics \
  --to-revisions REVISION_NAME=100 \
  --region us-central1
```

## Clean Up

```bash
# Delete Cloud Run service
gcloud run services delete hypermetrics --region us-central1

# Delete container images
gcloud container images delete gcr.io/YOUR_PROJECT_ID/hypermetrics:latest
```

## Additional Resources

- [Cloud Run Documentation](https://cloud.google.com/run/docs)
- [Cloud Build Documentation](https://cloud.google.com/build/docs)
- [Container Registry Documentation](https://cloud.google.com/container-registry/docs)
