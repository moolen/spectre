# Project State: Spectre MCP Plugin System + VictoriaLogs Integration

**Last updated:** 2026-01-21

## Project Reference

**Core Value:** Enable AI assistants to explore logs progressively—starting from high-level signals, drilling into patterns, and viewing raw logs only when context is narrow.

**Current Focus:** Phase 1 complete. Ready to plan Phase 2 (Config Management & UI).

## Current Position

**Phase:** 2 - Config Management & UI
**Plan:** None (awaiting `/gsd:plan-phase 2`)
**Status:** Pending
**Progress:** 8/31 requirements

```
[██████████] 100% Phase 1 (Complete ✓)
[░░░░░░░░░░] 0% Phase 2
[██▓░░░░░░░] 26% Overall (8/31 requirements)
```

## Performance Metrics

| Metric | Current | Target | Status |
|--------|---------|--------|--------|
| Requirements Complete | 8/31 | 31/31 | In Progress |
| Phases Complete | 1/5 | 5/5 | In Progress |
| Plans Complete | 4/4 | 4/4 (Phase 1) | Phase 1 Complete ✓ |
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

### Active Todos

- [ ] Plan Phase 2: Config Management & UI
- [ ] Implement REST API for integration config CRUD
- [ ] Build UI for integration enable/disable and configuration

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
**Stopped at:** Phase 1 execution complete

**What just happened:**
- Executed all 4 plans in Phase 1 across 4 waves
- Phase goal verified: 20/20 must-haves confirmed in codebase
- VERIFICATION.md created with detailed evidence
- ROADMAP.md and STATE.md updated
- REQUIREMENTS.md updated (Phase 1 requirements marked Complete)

**What's next:**
- User runs `/gsd:discuss-phase 2` or `/gsd:plan-phase 2`
- Phase 2 builds REST API and UI for integration configuration
- Enables users to enable/disable and configure integrations via browser

**Context for next agent:**
- Phase 1 infrastructure is complete and verified
- Integration system is ready for concrete integrations (VictoriaLogs in Phase 3)
- Config hot-reload working via file watcher
- Manager orchestrates lifecycle with version validation and health monitoring

---

*State initialized: 2026-01-21*
*Phase 1 completed: 2026-01-21*
