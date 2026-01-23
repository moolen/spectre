---
phase: 21
plan: 02
subsystem: grafana-integration
tags: [alerts, state-sync, deduplication, lifecycle, testing]

requires:
  - "21-01: Alert state API (GetAlertStates) and graph methods (CreateStateTransitionEdge, getLastKnownState)"
  - "20-02: AlertSyncer pattern for lifecycle management"

provides:
  - "Periodic alert state monitoring with 5-minute sync interval"
  - "State transition deduplication (only store actual changes)"
  - "Per-alert staleness tracking via last_synced_at timestamps"
  - "Integration lifecycle wiring for automatic state monitoring"

affects:
  - "21-03: MCP tools will query state transitions from graph"
  - "Future phases: State timeline provides context for alert analysis"

tech-stack:
  added: []
  patterns:
    - "State aggregation: worst-case across alert instances"
    - "Deduplication: compare previous state before creating edge"
    - "Graceful degradation: continue sync on partial failures"
    - "Independent timers: AlertSyncer (1h) vs AlertStateSyncer (5m)"

key-files:
  created:
    - "internal/integration/grafana/alert_state_syncer.go (273 lines)"
    - "internal/integration/grafana/alert_state_syncer_test.go (486 lines)"
  modified:
    - "internal/integration/grafana/dashboard_syncer.go (added GetAlertStates to interface)"
    - "internal/integration/grafana/grafana.go (added stateSyncer lifecycle wiring)"
    - "internal/integration/grafana/alert_syncer_test.go (added GetAlertStates stub)"
    - "internal/integration/grafana/dashboard_syncer_test.go (added GetAlertStates stub)"

decisions:
  - id: state-aggregation
    what: "Aggregate alert instance states to worst case: firing > pending > normal"
    why: "Matches Grafana's alert rule evaluation model - alert is firing if any instance fires"
    alternatives: ["Per-instance state tracking", "Majority vote aggregation"]

  - id: deduplication-strategy
    what: "Deduplicate by comparing current state vs last known state from graph"
    why: "Prevents storing redundant consecutive same-state syncs, reduces storage"
    impact: "Only actual state transitions create edges, skipped syncs don't pollute timeline"

  - id: staleness-granularity
    what: "Per-alert last_synced_at timestamp (not global)"
    why: "Enables AI to detect which alerts have stale state data after API errors"
    alternatives: ["Global timestamp", "No staleness tracking"]

  - id: error-handling-philosophy
    what: "Partial failures OK - log warning, continue with other alerts"
    why: "One alert's graph error shouldn't block state monitoring for all alerts"
    impact: "System degrades gracefully under partial failure conditions"

metrics:
  duration: "8 minutes"
  completed: "2026-01-23"
  commits: 3
  files_created: 2
  files_modified: 4
  test_coverage: "6 test cases covering deduplication, aggregation, lifecycle"
---

# Phase 21 Plan 02: Alert State Syncer Service Summary

**One-liner:** Periodic alert state monitoring with 5-minute sync interval, deduplication, per-alert staleness tracking, and integration lifecycle wiring.

## What Was Built

### AlertStateSyncer Core (alert_state_syncer.go)

**Type:** `AlertStateSyncer` struct following `AlertSyncer` pattern
- **Fields:** client, graphClient, builder, integrationName, logger
- **Lifecycle:** ctx, cancel, stopped channel for graceful shutdown
- **Thread-safe state:** mu, lastSyncTime, transitionCount, lastError, inProgress
- **Default interval:** 5 minutes (configurable via syncInterval field)

**Constructor:** `NewAlertStateSyncer` with 5-minute default interval

**Start/Stop methods:**
- Start: initial sync + background loop with ticker
- Stop: cancel context, wait for stopped channel with 5s timeout
- syncLoop: periodic sync triggered by ticker

**syncStates method (core logic):**
1. Call `client.GetAlertStates(ctx)` to fetch current alert states
2. For each alert, aggregate instance states to worst case (firing > pending > normal)
3. Call `builder.getLastKnownState(ctx, alertUID)` to get previous state
4. Compare current vs last state:
   - If different: call `builder.CreateStateTransitionEdge` with from/to states
   - If same: skip edge creation (deduplication), log "skipped (no change)"
5. Update per-alert `last_synced_at` timestamp on successful sync
6. Track metrics: transitionCount (only actual transitions, not skipped)
7. Log summary: "X transitions stored, Y skipped (no change), Z errors"

**aggregateInstanceStates method:**
- Priority: firing/alerting > pending > normal
- Returns "normal" for empty instances array
- Handles both "firing" and "alerting" state names (treats as same)

**updateLastSyncedAt method:**
- Updates `a.last_synced_at` timestamp in Alert node
- Uses MERGE to handle race with rule sync (alert might not exist yet)
- Per-alert granularity (not global timestamp)

**Error handling:**
- On API error: log warning, set lastError, DON'T update lastSyncTime (staleness)
- On graph error for individual alert: log warning, continue with other alerts
- Partial failures OK - sync what succeeded, return error count at end

### Unit Tests (alert_state_syncer_test.go)

**Test coverage:**
1. **TestAlertStateSyncer_SyncStates_Initial:** Verify initial transitions created for alerts with no previous state (getLastKnownState returns "unknown")
2. **TestAlertStateSyncer_SyncStates_Deduplication:** Verify no edge created when state unchanged (firing -> firing)
3. **TestAlertStateSyncer_SyncStates_StateChange:** Verify transition edge created with correct from/to states (normal -> firing)
4. **TestAlertStateSyncer_SyncStates_APIError:** Verify error handling (lastSyncTime not updated on API failure)
5. **TestAlertStateSyncer_AggregateInstanceStates:** 6 sub-tests verify state aggregation logic
   - firing has highest priority
   - pending has medium priority
   - all normal
   - empty instances defaults to normal
   - "alerting" state treated as "firing"
   - firing overrides pending
6. **TestAlertStateSyncer_StartStop:** Verify lifecycle (Start/Stop work correctly, stopped channel closes)

**Mock implementation:**
- `mockGrafanaClientForStates` with `getAlertStatesFunc` callback
- `mockGraphClientForStates` with `executeQueryFunc` callback
- Query detection using `strings.Contains` for key phrases:
  - "RETURN t.to_state" → getLastKnownState
  - "SET a.last_synced_at" → updateLastSyncedAt
  - `from_state` parameter → CreateStateTransitionEdge

**Mock updates for existing tests:**
- Added `GetAlertStates()` method to `mockGrafanaClientForAlerts` (alert_syncer_test.go)
- Added `GetAlertStates()` method to `mockGrafanaClient` (dashboard_syncer_test.go)
- Required after adding GetAlertStates to GrafanaClientInterface

### Integration Lifecycle Wiring (grafana.go)

**Struct changes:**
- Added `stateSyncer *AlertStateSyncer` field to GrafanaIntegration

**Start method changes:**
- After AlertSyncer starts, create and start AlertStateSyncer
- Share same GraphBuilder instance (already created for AlertSyncer)
- Comment: "Alert state syncer runs independently from rule syncer (5-min vs 1-hour interval)"
- Non-fatal: if Start fails, log warning but continue (alert rules still work)

**Stop method changes:**
- Stop AlertStateSyncer before AlertSyncer (reverse order)
- Log "Stopping alert state syncer for integration {name}"
- Clear stateSyncer reference on shutdown

**Independent operation:**
- AlertSyncer: 1-hour interval, syncs rule definitions
- AlertStateSyncer: 5-minute interval, syncs current state
- No coordination needed between syncers (MERGE handles races)

### Interface Updates (dashboard_syncer.go)

**GrafanaClientInterface:**
- Added `GetAlertStates(ctx context.Context) ([]AlertState, error)` method
- Required to use client.GetAlertStates in AlertStateSyncer
- GrafanaClient already implements this (from plan 21-01)

## Deviations from Plan

None - plan executed exactly as written.

## Lessons Learned

### Test Mock Design
**Challenge:** Detecting different graph query types (getLastKnownState vs updateLastSyncedAt) with similar parameters.

**Solution:** Use `strings.Contains(query.Query, "key phrase")` to identify queries by content:
- "RETURN t.to_state" → getLastKnownState
- "SET a.last_synced_at" → updateLastSyncedAt
- `from_state` parameter → CreateStateTransitionEdge

**Lesson:** For complex mocks, content-based detection is more reliable than parameter-based detection when parameters overlap.

### Error Handling Philosophy
**Approach:** Partial failures are acceptable - log warnings but continue with other alerts.

**Rationale:**
- One alert's graph error shouldn't block state monitoring for all alerts
- Grafana API might return partial data (some alerts succeed, some fail)
- System degrades gracefully under partial failure conditions

**Implementation:** Track error count, log warnings per alert, return aggregate error at end.

## Next Phase Readiness

**Ready for 21-03 (MCP tools):**
- ✅ State transitions stored in graph with 7-day TTL
- ✅ Per-alert last_synced_at timestamps enable staleness detection
- ✅ Deduplication ensures clean timeline (only actual state changes)
- ✅ State aggregation matches Grafana's alert rule model

**MCP tool requirements:**
- Query state transitions: `MATCH (a:Alert {uid: $uid})-[t:STATE_TRANSITION]->(a) WHERE t.expires_at > $now RETURN t ORDER BY t.timestamp`
- Check staleness: Compare `a.last_synced_at` timestamp age
- Filter by state: `WHERE t.to_state = 'firing'` for active alerts

**No blockers:** All phase 21-03 dependencies satisfied.

## Performance Notes

**Sync interval:** 5 minutes per CONTEXT.md decision
- Captures state changes with reasonable granularity
- Independent from AlertSyncer (1-hour interval)
- Future optimization: could increase frequency if needed

**Deduplication efficiency:**
- Prevents redundant edges for consecutive same-state syncs
- Reduces storage: only store ~5-10 transitions per alert over 7 days (vs ~2016 without deduplication)
- Estimated savings: 99.5% reduction in edge count for stable alerts

**Staleness tracking:**
- Per-alert granularity enables targeted re-sync on API recovery
- No global "stale" flag - AI interprets timestamp age
- Future optimization: could trigger immediate sync on Grafana API recovery

## Testing Evidence

```
=== RUN   TestAlertStateSyncer_SyncStates_Initial
--- PASS: TestAlertStateSyncer_SyncStates_Initial (0.00s)
=== RUN   TestAlertStateSyncer_SyncStates_Deduplication
--- PASS: TestAlertStateSyncer_SyncStates_Deduplication (0.00s)
=== RUN   TestAlertStateSyncer_SyncStates_StateChange
--- PASS: TestAlertStateSyncer_SyncStates_StateChange (0.00s)
=== RUN   TestAlertStateSyncer_SyncStates_APIError
--- PASS: TestAlertStateSyncer_SyncStates_APIError (0.00s)
=== RUN   TestAlertStateSyncer_AggregateInstanceStates
--- PASS: TestAlertStateSyncer_AggregateInstanceStates (0.00s)
=== RUN   TestAlertStateSyncer_StartStop
--- PASS: TestAlertStateSyncer_StartStop (0.10s)
PASS
ok  	github.com/moolen/spectre/internal/integration/grafana	0.110s
```

All tests pass, covering:
- Initial state transitions (unknown → current state)
- Deduplication (no edge on unchanged state)
- State changes (create edge with correct from/to)
- API error handling (lastSyncTime not updated)
- State aggregation (6 scenarios)
- Lifecycle management (Start/Stop)

## Commits

1. **36d9f1d** feat(21-02): create AlertStateSyncer with deduplication
   - AlertStateSyncer struct with 5-minute sync interval
   - State aggregation and deduplication logic
   - Per-alert last_synced_at timestamp tracking
   - Add GetAlertStates to GrafanaClientInterface

2. **caa156e** test(21-02): add AlertStateSyncer unit tests
   - 6 test cases covering all functionality
   - Mock implementations for state sync testing
   - Update existing mocks to implement GetAlertStates

3. **48fb79b** feat(21-02): wire AlertStateSyncer into integration lifecycle
   - Add stateSyncer field to GrafanaIntegration
   - Start/Stop AlertStateSyncer with proper lifecycle
   - Independent timers (1h vs 5m)
   - Non-fatal failure handling

---

**Phase:** 21-alert-sync-pipeline
**Plan:** 02
**Completed:** 2026-01-23
**Duration:** 8 minutes
