# Sensitive Data Scrubbing Design

Date: 2026-04-14
Status: Proposed

## Summary

Add a boolean CLI flag, `--scrub-sensitive-data`, to `spectre server` so Spectre scrubs sensitive values before writing resource data to storage. The scrubbing is ingest-time only: once enabled, live watcher ingestion and JSON imports both persist scrubbed payloads, and all downstream readers such as timeline, export, search, and gRPC inherit the safer data automatically.

The scrubber preserves object structure and key names so the data remains useful for debugging and timeline analysis. Only targeted value fields are transformed.

## Goals

- Prevent raw secret and config values from being persisted when the flag is enabled.
- Keep scrubbed values partially readable so operators can still recognize resources and changes.
- Apply one consistent scrubbing policy to both live watcher ingestion and imported events.
- Avoid any changes to query, storage, export, or response formats beyond the already-scrubbed values.

## Non-Goals

- Heuristically detect all possible secret-like values anywhere in resource JSON.
- Rename keys such as `JWT_SECRET` or `DATABASE_URL`.
- Change query APIs, export formats, or resource schemas.
- Retroactively scrub already persisted historical data.

## CLI Surface

Add a new server flag:

```text
spectre server --scrub-sensitive-data
```

Behavior:

- Default: `false`
- When `false`, Spectre behaves exactly as it does today.
- When `true`, Spectre scrubs supported sensitive fields before creating stored events.

This is intentionally a simple boolean. There are no policy levels or per-kind overrides in the initial version.

## Architecture

### Recommended approach

Introduce a shared ingest-time scrubber component and call it from every path that writes events to storage.

Suggested package:

```text
internal/scrub
```

Suggested API shape:

```go
type Scrubber struct {
    enabled bool
}

func New(enabled bool) *Scrubber
func (s *Scrubber) Enabled() bool
func (s *Scrubber) ScrubEventData(kind string, data json.RawMessage) (json.RawMessage, error)
```

### Integration points

1. `cmd/spectre/commands/server.go`
   - register `--scrub-sensitive-data`
   - pass the flag into runtime config and component wiring

2. `internal/config/config.go`
   - add `ScrubSensitiveData bool`

3. `internal/watcher/event_handler.go`
   - after marshaling and pruning managed fields, run the scrubber on the JSON payload
   - store the scrubbed result in `models.Event.Data`

4. `internal/importexport/json_import.go`
   - scrub each imported event payload before calling `AddEventsBatch`
   - use the same scrubber implementation as the watcher path

### Rationale

This boundary is preferred over response-time or storage-layer scrubbing because it:

- guarantees exports inherit safe data automatically
- keeps one implementation for watcher and import ingestion
- avoids duplicating scrubbing logic in timeline/search/gRPC/export handlers
- preserves current storage and query behavior because object shape stays intact

## Scrubbing Scope

When `--scrub-sensitive-data` is enabled, scrub the following fields:

### Secret

- `data`
- `stringData`

### ConfigMap

- `data`
- `binaryData`

### Workload env values

Scrub explicit environment variable values in workload specs:

- `spec.template.spec.containers[].env[].value`
- `spec.template.spec.initContainers[].env[].value`
- `spec.template.spec.ephemeralContainers[].env[].value`

Do not modify:

- `valueFrom`
- `envFrom`
- `secretKeyRef`
- `configMapKeyRef`
- names of env vars

### Nested JSON annotations

Always recurse into:

- `metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"]`

If that annotation contains valid JSON, parse it, apply the same scrubbing rules recursively, and write the scrubbed JSON back as a string annotation value.

If parsing fails, scrub the annotation value as a plain string instead of failing the entire event.

## Masking Rules

Scrubbing must preserve structure and approximate readability while removing most of the original value.

Rules:

- preserve original string length
- keep a small visible prefix and suffix
- replace the middle with `*`
- never leave a non-empty value fully visible

Concrete masking policy:

- length `<= 4`: keep first 1 character, mask the rest
- length `5..8`: keep first 1 and last 1 characters, mask the middle
- length `>= 9`: keep first 3 and last 2 characters, mask the middle

Examples:

- `abcd` -> `a***`
- `secret12` -> `s******2`
- `sk_test_fake_key_payment` -> `sk_*****************nt`
- `demo_jwt_secret_key` -> `dem************ey`

For base64-backed fields such as `Secret.data` and `binaryData`:

- attempt base64 decode first
- if decode succeeds, scrub the decoded plaintext and then re-encode to base64
- if decode fails, scrub the original string in place

This keeps the field syntactically usable while preventing recovery of the raw value from storage.

## JSON Handling Rules

The scrubber must modify values through structured JSON traversal, not raw substring replacement.

Requirements:

- preserve all non-sensitive fields unchanged
- preserve JSON object layout and array structure
- leave fields absent if they were absent originally
- avoid introducing new metadata or marker fields

Supported object forms:

- top-level Kubernetes resource JSON
- nested object JSON carried inside the last-applied annotation

## Error Handling

When the flag is disabled, no scrubber errors can occur because no scrubbing work is attempted.

When the flag is enabled:

- if top-level resource JSON cannot be parsed for scrubbing, treat ingestion as failed and do not write the event
- if a nested annotation JSON blob cannot be parsed, scrub it as a plain string and continue
- if base64 decode fails for `Secret.data` or `binaryData`, scrub the encoded string and continue

This keeps the system fail-closed for top-level persisted payloads while still being tolerant of malformed nested values.

## Import Behavior

Imported JSON event files must use the same scrubbing path as live watcher ingestion.

Behavior:

- if scrubbing is disabled, imports remain unchanged
- if scrubbing is enabled, imported `models.Event.Data` payloads are scrubbed before batch persistence
- existing enrichment such as `involvedObjectUID` extraction continues to work on the scrubbed payloads

## Compatibility Considerations

- Resource builder, status inference, and timeline rendering should continue to work because status-relevant structure remains intact.
- Search results should be unaffected because search currently returns minimal resource metadata.
- Exports automatically contain scrubbed payloads because storage only contains scrubbed data.
- Existing stored historical segments remain untouched; only newly ingested data is scrubbed.

## Testing Strategy

### Unit tests

Add scrubber-focused unit tests covering:

- `Secret.data`
- `Secret.stringData`
- `ConfigMap.data`
- `ConfigMap.binaryData`
- container `env[].value`
- `initContainers` and `ephemeralContainers`
- recursive scrubbing of `kubectl.kubernetes.io/last-applied-configuration`
- masking edge cases for short, medium, and long values
- malformed nested annotation JSON fallback
- base64 decode success and fallback behavior

### Integration-path tests

Add path-level tests covering:

- watcher ingestion with scrubbing enabled writes scrubbed payloads
- watcher ingestion with scrubbing disabled writes unchanged payloads
- import ingestion with scrubbing enabled scrubs before `AddEventsBatch`
- import ingestion with scrubbing disabled leaves payloads unchanged

### Regression tests

Confirm that existing consumers still accept scrubbed payloads:

- resource builder produces valid status segments from scrubbed payloads
- timeline responses remain structurally valid
- export remains readable and contains scrubbed rather than raw values

## Rollout Notes

- default remains off to avoid changing persistence behavior unexpectedly
- documentation for `spectre server --help` and operator docs should be updated during implementation
- migration of existing historical data is explicitly out of scope for this change

## Open Questions Resolved

- Scrubbing occurs before storage, not just at presentation time.
- The CLI uses a single boolean flag, not multiple policy levels.
- The initial version targets known sensitive fields rather than heuristic secret detection.
