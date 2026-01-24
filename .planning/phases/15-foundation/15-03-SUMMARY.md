---
phase: 15-foundation
plan: 03
subsystem: ui
tags: [react, typescript, grafana, integration-config, ui-form]

# Dependency graph
requires:
  - phase: 15-01
    provides: Grafana integration lifecycle and factory registration
  - phase: 15-02
    provides: Graph schema for dashboard queries
provides:
  - Grafana integration type in UI dropdown
  - Grafana-specific form fields (URL, SecretRef)
  - Test connection handler for Grafana via generic factory pattern
  - End-to-end configuration flow from UI to health check
affects: [16-metrics-tools, 17-graph-navigation]

# Tech tracking
tech-stack:
  added: []
  patterns: [generic-factory-test-handler, integration-form-fields]

key-files:
  created: []
  modified:
    - ui/src/components/IntegrationConfigForm.tsx
    - internal/api/handlers/integration_config_handler.go

key-decisions:
  - "Generic factory pattern eliminates need for type-specific switch cases in test handler"
  - "Blank import pattern for factory registration via init() functions"

patterns-established:
  - "Integration forms follow consistent pattern: type dropdown → type-specific fields → authentication section"
  - "Authentication section uses visual grouping (border, background) for SecretRef fields"

# Metrics
duration: 2min
completed: 2026-01-22
---

# Phase 15 Plan 03: UI Configuration Form Summary

**Grafana integration configurable via UI with URL and SecretRef fields, test connection validates via generic factory pattern**

## Performance

- **Duration:** 2 min
- **Started:** 2026-01-22T21:20:37Z
- **Completed:** 2026-01-22T21:22:34Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Grafana type added to integration dropdown in UI
- Grafana form displays URL field and Authentication section (secret name + key)
- Test connection handler supports Grafana via generic factory pattern
- Complete configuration flow: user selects Grafana → fills form → tests connection → backend validates via health check

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Grafana Form Fields to IntegrationConfigForm** - `9dc6258` (feat)
2. **Task 2: Add Grafana Test Connection Handler** - `7f9dfa1` (feat)

## Files Created/Modified
- `ui/src/components/IntegrationConfigForm.tsx` - Added Grafana form section with URL and SecretRef fields following Logz.io visual pattern
- `internal/api/handlers/integration_config_handler.go` - Added blank import for grafana package to register factory with existing generic test handler

## Decisions Made

**Generic factory pattern eliminates type-specific code:**
- Existing `HandleTest` method already uses `integration.GetFactory(testReq.Type)` for all integration types
- No switch statement needed - just register factory via init() function
- Blank import `_ "internal/integration/grafana"` ensures factory registration
- testConnection helper handles full lifecycle: create, start, health check, stop
- This pattern scales: adding new integration types requires zero changes to handler code

**Form structure follows established pattern:**
- Grafana form matches Logz.io visual design: bordered authentication section with grouped SecretRef fields
- Placeholder shows both Cloud and self-hosted URL patterns for user guidance
- Reuses existing handleSecretNameChange/handleSecretKeyChange handlers
- Type dropdown extends naturally with new "grafana" option

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required at this stage. Users will create Kubernetes Secrets manually as documented in integration guides.

## Next Phase Readiness

**Phase 15 Foundation complete:**
- Grafana API client implemented with SecretWatcher (15-01)
- Graph schema defined for dashboard/panel queries (15-02)
- UI configuration form complete with test connection (15-03)

**Ready for Phase 16 (MCP Metrics Tools):**
- get_metrics_overview tool can use client.ListDashboards()
- query_metrics tool can use client.QueryRange() with dashboard context
- Graph navigation tools can traverse dashboard → panel → query structure
- All Grafana configuration accessible via integration manager

**No blockers:**
- Generic factory pattern supports Grafana test connection
- Health check validates both dashboard and datasource access
- Form validation ensures correct configuration before save

---
*Phase: 15-foundation*
*Completed: 2026-01-22*
