# Project State: Spectre MCP Plugin System + VictoriaLogs Integration

**Last updated:** 2026-01-20

## Project Reference

**Core Value:** Enable AI assistants to explore logs progressively—starting from high-level signals, drilling into patterns, and viewing raw logs only when context is narrow.

**Current Focus:** Phase 1 (Plugin Infrastructure Foundation) - executing plans to build integration system.

## Current Position

**Phase:** 1 of 5 (Plugin Infrastructure Foundation)
**Plan:** 3 of 4 complete
**Status:** In progress
**Last activity:** 2026-01-20 - Completed 01-03-PLAN.md

**Progress:**
```
[███████░░░] 75% Phase 1 (3/4 plans)
[███░░░░░░░] 38% Overall (3/8 plans across all phases)
```

## Performance Metrics

| Metric | Current | Target | Status |
|--------|---------|--------|--------|
| Requirements Complete | ~6/31 | 31/31 | In Progress |
| Phases Complete | 0/5 | 5/5 | In Progress |
| Plans Complete | 3/4 | 4/4 (Phase 1) | In Progress |
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
- [ ] Complete Phase 1 plans (1 remaining: 01-04)

### Known Blockers

None currently.

### Research Flags

**Phase 4 (Log Template Mining):** NEEDS DEEPER RESEARCH during planning
- Sample production logs to validate template count is reasonable (<1000 for typical app)
- Tune Drain parameters: similarity threshold (0.3-0.6 range), tree depth (4-6), max clusters
- Test masking patterns with edge cases (variable-starting logs)

**Other phases:** Standard patterns, skip additional research.

## Session Continuity

**Last session:** 2026-01-20T23:57:30Z
**Stopped at:** Completed 01-03-PLAN.md
**Resume file:** None

**What just happened:**
- Plan 01-03 executed successfully (2 tasks, 2 commits, 3 min duration)
- IntegrationWatcher with fsnotify for file change detection
- Debouncing (500ms default) coalesces rapid file changes into single reload
- ReloadCallback pattern for notifying on validated config changes
- Graceful Start/Stop lifecycle with context cancellation and 5s timeout
- Invalid configs logged but don't crash watcher (resilience after initial load)
- Comprehensive test suite (8 tests) with no race conditions
- Two auto-fixes: unused koanf import/field (blocking) and WatcherConfig naming conflict (blocking)

**What's next:**
- Execute Plan 01-04: Integration Manager (orchestrates lifecycle of all integration instances)
- This is the final plan for Phase 1 - will tie together interface, registries, config loader, and watcher

**Context for next agent:**
- IntegrationWatcher provides foundation for hot-reload - use ReloadCallback to orchestrate instance restarts
- Watcher is resilient: invalid configs after initial load are logged but don't crash the system
- 500ms debounce is already tuned - don't change without good reason
- IntegrationWatcherConfig naming avoids conflict with K8s WatcherConfig in same package
- Factory registry, instance registry, config loader, and watcher are all independent - manager will coordinate them
- Degraded health state is key design feature - preserve resilience pattern in manager implementation

---

*State initialized: 2026-01-21*
*Last updated: 2026-01-20T23:51:48Z*
