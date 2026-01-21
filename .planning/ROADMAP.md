# Milestone v1.1: Server Consolidation

**Status:** IN PROGRESS
**Phases:** 6-9
**Started:** 2026-01-21

## Overview

Consolidate MCP server into main Spectre server for single-port deployment and in-process tool execution. Eliminates MCP sidecar container, reduces deployment complexity, and improves performance through shared service layer.

This roadmap delivers 21 v1.1 requirements across 4 phases, progressing from server consolidation through service layer extraction, Helm cleanup, and E2E validation.

## Phases

### Phase 6: Consolidated Server & Integration Manager

**Goal:** Single server binary serves REST API, UI, and MCP on port 8080 with in-process integration manager.

**Dependencies:** None (foundation for v1.1)

**Requirements:** SRVR-01, SRVR-02, SRVR-03, SRVR-04, INTG-01, INTG-02, INTG-03

**Success Criteria:**
1. User can access REST API, UI, and MCP endpoint (/mcp) on single port 8080
2. MCP stdio transport continues to work via `spectre server --transport=stdio`
3. Integration manager initializes with MCP server and dynamic tool registration works
4. Server gracefully shuts down all components (REST, MCP, integrations) on SIGTERM
5. Config hot-reload continues to work for integrations in consolidated mode

**Plans:** 2 plans

Plans:
- [ ] 06-01-PLAN.md — Integrate MCP server into main server with StreamableHTTP transport and integration manager
- [ ] 06-02-PLAN.md — Verify consolidated server with MCP endpoint, integrations, and graceful shutdown

**Status:** Ready to execute

---

### Phase 7: Service Layer Extraction

**Goal:** REST handlers and MCP tools share common service layer for timeline, graph, and metadata operations.

**Dependencies:** Phase 6 (needs consolidated server architecture)

**Requirements:** SRVC-01, SRVC-02, SRVC-03, SRVC-04, SRVC-05

**Success Criteria:**
1. TimelineService interface exists and both REST handlers and MCP tools call it directly
2. GraphService interface exists for FalkorDB queries used by REST and MCP
3. MetadataService interface exists for metadata operations shared by both layers
4. MCP tools execute service methods in-process (no HTTP self-calls to localhost)
5. REST handlers refactored to use service layer instead of inline business logic

**Plans:** TBD

**Status:** Pending

---

### Phase 8: Cleanup & Helm Chart Update

**Goal:** Remove standalone MCP command and update Helm chart for single-container deployment.

**Dependencies:** Phase 6 (needs working consolidated server), Phase 7 (needs service layer for stability)

**Requirements:** SRVR-05, HELM-01, HELM-02, HELM-03, HELM-04

**Success Criteria:**
1. Standalone `spectre mcp` command removed from CLI (only `spectre server` remains)
2. Helm chart deploys single Spectre container (no MCP sidecar)
3. Helm values.yaml removes MCP-specific configuration (mcp.enabled, mcp.port, etc.)
4. Deployed pod exposes MCP at /mcp path on main service port 8080

**Plans:** TBD

**Status:** Pending

---

### Phase 9: E2E Test Validation

**Goal:** E2E tests verify consolidated architecture works for MCP HTTP, MCP stdio, and config reload scenarios.

**Dependencies:** Phase 8 (needs deployed consolidated server)

**Requirements:** TEST-01, TEST-02, TEST-03, TEST-04

**Success Criteria:**
1. MCP HTTP tests connect to main server port 8080 at /mcp path and all tools respond
2. MCP stdio tests work with consolidated `spectre server --transport=stdio` binary
3. Config reload tests verify integration hot-reload works in consolidated architecture
4. MCP sidecar-specific test assumptions removed (no localhost:3000 hardcoding)

**Plans:** TBD

**Status:** Pending

---

## Progress

| Phase | Status | Plans | Requirements |
|-------|--------|-------|--------------|
| 6 - Consolidated Server & Integration Manager | Ready to execute | 0/2 | 7 |
| 7 - Service Layer Extraction | Pending | 0/0 | 5 |
| 8 - Cleanup & Helm Chart Update | Pending | 0/0 | 5 |
| 9 - E2E Test Validation | Pending | 0/0 | 4 |

**Total:** 0/2 plans complete, 21 requirements

---

## Milestone Summary

**Decimal Phases:** None

**Key Decisions:**
- TBD (updated as phases execute)

**Issues Resolved:**
- TBD

**Issues Deferred:**
- TBD

**Technical Debt Incurred:**
- TBD

---

*For current project status, see .planning/PROJECT.md*
*For previous milestone history, see .planning/milestones/v1-ROADMAP.md*

---

*Created: 2026-01-21*
*Last updated: 2026-01-21 — Phase 6 plans created (2 plans in 2 waves)*
