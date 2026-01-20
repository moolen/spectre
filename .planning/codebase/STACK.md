# Technology Stack

**Analysis Date:** 2026-01-20

## Languages

**Primary:**
- Go 1.24.4 - Backend services, API server, Kubernetes watchers, graph operations
- TypeScript ~5.8.2 - Frontend UI (React application)

**Secondary:**
- Protocol Buffers (proto3) - API definitions and gRPC service contracts

## Runtime

**Environment:**
- Go 1.25+ (production uses golang:1.25-alpine in `Dockerfile`)
- Node.js v20 (v20.20.0 detected locally, Node 25-alpine in `Dockerfile` for UI build)

**Package Manager:**
- Go: go mod (lockfile: `go.sum` present)
- Node.js: npm (lockfile: `ui/package-lock.json` present)

## Frameworks

**Core:**
- React 19.2.0 - Frontend UI framework
- Vite 6.2.0 - Frontend build tool and dev server
- Cobra v1.10.2 - CLI framework for Go commands
- Connect (connectrpc.com/connect v1.19.1) - gRPC/REST API framework

**Testing:**
- Vitest 4.0.16 - Unit testing framework for TypeScript/React
- Playwright 1.57.0 - E2E and component testing for UI
- @playwright/experimental-ct-react 1.57.0 - React component testing
- @testing-library/react 16.0.0 - React testing utilities
- testcontainers-go v0.31.0 - Integration testing with containers
- playwright-community/playwright-go v0.5200.1 - E2E testing from Go
- stretchr/testify v1.11.1 - Go assertion library

**Build/Dev:**
- Vite 6.2.0 - Frontend bundler, dev server, hot reload
- ts-proto 2.8.3 - TypeScript code generation from protobuf
- protoc-gen-grpc-web 1.5.0 - gRPC-Web code generation
- Docker multi-stage builds - Production image creation
- Make - Build orchestration (see `Makefile`)

## Key Dependencies

**Critical:**
- FalkorDB/falkordb-go/v2 v2.0.2 - Graph database client for relationship storage
- anthropics/anthropic-sdk-go v1.19.0 - AI agent integration (Claude)
- google.golang.org/genai v1.40.0 - Google Generative AI SDK
- @google/genai 1.30.0 - Google Generative AI SDK for UI
- mark3labs/mcp-go v0.43.2 - Model Context Protocol server implementation
- k8s.io/client-go v0.34.0 - Kubernetes API client
- k8s.io/api v0.34.0 - Kubernetes API types
- k8s.io/apimachinery v0.34.0 - Kubernetes API machinery
- helm.sh/helm/v3 v3.19.2 - Helm chart operations

**Infrastructure:**
- grpc-web 2.0.2 - gRPC-Web client for frontend
- react-router-dom 6.28.0 - Client-side routing
- d3 7.9.0 - Data visualization for graphs
- dagre 0.8.5 - Graph layout algorithms
- rxjs 7.8.2 - Reactive programming for streams
- sonner 2.0.7 - Toast notifications
- redis/go-redis/v9 v9.17.2 - Redis client (used by FalkorDB)
- google.golang.org/grpc v1.76.0 - gRPC framework
- google.golang.org/protobuf v1.36.10 - Protocol buffers runtime
- charmbracelet/bubbletea v1.3.10 - Terminal UI framework for agent
- charmbracelet/lipgloss v1.1.1 - Terminal UI styling
- charmbracelet/glamour v0.10.0 - Markdown rendering in terminal

**Observability:**
- go.opentelemetry.io/otel v1.38.0 - OpenTelemetry tracing
- go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.34.0 - OTLP gRPC exporter
- go.opentelemetry.io/otel/sdk v1.38.0 - OpenTelemetry SDK

## Configuration

**Environment:**
- Go services: CLI flags with environment variable fallbacks (see `cmd/spectre/commands/server.go` and `cmd/spectre/commands/mcp.go`)
- UI: Vite environment variables with `VITE_` prefix (see `ui/.env`)
- Key variables: `ANTHROPIC_API_KEY`, `ANTHROPIC_FOUNDRY_API_KEY`, `SPECTRE_URL`, `MCP_HTTP_ADDR`, `GRAPH_ENABLED`, `GRAPH_HOST`, `GRAPH_PORT`, `GRAPH_NAME`

**Build:**
- `go.mod` - Go module dependencies
- `ui/package.json` - Node.js dependencies and scripts
- `ui/vite.config.ts` - Vite bundler configuration
- `ui/tsconfig.json` - TypeScript compiler options
- `ui/vitest.config.ts` - Vitest test runner configuration
- `ui/playwright-ct.config.ts` - Playwright component test configuration
- `Dockerfile` - Multi-stage Docker build (Node 25-alpine for UI, Go 1.25-alpine for backend, Alpine 3.18 for runtime)
- `docker-compose.graph.yml` - Local development stack with FalkorDB
- `Makefile` - Build automation (build, test, deploy targets)
- `.golangci.yaml` - Go linter configuration
- `ui/.eslintrc.json` - ESLint configuration for TypeScript/React

## Platform Requirements

**Development:**
- Go 1.24.4+
- Node.js v20+
- Docker and Docker Compose (for FalkorDB local development)
- kubectl (for Kubernetes integration)
- Make (for build automation)
- Optional: kind v0.30.0 (for local Kubernetes testing via sigs.k8s.io/kind)
- Optional: Helm 3.19.2+ (for chart development)

**Production:**
- Kubernetes cluster (tested with k8s.io v0.34.0)
- FalkorDB v4.14.10-alpine (deployed as sidecar or standalone)
- Optional: OpenTelemetry collector (if tracing enabled)
- Container runtime (uses Alpine 3.18 base image)

---

*Stack analysis: 2026-01-20*
