# Project State: Spectre MCP Plugin System + VictoriaLogs Integration

**Last updated:** 2026-01-20

## Project Reference

**Core Value:** Enable AI assistants to explore logs progressively—starting from high-level signals, drilling into patterns, and viewing raw logs only when context is narrow.

**Current Focus:** Phase 1 (Plugin Infrastructure Foundation) - executing plans to build integration system.

## Current Position

**Phase:** 1 of 5 (Plugin Infrastructure Foundation)
**Plan:** 4 of 4 complete
**Status:** Phase complete
**Last activity:** 2026-01-21 - Completed 01-04-PLAN.md

**Progress:**
```
[██████████] 100% Phase 1 (4/4 plans) ✓ COMPLETE
[████░░░░░░] 50% Overall (4/8 plans across all phases)
```

## Performance Metrics

| Metric | Current | Target | Status |
|--------|---------|--------|--------|
| Requirements Complete | ~6/31 | 31/31 | In Progress |
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
| Atomic pointer swap pattern for race-free config reload | Roadmap | Planned for config loader implementation |
| Log processing package is integration-agnostic | Roadmap | Reusable beyond VictoriaLogs |
| Template mining uses Drain algorithm with pre-tokenization masking | Roadmap | Standard approach for log template extraction |

**Scope Boundaries:**
- Progressive disclosure: 3 levels maximum (global → aggregated → detail)
- Novelty detection: compare to previous time window (not long-term baseline)
- MCP tools: 10-20 maximum (context window constraints)
- VictoriaLogs: no authentication (just base URL)

### Active Todos

- [x] Design integration interface contract for tool registration (01-01 complete)
- [x] Implement factory registry for in-tree integration discovery (01-02 complete)
- [x] Implement integration instance registry (01-02 complete)
- [x] Implement config loader with Koanf (01-02 complete)
- [x] Implement config file watcher with debouncing (01-03 complete)
- [x] Implement integration lifecycle manager with version validation (01-04 complete)
- [x] **Phase 1 complete** - Plugin Infrastructure Foundation ready for VictoriaLogs integration
- [ ] Begin Phase 2 (VictoriaLogs Foundation)

### Known Blockers

None currently.

### Research Flags

**Phase 4 (Log Template Mining):** NEEDS DEEPER RESEARCH during planning
- Sample production logs to validate template count is reasonable (<1000 for typical app)
- Tune Drain parameters: similarity threshold (0.3-0.6 range), tree depth (4-6), max clusters
- Test masking patterns with edge cases (variable-starting logs)

**Other phases:** Standard patterns, skip additional research.

## Session Continuity

**Last session:** 2026-01-21T01:04:49Z
**Stopped at:** Completed 01-04-PLAN.md - **PHASE 1 COMPLETE**
**Resume file:** None

**What just happened:**
- Plan 01-04 executed successfully (2 tasks, 2 commits, 5 min duration)
- Integration lifecycle manager with version validation (PLUG-06) using semantic versioning
- Health monitoring with auto-recovery every 30s for degraded instances
- Hot-reload via IntegrationWatcher callback triggers full instance restart with re-validation
- Graceful shutdown with configurable timeout (default 10s per instance)
- Server command integration with --integrations-config and --min-integration-version flags
- Comprehensive test suite (6 tests) covering version validation, degraded handling, reload, recovery, shutdown
- Four auto-fixes: missing go-version dependency (blocking), import cycle (blocking), test name collision (bug), test timing (bug)

**Phase 1 Complete:**
All 4 plans executed successfully:
- 01-01: Integration interface and contract (PLUG-01, PLUG-02, PLUG-03)
- 01-02: Factory registry, instance registry, config loader with Koanf
- 01-03: Config file watcher with debouncing (fsnotify)
- 01-04: Integration lifecycle manager with version validation (PLUG-06)

**What's next:**
- Begin Phase 2: VictoriaLogs Foundation
- Will implement concrete VictoriaLogs integration using Phase 1 infrastructure
- VictoriaLogs factory will register via RegisterFactory(), manager will orchestrate lifecycle

**Context for next agent:**
- Manager validates integration versions on startup using semantic versioning (PLUG-06)
- Failed instance start marked as degraded, server continues with other instances (resilience)
- Health checks auto-recover degraded instances every 30s
- Config reload triggers full restart with re-validation (not partial reload)
- Manager registered as lifecycle component with no dependencies
- Integration infrastructure is complete and tested - ready for concrete integrations

---

*State initialized: 2026-01-21*
*Last updated: 2026-01-20T23:51:48Z*
