---
phase: 08-cleanup-helm-update
plan: 02
subsystem: deployment
tags: [helm-chart, mcp, single-container, kubernetes]

# Dependency graph
requires:
  - phase: 06-01
    provides: Consolidated server with in-process MCP
provides:
  - Helm chart deploying single Spectre container with integrated MCP
  - Service exposing MCP at /v1/mcp on port 8080
  - No MCP sidecar configuration or deployment
affects:
  - phase: 09
    impact: E2E tests will use single-container deployment

# Tech tracking
tech-stack:
  added: []
  removed:
    - "MCP sidecar container from Helm deployment"
    - "Port 8082 for MCP service"
    - "mcp: section from values.yaml and test fixtures"
  patterns:
    - "Single-container deployment: MCP runs in-process on main port"

key-files:
  created: []
  modified:
    - chart/templates/deployment.yaml
    - chart/templates/service.yaml
    - chart/templates/ingress.yaml
    - chart/values.yaml
    - tests/e2e/fixtures/helm-values-test.yaml
  deleted: []

key-decisions:
  - "Removed MCP sidecar completely from Helm chart"
  - "Service exposes only port 8080 (main) and optional 9999 (pprof)"
  - "MCP endpoint accessible at /v1/mcp on main service (no separate routing)"
  - "Test fixtures updated to match single-container architecture"

patterns-established:
  - "Single-container Kubernetes deployment for Spectre with integrated MCP"
  - "Port consolidation: All HTTP traffic (REST, gRPC-Web, MCP) on port 8080"

# Metrics
duration: 4min
completed: 2026-01-21
---

# Phase 08 Plan 02: Helm Chart MCP Sidecar Removal Summary

**Helm chart updated to deploy single Spectre container with integrated MCP server on port 8080**

## Performance

- **Duration:** 4 min
- **Started:** 2026-01-21T20:36:50Z
- **Completed:** 2026-01-21T20:40:54Z
- **Tasks:** 3/3 completed
- **Files modified:** 5 (deployment, service, ingress, values, test fixture)
- **Files deleted:** 0

## Accomplishments

- Removed MCP sidecar container from deployment.yaml
- Removed MCP port (8082) from service.yaml
- Simplified ingress.yaml to remove MCP-specific routing
- Deleted mcp: section (49 lines) from values.yaml
- Updated port allocation comment to show MCP at /v1/mcp on port 8080
- Updated test fixture to remove MCP sidecar configuration
- Verified Helm rendering works with updated chart
- Confirmed helm lint passes with no errors
- FalkorDB sidecar remains intact (graph.enabled still supported)

## Task Commits

1. **Task 1: Remove MCP sidecar from deployment and service templates** - `e46dfa8` (chore)
   - Removed MCP container block from deployment.yaml
   - Removed MCP port exposure from service.yaml

2. **Task 2: Remove MCP-specific ingress and update values.yaml** - `d28037b` (chore)
   - Simplified ingress.yaml conditionals
   - Removed MCP TLS and routing sections
   - Deleted entire mcp: section from values.yaml
   - Updated port allocation comment

3. **Task 3: Update test fixture and verify Helm rendering** - `dc3ec41` (chore)
   - Removed mcp: section from helm-values-test.yaml
   - Verified Helm template rendering
   - Confirmed helm lint passes

## Files Created/Modified

- `chart/templates/deployment.yaml` - Removed MCP sidecar container block (lines 158-206)
- `chart/templates/service.yaml` - Removed MCP port exposure (lines 39-44)
- `chart/templates/ingress.yaml` - Removed MCP-specific conditionals and routing
- `chart/values.yaml` - Deleted mcp: section (49 lines), updated port comment
- `tests/e2e/fixtures/helm-values-test.yaml` - Removed MCP sidecar configuration (lines 146-154)

## Decisions Made

**1. Remove MCP sidecar completely vs keep as optional**
- **Decision:** Remove completely
- **Rationale:** After Phase 6, MCP runs in-process. Sidecar architecture is obsolete.
- **Impact:** Helm chart deploys single container, simpler configuration, lower resource usage
- **Alternative considered:** Keep mcp.enabled flag for backward compatibility, but adds complexity for no benefit

**2. Port consolidation strategy**
- **Decision:** All HTTP traffic (REST API, gRPC-Web, MCP) on single port 8080
- **Rationale:** Aligns with Phase 6 consolidated server architecture
- **Impact:** Simplified service definition, ingress routing, and firewall rules
- **Benefits:** Easier configuration, fewer ports to manage, cleaner architecture

**3. Update test fixtures immediately vs defer**
- **Decision:** Update immediately as part of this plan
- **Rationale:** E2E tests in Phase 9 will use Helm chart, must match new architecture
- **Impact:** Test fixtures ready for Phase 9, no follow-up work needed
- **Alternative:** Could defer to Phase 9, but creates dependency and potential for missed updates

## Deviations from Plan

None - plan executed exactly as written.

All verification checks passed:
- Template files have no .Values.mcp references
- values.yaml has no mcp: section
- values.yaml has no 8082 references
- Port comment updated to show MCP at /v1/mcp
- Test fixture has no mcp: section
- Helm template renders successfully
- helm lint passes with no errors
- Rendered deployment has single Spectre container
- Rendered service exposes only port 8080
- FalkorDB sidecar still present when graph.enabled

## Next Phase Readiness

**Ready for Phase 8 Plan 03:**
- ✅ Helm chart updated to single-container architecture
- ✅ MCP sidecar removed from all templates and values
- ✅ Service exposes MCP at /v1/mcp on port 8080
- ✅ Test fixtures updated for E2E tests
- ✅ Helm rendering verified working

**Blockers:** None

**Concerns:** None

**Recommendations:**
- Proceed to Plan 08-03 (likely documentation or final cleanup)
- Phase 9 E2E tests should verify single-container deployment works correctly

## Technical Notes

### Architecture Change

**Before (Phase 5 and earlier):**
```
Pod:
  - Container: spectre (port 8080 - REST API)
  - Container: mcp (port 8082 - MCP server, calls REST API via localhost)
  - Container: falkordb (optional)

Service:
  - Port 8080 -> spectre container
  - Port 8082 -> mcp container
```

**After (Phase 6+):**
```
Pod:
  - Container: spectre (port 8080 - REST API + MCP at /v1/mcp)
  - Container: falkordb (optional)

Service:
  - Port 8080 -> spectre container (REST API + MCP)
```

### Helm Chart Simplification

- **Removed 49 lines** from values.yaml (mcp: section)
- **Removed 49 lines** from deployment.yaml (MCP container block)
- **Removed 6 lines** from service.yaml (MCP port)
- **Removed 20 lines** from ingress.yaml (MCP TLS and routing)
- **Removed 9 lines** from test fixture (MCP sidecar resources)

**Total:** 133 lines removed

### Resource Savings

**Per pod resource savings (MCP sidecar removed):**
- Memory request: -64Mi (or -32Mi in CI)
- Memory limit: -256Mi (or -128Mi in CI)
- CPU request: -50m (or -25m in CI)

**Network savings:**
- No localhost HTTP calls from MCP to REST API
- Direct service layer calls (eliminated in Phase 7)

### Ingress Simplification

**Before:** Two conditionals for ingress creation
- `.Values.ingress.enabled` OR `.Values.mcp.enabled`
- Separate host and routing for MCP

**After:** Single conditional
- `.Values.ingress.enabled` only
- MCP accessible at /v1/mcp on main host

### Test Fixture Alignment

Test fixture now matches production deployment:
- Single Spectre container
- MCP at /v1/mcp on port 8080
- FalkorDB sidecar (when graph.enabled)
- Lower resource limits for CI environment

---

*Phase 08 Plan 02 complete: Helm chart updated for single-container architecture*
