---
title: Incident Investigation
description: Step-by-step guide for investigating Kubernetes incidents with Spectre
keywords: [incident, investigation, troubleshooting, kubernetes, root cause, timeline]
---

# Incident Investigation

Quick and effective incident investigation using Spectre's event tracking and timeline reconstruction capabilities.

## Overview

When an incident occurs, time is critical. Spectre helps you quickly identify:
- **What changed** - Recent deployments, config updates, or resource modifications
- **When it failed** - Exact timestamps of state transitions and errors
- **Why it failed** - Event messages and status changes that reveal root cause
- **What's affected** - All related resources impacted by the incident

**Time to resolution**: Typically 5-15 minutes faster than manual investigation through kubectl and logs.

## Common Incident Scenarios

### Scenario 1: Pods Failing (CrashLoopBackOff)

**Symptoms**: Alert fires indicating pods are in CrashLoopBackOff state.

#### Step 1: Identify Affected Resources

**Query Spectre** for recent pod events:

```
Query: kind:Pod,status:Error,namespace:production
Time range: Last 30 minutes
```

**UI**: Navigate to Spectre UI → Enter query → View timeline

**API** (programmatic):
```bash
START=$(date -d '30 minutes ago' +%s)
END=$(date +%s)

curl "http://spectre:8080/api/search?query=kind:Pod,status:Error,namespace:production&start=$START&end=$END" | jq .
```

**Expected output**:
```json
{
  "events": [
    {
      "timestamp": "2024-12-12T10:05:45Z",
      "kind": "Pod",
      "name": "api-server-85f6c9b8-k4x2p",
      "namespace": "production",
      "status": "Error",
      "message": "Back-off restarting failed container",
      "count": 15
    }
  ]
}
```

#### Step 2: Find the Deployment

**Query parent Deployment**:

```
Query: kind:Deployment,namespace:production
Filter: Related to failing pods (check ownerReferences)
```

**Timeline view** shows deployment events before pod failures.

#### Step 3: Identify What Changed

**Query recent changes** to the deployment:

```
Query: kind:Deployment,name:api-server,namespace:production
Time range: 1 hour ago to now
```

**Look for**:
- Image updates
- ConfigMap or Secret references changed
- Resource limit adjustments
- Replica count changes

**Example finding**:
```
[10:04:12] Deployment updated: image v1.2.0 → v1.3.0
[10:05:45] Pods started failing (2 minutes later)
```

#### Step 4: Check Related Resources

**Query ConfigMaps and Secrets**:

```
Query: kind:ConfigMap,namespace:production
Or: kind:Secret,namespace:production
Time range: 1 hour ago to now
```

**Common root causes revealed**:
- ConfigMap deleted → pods can't start
- Secret expired → authentication failures
- ConfigMap updated incorrectly → missing required keys

#### Step 5: Review Pod Events

**Query Kubernetes events for the pod**:

```
Query: involvedObject:api-server-85f6c9b8-k4x2p
```

**Critical event messages**:
- "Back-off restarting failed container" → Container crash
- "Failed to pull image" → Registry issues
- "Error: configmap not found" → Missing config
- "OOMKilled" → Memory limit exceeded

#### Step 6: Determine Root Cause

**Correlate timeline**:
1. What changed (deployment, config)?
2. When did failures start?
3. What error messages appear?

**Example correlation**:
```
[10:04:00] ConfigMap/api-config updated (added DATABASE_URL)
[10:04:12] Deployment updated (references api-config)
[10:04:45] Pods failing with "environment variable DATABASE_URL not set"

Root cause: ConfigMap update added DATABASE_URL, but deployment
template doesn't mount it as environment variable.
```

#### Step 7: Take Action

**Immediate mitigation**:
```bash
# Option 1: Rollback deployment
kubectl rollout undo deployment/api-server -n production

# Option 2: Fix configuration
kubectl set env deployment/api-server DATABASE_URL="$(kubectl get configmap api-config -n production -o jsonpath='{.data.DATABASE_URL}')"

# Option 3: Fix ConfigMap
kubectl edit configmap api-config -n production
# Remove DATABASE_URL or fix deployment
```

**Verify recovery**:
```bash
# Watch pods recover
kubectl get pods -n production -w

# Query Spectre for new events
# Should see pods transitioning to Running state
```

### Scenario 2: Service Unavailable

**Symptoms**: Service returns 503 errors, no endpoints available.

#### Investigation Steps

**1. Check Service events**:
```
Query: kind:Service,name:api-server,namespace:production
```

**2. Find related Endpoint events**:
```
Query: kind:Endpoints,name:api-server,namespace:production
```

**Expected finding**:
```
[10:05:00] Endpoints removed (0 ready pods)
```

**3. Investigate why pods aren't ready**:
```
Query: kind:Pod,namespace:production,label:app=api-server
Status filter: Not Ready
```

**Common causes**:
- Readiness probe failing
- Pods stuck in CrashLoopBackOff
- Pods in ImagePullBackOff

**4. Check probe configuration**:
```bash
kubectl get deployment api-server -o yaml | grep -A5 readinessProbe
```

**5. Review pod logs** (Spectre shows events, kubectl shows logs):
```bash
kubectl logs <pod-name> -n production
```

**6. Correlate with Spectre timeline**:
- When did endpoints disappear?
- What changed before that?
- Are there deployment or config events?

### Scenario 3: Deployment Stuck

**Symptoms**: Deployment rollout not progressing, old and new pods coexist.

#### Investigation Steps

**1. Query Deployment events**:
```
Query: kind:Deployment,name:api-server,namespace:production
Status: Warning or Error
```

**2. Check ReplicaSet events**:
```
Query: kind:ReplicaSet,namespace:production,label:app=api-server
Time range: Last hour
```

**3. Identify stuck pods**:
```
Query: kind:Pod,namespace:production,status:Pending
Or: kind:Pod,namespace:production,status:ContainerCreating
```

**Common reasons**:
- Insufficient resources (CPU/memory)
- ImagePullBackOff on new version
- PVC mount failures
- Node selector not matching

**4. Check resource quota**:
```
Query: kind:ResourceQuota,namespace:production
```

**Event message might show**:
```
"Failed to create pod: exceeded quota"
```

**5. Check Node events** (if resource constraints):
```
Query: kind:Node
Status: Warnings related to capacity
```

## Using MCP for Faster Investigation

### Natural Language Investigation

**With Claude Desktop + MCP**:

```
You: Pods are failing in production namespace. Investigate.

Claude: [Automatically queries Spectre via MCP]
I found 3 pods in Error state (CrashLoopBackOff).
The deployment was updated 10 minutes ago.

Timeline:
[10:04:12] Deployment updated: image v1.2→v1.3
[10:05:45] Pods started failing
[10:06:00] Event: "Failed to pull image v1.3"

Root cause: Image v1.3 doesn't exist in registry or
authentication is failing.

Immediate fix:
kubectl rollout undo deployment/api-server -n production
```

**MCP benefits**:
- Automatic event correlation
- Natural language queries
- Suggested remediation steps
- No need to remember query syntax

**Learn more**: [MCP Integration Guide](../mcp-integration/index.md)

## Best Practices

### ✅ Do

- **Start broad, then narrow**: Query all pods in namespace first, then drill down to specific resources
- **Check timelines visually**: Use Spectre UI timeline view to see event correlation
- **Query related resources**: Check ConfigMaps, Secrets, and Services referenced by failing pods
- **Look for patterns**: Multiple pods failing at once suggests deployment or config issue
- **Verify with kubectl**: Confirm Spectre findings with `kubectl describe` and `kubectl logs`
- **Document findings**: Export Spectre timeline for incident reports

### ❌ Don't

- **Don't ignore time windows**: Narrow time ranges to incident period for faster queries
- **Don't skip config resources**: ConfigMap/Secret changes often cause pod failures
- **Don't forget node events**: Node issues can cause cluster-wide pod failures
- **Don't rely only on Spectre**: Use `kubectl logs` for application error details
- **Don't ignore recurring patterns**: Repeated failures at specific times indicate systemic issues

## Query Examples

### Find Recent Failures

```
Query: status:Error
Time range: Last 1 hour
Namespace: production
```

### Track Deployment Timeline

```
Query: kind:Deployment,name:api-server
Time range: 2 hours ago to now
```

### Find Config Changes

```
Query: kind:ConfigMap OR kind:Secret
Time range: Last 4 hours
Namespace: production
```

### All Events for Resource

```
Query: name:api-server-85f6c9b8-k4x2p
Time range: All available
```

### Cross-Namespace Issues

```
Query: kind:Node,status:Warning
Time range: Last 30 minutes
```

## Troubleshooting Tips

### Empty Results

**Problem**: Query returns no events

**Possible causes**:
- Time window doesn't overlap with event occurrence
- Namespace filter too restrictive
- Resource kind or name misspelled
- Spectre hasn't indexed events yet (check watcher is running)

**Solution**:
```bash
# Check Spectre is indexing
kubectl logs -n spectre-system deployment/spectre | grep "indexed"

# Widen time window
# Remove namespace filter
# Check exact resource name with kubectl
```

### Too Many Results

**Problem**: Query returns thousands of events

**Solution**:
- Narrow time window
- Add namespace filter
- Filter by status (Error, Warning only)
- Use specific resource names

### Correlation Confusion

**Problem**: Can't identify which event caused the issue

**Solution**:
- Sort by timestamp in UI
- Look for status transitions (Ready → Error)
- Check for events 1-5 minutes before failures
- Focus on Deployment, ConfigMap, Secret changes
- Use MCP for automatic correlation

## Integration with Monitoring

### Prometheus AlertManager

**Workflow**:
1. Alert fires with timestamp
2. Runbook includes Spectre query link
3. Query Spectre for events ±15 minutes around alert time
4. Correlate metric spike with Kubernetes events

**Example runbook entry**:
```
Runbook: High Pod Restart Rate

1. Check metrics: <grafana-link>
2. Check events: http://spectre/search?query=kind:Pod,namespace:production&start={{alert_time-15m}}&end={{alert_time+15m}}
3. Look for: Deployment updates, ConfigMap changes, OOMKilled events
```

### PagerDuty

**Workflow**:
1. PagerDuty alert includes namespace and resource
2. Operator queries Spectre with alert details
3. Timeline reveals root cause
4. Resolution time documented in PagerDuty

**Example alert enrichment**:
```json
{
  "incident_key": "prod-pod-failures",
  "description": "3 pods failing in production",
  "details": {
    "namespace": "production",
    "deployment": "api-server",
    "spectre_query": "kind:Pod,namespace:production,status:Error"
  }
}
```

## Related Documentation

- [Post-Mortem Analysis](./post-mortem-analysis.md) - Document incidents after resolution
- [Deployment Tracking](./deployment-tracking.md) - Monitor rollouts proactively
- [MCP Integration](../mcp-integration/index.md) - AI-assisted investigations
- [User Guide](../user-guide/querying-events.md) - Master Spectre query syntax

<!-- Source: README.md, MCP examples, incident investigation patterns -->
