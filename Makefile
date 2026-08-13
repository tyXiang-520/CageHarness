.PHONY: build test clean run vet tidy all

BINARY_NAME := harness
BUILD_DIR := build
CMD_DIR := ./cmd/harness

all: tidy vet test build

build:
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

test:
	go test ./... -v -count=1

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