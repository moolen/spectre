# Phase 2: Config Management & UI - Context

**Gathered:** 2026-01-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Users enable/configure integrations via UI backed by REST API. REST API endpoints for reading/writing integration configs. UI for integration enable/disable toggle and connection configuration. Config persistence to disk with hot-reload trigger.

</domain>

<decisions>
## Implementation Decisions

### REST API Design
- Endpoint structure: `/api/config/integrations` (nested under config namespace)
- RESTful: GET list, GET/PUT/DELETE by name
- Dedicated test endpoint: `POST /api/config/integrations/:name/test` — validates connection before saving
- Error format: JSON with code + message (`{"error": {"code": "INVALID_CONFIG", "message": "URL is required"}}`)
- Validation returns all errors at once (not fail-fast) — better for UI consumption

### UI Layout & Flow
- Use existing `IntegrationsPage.tsx` (not a new page)
- **Add Integration flow:**
  1. "+ Add Integration" button at top right corner
  2. Modal opens: dropdown to choose integration type → Next/Cancel buttons
  3. Next brings user to integration-specific config form
  4. Save button tests connection first (via test endpoint)
  5. If test fails: show warning but allow save anyway (useful for pre-staging)
- **Existing integrations view:**
  - Stub tiles disappear once integrations exist
  - Table replaces tiles showing: Name, Type, URL/Endpoint, Date Added, Health Status
  - Click table row to open edit/delete view
- Health status display: Color dot + text ("Healthy", "Degraded", "Offline")
- Form validation: on submit only (not real-time)

### Config Persistence
- File format: YAML
- Single file: `integrations.yaml` (all integrations in one file)
- Location: Same directory as main Spectre config
- Atomic writes: Write to temp file, then rename (prevents corruption)

### Integration List Display
- Table columns: Name, Type, URL/Endpoint, Date Added, Status
- Ordering: Grouped by integration type, then sorted by name (grouping not visually separated)
- No column sorting needed
- Delete only via edit page (not quick-action in table) — prevents accidental deletes

### Claude's Discretion
- Exact modal styling and animations
- Form field layouts within config forms
- Loading states during connection test
- Error message wording

</decisions>

<specifics>
## Specific Ideas

- Reuse existing Spectre UI component patterns from IntegrationsPage.tsx
- Config test endpoint provides "save with warning" UX — user can stage configs before target is reachable
- Table view is the primary interface once integrations exist (tiles are just empty state)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-config-management-ui*
*Context gathered: 2026-01-21*
