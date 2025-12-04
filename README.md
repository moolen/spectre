# Spectre

<div align="center">
  <img src="ui/public/ghost.svg" alt="Spectre" width="200">
</div>

## What is Spectre?

Spectre is a Kubernetes event monitoring and auditing system. It captures all resource changes (create, update, delete) across your cluster and provides a powerful visualization dashboard to understand what happened, when it happened, and why.

![Deployment Rollout](./docs/screenshot-2.png)

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

### Using Helm

```bash
# Add the Spectre Helm repository
helm install spectre oci://ghcr.io/moolen/charts/spectre \
  --namespace monitoring \
  --create-namespace

kubectl port-forward -n monitoring svc/spectre 8080:8080

# Open your browser to http://localhost:8080
```

## Configuration

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

### Demo mode

Spectre comes with a demo mode which allows you to navigate the UI with demo data.

```sh

docker run -it -p 8080:8080 docker pull ghcr.io/moolen/spectre:master --demo

```

## MCP

Spectre offers an MCP endpoint to support users doing a post-mortem analysis or live incident investigation. The MCP server offers the following APIs:

### Tools

#### `cluster_health`

Get a health overview of your cluster. Given a `start_time`, `end_time` and optional `namespace` it assesses the health of the cluster: Pods with CrashLoopBackoff, ImagePullBackOff, failed jobs, unhealthy nodes, not-ready custom resources such as Flux or ArgoCD are supported.

#### `resource_changes`

Get summarized resource changes with impact categorization to understand what changed and when it changed. This is optimised for use by a LLM. It provides a impact score to rank warning / error events.

Example:

```json
{
  "changes": [
    {
      "kind": "Deployment",
      "namespace": "prod",
      "name": "payment-api",
      "impact_score": 0.85,
      "change_count": 5,
      "event_count": 12,
      "error_events": 3,
      "warning_events": 2,
      "status_transitions": [
        {
          "from_status": "Ready",
          "to_status": "Warning",
          "timestamp": 1701432000,
          "message": "Deployment has minimum availability"
        },
        {
          "from_status": "Warning",
          "to_status": "Error",
          "timestamp": 1701435600,
          "message": "No replicas available"
        }
      ]
    }
  ],
  "total_changes": 12,
  "resources_affected": 8,
  "aggregation_time_ms": 45
}
```

#### `investigate`

Get detailed investigation evidence with status timeline and RCA prompts.

**Purpose**: Comprehensive incident investigation with investigation guidance

**Input Parameters**:
- `resource_kind` (required) - Resource kind to investigate (e.g., "Pod")
- `resource_name` (optional) - Specific resource name, or "*" for all
- `namespace` (optional) - Filter by namespace
- `start_time` (required) - Start timestamp
- `end_time` (required) - End timestamp
- `investigation_type` (optional) - "incident" (live response), "post-mortem" (historical), or "auto" (detect)

**Output**:
```json
{
  "investigations": [
    {
      "resource_id": "pod-789",
      "kind": "Pod",
      "namespace": "default",
      "name": "worker-1",
      "current_status": "Error",
      "current_message": "ImagePullBackOff",
      "timeline_start": 1701432000,
      "timeline_end": 1701435600,
      "status_segments": [
        {
          "start_time": 1701432000,
          "end_time": 1701432300,
          "duration_seconds": 300,
          "status": "Ready",
          "message": "Pod running"
        },
        {
          "start_time": 1701432300,
          "end_time": 1701435600,
          "duration_seconds": 3300,
          "status": "Error",
          "message": "ImagePullBackOff"
        }
      ],
      "events": [
        {
          "timestamp": 1701432300,
          "reason": "Failed",
          "message": "Failed to pull image 'myapp:v2.0'",
          "type": "Warning",
          "count": 3,
          "source": "kubelet"
        }
      ],
      "investigation_prompts": [
        "Analyze the status transitions for Pod/worker-1. What caused the transition from Ready to Error?",
        "The Pod worker-1 is currently in Error state since 3300 seconds. What are the immediate mitigation steps?",
        "Based on the events, what is the root cause of the current error in worker-1?"
      ],
      "raw_resource_snapshots": [
        {
          "timestamp": 1701432300,
          "status": "Error",
          "message": "ImagePullBackOff",
          "data": { /* full pod spec */ }
        }
      ]
    }
  ],
  "investigation_time_ms": 78
}
```

### Prompts

The MCP Server provides specialized prompts that guide the LLM through systematic incident investigation workflows. These prompts enforce strict grounding in tool results and prevent hallucinations.

### 1. `post_mortem_incident_analysis`

Conduct a comprehensive post-mortem analysis of a past incident.

**Purpose**: Systematic historical incident investigation with root cause analysis

**Arguments**:
- `start_time` (required) - Start of incident window (Unix seconds/milliseconds)
- `end_time` (required) - End of incident window (Unix seconds/milliseconds)
- `namespace` (optional) - Kubernetes namespace to focus investigation
- `incident_description` (optional) - Brief context about the incident

**Workflow**:
1. Confirms time window and namespace filter
2. Calls `cluster_health` to get incident window overview
3. Calls `resource_changes` to identify high-impact changes
4. Uses `investigate` for detailed timelines on critical resources
5. Uses `resource_explorer` for context on related resources
6. Builds chronological timeline with exact timestamps
7. Identifies contributing factors and root causes
8. Documents impact and suggests preventive measures
9. Lists follow-up actions and additional data needed

**Key Features**:
- **No Hallucinations**: Only reports events present in tool responses
- **Grounded Analysis**: All claims traceable to specific tool output
- **Log Guidance**: Explicitly recommends kubectl logs, describe, cloud logs
- **Acknowledges Gaps**: States when information is missing
- **Structured Output**: Timeline, RCA, impact, recommendations, data gaps

**Example Usage**:

Via Claude Desktop:
```
"Run a post-mortem analysis for the incident from 2pm to 3pm in the production namespace"
```

Via API:
```json
{
  "method": "prompts/get",
  "params": {
    "name": "post_mortem_incident_analysis",
    "arguments": {
      "start_time": 1701432000,
      "end_time": 1701435600,
      "namespace": "production",
      "incident_description": "API service degradation"
    }
  }
}
```

### 2. live_incident_handling

Triage and investigate an ongoing incident.

**Purpose**: Real-time incident response with immediate mitigation guidance

**Arguments**:
- `incident_start_time` (required) - When symptoms first appeared (Unix seconds/milliseconds)
- `current_time` (optional) - Current time (defaults to now)
- `namespace` (optional) - Kubernetes namespace to focus on
- `symptoms` (optional) - Brief description of observed behavior

**Workflow**:
1. Confirms incident start time and optional namespace/symptoms
2. Calls `cluster_health` for incident window to identify critical resources
3. Calls `resource_changes` to see what changed around incident start
4. Uses `investigate` for detailed timelines on suspect resources
5. Correlates events to identify likely root cause
6. Recommends immediate mitigation steps
7. Suggests monitoring and follow-up actions
8. Lists additional logs/metrics needed

**Key Features**:
- **Focus on Recent Data**: Looks slightly before incident_start_time for precursors
- **No Hallucinations**: Only reports actual tool results
- **Immediate Actions**: Concrete mitigation steps (restart, rollback, scale)
- **Log Guidance**: Specific kubectl commands and cloud logging queries
- **Acknowledges Uncertainty**: Clearly marks hypotheses when data is incomplete
