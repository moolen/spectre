---
title: Quick Start
description: Install and run Spectre in minutes using Helm
keywords: [kubernetes, helm, installation, quick start]
---

# Quick Start

Get Spectre running in your Kubernetes cluster in less than 5 minutes using Helm.

## Prerequisites

- Kubernetes cluster (version 1.20+)
- `kubectl` configured to access your cluster
- Helm 3+ installed

## Installation Steps

### 1. Install with Helm

```bash
# Install Spectre from the OCI registry
helm install spectre oci://ghcr.io/moolen/charts/spectre \
  --namespace monitoring \
  --create-namespace
```

This will:
- Create the `monitoring` namespace
- Deploy Spectre with default configuration
- Set up RBAC permissions
- Create persistent storage for events

### 2. Verify Installation

Check that the Spectre pod is running:

```bash
kubectl get pods -n monitoring
```

You should see output similar to:
```
NAME                       READY   STATUS    RESTARTS   AGE
spectre-xxxxxxxxxx-xxxxx   1/1     Running   0          30s
```

### 3. Access the UI

Forward the Spectre service port to your local machine:

```bash
kubectl port-forward -n monitoring svc/spectre 8080:8080
```

### 4. Open in Browser

Open your browser to http://localhost:8080

You should see the Spectre timeline interface!

## What's Next?

Now that Spectre is running:

- **Explore the UI** - Filter events by namespace, kind, or time range
- **Configure Resources** - Customize which resources to monitor in [Watcher Configuration](../configuration/watcher-config)
- **Set up MCP** - Enable AI-assisted incident analysis with [MCP Integration](../mcp-integration)
- **Learn More** - Read about [Architecture](../architecture) and [Operations](../operations)

## Troubleshooting

### Pod won't start

Check the logs:
```bash
kubectl logs -n monitoring deployment/spectre
```

### No events showing up

Make sure resources are being created/updated in your cluster. Spectre only captures events after it starts running.

### Need help?

See the full [Troubleshooting Guide](../operations/troubleshooting) or [open an issue](https://github.com/moolen/spectre/issues).

<!-- Source: /home/moritz/dev/spectre/README.md lines 29-42 -->
