# Project Research Summary

**Project:** Spectre MCP Plugin System with VictoriaLogs Integration
**Domain:** MCP server extensibility with observability integrations
**Researched:** 2026-01-20
**Confidence:** HIGH

## Executive Summary

This project extends the existing Spectre MCP server with a plugin architecture that enables dynamic tool registration for observability integrations. The primary use case is VictoriaLogs integration with intelligent log exploration using template mining and progressive disclosure UX patterns.

Expert systems build extensible observability platforms using compile-time plugin registration (not runtime .so loading) with RPC-based process isolation. The recommended approach uses HashiCorp go-plugin for plugin lifecycle, Koanf for hot-reload configuration management, and Drain algorithm for log template mining. Critical architecture decisions include: interface-based plugin registry (avoiding Go stdlib plugin versioning hell), pipeline stages with bounded channels for backpressure, and atomic pointer swap for race-free config reloads.

The primary risk is template mining instability with variable-starting logs, which causes template explosion and degrades accuracy from 90% to under 70%. Mitigation requires pre-tokenization with masking, periodic template rebalancing, and monitoring template growth metrics. Secondary risks include config hot-reload race conditions (prevented via atomic.Value) and progressive disclosure state loss (prevented via URL-based state). All critical risks have proven mitigation strategies from production deployments.

## Key Findings

### Recommended Stack

Research identified battle-tested technologies for plugin systems and log processing, avoiding common pitfalls like Go stdlib plugin versioning constraints and Viper's case-sensitivity bugs.

**Core technologies:**
- **HashiCorp go-plugin v1.7.0**: RPC-based plugin architecture — avoids stdlib plugin versioning hell, provides process isolation, production-proven in Terraform/Vault/Nomad
- **Koanf v2.3.0**: Hot-reload configuration management — modular design, built-in file watching, fixes Viper's case-insensitivity and bloat issues
- **LoggingDrain (Drain algorithm)**: Log template mining — O(log n) matching, handles high-volume streams, sub-microsecond performance
- **net/http (stdlib)**: VictoriaLogs client — sufficient for simple HTTP API, no custom client needed
- **Existing stack reuse**: mark3labs/mcp-go for MCP server, connectrpc for REST API, gopkg.in/yaml.v3 for config

**Stack confidence:** HIGH overall. Only MEDIUM component is LoggingDrain library (small community), but Drain algorithm itself is HIGH confidence (proven in academic research and IBM production systems). Mitigation: algorithm is simple enough to re-implement in 200-300 LOC if library proves buggy.

### Expected Features

Research revealed MCP ecosystem favors minimalist tool design (10-20 tools maximum) due to context window constraints, directly influencing how plugins expose functionality and how log exploration should be surfaced.

**Must have (table stakes):**
- Plugin discovery and lifecycle (load/unload with error isolation)
- Semantic versioning with compatibility checking
- Full-text log search with time range and field-based filtering
- Basic aggregation (count by time window, group by field, top-N queries)
- Progressive disclosure navigation (overview → aggregated → detail, max 3 levels)
- Clear MCP tool descriptions with JSON Schema inputs
- Breadcrumb navigation with state preservation

**Should have (competitive differentiators):**
- Automatic log template mining (extract patterns without manual config)
- Category-based tool loading (load tool groups on demand, not all upfront)
- High-cardinality field search (fast search on trace_id despite millions of unique values)
- Smart defaults with SLO-first views
- MCP Resources for context (expose docs/schemas as resources, not tools)

**Defer (v2+):**
- Novelty detection (time window comparison of patterns — requires baseline period)
- Anomaly scoring (rank logs by unusualness — complex ML implementation)
- Plugin marketplace/registry (centralized discovery — unnecessary for MVP)
- Hot reload without restart (advanced, can iterate to this)
- Network-based plugin discovery (adds deployment complexity without clear demand)

### Architecture Approach

The architecture uses interface-based plugin registration (compile-time, not runtime .so loading) with a pipeline processing pattern for log ingestion. Plugins implement a standard interface and register themselves in a compile-time registry. Log processing follows a staged pipeline with bounded channels for backpressure: ingestion → normalization → template mining → structuring → batching → VictoriaLogs storage.

**Major components:**
1. **Plugin Manager** (`internal/mcp/plugins/`) — maintains registry of plugins, reads config to enable/disable, handles lifecycle (init/reload/shutdown), registers tools with MCP server
2. **VictoriaLogs Plugin** (`internal/mcp/plugins/victorialogs/`) — implements Plugin interface, manages log processing pipeline, exposes MCP tools for querying, handles template persistence
3. **Log Processing Pipeline** (`pipeline/`) — chain of stages with buffered channels: normalize → mine → structure → batch → write; backpressure via bounded channels with drop-oldest policy
4. **Template Miner** (`miner/`) — Drain algorithm implementation, builds prefix tree by token count and first token, similarity scoring for matches, WAL persistence with snapshots
5. **Configuration Hot-Reload** (`internal/config/watcher.go`) — fsnotify-based file watching, debouncing, SIGHUP signal handling, atomic pointer swap for race-free updates
6. **VictoriaLogs Client** (`client/`) — HTTP wrapper for /insert/jsonline endpoint, NDJSON serialization, retry with backoff, circuit breaker

**Key patterns to follow:**
- Interface-based plugin registration (not runtime .so loading)
- Pipeline stages with bounded channels (prevents memory exhaustion)
- Drain-inspired template mining (O(log n) matching vs O(n) regex list)
- Atomic pointer swap for config reload (prevents torn reads)
- Template cache with WAL persistence (fast reads, durability across restarts)

### Critical Pitfalls

Research identified five critical pitfalls that cause rewrites or major production issues, plus several moderate pitfalls that cause delays.

1. **Go Stdlib Plugin Versioning Hell** — Using stdlib `plugin` package creates brittle deployment where plugins crash with version mismatches. All plugins and host must be built with exact same Go toolchain, dependency versions, GOPATH, and build flags. Prevention: Use HashiCorp go-plugin (RPC-based, process isolation, production-proven).

2. **Template Mining Instability with Variable-Starting Logs** — Drain fails when log messages start with variables instead of constants (e.g., "cupsd shutdown succeeded" vs "irqbalance shutdown succeeded" create separate templates instead of one). Causes template explosion, accuracy drops from 90% to <70%. Prevention: Pre-tokenize with masking (replace known variable patterns before feeding to Drain), use Drain3 with built-in masking, monitor template growth metrics.

3. **Race Conditions in Config Hot-Reload** — Using sync.RWMutex with in-place field updates creates torn reads where goroutines see partial config state (old URL with new API key). Prevention: Use atomic.Value pointer swap pattern — validate entire config, then single atomic swap (readers see old OR new, never partial).

4. **Template Drift Without Rebalancing** — Log formats evolve over time (syntactic drift), causing accuracy degradation and template explosion after 30-60 days. Prevention: Use Drain3 HELP implementation with iterative rebalancing, implement template TTL (expire templates not seen in 30d), monitor templates-per-1000-logs ratio.

5. **UI State Loss During Progressive Disclosure** — Component-local state resets on navigation, browser back button doesn't restore context. Prevention: Encode state in URL query params from day 1 (hard to retrofit), use React Router with location.state, implement breadcrumb navigation with clickable links.

**Moderate pitfalls:**
- MCP protocol version mismatch without graceful degradation (support multiple protocol versions)
- Cross-client template inconsistency (canonical storage in MCP server, deterministic IDs)
- VictoriaLogs live tailing without rate limiting (minimum 1s refresh, warn at >1K logs/sec)
- No config validation before hot-reload (validate and health-check before swap)

## Implications for Roadmap

Based on research, suggested phase structure follows dependency order identified in architecture patterns: plugin foundation must exist before integrations, log processing depends on VictoriaLogs client, template mining can be iterative, UI comes last.

### Phase 1: Plugin Infrastructure Foundation
**Rationale:** Plugin architecture is the foundation for all integrations. Must be correct from day 1 because changing plugin system later (e.g., stdlib plugin to go-plugin) forces complete rewrite.

**Delivers:**
- Plugin interface definition and registry
- Config loader extension for integrations.yaml
- Atomic config hot-reload with fsnotify
- Existing Kubernetes tools migrated to plugin pattern

**Addresses (from FEATURES.md):**
- Plugin discovery and lifecycle (table stakes)
- Semantic versioning with compatibility checking (table stakes)
- Config hot-reload (competitive differentiator)

**Avoids (from PITFALLS.md):**
- CRITICAL-1: Uses HashiCorp go-plugin, not stdlib plugin
- CRITICAL-3: Implements atomic pointer swap for config reload from start

**Stack elements:** Koanf v2.3.0 + providers, fsnotify (transitive), HashiCorp go-plugin v1.7.0

**Research flags:** Standard patterns, skip additional research. Well-documented in go-plugin and Koanf documentation.

### Phase 2: VictoriaLogs Client & Basic Pipeline
**Rationale:** Establish reliable external integration before adding complexity of template mining. Validates that log pipeline architecture works with real VictoriaLogs instance.

**Delivers:**
- HTTP client for /insert/jsonline endpoint
- Pipeline stages (normalize, batch, write)
- Kubernetes event ingestion
- Basic VictoriaLogs plugin registration
- Backpressure with bounded channels

**Addresses (from FEATURES.md):**
- Log ingestion and storage (prerequisite for query tools)
- Backpressure handling (reliability)

**Avoids (from PITFALLS.md):**
- MODERATE-4: Implements rate limiting for potential live tail
- MINOR-4: Uses correct VictoriaLogs time filter patterns

**Stack elements:** net/http (stdlib), existing Kubernetes event stream

**Research flags:** Standard patterns, skip additional research. VictoriaLogs API is well-documented.

### Phase 3: Log Template Mining
**Rationale:** Template mining is complex and can be iterated on. Start with basic Drain implementation, validate with production log samples, iterate on masking and rebalancing based on real data.

**Delivers:**
- Drain algorithm implementation for template extraction
- Template cache with in-memory storage
- Template persistence (WAL + snapshots)
- Integration with log pipeline
- Template metadata in VictoriaLogs logs

**Addresses (from FEATURES.md):**
- Automatic template mining (competitive differentiator)
- Pattern detection without manual config

**Avoids (from PITFALLS.md):**
- CRITICAL-2: Pre-tokenization with masking for variable-starting logs
- CRITICAL-4: Periodic rebalancing mechanism (use Drain3 HELP if available, or implement TTL)
- MINOR-3: Order normalization rules correctly (IPv6 before UUID)

**Stack elements:** LoggingDrain library (or custom implementation)

**Research flags:** NEEDS DEEPER RESEARCH during phase planning. Drain algorithm parameters (similarity threshold, tree depth, max clusters) need tuning based on actual log patterns. Recommend `/gsd:research-phase` to:
- Sample production logs from target namespaces
- Validate template count is reasonable (<1000 for typical app)
- Tune similarity threshold (0.3-0.6 range)
- Test masking patterns with edge cases

### Phase 4: MCP Query Tools
**Rationale:** Query tools depend on both VictoriaLogs client (Phase 2) and template mining (Phase 3). This phase exposes functionality to AI assistants via MCP.

**Delivers:**
- `query_logs` tool with LogsQL integration
- `analyze_log_patterns` tool using template data
- VictoriaLogs plugin full registration
- Tool descriptions and JSON schemas
- MCP Resources for VictoriaLogs schema docs

**Addresses (from FEATURES.md):**
- Full-text search with time range filtering (table stakes)
- Field-based filtering, aggregation (table stakes)
- High-cardinality field search (differentiator)
- MCP Resources for context (differentiator)

**Avoids (from PITFALLS.md):**
- MODERATE-1: Multi-version MCP protocol support
- Tool count minimization (10-20 tools, per MCP best practices)

**Stack elements:** Existing mark3labs/mcp-go, VictoriaLogs client from Phase 2, templates from Phase 3

**Research flags:** Standard MCP patterns, skip additional research. Mark3labs/mcp-go provides clear tool registration API.

### Phase 5: Progressive Disclosure UI
**Rationale:** UI comes last because it depends on query tools (Phase 4) and benefits from real template data. Can iterate on UX based on actual usage patterns.

**Delivers:**
- Three-level drill-down (global → aggregated → detail)
- URL-based state management
- Breadcrumb navigation
- Collapsible sections for details
- Smart defaults (SLO-first view)

**Addresses (from FEATURES.md):**
- Progressive disclosure navigation (table stakes)
- State preservation (table stakes)
- Smart defaults with SLO-first views (differentiator)

**Avoids (from PITFALLS.md):**
- CRITICAL-5: URL-based state from day 1 (hard to retrofit)
- MINOR-2: Limit to 3 levels maximum (global → aggregated → detail)
- MODERATE-5: Preserve context during drill-down

**Stack elements:** Existing React frontend, React Router

**Research flags:** Standard React patterns, skip additional research. Established SPA state management patterns.

### Phase 6: Template Consistency & Monitoring (Optional)
**Rationale:** Cross-client consistency and drift monitoring are operational excellence features. Can defer if MVP targets single client or if template drift isn't observed in practice.

**Delivers:**
- Canonical template storage in MCP server
- Deterministic template IDs (hash-based)
- Template drift detection metrics
- Template growth monitoring
- Health check endpoints

**Addresses (from FEATURES.md):**
- Cross-client consistency (nice-to-have)
- Template drift detection (operational excellence)

**Avoids (from PITFALLS.md):**
- MODERATE-2: Ensures same template IDs across clients
- Template growth monitoring (early warning for drift)

**Research flags:** Standard patterns, skip additional research.

### Phase Ordering Rationale

- **Sequential dependency chain:** Plugin infrastructure (1) → VictoriaLogs client (2) → Template mining (3) → Query tools (4) → UI (5)
- **Risk-first approach:** Critical decisions (plugin system choice, config reload pattern) in Phase 1 where changes are cheapest
- **Iterative complexity:** Start simple (basic pipeline in Phase 2), add complexity (template mining in Phase 3), iterate on UX (Phase 5)
- **Validation points:** Each phase delivers independently testable functionality (Phase 2 validates VictoriaLogs integration before adding template mining complexity)
- **Pitfall avoidance:** Phase 1 prevents CRITICAL-1 (plugin system) and CRITICAL-3 (config reload), Phase 3 prevents CRITICAL-2 and CRITICAL-4 (template mining), Phase 5 prevents CRITICAL-5 (UI state)

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3 (Template Mining):** Complex algorithm with production-sensitive tuning. Needs `/gsd:research-phase` to sample real logs, validate template count, tune parameters (similarity threshold, tree depth, masking patterns). Research questions: What's the typical template count for our log patterns? What similarity threshold prevents explosion? Which fields need masking?

Phases with standard patterns (skip research-phase):
- **Phase 1 (Plugin Infrastructure):** Well-documented in go-plugin and Koanf documentation, established patterns
- **Phase 2 (VictoriaLogs Client):** VictoriaLogs HTTP API is well-documented, standard Go HTTP client patterns
- **Phase 4 (MCP Query Tools):** Mark3labs/mcp-go provides clear API, existing MCP tools in codebase as reference
- **Phase 5 (Progressive Disclosure UI):** Standard React/SPA patterns, URL state management well-established

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | HashiCorp go-plugin (4+ years production), Koanf (stable v2), VictoriaLogs (official docs). Only MEDIUM: LoggingDrain library (small community, but algorithm is proven). |
| Features | HIGH | MCP patterns from 2026 best practices, progressive disclosure from UX research, log exploration features from VictoriaLogs docs and competitor analysis. MEDIUM: VictoriaLogs-specific query capabilities (not all features detailed in web search). |
| Architecture | HIGH | Existing codebase analysis provides foundation, external patterns verified with production examples (pipeline stages, Drain algorithm, atomic swap pattern). Interface-based plugin registry is idiomatic Go. |
| Pitfalls | MEDIUM-HIGH | Critical pitfalls verified with official sources (Go issue tracker for stdlib plugin, academic papers for Drain limitations, Go docs for atomic operations). MEDIUM: Progressive disclosure pitfalls (UX research from web only). |

**Overall confidence:** HIGH

Research covers all critical decisions with high-confidence sources. The one MEDIUM component (LoggingDrain library) has clear mitigation (re-implement algorithm if needed). Recommended phase order follows verified dependency patterns.

### Gaps to Address

**LoggingDrain library maturity (MEDIUM confidence):** Small community (16 stars), recent but limited production reports. Mitigation: Phase 3 should include spike to validate library works as expected. If bugs found, Drain algorithm is simple enough to implement in-house (200-300 LOC for core logic per research).

**VictoriaLogs query syntax details (MEDIUM confidence):** Web search provided high-level capabilities, but full LogsQL syntax not exhaustively documented in search results. Mitigation: Consult VictoriaLogs API documentation directly during Phase 4 implementation. No blocking risk — basic query patterns are well-documented.

**Template mining parameter tuning (production-dependent):** Optimal values for similarity threshold, tree depth, and max clusters depend on actual log patterns in target environment. Mitigation: Phase 3 planning should include `/gsd:research-phase` to sample production logs and validate parameters. Research identified ranges (similarity 0.3-0.6, depth 4-6) but exact values need empirical testing.

**Cross-client template consistency requirements (unclear):** Research identified the risk, but MVP scope doesn't clarify if multiple clients will access templates simultaneously. Mitigation: Phase 6 is marked optional, can prioritize based on actual multi-client usage patterns observed in Phases 4-5.

## Sources

### Primary (HIGH confidence)
- [HashiCorp go-plugin v1.7.0 on Go Packages](https://pkg.go.dev/github.com/hashicorp/go-plugin) — plugin architecture
- [Koanf v2.3.0 GitHub releases](https://github.com/knadh/koanf/releases) — config management
- [VictoriaLogs Official Documentation](https://docs.victoriametrics.com/victorialogs/) — log storage and querying
- [Drain3 algorithm](https://github.com/logpai/Drain3) — template mining
- [Go issue tracker (#27751, #31354)](https://github.com/golang/go/issues) — stdlib plugin limitations
- [MCP Protocol Specification](https://modelcontextprotocol.io/specification/) — MCP patterns
- [Semantic Versioning 2.0.0](https://semver.org/) — versioning
- [Nielsen Norman Group - Progressive Disclosure](https://www.nngroup.com/articles/progressive-disclosure/) — UX patterns

### Secondary (MEDIUM confidence)
- [Klavis - MCP Design Patterns](https://www.klavis.ai/blog/less-is-more-mcp-design-patterns-for-ai-agents) — tool count guidance
- [LoggingDrain GitHub](https://github.com/PalanQu/LoggingDrain) — Go implementation
- [Viper vs Koanf comparison](https://itnext.io/golang-configuration-management-library-viper-vs-koanf-eea60a652a22) — config library trade-offs
- [Investigating and Improving Log Parsing in Practice](https://yanmeng.github.io/papers/FSE221.pdf) — template mining pitfalls
- [Adaptive Log Anomaly Detection through Drift](https://openreview.net/pdf?id=6QXrawkcrX) — template drift research
- [React State Management 2025](https://www.developerway.com/posts/react-state-management-2025) — SPA state patterns

### Tertiary (LOW confidence)
- Various blog posts and Medium articles — supporting evidence for best practices, cross-validated with official sources

---
*Research completed: 2026-01-20*
*Ready for roadmap: yes*
