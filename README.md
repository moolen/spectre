# Kubernetes Event Monitor

A high-performance, disk-based event monitoring system for Kubernetes clusters. Captures CREATE/UPDATE/DELETE resource events, stores them efficiently with compression and indexing, and provides a query API for historical analysis.

## Features

- **Real-time Event Capture**: Monitors all Kubernetes resource changes using informers
- **Efficient Storage**: Hourly files with segment-based compression (≥30% compression ratio)
- **Multi-dimensional Filtering**: Query by namespace, resource kind, API group/version
- **Fast Queries**: <2 second response time for 24-hour time windows using sparse indexes
- **High Throughput**: Handles 1000+ events/minute sustained ingestion
- **Kubernetes Native**: Helm chart for easy deployment, RBAC support

## Quick Start

### Prerequisites

- Go 1.21+
- Docker (for containerized deployment)
- kubectl (for Kubernetes interaction)
- A running Kubernetes cluster (minikube, kind, or remote)
- Make

### Local Development

```bash
# Clone the repository
cd /path/to/rpk

# Build the application
make build

# Run locally
make run

# Run tests
make test

# Clean artifacts
make clean
```

### Query the API

```bash
# Query all events in a time window
curl "http://localhost:8080/v1/search?start=1700000000&end=1700086400"

# Query Deployments in default namespace
curl "http://localhost:8080/v1/search?start=1700000000&end=1700086400&kind=Deployment&namespace=default"

# Query all Nodes cluster-wide
curl "http://localhost:8080/v1/search?start=1700000000&end=1700086400&kind=Node"

# Pretty-print with jq
curl -s "http://localhost:8080/v1/search?start=1700000000&end=1700086400&kind=Pod" | jq .
```

### Kubernetes Deployment

```bash
# Create monitoring namespace and deploy via Helm
make deploy

# Verify deployment
kubectl get pods -n monitoring

# Port-forward to access API
kubectl port-forward -n monitoring svc/k8s-event-monitor 8080:8080

# Query from within cluster
curl "http://k8s-event-monitor:8080/v1/search?start=<timestamp>&end=<timestamp>"
```

## Project Structure

```
rpk/
├── cmd/
│   └── main.go                  # Application entry point
├── internal/
│   ├── watcher/                 # Kubernetes watcher implementation
│   │   ├── watcher.go
│   │   ├── event_handler.go
│   │   ├── event_queue.go
│   │   ├── pruner.go
│   │   └── validator.go
│   ├── storage/                 # Disk storage engine
│   │   ├── storage.go           # File management
│   │   ├── segment.go           # Segment writing and compression
│   │   ├── compression.go       # Gzip compression
│   │   ├── index.go             # Sparse timestamp indexing
│   │   ├── segment_metadata.go  # Resource metadata indexing
│   │   ├── file_metadata.go     # File-level metadata
│   │   ├── query.go             # Query execution
│   │   └── filters.go           # Filter matching logic
│   ├── api/                      # HTTP API server
│   │   ├── server.go            # HTTP server setup
│   │   ├── search_handler.go    # /v1/search endpoint
│   │   ├── response.go          # Response formatting
│   │   ├── validators.go        # Input validation
│   │   └── errors.go            # Error handling
│   ├── models/                   # Data structures
│   │   ├── event.go
│   │   ├── resource_metadata.go
│   │   ├── storage_segment.go
│   │   ├── segment_metadata.go
│   │   ├── sparse_index.go
│   │   ├── file_metadata.go
│   │   ├── query_request.go
│   │   ├── query_filters.go
│   │   └── query_result.go
│   ├── logging/                  # Structured logging
│   │   └── logger.go
│   └── config/                   # Configuration management
│       └── config.go
├── chart/                        # Helm chart for Kubernetes deployment
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
├── tests/                        # Test suites
│   ├── unit/                     # Unit tests
│   ├── integration/              # Integration tests
│   └── performance/              # Performance tests
├── Makefile                      # Build automation
├── Dockerfile                    # Container image definition
├── go.mod                        # Go module file
└── README.md                     # This file
```

## Configuration

Configure the application via environment variables:

```bash
# Data storage directory (default: ./data)
export RPKDATA_DIR=/data

# API server port (default: 8080)
export RPK_API_PORT=8080

# Log level (default: info)
export RPK_LOG_LEVEL=debug

# Kubernetes resources to watch (comma-separated, default: Pod,Deployment,Service,Node)
export RPK_WATCH_RESOURCES=Pod,Deployment,Service,Node,StatefulSet

# Segment size for compression (default: 1MB)
export RPK_SEGMENT_SIZE=1048576
```

## API Documentation

### Search Endpoint

**GET** `/v1/search`

Query historical events with optional filtering.

**Parameters**:
- `start` (required): Start timestamp (Unix seconds, inclusive)
- `end` (required): End timestamp (Unix seconds, inclusive)
- `kind` (optional): Resource kind (e.g., Pod, Deployment, Node)
- `namespace` (optional): Kubernetes namespace
- `group` (optional): API group (e.g., apps, batch)
- `version` (optional): API version (e.g., v1, v1beta1)

**Response**:
```json
{
  "count": 3,
  "events": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "timestamp": 1700000000000000000,
      "type": "CREATE",
      "resource": {
        "group": "apps",
        "version": "v1",
        "kind": "Deployment",
        "namespace": "default",
        "name": "nginx-deployment",
        "uid": "550e8400-e29b-41d4-a716-446655440001"
      },
      "data": {...},
      "dataSize": 4096,
      "compressedSize": 1024
    }
  ],
  "executionTimeMs": 145,
  "segmentsScanned": 24,
  "segmentsSkipped": 0,
  "filesSearched": 1
}
```

**Example**:
```bash
curl "http://localhost:8080/v1/search?start=1700000000&end=1700086400&kind=Deployment&namespace=default" | jq .
```

## Performance Targets

| Metric | Target | Status |
|--------|--------|--------|
| Event capture latency | <5 seconds | ✓ |
| Query response time (24h) | <2 seconds | ✓ |
| Compression ratio | ≥30% | ✓ |
| Sustained throughput | 1000+ events/min | ✓ |
| Monthly storage (100K events/day) | ≤10GB | ✓ |

## Development

### Running Tests

```bash
# Run all tests
make test

# Run with verbose output
make test-unit

# Run integration tests
make test-integration

# Generate coverage report
make test-coverage
```

### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint

# Run vet
make vet
```

### Docker Development

```bash
# Build Docker image
make docker-build

# Run in Docker
make docker-run

# Push to registry (requires configuration)
docker push <registry>/k8s-event-monitor:latest
```

## Documentation

- **Feature Specification**: [spec.md](specs/001-k8s-event-monitor/spec.md)
- **Implementation Plan**: [plan.md](specs/001-k8s-event-monitor/plan.md)
- **Data Model**: [data-model.md](specs/001-k8s-event-monitor/data-model.md)
- **API Contract**: [contracts/search-api.openapi.yaml](specs/001-k8s-event-monitor/contracts/search-api.openapi.yaml)
- **Research & Decisions**: [research.md](specs/001-k8s-event-monitor/research.md)
- **Quickstart Guide**: [quickstart.md](specs/001-k8s-event-monitor/quickstart.md)

## Architecture

The system is organized into three main components:

### Watcher (Event Capture)
- Monitors Kubernetes resource changes using informer pattern
- Captures CREATE, UPDATE, DELETE events
- Prunes managedFields to reduce data size
- Routes events to storage with minimal latency

### Storage (Persistent Storage)
- Organizes events into hourly files
- Compresses segments using gzip
- Builds sparse timestamp indexes for fast lookups
- Maintains segment metadata for efficient filtering
- Supports queries spanning multiple files

### API (Query Interface)
- HTTP server on port 8080
- `/v1/search` endpoint for querying events
- Multi-dimensional filtering (namespace, kind, group, version)
- Returns results with execution statistics

## Troubleshooting

### Application won't start
```bash
# Check logs
make run

# Verify Kubernetes connectivity
kubectl auth can-i watch pods

# Check RBAC permissions
kubectl auth can-i get pods --as=system:serviceaccount:monitoring:k8s-event-monitor
```

### No events captured
```bash
# Create a test resource
kubectl run test-pod --image=nginx

# Query for recent events
curl "http://localhost:8080/v1/search?start=$(date +%s)&end=$(($(date +%s)+3600))"
```

### Query returns empty results
```bash
# Verify storage files exist
ls -la ./data/

# Query with broader time range
curl "http://localhost:8080/v1/search?start=0&end=9999999999"
```

### Performance issues
```bash
# Check query execution time
curl "http://localhost:8080/v1/search?start=<start>&end=<end>" | jq .executionTimeMs

# Check segment filtering effectiveness
curl "http://localhost:8080/v1/search?start=<start>&end=<end>" | jq '.segmentsScanned, .segmentsSkipped'
```

## License

Apache 2.0

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.

## Support

For issues, questions, or contributions, please open an issue or pull request on the repository.
