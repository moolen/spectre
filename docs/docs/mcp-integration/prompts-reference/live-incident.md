---
title: live_incident_handling Prompt
description: Real-time incident triage and immediate mitigation guidance
keywords: [mcp, prompts, live-incident, triage, mitigation, troubleshooting]
---

# live_incident_handling Prompt

Real-time incident triage and investigation with immediate mitigation guidance for ongoing issues.

## Overview

The `live_incident_handling` prompt guides an LLM through immediate incident response, focusing on quick triage, root cause identification, and actionable mitigation steps.

**Key Capabilities:**
- **Real-Time Focus**: Analyzes recent events (looks before incident start for precursors)
- **Immediate Mitigation**: Concrete action steps (restart, rollback, scale)
- **Fast Triage**: 8-step workflow optimized for speed
- **No Hallucinations**: Only reports actual tool results
- **Log Guidance**: Specific kubectl commands for verification
- **Acknowledges Uncertainty**: Marks hypotheses when data is incomplete
- **Monitoring Suggestions**: What to watch for escalation or recovery

**When to Use:**
- Active incident troubleshooting
- Service degradation or outage in progress
- Pods crashing or failing to start
- Deployment or rollout issues
- Resource quota or capacity problems
- Real-time triage before full post-mortem

**When NOT to Use:**
- Historical incident analysis (use `post_mortem_incident_analysis` instead)
- Routine health checks (use `cluster_health` tool directly)
- Proactive monitoring (use external monitoring tools)

## Arguments

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `incident_start_time` | int64 | **Yes** | When symptoms first appeared (Unix timestamp in seconds or milliseconds) |
| `current_time` | int64 | No | Current time (Unix timestamp); defaults to now if omitted |
| `namespace` | string | No | Kubernetes namespace to focus on (optional) |
| `symptoms` | string | No | Brief description of observed behavior (optional) |

### Timestamp Format

Both **Unix seconds** and **Unix milliseconds** are supported:

```
Unix seconds: 1702382400
Unix milliseconds: 1702382400000
```

**Getting current time**:
```bash
# Current time (Unix seconds)
date +%s
# Output: 1702386000

# Incident start (15 minutes ago)
date -d "15 minutes ago" +%s
# Output: 1702385100
```

### Incident Window Calculation

**How the prompt determines time range**:
```
# If current_time not provided
analysis_end = now()

# Investigation window
analysis_start = incident_start_time - 15 minutes  # Look for precursors
analysis_end = current_time (or now)
```

**Example**:
```
incident_start_time: 10:00
current_time: 10:15 (or omitted)

Actual window analyzed: 09:45 to 10:15
```

**Purpose**: Capture events before symptoms appeared

## Workflow

The prompt guides the LLM through an 8-step rapid investigation:

### Step 1: Confirm Parameters & Calculate Window

**What happens**: LLM establishes investigation scope

**Output**:
```
Incident started: [incident_start_time]
Current time: [current_time or now]
Namespace filter: [namespace] (or "all namespaces")
Symptoms: [symptoms description] (if provided)

Investigation window: [start -15min] to [current_time]
```

**Purpose**: Set boundaries and context for rapid analysis

### Step 2: Identify Critical Resources

**Tool called**: `cluster_health`

**Parameters**:
```json
{
  "start_time": "<incident_start_time - 15min>",
  "end_time": "<current_time or now>",
  "namespace": "<namespace>" // if provided
}
```

**What it provides**:
- Resources in Error state NOW
- Top issues currently occurring
- Affected resource counts by kind

**Purpose**: Quickly identify what's broken RIGHT NOW

### Step 3: Identify Recent Changes

**Tool called**: `resource_changes`

**Parameters**:
```json
{
  "start_time": "<incident_start_time - 15min>",
  "end_time": "<current_time or now>",
  "impact_threshold": 0.3,
  "max_resources": 20
}
```

**What it provides**:
- What changed around incident start
- High-impact changes (deployments, config updates)
- Correlation between changes and failures

**Purpose**: Find "what changed just before this started?"

### Step 4: Investigate Failing Resources

**Tool called**: `investigate` (for top 2-3 Error resources from Step 2)

**Parameters**:
```json
{
  "resource_kind": "<kind>",
  "resource_name": "<name>",
  "namespace": "<namespace>",
  "start_time": "<incident_start_time - 15min>",
  "end_time": "<current_time or now>",
  "investigation_type": "incident"
}
```

**What it provides**:
- Recent timeline with status transitions
- Latest events and error messages
- RCA prompts focused on immediate mitigation

**Purpose**: Understand WHY resources are failing

### Step 5: Correlate Events to Root Cause

**LLM analyzes** tool outputs to identify likely cause:

```
Correlation:
[09:58] Deployment/production/api-server updated (v1.2 → v1.3)
[10:00] Pod/production/api-server-85f6c9b8-k4x2p → Error
[10:00] Event: CrashLoopBackOff (container startup failure)

Likely Cause: Image v1.3 has startup issue
```

**Purpose**: Connect "what changed" to "what's failing"

### Step 6: Recommend Immediate Mitigation

**LLM provides** concrete action steps:

```
Immediate Mitigation Steps:

1. Rollback Deployment (Fastest Recovery):
   kubectl rollout undo deployment/api-server -n production

2. Check Pod Logs (Verify Root Cause):
   kubectl logs api-server-85f6c9b8-k4x2p -n production --previous

3. Monitor Recovery:
   kubectl get pods -n production -l app=api-server -w

Expected Recovery Time: 30-60 seconds
```

**Purpose**: Give operator immediate actionable steps

### Step 7: Suggest Monitoring & Follow-Up

**LLM lists** what to watch:

```
Monitor for:
- New pods becoming Ready (kubectl get pods -w)
- Service endpoint health (kubectl get endpoints api-server)
- Error rate metrics (check Prometheus/Datadog)

Follow-Up Actions:
1. Verify rollback completed successfully
2. Run full post-mortem once resolved
3. Review deployment pipeline for v1.3 issues
4. Add pre-deployment validation tests
```

**Purpose**: Ensure successful mitigation and prevent recurrence

### Step 8: List Additional Data Needed

**LLM identifies** information gaps:

```
Additional Investigation (If Mitigation Fails):

1. Container logs:
   kubectl logs <pod-name> -n production --previous

2. Pod details:
   kubectl describe pod <pod-name> -n production

3. Recent changes:
   kubectl rollout history deployment/api-server -n production

4. Node status (if suspecting node issues):
   kubectl describe node <node-name>

5. Application metrics:
   Check Prometheus/Datadog for error spike timing
```

**Purpose**: Guide deeper investigation if initial mitigation insufficient

## Key Features

### 1. Focus on Recent Data

**How it works**:
- Automatically looks 15 minutes before `incident_start_time`
- Captures precursor events (deployments, config changes)
- Focuses on current state (Error resources)

**Example**:
```
Incident reported at 10:00
Window analyzed: 09:45 to 10:15

Findings:
[09:58] ConfigMap updated (precursor found!)
[10:00] Pods started failing (symptom onset)
```

### 2. Immediate Actions

**How it works**:
- Provides kubectl commands ready to execute
- Prioritizes fastest recovery path
- Includes rollback, restart, scale commands

**Example**:
```
✅ Good: "kubectl rollout undo deployment/api-server -n production"
         (Concrete, executable command)

❌ Bad: "Consider rolling back the deployment"
        (Vague, requires operator to figure out how)
```

### 3. Acknowledges Uncertainty

**How it works**:
- Marks hypotheses vs. confirmed facts
- States when more data is needed
- Provides paths to get missing information

**Example**:
```
Confirmed (from tool data):
- Pod api-server-85f6c9b8-k4x2p in CrashLoopBackOff
- Deployment updated at 09:58

Hypothesis (Requires Verification):
- Container startup failing due to missing environment variable
  → Verify with: kubectl logs <pod> --previous
```

### 4. No Hallucinations

**How it works**:
- Same grounding principles as post-mortem prompt
- Only reports events from tool outputs
- Forbidden to infer without evidence

**Example**:
```
✅ Good: "resource_changes shows Deployment updated at 09:58,
         investigate shows Pod failed at 10:00 (2min after)"
         (Directly from tool data)

❌ Bad: "The deployment probably caused a memory leak"
        (Speculation without evidence)
```

## Usage Examples

### Example 1: Via Claude Desktop (Natural Language)

**User**:
```
Pods are failing in the production namespace. Started about 10 minutes ago.
```

**What happens**:
1. Claude recognizes this as live incident
2. Calculates incident_start_time (now - 10 minutes)
3. Calls `live_incident_handling` prompt
4. Executes 8-step workflow
5. Returns immediate mitigation steps

**LLM Output** (abbreviated):
```
## Immediate Situation
Incident started: ~10 minutes ago
Namespace: production
Status: Active (ongoing)

## Current Impact
- 3 Pods in Error state (CrashLoopBackOff)
- Service api-server has no ready endpoints
- Deployment api-server in Warning state

## Likely Root Cause
Deployment updated 12 minutes ago (v1.2.0 → v1.3.0)
Pods failing to start immediately after update

Evidence:
- resource_changes: Deployment impact_score 0.65
- investigate: Pod transition Running → Error at 10:02

## Immediate Mitigation Steps

1. Rollback Deployment (30-60 sec recovery):
   kubectl rollout undo deployment/api-server -n production

2. Verify Rollback:
   kubectl rollout status deployment/api-server -n production

3. Check Recovery:
   kubectl get pods -n production -l app=api-server

Expected Recovery: 30-60 seconds

## Verification
After rollback, confirm:
- kubectl logs <new-pod-name> -n production --previous
  → Check why v1.3.0 failed to start

## Next Steps
1. Wait for rollback completion
2. Run post-mortem analysis
3. Fix v1.3.0 issues before retry
```

### Example 2: Via MCP API

**Request**:
```json
{
  "jsonrpc": "2.0",
  "method": "prompts/get",
  "params": {
    "name": "live_incident_handling",
    "arguments": {
      "incident_start_time": 1702385100,
      "namespace": "production",
      "symptoms": "API returning 500 errors, pods crashing"
    }
  },
  "id": 1
}
```

**Note**: `current_time` omitted, will default to now()

**Response**: (prompt executed, LLM calls tools and generates mitigation steps)

### Example 3: Resource Quota Issue

**User** (Claude Desktop):
```
Deployment can't scale up. Pods stuck in Pending. Started 5 minutes ago.
```

**Tools Called**:
1. `cluster_health` → 0 Error pods, but Deployment in Warning
2. `resource_changes` → Deployment scale event (3 → 5 replicas)
3. `investigate` (Deployment) → Event: FailedCreate (quota exceeded)

**LLM Output**:
```
## Root Cause
Resource quota exceeded during scale-up

Timeline:
[10:10] Deployment scaled 3 → 5 replicas
[10:10] Event: FailedCreate - exceeded quota: pods=5, limit=3

## Immediate Mitigation

Option 1 (Fastest - Scale Back):
kubectl scale deployment/app --replicas=3 -n production

Option 2 (Increase Quota):
# Edit namespace quota
kubectl edit resourcequota production-quota -n production
# Increase pods limit from 3 to 5

## Recommendation
Use Option 1 (scale back) immediately if service is degraded.
Increase quota during next maintenance window.

## Verification
kubectl get pods -n production
→ Should see 3 running pods after scale-back
```

## Integration with Tools

```
┌──────────────────────────────────────────────┐
│  User: "Pods failing in production"         │
└────────────────┬─────────────────────────────┘
                 │
                 v
┌──────────────────────────────────────────────┐
│  Prompt: live_incident_handling              │
│  - Calculates: start_time - 15min           │
│  - Focuses: Current Error resources         │
└────────────────┬─────────────────────────────┘
                 │
       ┌─────────┼──────────┬─────────────┐
       v         v          v             v
┌────────────┐ ┌───────────┐ ┌──────────┐ ┌──────────┐
│cluster     │ │resource   │ │investigate│ │(optional)│
│_health     │ │_changes   │ │(×2-3 res)│ │explorer  │
│NOW        │ │RECENT     │ │INCIDENT   │ │          │
└──────┬─────┘ └─────┬─────┘ └────┬─────┘ └────┬─────┘
       │             │            │            │
       └─────────────┴────────────┴────────────┘
                        │
                        v
         ┌──────────────────────────────┐
         │ LLM Provides:                │
         │ - Immediate mitigation steps │
         │ - kubectl commands           │
         │ - Recovery monitoring        │
         │ - Follow-up actions          │
         └──────────────────────────────┘
```

## Best Practices

### ✅ Do

- **Act quickly** - Run prompt as soon as incident detected
- **Provide symptoms** - Helps LLM focus investigation
- **Execute mitigation** - Run suggested kubectl commands
- **Monitor recovery** - Watch for pod/service stabilization
- **Verify hypothesis** - Check logs after mitigation
- **Run post-mortem** - Full analysis after recovery
- **Use for triage** - Quick assessment before manual intervention
- **Specify namespace** - Faster analysis when scoped

### ❌ Don't

- **Don't delay** - Prompt optimized for real-time use
- **Don't skip verification** - Always check container logs
- **Don't ignore follow-up** - Post-mortem prevents recurrence
- **Don't assume complete** - LLM analysis limited to Spectre data
- **Don't use for old incidents** - Use `post_mortem_incident_analysis` instead
- **Don't execute blindly** - Understand mitigation before running
- **Don't expect logs** - Prompt cannot access container stdout/stderr
- **Don't use without tools** - Requires MCP server connection

## Limitations

### 1. Real-Time Data Only

**Limitation**: Analyzes events up to current_time

**What's Missing**:
- Future events (obviously)
- Events after analysis completes

**Mitigation**: Re-run prompt if situation changes

### 2. No Container Logs

**Limitation**: Cannot access pod logs directly

**What's Missing**:
- Container stdout/stderr
- Application error messages
- Startup failure reasons

**Mitigation**: Prompt suggests kubectl logs commands

### 3. No External Metrics

**Limitation**: Spectre data only (no Prometheus, Datadog, etc.)

**What's Missing**:
- CPU/memory metrics
- Request rates, latencies
- Custom application metrics

**Mitigation**: Prompt recommends checking external tools

### 4. Hypothesis vs. Fact

**Limitation**: LLM may form hypotheses without full evidence

**Mitigation**: Prompt explicitly marks "Hypothesis (Unconfirmed)" vs. "Confirmed"

## Related Documentation

- [post_mortem_incident_analysis Prompt](./post-mortem.md) - Historical incident analysis after resolution
- [cluster_health Tool](../tools-reference/cluster-health.md) - Current cluster status (used by prompt)
- [resource_changes Tool](../tools-reference/resource-changes.md) - Recent changes (used by prompt)
- [investigate Tool](../tools-reference/investigate.md) - Resource timelines (used by prompt with type=incident)
- [MCP Configuration](../../configuration/mcp-configuration.md) - MCP server setup

<!-- Source: internal/mcp/handler.go, README.md lines 257-285 -->
