# Grafana Integration Architecture

**Domain:** Grafana dashboard ingestion and semantic graph storage
**Researched:** 2026-01-22
**Confidence:** HIGH

## Executive Summary

The Grafana integration follows Spectre's existing plugin architecture pattern, extending it for metrics-focused observability. The architecture consists of six main components: dashboard sync, PromQL parser, graph storage, query executor, anomaly detector, and MCP tools. The design prioritizes incremental sync, structured graph queries, and integration with existing infrastructure (FalkorDB, MCP server, plugin system).

**Key architectural decision:** Parse PromQL **at ingestion time** (not query time) to extract metric selectors, labels, and aggregation functions into the graph. This enables semantic queries ("show me all dashboards tracking pod memory") without re-parsing queries.

## Recommended Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         MCP Tools Layer                              │
│  grafana_{name}_dashboards | grafana_{name}_metrics_for_resource   │
│  grafana_{name}_query      | grafana_{name}_detect_anomalies       │
└────────────────┬────────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────────┐
│                    Service Layer (new)                               │
│  GrafanaQueryService     | GrafanaAnomalyService                    │
│  (execute PromQL)        | (baseline + comparison)                  │
└────────────────┬────────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────────┐
│                    Graph Storage (FalkorDB)                          │
│  Nodes: Dashboard, Panel, Metric, Resource (K8s)                    │
│  Edges: CONTAINS, QUERIES, TRACKS, AGGREGATES_WITH                  │
└────────────────┬────────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────────┐
│                    PromQL Parser (new)                               │
│  github.com/prometheus/prometheus/promql/parser                      │
│  Extract: metric names, label selectors, aggregations               │
└────────────────┬────────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────────┐
│                Dashboard Sync Pipeline (new)                         │
│  GrafanaSyncer: Poll API → Parse dashboards → Update graph          │
│  Sync strategy: Incremental (uid-based change detection)            │
└────────────────┬────────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────────┐
│                    Grafana HTTP Client (new)                         │
│  API endpoints: /api/search, /api/dashboards/uid/:uid               │
│  Auth: Service account token (secret ref pattern)                   │
└──────────────────────────────────────────────────────────────────────┘
```

### Component Boundaries

| Component | Responsibility | Package Path | Communicates With |
|-----------|---------------|--------------|-------------------|
| **GrafanaIntegration** | Lifecycle management, tool registration | `internal/integration/grafana/` | Integration manager, MCP registry |
| **GrafanaClient** | HTTP API wrapper for Grafana | `internal/integration/grafana/client.go` | Grafana Cloud/self-hosted API |
| **DashboardSyncer** | Dashboard ingestion pipeline | `internal/integration/grafana/syncer.go` | GrafanaClient, PromQLParser, GraphClient |
| **PromQLParser** | Parse PromQL into semantic AST | `internal/integration/grafana/promql_parser.go` | Prometheus parser library |
| **GraphSchema** | Graph node/edge definitions | `internal/integration/grafana/graph_schema.go` | FalkorDB (via existing graph.Client) |
| **QueryService** | Execute queries against Grafana | `internal/integration/grafana/query_service.go` | GrafanaClient, GraphClient |
| **AnomalyService** | Baseline computation, comparison | `internal/integration/grafana/anomaly_service.go` | QueryService, GraphClient |
| **MCP Tools** | Tool implementations | `internal/integration/grafana/tools_*.go` | QueryService, AnomalyService |

### Data Flow

```
Dashboard Ingestion Flow:
1. GrafanaSyncer.Poll() → GET /api/search (list dashboards)
2. For each changed dashboard (compare uid + version):
   a. GET /api/dashboards/uid/:uid → full dashboard JSON
   b. PromQLParser.ParseDashboard() → extract panels + PromQL
   c. For each panel with PromQL:
      - PromQLParser.Parse(query) → AST
      - ExtractSemantics(AST) → {metric, labels, aggregations}
   d. GraphClient.ExecuteQuery(UpsertDashboard) → create/update nodes
   e. GraphClient.ExecuteQuery(LinkToResources) → connect to K8s resources
3. Store sync state (last_synced timestamp)

Query Execution Flow:
1. MCP tool receives request → QueryService.ExecuteQuery(promql, timeRange)
2. QueryService → GrafanaClient.QueryRange(promql, start, end)
3. GrafanaClient → POST /api/datasources/proxy/:id/api/v1/query_range
4. Return time series data to MCP tool

Anomaly Detection Flow:
1. MCP tool → AnomalyService.DetectAnomalies(resourceUID, metricName, timeRange)
2. AnomalyService.ComputeBaseline() → query past 7 days → calculate p50, p95, stddev
3. AnomalyService.QueryCurrent() → query current window
4. AnomalyService.Compare() → detect outliers (z-score, percentile thresholds)
5. Return anomaly events with severity
```

## Graph Schema Design

### Node Types

```cypher
// Dashboard node represents a Grafana dashboard
(:Dashboard {
  uid: string,              // Grafana dashboard UID (primary key)
  title: string,            // Dashboard title
  folder: string,           // Folder name
  tags: [string],           // Dashboard tags
  url: string,              // Full URL to dashboard
  version: int,             // Dashboard version (for change detection)
  grafana_instance: string, // Instance name (e.g., "grafana-prod")
  last_synced: int64,       // Unix nanoseconds
  created: int64,
  updated: int64
})

// Panel node represents a single panel in a dashboard
(:Panel {
  id: string,               // Composite: "{dashboard_uid}:{panel_id}"
  dashboard_uid: string,    // Parent dashboard UID
  panel_id: int,            // Panel ID within dashboard
  title: string,            // Panel title
  panel_type: string,       // "graph", "stat", "table", etc.
  datasource: string,       // Datasource name/UID
  promql: string,           // Original PromQL query (if applicable)
  description: string
})

// Metric node represents a Prometheus metric being queried
(:Metric {
  name: string,             // Metric name (e.g., "container_memory_usage_bytes")
  metric_type: string,      // "counter", "gauge", "histogram", "summary" (inferred)
  help: string,             // Metric description (from /api/v1/metadata if available)
  unit: string,             // Metric unit (inferred from name/metadata)
  first_seen: int64,
  last_seen: int64
})

// MetricLabel represents a label selector in PromQL
(:MetricLabel {
  key: string,              // Label key (e.g., "namespace")
  value: string,            // Label value (e.g., "prod") or pattern (e.g., "~prod-.*")
  operator: string          // "=", "!=", "=~", "!~"
})

// Aggregation represents an aggregation function in PromQL
(:Aggregation {
  function: string,         // "sum", "avg", "max", "min", "count", etc.
  by_labels: [string],      // GROUP BY labels
  without_labels: [string]  // GROUP WITHOUT labels
})
```

**Reuse existing nodes:**
- `ResourceIdentity` - K8s resources (Pod, Deployment, etc.) already in graph
- `ChangeEvent` - K8s state changes already tracked

### Edge Types

```cypher
// Dashboard → Panel relationship
(Dashboard)-[:CONTAINS {
  position: int             // Panel position/order in dashboard
}]->(Panel)

// Panel → Metric relationship (what metrics does this panel query?)
(Panel)-[:QUERIES {
  promql_fragment: string   // Specific PromQL subquery if panel has multiple
}]->(Metric)

// Panel → MetricLabel relationship (what label selectors are used?)
(Panel)-[:FILTERS_BY]->(MetricLabel)

// Panel → Aggregation relationship (what aggregations are applied?)
(Panel)-[:AGGREGATES_WITH]->(Aggregation)

// Metric → ResourceIdentity relationship (semantic linking)
// Links metrics to K8s resources based on label matching
(Metric)-[:TRACKS {
  confidence: float,        // 0.0-1.0 confidence score
  label_match: string,      // Which label was used for linking (e.g., "pod")
  evidence: string          // JSON evidence for relationship
}]->(ResourceIdentity)

// Panel → ResourceIdentity relationship (derived from QUERIES + TRACKS)
// Enables: "show me dashboards tracking this pod"
(Panel)-[:MONITORS {
  via_metric: string,       // Metric name used for connection
  confidence: float
}]->(ResourceIdentity)
```

### Schema Indexing

Following existing pattern in `internal/graph/client.go`:

```go
// Create indexes for fast lookups
CREATE INDEX ON :Dashboard(uid)
CREATE INDEX ON :Dashboard(grafana_instance)
CREATE INDEX ON :Panel(id)
CREATE INDEX ON :Panel(dashboard_uid)
CREATE INDEX ON :Metric(name)
CREATE INDEX ON :MetricLabel(key)
CREATE INDEX ON :Aggregation(function)
```

## PromQL Parsing Strategy

### When to Parse

**Parse at ingestion time** (dashboard sync), not query time.

**Rationale:**
- Parsing is expensive - do it once during sync, not on every MCP query
- Enables semantic graph queries without re-parsing
- Allows pre-computation of metric→resource relationships
- Supports "show me all dashboards using this metric" queries instantly

### What to Extract

Using `github.com/prometheus/prometheus/promql/parser`:

```go
// Example PromQL: sum(rate(container_cpu_usage_seconds_total{namespace="prod", pod=~"api-.*"}[5m])) by (pod)

type ParsedQuery struct {
  OriginalQuery string
  Metrics       []string          // ["container_cpu_usage_seconds_total"]
  Labels        []LabelSelector   // [{key: "namespace", op: "=", value: "prod"}, ...]
  Aggregations  []AggregationFunc // [{function: "sum", by: ["pod"]}]
  RangeDuration string            // "5m" (for rate/increase/etc.)
  Functions     []string          // ["rate", "sum"]
}

func ParsePromQL(query string) (*ParsedQuery, error) {
  // Use prometheus/promql/parser
  expr, err := parser.ParseExpr(query)
  if err != nil {
    return nil, err
  }

  // Traverse AST with parser.Inspect()
  parsed := &ParsedQuery{OriginalQuery: query}
  parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
    switch n := node.(type) {
    case *parser.VectorSelector:
      parsed.Metrics = append(parsed.Metrics, n.Name)
      for _, matcher := range n.LabelMatchers {
        parsed.Labels = append(parsed.Labels, LabelSelector{
          Key: matcher.Name,
          Op: matcher.Type.String(),
          Value: matcher.Value,
        })
      }
    case *parser.AggregateExpr:
      parsed.Aggregations = append(parsed.Aggregations, AggregationFunc{
        Function: n.Op.String(),
        By: n.Grouping,
        Without: n.Without,
      })
    case *parser.Call:
      parsed.Functions = append(parsed.Functions, n.Func.Name)
    }
    return nil
  })

  return parsed, nil
}
```

### Handling Complex Queries

**PromQL supports:**
- Binary operations: `metric1 / metric2`
- Subqueries: `max_over_time(rate(metric[5m])[1h:1m])`
- Multiple vector selectors in one query

**Strategy:**
- Extract ALL metrics referenced (may be multiple per panel)
- Create separate `QUERIES` edges for each metric
- Store aggregation tree as JSON if needed for reconstruction
- **Limitation:** Don't try to execute PromQL in Spectre - delegate to Grafana

## Sync Frequency and Strategy

### Incremental Sync (Recommended)

Based on research, Grafana's API supports UID-based dashboard retrieval and version tracking.

**Sync algorithm:**
```go
func (s *DashboardSyncer) SyncIncremental(ctx context.Context) error {
  // 1. List all dashboards (lightweight)
  dashboards, err := s.client.SearchDashboards(ctx, SearchParams{})

  // 2. Compare with last sync state
  for _, dash := range dashboards {
    lastVersion := s.getSyncedVersion(dash.UID)
    if dash.Version > lastVersion {
      // 3. Fetch full dashboard
      full, err := s.client.GetDashboard(ctx, dash.UID)

      // 4. Parse and update graph
      if err := s.ingestDashboard(ctx, full); err != nil {
        s.logger.Warn("Failed to ingest %s: %v", dash.UID, err)
        continue
      }

      // 5. Update sync state
      s.setSyncedVersion(dash.UID, dash.Version)
    }
  }

  return nil
}
```

**Sync frequency:** 60 seconds (default), configurable via integration config

**Change detection:**
- Use dashboard `version` field (incremented by Grafana on each save)
- Store last synced version in graph: `Dashboard.version`
- Only fetch changed dashboards (reduces API calls)

**Fallback for version-less dashboards:**
- Use `updated` timestamp comparison
- Full re-sync if state is lost (initial sync or after restart)

### Full Sync (Initial Load)

```go
func (s *DashboardSyncer) SyncFull(ctx context.Context) error {
  // Fetch ALL dashboards and ingest
  // Used for:
  // - Initial sync when integration starts
  // - Manual refresh triggered by operator
  // - Recovery after graph clear
}
```

## Query Execution Architecture

### Service Layer Design

Following Spectre's pattern of service injection into tools:

```go
// GrafanaQueryService executes PromQL queries against Grafana
type GrafanaQueryService struct {
  client      *GrafanaClient
  graphClient graph.Client
  logger      *logging.Logger
}

func (s *GrafanaQueryService) QueryRange(ctx context.Context, params QueryRangeParams) (*QueryRangeResult, error) {
  // 1. Validate params
  // 2. Query Grafana datasource proxy API
  // 3. Parse Prometheus response format
  // 4. Return time series data
}

func (s *GrafanaQueryService) GetDashboardsForResource(ctx context.Context, resourceUID string) ([]DashboardInfo, error) {
  // Use graph query to find dashboards monitoring this resource
  query := `
    MATCH (r:ResourceIdentity {uid: $uid})<-[:MONITORS]-(p:Panel)<-[:CONTAINS]-(d:Dashboard)
    RETURN DISTINCT d
  `
  // Execute and parse
}
```

### MCP Tool Invocation Flow

```go
// Tool: grafana_{name}_query
type QueryTool struct {
  queryService *GrafanaQueryService
}

func (t *QueryTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
  var params QueryParams
  json.Unmarshal(args, &params)

  // Delegate to service
  result, err := t.queryService.QueryRange(ctx, params.ToQueryRangeParams())

  // Format for LLM consumption
  return FormatTimeSeriesForLLM(result), nil
}
```

**Why this pattern:**
- Services are testable in isolation (mock client)
- Tools remain thin adapters
- Matches existing pattern (TimelineService, GraphService)

## Anomaly Detection Pipeline

### Baseline Computation Strategy

Based on research, statistical methods are effective and avoid ML complexity:

```go
type BaselineMetrics struct {
  Metric     string
  TimeWindow time.Duration  // e.g., 7 days
  P50        float64        // Median
  P95        float64        // 95th percentile
  P99        float64        // 99th percentile
  Mean       float64
  StdDev     float64
  SampleSize int
}

func (s *GrafanaAnomalyService) ComputeBaseline(ctx context.Context, params BaselineParams) (*BaselineMetrics, error) {
  // 1. Query historical data (past 7 days by default)
  queryParams := QueryRangeParams{
    Query: params.PromQL,
    Start: time.Now().Add(-7 * 24 * time.Hour),
    End:   time.Now(),
    Step:  5 * time.Minute,  // Configurable resolution
  }

  result, err := s.queryService.QueryRange(ctx, queryParams)

  // 2. Aggregate samples (flatten time series)
  samples := flattenTimeSeries(result)

  // 3. Calculate statistics
  baseline := &BaselineMetrics{
    Metric:     params.Metric,
    TimeWindow: 7 * 24 * time.Hour,
    P50:        percentile(samples, 0.50),
    P95:        percentile(samples, 0.95),
    P99:        percentile(samples, 0.99),
    Mean:       mean(samples),
    StdDev:     stddev(samples),
    SampleSize: len(samples),
  }

  return baseline, nil
}
```

**Baseline caching:**
- Store baselines in FalkorDB with TTL (e.g., 1 hour)
- Node: `(:MetricBaseline {metric: string, computed_at: int64, ...stats})`
- Recompute on cache miss or TTL expiry

### Comparison Logic

```go
type AnomalyDetectionParams struct {
  ResourceUID   string
  MetricName    string
  StartTime     time.Time
  EndTime       time.Time
  Sensitivity   string  // "low", "medium", "high"
}

type AnomalyEvent struct {
  Timestamp     time.Time
  Value         float64
  BaselineValue float64  // Expected value (p50 or mean)
  Deviation     float64  // How many stddevs away
  Severity      string   // "info", "warning", "critical"
  Reason        string   // Human-readable explanation
}

func (s *GrafanaAnomalyService) DetectAnomalies(ctx context.Context, params AnomalyDetectionParams) ([]AnomalyEvent, error) {
  // 1. Get or compute baseline
  baseline, err := s.getOrComputeBaseline(ctx, params.MetricName)

  // 2. Query current window
  current, err := s.queryService.QueryRange(ctx, QueryRangeParams{
    Query: buildQueryForMetric(params.MetricName, params.ResourceUID),
    Start: params.StartTime,
    End:   params.EndTime,
  })

  // 3. Compare each sample to baseline
  anomalies := []AnomalyEvent{}
  threshold := getSensitivityThreshold(params.Sensitivity)

  for _, sample := range current.Samples {
    zscore := (sample.Value - baseline.Mean) / baseline.StdDev

    if math.Abs(zscore) > threshold {
      severity := classifySeverity(zscore, baseline)
      anomalies = append(anomalies, AnomalyEvent{
        Timestamp:     sample.Timestamp,
        Value:         sample.Value,
        BaselineValue: baseline.Mean,
        Deviation:     zscore,
        Severity:      severity,
        Reason:        fmt.Sprintf("Value %.2f is %.1f stddevs from baseline mean %.2f",
                                   sample.Value, zscore, baseline.Mean),
      })
    }
  }

  return anomalies, nil
}

func getSensitivityThreshold(sensitivity string) float64 {
  switch sensitivity {
  case "high":
    return 2.0  // 2 sigma
  case "medium":
    return 2.5  // 2.5 sigma
  case "low":
    return 3.0  // 3 sigma
  default:
    return 2.5
  }
}
```

**Anomaly severity classification:**
- `info`: 2-3 sigma deviation, within p95
- `warning`: 3-4 sigma, exceeds p95 but below p99
- `critical`: >4 sigma, exceeds p99

## Integration with Existing Plugin System

### Integration Config Structure

Following VictoriaLogs pattern in `internal/config/integration_config.go`:

```yaml
schema_version: v1
instances:
  - name: grafana-prod
    type: grafana
    enabled: true
    config:
      url: "https://myorg.grafana.net"
      apiTokenRef:
        secretName: grafana-api-token
        key: token
      datasource_uid: "prometheus-prod"  # Which datasource to query
      sync_interval: 60  # seconds
      sync_enabled: true
```

**Config validation:**
```go
type Config struct {
  URL            string     `json:"url" yaml:"url"`
  APITokenRef    *SecretRef `json:"apiTokenRef,omitempty" yaml:"apiTokenRef,omitempty"`
  DatasourceUID  string     `json:"datasource_uid" yaml:"datasource_uid"`
  SyncInterval   int        `json:"sync_interval" yaml:"sync_interval"`
  SyncEnabled    bool       `json:"sync_enabled" yaml:"sync_enabled"`
}

func (c *Config) Validate() error {
  if c.URL == "" {
    return fmt.Errorf("url is required")
  }
  if c.APITokenRef == nil {
    return fmt.Errorf("apiTokenRef is required")
  }
  if c.DatasourceUID == "" {
    return fmt.Errorf("datasource_uid is required")
  }
  if c.SyncInterval < 10 {
    return fmt.Errorf("sync_interval must be >= 10 seconds")
  }
  return nil
}
```

### Factory Registration

```go
// internal/integration/grafana/grafana.go
func init() {
  if err := integration.RegisterFactory("grafana", NewGrafanaIntegration); err != nil {
    logger := logging.GetLogger("integration.grafana")
    logger.Warn("Failed to register grafana factory: %v", err)
  }
}

func NewGrafanaIntegration(name string, configMap map[string]interface{}) (integration.Integration, error) {
  // Parse config
  configJSON, _ := json.Marshal(configMap)
  var config Config
  json.Unmarshal(configJSON, &config)

  if err := config.Validate(); err != nil {
    return nil, err
  }

  return &GrafanaIntegration{
    name:   name,
    config: config,
    logger: logging.GetLogger("integration.grafana." + name),
  }, nil
}
```

### Lifecycle Implementation

```go
type GrafanaIntegration struct {
  name          string
  config        Config
  client        *GrafanaClient
  syncer        *DashboardSyncer
  queryService  *GrafanaQueryService
  anomalyService *GrafanaAnomalyService
  secretWatcher *SecretWatcher
  logger        *logging.Logger
}

func (g *GrafanaIntegration) Start(ctx context.Context) error {
  // 1. Create secret watcher for API token
  // 2. Create HTTP client
  // 3. Test connectivity
  // 4. Initialize services
  // 5. Start dashboard syncer if enabled
  // 6. Initial sync
}

func (g *GrafanaIntegration) Stop(ctx context.Context) error {
  // Graceful shutdown: stop syncer, close connections
}

func (g *GrafanaIntegration) Health(ctx context.Context) integration.HealthStatus {
  // Test Grafana API connectivity
}

func (g *GrafanaIntegration) RegisterTools(registry integration.ToolRegistry) error {
  // Register MCP tools (dashboards, query, anomaly detection)
}
```

### Tool Registration Pattern

Following VictoriaLogs pattern:

```go
func (g *GrafanaIntegration) RegisterTools(registry integration.ToolRegistry) error {
  // Tool 1: List dashboards
  registry.RegisterTool(
    fmt.Sprintf("grafana_%s_dashboards", g.name),
    "List Grafana dashboards with optional filters",
    (&DashboardsTool{queryService: g.queryService}).Execute,
    dashboardsSchema,
  )

  // Tool 2: Query metrics
  registry.RegisterTool(
    fmt.Sprintf("grafana_%s_query", g.name),
    "Execute PromQL query and return time series data",
    (&QueryTool{queryService: g.queryService}).Execute,
    querySchema,
  )

  // Tool 3: Get metrics for resource
  registry.RegisterTool(
    fmt.Sprintf("grafana_%s_metrics_for_resource", g.name),
    "Find all metrics being tracked for a Kubernetes resource",
    (&MetricsForResourceTool{queryService: g.queryService}).Execute,
    metricsForResourceSchema,
  )

  // Tool 4: Detect anomalies
  registry.RegisterTool(
    fmt.Sprintf("grafana_%s_detect_anomalies", g.name),
    "Detect anomalies in metrics using baseline comparison",
    (&AnomalyDetectionTool{anomalyService: g.anomalyService}).Execute,
    anomalyDetectionSchema,
  )

  return nil
}
```

## Component Build Order

Suggested implementation sequence based on dependencies:

### Phase 1: Foundation (Week 1)
1. **HTTP Client** (`client.go`)
   - Grafana API wrapper
   - Authentication with secret ref
   - Endpoints: `/api/search`, `/api/dashboards/uid/:uid`, `/api/datasources/proxy`

2. **Graph Schema** (`graph_schema.go`)
   - Define node types (Dashboard, Panel, Metric)
   - Define edge types (CONTAINS, QUERIES, TRACKS)
   - Schema initialization queries

3. **Config & Integration Skeleton** (`grafana.go`, `types.go`)
   - Config struct and validation
   - Integration lifecycle (Start/Stop/Health)
   - Factory registration

### Phase 2: Ingestion (Week 2)
4. **PromQL Parser** (`promql_parser.go`)
   - Parse PromQL using Prometheus library
   - Extract metrics, labels, aggregations
   - Unit tests with various PromQL patterns

5. **Dashboard Syncer** (`syncer.go`)
   - Incremental sync algorithm
   - Dashboard → graph transformation
   - Version tracking for change detection

6. **Metric→Resource Linking** (`resource_linker.go`)
   - Heuristic matching (label-based)
   - Create TRACKS edges with confidence scores
   - Handle namespace, pod, container labels

### Phase 3: Query & Anomaly (Week 3)
7. **Query Service** (`query_service.go`)
   - Execute PromQL via Grafana datasource proxy
   - Format results for MCP tools
   - Graph queries for dashboard discovery

8. **Anomaly Service** (`anomaly_service.go`)
   - Baseline computation
   - Statistical comparison
   - Baseline caching in graph

### Phase 4: MCP Tools (Week 4)
9. **MCP Tools** (`tools_*.go`)
   - `grafana_{name}_dashboards` - List/search dashboards
   - `grafana_{name}_query` - Execute PromQL
   - `grafana_{name}_metrics_for_resource` - Reverse lookup
   - `grafana_{name}_detect_anomalies` - Anomaly detection

10. **Integration Testing** (`integration_test.go`)
    - End-to-end test with mock Grafana API
    - Sync pipeline test
    - Tool execution tests

## Integration Points with Existing Code

### 1. FalkorDB Graph Client

**Existing:** `internal/graph/client.go`, `internal/graph/schema.go`

**Usage:**
```go
// Reuse existing graph client interface
type Client interface {
  ExecuteQuery(ctx context.Context, query GraphQuery) (*QueryResult, error)
  InitializeSchema(ctx context.Context) error
}

// In DashboardSyncer
func (s *DashboardSyncer) ingestDashboard(ctx context.Context, dashboard *Dashboard) error {
  query := UpsertDashboardQuery(dashboard)
  _, err := s.graphClient.ExecuteQuery(ctx, query)
  return err
}
```

**New schema initialization:**
```go
// Add to internal/graph/schema.go
func InitializeGrafanaSchema(ctx context.Context, client Client) error {
  queries := []string{
    "CREATE INDEX ON :Dashboard(uid)",
    "CREATE INDEX ON :Dashboard(grafana_instance)",
    "CREATE INDEX ON :Panel(id)",
    "CREATE INDEX ON :Metric(name)",
  }
  // Execute schema queries
}
```

### 2. MCP Server

**Existing:** `internal/mcp/server.go`, tool registry pattern

**Usage:** Same pattern as VictoriaLogs - tools registered via `RegisterTools()`

### 3. Integration Manager

**Existing:** `internal/integration/manager.go`, factory registry

**Usage:** Register Grafana factory in `init()`, manager handles lifecycle

### 4. Config Hot-Reload

**Existing:** `internal/config/integration_watcher.go` (fsnotify-based)

**Automatic:** Config changes trigger integration restart via manager

### 5. Secret Management

**Existing:** VictoriaLogs `secret_watcher.go` pattern

**Reuse:** Copy pattern for Grafana API token management

```go
// Same pattern as VictoriaLogs
secretWatcher, err := NewSecretWatcher(
  clientset,
  namespace,
  g.config.APITokenRef.SecretName,
  g.config.APITokenRef.Key,
  g.logger,
)
```

## Performance Considerations

### Sync Pipeline

**Challenge:** Large Grafana instances (100+ dashboards, 1000+ panels)

**Mitigations:**
- Incremental sync (only changed dashboards)
- Concurrent dashboard fetching (worker pool pattern)
- Rate limiting for Grafana API (configurable QPS)
- Progress tracking and resumability

```go
type SyncProgress struct {
  TotalDashboards   int
  SyncedDashboards  int
  FailedDashboards  []string
  LastSyncTime      time.Time
  Duration          time.Duration
}
```

### Graph Query Performance

**Challenge:** "Show me all dashboards for this pod" could traverse many nodes

**Mitigations:**
- Indexes on frequently queried fields (uid, name, grafana_instance)
- Limit result sets (max 100 dashboards per query)
- Cache frequently accessed queries (e.g., dashboard list)
- Use graph query optimizer (FalkorDB's GraphBLAS backend)

### Baseline Computation

**Challenge:** 7-day baseline requires querying 2016 data points (5min resolution)

**Mitigations:**
- Cache baselines in graph (1 hour TTL)
- Async baseline computation (don't block tool calls)
- Configurable baseline window (trade accuracy for speed)
- Use Grafana's query downsampling (larger step size)

## API Rate Limiting

Grafana Cloud has rate limits: **600 requests/hour** for API endpoints.

**Strategy:**
```go
type RateLimiter struct {
  qps     float64  // Queries per second
  limiter *rate.Limiter
}

func (c *GrafanaClient) SearchDashboards(ctx context.Context) {
  // Wait for rate limit token
  c.rateLimiter.Wait(ctx)

  // Execute request
  resp, err := c.httpClient.Get(...)
}
```

**Configuration:**
```yaml
config:
  rate_limit_qps: 0.16  # 600/hour ≈ 0.16/sec, leave headroom
```

## Security Considerations

### API Token Storage

**Follow VictoriaLogs pattern:**
- Store token in Kubernetes Secret
- Reference via `apiTokenRef` in config
- SecretWatcher monitors for updates
- Never log token value

### PromQL Injection

**Risk:** User-controlled PromQL could query unauthorized metrics

**Mitigation:**
- MCP tools construct PromQL (don't accept arbitrary queries)
- Validate metric names against known set
- Use Grafana's RBAC (token permissions)

### Dashboard Access Control

**Risk:** Syncing dashboards user shouldn't see

**Mitigation:**
- Service account token with read-only access
- Sync only dashboards in allowed folders (config filter)
- Tag-based filtering (only sync tagged dashboards)

## Monitoring and Observability

### Prometheus Metrics

Following VictoriaLogs metrics pattern:

```go
type Metrics struct {
  syncDuration       prometheus.Histogram
  syncErrors         prometheus.Counter
  dashboardsSynced   prometheus.Counter
  apiRequestDuration prometheus.Histogram
  apiRequestErrors   *prometheus.CounterVec  // by endpoint
  baselineComputeDuration prometheus.Histogram
  anomaliesDetected  *prometheus.CounterVec  // by severity
}
```

### Logging

Structured logging at key points:
- Sync start/completion (with stats)
- API errors (with retry logic)
- Graph write errors
- Anomaly detection results

## Testing Strategy

### Unit Tests
- PromQL parser (various query patterns)
- Config validation
- Graph query builders
- Statistical functions (baseline, zscore)

### Integration Tests
- Mock Grafana API (httptest)
- In-memory graph (or test FalkorDB)
- End-to-end sync pipeline
- Tool execution

### E2E Tests
- Real Grafana instance (test env)
- Verify graph state after sync
- Query accuracy
- Anomaly detection with known data

## Open Questions & Future Work

### Unanswered in Research
1. **Metric metadata availability** - Can we get metric type/unit from Grafana API? (Fallback: heuristics from metric name)
2. **Dashboard provisioning sync** - How to handle Git-synced dashboards? (May have different change detection)
3. **Alert rule integration** - Should we sync Grafana alert rules? (Future phase)

### Future Enhancements
1. **Multi-datasource support** - Currently assumes single Prometheus datasource
2. **Dashboard annotations** - Sync annotations for correlation with K8s events
3. **Custom variable handling** - Parse dashboard variables for dynamic queries
4. **Metric cardinality tracking** - Warn on high-cardinality metrics
5. **Cross-instance correlation** - Link dashboards across multiple Grafana instances

## Sources

**Grafana API Documentation:**
- [Dashboard HTTP API | Grafana documentation](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/dashboard/)
- [Grafana Cloud API | Grafana documentation](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/cloud-api/)
- [Git Sync | Grafana documentation](https://grafana.com/docs/grafana/latest/as-code/observability-as-code/provision-resources/intro-git-sync/)

**PromQL Parsing:**
- [prometheus/promql/parser - Go Packages](https://pkg.go.dev/github.com/prometheus/prometheus/promql/parser)
- [Inside PromQL: A closer look at the mechanics of a Prometheus query | Grafana Labs](https://grafana.com/blog/2024/10/08/inside-promql-a-closer-look-at-the-mechanics-of-a-prometheus-query/)

**Graph Database Design:**
- [Graph Database Guide for AI Architects | 2026 - FalkorDB](https://www.falkordb.com/blog/graph-database-guide/)
- [The FalkorDB Design | FalkorDB Docs](https://docs.falkordb.com/design/)

**Anomaly Detection:**
- [Anomaly Detection in Time Series Using Statistical Analysis | Booking.com Engineering](https://medium.com/booking-com-development/anomaly-detection-in-time-series-using-statistical-analysis-cc587b21d008)
- [TSB-AD: Towards A Reliable Time-Series Anomaly Detection Benchmark](https://github.com/TheDatumOrg/TSB-AD)

**Sync Strategies:**
- [Polling | Grafana Tempo documentation](https://grafana.com/docs/tempo/latest/configuration/polling/)
- [Common options | grafana-operator](https://grafana.github.io/grafana-operator/docs/examples/common_options/)
