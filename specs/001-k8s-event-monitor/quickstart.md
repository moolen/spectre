# Quickstart Guide: Kubernetes Event Monitoring System

**Feature**: Kubernetes Event Monitoring and Storage System
**Date**: 2025-11-25

## Overview

This guide walks you through setting up the Kubernetes event monitoring system for local development and testing.

## Prerequisites

- Go 1.21 or later
- Docker (for building container image)
- kubectl (for Kubernetes cluster interaction)
- A running Kubernetes cluster (local: minikube, kind, or Docker Desktop; remote: any 1.19+)
- Make

## Local Development Setup

### 1. Clone and Setup

```bash
# Navigate to the project root
cd /path/to/rpk

# Create data directory for local storage
mkdir -p ./data

# Verify Go version
go version  # Should be 1.21+
```

### 2. Install Dependencies

```bash
# Download Go module dependencies
go mod download

# Verify dependencies
go mod tidy
go mod verify
```

### 3. Build the Application

```bash
# Using Makefile (recommended)
make build

# Or manually
mkdir -p bin/
go build -o bin/k8s-event-monitor ./cmd/main.go
```

### 4. Run Locally (Standalone Mode)

For local testing without a real Kubernetes cluster:

```bash
# Start the monitoring service
make run

# In another terminal, test the API
curl -X GET "http://localhost:8080/v1/search?start=1700000000&end=1700086400"
```

The application will start watching available Kubernetes resources (if cluster is accessible).

## Kubernetes Deployment

### 1. Prerequisites for Cluster Deployment

Ensure you have:
- kubectl configured and pointing to your target cluster
- A persistent volume or storage class available
- RBAC permissions to create deployments, services, and RBAC resources

### 2. Deploy with Helm

```bash
# Install the Helm chart
make deploy

# Or manually:
helm install k8s-event-monitor ./chart \
  --namespace monitoring \
  --create-namespace \
  --values ./chart/values.yaml

# Verify deployment
kubectl get pods -n monitoring
kubectl get svc -n monitoring
```

### 3. Configure Deployment

Edit `chart/values.yaml` to configure:
- Storage: persistence volume size and mount path
- Resources: CPU/memory requests and limits
- Logging: log level and format
- API: port and security settings

### 4. Query Events from Cluster

```bash
# Port-forward the service
kubectl port-forward -n monitoring svc/k8s-event-monitor 8080:8080 &

# Query events
curl -X GET "http://localhost:8080/v1/search?start=1700000000&end=1700086400&kind=Deployment&namespace=default"

# Stop port-forward
jobs -l
kill %1
```

## Development Workflow

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific test package
go test -v ./internal/storage

# Run tests with coverage
go test -cover ./...
```

### Building Docker Image

```bash
# Using Makefile
make docker-build

# Or manually
docker build -t k8s-event-monitor:latest .
```

### Local Development with Code Changes

```bash
# Watch and rebuild on changes
make watch   # if available

# Or manual rebuild cycle
make clean
make build
make run
```

## API Examples

### Query All Events in Time Window

```bash
curl -X GET "http://localhost:8080/v1/search?start=1700000000&end=1700086400"
```

### Query Deployments in Default Namespace

```bash
curl -X GET "http://localhost:8080/v1/search?start=1700000000&end=1700086400&kind=Deployment&namespace=default"
```

### Query All Nodes Cluster-Wide

```bash
curl -X GET "http://localhost:8080/v1/search?start=1700000000&end=1700086400&kind=Node"
```

### Query Events from Specific API Group

```bash
curl -X GET "http://localhost:8080/v1/search?start=1700000000&end=1700086400&group=apps&version=v1&kind=StatefulSet"
```

### Pretty-Print Response

```bash
curl -s -X GET "http://localhost:8080/v1/search?start=1700000000&end=1700086400&kind=Deployment&namespace=default" | jq .
```

## Monitoring Storage Usage

### Check Local Storage

```bash
# Size of stored events
du -sh ./data/

# List storage files
ls -lah ./data/

# Files per hour
ls -1 ./data/ | wc -l
```

### Monitor Running Application

```bash
# Check application logs
kubectl logs -n monitoring deployment/k8s-event-monitor -f

# Check pod resource usage
kubectl top pod -n monitoring

# Check persistent volume usage
kubectl exec -n monitoring <pod-name> -- df -h /data
```

## Troubleshooting

### Application Won't Start

```bash
# Check logs
kubectl logs -n monitoring <pod-name>

# Common issues:
# - RBAC permissions: Add cluster-admin role
# - Storage: Check persistent volume exists and is mounted
# - Kubernetes connectivity: Verify in-cluster service account has permissions
```

### No Events Captured

```bash
# Verify watchers are active
kubectl logs -n monitoring <pod-name> | grep -i watcher

# Check RBAC permissions
kubectl auth can-i watch pods --as=system:serviceaccount:monitoring:k8s-event-monitor

# Create a test resource
kubectl run test-pod --image=nginx

# Check if event was captured
curl "http://localhost:8080/v1/search?start=<now>&end=<now>&kind=Pod&namespace=default"
```

### Query Returns Empty Results

```bash
# Verify storage files exist
kubectl exec -n monitoring <pod-name> -- ls -la /data/

# Check correct time range (use current Unix time)
date +%s

# Query broader time range
curl "http://localhost:8080/v1/search?start=0&end=9999999999"
```

### Performance Issues

```bash
# Check query execution time in response
curl "http://localhost:8080/v1/search?..." | jq .executionTimeMs

# Monitor segments skipped (high value = good filtering)
curl "http://localhost:8080/v1/search?..." | jq '.segmentsScanned, .segmentsSkipped'

# If slow:
# - Reduce time window in queries
# - Add more specific filters (namespace/kind)
# - Check disk I/O performance
```

## File Structure After Setup

```
rpk/
├── cmd/
│   └── main.go                  # Application entry point
├── internal/
│   ├── watcher/                 # K8s watcher implementation
│   ├── storage/                 # Storage engine
│   ├── api/                      # HTTP API server
│   └── models/                   # Data structures
├── chart/                        # Helm chart
├── data/                         # Local storage (created by app)
├── bin/                          # Compiled binary (created by make)
├── Makefile                      # Build automation
├── go.mod
└── go.sum
```

## Next Steps

- Review [data-model.md](data-model.md) for entity definitions
- Check [contracts/search-api.openapi.yaml](contracts/search-api.openapi.yaml) for API specification
- See [research.md](research.md) for design decisions and rationale
- Run `/speckit.tasks` to generate implementation tasks

## Support & Documentation

- **Feature Spec**: [spec.md](spec.md)
- **Implementation Plan**: [plan.md](plan.md)
- **Data Model**: [data-model.md](data-model.md)
- **API Specification**: [contracts/search-api.openapi.yaml](contracts/search-api.openapi.yaml)
