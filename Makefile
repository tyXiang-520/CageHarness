.PHONY: build test clean run vet tidy all build-all build-scf

BINARY_NAME := harness
BUILD_DIR := build
CMD_DIR := ./cmd/harness

# Default target
all: tidy vet test build

# Single-platform build (current OS)
build:
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

# Cross-platform build — all 5 targets
build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/harness-linux-amd64 $(CMD_DIR)

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/harness-linux-arm64 $(CMD_DIR)

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/harness-darwin-amd64 $(CMD_DIR)

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/harness-darwin-arm64 $(CMD_DIR)

build-windows-amd64:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/harness-windows-amd64.exe $(CMD_DIR)

# SCF deployment build — Linux amd64, statically linked
build-scf:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/harness-scf $(CMD_DIR)

test:
	go test ./internal/... -v -count=1

test-demo:
	go test ./tests/demo/ -v -count=1

vet:
	go vet ./...

clean:
	rm -rf $(BUILD_DIR)/

run:
	go run $(CMD_DIR)

tidy:
	go mod tidy

# Alias for course requirement
unit-test: test