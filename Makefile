# Variables
BINARY_NAME=bucketsyncd
BUILD_DIR=build
LDFLAGS=-ldflags="-s -w"

# Default target
.PHONY: all
all: test lint build

# Build targets
.PHONY: build build-all
build: linux-amd64

build-all: linux-amd64 linux-arm64 macos-amd64 macos-arm64 windows-amd64

# Platform-specific builds
.PHONY: linux-amd64 linux-arm64 macos-amd64 macos-arm64 windows-amd64
linux-amd64:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64

linux-arm64:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64

macos-amd64:
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64

macos-arm64:
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64

windows-amd64:
	mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe

# Legacy targets for backward compatibility
.PHONY: linux macos windows
linux: linux-amd64
macos: macos-amd64
windows: windows-amd64

# Development targets
.PHONY: dev clean install-tools
dev:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) .

clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

install-tools:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.4.0
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	go install golang.org/x/tools/cmd/goimports@latest

# Testing targets
.PHONY: test test-coverage test-race
test:
	go test -v ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out

test-race:
	go test -race -v ./...

# Code quality targets
.PHONY: fmt lint vet security
fmt:
	go fmt ./...
	goimports -w .

lint:
	golangci-lint run

vet:
	go vet ./...

security:
	gosec ./...

# Dependency management
.PHONY: deps deps-update deps-verify
deps:
	go mod download

deps-update:
	go mod tidy
	go get -u ./...

deps-verify:
	go mod verify

# Docker targets
.PHONY: docker docker-build docker-run
docker: docker-build

docker-build:
	docker build -t $(BINARY_NAME) .

docker-run:
	docker run --rm $(BINARY_NAME)

# Release preparation
.PHONY: release-prep
release-prep: clean test lint security build-all
	cd $(BUILD_DIR) && sha256sum * > checksums.txt

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all            - Run tests, linting, and build"
	@echo "  build          - Build for current platform"
	@echo "  build-all      - Build for all supported platforms"
	@echo "  linux-amd64    - Build for Linux AMD64"
	@echo "  linux-arm64    - Build for Linux ARM64"
	@echo "  macos-amd64    - Build for macOS AMD64"
	@echo "  macos-arm64    - Build for macOS ARM64"
	@echo "  windows-amd64  - Build for Windows AMD64"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  test-race      - Run tests with race detection"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  security       - Run security scan"
	@echo "  clean          - Clean build artifacts"
	@echo "  docker-build   - Build Docker image"
	@echo "  install-tools  - Install development tools"
	@echo "  release-prep   - Prepare release artifacts"

