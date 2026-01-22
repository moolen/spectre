# Phase 16: Ingestion Pipeline - Dashboard Sync & PromQL Parsing - Research

**Researched:** 2026-01-22
**Domain:** Dashboard synchronization, PromQL parsing, graph database modeling
**Confidence:** HIGH

## Summary

Phase 16 implements incremental dashboard synchronization from Grafana with full semantic extraction of PromQL queries to build a comprehensive knowledge graph. The core technical challenges are: (1) parsing PromQL queries to extract metrics, labels, and aggregations using the official Prometheus parser library, (2) detecting dashboard changes via version field comparison for efficient incremental sync, and (3) modeling Dashboard→Panel→Query→Metric relationships in FalkorDB with proper handling of Grafana variables.

The standard approach uses the official `github.com/prometheus/prometheus/promql/parser` library for AST-based PromQL parsing, Grafana's REST API for dashboard fetching with version-based change detection, and FalkorDB's Cypher interface for creating graph nodes and relationships. The codebase already has established patterns for integration watchers (SecretWatcher), periodic sync loops (IntegrationWatcher), and graph operations (graph.Client interface).

**Primary recommendation:** Follow the VictoriaLogs integration pattern for consistency (SecretWatcher + config file patterns), use the Prometheus PromQL parser's Inspect function for AST traversal to extract VectorSelector nodes, and implement version-based incremental sync with full-replace semantics on dashboard update.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/prometheus/prometheus/promql/parser | Latest (v2.x) | PromQL parsing and AST traversal | Official Prometheus parser, battle-tested, complete AST node types |
| github.com/FalkorDB/falkordb-go | v2 | Graph database client | Official FalkorDB Go client, Cypher query execution |
| github.com/fsnotify/fsnotify | v1.x | File watching for config reload | Standard Go file watcher, used in existing IntegrationWatcher |
| k8s.io/client-go | v0.x | Kubernetes API and informers | Standard K8s client, used in existing SecretWatcher |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| encoding/json | stdlib | JSON parsing for dashboard structure | Parse Grafana API responses and dashboard JSON |
| time | stdlib | Interval-based sync scheduling | Hourly sync intervals, debouncing |
| context | stdlib | Cancellation and timeout | Graceful shutdown, API timeouts |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Prometheus parser | Write custom PromQL parser | Custom parser would be incomplete, miss edge cases, require extensive testing |
| Version-based sync | Timestamp-based sync | Timestamps have granularity issues, version is authoritative change indicator |
| FalkorDB Cypher | Direct Redis commands | Cypher provides type safety, readability, and query optimization |

**Installation:**
```bash
go get github.com/prometheus/prometheus/promql/parser
go get github.com/FalkorDB/falkordb-go/v2
# fsnotify and k8s.io/client-go already in project
```

## Architecture Patterns

### Recommended Project Structure
```
internal/integration/grafana/
├── dashboard_syncer.go      # Main sync orchestrator
├── dashboard_syncer_test.go
├── promql_parser.go          # PromQL AST extraction
├── promql_parser_test.go
├── graph_builder.go          # Graph node/edge creation
├── graph_builder_test.go
├── secret_watcher.go         # Already exists
└── secret_watcher_test.go    # Already exists
```

### Pattern 1: Incremental Sync with Version-Based Change Detection
**What:** Compare dashboard version field between local cache and Grafana API to detect changes
**When to use:** All dashboard sync operations to avoid re-syncing unchanged dashboards
**Example:**
```go
// Source: Incremental sync pattern research
type DashboardCache struct {
    UID     string
    Version int
    LastSynced time.Time
}

func (s *DashboardSyncer) NeedsSync(dashboard GrafanaDashboard, cached *DashboardCache) bool {
    if cached == nil {
        return true // Never synced before
    }
    // Version field is authoritative for change detection
    return dashboard.Version > cached.Version
}

func (s *DashboardSyncer) SyncDashboard(ctx context.Context, dashboard GrafanaDashboard) error {
    // Full replace pattern - delete all Panel/Query nodes for this dashboard
    // This ensures removed panels/queries are cleaned up
    if err := s.deleteExistingPanelsAndQueries(ctx, dashboard.UID); err != nil {
        return fmt.Errorf("failed to delete old panels: %w", err)
    }

    // Recreate from scratch
    return s.createDashboardGraph(ctx, dashboard)
}
```

### Pattern 2: PromQL AST Traversal with Inspect
**What:** Use parser.Inspect to walk the PromQL AST in depth-first order and extract semantic components
**When to use:** Extracting metric names, label selectors, and aggregations from PromQL queries
**Example:**
```go
// Source: https://pkg.go.dev/github.com/prometheus/prometheus/promql/parser
import (
    "github.com/prometheus/prometheus/promql/parser"
    "github.com/prometheus/prometheus/pkg/labels"
)

type QueryExtraction struct {
    MetricNames   []string
    LabelMatchers []*labels.Matcher
    Aggregations  []string
}

func ExtractFromPromQL(queryStr string) (*QueryExtraction, error) {
    expr, err := parser.ParseExpr(queryStr)
    if err != nil {
        return nil, fmt.Errorf("parse error: %w", err)
    }

    extraction := &QueryExtraction{
        MetricNames:  make([]string, 0),
        Aggregations: make([]string, 0),
    }

    // Walk AST in depth-first order
    parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
        switch n := node.(type) {
        case *parser.VectorSelector:
            // Extract metric name from VectorSelector
            if n.Name != "" {
                extraction.MetricNames = append(extraction.MetricNames, n.Name)
            }
            // Extract label matchers
            extraction.LabelMatchers = append(extraction.LabelMatchers, n.LabelMatchers...)

        case *parser.AggregateExpr:
            // Extract aggregation function (sum, avg, rate, etc.)
            extraction.Aggregations = append(extraction.Aggregations, n.Op.String())

        case *parser.Call:
            // Extract function calls (rate, increase, etc.)
            extraction.Aggregations = append(extraction.Aggregations, n.Func.Name)
        }
        return nil
    })

    return extraction, nil
}
```

### Pattern 3: Graph Schema with Query-Centric Relationships
**What:** Model Dashboard→Panel→Query→Metric as distinct nodes with typed relationships
**When to use:** Building knowledge graph for dashboard observability
**Example:**
```go
// Source: Graph database best practices + existing graph/models.go patterns
// Add to internal/graph/models.go
const (
    NodeTypeDashboard NodeType = "Dashboard"  // Already exists
    NodeTypePanel     NodeType = "Panel"
    NodeTypeQuery     NodeType = "Query"
    NodeTypeMetric    NodeType = "Metric"
)

const (
    EdgeTypeContains EdgeType = "CONTAINS"  // Dashboard → Panel
    EdgeTypeHas      EdgeType = "HAS"       // Panel → Query
    EdgeTypeUses     EdgeType = "USES"      // Query → Metric
    EdgeTypeTracks   EdgeType = "TRACKS"    // Metric → Service (future)
)

type PanelNode struct {
    ID          string `json:"id"`          // Panel ID (unique within dashboard)
    DashboardUID string `json:"dashboardUID"` // Parent dashboard
    Title       string `json:"title"`       // Panel title
    Type        string `json:"type"`        // Panel type (graph, table, etc.)
    GridPosX    int    `json:"gridPosX"`    // Layout position
    GridPosY    int    `json:"gridPosY"`
}

type QueryNode struct {
    ID           string   `json:"id"`           // Query ID (unique identifier)
    RefID        string   `json:"refId"`        // Query reference ID (A, B, C, etc.)
    RawPromQL    string   `json:"rawPromQL"`    // Original PromQL expression
    DatasourceUID string  `json:"datasourceUID"` // Datasource UID
    Aggregations []string `json:"aggregations"` // Extracted functions (sum, rate, etc.)
    LabelSelectors map[string]string `json:"labelSelectors"` // Extracted label matchers
}

type MetricNode struct {
    Name      string `json:"name"`      // Metric name (e.g., http_requests_total)
    FirstSeen int64  `json:"firstSeen"` // Unix nano timestamp
    LastSeen  int64  `json:"lastSeen"`  // Unix nano timestamp
}

// Cypher creation pattern
func (c *falkorClient) CreateDashboardGraph(ctx context.Context, dashboard GrafanaDashboard) error {
    // 1. Create/merge dashboard node
    query := `
    MERGE (d:Dashboard {uid: $uid})
    SET d.title = $title, d.version = $version, d.lastSeen = $lastSeen
    `

    // 2. Create panels
    for _, panel := range dashboard.Panels {
        query := `
        MATCH (d:Dashboard {uid: $dashboardUID})
        CREATE (p:Panel {id: $panelID, title: $title, type: $type})
        CREATE (d)-[:CONTAINS]->(p)
        `

        // 3. Create queries for each panel
        for _, target := range panel.Targets {
            extraction, err := ExtractFromPromQL(target.Expr)

            query := `
            MATCH (p:Panel {id: $panelID})
            CREATE (q:Query {
                id: $queryID,
                refId: $refId,
                rawPromQL: $rawPromQL,
                aggregations: $aggregations,
                labelSelectors: $labelSelectors
            })
            CREATE (p)-[:HAS]->(q)
            `

            // 4. Create metric nodes and relationships
            for _, metricName := range extraction.MetricNames {
                query := `
                MATCH (q:Query {id: $queryID})
                MERGE (m:Metric {name: $metricName})
                ON CREATE SET m.firstSeen = $now
                SET m.lastSeen = $now
                CREATE (q)-[:USES]->(m)
                `
            }
        }
    }

    return nil
}
```

### Pattern 4: Variable Handling as Passthrough with Metadata
**What:** Store Grafana variables as JSON metadata on Dashboard node, preserve variable syntax in PromQL
**When to use:** Handling dashboard-level template variables ($var, ${var}, [[var]])
**Example:**
```go
// Source: Grafana variable syntax documentation
type DashboardVariables struct {
    Variables []Variable `json:"variables"`
}

type Variable struct {
    Name        string      `json:"name"`
    Type        string      `json:"type"`        // query, custom, interval
    Query       string      `json:"query"`       // For query type
    Options     []string    `json:"options"`     // For custom type
    DefaultValue string     `json:"default"`
    MultiValue  bool        `json:"multi"`
}

// Extract from dashboard JSON
func ExtractVariables(dashboard GrafanaDashboard) *DashboardVariables {
    vars := &DashboardVariables{Variables: make([]Variable, 0)}

    for _, v := range dashboard.Templating.List {
        vars.Variables = append(vars.Variables, Variable{
            Name:        v.Name,
            Type:        v.Type,
            Query:       v.Query,
            DefaultValue: v.Current.Value,
            MultiValue:  v.Multi,
        })
    }

    return vars
}

// Store as JSON property on Dashboard node
query := `
MERGE (d:Dashboard {uid: $uid})
SET d.variables = $variablesJSON
`

// Variable syntax patterns to preserve (don't parse)
var variablePatterns = []string{
    `\$\w+`,           // $var
    `\$\{\w+\}`,       // ${var}
    `\$\{\w+:\w+\}`,   // ${var:format}
    `\[\[\w+\]\]`,     // [[var]] (deprecated but still in use)
}

// When metric name contains variable, create relationship based on template
func shouldCreateMetricNode(metricName string) bool {
    // If metric contains variable syntax, don't create concrete Metric node
    for _, pattern := range variablePatterns {
        if matched, _ := regexp.MatchString(pattern, metricName); matched {
            return false // Store as pattern, not concrete metric
        }
    }
    return true
}
```

### Pattern 5: Periodic Sync with Watcher Pattern
**What:** Use IntegrationWatcher pattern for config file watching + independent sync loop for API polling
**When to use:** Background dashboard sync orchestration
**Example:**
```go
// Source: internal/config/integration_watcher.go pattern
type DashboardSyncer struct {
    grafanaClient *GrafanaClient
    graphClient   graph.Client
    logger        *logging.Logger

    syncInterval  time.Duration
    cancel        context.CancelFunc
    stopped       chan struct{}
}

func (s *DashboardSyncer) Start(ctx context.Context) error {
    ctx, cancel := context.WithCancel(ctx)
    s.cancel = cancel
    s.stopped = make(chan struct{})

    // Initial sync on startup
    if err := s.syncAll(ctx); err != nil {
        s.logger.Warn("Initial dashboard sync failed: %v", err)
    }

    // Start periodic sync loop
    go s.syncLoop(ctx)

    return nil
}

func (s *DashboardSyncer) syncLoop(ctx context.Context) {
    defer close(s.stopped)

    ticker := time.NewTicker(s.syncInterval) // 1 hour
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            s.logger.Info("Dashboard sync loop stopped")
            return

        case <-ticker.C:
            if err := s.syncAll(ctx); err != nil {
                s.logger.Error("Dashboard sync failed: %v", err)
            }
        }
    }
}

func (s *DashboardSyncer) syncAll(ctx context.Context) error {
    // Fetch all dashboards via Grafana API
    dashboards, err := s.grafanaClient.SearchDashboards(ctx)
    if err != nil {
        return fmt.Errorf("failed to fetch dashboards: %w", err)
    }

    s.logger.Info("Syncing %d dashboards", len(dashboards))

    for i, dash := range dashboards {
        // Log progress for UI feedback
        s.logger.Info("Syncing dashboard %d of %d: %s", i+1, len(dashboards), dash.Title)

        // Check if sync needed (version comparison)
        if !s.needsSync(dash) {
            continue
        }

        // Fetch full dashboard details
        full, err := s.grafanaClient.GetDashboard(ctx, dash.UID)
        if err != nil {
            s.logger.Warn("Failed to fetch dashboard %s: %v", dash.UID, err)
            continue // Log and continue
        }

        // Sync to graph
        if err := s.syncDashboard(ctx, full); err != nil {
            s.logger.Warn("Failed to sync dashboard %s: %v", dash.UID, err)
            continue // Log and continue
        }
    }

    return nil
}
```

### Anti-Patterns to Avoid
- **Parsing variables as metrics:** Grafana variables like `$service` should NOT create Metric nodes - store as metadata
- **Partial dashboard updates:** Always use full-replace pattern to ensure removed panels/queries are cleaned up
- **Blocking on parse errors:** Log unparseable PromQL and continue sync - don't fail entire sync for one bad query
- **Creating separate nodes for aggregation functions:** Store as properties on Query node, not as separate Function nodes
- **Timestamp-only change detection:** Use version field as authoritative change indicator, timestamps have granularity issues

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| PromQL parsing | Custom regex-based parser | prometheus/prometheus/promql/parser | 160+ built-in functions, complex grammar (subqueries, operators, precedence), extensive edge cases |
| Metric name extraction | String splitting on `{` | parser.VectorSelector.Name | Handles metric names with special chars, nested expressions, matrix selectors |
| Variable syntax detection | Simple regex replace | Preserve original + metadata | Grafana has 4+ syntax variants, format specifiers (:csv, :raw, :regex), multi-value expansion |
| Change detection | File checksum/hash | Version field comparison | Grafana maintains authoritative version counter, increments on every save |
| Dashboard fetching | HTTP client from scratch | Existing HTTP patterns | Authentication, pagination, rate limiting, error handling already solved |
| Graph schema evolution | Manual Cypher migration | MERGE with ON CREATE SET | FalkorDB handles upsert semantics, idempotent operations |

**Key insight:** PromQL is a complex expression language with 160+ functions, operator precedence, subqueries, and matrix/vector selectors. The official Prometheus parser handles all edge cases including nested aggregations (`sum(rate(metric[5m])) by (label)`), binary operators, and comparison operators. Building a custom parser would miss critical features and fail on production queries.

## Common Pitfalls

### Pitfall 1: Assuming VectorSelector Always Has Name
**What goes wrong:** Some PromQL queries use label matchers without metric name: `{job="api", handler="/health"}`
**Why it happens:** VectorSelector.Name is empty string when query selects by labels only
**How to avoid:** Check `if vs.Name != ""` before using metric name, consider label matchers as alternative
**Warning signs:** Panics or empty metric names in graph, queries with only `{}` selectors

### Pitfall 2: Not Handling Parser Errors Gracefully
**What goes wrong:** Single unparseable query crashes entire dashboard sync
**Why it happens:** Grafana dashboards may contain invalid PromQL (typos, unsupported extensions)
**How to avoid:** Wrap parser.ParseExpr in error handler, log error and continue sync
**Warning signs:** Sync stops partway through dashboard list, no error visibility in UI

### Pitfall 3: Creating Duplicate Metric Nodes
**What goes wrong:** Same metric name creates multiple nodes because of different label matchers
**Why it happens:** Using full query string as node identifier instead of just metric name
**How to avoid:** Use `MERGE (m:Metric {name: $metricName})` - upsert based on name only
**Warning signs:** Graph grows unbounded, duplicate metrics in query results

### Pitfall 4: Deleting Metrics Used by Other Dashboards
**What goes wrong:** Orphan cleanup deletes Metric nodes still referenced by other dashboards
**Why it happens:** Deleting dashboard removes all connected nodes without checking references
**How to avoid:** Only delete Dashboard/Panel/Query nodes, keep Metric nodes (they're shared entities)
**Warning signs:** Metrics disappear from graph when one dashboard is deleted

### Pitfall 5: Variable Syntax in Metric Names Breaking Graph Relationships
**What goes wrong:** Metrics like `http_requests_$service_total` create nonsense nodes or fail to parse
**Why it happens:** Treating variable syntax as literal metric name
**How to avoid:** Detect variable patterns before creating Metric nodes, store query pattern instead
**Warning signs:** Metric nodes with `$`, `${`, or `[[` in name field

### Pitfall 6: Grafana API Version Field Not Incrementing
**What goes wrong:** Version field comparison misses changes
**Why it happens:** Assumption that version field is maintained correctly
**How to avoid:** Log version transitions, add fallback to timestamp comparison
**Warning signs:** Dashboards not re-syncing after known changes

### Pitfall 7: SecretWatcher Duplication
**What goes wrong:** Both VictoriaLogs and Grafana integrations have separate SecretWatcher implementations
**Why it happens:** Each integration developed independently
**How to avoid:** Accept duplication for Phase 16, plan refactor to common package in future phase
**Warning signs:** Identical code in victorialogs/ and grafana/ packages

## Code Examples

Verified patterns from official sources:

### Grafana API - Fetch Dashboards with Version
```go
// Source: https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/dashboard/
type GrafanaDashboard struct {
    Dashboard struct {
        UID     string `json:"uid"`
        Title   string `json:"title"`
        Version int    `json:"version"`
        Panels  []struct {
            ID      int    `json:"id"`
            Title   string `json:"title"`
            Type    string `json:"type"`
            GridPos struct {
                X int `json:"x"`
                Y int `json:"y"`
                W int `json:"w"`
                H int `json:"h"`
            } `json:"gridPos"`
            Targets []struct {
                RefID      string `json:"refId"`
                Expr       string `json:"expr"`       // PromQL query
                Datasource struct {
                    Type string `json:"type"`
                    UID  string `json:"uid"`
                } `json:"datasource"`
            } `json:"targets"`
        } `json:"panels"`
        Templating struct {
            List []struct {
                Name    string `json:"name"`
                Type    string `json:"type"`
                Query   string `json:"query"`
                Current struct {
                    Value string `json:"value"`
                } `json:"current"`
                Multi bool `json:"multi"`
            } `json:"list"`
        } `json:"templating"`
    } `json:"dashboard"`
    Meta struct {
        URL      string `json:"url"`
        FolderID int    `json:"folderId"`
    } `json:"meta"`
}

func (c *GrafanaClient) GetDashboard(ctx context.Context, uid string) (*GrafanaDashboard, error) {
    url := fmt.Sprintf("%s/api/dashboards/uid/%s", c.baseURL, uid)
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+c.token)

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var dashboard GrafanaDashboard
    if err := json.NewDecoder(resp.Body).Decode(&dashboard); err != nil {
        return nil, err
    }

    return &dashboard, nil
}
```

### PromQL Parser - Extract Aggregations
```go
// Source: https://pkg.go.dev/github.com/prometheus/prometheus/promql/parser
import "github.com/prometheus/prometheus/promql/parser"

func ExtractAggregations(queryStr string) ([]string, error) {
    expr, err := parser.ParseExpr(queryStr)
    if err != nil {
        return nil, fmt.Errorf("parse error: %w", err)
    }

    aggregations := make([]string, 0)

    parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
        switch n := node.(type) {
        case *parser.AggregateExpr:
            // Aggregation operators: sum, min, max, avg, stddev, count, etc.
            aggregations = append(aggregations, n.Op.String())

        case *parser.Call:
            // Function calls: rate, increase, irate, etc.
            aggregations = append(aggregations, n.Func.Name)
        }
        return nil
    })

    return aggregations, nil
}

// Example: "sum(rate(http_requests_total[5m])) by (status)"
// Returns: ["sum", "rate"]
```

### FalkorDB - Create Dashboard Graph
```go
// Source: https://github.com/FalkorDB/falkordb-go + internal/graph/client.go pattern
func (c *falkorClient) CreateDashboardNode(ctx context.Context, dashboard *DashboardNode) error {
    query := `
    MERGE (d:Dashboard {uid: $uid})
    ON CREATE SET
        d.title = $title,
        d.version = $version,
        d.tags = $tags,
        d.folder = $folder,
        d.url = $url,
        d.firstSeen = $firstSeen,
        d.lastSeen = $lastSeen
    ON MATCH SET
        d.title = $title,
        d.version = $version,
        d.tags = $tags,
        d.folder = $folder,
        d.url = $url,
        d.lastSeen = $lastSeen
    `

    params := map[string]interface{}{
        "uid":       dashboard.UID,
        "title":     dashboard.Title,
        "version":   dashboard.Version,
        "tags":      dashboard.Tags,
        "folder":    dashboard.Folder,
        "url":       dashboard.URL,
        "firstSeen": dashboard.FirstSeen,
        "lastSeen":  dashboard.LastSeen,
    }

    _, err := c.graph.Query(query, params, nil)
    return err
}

func (c *falkorClient) DeletePanelsForDashboard(ctx context.Context, dashboardUID string) error {
    // Full replace pattern - delete all panels and queries for this dashboard
    // Keep Metric nodes as they may be shared with other dashboards
    query := `
    MATCH (d:Dashboard {uid: $uid})-[:CONTAINS]->(p:Panel)
    OPTIONAL MATCH (p)-[:HAS]->(q:Query)
    DETACH DELETE p, q
    `

    params := map[string]interface{}{
        "uid": dashboardUID,
    }

    _, err := c.graph.Query(query, params, nil)
    return err
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| String parsing PromQL | AST-based parsing with prometheus/promql/parser | Prometheus 2.x (2017+) | Reliable metric extraction, handles complex queries |
| Grafana API v1 (numeric IDs) | Dashboard UID-based API | Grafana 5.0+ (2018) | Stable identifiers across renames |
| `[[var]]` variable syntax | `$var` and `${var}` syntax | Grafana 7.0+ (2020) | Simplified, `[[]]` deprecated |
| Manual dashboard version tracking | Built-in version field | Grafana core feature | Authoritative change detection |
| Full graph rebuild | Incremental sync with version comparison | Best practice evolution | Performance at scale |

**Deprecated/outdated:**
- `[[varname]]` bracket syntax: Deprecated in Grafana 7.0+, will be removed in future release - still parse for compatibility
- Dashboard numeric ID: Replaced by UID for stable references
- `/api/dashboards/db` endpoint: Legacy, use `/api/dashboards/uid/:uid` instead

## Open Questions

Things that couldn't be fully resolved:

1. **Query→Metric relationship when metric name contains variable**
   - What we know: Variables like `${service}` can appear in metric names
   - What's unclear: Whether to create pattern-based Metric node or skip entirely
   - Recommendation: Don't create Metric nodes for variable-containing names, store query pattern as property on Query node for downstream MCP tools

2. **Grafana API rate limiting and pagination**
   - What we know: Search dashboards endpoint exists
   - What's unclear: Maximum dashboards per response, rate limits
   - Recommendation: Start with simple search, add pagination if needed (test with 100+ dashboards)

3. **Dashboard deletion detection**
   - What we know: Version field helps detect changes
   - What's unclear: How to detect when dashboard is deleted from Grafana
   - Recommendation: Compare fetched dashboard UIDs with existing Dashboard nodes, mark missing ones as deleted

4. **PromQL query validation before storage**
   - What we know: parser.ParseExpr handles validation
   - What's unclear: Whether to store unparseable queries or skip entirely
   - Recommendation: Store raw PromQL even if unparseable (for debugging), mark Query node as `parseable: false`

## Sources

### Primary (HIGH confidence)
- [Prometheus PromQL Parser - pkg.go.dev](https://pkg.go.dev/github.com/prometheus/prometheus/promql/parser) - Official parser API documentation
- [Grafana Dashboard HTTP API](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/dashboard/) - Dashboard API with version field
- [Grafana Variable Syntax](https://grafana.com/docs/grafana/latest/visualizations/dashboards/variables/variable-syntax/) - Official variable syntax documentation
- [FalkorDB Go Client - GitHub](https://github.com/FalkorDB/falkordb-go) - Official Go client library
- [FalkorDB Cypher CREATE](https://docs.falkordb.com/cypher/create.html) - Official Cypher documentation

### Secondary (MEDIUM confidence)
- [PromQL Query Functions](https://prometheus.io/docs/prometheus/latest/querying/functions/) - Official aggregation function reference
- [Graph Database Best Practices - Microsoft](https://playbook.microsoft.com/code-with-dataops/guidance/graph-database-best-practices/) - Node/relationship modeling patterns
- [Incremental Synchronization - Airbyte](https://glossary.airbyte.com/term/incremental-synchronization/) - Version-based sync patterns

### Tertiary (LOW confidence)
- [PromQL Cheat Sheet - PromLabs](https://promlabs.com/promql-cheat-sheet/) - Community aggregation examples
- [Grafana Dashboard JSON Model](https://grafana.com/docs/grafana/latest/visualizations/dashboards/build-dashboards/view-dashboard-json-model/) - Panel structure (incomplete targets documentation)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Official libraries verified via pkg.go.dev and GitHub
- Architecture: HIGH - Patterns verified in existing codebase (internal/config/integration_watcher.go, internal/graph/client.go)
- Pitfalls: MEDIUM - Based on WebSearch findings and parser documentation, not production experience
- PromQL parsing: HIGH - Official Prometheus parser documentation with code examples
- Grafana API: HIGH - Official Grafana documentation
- Graph patterns: MEDIUM - FalkorDB official docs + graph database best practices

**Research date:** 2026-01-22
**Valid until:** 2026-02-22 (30 days - stable libraries, established patterns)
