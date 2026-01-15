---
title: resource_timeline_changes Tool
description: Get semantic field-level changes for resources by UID with noise filtering
keywords: [mcp, tools, resource_timeline_changes, incident, analysis, diff]
---

# resource_timeline_changes Tool

Get semantic field-level changes for Kubernetes resources by UID with automatic noise filtering and status condition summarization.

## Overview

The `resource_timeline_changes` tool retrieves detailed semantic diffs between resource versions over time. Unlike simple event logging, this tool computes field-level changes (path, old value, new value) and filters out noisy auto-generated fields like `managedFields` and `resourceVersion`.

**Key Capabilities:**
- **Semantic Diffs**: Field-level changes with path, old/new values, and operation type
- **Noise Filtering**: Automatically removes `managedFields`, `resourceVersion`, `generation`, etc.
- **Status Summarization**: Condenses status condition history to save tokens
- **Batch Queries**: Query multiple resources by UID in a single call
- **Change Categorization**: Classifies changes as Config, Status, Labels, Annotations, etc.

**When to Use:**
- Understanding exactly what changed in a resource
- Tracking configuration drift over time
- Investigating status condition transitions
- Correlating changes across multiple resources

**When NOT to Use:**
- Cluster-wide health overview (use `cluster_health` instead)
- Deep investigation with events and logs (use `investigate` instead)
- Finding resources by kind/namespace (use cluster_health first to discover UIDs)

## Quick Example

### Minimal Usage

```json
{
  "resource_uids": ["abc-123-def-456"]
}
```

Returns semantic changes for the resource in the last hour (default time window).

### Typical Usage

```json
{
  "resource_uids": ["abc-123-def-456", "xyz-789-ghi-012"],
  "start_time": 1702382400,
  "end_time": 1702386000,
  "max_changes_per_resource": 50
}
```

Returns semantic changes for multiple resources in the specified time window.

## Input Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `resource_uids` | array | **Yes** | - | List of resource UIDs to query (max 10) |
| `start_time` | int64 | No | 1 hour ago | Start of time window (Unix timestamp in seconds or milliseconds) |
| `end_time` | int64 | No | now | End of time window (Unix timestamp in seconds or milliseconds) |
| `include_full_snapshot` | bool | No | `false` | Include first segment's full resource JSON |
| `max_changes_per_resource` | int | No | `50` | Maximum changes per resource (max 200) |

### Getting Resource UIDs

Resource UIDs can be obtained from:
1. **cluster_health** tool output - resources include their UIDs
2. **investigate** tool output - includes resource UID
3. Kubernetes API - `kubectl get pod <name> -o jsonpath='{.metadata.uid}'`

## Output Structure

```json
{
  "resources": [
    {
      "uid": "abc-123-def-456",
      "kind": "Deployment",
      "namespace": "production",
      "name": "api-server",
      "changes": [
        {
          "timestamp": 1702384200,
          "timestamp_text": "2024-12-12T10:30:00Z",
          "path": "spec.template.spec.containers[0].image",
          "old": "api-server:v1.0.0",
          "new": "api-server:v1.1.0",
          "op": "replace",
          "category": "Config"
        },
        {
          "timestamp": 1702384300,
          "timestamp_text": "2024-12-12T10:31:40Z",
          "path": "status.replicas",
          "old": 3,
          "new": 2,
          "op": "replace",
          "category": "Status"
        }
      ],
      "status_summary": {
        "current_status": "Warning",
        "transitions": [
          {
            "from_status": "Ready",
            "to_status": "Warning",
            "timestamp": 1702384300,
            "timestamp_text": "2024-12-12T10:31:40Z",
            "reason": "UnavailableReplicas"
          }
        ],
        "condition_history": {
          "Available": "True(2h) -> False(5m)",
          "Progressing": "True(2h)"
        }
      },
      "change_count": 2
    }
  ],
  "summary": {
    "total_resources": 1,
    "total_changes": 2,
    "resources_with_errors": 0,
    "resources_not_found": 0
  },
  "execution_time_ms": 45
}
```

### Top-Level Fields

| Field | Type | Description |
|-------|------|-------------|
| `resources` | array | List of resources with their semantic changes |
| `summary` | object | Aggregated summary across all resources |
| `execution_time_ms` | int64 | Processing time in milliseconds |

### Resource Entry Fields

| Field | Type | Description |
|-------|------|-------------|
| `uid` | string | Resource UID |
| `kind` | string | Resource kind (e.g., `Pod`, `Deployment`) |
| `namespace` | string | Kubernetes namespace |
| `name` | string | Resource name |
| `changes` | array | List of semantic changes (sorted by timestamp) |
| `status_summary` | object | Summarized status condition history |
| `change_count` | int | Total number of changes detected |
| `first_snapshot` | object | Full resource JSON (only if `include_full_snapshot: true`) |

### SemanticChange Fields

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | int64 | Unix timestamp when change occurred |
| `timestamp_text` | string | Human-readable timestamp (ISO 8601) |
| `path` | string | JSON path to changed field (e.g., `spec.replicas`) |
| `old` | any | Previous value (null for additions) |
| `new` | any | New value (null for deletions) |
| `op` | string | Operation type: `add`, `replace`, `remove` |
| `category` | string | Change category (see below) |

### Change Categories

| Category | Description | Example Paths |
|----------|-------------|---------------|
| `Config` | Configuration changes | `spec.*`, `data.*` |
| `Status` | Status field changes | `status.*` |
| `Labels` | Label modifications | `metadata.labels.*` |
| `Annotations` | Annotation changes | `metadata.annotations.*` |
| `Finalizers` | Finalizer changes | `metadata.finalizers` |
| `OwnerRef` | Owner reference changes | `metadata.ownerReferences` |
| `Other` | Uncategorized changes | Everything else |

### StatusSummary Fields

| Field | Type | Description |
|-------|------|-------------|
| `current_status` | string | Current overall status (Ready, Warning, Error, Terminating) |
| `transitions` | array | List of status transitions with timestamps |
| `condition_history` | object | Condensed condition timeline per condition type |

## Noise Filtering

The following paths are automatically filtered to reduce token usage:

- `metadata.managedFields`
- `metadata.resourceVersion`
- `metadata.generation`
- `metadata.uid`
- `metadata.creationTimestamp`
- `status.observedGeneration`

This filtering ensures you see meaningful changes without auto-generated noise.

## Usage Patterns

### Pattern 1: Investigate a Failing Deployment

**Step 1**: Get resource UID from cluster_health
```json
// cluster_health response includes:
{
  "resources": [
    {
      "uid": "abc-123",
      "kind": "Deployment",
      "name": "api-server",
      "status": "Error"
    }
  ]
}
```

**Step 2**: Get semantic changes
```json
{
  "resource_uids": ["abc-123"],
  "start_time": 1702382400,
  "end_time": 1702386000
}
```

### Pattern 2: Correlate Multiple Resources

```json
{
  "resource_uids": [
    "deployment-uid-123",
    "replicaset-uid-456",
    "pod-uid-789"
  ],
  "start_time": 1702382400,
  "end_time": 1702386000
}
```

Returns changes for all resources, allowing correlation of deployment → replicaset → pod changes.

### Pattern 3: Track Configuration Drift

```json
{
  "resource_uids": ["configmap-uid-abc"],
  "start_time": 1702296000,
  "end_time": 1702382400
}
```

Returns all configuration changes over a 24-hour period.

## Real-World Example

### Deployment Rollout Analysis

**Request**:
```json
{
  "resource_uids": ["deployment-abc-123"],
  "start_time": 1702382400,
  "end_time": 1702386000
}
```

**Response**:
```json
{
  "resources": [
    {
      "uid": "deployment-abc-123",
      "kind": "Deployment",
      "namespace": "production",
      "name": "api-server",
      "changes": [
        {
          "timestamp": 1702384000,
          "timestamp_text": "2024-12-12T10:26:40Z",
          "path": "spec.template.spec.containers[0].image",
          "old": "api-server:v1.0.0",
          "new": "api-server:v1.1.0",
          "op": "replace",
          "category": "Config"
        },
        {
          "timestamp": 1702384200,
          "timestamp_text": "2024-12-12T10:30:00Z",
          "path": "status.updatedReplicas",
          "old": 3,
          "new": 1,
          "op": "replace",
          "category": "Status"
        },
        {
          "timestamp": 1702384500,
          "timestamp_text": "2024-12-12T10:35:00Z",
          "path": "status.unavailableReplicas",
          "old": null,
          "new": 2,
          "op": "add",
          "category": "Status"
        }
      ],
      "status_summary": {
        "current_status": "Warning",
        "transitions": [
          {
            "from_status": "Ready",
            "to_status": "Warning",
            "timestamp": 1702384500,
            "reason": "UnavailableReplicas"
          }
        ],
        "condition_history": {
          "Available": "True(1h) -> False(30m)",
          "Progressing": "True(1h30m)"
        }
      },
      "change_count": 3
    }
  ],
  "summary": {
    "total_resources": 1,
    "total_changes": 3,
    "resources_with_errors": 0,
    "resources_not_found": 0
  }
}
```

**Analysis**:
- Image changed from v1.0.0 to v1.1.0 at 10:26
- Rolling update started, reducing updated replicas
- Unavailable replicas appeared at 10:35 (pods not starting)
- Status transitioned from Ready to Warning

## Best Practices

### Do
- **Get UIDs from cluster_health first** - Don't guess UIDs
- **Use time windows** - Default 1 hour is good for incidents
- **Batch related resources** - Query deployment + pods together
- **Check status_summary** - Condensed view saves time

### Don't
- **Don't query >10 UIDs** - Tool has a limit
- **Don't use >24h windows** - Results become overwhelming
- **Don't skip cluster_health** - You need UIDs first

## Related Documentation

- [cluster_health Tool](./cluster-health.md) - Get resource UIDs and health overview
- [investigate Tool](./investigate.md) - Deep investigation with events and logs
- [Post-Mortem Prompt](../prompts-reference/post-mortem.md) - Uses resource_timeline_changes in workflow

<!-- Source: internal/mcp/tools/resource_timeline_changes.go -->
