# Phase 8: Cleanup & Helm Chart Update - Context

**Gathered:** 2026-01-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Remove standalone MCP command and update Helm chart for single-container deployment. This includes deleting dead code (mcp command, agent command, agent package), updating Helm chart to remove MCP sidecar, and updating documentation to reflect consolidated architecture.

</domain>

<decisions>
## Implementation Decisions

### CLI Removal Approach
- Silent removal of `spectre mcp` command — let Go show "unknown command"
- Silent removal of `spectre agent` command — same treatment
- Delete `internal/agent/` package entirely (currently excluded by build constraints)
- Clean deletion with no traces — git history preserves if needed
- No TODO comments, no deprecation stubs

### Helm Values Migration
- Old MCP values (mcp.enabled, mcp.port, etc.) silently ignored if present
- Remove mcp.port entirely — single port (8080), no separate MCP port config
- Add `mcp.path` option to allow customizing the MCP endpoint path (default: /v1/mcp)
- Remove MCP sidecar resource limits entirely — only main container resources

### Documentation Updates
- Update project README in this phase to reflect consolidated architecture
- No separate migration guide — changes are minor enough
- Minimal update to Helm chart README — remove MCP sidecar references, keep structure
- Update stale code comments referencing old MCP sidecar architecture

### Backward Compatibility
- Breaking change OK — v1.1 is a clean break, users must update configs
- No compatibility shim for old MCP endpoint (localhost:3000)
- No warning mechanism for old endpoint configs — connection fails, users update
- Minor version bump OK — v1.1 name already signals significant update

### Claude's Discretion
- Exact wording of updated documentation
- Which specific code comments to update
- Default value for mcp.path option

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches for cleanup and Helm chart updates.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 08-cleanup-helm-update*
*Context gathered: 2026-01-21*
