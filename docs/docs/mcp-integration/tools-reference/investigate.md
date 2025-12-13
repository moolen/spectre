---
title: investigate Tool
description: Deep resource investigation with timeline reconstruction and RCA prompts
keywords: [mcp, tools, investigate, timeline, rca, incident, analysis]
---

# investigate Tool

Deep investigation of specific Kubernetes resources with timeline reconstruction, status transitions, and AI-generated root cause analysis prompts.

## Overview

The `investigate` tool provides detailed forensic analysis of individual resources or resource groups, reconstructing what happened over time and generating investigation prompts to guide root cause analysis.

**Key Capabilities:**
- **Timeline Reconstruction**: Chronological view of status changes and events
- **Status Segment Analysis**: Understand how long resources stayed in each state
- **AI-Generated RCA Prompts**: Contextual investigation questions for LLMs
- **Investigation Types**: Incident (live), post-mortem (historical), or auto-detection
- **Multi-Resource Support**: Investigate multiple resources with wildcard (`*`)
- **Event Correlation**: Link Kubernetes events to status transitions

**When to Use:**
- Deep investigation of a specific resource after identifying it with `cluster_health` or `resource_changes`
- Building a detailed timeline for post-mortem documentation
- Live incident analysis with RCA prompts
- Understanding why a resource transitioned through states
- Correlating events with status changes

**When NOT to Use:**
- Finding which resources changed (use `resource_changes` instead)
- Cluster-wide health overview (use `cluster_health` instead)
- Browsing all resources (use `resource_explorer` instead)

## Quick Example

### Single Resource Investigation

```json
{
  "resource_kind": "Pod",
  "resource_name": "nginx-7d8b5f9c6b-x7k2p",
  "namespace": "default",
  "start_time": 1702382400,
  "end_time": 1702386000
}
```

Returns detailed timeline, events, and investigation prompts for the specific Pod.

### Multi-Resource Investigation (Wildcard)

```json
{
  "resource_kind": "Pod",
  "resource_name": "*",
  "namespace": "production",
  "start_time": 1702382400,
  "end_time": 1702386000,
  "investigation_type": "incident",
  "max_investigations": 10
}
```

Returns top 10 Pods in `production` namespace with incident-focused prompts.

### Post-Mortem Investigation

```json
{
  "resource_kind": "Deployment",
  "resource_name": "api-server",
  "namespace": "production",
  "start_time": 1702378800,
  "end_time": 1702386000,
  "investigation_type": "post-mortem"
}
```

Returns historical analysis with post-mortem-focused RCA prompts.

## Input Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `resource_kind` | string | **Yes** | - | Resource kind (e.g., `Pod`, `Deployment`, `Service`) |
| `resource_name` | string | No | `"*"` | Resource name or `"*"` for all resources of this kind |
| `namespace` | string | No | `""` (all) | Kubernetes namespace to filter by |
| `start_time` | int64 | **Yes** | - | Start of investigation window (Unix timestamp in seconds or milliseconds) |
| `end_time` | int64 | **Yes** | - | End of investigation window (Unix timestamp in seconds or milliseconds) |
| `investigation_type` | string | No | `"auto"` | Investigation type: `"incident"`, `"post-mortem"`, or `"auto"` |
| `max_investigations` | int | No | `20` | Maximum resources to investigate when using wildcard (max: 100) |

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

### Investigation Types

| Type | Use Case | Prompt Focus | Auto-Detection |
|------|----------|--------------|----------------|
| `incident` | Live incident troubleshooting | Immediate mitigation, error root cause | Current status = Error |
| `post-mortem` | Historical incident analysis | Timeline, patterns, prevention | Current status ≠ Error |
| `auto` | Automatic type selection | Contextual based on status | Default behavior |

**Auto-Detection Logic**:
```
if current_status == "Error":
    investigation_type = "incident"
else:
    investigation_type = "post-mortem"
```

### Timestamp Format

Both **Unix seconds** and **Unix milliseconds** are supported:

```json
// Unix seconds (recommended)
{"start_time": 1702382400, "end_time": 1702386000}

// Unix milliseconds
{"start_time": 1702382400000, "end_time": 1702386000000}
```

**Tip**: Use `date +%s` for Unix seconds or `date +%s%3N` for milliseconds.

## Output Structure

```json
{
  "investigations": [
    {
      "resource_id": "Pod/default/nginx-7d8b5f9c6b-x7k2p",
      "kind": "Pod",
      "namespace": "default",
      "name": "nginx-7d8b5f9c6b-x7k2p",
      "current_status": "Error",
      "current_message": "Back-off restarting failed container",
      "timeline_start": 1702382400,
      "timeline_end": 1702385800,
      "timeline_start_text": "2024-12-12 10:00:00 UTC",
      "timeline_end_text": "2024-12-12 10:56:40 UTC",
      "status_segments": [
        {
          "start_time": 1702382400,
          "end_time": 1702383200,
          "duration_seconds": 800,
          "status": "Running",
          "message": "All containers running",
          "start_time_text": "2024-12-12 10:00:00 UTC",
          "end_time_text": "2024-12-12 10:13:20 UTC"
        },
        {
          "start_time": 1702383200,
          "end_time": 1702385800,
          "duration_seconds": 2600,
          "status": "Error",
          "message": "CrashLoopBackOff",
          "start_time_text": "2024-12-12 10:13:20 UTC",
          "end_time_text": "2024-12-12 10:56:40 UTC"
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
          "timestamp_text": "2024-12-12 10:13:20 UTC",
          "first_timestamp_text": "2024-12-12 10:13:20 UTC",
          "last_timestamp_text": "2024-12-12 10:56:40 UTC"
        }
      ],
      "investigation_prompts": [
        "Analyze the status transitions for Pod/nginx-7d8b5f9c6b-x7k2p. What caused the transition from Running to Error?",
        "The Pod nginx-7d8b5f9c6b-x7k2p is currently in Error state since 2600 seconds. What are the immediate mitigation steps?",
        "Based on the events, what is the root cause of the current error in nginx-7d8b5f9c6b-x7k2p?",
        "There are 1 error events. Summarize the error pattern and suggest preventive measures.",
        "Interpret the current message: 'CrashLoopBackOff'. What does it mean?"
      ],
      "raw_resource_snapshots": [
        {
          "timestamp": 1702383200,
          "status": "Error",
          "message": "CrashLoopBackOff",
          "key_changes": [],
          "timestamp_text": "2024-12-12 10:13:20 UTC"
        }
      ]
    }
  ],
  "investigation_time_ms": 387
}
```

### Top-Level Fields

| Field | Type | Description |
|-------|------|-------------|
| `investigations` | array | List of investigation evidence objects (one per resource) |
| `investigation_time_ms` | int64 | Processing time in milliseconds |

### InvestigationEvidence Fields

| Field | Type | Description |
|-------|------|-------------|
| `resource_id` | string | Unique resource identifier (format: `Kind/Namespace/Name`) |
| `kind` | string | Resource kind (e.g., `Pod`, `Deployment`) |
| `namespace` | string | Kubernetes namespace |
| `name` | string | Resource name |
| `current_status` | string | Last known status (`Ready`, `Running`, `Error`, `Warning`, etc.) |
| `current_message` | string | Last known status message |
| `timeline_start` | int64 | First event/status timestamp in investigation window |
| `timeline_end` | int64 | Last event/status timestamp in investigation window |
| `timeline_start_text` | string | Human-readable timeline start (e.g., "2024-12-12 10:00:00 UTC") |
| `timeline_end_text` | string | Human-readable timeline end |
| `status_segments` | array | Chronological status periods (see [SegmentSummary](#segmentsummary-fields)) |
| `events` | array | Kubernetes events for this resource (see [EventSummary](#eventsummary-fields)) |
| `investigation_prompts` | array | AI-generated RCA prompts for LLMs |
| `raw_resource_snapshots` | array | Resource snapshots at Error/Warning transitions (optional) |

### SegmentSummary Fields

| Field | Type | Description |
|-------|------|-------------|
| `start_time` | int64 | Unix timestamp when segment started |
| `end_time` | int64 | Unix timestamp when segment ended |
| `duration_seconds` | int64 | How long resource stayed in this status (seconds) |
| `status` | string | Status value (`Ready`, `Running`, `Error`, `Warning`, etc.) |
| `message` | string | Status message/reason |
| `start_time_text` | string | Human-readable start time |
| `end_time_text` | string | Human-readable end time |

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
| `timestamp_text` | string | Human-readable timestamp |
| `first_timestamp_text` | string | Human-readable first timestamp |
| `last_timestamp_text` | string | Human-readable last timestamp |

### ResourceSnapshot Fields

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | int64 | Unix timestamp of snapshot |
| `status` | string | Status at this point (`Error`, `Warning`) |
| `message` | string | Status message |
| `key_changes` | array | Important changes at this transition (future enhancement) |
| `timestamp_text` | string | Human-readable timestamp |

**Note**: Snapshots are only included for Error and Warning transitions to reduce token usage.

## Investigation Prompts

The `investigation_prompts` field contains AI-generated questions designed to guide LLM-based root cause analysis.

### Common Prompts (All Types)

```
"Analyze the status transitions for {Kind}/{Name}. What caused the transition from {PreviousStatus} to {CurrentStatus}?"
```

### Incident-Specific Prompts

When `investigation_type = "incident"` or auto-detected for resources in Error state:

```
"The {Kind} {Name} is currently in Error state since {Duration} seconds. What are the immediate mitigation steps?"

"Based on the events, what is the root cause of the current error in {Name}?"

"There are {Count} error events. Summarize the error pattern and suggest preventive measures."

"The {Kind} {Name} is in Warning state. What should we monitor to prevent escalation to Error?"
```

### Post-Mortem-Specific Prompts

When `investigation_type = "post-mortem"` or auto-detected for non-Error resources:

```
"Create a timeline of what happened to {Kind}/{Name} during this period."

"What were the contributing factors that led to the status changes in {Name}?"

"How long did {Name} remain in each problematic state? Suggest improvements."

"Document the {Count} error events and identify any patterns that could be prevented."
```

### Contextual Prompts

If `current_message` is present:

```
"Interpret the current message: '{Message}'. What does it mean?"
```

### Example Prompt Set

For a Pod in CrashLoopBackOff with incident type:

```json
{
  "investigation_prompts": [
    "Analyze the status transitions for Pod/nginx-7d8b5f9c6b-x7k2p. What caused the transition from Running to Error?",
    "The Pod nginx-7d8b5f9c6b-x7k2p is currently in Error state since 2600 seconds. What are the immediate mitigation steps?",
    "Based on the events, what is the root cause of the current error in nginx-7d8b5f9c6b-x7k2p?",
    "There are 3 error events. Summarize the error pattern and suggest preventive measures.",
    "Interpret the current message: 'CrashLoopBackOff'. What does it mean?"
  ]
}
```

## Usage Patterns

### Pattern 1: Single Resource Deep Dive

**Goal**: Investigate a specific resource identified from `resource_changes`

```json
{
  "resource_kind": "Pod",
  "resource_name": "api-server-85f6c9b8-k4x2p",
  "namespace": "production",
  "start_time": 1702382400,
  "end_time": 1702386000,
  "investigation_type": "incident"
}
```

**Use Case**: "Show me exactly what happened to this failing Pod"

### Pattern 2: Multi-Resource Investigation

**Goal**: Investigate all resources of a kind in a namespace

```json
{
  "resource_kind": "Pod",
  "resource_name": "*",
  "namespace": "default",
  "start_time": 1702382400,
  "end_time": 1702386000,
  "max_investigations": 20
}
```

**Use Case**: "Investigate all Pods that had issues in the last hour"

### Pattern 3: Post-Mortem Documentation

**Goal**: Build comprehensive timeline for incident report

```json
{
  "resource_kind": "Deployment",
  "resource_name": "frontend",
  "namespace": "production",
  "start_time": 1702378800,
  "end_time": 1702386000,
  "investigation_type": "post-mortem"
}
```

**Use Case**: "Document what happened during yesterday's outage"

### Pattern 4: Live Incident Investigation

**Goal**: Real-time troubleshooting with immediate guidance

```json
{
  "resource_kind": "StatefulSet",
  "resource_name": "database",
  "namespace": "production",
  "start_time": 1702385000,
  "end_time": 1702386000,
  "investigation_type": "incident"
}
```

**Use Case**: "Help me fix this database StatefulSet that just started failing"

### Pattern 5: Deployment Investigation

**Goal**: Understand Deployment behavior over time

```json
{
  "resource_kind": "Deployment",
  "resource_name": "api-server",
  "namespace": "production",
  "start_time": 1702382400,
  "end_time": 1702386000
}
```

**Use Case**: "Why did this deployment update cause issues?"

## Real-World Examples

### Example 1: Pod CrashLoopBackOff Investigation

**Scenario**: Pod is repeatedly crashing, need to understand why

**Request**:
```json
{
  "resource_kind": "Pod",
  "resource_name": "nginx-7d8b5f9c6b-x7k2p",
  "namespace": "default",
  "start_time": 1702382400,
  "end_time": 1702386000,
  "investigation_type": "incident"
}
```

**Response** (abbreviated):
```json
{
  "investigations": [
    {
      "resource_id": "Pod/default/nginx-7d8b5f9c6b-x7k2p",
      "kind": "Pod",
      "namespace": "default",
      "name": "nginx-7d8b5f9c6b-x7k2p",
      "current_status": "Error",
      "current_message": "CrashLoopBackOff",
      "timeline_start": 1702382400,
      "timeline_end": 1702385800,
      "status_segments": [
        {
          "start_time": 1702382400,
          "end_time": 1702383200,
          "duration_seconds": 800,
          "status": "Running",
          "message": "All containers running"
        },
        {
          "start_time": 1702383200,
          "end_time": 1702385800,
          "duration_seconds": 2600,
          "status": "Error",
          "message": "CrashLoopBackOff"
        }
      ],
      "events": [
        {
          "timestamp": 1702383200,
          "reason": "BackOff",
          "message": "Back-off restarting failed container nginx in pod nginx-7d8b5f9c6b-x7k2p",
          "type": "Warning",
          "count": 15,
          "source": "kubelet"
        },
        {
          "timestamp": 1702383250,
          "reason": "Failed",
          "message": "Error: ErrImagePull - image 'nginx:nonexistent' not found",
          "type": "Warning",
          "count": 3,
          "source": "kubelet"
        }
      ],
      "investigation_prompts": [
        "Analyze the status transitions for Pod/nginx-7d8b5f9c6b-x7k2p. What caused the transition from Running to Error?",
        "The Pod nginx-7d8b5f9c6b-x7k2p is currently in Error state since 2600 seconds. What are the immediate mitigation steps?",
        "Based on the events, what is the root cause of the current error in nginx-7d8b5f9c6b-x7k2p?",
        "Interpret the current message: 'CrashLoopBackOff'. What does it mean?"
      ]
    }
  ],
  "investigation_time_ms": 187
}
```

**Analysis**:
- Pod was Running for 800 seconds (13.3 minutes)
- Transitioned to Error state at 10:13:20
- CrashLoopBackOff caused by image pull failure (`nginx:nonexistent`)
- Event repeated 15 times over 43 minutes
- Investigation prompts guide LLM to identify image tag issue

### Example 2: Deployment Post-Mortem

**Scenario**: Deployment update caused outage, need full timeline

**Request**:
```json
{
  "resource_kind": "Deployment",
  "resource_name": "api-server",
  "namespace": "production",
  "start_time": 1702378800,
  "end_time": 1702386000,
  "investigation_type": "post-mortem"
}
```

**Response** (abbreviated):
```json
{
  "investigations": [
    {
      "resource_id": "Deployment/production/api-server",
      "kind": "Deployment",
      "namespace": "production",
      "name": "api-server",
      "current_status": "Ready",
      "current_message": "Deployment has minimum availability",
      "timeline_start": 1702378800,
      "timeline_end": 1702385900,
      "status_segments": [
        {
          "start_time": 1702378800,
          "end_time": 1702384000,
          "duration_seconds": 5200,
          "status": "Ready",
          "message": "Deployment has minimum availability"
        },
        {
          "start_time": 1702384000,
          "end_time": 1702384800,
          "duration_seconds": 800,
          "status": "Warning",
          "message": "ReplicaFailure: pods failed to start"
        },
        {
          "start_time": 1702384800,
          "end_time": 1702385900,
          "duration_seconds": 1100,
          "status": "Ready",
          "message": "Deployment has minimum availability"
        }
      ],
      "events": [
        {
          "timestamp": 1702384000,
          "reason": "ScalingReplicaSet",
          "message": "Scaled up replica set api-server-85f6c9b8 to 3",
          "type": "Normal",
          "count": 1,
          "source": "deployment-controller"
        },
        {
          "timestamp": 1702384150,
          "reason": "FailedCreate",
          "message": "Error creating: pods 'api-server-85f6c9b8-' is forbidden: exceeded quota",
          "type": "Warning",
          "count": 3,
          "source": "replicaset-controller"
        }
      ],
      "investigation_prompts": [
        "Create a timeline of what happened to Deployment/api-server during this period.",
        "What were the contributing factors that led to the status changes in api-server?",
        "How long did api-server remain in each problematic state? Suggest improvements.",
        "Document the 1 error events and identify any patterns that could be prevented."
      ]
    }
  ],
  "investigation_time_ms": 245
}
```

**Analysis**:
- Deployment was healthy for 86 minutes
- Scaling triggered at 10:40:00
- Entered Warning state for 13.3 minutes (resource quota exceeded)
- Recovered after 18.3 minutes
- Post-mortem prompts guide documentation of timeline and prevention

### Example 3: Multi-Pod Wildcard Investigation

**Scenario**: Multiple pods failing, investigate all at once

**Request**:
```json
{
  "resource_kind": "Pod",
  "resource_name": "*",
  "namespace": "production",
  "start_time": 1702382400,
  "end_time": 1702386000,
  "investigation_type": "incident",
  "max_investigations": 5
}
```

**Response** (abbreviated):
```json
{
  "investigations": [
    {
      "resource_id": "Pod/production/app-7d9f8c5b-z9k3p",
      "kind": "Pod",
      "namespace": "production",
      "name": "app-7d9f8c5b-z9k3p",
      "current_status": "Error",
      "status_segments": [...]
    },
    {
      "resource_id": "Pod/production/app-7d9f8c5b-a2b4c",
      "kind": "Pod",
      "namespace": "production",
      "name": "app-7d9f8c5b-a2b4c",
      "current_status": "Error",
      "status_segments": [...]
    },
    {
      "resource_id": "Pod/production/worker-6c8d4f-x7k2p",
      "kind": "Pod",
      "namespace": "production",
      "name": "worker-6c8d4f-x7k2p",
      "current_status": "Running",
      "status_segments": [...]
    }
  ],
  "investigation_time_ms": 512
}
```

**Analysis**:
- Returned top 5 pods from production namespace
- 2 pods in Error state, 1 Running
- Each has full timeline and investigation prompts
- Useful for identifying patterns across multiple failing pods

## Performance Characteristics

### Execution Time

| Scenario | Resource Count | Avg Time | P95 Time |
|----------|---------------|----------|----------|
| Single resource | 1 | 50-100 ms | 150 ms |
| Wildcard (10 resources) | 10 | 200-400 ms | 600 ms |
| Wildcard (50 resources) | 50 | 800-1200 ms | 1,800 ms |
| Wildcard (100 resources) | 100 | 1,500-2,500 ms | 3,500 ms |

**Note**: Execution time depends on:
- Number of resources investigated
- Event count per resource
- Status segment complexity
- Time window size

### Optimization Tips

**1. Investigate Specific Resources**
```json
// Slower: Wildcard all Pods
{"resource_name": "*"}

// Faster: Specific Pod
{"resource_name": "nginx-7d8b5f9c6b-x7k2p"}
```

**2. Limit Max Investigations**
```json
// Slower: 100 resources
{"max_investigations": 100}

// Faster: 10 resources
{"max_investigations": 10}
```

**3. Narrow Namespace**
```json
// Slower: All namespaces
{"namespace": ""}

// Faster: Specific namespace
{"namespace": "production"}
```

**4. Shorter Time Windows**
```json
// Slower: 24-hour window
{"start_time": 1702296000, "end_time": 1702382400}

// Faster: 1-hour window
{"start_time": 1702382400, "end_time": 1702386000}
```

### Memory Usage

**Estimated Memory per Investigation:**
```
Memory = (events × 1 KB) + (segments × 0.5 KB) + overhead

Typical Pod: 10 events + 3 segments = ~13 KB
Busy Pod: 100 events + 10 segments = ~105 KB
100 resources: ~1-10 MB total
```

## Integration Patterns

### Pattern 1: Three-Step RCA Workflow

**Step 1**: Find problematic resources with `cluster_health`
```json
{"start_time": 1702382400, "end_time": 1702386000}
```

**Step 2**: Identify high-impact changes with `resource_changes`
```json
{"impact_threshold": 0.5, "max_resources": 10}
```

**Step 3**: **Deep investigate** top resource
```json
{
  "resource_kind": "Pod",
  "resource_name": "api-server-85f6c9b8-k4x2p",
  "investigation_type": "incident"
}
```

### Pattern 2: Post-Mortem Prompt Integration

The `post_mortem_incident_analysis` prompt uses `investigate` internally:

**Natural Language** (Claude Desktop):
```
Analyze incident at 10:00 AM on December 12, 2024
```

**Behind the scenes**:
1. Calls `cluster_health` for overview
2. Calls `resource_changes` to find impacted resources
3. **Calls `investigate`** for top 3-5 resources
4. Uses investigation_prompts to generate RCA

### Pattern 3: Live Incident Prompt Integration

The `live_incident_handling` prompt uses `investigate` for immediate guidance:

**Natural Language** (Claude Desktop):
```
Help me fix failing pods in production
```

**Behind the scenes**:
1. Calls `cluster_health` to identify Error pods
2. **Calls `investigate`** with `investigation_type: "incident"`
3. Uses prompts for immediate mitigation steps

### Pattern 4: Timeline Export for Documentation

**Use Case**: Export investigation timeline to incident report

```json
{
  "resource_kind": "Deployment",
  "resource_name": "api-server",
  "investigation_type": "post-mortem"
}
```

**Extract**:
- `status_segments` → Timeline section
- `events` → Event log section
- `investigation_prompts` → LLM analysis prompts

## Troubleshooting

### Empty Investigations Array

**Symptom**: `investigations: []`

**Common Causes:**

**1. No matching resources**
```json
// Request
{"resource_kind": "Pod", "resource_name": "nonexistent"}

// Response
{"investigations": [], "investigation_time_ms": 25}
```
**Solution**: Verify resource name and namespace

**2. Resource name mismatch**
```json
// Wrong: Using label selector
{"resource_name": "app=nginx"}

// Correct: Exact name or wildcard
{"resource_name": "nginx-7d8b5f9c6b-x7k2p"}
```

**3. No events in time window**
```json
// Request has correct name but wrong time range
{"start_time": 1234567890, "end_time": 1234571490}
```
**Solution**: Expand time range or check resource creation time

### Missing Status Segments

**Symptom**: `status_segments: []` but `events` has data

**Possible Causes:**
- Resource has events but no status transitions
- Resource kind doesn't have status field (ConfigMaps, Secrets)

**Solution**: This is expected for some resource kinds; rely on `events` array

### Investigation Prompts Not Helpful

**Symptom**: Generic prompts not relevant to specific issue

**Cause**: Auto-generated prompts are heuristic-based

**Solution**: Use prompts as starting point, customize analysis based on:
- `events` array for specific error messages
- `status_segments` for transition patterns
- `current_message` for latest state

### High Investigation Time

**Symptom**: `investigation_time_ms > 3000` (3+ seconds)

**Causes:**
- Wildcard with many resources (`max_investigations` too high)
- Very large time window (7+ days)
- High event volume per resource (100+ events)

**Solutions:**
1. Reduce `max_investigations` to 10-20
2. Narrow time window to 1-6 hours
3. Investigate specific resource instead of wildcard

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

**Single Resource**:
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "investigate",
    "arguments": {
      "resource_kind": "Pod",
      "resource_name": "nginx-7d8b5f9c6b-x7k2p",
      "namespace": "default",
      "start_time": 1702382400,
      "end_time": 1702386000,
      "investigation_type": "incident"
    }
  },
  "id": 1
}
```

**Wildcard**:
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "investigate",
    "arguments": {
      "resource_kind": "Pod",
      "resource_name": "*",
      "namespace": "production",
      "start_time": 1702382400,
      "end_time": 1702386000,
      "max_investigations": 10
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
        "text": "{\"investigations\":[...],\"investigation_time_ms\":187}"
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
      "name": "investigate",
      "arguments": {
        "resource_kind": "Pod",
        "resource_name": "nginx-7d8b5f9c6b-x7k2p",
        "namespace": "default",
        "start_time": 1702382400,
        "end_time": 1702386000,
        "investigation_type": "incident"
      }
    },
    "id": 1
  }'
```

### Claude Desktop Usage

**Natural Language** (leverages prompts):
```
Investigate the nginx pod that's failing in default namespace
```

**Direct Tool Call**:
```
Use the investigate tool with:
- resource_kind: Pod
- resource_name: nginx-7d8b5f9c6b-x7k2p
- namespace: default
- start_time: 1702382400
- end_time: 1702386000
- investigation_type: incident
```

**Note**: Claude Desktop automatically formats MCP requests; no manual JSON required.

## Best Practices

### ✅ Do

- **Use after resource_changes** - Identify targets first, then investigate deeply
- **Specify investigation_type** - Use `"incident"` for live issues, `"post-mortem"` for historical
- **Leverage investigation_prompts** - Feed prompts to LLMs for RCA guidance
- **Check status_segments** - Understand how long resources stayed in each state
- **Correlate events with segments** - Match event timestamps to status transitions
- **Use wildcards judiciously** - Set reasonable `max_investigations` limit
- **Review timeline_start/end** - Ensure investigation window covers incident
- **Export for documentation** - Use output for incident reports

### ❌ Don't

- **Don't investigate without context** - Use cluster_health or resource_changes first
- **Don't use wildcard without limits** - Always set `max_investigations` < 50
- **Don't ignore investigation_prompts** - They provide contextual RCA guidance
- **Don't investigate wrong resource kind** - Verify kind matches (Pod vs Deployment vs Service)
- **Don't use very wide time windows** - 1-6 hours is optimal for detailed analysis
- **Don't expect complete state** - Snapshots only saved for Error/Warning transitions
- **Don't use for discovery** - Use resource_explorer to find resources first
- **Don't ignore current_message** - Often contains critical error information

## Related Documentation

- [cluster_health Tool](./cluster-health.md) - Find unhealthy resources to investigate
- [resource_changes Tool](./resource-changes.md) - Identify impactful changes before deep dive
- [resource_explorer Tool](./resource-explorer.md) - Discover resources to investigate
- [Post-Mortem Prompt](../prompts-reference/post-mortem.md) - Uses investigate for historical RCA
- [Live Incident Prompt](../prompts-reference/live-incident.md) - Uses investigate for immediate guidance
- [MCP Configuration](../../configuration/mcp-configuration.md) - MCP server deployment

<!-- Source: internal/mcp/tools/investigate.go, internal/mcp/server.go, README.md lines 128-198 -->
