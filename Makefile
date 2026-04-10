.PHONY: build test run lint docker-build docker-compose-up helm-lint clean

# Variables
APP_NAME := opensearch-file-api
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GO_FLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Default target
all: lint test build

# Build
build:
	go build $(GO_FLAGS) -o bin/$(APP_NAME) ./cmd/server

# Run
run:
	go run ./cmd/server

# Test
test:
	go test -v -race -coverprofile=coverage.out ./...

# Test with coverage
test-coverage: test
	go tool cover -html=coverage.out -o coverage.html

# Integration tests (requires Docker)
test-integration:
	go test -v -tags=integration ./...

# Lint
lint:
	golangci-lint run

# Lint fixes
lint-fix:
	golangci-lint run --fix

# Generate mocks
generate:
	go generate ./...

# Docker build
docker-build:
	docker build -t $(APP_NAME):$(VERSION) -f deployments/docker/Dockerfile .

# Docker Compose
docker-compose-up:
	docker-compose -f deployments/docker/docker-compose.yml up -d

docker-compose-down:
	docker-compose -f deployments/docker/docker-compose.yml down

# Helm
helm-lint:
	helm lint deployments/helm/$(APP_NAME)

helm-template:
	helm template test deployments/helm/$(APP_NAME) --values deployments/helm/$(APP_NAME)/values.yaml

# Install dependencies
deps:
	go mod download
	go mod tidy

# Clean
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Help
help:
	@echo "Available targets:"
	@echo "  build            - Build the application"
	@echo "  run              - Run the application"
	@echo "  test             - Run tests with coverage"
	@echo "  test-coverage    - Generate coverage report"
	@echo "  test-integration - Run integration tests"
	@echo "  lint             - Run linter"
	@echo "  lint-fix         - Run linter with auto-fix"
	@echo "  docker-build     - Build Docker image"
	@echo "  docker-compose-up - Start Docker Compose"
	@echo "  docker-compose-down - Stop Docker Compose"
	@echo "  helm-lint        - Lint Helm chart"
	@echo "  clean            - Clean build artifacts"
