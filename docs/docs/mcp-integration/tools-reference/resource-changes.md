---
title: resource_changes Tool
description: Identify and analyze resource changes with impact scoring
keywords: [mcp, tools, resource_changes, incident, analysis, correlation]
---

# resource_changes Tool

Analyze Kubernetes resource changes over time with automatic impact scoring and change correlation.

## Overview

The `resource_changes` tool identifies which resources changed during a time window, calculates impact scores, and provides detailed change summaries for incident correlation and root cause analysis.

**Key Capabilities:**
- **Automatic Impact Scoring**: 0-1.0 scale based on error events, warnings, and status transitions
- **Change Correlation**: Group related changes by resource for incident analysis
- **Selective Filtering**: Focus on specific resource kinds and impact thresholds
- **Performance Optimized**: Quickly scans large event sets with efficient aggregation

**When to Use:**
- Correlating multiple changes during an incident
- Identifying high-impact changes in a time window
- Post-mortem analysis of what changed
- Maintenance window verification
- Change impact assessment

**When NOT to Use:**
- Deep investigation of a single resource (use `investigate` instead)
- Cluster-wide health overview (use `cluster_health` instead)
- Browsing resources (use `resource_explorer` instead)

## Quick Example

### Minimal Usage

```json
{
  "start_time": 1702382400,
  "end_time": 1702386000
}
```

Returns all resource changes in the 1-hour window with impact scores.

### Typical Usage

```json
{
  "start_time": 1702382400,
  "end_time": 1702386000,
  "kinds": "Pod,Deployment",
  "impact_threshold": 0.3,
  "max_resources": 20
}
```

Returns top 20 Pods and Deployments with impact score ≥ 0.3.

## Input Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `start_time` | int64 | **Yes** | - | Start of time window (Unix timestamp in seconds or milliseconds) |
| `end_time` | int64 | **Yes** | - | End of time window (Unix timestamp in seconds or milliseconds) |
| `kinds` | string | No | `""` (all) | Comma-separated resource kinds to include (e.g., `"Pod,Deployment"`) |
| `impact_threshold` | float64 | No | `0.0` | Minimum impact score (0-1.0) to include in results |
| `max_resources` | int | No | `50` | Maximum number of resources to return (sorted by impact score descending) |

### Timestamp Format

Both **Unix seconds** and **Unix milliseconds** are supported:

```json
// Unix seconds (recommended)
{"start_time": 1702382400, "end_time": 1702386000}

// Unix milliseconds
{"start_time": 1702382400000, "end_time": 1702386000000}
```

**Tip**: Use `date +%s` for Unix seconds or `date +%s%3N` for milliseconds.

### Impact Threshold Guidelines

| Threshold | Use Case | Typical Results |
|-----------|----------|-----------------|
| `0.0` | All changes (default) | High volume, includes routine updates |
| `0.2` | Moderate changes | Filters out routine updates, keeps warnings |
| `0.3` | High-impact changes | Errors, status transitions, significant events |
| `0.5` | Critical changes only | Multiple errors or severe status transitions |
| `0.7` | Extreme severity | Rare, very high event counts or cascading failures |

## Output Structure

```json
{
  "changes": [
    {
      "resource_id": "Pod/default/nginx-7d8b5f9c6b-x7k2p",
      "kind": "Pod",
      "namespace": "default",
      "name": "nginx-7d8b5f9c6b-x7k2p",
      "impact_score": 0.75,
      "changes": [
        {
          "timestamp": 1702384200,
          "event_type": "UPDATE",
          "change_type": "status.phase",
          "old_value": "Running",
          "new_value": "Failed"
        }
      ],
      "change_count": 3,
      "event_count": 12,
      "error_events": 2,
      "warning_events": 1,
      "status_transitions": [
        {
          "timestamp": 1702384200,
          "from_status": "Running",
          "to_status": "Failed"
        }
      ]
    }
  ],
  "total_resources": 47,
  "filtered_resources": 8,
  "time_range_ms": 3600000,
  "aggregation_time_ms": 245
}
```

### Top-Level Fields

| Field | Type | Description |
|-------|------|-------------|
| `changes` | array | List of resource change summaries (sorted by impact score descending) |
| `total_resources` | int | Total number of resources with events in time window |
| `filtered_resources` | int | Number of resources matching filters (kinds, threshold) |
| `time_range_ms` | int64 | Query time range duration in milliseconds |
| `aggregation_time_ms` | int64 | Processing time in milliseconds |

### ResourceChangeSummary Fields

| Field | Type | Description |
|-------|------|-------------|
| `resource_id` | string | Unique resource identifier (format: `Kind/Namespace/Name`) |
| `kind` | string | Resource kind (e.g., `Pod`, `Deployment`) |
| `namespace` | string | Kubernetes namespace |
| `name` | string | Resource name |
| `impact_score` | float64 | Calculated impact score (0-1.0) - see [Impact Score Algorithm](#impact-score-algorithm) |
| `changes` | array | List of specific changes detected (field modifications) |
| `change_count` | int | Number of distinct changes (unique field modifications) |
| `event_count` | int | Total number of events for this resource |
| `error_events` | int | Number of events with severity "Error" |
| `warning_events` | int | Number of events with severity "Warning" |
| `status_transitions` | array | State transitions (e.g., Running → Failed) |

### ChangeDetail Fields

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | int64 | Unix timestamp when change occurred |
| `event_type` | string | Event type: `CREATE`, `UPDATE`, `DELETE` |
| `change_type` | string | Field path that changed (e.g., `status.phase`, `spec.replicas`) |
| `old_value` | string | Previous value (JSON-encoded) |
| `new_value` | string | New value (JSON-encoded) |

### StatusTransition Fields

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | int64 | Unix timestamp of transition |
| `from_status` | string | Previous status (e.g., `Running`, `Pending`) |
| `to_status` | string | New status (e.g., `Failed`, `Error`) |

## Impact Score Algorithm

Impact scores range from **0.0 (no impact)** to **1.0 (maximum impact)** and are calculated based on:

### Scoring Components

```
Base Score = 0.0

+ 0.30  if resource has error events (error_events > 0)
+ 0.15  if resource has warning events (warning_events > 0)
+ 0.30  for each status transition to "Error"
+ 0.15  for each status transition to "Warning"
+ 0.10  if event count > 10 (high activity)

Final Score = min(Base Score, 1.0)
```

### Example Calculations

**Example 1: Pod CrashLoopBackOff**
```
Error events: 3 → +0.30
Warning events: 0 → +0.00
Status transition: Running → Error → +0.30
Event count: 15 → +0.10
-------------------------------
Impact Score: 0.70
```

**Example 2: Deployment Scale Up**
```
Error events: 0 → +0.00
Warning events: 0 → +0.00
Status transitions: none → +0.00
Event count: 5 → +0.00
-------------------------------
Impact Score: 0.00 (routine change)
```

**Example 3: Service with Warnings**
```
Error events: 0 → +0.00
Warning events: 2 → +0.15
Status transitions: none → +0.00
Event count: 8 → +0.00
-------------------------------
Impact Score: 0.15
```

**Example 4: Multiple Transitions**
```
Error events: 1 → +0.30
Warning events: 1 → +0.15
Status transitions: Pending → Warning (2×) → +0.30
Event count: 12 → +0.10
-------------------------------
Base Score: 0.85 → Capped at 1.0
Impact Score: 0.85
```

### Impact Score Interpretation

| Score Range | Severity | Typical Cause | Action |
|-------------|----------|---------------|--------|
| 0.0 - 0.1 | Routine | Normal updates, scale events | Monitor |
| 0.1 - 0.3 | Low Impact | Warnings, minor issues | Review if clustered |
| 0.3 - 0.5 | Moderate | Errors, status transitions | Investigate |
| 0.5 - 0.7 | High Impact | Multiple errors, significant transitions | Urgent investigation |
| 0.7 - 1.0 | Critical | Cascading failures, extreme event counts | Immediate action |

## Usage Patterns

### Pattern 1: Post-Incident Analysis

**Goal**: Identify all high-impact changes during an incident

```json
{
  "start_time": 1702382400,
  "end_time": 1702386000,
  "impact_threshold": 0.3,
  "max_resources": 30
}
```

**Use Case**: "What changed during the incident at 10:00 AM that could have caused the outage?"

### Pattern 2: Focused Kind Analysis

**Goal**: Analyze changes to specific resource types

```json
{
  "start_time": 1702382400,
  "end_time": 1702386000,
  "kinds": "Deployment,StatefulSet,DaemonSet",
  "impact_threshold": 0.2
}
```

**Use Case**: "Were there any controller changes during the maintenance window?"

### Pattern 3: Change Volume Assessment

**Goal**: Get all changes (no filtering) to assess overall activity

```json
{
  "start_time": 1702382400,
  "end_time": 1702386000,
  "impact_threshold": 0.0,
  "max_resources": 100
}
```

**Use Case**: "How many resources changed in the last hour?"

### Pattern 4: Critical Changes Only

**Goal**: Find only the most severe changes

```json
{
  "start_time": 1702378800,
  "end_time": 1702386000,
  "impact_threshold": 0.5,
  "max_resources": 10
}
```

**Use Case**: "What were the top 10 most impactful changes in the last 2 hours?"

### Pattern 5: Namespace-Scoped Analysis

**Goal**: Analyze changes in a specific namespace

```json
{
  "start_time": 1702382400,
  "end_time": 1702386000,
  "kinds": "Pod",
  "impact_threshold": 0.2
}
```

Then filter results by namespace in post-processing (namespace is included in output).

**Note**: MCP tool does not support namespace filtering directly. Use Spectre API `/api/v1/query?namespace=default` for namespace filtering.

## Real-World Examples

### Example 1: Deployment Rollout Gone Wrong

**Scenario**: A deployment update caused pods to crash

**Request**:
```json
{
  "start_time": 1702382400,
  "end_time": 1702386000,
  "kinds": "Deployment,Pod",
  "impact_threshold": 0.3,
  "max_resources": 20
}
```

**Response** (abbreviated):
```json
{
  "changes": [
    {
      "resource_id": "Pod/production/api-server-85f6c9b8-k4x2p",
      "kind": "Pod",
      "namespace": "production",
      "name": "api-server-85f6c9b8-k4x2p",
      "impact_score": 0.75,
      "changes": [
        {
          "timestamp": 1702384200,
          "event_type": "UPDATE",
          "change_type": "status.phase",
          "old_value": "Running",
          "new_value": "Failed"
        },
        {
          "timestamp": 1702384150,
          "event_type": "UPDATE",
          "change_type": "status.containerStatuses[0].restartCount",
          "old_value": "0",
          "new_value": "3"
        }
      ],
      "change_count": 5,
      "event_count": 18,
      "error_events": 3,
      "warning_events": 2,
      "status_transitions": [
        {
          "timestamp": 1702384200,
          "from_status": "Running",
          "to_status": "Failed"
        }
      ]
    },
    {
      "resource_id": "Deployment/production/api-server",
      "kind": "Deployment",
      "namespace": "production",
      "name": "api-server",
      "impact_score": 0.45,
      "changes": [
        {
          "timestamp": 1702384000,
          "event_type": "UPDATE",
          "change_type": "spec.template.spec.containers[0].image",
          "old_value": "api-server:v1.2.0",
          "new_value": "api-server:v1.3.0"
        }
      ],
      "change_count": 2,
      "event_count": 8,
      "error_events": 1,
      "warning_events": 0,
      "status_transitions": []
    }
  ],
  "total_resources": 47,
  "filtered_resources": 8,
  "time_range_ms": 3600000,
  "aggregation_time_ms": 187
}
```

**Analysis**:
- Deployment image change (v1.2.0 → v1.3.0) correlated with pod failures
- Pod impact score 0.75 (high) due to errors and status transition
- Deployment impact score 0.45 due to error events

### Example 2: Maintenance Window Verification

**Scenario**: Verify no unexpected changes during maintenance

**Request**:
```json
{
  "start_time": 1702378800,
  "end_time": 1702382400,
  "impact_threshold": 0.2,
  "max_resources": 50
}
```

**Response**:
```json
{
  "changes": [
    {
      "resource_id": "Node/worker-node-3",
      "kind": "Node",
      "namespace": "",
      "name": "worker-node-3",
      "impact_score": 0.25,
      "changes": [
        {
          "timestamp": 1702380000,
          "event_type": "UPDATE",
          "change_type": "status.conditions[0].status",
          "old_value": "True",
          "new_value": "Unknown"
        }
      ],
      "change_count": 1,
      "event_count": 4,
      "error_events": 0,
      "warning_events": 1,
      "status_transitions": []
    }
  ],
  "total_resources": 12,
  "filtered_resources": 1,
  "time_range_ms": 3600000,
  "aggregation_time_ms": 98
}
```

**Analysis**:
- Only 1 resource exceeded threshold (0.2)
- Node condition change was expected (planned maintenance)
- Verification successful: no unexpected high-impact changes

### Example 3: ConfigMap Change Correlation

**Scenario**: Find what changed around a configuration update

**Request**:
```json
{
  "start_time": 1702382100,
  "end_time": 1702382700,
  "kinds": "ConfigMap,Deployment,Pod",
  "impact_threshold": 0.0,
  "max_resources": 30
}
```

**Response** (abbreviated):
```json
{
  "changes": [
    {
      "resource_id": "ConfigMap/default/app-config",
      "kind": "ConfigMap",
      "namespace": "default",
      "name": "app-config",
      "impact_score": 0.1,
      "changes": [
        {
          "timestamp": 1702382200,
          "event_type": "UPDATE",
          "change_type": "data.database_url",
          "old_value": "postgres://old-db:5432",
          "new_value": "postgres://new-db:5432"
        }
      ],
      "change_count": 1,
      "event_count": 2,
      "error_events": 0,
      "warning_events": 0,
      "status_transitions": []
    },
    {
      "resource_id": "Pod/default/app-7d9f8c5b-z9k3p",
      "kind": "Pod",
      "namespace": "default",
      "name": "app-7d9f8c5b-z9k3p",
      "impact_score": 0.0,
      "changes": [
        {
          "timestamp": 1702382350,
          "event_type": "UPDATE",
          "change_type": "metadata.annotations.config-hash",
          "old_value": "abc123",
          "new_value": "def456"
        }
      ],
      "change_count": 1,
      "event_count": 3,
      "error_events": 0,
      "warning_events": 0,
      "status_transitions": []
    }
  ],
  "total_resources": 8,
  "filtered_resources": 8,
  "time_range_ms": 600000,
  "aggregation_time_ms": 42
}
```

**Analysis**:
- ConfigMap change at 10:03:20
- Pod config hash updated 2.5 minutes later (config propagation)
- Low impact scores indicate routine configuration update

## Performance Characteristics

### Execution Time

| Time Range | Event Count | Avg Execution Time | P95 Execution Time |
|------------|-------------|--------------------|--------------------|
| 1 hour | ~1,000 | 50-100 ms | 150 ms |
| 6 hours | ~6,000 | 150-250 ms | 400 ms |
| 24 hours | ~25,000 | 500-800 ms | 1,200 ms |
| 7 days | ~175,000 | 3-5 seconds | 8 seconds |

**Note**: Execution time depends on:
- Event volume in time window
- Number of unique resources
- Complexity of change detection (field-level diffs)
- Cache hit rate (block cache)

### Optimization Tips

**1. Limit Time Range**
```json
// Slower: 7-day window
{"start_time": 1702209600, "end_time": 1702814400}

// Faster: 1-hour window
{"start_time": 1702382400, "end_time": 1702386000}
```

**2. Filter by Kind**
```json
// Slower: All kinds
{"kinds": ""}

// Faster: Specific kinds
{"kinds": "Pod,Deployment"}
```

**3. Use Impact Threshold**
```json
// Slower: All changes (more processing)
{"impact_threshold": 0.0}

// Faster: High-impact only (filtered early)
{"impact_threshold": 0.3}
```

**4. Reduce Max Resources**
```json
// Slower: Return 100 resources
{"max_resources": 100}

// Faster: Return top 10 resources
{"max_resources": 10}
```

### Memory Usage

**Estimated Memory:**
```
Memory = event_count × 2 KB (average event size) + overhead

1 hour (~1,000 events): ~2 MB
24 hours (~25,000 events): ~50 MB
7 days (~175,000 events): ~350 MB
```

**Note**: Tool processes events in streaming fashion; memory usage is bounded by result set size.

## Integration Patterns

### Pattern 1: Two-Step Investigation

**Step 1**: Find high-impact changes with `resource_changes`
```json
{
  "start_time": 1702382400,
  "end_time": 1702386000,
  "impact_threshold": 0.5,
  "max_resources": 10
}
```

**Step 2**: Investigate top resource with `investigate`
```json
{
  "resource_kind": "Pod",
  "resource_namespace": "default",
  "resource_name": "nginx-7d8b5f9c6b-x7k2p",
  "start_time": 1702382400,
  "end_time": 1702386000
}
```

### Pattern 2: Cluster Health + Resource Changes

**Step 1**: Get cluster overview with `cluster_health`
```json
{
  "start_time": 1702382400,
  "end_time": 1702386000
}
```

**Step 2**: Find what changed in unhealthy kinds
```json
{
  "start_time": 1702382400,
  "end_time": 1702386000,
  "kinds": "Pod,Deployment",
  "impact_threshold": 0.3
}
```

### Pattern 3: Post-Mortem Prompt

The `post_mortem_incident_analysis` prompt uses `resource_changes` internally:

**Natural Language** (Claude Desktop):
```
Analyze incident at 10:00 AM on December 12, 2024
```

**Behind the scenes**:
1. Calls `cluster_health` for overview
2. **Calls `resource_changes`** to identify impactful changes
3. Calls `investigate` for top resources
4. Generates comprehensive RCA report

### Pattern 4: Live Incident Prompt

The `live_incident_handling` prompt uses `resource_changes` for correlation:

**Natural Language** (Claude Desktop):
```
Help me troubleshoot pods failing in production namespace
```

**Behind the scenes**:
1. Calls `cluster_health` with `namespace=production`
2. **Calls `resource_changes`** to find recent changes
3. Provides immediate mitigation guidance

## Troubleshooting

### Empty Results (`changes: []`)

**Symptom**: Tool returns empty array

**Common Causes:**

**1. No events in time range**
```json
{
  "changes": [],
  "total_resources": 0,
  "filtered_resources": 0
}
```
**Solution**: Expand time range or verify Spectre is receiving events

**2. Impact threshold too high**
```json
// Request
{"impact_threshold": 0.8}

// Response
{"total_resources": 47, "filtered_resources": 0}
```
**Solution**: Lower threshold (try 0.0 to see all changes)

**3. Kind filter too restrictive**
```json
// Request
{"kinds": "StatefulSet"}

// Response
{"total_resources": 47, "filtered_resources": 0}
```
**Solution**: Expand kinds or remove filter

### Low Impact Scores

**Symptom**: All resources have impact_score < 0.2

**Possible Causes:**
- Normal cluster operations (expected)
- No errors or warnings in time window
- Routine updates (scale, annotations, labels)

**Solution**: This may be expected. Use `impact_threshold: 0.0` to see all changes.

### High Aggregation Time

**Symptom**: `aggregation_time_ms > 5000` (5 seconds)

**Causes:**
- Very large time range (7+ days)
- High event volume (100k+ events)
- Cold cache (first query after restart)

**Solutions:**
1. Reduce time range
2. Filter by specific kinds
3. Increase `--cache-max-mb` in Spectre server
4. Add more memory to cache blocks

### Incorrect Impact Scores

**Symptom**: Resource has high impact but no errors visible

**Cause**: Impact score includes:
- Error events (may not be in `changes` array)
- Warning events
- Status transitions
- High event count (>10)

**Solution**: Check all fields:
```json
{
  "impact_score": 0.55,
  "error_events": 2,        // ← Check this
  "warning_events": 1,      // ← And this
  "status_transitions": [   // ← And this
    {"from_status": "Running", "to_status": "Error"}
  ],
  "event_count": 15         // ← And this (>10 adds 0.1)
}
```

### MCP Connection Errors

**Symptom**: Tool call fails with connection error

**Solutions**:

**1. Verify MCP server is running**
```bash
kubectl get pods -l app=spectre
kubectl logs spectre-server-0 -c mcp
```

**2. Check MCP endpoint**
```bash
curl -X POST http://localhost:8081/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/list","params":{},"id":1}'
```

**3. Verify Spectre API connectivity**
```bash
curl http://localhost:8080/api/v1/health
```

## API Reference

### MCP Protocol Request

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "resource_changes",
    "arguments": {
      "start_time": 1702382400,
      "end_time": 1702386000,
      "kinds": "Pod,Deployment",
      "impact_threshold": 0.3,
      "max_resources": 20
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
        "text": "{\"changes\":[...],\"total_resources\":47,\"filtered_resources\":8}"
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
      "name": "resource_changes",
      "arguments": {
        "start_time": 1702382400,
        "end_time": 1702386000,
        "impact_threshold": 0.3
      }
    },
    "id": 1
  }'
```

### Claude Desktop Usage

**Natural Language** (leverages prompts):
```
Show me high-impact changes in the last hour
```

**Direct Tool Call**:
```
Use the resource_changes tool with:
- start_time: 1702382400
- end_time: 1702386000
- impact_threshold: 0.3
```

**Note**: Claude Desktop automatically formats MCP requests; no manual JSON required.

## Best Practices

### ✅ Do

- **Use impact thresholds** - Filter noise with `impact_threshold: 0.3` for incident analysis
- **Limit time ranges** - Query 1-6 hours for fast results
- **Filter by kind** - Focus on relevant resource types with `kinds` parameter
- **Set max_resources** - Use `max_resources: 10-20` for actionable results
- **Combine with investigate** - Use resource_changes to find targets, then investigate deeply
- **Interpret impact scores contextually** - Score 0.3 for Deployment ≠ score 0.3 for Pod
- **Check total_resources vs filtered_resources** - Verify filtering isn't too aggressive
- **Use for correlation** - Identify temporal relationships between changes

### ❌ Don't

- **Don't query >7 days** - Execution time becomes slow (3-8 seconds)
- **Don't ignore impact_threshold** - Unfiltered results can be overwhelming
- **Don't over-interpret scores** - Impact score is heuristic, not absolute truth
- **Don't rely solely on changes array** - Check error_events, warning_events, status_transitions
- **Don't use for single resource investigation** - Use `investigate` tool instead
- **Don't assume namespace filtering** - Tool doesn't filter by namespace; use Spectre API query
- **Don't ignore aggregation_time_ms** - If >1s, consider optimizing query
- **Don't use for real-time monitoring** - Tool analyzes historical data, not live state

## Related Documentation

- [cluster_health Tool](./cluster-health.md) - Cluster-wide health overview
- [investigate Tool](./investigate.md) - Deep investigation of specific resources
- [resource_explorer Tool](./resource-explorer.md) - Browse and discover resources
- [Post-Mortem Prompt](../prompts-reference/post-mortem.md) - Uses resource_changes in workflow
- [Live Incident Prompt](../prompts-reference/live-incident.md) - Uses resource_changes for correlation
- [MCP Configuration](../../configuration/mcp-configuration.md) - MCP server deployment

<!-- Source: internal/mcp/tools/resource_changes.go, internal/mcp/server.go, README.md lines 88-126 -->
