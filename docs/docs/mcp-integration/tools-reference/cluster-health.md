---
title: cluster_health Tool
description: Get cluster health overview with resource status breakdown and top issues
keywords: [mcp, tools, kubernetes, health, monitoring, status]
---

# cluster_health

Get a comprehensive health overview of your Kubernetes cluster with resource status breakdowns and prioritized issue identification.

## Overview

### Purpose

The `cluster_health` tool provides a high-level assessment of cluster health during a specific time window. It aggregates resource statuses by kind, identifies resources in error or warning states, and highlights the most critical issues sorted by error duration.

### Use Cases

| Scenario | When to Use |
|----------|-------------|
| **Initial Triage** | Start of incident investigation to identify scope of impact |
| **Health Checks** | Regular monitoring or post-deployment verification |
| **Post-Mortem** | Historical health assessment during incident windows |
| **Scoping Analysis** | Determine which namespaces or resource types are affected |

### Typical Users

- **AI Agents**: Initial step in automated incident investigation workflows
- **Operators**: Quick cluster health assessment during incidents
- **SREs**: Regular health checks and monitoring

## Quick Example

### Simple Query

Get cluster health for the last hour:

```json
{
  "start_time": 1702393800,
  "end_time": 1702397400
}
```

**What this does**: Analyzes all resources across all namespaces in the last hour, returning status counts by kind and top 10 issues.

### Typical Use Case

During an incident, narrow down to a specific namespace:

```json
{
  "start_time": 1702393800,
  "end_time": 1702397400,
  "namespace": "production",
  "max_resources": 50
}
```

**Result**: Focused view of production namespace health with top 50 problem resources per status.

## Input Parameters

### Required Parameters

| Parameter | Type | Description | Example |
|-----------|------|-------------|---------|
| `start_time` | integer | Start timestamp (Unix seconds or milliseconds) | `1702393800` or `1702393800000` |
| `end_time` | integer | End timestamp (Unix seconds or milliseconds) | `1702397400` or `1702397400000` |

**Timestamp Format**:
- Accepts both Unix seconds (10 digits) and milliseconds (13 digits)
- Automatically detects and converts milliseconds to seconds
- Must satisfy: `start_time < end_time`

### Optional Parameters

| Parameter | Type | Default | Description | Example |
|-----------|------|---------|-------------|---------|
| `namespace` | string | `""` (all namespaces) | Filter by Kubernetes namespace | `"production"` |
| `max_resources` | integer | `100` | Max resources to list per status (max: 500) | `50` |

**Parameter Notes**:
- **namespace**: If omitted, queries all namespaces cluster-wide
- **max_resources**: Controls size of `error_resources`, `warning_resources` etc. lists. Higher values increase response size.

## Output Structure

### Response Format

```json
{
  "overall_status": "Degraded",
  "total_resources": 245,
  "error_resource_count": 3,
  "warning_resource_count": 12,
  "healthy_resource_count": 230,
  "resources_by_kind": [
    {
      "kind": "Pod",
      "ready": 185,
      "warning": 8,
      "error": 2,
      "terminating": 1,
      "unknown": 0,
      "total_count": 196,
      "error_rate": 0.051,
      "warning_resources": ["production/api-gateway-abc", "production/worker-xyz"],
      "error_resources": ["production/payment-processor-123", "staging/data-import-456"]
    },
    {
      "kind": "Deployment",
      "ready": 42,
      "warning": 3,
      "error": 1,
      "total_count": 46,
      "error_rate": 0.087,
      "error_resources": ["production/payment-api"]
    }
  ],
  "top_issues": [
    {
      "resource_id": "pod-789",
      "kind": "Pod",
      "namespace": "production",
      "name": "payment-processor-abc",
      "current_status": "Error",
      "error_duration_seconds": 3600,
      "error_duration_text": "1h 0m 0s",
      "error_message": "ImagePullBackOff",
      "event_count": 15
    }
  ],
  "aggregation_time_ms": 145
}
```

### Field Descriptions

#### Top-Level Fields

| Field | Type | Description |
|-------|------|-------------|
| `overall_status` | string | Cluster-wide status: `"Healthy"`, `"Degraded"`, `"Critical"` |
| `total_resources` | integer | Total number of resources analyzed |
| `error_resource_count` | integer | Count of resources in Error status |
| `warning_resource_count` | integer | Count of resources in Warning status |
| `healthy_resource_count` | integer | Count of resources in Ready status |
| `resources_by_kind` | array | Status breakdown by resource kind (sorted alphabetically) |
| `top_issues` | array | Top 10 issues sorted by error duration (descending) |
| `aggregation_time_ms` | integer | Query execution time in milliseconds |

#### ResourceStatusCount Object

| Field | Type | Description |
|-------|------|-------------|
| `kind` | string | Resource kind (e.g., "Pod", "Deployment") |
| `ready` | integer | Count of resources in Ready status |
| `warning` | integer | Count of resources in Warning status |
| `error` | integer | Count of resources in Error status |
| `terminating` | integer | Count of resources being terminated |
| `unknown` | integer | Count of resources with Unknown status |
| `total_count` | integer | Sum of all status counts for this kind |
| `error_rate` | float | Ratio of (error + warning) / total_count |
| `warning_resources` | array of strings | List of warning resources (format: `namespace/name`) |
| `error_resources` | array of strings | List of error resources (format: `namespace/name`) |
| `terminating_resources` | array of strings | List of terminating resources |
| `unknown_resources` | array of strings | List of unknown status resources |

**Note**: Resource lists are truncated to `max_resources` parameter (default 100).

#### Issue Object

| Field | Type | Description |
|-------|------|-------------|
| `resource_id` | string | Unique resource identifier |
| `kind` | string | Resource kind |
| `namespace` | string | Kubernetes namespace |
| `name` | string | Resource name |
| `current_status` | string | Current status: "Error", "Warning", "Terminating", "Unknown" |
| `error_duration_seconds` | integer | Time spent in error/warning state (seconds) |
| `error_duration_text` | string | Human-readable duration (e.g., "2h 30m 15s") |
| `error_message` | string | Current error or warning message |
| `event_count` | integer | Number of events associated with this resource |

### Status Values

| Status | Description | Included in Overall Status Calculation |
|--------|-------------|----------------------------------------|
| `Ready` | Resource is healthy and functioning | ✅ Counts toward "Healthy" |
| `Warning` | Resource has non-critical issues | ⚠️ Triggers "Degraded" overall status |
| `Error` | Resource has critical failures | ❌ Triggers "Critical" overall status |
| `Terminating` | Resource is being deleted | ℹ️ Not counted in overall status |
| `Unknown` | Resource status cannot be determined | ℹ️ Not counted in overall status |

## Usage Patterns

### Pattern 1: Cluster-Wide Health Check

**Scenario**: Regular health monitoring across entire cluster

```json
{
  "start_time": 1702393800,
  "end_time": 1702397400
}
```

**Expected Output**: All namespaces, all resource kinds, comprehensive overview

### Pattern 2: Namespace-Specific Investigation

**Scenario**: Incident affecting specific namespace

```json
{
  "start_time": 1702393800,
  "end_time": 1702397400,
  "namespace": "production"
}
```

**Expected Output**: Focused view of production namespace only

### Pattern 3: Recent Health Assessment

**Scenario**: Check current cluster state (last 15 minutes)

```json
{
  "start_time": 1702396500,  // 15 minutes ago
  "end_time": 1702397400     // now
}
```

**Expected Output**: Near real-time cluster health snapshot

### Pattern 4: Historical Analysis

**Scenario**: Post-mortem analysis of incident window

```json
{
  "start_time": 1702300800,  // 3pm yesterday
  "end_time": 1702304400     // 4pm yesterday
}
```

**Expected Output**: Historical health state during known incident

## Real-World Examples

### Example 1: Detecting Pod Issues

**Request**:

```json
{
  "start_time": 1702393800,
  "end_time": 1702397400,
  "namespace": "production",
  "max_resources": 100
}
```

**Response**:

```json
{
  "overall_status": "Critical",
  "total_resources": 187,
  "error_resource_count": 5,
  "warning_resource_count": 8,
  "healthy_resource_count": 174,
  "resources_by_kind": [
    {
      "kind": "Pod",
      "ready": 145,
      "warning": 6,
      "error": 4,
      "terminating": 2,
      "total_count": 157,
      "error_rate": 0.064,
      "warning_resources": [
        "production/api-gateway-7d9f8b-abc",
        "production/worker-queue-45abc-xyz"
      ],
      "error_resources": [
        "production/payment-processor-123",
        "production/payment-processor-456",
        "production/cache-redis-789",
        "production/database-primary-012"
      ],
      "terminating_resources": [
        "production/old-deployment-abc",
        "production/old-deployment-xyz"
      ]
    },
    {
      "kind": "Deployment",
      "ready": 27,
      "warning": 2,
      "error": 1,
      "total_count": 30,
      "error_rate": 0.1,
      "error_resources": ["production/payment-api"]
    }
  ],
  "top_issues": [
    {
      "resource_id": "pod-payment-123",
      "kind": "Pod",
      "namespace": "production",
      "name": "payment-processor-123",
      "current_status": "Error",
      "error_duration_seconds": 1800,
      "error_duration_text": "30m 0s",
      "error_message": "CrashLoopBackOff: container exited with code 1",
      "event_count": 25
    },
    {
      "resource_id": "pod-cache-789",
      "kind": "Pod",
      "namespace": "production",
      "name": "cache-redis-789",
      "current_status": "Error",
      "error_duration_seconds": 900,
      "error_duration_text": "15m 0s",
      "error_message": "Insufficient memory",
      "event_count": 12
    }
  ],
  "aggregation_time_ms": 156
}
```

**Analysis**: 5 critical errors in production, primarily Pods. Payment processor has been failing for 30 minutes.

### Example 2: All-Clear Health Check

**Request**:

```json
{
  "start_time": 1702393800,
  "end_time": 1702397400
}
```

**Response**:

```json
{
  "overall_status": "Healthy",
  "total_resources": 342,
  "error_resource_count": 0,
  "warning_resource_count": 0,
  "healthy_resource_count": 342,
  "resources_by_kind": [
    {
      "kind": "Deployment",
      "ready": 58,
      "warning": 0,
      "error": 0,
      "total_count": 58,
      "error_rate": 0.0
    },
    {
      "kind": "Pod",
      "ready": 234,
      "warning": 0,
      "error": 0,
      "total_count": 234,
      "error_rate": 0.0
    },
    {
      "kind": "Service",
      "ready": 50,
      "warning": 0,
      "error": 0,
      "total_count": 50,
      "error_rate": 0.0
    }
  ],
  "top_issues": [],
  "aggregation_time_ms": 98
}
```

**Analysis**: Cluster is healthy, no issues detected.

## Performance Characteristics

### Execution Time

| Cluster Size | Resources | Typical Latency | Notes |
|--------------|-----------|----------------|--------|
| Small (< 100 resources) | 50-100 | 50-100 ms | Single namespace queries |
| Medium (100-500 resources) | 200-400 | 100-200 ms | Multi-namespace, filtered |
| Large (500-2000 resources) | 1000-1500 | 200-400 ms | Cluster-wide queries |
| Very Large (2000+ resources) | 2000+ | 400-800 ms | Full cluster, wide time ranges |

**Factors Affecting Performance**:
- Time range width (wider = more events to process)
- Number of namespaces queried
- Spectre API response time
- Number of resources with status changes

### Optimization Tips

**✅ Improve Performance**:
1. **Narrow time window**: Query last 1 hour instead of last 24 hours
2. **Filter by namespace**: Reduce scope to specific namespaces
3. **Limit max_resources**: Use lower values (10-50) if you don't need full lists
4. **Cache results**: For dashboards, cache responses for 30-60 seconds

**❌ Avoid**:
- Very wide time ranges (> 7 days) without namespace filtering
- Querying all namespaces when you only need one
- Setting max_resources unnecessarily high (> 200)

### Resource Impact

**Memory**: ~5-10 MB per request (scales with result set size)
**CPU**: Minimal (mostly I/O waiting for Spectre API)
**Network**: Proportional to number of resources returned

## Integration Patterns

### With Other Tools

**Typical Investigation Workflow**:

```
1. cluster_health (this tool)
   ↓ Identifies: 5 Pods in Error state in production

2. resource_changes
   ↓ Discovers: Deployment updated 15 minutes ago

3. investigate (specific Pod)
   ↓ Analyzes: Timeline of Pod failures, root cause evidence
```

**When to Use Each**:
- **cluster_health**: Start here for overview, identify problem areas
- **resource_changes**: Investigate what changed to cause issues
- **investigate**: Deep dive into specific resources identified by cluster_health

### With Prompts

**Used by These Prompts**:

1. **post_mortem_incident_analysis**:
   - Step 2: Calls cluster_health to get incident window overview
   - Uses `overall_status` and `top_issues` to identify scope

2. **live_incident_handling**:
   - Step 2: Calls cluster_health for current state assessment
   - Focuses on `error_resources` and `top_issues` for triage

**Prompt Interpretation**:
- `overall_status = "Critical"` → Prompts investigate top_issues immediately
- `overall_status = "Degraded"` → Prompts ask if investigation needed
- `overall_status = "Healthy"` → Prompts look elsewhere for root cause

## Troubleshooting

### Common Errors

**Error: "start_time must be before end_time"**

**Cause**: Invalid time range

**Solution**:
```json
// ❌ Wrong
{
  "start_time": 1702397400,
  "end_time": 1702393800
}

// ✅ Correct
{
  "start_time": 1702393800,
  "end_time": 1702397400
}
```

**Error: "failed to query timeline: connection refused"**

**Cause**: MCP server cannot reach Spectre API

**Solution**: Check Spectre API is running and accessible from MCP server

### Empty Results

**Symptom**: `total_resources: 0`, empty `resources_by_kind`

**Possible Causes**:

1. **No events in time range**:
   - Spectre hasn't collected events yet
   - Time range outside Spectre retention
   - Solution: Verify Spectre is collecting events, adjust time range

2. **Namespace doesn't exist**:
   - Querying non-existent namespace
   - Solution: Check namespace spelling, omit namespace parameter

3. **Time format issue**:
   - Using milliseconds for both timestamps
   - Solution: Tool auto-converts, but verify timestamps are valid Unix times

### Performance Issues

**Symptom**: `aggregation_time_ms > 1000` (slow queries)

**Causes & Solutions**:

| Cause | Symptom | Solution |
|-------|---------|----------|
| Wide time range | High aggregation_time_ms | Narrow to last 1-6 hours |
| Cluster-wide query | Many resources | Add namespace filter |
| Spectre API slow | Check Spectre logs | Optimize Spectre storage/cache |
| High max_resources | Large response payload | Reduce to 50-100 |

## API Reference

### MCP Protocol Request

**Initialize Session** (first):

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": {
      "name": "my-client",
      "version": "1.0.0"
    }
  }
}
```

**Call cluster_health Tool**:

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "cluster_health",
    "arguments": {
      "start_time": 1702393800,
      "end_time": 1702397400,
      "namespace": "production",
      "max_resources": 100
    }
  }
}
```

### cURL Example

```bash
# Via HTTP transport
curl -X POST http://spectre-mcp:8081/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "cluster_health",
      "arguments": {
        "start_time": 1702393800,
        "end_time": 1702397400,
        "namespace": "production"
      }
    }
  }'
```

### Claude Desktop Example

**Natural Language**:

```
"What's the health status of the production namespace in the last hour?"
```

**Claude's Tool Call** (automatic):

```json
{
  "name": "cluster_health",
  "arguments": {
    "start_time": 1702393800,
    "end_time": 1702397400,
    "namespace": "production"
  }
}
```

**Claude's Response**:

> Based on the cluster health check for production in the last hour:
>
> **Overall Status**: Critical
>
> **Summary**:
> - Total Resources: 187
> - Errors: 5 resources
> - Warnings: 8 resources
> - Healthy: 174 resources
>
> **Top Issues**:
> 1. Pod `payment-processor-123` in Error state for 30 minutes (CrashLoopBackOff)
> 2. Pod `cache-redis-789` in Error state for 15 minutes (Insufficient memory)
>
> **Recommendation**: Investigate the payment-processor first as it has the longest error duration.

## Best Practices

### ✅ Do

- **Start with cluster_health** for any investigation - it provides essential context
- **Use specific time windows** - query the exact incident window when known
- **Filter by namespace** when investigating specific service issues
- **Check overall_status** first to understand severity
- **Review top_issues** to prioritize which resources need investigation
- **Use max_resources wisely** - lower values (10-50) for quick checks, higher (100-200) for comprehensive lists

### ❌ Don't

- **Don't query excessive time ranges** (> 7 days) without namespace filtering
- **Don't ignore aggregation_time_ms** - high values indicate performance issues
- **Don't assume empty results mean no issues** - check Spectre is collecting data
- **Don't set max_resources > 500** - it's capped at 500 anyway
- **Don't use cluster_health alone** - combine with resource_changes and investigate for full picture
- **Don't query all namespaces** if you already know which namespace is affected

## Related Documentation

- [resource_changes Tool](./resource-changes) - Identify what changed during incidents
- [investigate Tool](./investigate) - Deep dive into specific resources
- [Post-Mortem Prompt](../prompts-reference/post-mortem) - Uses cluster_health as step 2
- [Live Incident Prompt](../prompts-reference/live-incident) - Uses cluster_health for triage
- [Getting Started](../getting-started) - Setup and first investigation

<!-- Source: internal/mcp/tools/cluster_health.go, internal/mcp/server.go, README.md lines 84-86 -->
