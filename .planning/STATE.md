# Project State: Spectre MCP Plugin System + VictoriaLogs Integration

**Last updated:** 2026-01-21

## Project Reference

**Core Value:** Enable AI assistants to explore logs progressively—starting from high-level signals, drilling into patterns, and viewing raw logs only when context is narrow.

**Current Focus:** Phase 1 complete. Ready to plan Phase 2 (Config Management & UI).

## Current Position

**Phase:** 4 - Log Template Mining (In Progress)
**Plan:** 2 of 4 (04-02-PLAN.md complete)
**Status:** In Progress
**Progress:** 17/31 requirements
**Last activity:** 2026-01-21 - Completed 04-02-PLAN.md (Log Normalization & Variable Masking)

```
[██████████] 100% Phase 1 (Complete ✓)
[██████████] 100% Phase 2 (Complete ✓)
[██████████] 100% Phase 3 (Verified ✓)
[█████░░░░░]  50% Phase 4 (In Progress - 2/4 plans)
[█████████░]  55% Overall (17/31 requirements)
```

## Performance Metrics

| Metric | Current | Target | Status |
|--------|---------|--------|--------|
| Requirements Complete | 17/31 | 31/31 | In Progress |
| Phases Complete | 3/5 | 5/5 | In Progress |
| Plans Complete | 11/11 | 11/11 (Phases 1-3) | Phases 1-3 Verified ✓ |
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

### Active Todos

None - Phase 3 verified. Ready to plan Phase 4 (Log Template Mining) or Phase 5 (Progressive Disclosure MCP Tools).

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
**Stopped at:** Completed 04-01-PLAN.md (Drain Algorithm Foundation & Template Types)

**What just happened:**
- Executed gap closure plan 03-04: Enforced 15-minute minimum time range validation for VictoriaLogs queries
- Added ValidateMinimumDuration method to TimeRange type with error messages
- Added Duration helper method for time range calculations
- Created comprehensive test suite: types_test.go and query_test.go with 11 test cases
- Updated BuildLogsQLQuery to validate time ranges early and return empty string on failure
- All tests pass with 100% coverage of validation logic
- All tasks completed in ~2 minutes with no deviations
- Gap from 03-VERIFICATION.md closed: VLOG-03 requirement now fully satisfied
- Phase 3 complete (17/31 requirements, 55% overall progress)
- SUMMARY: .planning/phases/03-victorialogs-client-pipeline/03-04-SUMMARY.md

**What's next:**
- Phase 3 fully complete (all 4 plans executed successfully, including gap closure)
- Next: Plan Phase 4 (Log Template Mining) or Phase 5 (Progressive Disclosure MCP Tools)
- Options:
  - Phase 4: Drain algorithm, template pattern mining, mask detection
  - Phase 5: MCP tools for progressive disclosure (overview, patterns, logs)
  - Recommendation: Phase 5 first (delivers user value sooner), Phase 4 later (optimization)

**Context for next agent:**
- VictoriaLogs integration fully functional: client, pipeline, metrics all wired
- Time range validation protects VictoriaLogs from excessive query load (15-minute minimum enforced)
- Health checks return Healthy/Degraded/Stopped based on connectivity tests
- Prometheus metrics exposed: victorialogs_pipeline_queue_depth, victorialogs_pipeline_logs_total, victorialogs_pipeline_errors_total
- Integration framework from Phase 1 validates version compatibility
- Config management UI from Phase 2 allows runtime integration configuration
- Client provides QueryLogs, QueryHistogram, QueryAggregation for Phase 5 MCP tool implementation
- BuildLogsQLQuery validates all query parameters including time range constraints
- Pipeline ready for log ingestion (though no log source wired yet)

---

*State initialized: 2026-01-21*
*Phase 1 completed: 2026-01-21*
