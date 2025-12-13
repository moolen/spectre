---
title: Deployment Tracking
description: Monitor Kubernetes deployments proactively with real-time event tracking and rollout verification
keywords: [deployment, rollout, tracking, monitoring, verification, ci/cd, kubernetes]
---

# Deployment Tracking

Track deployment rollouts in real-time, detect issues early, and verify successful deployments using Spectre's comprehensive event monitoring.

## Overview

Deployment tracking helps you:
- **Monitor rollout progress** - Real-time visibility into pod creation and readiness
- **Detect issues early** - Catch failures before they impact all replicas
- **Verify success** - Confirm deployments completed without errors
- **Enable fast rollbacks** - Identify problems quickly for faster recovery
- **Integrate with CI/CD** - Automated deployment verification in pipelines

**Time saved**: 3-5 minutes per deployment through automated verification and early issue detection.

## Deployment Lifecycle

A typical Kubernetes deployment involves multiple resource changes that Spectre tracks:

### 1. Deployment Update
**Trigger**: `kubectl apply` or GitOps reconciliation

**Events captured**:
- Deployment spec change (image, replicas, config)
- ReplicaSet creation (new version)
- Old ReplicaSet scale-down (rolling update)

### 2. Pod Rollout
**Trigger**: ReplicaSet controller creates pods

**Events captured**:
- Pod created (Pending state)
- Container image pull
- Pod started (Running state)
- Readiness probe checks

### 3. Service Update
**Trigger**: Pods become ready

**Events captured**:
- Endpoints added (new pods)
- Endpoints removed (old pods)
- Service traffic shift

### 4. Completion
**Trigger**: All new pods ready, old pods terminated

**Events captured**:
- Deployment status: Progressing → Available
- Old ReplicaSet scaled to 0
- Old pods terminated

## Step-by-Step Deployment Tracking

### Step 1: Start Tracking Before Deployment

**Capture baseline** before applying changes:

```bash
# Note current deployment state
kubectl get deployment api-server -n production -o yaml > pre-deployment.yaml

# Record timestamp for Spectre query
DEPLOY_START=$(date +%s)
echo "Deployment started at: $(date -u)"
```

**Query current state** in Spectre:

```
Query: kind:Deployment,name:api-server,namespace:production
Time range: Last 5 minutes
```

**Verify**: Current image version, replica count, and status.

### Step 2: Apply Deployment

**Execute deployment**:

```bash
# Via kubectl
kubectl set image deployment/api-server api-server=v1.3.0 -n production

# Via GitOps (commit and push)
git commit -m "Update api-server to v1.3.0"
git push origin main
# (FluxCD or ArgoCD will reconcile)
```

### Step 3: Monitor Rollout Progress

**Query deployment events** in real-time:

**Spectre UI**:
1. Navigate to Spectre UI
2. Set time range: "Last 15 minutes" (auto-refresh)
3. Query: `kind:Deployment OR kind:ReplicaSet OR kind:Pod,namespace:production,name:api-server`
4. Watch timeline as events appear

**Spectre API** (programmatic):

```bash
# Poll for deployment events every 10 seconds
while true; do
  CURRENT_TIME=$(date +%s)
  curl -s "http://spectre:8080/api/search?query=kind:Deployment,name:api-server,namespace:production&start=$DEPLOY_START&end=$CURRENT_TIME" | \
    jq -r '.events[] | "\(.timestamp) - \(.message)"'
  sleep 10
done
```

**Expected events during healthy rollout**:

```
[10:15:00] Deployment/api-server - Updated: image v1.2.0 → v1.3.0
[10:15:02] ReplicaSet/api-server-7d9f8c5b - Created (new version)
[10:15:05] Pod/api-server-7d9f8c5b-x7k2p - Created (Pending)
[10:15:08] Pod/api-server-7d9f8c5b-x7k2p - Image pulled successfully
[10:15:10] Pod/api-server-7d9f8c5b-x7k2p - Status: Running
[10:15:15] Pod/api-server-7d9f8c5b-x7k2p - Ready (passed readiness probe)
[10:15:18] Endpoints/api-server - Added endpoint (new pod)
[10:15:20] Pod/api-server-9c8f7b6d-a3m5n - Terminating (old version)
[10:15:25] Deployment/api-server - Status: Available (rollout complete)
```

### Step 4: Detect Issues Early

**Query for error events** during rollout:

```
Query: status:Error OR status:Warning,namespace:production
Time range: Since deployment start
```

**Common issues detected**:

#### ImagePullBackOff

**Spectre shows**:
```
[10:15:08] Pod/api-server-7d9f8c5b-x7k2p - Status: Error
           Event: "Failed to pull image v1.3.0: authentication required"
```

**Action**: Image doesn't exist or registry credentials invalid
```bash
# Fix: Verify image tag exists
docker pull registry.example.com/api-server:v1.3.0

# If credentials issue, update secret
kubectl create secret docker-registry regcred \
  --docker-server=registry.example.com \
  --docker-username=user \
  --docker-password=pass \
  -n production
```

#### CrashLoopBackOff

**Spectre shows**:
```
[10:15:12] Pod/api-server-7d9f8c5b-x7k2p - Status: Running
[10:15:15] Pod/api-server-7d9f8c5b-x7k2p - Status: Error
           Event: "Back-off restarting failed container"
```

**Action**: Application crashes on startup
```bash
# Check logs for crash reason
kubectl logs api-server-7d9f8c5b-x7k2p -n production

# Common causes:
# - Missing environment variables
# - Config file errors
# - Database connection failures
```

#### ConfigMap/Secret Not Found

**Spectre shows**:
```
[10:15:10] Pod/api-server-7d9f8c5b-x7k2p - Status: Error
           Event: "Error: configmap 'api-config' not found"
```

**Action**: Referenced config doesn't exist
```bash
# Verify ConfigMap exists
kubectl get configmap api-config -n production

# If missing, create it
kubectl apply -f config/api-config.yaml
```

#### Readiness Probe Failure

**Spectre shows**:
```
[10:15:15] Pod/api-server-7d9f8c5b-x7k2p - Status: Running
[10:16:00] Pod/api-server-7d9f8c5b-x7k2p - Warning: Readiness probe failed
           (Pod never becomes Ready)
```

**Action**: Application healthy but probe misconfigured
```bash
# Check probe definition
kubectl get deployment api-server -n production -o yaml | grep -A10 readinessProbe

# Test probe endpoint
kubectl exec api-server-7d9f8c5b-x7k2p -n production -- curl localhost:8080/health
```

### Step 5: Verify Deployment Success

**Success criteria**:
1. ✅ All new pods in Running state
2. ✅ All new pods passed readiness checks
3. ✅ Endpoints updated with new pods
4. ✅ Old pods terminated
5. ✅ No error events

**Query for verification**:

```
Query: kind:Pod,namespace:production,label:app=api-server
Time range: Last 30 minutes
Status filter: Error OR Warning
```

**Expected result**: No error events, or only old pod termination events.

**Verification script**:

```bash
DEPLOY_END=$(date +%s)

# Query Spectre for errors during deployment window
ERROR_COUNT=$(curl -s "http://spectre:8080/api/search?query=kind:Pod,namespace:production,status:Error&start=$DEPLOY_START&end=$DEPLOY_END" | \
  jq '[.events[] | select(.name | contains("api-server"))] | length')

if [ "$ERROR_COUNT" -eq 0 ]; then
  echo "✅ Deployment successful - No errors detected"
  exit 0
else
  echo "❌ Deployment failed - $ERROR_COUNT error events detected"
  exit 1
fi
```

### Step 6: Rollback Decision

**When to rollback**:
- ❌ New pods failing to start (CrashLoopBackOff, ImagePullBackOff)
- ❌ Readiness probes failing after 5+ minutes
- ❌ Error rate spike in application metrics
- ❌ Multiple pods stuck in Pending state

**Rollback with kubectl**:

```bash
# Option 1: Rollback to previous version
kubectl rollout undo deployment/api-server -n production

# Option 2: Rollback to specific revision
kubectl rollout history deployment/api-server -n production
kubectl rollout undo deployment/api-server -n production --to-revision=3

# Verify rollback success
kubectl rollout status deployment/api-server -n production
```

**Track rollback in Spectre**:

```
Query: kind:Deployment,name:api-server,namespace:production
Time range: Last 15 minutes
```

**Expected events**:
```
[10:20:00] Deployment/api-server - Updated: image v1.3.0 → v1.2.0 (rollback)
[10:20:05] ReplicaSet/api-server-9c8f7b6d - Scaled up (previous version)
[10:20:10] Pods created with v1.2.0 image
[10:20:30] Service traffic shifted back to v1.2.0
```

## CI/CD Pipeline Integration

### Automated Verification

**Integration pattern**:
1. CI/CD applies deployment
2. Wait for rollout to stabilize (30-60 seconds)
3. Query Spectre for error events
4. Pass/fail pipeline based on results

**Example GitLab CI/CD**:

```yaml
deploy:
  stage: deploy
  script:
    # Record deployment start time
    - export DEPLOY_START=$(date +%s)

    # Apply deployment
    - kubectl set image deployment/api-server api-server=${CI_COMMIT_SHORT_SHA} -n production

    # Wait for rollout
    - kubectl rollout status deployment/api-server -n production --timeout=5m

    # Verify with Spectre
    - |
      export DEPLOY_END=$(date +%s)
      ERROR_COUNT=$(curl -s "http://spectre:8080/api/search?query=kind:Pod,namespace:production,status:Error&start=$DEPLOY_START&end=$DEPLOY_END" | \
        jq '[.events[] | select(.name | contains("api-server"))] | length')

      if [ "$ERROR_COUNT" -gt 0 ]; then
        echo "❌ Deployment verification failed - errors detected"
        kubectl rollout undo deployment/api-server -n production
        exit 1
      fi

      echo "✅ Deployment verified successfully"
```

**Example GitHub Actions**:

```yaml
- name: Deploy and Verify
  run: |
    export DEPLOY_START=$(date +%s)

    # Apply deployment
    kubectl set image deployment/api-server api-server=${{ github.sha }} -n production

    # Wait for rollout
    kubectl rollout status deployment/api-server -n production --timeout=5m

    # Query Spectre for verification
    sleep 10  # Allow events to be indexed

    ERROR_EVENTS=$(curl -s "http://spectre:8080/api/search?query=kind:Pod,namespace:production,status:Error&start=$DEPLOY_START&end=$(date +%s)" | jq -r '.events[]')

    if [ -n "$ERROR_EVENTS" ]; then
      echo "::error::Deployment verification failed"
      echo "$ERROR_EVENTS" | jq .
      kubectl rollout undo deployment/api-server -n production
      exit 1
    fi

    echo "::notice::Deployment verified successfully"
```

### Progressive Delivery

**Canary deployments** with Spectre monitoring:

```bash
# Phase 1: Deploy canary (10% traffic)
kubectl set image deployment/api-server-canary api-server=v1.3.0 -n production
kubectl scale deployment/api-server-canary --replicas=1

# Monitor canary with Spectre (5 minutes)
sleep 300

# Query canary pod health
CANARY_ERRORS=$(curl -s "http://spectre:8080/api/search?query=kind:Pod,namespace:production,label:version=canary,status:Error&start=$(date -d '5 minutes ago' +%s)&end=$(date +%s)" | jq '.events | length')

if [ "$CANARY_ERRORS" -eq 0 ]; then
  echo "✅ Canary healthy - Proceeding with full rollout"
  kubectl set image deployment/api-server api-server=v1.3.0 -n production
else
  echo "❌ Canary unhealthy - Aborting rollout"
  kubectl delete deployment api-server-canary -n production
  exit 1
fi
```

## Using MCP for Deployment Tracking

### Conversational Deployment Verification

**With Claude Desktop + MCP**:

```
You: I just deployed api-server to v1.3.0 in production namespace. Verify it's healthy.

Claude: [Automatically uses resource_changes and investigate tools]

I've analyzed the deployment:

✅ Deployment Status: Successful
- Deployment updated at 10:15:00 UTC
- 3 new pods created successfully
- All pods reached Running and Ready state
- Endpoints updated at 10:15:18 UTC
- Old pods terminated cleanly

⚠️ Minor Issues Detected:
- Pod api-server-7d9f8c5b-x7k2p took 2 retry attempts to pull image
  (transient registry issue, resolved)

Overall: Deployment completed successfully. No action needed.
```

**Natural language rollback**:

```
You: Pods are crashing after the deployment. Rollback immediately.

Claude: [Investigates crash events, provides rollback command]

I found CrashLoopBackOff events starting at 10:15:15.
Root cause: Missing DATABASE_URL environment variable in new image.

Immediate rollback command:
kubectl rollout undo deployment/api-server -n production

Then fix the issue by adding DATABASE_URL to deployment spec.
```

### MCP Query Examples

**Check deployment progress**:

```bash
# Via MCP HTTP API
curl -X POST http://localhost:8081/mcp/v1 \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"method\": \"tools/call\",
    \"params\": {
      \"name\": \"resource_changes\",
      \"arguments\": {
        \"start_time\": $(date -d '15 minutes ago' +%s),
        \"end_time\": $(date +%s),
        \"namespace\": \"production\",
        \"kinds\": [\"Deployment\", \"Pod\", \"ReplicaSet\"]
      }
    },
    \"id\": 1
  }"
```

**Investigate failed deployment**:

```bash
curl -X POST http://localhost:8081/mcp/v1 \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"method\": \"tools/call\",
    \"params\": {
      \"name\": \"investigate\",
      \"arguments\": {
        \"kind\": \"Deployment\",
        \"namespace\": \"production\",
        \"name\": \"api-server\",
        \"start_time\": $(date -d '1 hour ago' +%s),
        \"end_time\": $(date +%s)
      }
    },
    \"id\": 2
  }"
```

## Deployment Tracking Queries

### Query All Deployment Activity

```
Query: kind:Deployment OR kind:ReplicaSet
Time range: Last 1 hour
Namespace: production
```

**Use case**: Overview of all deployment changes in timeframe.

### Query Specific Deployment Timeline

```
Query: name:api-server
Time range: 30 minutes ago to now
```

**Use case**: Complete timeline for single deployment (all related resources).

### Query Pod Failures During Deployment

```
Query: kind:Pod,status:Error,namespace:production
Time range: Since deployment start
```

**Use case**: Identify which pods failed and why during rollout.

### Query ConfigMap/Secret Changes

```
Query: kind:ConfigMap OR kind:Secret,namespace:production
Time range: 1 hour ago to now
```

**Use case**: Verify config changes applied before deployment.

### Query Service Endpoint Changes

```
Query: kind:Endpoints,name:api-server,namespace:production
Time range: Last 30 minutes
```

**Use case**: Confirm traffic shifted to new pods.

## Best Practices

### ✅ Do

- **Track before applying** - Query current state before deployment starts
- **Monitor in real-time** - Watch Spectre timeline during rollout (auto-refresh UI)
- **Set time windows** - Use deployment start timestamp for accurate queries
- **Query related resources** - Check Pods, ReplicaSets, Endpoints, and ConfigMaps
- **Verify success explicitly** - Don't assume success, query for errors
- **Integrate with CI/CD** - Automate verification in deployment pipelines
- **Use MCP for triage** - Let AI correlate events and suggest fixes
- **Document deployment windows** - Record start/end times for post-mortem analysis
- **Monitor endpoints** - Ensure service traffic shifts to new pods
- **Check old pod termination** - Verify graceful shutdown of old replicas

### ❌ Don't

- **Don't ignore warnings** - Warning events often precede failures
- **Don't skip verification** - Always check Spectre after deployment completes
- **Don't deploy without baseline** - Know the current state before making changes
- **Don't rely only on kubectl** - `kubectl rollout status` doesn't show event details
- **Don't forget config changes** - ConfigMap/Secret updates often cause deployment issues
- **Don't assume image exists** - ImagePullBackOff is a common early failure
- **Don't ignore readiness failures** - Pods Running ≠ Pods Ready
- **Don't rush rollbacks** - Investigate with Spectre first to confirm root cause
- **Don't forget time zones** - Use UTC timestamps consistently

## Example Deployment Scenarios

### Scenario 1: Successful Deployment

**Timeline**:
```
[14:00:00] Deployment/api-server - Updated: replicas 3→5, image v1.2→v1.3
[14:00:02] ReplicaSet/api-server-abc123 - Created (new version)
[14:00:05] Pod/api-server-abc123-p1 - Created, Status: Pending
[14:00:05] Pod/api-server-abc123-p2 - Created, Status: Pending
[14:00:08] Pod/api-server-abc123-p1 - Image pulled, Status: Running
[14:00:08] Pod/api-server-abc123-p2 - Image pulled, Status: Running
[14:00:12] Pod/api-server-abc123-p1 - Ready (readiness probe passed)
[14:00:13] Pod/api-server-abc123-p2 - Ready (readiness probe passed)
[14:00:15] Endpoints/api-server - Added 2 endpoints
[14:00:18] Pod/api-server-xyz789-old1 - Terminating
[14:00:20] ReplicaSet/api-server-xyz789 - Scaled to 3 (old version)
[14:00:25] Deployment/api-server - Status: Available, Ready: 5/5
```

**Verification**: ✅ All pods Running and Ready, no errors, endpoints updated.

### Scenario 2: Failed Deployment (CrashLoopBackOff)

**Timeline**:
```
[14:10:00] Deployment/api-server - Updated: image v1.3→v1.4
[14:10:02] ReplicaSet/api-server-def456 - Created
[14:10:05] Pod/api-server-def456-p1 - Created, Status: Pending
[14:10:08] Pod/api-server-def456-p1 - Image pulled, Status: Running
[14:10:12] Pod/api-server-def456-p1 - Status: Error (Exit code 1)
[14:10:15] Pod/api-server-def456-p1 - Status: Running (restarted)
[14:10:18] Pod/api-server-def456-p1 - Status: Error (Exit code 1)
[14:10:25] Pod/api-server-def456-p1 - Status: CrashLoopBackOff
```

**Action**: Rollback immediately
```bash
kubectl rollout undo deployment/api-server -n production
```

**Investigation**: Check logs for crash reason
```bash
kubectl logs api-server-def456-p1 -n production --previous
```

### Scenario 3: Config Change Causes Failure

**Timeline**:
```
[14:20:00] ConfigMap/api-config - Updated: DATABASE_URL changed
[14:20:05] Deployment/api-server - Rolling update triggered (ConfigMap mounted)
[14:20:10] Pod/api-server-ghi789-p1 - Created, Status: Running
[14:20:30] Pod/api-server-ghi789-p1 - Warning: Readiness probe failed
           (Application can't connect to new database URL)
[14:21:00] Pod/api-server-ghi789-p1 - Still not Ready (1 minute timeout)
```

**Action**: Revert ConfigMap change
```bash
kubectl edit configmap api-config -n production
# Revert DATABASE_URL to previous value

# Restart deployment to pick up fix
kubectl rollout restart deployment/api-server -n production
```

### Scenario 4: Image Pull Failure

**Timeline**:
```
[14:30:00] Deployment/api-server - Updated: image v1.5
[14:30:02] ReplicaSet/api-server-jkl012 - Created
[14:30:05] Pod/api-server-jkl012-p1 - Created, Status: Pending
[14:30:10] Pod/api-server-jkl012-p1 - Status: ErrImagePull
           Event: "Failed to pull image: manifest unknown"
[14:30:30] Pod/api-server-jkl012-p1 - Status: ImagePullBackOff
```

**Root cause**: Image tag v1.5 doesn't exist in registry

**Action**: Fix image tag
```bash
# Verify correct tag
docker pull registry.example.com/api-server:v1.5
# Error: manifest unknown

# Update to correct tag
kubectl set image deployment/api-server api-server=v1.5.0 -n production
```

## Monitoring Integration

### Prometheus Alerts

**Alert on deployment failures**:

```yaml
- alert: DeploymentRolloutFailed
  expr: kube_deployment_status_replicas_unavailable > 0
  for: 5m
  annotations:
    summary: "Deployment {{ $labels.deployment }} has unavailable replicas"
    runbook: |
      1. Check Spectre for deployment events:
         http://spectre/search?query=kind:Deployment,name={{ $labels.deployment }}

      2. Look for pod errors:
         http://spectre/search?query=kind:Pod,status:Error,namespace={{ $labels.namespace }}

      3. Investigate with MCP or kubectl logs
```

### GitOps Integration (FluxCD)

**Track Flux reconciliation**:

```
Query: kind:GitRepository OR kind:Kustomization,namespace:flux-system
Time range: Last 1 hour
```

**Correlated timeline**:
```
[14:40:00] GitRepository/flux-system/main - Reconciled (new commit: abc123)
[14:40:05] Kustomization/flux-system/apps - Applied changes
[14:40:10] Deployment/api-server - Updated (via Flux)
[14:40:15] Pods start rolling out...
```

**Use case**: Verify GitOps changes propagated correctly from Git to cluster.

## Troubleshooting

### Deployment Not Progressing

**Symptoms**: Deployment stuck, no new pods created

**Spectre query**:
```
Query: kind:Deployment,name:api-server,namespace:production
Time range: Last 30 minutes
```

**Possible causes** (revealed by events):
- ResourceQuota exceeded (Spectre shows quota events)
- Node selector doesn't match any nodes (pod stays Pending)
- PVC mount failure (pod can't start)
- Image pull secrets missing (ImagePullBackOff)

### Pods Created But Not Ready

**Symptoms**: Pods in Running state but not passing readiness checks

**Spectre query**:
```
Query: kind:Pod,namespace:production,name:api-server
Status filter: Warning
```

**Look for**:
- Readiness probe failure events
- Liveness probe killing pods
- Application errors in Events

**Action**: Review probe configuration and application startup

### Endpoints Not Updating

**Symptoms**: Old pods still receiving traffic after deployment

**Spectre query**:
```
Query: kind:Endpoints,name:api-server,namespace:production
Time range: Last 15 minutes
```

**Verify**:
- Endpoints include new pod IPs
- Old pod IPs removed from endpoints
- Service selector matches pod labels

## Related Documentation

- [Incident Investigation](./incident-investigation.md) - Troubleshoot deployment failures
- [Post-Mortem Analysis](./post-mortem-analysis.md) - Document deployment incidents
- [MCP Integration](../mcp-integration/index.md) - AI-assisted deployment verification
- [User Guide](../user-guide/querying-events.md) - Master Spectre query syntax

<!-- Source: README.md deployment tracking, incident investigation patterns, CI/CD integration -->
