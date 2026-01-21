# Requirements: Spectre v1.1 Server Consolidation

**Defined:** 2026-01-21
**Core Value:** Single-port deployment with in-process MCP execution

## v1.1 Requirements

Requirements for server consolidation. Each maps to roadmap phases.

### Server Consolidation

- [x] **SRVR-01**: Single HTTP server on port 8080 serves REST API, UI, and MCP
- [x] **SRVR-02**: MCP endpoint available at `/v1/mcp` path on main server
- [x] **SRVR-03**: MCP stdio transport remains available via `--stdio` flag
- [x] **SRVR-04**: Graceful shutdown handles all components (REST, MCP, integrations)
- [x] **SRVR-05**: Remove standalone `mcp` command from CLI

### Service Layer

- [x] **SRVC-01**: TimelineService interface shared by REST handlers and MCP tools
- [x] **SRVC-02**: GraphService interface for graph queries shared by REST and MCP
- [x] **SRVC-03**: MetadataService interface for metadata operations
- [x] **SRVC-04**: MCP tools use service layer directly (no HTTP self-calls)
- [x] **SRVC-05**: REST handlers refactored to use service layer

### Integration Manager

- [x] **INTG-01**: Integration manager initializes with MCP server in consolidated mode
- [x] **INTG-02**: Dynamic tool registration works on consolidated server
- [x] **INTG-03**: Config hot-reload continues to work for integrations

### Helm Chart

- [x] **HELM-01**: Remove MCP sidecar container from deployment template
- [x] **HELM-02**: Remove MCP-specific values (mcp.enabled, mcp.port, etc.)
- [x] **HELM-03**: Single container deployment for Spectre
- [x] **HELM-04**: MCP available at /mcp on main service port

### E2E Tests

- [ ] **TEST-01**: MCP HTTP tests connect to main server port at /mcp
- [ ] **TEST-02**: MCP stdio tests work with consolidated server binary
- [ ] **TEST-03**: Config reload tests work with consolidated architecture
- [ ] **TEST-04**: Remove MCP sidecar-specific test assumptions

## Out of Scope

| Feature | Reason |
|---------|--------|
| MCP authentication | Not needed for v1.1, defer to future |
| Multiple MCP endpoints | Single /mcp path sufficient |
| gRPC transport for MCP | HTTP and stdio sufficient |
| Separate MCP process option | Consolidation is the goal |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| SRVR-01 | Phase 6 | Complete |
| SRVR-02 | Phase 6 | Complete |
| SRVR-03 | Phase 6 | Complete |
| SRVR-04 | Phase 6 | Complete |
| INTG-01 | Phase 6 | Complete |
| INTG-02 | Phase 6 | Complete |
| INTG-03 | Phase 6 | Complete |
| SRVC-01 | Phase 7 | Complete |
| SRVC-02 | Phase 7 | Complete |
| SRVC-03 | Phase 7 | Complete |
| SRVC-04 | Phase 7 | Complete |
| SRVC-05 | Phase 7 | Complete |
| SRVR-05 | Phase 8 | Complete |
| HELM-01 | Phase 8 | Complete |
| HELM-02 | Phase 8 | Complete |
| HELM-03 | Phase 8 | Complete |
| HELM-04 | Phase 8 | Complete |
| TEST-01 | Phase 9 | Pending |
| TEST-02 | Phase 9 | Pending |
| TEST-03 | Phase 9 | Pending |
| TEST-04 | Phase 9 | Pending |

**Coverage:**
- v1.1 requirements: 21 total
- Mapped to phases: 21
- Unmapped: 0 ✓

---
*Requirements defined: 2026-01-21*
*Last updated: 2026-01-21 — Phase 8 requirements marked Complete (17/21)*
