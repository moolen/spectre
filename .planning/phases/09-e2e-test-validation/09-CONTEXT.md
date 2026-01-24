# Phase 9: E2E Test Validation - Context

**Gathered:** 2026-01-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Update existing E2E tests to work with the consolidated server architecture from Phases 6-8. Tests verify MCP HTTP transport works on port 8080 at /v1/mcp endpoint. Config reload tests verify integration hot-reload in consolidated mode.

</domain>

<decisions>
## Implementation Decisions

### Test coverage scope
- Update existing tests to point at new MCP endpoint — do not write new tests
- Focus on happy path — existing mcp_failure_scenarios tests cover error handling
- Delete stdio transport tests (mcp_stdio_test.go, mcp_stdio_stage_test.go) — `spectre mcp` command was removed in Phase 8
- Keep existing tool coverage: cluster_health, prompts, MCP protocol operations

### Test environment setup
- Use dedicated test namespace — tests deploy their own spectre instance
- Update port from 8082 to 8080 — MCP now integrated on main server
- Use existing test infrastructure — FalkorDB/VictoriaLogs already in kind cluster
- Helm fixtures already updated in Phase 8 — use as-is

### Assertion strategy
- Keep existing assertions: tool result has 'content', isError is false, prompts have 'messages'
- Update MCPClient to use /v1/mcp instead of /mcp path
- Keep current tool count assertion (5 tools)
- No additional schema validation needed

### CI/CD integration
- Keep existing CI setup — tests run with make test-e2e
- No coverage tracking changes — deleted stdio tests naturally reduce count
- Keep current timeouts (30s for tool calls)

### Claude's Discretion
- Exact changes to shared_setup.go for port forwarding
- Whether to consolidate MCP-specific deployment helpers
- Any test file cleanup beyond stdio removal

</decisions>

<specifics>
## Specific Ideas

- MCPClient in helpers/mcp_client.go sends to `/mcp` — change to `/v1/mcp`
- mcp_http_stage_test.go port-forwards to port 8082 — change to 8080
- Delete mcp_stdio_test.go and mcp_stdio_stage_test.go completely
- Delete helpers/mcp_subprocess.go (only used by stdio tests)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 09-e2e-test-validation*
*Context gathered: 2026-01-21*
