# Architecture

**Analysis Date:** 2026-01-20

## Pattern Overview

**Overall:** Event-driven microservices with graph-based reasoning

**Key Characteristics:**
- Kubernetes watcher captures resource changes as events
- Events flow through processing pipeline into FalkorDB graph database
- Graph stores resources as nodes with relationship edges (ownership, references, causality)
- Multiple query layers: REST API, gRPC streaming, MCP server for AI assistants
- React SPA frontend consumes gRPC-Web streams for timeline and graph visualization
- AI agent system with Google ADK for incident investigation

## Layers

**Event Capture Layer:**
- Purpose: Watch Kubernetes API for resource changes
- Location: `internal/watcher`
- Contains: Dynamic client watchers, event handlers, hot-reload config
- Depends on: Kubernetes client-go, config
- Used by: Server command to populate event pipeline

**Event Processing Pipeline:**
- Purpose: Transform Kubernetes events into graph updates
- Location: `internal/graph/sync`
- Contains: Pipeline orchestrator, graph builder, causality engine, retention manager
- Depends on: Graph client, extractors, models
- Used by: Watcher event handler to persist state changes

**Graph Storage Layer:**
- Purpose: Persist and query resource relationships in FalkorDB
- Location: `internal/graph`
- Contains: Client interface, query executor, schema manager, cached client wrapper
- Depends on: FalkorDB Go client
- Used by: Pipeline, analysis modules, API handlers

**Relationship Extraction:**
- Purpose: Extract edges between resources from Kubernetes manifests
- Location: `internal/graph/sync/extractors`
- Contains: Extractor registry, native K8s extractors, CRD extractors (ArgoCD, Flux, Cert-Manager, Gateway API)
- Depends on: Unstructured objects from client-go
- Used by: Graph builder during event processing

**Analysis Layer:**
- Purpose: Detect anomalies and find causal paths through graph
- Location: `internal/analysis`
- Contains: Anomaly detector, causal path analyzer, namespace graph builder
- Depends on: Graph client, analyzer utilities
- Used by: API handlers, MCP tools

**API Layer:**
- Purpose: Expose query endpoints for frontends and tools
- Location: `internal/api`
- Contains: gRPC/Connect handlers for timeline, metadata, anomalies, causal paths
- Depends on: Storage (future), graph client, analysis modules
- Used by: Web UI, MCP server

**MCP Integration:**
- Purpose: Expose cluster state to AI assistants via Model Context Protocol
- Location: `internal/mcp`
- Contains: MCP server, tools (cluster_health, resource_timeline, detect_anomalies, causal_paths), prompts
- Depends on: API client, analyzer
- Used by: AI assistants (Claude Desktop, etc.)

**Agent System:**
- Purpose: Multi-agent incident investigation using LLMs
- Location: `internal/agent`
- Contains: Google ADK runner, TUI, tools registry, provider abstraction, multiagent coordinator
- Depends on: MCP client, Google GenAI SDK, Anthropic SDK
- Used by: Agent command for CLI-based investigations

**Web UI:**
- Purpose: Visualize timeline and graph for human operators
- Location: `ui/src`
- Contains: React pages, D3 graph rendering, gRPC-Web client, timeline components
- Depends on: gRPC-Web generated clients, React Router
- Used by: Browser users

## Data Flow

**Kubernetes Event → Graph Storage:**

1. Watcher receives K8s watch event (Add/Update/Delete)
2. Event wrapped in models.Event with timestamp, UID, JSON data
3. Pipeline.ProcessEvent builds GraphUpdate via extractors
4. Graph client executes Cypher CREATE/MERGE for nodes and edges
5. Causality engine adds temporal edges based on timestamp proximity

**User Query → Timeline Response:**

1. Frontend sends gRPC TimelineRequest with filters (kind, namespace, time range)
2. API handler queries graph for matching resources
3. Results streamed as TimelineChunks (metadata, then resource batches)
4. Frontend renders timeline segments with status colors
5. User clicks resource → fetches diff via resource_timeline_changes

**AI Investigation → Root Cause:**

1. Agent calls cluster_health MCP tool → finds unhealthy resources
2. For each issue, calls detect_anomalies → gets anomaly types (crash loop, OOM, etc.)
3. Calls causal_paths → traverses graph backwards through ownership/reference edges
4. Returns ranked paths with confidence scores based on temporal proximity
5. Agent presents findings to user in structured format

**State Management:**
- Server maintains no client state (stateless REST/gRPC)
- Graph database is single source of truth
- UI manages local state with React hooks
- Agent maintains conversation history in ADK session storage

## Key Abstractions

**models.Event:**
- Purpose: Represents a single Kubernetes resource change
- Examples: `internal/models/event.proto`
- Pattern: Protobuf message with timestamp, type (CREATE/UPDATE/DELETE), resource metadata, compressed data

**graph.Node:**
- Purpose: Represents a resource or event in graph
- Examples: `internal/graph/models.go`
- Pattern: NodeType enum (Resource, Event, ChangeEvent) with properties map

**graph.Edge:**
- Purpose: Represents relationships between nodes
- Examples: `internal/graph/models.go`
- Pattern: EdgeType enum (Owns, References, Schedules, Manages, Causes, Precedes) with optional properties

**sync.Pipeline:**
- Purpose: Orchestrates event processing into graph
- Examples: `internal/graph/sync/pipeline.go`
- Pattern: Interface with Start/Stop lifecycle, ProcessEvent/ProcessBatch methods

**extractors.RelationshipExtractor:**
- Purpose: Plugin for extracting edges from specific resource types
- Examples: `internal/graph/sync/extractors/native/*.go`
- Pattern: Interface with CanExtract, Extract methods; registry pattern for lookup

**analysis.Anomaly:**
- Purpose: Detected issue in resource state/events
- Examples: `internal/analysis/anomaly/types.go`
- Pattern: Struct with Type, Severity, Description, AffectedResources, Timestamp

**mcp.Tool:**
- Purpose: MCP tool exposed to AI assistants
- Examples: `internal/mcp/tools/*.go`
- Pattern: Interface with Name, Description, Schema, Call methods

## Entry Points

**cmd/spectre/main.go:**
- Location: `cmd/spectre/main.go`
- Triggers: CLI invocation
- Responsibilities: Delegates to cobra command tree

**cmd/spectre/commands/server.go:**
- Location: `cmd/spectre/commands/server.go`
- Triggers: `spectre server` command
- Responsibilities: Creates lifecycle manager, starts watcher, graph pipeline, API server, reconciler

**cmd/spectre/commands/mcp.go:**
- Location: `cmd/spectre/commands/mcp.go`
- Triggers: `spectre mcp` command
- Responsibilities: Starts MCP server in HTTP or stdio mode, connects to Spectre API

**cmd/spectre/commands/agent.go:**
- Location: `cmd/spectre/commands/agent.go`
- Triggers: `spectre agent` command
- Responsibilities: Initializes ADK runner with tools, starts TUI, handles user prompts

**ui/src/index.tsx:**
- Location: `ui/src/index.tsx`
- Triggers: Browser loads HTML
- Responsibilities: Mounts React app with router

**ui/src/App.tsx:**
- Location: `ui/src/App.tsx`
- Triggers: React render
- Responsibilities: Sets up routes, sidebar, toast notifications

## Error Handling

**Strategy:** Layered error handling with logging at each boundary

**Patterns:**
- Graph pipeline logs errors but continues processing (no event drops entire pipeline)
- API handlers return structured errors via Connect protocol (gRPC status codes)
- Watcher retries failed API calls with exponential backoff
- Frontend displays errors in toast notifications (Sonner)
- Agent system surfaces tool errors to LLM for recovery

## Cross-Cutting Concerns

**Logging:** Structured logger in `internal/logging` with component-prefixed messages, configurable levels per package

**Validation:** Input validation in `internal/api/validation` for timeline queries; graph schema validation in `internal/graph/validation`

**Authentication:** Not implemented (assumes trusted network or external auth proxy)

---

*Architecture analysis: 2026-01-20*
