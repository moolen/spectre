# Phase 11: Secret File Management - Context

**Gathered:** 2026-01-22
**Status:** Ready for planning

<domain>
## Phase Boundary

File-based secret storage with hot-reload for zero-downtime credential rotation. This phase implements the infrastructure for securely fetching and watching API tokens from Kubernetes Secrets.

**Pivot from original plan:** Instead of mounting secrets as files, Spectre will fetch secrets directly from the Kubernetes API server. The user specifies `secretName` and `key` in the integration config; Spectre fetches the secret, extracts the key, and uses it for authentication. Watch API provides hot-reload on secret rotation.

</domain>

<decisions>
## Implementation Decisions

### Secret Source
- Fetch directly from Kubernetes API server (not file mount)
- Secret is by convention in the same namespace as Spectre
- Config specifies `secretName` and `key` within that secret
- Use Kubernetes Watch API for immediate notification on changes

### Token Format
- Raw token value only (no JSON wrapper, no key-value format)
- Trim leading/trailing whitespace including newlines
- Accept whatever is stored in the Secret's key

### Error Behavior - Missing Secret
- Start in degraded state (don't fail startup)
- Mark integration unhealthy
- Watch will pick up secret when created

### Error Behavior - Missing Key
- Clear error message: "key X not found in Secret Y, available keys: [a, b, c]"
- Helps user debug misconfiguration

### Error Behavior - Empty Token
- Treat empty/whitespace-only token as missing
- Go degraded, mark unhealthy

### Error Behavior - Watch Failure
- Retry with exponential backoff
- Continue using cached token during reconnection
- Standard Kubernetes client reconnection behavior

### Observability - Success
- INFO log on successful token rotation: "Token rotated for integration X"
- No metrics for now (keep it simple)

### Observability - Failure
- WARN log per failed fetch attempt with reason
- No log throttling - each retry logs

### Observability - Token Masking
- Token values NEVER appear in logs
- Replace with [REDACTED] in any debug output

### Health Status
- Integration unhealthy if no valid token
- Health endpoint reflects token state

### Degraded Mode - MCP Tools
- Return error: "Integration X is degraded: missing API token"
- Don't return empty results

### Degraded Mode - Auth Failure (401)
- Fail the request, return error to caller
- Mark integration degraded
- Don't auto-retry with refresh

### Degraded Mode - UI
- Status badge showing "Degraded"
- Hover text explains the issue

### Degraded Mode - Recovery
- Auto-heal when valid token obtained
- Watch detects secret update, fetches new value, marks healthy

### Claude's Discretion
- Exact exponential backoff parameters
- Watch implementation details (informer vs raw watch)
- Thread-safety mechanism for token updates
- Kubernetes client library choice

</decisions>

<specifics>
## Specific Ideas

- Follows standard Kubernetes operator pattern for secret consumption
- Secret in same namespace simplifies RBAC (only needs get/watch on secrets in own namespace)
- "I want it to just work when I rotate secrets - no pod restarts"

</specifics>

<deferred>
## Deferred Ideas

None - discussion stayed within phase scope

</deferred>

---

*Phase: 11-secret-file-management*
*Context gathered: 2026-01-22*
