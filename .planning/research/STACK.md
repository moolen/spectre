# Technology Stack: MCP Plugin System + VictoriaLogs Integration

**Project:** Spectre MCP Plugin System with VictoriaLogs
**Researched:** 2026-01-20
**Confidence:** HIGH for plugin systems and config management, MEDIUM for log template mining, HIGH for VictoriaLogs API

---

## Recommended Stack

### 1. Plugin System: HashiCorp go-plugin

| Technology | Version | Purpose | Confidence |
|------------|---------|---------|------------|
| `github.com/hashicorp/go-plugin` | v1.7.0 | RPC-based plugin architecture for observability integrations | HIGH |

**Why HashiCorp go-plugin over native Go plugins:**

The native `plugin` package has critical limitations that make it unsuitable for this use case:
- **Platform-locked**: Only works on Linux, FreeBSD, and macOS (no Windows support)
- **Build coupling**: Plugins and host must be built with identical toolchain versions, build tags, and flags
- **No unloading**: Once loaded, plugins cannot be unloaded (memory leak risk)
- **Race detector incompatibility**: Poor support for race condition detection

HashiCorp go-plugin solves these problems through RPC-based isolation:
- **Cross-platform**: Works everywhere Go runs via standard net/rpc or gRPC
- **Process isolation**: Plugin crashes don't crash the host MCP server
- **Independent builds**: Plugins can be compiled separately and upgraded independently
- **Security**: Plugins only access explicitly exposed interfaces, not entire process memory
- **Battle-tested**: Used by Terraform, Vault, Nomad, Packer (production-proven on millions of machines)

**Trade-off**: Slightly lower performance vs native plugins (RPC overhead), but negligible for observability integrations where network I/O dominates.

**Installation:**
```bash
go get github.com/hashicorp/go-plugin@v1.7.0
```

**Sources:**
- [HashiCorp go-plugin on Go Packages](https://pkg.go.dev/github.com/hashicorp/go-plugin) (HIGH confidence)
- [Building Dynamic Applications with Go Plugins](https://leapcell.io/blog/building-dynamic-and-extensible-applications-with-go-plugins) (MEDIUM confidence)
- [Native plugin limitations](https://pkg.go.dev/plugin) (HIGH confidence - official docs)

---

### 2. Configuration Management: Koanf

| Technology | Version | Purpose | Confidence |
|------------|---------|---------|------------|
| `github.com/knadh/koanf/v2` | v2.3.0 | Hot-reload configuration management | HIGH |
| `github.com/knadh/koanf/providers/file/v2` | v2.3.0 | File watching provider | HIGH |
| `github.com/knadh/koanf/parsers/yaml/v2` | v2.3.0 | YAML parsing | HIGH |
| `github.com/fsnotify/fsnotify` | v1.9.0 | File system watching (transitive) | HIGH |

**Why Koanf over Viper:**

Viper has fundamental design flaws that make it problematic:
- **Case sensitivity breaking**: Forcibly lowercases all keys, violating JSON/YAML/TOML specs
- **Bloated binaries**: viper binary is 313% larger than koanf for equivalent functionality
- **Tight coupling**: Config parsing hardcoded to file extensions; no clean abstractions
- **Dependency hell**: Pulls in dependencies for ALL formats even if you only use one (YAML, TOML, HCL, etc. all bundled)
- **Mutation bugs**: `Get()` returns references to slices/maps; external mutations leak into config

Koanf advantages:
- **Modular**: Each provider (file, env, S3) and parser (JSON, YAML, TOML) is a separate module
- **Correct semantics**: Respects case sensitivity and language specs
- **Hot-reload built-in**: `Watch()` method on file provider triggers callbacks on config changes
- **Lightweight**: Minimal dependencies per module
- **v2 architecture**: One repository, many modules—only install what you need

**Thread safety note**: Koanf's Watch callback is NOT goroutine-safe with concurrent `Get()` calls during `Load()`. Solution: Use mutex locking or atomic pointer swapping for config reloads.

**Installation:**
```bash
# Core + file provider + YAML parser
go get github.com/knadh/koanf/v2@v2.3.0
go get github.com/knadh/koanf/providers/file/v2@v2.3.0
go get github.com/knadh/koanf/parsers/yaml/v2@v2.3.0
```

**Sources:**
- [Koanf GitHub releases](https://github.com/knadh/koanf/releases) (HIGH confidence)
- [Viper vs Koanf comparison](https://itnext.io/golang-configuration-management-library-viper-vs-koanf-eea60a652a22) (MEDIUM confidence)
- [Koanf official comparison with Viper](https://github.com/knadh/koanf/wiki/Comparison-with-spf13-viper) (HIGH confidence)

---

### 3. Log Template Mining: LoggingDrain

| Technology | Version | Purpose | Confidence |
|------------|---------|---------|------------|
| `github.com/PalanQu/LoggingDrain` | Latest (main) | Drain algorithm implementation for log template extraction | MEDIUM |

**Why LoggingDrain over alternatives:**

**Algorithm choice: Drain** is the recommended algorithm for production log template mining:
- **Online processing**: Streaming algorithm, no need to batch all logs
- **Fixed-depth tree**: O(log n) search complexity vs linear scan in IPLoM/Spell
- **Parameter stability**: Only 2 main tuning parameters (sim_th, depth) vs complex heuristics
- **Proven at scale**: Used in industrial AIOps systems (IBM research, production deployments)

**Go implementations comparison:**

| Library | Status | Performance | Features | Recommendation |
|---------|--------|-------------|----------|----------------|
| `faceair/drain` | Stale (last update: Feb 2022) | Unknown | Basic Drain port | DO NOT USE (inactive) |
| `PalanQu/LoggingDrain` | Active (Oct 2024) | 699ns/op (build), 349ns/op (match) | Redis persistence, benchmarked | RECOMMENDED |

**LoggingDrain advantages:**
- **Recent updates**: Last commit October 2024 (active maintenance)
- **Performance**: Sub-microsecond matching, suitable for high-volume logs
- **Persistence**: Built-in Redis support (optional, useful for canonical template storage)
- **Benchmarked**: darwin/arm64 performance metrics published

**Alternative if LoggingDrain proves immature:** Implement Drain from scratch using the original paper. The algorithm is straightforward (fixed-depth prefix tree + similarity threshold).

**Installation:**
```bash
go get github.com/PalanQu/LoggingDrain@latest
```

**Drain Configuration for production:**
```go
config := &drain.Config{
    LogClusterDepth:  4,      // Tree depth (increase for long structured logs)
    SimTh:           0.4,     // Similarity threshold (0.3 for structured, 0.5-0.6 for messy)
    MaxChildren:     100,     // Max branches per node
    MaxClusters:     1000,    // Max templates to track
    ParamString:     "<*>",   // Wildcard replacement
}
```

**Sources:**
- [LoggingDrain GitHub](https://github.com/PalanQu/LoggingDrain) (MEDIUM confidence - recent but small community)
- [Drain3 research paper](https://github.com/logpai/Drain3) (HIGH confidence - original algorithm)
- [faceair/drain package](https://pkg.go.dev/github.com/faceair/drain) (LOW confidence - stale)

**Risk mitigation:** If LoggingDrain has bugs or lacks features, the Drain algorithm is simple enough to implement in-house (200-300 LOC for core logic).

---

### 4. VictoriaLogs Client: Standard net/http

| Technology | Version | Purpose | Confidence |
|------------|---------|---------|------------|
| `net/http` (stdlib) | Go 1.24.4+ | VictoriaLogs HTTP API client | HIGH |

**Why standard library over dedicated client:**

VictoriaLogs exposes a simple HTTP API—no official Go client exists, and none is needed:
- **HTTP endpoints**: `/select/logsql/query`, `/select/logsql/tail`, `/select/logsql/stats_query*`
- **Request format**: Query via `query` parameter (GET or POST with x-www-form-urlencoded)
- **Response format**: Line-delimited JSON for streaming results
- **No authentication**: Base URL only (no auth tokens, API keys)

**API patterns:**

```go
// Query endpoint
POST /select/logsql/query
Content-Type: application/x-www-form-urlencoded

query=error | stats count() by namespace

// Response: streaming newline-delimited JSON
{"_msg": "...", "namespace": "default", ...}
{"_msg": "...", "namespace": "kube-system", ...}

// Stats query (Prometheus-compatible)
GET /select/logsql/stats_query?query=error | stats count()&time=2026-01-20T10:00:00Z
```

**Best practices (from VictoriaMetrics team):**
- **HTTP/2**: Use HTTPS for automatic HTTP/2 multiplexing (reduces latency for parallel queries)
- **Streaming**: Read response as stream, don't buffer entire result set
- **Keep-alive**: Reuse HTTP client with connection pooling (`http.Client` with `MaxIdleConns`)
- **Context**: Use `context.Context` for query timeouts and cancellation

**Thin client wrapper recommended:** Create a small `victorialogsclient` package wrapping `net/http` with typed methods:
- `Query(ctx, logsql) (io.ReadCloser, error)`
- `StatsQuery(ctx, logsql, time) (PrometheusResponse, error)`
- `Tail(ctx, logsql) (io.ReadCloser, error)`

**Installation:** No external dependencies—`net/http` is stdlib.

**Sources:**
- [VictoriaLogs Querying API docs](https://docs.victoriametrics.com/victorialogs/querying/) (HIGH confidence - official docs)
- [VictoriaLogs HTTP API search results](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6943) (HIGH confidence)
- [Go HTTP/2 best practices (VictoriaMetrics blog)](https://victoriametrics.com/blog/go-http2/) (HIGH confidence)

---

## Supporting Libraries

### Already in go.mod (Reuse)

| Library | Current Version | Purpose | Notes |
|---------|-----------------|---------|-------|
| `github.com/mark3labs/mcp-go` | v0.43.2 | MCP server framework | Already integrated; use tool registration API |
| `connectrpc.com/connect` | v1.19.1 | REST API (gRPC/Connect) | Already integrated; add integration management endpoints |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML parsing | Already indirect; use for config serialization |
| `golang.org/x/sync` | v0.18.0 | Synchronization primitives | Use `singleflight` for deduplicating concurrent config reloads |

### New Dependencies Required

| Library | Version | Purpose | When to Install |
|---------|---------|---------|-----------------|
| `github.com/hashicorp/go-plugin` | v1.7.0 | Plugin system | Phase 1: Plugin architecture |
| `github.com/knadh/koanf/v2` | v2.3.0 | Config management | Phase 1: Hot-reload config |
| `github.com/knadh/koanf/providers/file/v2` | v2.3.0 | File watching | Phase 1: Hot-reload config |
| `github.com/knadh/koanf/parsers/yaml/v2` | v2.3.0 | YAML parser | Phase 1: Hot-reload config |
| `github.com/PalanQu/LoggingDrain` | Latest | Log template mining | Phase 2: VictoriaLogs integration |

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| **Plugin System** | HashiCorp go-plugin (RPC) | Native `plugin` package | Platform-locked (Linux/Mac only), build coupling, no unloading, race detector issues |
| **Config Management** | Koanf v2 | Viper | Case-insensitivity bugs, bloated dependencies (313% larger binaries), poor abstractions |
| **Config Hot-reload** | Koanf Watch() + fsnotify | SIGHUP signal handler | Koanf's file watcher is cleaner; SIGHUP requires manual signal handling and inode tracking |
| **Log Template Mining** | Drain (LoggingDrain) | IPLoM | O(n) linear scan vs O(log n) tree search; Drain is faster for high-volume logs |
| **Log Template Mining** | Drain (LoggingDrain) | Spell | Spell requires tuning LCS thresholds; Drain's similarity threshold is simpler |
| **Log Template Mining** | LoggingDrain | faceair/drain | faceair/drain is stale (last update Feb 2022); LoggingDrain actively maintained |
| **VictoriaLogs Client** | net/http (stdlib) | Custom fasthttp client | VictoriaMetrics' fasthttp fork is for internal use only; net/http is sufficient and well-supported |
| **VictoriaLogs Client** | net/http (stdlib) | Official Go client | No official client exists; HTTP API is simple enough that net/http is ideal |

---

## Installation Commands

### Phase 1: Plugin System + Config Hot-reload

```bash
# Plugin system
go get github.com/hashicorp/go-plugin@v1.7.0

# Configuration management
go get github.com/knadh/koanf/v2@v2.3.0
go get github.com/knadh/koanf/providers/file/v2@v2.3.0
go get github.com/knadh/koanf/parsers/yaml/v2@v2.3.0
```

### Phase 2: VictoriaLogs Integration

```bash
# Log template mining
go get github.com/PalanQu/LoggingDrain@latest

# VictoriaLogs client: no dependencies (stdlib net/http)
```

---

## Architecture Integration Notes

### MCP-Go Plugin Pattern

The `mark3labs/mcp-go` library uses a **composable handler pattern** rather than traditional plugins:
- Tools registered via `server.AddTool(name, handler, schema)`
- Resources registered via `server.AddResource(uri, handler)`
- No built-in plugin loading—manual registration in server initialization

**Integration strategy:** Use HashiCorp go-plugin to load observability integrations as separate processes, then have each plugin register its tools/resources with the MCP server via RPC interface.

```go
// Plugin interface (shared between host and plugins)
type ObservabilityPlugin interface {
    GetTools() []mcp.Tool
    GetResources() []mcp.Resource
}

// Host loads plugin via go-plugin
client := plugin.NewClient(&plugin.ClientConfig{...})
raw, _ := client.Client().Dispense("observability")
integration := raw.(ObservabilityPlugin)

// Register plugin's tools with MCP server
for _, tool := range integration.GetTools() {
    mcpServer.AddTool(tool.Name, tool.Handler, tool.Schema)
}
```

### Configuration Structure

```yaml
# config/integrations.yaml
integrations:
  victorialogs:
    enabled: true
    base_url: http://localhost:9428
    default_time_range: 60m
    sampling_threshold: 10000  # Sample if namespace has >10k logs
    template_mining:
      algorithm: drain
      similarity_threshold: 0.4
      max_clusters: 1000
```

Hot-reload flow:
1. Koanf file watcher detects `integrations.yaml` change
2. Callback triggered → reload config with mutex lock
3. Notify plugin manager of config change
4. Plugin manager restarts affected plugins with new config
5. MCP server re-registers tools from reloaded plugins

---

## Performance Considerations

| Component | Throughput | Latency | Bottleneck |
|-----------|------------|---------|------------|
| HashiCorp go-plugin RPC | ~10k req/s | <1ms overhead | Negligible vs network I/O to VictoriaLogs |
| Koanf config reload | N/A | <10ms for typical config files | Mutex contention during reload (use atomic pointer swap) |
| LoggingDrain template mining | ~1.4M logs/s (699ns build + 349ns match) | Sub-microsecond | None (faster than VictoriaLogs query latency) |
| VictoriaLogs HTTP API | Depends on log volume | Streaming (progressive results) | Network + query complexity |

**Scalability:** All components scale to production workloads. The plugin RPC overhead is negligible compared to log query network latency (typically 100ms-1s for large time ranges).

---

## Confidence Assessment

| Area | Confidence | Rationale |
|------|------------|-----------|
| **Plugin System (go-plugin)** | HIGH | HashiCorp go-plugin is battle-tested in Terraform/Vault/Nomad with 4+ years production use; official documentation and 3,570+ imports validate maturity |
| **Config Management (Koanf)** | HIGH | v2.3.0 released Sept 2024; modular architecture solves known Viper issues; comparison wiki directly addresses use case |
| **Hot-reload (fsnotify)** | HIGH | v1.9.0 released April 2025; cross-platform; imported by 12,768 packages; stdlib-quality maturity |
| **Log Mining (LoggingDrain)** | MEDIUM | Active maintenance (Oct 2024) and benchmarked performance, BUT small community (16 stars); risk mitigated by simple algorithm (can reimplement if needed) |
| **Log Mining (Drain algorithm)** | HIGH | Original research paper (ICWS 2017); proven in industrial AIOps (IBM, production deployments); algorithm simplicity reduces implementation risk |
| **VictoriaLogs API** | HIGH | Official documentation (docs.victoriametrics.com); HTTP API is simple and well-documented; no client needed (stdlib sufficient) |

**Overall stack confidence:** HIGH. The only MEDIUM-confidence component (LoggingDrain) has a clear mitigation path (re-implement Drain in 200-300 LOC if library proves buggy).

---

## Sources

### High-Confidence Sources (Official Docs, Package Registries)
- [hashicorp/go-plugin v1.7.0 on Go Packages](https://pkg.go.dev/github.com/hashicorp/go-plugin)
- [hashicorp/go-plugin GitHub releases](https://github.com/hashicorp/go-plugin/releases)
- [knadh/koanf v2.3.0 GitHub releases](https://github.com/knadh/koanf/releases)
- [knadh/koanf comparison with Viper (official wiki)](https://github.com/knadh/koanf/wiki/Comparison-with-spf13-viper)
- [fsnotify v1.9.0 releases](https://github.com/fsnotify/fsnotify/releases)
- [fsnotify on Go Packages](https://pkg.go.dev/github.com/fsnotify/fsnotify)
- [VictoriaLogs Querying API (official docs)](https://docs.victoriametrics.com/victorialogs/querying/)
- [Native Go plugin package (stdlib docs)](https://pkg.go.dev/plugin)
- [mark3labs/mcp-go GitHub](https://github.com/mark3labs/mcp-go)

### Medium-Confidence Sources (Blog Posts, Comparisons)
- [Building Dynamic Applications with Go Plugins (Leapcell blog)](https://leapcell.io/blog/building-dynamic-and-extensible-applications-with-go-plugins)
- [Viper vs Koanf comparison (ITNEXT)](https://itnext.io/golang-configuration-management-library-viper-vs-koanf-eea60a652a22)
- [The Best Go Configuration Management Library (Medium)](https://medium.com/pragmatic-programmers/koanf-for-go-967577726cd8)
- [Go HTTP/2 best practices (VictoriaMetrics blog)](https://victoriametrics.com/blog/go-http2/)
- [PalanQu/LoggingDrain GitHub](https://github.com/PalanQu/LoggingDrain)
- [Drain3 algorithm (logpai GitHub)](https://github.com/logpai/Drain3)

### Low-Confidence Sources (Unverified or Stale)
- [faceair/drain on Go Packages](https://pkg.go.dev/github.com/faceair/drain) — Stale (last update Feb 2022)

---

## Next Steps for Roadmap

Based on this stack research, suggested phase structure:

1. **Phase 1: Plugin Foundation**
   - Implement HashiCorp go-plugin architecture
   - Add Koanf-based config hot-reload
   - Define `ObservabilityPlugin` interface
   - Stub VictoriaLogs plugin (no-op tools)

2. **Phase 2: VictoriaLogs Integration**
   - Implement VictoriaLogs HTTP client (net/http wrapper)
   - Integrate LoggingDrain for template mining
   - Build progressive disclosure tools (global overview → aggregated → full logs)
   - Canonical template storage (in-memory or Redis)

3. **Phase 3: UI & API**
   - REST API endpoints for integration management
   - React UI for enabling/configuring integrations
   - Config persistence and validation

**Ordering rationale:** Plugin architecture must exist before VictoriaLogs integration. Log template mining (Phase 2) is independent of UI (Phase 3), so they could be parallelized if needed.

**Research flags:** No additional research needed—all stack decisions are high-confidence or have clear mitigation paths.
