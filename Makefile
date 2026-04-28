GO_VERSION := $(shell go version | awk '{print $$3}')
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BINARY_NAME := somcli
VERSION := $(shell git describe --tags --always --dirty)
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

.PHONY: all build clean test install uninstall fmt vet lint run help

all: build

build:
	@echo "Building $(BINARY_NAME) version $(VERSION) for $(GOOS)/$(GOARCH)..."
	go build -o bin/$(BINARY_NAME) -ldflags "-X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE)" main.go

build-all:
	@echo "Building for all platforms..."
	mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY_NAME)-linux-amd64 -ldflags "-X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE)" main.go
	GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY_NAME)-darwin-amd64 -ldflags "-X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE)" main.go
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY_NAME)-windows-amd64.exe -ldflags "-X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE)" main.go
	GOOS=linux GOARCH=arm64 go build -o bin/$(BINARY_NAME)-linux-arm64 -ldflags "-X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE)" main.go

clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	go clean

test:
	@echo "Running tests..."
	go test ./... -v -count=1

test-coverage:
	@echo "Running tests with coverage..."
	go test ./... -v -count=1 -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html

install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp bin/$(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)

uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)

fmt:
	@echo "Formatting code..."
	go fmt ./...

vet:
	@echo "Running go vet..."
	go vet ./...

lint:
	@echo "Running lint..."
	@golangci-lint run ./...

run: build
	@echo "Running $(BINARY_NAME)..."
	./bin/$(BINARY_NAME) --help

version: build
	@echo "Version: $(VERSION)"
	./bin/$(BINARY_NAME) version

help:
	@echo "Available targets:"
	@echo "  all          - Build the project (default)"
	@echo "  build        - Build the binary for current platform"
	@echo "  build-all    - Build binaries for all platforms"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run all tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  install      - Build and install to /usr/local/bin"
	@echo "  uninstall    - Remove the installed binary"
	@echo "  fmt          - Format code with go fmt"
	@echo "  vet          - Run go vet"
	@echo "  lint         - Run golangci-lint"
	@echo "  run          - Build and run the binary"
	@echo "  version      - Show version information"
	@echo "  help         - Show this help message"

info:
	@echo "Go Version: $(GO_VERSION)"
	@echo "GOOS: $(GOOS)"
	@echo "GOARCH: $(GOARCH)"
	@echo "Version: $(VERSION)"
	@echo "Build Date: $(BUILD_DATE)"