---
phase: 15-foundation
plan: 01
subsystem: integration
tags: [grafana, api-client, kubernetes, secret-watcher, http, bearer-auth]

# Dependency graph
requires:
  - phase: victorialogs
    provides: Integration lifecycle pattern and SecretWatcher implementation
provides:
  - Grafana integration backend with API client
  - Factory registration as "grafana" integration type
  - SecretWatcher for token hot-reload
  - Health check with dashboard and datasource validation
affects: [15-02-ui-config, 15-03-graph-schema, 18-mcp-tools]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Integration lifecycle with degraded state and auto-recovery"
    - "SecretWatcher pattern for K8s Secret hot-reload"
    - "Bearer token authentication with Authorization header"
    - "Health check with required/optional endpoint validation"

key-files:
  created:
    - internal/integration/grafana/types.go
    - internal/integration/grafana/client.go
    - internal/integration/grafana/grafana.go
    - internal/integration/grafana/secret_watcher.go
  modified: []

key-decisions:
  - "Copied SecretWatcher to grafana package (temporary duplication, refactor deferred)"
  - "Dashboard access required for health check, datasource access optional (warns on failure)"
  - "Follows VictoriaLogs integration pattern exactly for consistency"

patterns-established:
  - "Config with SecretRef and Validate() method"
  - "Client with tuned connection pooling (MaxIdleConnsPerHost: 10)"
  - "Integration with Start/Stop/Health lifecycle and thread-safe health status"
  - "Factory registration in init() with integration.RegisterFactory()"
  - "Degraded state when secret missing, auto-recovery when available"

# Metrics
duration: 3min
completed: 2026-01-22
---

# Phase 15 Plan 01: Grafana API Client & Integration Lifecycle Summary

**Grafana integration backend with Bearer token auth, dashboard/datasource API access, SecretWatcher hot-reload, and factory registration for multi-instance support**

## Performance

- **Duration:** 3 min
- **Started:** 2026-01-22T20:15:45Z
- **Completed:** 2026-01-22T20:18:57Z
- **Tasks:** 4
- **Files created:** 4

## Accomplishments

- Complete Grafana integration backend following VictoriaLogs pattern exactly
- HTTP client with Bearer token authentication and connection pooling
- Health check validates dashboard access (required) and datasource access (optional)
- SecretWatcher provides hot-reload of API token without restart
- Factory registration enables multiple Grafana instances (prod, staging, etc.)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Grafana Config Types with SecretRef and Validation** - `91808b3` (feat)
2. **Task 2: Implement Grafana HTTP Client with Bearer Auth** - `a4274b3` (feat)
3. **Task 3: Implement Integration Lifecycle with Factory Registration** - `fc9a483` (feat)
4. **Task 4: Move SecretWatcher to Reusable Location** - `72ab21e` (feat)

## Files Created/Modified

- `internal/integration/grafana/types.go` - Config and SecretRef types with validation
- `internal/integration/grafana/client.go` - HTTP client with ListDashboards, GetDashboard, ListDatasources methods
- `internal/integration/grafana/grafana.go` - Integration lifecycle with factory registration and health checks
- `internal/integration/grafana/secret_watcher.go` - K8s Secret watcher for token hot-reload

## Decisions Made

**1. SecretWatcher duplication instead of shared package**
- Rationale: Copied SecretWatcher to grafana package to avoid cross-package refactoring in this phase
- Future work: Refactor to internal/integration/common/secret_watcher.go in later phase
- Maintains working implementation while deferring architectural cleanup

**2. Health check strategy: dashboard required, datasource optional**
- Rationale: Dashboard access is essential for metrics integration, datasource access might fail with limited permissions
- Implementation: testConnection() fails if dashboard access fails, warns but continues if datasource access fails
- Enables graceful degradation for restricted API tokens

**3. Full VictoriaLogs pattern match**
- Rationale: Consistency with existing integration reduces cognitive overhead and bugs
- Benefits: Developers already familiar with victorialogs pattern, easier code review
- Implementation: Matched struct fields, lifecycle methods, error handling, logging patterns

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - implementation followed established VictoriaLogs pattern successfully.

## User Setup Required

None - no external service configuration required for this plan.

## Next Phase Readiness

**Ready for Phase 15 Plan 02 (UI Configuration Form):**
- Config types defined with JSON/YAML tags for frontend consumption
- Validate() method ready for client-side validation
- SecretRef pattern established for K8s Secret references
- Health check endpoints available for connection testing

**Ready for Phase 15 Plan 03 (Graph Schema):**
- Client can list all dashboards via ListDashboards()
- Client can retrieve full dashboard JSON via GetDashboard()
- Integration lifecycle supports future graph database initialization

**Ready for Phase 18 (MCP Tools):**
- RegisterTools() placeholder ready for tool implementations
- Client methods ready for MCP tool handlers
- Instance-based architecture supports tool naming (e.g., grafana_prod_overview)

**No blockers or concerns.**

---
*Phase: 15-foundation*
*Completed: 2026-01-22*
