---
title: resource_explorer Tool
description: Browse and discover Kubernetes resources with flexible filtering
keywords: [mcp, tools, resource_explorer, browse, discover, filter]
---

# resource_explorer Tool

Browse, discover, and explore Kubernetes resources across your cluster with flexible filtering by kind, namespace, and status.

## Overview

The `resource_explorer` tool provides a browsing interface for discovering resources in your cluster, helping you find what exists before investigating specific resources in detail.

**Key Capabilities:**
- **Resource Discovery**: Find all resources of a specific kind or across all kinds
- **Status Filtering**: Filter by resource status (Ready, Warning, Error, Terminating, Unknown)
- **Namespace Filtering**: Scope exploration to specific namespaces
- **Time-based Snapshots**: View resources as they existed at a specific point in time
- **Available Options**: Discover what kinds, namespaces, and statuses exist in your cluster
- **Issue Counting**: See error and warning counts at a glance

**When to Use:**
- Discovering what resources exist in your cluster
- Finding resources by status (all Error pods, Warning deployments, etc.)
- Browsing resources before deep investigation
- Understanding cluster inventory (what kinds and namespaces exist)
- Finding resources to investigate with the `investigate` tool

**When NOT to Use:**
- Deep investigation of a specific resource (use `investigate` instead)
- Identifying what changed during an incident (use `resource_changes` instead)
- Cluster health overview with aggregated metrics (use `cluster_health` instead)

## Quick Example

### Browse All Resources

```json
{}
```

Returns up to 200 resources (default limit) from the entire cluster with available filter options.

### Filter by Kind

```json
{
  "kind": "Pod"
}
```

Returns all Pods in the cluster.

### Filter by Status

```json
{
  "status": "Error"
}
```

Returns all resources currently in Error state.

### Combined Filters

```json
{
  "kind": "Pod",
  "namespace": "production",
  "status": "Error",
  "max_resources": 50
}
```

Returns up to 50 Error Pods in the `production` namespace.

## Input Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `kind` | string | No | `""` (all) | Resource kind to filter by (e.g., `Pod`, `Deployment`) |
| `namespace` | string | No | `""` (all) | Kubernetes namespace to filter by |
| `status` | string | No | `""` (all) | Status to filter by: `Ready`, `Warning`, `Error`, `Terminating`, `Unknown` |
| `time` | int64 | No | `0` (latest) | Snapshot at specific time (Unix timestamp); `0` = use latest data |
| `max_resources` | int | No | `200` | Maximum resources to return (max: 1000) |

### Status Values

| Status | Meaning | Example Resources |
|--------|---------|-------------------|
| `Ready` | Resource is healthy and functioning normally | Running Pods, available Deployments |
| `Warning` | Resource has warnings but is still functioning | Pod with restart count > 0, Deployment with partial availability |
| `Error` | Resource is in error state | CrashLoopBackOff Pods, failed Deployments |
| `Terminating` | Resource is being deleted | Pods being terminated, Deployments being deleted |
| `Unknown` | Status cannot be determined | Resources without status information |

### Time-Based Snapshots

**Latest (default)**:
```json
{"time": 0}  // Uses full available time range from Spectre
```

**Specific Point in Time**:
```json
{"time": 1702382400}  // View resources as they were at this timestamp
```

**How it works**:
- If `time` is specified, tool queries a 1-hour window (±30 minutes) around that time
- If `time = 0` (default), tool queries the full available data range

### Resource Limits

| Limit | Use Case | Performance |
|-------|----------|-------------|
| `50` | Quick discovery, focused exploration | ~50-100 ms |
| `200` (default) | Standard browsing | ~150-300 ms |
| `500` | Large cluster exploration | ~400-800 ms |
| `1000` (max) | Comprehensive inventory | ~800-1500 ms |

## Output Structure

```json
{
  "resources": [
    {
      "kind": "Pod",
      "namespace": "production",
      "name": "api-server-85f6c9b8-k4x2p",
      "current_status": "Error",
      "issue_count": 5,
      "error_count": 2,
      "warning_count": 3,
      "event_count": 18,
      "last_status_change": 1702383200,
      "last_status_change_text": "2024-12-12 10:13:20 UTC"
    },
    {
      "kind": "Pod",
      "namespace": "production",
      "name": "worker-6c8d4f-x7k2p",
      "current_status": "Running",
      "issue_count": 0,
      "error_count": 0,
      "warning_count": 0,
      "event_count": 5,
      "last_status_change": 1702382500,
      "last_status_change_text": "2024-12-12 10:01:40 UTC"
    }
  ],
  "available_options": {
    "kinds": ["Pod", "Deployment", "Service", "ConfigMap", "StatefulSet"],
    "namespaces": ["default", "production", "staging", "kube-system"],
    "statuses": ["Ready", "Warning", "Error", "Terminating", "Unknown"],
    "time_range": {
      "start": 1702296000,
      "end": 1702382400,
      "start_text": "2024-12-11 10:00:00 UTC",
      "end_text": "2024-12-12 10:00:00 UTC"
    }
  },
  "resource_count": 47,
  "exploration_time_ms": 187
}
```

### Top-Level Fields

| Field | Type | Description |
|-------|------|-------------|
| `resources` | array | List of discovered resources (sorted by kind, namespace, name) |
| `available_options` | object | Available values for filters (kinds, namespaces, statuses, time_range) |
| `resource_count` | int | Number of resources returned (after filtering and limiting) |
| `exploration_time_ms` | int64 | Processing time in milliseconds |

### ResourceInfo Fields

| Field | Type | Description |
|-------|------|-------------|
| `kind` | string | Resource kind (e.g., `Pod`, `Deployment`) |
| `namespace` | string | Kubernetes namespace |
| `name` | string | Resource name |
| `current_status` | string | Current status (`Ready`, `Warning`, `Error`, `Terminating`, `Unknown`) |
| `issue_count` | int | Total issues (error_count + warning_count) |
| `error_count` | int | Number of error status segments |
| `warning_count` | int | Number of warning events |
| `event_count` | int | Total number of Kubernetes events for this resource |
| `last_status_change` | int64 | Unix timestamp of last status transition |
| `last_status_change_text` | string | Human-readable timestamp of last change |

### AvailableOptions Fields

| Field | Type | Description |
|-------|------|-------------|
| `kinds` | array | List of resource kinds available in cluster (e.g., `["Pod", "Deployment"]`) |
| `namespaces` | array | List of namespaces available (e.g., `["default", "production"]`) |
| `statuses` | array | List of possible status values (always fixed list) |
| `time_range` | object | Available time range for historical queries |

### TimeRange Fields

| Field | Type | Description |
|-------|------|-------------|
| `start` | int64 | Earliest data available (Unix timestamp) |
| `end` | int64 | Latest data available (Unix timestamp) |
| `start_text` | string | Human-readable start time |
| `end_text` | string | Human-readable end time |

## Usage Patterns

### Pattern 1: Discover Cluster Inventory

**Goal**: Find out what kinds of resources exist

```json
{}
```

**What you get**:
- List of resources (up to 200)
- `available_options.kinds` → All resource types in cluster
- `available_options.namespaces` → All namespaces
- `available_options.time_range` → Data availability window

**Use Case**: "What's running in my cluster?"

### Pattern 2: Find All Error Resources

**Goal**: Quickly identify all failing resources

```json
{
  "status": "Error",
  "max_resources": 50
}
```

**What you get**:
- Top 50 resources in Error state across all kinds and namespaces
- Issue counts for each resource

**Use Case**: "Show me everything that's broken"

### Pattern 3: Browse Specific Kind

**Goal**: See all resources of a specific type

```json
{
  "kind": "Pod",
  "max_resources": 100
}
```

**What you get**:
- Up to 100 Pods across all namespaces
- Status and issue counts for each Pod

**Use Case**: "List all Pods in my cluster"

### Pattern 4: Namespace-Scoped Exploration

**Goal**: Explore resources in a specific namespace

```json
{
  "namespace": "production",
  "max_resources": 200
}
```

**What you get**:
- All resource types in `production` namespace (up to 200)
- Cross-kind view of production environment

**Use Case**: "What's deployed in production?"

### Pattern 5: Find Investigation Targets

**Goal**: Find Error resources to investigate

```json
{
  "kind": "Pod",
  "namespace": "production",
  "status": "Error",
  "max_resources": 10
}
```

**Then investigate each**:
```json
{
  "resource_kind": "Pod",
  "resource_name": "{name from explorer}",
  "namespace": "production"
}
```

**Use Case**: "Find failing Pods, then investigate them"

### Pattern 6: Historical Snapshot

**Goal**: See what existed at a specific time

```json
{
  "time": 1702382400,
  "kind": "Deployment",
  "namespace": "production"
}
```

**What you get**:
- Deployments as they were at 10:00 AM on Dec 12
- Status at that point in time

**Use Case**: "What deployments were running during the incident?"

## Real-World Examples

### Example 1: Find All Error Pods

**Scenario**: Quick overview of all failing Pods

**Request**:
```json
{
  "kind": "Pod",
  "status": "Error",
  "max_resources": 20
}
```

**Response** (abbreviated):
```json
{
  "resources": [
    {
      "kind": "Pod",
      "namespace": "production",
      "name": "api-server-85f6c9b8-k4x2p",
      "current_status": "Error",
      "issue_count": 5,
      "error_count": 2,
      "warning_count": 3,
      "event_count": 18,
      "last_status_change": 1702383200,
      "last_status_change_text": "2024-12-12 10:13:20 UTC"
    },
    {
      "kind": "Pod",
      "namespace": "staging",
      "name": "worker-7d9f8c5b-z9k3p",
      "current_status": "Error",
      "issue_count": 2,
      "error_count": 1,
      "warning_count": 1,
      "event_count": 8,
      "last_status_change": 1702384100,
      "last_status_change_text": "2024-12-12 10:28:20 UTC"
    }
  ],
  "available_options": {
    "kinds": ["Pod", "Deployment", "Service"],
    "namespaces": ["default", "production", "staging"],
    "statuses": ["Ready", "Warning", "Error", "Terminating", "Unknown"],
    "time_range": {
      "start": 1702296000,
      "end": 1702382400
    }
  },
  "resource_count": 2,
  "exploration_time_ms": 87
}
```

**Analysis**:
- 2 Error Pods found across production and staging
- Both have multiple issues (warnings + errors)
- Can use this to prioritize investigation

### Example 2: Browse Production Namespace

**Scenario**: See all resources in production

**Request**:
```json
{
  "namespace": "production",
  "max_resources": 100
}
```

**Response** (abbreviated):
```json
{
  "resources": [
    {
      "kind": "ConfigMap",
      "namespace": "production",
      "name": "app-config",
      "current_status": "Ready",
      "issue_count": 0,
      "event_count": 2
    },
    {
      "kind": "Deployment",
      "namespace": "production",
      "name": "api-server",
      "current_status": "Ready",
      "issue_count": 0,
      "event_count": 5
    },
    {
      "kind": "Pod",
      "namespace": "production",
      "name": "api-server-85f6c9b8-k4x2p",
      "current_status": "Error",
      "issue_count": 5,
      "event_count": 18
    },
    {
      "kind": "Service",
      "namespace": "production",
      "name": "api-server",
      "current_status": "Ready",
      "issue_count": 0,
      "event_count": 1
    }
  ],
  "available_options": {
    "kinds": ["ConfigMap", "Deployment", "Pod", "Service"],
    "namespaces": ["production"]
  },
  "resource_count": 4,
  "exploration_time_ms": 124
}
```

**Analysis**:
- 4 resources in production namespace
- Sorted by kind (ConfigMap → Deployment → Pod → Service)
- 1 Pod in Error state, others Ready
- Can see full inventory

### Example 3: Cluster-Wide Discovery

**Scenario**: First-time cluster exploration

**Request**:
```json
{
  "max_resources": 50
}
```

**Response** (abbreviated):
```json
{
  "resources": [
    {"kind": "ConfigMap", "namespace": "default", "name": "cluster-config", "current_status": "Ready"},
    {"kind": "ConfigMap", "namespace": "production", "name": "app-config", "current_status": "Ready"},
    {"kind": "Deployment", "namespace": "default", "name": "nginx", "current_status": "Ready"},
    {"kind": "Deployment", "namespace": "production", "name": "api-server", "current_status": "Warning"},
    {"kind": "Pod", "namespace": "default", "name": "nginx-7d8b5f9c6b-x7k2p", "current_status": "Running"},
    {"kind": "Pod", "namespace": "production", "name": "api-server-85f6c9b8-k4x2p", "current_status": "Error"}
  ],
  "available_options": {
    "kinds": ["ConfigMap", "Deployment", "Pod", "Service", "StatefulSet"],
    "namespaces": ["default", "production", "staging", "kube-system"],
    "statuses": ["Ready", "Warning", "Error", "Terminating", "Unknown"],
    "time_range": {
      "start": 1702296000,
      "end": 1702382400,
      "start_text": "2024-12-11 10:00:00 UTC",
      "end_text": "2024-12-12 10:00:00 UTC"
    }
  },
  "resource_count": 6,
  "exploration_time_ms": 156
}
```

**Analysis**:
- Discovered 5 resource kinds in cluster
- 4 namespaces available
- Data available from Dec 11 10:00 to Dec 12 10:00 (24 hours)
- Can refine search using `available_options`

### Example 4: Historical Snapshot

**Scenario**: View resources as they were during an incident

**Request**:
```json
{
  "kind": "Deployment",
  "time": 1702382400,
  "max_resources": 20
}
```

**Response** (abbreviated):
```json
{
  "resources": [
    {
      "kind": "Deployment",
      "namespace": "production",
      "name": "api-server",
      "current_status": "Warning",
      "issue_count": 2,
      "last_status_change": 1702382100,
      "last_status_change_text": "2024-12-12 09:55:00 UTC"
    },
    {
      "kind": "Deployment",
      "namespace": "production",
      "name": "frontend",
      "current_status": "Ready",
      "issue_count": 0,
      "last_status_change": 1702381800,
      "last_status_change_text": "2024-12-12 09:50:00 UTC"
    }
  ],
  "resource_count": 2,
  "exploration_time_ms": 98
}
```

**Analysis**:
- Snapshot at 10:00 AM (time = 1702382400)
- api-server Deployment was in Warning state 5 minutes before snapshot
- Useful for understanding pre-incident state

## Performance Characteristics

### Execution Time

| Scenario | Resource Count | Avg Time | P95 Time |
|----------|---------------|----------|----------|
| Small query (50 resources) | 50 | 50-100 ms | 150 ms |
| Default query (200 resources) | 200 | 150-300 ms | 450 ms |
| Large query (500 resources) | 500 | 400-800 ms | 1,200 ms |
| Max query (1000 resources) | 1000 | 800-1500 ms | 2,000 ms |

**Note**: Execution time depends on:
- Resource count in cluster
- Filtering complexity (status filtering requires status calculation)
- Time range queried
- Cache hit rate

### Optimization Tips

**1. Use Specific Filters**
```json
// Slower: No filters (returns up to 200)
{}

// Faster: Filter by kind and namespace
{"kind": "Pod", "namespace": "production"}
```

**2. Reduce Max Resources**
```json
// Slower: 1000 resources
{"max_resources": 1000}

// Faster: 50 resources
{"max_resources": 50}
```

**3. Filter by Status Early**
```json
// Efficient: Filter by status
{"status": "Error", "max_resources": 50}
```

### Memory Usage

**Estimated Memory:**
```
Memory = resource_count × 1 KB (average resource info) + overhead

50 resources: ~50 KB
200 resources: ~200 KB
1000 resources: ~1 MB
```

**Note**: Tool returns summary information, not full resource manifests, so memory usage is low.

## Integration Patterns

### Pattern 1: Explore → Investigate Workflow

**Step 1**: Find Error resources with `resource_explorer`
```json
{
  "kind": "Pod",
  "status": "Error",
  "namespace": "production",
  "max_resources": 10
}
```

**Step 2**: Pick a resource and investigate with `investigate`
```json
{
  "resource_kind": "Pod",
  "resource_name": "api-server-85f6c9b8-k4x2p",
  "namespace": "production"
}
```

### Pattern 2: Cluster Health → Explorer → Investigate

**Step 1**: Get overview with `cluster_health`
```json
{"start_time": 1702382400, "end_time": 1702386000}
```

**Step 2**: **Explore problematic kind** with `resource_explorer`
```json
{"kind": "Pod", "status": "Error"}
```

**Step 3**: Investigate specific resource with `investigate`

### Pattern 3: Multi-Namespace Discovery

**Goal**: Find all Error resources across namespaces

```json
{"status": "Error", "max_resources": 100}
```

**Analysis**: Group by namespace to identify which environments have issues

### Pattern 4: Pre-Investigation Discovery

**Before using `investigate` tool with wildcards**, use `resource_explorer` to:
1. Discover available namespaces: `{}`
2. See resource counts: `{"kind": "Pod", "namespace": "production"}`
3. Filter to interesting resources: `{"status": "Error"}`
4. Investigate: Use names from explorer

## Troubleshooting

### Empty Resources Array

**Symptom**: `resources: []`

**Common Causes:**

**1. No matching resources**
```json
// Request
{"kind": "StatefulSet", "namespace": "nonexistent"}

// Response
{"resources": [], "resource_count": 0}
```
**Solution**: Check `available_options.kinds` and `available_options.namespaces`

**2. Status filter too restrictive**
```json
// Request
{"status": "Error"}

// Response if no errors
{"resources": [], "resource_count": 0}
```
**Solution**: Remove status filter or try different status

**3. Time snapshot out of range**
```json
// Request with time before data availability
{"time": 1234567890}

// Response
{"resources": []}
```
**Solution**: Check `available_options.time_range` for valid times

### Limited Results

**Symptom**: Got results but less than expected

**Cause**: Hit `max_resources` limit

```json
// Request
{"max_resources": 10}

// Response
{"resource_count": 10}  // Exactly 10 suggests there may be more
```

**Solution**: Increase `max_resources` or add more specific filters

### Missing Resource Kinds

**Symptom**: Expected kind not in `available_options.kinds`

**Possible Causes:**
- Kind has no events in Spectre data
- Kind name case mismatch (use `Deployment`, not `deployment`)
- Resource type not supported by Kubernetes API

**Solution**: Verify kind name and check Spectre is receiving events for that kind

### High Exploration Time

**Symptom**: `exploration_time_ms > 1000` (1+ second)

**Causes:**
- Very large cluster (1000+ resources)
- Status filter requires processing all resources
- Historical time query

**Solutions:**
1. Add kind or namespace filter to reduce scope
2. Reduce `max_resources`
3. Use latest data (time = 0) instead of historical snapshots

## API Reference

### MCP Protocol Request

**Basic Discovery**:
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "resource_explorer",
    "arguments": {}
  },
  "id": 1
}
```

**Filtered Query**:
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "resource_explorer",
    "arguments": {
      "kind": "Pod",
      "namespace": "production",
      "status": "Error",
      "max_resources": 50
    }
  },
  "id": 1
}
```

**Historical Snapshot**:
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "resource_explorer",
    "arguments": {
      "kind": "Deployment",
      "time": 1702382400,
      "max_resources": 100
    }
  },
  "id": 1
}
```

### MCP Protocol Response

```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"resources\":[...],\"available_options\":{...},\"resource_count\":47}"
      }
    ]
  },
  "id": 1
}
```

**Note**: Response is JSON-encoded string in `text` field.

### cURL Example

```bash
curl -X POST http://localhost:8081/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
      "name": "resource_explorer",
      "arguments": {
        "kind": "Pod",
        "status": "Error",
        "max_resources": 20
      }
    },
    "id": 1
  }'
```

### Claude Desktop Usage

**Natural Language**:
```
Show me all failing pods in production
```

**Direct Tool Call**:
```
Use the resource_explorer tool with:
- kind: Pod
- namespace: production
- status: Error
```

**Note**: Claude Desktop automatically formats MCP requests; no manual JSON required.

## Best Practices

### ✅ Do

- **Start with broad queries** - Use `{}` to discover available options
- **Check available_options** - Use kinds/namespaces from response for filtering
- **Use status filters** - Quickly find Error or Warning resources
- **Set reasonable limits** - Use `max_resources: 50-200` for fast queries
- **Combine filters** - Use kind + namespace + status for targeted discovery
- **Sort results mentally** - Resources are sorted by kind, namespace, name
- **Use for discovery** - Find resources before investigating deeply
- **Check time_range** - Understand data availability before historical queries

### ❌ Don't

- **Don't use for deep analysis** - Use `investigate` tool for detailed timelines
- **Don't ignore available_options** - Shows what's actually in your cluster
- **Don't set max_resources too high** - 1000 is slow, 200 is usually sufficient
- **Don't assume resource exists** - Check available_options.kinds first
- **Don't use for change detection** - Use `resource_changes` tool instead
- **Don't query very old times** - Check time_range.start for data availability
- **Don't expect full manifests** - Tool returns summary info, not full YAML
- **Don't use for monitoring** - Tool analyzes historical data, not live state

## Related Documentation

- [investigate Tool](./investigate.md) - Deep investigation after finding resources with explorer
- [cluster_health Tool](./cluster-health.md) - Cluster-wide health overview
- [resource_changes Tool](./resource-changes.md) - Identify what changed
- [MCP Configuration](../../configuration/mcp-configuration.md) - MCP server deployment

<!-- Source: internal/mcp/tools/resource_explorer.go, internal/mcp/server.go -->
