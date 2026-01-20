# Project State: Spectre MCP Plugin System + VictoriaLogs Integration

**Last updated:** 2026-01-20

## Project Reference

**Core Value:** Enable AI assistants to explore logs progressively—starting from high-level signals, drilling into patterns, and viewing raw logs only when context is narrow.

**Current Focus:** Phase 1 (Plugin Infrastructure Foundation) - executing plans to build integration system.

## Current Position

**Phase:** 1 of 5 (Plugin Infrastructure Foundation)
**Plan:** 2 of 4 complete
**Status:** In progress
**Last activity:** 2026-01-20 - Completed 01-02-PLAN.md

**Progress:**
```
[█████░░░░░] 50% Phase 1 (2/4 plans)
[██░░░░░░░░] 25% Overall (2/8 plans across all phases)
```

## Performance Metrics

| Metric | Current | Target | Status |
|--------|---------|--------|--------|
| Requirements Complete | ~6/31 | 31/31 | In Progress |
| Phases Complete | 0/5 | 5/5 | In Progress |
| Plans Complete | 2/4 | 4/4 (Phase 1) | In Progress |
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
- [ ] Integrate with existing MCP server (01-03)
- [ ] Complete Phase 1 plans (2 remaining: 01-03, 01-04)

### Known Blockers

None currently.

### Research Flags

**Phase 4 (Log Template Mining):** NEEDS DEEPER RESEARCH during planning
- Sample production logs to validate template count is reasonable (<1000 for typical app)
- Tune Drain parameters: similarity threshold (0.3-0.6 range), tree depth (4-6), max clusters
- Test masking patterns with edge cases (variable-starting logs)

**Other phases:** Standard patterns, skip additional research.

## Session Continuity

**Last session:** 2026-01-20T23:51:48Z
**Stopped at:** Completed 01-02-PLAN.md
**Resume file:** None

**What just happened:**
- Plan 01-02 executed successfully (3 tasks, 3 commits, 4 min duration)
- Factory registry for compile-time integration type discovery (PLUG-01) with RegisterFactory/GetFactory
- Instance registry for runtime integration management with Register/Get/List/Remove
- Config loader using Koanf v2.3.0 to read and validate YAML integration files
- All tests passing including concurrent access verification
- Two auto-fixes: missing fmt import (bug) and Koanf UnmarshalWithConf for yaml tags (blocking)

**What's next:**
- Execute Plan 01-03: MCP server integration
- Execute Plan 01-04: (check plan file for details)

**Context for next agent:**
- Factory registry is global (defaultRegistry) - use RegisterFactory/GetFactory convenience functions
- Koanf v2 requires UnmarshalWithConf with Tag: "yaml" for struct tag support
- Both registries use sync.RWMutex - maintain thread-safe patterns
- Integration interface from 01-01 is stable - don't modify without careful consideration
- Config schema v1 is locked - future changes require migration support
- Degraded health state is key design feature - preserve resilience pattern

---

*State initialized: 2026-01-21*
*Last updated: 2026-01-20T23:51:48Z*
