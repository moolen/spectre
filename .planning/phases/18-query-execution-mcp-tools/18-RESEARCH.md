# Phase 18: Query Execution & MCP Tools Foundation - Research

**Researched:** 2026-01-23
**Domain:** Grafana Query API, MCP Tools, Time Series Data Formatting
**Confidence:** HIGH

## Summary

This phase builds three MCP tools (overview, aggregated, details) that execute Grafana queries via the `/api/ds/query` endpoint. The research covers Grafana's query API structure, time range handling, variable substitution, response formatting, and progressive disclosure patterns for MCP tools.

**Key findings:**
- Grafana `/api/ds/query` endpoint uses POST requests with datasource UID, query expressions, and time ranges
- Time ranges accept epoch milliseconds or relative formats (e.g., "now-5m")
- Variable substitution happens server-side via `scopedVars` parameter (not local interpolation)
- Progressive disclosure pattern essential for MCP tools - start minimal, expand on demand
- Partial results pattern critical for resilience - return what works, list what failed

The existing Grafana integration provides dashboard syncing, graph storage, and PromQL parsing. This phase adds query execution and tool registration on top of that foundation.

**Primary recommendation:** Build GrafanaQueryService using Grafana `/api/ds/query` endpoint, implement progressive disclosure in MCP tools (5 panels → drill-down → all panels), return partial results with clear error messages, use ISO8601 timestamps for precision.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http` (stdlib) | Go 1.24+ | Grafana API client | Production-ready, connection pooling, already used in existing GrafanaClient |
| `encoding/json` (stdlib) | Go 1.24+ | Request/response marshaling | Standard Go JSON handling, sufficient for API data |
| `time` (stdlib) | Go 1.24+ | Time range handling | ISO8601 formatting, duration calculations |
| `github.com/prometheus/prometheus/promql/parser` | v0.61.3+ | PromQL parsing (already integrated) | Official parser, extract metrics from queries |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/FalkorDB/falkordb-go/v2` | v2.0.2 (existing) | Graph queries for dashboard lookup | Find dashboards by hierarchy level |
| `github.com/mark3labs/mcp-go` | (existing in project) | MCP tool registration | Register three tools with MCP server |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Grafana `/api/ds/query` | Direct Prometheus API | Bypasses Grafana auth/variables, more complex |
| Absolute timestamps | Relative time ranges ("now-5m") | Relative simpler but less precise for historical queries |
| Full dashboard results | Lazy pagination | Adds complexity, not needed for AI consumption |

**Installation:**
```bash
# All dependencies already in project
# No new packages required for Phase 18
```

## Architecture Patterns

### Recommended Project Structure
```
internal/integration/grafana/
├── query_service.go        # NEW: GrafanaQueryService (executes queries)
├── tools_metrics_overview.go    # NEW: Overview tool (5 panels)
├── tools_metrics_aggregated.go  # NEW: Aggregated tool (drill-down)
├── tools_metrics_details.go     # NEW: Details tool (full dashboard)
├── response_formatter.go   # NEW: Format Grafana response for AI
├── client.go              # EXISTING: Add QueryDataSource method
├── graph_builder.go       # EXISTING: Used to find dashboards by hierarchy
├── grafana.go             # EXISTING: Register tools in Start()
```

### Pattern 1: Query Service Layer
**What:** Separate service that handles query execution, independent of MCP tools
**When to use:** When multiple tools need to execute queries with different filtering logic
**Example:**
```go
// Query service abstracts Grafana API details
type GrafanaQueryService struct {
    grafanaClient *GrafanaClient
    graphClient   graph.Client
    logger        *logging.Logger
}

// ExecuteDashboard executes all panels in a dashboard with variable substitution
func (s *GrafanaQueryService) ExecuteDashboard(
    ctx context.Context,
    dashboardUID string,
    timeRange TimeRange,
    scopedVars map[string]string,
    maxPanels int, // 0 = all panels
) (*DashboardQueryResult, error) {
    // 1. Fetch dashboard JSON from graph
    // 2. Filter panels (maxPanels for overview)
    // 3. Execute queries via /api/ds/query
    // 4. Format time series response
    // 5. Return partial results + errors
}
```

### Pattern 2: Progressive Disclosure in MCP Tools
**What:** Tools expose increasing detail levels based on hierarchy
**When to use:** When full data would overwhelm context window or AI processing
**Example:**
```go
// Overview: Key metrics only (first 5 panels per overview dashboard)
func (t *OverviewTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
    params := parseParams(args)

    // Find overview-level dashboards from graph
    dashboards := t.findDashboards(ctx, "overview")

    results := make([]DashboardResult, 0)
    for _, dash := range dashboards {
        // Execute only first 5 panels
        result, err := t.queryService.ExecuteDashboard(
            ctx, dash.UID, params.TimeRange, params.ScopedVars, 5,
        )
        results = append(results, result)
    }

    return &OverviewResponse{
        Dashboards: results,
        TimeRange: formatTimeRange(params.TimeRange),
    }, nil
}

// Aggregated: Service/namespace drill-down (all panels in drill-down dashboards)
func (t *AggregatedTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
    params := parseParams(args)

    // Find drill-down dashboards for service/namespace
    dashboards := t.findDashboards(ctx, "drilldown", params.Service, params.Namespace)

    results := make([]DashboardResult, 0)
    for _, dash := range dashboards {
        // Execute all panels in drill-down dashboards
        result, err := t.queryService.ExecuteDashboard(
            ctx, dash.UID, params.TimeRange, params.ScopedVars, 0,
        )
        results = append(results, result)
    }

    return &AggregatedResponse{
        Service: params.Service,
        Dashboards: results,
    }, nil
}

// Details: Full dashboard expansion (all panels in detail dashboards)
func (t *DetailsTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
    params := parseParams(args)

    // Find detail-level dashboards
    dashboards := t.findDashboards(ctx, "detail")

    results := make([]DashboardResult, 0)
    for _, dash := range dashboards {
        // Execute all panels
        result, err := t.queryService.ExecuteDashboard(
            ctx, dash.UID, params.TimeRange, params.ScopedVars, 0,
        )
        results = append(results, result)
    }

    return &DetailsResponse{
        Dashboards: results,
    }, nil
}
```

### Pattern 3: Grafana Query API Request
**What:** POST to `/api/ds/query` with datasource UID, queries, and time range
**When to use:** Every panel query execution
**Example:**
```go
// Source: https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/data_source/
type QueryRequest struct {
    Queries []Query `json:"queries"`
    From    string  `json:"from"` // ISO8601 or epoch milliseconds
    To      string  `json:"to"`
}

type Query struct {
    RefID         string                 `json:"refId"`
    Datasource    Datasource             `json:"datasource"`
    Expr          string                 `json:"expr"`          // PromQL query
    Format        string                 `json:"format"`        // "time_series"
    MaxDataPoints int                    `json:"maxDataPoints"` // 100
    IntervalMs    int                    `json:"intervalMs"`    // 1000
    ScopedVars    map[string]ScopedVar   `json:"scopedVars,omitempty"` // Variable substitution
}

type Datasource struct {
    UID string `json:"uid"`
}

type ScopedVar struct {
    Text  string `json:"text"`
    Value string `json:"value"`
}

// Execute query
func (c *GrafanaClient) QueryDataSource(ctx context.Context, req QueryRequest) (*QueryResponse, error) {
    reqBody, _ := json.Marshal(req)

    httpReq, _ := http.NewRequestWithContext(
        ctx, "POST", c.baseURL + "/api/ds/query", bytes.NewReader(reqBody),
    )
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Authorization", "Bearer " + c.token)

    resp, err := c.httpClient.Do(httpReq)
    // Handle response...
}
```

### Pattern 4: Partial Results with Errors
**What:** Return successful panel results + list of failed panels, don't fail entire request
**When to use:** Multi-panel queries where some panels may fail but others succeed
**Example:**
```go
type DashboardQueryResult struct {
    DashboardUID   string         `json:"dashboard_uid"`
    DashboardTitle string         `json:"dashboard_title"`
    Panels         []PanelResult  `json:"panels"`          // Successful panels only
    Errors         []PanelError   `json:"errors,omitempty"` // Failed panels
    TimeRange      string         `json:"time_range"`
}

type PanelResult struct {
    PanelID    int            `json:"panel_id"`
    PanelTitle string         `json:"panel_title"`
    Query      string         `json:"query,omitempty"` // PromQL, only on error
    Metrics    []MetricSeries `json:"metrics"`
}

type PanelError struct {
    PanelID    int    `json:"panel_id"`
    PanelTitle string `json:"panel_title"`
    Query      string `json:"query"`
    Error      string `json:"error"`
}

type MetricSeries struct {
    Labels    map[string]string `json:"labels"`
    Unit      string            `json:"unit,omitempty"`
    Values    []DataPoint       `json:"values"` // [timestamp, value] pairs
}

type DataPoint struct {
    Timestamp string  `json:"timestamp"` // ISO8601: "2026-01-23T10:00:00Z"
    Value     float64 `json:"value"`
}

// Example: 8 panels succeed, 2 fail
{
  "dashboard_uid": "abc123",
  "dashboard_title": "Service Overview",
  "panels": [
    {
      "panel_id": 1,
      "panel_title": "Request Rate",
      "metrics": [
        {
          "labels": {"service": "api", "cluster": "prod"},
          "unit": "reqps",
          "values": [
            {"timestamp": "2026-01-23T10:00:00Z", "value": 123.45},
            {"timestamp": "2026-01-23T10:01:00Z", "value": 126.78}
          ]
        }
      ]
    }
  ],
  "errors": [
    {
      "panel_id": 5,
      "panel_title": "Error Rate",
      "query": "rate(http_errors_total[5m])",
      "error": "Grafana API returned 403: insufficient permissions for datasource prom-2"
    }
  ],
  "time_range": "2026-01-23T09:00:00Z to 2026-01-23T10:00:00Z"
}
```

### Pattern 5: Time Range Handling
**What:** Accept absolute ISO8601 timestamps, convert to Grafana API format
**When to use:** All tool parameters
**Example:**
```go
type TimeRange struct {
    From string `json:"from"` // ISO8601: "2026-01-23T09:00:00Z"
    To   string `json:"to"`   // ISO8601: "2026-01-23T10:00:00Z"
}

func (tr TimeRange) ToGrafanaRequest() (string, string) {
    // Parse ISO8601 to time.Time
    fromTime, _ := time.Parse(time.RFC3339, tr.From)
    toTime, _ := time.Parse(time.RFC3339, tr.To)

    // Convert to epoch milliseconds for Grafana
    fromMs := fromTime.UnixMilli()
    toMs := toTime.UnixMilli()

    return fmt.Sprintf("%d", fromMs), fmt.Sprintf("%d", toMs)
}

func (tr TimeRange) Validate() error {
    fromTime, err := time.Parse(time.RFC3339, tr.From)
    if err != nil {
        return fmt.Errorf("invalid from timestamp: %w", err)
    }
    toTime, err := time.Parse(time.RFC3339, tr.To)
    if err != nil {
        return fmt.Errorf("invalid to timestamp: %w", err)
    }
    if !toTime.After(fromTime) {
        return fmt.Errorf("to must be after from")
    }
    return nil
}
```

### Anti-Patterns to Avoid
- **Local variable interpolation:** Don't replace `$cluster` in query strings locally - pass via scopedVars to Grafana API for server-side substitution
- **Synchronous multi-dashboard queries:** Parallelize dashboard queries with goroutines (e.g., 10 dashboards × 5 panels = 50 queries can run concurrently)
- **Including PromQL in successful responses:** Only include query text in errors/empty results - keeps successful responses clean
- **Relative time ranges:** Use absolute timestamps for precision and clarity (AI needs exact bounds)
- **Failing on first error:** Collect partial results, return what worked + error list

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP connection pooling | Custom connection manager | `http.Client` with tuned `Transport` | Default `MaxIdleConnsPerHost=2` causes TIME_WAIT buildup; tune to 20+ |
| PromQL parsing | Regex extraction | `prometheus/promql/parser` (existing) | Complex grammar, subqueries, binary ops - parser handles edge cases |
| Time parsing | String manipulation | `time.Parse(time.RFC3339, ...)` | Handles timezones, validates format, returns structured time.Time |
| JSON response formatting | String concatenation | `json.Marshal` / `json.MarshalIndent` | Handles escaping, nested structures, proper formatting |
| Dashboard hierarchy lookup | Manual Cypher queries | `GraphBuilder.classifyHierarchy()` (existing) | Already implements tag priority, HierarchyMap fallback |
| Variable classification | Custom pattern matching | `classifyVariable()` (existing in graph_builder.go) | Case-insensitive patterns for scoping/entity/detail |

**Key insight:** The Grafana client HTTP transport requires explicit tuning - Go's default `MaxIdleConnsPerHost=2` will cause connection churn under concurrent queries (100 goroutines × 2 connections = 98 TIME_WAIT per round). Set `MaxIdleConnsPerHost=20` and `MaxConnsPerHost=20` to match expected query concurrency.

## Common Pitfalls

### Pitfall 1: HTTP Connection Pool Exhaustion
**What goes wrong:** Default Go HTTP client has `MaxIdleConnsPerHost=2`, causing connection churn and TIME_WAIT buildup when executing concurrent queries (e.g., 50 panels across 10 dashboards)
**Why it happens:** Go's `DefaultTransport` has conservative defaults - `MaxIdleConns=100` but `MaxIdleConnsPerHost=2`, so only 2 connections reused per host
**How to avoid:** Explicitly tune `http.Transport` in GrafanaClient
**Warning signs:** Increased latency after initial queries, `netstat` shows thousands of TIME_WAIT connections, "too many open files" errors

**Fix:**
```go
// Source: https://davidbacisin.com/writing/golang-http-connection-pools-1
transport := &http.Transport{
    MaxIdleConns:        100,  // Global pool size
    MaxConnsPerHost:     20,   // Per-host connection limit
    MaxIdleConnsPerHost: 20,   // CRITICAL: default 2 causes churn
    IdleConnTimeout:     90 * time.Second,
    TLSHandshakeTimeout: 10 * time.Second,
    DialContext: (&net.Dialer{
        Timeout:   5 * time.Second,
        KeepAlive: 30 * time.Second,
    }).DialContext,
}
httpClient := &http.Client{Transport: transport, Timeout: 30 * time.Second}
```

### Pitfall 2: Grafana Response Body Not Read
**What goes wrong:** HTTP connection not returned to pool, leading to connection exhaustion and "connection refused" errors
**Why it happens:** Go's HTTP client requires reading response body to completion for connection reuse (`resp.Body` must be fully read and closed)
**How to avoid:** Always use `io.ReadAll(resp.Body)` before processing, even if you plan to discard the body
**Warning signs:** Connection pool grows unbounded, new connections opened for each request despite idle pool

**Fix:**
```go
resp, err := client.Do(req)
if err != nil {
    return nil, err
}
defer resp.Body.Close()

// CRITICAL: Always read body to completion for connection reuse
body, err := io.ReadAll(resp.Body)
if err != nil {
    return nil, err
}

if resp.StatusCode != http.StatusOK {
    return nil, fmt.Errorf("query failed (status %d): %s", resp.StatusCode, string(body))
}

// Now parse body
var result QueryResponse
json.Unmarshal(body, &result)
```

### Pitfall 3: scopedVars Not Passed to Grafana API
**What goes wrong:** Dashboard variables (like `$cluster`) not substituted in queries, resulting in errors or empty results
**Why it happens:** Assuming variable substitution happens locally or that Grafana automatically fills variables
**How to avoid:** Explicitly pass `scopedVars` in every query request with user-provided values
**Warning signs:** Queries with `$cluster` return errors like "invalid label matcher", Grafana logs show "template variable not found"

**Fix:**
```go
// Tool parameters include variable values
type ToolParams struct {
    Cluster   string `json:"cluster"`   // Required
    Region    string `json:"region"`    // Required
    Namespace string `json:"namespace,omitempty"`
}

// Convert to Grafana scopedVars format
scopedVars := map[string]ScopedVar{
    "cluster": {Text: params.Cluster, Value: params.Cluster},
    "region":  {Text: params.Region, Value: params.Region},
}
if params.Namespace != "" {
    scopedVars["namespace"] = ScopedVar{Text: params.Namespace, Value: params.Namespace}
}

// Include in query request
query := Query{
    RefID:      "A",
    Datasource: Datasource{UID: datasourceUID},
    Expr:       panel.Expr, // Contains "$cluster"
    ScopedVars: scopedVars,  // Grafana substitutes server-side
}
```

### Pitfall 4: Failing Entire Request on Single Panel Error
**What goes wrong:** One panel fails (e.g., datasource auth error), entire dashboard query returns error, AI gets no data
**Why it happens:** Not implementing partial results pattern - treating multi-panel query as atomic
**How to avoid:** Execute panels independently, collect successes and failures separately, return both
**Warning signs:** Intermittent tool failures when single datasource is down, "all or nothing" results

**Fix:**
```go
func (s *GrafanaQueryService) ExecuteDashboard(...) (*DashboardQueryResult, error) {
    result := &DashboardQueryResult{
        DashboardUID: dashboardUID,
        Panels:       make([]PanelResult, 0),
        Errors:       make([]PanelError, 0),
    }

    for _, panel := range panels {
        panelResult, err := s.executePanel(ctx, panel, timeRange, scopedVars)
        if err != nil {
            // Don't fail entire request - collect error
            result.Errors = append(result.Errors, PanelError{
                PanelID:    panel.ID,
                PanelTitle: panel.Title,
                Query:      panel.Expr,
                Error:      err.Error(),
            })
            continue
        }

        // Skip panels with no data (don't clutter response)
        if len(panelResult.Metrics) == 0 {
            continue
        }

        result.Panels = append(result.Panels, panelResult)
    }

    // Return partial results (not an error!)
    return result, nil
}
```

### Pitfall 5: Including PromQL in Every Response
**What goes wrong:** Response size bloated with redundant query text, wastes tokens in AI context window
**Why it happens:** Including query for debugging/transparency without considering token cost
**How to avoid:** Only include query text in errors or when results are empty (helps debugging failures)
**Warning signs:** Response size >> data size, AI context window fills quickly

**Fix:**
```go
type PanelResult struct {
    PanelID    int            `json:"panel_id"`
    PanelTitle string         `json:"panel_title"`
    Query      string         `json:"query,omitempty"` // Only if empty/error
    Metrics    []MetricSeries `json:"metrics"`
}

// In successful case - omit query
if len(metrics) > 0 {
    return &PanelResult{
        PanelID:    panel.ID,
        PanelTitle: panel.Title,
        Metrics:    metrics, // Query omitted - clean response
    }
}

// In empty/error case - include query for debugging
if len(metrics) == 0 {
    return &PanelResult{
        PanelID:    panel.ID,
        PanelTitle: panel.Title,
        Query:      panel.Expr, // Include for debugging
        Metrics:    []MetricSeries{},
    }
}
```

### Pitfall 6: Not Validating Time Range
**What goes wrong:** Invalid timestamps cause cryptic Grafana errors, AI gets unclear feedback
**Why it happens:** Assuming AI provides valid ISO8601 without validation
**How to avoid:** Parse and validate timestamps before making Grafana request
**Warning signs:** Grafana errors like "invalid time range", "from must be before to"

**Fix:**
```go
func (tr TimeRange) Validate() error {
    fromTime, err := time.Parse(time.RFC3339, tr.From)
    if err != nil {
        return fmt.Errorf("invalid from timestamp (expected ISO8601): %w", err)
    }
    toTime, err := time.Parse(time.RFC3339, tr.To)
    if err != nil {
        return fmt.Errorf("invalid to timestamp (expected ISO8601): %w", err)
    }
    if !toTime.After(fromTime) {
        return fmt.Errorf("to must be after from (got from=%s, to=%s)", tr.From, tr.To)
    }
    duration := toTime.Sub(fromTime)
    if duration > 7*24*time.Hour {
        return fmt.Errorf("time range too large (max 7 days, got %s)", duration)
    }
    return nil
}
```

## Code Examples

Verified patterns from official sources:

### Grafana /api/ds/query Request
```go
// Source: https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/data_source/
// Execute Prometheus query via Grafana API
func (c *GrafanaClient) QueryDataSource(
    ctx context.Context,
    datasourceUID string,
    query string,
    from, to string, // Epoch milliseconds or ISO8601
    scopedVars map[string]ScopedVar,
) (*QueryResponse, error) {
    reqBody := QueryRequest{
        Queries: []Query{
            {
                RefID:      "A",
                Datasource: Datasource{UID: datasourceUID},
                Expr:       query,
                Format:     "time_series",
                MaxDataPoints: 100,
                IntervalMs: 1000,
                ScopedVars: scopedVars,
            },
        },
        From: from,
        To:   to,
    }

    reqJSON, _ := json.Marshal(reqBody)
    req, _ := http.NewRequestWithContext(
        ctx, "POST", c.baseURL+"/api/ds/query", bytes.NewReader(reqJSON),
    )
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+c.token)

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("execute query request: %w", err)
    }
    defer resp.Body.Close()

    // CRITICAL: Read body to completion for connection reuse
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response body: %w", err)
    }

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("query failed (status %d): %s", resp.StatusCode, string(body))
    }

    var result QueryResponse
    if err := json.Unmarshal(body, &result); err != nil {
        return nil, fmt.Errorf("parse query response: %w", err)
    }

    return &result, nil
}
```

### MCP Tool Registration
```go
// Register three tools with MCP server during integration Start()
func (g *GrafanaIntegration) Start(ctx context.Context, registry integration.ToolRegistry) error {
    // Create shared query service
    queryService := NewGrafanaQueryService(g.client, g.graphClient, g.logger)

    // Register overview tool
    registry.RegisterTool(
        fmt.Sprintf("grafana_%s_metrics_overview", g.name),
        "Get overview of key metrics across all services",
        NewOverviewTool(queryService, g.graphClient, g.logger).Execute,
        map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "from": map[string]interface{}{
                    "type": "string",
                    "description": "Start time (ISO8601: 2026-01-23T10:00:00Z)",
                },
                "to": map[string]interface{}{
                    "type": "string",
                    "description": "End time (ISO8601: 2026-01-23T11:00:00Z)",
                },
                "cluster": map[string]interface{}{
                    "type": "string",
                    "description": "Cluster name (required)",
                },
                "region": map[string]interface{}{
                    "type": "string",
                    "description": "Region name (required)",
                },
            },
            "required": []string{"from", "to", "cluster", "region"},
        },
    )

    // Register aggregated tool
    registry.RegisterTool(
        fmt.Sprintf("grafana_%s_metrics_aggregated", g.name),
        "Get aggregated metrics for a specific service or namespace",
        NewAggregatedTool(queryService, g.graphClient, g.logger).Execute,
        map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "from":      /* same as overview */,
                "to":        /* same as overview */,
                "cluster":   /* same as overview */,
                "region":    /* same as overview */,
                "service": map[string]interface{}{
                    "type": "string",
                    "description": "Service name (optional, requires service OR namespace)",
                },
                "namespace": map[string]interface{}{
                    "type": "string",
                    "description": "Namespace name (optional, requires service OR namespace)",
                },
            },
            "required": []string{"from", "to", "cluster", "region"},
        },
    )

    // Register details tool
    registry.RegisterTool(
        fmt.Sprintf("grafana_%s_metrics_details", g.name),
        "Get detailed metrics with full dashboard panels",
        NewDetailsTool(queryService, g.graphClient, g.logger).Execute,
        map[string]interface{}{
            // Same parameters as overview
        },
    )

    return nil
}
```

### Finding Dashboards by Hierarchy Level
```go
// Use existing graph to find dashboards by hierarchy level
func (t *OverviewTool) findDashboards(ctx context.Context, level string) ([]Dashboard, error) {
    // Query graph for dashboards with hierarchy level
    query := `
        MATCH (d:Dashboard {hierarchy_level: $level})
        RETURN d.uid, d.title, d.tags
        ORDER BY d.title
    `

    result, err := t.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
        Query: query,
        Params: map[string]interface{}{
            "level": level,
        },
    })

    if err != nil {
        return nil, fmt.Errorf("find dashboards: %w", err)
    }

    dashboards := make([]Dashboard, 0)
    for _, record := range result.Records {
        dashboards = append(dashboards, Dashboard{
            UID:   record["d.uid"].(string),
            Title: record["d.title"].(string),
            Tags:  record["d.tags"].([]string),
        })
    }

    return dashboards, nil
}
```

### Parallel Dashboard Execution
```go
// Execute multiple dashboards concurrently for performance
func (s *GrafanaQueryService) ExecuteMultipleDashboards(
    ctx context.Context,
    dashboards []Dashboard,
    timeRange TimeRange,
    scopedVars map[string]string,
    maxPanels int,
) ([]DashboardQueryResult, error) {
    results := make([]DashboardQueryResult, len(dashboards))

    // Use errgroup for concurrent execution with context
    g, ctx := errgroup.WithContext(ctx)

    for i, dash := range dashboards {
        i, dash := i, dash // Capture loop variables
        g.Go(func() error {
            result, err := s.ExecuteDashboard(
                ctx, dash.UID, timeRange, scopedVars, maxPanels,
            )
            if err != nil {
                // Don't fail entire batch - log and continue
                s.logger.Warn("Dashboard %s query failed: %v", dash.UID, err)
                return nil // Continue with other dashboards
            }
            results[i] = *result
            return nil
        })
    }

    // Wait for all dashboards (errors logged but not propagated)
    g.Wait()

    return results, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Direct Prometheus API | Grafana /api/ds/query | Phase 18 decision | Simpler auth, variable handling delegated to Grafana |
| Static tool definitions | Progressive disclosure (overview→aggregated→details) | 2026 MCP best practice | Reduces token usage, improves tool accuracy |
| All-or-nothing results | Partial results + errors | Go error handling best practice | Resilient to datasource failures, AI gets useful data |
| String interpolation | scopedVars server-side | Grafana API design | Security, consistency, handles complex variables |

**Deprecated/outdated:**
- Relative time ranges for AI tools: Absolute timestamps (ISO8601) are clearer and more precise for AI reasoning about time
- Local variable substitution: Server-side scopedVars prevent injection and handle complex patterns

## Open Questions

Things that couldn't be fully resolved:

1. **Grafana /api/ds/query response format variations**
   - What we know: Response contains `results[refId].frames[].schema.fields` and `data.values` arrays
   - What's unclear: Exact field types for all datasource types (Prometheus vs others), handling of annotations/exemplars
   - Recommendation: Start with Prometheus time_series format, add datasource-specific handling if needed in Phase 19+

2. **Optimal maxPanels limit for overview tool**
   - What we know: Decision says 5 panels per dashboard, VictoriaLogs uses parallel queries successfully
   - What's unclear: Performance impact with 10 overview dashboards × 5 panels = 50 concurrent queries
   - Recommendation: Start with 5, add rate limiting or batching if Grafana rate limits encountered

3. **Empty results vs errors distinction**
   - What we know: Decision says "omit panels with no data"
   - What's unclear: How to distinguish "no data in time range" (valid) from "query error" (invalid)
   - Recommendation: Check Grafana response status - 200 with empty frames = no data (omit), 4xx/5xx = error (include in errors list)

4. **Variable multi-value handling**
   - What we know: scopedVars format has `text` and `value` fields
   - What's unclear: How to pass multi-select variables (e.g., cluster=["us-west", "us-east"]) via scopedVars
   - Recommendation: Start with single-value variables (matches tool parameters), defer multi-value to Phase 19+ if needed

## Sources

### Primary (HIGH confidence)
- [Grafana Data Source HTTP API](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/data_source/) - /api/ds/query endpoint documentation
- [Grafana Community: Query /api/ds/query](https://community.grafana.com/t/query-data-from-grafanas-api-api-ds-query/143474) - Request/response examples
- [Grafana Community: ScopedVars](https://community.grafana.com/t/what-are-scopedvars-and-what-are-they-used-for/38828) - Variable substitution
- [Go HTTP Connection Pooling](https://davidbacisin.com/writing/golang-http-connection-pools-1) - MaxIdleConnsPerHost pitfall
- Existing codebase: `internal/integration/grafana/client.go`, `graph_builder.go`, `dashboard_syncer.go`

### Secondary (MEDIUM confidence)
- [MCP Design Patterns: Progressive Disclosure](https://www.klavis.ai/blog/less-is-more-mcp-design-patterns-for-ai-agents) - MCP tool best practices
- [Progressive Discovery vs Static Toolsets](https://www.speakeasy.com/blog/100x-token-reduction-dynamic-toolsets) - Token reduction techniques
- [Go HTTP Connection Churn](https://dev.to/gkampitakis/http-connection-churn-in-go-34pl) - TIME_WAIT buildup
- Phase 18 CONTEXT.md - User decisions on response format, error handling, progressive disclosure

### Tertiary (LOW confidence)
- [Medium: Reverse Engineering Grafana API](https://medium.com/@mattam808/reverse-engineering-the-grafana-api-to-get-the-data-from-a-dashboard-48c2a399f797) - Practical examples (unverified with official docs)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All stdlib or existing dependencies, no new packages needed
- Architecture: HIGH - Patterns align with existing VictoriaLogs tools, Grafana API documented
- Pitfalls: HIGH - HTTP connection pooling well-documented, existing GrafanaClient proves pattern

**Research date:** 2026-01-23
**Valid until:** 2026-02-23 (30 days - stable APIs, Go stdlib patterns)

**Assumptions:**
- Grafana instance is v9.0+ (modern /api/ds/query format)
- Prometheus is primary datasource type (PromQL queries)
- Dashboard hierarchy levels already classified in graph (Phase 17)
- Variables already classified as scoping/entity/detail (Phase 17)
