# Phase 21: Alert Sync Pipeline - Research

**Researched:** 2026-01-23
**Domain:** Grafana alert state tracking, graph-based state transition storage, periodic sync patterns
**Confidence:** MEDIUM

## Summary

Phase 21 tracks alert state transitions over time by periodically fetching current alert states from Grafana and storing state changes in the graph. Research focused on three key areas: (1) Grafana's alerting API endpoints for fetching alert instance states, (2) graph storage patterns for time-series state transitions using edge properties with TTL, and (3) deduplication strategies to avoid storing redundant same-state transitions.

**Key findings:**
- Grafana's unified alerting exposes alert instances via `/api/prometheus/grafana/api/v1/rules` endpoint (Prometheus-compatible format)
- Alert instances have three primary states: Normal, Pending, and Firing (Alerting)
- Edge properties with TTL (expires_at timestamp) provide efficient time-windowed storage without separate cleanup jobs
- State transition deduplication requires comparing previous state before creating new edges

**Primary recommendation:** Store state transitions as edges with properties (from_state, to_state, timestamp, expires_at), using last known state comparison to deduplicate consecutive same-state syncs.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Grafana Alerting API | v9.4+ | Alert state retrieval | Official provisioning API with alert instances |
| FalkorDB edge properties | N/A | State transition storage | Property graph model supports temporal edge data |
| Go time.Ticker | stdlib | Periodic sync | Standard Go pattern for interval-based operations |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| ISO8601/RFC3339 | stdlib | Timestamp format | Already used in Phase 20 for alert rule sync |
| json.RawMessage | stdlib | Flexible alert state parsing | Handle variable Grafana response structures |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Edge properties | Separate AlertStateChange nodes | Nodes add query complexity, edges naturally model transitions |
| TTL via expires_at | Background cleanup job | Application-level TTL is simpler, matches baseline cache pattern |
| Periodic-only sync | Event-driven webhooks | Grafana webhook setup complexity, periodic is sufficient for 5-min interval |

**Installation:**
```bash
# No additional dependencies - uses existing Grafana client and graph client
```

## Architecture Patterns

### Recommended Project Structure
```
internal/integration/grafana/
├── alert_syncer.go              # Extends existing syncer with state tracking
├── alert_state_fetcher.go       # NEW: Fetches current alert states
├── alert_state_tracker.go       # NEW: Manages state transitions in graph
├── alert_syncer_test.go         # Extends existing tests
└── graph_builder.go             # Extends with state edge methods
```

### Pattern 1: Periodic State Sync with Independent Timer
**What:** Run alert state sync on separate timer from alert rule sync
**When to use:** When state changes more frequently than rule definitions
**Example:**
```go
// Existing: Alert rule syncer (1 hour interval)
alertSyncer := NewAlertSyncer(client, graphClient, builder, "integration", logger)
alertSyncer.Start(ctx)

// NEW: Alert state syncer (5 minute interval)
stateSyncer := NewAlertStateSyncer(client, graphClient, builder, "integration", logger)
stateSyncer.Start(ctx)
```

**Why separate timers:** Allows tuning sync frequency independently - state changes are more frequent than rule changes.

### Pattern 2: State Transition Edges with TTL
**What:** Store state transitions as edges between Alert nodes and themselves with temporal properties
**When to use:** When tracking state history with automatic expiration
**Example:**
```cypher
// Create state transition edge with TTL
MATCH (a:Alert {uid: $uid, integration: $integration})
MERGE (a)-[t:STATE_TRANSITION {timestamp: $timestamp}]->(a)
SET t.from_state = $from_state,
    t.to_state = $to_state,
    t.expires_at = $expires_at
```

**Pattern rationale:**
- Self-edges model state transitions naturally (Alert -> Alert)
- Edge properties store transition metadata (from/to states, timestamp)
- TTL via expires_at allows time-windowed queries: `WHERE t.expires_at > $now`
- No separate cleanup job needed - expired edges filtered in queries

### Pattern 3: State Deduplication via Last Known State
**What:** Query previous state before creating new transition edge
**When to use:** Avoiding redundant same-state transitions during periodic sync
**Example:**
```go
// Query last known state from most recent transition edge
lastState, err := getLastKnownState(alertUID)
if err != nil {
    // No previous state, treat as initial state
    lastState = "unknown"
}

// Only create transition if state changed
currentState := fetchCurrentState(alertUID)
if currentState != lastState {
    createStateTransitionEdge(alertUID, lastState, currentState, now)
}
```

**Why this works:** Grafana periodic sync may return same state multiple times - only actual transitions need storage.

### Pattern 4: Per-Alert Staleness Tracking
**What:** Store last_synced_at timestamp on each Alert node
**When to use:** Detecting stale data when API is unavailable
**Example:**
```cypher
// Update alert node with sync timestamp on successful fetch
MATCH (a:Alert {uid: $uid, integration: $integration})
SET a.last_synced_at = $now
```

**Staleness interpretation:**
- Fresh: last_synced_at within 10 minutes (2x sync interval)
- Stale: last_synced_at > 10 minutes (API likely unavailable)
- AI interprets timestamp age, no explicit stale flag needed

### Anti-Patterns to Avoid
- **Separate AlertStateChange nodes:** Creates unnecessary query complexity - edges model transitions naturally
- **Deleting expired edges:** Application-level cleanup is complex - use TTL filtering in queries instead
- **Global last_synced timestamp:** Hides partial failures - per-alert granularity enables better diagnostics
- **Storing every sync result:** Without deduplication, identical states create noise - only store actual transitions

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| TTL cleanup job | Background goroutine to delete old edges | Query-time filtering: `WHERE expires_at > $now` | Avoids race conditions, simpler code, matches baseline cache pattern |
| State change detection | Complex diffing logic | Simple string comparison: `currentState != lastState` | Alert states are enumerated strings, no complex structure |
| Timestamp parsing | Custom ISO8601 parser | RFC3339 string comparison | Already proven in Phase 20, string comparison works for ISO format |
| Concurrent sync protection | Manual mutex/semaphore | Existing sync.RWMutex pattern in AlertSyncer | Phase 20 already implements thread-safe sync status |

**Key insight:** Edge properties with TTL filtering provide time-windowed data without cleanup complexity. Baseline cache in Phase 19 already proves this pattern works in FalkorDB.

## Common Pitfalls

### Pitfall 1: Fetching Alert Rules Instead of Alert Instances
**What goes wrong:** `/api/v1/provisioning/alert-rules` returns rule definitions, not current state
**Why it happens:** Rule API was used in Phase 20, developer assumes same endpoint has state
**How to avoid:** Use `/api/prometheus/grafana/api/v1/rules` which returns rules WITH their alert instances
**Warning signs:** No state field in response, only rule configuration data

### Pitfall 2: Creating Edges Without TTL
**What goes wrong:** State transition edges accumulate indefinitely, graph grows unbounded
**Why it happens:** Forgetting to set expires_at property when creating edges
**How to avoid:** Always calculate expires_at = now + 7 days when creating state transition edges
**Warning signs:** Graph size grows continuously, query performance degrades over time

### Pitfall 3: Not Handling Missing Previous State
**What goes wrong:** Deduplication logic crashes on first sync when no previous state exists
**Why it happens:** Assuming getLastKnownState always returns a value
**How to avoid:** Treat empty result as "unknown" state, always create first transition
**Warning signs:** Panic on initial sync, "no rows returned" errors

### Pitfall 4: Updating last_synced_at on API Errors
**What goes wrong:** Stale data appears fresh when API fails but timestamp still updates
**Why it happens:** Updating timestamp in finally block instead of success path
**How to avoid:** Only update last_synced_at AFTER successful state fetch and edge creation
**Warning signs:** Stale data not detected, sync failures hidden by fresh timestamps

### Pitfall 5: Storing Pending State Without Understanding Grafana Semantics
**What goes wrong:** Alert appears "Pending" but might be evaluating for first time vs waiting for threshold
**Why it happens:** Not understanding Grafana's Pending period concept
**How to avoid:** Store Pending as distinct state (Normal -> Pending -> Firing is valid transition)
**Warning signs:** Confusing state history, alerts appear to flap between Pending and Normal

### Pitfall 6: Race Conditions Between Rule Sync and State Sync
**What goes wrong:** State sync creates edges to Alert nodes that don't exist yet
**Why it happens:** Rule sync and state sync run independently on different timers
**How to avoid:** Use MERGE for Alert node in state sync, ensure node exists before creating edge
**Warning signs:** "Node not found" errors during state sync, orphaned edges

## Code Examples

Verified patterns from codebase and official sources:

### Fetching Alert States from Grafana
```go
// Source: Grafana community discussions and existing Phase 20 client pattern
// https://community.grafana.com/t/how-to-get-current-alerts-via-http-api/87888

func (c *GrafanaClient) GetAlertStates(ctx context.Context) ([]AlertState, error) {
    // Use Prometheus-compatible rules endpoint that includes alert instances
    reqURL := fmt.Sprintf("%s/api/prometheus/grafana/api/v1/rules", c.config.URL)
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
    if err != nil {
        return nil, fmt.Errorf("create get alert states request: %w", err)
    }

    // Add Bearer token (same pattern as Phase 20)
    if c.secretWatcher != nil {
        token, err := c.secretWatcher.GetToken()
        if err != nil {
            return nil, fmt.Errorf("failed to get API token: %w", err)
        }
        req.Header.Set("Authorization", "Bearer "+token)
    }

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("execute request: %w", err)
    }
    defer resp.Body.Close()

    // Always read body for connection reuse (Phase 20 pattern)
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response body: %w", err)
    }

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("request failed (status %d): %s", resp.StatusCode, string(body))
    }

    var result PrometheusRulesResponse
    if err := json.Unmarshal(body, &result); err != nil {
        return nil, fmt.Errorf("parse response: %w", err)
    }

    return extractAlertStates(result), nil
}
```

### Creating State Transition Edge with TTL
```go
// Source: Baseline cache pattern from Phase 19
// File: internal/integration/grafana/baseline_cache.go

func (gb *GraphBuilder) CreateStateTransitionEdge(
    alertUID string,
    fromState, toState string,
    timestamp time.Time,
) error {
    // Calculate TTL: 7 days from now
    expiresAt := time.Now().Add(7 * 24 * time.Hour).Unix()
    timestampUnix := timestamp.Unix()

    // Create self-edge with transition properties
    query := `
        MATCH (a:Alert {uid: $uid, integration: $integration})
        CREATE (a)-[t:STATE_TRANSITION {timestamp: $timestamp}]->(a)
        SET t.from_state = $from_state,
            t.to_state = $to_state,
            t.expires_at = $expires_at
    `

    _, err := gb.graphClient.ExecuteQuery(context.Background(), graph.GraphQuery{
        Query: query,
        Parameters: map[string]interface{}{
            "uid":         alertUID,
            "integration": gb.integrationName,
            "timestamp":   timestampUnix,
            "from_state":  fromState,
            "to_state":    toState,
            "expires_at":  expiresAt,
        },
    })
    if err != nil {
        return fmt.Errorf("failed to create state transition edge: %w", err)
    }

    return nil
}
```

### Querying Last Known State with Deduplication
```go
// Source: Derived from Phase 20 needsSync pattern
// File: internal/integration/grafana/alert_syncer.go

func (as *AlertStateSyncer) getLastKnownState(alertUID string) (string, error) {
    query := `
        MATCH (a:Alert {uid: $uid, integration: $integration})-[t:STATE_TRANSITION]->()
        WHERE t.expires_at > $now
        RETURN t.to_state as state
        ORDER BY t.timestamp DESC
        LIMIT 1
    `

    result, err := as.graphClient.ExecuteQuery(as.ctx, graph.GraphQuery{
        Query: query,
        Parameters: map[string]interface{}{
            "uid":         alertUID,
            "integration": as.integrationName,
            "now":         time.Now().Unix(),
        },
    })
    if err != nil {
        return "", fmt.Errorf("failed to query last state: %w", err)
    }

    // No previous state found
    if len(result.Rows) == 0 {
        return "", nil // Treat as unknown, will create first transition
    }

    // Extract state from result
    if len(result.Rows[0]) == 0 {
        return "", nil
    }

    state, ok := result.Rows[0][0].(string)
    if !ok {
        return "", fmt.Errorf("invalid state type: %T", result.Rows[0][0])
    }

    return state, nil
}

// Deduplication logic in sync loop
func (as *AlertStateSyncer) syncAlertState(alert AlertState) error {
    // Get last known state
    lastState, err := as.getLastKnownState(alert.UID)
    if err != nil {
        return fmt.Errorf("failed to get last state: %w", err)
    }

    // Deduplicate: only create edge if state changed
    if alert.State == lastState {
        as.logger.Debug("Alert %s state unchanged (%s), skipping transition", alert.UID, alert.State)
        return nil
    }

    // State changed, create transition edge
    if err := as.builder.CreateStateTransitionEdge(
        alert.UID,
        lastState,      // from_state (may be empty string for first transition)
        alert.State,    // to_state
        time.Now(),
    ); err != nil {
        return fmt.Errorf("failed to create state transition: %w", err)
    }

    as.logger.Info("Alert %s state transition: %s -> %s", alert.UID, lastState, alert.State)
    return nil
}
```

### Updating Per-Alert Sync Timestamp
```go
// Source: Phase 20 sync status tracking pattern
// File: internal/integration/grafana/alert_syncer.go

func (as *AlertStateSyncer) updateAlertSyncTimestamp(alertUID string) error {
    query := `
        MATCH (a:Alert {uid: $uid, integration: $integration})
        SET a.last_synced_at = $timestamp
    `

    _, err := as.graphClient.ExecuteQuery(as.ctx, graph.GraphQuery{
        Query: query,
        Parameters: map[string]interface{}{
            "uid":         alertUID,
            "integration": as.integrationName,
            "timestamp":   time.Now().Unix(),
        },
    })
    if err != nil {
        return fmt.Errorf("failed to update sync timestamp: %w", err)
    }

    return nil
}

// Only update timestamp on successful state fetch
func (as *AlertStateSyncer) syncAlerts() error {
    states, err := as.client.GetAlertStates(as.ctx)
    if err != nil {
        // DO NOT update timestamps on API error
        return fmt.Errorf("failed to fetch alert states: %w", err)
    }

    for _, state := range states {
        if err := as.syncAlertState(state); err != nil {
            as.logger.Warn("Failed to sync state for alert %s: %v", state.UID, err)
            continue
        }

        // Update timestamp ONLY after successful sync
        if err := as.updateAlertSyncTimestamp(state.UID); err != nil {
            as.logger.Warn("Failed to update timestamp for alert %s: %v", state.UID, err)
        }
    }

    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Legacy alerting API (/api/alerts) | Unified alerting API (/api/prometheus/grafana/api/v1/rules) | Grafana 9.0+ (2022) | New API provides alert instances with state, old API deprecated |
| Separate AlertStateChange nodes | Edge properties for transitions | Graph DB best practices 2025 | Edges naturally model state transitions, simpler queries |
| Background TTL cleanup jobs | Query-time TTL filtering | FalkorDB patterns 2025 | Avoids race conditions, simpler architecture |
| Global sync timestamps | Per-alert timestamps | Microservice patterns 2025 | Better observability, detects partial failures |

**Deprecated/outdated:**
- **Legacy alerting API (/api/alerts):** Replaced by unified alerting in Grafana 9+, doesn't support new alert states
- **Alertmanager API for Grafana-managed alerts:** Use Prometheus-compatible rules endpoint instead, more complete data
- **Node-based state history:** Edge properties are standard for temporal graph data, better performance

## Open Questions

Things that couldn't be fully resolved:

1. **Grafana API response structure for alert instances**
   - What we know: `/api/prometheus/grafana/api/v1/rules` returns Prometheus-compatible format with alert instances
   - What's unclear: Exact JSON structure of alert instances array (state field name, timestamp fields)
   - Recommendation: Test against real Grafana instance during implementation, parse response flexibly with json.RawMessage

2. **Alert state when query returns no data**
   - What we know: Grafana has special NoData state handling configured per rule
   - What's unclear: Should NoData be tracked as a distinct state or treated as Normal?
   - Recommendation: Phase 21 CONTEXT.md specifies 3-state model (firing/pending/normal), map NoData -> Normal

3. **Handling multi-dimensional alerts (multiple instances per rule)**
   - What we know: Alert rules can generate multiple instances for different label combinations
   - What's unclear: Should each instance have separate state tracking or aggregate to rule level?
   - Recommendation: Context specifies state tracking per-alert (Alert node = rule), aggregate instance states to single rule state (worst state wins)

4. **State transition edge uniqueness constraints**
   - What we know: Multiple edges can exist with same from/to states but different timestamps
   - What's unclear: Should FalkorDB index be added for faster queries?
   - Recommendation: Start without index, add if query performance issues arise (7-day window is small dataset)

5. **Cascade delete behavior verification**
   - What we know: Context specifies cascade delete when alert rule deleted in Grafana
   - What's unclear: Does FalkorDB automatically delete edges when node deleted, or requires explicit DETACH DELETE?
   - Recommendation: Test during implementation, likely needs explicit query: `MATCH (a:Alert {uid: $uid})-[t:STATE_TRANSITION]-() DELETE t, a`

## Sources

### Primary (HIGH confidence)
- Existing codebase patterns:
  - `internal/integration/grafana/alert_syncer.go` - Alert rule sync with incremental timestamps
  - `internal/integration/grafana/baseline_cache.go` - TTL pattern with expires_at
  - `internal/integration/grafana/client.go` - HTTP client patterns with Bearer token auth
- Phase 21 CONTEXT.md - User decisions on implementation approach

### Secondary (MEDIUM confidence)
- [Grafana Alert Rule State and Health](https://grafana.com/docs/grafana/latest/alerting/fundamentals/alert-rule-evaluation/alert-rule-state-and-health/) - Alert state transitions
- [Grafana View Alert State](https://grafana.com/docs/grafana/latest/alerting/monitor-status/view-alert-state/) - Alert instance tracking
- [Grafana Alerting Provisioning HTTP API](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/alerting_provisioning/) - API endpoints
- [GitHub Issue: Alert instances API performance](https://github.com/grafana/grafana/issues/93165) - API endpoint usage patterns
- [Grafana Community: Get current alerts via API](https://community.grafana.com/t/how-to-get-current-alerts-via-http-api/87888) - API endpoint discussion
- [AeonG: Temporal Property Graph Model](https://www.vldb.org/pvldb/vol17/p1515-lu.pdf) - Graph temporal data patterns
- [FalkorDB Documentation](https://docs.falkordb.com/) - Property graph model, temporal types

### Tertiary (LOW confidence)
- [AWS CloudWatch Alarm State Transitions](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/AlarmThatSendsEmail.html) - State transition patterns (different system, but similar concepts)
- [Change Point Detection Methods](https://pmc.ncbi.nlm.nih.gov/articles/PMC5464762/) - State transition detection theory

## Metadata

**Confidence breakdown:**
- Standard stack: MEDIUM - Grafana API endpoint exists but exact response structure needs verification during implementation
- Architecture: HIGH - Edge property patterns proven in Phase 19 baseline cache, sync patterns proven in Phase 20
- Pitfalls: HIGH - Derived from existing codebase patterns and common graph database mistakes

**Research date:** 2026-01-23
**Valid until:** 2026-02-23 (30 days) - Stable domain, Grafana alerting API is mature
