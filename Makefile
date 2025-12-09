.PHONY: help build build-ui build-mcp run test clean docker-build docker-run deploy watch lint fmt vet favicons helm-lint helm-test helm-test-local helm-unittest helm-unittest-install

# Default target
help:
	@echo "Kubernetes Event Monitor - Available targets:"
	@echo "  build          - Build the application binary"
	@echo "  build-ui       - Build the React UI"
	@echo "  build-mcp      - Build the MCP server for Claude integration"
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
	@echo "  helm-lint      - Lint Helm chart"
	@echo "  helm-unittest  - Run Helm unit tests"
	@echo "  helm-unittest-install - Install helm-unittest plugin"
	@echo "  helm-test      - Run Helm tests (requires active k8s cluster)"
	@echo "  helm-test-local - Create Kind cluster and run Helm tests locally"
	@echo "  watch          - Watch and rebuild on file changes (requires entr)"
	@echo "  favicons       - Generate all favicon versions from favicon.svg"

# Variables
BINARY_NAME=spectre
BINARY_PATH=bin/$(BINARY_NAME)
MCP_BINARY_NAME=spectre-mcp
MCP_BINARY_PATH=bin/$(MCP_BINARY_NAME)
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
	@go build -o $(BINARY_PATH) ./cmd/main.go
	@echo "Build complete: $(BINARY_PATH)"

# Build the React UI
build-ui:
	@echo "Building React UI..."
	@cd ui && npm ci && npm run build
	@echo "UI build complete: ui/dist"

# Build the MCP server
build-mcp:
	@echo "Building $(MCP_BINARY_NAME)..."
	@mkdir -p bin
	@go build -o $(MCP_BINARY_PATH) ./cmd/mcp-server
	@echo "Build complete: $(MCP_BINARY_PATH)"
	@echo "Start with: ./$(MCP_BINARY_PATH) --spectre-url http://localhost:8080"

# Run the application locally
run: build build-ui
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

.PHONY: benchmark
benchmark:
	@echo "Generating data..."
	rm -rf ./benchmark-data
	go run ./cmd/gendata --output-dir ./benchmark-data \
	--event-count 1000000 --duration-hours 24 \
	--segment-size 1048576 --distribution skewed

	go run ./cmd/main.go \
		--data-dir ./benchmark-data \
		--api-port 8080 \
		--log-level debug \
		--watcher-config ./hack/watcher.yaml \
		--segment-size 1048576 \
		--max-concurrent-requests 100

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

# Generate favicons from SVG source
favicons:
	@echo "Generating favicons..."
	@./hack/generate-favicons.sh
	@echo "Favicons generated successfully"

# Helm chart testing
helm-lint:
	@echo "Linting Helm chart..."
	@helm lint $(CHART_PATH)
	@echo "Helm lint complete"

helm-template:
	@echo "Templating Helm chart..."
	@helm template test-release $(CHART_PATH) --debug

# Install helm-unittest plugin
helm-unittest-install:
	@echo "Installing helm-unittest plugin..."
	@helm plugin list | grep -q unittest || helm plugin install https://github.com/helm-unittest/helm-unittest.git
	@echo "helm-unittest plugin installed"

# Run helm unit tests
helm-unittest: helm-unittest-install
	@echo "Running Helm unit tests..."
	@helm unittest $(CHART_PATH) --color --output-type JUnit --output-file test-results.xml
	@echo "Helm unit tests complete!"

helm-test: docker-build
	@echo "Running Helm tests (requires active Kubernetes cluster)..."
	@echo "Loading Docker image to cluster..."
	@kind load docker-image $(DOCKER_IMAGE) --name $(shell kubectl config current-context | sed 's/kind-//') 2>/dev/null || \
		echo "Note: kind load failed, assuming image is already available in cluster"
	@echo "Installing chart..."
	@helm upgrade --install spectre-test $(CHART_PATH) \
		--namespace $(NAMESPACE) \
		--create-namespace \
		--set image.repository=$(IMAGE_NAME) \
		--set image.tag=$(IMAGE_TAG) \
		--set image.pullPolicy=IfNotPresent \
		--wait \
		--timeout=5m
	@echo "Waiting for deployment..."
	@kubectl wait --for=condition=available --timeout=300s \
		deployment/spectre-test-spectre -n $(NAMESPACE) || \
		(kubectl logs -n $(NAMESPACE) -l app.kubernetes.io/name=spectre --tail=100 && exit 1)
	@echo "Running Helm tests..."
	@helm test spectre-test --namespace $(NAMESPACE) --logs
	@echo "Helm tests complete!"

helm-test-local: docker-build
	@echo "Creating local Kind cluster for Helm testing..."
	@kind create cluster --name helm-test --wait 300s || \
		(echo "Cluster 'helm-test' already exists, using existing cluster" && exit 0)
	@echo "Loading Docker image to Kind cluster..."
	@kind load docker-image $(DOCKER_IMAGE) --name helm-test
	@echo "Installing chart to Kind cluster..."
	@helm upgrade --install spectre-test $(CHART_PATH) \
		--namespace $(NAMESPACE) \
		--create-namespace \
		--set image.repository=$(IMAGE_NAME) \
		--set image.tag=$(IMAGE_TAG) \
		--set image.pullPolicy=IfNotPresent \
		--set persistence.size=1Gi \
		--wait \
		--timeout=5m
	@echo "Waiting for deployment to be ready..."
	@kubectl wait --for=condition=available --timeout=300s \
		deployment/spectre-test-spectre -n $(NAMESPACE) || \
		(kubectl logs -n $(NAMESPACE) -l app.kubernetes.io/name=spectre --tail=100 && exit 1)
	@echo "Running Helm tests..."
	@helm test spectre-test --namespace $(NAMESPACE) --logs
	@echo ""
	@echo "Helm tests complete!"
	@echo ""
	@echo "To cleanup, run: kind delete cluster --name helm-test"
	@echo "To keep cluster and cleanup helm release: helm uninstall spectre-test -n $(NAMESPACE)"

helm-clean:
	@echo "Cleaning up Helm test resources..."
	@helm uninstall spectre-test --namespace $(NAMESPACE) 2>/dev/null || true
	@kubectl delete namespace $(NAMESPACE) 2>/dev/null || true
	@kind delete cluster --name helm-test 2>/dev/null || true
	@echo "Cleanup complete"

# Default target
.DEFAULT_GOAL := help
