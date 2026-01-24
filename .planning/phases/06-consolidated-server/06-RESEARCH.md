# Phase 6: Consolidated Server & Integration Manager - Research

**Researched:** 2026-01-21
**Domain:** Go HTTP server consolidation, MCP protocol over HTTP, graceful shutdown orchestration
**Confidence:** HIGH

## Summary

This phase consolidates the separate MCP sidecar into the main Spectre server, serving REST API, UI, and MCP on a single port (8080) with in-process integration manager. The research reveals that:

1. **Current Architecture:** Spectre has a mature lifecycle manager that orchestrates component startup/shutdown in dependency order. The MCP server currently runs as a standalone command using `mcp-go` library's StreamableHTTPServer with SSE transport. The integration manager already exists and can be easily integrated.

2. **MCP HTTP Transport:** The `mcp-go` v0.43.2 library provides `StreamableHTTPServer` with stateless mode support. Context decision: SSE transport was chosen, but `mcp-go` documentation reveals SSE is deprecated as of MCP spec 2025-03-26 in favor of StreamableHTTP. **Recommendation: Use StreamableHTTP transport instead of SSE** - it's the current standard and already implemented in existing `mcp.go` command.

3. **Integration Strategy:** Minimal code changes required. The existing integration manager (internal/integration/manager.go) can be passed to the MCP server via `MCPToolRegistry` adapter. Config hot-reload with 500ms debounce already implemented.

4. **Shutdown Orchestration:** Go 1.16+ provides `signal.NotifyContext` for clean signal handling. Lifecycle manager handles component shutdown in reverse dependency order with per-component timeout (currently 30s, will override to 10s per requirements).

**Primary recommendation:** Use StreamableHTTP transport (already in use) instead of SSE. Add MCP server as a lifecycle component alongside REST server on the same http.ServeMux. Integration manager already supports MCP tool registration.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| mark3labs/mcp-go | v0.43.2 (current) | MCP protocol implementation | Already in use, supports StreamableHTTP transport |
| net/http | stdlib | HTTP server | Go standard library, proven at scale |
| fsnotify/fsnotify | v1.9.0 (current) | File watching for config reload | Already used for integration config hot-reload |
| os/signal | stdlib | Signal handling for graceful shutdown | Go 1.16+ standard pattern |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| context | stdlib | Cancellation propagation | Shutdown coordination across components |
| sync | stdlib | Concurrency primitives | Lifecycle manager state protection |
| time | stdlib | Timeout management | Graceful shutdown deadlines |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| StreamableHTTP | SSE (Server-Sent Events) | SSE deprecated in MCP spec 2025-03-26, StreamableHTTP is current standard |
| Single http.Server | Separate servers for REST/MCP | Single server simplifies deployment, uses same port, easier CORS handling |
| cenkalti/backoff | Manual exponential backoff | Library provides jitter, but simple implementation may suffice for integration retry |

**Installation:**
```bash
# Already in go.mod:
github.com/mark3labs/mcp-go v0.43.2
github.com/fsnotify/fsnotify v1.9.0
```

## Architecture Patterns

### Recommended Project Structure
Current structure is already well-organized:
```
cmd/spectre/commands/
├── server.go              # Main server startup (will add MCP)
└── mcp.go                 # Standalone MCP (Phase 8 removal)

internal/
├── apiserver/             # REST API server (lifecycle component)
├── mcp/                   # MCP server logic
│   ├── server.go          # SpectreServer wrapper
│   └── tools/             # MCP tool implementations
├── integration/           # Integration manager
│   ├── manager.go         # Lifecycle component
│   └── types.go           # ToolRegistry interface
├── lifecycle/             # Component orchestration
│   ├── manager.go         # Dependency-aware startup/shutdown
│   └── component.go       # Component interface
└── config/                # Configuration
    └── integration_watcher.go  # 500ms debounced reload
```

### Pattern 1: Lifecycle Component Integration
**What:** Components implement `Start(ctx)`, `Stop(ctx)`, `Name()` interface and register with lifecycle manager with explicit dependencies.

**When to use:** Any long-running service that needs coordinated startup/shutdown.

**Example from existing code:**
```go
// Source: internal/lifecycle/component.go
type Component interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Name() string
}

// Source: cmd/spectre/commands/server.go (lines 168-203)
manager := lifecycle.NewManager()

// Integration manager has no dependencies
manager.Register(integrationMgr)

// API server depends on graph service
manager.Register(apiComponent, graphServiceComponent)

// Start all in dependency order
ctx, cancel := context.WithCancel(context.Background())
manager.Start(ctx)

// Stop in reverse order on signal
<-sigChan
manager.Stop(shutdownCtx)
```

### Pattern 2: Shared http.ServeMux for Multiple Handlers
**What:** Single http.ServeMux routes different paths to different handlers. Go 1.22+ supports method-specific routing on same path.

**When to use:** Consolidating multiple services on one port.

**Example structure:**
```go
// Source: internal/apiserver/routes.go pattern + StreamableHTTP pattern
router := http.NewServeMux()

// REST API routes
router.Handle("/api/v1/timeline", timelineHandler)
router.HandleFunc("/health", healthHandler)

// MCP endpoint (StreamableHTTP)
mcpServer := server.NewStreamableHTTPServer(spectreServer.GetMCPServer(),
    server.WithEndpointPath("/v1/mcp"),
    server.WithStateLess(true),
)
router.Handle("/v1/mcp", mcpServer)

// Static UI (catch-all, must be last)
router.HandleFunc("/", serveStaticUI)

// Wrap with CORS middleware
handler := corsMiddleware(router)
httpServer := &http.Server{Addr: ":8080", Handler: handler}
```

### Pattern 3: MCP Tool Registry Adapter
**What:** Integration manager calls `RegisterTool()` on `MCPToolRegistry` which adapts to mcp-go's `AddTool()` method.

**When to use:** Integrations need to expose tools via MCP dynamically.

**Example from existing code:**
```go
// Source: internal/mcp/server.go (lines 369-429)
type MCPToolRegistry struct {
    mcpServer *server.MCPServer
}

func (r *MCPToolRegistry) RegisterTool(name string, handler integration.ToolHandler) error {
    // Adapter: integration.ToolHandler -> mcp.CallToolRequest
    adaptedHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        args, _ := json.Marshal(request.Params.Arguments)
        result, err := handler(ctx, args)
        if err != nil {
            return mcp.NewToolResultError(fmt.Sprintf("Tool execution failed: %v", err)), nil
        }
        resultJSON, _ := json.MarshalIndent(result, "", "  ")
        return mcp.NewToolResultText(string(resultJSON)), nil
    }

    mcpTool := mcp.NewToolWithRawSchema(name, "", schemaJSON)
    r.mcpServer.AddTool(mcpTool, adaptedHandler)
    return nil
}
```

### Pattern 4: Graceful Shutdown with Context Timeout
**What:** Use `signal.NotifyContext` to create cancellable context, then give each component its own timeout for graceful stop.

**When to use:** Multi-component server needs coordinated shutdown.

**Example from existing lifecycle manager:**
```go
// Source: internal/lifecycle/manager.go (lines 236-284)
// Setup signal handling
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

// Wait for signal
<-sigChan
logger.Info("Shutdown signal received")
cancel() // Cancel main context

// Stop each component with its own timeout
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

for _, component := range toStop {
    componentCtx, cancel := context.WithTimeout(shutdownCtx, componentTimeout)
    err := component.Stop(componentCtx)
    cancel()

    if errors.Is(err, context.DeadlineExceeded) {
        logger.Warn("Component %s exceeded grace period, forcing termination", component.Name())
    }
}
```

### Pattern 5: Stdio Transport Alongside HTTP
**What:** When `--stdio` flag is present, run stdio transport in goroutine alongside HTTP server. Both share same MCP server instance.

**When to use:** Need to support both HTTP and stdio MCP clients simultaneously.

**Example:**
```go
// HTTP server always runs
go func() {
    httpServer.ListenAndServe()
}()

// Stdio transport optionally runs alongside
if stdioEnabled {
    go func() {
        server.ServeStdio(mcpServer)
    }()
}

// Both transports stop on context cancellation
<-ctx.Done()
```

### Anti-Patterns to Avoid
- **Separate HTTP servers on different ports:** Complicates deployment, firewall rules, and client configuration. Use single server with path-based routing.
- **Blocking Start() methods:** Components should start async work in goroutines and return quickly. Lifecycle manager doesn't wait for "ready" state, just successful initialization.
- **Ignoring shutdown errors:** Log shutdown failures but don't fail the shutdown process - other components still need to stop.
- **Mutex locks during shutdown:** Can cause deadlocks if component is already stopping. Use channels or atomic flags for shutdown coordination.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| File watching with debounce | Custom fsnotify loop with timer | Existing IntegrationWatcher (internal/config/integration_watcher.go) | Already handles debouncing (500ms), reload errors, graceful stop. Tested in production. |
| Exponential backoff for retries | Manual time.Sleep loop | Simple doubling with max (or cenkalti/backoff if complex) | Integration retry needs jitter to avoid thundering herd. Keep it simple: start 1s, double each time, max 30s. |
| Signal handling boilerplate | Custom signal channel setup | signal.NotifyContext (Go 1.16+) | Creates cancellable context automatically, cleaner API |
| Component dependency ordering | Manual startup sequence | Existing lifecycle.Manager (internal/lifecycle/manager.go) | Topological sort for dependencies, rollback on failure, reverse-order shutdown. Don't recreate this. |
| CORS middleware | Custom header setting | Existing corsMiddleware (internal/apiserver/middleware.go) | Already handles preflight, all origins, proper headers for browser clients |
| MCP transport setup | Raw HTTP handler for MCP | mcp-go StreamableHTTPServer | Handles session management, request routing, error formatting per MCP spec |

**Key insight:** Most "plumbing" already exists. This phase is primarily about composition - connecting existing pieces (lifecycle manager, integration manager, MCP server, REST server) into a unified startup/shutdown flow.

## Common Pitfalls

### Pitfall 1: SSE vs StreamableHTTP Confusion
**What goes wrong:** Context document specifies SSE transport, but MCP spec deprecated SSE as of 2025-03-26. Existing `mcp.go` command uses StreamableHTTP successfully.

**Why it happens:** Context document was written before researching current MCP transport standards.

**How to avoid:** Use StreamableHTTP transport (already in use). It's the current standard and provides better compatibility with MCP clients.

**Warning signs:** If seeing "SSE Transport has been deprecated" in mcp-go documentation, you're on the wrong path.

### Pitfall 2: Integration Manager Initialization Order
**What goes wrong:** Integration manager starts before MCP server exists, tries to register tools, crashes with nil pointer.

**Why it happens:** Natural instinct is to start integrations early, but they need MCP server for tool registration.

**How to avoid:**
1. Create MCP server first (but don't start HTTP listener yet)
2. Pass MCPToolRegistry to integration manager
3. Start integration manager (calls RegisterTools on each integration)
4. Then start HTTP server listening

**Warning signs:** Panic on `MCPToolRegistry.RegisterTool()` with nil mcpServer.

### Pitfall 3: Shutdown Timeout Too Short
**What goes wrong:** Components don't finish cleanup within timeout, lifecycle manager force-terminates, resources leak (open files, connections).

**Why it happens:** Requirements specify 10s timeout, but some components (integrations, graph pipeline) may need longer.

**How to avoid:** Test shutdown behavior under load. If timeout exceeded consistently, either:
- Optimize component shutdown (close connections faster)
- Increase timeout for specific components (lifecycle manager supports per-component timeout)

**Warning signs:** Logs show "exceeded grace period, forcing termination" frequently.

### Pitfall 4: Stdio and HTTP Mutual Exclusivity
**What goes wrong:** Implementing `--stdio` as mutually exclusive with HTTP means no HTTP server runs in stdio mode.

**Why it happens:** Original MCP command has "http" or "stdio" transport choice.

**How to avoid:** Requirements clarify: `--stdio` flag ADDS stdio alongside HTTP. HTTP always runs. Stdio is optional addition.

**Warning signs:** Tests fail because no REST API available when using `--stdio`.

### Pitfall 5: CORS Not Applied to MCP Endpoint
**What goes wrong:** Browser-based MCP clients can't connect to `/v1/mcp` endpoint due to CORS errors.

**Why it happens:** MCP handler registered directly without going through CORS middleware.

**How to avoid:** CORS middleware wraps entire router (already done in `apiserver.configureHTTPServer`). Ensure MCP handler is registered on the router BEFORE wrapping with CORS.

**Warning signs:** Browser console shows "CORS policy: No 'Access-Control-Allow-Origin' header" for `/v1/mcp` requests.

### Pitfall 6: Route Registration Order
**What goes wrong:** Static UI catch-all (`router.HandleFunc("/", ...)`) intercepts MCP requests.

**Why it happens:** http.ServeMux matches routes in registration order when specificity is equal.

**How to avoid:** Register routes from most specific to least specific:
1. Exact paths (`/health`, `/v1/mcp`)
2. API paths with prefixes (`/api/v1/*`)
3. Static UI catch-all (`/`) MUST BE LAST

**Warning signs:** MCP endpoint returns UI HTML instead of handling MCP protocol.

### Pitfall 7: MCP Server Lifecycle Component Implementation
**What goes wrong:** Treating MCP server as separate lifecycle component creates shutdown ordering problems.

**Why it happens:** MCP server and REST server need to stop together, not in dependency order.

**How to avoid:** MCP endpoint is just a handler on the same http.Server as REST. Don't create separate MCP lifecycle component. The apiserver component shuts down the http.Server which stops both REST and MCP.

**Warning signs:** Need complex dependency declarations between "REST server" and "MCP server" components.

## Code Examples

Verified patterns from official sources:

### StreamableHTTP Server Setup
```go
// Source: existing cmd/spectre/commands/mcp.go (lines 159-183)
// with stateless mode for compatibility
endpointPath := "/v1/mcp"

streamableServer := server.NewStreamableHTTPServer(
    mcpServer,
    server.WithEndpointPath(endpointPath),
    server.WithStateLess(true), // Stateless mode per requirements
)

// Register on router
router.Handle(endpointPath, streamableServer)

// StreamableHTTPServer handles:
// - GET /v1/mcp (SSE stream)
// - POST /v1/mcp (messages)
// - Session management (or stateless if WithStateLess(true))
```

### Integration Manager with MCP Registry
```go
// Source: internal/integration/manager.go + internal/mcp/server.go patterns
// Create MCP server first
spectreServer, err := mcp.NewSpectreServerWithOptions(mcp.ServerOptions{
    SpectreURL: "http://localhost:8080", // Self-reference for in-process
    Version:    version,
})

// Create tool registry adapter
mcpRegistry := mcp.NewMCPToolRegistry(spectreServer.GetMCPServer())

// Create integration manager with registry
integrationMgr, err := integration.NewManagerWithMCPRegistry(
    integration.ManagerConfig{
        ConfigPath:            integrationsConfigPath,
        MinIntegrationVersion: minIntegrationVersion,
    },
    mcpRegistry,
)

// Register with lifecycle (no dependencies)
manager.Register(integrationMgr)

// When manager starts, it calls RegisterTools() on each integration
// which calls mcpRegistry.RegisterTool() which calls mcpServer.AddTool()
```

### Graceful Shutdown Flow
```go
// Source: cmd/spectre/commands/server.go (lines 526-549) + lifecycle manager
logger.Info("Starting Spectre v%s", Version)

// Create lifecycle manager
manager := lifecycle.NewManager()
manager.SetShutdownTimeout(10 * time.Second) // Per requirements

// Register components in dependency order
manager.Register(integrationMgr)              // No dependencies
manager.Register(graphServiceComponent)       // No dependencies
manager.Register(apiComponent, graphServiceComponent) // Depends on graph

// Start all
ctx, cancel := context.WithCancel(context.Background())
if err := manager.Start(ctx); err != nil {
    logger.Error("Failed to start: %v", err)
    os.Exit(1)
}

logger.Info("Application started successfully")

// Wait for shutdown signal
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
<-sigChan

logger.Info("Shutdown signal received, gracefully shutting down...")
cancel()

// Graceful shutdown with timeout
shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
defer shutdownCancel()

if err := manager.Stop(shutdownCtx); err != nil {
    logger.Error("Error during shutdown: %v", err)
    os.Exit(1)
}

logger.Info("Shutdown complete")
```

### Stdio Transport Alongside HTTP
```go
// Pattern for running stdio alongside HTTP server
// HTTP server runs as lifecycle component
httpServer := &http.Server{Addr: ":8080", Handler: handler}
go func() {
    if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        logger.Error("HTTP server error: %v", err)
    }
}()

// Stdio runs optionally in separate goroutine
if stdioEnabled {
    logger.Info("Starting stdio MCP transport")
    go func() {
        // Blocks until client closes connection or context cancelled
        if err := server.ServeStdio(mcpServer); err != nil {
            logger.Error("Stdio transport error: %v", err)
        }
    }()
}

// Both stop when context cancelled
<-ctx.Done()
shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
httpServer.Shutdown(shutdownCtx)
// Stdio stops automatically when context cancelled
```

### Config Hot-Reload with Debouncing
```go
// Source: internal/config/integration_watcher.go (already implemented)
// This is used by integration manager, no changes needed

watcherConfig := config.IntegrationWatcherConfig{
    FilePath:       integrationsConfigPath,
    DebounceMillis: 500, // Per requirements
}

watcher, err := config.NewIntegrationWatcher(watcherConfig, func(newConfig *config.IntegrationsFile) error {
    // Callback: restart all integrations
    logger.Info("Config reloaded, restarting integrations")
    return integrationMgr.handleConfigReload(newConfig)
})

watcher.Start(ctx) // Starts watching in background
// Multiple file changes within 500ms coalesce to single reload
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| SSE Transport for MCP | StreamableHTTP Transport | MCP spec 2025-03-26 | SSE deprecated, use StreamableHTTP. Existing code already uses StreamableHTTP. |
| Manual signal handling | signal.NotifyContext | Go 1.16 (2021) | Cleaner API, automatic context cancellation |
| Gorilla mux for routing | stdlib http.ServeMux | Go 1.22 (2024) | Method-based routing, wildcards now in stdlib |
| Separate MCP sidecar | In-process MCP server | Phase 6 (now) | Single binary, simpler deployment |
| MCP tools via HTTP self-calls | Direct service layer calls | Phase 7 (future) | Better performance, no localhost HTTP |

**Deprecated/outdated:**
- SSE Transport for MCP: Deprecated in MCP spec 2025-03-26, replaced by StreamableHTTP
- Separate mcp command: Will be removed in Phase 8 after consolidation proven
- Integration manager as sidecar concern: Now in-process with main server

## Implementation Strategy

### Recommended Approach

**Minimal code changes required.** This phase is primarily composition:

1. **In `cmd/spectre/commands/server.go` (main change):**
   - After integration manager initialization (line ~203)
   - Create SpectreServer with "http://localhost:8080" as SpectreURL (self-reference)
   - Create MCPToolRegistry adapter
   - Pass registry when creating integration manager (already supports this via NewManagerWithMCPRegistry)
   - Add MCP StreamableHTTPServer to router in registerHandlers()
   - Add `--stdio` flag handling to optionally start stdio transport alongside HTTP

2. **In `internal/apiserver/routes.go`:**
   - Add new method `registerMCPHandler(mcpServer *server.MCPServer)`
   - Create StreamableHTTPServer with `/v1/mcp` endpoint, stateless mode
   - Register on router

3. **In `cmd/spectre/commands/server.go` (flags):**
   - Add `--stdio` bool flag (default false)
   - Remove mutual exclusivity - HTTP always runs

4. **Testing:**
   - Verify MCP tools work via HTTP at `/v1/mcp`
   - Verify integration tools registered dynamically
   - Verify config hot-reload still works (debounced at 500ms)
   - Verify graceful shutdown within 10s timeout
   - Verify stdio works alongside HTTP when `--stdio` flag present

### Self-Reference Pattern

The SpectreServer needs to call Spectre REST API for tool execution. In consolidated mode:
- Current MCP command uses flag `--spectre-url=http://localhost:8080` (separate process)
- Consolidated mode: Use same pattern but both in same process
- Still use HTTP client to localhost - allows reusing existing tool implementations
- Phase 7 will replace HTTP calls with direct service layer calls

### Shutdown Order (Claude's Discretion)

Recommended shutdown sequence:
1. **Stop accepting new requests:** Cancel context, stop http.ServeMux from accepting new connections
2. **Drain in-flight requests:** http.Server.Shutdown() waits for requests to complete (up to timeout)
3. **Stop integrations:** Integration manager stops all instances (they clean up connections)
4. **Force exit if timeout exceeded:** After 10s total, exit process

Rationale: REST and MCP handlers share same http.Server, so they drain together. Integrations stop after to allow MCP tools to finish current operations.

### Exponential Backoff Parameters (Claude's Discretion)

For integration startup retry (when connection fails):

```go
// Simple exponential backoff with jitter
initialDelay := 1 * time.Second
maxDelay := 30 * time.Second
maxRetries := 5

for retry := 0; retry < maxRetries; retry++ {
    if err := integration.Start(ctx); err == nil {
        break // Success
    }

    // Calculate delay: 1s, 2s, 4s, 8s, 16s (capped at 30s)
    delay := initialDelay * (1 << retry)
    if delay > maxDelay {
        delay = maxDelay
    }

    // Add jitter (±10%)
    jitter := time.Duration(rand.Int63n(int64(delay) / 10))
    delay = delay + jitter - (delay / 10)

    logger.Debug("Retry %d/%d after %v", retry+1, maxRetries, delay)
    time.Sleep(delay)
}
```

Rationale: Simple doubling is sufficient. Jitter prevents thundering herd. Max 5 retries = ~30s total (non-blocking, happens in background per requirements).

### SSE Implementation Details (Claude's Discretion)

**Recommendation: Skip SSE, use StreamableHTTP.** The existing `mcp.go` command already uses StreamableHTTP successfully. Requirements specified SSE but research shows:
- SSE deprecated in MCP spec 2025-03-26
- StreamableHTTP is current standard
- mcp-go library supports StreamableHTTP with same API
- No heartbeat configuration needed (library handles it)

If StreamableHTTP used (recommended):
- No custom heartbeat needed (library default)
- Stateless mode per requirements (`WithStateLess(true)`)
- No reconnection hints needed (client-side responsibility)

## Open Questions

Things that couldn't be fully resolved:

1. **SpectreClient localhost behavior**
   - What we know: SpectreClient in mcp/spectre_client.go makes HTTP calls to Spectre REST API
   - What's unclear: Whether localhost HTTP calls within same process cause issues (port binding, timing)
   - Recommendation: Test end-to-end. If problems arise, Phase 7 service layer extraction will eliminate HTTP calls entirely.

2. **Integration retry during shutdown**
   - What we know: Integrations retry with exponential backoff on Start() failure
   - What's unclear: Should retries continue during shutdown, or abort immediately?
   - Recommendation: Use context cancellation to abort retries when shutdown starts. Don't wait for max retries during shutdown.

3. **MCP notifications during config reload**
   - What we know: Server should send MCP notifications when tools change (per requirements)
   - What's unclear: mcp-go library API for sending tool change notifications
   - Recommendation: Research `SendNotificationToClient()` API in mcp-go. May need to track active sessions for notification broadcast.

4. **Stdio transport lifecycle**
   - What we know: `server.ServeStdio()` blocks until stdin closes
   - What's unclear: How to gracefully stop stdio transport on shutdown signal
   - Recommendation: Context cancellation should stop it. Test with timeout to ensure it doesn't block shutdown.

## Sources

### Primary (HIGH confidence)
- mark3labs/mcp-go v0.43.2 - Current dependency in go.mod
- Existing codebase files examined:
  - cmd/spectre/commands/server.go (server startup and shutdown)
  - cmd/spectre/commands/mcp.go (current MCP standalone command)
  - internal/mcp/server.go (MCP server wrapper and tool registry)
  - internal/integration/manager.go (integration lifecycle)
  - internal/lifecycle/manager.go (component orchestration)
  - internal/apiserver/server.go (REST API server)
  - internal/config/integration_watcher.go (config hot-reload)

### Secondary (MEDIUM confidence)
- [MCP-Go SSE Transport Documentation](https://mcp-go.dev/transports/sse/)
- [MCP-Go StreamableHTTP Transport Documentation](https://mcp-go.dev/transports/http/)
- [mcp-go pkg.go.dev](https://pkg.go.dev/github.com/mark3labs/mcp-go/server) - StreamableHTTPServer API
- [Go 1.22+ Enhanced ServeMux](https://dev.to/leapcell/gos-httpservemux-is-all-you-need-1mam)
- [Go Graceful Shutdown Best Practices](https://victoriametrics.com/blog/go-graceful-shutdown/)
- [Go Exponential Backoff Implementation](https://oneuptime.com/blog/post/2026-01-07-go-retry-exponential-backoff/view)

### Tertiary (LOW confidence)
- [SSE Transport Deprecation Notice](https://deepwiki.com/mark3labs/mcp-go/4.1-sse-transport) - "SSE Transport has been deprecated as of MCP specification version 2025-03-26"
- [Go SSE Best Practices](https://www.freecodecamp.org/news/how-to-implement-server-sent-events-in-go/) - General patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries already in use, versions confirmed in go.mod
- Architecture: HIGH - Existing patterns examined, lifecycle manager well-tested
- Pitfalls: HIGH - Based on code review and common Go server patterns
- Implementation strategy: HIGH - Minimal changes to existing well-structured code
- Exponential backoff: MEDIUM - Simple pattern recommended, not library-based
- MCP transport: MEDIUM - StreamableHTTP recommended over SSE (user chose SSE in context)

**Research date:** 2026-01-21
**Valid until:** ~30 days (stable Go stdlib, mcp-go library updates infrequent)

**Key Decision Point:**
User context specified SSE transport, but research reveals SSE deprecated in MCP spec 2025-03-26. Existing mcp.go command successfully uses StreamableHTTP. **Recommend discussing with user: switch to StreamableHTTP or proceed with deprecated SSE?**
