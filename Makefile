.PHONY: test test-v test-cover test-race test-short test-ndclient test-lanfabric test-all build build-grpc run run-grpc clean lint fmt vet proto

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOFMT=$(GOCMD) fmt
GOMOD=$(GOCMD) mod
BINARY_NAME=go-nd
GRPC_BINARY_NAME=go-nd-grpc

# Test parameters
TEST_FLAGS=-count=1
COVER_FLAGS=-coverprofile=coverage.out
RACE_FLAGS=-race

# Default target
all: test build

# Build the REST application
build:
	$(GOBUILD) -o $(BINARY_NAME) ./cmd/server

# Build the gRPC application
build-grpc: proto
	$(GOBUILD) -o $(GRPC_BINARY_NAME) ./cmd/grpc_server

# Run the REST application
run: build
	./$(BINARY_NAME)

# Run the gRPC application
run-grpc: build-grpc
	./$(GRPC_BINARY_NAME)

# Generate protobuf code (requires buf: https://buf.build/docs/installation)
proto:
	@which buf > /dev/null || (echo "buf not installed. Install: https://buf.build/docs/installation" && exit 1)
	buf generate

# Run all tests
test:
	$(GOTEST) $(TEST_FLAGS) ./...

# Run tests with verbose output
test-v:
	$(GOTEST) $(TEST_FLAGS) -v ./...

# Run tests with coverage
test-cover:
	$(GOTEST) $(TEST_FLAGS) $(COVER_FLAGS) ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run tests with race detector
test-race:
	$(GOTEST) $(TEST_FLAGS) $(RACE_FLAGS) ./...

# Run short tests only (skip long-running tests like retry tests)
test-short:
	$(GOTEST) $(TEST_FLAGS) -short ./...

# Run NDFC client tests only
test-ndclient:
	$(GOTEST) $(TEST_FLAGS) -v ./internal/ndclient/...

# Run LAN fabric tests only
test-lanfabric:
	$(GOTEST) $(TEST_FLAGS) -v ./internal/ndclient/lanfabric/...

# Run security client tests only
test-security:
	$(GOTEST) $(TEST_FLAGS) -v ./internal/ndclient/... -run "Security|Contract|Config|Wrap|Batch"

# Run all tests with verbose output and race detection
test-all: test-race test-cover

# Run integration test (requires NDFC connection)
test-integration:
	$(GOCMD) run ./cmd/integration_test/main.go

# Format code
fmt:
	$(GOFMT) ./...

# Run go vet
vet:
	$(GOVET) ./...

# Run linter (requires golangci-lint)
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed" && exit 1)
	golangci-lint run ./...

# Tidy modules
tidy:
	$(GOMOD) tidy

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME) $(GRPC_BINARY_NAME)
	rm -f coverage.out coverage.html
	rm -rf gen/

# Start dependencies (postgres, valkey)
deps-up:
	docker-compose up -d postgres valkey

# Stop dependencies
deps-down:
	docker-compose down

# Help
help:
	@echo "Available targets:"
	@echo "  make test          - Run all tests"
	@echo "  make test-v        - Run tests with verbose output"
	@echo "  make test-cover    - Run tests with coverage report"
	@echo "  make test-race     - Run tests with race detector"
	@echo "  make test-short    - Run short tests only (skip slow tests)"
	@echo "  make test-ndclient - Run NDFC client tests only"
	@echo "  make test-lanfabric- Run LAN fabric tests only"
	@echo "  make test-security - Run security client tests only"
	@echo "  make test-all      - Run all tests with race detection and coverage"
	@echo "  make test-integration - Run integration test (requires NDFC)"
	@echo "  make build         - Build the REST application"
	@echo "  make build-grpc    - Build the gRPC application"
	@echo "  make run           - Build and run the REST application"
	@echo "  make run-grpc      - Build and run the gRPC application"
	@echo "  make proto         - Generate protobuf code (requires buf)"
	@echo "  make fmt           - Format code"
	@echo "  make vet           - Run go vet"
	@echo "  make lint          - Run golangci-lint"
	@echo "  make tidy          - Tidy go modules"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make deps-up       - Start docker dependencies"
	@echo "  make deps-down     - Stop docker dependencies"
