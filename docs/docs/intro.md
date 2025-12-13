---
title: Introduction to Spectre
description: Learn about Spectre, a Kubernetes event monitoring and auditing system
keywords: [kubernetes, monitoring, events, auditing, k8s, observability]
sidebar_position: 1
---

# Introduction to Spectre

Spectre is a Kubernetes event monitoring and auditing system that captures all resource changes across your cluster and provides a powerful visualization dashboard to understand what happened, when it happened, and why.

![Spectre Timeline](/img/screenshot-2.png)

## What is Spectre?

In Kubernetes environments, resources are constantly changing. Without proper visibility, it's difficult to:

- **Track resource changes** - What changed and when?
- **Debug issues** - Understand the sequence of events that led to a problem
- **Troubleshoot failures** - Help with incident response or post-mortem analysis

Spectre solves these problems by providing comprehensive event monitoring and auditing capabilities specifically designed for Kubernetes.

## Key Features

### 1. Real-time Event Capture

Every resource change is captured instantly using the Kubernetes watch API. Spectre monitors any resource type (Pods, Deployments, Services, Custom Resources, etc.) and records:
- CREATE events - When resources are created
- UPDATE events - When resources are modified
- DELETE events - When resources are removed

### 2. Efficient Storage

Events are compressed and indexed for fast retrieval:
- **90%+ compression ratio** - Efficient storage using block-based compression
- **Bloom filters** - Fast filtering without reading all data
- **Sparse indexing** - O(log N) timestamp lookups
- **Inverted indexes** - Quick filtering by kind, namespace, and group

### 3. Interactive Audit Timeline

Visualize resource state changes over time with an intuitive React-based UI:
- Timeline view showing resource transitions
- Filter by namespace, resource kind, or name
- Zoom and pan through time ranges
- View full resource snapshots at any point

### 4. Flexible Filtering

Find exactly what you're looking for:
- Filter by namespace
- Filter by resource kind (Pod, Deployment, etc.)
- Filter by API group and version
- Time range queries

### 5. Historical Analysis

Query any time period to understand what happened:
- Investigate past incidents
- Build post-mortem timelines
- Track deployment rollouts
- Monitor configuration changes

### 6. AI-Assisted Analysis (MCP Integration)

Spectre provides a Model Context Protocol (MCP) server that enables AI assistants like Claude to help with:
- Automated incident investigation
- Root cause analysis
- Post-mortem report generation
- Real-time incident triage

## Architecture Overview

Spectre consists of three main components:

1. **Watcher** - Monitors Kubernetes resources and captures events
2. **Storage Engine** - Compresses and indexes events for efficient storage and retrieval
3. **API & UI** - Provides REST API and web interface for querying and visualization

```
┌─────────────┐        ┌──────────────┐        ┌─────────────┐
│ Kubernetes  │───────▶│   Watcher    │───────▶│   Storage   │
│   Cluster   │ watch  │              │ events │   Engine    │
└─────────────┘        └──────────────┘        └─────────────┘
                                                       │
                                                       │ query
                                                       ▼
                                                ┌─────────────┐
                                                │   API/UI    │
                                                │   + MCP     │
                                                └─────────────┘
```

## Who Should Use Spectre?

Spectre is ideal for:

- **SREs and DevOps Engineers** - Debug production issues and investigate incidents
- **Platform Teams** - Monitor cluster activity and track changes
- **Security Teams** - Audit resource modifications for compliance
- **Development Teams** - Understand deployment behavior and troubleshoot issues

## Comparison with Other Tools

| Feature                 | Spectre  | kubectl logs | Kubernetes Audit Logs |
| ----------------------- | -------- | ------------ | --------------------- |
| Resource state changes  | ✅        | ❌            | ✅                     |
| Full resource snapshots | ✅        | ❌            | Partial               |
| Time-based queries      | ✅        | Limited      | Partial               |
| Visual timeline         | ✅        | ❌            | ❌                     |
| Compression             | ✅ (90%+) | ❌            | ❌                     |
| AI-assisted analysis    | ✅        | ❌            | ❌                     |
| Easy setup              | ✅        | N/A          | Complex               |

## Next Steps

Ready to get started? Check out these guides:

- [Quick Start](/docs/getting-started/quick-start) - Install and run Spectre in minutes
- [Demo Mode](/docs/getting-started/demo-mode) - Try Spectre with sample data
- [Installation](/docs/installation) - Detailed installation guides
- [MCP Integration](/docs/mcp-integration) - Set up AI-assisted analysis

<!-- Source: /home/moritz/dev/spectre/README.md lines 1-27 -->
