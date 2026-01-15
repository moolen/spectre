---
title: Claude Desktop Integration
description: Complete guide to connecting Claude Desktop with Spectre MCP for conversational Kubernetes investigations
keywords: [claude, claude-desktop, mcp, integration, stdio, configuration, setup]
---

# Claude Desktop Integration

Transform Kubernetes troubleshooting into natural conversations with AI. This guide shows you how to connect Claude Desktop to Spectre's MCP server for conversational incident investigation.

## What You'll Achieve

After completing this setup, you'll be able to:

**Natural Language Investigations**:
```
You: What's wrong with my production namespace?

Claude: [Automatically calls cluster_health and investigate tools]
I found 3 pods in Error state. The api-server deployment
had a failed rollout 15 minutes ago due to missing ConfigMap.
Would you like me to investigate the deployment timeline?

You: Yes, and show me what changed before it failed

Claude: [Calls resource_changes tool]
Here's what happened:
1. [10:05] ConfigMap "api-config" was deleted
2. [10:06] Deployment triggered rolling update
3. [10:07] New pods failed with "ConfigMap not found"

I recommend:
1. Restore the ConfigMap or rollback the deployment
2. Check kubectl logs for the specific error
```

**Post-Mortem Analysis**:
```
You: Analyze the incident from yesterday 10:00 to 11:00 in production namespace

Claude: [Automatically uses post_mortem_incident_analysis prompt]
[Generates comprehensive report with timeline, root cause, and recommendations]
```

## Requirements

Before starting, ensure you have:

- ✅ **Claude Desktop**: Download from https://claude.ai/download (macOS, Windows, or Linux)
- ✅ **Spectre Deployed**: MCP-enabled Spectre instance (see [Getting Started](./getting-started.md))
- ✅ **Network Access**: Claude Desktop must reach Spectre (local cluster or port-forward/proxy)
- ✅ **File System Access**: Permission to edit Claude Desktop config file

## Architecture: How It Works

```
┌──────────────────────────────────────────────────┐
│  Claude Desktop Application                      │
│  - Conversational UI                             │
│  - Automatic tool selection                      │
│  - Context-aware investigations                  │
└────────────────┬─────────────────────────────────┘
                 │ MCP Protocol (stdio)
                 │ Reads/writes newline-delimited JSON
                 │
┌────────────────▼─────────────────────────────────┐
│  MCP Server Process (spectre mcp --stdio)       │
│  - Started by Claude as subprocess               │
│  - stdin/stdout for MCP messages                 │
│  - stderr for logs                               │
└────────────────┬─────────────────────────────────┘
                 │ HTTP API
                 │
┌────────────────▼─────────────────────────────────┐
│  Spectre API Server                              │
│  - Event storage and querying                    │
│  - Running in Kubernetes or locally              │
└──────────────────────────────────────────────────┘
```

### Stdio Transport

Claude Desktop uses **stdio transport** (not HTTP):
- **stdin**: Claude sends JSON-RPC requests as newline-delimited JSON
- **stdout**: MCP server responds with JSON-RPC responses (one per line)
- **stderr**: Logs from MCP server (separate from protocol messages)

**Why stdio?**
- Simpler setup (no network ports to manage)
- Automatic process lifecycle (Claude starts/stops MCP server)
- Secure by default (local subprocess, no network exposure)

## Setup Path 1: Local MCP Server (Development)

**Best for**: Local development, kind/minikube clusters, quick testing.

### Step 1: Ensure Spectre is Accessible

**Option A: Local Cluster (kind/minikube)**
```bash
# Spectre should be accessible at localhost via port-forward
kubectl port-forward -n spectre-system svc/spectre 8080:8080

# Leave this running in a terminal
# MCP server will connect to http://localhost:8080
```

**Option B: Remote Cluster with Port-Forward**
```bash
# Forward Spectre API to localhost
kubectl port-forward -n spectre-system svc/spectre 8080:8080

# Leave this running
```

**Verify Spectre is accessible**:
```bash
curl http://localhost:8080/api/search | head -n 5
# Should return JSON response (not connection refused)
```

### Step 2: Get Spectre Binary

You need the `spectre` binary to run the MCP server locally.

**Option A: Download from Release**
```bash
# Download latest release for your platform
curl -L https://github.com/moolen/spectre/releases/latest/download/spectre-$(uname -s)-$(uname -m) \
  -o /usr/local/bin/spectre

chmod +x /usr/local/bin/spectre

# Verify
spectre version
```

**Option B: Build from Source**
```bash
git clone https://github.com/moolen/spectre.git
cd spectre
make build

# Binary is at ./bin/spectre
sudo cp ./bin/spectre /usr/local/bin/spectre
```

**Option C: Extract from Container Image**
```bash
# Pull Spectre image
docker pull ghcr.io/moolen/spectre:latest

# Extract binary
docker create --name spectre-temp ghcr.io/moolen/spectre:latest
docker cp spectre-temp:/spectre /usr/local/bin/spectre
docker rm spectre-temp

chmod +x /usr/local/bin/spectre
```

### Step 3: Test MCP Server Manually

Before configuring Claude, verify the MCP server works:

```bash
# Start MCP server in stdio mode
spectre mcp \
  --api-url=http://localhost:8080 \
  --stdio

# You should see:
# {"jsonrpc":"2.0","method":"initialize",...}
# (Server is waiting for input on stdin)

# Press Ctrl+C to stop
```

**If you see errors**:
- **"connection refused to localhost:8080"**: Spectre API not accessible (check port-forward)
- **"command not found: spectre"**: Binary not in PATH or not executable

### Step 4: Create Wrapper Script (Recommended)

Claude Desktop requires a shell command to start the MCP server. Create a wrapper script for easy configuration and debugging.

```bash
# Create wrapper script
cat > /usr/local/bin/spectre-mcp-claude.sh << 'EOF'
#!/bin/bash

# Spectre MCP Wrapper for Claude Desktop
# This script starts the MCP server with proper configuration

# Configuration
API_URL="${SPECTRE_API_URL:-http://localhost:8080}"
LOG_LEVEL="${SPECTRE_LOG_LEVEL:-info}"

# Optional: Log to file for debugging
# Uncomment to capture logs (Claude only sees stderr)
# LOG_FILE="/tmp/spectre-mcp-claude.log"
# exec 2>> "$LOG_FILE"

# Start MCP server
exec /usr/local/bin/spectre mcp \
  --api-url="$API_URL" \
  --stdio \
  --log-level="$LOG_LEVEL"
EOF

chmod +x /usr/local/bin/spectre-mcp-claude.sh
```

**Test the wrapper**:
```bash
# Test with defaults
/usr/local/bin/spectre-mcp-claude.sh

# Test with custom API URL
SPECTRE_API_URL=http://192.168.1.10:8080 /usr/local/bin/spectre-mcp-claude.sh

# Press Ctrl+C to stop
```

### Step 5: Configure Claude Desktop

Claude Desktop reads MCP server configuration from a JSON file.

**Config file location**:
- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json`

**Create or edit the config file**:
```json
{
  "mcpServers": {
    "spectre": {
      "command": "/usr/local/bin/spectre-mcp-claude.sh",
      "args": [],
      "env": {
        "SPECTRE_API_URL": "http://localhost:8080",
        "SPECTRE_LOG_LEVEL": "info"
      }
    }
  }
}
```

**Configuration fields**:
- **`mcpServers`**: Map of MCP server definitions (key is display name)
- **`command`**: Absolute path to executable (wrapper script or binary)
- **`args`**: Command-line arguments (empty if using wrapper script)
- **`env`**: Environment variables passed to subprocess

**Alternative: Direct Binary Configuration** (without wrapper script):
```json
{
  "mcpServers": {
    "spectre": {
      "command": "/usr/local/bin/spectre",
      "args": [
        "mcp",
        "--api-url=http://localhost:8080",
        "--stdio",
        "--log-level=info"
      ]
    }
  }
}
```

### Step 6: Restart Claude Desktop

After editing the config:

1. **Quit Claude Desktop** completely (not just close window)
   - macOS: `Cmd+Q` or right-click dock icon → Quit
   - Windows: Right-click system tray → Exit
   - Linux: Kill the process or use application menu

2. **Relaunch Claude Desktop**

3. **Verify MCP Connection**:
   - Look for "Spectre" in available tools/extensions
   - Or start a conversation and ask: "What MCP tools do you have access to?"

**Expected response**:
```
I have access to the following Spectre MCP tools:
- cluster_health: Get cluster overview with resource status
- resource_changes: Identify high-impact changes
- investigate: Deep dive into specific resources

I also have access to two prompts:
- post_mortem_incident_analysis
- live_incident_handling
```

## Setup Path 2: Remote Spectre (Production)

**Best for**: Production clusters, remote access, shared Spectre instances.

### When to Use Remote Access

- Spectre running in a production Kubernetes cluster
- Multiple users sharing one Spectre instance
- Claude Desktop running on a different machine than kubectl

### Option 2A: kubectl port-forward

**Simplest approach** for authenticated cluster access:

```bash
# Forward Spectre API to localhost (keep running)
kubectl port-forward -n spectre-system svc/spectre 8080:8080

# Configure Claude Desktop to use http://localhost:8080
# (Same as Setup Path 1, Step 5)
```

**Advantages**:
- Uses existing kubectl authentication
- No additional network exposure
- Works through bastion hosts and VPNs

**Disadvantages**:
- Port-forward must stay running while using Claude
- Connection can be unstable (auto-reconnect not always reliable)

### Option 2B: kubectl exec (Sidecar MCP)

If Spectre has MCP sidecar enabled, you can run MCP server **inside the pod**:

```bash
# Create wrapper script that runs MCP in-cluster
cat > /usr/local/bin/spectre-mcp-kubectl.sh << 'EOF'
#!/bin/bash

NAMESPACE="${SPECTRE_NAMESPACE:-spectre-system}"
POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=spectre -o jsonpath='{.items[0].metadata.name}')

if [ -z "$POD" ]; then
  echo "Error: No Spectre pod found in namespace $NAMESPACE" >&2
  exit 1
fi

# Execute MCP server inside pod (stdio mode)
exec kubectl exec -n "$NAMESPACE" "$POD" -c spectre -i -- \
  /spectre mcp --api-url=http://localhost:8080 --stdio
EOF

chmod +x /usr/local/bin/spectre-mcp-kubectl.sh
```

**Claude Desktop config**:
```json
{
  "mcpServers": {
    "spectre": {
      "command": "/usr/local/bin/spectre-mcp-kubectl.sh",
      "args": [],
      "env": {
        "SPECTRE_NAMESPACE": "spectre-system"
      }
    }
  }
}
```

**Advantages**:
- No port-forward needed
- Uses cluster networking directly
- More stable than port-forward

**Disadvantages**:
- Requires kubectl exec permissions
- Slightly higher latency

### Option 2C: HTTP Proxy (Advanced)

For **remote access without kubectl**, expose Spectre API via ingress or LoadBalancer:

```bash
# Expose Spectre API (production only, with authentication!)
# Option 1: LoadBalancer
kubectl patch svc spectre -n spectre-system -p '{"spec":{"type":"LoadBalancer"}}'

# Option 2: Ingress (with TLS and auth)
# See MCP Configuration Guide for details
```

**Claude Desktop config** (with remote API):
```json
{
  "mcpServers": {
    "spectre": {
      "command": "/usr/local/bin/spectre",
      "args": [
        "mcp",
        "--api-url=https://spectre.example.com",
        "--stdio"
      ]
    }
  }
}
```

**⚠️ Security Warning**: Only expose Spectre API with proper authentication (mTLS, OAuth, API tokens). See [MCP Configuration Guide](../configuration/mcp-configuration.md) security section.

## First Investigation with Claude

Now that Claude Desktop is connected, let's run some investigations.

### Example 1: Quick Health Check

**You**:
```
What's the current state of my Kubernetes cluster?
```

**Claude** (automatic tool calls):
```
[Calling cluster_health tool with start_time: now-1h, end_time: now]

Your cluster has 42 resources tracked. Overall status: Healthy

Resources by kind:
- Pods: 15 total (14 healthy, 1 warning)
- Deployments: 8 total (all healthy)
- Services: 10 total (all healthy)

Top issues:
- Pod "api-cache-7x9k2" in namespace "production" has status Warning
  (High restart count: 5 restarts in last hour)

Would you like me to investigate the warning pod?
```

### Example 2: Investigate Failing Pods

**You**:
```
Pods are failing in the production namespace. Can you investigate?
```

**Claude** (automatic tool calls):
```
[Calling cluster_health with namespace filter: production]

I found 3 pods in Error state in the production namespace:

1. api-server-85f6c9b8-k4x2p - CrashLoopBackOff
2. api-server-85f6c9b8-m9n4r - CrashLoopBackOff
3. api-server-85f6c9b8-p2q7t - CrashLoopBackOff

All three pods belong to the same Deployment: api-server

Let me investigate the deployment timeline...

[Calling investigate tool for Deployment/production/api-server]

Timeline:
[10:05:12] Deployment updated - Image changed: v1.2.0 → v1.3.0
[10:05:45] Pods started failing - Container startup error
[10:06:30] Status changed to Warning (ReplicaFailure)

Recent events:
- "Back-off restarting failed container" (count: 15)
- "Failed to pull image registry.example.com/api-server:v1.3.0"

Root Cause Analysis:
The deployment updated to a new image (v1.3.0) that cannot be pulled.
This is likely a registry authentication issue or the image doesn't exist.

Immediate Steps:
1. Verify image exists: docker pull registry.example.com/api-server:v1.3.0
2. Check imagePullSecrets: kubectl get pods -n production -o yaml | grep imagePullSecrets
3. Rollback deployment: kubectl rollout undo deployment/api-server -n production

Would you like me to check what changed before this deployment update?
```

### Example 3: Post-Mortem Analysis

**You**:
```
Analyze the incident from yesterday 10:00 to 11:00 in production namespace
```

**Claude** (uses post_mortem_incident_analysis prompt):
```
[Executing post-mortem analysis prompt with arguments:
  start_time: 2024-12-11 10:00 UTC
  end_time: 2024-12-11 11:00 UTC
  namespace: production]

## Incident Summary

Service outage caused by ConfigMap deletion, affecting api-server deployment
for 12 minutes. Restored after ConfigMap was recreated.

## Timeline

[10:00:05] ConfigMap/production/api-config - Deleted
[10:02:18] Deployment/production/api-server - Triggered rolling update
[10:02:45] Pods started failing - "ConfigMap api-config not found"
[10:03:00] Service endpoints removed (no ready pods)
[10:12:30] ConfigMap/production/api-config - Created (restored from backup)
[10:12:55] Pods became healthy
[10:13:10] Service endpoints restored

## Root Cause Analysis

Primary Cause: ConfigMap "api-config" was accidentally deleted

Contributing Factors:
1. No backup or version control for ConfigMaps
2. Deployment references ConfigMap but has no failure handling
3. No alerting on ConfigMap deletions
4. Manual restoration took 12 minutes

Evidence:
- resource_changes shows ConfigMap deletion at 10:00:05 (impact_score: 0.75)
- investigate shows pods transitioned Running → Error at 10:02:45
- Events: "ConfigMap api-config not found" (count: 24)

## Impact Assessment

- Service downtime: 12 minutes
- Affected resources: 1 Deployment, 3 Pods, 1 Service
- User impact: API unavailable (100% error rate)

## Recommendations

1. Store ConfigMaps in version control (GitOps with FluxCD/ArgoCD)
2. Enable RBAC restrictions on ConfigMap deletion
3. Add ConfigMap change alerts to monitoring
4. Implement graceful degradation (default config values)
5. Document ConfigMap restoration procedure

## Data Gaps & Follow-Up

To complete the analysis:
1. Check who deleted the ConfigMap: kubectl logs -n kube-system -l app=kube-apiserver
2. Review application logs: kubectl logs -n production <pod-name> --previous
3. Verify monitoring alerts fired: Check Prometheus/Datadog alerts for this timeframe
```

## Conversation Patterns

### Pattern 1: Discovery → Investigation

**Efficient workflow** for unknown issues:

```
You: Check the production namespace for any issues

Claude: [cluster_health] Found 2 pods in Error state...

You: Investigate the api-server deployment

Claude: [investigate] Here's the timeline...

You: What changed before it started failing?

Claude: [resource_changes] A ConfigMap was deleted 2 minutes before...
```

### Pattern 2: Direct Deep Dive

When you know the problem resource:

```
You: Investigate the deployment api-server in namespace production
     from 30 minutes ago to now

Claude: [investigate] Status transitions: Ready → Warning → Ready
        Timeline shows rollback at 10:15...
```

### Pattern 3: Time-Based Analysis

For historical incidents:

```
You: What happened in production between 10:00 and 10:30 today?

Claude: [resource_changes] 5 high-impact changes detected...

You: Focus on the deployment changes

Claude: [investigate each deployment] Here's what happened to each...
```

### Pattern 4: Iterative Follow-Up

Build on previous responses:

```
You: Show me cluster health for the last hour

Claude: [cluster_health] Overall: Warning, 3 pods failing...

You: Investigate those failing pods

Claude: [investigate for each pod] Pod 1: CrashLoopBackOff...

You: Were there any changes to their deployments recently?

Claude: [resource_changes for deployments] Yes, deployment X was updated...
```

## Claude Desktop Configuration Reference

### Config File Schema

```json
{
  "mcpServers": {
    "<server-name>": {
      "command": "<absolute-path-to-executable>",
      "args": ["<arg1>", "<arg2>"],
      "env": {
        "<ENV_VAR>": "<value>"
      },
      "disabled": false  // Optional: set true to disable without removing
    }
  }
}
```

### Multiple Spectre Instances

You can configure multiple Spectre connections:

```json
{
  "mcpServers": {
    "spectre-dev": {
      "command": "/usr/local/bin/spectre",
      "args": ["mcp", "--api-url=http://localhost:8080", "--stdio"]
    },
    "spectre-prod": {
      "command": "/usr/local/bin/spectre-mcp-kubectl.sh",
      "env": {
        "SPECTRE_NAMESPACE": "spectre-production"
      }
    }
  }
}
```

**Usage**: Ask Claude to specify which instance:
```
You: Use spectre-prod to check cluster health
```

### Environment Variables

Available environment variables for wrapper scripts:

| Variable | Purpose | Example |
|----------|---------|---------|
| `SPECTRE_API_URL` | Spectre API endpoint | `http://localhost:8080` |
| `SPECTRE_LOG_LEVEL` | Logging verbosity | `debug`, `info`, `warn`, `error` |
| `SPECTRE_NAMESPACE` | Default namespace for kubectl | `spectre-system` |

## Troubleshooting

### Issue: Claude Doesn't See Spectre Tools

**Symptoms**: Claude responds "I don't have access to Spectre tools" or doesn't show MCP extensions.

**Diagnosis**:
1. Check config file location is correct for your OS
2. Verify JSON syntax is valid (use `jq` or online validator)
3. Check Claude Desktop logs for errors

**Logs location**:
- **macOS**: `~/Library/Logs/Claude/`
- **Windows**: `%LOCALAPPDATA%\Claude\logs\`
- **Linux**: `~/.local/share/Claude/logs/`

**Look for**:
```
[MCP] Failed to start server "spectre": command not found
[MCP] Server "spectre" exited with code 1
```

**Common fixes**:
- **Command not found**: Use absolute path to executable
- **Permission denied**: `chmod +x /path/to/spectre`
- **JSON parse error**: Validate JSON syntax

### Issue: Claude Says "Tool Call Failed"

**Symptoms**: Claude tries to call a tool but gets an error.

**Diagnosis**: Check MCP server logs

```bash
# If using wrapper script with LOG_FILE enabled:
tail -f /tmp/spectre-mcp-claude.log

# Or enable logging temporarily:
echo 'exec 2>> /tmp/spectre-mcp-debug.log' > /tmp/test-wrapper.sh
echo 'exec /usr/local/bin/spectre mcp --api-url=http://localhost:8080 --stdio' >> /tmp/test-wrapper.sh
chmod +x /tmp/test-wrapper.sh

# Update Claude config to use /tmp/test-wrapper.sh temporarily
# Restart Claude, try tool again, check /tmp/spectre-mcp-debug.log
```

**Common errors**:
1. **"connection refused to localhost:8080"**
   - **Cause**: Spectre API not accessible
   - **Fix**: Start port-forward or verify Spectre is running

2. **"context deadline exceeded"**
   - **Cause**: Tool execution timed out
   - **Fix**: Check Spectre performance, reduce time window

3. **"no resources found"**
   - **Cause**: Spectre has no data for the requested time window
   - **Fix**: Verify Spectre is indexing events, adjust time window

### Issue: Logs Show "Failed to Parse JSON-RPC Request"

**Symptoms**: MCP server logs show parse errors.

**Cause**: This is usually a bug in Claude Desktop or the MCP server implementation.

**Temporary fix**:
```bash
# Capture raw stdin/stdout for debugging
cat > /tmp/spectre-mcp-debug-wrapper.sh << 'EOF'
#!/bin/bash
tee /tmp/mcp-stdin.log | \
  /usr/local/bin/spectre mcp --api-url=http://localhost:8080 --stdio | \
  tee /tmp/mcp-stdout.log
EOF

chmod +x /tmp/spectre-mcp-debug-wrapper.sh

# Update Claude config to use debug wrapper
# Restart Claude, try tool, inspect /tmp/mcp-stdin.log and /tmp/mcp-stdout.log
```

### Issue: Port-Forward Keeps Disconnecting

**Symptoms**: Tool calls work initially but fail after a few minutes.

**Cause**: `kubectl port-forward` can be unstable.

**Fix**: Use a more stable connection method:

```bash
# Option 1: kubefwd (more stable port-forwarding)
sudo kubefwd svc -n spectre-system -l app.kubernetes.io/name=spectre

# Option 2: Use kubectl exec instead (Setup Path 2, Option 2B)
```

## Best Practices

### ✅ Do

- **Be specific with time windows**: "last 30 minutes" is better than "recently"
- **Provide namespace context**: Mention namespace to speed up queries
- **Use prompt names explicitly**: "Run post_mortem_incident_analysis from 10:00 to 11:00"
- **Follow up with kubectl commands**: Claude suggests kubectl commands - run them for complete analysis
- **Ask Claude to explain**: "Why did you choose to investigate that resource?"
- **Iterate incrementally**: Start broad (cluster_health), then narrow down (investigate specific resources)
- **Keep port-forward running**: Ensure Spectre API stays accessible during investigations

### ❌ Don't

- **Don't expect real-time logs**: Spectre tracks events, not container logs (use kubectl logs)
- **Don't rely solely on AI**: Verify recommendations before executing destructive commands
- **Don't use for live incidents without backup plan**: Always have kubectl ready for manual intervention
- **Don't share sensitive config**: Claude Desktop config may contain API URLs - keep it private
- **Don't expect instant responses**: Large time windows or complex investigations may take 30-60 seconds
- **Don't query very old data**: Check Spectre retention settings (default: 7 days)

## Security Considerations

### Local Development

- **Low risk**: MCP server runs locally, connects to port-forwarded API
- **No network exposure**: stdio transport doesn't open any ports
- **Subprocess isolation**: Claude manages MCP server lifecycle

### Production Access

**⚠️ Important**: Exposing Spectre API requires authentication

**Recommended approach**:
1. **kubectl port-forward** (simplest, uses existing auth)
2. **kubectl exec** (runs MCP in-cluster)
3. **VPN + LoadBalancer** (for remote teams)

**NOT recommended**:
- ❌ Public LoadBalancer without auth
- ❌ HTTP ingress without TLS + authentication
- ❌ Shared API tokens in Claude config (visible in plain text)

**Future**: Spectre v2.0 will add built-in authentication for MCP server. Until then, rely on network-level security (VPN, kubectl auth, mTLS).

## Example Workflow: Production Incident

End-to-end example of using Claude for real incident:

```
[10:15 AM - Pagerduty alert: API 500 errors spiking]

You: Check production namespace cluster health right now

Claude: [cluster_health] Found 5 pods in Error state, all in api-server deployment

You: Run live incident handling prompt

Claude: [live_incident_handling prompt]
        Incident started ~5 minutes ago
        Timeline:
        [10:10] Deployment api-server updated
        [10:11] Pods started failing
        [10:12] Service endpoints removed

        Immediate mitigation:
        kubectl rollout undo deployment/api-server -n production

You: [Runs rollback command]

You: Monitor recovery

Claude: [cluster_health]
        Rollback complete, 3/3 pods Running
        Service endpoints restored

You: What changed before the incident?

Claude: [resource_changes]
        Deployment updated: image tag v1.2 → v1.3
        ConfigMap api-config updated 30 seconds before deployment

You: Run post-mortem from 10:00 to 10:20

Claude: [post_mortem_incident_analysis]
        [Generates full report with recommendations]

[10:20 AM - Incident resolved, post-mortem documented]
```

## Advanced Configuration

### Custom Wrapper Script with Debugging

```bash
#!/bin/bash

# Spectre MCP Wrapper with Debug Logging

LOG_FILE="/tmp/spectre-mcp-$(date +%Y%m%d-%H%M%S).log"
API_URL="${SPECTRE_API_URL:-http://localhost:8080}"

# Log startup
echo "[$(date)] Starting Spectre MCP" >> "$LOG_FILE"
echo "[$(date)] API URL: $API_URL" >> "$LOG_FILE"

# Redirect stderr to log file
exec 2>> "$LOG_FILE"

# Start MCP server
exec /usr/local/bin/spectre mcp \
  --api-url="$API_URL" \
  --stdio \
  --log-level=debug
```

### Multiple Clusters with Context Selection

```json
{
  "mcpServers": {
    "spectre-us-west": {
      "command": "/usr/local/bin/spectre-mcp-kubectl.sh",
      "env": {
        "KUBECONFIG": "/Users/you/.kube/config-us-west",
        "SPECTRE_NAMESPACE": "spectre-system"
      }
    },
    "spectre-eu-central": {
      "command": "/usr/local/bin/spectre-mcp-kubectl.sh",
      "env": {
        "KUBECONFIG": "/Users/you/.kube/config-eu-central",
        "SPECTRE_NAMESPACE": "spectre-system"
      }
    }
  }
}
```

### Integration with Other MCP Servers

Claude Desktop supports multiple MCP servers simultaneously:

```json
{
  "mcpServers": {
    "spectre": {
      "command": "/usr/local/bin/spectre-mcp-claude.sh"
    },
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/files"]
    },
    "database": {
      "command": "mcp-server-postgres",
      "args": ["postgresql://user:pass@localhost/db"]
    }
  }
}
```

**Usage**: Claude can use multiple tools together:
```
You: Check Spectre for failing pods, then write a summary to incident-report.md

Claude: [Uses Spectre tools to investigate]
        [Uses filesystem MCP to write file]
```

## Limitations

### What Claude CAN Do with Spectre

- ✅ Investigate Kubernetes events and resource status changes
- ✅ Identify high-impact changes and correlate failures
- ✅ Provide timeline reconstruction for incidents
- ✅ Generate investigation prompts for root cause analysis
- ✅ Browse and discover resources across namespaces
- ✅ Run pre-built prompts (post-mortem, live incident handling)

### What Claude CANNOT Do (Yet)

- ❌ Access container logs directly (Spectre doesn't index logs)
- ❌ Execute kubectl commands (you must run suggested commands manually)
- ❌ Access metrics (CPU, memory, network) - Spectre tracks events only
- ❌ Modify cluster resources (read-only investigation)
- ❌ Access external systems (Prometheus, Datadog, etc.)

### Bridging the Gap

For complete investigations, combine Claude + manual steps:

1. **Claude investigates events** → Identifies likely cause and affected resources
2. **You run kubectl logs** → Get container error messages
3. **You check metrics** → Verify CPU/memory issues
4. **Claude synthesizes findings** → "Based on the logs you shared, the issue is..."

## Related Documentation

- [Getting Started with MCP](./getting-started.md) - Initial MCP deployment and verification
- [MCP Configuration Guide](../configuration/mcp-configuration.md) - Complete configuration reference
- [Tools Reference](./tools-reference/cluster-health.md) - Detailed API documentation for all 4 tools
- [Prompts Reference](./prompts-reference/post-mortem.md) - Workflow guides for pre-built prompts
- [Real-World Examples](./examples.md) - Complete investigation scenarios
- [MCP Protocol Specification](https://modelcontextprotocol.io) - Official MCP documentation

<!-- Source: internal/mcp/transport/stdio/transport.go, cmd/spectre/commands/mcp.go, tests/e2e/mcp_stdio_test.go -->
