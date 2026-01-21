# Project State: Spectre MCP Plugin System + VictoriaLogs Integration

**Last updated:** 2026-01-21

## Project Reference

**Core Value:** Enable AI assistants to explore logs progressively—starting from high-level signals, drilling into patterns, and viewing raw logs only when context is narrow.

**Current Focus:** Phase 1 complete. Ready to plan Phase 2 (Config Management & UI).

## Current Position

**Phase:** 3 - VictoriaLogs Client & Basic Pipeline
**Plan:** 1 of 3 (03-01-PLAN.md - just completed)
**Status:** In Progress
**Progress:** 12/31 requirements
**Last activity:** 2026-01-21 - Completed 03-01-PLAN.md (VictoriaLogs Client & Query Builder)

```
[██████████] 100% Phase 1 (Complete ✓)
[██████████] 100% Phase 2 (Complete ✓)
[███▓░░░░░░] 33% Phase 3 (1/3 plans complete)
[██████░░░░] 39% Overall (12/31 requirements)
```

## Performance Metrics

| Metric | Current | Target | Status |
|--------|---------|--------|--------|
| Requirements Complete | 12/31 | 31/31 | In Progress |
| Phases Complete | 2/5 | 5/5 | In Progress |
| Plans Complete | 8/10 | 10/10 (Phases 1-3) | Phase 3 in progress |
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

**Phase 3: VictoriaLogs Client & Basic Pipeline** (In Progress)
- 03-01: VictoriaLogs HTTP client with LogsQL query builder ✓

### Active Todos

- [ ] Implement log ingestion pipeline with backpressure handling (Plan 03-02)
- [ ] Wire VictoriaLogs integration with client and pipeline (Plan 03-03)

### Known Blockers

None currently.

### Research Flags

**Phase 4 (Log Template Mining):** NEEDS DEEPER RESEARCH during planning
- Sample production logs to validate template count is reasonable (<1000 for typical app)
- Tune Drain parameters: similarity threshold (0.3-0.6 range), tree depth (4-6), max clusters
- Test masking patterns with edge cases (variable-starting logs)

**Other phases:** Standard patterns, skip additional research.

## Session Continuity

**Last session:** 2026-01-21
**Stopped at:** Completed 03-01-PLAN.md (VictoriaLogs Client & Query Builder)

**What just happened:**
- Executed plan 03-01: VictoriaLogs HTTP client with LogsQL query builder
- Created types.go with QueryParams, TimeRange, LogEntry, and response types
- Implemented query.go with BuildLogsQLQuery, BuildHistogramQuery, BuildAggregationQuery
- Implemented client.go with QueryLogs, QueryHistogram, QueryAggregation, IngestBatch methods
- Tuned HTTP transport settings (MaxIdleConnsPerHost: 10) for production workloads
- Ensured connection reuse pattern (io.ReadAll before close) in all methods
- All tasks completed in 3 minutes with no deviations
- SUMMARY: .planning/phases/03-victorialogs-client-pipeline/03-01-SUMMARY.md

**What's next:**
- Phase 3 in progress (1 of 3 plans complete)
- Next: Plan 03-02 (Pipeline with Backpressure)
- Next: Execute `/gsd:execute-phase 3 --plan 2` when ready

**Context for next agent:**
- HTTP client foundation complete with four operations (query, histogram, aggregation, ingestion)
- Query builder uses structured parameters (no raw LogsQL exposure)
- Connection pooling tuned for high-throughput queries
- IngestBatch method ready for pipeline integration (Plan 03-02)
- All error responses include VictoriaLogs details for debugging

---

*State initialized: 2026-01-21*
*Phase 1 completed: 2026-01-21*
