---
title: Helm Installation
description: Install Spectre using Helm
keywords: [helm, installation, kubernetes, deployment]
---

# Helm Installation

Install Spectre in your Kubernetes cluster using Helm - the recommended installation method.

## Prerequisites

- Kubernetes cluster (version 1.20+)
- `kubectl` configured to access your cluster
- Helm 3+ installed
- Sufficient permissions to create namespaces and deploy workloads

## Basic Installation

### 1. Install Spectre

```bash
helm install spectre oci://ghcr.io/moolen/charts/spectre \
  --namespace monitoring \
  --create-namespace
```

### 2. Verify Installation

```bash
kubectl get pods -n monitoring
kubectl logs -n monitoring deployment/spectre
```

## Custom Installation

### Using a Values File

Create a `custom-values.yaml` file:

```yaml
# custom-values.yaml
persistence:
  enabled: true
  size: 50Gi
  storageClass: "fast-ssd"

resources:
  requests:
    memory: "512Mi"
    cpu: "200m"
  limits:
    memory: "2Gi"
    cpu: "1000m"

config:
  watcher:
    resources:
      - group: ""
        version: "v1"
        kind: "Pod"
      - group: "apps"
        version: "v1"
        kind: "Deployment"
      - group: "apps"
        version: "v1"
        kind: "StatefulSet"

mcp:
  enabled: true
```

Install with custom values:

```bash
helm install spectre oci://ghcr.io/moolen/charts/spectre \
  --namespace monitoring \
  --create-namespace \
  -f custom-values.yaml
```

## Configuration Options

<!-- TODO: Add detailed configuration table from chart/values.yaml -->

Key configuration options:

- **persistence.size** - Storage size for events (default: 10Gi)
- **resources** - CPU and memory limits
- **config.watcher.resources** - Which Kubernetes resources to monitor
- **mcp.enabled** - Enable MCP sidecar for AI integration

See [Helm Values Reference](../reference/helm-values) for all options.

## Accessing Spectre

### Port Forward (Development)

```bash
kubectl port-forward -n monitoring svc/spectre 8080:8080
```

Access at http://localhost:8080

### Ingress (Production)

<!-- TODO: Add ingress configuration example -->

### LoadBalancer

<!-- TODO: Add LoadBalancer configuration example -->

## Upgrading

```bash
helm upgrade spectre oci://ghcr.io/moolen/charts/spectre \
  --namespace monitoring \
  -f custom-values.yaml
```

## Uninstalling

```bash
helm uninstall spectre --namespace monitoring
```

**Warning:** This will delete all stored events unless persistence is configured.

## Next Steps

- [Configure Watcher](../configuration/watcher-config) to monitor specific resources
- [Configure Storage](../configuration/storage-settings) for optimal performance
- [Enable MCP](../configuration/mcp-configuration) for AI-assisted analysis

<!-- Source: /home/moritz/dev/spectre/README.md, docs/OPERATIONS.md lines 18-88, chart/values.yaml -->
