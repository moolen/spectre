---
title: Getting Started with MCP
description: End-to-end guide from MCP deployment to your first investigation
keywords: [mcp, setup, claude, deployment, quick-start, helm, verification]
---

# Getting Started with MCP Integration

Complete guide from deploying Spectre's MCP server to running your first Kubernetes investigation.

## What is MCP Integration?

**MCP (Model Context Protocol)** enables AI assistants like Claude to interact with Spectre's Kubernetes event data through a standardized protocol. Instead of manually running queries, you can have natural conversations with AI to investigate incidents, analyze changes, and troubleshoot cluster issues.

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│  AI Assistant (Claude Desktop, LLM applications)           │
└───────────────────────┬─────────────────────────────────────┘
                        │ MCP Protocol (JSON-RPC 2.0)
                        │
┌───────────────────────▼─────────────────────────────────────┐
│  MCP Server (sidecar or standalone)                        │
│  - Exposes 4 tools (cluster_health, resource_changes, etc) │
│  - Provides 2 prompts (post-mortem, live incident)         │
└───────────────────────┬─────────────────────────────────────┘
                        │ HTTP API (localhost or network)
                        │
┌───────────────────────▼─────────────────────────────────────┐
│  Spectre API Server (port 8080)                            │
│  - Event storage and querying                               │
└─────────────────────────────────────────────────────────────┘
```

### Capabilities

Once deployed, AI assistants can:

- **🔍 Investigate Incidents**: Ask "What happened to pods in production namespace 30 minutes ago?"
- **📊 Analyze Changes**: "Show me high-impact resource changes in the last hour"
- **🚨 Live Triage**: "Pods are crashing right now in namespace X, help me troubleshoot"
- **📝 Post-Mortems**: "Analyze the incident from 10:00 to 11:00 yesterday"
- **🗂️ Browse Resources**: "List all deployments in namespace Y with Error status"

## Prerequisites

### For Operators (Deploying MCP)

- ✅ Spectre already deployed via Helm (any namespace)
- ✅ `kubectl` access to the cluster
- ✅ Helm 3.x installed
- ✅ (Optional) Network policy allowing MCP server access

### For AI Users (After Deployment)

- ✅ MCP server deployed and accessible
- ✅ **Option A (Local)**: Claude Desktop installed + stdio transport configured
- ✅ **Option B (Remote)**: HTTP endpoint accessible (direct or via port-forward)

## Deployment Path 1: Helm Sidecar (Recommended)

**When to use**: Production environments, simplest setup, shared network namespace.

### Step 1: Update Helm Release

Enable MCP server in your existing Spectre Helm release:

```bash
# Update your values.yaml or use --set flags
helm upgrade spectre spectre/spectre \
  --namespace spectre-system \
  --reuse-values \
  --set mcp.enabled=true \
  --set mcp.httpAddr=":8081"
```

**What this does**:
- Adds MCP server container as sidecar to Spectre pod
- Exposes port 8081 for MCP protocol
- Shares network namespace with Spectre API (localhost communication)

### Step 2: Verify Deployment

```bash
# Check pod is running with 2 containers (spectre + mcp)
kubectl get pods -n spectre-system -l app.kubernetes.io/name=spectre

# Expected output:
# NAME                       READY   STATUS    RESTARTS   AGE
# spectre-5f7d8c9b4-x7k2p   2/2     Running   0          2m
```

**Troubleshooting**: If pod shows `1/2` Ready:
```bash
# Check MCP container logs
kubectl logs -n spectre-system <pod-name> -c mcp

# Common issues:
# - "connection refused" → Spectre API not ready yet (wait 30s)
# - "bind: address already in use" → Port conflict (check mcp.httpAddr)
```

### Step 3: Create Port-Forward

Access MCP server from your local machine:

```bash
# Forward MCP port to localhost
kubectl port-forward -n spectre-system svc/spectre 8081:8081

# Leave this running in a terminal
```

### Step 4: Verify MCP Connection

Test the MCP server is responding:

```bash
# Health check (simple HTTP GET)
curl http://localhost:8081/health

# Expected: {"status":"healthy"}
```

### Step 5: Test MCP Protocol

Initialize a session and list tools:

```bash
# Initialize MCP session
curl -X POST http://localhost:8081/mcp/v1 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {"name": "curl-test", "version": "1.0"}
    },
    "id": 1
  }'

# Expected response includes:
# - "serverInfo": {"name": "Spectre MCP Server", "version": "..."}
# - "capabilities": {"tools": {...}, "prompts": {...}}
```

**List available tools**:
```bash
curl -X POST http://localhost:8081/mcp/v1 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/list",
    "id": 2
  }'

# Expected: 4 tools (cluster_health, resource_changes, investigate, resource_explorer)
```

## Deployment Path 2: Standalone Server

**When to use**: Running MCP server independently (different pod, different namespace, or outside cluster).

### When to Choose Standalone

- MCP server needs different scaling than Spectre API
- Running on a separate node or machine
- Development/testing environment
- Multiple Spectre instances with shared MCP server

### Step 1: Deploy Standalone Pod

Create a standalone deployment:

```yaml
# mcp-standalone.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: spectre-mcp
  namespace: spectre-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: spectre-mcp
  template:
    metadata:
      labels:
        app: spectre-mcp
    spec:
      containers:
      - name: mcp
        image: spectre:latest  # Use your Spectre image
        command: ["/spectre"]
        args:
          - mcp
          - --api-url=http://spectre.spectre-system.svc.cluster.local:8080
          - --http-addr=:8081
          - --log-level=info
        ports:
        - containerPort: 8081
          name: mcp
        resources:
          requests:
            memory: 64Mi
            cpu: 50m
          limits:
            memory: 256Mi
            cpu: 200m
---
apiVersion: v1
kind: Service
metadata:
  name: spectre-mcp
  namespace: spectre-system
spec:
  selector:
    app: spectre-mcp
  ports:
  - port: 8081
    targetPort: 8081
    name: mcp
```

Apply the manifest:
```bash
kubectl apply -f mcp-standalone.yaml
```

### Step 2: Configure API URL

**Critical**: MCP server must reach Spectre API. Options:

- **Same namespace**: `http://spectre:8080` (service name)
- **Different namespace**: `http://spectre.spectre-system.svc.cluster.local:8080`
- **External**: `http://spectre.example.com` (if ingress configured)

### Step 3: Verify Connectivity

```bash
# Check MCP pod logs
kubectl logs -n spectre-system deployment/spectre-mcp

# Expected: "MCP server listening on :8081"
# No errors about API connection
```

### Step 4: Access MCP Server

Create port-forward or expose via service:

```bash
# Port-forward (development)
kubectl port-forward -n spectre-system svc/spectre-mcp 8081:8081

# OR expose via LoadBalancer (production)
kubectl patch svc spectre-mcp -n spectre-system -p '{"spec":{"type":"LoadBalancer"}}'
```

## First Investigation (HTTP API)

Now that MCP server is running, let's run a real investigation.

### Example 1: Cluster Health Check

**Question**: "What's the current state of my cluster?"

**MCP Tool**: `cluster_health`

```bash
# Get current Unix timestamp
END_TIME=$(date +%s)
START_TIME=$((END_TIME - 3600))  # Last hour

# Call cluster_health tool
curl -X POST http://localhost:8081/mcp/v1 \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"method\": \"tools/call\",
    \"params\": {
      \"name\": \"cluster_health\",
      \"arguments\": {
        \"start_time\": $START_TIME,
        \"end_time\": $END_TIME
      }
    },
    \"id\": 3
  }"
```

**Response** (abbreviated):
```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"overall_status\":\"Warning\",\"total_resources\":42,\"resources_by_kind\":[{\"kind\":\"Pod\",\"total\":15,\"healthy\":12,\"warning\":2,\"error\":1}],\"top_issues\":[{\"kind\":\"Pod\",\"namespace\":\"production\",\"name\":\"api-server-x7k2p\",\"status\":\"Error\",\"message\":\"CrashLoopBackOff\"}],\"error_resource_count\":1,\"warning_resource_count\":2}"
      }
    ]
  },
  "id": 3
}
```

**Interpretation**:
- Overall status: **Warning** (some issues present)
- 42 total resources tracked
- 1 Pod in **Error** state: `api-server-x7k2p` (CrashLoopBackOff)
- 2 Pods in **Warning** state

### Example 2: Investigate Failing Pod

**Question**: "Why is api-server-x7k2p failing?"

**MCP Tool**: `investigate`

```bash
curl -X POST http://localhost:8081/mcp/v1 \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"method\": \"tools/call\",
    \"params\": {
      \"name\": \"investigate\",
      \"arguments\": {
        \"resource_kind\": \"Pod\",
        \"resource_name\": \"api-server-x7k2p\",
        \"namespace\": \"production\",
        \"start_time\": $START_TIME,
        \"end_time\": $END_TIME,
        \"investigation_type\": \"incident\"
      }
    },
    \"id\": 4
  }"
```

**Response** includes:
- **Timeline**: Status transitions (Running → Error at 10:32:15)
- **Events**: Recent Kubernetes events ("Back-off restarting failed container", count: 15)
- **Investigation Prompts**: AI-generated questions to guide RCA:
  - "What caused the pod to transition from Running to Error?"
  - "Why is the container repeatedly failing to start?"
  - "Were there any configuration or deployment changes before the failure?"

**Next Steps**: Use the investigation prompts to guide further analysis (check logs, recent deployments, etc.)

## First Investigation (Claude Desktop)

**Preview**: With Claude Desktop integration, you can have natural conversations instead of crafting JSON-RPC requests.

**Example conversation**:
```
You: What's wrong with my production namespace right now?

Claude: [Calls cluster_health tool automatically]
I found 1 pod in Error state in the production namespace:
- Pod: api-server-x7k2p
- Issue: CrashLoopBackOff (container failing to start)

Would you like me to investigate this pod in detail?

You: Yes, investigate it

Claude: [Calls investigate tool]
The pod transitioned from Running to Error at 10:32:15.
Recent events show the container is repeatedly failing to start (15 restart attempts).

Based on the investigation, I recommend:
1. Check container logs: kubectl logs api-server-x7k2p -n production --previous
2. Review recent deployment changes (this may have started after an update)
```

**Full Claude Desktop setup**: See [Claude Integration Guide](./claude-integration.md)

## Verification Checklist

Use this checklist to confirm your MCP setup is working correctly:

- [ ] **1. Pod Running**: `kubectl get pods` shows Spectre pod with `2/2` Ready (sidecar) or standalone MCP pod with `1/1` Ready
- [ ] **2. Health Check**: `curl http://localhost:8081/health` returns `{"status":"healthy"}`
- [ ] **3. MCP Initialize**: Initialize request returns `serverInfo` and `capabilities`
- [ ] **4. Tools Available**: `tools/list` returns 4 tools (cluster_health, resource_changes, investigate, resource_explorer)
- [ ] **5. Prompts Available**: `prompts/list` returns 2 prompts (post_mortem_incident_analysis, live_incident_handling)
- [ ] **6. Tool Execution**: `cluster_health` tool call succeeds and returns cluster data
- [ ] **7. API Connectivity**: MCP server logs show no errors connecting to Spectre API

**All checks passed?** ✅ Your MCP integration is ready!

## Common Setup Issues

### Issue: MCP Container Not Starting

**Symptoms**: Pod shows `1/2` Ready, MCP container in `CrashLoopBackOff`

**Diagnosis**:
```bash
kubectl logs -n spectre-system <pod-name> -c mcp
```

**Common Causes**:
1. **"connection refused to :8080"**
   - **Cause**: Spectre API not ready yet
   - **Fix**: Wait 30-60 seconds for Spectre to start, MCP will retry

2. **"bind: address already in use"**
   - **Cause**: Port 8081 conflict
   - **Fix**: Change `mcp.httpAddr` to different port (e.g., `:8082`)

3. **"invalid API URL"**
   - **Cause**: Incorrect `mcp.apiUrl` (standalone mode)
   - **Fix**: Verify Spectre service name and namespace

### Issue: Tools Return Empty Results

**Symptoms**: `cluster_health` returns `"total_resources": 0`

**Diagnosis**: Spectre may not have collected events yet

**Fixes**:
1. **Check Spectre has data**:
   ```bash
   kubectl logs -n spectre-system <pod-name> -c spectre | grep "events indexed"
   ```
   Expected: Log entries showing events being indexed

2. **Verify time window**: Ensure `start_time` and `end_time` cover a period with activity

3. **Check namespace filter**: If using `namespace` parameter, verify the namespace exists and has resources

### Issue: Port-Forward Keeps Disconnecting

**Symptoms**: `curl` commands fail with "connection refused" intermittently

**Cause**: `kubectl port-forward` can be unstable

**Fix**: Use a tool like `kubefwd` or expose MCP via ingress:

```bash
# Option 1: kubefwd (more stable)
sudo kubefwd svc -n spectre-system -l app.kubernetes.io/name=spectre

# Option 2: Ingress (production)
# Add mcp.ingress.enabled=true to Helm values
```

### Issue: MCP Protocol Version Mismatch

**Symptoms**: Initialize request fails with "unsupported protocol version"

**Fix**: Spectre MCP server uses protocol version **2024-11-05**. Update your client:

```json
{
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",  // Use this exact version
    ...
  }
}
```

## Next Steps

### For Operators

- ✅ **Production Configuration**: Review [MCP Configuration Guide](../configuration/mcp-configuration.md) for resource planning, security, and monitoring
- ✅ **Enable Ingress**: Expose MCP server via ingress for remote access (see configuration guide)
- ✅ **Set Up Monitoring**: Add Prometheus metrics and alerting (see Operations docs)
- ✅ **Network Policies**: Restrict MCP access to authorized clients only

### For AI Users

- ✅ **Claude Desktop Setup**: Follow [Claude Integration Guide](./claude-integration.md) for conversational investigations
- ✅ **Learn the Tools**: Read [Tools Reference](./tools-reference/cluster-health.md) to understand capabilities
- ✅ **Try Prompts**: Use pre-built prompts for [Post-Mortems](./prompts-reference/post-mortem.md) and [Live Incidents](./prompts-reference/live-incident.md)
- ✅ **See Examples**: Explore [Real-World Examples](./examples.md) for common scenarios

### For Developers

- ✅ **MCP Protocol**: Read MCP specification at https://modelcontextprotocol.io
- ✅ **Tool Schemas**: Review tool input/output schemas in [Tools Reference](./tools-reference/cluster-health.md)
- ✅ **Build Integrations**: Use MCP client libraries to integrate with your own applications
- ✅ **Extend Functionality**: Contribute new tools or prompts (see Development docs)

## Quick Reference

### Endpoints

| Endpoint | Purpose | Transport |
|----------|---------|-----------|
| `http://localhost:8081/health` | Health check (HTTP GET) | HTTP |
| `http://localhost:8081/mcp/v1` | MCP protocol (JSON-RPC 2.0 POST) | HTTP |
| `stdio` | MCP over stdin/stdout | stdio (Claude Desktop) |

### Helm Configuration Keys

| Key | Default | Description |
|-----|---------|-------------|
| `mcp.enabled` | `false` | Enable MCP server |
| `mcp.httpAddr` | `":8081"` | HTTP listen address |
| `mcp.apiUrl` | `"http://localhost:8080"` | Spectre API URL (sidecar uses localhost) |
| `mcp.logLevel` | `"info"` | Log level (debug, info, warn, error) |

### MCP Tools (4 available)

| Tool | Purpose | Use Case |
|------|---------|----------|
| `cluster_health` | Cluster overview with status breakdown | "What's the current state?" |
| `resource_changes` | High-impact changes with correlation | "What changed recently?" |
| `investigate` | Detailed timeline for specific resource | "Why is this pod failing?" |
| `resource_explorer` | Browse and discover resources | "List all deployments in namespace X" |

### MCP Prompts (2 available)

| Prompt | Purpose | Use Case |
|--------|---------|----------|
| `post_mortem_incident_analysis` | Historical incident investigation | "Analyze the outage from 10:00-11:00 yesterday" |
| `live_incident_handling` | Real-time triage and mitigation | "Pods are failing right now, help me troubleshoot" |

## Troubleshooting Quick Commands

```bash
# Check MCP pod status
kubectl get pods -n spectre-system -l app.kubernetes.io/name=spectre

# View MCP logs
kubectl logs -n spectre-system <pod-name> -c mcp -f

# Test health endpoint
curl http://localhost:8081/health

# Initialize MCP session
curl -X POST http://localhost:8081/mcp/v1 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}'

# List tools
curl -X POST http://localhost:8081/mcp/v1 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/list","id":2}'

# Quick cluster health check
END=$(date +%s); START=$((END-3600)); \
curl -X POST http://localhost:8081/mcp/v1 \
  -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"tools/call\",\"params\":{\"name\":\"cluster_health\",\"arguments\":{\"start_time\":$START,\"end_time\":$END}},\"id\":3}"
```

## Related Documentation

- [MCP Configuration Guide](../configuration/mcp-configuration.md) - Complete configuration reference with Helm values
- [Claude Integration Guide](./claude-integration.md) - Set up Claude Desktop for conversational investigations
- [Tools Reference](./tools-reference/cluster-health.md) - Detailed API documentation for all 4 MCP tools
- [Prompts Reference](./prompts-reference/post-mortem.md) - Workflow guides for pre-built prompts
- [Real-World Examples](./examples.md) - Complete investigation scenarios with step-by-step walkthroughs

<!-- Source: README.md lines 72-285, tests/e2e/mcp_http_test.go, internal/mcp/handler.go -->
