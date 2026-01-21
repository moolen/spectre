# Phase 4: Log Template Mining - Context

**Gathered:** 2026-01-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Automatic log clustering into templates using Drain algorithm for pattern detection without manual configuration. Logs are normalized, clustered into templates with stable hash IDs, and stored for use by Phase 5 MCP tools. This phase handles the processing pipeline — user-facing tools are Phase 5.

</domain>

<decisions>
## Implementation Decisions

### Template granularity
- Loose clustering (fewer templates) — aggressively group similar logs
- Target 100-500 templates per namespace (balanced, not overwhelming)
- Log level IS part of template — same message at INFO vs ERROR = different templates
- For JSON logs, extract and template the message/msg field only (ignore JSON structure)

### Variable masking
- Aggressive masking: IPs, UUIDs, timestamps, numbers, hex strings, file paths, URLs, email addresses
- Kubernetes-specific patterns get special treatment — pod names (app-xyz-abc123), deployment suffixes, replicaset hashes become `<K8S_NAME>`
- Preserve HTTP status codes and ports as literals — 'returned 404' vs 'returned 500' stay distinct
- Masking happens AFTER Drain clustering (post-tokenization) — cluster raw logs first, then identify variables in resulting templates

### Template lifecycle
- Count-based expiry — templates below occurrence threshold get pruned
- Low threshold (10+ occurrences) to stabilize — catches rare but important error patterns
- Auto-merge similar templates periodically to handle log format drift (self-healing)
- Templates scoped per-namespace — same log pattern in different namespaces = different template IDs

### Storage & persistence
- In-memory with periodic disk snapshots (simple, works for single instance)
- Persist every 5 minutes (balanced — lose at most 5 min on crash)
- JSON format for persistence (human-readable, debuggable)
- Start empty on first run (no bootstrap from VictoriaLogs, build from incoming logs)

### Claude's Discretion
- Exact Drain algorithm parameters (similarity threshold, tree depth, max clusters)
- Auto-merge detection algorithm and thresholds
- JSON field extraction patterns for message/msg identification
- Kubernetes name pattern regex specifics

</decisions>

<specifics>
## Specific Ideas

- "Loose clustering" means prioritizing groupability over precision — when in doubt, merge templates
- HTTP status codes preserved because 404 vs 500 distinction is critical for debugging
- Per-namespace scoping keeps multi-tenant environments clean — one team's log patterns don't pollute another's template space
- Post-tokenization masking preserves Drain's ability to detect structure before normalizing variables

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 04-log-template-mining*
*Context gathered: 2026-01-21*
