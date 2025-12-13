---
title: Use Cases
description: Practical guides for common Spectre use cases and operational scenarios
keywords: [use cases, examples, scenarios, kubernetes, troubleshooting, incident, deployment]
---

# Use Cases

Discover how Spectre helps solve real-world Kubernetes operational challenges through comprehensive event tracking and analysis.

## Overview

Spectre provides a unified view of cluster events and resource state changes, enabling teams to:

- 🔍 **Investigate incidents** - Quickly identify root causes by correlating events and timeline reconstruction
- 📊 **Analyze post-mortems** - Generate comprehensive incident reports with complete event history
- 🚀 **Track deployments** - Monitor rollout progress, detect issues, and verify success
- 🤖 **AI-assisted analysis** - Use MCP integration for conversational troubleshooting with Claude

## Use Cases

### [Incident Investigation](./incident-investigation)

**Scenario**: Production alert fires, pods are failing, and you need to understand what went wrong.

**What Spectre provides**:
- Complete timeline of events leading to the incident
- Resource state transitions at the moment of failure
- Related events across dependent resources
- Timeline visualization in the UI or via API

**Time saved**: 5-15 minutes per incident through automated event correlation

### [Post-Mortem Analysis](./post-mortem-analysis)

**Scenario**: Incident is resolved, and you need to document what happened for future prevention.

**What Spectre provides**:
- Historical event data for complete incident reconstruction
- Chronological timeline with exact timestamps
- Impact assessment across affected resources
- Exportable reports for documentation

**Time saved**: 30-60 minutes per post-mortem with structured analysis

### [Deployment Tracking](./deployment-tracking)

**Scenario**: You deployed a new version and want to verify everything is healthy or detect issues early.

**What Spectre provides**:
- Real-time deployment event monitoring
- Pod creation, ready, and failure events
- ConfigMap and Secret change tracking
- Rollout status progression

**Time saved**: 3-5 minutes per deployment verification

## Common Patterns

### Pattern 1: Alert → Investigation → Resolution

**Workflow**:
1. **Alert fires** (Prometheus, PagerDuty, etc.)
2. **Query Spectre** for events around alert time
3. **Identify root cause** from timeline and state transitions
4. **Resolve issue** with context-aware fix
5. **Document** using Spectre's event data

**Example**: `kubectl logs` shows errors, but Spectre reveals the ConfigMap was deleted 2 minutes earlier, causing the failure.

### Pattern 2: Incident → Post-Mortem → Prevention

**Workflow**:
1. **Incident occurs** and is resolved
2. **Export event data** from Spectre for incident window
3. **Generate post-mortem** with complete timeline
4. **Identify prevention measures** based on event patterns
5. **Implement safeguards** (alerts, RBAC, validation)

**Example**: Post-mortem shows deployment failures correlate with ConfigMap updates, leading to implementation of validation webhooks.

### Pattern 3: Deployment → Verification → Rollback

**Workflow**:
1. **Deploy new version** to cluster
2. **Monitor events** in Spectre during rollout
3. **Detect issues early** (pod failures, config errors)
4. **Rollback if needed** before full impact
5. **Analyze failures** to fix before retry

**Example**: New deployment shows ImagePullBackOff events immediately, allowing quick rollback before users are affected.

## Integration with Other Tools

### MCP (Model Context Protocol)

**Use case**: AI-assisted incident investigation

**How it works**: Claude Desktop connects to Spectre via MCP and provides natural language investigation:

```
You: What happened in production namespace 30 minutes ago?

Claude: [Queries Spectre via MCP]
I found a deployment update that caused pods to fail...
[Provides timeline, root cause, and suggested fix]
```

**Learn more**: [MCP Integration Guide](../mcp-integration/index.md)

### Prometheus + Alertmanager

**Use case**: Correlate alerts with Kubernetes events

**How it works**:
1. Prometheus detects metric anomaly
2. Alertmanager fires alert
3. Runbook includes Spectre query for alert time window
4. Events reveal what changed to cause the metric spike

**Example**: High CPU alert → Spectre shows HPA scaled deployment 5 minutes earlier → New pods are stuck in CrashLoopBackOff

### GitOps (FluxCD, ArgoCD)

**Use case**: Track GitOps-driven changes

**How it works**:
1. Git commit triggers GitOps reconciliation
2. Spectre tracks GitRepository, Kustomization, and resource events
3. Timeline shows git commit → reconciliation → resource updates
4. Failures are correlated to specific commits

**Example**: Flux GitRepository shows "failed to checkout" → Spectre reveals SSH key secret was rotated at the same time

### CI/CD Pipelines

**Use case**: Deployment verification in pipelines

**How it works**:
1. CI/CD deploys to cluster
2. Pipeline queries Spectre API for deployment events
3. Script checks for error events in time window
4. Pipeline fails if issues detected, passes if clean

**Example automation**:
```bash
# Query Spectre after deployment
curl "http://spectre/api/search?query=kind:Pod,namespace:production&start=$START&end=$END" | \
  jq -e '.events[] | select(.status=="Error")' && exit 1 || exit 0
```

## Choosing the Right Approach

| Scenario | Recommended Tool | Why |
|----------|------------------|-----|
| Live incident troubleshooting | MCP + Claude Desktop | Conversational, automatic correlation |
| Historical incident analysis | Spectre UI + Export | Visual timeline, exportable data |
| Deployment monitoring | Spectre API + Scripts | Programmatic, integrates with CI/CD |
| Daily health checks | MCP prompts | Automated, structured reports |
| Audit trail | Spectre Storage + Export | Complete history, compliance-ready |

## Getting Started

1. **Deploy Spectre**: Follow [Installation Guide](../installation/index.md)
2. **Enable MCP** (optional): See [MCP Configuration](../configuration/mcp-configuration.md)
3. **Choose your use case**: Click on one of the detailed guides below
4. **Try the examples**: Each guide includes practical examples with queries

## Detailed Use Case Guides

- **[Incident Investigation](./incident-investigation)** - Step-by-step guide for troubleshooting failures
- **[Post-Mortem Analysis](./post-mortem-analysis)** - Generate comprehensive incident reports
- **[Deployment Tracking](./deployment-tracking)** - Monitor and verify deployments

## Related Documentation

- [User Guide](../user-guide/index.md) - How to use Spectre UI and API
- [MCP Integration](../mcp-integration/index.md) - AI-assisted investigations
- [Configuration](../configuration/index.md) - Optimize for your needs

<!-- Source: README.md, docs/MCP.md, use case examples -->
