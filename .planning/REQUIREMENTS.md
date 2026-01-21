# Requirements: Spectre v1.1 Server Consolidation

**Defined:** 2026-01-21
**Core Value:** Single-port deployment with in-process MCP execution

## v1.1 Requirements

Requirements for server consolidation. Each maps to roadmap phases.

### Server Consolidation

- [ ] **SRVR-01**: Single HTTP server on port 8080 serves REST API, UI, and MCP
- [ ] **SRVR-02**: MCP endpoint available at `/mcp` path on main server
- [ ] **SRVR-03**: MCP stdio transport remains available via `--transport=stdio` flag
- [ ] **SRVR-04**: Graceful shutdown handles all components (REST, MCP, integrations)
- [ ] **SRVR-05**: Remove standalone `mcp` command from CLI

### Service Layer

- [ ] **SRVC-01**: TimelineService interface shared by REST handlers and MCP tools
- [ ] **SRVC-02**: GraphService interface for graph queries shared by REST and MCP
- [ ] **SRVC-03**: MetadataService interface for metadata operations
- [ ] **SRVC-04**: MCP tools use service layer directly (no HTTP self-calls)
- [ ] **SRVC-05**: REST handlers refactored to use service layer

### Integration Manager

- [ ] **INTG-01**: Integration manager initializes with MCP server in consolidated mode
- [ ] **INTG-02**: Dynamic tool registration works on consolidated server
- [ ] **INTG-03**: Config hot-reload continues to work for integrations

### Helm Chart

- [ ] **HELM-01**: Remove MCP sidecar container from deployment template
- [ ] **HELM-02**: Remove MCP-specific values (mcp.enabled, mcp.port, etc.)
- [ ] **HELM-03**: Single container deployment for Spectre
- [ ] **HELM-04**: MCP available at /mcp on main service port

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
| SRVR-01 | Phase 1 | Pending |
| SRVR-02 | Phase 1 | Pending |
| SRVR-03 | Phase 1 | Pending |
| SRVR-04 | Phase 1 | Pending |
| SRVR-05 | Phase 3 | Pending |
| SRVC-01 | Phase 2 | Pending |
| SRVC-02 | Phase 2 | Pending |
| SRVC-03 | Phase 2 | Pending |
| SRVC-04 | Phase 2 | Pending |
| SRVC-05 | Phase 2 | Pending |
| INTG-01 | Phase 1 | Pending |
| INTG-02 | Phase 1 | Pending |
| INTG-03 | Phase 1 | Pending |
| HELM-01 | Phase 3 | Pending |
| HELM-02 | Phase 3 | Pending |
| HELM-03 | Phase 3 | Pending |
| HELM-04 | Phase 3 | Pending |
| TEST-01 | Phase 4 | Pending |
| TEST-02 | Phase 4 | Pending |
| TEST-03 | Phase 4 | Pending |
| TEST-04 | Phase 4 | Pending |

**Coverage:**
- v1.1 requirements: 21 total
- Mapped to phases: 21
- Unmapped: 0 ✓

---
*Requirements defined: 2026-01-21*
*Last updated: 2026-01-21 after initial definition*
