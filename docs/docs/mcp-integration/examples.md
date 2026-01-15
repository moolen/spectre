---
title: MCP Real-World Examples
description: Complete investigation scenarios demonstrating Spectre MCP capabilities
keywords: [examples, mcp, claude, scenarios, investigations, troubleshooting, post-mortem]
---

# MCP Real-World Examples

Practical scenarios demonstrating how to use Spectre's MCP integration for Kubernetes troubleshooting, incident investigation, and post-mortem analysis.

## Overview

This guide presents **complete, end-to-end examples** showing:
- Claude Desktop conversations (natural language)
- Direct HTTP API calls (programmatic access)
- Expected tool outputs
- How to interpret results and take action

**Scenarios covered**:
1. CrashLoopBackOff Investigation
2. Post-Mortem Analysis (Complete Workflow)
3. Live Incident Handling
4. Resource Change Correlation
5. Multi-Namespace Investigation
6. Custom Resource Investigation (Flux GitRepository)

## Example 1: CrashLoopBackOff Investigation

**Scenario**: You receive an alert that pods are failing in the production namespace. You don't know which pods or why.

### Via Claude Desktop

**You**:
```
I got an alert about pods failing in production. Can you investigate?
```

**Claude** (Step 1 - Discovery):
```
[Automatically calling cluster_health tool]

I found issues in your production namespace. Let me investigate...

Cluster Health Status: Warning

Resources in Production Namespace:
- Pods: 8 total (5 healthy, 3 error)
- Deployments: 3 total (2 healthy, 1 warning)
- Services: 4 total (all healthy)

Top Issues:
1. Pod "api-server-85f6c9b8-k4x2p" - Error (CrashLoopBackOff)
2. Pod "api-server-85f6c9b8-m9n4r" - Error (CrashLoopBackOff)
3. Pod "api-server-85f6c9b8-p2q7t" - Error (CrashLoopBackOff)

All three pods belong to Deployment "api-server". Would you like me to investigate
the deployment timeline to see what changed?
```

**You**:
```
Yes, investigate the deployment
```

**Claude** (Step 2 - Investigation):
```
[Calling investigate tool for Deployment/production/api-server]

Timeline for api-server Deployment:

[10:05:12] Status: Ready → Warning
           Image updated: registry.example.com/api-server:v1.2.0 → v1.3.0

[10:05:45] New pods failed to start
           Event: "Back-off restarting failed container"

[10:06:30] Status: Warning (ReplicaFailure)
           Event: "Failed to pull image registry.example.com/api-server:v1.3.0:
                   authentication required"

Root Cause Analysis:
The deployment updated to image v1.3.0, but pods cannot pull the image due
to authentication failure. This is likely an imagePullSecrets issue.

Immediate Actions:
1. Verify imagePullSecrets are configured:
   kubectl get deployment api-server -n production -o yaml | grep imagePullSecrets

2. Check if secret exists:
   kubectl get secret regcred -n production

3. Rollback deployment:
   kubectl rollout undo deployment/api-server -n production

4. Check image exists in registry:
   docker pull registry.example.com/api-server:v1.3.0
```

**You**:
```
[Runs kubectl commands, confirms secret is missing]
[Creates secret and redeploys]
Thanks! That solved it.
```

### Via HTTP API (Programmatic)

Same investigation without Claude:

**Step 1: Cluster Health Check**

```bash
curl -X POST http://localhost:8081/mcp/v1 \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"method\": \"tools/call\",
    \"params\": {
      \"name\": \"cluster_health\",
      \"arguments\": {
        \"start_time\": $(date -d '1 hour ago' +%s),
        \"end_time\": $(date +%s),
        \"namespace\": \"production\"
      }
    },
    \"id\": 1
  }"
```

**Response** (abbreviated):
```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [{
      "type": "text",
      "text": "{\"overall_status\":\"Warning\",\"resources_by_kind\":[{\"kind\":\"Pod\",\"total\":8,\"healthy\":5,\"error\":3}],\"top_issues\":[{\"kind\":\"Pod\",\"name\":\"api-server-85f6c9b8-k4x2p\",\"namespace\":\"production\",\"status\":\"Error\",\"message\":\"CrashLoopBackOff\"}]}"
    }]
  },
  "id": 1
}
```

**Step 2: Investigate Deployment**

```bash
curl -X POST http://localhost:8081/mcp/v1 \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"method\": \"tools/call\",
    \"params\": {
      \"name\": \"investigate\",
      \"arguments\": {
        \"resource_kind\": \"Deployment\",
        \"resource_name\": \"api-server\",
        \"namespace\": \"production\",
        \"start_time\": $(date -d '1 hour ago' +%s),
        \"end_time\": $(date +%s),
        \"investigation_type\": \"incident\"
      }
    },
    \"id\": 2
  }"
```

**Response** includes timeline with image update and failure events.

## Example 2: Post-Mortem Analysis (Complete Workflow)

**Scenario**: Yesterday there was a 15-minute outage. You need a comprehensive post-mortem report.

### Via Claude Desktop

**You**:
```
Run a post-mortem analysis for the incident yesterday from 14:00 to 14:30 UTC
in the production namespace
```

**Claude** (Using `post_mortem_incident_analysis` prompt):
```
[Executing post-mortem analysis prompt with:
  start_time: 2024-12-11 14:00 UTC
  end_time: 2024-12-11 14:30 UTC
  namespace: production]

## Incident Post-Mortem Report

### Incident Summary

Service outage in production namespace caused by ConfigMap deletion,
affecting api-server deployment for 15 minutes. Service was restored
after ConfigMap was manually recreated from backup.

### Timeline

[14:00:05] **ConfigMap/production/api-config** - Deleted
           Impact Score: 0.75 (high)

[14:02:18] **Deployment/production/api-server** - Triggered rolling update
           (ConfigMap referenced in pod spec)

[14:02:45] **Pod/production/api-server-7d9f8c5b-x7k2p** - Status: Running → Error
           Event: "Error: ConfigMap api-config not found"

[14:02:52] **Pod/production/api-server-7d9f8c5b-z9k3p** - Status: Running → Error
           (Same error)

[14:03:00] **Service/production/api-server** - No ready endpoints
           All pods unhealthy

[14:15:30] **ConfigMap/production/api-config** - Created (restored from backup)

[14:15:55] **Pod/production/api-server-9c8f7b6d-a3m5n** - Status: Running
           (New pod started successfully)

[14:16:10] **Service/production/api-server** - Endpoints restored
           Service traffic resumed

### Root Cause Analysis

**Primary Cause**:
ConfigMap "api-config" was deleted, causing all pods referencing it to fail
during the subsequent rolling update.

**Contributing Factors**:
1. No version control or backup system for ConfigMaps
2. Deployment configuration requires ConfigMap, but has no failure handling
3. No RBAC restrictions preventing ConfigMap deletion
4. No monitoring alerts on ConfigMap changes
5. Manual restoration took 15 minutes (no documented procedure)

**Evidence**:
- resource_changes tool: ConfigMap deletion at 14:00:05 (impact_score: 0.75)
- investigate tool: Pods transitioned Running → Error at 14:02:45
- Kubernetes events: "ConfigMap api-config not found" (count: 24)

### Impact Assessment

**Service Impact**:
- Total downtime: 15 minutes 55 seconds
- Affected services: api-server (100% unavailable)
- Error rate: 100% during outage

**Resources Affected**:
- 1 ConfigMap (deleted)
- 1 Deployment (failed rollout)
- 3 Pods (failed to start)
- 1 Service (no endpoints)

**User Impact**:
- API requests returned 503 errors
- Estimated affected users: ~500 (based on typical traffic)

### Recommendations

**Immediate Actions**:
1. ✅ Restore ConfigMap from backup (completed)
2. ✅ Verify all pods are healthy (completed)
3. ⏳ Document ConfigMap restoration procedure

**Short-Term (This Week)**:
1. Implement GitOps for ConfigMap management (FluxCD/ArgoCD)
2. Add RBAC policy restricting ConfigMap deletions in production
3. Set up alerts for ConfigMap changes (via Prometheus or kube-state-metrics)
4. Add readiness probes with longer failure thresholds

**Long-Term (This Quarter)**:
1. Implement automated ConfigMap backup system
2. Add application fallback to default configuration values
3. Create runbook for ConfigMap-related incidents
4. Review all deployments for critical ConfigMap dependencies

### Data Gaps & Follow-Up Investigation

To complete the analysis, the following information is needed:

1. **Who deleted the ConfigMap?**
   ```
   kubectl logs -n kube-system -l component=kube-apiserver \
     | grep "ConfigMap.*api-config.*delete"
   ```

2. **Application error logs**:
   ```
   kubectl logs -n production api-server-7d9f8c5b-x7k2p --previous
   ```

3. **Monitoring data**:
   - Check Prometheus for request rate drop at 14:03
   - Review alerting timeline (did alerts fire?)
   - External monitoring (Datadog, New Relic) for user impact

4. **Related changes**:
   ```
   git log --since="2024-12-11 13:00" --until="2024-12-11 14:00" \
     -- k8s/production/configmaps/
   ```

### Lessons Learned

**What Went Well**:
- Incident was detected within 3 minutes
- ConfigMap backup was available
- Team restored service within 16 minutes

**What Went Wrong**:
- No prevention mechanisms for accidental deletion
- No automated recovery process
- Lack of graceful degradation in application

**Action Items**:
- [ ] @ops-team: Implement GitOps for ConfigMaps (by Dec 18)
- [ ] @ops-team: Add RBAC restrictions (by Dec 15)
- [ ] @dev-team: Add ConfigMap change alerts (by Dec 20)
- [ ] @dev-team: Implement application fallback config (by Q1 2025)
- [ ] @all: Review and update incident runbooks (by Dec 22)
```

### Via HTTP API

Use the `post_mortem_incident_analysis` prompt via HTTP:

```bash
# Convert timestamps
START_TIME=$(date -u -d "2024-12-11 14:00:00" +%s)
END_TIME=$(date -u -d "2024-12-11 14:30:00" +%s)

curl -X POST http://localhost:8081/mcp/v1 \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"method\": \"prompts/get\",
    \"params\": {
      \"name\": \"post_mortem_incident_analysis\",
      \"arguments\": {
        \"start_time\": $START_TIME,
        \"end_time\": $END_TIME,
        \"namespace\": \"production\",
        \"incident_description\": \"API service outage\"
      }
    },
    \"id\": 3
  }"
```

Response includes the prompt that instructs the LLM to execute the 9-step post-mortem workflow.

## Example 3: Live Incident Handling

**Scenario**: Pods are failing RIGHT NOW. You need immediate guidance.

### Via Claude Desktop

**You**:
```
Pods in production namespace are failing right now. Help me troubleshoot!
```

**Claude** (Using `live_incident_handling` prompt):
```
[Executing live incident handling prompt with:
  incident_start_time: now - 10 minutes (estimated)
  namespace: production]

## Live Incident Triage

### Current Situation (as of 10:15:30)

Incident started approximately 10 minutes ago (10:05).

**Critical Issues**:
- 5 Pods in Error state (all api-server pods)
- Deployment "api-server" in Warning state
- Service "api-server" has 0/5 ready endpoints

### Recent Changes (Last 15 Minutes)

[Calling resource_changes tool]

High-impact changes detected:

1. **[10:04:12] Deployment/production/api-server**
   - Image updated: v1.2.0 → v1.3.0
   - Impact Score: 0.80 (very high)
   - Status transitions: Ready → Warning

2. **[10:04:05] ConfigMap/production/api-config**
   - ConfigMap updated (3 fields changed)
   - Impact Score: 0.45 (medium)

### Timeline of Failures

[Calling investigate tool for Deployment/api-server]

[10:04:12] Deployment updated (image + ConfigMap reference)
[10:04:45] First pod failed to start
           Event: "Error: environment variable DATABASE_URL not set"
[10:05:10] All pods failing
           Status: CrashLoopBackOff (restart count: 8)

### Root Cause (Likely)

The deployment update introduced a new image (v1.3.0) that requires
environment variable DATABASE_URL, but this variable is not defined
in the updated ConfigMap.

**Evidence**:
- Pod events show "environment variable DATABASE_URL not set"
- ConfigMap was updated 7 seconds before deployment
- New image version likely added new required config

### Immediate Mitigation Steps

**Option 1: Rollback Deployment (Fastest - 30-60 seconds)**
```bash
kubectl rollout undo deployment/api-server -n production
```

**Option 2: Fix ConfigMap (If rollback not preferred)**
```bash
# Add missing variable to ConfigMap
kubectl edit configmap api-config -n production
# Add: DATABASE_URL: "postgresql://..."

# Restart deployment to pick up new config
kubectl rollout restart deployment/api-server -n production
```

**Option 3: Emergency Patch**
```bash
# Add env var directly to deployment
kubectl set env deployment/api-server -n production \
  DATABASE_URL="postgresql://postgres:5432/api"
```

### Recommended Action

**ROLLBACK NOW** (Option 1) for fastest recovery, then fix the configuration
issue before re-deploying v1.3.0.

### Monitoring for Recovery

After executing rollback:

```bash
# Watch pod status (should become Running in 30-60 sec)
kubectl get pods -n production -l app=api-server -w

# Verify service endpoints restored
kubectl get endpoints api-server -n production

# Check for new errors
kubectl get events -n production --sort-by='.lastTimestamp' | head -20
```

**Expected Recovery Time**: 30-60 seconds after rollback

### Follow-Up Actions (After Recovery)

1. Verify rollback succeeded:
   ```
   kubectl rollout status deployment/api-server -n production
   ```

2. Check why v1.3.0 requires DATABASE_URL:
   ```
   # Review image changelog or contact dev team
   ```

3. Update ConfigMap with missing variable

4. Test v1.3.0 in staging environment before re-deploying

5. Run full post-mortem analysis:
   "Run post_mortem_incident_analysis from 10:00 to 10:20"

### Additional Investigation Needed

If rollback doesn't resolve the issue:

1. Check container logs:
   ```
   kubectl logs api-server-85f6c9b8-k4x2p -n production --previous
   ```

2. Verify image exists and is accessible:
   ```
   kubectl describe pod <pod-name> -n production | grep -A5 Events
   ```

3. Check for resource constraints:
   ```
   kubectl top pods -n production
   kubectl describe nodes
   ```
```

## Example 4: Resource Change Correlation

**Scenario**: You notice degraded performance. You want to see what changed in the last 30 minutes.

### Via Claude Desktop

**You**:
```
Show me all high-impact changes in the production namespace
from 30 minutes ago to now
```

**Claude**:
```
[Calling resource_changes tool]

High-impact changes in production (last 30 minutes):

### Changes with Impact Score > 0.5

1. **Deployment/production/api-server** (Impact: 0.80)
   - Changed at: 10:04:12
   - Image: v1.2.0 → v1.3.0
   - Status transitions: Ready → Warning → Ready
   - Error events: 15
   - Rollback performed at 10:08:30

2. **ConfigMap/production/api-config** (Impact: 0.65)
   - Changed at: 10:04:05
   - 3 fields modified
   - Associated with deployment failure above

3. **HPA/production/api-server-hpa** (Impact: 0.55)
   - Changed at: 10:15:00
   - Scaled: 3 → 5 replicas (due to increased load during incident)

### Changes with Impact Score 0.3-0.5

4. **Service/production/api-server** (Impact: 0.40)
   - Warning events: 5
   - Endpoints removed/restored (correlated with pod failures)

5. **Pod/production/cache-redis-0** (Impact: 0.35)
   - Warning at: 10:12:15
   - High restart count (3 restarts)
   - Event: "Liveness probe failed"

### Correlation Analysis

The timeline suggests:
1. ConfigMap updated (10:04:05)
2. Deployment updated shortly after (10:04:12)
3. Pods failed due to missing configuration (10:04:45)
4. Service unavailable (10:05:00)
5. Rollback performed (10:08:30)
6. HPA scaled up (10:15:00) to handle recovery traffic

The redis pod restarts (10:12:15) may be a side effect of the increased
load during the incident, or a separate issue. Would you like me to
investigate the redis pod?
```

## Example 5: Multi-Namespace Investigation

**Scenario**: You suspect an issue is affecting multiple namespaces.

### Via Claude Desktop

**You**:
```
Check cluster health across all namespaces and identify any problems
```

**Claude**:
```
[Calling cluster_health without namespace filter]

Cluster-Wide Health Status: Warning

### Overall Summary

- Total resources tracked: 247
- Healthy: 228 (92%)
- Warning: 14 (6%)
- Error: 5 (2%)

### Issues by Namespace

**production** (3 errors, 2 warnings):
- Pods: 3 in Error (api-server deployment)
- Deployments: 1 in Warning

**staging** (2 errors, 5 warnings):
- Pods: 2 in Error (database-migration job)
- Jobs: 1 Failed

**monitoring** (0 errors, 4 warnings):
- Pods: 4 in Warning (prometheus-server high memory)

**kube-system** (0 errors, 3 warnings):
- Pods: 3 in Warning (coredns high restart count)

### Top Priority Issues

1. **production/api-server** - CrashLoopBackOff (3 pods)
   → Needs immediate attention

2. **staging/database-migration** - Job failed (2 pods)
   → Check migration logs

3. **monitoring/prometheus-server** - High memory usage
   → May affect monitoring reliability

4. **kube-system/coredns** - High restart count
   → Potential DNS issues

Would you like me to investigate any of these in detail?
```

**You**:
```
Yes, investigate the coredns issue. It might be causing problems across namespaces.
```

**Claude**:
```
[Calling investigate tool for coredns pods]

CoreDNS Investigation:

Timeline (last hour):

[09:30] coredns-7d8b5f9c6b-x7k2p - Restart count: 0 → 3
        Events: "Liveness probe failed" (3 times)

[09:45] coredns-7d8b5f9c6b-z9k3p - Restart count: 0 → 2
        Events: "Liveness probe failed" (2 times)

Root Cause Indicators:
- High restart count (5 total across 2 pods in 30 minutes)
- All failures are liveness probe failures
- Coincides with increased DNS query rate (per events)

Investigation Prompts:
1. Why are the CoreDNS liveness probes failing?
   - Check if DNS resolution is slow (timeout issues)
   - Review CoreDNS resource limits (CPU/memory constraints)

2. Is this affecting other services?
   - DNS failures would cause pod startup issues cluster-wide
   - May explain some of the errors in production/staging

Recommended Actions:
1. Check CoreDNS resource usage:
   kubectl top pods -n kube-system -l k8s-app=kube-dns

2. Review CoreDNS logs for errors:
   kubectl logs -n kube-system -l k8s-app=kube-dns --tail=100

3. Increase CoreDNS resource limits if needed:
   kubectl edit deployment coredns -n kube-system

4. Consider scaling CoreDNS replicas:
   kubectl scale deployment coredns -n kube-system --replicas=3

Note: DNS issues are cluster-wide and high priority!
```

## Example 6: Custom Resource Investigation (Flux GitRepository)

**Scenario**: Your Flux GitRepository resource is failing, and you don't know why.

### Via Claude Desktop

**You**:
```
Investigate the GitRepository resource flux-system/main-repo that's been failing
for the last 2 hours
```

**Claude**:
```
[Calling investigate tool for GitRepository/flux-system/main-repo]

GitRepository Investigation: flux-system/main-repo

Timeline (last 2 hours):

[08:15:00] Status: Ready
           Revision: main@sha1:a3b4c5d6

[08:45:30] Status: Ready → Error
           Event: "Failed to checkout repository: authentication required"
           Revision: Failed to fetch

[09:00:15] Reconciliation attempts: 8 (all failed)
           Event: "SSH key authentication failed"

[09:15:45] Status: Still Error
           Event: "Unable to clone: git@github.com:org/repo.git"

Root Cause Analysis:

The GitRepository resource is failing to authenticate with GitHub
using SSH keys. This started at 08:45:30 and has persisted for
~1.5 hours.

Investigation Prompts:
1. Why did SSH authentication suddenly fail?
   - Was the SSH key rotated or revoked?
   - Did the GitHub repository change permissions?
   - Is the Secret containing SSH keys still present?

2. Are other GitRepository resources affected?
   - This may indicate a cluster-wide credential issue

Immediate Actions:

1. Check if the Secret exists:
   kubectl get secret flux-system-ssh -n flux-system

2. Verify Secret has correct SSH key:
   kubectl get secret flux-system-ssh -n flux-system -o yaml | grep -A5 identity

3. Test Git access manually:
   kubectl run debug-git --rm -it --image=alpine/git -- \
     git ls-remote git@github.com:org/repo.git

4. If key is missing or expired, recreate Secret:
   flux create secret git flux-system-ssh \
     --url=ssh://git@github.com/org/repo.git \
     --private-key-file=/path/to/new/key

5. Force reconciliation:
   flux reconcile source git main-repo

Expected Recovery: Immediate after Secret is fixed

Would you like me to check other GitRepository resources to see if this is
a widespread issue?
```

## Integration Patterns

Common ways to integrate MCP tools into workflows:

### Pattern 1: Alert → Investigation

**Trigger**: Prometheus/Alertmanager alert fires

**Workflow**:
1. Alert notification includes namespace and resource
2. Operator asks Claude: "Investigate <resource> in <namespace>"
3. Claude uses `investigate` tool to provide timeline and RCA prompts
4. Operator runs suggested kubectl commands
5. Issue is resolved or escalated

**Example**:
```
[PagerDuty Alert: High Pod Restart Rate - production/api-server]

You: Investigate deployment api-server in production namespace

Claude: [investigate tool]
        Timeline shows image update 10 minutes ago...
        [Provides immediate mitigation steps]

You: [Runs rollback command from Claude's suggestion]
```

### Pattern 2: Incident → Post-Mortem

**Trigger**: Incident is resolved, need documentation

**Workflow**:
1. Incident resolution completed
2. Operator asks Claude: "Run post_mortem_incident_analysis from <start> to <end>"
3. Claude executes full post-mortem prompt workflow
4. Claude generates structured report
5. Report is saved to incident management system

**Example**:
```
[Incident resolved at 15:30]

You: Run post-mortem analysis from 14:00 to 15:30 in production

Claude: [post_mortem_incident_analysis prompt]
        [Generates full report with timeline, RCA, recommendations]

You: [Copies report to Jira/GitHub/Confluence]
```

### Pattern 3: Maintenance Window Verification

**Trigger**: After deploying changes

**Workflow**:
1. Deploy changes (deployment, configmap, etc.)
2. Ask Claude: "Show me changes in the last 15 minutes"
3. Claude uses `resource_changes` to identify what actually changed
4. Verify expected changes occurred, no unexpected side effects

**Example**:
```
[Deployed v2.0 to production at 16:00]

You: Show high-impact changes in production in last 15 minutes

Claude: [resource_changes tool]
        Deployment api-server updated: v1.9 → v2.0
        ConfigMap api-config updated (expected)
        HPA scaled 3 → 5 replicas (expected due to new load)

        No unexpected errors detected.

You: Great, deployment looks clean!
```

### Pattern 4: Proactive Health Check

**Trigger**: Daily/weekly routine check

**Workflow**:
1. Operator asks Claude: "Check cluster health for the last 24 hours"
2. Claude uses `cluster_health` to identify any warnings or errors
3. Operator addresses issues proactively before they become incidents

**Example**:
```
[Monday morning routine]

You: Check cluster health across all namespaces for the last 24 hours

Claude: [cluster_health tool]
        Overall: Healthy

        Minor issues detected:
        - staging/database: 1 pod in Warning (high memory usage)
        - monitoring/prometheus: 2 restarts (known issue, non-critical)

        No action required, but consider increasing database memory limits.

You: Noted, will increase memory in next maintenance window.
```

## Claude Desktop Conversation Tips

### Be Specific with Time Windows

**❌ Vague**:
```
Check for issues recently
```

**✅ Specific**:
```
Check cluster health in the production namespace for the last 30 minutes
```

### Provide Context

**❌ Minimal**:
```
Investigate pod api-server-x7k2p
```

**✅ With Context**:
```
Investigate pod api-server-x7k2p in production namespace.
We deployed a new version 15 minutes ago and it started failing.
```

### Ask for Specific Tools

**❌ Unclear**:
```
Tell me what happened
```

**✅ Tool-specific**:
```
Use resource_changes to show me what changed in production in the last hour
```

### Follow Up with "Why" Questions

**Good conversation flow**:
```
You: Check production namespace health

Claude: [Shows 3 pods in error]

You: Why are those pods failing?

Claude: [Uses investigate tool]
        They failed after a deployment update...

You: What was updated in the deployment?

Claude: [Shows configuration change details]
```

## Troubleshooting Common Scenarios

### Empty Results

**Problem**: Tool returns no data or empty results

**Cause**: Spectre may not have data for the requested time window

**Solution**:
```
You: I'm getting empty results. Does Spectre have data for the last hour?

Claude: [Adjusts time window]
        Let me check a longer time period...
        [Calls cluster_health with 24-hour window]

        I found data from 6 hours ago, but nothing in the last hour.
        This suggests Spectre may not be actively indexing events.

        Check if Spectre is running: kubectl get pods -n spectre-system
```

### Incomplete Analysis

**Problem**: Claude's analysis seems superficial

**Cause**: Need to dig deeper with multiple tools

**Solution**:
```
You: That analysis seems incomplete. Can you investigate further?

Claude: You're right. Let me use additional tools...
        [Calls investigate tool for related resources]
        [Calls resource_changes to find correlated events]
```

### Claude Doesn't Use Tools

**Problem**: Claude responds without calling MCP tools

**Cause**: Question wasn't framed to trigger tool use

**Solution**:
```
❌ Bad: "What might cause pods to fail?"
       (Claude gives generic answer without using tools)

✅ Good: "Use cluster_health to check if any pods are currently failing"
       (Claude calls tool)
```

## Best Practices from Examples

### ✅ Do

- **Start broad, then narrow**: Use `cluster_health` first, then `investigate` specific resources
- **Specify namespaces**: Faster queries when scoped to one namespace
- **Use time windows**: "Last 30 minutes" is more efficient than "last 7 days"
- **Follow Claude's suggestions**: Run kubectl commands Claude recommends
- **Chain investigations**: Use results from one tool to inform the next query
- **Save post-mortem reports**: Copy Claude's formatted reports to your docs
- **Verify with kubectl**: Always confirm Claude's findings with live cluster state

### ❌ Don't

- **Don't expect Claude to execute commands**: Claude suggests kubectl commands, you must run them
- **Don't ignore kubectl logs**: Spectre shows events, but container logs are still needed
- **Don't query very old data**: Check Spectre retention period (default: 7 days)
- **Don't rely solely on AI**: Use Claude as an assistant, not a replacement for expertise
- **Don't skip follow-up**: Claude often says "Would you like me to investigate X?" - say yes!
- **Don't ignore data gaps**: Claude will mention "Additional investigation needed" - follow up on those

## Related Documentation

- [Getting Started with MCP](./getting-started.md) - Deploy and verify MCP integration
- [Claude Desktop Integration](./claude-integration.md) - Set up Claude Desktop
- [Tools Reference](./tools-reference/cluster-health.md) - Detailed API docs for all tools
- [Prompts Reference](./prompts-reference/post-mortem.md) - Workflow guides for prompts
- [MCP Configuration](../configuration/mcp-configuration.md) - Production configuration and tuning

<!-- Source: Synthesized from tool and prompt documentation, README.md examples -->
