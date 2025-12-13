---
title: MCP Integration
description: AI-assisted incident analysis with Claude and Model Context Protocol
keywords: [mcp, ai, claude, incident analysis, kubernetes, troubleshooting]
---

# MCP Integration

Transform Kubernetes troubleshooting into natural conversations with AI. Spectre's Model Context Protocol (MCP) integration enables AI assistants like Claude to investigate incidents, analyze changes, and generate post-mortem reports automatically.

## What is MCP Integration?

**MCP (Model Context Protocol)** is an open standard that allows AI assistants to interact with external data sources and tools through a standardized protocol. Spectre implements an MCP server that exposes Kubernetes event data and investigation capabilities to LLMs.

**Instead of this**:
```bash
# Manual investigation workflow
kubectl get pods -n production
kubectl describe pod failing-pod-x7k2p
kubectl logs failing-pod-x7k2p --previous
kubectl get events -n production --sort-by='.lastTimestamp'
# Manually correlate events, search for changes, write post-mortem...
```

**You get this**:
```
You: Pods are failing in production namespace. What happened?

Claude: [Automatically uses MCP tools to investigate]
I found 3 pods in CrashLoopBackOff state. The deployment was
updated 15 minutes ago to image v1.3.0, which is failing to
pull due to authentication issues.

Immediate fix:
kubectl rollout undo deployment/api-server -n production

[Provides timeline, root cause, and mitigation steps]
```

## Quick Start Paths

Choose your path based on your role:

### For Operators (Deploying MCP)

**Goal**: Deploy and verify MCP server

**Time**: 10-15 minutes

**Steps**:
1. [Deploy MCP server](./getting-started.md#deployment-path-1-helm-sidecar-recommended) via Helm sidecar
2. [Verify MCP connection](./getting-started.md#verification-checklist) with health checks
3. [Configure production settings](../configuration/mcp-configuration.md) (resources, monitoring)

**What you'll achieve**: MCP server running and accessible for AI assistants

### For AI Users (Using Claude Desktop)

**Goal**: Chat with Claude to investigate Kubernetes issues

**Time**: 15-20 minutes

**Prerequisites**: MCP server already deployed (ask your ops team)

**Steps**:
1. [Install Spectre binary](./claude-integration.md#step-2-get-spectre-binary) locally
2. [Configure Claude Desktop](./claude-integration.md#step-5-configure-claude-desktop) with MCP server
3. [Run first investigation](./claude-integration.md#first-investigation-with-claude) with Claude

**What you'll achieve**: Natural language Kubernetes troubleshooting

### For Developers (Building Integrations)

**Goal**: Integrate MCP tools into custom applications

**Time**: 30-60 minutes

**Steps**:
1. Review [MCP protocol specification](https://modelcontextprotocol.io)
2. Explore [tool schemas](./tools-reference/cluster-health.md) and input/output formats
3. Use [HTTP API examples](./getting-started.md#via-http-api-programmatic) to build integrations
4. See [real-world examples](./examples.md) for common patterns

**What you'll achieve**: Programmatic access to Spectre investigation capabilities

## Architecture

```
┌────────────────────────────────────────────────────────┐
│  AI Assistant (Claude Desktop, LLM applications)      │
│  • Natural language interface                          │
│  • Automatic tool selection                            │
│  • Conversation-based investigation                    │
└──────────────────────┬─────────────────────────────────┘
                       │ MCP Protocol (JSON-RPC 2.0)
                       │ Transport: HTTP or stdio
                       │
┌──────────────────────▼─────────────────────────────────┐
│  MCP Server (sidecar or standalone)                   │
│  • Protocol: 2024-11-05                                │
│  • Exposes 4 tools + 2 prompts                         │
│  • Translates natural language → API calls             │
└──────────────────────┬─────────────────────────────────┘
                       │ HTTP API (port 8080)
                       │
┌──────────────────────▼─────────────────────────────────┐
│  Spectre API Server (in Kubernetes)                   │
│  • Event storage and querying                          │
│  • Timeline reconstruction                             │
│  • Change correlation                                  │
└────────────────────────────────────────────────────────┘
```

## Key Features

### 1. Conversational Investigations

Ask questions in plain English, get structured answers with timelines and recommendations:

```
You: What's wrong with my production namespace?
→ Claude automatically calls cluster_health and investigate tools

You: Show me what changed before the incident
→ Claude uses resource_changes to correlate events

You: Run a post-mortem analysis
→ Claude executes full 9-step analysis workflow
```

### 2. Comprehensive Toolset

**4 Investigation Tools**:
- **cluster_health**: Cluster overview with resource status breakdown
- **resource_changes**: High-impact change identification with correlation
- **investigate**: Detailed timeline reconstruction with RCA prompts
- **resource_explorer**: Resource discovery and browsing across namespaces

**2 Pre-Built Prompts**:
- **post_mortem_incident_analysis**: 9-step historical incident investigation
- **live_incident_handling**: 8-step real-time triage and mitigation

### 3. Grounded Analysis

**No hallucinations**: All findings are traceable to actual Kubernetes events. The MCP server only reports what Spectre observed, never invents data.

**Evidence-based RCA**: Every claim includes:
- Exact timestamps
- Resource names and namespaces
- Event messages from Kubernetes
- Status transition evidence

### 4. Flexible Deployment

**Transport Modes**:
- **HTTP** (port 8081): For programmatic access, curl, remote clients
- **stdio**: For Claude Desktop (subprocess communication)

**Deployment Patterns**:
- **Sidecar** (recommended): Runs alongside Spectre API in same pod
- **Standalone**: Separate deployment for different scaling needs

## Capabilities Table

| Tool/Prompt | Purpose | Use Case | Time Window |
|-------------|---------|----------|-------------|
| **cluster_health** | Cluster-wide health snapshot | "What's the current state?" | Minutes to hours |
| **resource_changes** | Change identification + correlation | "What changed recently?" | 1-6 hours |
| **investigate** | Deep dive into specific resource | "Why is this pod failing?" | Minutes to days |
| **resource_explorer** | Resource discovery | "List all errored deployments" | Snapshot (optional time) |
| **post_mortem_incident_analysis** | Historical incident RCA | "Analyze yesterday's outage" | 15 min - 4 hours |
| **live_incident_handling** | Real-time triage | "Pods failing RIGHT NOW" | Last 15-30 minutes |

## Deployment Modes

### Sidecar Mode (Recommended)

**Best for**: Production environments, simplest setup

```yaml
mcp:
  enabled: true
  httpAddr: ":8081"
  # Shares network namespace with Spectre API
```

**Advantages**:
- Automatic lifecycle management (starts/stops with Spectre)
- Localhost communication (low latency, no network exposure)
- No separate RBAC or service configuration
- Simplest Helm configuration

**Use when**: Running Spectre in Kubernetes and want MCP for production use

### Standalone Mode

**Best for**: Independent scaling, development, multi-instance access

```yaml
# Separate deployment with custom resource limits
apiVersion: apps/v1
kind: Deployment
metadata:
  name: spectre-mcp-standalone
spec:
  ...
```

**Advantages**:
- Scale MCP independently from Spectre API
- Different resource limits (MCP is lightweight)
- Can serve multiple Spectre instances
- Easier local development (run MCP locally, Spectre in cluster)

**Use when**: Need different scaling, running MCP outside cluster, or development/testing

## Use Cases

### Incident Investigation

**Scenario**: Alert fires, need to understand what's broken

**Workflow**: Claude calls `cluster_health` → identifies errors → uses `investigate` on failing resources → provides timeline and mitigation steps

**Time Saved**: 5-10 minutes per incident (automated discovery and correlation)

### Post-Mortem Analysis

**Scenario**: Incident resolved, need comprehensive documentation

**Workflow**: Claude executes `post_mortem_incident_analysis` prompt → generates full report with timeline, RCA, impact, recommendations

**Time Saved**: 30-60 minutes per post-mortem (automated report generation)

### Deployment Verification

**Scenario**: Just deployed changes, want to verify everything is healthy

**Workflow**: Claude uses `resource_changes` → lists what actually changed → confirms expected changes, no unexpected errors

**Time Saved**: 3-5 minutes per deployment (instant verification)

### Change Correlation

**Scenario**: Performance degradation, need to identify what changed

**Workflow**: Claude calls `resource_changes` with high impact threshold → shows correlated timeline of changes and failures

**Time Saved**: 10-15 minutes (automated change identification)

### Proactive Monitoring

**Scenario**: Daily/weekly health checks to catch issues early

**Workflow**: Claude runs `cluster_health` across all namespaces → reports warnings before they become incidents

**Time Saved**: 5-10 minutes per health check (automated triage)

## Requirements

### For Operators

- ✅ Spectre v1.0+ deployed in Kubernetes
- ✅ Helm 3.x for sidecar deployment
- ✅ (Optional) Network access for HTTP transport

### For AI Users (Claude Desktop)

- ✅ MCP server deployed and accessible
- ✅ Claude Desktop installed (macOS, Windows, or Linux)
- ✅ Spectre binary locally (for stdio transport)
- ✅ Network access to Spectre API (port-forward or direct)

### For Developers

- ✅ MCP client library (or raw JSON-RPC 2.0 client)
- ✅ HTTP access to MCP server (port 8081)
- ✅ Understanding of tool schemas and parameters

## Limitations

### What MCP CAN Do

- ✅ Investigate Kubernetes events and resource status changes
- ✅ Reconstruct timelines from Spectre event data
- ✅ Identify high-impact changes and correlate failures
- ✅ Provide RCA prompts based on observed patterns
- ✅ Generate structured post-mortem reports

### What MCP CANNOT Do

- ❌ Access container logs (Spectre doesn't index logs)
- ❌ Execute kubectl commands (provides suggestions only)
- ❌ Access metrics (CPU, memory, network) - events only
- ❌ Modify cluster resources (read-only investigation)
- ❌ Predict future incidents (analyzes historical data)

### Bridging the Gap

For complete investigations:
1. **Claude investigates events** via MCP tools → Identifies likely cause
2. **You run kubectl logs** → Get container error messages
3. **You check metrics** → Verify resource constraints
4. **Claude synthesizes** → "Based on the logs you shared, the root cause is..."

## Getting Started

### 1. Deploy MCP Server

```bash
# Enable MCP in your Spectre Helm release
helm upgrade spectre spectre/spectre \
  --namespace spectre-system \
  --reuse-values \
  --set mcp.enabled=true \
  --set mcp.httpAddr=":8081"

# Verify deployment
kubectl get pods -n spectre-system
# Should show 2/2 Ready (spectre + mcp containers)
```

### 2. Verify Connection

```bash
# Port-forward MCP server
kubectl port-forward -n spectre-system svc/spectre 8081:8081

# Test health endpoint
curl http://localhost:8081/health
# Should return: {"status":"healthy"}
```

### 3. Try Your First Investigation

**Via curl** (HTTP API):
```bash
# Get cluster health for last hour
curl -X POST http://localhost:8081/mcp/v1 \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"method\": \"tools/call\",
    \"params\": {
      \"name\": \"cluster_health\",
      \"arguments\": {
        \"start_time\": $(date -d '1 hour ago' +%s),
        \"end_time\": $(date +%s)
      }
    },
    \"id\": 1
  }"
```

**Via Claude Desktop**:
```
You: What's the current state of my Kubernetes cluster?

Claude: [Automatically calls cluster_health]
        Your cluster has 42 resources tracked...
```

## Next Steps

### For Operators

1. **Configure Production**: Review [MCP Configuration Guide](../configuration/mcp-configuration.md) for resource planning, security, and monitoring
2. **Set Up Monitoring**: Add health checks and metrics for MCP server
3. **Enable Ingress**: Expose MCP for remote access (with authentication)
4. **Document Access**: Share MCP endpoint with your team

### For AI Users

1. **Install Claude Desktop**: Follow [Claude Integration Guide](./claude-integration.md) for step-by-step setup
2. **Learn the Tools**: Read [Tools Reference](./tools-reference/cluster-health.md) to understand capabilities
3. **Try Examples**: Work through [Real-World Examples](./examples.md) to see common patterns
4. **Explore Prompts**: Use [Prompts Reference](./prompts-reference/post-mortem.md) for structured workflows

### For Developers

1. **Review API Schemas**: Check [Tools Reference](./tools-reference/cluster-health.md) for input/output formats
2. **Study Protocol**: Read MCP specification at https://modelcontextprotocol.io
3. **Build Integration**: Use [HTTP API examples](./examples.md#via-http-api-programmatic) as templates
4. **Test Locally**: Deploy MCP in development environment and iterate

## Documentation Index

### Getting Started Guides

- [**Getting Started**](./getting-started.md) - Deploy MCP server and run first investigation
- [**Claude Integration**](./claude-integration.md) - Set up Claude Desktop for conversational investigations
- [**MCP Configuration**](../configuration/mcp-configuration.md) - Production configuration, security, and tuning

### Tool References (API Documentation)

- [**cluster_health**](./tools-reference/cluster-health.md) - Cluster overview with resource status breakdown
- [**resource_changes**](./tools-reference/resource-changes.md) - High-impact change identification
- [**investigate**](./tools-reference/investigate.md) - Detailed resource timeline reconstruction
- [**resource_explorer**](./tools-reference/resource-explorer.md) - Resource discovery and browsing

### Prompt References (Workflows)

- [**post_mortem_incident_analysis**](./prompts-reference/post-mortem.md) - Historical incident investigation (9-step workflow)
- [**live_incident_handling**](./prompts-reference/live-incident.md) - Real-time incident triage (8-step workflow)

### Usage Examples

- [**Real-World Examples**](./examples.md) - Complete scenarios with Claude Desktop and HTTP API examples

## FAQ

**Q: Do I need Claude Desktop to use MCP?**
A: No. You can use any MCP-compatible client, or call the HTTP API directly with curl/scripts.

**Q: Can multiple users share one MCP server?**
A: Yes. MCP server is stateless and can handle concurrent requests from multiple clients.

**Q: Does MCP require authentication?**
A: Not in v1.0. Use network-level security (VPN, kubectl port-forward). Authentication planned for v2.0.

**Q: Can I use MCP with production incidents?**
A: Yes. MCP is read-only and provides suggestions. Always verify before executing commands.

**Q: How much does MCP server cost (resources)?**
A: Minimal. Default: 64Mi-256Mi memory, 50m-200m CPU. Sidecar adds ~5% overhead.

**Q: Does MCP work with custom resources (CRDs)?**
A: Yes. Spectre tracks all resource kinds, including custom resources. See [Example 6](./examples.md#example-6-custom-resource-investigation-flux-gitrepository).

**Q: Can I extend MCP with custom tools?**
A: Not yet. Planned for v2.0. For now, use the 4 built-in tools and 2 prompts.

## Troubleshooting

**Problem**: Claude doesn't see Spectre tools
**Solution**: Check Claude Desktop config file location, validate JSON syntax, check logs at `~/Library/Logs/Claude/`

**Problem**: Tools return empty results
**Solution**: Verify Spectre is indexing events, check time window isn't outside retention period (default: 7 days)

**Problem**: MCP server not starting (sidecar)
**Solution**: Check Spectre API is ready first (MCP connects to localhost:8080), view logs with `kubectl logs <pod> -c mcp`

**Problem**: Port-forward keeps disconnecting
**Solution**: Use `kubefwd` for more stable forwarding, or try `kubectl exec` to run MCP inside pod

For more troubleshooting: See [Getting Started - Common Setup Issues](./getting-started.md#common-setup-issues)

## Learn More

- **MCP Protocol**: https://modelcontextprotocol.io
- **Spectre GitHub**: https://github.com/moolen/spectre
- **Claude Desktop**: https://claude.ai/download

## Support

- **Report Issues**: https://github.com/moolen/spectre/issues
- **Discussions**: https://github.com/moolen/spectre/discussions
- **Documentation**: https://moolen.github.io/spectre/

<!-- Source: README.md lines 72-285, all MCP documentation, plan.md -->
