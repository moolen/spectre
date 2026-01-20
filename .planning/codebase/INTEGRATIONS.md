# External Integrations

**Analysis Date:** 2026-01-20

## APIs & External Services

**AI Providers:**
- Anthropic Claude - AI agent for incident response
  - SDK/Client: anthropic-sdk-go v1.19.0
  - Auth: `ANTHROPIC_API_KEY` environment variable
  - Used in: `internal/agent/provider/anthropic.go`, `cmd/spectre/commands/agent.go`
  - Models: claude-sonnet-4-5-20250929 (default), configurable via `--model` flag
  - Alternative: Azure AI Foundry endpoint via `ANTHROPIC_FOUNDRY_API_KEY`

- Google Generative AI - AI capabilities
  - SDK/Client: google.golang.org/genai v1.40.0 (Go), @google/genai 1.30.0 (TypeScript)
  - Used in: `ui/src/services/geminiService.ts`
  - Auth: Configured via Google ADK (google.golang.org/adk v0.3.0)

**Model Context Protocol (MCP):**
- MCP Server - Exposes Spectre tools to AI assistants
  - SDK/Client: mark3labs/mcp-go v0.43.2
  - Endpoint: Configurable via `MCP_ENDPOINT` env var (default: `/mcp`)
  - HTTP Address: Configurable via `MCP_HTTP_ADDR` env var (default: `:8082`)
  - Transport modes: HTTP server or stdio
  - Tools: cluster_health, resource_timeline, resource_timeline_changes, detect_anomalies, causal_paths
  - Prompts: post_mortem_incident_analysis, live_incident_handling
  - Implementation: `internal/mcp/`, `cmd/spectre/commands/mcp.go`

## Data Storage

**Databases:**
- FalkorDB (graph database)
  - Connection: `GRAPH_HOST` (default: localhost), `GRAPH_PORT` (default: 6379), `GRAPH_NAME` (default: spectre)
  - Client: FalkorDB/falkordb-go/v2 v2.0.2
  - Protocol: Redis wire protocol (uses redis/go-redis/v9 v9.17.2 under the hood)
  - Storage: Graph nodes (resources, events, secrets) and edges (ownership, references, scheduling, traffic, management)
  - Implementation: `internal/graph/client.go`, `internal/graph/cached_client.go`
  - Docker image: falkordb/falkordb:v4.14.10-alpine
  - Deployment: Sidecar container in Helm chart or standalone via `docker-compose.graph.yml`
  - Retention: Configurable via `--graph-retention-hours` (default: 168 hours = 7 days)

**File Storage:**
- Local filesystem only
  - Event storage: Binary format in `/data` directory
  - Audit logs: JSONL format (if `--audit-log` flag provided)
  - Import/export: Binary event files via `--import-path` flag
  - Implementation: `internal/importexport/`

**Caching:**
- In-memory LRU cache for graph queries
  - Library: hashicorp/golang-lru/v2 v2.0.7
  - Implementation: `internal/graph/cached_client.go`
  - Configurable namespace graph cache via flags: `--namespace-graph-cache-enabled`, `--namespace-graph-cache-refresh-seconds`, `--namespace-graph-cache-memory-mb`

## Authentication & Identity

**Auth Provider:**
- Kubernetes RBAC
  - Implementation: Uses Kubernetes client-go ServiceAccount token authentication
  - In-cluster: Automatic ServiceAccount credential mounting
  - Out-of-cluster: Uses kubeconfig from standard locations
  - RBAC permissions: ClusterRole with get, list, watch on monitored resources
  - Implementation: `internal/watcher/watcher.go`

**API Authentication:**
- None (currently unauthenticated)
  - API server on port 8080 has no authentication layer
  - MCP server on port 8082 has no authentication layer
  - Relies on network-level security (ClusterIP service in Kubernetes)

## Monitoring & Observability

**Error Tracking:**
- None (no external error tracking service)

**Logs:**
- Structured logging to stdout
  - Library: Custom logger in `internal/logging/logger.go`
  - Configurable per-package log levels via `LOG_LEVEL_<PACKAGE>` environment variables
  - Example: `LOG_LEVEL_GRAPH_SYNC=debug`
  - Format: Structured text format with timestamps and log levels

**Tracing:**
- OpenTelemetry OTLP
  - Enabled via `--tracing-enabled` flag
  - Endpoint: Configurable via `--tracing-endpoint` (e.g., victorialogs:4317)
  - Protocol: OTLP gRPC (go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.34.0)
  - TLS: Optional CA certificate via `--tracing-tls-ca`, insecure mode via `--tracing-tls-insecure`
  - Implementation: `internal/tracing/`, instrumented in API handlers and graph operations
  - Traces: HTTP requests, gRPC calls, graph queries, causal path discovery

**Profiling:**
- pprof profiling server
  - Enabled via `--pprof-enabled` flag
  - Port: Configurable via `--pprof-port` (default: 9999)
  - Endpoints: Standard Go pprof endpoints (/debug/pprof/*)
  - Implementation: net/http/pprof imported in `cmd/spectre/commands/server.go`

## CI/CD & Deployment

**Hosting:**
- Kubernetes (primary deployment target)
  - Helm chart: `chart/` directory
  - Namespace: monitoring (default)
  - Container registry: ghcr.io/moolen/spectre
  - Chart registry: oci://ghcr.io/moolen/charts/spectre

**CI Pipeline:**
- GitHub Actions
  - Workflows: `.github/workflows/pr-checks.yml`, `.github/workflows/helm-tests.yml`, `.github/workflows/release.yml`, `.github/workflows/docs.yml`
  - Tests: Go tests, UI component tests (Playwright), Helm chart tests
  - Go version: 1.24.1 (in CI)
  - Node version: 20 (in CI)
  - Linting: golangci-lint, ESLint

**Container Build:**
- Multi-stage Dockerfile
  - Stage 1: Node.js 25-alpine for UI build
  - Stage 2: Go 1.25-alpine for backend build
  - Final: Alpine 3.18 with compiled binaries
  - Health check: wget to /health endpoint every 30s
  - Entry point: `/app/spectre server`

## Environment Configuration

**Required env vars:**
- None (all have defaults)

**Optional env vars:**
- `ANTHROPIC_API_KEY` - Anthropic API key for AI agent
- `ANTHROPIC_FOUNDRY_API_KEY` - Azure AI Foundry API key (alternative to Anthropic)
- `SPECTRE_URL` - Spectre API server URL (for MCP server, default: http://localhost:8080)
- `MCP_HTTP_ADDR` - MCP HTTP server address (default: :8082)
- `MCP_ENDPOINT` - MCP endpoint path (default: /mcp)
- `GRAPH_ENABLED` - Enable graph features (set via flag or env)
- `GRAPH_HOST` - FalkorDB host (set via flag or env)
- `GRAPH_PORT` - FalkorDB port (set via flag or env)
- `GRAPH_NAME` - FalkorDB graph name (set via flag or env)
- `LOG_LEVEL_*` - Per-package log level configuration
- `VITE_API_BASE` - Frontend API base path (default: /v1)
- `VITE_BASE_PATH` - Frontend base path for routing

**Secrets location:**
- Kubernetes Secrets (in production via Helm chart)
- Local .env files for development (`ui/.env`, `ui/.env.local`)
- Environment variables for API keys

## Webhooks & Callbacks

**Incoming:**
- None (no webhook endpoints exposed)

**Outgoing:**
- None (no webhooks sent to external services)

## Kubernetes Integration

**Watched Resources:**
- Configurable via `watcher.yaml` file
- Default resources: Pods, Deployments, ReplicaSets, Services, ConfigMaps, Secrets, etc.
- Custom resources: Supports any CRD (Gateway API, ArgoCD, Cert-Manager, External Secrets, etc.)
- Watch API: Kubernetes Watch API via k8s.io/client-go v0.34.0
- Event handling: `internal/watcher/event_handler.go`, `internal/watcher/watcher.go`

**Resource Discovery:**
- Dynamic client for CRDs
- Namespace filtering supported
- Label selectors supported

## gRPC/Connect APIs

**Protocol Support:**
- gRPC-Web - Frontend to backend communication
  - Library: grpc-web 2.0.2 (UI), connectrpc.com/connect v1.19.1 (backend)
  - Transport: HTTP/1.1 compatible (works behind load balancers)
  - Implementation: `ui/src/services/grpc-transport.ts`, `ui/src/services/timeline-grpc.ts`

- Connect Protocol - Dual REST/gRPC API
  - Server: `internal/api/timeline_connect_service.go`
  - Supports: Connect, gRPC, and gRPC-Web protocols
  - Content types: Protobuf binary and JSON

- gRPC (native) - Alternative transport
  - Server: `internal/api/timeline_grpc_service.go`
  - Protocol: HTTP/2 gRPC

**Protobuf Definitions:**
- `internal/api/proto/timeline.proto` - Timeline API service
- `internal/models/event.proto` - Event data models
- Generated code: `internal/api/proto/pbconnect/`, `ui/src/generated/timeline.ts`

---

*Integration audit: 2026-01-20*
