.PHONY: help build build-ui build-mcp build-docs run test test-go test-ui test-e2e test-e2e-root-cause test-e2e-ui test-e2e-all clean clean-test-clusters docker-build docker-run deploy watch lint fmt vet favicons helm-lint helm-test helm-test-local helm-unittest helm-unittest-install proto dev-iterate dev-stop dev-logs graph-up graph-down test-graph test-graph-integration test-integration test-graph-integration-coverage test-graph-integration-single golden-generator test-golden docs-dev docs-preview

# Default target
help:
	@echo "Kubernetes Event Monitor - Available targets:"
	@echo ""
	@echo "Build:"
	@echo "  build          - Build the application binary"
	@echo "  build-ui       - Build the React UI"
	@echo "  build-mcp      - Build the MCP server for Claude integration"
	@echo "  build-docs     - Build the documentation site"
	@echo "  proto          - Generate protobuf code"
	@echo ""
	@echo "Run:"
	@echo "  run            - Run the application locally"
	@echo ""
	@echo "Test:"
	@echo "  test           - Run all tests (Go + UI)"
	@echo "  test-go        - Run Go tests only"
	@echo "  test-ui        - Run UI tests only"
	@echo "  golden-generator - Build the golden fixture generator"
	@echo "  generate-golden-fixtures - Generate golden test fixtures from Kind cluster"
	@echo "  test-golden    - Run golden fixture tests"
	@echo "  clean-test-clusters - Delete persistent test Kind clusters"
	@echo "  clean-golden-cluster - Delete golden test Kind cluster"
	@echo ""
	@echo "Graph Layer:"
	@echo "  graph-up       - Start FalkorDB for development"
	@echo "  graph-down     - Stop FalkorDB"
	@echo ""
	@echo "Development:"
	@echo "  dev-iterate    - Quick iteration: clean, build, restart all services locally"
	@echo "  dev-stop       - Stop all development services"
	@echo "  dev-logs       - Tail all development logs"
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
	@echo "Documentation:"
	@echo "  build-docs     - Build the documentation site for production"
	@echo "  docs-dev       - Run documentation dev server locally"
	@echo "  docs-preview   - Preview production build locally"
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
DATA_LOCAL_DIR=./data-local

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
	script -q -e -c "go test -v -cover -count 1 -timeout 60m ./..." /dev/null

# Run UI tests only (unit tests + component tests)
test-ui:
	@echo "Running UI unit tests..."
	@cd ui && npm ci --prefer-offline --no-audit --no-fund 2>/dev/null && npm run test
	@echo "Running UI component tests..."
	@cd ui && npx playwright install chromium --with-deps && npm run test:ct

# Run all tests (Go + UI)
test: test-go test-ui
	@echo "All tests completed successfully!"

# Clean up test Kind clusters
clean-test-clusters:
	@echo "Cleaning up test Kind clusters..."
	@kind delete cluster --name spectre-e2e-shared 2>/dev/null || true
	@kind delete cluster --name spectre-ui-e2e-shared 2>/dev/null || true
	@echo "✓ Test clusters cleaned up"

# Build golden fixture generator
golden-generator:
	@echo "Building golden fixture generator..."
	@go build ./tests/integration/golden/cmd/golden-generator/
	@echo "✓ Golden fixture generator built (./golden-generator)"

# Generate golden test fixtures
generate-golden-fixtures:
	@echo "Generating golden test fixtures..."
	@./golden-generator --cluster spectre-golden --output tests/integration/fixtures/golden
	@echo "✓ Golden fixtures generated"

# Run golden tests
test-golden:
	@echo "Running golden tests..."
	@go test -v -tags integration -timeout 30m ./tests/integration/api -run TestGoldenScenarios
	@echo "✓ Golden tests completed"

# Clean up golden cluster
clean-golden-cluster:
	@echo "Cleaning up golden cluster..."
	@kind delete cluster --name spectre-golden 2>/dev/null || true
	@echo "✓ Golden cluster cleaned up"

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
	@protoc --go_out=. --go_opt=module=github.com/moolen/spectre internal/models/event.proto
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		--connect-go_out=. --connect-go_opt=paths=source_relative \
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

# Stop development services
dev-stop:
	@echo "==> Stopping all development services..."
	@-pkill -f "$(BINARY_PATH) server" || true
	@-pkill -f "$(BINARY_PATH) mcp" || true
	@docker compose -f docker-compose.graph.yml down || true
	@echo "All services stopped"

# ============================================================================
# Graph Layer Targets
# ============================================================================

# Start FalkorDB for development
graph-up:
	@echo "Starting FalkorDB..."
	@docker compose -f docker-compose.graph.yml up -d falkordb
	@echo "Waiting for FalkorDB to be ready..."
	@sleep 3
	@echo "FalkorDB is running on localhost:6379"

# Stop FalkorDB
graph-down:
	@echo "Stopping FalkorDB..."
	@docker compose -f docker-compose.graph.yml down falkordb
	@echo "FalkorDB stopped"


# ============================================================================

# Tail development logs
dev-logs:
	@echo "==> Tailing development logs (Ctrl+C to exit)..."
	@tail -f $(DATA_LOCAL_DIR)/logs/*.log

# Quick iteration for MCP/Spectre/FalkorDB development
dev-iterate: build
	@echo "==> Stopping all services..."
	-pkill -f "$(BINARY_PATH) server" || true
	-pkill -f "$(BINARY_PATH) mcp" || true
	docker compose -f docker-compose.graph.yml down falkordb || true
	sleep 2
	@echo ""
	@echo "==> Cleaning local state..."
	@echo ""
	@echo "==> Building spectre binary..."
	mkdir -p bin
	go build -o $(BINARY_PATH) ./cmd/spectre
	@echo ""
	@echo "==> Starting FalkorDB..."
	docker compose -f docker-compose.graph.yml up -d falkordb
	@echo "Waiting for FalkorDB to be ready..."
	sleep 3
	@echo ""
	@echo "==> Starting Spectre server..."
	mkdir -p $(DATA_LOCAL_DIR)/logs
	KUBECONFIG=$(KUBECONFIG) \
		$(BINARY_PATH) server \
		--log-level=debug \
		--graph-enabled=true \
		--graph-host=localhost \
		--graph-rebuild-on-start=false \
		--graph-port=6379 \
		--watcher-config=hack/watcher.yaml \
		> $(DATA_LOCAL_DIR)/logs/spectre.log 2>&1 &
	@echo "Spectre server PID: $$!"
	@echo "Waiting for Spectre server to be ready..."
	@timeout=60; \
	while [ $$timeout -gt 0 ]; do \
		if nc -z localhost 8080 2>/dev/null; then \
			if curl -sf http://localhost:8080/ready >/dev/null 2>&1; then \
				echo "Spectre server is ready!"; \
				break; \
			fi; \
		fi; \
		sleep 1; \
		timeout=$$((timeout - 1)); \
	done; \
	if [ $$timeout -eq 0 ]; then \
		echo "ERROR: Spectre server did not become ready within 60 seconds"; \
		exit 1; \
	fi
	@echo ""
	@echo "==> Starting MCP server..."
	$(BINARY_PATH) mcp \
		--spectre-url=http://localhost:8080 \
		--graph-enabled=true \
		--log-level=debug \
		--graph-host=localhost \
		--graph-port=6379 \
		--http-addr=:8082 \
		> $(DATA_LOCAL_DIR)/logs/mcp.log 2>&1 &
	@echo "MCP server PID: $$!"
	@echo ""
	@echo "==> All services started!"
	@echo ""
	@echo "Service URLs:"
	@echo "  - Spectre UI:     http://localhost:8080"
	@echo "  - Spectre API:    http://localhost:8080/api"
	@echo "  - MCP Server:     http://localhost:8082"
	@echo "  - FalkorDB:       localhost:6379"
	@echo ""
	@echo "Logs:"
	@echo "  - Spectre:        $(DATA_LOCAL_DIR)/logs/spectre.log"
	@echo "  - MCP:            $(DATA_LOCAL_DIR)/logs/mcp.log"
	@echo "  - FalkorDB:       docker compose -f docker-compose.graph.yml logs -f"
	@echo ""
	@echo "To stop services:"
	@echo "  pkill -f '$(BINARY_PATH)' && docker compose -f docker-compose.graph.yml down"
	@echo ""
	@echo "To view logs:"
	@echo "  tail -f $(DATA_LOCAL_DIR)/logs/spectre.log"
	@echo "  tail -f $(DATA_LOCAL_DIR)/logs/mcp.log"

.PHONY: dev-clean
dev-clean:
	@echo "==> Cleaning local state..."
	rm -rf $(DATA_LOCAL_DIR)
	mkdir -p $(DATA_LOCAL_DIR)

# ============================================================================
# Documentation Targets
# ============================================================================

# Build documentation site for production
build-docs:
	@echo "Building documentation site..."
	@cd docs && npm ci && npm run build
	@echo "Documentation build complete: docs/dist"

# Run documentation dev server
docs-dev:
	@echo "Starting documentation dev server..."
	@cd docs && npm ci && npm run dev

# Preview production documentation build
docs-preview: build-docs
	@echo "Starting documentation preview server..."
	@cd docs && npm run preview

# Default target
.DEFAULT_GOAL := help
