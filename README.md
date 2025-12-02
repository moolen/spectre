# Spectre

<div align="center">
  <img src="ui/public/ghost.svg" alt="Spectre" width="200">
</div>

> **See everything that happens in your Kubernetes cluster.** Spectre captures, stores, and visualizes all resource changes in real-time with an intuitive audit timeline.

## What is Spectre?

Spectre is a Kubernetes event monitoring and auditing system. It captures all resource changes (create, update, delete) across your cluster and provides a powerful visualization dashboard to understand what happened, when it happened, and why.

### Why Spectre?

In Kubernetes environments, resources are constantly changing. Without proper visibility, it's difficult to:
- **Track resource changes** - What changed and When?
- **Debug issues** - Understand the sequence of events that led to a problem
- **Troubleshoot failures** - Helps with Incident Response or post mortem analysis

Spectre solves this by providing:

1. **Real-time Event Capture** - Every resource change is captured instantly using watches
2. **Efficient Storage** - Events are compressed and indexed for fast retrieval
3. **Interactive Audit Timeline** - Visualize resource state changes over time
4. **Flexible Filtering** - Find exactly what you're looking for by namespace, kind, or name
5. **Historical Analysis** - Query any time period to understand what happened

## Quick Start

### Prerequisites

- Kubernetes 1.20+ cluster
- `helm` 3.x
- `kubectl` configured to access your cluster

### Using Helm (Recommended)

```bash
# Add the Spectre Helm repository
helm repo add spectre oci://ghcr.io/moolen/charts
helm repo update

# Install Spectre
helm install spectre spectre/k8s-event-monitor \
  --namespace monitoring \
  --create-namespace

# Access the UI
kubectl port-forward -n monitoring svc/k8s-event-monitor 8080:8080

# Open your browser to http://localhost:8080
```

### Manual Installation

```bash
# Clone the repository
git clone https://github.com/moolen/spectre.git
cd spectre

# Install using local Helm chart
helm install spectre ./chart \
  --namespace monitoring \
  --create-namespace

# Port forward to access UI
kubectl port-forward -n monitoring svc/k8s-event-monitor 8080:8080
```

## Development

### Prerequisites

- Go 1.24.1+
- Node.js 22+ (for UI)
- Make

### Building Locally

```bash
# Build the Go binary
make build

# Build the React UI
make build-ui

# Run the application
make run
```

### Testing

```bash
# Run all tests (unit + integration)
make test

# Run specific test suites
make test-unit
make test-integration

# Generate coverage report
make test-coverage
```

### Running with Docker

```bash
# Build Docker image
make docker-build

# Run in Docker
make docker-run
```

### Demo Mode (no cluster required)

Spectre ships with a curated set of Kubernetes events (`internal/demo/`) that can be served without connecting to a live cluster. Demo mode only starts the API/UI stack, so you can explore the product UI immediately.

**Run directly with Go**
```bash
# Start demo mode without building binaries
go run ./cmd/main.go --demo --api-port 8080

# Or build once and re-run quickly
go build -o bin/spectre ./cmd/main.go
bin/spectre --demo --api-port 9000 --log-level debug
```

**Run with Docker**
```bash
# Build the image (only needed once)
docker build -t spectre-demo .

# Launch the container in demo mode
docker run --rm -p 8080:8080 spectre-demo --demo
```

Then open `http://localhost:8080` (or the port you passed via `--api-port`) in your browser. You will find:
- Multiple resource types (Deployments, Pods, StatefulSets, Services, ConfigMaps, Nodes, HelmReleases)
- Real-world failure scenarios (ImagePullBackOff, disk pressure, Helm rollback, crash loops)
- Successful deployment/scaling events across seven days of simulated history
- Full filtering/search support identical to a live cluster

## Architecture

Spectre consists of three main components:

### 1. Event Watcher (`internal/watcher/`)
Monitors Kubernetes resources in real-time and captures all state changes through the Kubernetes watch API.

### 2. Block-based Storage (`internal/storage/`)
Efficiently stores compressed events organized by time blocks with sparse indexes for fast filtering:
- Events are grouped into hourly segments
- Each segment contains metadata indexes (namespaces, kinds, API groups)
- Compression reduces storage footprint by 60-80%

### 3. Query API & Timeline UI (`internal/api/`, `ui/`)
Provides REST API for querying events and a React-based interactive timeline visualization:
- Filter by time range, namespace, kind, name
- Real-time resource status tracking
- Interactive timeline with detailed event inspection

## API Documentation

### Query Events

```bash
curl "http://localhost:8080/api/events?namespace=default&kind=Pod&start=2025-01-01T00:00:00Z&end=2025-01-02T00:00:00Z"
```

**Query Parameters:**
- `start` - Start time (RFC3339 format)
- `end` - End time (RFC3339 format)
- `namespace` - Filter by namespace (optional)
- `kind` - Filter by resource kind (optional)
- `name` - Filter by resource name (optional)

### Search Resources

```bash
curl "http://localhost:8080/api/search?q=deployment-name&namespace=default"
```

## Configuration

Spectre is configured via command-line flags:

```
--demo                  Run in demo mode with preset data (default: false)
--data-dir              Directory where events are stored (default: /data)
--api-port              Port the API server listens on (default: 8080)
--log-level             Logging level: debug, info, warn, error (default: info)
--watcher-config        Path to YAML file containing watcher configuration
--segment-size          Target size for compression segments in bytes (default: 10MB)
--max-concurrent-requests Maximum number of concurrent API requests (default: 100)
```

### Watcher Configuration

Create a `watcher.yaml` file to specify which resources to monitor:

```yaml
resources:
  - group: ""
    version: "v1"
    kind: "Pod"
    namespace: ""
  - group: "apps"
    version: "v1"
    kind: "Deployment"
    namespace: ""
```

## Deployment

### Kubernetes Deployment

See [`chart/`](./chart/) for the Helm chart. Key features:

- RBAC configuration for secure cluster access
- Service and Ingress for external access
- Persistent volume for event storage
- Resource limits and requests
- Health checks and readiness probes

### Environment Variables

Inside the container, Spectre respects standard Kubernetes environment patterns:
- `KUBECONFIG` - Path to kubeconfig file (uses in-cluster auth by default)
- All configuration via command-line flags (see Configuration section)

### Testing

- Add unit tests alongside code changes
- Integration tests verify component interactions
- E2E tests validate full workflows

```bash
make test          # Run all tests
make test-coverage # Generate coverage report
```
