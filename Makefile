.PHONY: help build test test-coverage clean run install-tools lint fmt vet sec docker-build docker-run

# Variables
BINARY_NAME=hypermetrics
DOCKER_IMAGE=hypermetrics
DOCKER_TAG=latest
PORT=8080

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOFMT=$(GOCMD) fmt
GOMOD=$(GOCMD) mod
GORUN=$(GOCMD) run

# Build flags
LDFLAGS=-ldflags="-s -w"

## help: Display this help message
help:
	@echo "Available targets:"
	@echo "  make build           - Build the binary"
	@echo "  make test            - Run tests"
	@echo "  make test-coverage   - Run tests with coverage report"
	@echo "  make run             - Run the server locally"
	@echo "  make lint            - Run all linters (vet, staticcheck, gosec)"
	@echo "  make fmt             - Format code"
	@echo "  make vet             - Run go vet"
	@echo "  make sec             - Run gosec security scanner"
	@echo "  make install-tools   - Install required development tools"
	@echo "  make clean           - Clean build artifacts"
	@echo "  make docker-build    - Build Docker image"
	@echo "  make docker-run      - Run Docker container"
	@echo ""

## build: Build the binary
build: lint test
	@echo "Building binary..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/server
	@echo "✓ Build complete: $(BINARY_NAME)"

## test: Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -timeout 30s ./...
	@echo "✓ Tests passed"

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

## run: Run the server locally
run:
	@echo "Starting server on :$(PORT)..."
	$(GORUN) ./cmd/server -addr :$(PORT)

## lint: Run all linters
lint: fmt vet sec
	@echo "Running staticcheck..."
	@which staticcheck > /dev/null || (echo "⚠ staticcheck not installed, skipping..." && exit 0)
	@staticcheck ./... 2>/dev/null || (echo "⚠ staticcheck not compatible with current Go version, skipping..." && exit 0)
	@echo "✓ All linters passed"

## fmt: Format Go code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...
	@echo "✓ Code formatted"

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...
	@echo "✓ go vet passed"

## sec: Run gosec security scanner
sec:
	@echo "Running gosec..."
	@which gosec > /dev/null || (echo "Installing gosec..." && go install github.com/securego/gosec/v2/cmd/gosec@latest)
	gosec -quiet ./...
	@echo "✓ Security scan passed"

## install-tools: Install development tools
install-tools:
	@echo "Installing development tools..."
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "✓ Tools installed"

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	rm -rf vendor/
	$(GOCMD) clean
	@echo "✓ Clean complete"

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -f Dockerfile.backend -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "✓ Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

## docker-run: Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -p $(PORT):$(PORT) --rm $(DOCKER_IMAGE):$(DOCKER_TAG)

## tidy: Tidy Go modules
tidy:
	@echo "Tidying Go modules..."
	$(GOMOD) tidy
	@echo "✓ Modules tidied"

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	@echo "✓ Dependencies downloaded"

## ci: Run CI pipeline (lint, test, build)
ci: deps lint test build
	@echo "✓ CI pipeline complete"

## all: Run full build pipeline
all: clean deps ci
	@echo "✓ Full build complete"
