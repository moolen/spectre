---
title: Watcher Configuration
description: Configure which Kubernetes resources to monitor
keywords: [watcher, resources, monitoring, gvk]
---

# Watcher Configuration

The Spectre watcher monitors Kubernetes resources and captures their state changes over time. This page explains how to configure which resources to watch and how namespace filtering works.

## Overview

The watcher:
- Monitors any Kubernetes resource type (built-in or custom)
- Captures CREATE, UPDATE, and DELETE events
- Supports namespace filtering for focused monitoring
- Automatically reloads configuration without restarts
- Efficiently uses a single watcher per resource type (GVR)

## Default Configuration

By default, Spectre monitors **7 core Kubernetes resource types** as defined in `chart/values.yaml`:

```yaml
config:
  watcher:
    resources:
      - group: ""
        version: "v1"
        kind: "Pod"
      - group: "apps"
        version: "v1"
        kind: "Deployment"
      - group: ""
        version: "v1"
        kind: "Service"
      - group: ""
        version: "v1"
        kind: "Node"
      - group: "apps"
        version: "v1"
        kind: "StatefulSet"
      - group: "apps"
        version: "v1"
        kind: "DaemonSet"
      - group: ""
        version: "v1"
        kind: "ConfigMap"
```

These defaults cover the most commonly monitored workload and infrastructure resources.

## Resource Specification Format

Each resource specification requires three fields:

### Required Fields

- **`group`**: API group (use empty string `""` for core API resources like Pod, Service, Node)
- **`version`**: API version (e.g., `"v1"`, `"v1beta1"`)
- **`kind`**: Resource kind in PascalCase (e.g., `"Pod"`, `"Deployment"`)

### Optional Fields

- **`namespace`**: Specific namespace to watch (omit or leave empty for cluster-wide watching)

### Examples

**Core API resource (Pod):**
```yaml
- group: ""          # Core API uses empty string
  version: "v1"
  kind: "Pod"
```

**Apps API resource (Deployment):**
```yaml
- group: "apps"
  version: "v1"
  kind: "Deployment"
```

**Custom Resource (CRD):**
```yaml
- group: "cert-manager.io"
  version: "v1"
  kind: "Certificate"
```

## Automatic RBAC Permissions

The Helm chart **automatically grants RBAC permissions** for all configured resources via `chart/templates/clusterrole.yaml`.

### Static Permissions

The ClusterRole includes static permissions for common Kubernetes resources:

```yaml
# Core API resources
- apiGroups: [""]
  resources: [pods, services, configmaps, secrets, nodes, ...]
  verbs: ["watch", "list", "get"]

# Apps API group
- apiGroups: ["apps"]
  resources: [deployments, statefulsets, daemonsets, replicasets]
  verbs: ["watch", "list", "get"]

# Batch, Storage, Networking, Policy, RBAC...
```

### Dynamic Permissions

For resources defined in `config.watcher.resources`, permissions are **automatically generated**:

```yaml
{{- $watchResources := .Values.config.watcher.resources | default list }}
{{- range $watchResources }}
- apiGroups:
    - {{ default "" .group | quote }}
  resources:
    - {{ include "spectre.kindToResource" . }}
  verbs: ["watch", "list", "get"]
{{- end }}
```

The `spectre.kindToResource` helper converts Kind names to resource names:
- `Pod` → `pods`
- `Deployment` → `deployments`
- `Ingress` → `ingresses` (special case)
- Generally: lowercase + 's' suffix

**Important:** When you add new resources to `config.watcher.resources`, you must run `helm upgrade` to update the ClusterRole. Once the config changes, spectre will pick up the change at runtime. No need to re-deploy.

## Namespace Management

Spectre supports both **cluster-wide** and **namespace-scoped** watching.

### How It Works

1. **Cluster-wide watching** (default): Monitors resources across all namespaces
2. **Namespace filtering**: Client-side filtering of events from specific namespaces
3. **Efficiency**: One watcher per GVR (Group/Version/Resource), regardless of namespace count

**Note:** The watcher always watches at the cluster level but filters events client-side. This means:
- ✅ Simple configuration
- ✅ No missed resources in new namespaces (when cluster-wide)
- ❌ Cannot reduce Kubernetes API server load via namespace scoping
- ❌ Namespace changes require config reload

### Cluster-Wide Watching

**Configuration:**
```yaml
resources:
  - group: ""
    version: "v1"
    kind: "Pod"
    # No namespace specified = all namespaces
```

**Use when:**
- You want complete cluster visibility
- Namespaces are dynamic
- Storage is not a concern

### Single Namespace

**Configuration:**
```yaml
resources:
  - group: ""
    version: "v1"
    kind: "Pod"
    namespace: "production"
```

**Use when:**
- You only care about specific namespaces
- Want to reduce storage usage
- Have many namespaces but monitor few

### Multiple Namespaces

**Configuration:**
```yaml
resources:
  - group: ""
    version: "v1"
    kind: "Pod"
    namespace: "production"
  - group: ""
    version: "v1"
    kind: "Pod"
    namespace: "staging"
  - group: ""
    version: "v1"
    kind: "Pod"
    namespace: "development"
```

This creates a **single watcher** for `v1/pods` that filters for all three namespaces.

**Use when:**
- You have a known set of important namespaces
- Want to exclude noisy namespaces (kube-system, etc.)
- Need to balance visibility and storage

### Mixing Cluster-Wide and Namespaced

```yaml
resources:
  # Cluster-wide for Nodes (cluster-scoped resource)
  - group: ""
    version: "v1"
    kind: "Node"

  # All namespaces for Deployments
  - group: "apps"
    version: "v1"
    kind: "Deployment"

  # Specific namespace for Pods
  - group: ""
    version: "v1"
    kind: "Pod"
    namespace: "production"
```

## Configuration Examples

### Basic Setup

Monitor essential workload resources:

```yaml
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
```

### Production Monitoring

Focus on production namespace with key resources:

```yaml
config:
  watcher:
    resources:
      # Production workloads
      - group: ""
        version: "v1"
        kind: "Pod"
        namespace: "production"
      - group: "apps"
        version: "v1"
        kind: "Deployment"
        namespace: "production"
      - group: "apps"
        version: "v1"
        kind: "StatefulSet"
        namespace: "production"

      # Cluster-wide infrastructure
      - group: ""
        version: "v1"
        kind: "Node"
      - group: ""
        version: "v1"
        kind: "PersistentVolume"
```

### Custom Resources (CRDs)

Monitor Flux CD resources:

```yaml
config:
  watcher:
    resources:
      # Flux GitOps resources
      - group: "source.toolkit.fluxcd.io"
        version: "v1"
        kind: "GitRepository"
      - group: "kustomize.toolkit.fluxcd.io"
        version: "v1"
        kind: "Kustomization"
      - group: "helm.toolkit.fluxcd.io"
        version: "v2"
        kind: "HelmRelease"

      # Cert-Manager
      - group: "cert-manager.io"
        version: "v1"
        kind: "Certificate"
      - group: "cert-manager.io"
        version: "v1"
        kind: "CertificateRequest"
```

**Note:** CRDs must exist in the cluster before Spectre starts, or they will be logged as errors (non-fatal).

### Multi-Environment

Monitor multiple environments separately:

```yaml
config:
  watcher:
    resources:
      # Production
      - group: "apps"
        version: "v1"
        kind: "Deployment"
        namespace: "prod-frontend"
      - group: "apps"
        version: "v1"
        kind: "Deployment"
        namespace: "prod-backend"

      # Staging
      - group: "apps"
        version: "v1"
        kind: "Deployment"
        namespace: "staging"

      # Shared infrastructure (all namespaces)
      - group: ""
        version: "v1"
        kind: "Service"
      - group: "networking.k8s.io"
        version: "v1"
        kind: "Ingress"
```

## Hot Reload

Spectre **automatically reloads** watcher configuration without restarts.

### How It Works

1. Configuration file is checked every **5 seconds**
2. Changes are detected via SHA256 hash comparison
3. Watchers are gracefully stopped and restarted
4. **Zero downtime** - readiness stays true during reload
5. Invalid configurations are logged but don't stop existing watchers

### Updating Configuration

**Via Helm:**
```bash
# Update values.yaml
helm upgrade spectre ./chart -f values.yaml

# The ConfigMap is updated and Spectre detects it within 5 seconds
```

**Via kubectl:**
```bash
# Edit ConfigMap directly
kubectl edit configmap spectre -n monitoring

# Changes take effect within 5 seconds
```

### Monitoring Reloads

Check logs for reload activity:
```bash
kubectl logs -n monitoring deployment/spectre | grep -i reload
```

Expected output:
```
[INFO] watcher: Configuration changed, reloading watchers...
[INFO] watcher: Successfully reloaded 5 watchers
```

## Performance Considerations

### Memory Usage

Typical resource usage:

| Cluster Size          | Resources Watched | Memory Request | Memory Limit |
| --------------------- | ----------------- | -------------- | ------------ |
| Small (50 resources) | 1-10 types        | 128Mi          | 512Mi        |
| Medium (50-500)       | 5-20 types        | 256Mi          | 1Gi          |
| Large (>500)          | 10-30 types       | 512Mi          | 2Gi          |

**Default from `values.yaml`:**
```yaml
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

### Watch Efficiency

- **One watcher per GVR**: Watching Pods in 10 namespaces = 1 watcher, not 10
- **Pagination**: Lists resources in batches of 500
- **Client-side filtering**: Namespace filtering happens in Spectre, not at API server
- **Retry logic**: Exponential backoff on errors (5s initial delay)

### Storage Optimization

- **ManagedFields pruning**: Removes Kubernetes metadata to reduce object size by ~30-50%
- **Compression**: Events are compressed before storage
- **Event queue**: Buffered queue prevents memory spikes (drops events when full)

### Recommendations

**Start Simple:**
```yaml
# Begin with defaults
config:
  watcher:
    resources:
      - group: ""
        version: "v1"
        kind: "Pod"
```

**Add Gradually:**
Monitor resource usage as you add more resource types.

**Use Namespace Filtering:**
If you have many namespaces but only care about a few, use namespace filtering to reduce event volume:

```yaml
resources:
  - group: ""
    version: "v1"
    kind: "Pod"
    namespace: "critical-app"
```

**Watch Cluster-Scoped Resources Sparingly:**
Resources like Nodes, PersistentVolumes, and ClusterRoles change less frequently but can't be namespace-filtered.

## Best Practices

### ✅ Do

- **Start with defaults** and add resources as needed
- **Use namespace filtering** to reduce noise in large clusters
- **Monitor memory usage** and adjust limits accordingly
- **Group related resources** in your configuration for clarity
- **Include CRDs** that are critical to your infrastructure (GitOps, service mesh, etc.)
- **Test configuration changes** in development before production
- **Monitor queue metrics** to ensure events aren't being dropped

### ❌ Don't

- **Don't watch resources you don't query** - it wastes storage
- **Don't add all CRDs blindly** - only watch what you need
- **Don't ignore memory limits** - set them based on your cluster size
- **Don't forget RBAC updates** - `helm upgrade` when adding new resource types
- **Don't mix versions** - if you watch `apps/v1/Deployment`, don't also watch `apps/v1beta1/Deployment`

### Validation Checklist

Before deploying a new watcher configuration:

- [ ] All required fields present (group, version, kind)
- [ ] API versions match your cluster (check with `kubectl api-resources`)
- [ ] CRDs exist if watching custom resources
- [ ] RBAC permissions will be granted (via Helm upgrade)
- [ ] Namespace names are correct (if using namespace filtering)
- [ ] Memory limits adjusted for cluster size
- [ ] Configuration validated locally:
  ```bash
  # Validate YAML syntax
  cat watcher.yaml | yq eval
  ```

## Troubleshooting

### Resource Not Being Watched

**Problem:** Added a resource but not seeing events

**Solutions:**
1. Check if resource exists: `kubectl api-resources | grep <resource>`
2. Verify RBAC: `kubectl auth can-i watch <resource> --as=system:serviceaccount:monitoring:spectre`
3. Check logs: `kubectl logs -n monitoring deployment/spectre | grep -i <kind>`
4. Confirm Helm upgrade ran: `kubectl get clusterrole spectre -o yaml`

### CRD Not Found

**Problem:** Logs show "failed to resolve GVR" for custom resource

**Solution:**
The CRD doesn't exist. Install it first:
```bash
# Check if CRD exists
kubectl get crd | grep <name>

# Install the CRD (example for cert-manager)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.crds.yaml
```

### Events Being Dropped

**Problem:** Logs show "Event queue is full, dropping event"

**Solutions:**
1. Increase queue size (requires code change)
2. Reduce watched resources
3. Add namespace filtering
4. Increase CPU/memory limits

### High Memory Usage

**Problem:** Spectre pod using more memory than expected

**Solutions:**
1. Check number of resources being watched
2. Add namespace filtering to reduce event volume
3. Verify managedFields pruning is working (check event sizes in storage)
4. Increase memory limits in `values.yaml`:
   ```yaml
   resources:
     limits:
       memory: "1Gi"  # Increase from 512Mi
   ```

### Configuration Not Reloading

**Problem:** Changed ConfigMap but Spectre still using old config

**Solutions:**
1. Check if ConfigMap was updated: `kubectl get cm spectre -n monitoring -o yaml`
2. Wait 5 seconds (reload interval)
3. Check logs for reload messages
4. Restart pod if necessary: `kubectl rollout restart deployment/spectre -n monitoring`

## Integration Details

### ConfigMap Mounting

Configuration is stored in a Kubernetes ConfigMap and mounted into the pod:

**ConfigMap:** `chart/templates/configmap.yaml`
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: spectre
data:
  watcher.yaml: |
    resources:
    {{- range .Values.config.watcher.resources }}
      - group: {{ .group | quote }}
        version: {{ .version | quote }}
        kind: {{ .kind | quote }}
        {{- if .namespace }}
        namespace: {{ .namespace | quote }}
        {{- end }}
    {{- end }}
```

**Mount Point:** `/etc/watcher/watcher.yaml`

### Command-Line Flags

Spectre binary is started with:
```bash
spectre --watcher-config=/etc/watcher/watcher.yaml
```

### Storage Integration

Each event captured by the watcher includes:

- **Event ID**: Unique UUID
- **Timestamp**: Unix nanoseconds
- **Type**: CREATE, UPDATE, or DELETE
- **Resource Metadata**: Group, version, kind, namespace, name, UID
- **Data**: Full resource JSON (with managedFields pruned)
- **Sizes**: Original and compressed sizes

Events are written to block-based storage for efficient querying.

## Related Documentation

- [Architecture Overview](../architecture/overview.md) - Understanding Spectre's design
- [Storage Configuration](./storage.md) - Configuring event storage
- [API Reference](../../api/search.md) - Querying stored events
- [Operations Guide](../../operations/monitoring.md) - Monitoring and troubleshooting

## Examples in Repository

See `hack/watcher.yaml` for an extended example configuration with various resource types and comments.

<!-- Source: /home/moritz/dev/spectre/README.md lines 45-60, chart/values.yaml lines 104-127, internal/watcher/ -->
