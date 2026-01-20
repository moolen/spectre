# Project State: Spectre MCP Plugin System + VictoriaLogs Integration

**Last updated:** 2026-01-20

## Project Reference

**Core Value:** Enable AI assistants to explore logs progressively—starting from high-level signals, drilling into patterns, and viewing raw logs only when context is narrow.

**Current Focus:** Phase 1 (Plugin Infrastructure Foundation) - executing plans to build integration system.

## Current Position

**Phase:** 1 of 5 (Plugin Infrastructure Foundation)
**Plan:** 1 of 4 complete
**Status:** In progress
**Last activity:** 2026-01-20 - Completed 01-01-PLAN.md

**Progress:**
```
[██░░░░░░░░] 25% Phase 1 (1/4 plans)
[█░░░░░░░░░] 25% Overall (1/4 plans)
```

## Performance Metrics

| Metric | Current | Target | Status |
|--------|---------|--------|--------|
| Requirements Complete | ~3/31 | 31/31 | In Progress |
| Phases Complete | 0/5 | 5/5 | In Progress |
| Plans Complete | 1/4 | 4/4 (Phase 1) | In Progress |
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
- [ ] Implement integration manager with lifecycle orchestration (01-02)
- [ ] Implement config loader with Koanf hot-reload (01-02)
- [ ] Integrate with existing MCP server (01-03)
- [ ] Complete Phase 1 plans (3 remaining: 01-02, 01-03, 01-04)

### Known Blockers

None currently.

### Research Flags

**Phase 4 (Log Template Mining):** NEEDS DEEPER RESEARCH during planning
- Sample production logs to validate template count is reasonable (<1000 for typical app)
- Tune Drain parameters: similarity threshold (0.3-0.6 range), tree depth (4-6), max clusters
- Test masking patterns with edge cases (variable-starting logs)

**Other phases:** Standard patterns, skip additional research.

## Session Continuity

**Last session:** 2026-01-20T23:45:06Z
**Stopped at:** Completed 01-01-PLAN.md
**Resume file:** None

**What just happened:**
- Plan 01-01 executed successfully (3 tasks, 3 commits)
- Integration interface contract defined (Integration, IntegrationMetadata, HealthStatus, ToolRegistry)
- Config schema with versioning created (IntegrationsFile, IntegrationConfig, Validate())
- Koanf v2.3.0 added for config hot-reload capability
- All tests passing, no import cycles

**What's next:**
- Execute Plan 01-02: Integration manager with lifecycle orchestration + config loader
- Execute Plan 01-03: MCP server integration
- Execute Plan 01-04: (check plan file for details)

**Context for next agent:**
- Integration interface is stable - don't modify contract without careful consideration
- Config schema v1 is locked - future changes require migration support
- ToolRegistry is placeholder - concrete implementation in 01-02 or 01-03
- Koanf dependencies ready but not yet imported in loader code
- Degraded health state is key design feature - preserve resilience pattern

---

*State initialized: 2026-01-21*
*Last updated: 2026-01-20*
