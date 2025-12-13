---
title: post_mortem_incident_analysis Prompt
description: Comprehensive post-mortem analysis with systematic investigation workflow
keywords: [mcp, prompts, post-mortem, rca, incident, analysis]
---

# post_mortem_incident_analysis Prompt

Comprehensive post-mortem analysis of past incidents with systematic investigation workflow and root cause analysis.

## Overview

The `post_mortem_incident_analysis` prompt guides an LLM through a structured investigation of historical incidents, ensuring grounded analysis with no hallucinations.

**Key Capabilities:**
- **Systematic 9-Step Workflow**: Structured investigation from overview to recommendations
- **Grounded Analysis**: All claims traceable to actual tool outputs
- **No Hallucinations**: Only reports events present in tool responses
- **Chronological Timeline**: Exact timestamps and event sequences
- **Root Cause Analysis**: Identifies contributing factors with evidence
- **Impact Assessment**: Documents incident scope and affected resources
- **Preventive Measures**: Actionable recommendations to prevent recurrence
- **Data Gap Identification**: Acknowledges missing information and suggests additional investigation

**When to Use:**
- Post-mortem analysis after incident resolution
- Historical incident investigation for documentation
- Root cause analysis for recurring issues
- Compliance and audit requirements
- Learning from production incidents
- Building incident response knowledge base

**When NOT to Use:**
- Live incident troubleshooting (use `live_incident_handling` instead)
- Real-time monitoring or alerting
- Routine health checks (use `cluster_health` tool directly)

## Arguments

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `start_time` | int64 | **Yes** | Start of incident window (Unix timestamp in seconds or milliseconds) |
| `end_time` | int64 | **Yes** | End of incident window (Unix timestamp in seconds or milliseconds) |
| `namespace` | string | No | Kubernetes namespace to focus investigation (optional) |
| `incident_description` | string | No | Brief context about the incident (optional) |

### Timestamp Format

Both **Unix seconds** and **Unix milliseconds** are supported:

```
Unix seconds: 1702382400
Unix milliseconds: 1702382400000
```

**Getting timestamps**:
```bash
# Start time (Dec 12, 2024 10:00 AM UTC)
date -u -d "2024-12-12 10:00:00" +%s
# Output: 1702382400

# End time (Dec 12, 2024 11:00 AM UTC)
date -u -d "2024-12-12 11:00:00" +%s
# Output: 1702386000
```

### Time Window Guidelines

| Window | Use Case | Analysis Depth |
|--------|----------|----------------|
| 5-15 minutes | Specific incident (deployment failure, pod crash) | Very detailed |
| 30-60 minutes | Service degradation, partial outage | Detailed |
| 1-2 hours | Complete incident lifecycle | Standard depth |
| 2-4 hours | Complex incident with multiple phases | Comprehensive |
| 4+ hours | Extended outage, multi-system issues | High-level overview + detailed critical periods |

**Recommendation**: Include 15-30 minutes before symptom onset to capture precursor events.

## Workflow

The prompt guides the LLM through a systematic 9-step investigation:

### Step 1: Confirm Parameters

**What happens**: LLM confirms the investigation scope

**Output**:
```
Investigating incident from [start_time] to [end_time]
Namespace filter: [namespace] (or "all namespaces")
Context: [incident_description] (if provided)
```

**Purpose**: Establish investigation boundaries and focus

### Step 2: Cluster Health Overview

**Tool called**: `cluster_health`

**Parameters**:
```json
{
  "start_time": "<incident_start_time>",
  "end_time": "<incident_end_time>",
  "namespace": "<namespace>" // if provided
}
```

**What it provides**:
- Overall cluster status during incident
- Resources by kind with status breakdown
- Top issues encountered
- Error/warning resource counts

**Purpose**: Understand incident scope and severity

### Step 3: Identify High-Impact Changes

**Tool called**: `resource_changes`

**Parameters**:
```json
{
  "start_time": "<incident_start_time>",
  "end_time": "<incident_end_time>",
  "impact_threshold": 0.3,
  "max_resources": 30
}
```

**What it provides**:
- Resources with high impact scores
- Status transitions
- Error and warning counts
- Change correlation

**Purpose**: Identify what changed during the incident

### Step 4: Investigate Critical Resources

**Tool called**: `investigate` (for top 3-5 resources from Step 3)

**Parameters** (per resource):
```json
{
  "resource_kind": "<kind>",
  "resource_name": "<name>",
  "namespace": "<namespace>",
  "start_time": "<incident_start_time>",
  "end_time": "<incident_end_time>",
  "investigation_type": "post-mortem"
}
```

**What it provides**:
- Detailed timeline with status segments
- Kubernetes events
- Investigation prompts for RCA
- Resource snapshots at transitions

**Purpose**: Deep dive into root cause candidates

### Step 5: Explore Related Resources

**Tool called**: `resource_explorer` (optional, as needed)

**Parameters**:
```json
{
  "kind": "<related_kind>",
  "namespace": "<namespace>",
  "status": "Error"
}
```

**What it provides**:
- Broader context of related failures
- Resource inventory at incident time
- Cross-resource impact patterns

**Purpose**: Identify cascading failures or dependencies

### Step 6: Build Chronological Timeline

**LLM synthesizes** tool results into timeline:

```
[10:00:05] Deployment/production/api-server - Updated image v1.2.0 → v1.3.0
[10:00:32] Pod/production/api-server-85f6c9b8-k4x2p - Transitioned Running → Error
[10:00:45] Pod/production/api-server-85f6c9b8-k4x2p - Event: BackOff (count: 3)
[10:01:12] Service/production/api-server - Endpoint removed (no ready pods)
[10:05:47] Deployment/production/api-server - Rolled back to v1.2.0
[10:06:15] Pod/production/api-server-7d9f8c5b-z9k3p - Transitioned Pending → Running
```

**Purpose**: Visualize incident progression with exact timing

### Step 7: Root Cause Analysis

**LLM analyzes** investigation_prompts and tool results:

```
Root Cause:
- Image tag v1.3.0 had incorrect environment variable configuration
- Pod failed to start due to missing CONFIG_URL variable
- CrashLoopBackOff prevented service availability

Contributing Factors:
1. No pre-deployment validation of container startup
2. Missing health check delay allowed traffic before readiness
3. Rollback took 5 minutes (manual process)
```

**Purpose**: Identify primary cause and contributing factors

### Step 8: Impact Assessment & Recommendations

**LLM documents**:

**Impact**:
- Service downtime: 5 minutes 42 seconds
- Affected resources: 1 Deployment, 3 Pods, 1 Service
- Error rate: 100% during outage
- Customer impact: API unavailable

**Preventive Measures**:
1. Add startup validation in CI/CD pipeline
2. Implement health check initialDelaySeconds
3. Automate rollback detection and execution
4. Add alerting for pod CrashLoopBackOff

**Purpose**: Document impact and prevent recurrence

### Step 9: Identify Data Gaps

**LLM lists** missing information:

```
Additional Investigation Needed:
1. kubectl logs api-server-85f6c9b8-k4x2p --previous
   → Check exact container error message
2. kubectl describe pod api-server-85f6c9b8-k4x2p
   → Verify environment variable configuration
3. Review CI/CD logs for build v1.3.0
   → Understand configuration change origin
4. Check application metrics (Prometheus/Datadog)
   → Correlate with external monitoring
```

**Purpose**: Guide follow-up investigation

## Key Features

### 1. No Hallucinations

**How it works**:
- Prompt explicitly instructs: "Only report events present in tool responses"
- LLM must quote exact timestamps, resource names, and messages
- Forbidden to infer events not in data

**Example**:
```
✅ Good: "At 10:00:32, Pod api-server-85f6c9b8-k4x2p transitioned to Error status"
         (Directly from investigate tool output)

❌ Bad: "The database likely became overloaded"
        (Speculation without evidence from tools)
```

### 2. Grounded Analysis

**How it works**:
- All claims must be traceable to specific tool output
- Uses investigation_prompts from `investigate` tool as RCA guidance
- References exact field values from tool responses

**Example**:
```
✅ Good: "impact_score: 0.75 indicates high impact due to:
         - error_events: 3 (+0.30)
         - status transition Running → Error (+0.30)
         - event_count: 18 (+0.10)"
         (Based on resource_changes output)

❌ Bad: "This was a high-impact incident"
        (Vague claim without supporting data)
```

### 3. Log Guidance

**How it works**:
- Prompt instructs LLM to recommend specific kubectl commands
- Suggests cloud logging queries (CloudWatch, Stackdriver, etc.)
- Points to external observability tools when appropriate

**Example**:
```
Additional Logs to Review:
1. kubectl logs nginx-7d8b5f9c6b-x7k2p --previous
   → Container exit reason and error messages
2. kubectl describe pod nginx-7d8b5f9c6b-x7k2p
   → Full event history and configuration
3. AWS CloudWatch Logs: /aws/eks/production/cluster
   → Node-level issues or kubelet errors
```

### 4. Acknowledges Gaps

**How it works**:
- Prompt requires LLM to state when information is missing
- Explicitly marks hypotheses vs. confirmed facts
- Lists specific commands/queries needed to fill gaps

**Example**:
```
Data Gaps:
- Container logs not available (requires kubectl logs --previous)
- Application metrics not in Spectre data (check Prometheus)
- Network policy changes unknown (review audit logs)

Hypothesis (Unconfirmed):
- Pod may have exceeded memory limit (requires kubectl describe pod)
```

### 5. Structured Output

**Expected sections**:

```markdown
## Incident Summary
[Brief overview]

## Timeline
[Chronological event list with exact timestamps]

## Root Cause Analysis
[Primary cause + contributing factors with evidence]

## Impact Assessment
[Affected resources, downtime duration, user impact]

## Recommendations
[Preventive measures, process improvements, automation opportunities]

## Data Gaps & Follow-Up
[Missing information, kubectl commands, additional logs to review]
```

## Usage Examples

### Example 1: Via Claude Desktop (Natural Language)

**User**:
```
Analyze the incident from 10:00 AM to 11:00 AM on December 12, 2024 in the production namespace
```

**What happens**:
1. Claude recognizes this as a post-mortem request
2. Converts timestamps to Unix format
3. Calls `post_mortem_incident_analysis` prompt
4. Executes 9-step workflow
5. Returns structured analysis

**LLM Output** (abbreviated):
```
## Incident Summary
Deployment update to api-server v1.3.0 caused 5min 42sec service outage due to
missing environment variable configuration.

## Timeline
[10:00:05] Deployment/production/api-server - Image updated v1.2.0 → v1.3.0
[10:00:32] Pod/production/api-server-85f6c9b8-k4x2p - Status Running → Error
[10:00:45] Event: BackOff - Back-off restarting failed container (count: 15)
...

## Root Cause Analysis
Primary Cause: Container v1.3.0 missing CONFIG_URL environment variable

Contributing Factors:
1. No pre-deployment validation
2. Manual rollback process (5min delay)

Evidence:
- investigate tool showed status transition at 10:00:32
- resource_changes impact_score: 0.75 (high)
- Event message: "Back-off restarting failed container"

## Recommendations
1. Add container startup validation to CI/CD
2. Implement automated rollback on CrashLoopBackOff
...

## Data Gaps
Need: kubectl logs api-server-85f6c9b8-k4x2p --previous
```

### Example 2: Via MCP API (Direct Prompt Call)

**Request**:
```json
{
  "jsonrpc": "2.0",
  "method": "prompts/get",
  "params": {
    "name": "post_mortem_incident_analysis",
    "arguments": {
      "start_time": 1702382400,
      "end_time": 1702386000,
      "namespace": "production",
      "incident_description": "API service degradation - 500 errors"
    }
  },
  "id": 1
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "result": {
    "description": "Prompt: post_mortem_incident_analysis",
    "messages": [
      {
        "role": "user",
        "content": {
          "type": "text",
          "text": "Execute prompt post_mortem_incident_analysis..."
        }
      }
    ]
  },
  "id": 1
}
```

**Note**: The prompt is then executed by the LLM, which calls tools and generates the structured analysis.

### Example 3: Deployment Failure

**User** (Claude Desktop):
```
Analyze deployment failure for api-server from 10:15 to 10:30 this morning
```

**Tools Called by Prompt**:
1. `cluster_health` → Found Deployment in Warning state
2. `resource_changes` → impact_score: 0.45 for api-server Deployment
3. `investigate` (Deployment) → Status: Ready → Warning → Ready
4. `investigate` (Pods) → 3 pods failed to start

**RCA Output**:
```
Root Cause: Resource quota exceeded during scale-up

Timeline:
[10:15:00] Deployment scaled 3 → 5 replicas
[10:15:12] Event: FailedCreate - exceeded quota
[10:16:45] Deployment Warning - ReplicaFailure
[10:28:00] Quota increased
[10:28:30] Pods successfully created

Recommendations:
1. Increase default namespace quota
2. Add quota monitoring alerts
3. Implement progressive rollout (1 pod at a time)
```

## Best Practices

### ✅ Do

- **Include precursor time** - Start 15-30 minutes before symptoms
- **Provide context** - Use `incident_description` for focused analysis
- **Review tool outputs** - Verify LLM grounded analysis in actual data
- **Follow log guidance** - Run suggested kubectl commands
- **Document findings** - Save structured output for incident reports
- **Use for learning** - Build runbooks from RCA recommendations
- **Verify timestamps** - Check timeline matches your incident logs
- **Cross-reference** - Compare with external monitoring tools

### ❌ Don't

- **Don't omit namespace** - Speeds up analysis when incident is scoped
- **Don't ignore data gaps** - Follow up with suggested kubectl commands
- **Don't trust without verification** - LLMs can make mistakes despite grounding
- **Don't use for live incidents** - Use `live_incident_handling` instead
- **Don't expect logs** - Prompt cannot access container logs
- **Don't query very old incidents** - Check Spectre retention first
- **Don't assume completeness** - Analysis limited to Spectre data
- **Don't skip recommendations** - Implement preventive measures

## Related Documentation

- [live_incident_handling Prompt](./live-incident.md) - Real-time incident response
- [cluster_health Tool](../tools-reference/cluster-health.md) - Cluster overview (used by prompt)
- [resource_changes Tool](../tools-reference/resource-changes.md) - Change identification (used by prompt)
- [investigate Tool](../tools-reference/investigate.md) - Resource deep dive (used by prompt)
- [resource_explorer Tool](../tools-reference/resource-explorer.md) - Resource discovery (used by prompt)
- [MCP Configuration](../../configuration/mcp-configuration.md) - MCP server setup

<!-- Source: internal/mcp/handler.go, README.md lines 204-254 -->
