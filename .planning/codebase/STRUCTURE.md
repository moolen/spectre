# Codebase Structure

**Analysis Date:** 2026-01-20

## Directory Layout

```
spectre-via-ssh/
├── cmd/                    # CLI entry points
│   └── spectre/            # Main binary commands
├── internal/               # Private Go packages
│   ├── agent/              # Multi-agent incident investigation
│   ├── analysis/           # Anomaly detection, causal analysis
│   ├── api/                # gRPC/Connect API handlers
│   ├── graph/              # FalkorDB client and graph operations
│   ├── importexport/       # Event import/export utilities
│   ├── mcp/                # Model Context Protocol server
│   └── watcher/            # Kubernetes resource watcher
├── ui/                     # React frontend
│   ├── src/                # TypeScript source
│   └── public/             # Static assets
├── tests/                  # End-to-end tests
├── chart/                  # Helm chart for deployment
├── docs/                   # Docusaurus documentation site
├── hack/                   # Development scripts and demo configs
├── .planning/              # GSD planning documents
│   └── codebase/           # Codebase analysis (this file)
├── go.mod                  # Go module definition
├── Makefile                # Build automation
└── README.md               # Project overview
```

## Directory Purposes

**cmd/spectre/:**
- Purpose: CLI command definitions
- Contains: Cobra command tree, flag definitions, entry point
- Key files: `main.go`, `commands/server.go`, `commands/mcp.go`, `commands/agent.go`

**internal/agent/:**
- Purpose: Multi-agent AI system for incident investigation
- Contains: Google ADK runner, TUI components, tool registry, provider abstraction, multiagent coordinator
- Key files: `runner/runner.go`, `tui/tui.go`, `tools/registry.go`, `multiagent/coordinator/coordinator.go`

**internal/analysis/:**
- Purpose: Graph analysis algorithms
- Contains: Anomaly detectors (crash loops, OOM, image pull failures), causal path finder, namespace graph builder
- Key files: `anomaly/detector.go`, `causal_paths/analyzer.go`, `namespace_graph/builder.go`

**internal/api/:**
- Purpose: gRPC/Connect API handlers
- Contains: Timeline streaming, metadata queries, anomaly detection, causal graph endpoints
- Key files: `handlers/timeline_handler.go`, `handlers/anomaly_handler.go`, `proto/timeline.proto`

**internal/graph/:**
- Purpose: FalkorDB graph database operations
- Contains: Client interface, query builder, schema manager, sync pipeline, reconciler, extractors
- Key files: `client.go`, `sync/pipeline.go`, `sync/extractors/registry.go`, `reconciler/reconciler.go`

**internal/graph/sync/extractors/:**
- Purpose: Relationship extraction plugins for different resource types
- Contains: Native K8s extractors (Pod→Node, Deployment→ReplicaSet), CRD extractors (ArgoCD, Flux, Cert-Manager, Gateway API)
- Key files: `registry.go`, `native/*.go`, `argocd/*.go`, `flux_helmrelease.go`, `gateway/*.go`

**internal/importexport/:**
- Purpose: Bulk event import/export
- Contains: Binary format reader/writer, enrichment pipeline
- Key files: `fileio/reader.go`, `enrichment/enrichment.go`

**internal/mcp/:**
- Purpose: Model Context Protocol server for AI assistants
- Contains: MCP server setup, tool implementations, client wrapper
- Key files: `server.go`, `tools/cluster_health.go`, `client/client.go`

**internal/models/:**
- Purpose: Core data models
- Contains: Protobuf definitions for events
- Key files: `event.proto`, `pb/event.pb.go`

**internal/watcher/:**
- Purpose: Kubernetes resource watching
- Contains: Dynamic client watcher, event handler interface, hot-reload config
- Key files: `watcher.go`, `event_handler.go`

**ui/src/:**
- Purpose: React frontend source code
- Contains: Pages, components, services, type definitions
- Key files: `App.tsx`, `pages/TimelinePage.tsx`, `pages/NamespaceGraphPage.tsx`, `services/timeline-grpc.ts`

**ui/src/components/:**
- Purpose: Reusable React components
- Contains: Namespace graph renderer, common UI elements
- Key files: `NamespaceGraph/*.tsx`, `Common/*.tsx`

**ui/src/services/:**
- Purpose: Frontend API clients
- Contains: gRPC-Web transport, timeline streaming, data transformers
- Key files: `timeline-grpc.ts`, `grpc-transport.ts`, `apiTypes.ts`

**ui/src/generated/:**
- Purpose: Auto-generated TypeScript from protobuf
- Contains: gRPC client stubs
- Key files: `timeline.ts`

**tests/:**
- Purpose: Integration and E2E tests
- Contains: Go test files using testcontainers
- Key files: `e2e_test.go`, `graph_test.go`

**chart/:**
- Purpose: Kubernetes deployment manifests
- Contains: Helm chart templates, values files
- Key files: `Chart.yaml`, `values.yaml`, `templates/deployment.yaml`

**docs/:**
- Purpose: User-facing documentation
- Contains: Docusaurus site with architecture, API reference, user guide
- Key files: `docs/architecture/*.md`, `docs/api/*.md`

**hack/:**
- Purpose: Development tools and demo resources
- Contains: Demo Kubernetes manifests, scripts
- Key files: `demo/workloads/*.yaml`, `demo/flux/*.yaml`

## Key File Locations

**Entry Points:**
- `cmd/spectre/main.go`: CLI entry point
- `cmd/spectre/commands/server.go`: Server command
- `cmd/spectre/commands/mcp.go`: MCP server command
- `cmd/spectre/commands/agent.go`: Agent command
- `ui/src/index.tsx`: React app entry

**Configuration:**
- `watcher.yaml`: Watcher resource configuration (not in repo, runtime)
- `ui/vite.config.ts`: Vite build config
- `.golangci.yaml`: Go linter config
- `tsconfig.json`: TypeScript config (in ui/)

**Core Logic:**
- `internal/watcher/watcher.go`: K8s event capture
- `internal/graph/sync/pipeline.go`: Event processing
- `internal/graph/client.go`: FalkorDB interface
- `internal/analysis/anomaly/detector.go`: Anomaly detection
- `internal/api/handlers/timeline_handler.go`: Timeline API
- `ui/src/services/timeline-grpc.ts`: Frontend data fetching

**Testing:**
- `internal/*/\*_test.go`: Unit tests
- `tests/e2e_test.go`: End-to-end tests
- `ui/src/test/`: Frontend tests
- `ui/playwright/`: Playwright component tests

## Naming Conventions

**Files:**
- Go: `snake_case.go` for implementation, `*_test.go` for tests
- TypeScript: `PascalCase.tsx` for React components, `camelCase.ts` for utilities
- Protobuf: `snake_case.proto`

**Directories:**
- Go: `lowercase` package names (no underscores)
- TypeScript: `camelCase` for directories

## Where to Add New Code

**New Kubernetes Resource Type Support:**
- Primary code: `internal/graph/sync/extractors/native/` or `internal/graph/sync/extractors/<crd-type>/`
- Register in: `internal/graph/sync/extractors/registry.go`
- Tests: Same directory as extractor with `*_test.go` suffix

**New API Endpoint:**
- Protocol definition: `internal/api/proto/*.proto`
- Handler: `internal/api/handlers/*_handler.go`
- Register in: `internal/api/handlers/register.go`
- Tests: `internal/api/handlers/*_test.go`

**New MCP Tool:**
- Implementation: `internal/mcp/tools/<tool_name>.go`
- Register in: `internal/mcp/server.go` (AddTool calls)
- Tests: `internal/mcp/tools/*_test.go`

**New Analysis Algorithm:**
- Implementation: `internal/analysis/<algorithm_name>/`
- Called from: API handlers or MCP tools
- Tests: `internal/analysis/<algorithm_name>/*_test.go`

**New UI Page:**
- Implementation: `ui/src/pages/<PageName>.tsx`
- Route in: `ui/src/App.tsx`
- Services: `ui/src/services/<feature>.ts`
- Components: `ui/src/components/<Feature>/`

**Utilities:**
- Shared Go helpers: `internal/graph/`, `internal/api/`, `internal/watcher/` (package-scoped)
- Frontend utilities: `ui/src/utils/`
- Constants: `ui/src/constants.ts` (frontend), `internal/*/constants.go` (backend)

## Special Directories

**.planning/:**
- Purpose: GSD codebase mapping documents
- Generated: By `/gsd:map-codebase` command
- Committed: Yes

**.planning/codebase/:**
- Purpose: Current codebase state analysis
- Contains: ARCHITECTURE.md, STRUCTURE.md, STACK.md, etc.
- Used by: `/gsd:plan-phase` and `/gsd:execute-phase`

**ui/dist/:**
- Purpose: Compiled frontend assets
- Generated: By `vite build`
- Committed: No

**ui/node_modules/:**
- Purpose: Node.js dependencies
- Generated: By `npm install`
- Committed: No

**internal/api/pb/:**
- Purpose: Generated Go code from protobuf
- Generated: By `protoc`
- Committed: Yes (for ease of use)

**internal/models/pb/:**
- Purpose: Generated Go code from protobuf models
- Generated: By `protoc`
- Committed: Yes

**ui/src/generated/:**
- Purpose: Generated TypeScript from protobuf
- Generated: By `ts-proto`
- Committed: Yes

**bin/:**
- Purpose: Compiled binaries
- Generated: By `make build`
- Committed: No

---

*Structure analysis: 2026-01-20*
