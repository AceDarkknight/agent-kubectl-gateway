# Makefile for agent-kubectl-gateway

# Go build variables
BINARY_NAME=agent-kubectl-gateway
GO=go
GOFLAGS=-v
BUILD_DIR=./build

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/agent-kubectl-gateway

# Run the application
.PHONY: run
run:
	$(GO) run ./cmd/agent-kubectl-gateway

# Run tests
.PHONY: test
test:
	$(GO) test -v ./...

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

# Tidy up dependencies
.PHONY: tidy
tidy:
	$(GO) mod tidy

# Install dependencies
.PHONY: deps
deps:
	$(GO) mod download

# Run linter (requires golangci-lint)
.PHONY: lint
lint:
	golangci-lint run ./...

# Build Docker image
.PHONY: docker-build
docker-build:
	docker build -t $(BINARY_NAME):latest -f deploy/Dockerfile .

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  run          - Run the application"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  tidy         - Tidy up dependencies"
	@echo "  deps         - Install dependencies"
	@echo "  lint         - Run linter"
	@echo "  docker-build - Build Docker image"
