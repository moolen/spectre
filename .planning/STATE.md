# GSD State: Spectre

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-22)

**Core value:** Enable AI assistants to explore logs from multiple backends through unified MCP interface
**Current focus:** Phase 12 - MCP Tools Overview and Logs

## Current Position

Phase: 12 of 14 (MCP Tools - Overview and Logs)
Plan: 2 of 3 complete
Status: In progress - Plan 12-02 complete
Last activity: 2026-01-22 — Completed 12-02-PLAN.md

Progress: [████████████░░] 74% (10.67 of 14 phases complete)

## Milestone History

- **v1.2 Logz.io Integration + Secret Management** — in progress
  - 5 phases (10-14), 21 requirements
  - Logz.io as second log backend with secret management
  - See .planning/ROADMAP-v1.2.md

- **v1.1 Server Consolidation** — shipped 2026-01-21
  - 4 phases, 12 plans, 21 requirements
  - Single-port deployment with in-process MCP
  - See .planning/milestones/v1.1-ROADMAP.md

- **v1 MCP Plugin System + VictoriaLogs** — shipped 2026-01-21
  - 5 phases, 19 plans, 31 requirements
  - Plugin infrastructure + VictoriaLogs integration
  - See .planning/milestones/v1-ROADMAP.md

## Open Blockers

None

## Tech Debt

- DateAdded field not persisted in integration config (from v1)
- GET /{name} endpoint unused by UI (from v1)

## Phase 12 Deliverables (Available for Plan 03)

### Plan 01: Logzio Integration Bootstrap
- **Logzio Integration**: `internal/integration/logzio/logzio.go`
  - Factory registered as "logzio" type
  - Start/Stop lifecycle with SecretWatcher management
  - Health check with SecretWatcher validation

- **Elasticsearch DSL Builder**: `internal/integration/logzio/query.go`
  - BuildLogsQuery with bool queries and .keyword suffixes
  - BuildAggregationQuery with terms aggregation (size 1000)
  - ValidateQueryParams rejecting leading wildcards

- **HTTP Client**: `internal/integration/logzio/client.go`
  - QueryLogs with X-API-TOKEN authentication
  - QueryAggregation with terms aggregation parsing
  - Regional endpoint support (5 regions)

- **Severity Patterns**: `internal/integration/logzio/severity.go`
  - GetErrorPattern() and GetWarningPattern()

### Plan 02: MCP Tools (Overview + Logs)
- **Overview Tool**: `internal/integration/logzio/tools_overview.go`
  - Parallel aggregations (3 goroutines: total, errors, warnings)
  - NamespaceSeverity breakdown (Errors, Warnings, Other, Total)
  - parseTimeRange helper (Unix seconds/milliseconds detection)
  - Registered as logzio_{name}_overview

- **Logs Tool**: `internal/integration/logzio/tools_logs.go`
  - Namespace required, max 100 logs enforced
  - Truncation detection via Limit+1 pattern
  - Structured filters only (no regex exposure to users)
  - Registered as logzio_{name}_logs

- **Tool Registration**: `internal/integration/logzio/logzio.go`
  - RegisterTools with 2 MCP tools
  - Tool schemas with parameter descriptions
  - ToolContext pattern for dependency injection

## Decisions Accumulated

| Phase   | Decision | Impact |
|---------|----------|--------|
| 12-01   | Reused victorialogs.SecretWatcher for token management | No code duplication, proven reliability |
| 12-01   | X-API-TOKEN header instead of Authorization: Bearer | Logz.io API requirement |
| 12-01   | .keyword suffix on exact-match fields | Elasticsearch requirement for exact matching |
| 12-01   | ValidateQueryParams validates internal severity patterns | Protects overview tool from leading wildcard perf issues |
| 12-02   | Logs tool max 100 entries (not 500 like VictoriaLogs) | More conservative limit per CONTEXT.md prevents AI context overflow |
| 12-02   | ValidateQueryParams scope: internal patterns only | Logs tool only exposes structured filters, no user regex exposure |
| 12-02   | Parallel aggregation queries (3 goroutines) | Reduces latency from ~16s to ~10s |
| 12-02   | Logs tool schema: no regex parameter | Users can only use structured filters (namespace, pod, container, level) |

## Next Steps

1. `/gsd:execute-phase 12 --plan 03` — Implement patterns tool (log template mining)

## Cumulative Stats

- Milestones: 2 shipped (v1, v1.1), 1 in progress (v1.2)
- Total phases: 14 planned (10 complete, 4 pending)
- Total plans: 37 complete (31 from v1/v1.1, 4 from Phase 11, 2 from Phase 12)
- Total requirements: 73 (52 complete, 21 pending)
- Total LOC: ~123k (Go + TypeScript)

## Session Continuity

**Last command:** /gsd:execute-phase 12 --plan 02
**Context preserved:** Plan 12-02 complete, Logzio MCP tools (overview + logs) operational

**On next session:**
- Plan 12-01 complete: Logzio integration, Elasticsearch DSL builder, X-API-TOKEN client
- Plan 12-02 complete: Overview tool (parallel aggregations), Logs tool (100-log limit)
- Plan 12-03 ready: Implement patterns tool (log template mining)
- Start with `/gsd:execute-phase 12 --plan 03`

---
*Last updated: 2026-01-22 — Plan 12-02 complete*
