---
title: resource_timeline Tool
description: Get resource timeline with status segments, events, and transitions for root cause analysis
keywords: [mcp, tools, resource_timeline, timeline, incident, analysis]
---

# resource_timeline Tool

Get resource timeline with status segments, events, and transitions for root cause analysis.

## Overview

The `resource_timeline` tool provides timeline data for Kubernetes resources, including status segments, events, and transitions. It's designed for understanding how resources changed over time.

**Key Capabilities:**
- **Timeline Reconstruction**: Chronological view of status changes and events
- **Status Segment Deduplication**: Adjacent segments with same status/message are merged
- **Multi-Resource Support**: Query multiple resources with wildcard (`*`)
- **Event Correlation**: Link Kubernetes events to status transitions

**When to Use:**
- Deep investigation of a specific resource after identifying it with `cluster_health`
- Building a detailed timeline for post-mortem documentation
- Understanding why a resource transitioned through states
- Correlating events with status changes

**When NOT to Use:**
- Getting semantic field-level diffs (use `resource_timeline_changes` instead)
- Cluster-wide health overview (use `cluster_health` instead)

## Quick Example

### Single Resource Timeline

```json
{
  "resource_kind": "Pod",
  "resource_name": "nginx-7d8b5f9c6b-x7k2p",
  "namespace": "default",
  "start_time": 1702382400,
  "end_time": 1702386000
}
```

Returns detailed timeline and events for the specific Pod.

### Multi-Resource Timeline (Wildcard)

```json
{
  "resource_kind": "Pod",
  "resource_name": "*",
  "namespace": "production",
  "start_time": 1702382400,
  "end_time": 1702386000,
  "max_results": 10
}
```

Returns timelines for up to 10 Pods in `production` namespace.

## Input Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `resource_kind` | string | **Yes** | - | Resource kind (e.g., `Pod`, `Deployment`, `Service`) |
| `resource_name` | string | No | `"*"` | Resource name or `"*"` for all resources of this kind |
| `namespace` | string | No | `""` (all) | Kubernetes namespace to filter by |
| `start_time` | int64 | **Yes** | - | Start of timeline window (Unix timestamp in seconds or milliseconds) |
| `end_time` | int64 | **Yes** | - | End of timeline window (Unix timestamp in seconds or milliseconds) |
| `max_results` | int | No | `20` | Maximum resources to return when using wildcard (max: 100) |

### Resource Name Wildcards

**Specific Resource**:
```json
{"resource_name": "nginx-7d8b5f9c6b-x7k2p"}  // Single resource
```

**All Resources of Kind**:
```json
{"resource_name": "*"}  // All Pods, Deployments, etc.
```

**Empty (treated as wildcard)**:
```json
{"resource_name": ""}  // Equivalent to "*"
```

### Timestamp Format

Both **Unix seconds** and **Unix milliseconds** are supported:

```json
// Unix seconds (recommended)
{"start_time": 1702382400, "end_time": 1702386000}

// Unix milliseconds
{"start_time": 1702382400000, "end_time": 1702386000000}
```

## Output Structure

```json
{
  "timelines": [
    {
      "resource_id": "Pod/default/nginx-7d8b5f9c6b-x7k2p",
      "kind": "Pod",
      "namespace": "default",
      "name": "nginx-7d8b5f9c6b-x7k2p",
      "current_status": "Error",
      "current_message": "Back-off restarting failed container",
      "timeline_start": 1702382400,
      "timeline_end": 1702385800,
      "timeline_start_text": "2024-12-12T10:00:00Z",
      "timeline_end_text": "2024-12-12T10:56:40Z",
      "status_segments": [
        {
          "start_time": 1702382400,
          "end_time": 1702383200,
          "duration": 800,
          "status": "Running",
          "message": "All containers running",
          "start_time_text": "2024-12-12T10:00:00Z",
          "end_time_text": "2024-12-12T10:13:20Z"
        },
        {
          "start_time": 1702383200,
          "end_time": 1702385800,
          "duration": 2600,
          "status": "Error",
          "message": "CrashLoopBackOff",
          "start_time_text": "2024-12-12T10:13:20Z",
          "end_time_text": "2024-12-12T10:56:40Z"
        }
      ],
      "events": [
        {
          "timestamp": 1702383200,
          "reason": "BackOff",
          "message": "Back-off restarting failed container nginx in pod nginx-7d8b5f9c6b-x7k2p",
          "type": "Warning",
          "count": 15,
          "source": "kubelet",
          "first_timestamp": 1702383200,
          "last_timestamp": 1702385800,
          "timestamp_text": "2024-12-12T10:13:20Z",
          "first_timestamp_text": "2024-12-12T10:13:20Z",
          "last_timestamp_text": "2024-12-12T10:56:40Z"
        }
      ],
      "raw_resource_snapshots": [
        {
          "timestamp": 1702383200,
          "status": "Error",
          "message": "CrashLoopBackOff",
          "key_changes": [],
          "timestamp_text": "2024-12-12T10:13:20Z"
        }
      ]
    }
  ],
  "execution_time_ms": 387
}
```

### Top-Level Fields

| Field | Type | Description |
|-------|------|-------------|
| `timelines` | array | List of timeline evidence objects (one per resource) |
| `execution_time_ms` | int64 | Processing time in milliseconds |

### ResourceTimelineEvidence Fields

| Field | Type | Description |
|-------|------|-------------|
| `resource_id` | string | Unique resource identifier (format: `Kind/Namespace/Name`) |
| `kind` | string | Resource kind (e.g., `Pod`, `Deployment`) |
| `namespace` | string | Kubernetes namespace |
| `name` | string | Resource name |
| `current_status` | string | Last known status (`Ready`, `Running`, `Error`, `Warning`, etc.) |
| `current_message` | string | Last known status message |
| `timeline_start` | int64 | First event/status timestamp in timeline window |
| `timeline_end` | int64 | Last event/status timestamp in timeline window |
| `timeline_start_text` | string | Human-readable timeline start (ISO 8601) |
| `timeline_end_text` | string | Human-readable timeline end |
| `status_segments` | array | Chronological status periods (deduplicated) |
| `events` | array | Kubernetes events for this resource |
| `raw_resource_snapshots` | array | Resource snapshots at Error/Warning transitions (optional) |

### SegmentSummary Fields

| Field | Type | Description |
|-------|------|-------------|
| `start_time` | int64 | Unix timestamp when segment started |
| `end_time` | int64 | Unix timestamp when segment ended |
| `duration` | int64 | How long resource stayed in this status (seconds) |
| `status` | string | Status value (`Ready`, `Running`, `Error`, `Warning`, etc.) |
| `message` | string | Status message/reason |
| `start_time_text` | string | Human-readable start time |
| `end_time_text` | string | Human-readable end time |

**Note**: Adjacent segments with the same status and message are automatically merged (deduplicated).

### EventSummary Fields

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | int64 | Unix timestamp of event |
| `reason` | string | Event reason (e.g., `BackOff`, `Pulled`, `Failed`) |
| `message` | string | Event message describing what happened |
| `type` | string | Event type: `Normal` or `Warning` |
| `count` | int32 | Number of times this event occurred |
| `source` | string | Event source component (e.g., `kubelet`, `scheduler`) |
| `first_timestamp` | int64 | First occurrence of this event |
| `last_timestamp` | int64 | Most recent occurrence of this event |

## Status Segment Deduplication

The `resource_timeline` tool automatically merges adjacent status segments that have the same `status` and `message`. This reduces noise and provides a cleaner timeline view.

**Before Deduplication:**
```json
{
  "status_segments": [
    {"start_time": 100, "end_time": 110, "status": "Error", "message": "CrashLoopBackOff"},
    {"start_time": 110, "end_time": 120, "status": "Error", "message": "CrashLoopBackOff"},
    {"start_time": 120, "end_time": 130, "status": "Error", "message": "CrashLoopBackOff"}
  ]
}
```

**After Deduplication:**
```json
{
  "status_segments": [
    {"start_time": 100, "end_time": 130, "status": "Error", "message": "CrashLoopBackOff", "duration": 30}
  ]
}
```

## Usage Patterns

### Pattern 1: Single Resource Deep Dive

**Goal**: Investigate a specific resource identified from `cluster_health`

```json
{
  "resource_kind": "Pod",
  "resource_name": "api-server-85f6c9b8-k4x2p",
  "namespace": "production",
  "start_time": 1702382400,
  "end_time": 1702386000
}
```

### Pattern 2: Multi-Resource Timeline

**Goal**: Get timelines for all resources of a kind in a namespace

```json
{
  "resource_kind": "Pod",
  "resource_name": "*",
  "namespace": "default",
  "start_time": 1702382400,
  "end_time": 1702386000,
  "max_results": 20
}
```

### Pattern 3: Post-Mortem Documentation

**Goal**: Build comprehensive timeline for incident report

```json
{
  "resource_kind": "Deployment",
  "resource_name": "frontend",
  "namespace": "production",
  "start_time": 1702378800,
  "end_time": 1702386000
}
```

## Best Practices

### Do
- **Use after cluster_health** - Identify targets first, then get detailed timelines
- **Check status_segments** - Understand how long resources stayed in each state
- **Correlate events with segments** - Match event timestamps to status transitions
- **Use wildcards judiciously** - Set reasonable `max_results` limit
- **Review timeline_start/end** - Ensure timeline window covers incident

### Don't
- **Don't query without context** - Use cluster_health first to identify resources
- **Don't use wildcard without limits** - Always set `max_results` < 50
- **Don't use very wide time windows** - 1-6 hours is optimal for detailed analysis
- **Don't use for semantic diffs** - Use `resource_timeline_changes` for field-level changes

## Related Documentation

- [cluster_health Tool](./cluster-health.md) - Find unhealthy resources to investigate
- [resource_timeline_changes Tool](./resource-changes.md) - Get semantic field-level diffs
- [Post-Mortem Prompt](../prompts-reference/post-mortem.md) - Uses resource_timeline in workflow

<!-- Source: internal/mcp/tools/resource_timeline.go -->
