# Phase 20: Alert API Client & Graph Schema - Research

**Researched:** 2026-01-23
**Domain:** Grafana Alerting API, Graph Database Schema, PromQL Parsing
**Confidence:** HIGH

## Summary

Phase 20 introduces Grafana alert rule synchronization to Spectre's knowledge graph. This phase follows the established patterns from dashboard sync (Phase 19) but adapts them for alert rules. The research reveals a well-defined Grafana Alerting Provisioning API with `/api/v1/provisioning/alert-rules` endpoint, an existing PromQL parser already in the codebase (`prometheus/prometheus`), and a clear graph schema pattern using FalkorDB.

The standard approach is incremental synchronization using the `updated` timestamp field (similar to dashboard `version` field), reusing the existing PromQL parser to extract metrics from alert expressions, and extending the graph schema with Alert nodes that form MONITORS edges to Metric nodes and transitive relationships to Service nodes through those metrics.

Key architectural decision: Alert rules are synced as definitions (metadata, PromQL, labels), but alert *state* (firing/pending/normal) is deferred to Phase 21. This phase focuses solely on the alert rule structure and its relationships to metrics/services.

**Primary recommendation:** Follow the established dashboard sync pattern (DashboardSyncer → GraphBuilder) by creating AlertSyncer and extending GraphBuilder with alert-specific methods, reusing existing PromQL parser and HTTP client infrastructure.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/prometheus/prometheus | v0.309.1 | PromQL parsing | Official Prometheus parser with AST-based extraction, already used for dashboard queries |
| github.com/FalkorDB/falkordb-go/v2 | v2.0.2 | Graph database client | Existing graph client with Cypher query support |
| net/http | stdlib | HTTP client | Standard library HTTP with connection pooling, already configured in GrafanaClient |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| encoding/json | stdlib | JSON parsing | Alert rule API responses and metadata serialization |
| time | stdlib | Timestamp handling | Alert rule `updated` field for incremental sync |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| prometheus/prometheus parser | Hand-written PromQL parser | Existing parser handles edge cases, maintains compatibility with Prometheus/Grafana PromQL dialect |
| FalkorDB | Neo4j, TigerGraph | FalkorDB already integrated, supports Cypher, optimized for sparse graphs |

**Installation:**
```bash
# No new dependencies required - all libraries already in go.mod
# github.com/prometheus/prometheus v0.309.1 (existing)
# github.com/FalkorDB/falkordb-go/v2 v2.0.2 (existing)
```

## Architecture Patterns

### Recommended Project Structure
```
internal/integration/grafana/
├── grafana.go               # Integration orchestrator (existing)
├── client.go                # HTTP client with alert endpoints (extend)
├── alert_syncer.go          # Alert sync orchestrator (NEW)
├── graph_builder.go         # Graph creation logic (extend)
├── promql_parser.go         # PromQL parsing (existing, reuse)
├── types.go                 # Config and types (existing)
└── alert_syncer_test.go     # Alert sync tests (NEW)
```

### Pattern 1: Incremental Sync with Timestamp Comparison
**What:** Check `updated` timestamp field in graph vs Grafana API to determine if alert rule needs sync
**When to use:** For alert rules (similar to dashboard `version` field pattern)
**Example:**
```go
// Source: Existing dashboard_syncer.go pattern
func (as *AlertSyncer) needsSync(ctx context.Context, uid string) (bool, error) {
    // Query graph for existing alert node
    query := `
        MATCH (a:Alert {uid: $uid})
        RETURN a.updated as updated
    `
    result, err := as.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
        Query: query,
        Parameters: map[string]interface{}{"uid": uid},
    })
    if err != nil {
        return false, fmt.Errorf("failed to query alert updated time: %w", err)
    }

    // If alert doesn't exist, needs sync
    if len(result.Rows) == 0 {
        return true, nil
    }

    // Parse existing updated timestamp
    existingUpdated, err := parseTimestamp(result.Rows[0][0])
    if err != nil {
        return true, nil // Unparseable, assume needs sync
    }

    // Get current alert rule from API
    alertRule, err := as.grafanaClient.GetAlertRule(ctx, uid)
    if err != nil {
        return false, fmt.Errorf("failed to get alert rule: %w", err)
    }

    // Compare timestamps
    return alertRule.Updated.After(existingUpdated), nil
}
```

### Pattern 2: Graph Node Upsert with MERGE
**What:** Use Cypher MERGE to create or update graph nodes atomically
**When to use:** For all graph node creation (alerts, metrics, relationships)
**Example:**
```go
// Source: Existing graph_builder.go pattern
func (gb *GraphBuilder) createAlertNode(ctx context.Context, alert *AlertRule) error {
    alertQuery := `
        MERGE (a:Alert {uid: $uid})
        ON CREATE SET
            a.title = $title,
            a.folderUID = $folderUID,
            a.ruleGroup = $ruleGroup,
            a.labels = $labels,
            a.annotations = $annotations,
            a.condition = $condition,
            a.noDataState = $noDataState,
            a.execErrState = $execErrState,
            a.forDuration = $forDuration,
            a.updated = $updated,
            a.firstSeen = $now,
            a.lastSeen = $now
        ON MATCH SET
            a.title = $title,
            a.folderUID = $folderUID,
            a.ruleGroup = $ruleGroup,
            a.labels = $labels,
            a.annotations = $annotations,
            a.condition = $condition,
            a.noDataState = $noDataState,
            a.execErrState = $execErrState,
            a.forDuration = $forDuration,
            a.updated = $updated,
            a.lastSeen = $now
    `

    _, err := gb.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
        Query: alertQuery,
        Parameters: map[string]interface{}{
            "uid":          alert.UID,
            "title":        alert.Title,
            "folderUID":    alert.FolderUID,
            "ruleGroup":    alert.RuleGroup,
            "labels":       serializeJSON(alert.Labels),
            "annotations":  serializeJSON(alert.Annotations),
            "condition":    alert.Condition,
            "noDataState":  alert.NoDataState,
            "execErrState": alert.ExecErrState,
            "forDuration":  alert.For,
            "updated":      alert.Updated.UnixNano(),
            "now":          time.Now().UnixNano(),
        },
    })
    return err
}
```

### Pattern 3: PromQL Extraction and Metric Relationship
**What:** Parse alert rule PromQL expressions to extract metric names, then create MONITORS edges
**When to use:** For all alert rules with PromQL queries in their data array
**Example:**
```go
// Source: Existing graph_builder.go createQueryGraph pattern
func (gb *GraphBuilder) createAlertMetricRelationships(ctx context.Context, alert *AlertRule) error {
    // Process each query in alert data array
    for _, query := range alert.Data {
        // Skip non-PromQL queries (e.g., expressions, reducers)
        if query.QueryType != "" && query.QueryType != "prometheus" {
            continue
        }

        // Extract PromQL expression from model
        expr := extractExprFromModel(query.Model)
        if expr == "" {
            continue
        }

        // Parse PromQL using existing parser (reuse from dashboard queries)
        extraction, err := gb.parser.Parse(expr)
        if err != nil {
            gb.logger.Warn("Failed to parse alert PromQL: %v", err)
            continue
        }

        // Skip if query has variables (can't create concrete relationships)
        if extraction.HasVariables {
            gb.logger.Debug("Alert query has variables, skipping metric extraction")
            continue
        }

        // Create MONITORS edges to each metric
        for _, metricName := range extraction.MetricNames {
            if err := gb.createAlertMonitorsMetric(ctx, alert.UID, metricName); err != nil {
                gb.logger.Warn("Failed to create MONITORS edge: %v", err)
                continue
            }
        }
    }
    return nil
}

func (gb *GraphBuilder) createAlertMonitorsMetric(ctx context.Context, alertUID, metricName string) error {
    query := `
        MATCH (a:Alert {uid: $alertUID})
        MERGE (m:Metric {name: $metricName})
        ON CREATE SET m.firstSeen = $now, m.lastSeen = $now
        ON MATCH SET m.lastSeen = $now
        MERGE (a)-[:MONITORS]->(m)
    `

    _, err := gb.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
        Query: query,
        Parameters: map[string]interface{}{
            "alertUID":   alertUID,
            "metricName": metricName,
            "now":        time.Now().UnixNano(),
        },
    })
    return err
}
```

### Pattern 4: Transitive Service Relationships
**What:** Alert→Service relationships established through existing Metric→Service edges
**When to use:** Querying service-level alert relationships (no explicit edges needed)
**Example:**
```cypher
// Source: Graph database best practices - transitive relationships
// Query: Find all services monitored by alert X
MATCH (a:Alert {uid: $alertUID})-[:MONITORS]->(m:Metric)-[:TRACKS]->(s:Service)
RETURN DISTINCT s.name, s.cluster, s.namespace

// Query: Find all alerts monitoring service Y
MATCH (s:Service {name: $serviceName, cluster: $cluster})<-[:TRACKS]-(m:Metric)<-[:MONITORS]-(a:Alert)
RETURN a.uid, a.title, a.labels
```

### Anti-Patterns to Avoid
- **Creating Alert→Service direct edges:** Violates normalization, duplicates Metric→Service relationships. Use transitive queries instead.
- **Parsing PromQL with regex:** PromQL has complex grammar (subqueries, binary ops, functions). Use official parser AST traversal.
- **Storing alert state in Alert node:** Alert state is temporal (firing/pending/normal changes frequently). Store in separate AlertStateChange nodes (Phase 21).
- **Fetching all alerts on every sync:** Use incremental sync with `updated` timestamp comparison to minimize API calls and graph writes.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| PromQL parsing | Custom regex-based parser | github.com/prometheus/prometheus/promql/parser | PromQL grammar includes subqueries, binary ops, label matchers, aggregations, functions - regex cannot handle AST correctly |
| HTTP connection pooling | Default http.Client | http.Transport with tuned MaxIdleConnsPerHost | Default MaxIdleConnsPerHost=2 causes connection churn under load, existing GrafanaClient shows optimal tuning |
| Timestamp comparison logic | Manual time parsing | Use time.Time and .After() | Handles timezones, leap seconds, monotonic clock correctly |
| Alert severity extraction | Parse labels with string manipulation | Store labels as JSON, query with json_extract in Cypher | Labels are key-value maps, JSON storage enables flexible querying |
| Graph node deduplication | Check existence before create | MERGE with ON CREATE/ON MATCH | MERGE is atomic, handles concurrency correctly, avoids race conditions |

**Key insight:** Alert sync is 90% similar to dashboard sync - reuse the DashboardSyncer pattern (list → version check → fetch → parse → graph update). The Prometheus parser handles all PromQL complexity. FalkorDB's MERGE handles deduplication atomically.

## Common Pitfalls

### Pitfall 1: Alert API Response Structure Mismatch
**What goes wrong:** Grafana Alerting Provisioning API returns different JSON structure than export API
**Why it happens:** Export API returns file-provisioning format, Provisioning API returns HTTP API format
**How to avoid:** Use `/api/v1/provisioning/alert-rules` endpoint (not export endpoints), test JSON parsing with real Grafana instance
**Warning signs:** Fields missing or nested differently than documentation examples, marshal/unmarshal errors

### Pitfall 2: Alert Rule Version vs Updated Field
**What goes wrong:** Assuming alert rules have a `version` integer field like dashboards
**Why it happens:** Dashboard sync uses `version` field, but alert rules use `updated` timestamp
**How to avoid:** Use `updated` (ISO8601 timestamp string) for incremental sync comparison, not `version`
**Warning signs:** Sync logic always thinks alerts need update, timestamp parsing errors

### Pitfall 3: PromQL Expression Location in Alert Data
**What goes wrong:** Expecting flat `expr` field, but alert data is complex nested structure
**Why it happens:** Alert rules have multi-query data array with different query types (queries, expressions, reducers)
**How to avoid:** Parse `data[].model` field (JSON-encoded), check `queryType` field, only extract from Prometheus queries
**Warning signs:** Empty metric extractions, "expr field not found" errors

### Pitfall 4: Creating Redundant Alert→Service Edges
**What goes wrong:** Creating direct Alert→Service edges alongside existing Metric→Service edges
**Why it happens:** Intuitive to create direct relationship, but violates graph normalization
**How to avoid:** Use transitive queries `(Alert)-[:MONITORS]->(Metric)-[:TRACKS]->(Service)` instead of direct edges
**Warning signs:** Duplicate relationship maintenance code, inconsistencies between Alert→Service and Metric→Service paths

### Pitfall 5: Storing Alert State in Alert Node
**What goes wrong:** Adding `state` field to Alert node that changes frequently (firing/pending/normal)
**Why it happens:** Seems natural to store current state with alert definition
**How to avoid:** Alert nodes store *definition* (title, labels, PromQL), AlertStateChange nodes store *timeline* (Phase 21)
**Warning signs:** Frequent Alert node updates, inability to track state history, graph write contention

## Code Examples

Verified patterns from codebase and official documentation:

### Grafana Alerting API - List Alert Rules
```go
// Source: https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/alerting_provisioning/
// GET /api/v1/provisioning/alert-rules

func (c *GrafanaClient) ListAlertRules(ctx context.Context) ([]AlertRuleMeta, error) {
    reqURL := fmt.Sprintf("%s/api/v1/provisioning/alert-rules", c.config.URL)
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
    if err != nil {
        return nil, fmt.Errorf("create list alert rules request: %w", err)
    }

    // Add Bearer token authentication (reuse secretWatcher pattern)
    if c.secretWatcher != nil {
        token, err := c.secretWatcher.GetToken()
        if err != nil {
            return nil, fmt.Errorf("failed to get API token: %w", err)
        }
        req.Header.Set("Authorization", "Bearer "+token)
    }

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("execute list alert rules request: %w", err)
    }
    defer resp.Body.Close()

    // CRITICAL: Always read response body to completion for connection reuse
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response body: %w", err)
    }

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("list alert rules failed (status %d): %s", resp.StatusCode, string(body))
    }

    var alertRules []AlertRuleMeta
    if err := json.Unmarshal(body, &alertRules); err != nil {
        return nil, fmt.Errorf("parse alert rules response: %w", err)
    }

    return alertRules, nil
}

// AlertRuleMeta represents an alert rule in the list response
type AlertRuleMeta struct {
    UID         string            `json:"uid"`
    Title       string            `json:"title"`
    RuleGroup   string            `json:"ruleGroup"`
    FolderUID   string            `json:"folderUID"`
    Updated     time.Time         `json:"updated"`
    Labels      map[string]string `json:"labels"`
}
```

### Alert Rule Full Structure
```go
// Source: https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/alerting_provisioning/
// GET /api/v1/provisioning/alert-rules/{uid}

type AlertRule struct {
    UID          string                 `json:"uid"`
    Title        string                 `json:"title"`
    RuleGroup    string                 `json:"ruleGroup"`
    FolderUID    string                 `json:"folderUID"`
    NoDataState  string                 `json:"noDataState"`  // "OK", "NoData", "Alerting"
    ExecErrState string                 `json:"execErrState"` // "OK", "Alerting"
    For          string                 `json:"for"`          // Duration string: "5m", "1h"
    Condition    string                 `json:"condition"`    // RefId of condition expression
    Labels       map[string]string      `json:"labels"`
    Annotations  map[string]string      `json:"annotations"`
    Updated      time.Time              `json:"updated"`
    Data         []AlertQueryOrExpr     `json:"data"`
}

type AlertQueryOrExpr struct {
    RefID               string                 `json:"refId"`
    QueryType           string                 `json:"queryType,omitempty"` // "" for Prometheus, "expression" for reducers
    RelativeTimeRange   *RelativeTimeRange     `json:"relativeTimeRange"`
    DatasourceUID       string                 `json:"datasourceUid"`
    Model               map[string]interface{} `json:"model"` // Query-specific, contains "expr" for PromQL
}

type RelativeTimeRange struct {
    From int64 `json:"from"` // Seconds before now
    To   int64 `json:"to"`   // Seconds before now
}

// Extract PromQL expression from model
func extractExprFromModel(model map[string]interface{}) string {
    if expr, ok := model["expr"].(string); ok {
        return expr
    }
    return ""
}
```

### Graph Schema: Alert Node with Relationships
```cypher
-- Source: Existing graph_builder.go MERGE pattern + FalkorDB Cypher docs

-- Create Alert node
MERGE (a:Alert {uid: $uid})
ON CREATE SET
    a.title = $title,
    a.folderUID = $folderUID,
    a.ruleGroup = $ruleGroup,
    a.labels = $labels,           -- JSON string
    a.annotations = $annotations, -- JSON string
    a.condition = $condition,
    a.noDataState = $noDataState,
    a.execErrState = $execErrState,
    a.forDuration = $forDuration,
    a.updated = $updated,         -- UnixNano timestamp
    a.firstSeen = $now,
    a.lastSeen = $now
ON MATCH SET
    a.title = $title,
    a.folderUID = $folderUID,
    a.ruleGroup = $ruleGroup,
    a.labels = $labels,
    a.annotations = $annotations,
    a.condition = $condition,
    a.noDataState = $noDataState,
    a.execErrState = $execErrState,
    a.forDuration = $forDuration,
    a.updated = $updated,
    a.lastSeen = $now

-- Create Alert→Metric MONITORS relationship
MATCH (a:Alert {uid: $alertUID})
MERGE (m:Metric {name: $metricName})
ON CREATE SET m.firstSeen = $now, m.lastSeen = $now
ON MATCH SET m.lastSeen = $now
MERGE (a)-[:MONITORS]->(m)

-- Query: Find services monitored by alert (transitive)
MATCH (a:Alert {uid: $alertUID})-[:MONITORS]->(m:Metric)-[:TRACKS]->(s:Service)
RETURN DISTINCT s.name, s.cluster, s.namespace

-- Query: Find alerts monitoring a service (transitive)
MATCH (s:Service {name: $serviceName, cluster: $cluster})<-[:TRACKS]-(m:Metric)<-[:MONITORS]-(a:Alert)
RETURN a.uid, a.title, a.labels
```

### Reusing Existing PromQL Parser
```go
// Source: internal/integration/grafana/promql_parser.go (existing)
// The parser is already implemented and tested, just reuse it

import "github.com/moolen/spectre/internal/integration/grafana"

// Extract metrics from alert rule PromQL expressions
func extractMetricsFromAlert(alert *AlertRule) ([]string, error) {
    var allMetrics []string

    for _, query := range alert.Data {
        // Skip non-Prometheus queries
        if query.QueryType != "" && query.QueryType != "prometheus" {
            continue
        }

        // Extract PromQL expression from model
        expr := extractExprFromModel(query.Model)
        if expr == "" {
            continue
        }

        // Use existing parser (handles variables, complex queries, error cases)
        extraction, err := grafana.ExtractFromPromQL(expr)
        if err != nil {
            // Parser returns error for unparseable queries
            // This is expected for queries with Grafana variables
            continue
        }

        // Skip if query has variables (metric names may be templated)
        if extraction.HasVariables {
            continue
        }

        // Add all extracted metric names
        allMetrics = append(allMetrics, extraction.MetricNames...)
    }

    return allMetrics, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Legacy Grafana Alert API (/api/alerts) | Unified Alerting Provisioning API (/api/v1/provisioning/alert-rules) | Grafana 9.0+ (2022) | New API supports rule groups, multiple datasources, better structure |
| Alert version field | Alert updated timestamp | Grafana Unified Alerting | Use ISO8601 timestamp for sync comparison, not integer version |
| Direct PromQL string parsing | Prometheus parser AST traversal | Always recommended | AST handles complex queries, subqueries, binary operations correctly |
| Flattened alert metadata | Structured data array with query types | Grafana 9.0+ | Alerts can have multiple queries, expressions, and reducers |

**Deprecated/outdated:**
- **Legacy Alert API (/api/alerts)**: Deprecated in Grafana 9.0, removed in 11.0. Use Unified Alerting `/api/v1/provisioning/alert-rules` instead.
- **Dashboard alert panels**: Old alerting system stored alerts in dashboard panels. New system stores alerts independently with optional `__dashboardUid__` annotation for linking.

## Open Questions

Things that couldn't be fully resolved:

1. **Alert Rule State Endpoint**
   - What we know: Provisioning API returns alert *definitions*, not current *state* (firing/pending/normal)
   - What's unclear: Optimal endpoint for fetching current alert state - options include:
     - Ruler API: `/api/ruler/grafana/api/v1/rules/` (returns rules with state)
     - Prometheus Alertmanager API: `/api/v1/alerts` (returns active alerts only)
     - Alerting State History API (requires configuration)
   - Recommendation: Defer alert state fetching to Phase 21, focus Phase 20 on rule definitions only. Research Ruler API vs Alertmanager API in Phase 21.

2. **Alert Severity Field**
   - What we know: Grafana doesn't have built-in severity field, users typically use labels (e.g., `severity: "critical"`)
   - What's unclear: Standard label names for severity (severity vs priority vs level)
   - Recommendation: Store all labels as JSON, allow flexible querying. Document common patterns (severity, priority) in MCP tool descriptions (Phase 23).

3. **Folder Hierarchy Depth**
   - What we know: Alerts have `folderUID` field, folders can be nested
   - What's unclear: Whether to traverse folder hierarchy and create Folder nodes in graph
   - Recommendation: Store `folderUID` in Alert node, defer folder hierarchy to future enhancement. Phase 20 focuses on Alert→Metric→Service relationships.

4. **Alert Rule Group Relationships**
   - What we know: Alerts belong to rule groups (`ruleGroup` field), groups are evaluated together
   - What's unclear: Whether to create RuleGroup nodes and relationships, or store as simple string property
   - Recommendation: Store `ruleGroup` as Alert node property (string), defer RuleGroup nodes to v2 if needed for group-level queries.

## Sources

### Primary (HIGH confidence)
- Grafana Alerting Provisioning HTTP API - https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/alerting_provisioning/
- Codebase: internal/integration/grafana/dashboard_syncer.go - Incremental sync pattern
- Codebase: internal/integration/grafana/promql_parser.go - PromQL extraction (github.com/prometheus/prometheus)
- Codebase: internal/integration/grafana/graph_builder.go - Graph schema patterns (MERGE, relationships)
- Codebase: internal/integration/grafana/client.go - HTTP client with connection pooling
- FalkorDB Cypher Coverage - https://docs.falkordb.com/cypher/cypher-support.html

### Secondary (MEDIUM confidence)
- [Grafana Alert Rule State and Health](https://grafana.com/docs/grafana/latest/alerting/fundamentals/alert-rule-evaluation/alert-rule-state-and-health/) - Alert state concepts
- [Grafana Alert Rules Documentation](https://grafana.com/docs/grafana/latest/alerting/fundamentals/alert-rules/) - Alert rule fundamentals
- [FalkorDB Edges Blog](https://www.falkordb.com/blog/edges-in-falkordb/) - Edge implementation details
- [Graph-based Alerting (GraphAware)](https://graphaware.com/blog/hume/graph-based-alerting.html) - Graph alerting patterns
- [Graph Database Best Practices (Microsoft)](https://playbook.microsoft.com/code-with-dataops/guidance/graph-database-best-practices/) - Relationship design patterns

### Tertiary (LOW confidence)
- Community discussions on Grafana Alerting API usage - Verified against official docs
- Graph database monitoring patterns - General concepts, not FalkorDB-specific

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries already in codebase and actively used (prometheus/prometheus, FalkorDB client, stdlib)
- Architecture: HIGH - Dashboard sync pattern is proven, alert sync is direct adaptation with same structure
- Pitfalls: HIGH - Based on codebase analysis and official API documentation discrepancies
- Graph schema: HIGH - Follows existing patterns (MERGE, relationship types, transitive queries)
- Alert state endpoints: MEDIUM - Multiple API options, optimal choice deferred to Phase 21

**Research date:** 2026-01-23
**Valid until:** 2026-02-23 (30 days - Grafana API stable, alerting provisioning API GA since v9.0)

**Notes:**
- Phase 20 scope is alert rule *definitions* only, not state (firing/pending). State is Phase 21.
- All patterns reuse existing codebase - no new architectural decisions required.
- PromQL parser already handles alert query extraction, no modifications needed.
- Graph schema extends naturally: Alert→Metric (new), Metric→Service (existing).
