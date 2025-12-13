---
title: MCP Configuration
description: Deploy and configure the Model Context Protocol server
keywords: [mcp, ai, claude, configuration, deployment, sidecar, helm]
---

# MCP Configuration

This guide explains how to deploy and configure Spectre's MCP (Model Context Protocol) server for AI-assisted incident investigation.

## Overview

### What is the MCP Server?

The MCP server is a separate component that exposes Spectre's Kubernetes event data through the standardized Model Context Protocol (JSON-RPC 2.0). It enables AI assistants like Claude to help with:

- Automated incident investigation
- Root cause analysis
- Post-mortem report generation
- Real-time incident triage

### Architecture

```
┌─────────────────┐
│ AI Assistant    │ (Claude Desktop, API clients, etc.)
│  (MCP Client)   │
└────────┬────────┘
         │ JSON-RPC (HTTP or stdio)
         │
┌────────▼────────┐
│  Spectre MCP    │
│     Server      │
└────────┬────────┘
         │ HTTP
         │
┌────────▼────────┐
│  Spectre API    │
│   (Main App)    │
└────────┬────────┘
         │
   Kubernetes Events
```

**Key Characteristics**:
- **Protocol**: MCP 2024-11-05 (JSON-RPC 2.0)
- **Transport Modes**: HTTP (default) or stdio
- **Deployment**: Sidecar (recommended) or standalone
- **Communication**: Connects to Spectre API server

## Quick Start

### Enabling in Helm (Sidecar Mode)

The simplest way to enable MCP is via the Helm chart sidecar configuration:

```yaml
# values.yaml
mcp:
  enabled: true
  spectreURL: "http://localhost:8080"  # Main container via localhost
  httpAddr: ":8081"
  port: 8081
```

Deploy or upgrade:

```bash
helm upgrade --install spectre ./chart \
  --set mcp.enabled=true \
  --namespace monitoring
```

**Verification**:

```bash
# Check MCP sidecar is running
kubectl get pods -n monitoring -l app.kubernetes.io/name=spectre

# Test health endpoint
kubectl port-forward -n monitoring svc/spectre-mcp 8081:8081
curl http://localhost:8081/health
# {"status":"ok"}
```

## Configuration Reference

### CLI Flags

When running MCP server standalone:

| Flag            | Default                 | Description                                    |
| --------------- | ----------------------- | ---------------------------------------------- |
| `--spectre-url` | `http://localhost:8080` | URL to Spectre API server (env: `SPECTRE_URL`) |
| `--http-addr`   | `:8081`                 | HTTP server address (env: `MCP_HTTP_ADDR`)     |
| `--transport`   | `http`                  | Transport type: `http` or `stdio`              |


### Endpoints

| Endpoint  | Method | Description                              |
| --------- | ------ | ---------------------------------------- |
| `/mcp`    | POST   | Main MCP JSON-RPC endpoint               |
| `/health` | GET    | Health check (returns `{"status":"ok"}`) |
| `/`       | GET    | Server info (name and version)           |

## Transport Modes

### HTTP Transport (Default)

**Use Case**: Independent deployment, cloud environments, multi-cluster access

**Characteristics**:
- HTTP server on configurable port (default: 8081)
- JSON-RPC 2.0 over HTTP POST
- Stateless request/response
- Suitable for remote clients

**Configuration**:

```bash
# Helm sidecar (default)
mcp:
  enabled: true
  httpAddr: ":8081"

# Standalone CLI
spectre mcp --transport=http --http-addr=:8081 --spectre-url=http://spectre-api:8080
```

### Stdio Transport

You can also use stdio-based transport if you don't want to use HTTP.

**Configuration**:

```bash
spectre mcp --transport=stdio --spectre-url=http://localhost:8080
```

**Use with Claude Desktop**:

See [Claude Integration](../mcp-integration/claude-integration) for complete setup.

**Limitations**:
- No HTTP endpoints (health checks not available)
- Subprocess mode only
- Requires process spawning capability

## Deployment Patterns

### Sidecar Mode (Recommended)

**Why Sidecar**:
- ✅ Shared network namespace (localhost communication to Spectre API)
- ✅ Simplest configuration
- ✅ Automatic lifecycle management
- ✅ Resource limits per pod
- ✅ Same security context as main container

**Architecture**:

```
┌─────────────────────────────────────────────┐
│ Pod: spectre                                 │
│  ┌──────────────┐      ┌─────────────────┐ │
│  │  Container:  │      │   Container:    │ │
│  │   spectre    │◄────►│   spectre-mcp   │ │
│  │  (port 8080) │ localhost  (port 8081) │ │
│  └──────────────┘      └─────────────────┘ │
└─────────────────────────────────────────────┘
```

**Configuration**:

```yaml
# values.yaml
mcp:
  enabled: true
  spectreURL: "http://localhost:8080"  # Localhost within pod
  port: 8081
  resources:
    requests:
      memory: "64Mi"
      cpu: "50m"
    limits:
      memory: "256Mi"
      cpu: "200m"
```

### Standalone Mode

**When to Use**:
- Separate scaling requirements (MCP and Spectre scale independently)
- Multi-cluster support (one MCP server, multiple Spectre instances)
- Cloud MCP services (external AI platforms)
- Development/testing isolation

**Architecture**:

```
┌───────────────┐      ┌───────────────┐
│ Pod: spectre  │      │ Pod: mcp      │
│ (port 8080)   │◄────►│ (port 8081)   │
└───────────────┘ http └───────────────┘
```

**Network Configuration**:

```yaml
# Ensure network policy allows MCP → Spectre traffic
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: spectre-mcp-access
  namespace: monitoring
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: spectre
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: spectre-mcp
    ports:
    - protocol: TCP
      port: 8080
```

## Resource Planning

### Memory Requirements

| Component          | Typical        | Peak         | Notes                |
| ------------------ | -------------- | ------------ | -------------------- |
| Base memory        | 40-50 Mi       | 60-80 Mi     | Idle server          |
| Per request        | +5-10 Mi       | +20 Mi       | Active investigation |
| Total (sidecar)    | 64 Mi request  | 256 Mi limit | Recommended          |
| Total (standalone) | 128 Mi request | 512 Mi limit | Higher concurrency   |

**Factors**:
- Number of concurrent MCP sessions
- Query time ranges (wider = more memory)
- Result set sizes (filtered queries use less memory)

### CPU Requirements

| Workload                  | CPU Request | CPU Limit | Notes              |
| ------------------------- | ----------- | --------- | ------------------ |
| Low (1-2 queries/min)     | 50m         | 200m      | Single user        |
| Medium (5-10 queries/min) | 100m        | 500m      | Team usage         |
| High (20+ queries/min)    | 250m        | 1000m     | Automated analysis |

**CPU Usage**:
- Mostly idle (waiting for Spectre API responses)
- Bursts during JSON parsing/serialization
- Minimal CPU for typical AI-assistant workloads

### Scaling Considerations

**Vertical Scaling**:
- Increase memory limits for larger query results
- Increase CPU for high-concurrency scenarios

**Horizontal Scaling**:
- MCP server is stateless (safe to scale horizontally)
- Load balance across multiple MCP pods
- No session affinity required

## Security

### Network Policies

**Restrict MCP Port Access**:

```yaml
# Only allow internal cluster access to MCP
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: spectre-mcp-ingress
  namespace: monitoring
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: spectre
      app.kubernetes.io/component: mcp
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: monitoring
    ports:
    - protocol: TCP
      port: 8081
```

### RBAC

**MCP Server Permissions**:
- ✅ No direct Kubernetes API access required
- ✅ Communicates only with Spectre API
- ✅ Spectre API enforces access control

**User Access Control**:
- MCP server itself has **no authentication** in v1.0
- Access control via network policies and service exposure
- For production: place behind authenticated proxy/gateway

### Authentication and Authorization

**Current State (v1.0)**:
- ❌ No built-in authentication
- ❌ No API key support
- ⚠️ Secure via network policies only

**Future (v2.0 planned)**:
- API key authentication
- Per-client access control
- Audit logging

**Workarounds for v1.0**:

1. **Network Isolation**: Don't expose MCP port publicly
2. **Authenticated Proxy**: Use nginx/Envoy with auth
3. **VPN/Bastion**: Require VPN access to MCP endpoint

### TLS/Encryption

**HTTP Transport**:
- Currently plain HTTP
- Add TLS via ingress controller or reverse proxy

**Example: Nginx Ingress with TLS**:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: spectre-mcp
  namespace: monitoring
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/auth-type: basic
    nginx.ingress.kubernetes.io/auth-secret: mcp-basic-auth
spec:
  tls:
  - hosts:
    - mcp.example.com
    secretName: mcp-tls
  rules:
  - host: mcp.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: spectre-mcp
            port:
              number: 8081
```

## Health Monitoring

### Health Endpoint

**Endpoint**: `GET /health`
**Response**: `{"status":"ok"}`

**Usage**:

```bash
# Direct check
curl http://spectre-mcp:8081/health

# Port-forward check
kubectl port-forward -n monitoring svc/spectre-mcp 8081:8081
curl http://localhost:8081/health
```

### Kubernetes Probes

**Liveness Probe** (default configuration):

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8081
  initialDelaySeconds: 5
  periodSeconds: 10
  timeoutSeconds: 3
  failureThreshold: 3
```

**Purpose**: Restart unhealthy MCP containers

**Readiness Probe** (default configuration):

```yaml
readinessProbe:
  httpGet:
    path: /health
    port: 8081
  initialDelaySeconds: 3
  periodSeconds: 5
  timeoutSeconds: 2
  failureThreshold: 3
```

**Purpose**: Remove unready pods from service endpoints

### Logging

**Log Output**:
- HTTP transport: Logs to container stdout/stderr
- Stdio transport: Logs to stderr (stdout reserved for MCP protocol)

**Viewing Logs**:

```bash
# Sidecar logs
kubectl logs -n monitoring <pod-name> -c spectre-mcp

# Standalone logs
kubectl logs -n monitoring -l app=spectre-mcp

# Follow logs
kubectl logs -n monitoring <pod-name> -c spectre-mcp -f
```

**Log Levels**:
- Controlled by main Spectre log level configuration
- Includes: connection events, tool calls, errors

## Troubleshooting

### Connection Failures

**Symptom**: MCP server cannot connect to Spectre API

**Check**:

```bash
# From MCP pod, test Spectre API reachability
kubectl exec -n monitoring <pod-name> -c spectre-mcp -- \
  curl http://localhost:8080/api/v1/metadata

# Check logs for connection errors
kubectl logs -n monitoring <pod-name> -c spectre-mcp | grep -i error
```

**Causes**:
- ❌ Spectre API not running
- ❌ Incorrect `spectreURL` configuration
- ❌ Network policy blocking traffic
- ❌ Spectre API authentication required (not supported in v1.0)

**Solutions**:
- Verify Spectre main container is healthy
- Check `spectreURL` in values.yaml (should be `http://localhost:8080` for sidecar)
- Review network policies

### Tool Execution Errors

**Symptom**: MCP tools return errors or empty results

**Check**:

```bash
# Test Spectre API directly
curl "http://spectre-api:8080/api/v1/query?kind=Pod&time=\[now-1h,now\]"

# Check Spectre has data
kubectl logs -n monitoring <spectre-pod> | grep "events written"
```

**Causes**:
- ❌ No events in Spectre for query time range
- ❌ Namespace doesn't exist
- ❌ Spectre API timeout (large queries)
- ❌ Invalid time format

**Solutions**:
- Verify Spectre is collecting events
- Check time range is within Spectre retention
- Use namespace and time filters to reduce query scope

### Pod CrashLoopBackOff

**Symptom**: MCP container repeatedly crashes

**Check**:

```bash
kubectl describe pod -n monitoring <pod-name>
kubectl logs -n monitoring <pod-name> -c spectre-mcp --previous
```

**Causes**:
- ❌ Out of memory (query results too large)
- ❌ Cannot bind to port (port conflict)
- ❌ Missing required configuration

**Solutions**:
- Increase memory limits
- Verify `httpAddr` port is not in use
- Check all required flags are set

### Health Check Failures

**Symptom**: Liveness/readiness probes failing

**Check**:

```bash
# Manual health check
kubectl exec -n monitoring <pod-name> -c spectre-mcp -- \
  curl -f http://localhost:8081/health

# Check probe configuration
kubectl get pod -n monitoring <pod-name> -o yaml | grep -A 10 livenessProbe
```

**Causes**:
- ❌ MCP server not started
- ❌ Port mismatch in probe configuration
- ❌ HTTP server crash

**Solutions**:
- Review startup logs for errors
- Verify probe port matches `httpAddr`
- Check resource limits (CPU throttling can delay startup)

## Configuration Examples

### Development (Local)

**Port-forward for local access**:

```bash
# Deploy with Helm
helm install spectre ./chart --set mcp.enabled=true

# Port-forward MCP
kubectl port-forward -n default svc/spectre 8081:8081

# Test
curl http://localhost:8081/health
```

### Production (Sidecar)

**Full Helm values**:

```yaml
# values.yaml
mcp:
  enabled: true
  spectreURL: "http://localhost:8080"
  httpAddr: ":8081"
  port: 8081

  resources:
    requests:
      memory: "128Mi"
      cpu: "100m"
    limits:
      memory: "512Mi"
      cpu: "500m"

  securityContext:
    allowPrivilegeEscalation: false
    capabilities:
      drop: ["ALL"]
    runAsNonRoot: true
    runAsUser: 1000

  livenessProbe:
    enabled: true
    httpGet:
      path: /health
      port: mcp
    initialDelaySeconds: 10
    periodSeconds: 15
    failureThreshold: 3

  readinessProbe:
    enabled: true
    httpGet:
      path: /health
      port: mcp
    initialDelaySeconds: 5
    periodSeconds: 10
    failureThreshold: 2
```

### Production (Standalone with Monitoring)

**Deployment with ServiceMonitor**:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: spectre-mcp
  namespace: monitoring
  labels:
    app: spectre-mcp
spec:
  replicas: 2  # Horizontal scaling
  selector:
    matchLabels:
      app: spectre-mcp
  template:
    metadata:
      labels:
        app: spectre-mcp
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8081"
    spec:
      containers:
      - name: mcp
        image: ghcr.io/moolen/spectre:latest
        command: ["/spectre"]
        args:
          - "mcp"
          - "--transport=http"
          - "--http-addr=:8081"
        env:
        - name: SPECTRE_URL
          valueFrom:
            configMapKeyRef:
              name: spectre-config
              key: api-url
        ports:
        - name: mcp
          containerPort: 8081
        livenessProbe:
          httpGet:
            path: /health
            port: 8081
          initialDelaySeconds: 10
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /health
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          requests:
            memory: "256Mi"
            cpu: "200m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        securityContext:
          allowPrivilegeEscalation: false
          runAsNonRoot: true
          runAsUser: 1000
```

## Best Practices

### ✅ Do

- **Use sidecar mode** for simplicity and localhost communication
- **Set resource limits** to prevent runaway memory usage
- **Enable health probes** for automatic failure recovery
- **Use HTTP transport** for cloud deployments and remote access
- **Monitor MCP logs** for errors and tool execution patterns
- **Place behind authenticated proxy** if exposing externally

### ❌ Don't

- **Don't expose MCP port publicly** without authentication (no built-in auth in v1.0)
- **Don't run without resource limits** (queries can consume memory)
- **Don't skip health probes** (prevents automatic recovery)
- **Don't use stdio transport** in Kubernetes (HTTP is better for deployments)
- **Don't set spectreURL to remote host** in sidecar mode (should be localhost)
- **Don't expect real-time log access** via MCP (only Kubernetes events, not pod logs)

## Related Documentation

- [Getting Started with MCP](../mcp-integration/getting-started) - Setup and first investigation
- [Claude Integration](../mcp-integration/claude-integration) - Claude Desktop configuration
- [Tools Reference](../mcp-integration/tools-reference/cluster-health) - Available MCP tools
- [Helm Values Reference](../reference/helm-values) - Complete Helm chart values

<!-- Source: chart/values.yaml lines 44-84, cmd/spectre/commands/mcp.go, internal/mcp/transport/http/transport.go -->
