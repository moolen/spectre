# Project State: Spectre MCP Plugin System + VictoriaLogs Integration

**Last updated:** 2026-01-21

## Project Reference

**Core Value:** Enable AI assistants to explore logs progressively—starting from high-level signals, drilling into patterns, and viewing raw logs only when context is narrow.

**Current Focus:** Phase 1 complete. Ready to plan Phase 2 (Config Management & UI).

## Current Position

**Phase:** 4 - Log Template Mining (Verified ✓)
**Plan:** 4 of 4 (04-04-PLAN.md complete)
**Status:** Phase Verified
**Progress:** 21/31 requirements
**Last activity:** 2026-01-21 - Completed 04-04-PLAN.md (Template Lifecycle & Testing)

```
[██████████] 100% Phase 1 (Complete ✓)
[██████████] 100% Phase 2 (Complete ✓)
[██████████] 100% Phase 3 (Verified ✓)
[██████████] 100% Phase 4 (Verified ✓)
[░░░░░░░░░░]   0% Phase 5 (Not Started)
[██████████]  68% Overall (21/31 requirements)
```

## Performance Metrics

| Metric | Current | Target | Status |
|--------|---------|--------|--------|
| Requirements Complete | 21/31 | 31/31 | In Progress |
| Phases Complete | 4/5 | 5/5 | In Progress |
| Plans Complete | 15/15 | 15/15 (Phases 1-4) | Phases 1-4 Complete ✓ |
| Blockers | 0 | 0 | On Track |

## Accumulated Context

### Key Decisions

| Decision | Plan | Rationale |
|----------|------|-----------|
| Integrations are in-tree (compiled into Spectre), not external plugins | 01-01 | Simplifies deployment, eliminates version compatibility issues |
| Multiple instances of same integration type supported | 01-01 | Allows multiple VictoriaLogs instances (prod, staging) with different configs |
| Failed connections mark instance as Degraded, not crash server | 01-01 | Resilience - one integration failure doesn't bring down entire server |
| Config schema versioning starting with v1 | 01-01 | Enables in-memory migration for future config format changes |
| ToolRegistry placeholder interface | 01-01 | Avoids premature coupling - concrete implementation in Plan 02 |
| Context-based lifecycle methods | 01-01 | Start/Stop/Health use context.Context for cancellation and timeouts |
| Koanf v2.3.0 for config hot-reload | 01-01 | Superior to Viper (modular, ESM-native, fixes case-sensitivity bugs) |
| Factory registry uses global default instance with package-level functions | 01-02 | Simplifies integration registration - no registry instance management needed |
| Koanf v2 requires UnmarshalWithConf with Tag: "yaml" | 01-02 | Default Unmarshal doesn't respect yaml struct tags - fields come back empty |
| Both registries use sync.RWMutex for thread safety | 01-02 | Concurrent reads (Get/List) while ensuring safe writes (Register) |
| Registry.Register errors on duplicate names and empty strings | 01-02 | Prevents ambiguity in instance lookup and invalid identifiers |
| IntegrationWatcherConfig naming to avoid conflict with K8s WatcherConfig | 01-03 | Maintains clear separation between integration and K8s resource watching |
| 500ms default debounce prevents editor save storms | 01-03 | Multiple rapid file changes coalesced into single reload |
| fsnotify directly instead of Koanf file provider | 01-03 | Better control over event handling, debouncing, and error resilience |
| Invalid configs after initial load logged but don't crash watcher | 01-03 | Resilience - one bad edit doesn't break system. Initial load still fails fast |
| Manager validates integration versions on startup (PLUG-06) | 01-04 | Semantic version comparison using hashicorp/go-version |
| Failed instance start marked as degraded, not crash server | 01-04 | Resilience pattern - server continues with other instances |
| Health checks auto-recover degraded instances | 01-04 | Every 30s (configurable), calls Start() for degraded instances |
| Config reload triggers full restart with re-validation | 01-04 | Stop all → clear registry → re-validate versions → start new |
| Manager registered as lifecycle component | 01-04 | No dependencies, follows existing lifecycle.Manager pattern |
| Atomic writes prevent config corruption on crashes | 02-01 | Temp-file-then-rename ensures readers never see partial writes (POSIX atomicity) |
| Health status enriched from manager registry in real-time | 02-01 | Config file only has static data - runtime status from registry.Get().Health() |
| Test endpoint validates and attempts connection with 5s timeout | 02-01 | UI "Test Connection" needs to validate config without persisting |
| Panic recovery in test endpoint | 02-01 | Malformed configs might panic - catch with recover() and return error message |
| Path parameters extracted with strings.TrimPrefix | 02-01 | Codebase uses stdlib http.ServeMux - follow existing patterns |
| Default --integrations-config to "integrations.yaml" with auto-create | 02-03 | Better UX - no manual file creation required, server starts immediately |
| Static file handler excludes /api/* paths | 02-03 | Prevents API route conflicts - static handler returns early for /api/* |
| /api/config/integrations/test endpoint for unsaved integrations | 02-03 | Test connection before saving to config file |
| VictoriaLogs integration placeholder for UI testing | 02-03 | Enables end-to-end testing, full implementation in Phase 3 |
| Health status 'not_started' displayed as gray 'Unknown' | 02-03 | Better UX - clearer than technical state name |
| Helm chart supports extraVolumeMounts and extraArgs | 02-03 | Production deployments need to mount config as ConfigMap |
| IntegrationModal uses React portal for rendering at document.body | 02-02 | Proper z-index stacking, avoids parent container constraints |
| Focus trap cycles Tab between focusable elements in modal | 02-02 | Accessibility - keyboard navigation stays within modal context |
| Delete button only in edit mode with confirmation dialog | 02-02 | Prevents accidental deletes, clear separation add vs edit modes |
| Test Connection allows save even if test fails | 02-02 | Supports pre-staging - user can configure before target is reachable |
| Empty state shows tiles, table replaces tiles when data exists | 02-02 | Progressive disclosure - simple empty state, functional table when needed |
| Name field disabled in edit mode | 02-02 | Name is immutable identifier - prevents breaking references |
| Inline CSS-in-JS following Sidebar.tsx patterns | 02-02 | Consistent with existing codebase styling approach |
| LogsQL exact match operator is := not = | 03-01 | VictoriaLogs LogsQL syntax for precise field matching |
| Always include _time filter in queries | 03-01 | Prevents full history scans - default to last 1 hour when unspecified |
| Read response body to completion with io.ReadAll | 03-01 | Critical for HTTP connection reuse - even on error responses |
| MaxIdleConnsPerHost set to 10 (up from default 2) | 03-01 | Prevents connection churn under load for production workloads |
| Use RFC3339 for VictoriaLogs timestamps | 03-01 | ISO 8601-compliant time format for API requests |
| Empty field values omitted from LogsQL queries | 03-01 | Cleaner queries - only include non-empty filter parameters |
| Bounded channel with size 1000 provides natural backpressure | 03-02 | Blocking send when full prevents memory overflow without explicit flow control |
| No default case in Ingest select - intentional blocking | 03-02 | Prevents data loss (alternative would be to drop logs) |
| Batch size fixed at 100 entries | 03-02 | Consistent memory usage and reasonable HTTP payload size |
| 1-second flush ticker for partial batches | 03-02 | Prevents logs from stalling indefinitely while waiting for full batch |
| BatchesTotal counter tracks log count, not batch count | 03-02 | Increments by len(batch) for accurate throughput metrics |
| ConstLabels with instance name for metrics | 03-02 | Enables multiple VictoriaLogs pipeline instances with separate metrics |
| Pipeline errors logged and counted but don't crash | 03-02 | Temporary VictoriaLogs unavailability doesn't stop processing |
| Client, pipeline, metrics created in Start(), not constructor | 03-03 | Lifecycle pattern - heavy resources only created when integration starts |
| Failed connectivity test doesn't block startup | 03-03 | Degraded state with auto-recovery via health checks |
| 30-second query timeout for VictoriaLogs client | 03-03 | Balance between slow LogsQL queries and user patience |
| ValidateMinimumDuration skips validation for zero time ranges | 03-04 | Zero ranges use default 1-hour duration, validation not needed |
| BuildLogsQLQuery returns empty string on validation failure | 03-04 | Explicit failure clearer than logging/clamping; avoids silent behavior changes |
| 15-minute minimum time range hardcoded per VLOG-03 | 03-04 | Protects VictoriaLogs from excessive query load; no business need for configuration |
| DrainConfig uses sim_th=0.4, tree depth=4, maxChildren=100 | 04-01 | Research-recommended defaults for structured logs; balances clustering vs explosion |
| Templates scoped per-namespace with composite key | 04-01 | Multi-tenancy - same pattern in different namespaces has different semantics |
| SHA-256 hashing for template IDs | 04-01 | Deterministic, collision-resistant IDs for cross-client consistency (MINE-03) |
| Linear search for template lookup | 04-01 | Target <1000 templates per namespace; premature optimization unnecessary |
| JSON message field extraction with fallback order | 04-02 | Try message, msg, log, text, _raw, event - covers most frameworks while allowing structured event logs |
| Masking happens AFTER Drain clustering | 04-02 | Preserves Drain's structure detection before normalizing variables (user decision) |
| HTTP status codes preserved in templates | 04-02 | "returned 404" vs "returned 500" must stay distinct for debugging (user decision) |
| Kubernetes pod/replicaset names masked with <K8S_NAME> | 04-02 | Dynamic K8s resource names (deployment-replicaset-pod format) unified for stable templates |
| File path regex without word boundaries | 04-02 | Word boundaries don't work with slash separators; removed for correct full-path matching |
| Pattern normalization for stable template IDs | 04-03 | All placeholders (<IP>, <UUID>, <*>, etc.) normalized to <VAR> for ID generation; semantic patterns preserved for display |
| Per-namespace Drain instances in TemplateStore | 04-03 | Namespace isolation with separate clustering state; each namespace gets own DrainProcessor |
| Deep copy templates on retrieval | 04-03 | GetTemplate/ListTemplates return copies to prevent external mutation of internal state |
| Load errors don't crash server | 04-03 | Corrupted snapshots logged but server continues with empty state; resilience over strict validation |
| Failed snapshots don't stop periodic loop | 04-03 | Snapshot errors logged but don't halt persistence manager; lose max 5 minutes on crash (user decision) |
| Atomic writes for snapshots using temp-file-then-rename | 04-03 | POSIX atomicity prevents corruption; readers never see partial writes |
| Double-checked locking for namespace creation | 04-03 | Fast read path for existing namespaces, slow write path with recheck for thread-safe lazy initialization |
| Default rebalancing config: prune threshold 10, merge interval 5min, similarity 0.7 | 04-04 | Prune threshold catches rare but important patterns; 5min matches persistence; 0.7 for loose clustering per CONTEXT.md |
| Namespace lock protects entire Drain.Train() operation | 04-04 | Drain library not thread-safe; race condition fix - lock before Train() not after |
| Existing test suite organization kept as-is | 04-04 | Tests already comprehensive at 85.2% coverage; better organized than plan suggested (rebalancer_test.go vs store_test.go) |

**Scope Boundaries:**
- Progressive disclosure: 3 levels maximum (global → aggregated → detail)
- Novelty detection: compare to previous time window (not long-term baseline)
- MCP tools: 10-20 maximum (context window constraints)
- VictoriaLogs: no authentication (just base URL)

### Completed Phases

**Phase 1: Plugin Infrastructure Foundation** ✓
- 01-01: Integration interface and contract (PLUG-01, PLUG-02, PLUG-03)
- 01-02: Factory registry, instance registry, config loader with Koanf
- 01-03: Config file watcher with debouncing (fsnotify)
- 01-04: Integration lifecycle manager with version validation (PLUG-06)

**Phase 2: Config Management & UI** ✓
- 02-01: REST API for integration config CRUD with atomic writes (CONF-02)
- 02-02: React UI components for integration management (CONF-04, CONF-05)
- 02-03: Server integration and end-to-end verification

**Phase 3: VictoriaLogs Client & Basic Pipeline** ✓ (Verified)
- 03-01: VictoriaLogs HTTP client with LogsQL query builder
- 03-02: Backpressure-aware pipeline with batch processing and Prometheus metrics
- 03-03: Wire VictoriaLogs integration with client, pipeline, and metrics
- 03-04: Time range validation enforcing 15-minute minimum (gap closure for VLOG-03)

**Phase 4: Log Template Mining** ✓
- 04-01: Drain algorithm wrapper with configuration (MINE-01)
- 04-02: Log normalization and aggressive variable masking (MINE-02)
- 04-03: Namespace-scoped template storage with periodic persistence (MINE-03, MINE-04)
- 04-04: Template lifecycle management with pruning, auto-merge, and comprehensive testing (85.2% coverage)

### Active Todos

None - Phase 4 complete. Ready to plan Phase 5 (Progressive Disclosure MCP Tools).

### Known Blockers

None currently.

### Research Flags

**Phase 4 (Log Template Mining):** ✓ COMPLETE
- Research was performed during planning (04-RESEARCH.md)
- Drain parameters tuned: sim_th=0.4, tree depth=4, maxChildren=100
- Masking patterns tested with comprehensive test suite
- Template count management via pruning (threshold 10) and auto-merge (similarity 0.7)

**Phase 5 (Progressive Disclosure MCP Tools):** Standard patterns, skip additional research.

## Session Continuity

**Last session:** 2026-01-21
**Stopped at:** Completed 04-04-PLAN.md (Template Lifecycle & Testing)

**What just happened:**
- Executed plan 04-04: Template lifecycle management and comprehensive testing
- Created TemplateRebalancer with count-based pruning and similarity-based auto-merge
- Added levenshtein library for edit distance calculation in template similarity
- Fixed critical race condition: Drain library not thread-safe, moved lock before Train() call
- Achieved 85.2% test coverage across entire logprocessing package (exceeds 80% target)
- All tests pass with race detector enabled
- Phase 4 COMPLETE: Production-ready log template mining package
- All tasks completed in ~4 minutes
- SUMMARY: .planning/phases/04-log-template-mining/04-04-SUMMARY.md

**What's next:**
- Phase 4 COMPLETE (all 4 plans done)
- Ready to plan Phase 5: Progressive Disclosure MCP Tools
- Log processing foundation complete: Drain + storage + persistence + rebalancing
- Next phase will integrate template mining with VictoriaLogs and build MCP tools

**Context for next agent:**
- Complete log processing pipeline: PreProcess → Drain → AggressiveMask → Normalize → Store → Rebalance
- TemplateStore interface: Process(), GetTemplate(), ListTemplates(), GetNamespaces()
- PersistenceManager: 5-minute JSON snapshots with atomic writes
- TemplateRebalancer: 5-minute rebalancing with pruning (threshold 10) and auto-merge (similarity 0.7)
- Thread-safe with proper locking (race condition fixed)
- Test coverage: 85.2% with comprehensive test suite
- VictoriaLogs integration from Phase 3 ready for log source
- Integration framework from Phases 1-2 provides config management

---

*State initialized: 2026-01-21*
*Phase 1 completed: 2026-01-21*
