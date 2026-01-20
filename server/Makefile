# Makefile for ODI Server

.PHONY: all build run test test-unit test-integration test-e2e test-coverage test-verbose clean help lint fmt docker-build docker-push docker-run

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# Binary name
BINARY_NAME=server
BINARY_PATH=./cmd/server

# Docker parameters
DOCKER_IMAGE ?= odi-server
DOCKER_TAG ?= latest
DOCKER_REGISTRY ?= 

# Test parameters
TEST_TIMEOUT=300s
COVERAGE_FILE=coverage.out
COVERAGE_HTML=coverage.html

# Environment variables for tests (set in .env or override via command line)
TEST_DATABASE_URL ?= 
TEST_REDIS_ADDR ?= 
TEST_SERVER_URL ?= 

all: build

## Build commands

build:
	@echo "Building..."
	$(GOBUILD) -o $(BINARY_NAME) $(BINARY_PATH)

build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags="-w -s" -o $(BINARY_NAME)-linux-amd64 $(BINARY_PATH)

build-all:
	@echo "Building for all platforms..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags="-w -s" -o dist/$(BINARY_NAME)-linux-amd64 $(BINARY_PATH)
	GOOS=linux GOARCH=arm64 $(GOBUILD) -ldflags="-w -s" -o dist/$(BINARY_NAME)-linux-arm64 $(BINARY_PATH)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags="-w -s" -o dist/$(BINARY_NAME)-darwin-amd64 $(BINARY_PATH)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags="-w -s" -o dist/$(BINARY_NAME)-darwin-arm64 $(BINARY_PATH)

run:
	@echo "Running server..."
	$(GORUN) $(BINARY_PATH)/main.go

## Docker commands

docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-push:
	@echo "Pushing Docker image..."
ifdef DOCKER_REGISTRY
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG)
else
	@echo "Error: DOCKER_REGISTRY not set"
	@exit 1
endif

docker-run:
	@echo "Running Docker container..."
	docker run -p 3000:3000 --env-file .env $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-stop:
	@echo "Stopping Docker container..."
	docker stop $(DOCKER_IMAGE) || true
	docker rm $(DOCKER_IMAGE) || true

## Test commands

# Run all unit tests (no external dependencies required)
test: test-unit

# Run only unit tests (fast, no external dependencies)
test-unit:
	@echo "Running unit tests..."
	$(GOTEST) -v -timeout $(TEST_TIMEOUT) \
		./internal/auth/... \
		./internal/cache/... \
		./internal/config/... \
		./internal/database/... \
		./internal/handlers/... \
		./internal/middleware/... \
		./internal/testutil/...

# Run unit tests with race detection
test-race:
	@echo "Running unit tests with race detection..."
	$(GOTEST) -v -race -timeout $(TEST_TIMEOUT) \
		./internal/...

# Run integration tests (requires PostgreSQL and Redis)
test-integration:
	@echo "Running integration tests..."
	@echo "Requires: TEST_DATABASE_URL and TEST_REDIS_ADDR environment variables"
ifndef TEST_DATABASE_URL
	@echo "Error: TEST_DATABASE_URL not set"
	@exit 1
endif
ifndef TEST_REDIS_ADDR
	@echo "Error: TEST_REDIS_ADDR not set"
	@exit 1
endif
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) \
	TEST_REDIS_ADDR=$(TEST_REDIS_ADDR) \
	$(GOTEST) -v -tags=integration -timeout $(TEST_TIMEOUT) \
		./tests/integration/...

# Run E2E tests (requires running server, PostgreSQL, and Redis)
test-e2e:
	@echo "Running E2E tests..."
	@echo "Requires: TEST_DATABASE_URL, TEST_REDIS_ADDR, and TEST_SERVER_URL environment variables"
ifndef TEST_DATABASE_URL
	@echo "Error: TEST_DATABASE_URL not set"
	@exit 1
endif
ifndef TEST_REDIS_ADDR
	@echo "Error: TEST_REDIS_ADDR not set"
	@exit 1
endif
ifndef TEST_SERVER_URL
	@echo "Error: TEST_SERVER_URL not set"
	@exit 1
endif
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) \
	TEST_REDIS_ADDR=$(TEST_REDIS_ADDR) \
	TEST_SERVER_URL=$(TEST_SERVER_URL) \
	$(GOTEST) -v -tags=e2e -timeout $(TEST_TIMEOUT) \
		./tests/e2e/...

# Run all tests including integration and E2E
test-all: test-unit test-integration test-e2e
	@echo "All tests completed!"

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=$(COVERAGE_FILE) -covermode=atomic \
		./internal/auth/... \
		./internal/cache/... \
		./internal/config/... \
		./internal/database/... \
		./internal/handlers/... \
		./internal/middleware/...
	$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	$(GOTEST) -v -count=1 -timeout $(TEST_TIMEOUT) ./internal/...

# Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem -timeout $(TEST_TIMEOUT) ./internal/...

# Run specific package tests
test-auth:
	$(GOTEST) -v ./internal/auth/...

test-cache:
	$(GOTEST) -v ./internal/cache/...

test-config:
	$(GOTEST) -v ./internal/config/...

test-handlers:
	$(GOTEST) -v ./internal/handlers/...

test-middleware:
	$(GOTEST) -v ./internal/middleware/...

test-database:
	$(GOTEST) -v ./internal/database/...

## Code quality commands

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	$(GOLINT) run ./...

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

# Run all checks
check: fmt vet lint test-unit
	@echo "All checks passed!"

## Dependency management

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# Update dependencies
update-deps:
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy

## Database commands

# Run migrations
migrate:
	@echo "Running migrations..."
	$(GORUN) ./cmd/migrate/main.go

# Verify database
verify-db:
	@echo "Verifying database..."
	$(GORUN) ./cmd/verify-db/main.go

## Cleanup

clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*
	rm -rf dist/
	rm -f $(COVERAGE_FILE)
	rm -f $(COVERAGE_HTML)
	$(GOCMD) clean -testcache

## Docker test infrastructure

# Start test infrastructure
docker-test-up:
	@echo "Starting test infrastructure..."
	docker-compose -f docker-compose.test.yml up -d postgres redis

# Stop test infrastructure
docker-test-down:
	@echo "Stopping test infrastructure..."
	docker-compose -f docker-compose.test.yml down

# Run tests with Docker infrastructure
docker-test: docker-test-up
	@echo "Waiting for services to be ready..."
	@sleep 5
	@make test-all
	@make docker-test-down

## Help

help:
	@echo "ODI Server - Available commands:"
	@echo ""
	@echo "Build & Run:"
	@echo "  make build              Build the server binary"
	@echo "  make build-linux        Build for Linux (amd64)"
	@echo "  make build-all          Build for all platforms"
	@echo "  make run                Run the server"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build       Build Docker image"
	@echo "  make docker-push        Push to registry (requires DOCKER_REGISTRY)"
	@echo "  make docker-run         Run Docker container"
	@echo "  make docker-stop        Stop Docker container"
	@echo ""
	@echo "Testing:"
	@echo "  make test               Run unit tests (alias for test-unit)"
	@echo "  make test-unit          Run unit tests only (fast, no dependencies)"
	@echo "  make test-race          Run unit tests with race detection"
	@echo "  make test-integration   Run integration tests (requires DB & Redis)"
	@echo "  make test-e2e           Run E2E tests (requires running server)"
	@echo "  make test-all           Run all tests"
	@echo "  make test-coverage      Generate test coverage report"
	@echo "  make test-verbose       Run tests with verbose output"
	@echo "  make benchmark          Run benchmarks"
	@echo ""
	@echo "Package-specific tests:"
	@echo "  make test-auth          Test auth package"
	@echo "  make test-cache         Test cache package"
	@echo "  make test-config        Test config package"
	@echo "  make test-handlers      Test handlers package"
	@echo "  make test-middleware    Test middleware package"
	@echo "  make test-database      Test database package"
	@echo ""
	@echo "Code Quality:"
	@echo "  make fmt                Format code with gofmt"
	@echo "  make lint               Run golangci-lint"
	@echo "  make vet                Run go vet"
	@echo "  make check              Run all checks (fmt, vet, lint, test)"
	@echo ""
	@echo "Dependencies:"
	@echo "  make deps               Download dependencies"
	@echo "  make tidy               Tidy go.mod"
	@echo "  make update-deps        Update all dependencies"
	@echo ""
	@echo "Database:"
	@echo "  make migrate            Run database migrations"
	@echo "  make verify-db          Verify database connection"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean              Remove build artifacts and caches"
	@echo ""
	@echo "Environment Variables (required - set in .env or export):"
	@echo "  TEST_DATABASE_URL       Database URL for integration/e2e tests"
	@echo "  TEST_REDIS_ADDR         Redis address for integration/e2e tests"
	@echo "  TEST_SERVER_URL         Server URL for E2E tests"
	@echo "  DOCKER_REGISTRY         Docker registry for push (e.g., docker.io/username)"
