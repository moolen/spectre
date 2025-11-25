.PHONY: help build run test clean docker-build docker-run deploy watch lint fmt vet

# Default target
help:
	@echo "Kubernetes Event Monitor - Available targets:"
	@echo "  build          - Build the application binary"
	@echo "  run            - Run the application locally"
	@echo "  test           - Run all tests"
	@echo "  test-unit      - Run unit tests only"
	@echo "  test-integration - Run integration tests only"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  clean          - Clean build artifacts and temporary files"
	@echo "  lint           - Run linter (golangci-lint if available)"
	@echo "  fmt            - Format code with gofmt"
	@echo "  vet            - Run go vet"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run application in Docker"
	@echo "  deploy         - Deploy to Kubernetes via Helm"
	@echo "  watch          - Watch and rebuild on file changes (requires entr)"

# Variables
BINARY_NAME=k8s-event-monitor
BINARY_PATH=bin/$(BINARY_NAME)
IMAGE_NAME=k8s-event-monitor
IMAGE_TAG=latest
DOCKER_IMAGE=$(IMAGE_NAME):$(IMAGE_TAG)
CHART_PATH=./chart
NAMESPACE=monitoring
DATA_DIR=./data

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	@go build -o $(BINARY_PATH) ./cmd/main.go
	@echo "Build complete: $(BINARY_PATH)"

# Run the application locally
run: build
	@echo "Running $(BINARY_NAME)..."
	@mkdir -p $(DATA_DIR)
	@export KUBECONFIG=$(KUBECONFIG); \
	$(BINARY_PATH)

# Run all tests
test:
	@echo "Running all tests..."
	@go test -v -cover ./...

# Run unit tests only
test-unit:
	@echo "Running unit tests..."
	@go test -v -cover ./tests/unit/...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	@go test -v -cover ./tests/integration/...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -rf dist/
	@rm -f coverage.out coverage.html
	@rm -f *.test
	@go clean
	@echo "Clean complete"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Format complete"

# Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...
	@echo "Vet complete"

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed, skipping" && exit 0)
	@golangci-lint run ./... || true
	@echo "Lint complete"

# Build Docker image
docker-build: build
	@echo "Building Docker image $(DOCKER_IMAGE)..."
	@docker build -t $(DOCKER_IMAGE) .
	@echo "Docker image built: $(DOCKER_IMAGE)"

# Run in Docker
docker-run: docker-build
	@echo "Running in Docker..."
	@docker run --rm -p 8080:8080 -v $(shell pwd)/data:/data $(DOCKER_IMAGE)

# Deploy to Kubernetes via Helm
deploy:
	@echo "Deploying to Kubernetes cluster..."
	@kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@helm upgrade --install k8s-event-monitor $(CHART_PATH) \
		--namespace $(NAMESPACE) \
		--create-namespace \
		--values $(CHART_PATH)/values.yaml
	@echo "Deployment complete. Check status:"
	@kubectl get pods -n $(NAMESPACE)

# Watch and rebuild on file changes (requires entr)
watch:
	@echo "Watching for changes (requires 'entr')..."
	@find . -name "*.go" | entr make build

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies updated"

# Verify dependencies
deps-verify:
	@echo "Verifying dependencies..."
	@go mod verify
	@echo "Dependencies verified"

# Default target
.DEFAULT_GOAL := help
