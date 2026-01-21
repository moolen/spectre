# Project State: Spectre MCP Plugin System + VictoriaLogs Integration

**Last updated:** 2026-01-21

## Project Reference

**Core Value:** Enable AI assistants to explore logs progressively—starting from high-level signals, drilling into patterns, and viewing raw logs only when context is narrow.

**Current Focus:** Phase 1 complete. Ready to plan Phase 2 (Config Management & UI).

## Current Position

**Phase:** 2 - Config Management & UI
**Plan:** 3 of 3 (02-03-PLAN.md - just completed)
**Status:** Phase Complete ✓
**Progress:** 11/31 requirements
**Last activity:** 2026-01-21 - Completed Phase 2 (Config Management & UI)

```
[██████████] 100% Phase 1 (Complete ✓)
[██████████] 100% Phase 2 (Complete ✓)
[█████▓░░░░] 35% Overall (11/31 requirements)
```

## Performance Metrics

| Metric | Current | Target | Status |
|--------|---------|--------|--------|
| Requirements Complete | 11/31 | 31/31 | In Progress |
| Phases Complete | 2/5 | 5/5 | In Progress |
| Plans Complete | 7/7 | 7/7 (Phases 1-2) | Phases 1-2 Complete ✓ |
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

### Active Todos

- [ ] Plan Phase 3: VictoriaLogs Client & Basic Pipeline
- [ ] Implement VictoriaLogs HTTP client with LogsQL query support
- [ ] Build log ingestion pipeline with backpressure handling

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
**Stopped at:** Completed Phase 2 (Config Management & UI)

**What just happened:**
- Executed plan 02-03: Server integration and end-to-end verification
- Wired REST API handlers into server startup (pass configPath and integrationManager)
- Human verification discovered and approved 7 bug fixes
- Added VictoriaLogs integration placeholder for UI testing
- Set default --integrations-config to "integrations.yaml" with auto-create
- Fixed API routing conflict (static handler serving /api/* paths)
- Added /test endpoint for unsaved integration validation
- Added Helm chart extraVolumeMounts and extraArgs for production deployment
- All tasks completed in 1h 24min with 7 auto-fixed issues
- SUMMARY: .planning/phases/02-config-management-ui/02-03-SUMMARY.md

**What's next:**
- Phase 2 complete (all 3 plans done)
- Ready for Phase 3: VictoriaLogs Client & Basic Pipeline
- Next: Plan Phase 3 with `/gsd:plan-phase 3`

**Context for next agent:**
- End-to-end integration management system working and tested
- Hot-reload chain verified: API → file → watcher → manager
- VictoriaLogs placeholder demonstrates integration pattern
- Default config auto-creation reduces deployment friction
- Helm chart ready for production ConfigMap mounting

---

*State initialized: 2026-01-21*
*Phase 1 completed: 2026-01-21*
