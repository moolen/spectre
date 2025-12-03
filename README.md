# Spectre

<div align="center">
  <img src="ui/public/ghost.svg" alt="Spectre" width="200">
</div>

## What is Spectre?

Spectre is a Kubernetes event monitoring and auditing system. It captures all resource changes (create, update, delete) across your cluster and provides a powerful visualization dashboard to understand what happened, when it happened, and why.

![Deployment Rollout](./docs/screenshot-2.png)

### Why Spectre?

In Kubernetes environments, resources are constantly changing. Without proper visibility, it's difficult to:
- **Track resource changes** - What changed and When?
- **Debug issues** - Understand the sequence of events that led to a problem
- **Troubleshoot failures** - Helps with Incident Response or post mortem analysis

Spectre solves this by providing:

1. **Real-time Event Capture** - Every resource change is captured instantly using watches
2. **Efficient Storage** - Events are compressed and indexed for fast retrieval
3. **Interactive Audit Timeline** - Visualize resource state changes over time
4. **Flexible Filtering** - Find exactly what you're looking for by namespace, kind, or name
5. **Historical Analysis** - Query any time period to understand what happened

## Quick Start

### Using Helm

```bash
# Add the Spectre Helm repository
helm repo add spectre oci://ghcr.io/moolen/charts
helm repo update

# Install Spectre
helm install spectre spectre/spectre \
  --namespace monitoring \
  --create-namespace

# Access the UI
kubectl port-forward -n monitoring svc/spectre 8080:8080

# Open your browser to http://localhost:8080
```

### Demo Mode (no cluster required)

Spectre ships with a set of Kubernetes events that can be served without connecting to a live cluster. Demo mode only starts the API/UI stack, so you can explore the product UI immediately.

```bash
# Launch the container in demo mode
docker run --rm -p 8080:8080 ghcr.io/moolen/spectre:latest
```

Then open `http://localhost:8080` (or the port you passed via `--api-port`) in your browser. You will find:
- Multiple resource types (Deployments, Pods, StatefulSets, Services, ConfigMaps, Nodes, HelmReleases)
- Successful deployment/scaling events across seven days of simulated history
- Full filtering/search support identical to a live cluster

## Configuration

### Watcher Configuration

Create a `watcher.yaml` file to specify which resources to monitor:

```yaml
resources:
  - group: ""
    version: "v1"
    kind: "Pod"
    namespace: ""
  - group: "apps"
    version: "v1"
    kind: "Deployment"
    namespace: ""
```
