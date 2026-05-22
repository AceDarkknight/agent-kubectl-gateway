# Makefile for agent-kubectl-gateway

# Go build variables
BINARY_NAME=agent-kubectl-gateway
GO=go
GOFLAGS=-v
LDFLAGS=-s -w
BUILD_DIR=./bin

# 交叉编译参数，可通过 make build GOOS=darwin GOARCH=arm64 覆盖
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -trimpath -a -installsuffix cgo -o $(BUILD_DIR)/$(BINARY_NAME)-$(GOOS)-$(GOARCH) ./cmd/agent-kubectl-gateway

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
	@echo "  build        - Build the binary (default: current OS/ARCH)"
	@echo "                 override: make build GOOS=linux GOARCH=arm64"
	@echo "  run          - Run the application"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  tidy         - Tidy up dependencies"
	@echo "  deps         - Install dependencies"
	@echo "  lint         - Run linter"
	@echo "  docker-build - Build Docker image"
