---
phase: 01-plugin-infrastructure-foundation
plan: 01
subsystem: infra
tags: [integration, config, koanf, yaml, lifecycle]

# Dependency graph
requires:
  - phase: none
    provides: foundation phase - no dependencies
provides:
  - Integration interface contract (lifecycle + tool registration)
  - IntegrationMetadata type for instance identification
  - HealthStatus enum (Healthy/Degraded/Stopped)
  - IntegrationsFile YAML config schema with versioning
  - IntegrationConfig per-instance schema
  - Config validation (schema version, duplicate names)
  - Koanf v2.3.0 for hot-reload capability
affects: [01-02, 01-03, all integration implementations]

# Tech tracking
tech-stack:
  added: [koanf/v2@v2.3.0, koanf/providers/file, koanf/parsers/yaml]
  patterns: [integration interface pattern, config schema versioning, health status states]

key-files:
  created:
    - internal/integration/types.go
    - internal/config/integration_config.go
    - internal/config/integration_config_test.go
    - internal/config/koanf_deps.go
  modified: [go.mod, go.sum]

key-decisions:
  - "Integration instances are in-tree (compiled into Spectre), not external plugins"
  - "Multiple instances of same integration type supported (e.g., victorialogs-prod, victorialogs-staging)"
  - "Failed connections mark instance as Degraded, not crash server"
  - "Config schema versioning with v1 as initial version"
  - "ToolRegistry placeholder interface for MCP tool registration (concrete implementation in Phase 2)"

patterns-established:
  - "Integration interface: Metadata/Start/Stop/Health/RegisterTools methods"
  - "HealthStatus tri-state: Healthy (normal), Degraded (connection failed but registered), Stopped (explicitly stopped)"
  - "Config validation rejects invalid schema versions and duplicate instance names"
  - "YAML config structure: schema_version + instances array with name/type/enabled/config fields"

# Metrics
duration: 3min
completed: 2026-01-20
---

# Phase 01 Plan 01: Integration Config & Interface Foundation Summary

**Integration interface contract with lifecycle methods and YAML config schema supporting versioned multi-instance configurations**

## Performance

- **Duration:** 3 minutes
- **Started:** 2026-01-20T23:42:30Z
- **Completed:** 2026-01-20T23:45:06Z
- **Tasks:** 3
- **Files modified:** 7

## Accomplishments
- Integration interface defining lifecycle contract (Start/Stop/Health/RegisterTools)
- Config schema with explicit versioning (v1) and validation
- Koanf v2.3.0 dependency added for config hot-reload in next plan
- HealthStatus enum with three states for health monitoring

## Task Commits

Each task was committed atomically:

1. **Task 1: Define integration interface and metadata types** - `561ef5f` (feat)
2. **Task 2: Define integration config schema with versioning** - `2a4fd7a` (feat)
3. **Task 3: Add Koanf dependency for config hot-reload** - `c6b10c3` (chore)

## Files Created/Modified

**Created:**
- `internal/integration/types.go` - Integration interface, HealthStatus enum, IntegrationMetadata struct, ToolRegistry placeholder
- `internal/config/integration_config.go` - IntegrationsFile and IntegrationConfig schemas with Validate() method
- `internal/config/integration_config_test.go` - Comprehensive validation tests (valid/invalid schema versions, duplicate names, missing fields)
- `internal/config/koanf_deps.go` - Blank imports to ensure Koanf dependencies in go.mod

**Modified:**
- `go.mod` - Added koanf/v2@v2.3.0, koanf/providers/file@v1.2.1, koanf/parsers/yaml@v1.1.0
- `go.sum` - Updated checksums for new dependencies

## Decisions Made

**Architecture:**
- **In-tree integrations:** Integration code compiled into Spectre binary, not external plugins. Simplifies deployment and eliminates version compatibility issues.
- **Multi-instance support:** Config file defines multiple instances with unique names (e.g., victorialogs-prod, victorialogs-staging). Each instance has independent lifecycle and health.
- **Degraded state design:** Failed connections mark instance as Degraded (not crash server). Instance stays registered, MCP tools return errors until health recovers via periodic checks.

**Config Schema:**
- **Explicit versioning:** `schema_version` field enables in-memory migration for future config format changes. Starting with "v1".
- **Instance-level config:** Each instance has `name` (unique), `type` (integration type), `enabled` (startup flag), and `config` (type-specific map).

**Interface Design:**
- **ToolRegistry placeholder:** Defined minimal interface for MCP tool registration. Concrete implementation deferred to Plan 02 (integration manager) to avoid premature coupling.
- **Context-based lifecycle:** Start/Stop/Health use `context.Context` for cancellation and timeout support, following Go best practices.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

**Go module behavior with unused imports:**
- **Issue:** Running `go get` downloaded Koanf packages but didn't add them to `go.mod` because no code imported them yet.
- **Solution:** Created `internal/config/koanf_deps.go` with blank imports (`import _ "package"`) to force dependencies into `go.mod`. This is standard Go practice for declaring dependencies before use.
- **Outcome:** All Koanf packages now in `go.mod`, ready for config loader implementation in Plan 02.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for Plan 02 (Integration Manager):**
- Integration interface contract defined and stable
- Config schema ready for file loading with Koanf
- HealthStatus states defined for health monitoring
- ToolRegistry interface ready for concrete implementation

**Blockers:** None

**Concerns:** None - foundation types established correctly

**Next steps:**
- Plan 02: Implement integration manager with lifecycle orchestration
- Plan 02: Implement config loader with Koanf and hot-reload via fsnotify
- Plan 03: Integrate with existing MCP server (internal/mcp/server.go)

---
*Phase: 01-plugin-infrastructure-foundation*
*Completed: 2026-01-20*
