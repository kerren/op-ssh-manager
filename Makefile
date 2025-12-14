.PHONY: build test lint clean install release-snapshot help

# Variables
BINARY_NAME := op-ssh-manager
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w \
	-X github.com/kerren/op-ssh-manager/internal/cmd.Version=$(VERSION) \
	-X github.com/kerren/op-ssh-manager/internal/cmd.Commit=$(COMMIT) \
	-X github.com/kerren/op-ssh-manager/internal/cmd.BuildDate=$(BUILD_DATE)

# Default target
all: build

## build: Build the binary
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/op-ssh-manager

## build-all: Build for all platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-amd64 ./cmd/op-ssh-manager
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-arm64 ./cmd/op-ssh-manager
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/op-ssh-manager
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/op-ssh-manager
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/op-ssh-manager

## test: Run tests
test:
	go test -v -race -coverprofile=coverage.out ./...

## test-short: Run short tests only
test-short:
	go test -v -short ./...

## coverage: Show test coverage
coverage: test
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## lint: Run linter
lint:
	golangci-lint run ./...

## lint-fix: Run linter with auto-fix
lint-fix:
	golangci-lint run --fix ./...

## fmt: Format code
fmt:
	go fmt ./...
	goimports -w -local github.com/kerren/op-ssh-manager .

## vet: Run go vet
vet:
	go vet ./...

## clean: Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf dist/
	rm -f coverage.out coverage.html

## install: Install the binary
install: build
	go install -ldflags "$(LDFLAGS)" ./cmd/op-ssh-manager

## release-snapshot: Create a snapshot release (for testing)
release-snapshot:
	goreleaser release --snapshot --clean

## deps: Download dependencies
deps:
	go mod download
	go mod tidy

## check: Run all checks (lint, vet, test)
check: lint vet test

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':'
