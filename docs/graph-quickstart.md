# Graph Reasoning Layer - Quick Start Guide

**Version:** 1.0
**Last Updated:** 2025-12-18

## Overview

The Graph Reasoning Layer adds graph-based causality analysis and root cause detection to Spectre using FalkorDB. This enables LLMs to perform multi-hop relationship traversal and temporal causality inference with provenance and confidence scores.

## Prerequisites

- Docker (for local development with docker-compose)
- Kubernetes cluster (for Helm deployment)
- Spectre v1.0+

## Quick Start (Local Development)

### 1. Start FalkorDB

```bash
# Start FalkorDB using docker-compose
docker-compose -f docker-compose.graph.yml up -d

# Verify it's running
docker ps | grep falkordb
```

### 2. Start Spectre with Graph Support

```bash
# Build Spectre
go build -o spectre ./cmd/spectre

# Start Spectre with graph enabled
./spectre server \
  --data-dir=./data \
  --graph-enabled=true \
  --graph-host=localhost \
  --graph-port=6379 \
  --graph-name=spectre
```

### 3. Start MCP Server with Graph Tools

```bash
# In another terminal
./spectre mcp \
  --spectre-url=http://localhost:8080 \
  --graph-enabled=true \
  --graph-host=localhost \
  --graph-port=6379 \
  --graph-name=spectre
```

### 4. Verify Graph Tools Are Available

```bash
# The MCP server should log:
# "Graph reasoning layer enabled - configuring graph tools"
# "Successfully connected to FalkorDB"

# Check MCP tools endpoint
curl http://localhost:8082/mcp

# Look for:
# - find_root_cause
# - calculate_blast_radius
```

## Deployment (Kubernetes with Helm)

### 1. Enable Graph in values.yaml

```yaml
# chart/values.yaml
graph:
  enabled: true  # Enable graph reasoning layer

  falkordb:
    sidecar: true  # Deploy as sidecar container
    image:
      repository: falkordb/falkordb
      tag: "v4.2.2"

    persistence:
      enabled: true
      size: 5Gi

    resources:
      requests:
        memory: "256Mi"
        cpu: "100m"
      limits:
        memory: "1Gi"
        cpu: "500m"

  sync:
    retentionHours: 24
    rebuildOnStart: true
    rebuildIfEmptyOnly: true
```

### 2. Deploy with Helm

```bash
# Install or upgrade
helm upgrade --install spectre ./chart \
  --namespace monitoring \
  --create-namespace \
  --set graph.enabled=true

# Check pods
kubectl get pods -n monitoring

# You should see 3 containers in the Spectre pod:
# - spectre (main)
# - mcp (MCP server)
# - falkordb (graph database)
```

### 3. Verify Graph Service

```bash
# Check Spectre logs
kubectl logs -n monitoring <spectre-pod> -c spectre | grep graph

# Expected output:
# "Graph reasoning layer enabled - initializing graph service"
# "Graph service initialized successfully"
# "Successfully connected to FalkorDB"
```

## Using Graph Tools

### find_root_cause

Traces backward from a failing resource to identify likely root cause.

**Example:**
```json
{
  "resourceUID": "pod-abc-123",
  "failureTimestamp": 1703001234,
  "maxDepth": 5,
  "minConfidence": 0.6
}
```

**Response:**
```json
{
  "candidates": [
    {
      "resource": {
        "uid": "deployment-frontend",
        "kind": "Deployment",
        "name": "frontend"
      },
      "changeEvent": {
        "timestamp": 1703001200,
        "eventType": "UPDATE",
        "status": "Ready"
      },
      "evidence": [
        {
          "type": "TRIGGERED_BY",
          "description": "Deployment update triggered rollout",
          "confidence": 0.9
        }
      ],
      "confidenceScore": 0.9,
      "timeLagMs": 34000
    }
  ],
  "investigationPrompt": "Root cause analysis for pod-abc-123: Most likely cause is Deployment 'frontend' update (confidence: 90%)..."
}
```

### calculate_blast_radius

Determines which resources are affected by a change.

**Example:**
```json
{
  "resourceUID": "deployment-frontend",
  "changeTimestamp": 1703001000,
  "timeWindowMs": 300000,
  "relationshipTypes": ["OWNS", "SELECTS"]
}
```

**Response:**
```json
{
  "impactedResources": [
    {
      "resource": {
        "uid": "pod-abc-123",
        "kind": "Pod",
        "name": "frontend-abc"
      },
      "relationship": {
        "type": "OWNS",
        "distance": 2
      },
      "impactEvents": [
        {
          "timestamp": 1703001050,
          "status": "Error",
          "lagFromTrigger": 50000
        }
      ],
      "severity": "high"
    }
  ],
  "totalImpacted": 12,
  "byKind": {"Pod": 10, "Job": 2}
}
```

## Configuration Reference

### Spectre Server Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--graph-enabled` | `false` | Enable graph reasoning layer |
| `--graph-host` | `localhost` | FalkorDB host |
| `--graph-port` | `6379` | FalkorDB port |
| `--graph-name` | `spectre` | Graph database name |
| `--graph-retention-hours` | `24` | Data retention window |
| `--graph-rebuild-on-start` | `true` | Rebuild graph on startup |
| `--graph-rebuild-if-empty` | `true` | Only rebuild if empty |
| `--graph-rebuild-window-hours` | `24` | Rebuild time window |

### MCP Server Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--graph-enabled` | `false` | Enable graph tools |
| `--graph-host` | `localhost` | FalkorDB host |
| `--graph-port` | `6379` | FalkorDB port |
| `--graph-name` | `spectre` | Graph database name |

### Environment Variables

MCP server can also be configured via environment variables:

```bash
export GRAPH_ENABLED=true
export GRAPH_HOST=localhost
export GRAPH_PORT=6379
export GRAPH_NAME=spectre
```

## Architecture Overview

```
┌─────────────────┐
│  K8s Cluster    │
└────────┬────────┘
         │ Events
         ▼
┌─────────────────┐     ┌────────────────┐
│ Spectre Storage │────>│ Graph Service  │
│  (Append-only)  │     │  (Sync Pipeline)│
└─────────────────┘     └───────┬────────┘
                                │
                                ▼
                        ┌────────────────┐
                        │   FalkorDB     │
                        │  (24h window)  │
                        └───────┬────────┘
                                │
                                ▼
                        ┌────────────────┐
                        │   MCP Tools    │
                        │ - find_root_   │
                        │   cause        │
                        │ - blast_radius │
                        └────────────────┘
```

## Performance Expectations

- **Memory:** ~250 MB for 24h window (200 resources, 5K events)
- **Sync Lag:** <10 seconds from event capture to graph update
- **Query Latency:**
  - Single-hop: <5ms
  - Multi-hop (3-5 levels): 10-50ms
  - Complex causality: 50-200ms

## Monitoring

### Health Checks

```bash
# FalkorDB health
redis-cli -h localhost -p 6379 ping

# Graph service status (check logs)
kubectl logs -n monitoring <pod> -c spectre | grep "Graph service"

# MCP tools availability
curl http://localhost:8082/mcp | jq '.tools[] | select(.name | contains("graph"))'
```

### Metrics to Watch

- Graph node count
- Graph edge count
- Sync lag (time between Spectre write and graph update)
- Query latency
- Memory usage

## Troubleshooting

### Graph Service Won't Start

**Symptom:** Logs show "Failed to connect to graph database"

**Solutions:**
1. Verify FalkorDB is running: `redis-cli -h localhost -p 6379 ping`
2. Check connection config matches FalkorDB deployment
3. Review firewall/network policies

### Graph Tools Not Available in MCP

**Symptom:** `find_root_cause` and `calculate_blast_radius` missing

**Solutions:**
1. Check MCP server started with `--graph-enabled=true`
2. Verify graph client connected (check MCP logs)
3. Ensure FalkorDB is accessible from MCP container

### High Memory Usage

**Symptom:** FalkorDB consuming >2GB RAM

**Solutions:**
1. Reduce retention window: `--graph-retention-hours=12`
2. Increase cleanup frequency
3. Scale vertically or disable graph layer

### Stale Data in Graph

**Symptom:** Graph queries return outdated information

**Solutions:**
1. Check sync lag in logs
2. Verify storage is receiving events
3. Restart graph service to trigger rebuild

## Advanced Topics

### Manual Graph Rebuild

```bash
# Trigger manual rebuild (requires admin access)
# Currently requires restart with --graph-rebuild-on-start=true

# Future: Admin API endpoint
# curl -X POST http://localhost:8080/v1/graph/rebuild
```

### Custom Causality Heuristics

See `internal/graph/sync/causality.go` for implementing custom relationship inference logic.

### Extending with New Edge Types

1. Add edge type to `internal/graph/models.go`
2. Implement extraction in `internal/graph/sync/builder.go`
3. Update schema queries in `internal/graph/schema.go`
4. Create MCP tool in `internal/mcp/tools/`

## Next Steps

- Review [Design Document](./graph-reasoning-layer-design.md) for architecture details
- Check [Implementation Status](./graph-implementation-status.md) for feature roadmap
- Explore E2E tests in `tests/e2e/mcp_failure_scenarios_test.go`

## Support

For issues or questions:
- Review design doc: `docs/graph-reasoning-layer-design.md`
- Check test files for usage examples
- Run integration tests: `docker-compose -f docker-compose.graph.yml up -d && go test ./internal/graph -v -tags=integration`

