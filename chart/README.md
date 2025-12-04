# Kubernetes Event Monitor Helm Chart

A Helm chart for deploying the Kubernetes Event Monitoring and Storage System to a Kubernetes cluster.

## Prerequisites

- Kubernetes 1.19+
- Helm 3+
- A persistent volume or storage class for event data storage

## Installation

### Quick Start

```bash
# Add the chart repository (if hosted remotely)
helm repo add rpk https://example.com/helm-charts

# Install with default values
helm install spectre rpk/spectre \
  --namespace monitoring \
  --create-namespace

# Verify installation
kubectl get pods -n monitoring
```

### Local Installation

```bash
# From the chart directory
helm install spectre ./chart \
  --namespace monitoring \
  --create-namespace
```

## Configuration

The chart provides comprehensive configuration options. Common configurations are shown below:

### Basic Configuration

```bash
helm install spectre ./chart \
  --namespace monitoring \
  --set persistence.size=20Gi \
  --set resources.limits.memory=1Gi
```

### With Custom Values File

```bash
helm install spectre ./chart \
  --namespace monitoring \
  --values custom-values.yaml
```

## Values

### Global Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `1` |
| `namespace` | Kubernetes namespace | `monitoring` |

### Image Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Image repository | `spectre` |
| `image.tag` | Image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |

### Resource Management

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.requests.memory` | Memory request | `128Mi` |
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.limits.memory` | Memory limit | `512Mi` |
| `resources.limits.cpu` | CPU limit | `500m` |

### Persistence

| Parameter | Description | Default |
|-----------|-------------|---------|
| `persistence.enabled` | Enable persistent storage | `true` |
| `persistence.size` | PVC size | `10Gi` |
| `persistence.mountPath` | Mount path in container | `/data` |
| `persistence.storageClassName` | Storage class name | (default) |

### Service Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `8080` |
| `service.targetPort` | Container port | `8080` |

### Health Probes

| Parameter | Description | Default |
|-----------|-------------|---------|
| `livenessProbe.enabled` | Enable liveness probe | `true` |
| `livenessProbe.initialDelaySeconds` | Initial delay | `10` |
| `livenessProbe.periodSeconds` | Probe interval | `10` |
| `readinessProbe.enabled` | Enable readiness probe | `true` |
| `readinessProbe.initialDelaySeconds` | Initial delay | `5` |
| `readinessProbe.periodSeconds` | Probe interval | `5` |

### Application Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.logLevel` | Log level | `info` |
| `config.watcher.resources` | List of resources to watch (GVK format) | (see values.yaml) |

#### Watcher Configuration

The watcher configuration uses Group/Version/Kind (GVK) format to specify which Kubernetes resources to monitor. Each resource can optionally specify a namespace (if omitted, watches cluster-wide).

Example configuration:

```yaml
config:
  watcher:
    resources:
      # Core v1 Pods (cluster-wide)
      - group: ""
        version: "v1"
        kind: "Pod"
      # Apps v1 Deployments in specific namespace
      - group: "apps"
        version: "v1"
        kind: "Deployment"
        namespace: "default"
      # Core v1 Services (cluster-wide)
      - group: ""
        version: "v1"
        kind: "Service"
```

**Notes:**
- For core resources (Pod, Service, Node, etc.), use an empty string `""` for the group
- If `namespace` is omitted, the watcher monitors the resource cluster-wide
- Cluster-scoped resources (like Node) ignore the namespace field
- The watcher config is mounted as a ConfigMap at `/etc/watcher/watcher.yaml`
- Changes to the ConfigMap trigger automatic hot-reload of watchers

### Security

| Parameter | Description | Default |
|-----------|-------------|---------|
| `securityContext.runAsNonRoot` | Run as non-root | `true` |
| `securityContext.runAsUser` | User ID | `1000` |
| `securityContext.fsGroup` | Filesystem group | `1000` |

## Advanced Usage

### Different Storage Classes

```bash
helm install spectre ./chart \
  --namespace monitoring \
  --set persistence.storageClassName=fast-ssd
```

### Custom Resource Limits

```bash
helm install spectre ./chart \
  --namespace monitoring \
  --set resources.requests.memory=256Mi \
  --set resources.limits.memory=2Gi
```

### Node Affinity

```bash
helm install spectre ./chart \
  --namespace monitoring \
  -f - <<EOF
nodeSelector:
  disk: fast
tolerations:
  - key: dedicated
    operator: Equal
    value: monitoring
    effect: NoSchedule
EOF
```

## Verification

After installation, verify the deployment:

```bash
# Check pod status
kubectl get pods -n monitoring

# View pod logs
kubectl logs -n monitoring deployment/spectre -f

# Port-forward to access API
kubectl port-forward -n monitoring svc/spectre 8080:8080 &

# Test API
curl http://localhost:8080/v1/search?start=1&end=2

# Check storage
kubectl exec -n monitoring -it deployment/spectre -- df -h /data
```

## Troubleshooting

### Pod Won't Start

```bash
# Check pod events
kubectl describe pod -n monitoring -l app.kubernetes.io/name=spectre

# View logs
kubectl logs -n monitoring -l app.kubernetes.io/name=spectre
```

### RBAC Issues

```bash
# Verify service account has permissions
kubectl auth can-i watch pods \
  --as=system:serviceaccount:monitoring:spectre

# Check cluster role
kubectl describe clusterrole spectre

# Check cluster role binding
kubectl describe clusterrolebinding spectre
```

### Storage Issues

```bash
# Check PVC status
kubectl get pvc -n monitoring

# Check available storage classes
kubectl get storageclass

# Check disk usage
kubectl exec -n monitoring -it deployment/spectre -- du -sh /data
```

## Upgrade

```bash
# Upgrade to new values
helm upgrade spectre ./chart \
  --namespace monitoring \
  --values new-values.yaml
```

## Uninstall

```bash
# Remove the deployment
helm uninstall spectre --namespace monitoring

# Remove the namespace (optional)
kubectl delete namespace monitoring
```

## Support

For issues and questions, refer to:
- Main repository: https://github.com/moolen/spectre
- Feature specification: `specs/001-spectre/spec.md`
- Quickstart guide: `specs/001-spectre/quickstart.md`
