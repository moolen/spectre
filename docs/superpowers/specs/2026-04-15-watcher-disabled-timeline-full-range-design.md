# Watcher-Disabled Timeline Full-Range Design

Date: 2026-04-15
Status: Proposed

## Summary

When Spectre runs with `--watcher-enabled=false`, the timeline UI should default to the full available event window instead of a recent relative range. On first load of the timeline page, if the URL does not already provide `start` and `end`, the UI should use the timestamp of the first available event as the timeline start and the timestamp of the last available event as the timeline end.

This change is intended for offline and CI-investigation workflows where Spectre is started against already-imported data and no new watcher events will arrive.

## Goals

- Make the initial timeline view useful for read-only and imported-data workflows.
- Preserve explicit user or shared URL ranges without overriding them.
- Keep the current live-watcher startup behavior unchanged.
- Reuse existing backend metadata rather than adding a new timeline query just to discover bounds.

## Non-Goals

- Change behavior when `start` and `end` are already present in the URL.
- Change demo mode startup behavior.
- Change timeline behavior after the user manually selects, zooms, or edits a range.
- Infer "static dataset" heuristically from timestamps alone.

## User-Facing Behavior

### Default startup behavior

On timeline page load:

- if the URL already contains both `start` and `end`, preserve them exactly
- else if Spectre is running with watcher enabled, keep the existing startup behavior
- else if Spectre is running with watcher disabled and metadata reports available events, initialize the URL time range to the full dataset bounds
- else if Spectre is running with watcher disabled and metadata reports no available events, keep the existing empty picker flow

### Full-range definition

For watcher-disabled startup, "all available data" means:

- `start = first event timestamp`
- `end = last event timestamp`

These values come from backend metadata time bounds, not from the rendered resource list.

### URL precedence

Explicit URL parameters always win. This includes:

- bookmarked investigation links
- links shared between users
- ranges produced by the time picker
- ranges produced by timeline zoom/pan

The watcher-disabled auto-range logic runs only when the page opens without `start` and `end`.

## Recommended Approach

Expose watcher runtime mode through the existing `/health` response and let the timeline page choose its initial range based on that signal plus `/v1/metadata`.

This is preferred over frontend inference because the UI currently has no reliable way to distinguish:

- watcher-disabled static data, from
- watcher-enabled live mode with older or sparse data

An explicit backend flag keeps the decision deterministic and easy to reason about.

## Architecture

### Backend

Extend the `/health` response to include:

```json
{
  "status": "healthy",
  "demo": false,
  "watcherEnabled": false
}
```

The API server already knows whether the watcher component was enabled during startup, so this is the correct source of truth.

Suggested changes:

1. `cmd/spectre/commands/server.go`
   - pass watcher-enabled state into API server construction

2. `internal/api/server.go`
   - store watcher-enabled runtime state on `Server`
   - include `watcherEnabled` in `/health`

### Frontend bootstrap

The frontend already calls `/health` before rendering in `ui/src/index.tsx`. Extend that runtime bootstrap so it caches both:

- `demo`
- `watcherEnabled`

Suggested changes:

1. `ui/src/services/api.ts`
   - replace the demo-only runtime cache with a small runtime mode cache
   - keep `getDemoMode()` behavior intact
   - add `getWatcherEnabled()` or equivalent accessor

2. `ui/src/index.tsx`
   - keep the current pre-render health detection flow
   - no extra startup request beyond the existing `/health` call

### Timeline startup flow

Add a first-load initialization path in `ui/src/pages/TimelinePage.tsx`:

1. Read `start` and `end` from the URL.
2. If both exist, do nothing.
3. If demo mode is active, keep existing demo handling.
4. If watcher is enabled, keep existing default handling.
5. If watcher is disabled, fetch `/v1/metadata` without a user-selected time range.
6. If metadata returns valid non-zero bounds, set URL params to those bounds.
7. Let existing URL parsing and `useTimeline()` logic proceed normally.

This keeps the URL as the single source of truth for the rest of the page.

## Data Flow

Watcher-disabled startup without URL params:

1. Browser loads app.
2. Frontend calls `/health`.
3. Runtime mode cache stores `demo` and `watcherEnabled`.
4. Timeline page sees there is no `start`/`end`.
5. Timeline page sees `watcherEnabled=false`.
6. Timeline page calls `/v1/metadata`.
7. Backend returns `timeRange.earliest` and `timeRange.latest`.
8. Timeline page writes those timestamps into the URL.
9. Existing time-range parsing and timeline fetching use that full-range URL.

Startup with URL params:

1. Browser loads app.
2. Frontend calls `/health`.
3. Timeline page sees `start` and `end` already exist.
4. No watcher-disabled auto-initialization occurs.

## Metadata Source

The existing `/v1/metadata` response already includes:

- `timeRange.earliest`
- `timeRange.latest`

Those bounds should be reused for full-range initialization. No new endpoint is required for the initial version.

## Edge Cases

### No data

If metadata reports no events, do not invent a synthetic range. Keep the current time-range picker behavior so the UI remains understandable and avoids showing a misleading empty full-range URL.

### Single-event dataset

If `earliest == latest`, the UI should still produce a usable range.

Recommended behavior:

- pad by 1 second on each side before writing URL params

This avoids a zero-width range, which current validation rejects because `start` must be less than `end`.

### Invalid metadata bounds

If metadata returns:

- zero timestamps
- negative timestamps
- `latest < earliest`

then skip auto-initialization and keep the current picker flow. The UI should fail open to the existing manual range selection rather than writing a broken URL.

### Auto-refresh

Auto-refresh settings do not need special treatment for this feature. The initial range changes, but once the page is initialized the existing refresh behavior can remain unchanged.

## Alternatives Considered

### Infer from metadata only

Have the UI always fetch metadata and auto-select the full data range whenever bounds are available.

Rejected because it changes live-mode behavior and makes watcher mode implicit instead of explicit.

### Infer from readiness or lack of new events

Have the UI guess watcher-disabled mode based on readiness checks or stagnant data.

Rejected because this is ambiguous and would produce surprising behavior in sparse but live clusters.

### Add a dedicated startup config endpoint

Introduce a new endpoint for frontend runtime mode and startup rules.

Rejected because `/health` already exists and is already called during frontend boot.

## Testing Strategy

### Backend tests

Add or update API tests covering:

- `/health` includes `watcherEnabled=true` when watcher is enabled
- `/health` includes `watcherEnabled=false` when watcher is disabled
- existing `status` and `demo` behavior remains unchanged

### Frontend unit tests

Add tests covering startup initialization logic:

- URL params present: no override
- watcher enabled: no full-range auto-init
- watcher disabled with valid metadata bounds: URL set to full range
- watcher disabled with no data: no auto-init
- watcher disabled with single-event bounds: padded range is used

### UI/integration tests

Add a timeline UI test for the offline investigation flow:

- start Spectre with watcher disabled
- import or provide historical data
- open timeline without `start` and `end`
- verify the page initializes to the earliest and latest event timestamps

Add a regression test:

- open timeline with explicit `start` and `end` while watcher is disabled
- verify the provided range remains untouched

## Implementation Notes

- Prefer implementing startup initialization in `TimelinePage` rather than inside `useTimeline`, because this is a routing concern and should happen before data-fetch hooks run.
- Keep the change minimal: expose one backend boolean and reuse existing metadata time bounds.
- Avoid tying this feature to persisted quick presets; explicit URL state should remain higher priority than local preset state.

## Open Questions Resolved

- The feature applies only when no explicit `start` and `end` are present.
- Watcher-disabled mode should be determined explicitly by the backend.
- The full-range bounds should come from the first and last event timestamps.
- Demo mode should remain unchanged.
