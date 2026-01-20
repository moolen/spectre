# Architecture Patterns: MCP Plugin System + Log Processing Integration

**Domain:** MCP server extension with plugin system and VictoriaLogs integration
**Researched:** 2026-01-20
**Confidence:** HIGH (existing codebase + verified external patterns)

## Executive Summary

This architecture extends the existing Spectre MCP server with a plugin system for dynamic tool registration and a log processing pipeline for VictoriaLogs integration. The design follows interface-based plugin patterns proven in Go ecosystems, separates concerns between log ingestion/mining/storage, and enables hot-reload for configuration changes.

**Key Decision:** Use compile-time plugin registration (not runtime .so loading) for reliability and testability. Interface-based registry pattern with config-driven enablement.

## Recommended Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                          MCP Server Layer                            │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │  MCP Server (existing)                                         │ │
│  │  - Tool registration                                           │ │
│  │  - Prompt registration                                         │ │
│  └────────────────────────────────────────────────────────────────┘ │
│           │ uses                                                     │
│           ▼                                                          │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │  Plugin Manager (NEW)                                          │ │
│  │  - Interface-based registry                                    │ │
│  │  - Config-driven enablement                                    │ │
│  │  - Dynamic tool/prompt registration                            │ │
│  └────────────────────────────────────────────────────────────────┘ │
│           │ manages                                                  │
│           ▼                                                          │
│  ┌──────────────────┐  ┌──────────────────┐  ┌─────────────────┐  │
│  │ Kubernetes Plugin│  │ VictoriaLogs     │  │  Future Plugin  │  │
│  │ (existing tools) │  │ Plugin (NEW)     │  │  (template)     │  │
│  └──────────────────┘  └──────────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Log Processing Pipeline (NEW)                     │
│                                                                       │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  1. Ingestion Layer                                           │  │
│  │  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐ │  │
│  │  │ Kubernetes   │────▶│  Normalizer  │────▶│   Buffer     │ │  │
│  │  │ Event Stream │     │  (timestamp, │     │  (channel)   │ │  │
│  │  │              │     │   metadata)  │     │              │ │  │
│  │  └──────────────┘     └──────────────┘     └──────────────┘ │  │
│  └───────────────────────────────────────────────────────────────┘  │
│           │                                                          │
│           ▼                                                          │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  2. Processing Layer                                          │  │
│  │  ┌──────────────┐     ┌──────────────┐                       │  │
│  │  │ Template     │────▶│  Template    │                       │  │
│  │  │ Miner        │     │  Cache       │                       │  │
│  │  │ (Drain3-like)│     │  (in-memory) │                       │  │
│  │  └──────────────┘     └──────────────┘                       │  │
│  │        │                     │                                │  │
│  │        │                     │ template lookup                │  │
│  │        ▼                     ▼                                │  │
│  │  ┌──────────────────────────────────────┐                    │  │
│  │  │  Structured Log Builder              │                    │  │
│  │  │  - Apply template                    │                    │  │
│  │  │  - Extract variables                 │                    │  │
│  │  │  - Add metadata                      │                    │  │
│  │  └──────────────────────────────────────┘                    │  │
│  └───────────────────────────────────────────────────────────────┘  │
│           │                                                          │
│           ▼                                                          │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  3. Storage Layer                                             │  │
│  │  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐ │  │
│  │  │ Batch        │────▶│ VictoriaLogs │────▶│  Persistent  │ │  │
│  │  │ Aggregator   │     │ HTTP Client  │     │  Template    │ │  │
│  │  │              │     │ (NDJSON)     │     │  Store       │ │  │
│  │  └──────────────┘     └──────────────┘     └──────────────┘ │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Configuration Hot-Reload (NEW)                    │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  File Watcher (fsnotify)                                      │  │
│  │  - Watches config files (watcher.yaml + integrations.yaml)    │  │
│  │  - Debounces rapid changes (100ms window)                     │  │
│  │  - Triggers SIGHUP on change                                  │  │
│  └───────────────────────────────────────────────────────────────┘  │
│           │                                                          │
│           ▼                                                          │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  Signal Handler                                               │  │
│  │  - SIGHUP: Reload config, re-register plugins                │  │
│  │  - SIGTERM/SIGINT: Graceful shutdown                          │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

## Component Boundaries

### 1. Plugin Manager
**Location:** `internal/mcp/plugins/`

**Responsibilities:**
- Maintain registry of available plugins (compile-time)
- Read configuration to determine enabled plugins
- Initialize enabled plugins with their dependencies
- Register tools/prompts with MCP server
- Handle plugin lifecycle (init, reload, shutdown)

**Interfaces:**
```go
type Plugin interface {
    Name() string
    Enabled(config Config) bool
    Initialize(ctx context.Context, deps Dependencies) error
    RegisterTools(server *SpectreServer) error
    RegisterPrompts(server *SpectreServer) error
    Shutdown(ctx context.Context) error
}

type PluginRegistry struct {
    plugins map[string]Plugin
    config  *Config
}
```

**Communicates With:**
- MCP Server (registers tools/prompts)
- Config loader (reads enabled integrations)
- Individual plugins (lifecycle management)

**Configuration:**
```yaml
# integrations.yaml
integrations:
  kubernetes:
    enabled: true
  victorialogs:
    enabled: true
    endpoint: "http://victorialogs:9428"
    batch_size: 100
    flush_interval: "10s"
```

### 2. VictoriaLogs Plugin
**Location:** `internal/mcp/plugins/victorialogs/`

**Responsibilities:**
- Implement Plugin interface
- Manage log processing pipeline
- Expose MCP tools for log querying
- Handle template persistence/loading

**Sub-components:**
- **Ingestion Handler:** Consumes Kubernetes events
- **Template Miner:** Drain-like algorithm for pattern extraction
- **VictoriaLogs Client:** HTTP client for /insert/jsonline endpoint
- **Template Cache:** In-memory template storage with persistence

**Communicates With:**
- Plugin Manager (registration)
- Kubernetes event stream (log source)
- VictoriaLogs HTTP API (storage)
- Disk (template persistence)

### 3. Template Miner
**Location:** `internal/mcp/plugins/victorialogs/miner/`

**Responsibilities:**
- Parse log messages into tokens
- Build prefix tree of templates (Drain algorithm)
- Detect new patterns vs existing templates
- Score template match confidence
- Persist templates to disk for cross-restart consistency

**Algorithm (Drain-inspired):**
```
1. Tokenize log message by whitespace
2. Get token count → navigate to depth layer
3. Get first token → navigate to first-token branch
4. For each template in leaf:
   - Calculate similarity score (matching tokens / total tokens)
   - If score >= threshold (e.g., 0.5): Match found
5. If no match: Create new template
6. Extract variables from matched template
```

**Data Structure:**
```go
type TemplateNode struct {
    Depth     int
    Token     string
    Templates []*Template
    Children  map[string]*TemplateNode
}

type Template struct {
    ID          string
    Pattern     []TokenMatcher  // <*> for variable, literal for constant
    Count       int64
    FirstSeen   time.Time
    LastSeen    time.Time
}
```

**Communicates With:**
- Log normalizer (receives parsed logs)
- Template cache (updates cache)
- Template store (persists templates)

### 4. Log Processing Pipeline
**Location:** `internal/mcp/plugins/victorialogs/pipeline/`

**Responsibilities:**
- Ingest raw Kubernetes events
- Normalize timestamps and metadata
- Apply template mining
- Build structured log entries
- Batch and forward to VictoriaLogs
- Handle backpressure and errors

**Data Flow:**
```
Event → Normalize → Mine/Match → Structure → Batch → VictoriaLogs
```

**Pipeline Stages:**
```go
type Stage interface {
    Process(ctx context.Context, input <-chan LogEntry) <-chan LogEntry
}

// Stages:
// 1. NormalizeStage: timestamp → UTC, add metadata
// 2. MiningStage: extract template, extract variables
// 3. BatchStage: accumulate until size/time threshold
// 4. VictoriaLogsStage: HTTP POST to /insert/jsonline
```

**Backpressure Handling:**
- Bounded channels between stages (buffer size: 1000)
- Drop-oldest policy when channel full
- Metrics for dropped logs
- Circuit breaker for VictoriaLogs failures

**Communicates With:**
- Kubernetes event source (input)
- Template miner (pattern extraction)
- VictoriaLogs HTTP API (output)
- Metrics collector (observability)

### 5. Template Storage
**Location:** `internal/mcp/plugins/victorialogs/store/`

**Responsibilities:**
- Persist templates to disk (JSON or msgpack)
- Load templates on startup
- Update templates incrementally
- Handle concurrent read/write
- Provide template lookup by ID

**Storage Format:**
```json
{
  "version": 1,
  "templates": [
    {
      "id": "tmpl_001",
      "pattern": ["Pod", "<*>", "in", "namespace", "<*>", "failed"],
      "count": 42,
      "first_seen": "2026-01-20T10:00:00Z",
      "last_seen": "2026-01-20T15:30:00Z"
    }
  ]
}
```

**Persistence Strategy:**
- Write-ahead log for incremental updates
- Full snapshot every N updates or on shutdown
- Load snapshot + apply WAL on startup
- fsync on shutdown for durability

**Communicates With:**
- Template miner (read/write)
- Filesystem (persistence)
- Plugin manager (lifecycle)

### 6. Configuration Hot-Reload
**Location:** `internal/config/watcher.go` (extend existing)

**Responsibilities:**
- Watch config files for changes (fsnotify)
- Debounce rapid changes
- Trigger reload signal
- Validate new config before applying

**Implementation Pattern:**
```go
type ConfigWatcher struct {
    watcher   *fsnotify.Watcher
    debouncer *time.Timer
    reloadCh  chan struct{}
}

// Watches:
// - watcher.yaml (existing)
// - integrations.yaml (new)

// On change:
// 1. Debounce (100ms)
// 2. Validate new config
// 3. Send SIGHUP to self OR channel notify
// 4. Plugin manager reloads enabled plugins
```

**Signal Handling:**
```go
// SIGHUP: Hot reload
// - Reload config files
// - Determine plugin changes (enabled/disabled)
// - Shutdown disabled plugins
// - Initialize new plugins
// - Re-register all tools with MCP server

// SIGTERM/SIGINT: Graceful shutdown
// - Flush log pipeline buffers
// - Persist templates to disk
// - Close VictoriaLogs connections
// - Shutdown plugins
// - Exit
```

**Communicates With:**
- Filesystem (inotify events)
- Plugin manager (reload trigger)
- Signal handler (OS signals)

### 7. VictoriaLogs HTTP Client
**Location:** `internal/mcp/plugins/victorialogs/client/`

**Responsibilities:**
- POST NDJSON to /insert/jsonline endpoint
- Handle multitenancy headers (AccountID, ProjectID)
- Configure stream fields, message field, time field
- Retry with exponential backoff
- Circuit breaker for failures

**Request Format:**
```http
POST http://victorialogs:9428/insert/jsonline
Content-Type: application/x-ndjson
VL-Stream-Fields: namespace,pod_name,container_name
VL-Msg-Field: message
VL-Time-Field: timestamp

{"timestamp":"2026-01-20T15:30:00Z","namespace":"default","pod_name":"app-1","container_name":"main","message":"Started server","template_id":"tmpl_042"}
{"timestamp":"2026-01-20T15:30:01Z","namespace":"default","pod_name":"app-1","container_name":"main","message":"Request processed in 45ms","template_id":"tmpl_043","duration_ms":45}
```

**Error Handling:**
- 429 (rate limit): Exponential backoff
- 5xx: Retry with backoff
- 4xx (except 429): Log and drop (malformed data)
- Network error: Circuit breaker, retry

**Communicates With:**
- VictoriaLogs /insert/jsonline endpoint
- Pipeline batch stage (input)
- Metrics collector (success/error rates)

## Patterns to Follow

### Pattern 1: Interface-Based Plugin Registration
**What:** Plugins implement a common interface, register themselves in a compile-time registry

**When:** Need extensibility without runtime .so loading complexity

**Why Better Than Alternatives:**
- Compile-time type safety (vs runtime .so crashes)
- Easy testing with mocks
- No CGO/versioning issues
- Fast initialization

**Example:**
```go
// internal/mcp/plugins/registry.go
var builtinPlugins = []Plugin{
    &kubernetes.Plugin{},
    &victorialogs.Plugin{},
}

func InitializePlugins(config *Config) (*PluginRegistry, error) {
    registry := &PluginRegistry{plugins: make(map[string]Plugin)}

    for _, plugin := range builtinPlugins {
        if plugin.Enabled(config) {
            if err := plugin.Initialize(ctx, deps); err != nil {
                return nil, err
            }
            registry.plugins[plugin.Name()] = plugin
        }
    }

    return registry, nil
}
```

**Reference:** [Interface-based plugin architecture in Go](https://www.dolthub.com/blog/2022-09-12-golang-interface-extension/), [Registry pattern in Golang](https://github.com/Faheetah/registry-pattern)

### Pattern 2: Pipeline Stages with Bounded Channels
**What:** Chain processing stages with buffered channels for backpressure

**When:** Processing stream data with multiple transformation steps

**Why Better Than Alternatives:**
- Natural backpressure (vs unbounded queues consuming memory)
- Easy to add/remove stages
- Testable in isolation

**Example:**
```go
type Pipeline struct {
    stages []Stage
}

func (p *Pipeline) Run(ctx context.Context, input <-chan LogEntry) <-chan LogEntry {
    current := input
    for _, stage := range p.stages {
        current = stage.Process(ctx, current)
    }
    return current
}

// Bounded channel between stages
func (s *NormalizeStage) Process(ctx context.Context, input <-chan LogEntry) <-chan LogEntry {
    output := make(chan LogEntry, 1000) // bounded
    go func() {
        defer close(output)
        for entry := range input {
            normalized := s.normalize(entry)
            select {
            case output <- normalized:
            case <-ctx.Done():
                return
            default:
                // Drop oldest if full
                s.metrics.DroppedLogs.Inc()
            }
        }
    }()
    return output
}
```

**Reference:** [Log processing pipeline architecture](https://aws.amazon.com/blogs/big-data/build-enterprise-scale-log-ingestion-pipelines-with-amazon-opensearch-service/), [Goxe log reduction pipeline](https://github.com/DumbNoxx/Goxe)

### Pattern 3: Drain-Inspired Template Mining
**What:** Build prefix tree by token count and first token, match logs to templates with similarity scoring

**When:** Need to extract patterns from unstructured logs

**Why Better Than Alternatives:**
- O(log n) matching (vs O(n) regex list)
- Handles variable parts naturally
- Low memory footprint

**Example:**
```go
type TemplateMiner struct {
    root       *TemplateNode
    maxDepth   int
    similarity float64
}

func (tm *TemplateMiner) Mine(message string) (*Template, map[string]string) {
    tokens := tokenize(message)
    depth := min(len(tokens), tm.maxDepth)

    // Navigate by token count
    node := tm.root.Children[depth]

    // Navigate by first token
    firstToken := tokens[0]
    node = node.Children[firstToken]

    // Find best matching template
    var bestTemplate *Template
    var bestScore float64

    for _, tmpl := range node.Templates {
        score := tm.similarity(tokens, tmpl.Pattern)
        if score > bestScore {
            bestScore = score
            bestTemplate = tmpl
        }
    }

    if bestScore >= tm.similarity {
        // Match found, extract variables
        vars := extractVariables(tokens, bestTemplate.Pattern)
        return bestTemplate, vars
    }

    // Create new template
    newTmpl := tm.createTemplate(tokens)
    node.Templates = append(node.Templates, newTmpl)
    return newTmpl, nil
}
```

**Reference:** [Drain3 algorithm](https://github.com/logpai/Drain3), [How Drain3 works](https://medium.com/@lets.see.1016/how-drain3-works-parsing-unstructured-logs-into-structured-format-3458ce05b69a)

### Pattern 4: File Watcher with Debouncing
**What:** Watch config files with fsnotify, debounce rapid changes, trigger reload

**When:** Need to respond to file changes without restarting process

**Why Better Than Alternatives:**
- OS-level events (vs polling)
- Debouncing prevents reload storms
- Works across platforms

**Example:**
```go
type ConfigWatcher struct {
    watcher  *fsnotify.Watcher
    debounce time.Duration
    reloadFn func() error
}

func (cw *ConfigWatcher) Watch(ctx context.Context, path string) error {
    // Watch parent directory (not file itself - editors create temp files)
    dir := filepath.Dir(path)
    cw.watcher.Add(dir)

    var debounceTimer *time.Timer

    for {
        select {
        case event := <-cw.watcher.Events:
            if event.Name != path {
                continue
            }

            // Debounce rapid changes
            if debounceTimer != nil {
                debounceTimer.Stop()
            }
            debounceTimer = time.AfterFunc(cw.debounce, func() {
                if err := cw.reloadFn(); err != nil {
                    log.Error("Reload failed: %v", err)
                }
            })

        case <-ctx.Done():
            return nil
        }
    }
}
```

**Reference:** [fsnotify best practices](https://pkg.go.dev/github.com/fsnotify/fsnotify), [Hot reload with SIGHUP](https://rossedman.io/blog/computers/hot-reload-sighup-with-go/)

### Pattern 5: Template Cache with Persistence
**What:** In-memory cache backed by disk persistence, write-ahead log for updates

**When:** Need fast lookups with durability across restarts

**Why Better Than Alternatives:**
- Fast reads (vs hitting disk)
- Durability (vs losing templates on crash)
- Incremental updates (vs full rewrites)

**Example:**
```go
type TemplateStore struct {
    cache     map[string]*Template  // in-memory
    walFile   *os.File              // write-ahead log
    snapFile  string                // snapshot path
    mu        sync.RWMutex
    dirty     int                   // updates since snapshot
}

func (ts *TemplateStore) Get(id string) (*Template, bool) {
    ts.mu.RLock()
    defer ts.mu.RUnlock()
    tmpl, ok := ts.cache[id]
    return tmpl, ok
}

func (ts *TemplateStore) Update(tmpl *Template) error {
    ts.mu.Lock()
    defer ts.mu.Unlock()

    // Update cache
    ts.cache[tmpl.ID] = tmpl

    // Append to WAL
    if err := ts.appendWAL(tmpl); err != nil {
        return err
    }

    ts.dirty++

    // Snapshot if threshold reached
    if ts.dirty >= 1000 {
        return ts.snapshot()
    }

    return nil
}

func (ts *TemplateStore) Load() error {
    // Load snapshot
    if err := ts.loadSnapshot(); err != nil {
        return err
    }

    // Replay WAL
    return ts.replayWAL()
}
```

**Reference:** [Distributed caching with consistency](https://dev.to/nayanraj-adhikary/deep-dive-caching-in-distributed-systems-at-scale-3h1g)

## Anti-Patterns to Avoid

### Anti-Pattern 1: Runtime Plugin Loading (.so files)
**What:** Using Go's plugin package to load .so files at runtime

**Why Bad:**
- Platform-specific (Linux only)
- Version sensitivity (Go version must match exactly)
- No type safety (reflect-based APIs)
- Debugging nightmares (crashes instead of compile errors)
- Build complexity (need to compile plugins separately)

**Instead:** Use compile-time registration with interface-based plugins

**When It Might Be Okay:** Extreme isolation requirements where plugin crashes must not affect main process (but then use RPC-based plugins instead)

**Reference:** [Plugins in Go - limitations](https://eli.thegreenplace.net/2021/plugins-in-go/), [Compile-time plugin architecture](https://medium.com/@mzawiejski/compile-time-plugin-architecture-in-go-923455cd2297)

### Anti-Pattern 2: Unbounded Channels in Pipeline
**What:** Using unbuffered or infinite-buffered channels between pipeline stages

**Why Bad:**
- Unbuffered: Creates artificial backpressure, slows entire pipeline to slowest stage
- Infinite-buffered: Memory exhaustion under load, no backpressure signal
- No visibility into queue depth

**Instead:** Use bounded channels with drop-oldest policy and metrics

**Example of What NOT to Do:**
```go
// BAD: Unbounded channel
output := make(chan LogEntry)  // blocks when consumer is slow

// BAD: No size limit
var buffer []LogEntry  // grows forever under load
```

**Instead:**
```go
// GOOD: Bounded with overflow handling
output := make(chan LogEntry, 1000)
select {
case output <- entry:
case <-ctx.Done():
    return
default:
    metrics.DroppedLogs.Inc()
    // Drop oldest or log warning
}
```

### Anti-Pattern 3: Watching Individual Config Files
**What:** Using fsnotify to watch specific config files directly

**Why Bad:**
- Many editors (vim, emacs) write to temp file then rename
- Original file watcher is lost after rename
- Results in reload not triggering after first edit

**Instead:** Watch parent directory and filter by filename

**Reference:** [fsnotify best practices](https://pkg.go.dev/github.com/fsnotify/fsnotify)

### Anti-Pattern 4: Synchronous VictoriaLogs Writes in Event Handler
**What:** Blocking Kubernetes event processing to write to VictoriaLogs

**Why Bad:**
- Event processing stalls if VictoriaLogs is slow/down
- Missed events if Kubernetes client buffer overflows
- Tight coupling between ingestion and storage

**Instead:** Async pipeline with buffering and circuit breaker

### Anti-Pattern 5: Template Matching with Regex List
**What:** Maintaining array of regex patterns, testing each sequentially

**Why Bad:**
- O(n) time complexity for n templates
- Slow regex compilation
- Hard to maintain as templates grow
- No learning (static patterns)

**Instead:** Use Drain prefix tree with similarity scoring

## Scalability Considerations

| Concern | At 100 pods | At 1K pods | At 10K pods |
|---------|------------|------------|-------------|
| **Event ingestion rate** | ~10 events/sec | ~100 events/sec | ~1K events/sec |
| **Approach** | Single pipeline goroutine | Single pipeline with batching | Multiple pipeline workers (shard by namespace) |
| **Template count** | ~50 templates | ~500 templates | ~5K templates |
| **Approach** | In-memory tree | In-memory tree + periodic snapshot | In-memory tree + LRU eviction for rare templates |
| **VictoriaLogs writes** | Batch every 10s | Batch every 5s or 100 entries | Batch every 1s or 1000 entries, multiple client instances |
| **Template persistence** | Single WAL file | Single WAL file + hourly snapshots | Partitioned WAL by namespace, parallel snapshot writers |
| **Memory footprint** | ~50MB | ~200MB | ~1GB |
| **Approach** | Default settings | Increase channel buffers to 5K | Tune GC, use sync.Pool for log entries |

## Build Order and Dependencies

### Phase 1: Plugin Infrastructure (Foundation)
**Goal:** Enable plugin-based architecture without breaking existing functionality

**Components:**
1. Plugin interface definition (`internal/mcp/plugins/interface.go`)
2. Plugin registry (`internal/mcp/plugins/registry.go`)
3. Config loader extension for `integrations.yaml`
4. Migrate existing tools to Kubernetes plugin

**Dependencies:**
- Existing MCP server structure
- Config package

**Validation:**
- Existing tools work via Kubernetes plugin
- Can disable Kubernetes plugin via config
- Plugin registry logs enabled plugins

### Phase 2: VictoriaLogs Client (External Integration)
**Goal:** Establish reliable communication with VictoriaLogs

**Components:**
1. HTTP client for /insert/jsonline endpoint
2. NDJSON serialization
3. Retry/backoff logic
4. Circuit breaker

**Dependencies:**
- VictoriaLogs instance (test with docker-compose)

**Validation:**
- Can write test logs to VictoriaLogs
- Handles VictoriaLogs downtime gracefully
- Metrics show success/error rates

### Phase 3: Log Processing Pipeline (Core Logic)
**Goal:** Transform Kubernetes events into structured logs

**Components:**
1. Pipeline stages (normalize, batch)
2. Kubernetes event ingestion
3. Channel-based backpressure
4. Integration with VictoriaLogs client

**Dependencies:**
- VictoriaLogs client (Phase 2)
- Existing Kubernetes event stream

**Validation:**
- Events flow from K8s to VictoriaLogs
- Backpressure prevents memory exhaustion
- Logs are queryable in VictoriaLogs

### Phase 4: Template Mining (Advanced Feature)
**Goal:** Extract patterns from logs for better querying

**Components:**
1. Drain-inspired template miner
2. Template cache (in-memory)
3. Template persistence (disk)
4. Integration with pipeline

**Dependencies:**
- Log processing pipeline (Phase 3)

**Validation:**
- Templates detected from event messages
- Template IDs in VictoriaLogs logs
- Templates persist across restarts

### Phase 5: MCP Tool Exposure (User Interface)
**Goal:** Enable AI assistants to query logs via MCP

**Components:**
1. `query_logs` tool implementation
2. `analyze_log_patterns` tool implementation
3. VictoriaLogs plugin registration

**Dependencies:**
- Plugin infrastructure (Phase 1)
- VictoriaLogs client (Phase 2)
- Template mining (Phase 4)

**Validation:**
- Can query logs via MCP tool
- Results include template information
- Cross-references with existing timeline tools

### Phase 6: Configuration Hot-Reload (Operational Excellence)
**Goal:** Enable config changes without restart

**Components:**
1. File watcher with debouncing
2. Signal handler (SIGHUP)
3. Plugin reload logic
4. Validation before applying config

**Dependencies:**
- Plugin infrastructure (Phase 1)

**Validation:**
- Config change triggers reload
- Invalid config rejected without restart
- Plugins re-register tools correctly

## Component Communication Matrix

| From → To | Plugin Manager | VictoriaLogs Plugin | Template Miner | VictoriaLogs API | Config Watcher |
|-----------|----------------|---------------------|----------------|------------------|----------------|
| **MCP Server** | Calls during startup | - | - | - | - |
| **Plugin Manager** | - | Initialize/shutdown | - | - | Receives reload signal |
| **VictoriaLogs Plugin** | Registers self | - | Uses for mining | Uses for storage | - |
| **Template Miner** | - | Returns templates | - | - | - |
| **Pipeline Stages** | - | Owned by plugin | Calls for mining | - | - |
| **Config Watcher** | Triggers reload | - | - | - | - |
| **K8s Event Stream** | - | Sends events to plugin | - | - | - |

## Data Flow Summary

### 1. Startup Flow
```
main()
  → Load config (watcher.yaml, integrations.yaml)
  → Initialize plugin registry
  → For each enabled plugin:
      → plugin.Initialize(deps)
      → plugin.RegisterTools(mcpServer)
  → Start MCP server
  → Start config watcher
  → Start log pipeline (if VictoriaLogs enabled)
```

### 2. Event Processing Flow
```
K8s Event
  → Normalize (UTC timestamp, add metadata)
  → Template Mining (match or create template)
  → Structure (template_id, extracted variables)
  → Batch (accumulate until threshold)
  → VictoriaLogs HTTP POST (NDJSON)
  → Persist Template Updates (WAL)
```

### 3. Reload Flow
```
Config file changed
  → fsnotify event
  → Debounce (100ms)
  → Validate new config
  → Send SIGHUP
  → Plugin manager:
      → Shutdown disabled plugins
      → Initialize new plugins
      → Re-register all tools
  → Log pipeline:
      → Flush buffers
      → Reload settings
```

### 4. Query Flow (MCP Tool)
```
MCP client calls query_logs
  → VictoriaLogs plugin
  → Build LogsQL query
  → HTTP GET to /select/logsql
  → Parse results
  → Enrich with template information
  → Return structured response
```

## Sources

Architecture patterns and best practices referenced:

### Plugin Architecture
- [DoltHub: Golang Interface Extension](https://www.dolthub.com/blog/2022-09-12-golang-interface-extension/)
- [Registry Pattern in Golang](https://github.com/Faheetah/registry-pattern)
- [Sling Academy: Plugin-Based Architecture in Go](https://www.slingacademy.com/article/leveraging-interfaces-for-plugin-based-architecture-in-go-applications/)
- [Eli Bendersky: Plugins in Go](https://eli.thegreenplace.net/2021/plugins-in-go/)
- [Medium: Compile-Time Plugin Architecture](https://medium.com/@mzawiejski/compile-time-plugin-architecture-in-go-923455cd2297)

### Log Processing Pipelines
- [AWS: Log Ingestion Pipelines](https://aws.amazon.com/blogs/big-data/build-enterprise-scale-log-ingestion-pipelines-with-amazon-opensearch-service/)
- [Goxe: Log Reduction Tool](https://github.com/DumbNoxx/Goxe)
- [Dattell: Log Ingestion Best Practices 2025](https://dattell.com/data-architecture-blog/log-ingestion-best-practices-for-elasticsearch-in-2025/)

### Template Mining (Drain Algorithm)
- [Drain3 Repository](https://github.com/logpai/Drain3)
- [IBM: Mining Log Templates](https://developer.ibm.com/blogs/how-mining-log-templates-can-help-ai-ops-in-cloud-scale-data-centers)
- [Medium: How Drain3 Works](https://medium.com/@lets.see.1016/how-drain3-works-parsing-unstructured-logs-into-structured-format-3458ce05b69a)
- [ClickHouse: Log Clustering](https://clickhouse.com/blog/improve-compression-log-clustering)

### File Watching and Hot Reload
- [fsnotify Documentation](https://pkg.go.dev/github.com/fsnotify/fsnotify)
- [fsnotify Repository](https://github.com/fsnotify/fsnotify)
- [rossedman: Hot Reload with SIGHUP](https://rossedman.io/blog/computers/hot-reload-sighup-with-go/)
- [ITNEXT: Hot-Reloading Go Applications](https://itnext.io/clean-and-simple-hot-reloading-on-uninterrupted-go-applications-5974230ab4c5)
- [Vai: Hot Reload Tool](https://github.com/sgtdi/vai)
- [Cybozu: Graceful Restart](https://github.com/cybozu-go/well/wiki/Graceful-restart)

### VictoriaLogs
- [VictoriaLogs Documentation](https://docs.victoriametrics.com/victorialogs/)
- [VictoriaLogs: Architecture Basics](https://victoriametrics.com/blog/victorialogs-architecture-basics/)
- [VictoriaLogs: LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/)
- [Greptime: VictoriaLogs Source Reading](https://greptime.com/blogs/2025-02-27-victorialogs-source-reading-greptimedb)
- [VictoriaLogs Data Ingestion](https://docs.victoriametrics.com/victorialogs/data-ingestion/)

### Distributed Systems Patterns
- [Frontiers: Distributed Caching with Strong Consistency](https://www.frontiersin.org/journals/computer-science/articles/10.3389/fcomp.2025.1511161/full)
- [DEV: Caching in Distributed Systems](https://dev.to/nayanraj-adhikary/deep-dive-caching-in-distributed-systems-at-scale-3h1g)
- [Baeldung: Dependency Injection vs Service Locator](https://www.baeldung.com/cs/dependency-injection-vs-service-locator)
- [Service Locator Pattern in Go](https://softwarepatternslexicon.com/patterns-go/10/2/)
