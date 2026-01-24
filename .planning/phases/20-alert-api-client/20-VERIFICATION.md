---
phase: 20-alert-api-client
verified: 2026-01-23T08:57:33Z
status: passed
score: 6/6 must-haves verified
---

# Phase 20: Alert API Client & Graph Schema Verification Report

**Phase Goal:** Alert rules are synced from Grafana and stored in FalkorDB with links to existing Metrics and Services.
**Verified:** 2026-01-23T08:57:33Z
**Status:** PASSED
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | GrafanaClient can fetch alert rules via Grafana Alerting API | ✓ VERIFIED | `ListAlertRules()` and `GetAlertRule()` methods exist in `client.go` lines 183-277, use `/api/v1/provisioning/alert-rules` endpoint with Bearer auth |
| 2 | Alert rules are synced incrementally based on version field | ✓ VERIFIED | `AlertSyncer.needsSync()` compares `Updated` timestamps (line 195-242 in `alert_syncer.go`), skips unchanged alerts, test coverage confirms behavior |
| 3 | Alert nodes exist in FalkorDB with metadata | ✓ VERIFIED | `AlertNode` struct in `models.go` lines 95-106 with 9 fields (UID, Title, FolderTitle, RuleGroup, Condition, Labels, Annotations, Updated, Integration), `BuildAlertGraph()` creates nodes via MERGE |
| 4 | PromQL parser extracts metrics from alert rule queries | ✓ VERIFIED | `BuildAlertGraph()` parses `AlertQuery.Model` JSON to extract `expr` field (lines 672-694), calls `parser.Parse(expr)` to extract metric names, reuses existing PromQL parser |
| 5 | Graph contains Alert→Metric relationships (MONITORS edges) | ✓ VERIFIED | `createAlertMetricEdge()` creates `MONITORS` edges from Alert to Metric (line 728 in `graph_builder.go`), EdgeTypeMonitors constant exists in `models.go` line 51 |
| 6 | Graph contains Alert→Service relationships (transitive through Metric nodes) | ✓ VERIFIED | No direct Alert→Service edges created (as designed), transitive path `(Alert)-[:MONITORS]->(Metric)-[:TRACKS]->(Service)` queryable, Service nodes created from PromQL label selectors (line 431) |

**Score:** 6/6 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/graph/models.go` | NodeTypeAlert, EdgeTypeMonitors, AlertNode struct | ✓ VERIFIED | NodeTypeAlert constant line 21, EdgeTypeMonitors constant line 51, AlertNode struct lines 95-106 with 9 fields |
| `internal/integration/grafana/client.go` | ListAlertRules(), GetAlertRule() methods | ✓ VERIFIED | AlertRule/AlertQuery structs lines 16-34, ListAlertRules() lines 183-229, GetAlertRule() lines 231-277, uses `/api/v1/provisioning/alert-rules` endpoint |
| `internal/integration/grafana/alert_syncer.go` | AlertSyncer with incremental sync | ✓ VERIFIED | 249 lines (substantive), exports NewAlertSyncer and AlertSyncer, implements Start/Stop/syncAlerts methods, needsSync() compares timestamps |
| `internal/integration/grafana/graph_builder.go` | BuildAlertGraph() method | ✓ VERIFIED | BuildAlertGraph() lines 588-715 creates Alert nodes and MONITORS edges, calls parser.Parse() for PromQL extraction, createAlertMetricEdge() lines 717-745 |
| `internal/integration/grafana/alert_syncer_test.go` | Test coverage for AlertSyncer | ✓ VERIFIED | 321 lines, 5 test cases: NewAlertRule, UpdatedAlertRule, UnchangedAlertRule, APIError, Lifecycle, all tests pass |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| AlertSyncer | GrafanaClient | ListAlertRules API call | ✓ WIRED | Line 132 in `alert_syncer.go`: `alertRules, err := as.client.ListAlertRules(as.ctx)`, mock interface confirms contract |
| AlertSyncer | GraphBuilder | BuildAlertGraph() call | ✓ WIRED | Line 165 in `alert_syncer.go`: `as.builder.BuildAlertGraph(alertRule)`, called for new/updated alerts |
| GraphBuilder | PromQLParser | parser.Parse() for metric extraction | ✓ WIRED | Line 688 in `graph_builder.go`: `extraction, err := gb.parser.Parse(expr)`, extracts MetricNames from PromQL expressions |
| GrafanaIntegration | AlertSyncer | Start/Stop lifecycle | ✓ WIRED | Lines 173-186 in `grafana.go`: AlertSyncer created with shared GraphBuilder, Start() called in integration lifecycle, Stop() at line 216 |

### Requirements Coverage

Requirements from ROADMAP.md Phase 20:

| Requirement | Status | Supporting Truths |
|-------------|--------|-------------------|
| ALRT-01: Grafana Alerting API client methods | ✓ SATISFIED | Truth 1 (ListAlertRules, GetAlertRule methods) |
| ALRT-02: Incremental alert rule sync | ✓ SATISFIED | Truth 2 (needsSync timestamp comparison) |
| GRPH-08: Alert node type with metadata | ✓ SATISFIED | Truth 3 (AlertNode struct with 9 fields) |
| GRPH-09: Alert→Metric MONITORS edges | ✓ SATISFIED | Truth 5 (MONITORS edge creation) |
| GRPH-10: Alert→Service transitive relationships | ✓ SATISFIED | Truth 6 (transitive via Metric nodes) |

### Anti-Patterns Found

None detected. Code follows established patterns:
- No TODO/FIXME comments found in implementation files
- No placeholder or stub implementations
- No console.log-only handlers
- All exports are substantive with real logic
- Graceful error handling throughout (log and continue pattern)

### Build & Test Verification

```bash
# Compilation check
$ go build ./internal/graph/...
✓ No errors

$ go build ./internal/integration/grafana/...
✓ No errors

# Test execution
$ go test ./internal/integration/grafana/... -run TestAlertSyncer
=== RUN   TestAlertSyncer_NewAlertRule
--- PASS: TestAlertSyncer_NewAlertRule (0.00s)
=== RUN   TestAlertSyncer_UpdatedAlertRule
--- PASS: TestAlertSyncer_UpdatedAlertRule (0.00s)
=== RUN   TestAlertSyncer_UnchangedAlertRule
--- PASS: TestAlertSyncer_UnchangedAlertRule (0.00s)
=== RUN   TestAlertSyncer_APIError
--- PASS: TestAlertSyncer_APIError (0.00s)
=== RUN   TestAlertSyncer_Lifecycle
--- PASS: TestAlertSyncer_Lifecycle (0.00s)
PASS
ok  	github.com/moolen/spectre/internal/integration/grafana	0.007s
```

## Detailed Verification

### 1. Graph Schema Extension

**Check:** Alert node types and MONITORS edge exist in graph schema

**Evidence:**
- File: `/home/moritz/dev/spectre-via-ssh/internal/graph/models.go`
- NodeTypeAlert constant: line 21
- EdgeTypeMonitors constant: line 51
- AlertNode struct: lines 95-106

**AlertNode struct fields (9 total):**
```go
type AlertNode struct {
    UID         string            `json:"uid"`         // Alert rule UID (primary key)
    Title       string            `json:"title"`       // Alert rule title
    FolderTitle string            `json:"folderTitle"` // Folder containing the rule
    RuleGroup   string            `json:"ruleGroup"`   // Alert rule group name
    Condition   string            `json:"condition"`   // PromQL expression (stored for display)
    Labels      map[string]string `json:"labels"`      // Alert labels
    Annotations map[string]string `json:"annotations"` // Alert annotations including severity
    Updated     string            `json:"updated"`     // ISO8601 timestamp for incremental sync
    Integration string            `json:"integration"` // Integration name (e.g., "grafana_prod")
}
```

**Status:** ✓ VERIFIED — All required fields present, follows pattern from Phase 16 DashboardNode

### 2. Grafana Alert API Client

**Check:** GrafanaClient has ListAlertRules and GetAlertRule methods

**Evidence:**
- File: `/home/moritz/dev/spectre-via-ssh/internal/integration/grafana/client.go`
- AlertRule struct: lines 16-26 (contains UID, Title, FolderUID, RuleGroup, Data, Labels, Annotations, Updated)
- AlertQuery struct: lines 28-34 (contains RefID, Model as json.RawMessage, DatasourceUID, QueryType)
- ListAlertRules(): lines 183-229
- GetAlertRule(): lines 231-277

**API endpoint verification:**
```go
// ListAlertRules: line 187
reqURL := fmt.Sprintf("%s/api/v1/provisioning/alert-rules", c.config.URL)

// GetAlertRule: line 235
reqURL := fmt.Sprintf("%s/api/v1/provisioning/alert-rules/%s", c.config.URL, uid)
```

**Authentication:** Bearer token via secretWatcher (same pattern as dashboard methods)

**Response handling:** io.ReadAll for connection reuse, error logging on failure, JSON unmarshal to AlertRule structs

**Status:** ✓ VERIFIED — Methods follow established GrafanaClient patterns, AlertQuery.Model stored as json.RawMessage enables flexible PromQL parsing

### 3. AlertSyncer Incremental Sync

**Check:** Alert rules synced incrementally based on Updated timestamp

**Evidence:**
- File: `/home/moritz/dev/spectre-via-ssh/internal/integration/grafana/alert_syncer.go`
- Line count: 249 lines (substantive implementation)
- Exports: NewAlertSyncer (line 35), AlertSyncer struct (line 14)

**Incremental sync logic (needsSync method, lines 195-242):**
1. Query graph for existing Alert node by UID and integration
2. If not found → needs sync
3. If found → compare Updated timestamps as RFC3339 strings
4. If currentUpdated > existingUpdated → needs sync
5. Otherwise skip (alert unchanged)

**Test coverage verification:**
- File: `/home/moritz/dev/spectre-via-ssh/internal/integration/grafana/alert_syncer_test.go`
- Line count: 321 lines
- TestAlertSyncer_NewAlertRule: Confirms new alerts are synced
- TestAlertSyncer_UpdatedAlertRule: Confirms timestamp-based detection (old: 2026-01-20, new: 2026-01-23)
- TestAlertSyncer_UnchangedAlertRule: Confirms alerts with same timestamp are skipped
- TestAlertSyncer_APIError: Confirms API error propagation
- TestAlertSyncer_Lifecycle: Confirms Start/Stop work correctly

**Sync interval:** 1 hour (line 48 in alert_syncer.go: `syncInterval: time.Hour`)

**Status:** ✓ VERIFIED — Incremental sync fully implemented with comprehensive test coverage

### 4. PromQL Metric Extraction

**Check:** PromQL parser extracts metrics from alert rule queries

**Evidence:**
- File: `/home/moritz/dev/spectre-via-ssh/internal/integration/grafana/graph_builder.go`
- BuildAlertGraph() method: lines 588-715

**PromQL extraction flow:**
1. Iterate alert rule Data (AlertQuery array)
2. Filter for QueryType == "prometheus"
3. Unmarshal AlertQuery.Model (json.RawMessage) to extract "expr" field (lines 672-678)
4. Call `gb.parser.Parse(expr)` to extract semantic info (line 688)
5. Extract MetricNames from QueryExtraction (line 703)
6. Create MONITORS edges for each metric (line 704)

**Parser integration:**
- Line 688: `extraction, err := gb.parser.Parse(expr)`
- Parser type: PromQLParserInterface (line 51-53)
- Production parser: defaultPromQLParser wraps ExtractFromPromQL (lines 84-89)
- ExtractFromPromQL uses prometheus/promql/parser for AST-based extraction

**Graceful error handling:**
- Line 691: Parse errors logged as warnings, continue with other queries
- Line 697: Queries with variables skipped (HasVariables flag)

**Status:** ✓ VERIFIED — Reuses existing PromQL parser from Phase 16, extracts metrics from alert query expressions

### 5. MONITORS Edge Creation

**Check:** Graph contains Alert→Metric relationships via MONITORS edges

**Evidence:**
- File: `/home/moritz/dev/spectre-via-ssh/internal/integration/grafana/graph_builder.go`
- createAlertMetricEdge() method: lines 717-745

**Cypher query (line 720-729):**
```cypher
MATCH (a:Alert {uid: $alertUID, integration: $integration})
MERGE (m:Metric {name: $metricName})
ON CREATE SET
    m.firstSeen = $now,
    m.lastSeen = $now
ON MATCH SET
    m.lastSeen = $now
MERGE (a)-[:MONITORS]->(m)
```

**MERGE semantics:**
- Creates Metric node if doesn't exist
- Updates lastSeen timestamp if exists
- Creates MONITORS edge (upsert)

**Called from:** BuildAlertGraph() line 704 for each extracted metric name

**Status:** ✓ VERIFIED — MONITORS edges created from Alert to Metric nodes, Metric nodes shared across dashboards and alerts

### 6. Transitive Alert→Service Relationships

**Check:** Alert→Service relationships queryable transitively through Metrics

**Evidence:**
- No direct Alert→Service edges created (by design)
- Transitive path: `(Alert)-[:MONITORS]->(Metric)-[:TRACKS]->(Service)`

**TRACKS edge creation (from Phase 17):**
- File: `/home/moritz/dev/spectre-via-ssh/internal/integration/grafana/graph_builder.go`
- createServiceNodes() method: lines 415-451
- Cypher query line 431: `MERGE (m)-[:TRACKS]->(s)`
- Service nodes inferred from PromQL label selectors (app/service/job)

**Queryability:**
```cypher
// Find services monitored by an alert
MATCH (a:Alert {uid: $alertUID})-[:MONITORS]->(m:Metric)-[:TRACKS]->(s:Service)
RETURN s

// Find alerts monitoring a service
MATCH (a:Alert)-[:MONITORS]->(m:Metric)-[:TRACKS]->(s:Service {name: $serviceName})
RETURN a
```

**Status:** ✓ VERIFIED — Transitive relationships work through existing Metric→Service edges from Phase 17, no direct edges needed

### 7. Integration Wiring

**Check:** AlertSyncer wired into Grafana integration lifecycle

**Evidence:**
- File: `/home/moritz/dev/spectre-via-ssh/internal/integration/grafana/grafana.go`
- alertSyncer field: line 36
- AlertSyncer creation: lines 173-185
- Start() call: line 182
- Stop() call: line 216

**Wiring details:**
```go
// Line 174: Create shared GraphBuilder for both dashboard and alert syncing
graphBuilder := NewGraphBuilder(g.graphClient, g.config, g.name, g.logger)

// Line 175-180: Create AlertSyncer with shared builder
g.alertSyncer = NewAlertSyncer(
    g.client,
    g.graphClient,
    graphBuilder,
    g.name, // Integration name
    g.logger,
)

// Line 182: Start alert syncer
if err := g.alertSyncer.Start(g.ctx); err != nil {
    g.logger.Warn("Failed to start alert syncer: %v (continuing without sync)", err)
}
```

**Lifecycle:**
- AlertSyncer created only when graphClient is available
- Shares GraphBuilder instance with DashboardSyncer for consistent integration field
- Started after DashboardSyncer in Start()
- Stopped before DashboardSyncer in Stop()

**Status:** ✓ WIRED — AlertSyncer fully integrated into GrafanaIntegration lifecycle with shared builder

## Summary

**All 6 success criteria VERIFIED:**

✓ **GrafanaClient can fetch alert rules via Grafana Alerting API**
  - ListAlertRules() and GetAlertRule() methods implemented
  - Uses `/api/v1/provisioning/alert-rules` endpoint
  - Bearer token authentication

✓ **Alert rules are synced incrementally based on version field**
  - needsSync() compares Updated timestamps
  - Skips unchanged alerts (string comparison of RFC3339 timestamps)
  - Hourly sync interval
  - Comprehensive test coverage

✓ **Alert nodes exist in FalkorDB with metadata**
  - AlertNode struct with 9 fields
  - NodeTypeAlert and EdgeTypeMonitors constants
  - MERGE-based upsert in graph

✓ **PromQL parser extracts metrics from alert rule queries**
  - Reuses existing PromQL parser from Phase 16
  - Parses AlertQuery.Model JSON to extract expr field
  - Graceful error handling (log and continue)

✓ **Graph contains Alert→Metric relationships (MONITORS edges)**
  - MONITORS edges created via createAlertMetricEdge()
  - Metric nodes shared across dashboards and alerts
  - MERGE semantics for upsert

✓ **Graph contains Alert→Service relationships (transitive through Metric nodes)**
  - No direct Alert→Service edges (as designed)
  - Transitive path: (Alert)-[:MONITORS]->(Metric)-[:TRACKS]->(Service)
  - Service nodes from PromQL label extraction (Phase 17)

**Code quality:**
- All code compiles without errors
- All tests pass (5 test cases in alert_syncer_test.go)
- No stub implementations or placeholders
- Follows established patterns from Phase 16 DashboardSyncer
- Graceful error handling throughout

**Phase goal ACHIEVED:** Alert rules are synced from Grafana and stored in FalkorDB with links to existing Metrics and Services.

---

_Verified: 2026-01-23T08:57:33Z_
_Verifier: Claude (gsd-verifier)_
