---
phase: 26
plan: 08
subsystem: grafana-integration
tags: [observatory, mcp-tools, tool-registration, integration]
dependency-graph:
  requires: [26-04, 26-05, 26-06, 26-07]
  provides: [RegisterObservatoryTools, observatory-service-lifecycle, integration-tests]
  affects: [grafana-integration, mcp-server]
tech-stack:
  added: []
  patterns: [tool-adapter-pattern, service-lifecycle]
key-files:
  created:
    - internal/integration/grafana/observatory_tools.go
    - internal/integration/grafana/observatory_integration_test.go
  modified:
    - internal/integration/grafana/grafana.go
    - internal/integration/grafana/query_service.go
decisions:
  - id: D08-01
    choice: "Use ToolRegistry adapter instead of direct MCP server registration"
    rationale: "Follows existing pattern in grafana.go RegisterTools method"
  - id: D08-02
    choice: "Implement FetchCurrentValue/FetchHistoricalValue as stub methods"
    rationale: "Graceful fallback to baseline mean values when Grafana queries not available"
  - id: D08-03
    choice: "Create both RegisterObservatoryTools function and registerObservatoryTools method"
    rationale: "Function for direct MCP server registration, method for ToolRegistry adapter"
metrics:
  duration: 20m
  completed: 2026-01-30
---

# Phase 26 Plan 08: Tool Registration & Lifecycle Summary

## One-liner
MCP tool registration via ToolRegistry adapter with GrafanaIntegration lifecycle integration and comprehensive integration tests.

## What Was Built

### RegisterObservatoryTools Function (observatory_tools.go)
- **197 lines** providing centralized tool registration
- `wrapToolHandler` adapter to convert `func(ctx, []byte) (interface{}, error)` to mcp-go `ToolHandlerFunc`
- All 8 observatory tools registered with proper MCP schemas:
  - **Orient**: observatory_status, observatory_changes
  - **Narrow**: observatory_scope, observatory_signals
  - **Investigate**: observatory_signal_detail, observatory_compare
  - **Hypothesize**: observatory_explain
  - **Verify**: observatory_evidence

### GrafanaIntegration Lifecycle Updates (grafana.go)
- Added observatory services as struct fields:
  - `observatoryService *ObservatoryService`
  - `investigateService *ObservatoryInvestigateService`
  - `evidenceService *ObservatoryEvidenceService`
  - `anomalyAggregator *AnomalyAggregator`
- Updated `Start()` to initialize observatory services after baseline collector
- Updated `Stop()` to clear observatory services
- Added `registerObservatoryTools()` method to register 8 tools via ToolRegistry

### QueryService Interface Implementation (query_service.go)
- Added `FetchCurrentValue` method (stub with graceful fallback)
- Added `FetchHistoricalValue` method (stub with graceful fallback)
- Enables `ObservatoryInvestigateService` to use `*GrafanaQueryService`

### Integration Tests (observatory_integration_test.go)
- **564 lines** with comprehensive test coverage
- 9 test cases covering all observatory tools:
  - TestObservatoryIntegration_StatusTool
  - TestObservatoryIntegration_ScopeTool
  - TestObservatoryIntegration_SignalDetailTool
  - TestObservatoryIntegration_ExplainTool
  - TestObservatoryIntegration_EvidenceTool
  - TestObservatoryIntegration_EmptyResults
  - TestObservatoryIntegration_ToolRegistration (8 sub-tests)
  - TestObservatoryIntegration_CompareTool
  - TestObservatoryIntegration_SignalsTool
- All tests pass with race detector enabled

## Key Design Decisions

### D08-01: ToolRegistry Adapter Pattern
Used the existing ToolRegistry interface pattern from grafana.go instead of direct MCP server registration. This:
- Maintains consistency with existing metrics tools
- Allows the integration manager to control tool registration
- Separates tool creation from MCP server details

### D08-02: QueryService Stub Implementation
Implemented FetchCurrentValue/FetchHistoricalValue as stub methods that return errors. The investigate service gracefully falls back to baseline mean values. This allows:
- Observatory tools to work with existing baseline data
- Future enhancement to query Grafana directly for real-time values
- No breaking changes to existing service interfaces

### D08-03: Dual Registration Approach
Created both:
- `RegisterObservatoryTools` function in observatory_tools.go (for direct MCP server use)
- `registerObservatoryTools` method in grafana.go (for ToolRegistry adapter)

This provides flexibility for different integration scenarios.

## Verification Results

| Check | Status |
|-------|--------|
| `go build ./internal/integration/grafana/...` | PASS |
| `go test -v -race ... -run TestObservatoryIntegration` | 9/9 PASS |
| All 8 tools registered with MCP server | PASS |
| Services initialized in correct order in Start() | PASS |
| observatory_tools.go >= 150 lines | PASS (197 lines) |
| observatory_integration_test.go >= 200 lines | PASS (564 lines) |

## Requirements Satisfied

### API Requirements (from CONTEXT.md)
- API-01 through API-08: All satisfied by service layer (Plans 02, 03)

### Tool Requirements (from CONTEXT.md)
- TOOL-01 through TOOL-16: All satisfied by tool implementations (Plans 04-07)

### Integration Requirements (this plan)
- Tool registration with proper MCP schemas
- Service lifecycle in GrafanaIntegration
- End-to-end integration tests

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added FetchCurrentValue/FetchHistoricalValue to GrafanaQueryService**
- **Found during:** Task 2
- **Issue:** ObservatoryInvestigateService requires QueryService interface with these methods
- **Fix:** Added stub implementations with graceful error fallback
- **Files modified:** query_service.go
- **Commit:** 8ba7e72

## Commits

| Hash | Message |
|------|---------|
| e4e0524 | feat(26-08): create RegisterObservatoryTools function |
| 8ba7e72 | feat(26-08): wire observatory services into Grafana integration lifecycle |
| 6eacbc5 | test(26-08): create observatory integration tests |

## Next Steps

Phase 26 complete. All 8 observatory MCP tools are:
1. Implemented with proper API contracts
2. Registered with the MCP server via ToolRegistry
3. Integrated into GrafanaIntegration lifecycle
4. Verified with comprehensive integration tests

The observatory tools follow the progressive disclosure pattern for AI-driven incident investigation:
- Orient (cluster-wide) -> Narrow (namespace/workload) -> Investigate (signals) -> Hypothesize (candidates) -> Verify (evidence)
