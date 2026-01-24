# Phase 13: MCP Tools - Patterns - Context

**Gathered:** 2026-01-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Pattern mining MCP tool for Logz.io integration exposing log templates with novelty detection. Reuses existing Drain algorithm from VictoriaLogs. Tool provides namespace-scoped pattern storage with live/known/novel modes.

</domain>

<decisions>
## Implementation Decisions

### VictoriaLogs Parity
- Exact match with VictoriaLogs patterns tool — same parameters, same output format, same behavior
- Consistent AI experience across log backends
- All three modes supported: live (current patterns), known (historical), novel (new patterns not seen before)
- Same result limits: max 50 templates per response

### Code Organization
- Extract Drain algorithm to `internal/logprocessing/` as common code
- Both VictoriaLogs and Logz.io import from shared location
- Single source of truth for pattern mining logic

### Pattern Storage
- In-memory storage, namespace-scoped
- Patterns persist for lifetime of integration instance
- Same approach as VictoriaLogs — no shared cross-backend storage

### Claude's Discretion
- Exact file organization within internal/logprocessing/
- Error handling specifics for Logz.io API failures during pattern fetch
- Any performance optimizations for pattern comparison

</decisions>

<specifics>
## Specific Ideas

- "Consistent AI experience across backends" — an AI using VictoriaLogs patterns tool should be able to use Logz.io patterns tool without learning new parameters or output format
- Refactoring Drain to common location is preparation for future backends

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 13-mcp-tools-patterns*
*Context gathered: 2026-01-22*
