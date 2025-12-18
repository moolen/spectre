.PHONY: help build build-ui build-mcp run test test-go test-ui test-e2e test-e2e-ui test-e2e-all clean docker-build docker-run deploy watch lint fmt vet favicons helm-lint helm-test helm-test-local helm-unittest helm-unittest-install proto

# Default target
help:
	@echo "Kubernetes Event Monitor - Available targets:"
	@echo ""
	@echo "Build:"
	@echo "  build          - Build the application binary"
	@echo "  build-ui       - Build the React UI"
	@echo "  build-mcp      - Build the MCP server for Claude integration"
	@echo "  proto          - Generate protobuf code"
	@echo ""
	@echo "Run:"
	@echo "  run            - Run the application locally"
	@echo ""
	@echo "Test:"
	@echo "  test           - Run all tests (Go + UI)"
	@echo "  test-go        - Run Go tests only"
	@echo "  test-ui        - Run UI tests only"
	@echo "  test-e2e       - Run e2e tests"
	@echo "  test-e2e-ui    - Run UI e2e tests"
	@echo "  test-e2e-all   - Run all e2e tests"
	@echo "  test-unit      - Run unit tests only"
	@echo "  test-integration - Run integration tests only"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo ""
	@echo "Code Quality:"
	@echo "  lint           - Run linter (golangci-lint if available)"
	@echo "  fmt            - Format code with gofmt"
	@echo "  vet            - Run go vet"
	@echo ""
	@echo "Docker & Deployment:"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run application in Docker"
	@echo "  deploy         - Deploy to Kubernetes via Helm"
	@echo ""
	@echo "Helm:"
	@echo "  helm-lint      - Lint Helm chart"
	@echo "  helm-unittest  - Run Helm unit tests"
	@echo "  helm-unittest-install - Install helm-unittest plugin"
	@echo "  helm-test      - Run Helm tests (requires active k8s cluster)"
	@echo "  helm-test-local - Create Kind cluster and run Helm tests locally"
	@echo ""
	@echo "Other:"
	@echo "  clean          - Clean build artifacts and temporary files"
	@echo "  watch          - Watch and rebuild on file changes (requires entr)"
	@echo "  favicons       - Generate all favicon versions from favicon.svg"

# Variables
BINARY_NAME=spectre
BINARY_PATH=bin/$(BINARY_NAME)
IMAGE_NAME=spectre
IMAGE_TAG=latest
DOCKER_IMAGE=$(IMAGE_NAME):$(IMAGE_TAG)
CHART_PATH=./chart
NAMESPACE=monitoring
DATA_DIR=./data

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	@go build -o $(BINARY_PATH) ./cmd/spectre
	@echo "Build complete: $(BINARY_PATH)"

# Build the React UI
build-ui:
	@echo "Building React UI..."
	@cd ui && npm ci && npm run build
	@echo "UI build complete: ui/dist"


# Run the application locally
run: build build-ui
	@echo "Running $(BINARY_NAME) server..."
	@mkdir -p $(DATA_DIR)
	@export KUBECONFIG=$(KUBECONFIG); \
	$(BINARY_PATH) server

# Run Go tests only
test-go:
	@echo "Running Go tests..."
	@go test -v -cover -count 1 -timeout 30m ./...

# Run UI tests only
test-ui:
	@echo "Running UI tests..."
	@cd ui && npm ci --prefer-offline --no-audit --no-fund 2>/dev/null && npm run test

# Run all tests (Go + UI)
test: test-go test-ui
	@echo "All tests completed successfully!"

# Run e2e tests
test-e2e:
	@echo "Running e2e tests..."
	@go test -v -timeout 60m ./tests/e2e/...

# Run UI e2e tests
test-e2e-ui:
	@echo "Running UI e2e tests..."
	@go test -v -timeout 60m ./tests/e2e/ui/...

# Run all e2e tests
test-e2e-all: test-e2e test-e2e-ui
	@echo "All e2e tests completed!"

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
docker-build: build build-ui
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
	@helm upgrade --install spectre $(CHART_PATH) \
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

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@protoc --go_out=. --go_opt=paths=source_relative internal/storage/index.proto
	@protoc --go_out=. --go_opt=module=github.com/moolen/spectre internal/models/event.proto
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/api/proto/timeline.proto
	@echo "Protobuf code generated successfully"

# Generate favicons from SVG source
favicons:
	@echo "Generating favicons..."
	@./hack/generate-favicons.sh
	@echo "Favicons generated successfully"

# Install helm-unittest plugin
helm-unittest-install:
	@echo "Installing helm-unittest plugin..."
	@helm plugin list | grep -q unittest || helm plugin install https://github.com/helm-unittest/helm-unittest.git --version v1.0.2
	@echo "helm-unittest plugin installed"

# Run helm unit tests
helm-unittest: helm-unittest-install
	@echo "Running Helm unit tests..."
	@helm unittest $(CHART_PATH) --color --output-type JUnit --output-file test-results.xml
	@echo "Helm unit tests complete!"

# Default target
.DEFAULT_GOAL := help
