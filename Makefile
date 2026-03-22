# Nori Makefile
# Build and development tasks

BINARY_NAME := nori
VERSION := "1.2.0"
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Go settings
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

.PHONY: all build clean test lint fmt install help

# Default target
all: build

# Build the binary
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/main.go

# Build for all platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/main.go
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/main.go

build-darwin:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/main.go
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/main.go

build-windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/main.go

# Install to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) ./cmd/main.go

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -cover ./...

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Lint the code
lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

# Format the code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	gofmt -s -w .

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Run the development version
run:
	go run ./cmd/main.go $(ARGS)

# Show help
help:
	@echo "Nori Build System"
	@echo ""
	@echo "Targets:"
	@echo "  build        Build the binary for the current platform"
	@echo "  build-all    Build for all supported platforms"
	@echo "  install      Install to GOPATH/bin"
	@echo "  test         Run tests"
	@echo "  test-coverage Run tests with coverage report"
	@echo "  lint         Run linters"
	@echo "  fmt          Format code"
	@echo "  clean        Clean build artifacts"
	@echo "  deps         Download and tidy dependencies"
	@echo "  run          Run the development version (use ARGS=...)"
	@echo "  help         Show this help"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION      $(VERSION)"
	@echo "  COMMIT       $(COMMIT)"
	@echo "  GOOS         $(GOOS)"
	@echo "  GOARCH       $(GOARCH)"

