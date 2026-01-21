# Phase 3: VictoriaLogs Client & Basic Pipeline - Research

**Researched:** 2026-01-21
**Domain:** VictoriaLogs HTTP API client, LogsQL query construction, Go HTTP patterns, channel-based pipeline with backpressure
**Confidence:** HIGH

## Summary

This phase implements a production-ready VictoriaLogs HTTP client with LogsQL query capabilities and a backpressure-aware pipeline for log ingestion. The research confirms that VictoriaLogs provides well-documented HTTP endpoints for querying logs with LogsQL syntax, histogram/aggregation APIs for time-series data, and JSON line-based responses that are straightforward to parse in Go.

The standard Go ecosystem provides all necessary components: `net/http` for the client with proper connection pooling, `context` for timeout control, buffered channels for backpressure handling, and `github.com/prometheus/client_golang` for metrics instrumentation (already in the project dependencies via transitive inclusion).

Key architectural decisions are validated by the research: structured parameters instead of raw LogsQL prevent injection issues and simplify query construction; bounded channels (1000-item buffer) provide natural backpressure without custom logic; batch sizes of 100 items align with common Go batching patterns; and 30-second query timeouts are standard for production HTTP clients.

**Primary recommendation:** Use VictoriaLogs `/select/logsql/query` endpoint for log retrieval, `/select/logsql/hits` for histograms, and `/select/logsql/stats_query` for aggregations. Implement structured query builders that construct LogsQL from K8s-focused parameters (namespace, pod, container, level). Handle backpressure via buffered channels with blocking semantics (no data loss). Instrument with Prometheus Gauge metrics for queue depth and Counter metrics for throughput.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http` | stdlib | HTTP client | Standard library HTTP client with proven connection pooling, timeout control, and context integration |
| `encoding/json` | stdlib | JSON parsing | Standard library JSON parser for VictoriaLogs JSON line responses |
| `context` | stdlib | Timeout/cancellation | Standard context-based timeout control for HTTP requests and graceful shutdown |
| `time` | stdlib | Time handling | RFC3339 time format parsing/formatting for ISO 8601 timestamps |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/prometheus/client_golang/prometheus` | transitive | Prometheus metrics | Pipeline instrumentation (queue depth, throughput, errors) - already in dependencies |
| `golang.org/x/sync/errgroup` | v0.18.0 | Worker coordination | Graceful shutdown coordination - already in dependencies |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `net/http.Client` | Third-party HTTP client (e.g., `resty`, `go-resty`) | Standard library is sufficient; third-party adds dependency weight without significant benefit for this use case |
| Buffered channels | `eapache/channels` batching channel | Standard buffered channels provide adequate backpressure control; specialized library unnecessary for bounded buffer pattern |
| Manual JSON parsing | Code generation (e.g., `easyjson`) | Standard `encoding/json` performance is adequate for log volumes; code generation adds build complexity |

**Installation:**
```bash
# Core dependencies already available in Go stdlib
# Prometheus client already in go.mod (transitive dependency)
# No additional dependencies required
```

## Architecture Patterns

### Recommended Project Structure
```
internal/integration/victorialogs/
├── victorialogs.go      # Integration interface implementation
├── client.go            # HTTP client wrapper for VictoriaLogs API
├── query.go             # LogsQL query builder (structured parameters)
├── pipeline.go          # Batch processing pipeline with backpressure
├── metrics.go           # Prometheus metrics registration
└── types.go             # Request/response types
```

### Pattern 1: HTTP Client with Connection Pooling
**What:** Reusable HTTP client with tuned connection pool settings for high-throughput querying
**When to use:** All VictoriaLogs HTTP API interactions
**Example:**
```go
// Source: https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
// Source: https://davidbacisin.com/writing/golang-http-connection-pools-1

func NewVictoriaLogsClient(baseURL string, queryTimeout time.Duration) *Client {
    transport := &http.Transport{
        MaxIdleConns:        100,                // Global connection pool
        MaxConnsPerHost:     20,                 // Per-host connection limit
        MaxIdleConnsPerHost: 10,                 // Reuse connections efficiently
        IdleConnTimeout:     90 * time.Second,   // Keep-alive for idle connections
        TLSHandshakeTimeout: 10 * time.Second,
        DialContext: (&net.Dialer{
            Timeout:   5 * time.Second,          // Connection establishment timeout
            KeepAlive: 30 * time.Second,
        }).DialContext,
    }

    return &Client{
        baseURL: baseURL,
        httpClient: &http.Client{
            Transport: transport,
            Timeout:   queryTimeout, // Overall request timeout (30s per requirements)
        },
    }
}
```

**Key insight:** Default `MaxIdleConnsPerHost` of 2 causes connection churn under load. Increase to 10-20 for production workloads.

### Pattern 2: Context-Based Request Timeout
**What:** Per-request timeout control using context for graceful cancellation
**When to use:** Every HTTP request to VictoriaLogs
**Example:**
```go
// Source: https://betterstack.com/community/guides/scaling-go/golang-timeouts/

func (c *Client) Query(ctx context.Context, query string, params QueryParams) (*QueryResponse, error) {
    // Context timeout already set at client level, but can be overridden per-request
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.queryURL(), body)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("execute query: %w", err)
    }
    defer resp.Body.Close()

    // CRITICAL: Always read response body to completion for connection reuse
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response: %w", err)
    }

    return parseResponse(body)
}
```

### Pattern 3: Structured LogsQL Query Builder
**What:** Type-safe query construction from structured parameters (no raw LogsQL exposure)
**When to use:** All log query operations
**Example:**
```go
// Source: https://docs.victoriametrics.com/victorialogs/logsql/

type QueryParams struct {
    Namespace  string
    Pod        string
    Container  string
    Level      string        // exact match: "error", "warn", etc.
    TimeRange  TimeRange
    Limit      int           // max 1000 per requirements
}

func BuildLogsQLQuery(params QueryParams) string {
    var filters []string

    // Field exact match using := operator
    if params.Namespace != "" {
        filters = append(filters, fmt.Sprintf(`namespace:="%s"`, params.Namespace))
    }
    if params.Pod != "" {
        filters = append(filters, fmt.Sprintf(`pod:="%s"`, params.Pod))
    }
    if params.Container != "" {
        filters = append(filters, fmt.Sprintf(`container:="%s"`, params.Container))
    }
    if params.Level != "" {
        filters = append(filters, fmt.Sprintf(`level:="%s"`, params.Level))
    }

    // Time range filter (default: last 1 hour)
    timeFilter := "_time:[1h ago, now]"
    if !params.TimeRange.IsZero() {
        timeFilter = fmt.Sprintf("_time:[%s, %s]",
            params.TimeRange.Start.Format(time.RFC3339),
            params.TimeRange.End.Format(time.RFC3339))
    }
    filters = append(filters, timeFilter)

    query := strings.Join(filters, " AND ")

    // Apply limit
    if params.Limit > 0 {
        query = fmt.Sprintf("%s | limit %d", query, params.Limit)
    }

    return query
}
```

**Key insight:** Use `:=` operator for exact field matches. Default to last 1 hour time range when unspecified.

### Pattern 4: Histogram/Aggregation Queries
**What:** Construct LogsQL stats queries for time-series aggregations
**When to use:** Overview and histogram endpoints
**Example:**
```go
// Source: https://docs.victoriametrics.com/victorialogs/querying/
// Source: https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6943

// For histogram endpoint: /select/logsql/hits
func BuildHistogramQuery(params QueryParams, bucket string) string {
    baseQuery := BuildLogsQLQuery(params)
    // hits endpoint handles time bucketing automatically with 'step' parameter
    return baseQuery
}

// For aggregation endpoint: /select/logsql/stats_query
func BuildAggregationQuery(params QueryParams, groupBy []string) string {
    baseQuery := BuildLogsQLQuery(params)

    // stats pipe for aggregation
    groupByClause := strings.Join(groupBy, ", ")
    return fmt.Sprintf("%s | stats count() by %s", baseQuery, groupByClause)
}
```

### Pattern 5: Bounded Channel Pipeline with Backpressure
**What:** Buffered channel pipeline that blocks producers when full (natural backpressure)
**When to use:** Log ingestion pipeline
**Example:**
```go
// Source: https://medium.com/capital-one-tech/buffered-channels-in-go-what-are-they-good-for-43703871828
// Source: https://medium.com/@smallnest/how-to-efficiently-batch-read-data-from-go-channels-7fe70774a8a5

type Pipeline struct {
    logChan    chan LogEntry        // Buffer size: 1000 items
    batchSize  int                  // Fixed: 100 logs per batch
    client     *Client
    metrics    *Metrics
    wg         sync.WaitGroup
    ctx        context.Context
    cancel     context.CancelFunc
}

func (p *Pipeline) Start(ctx context.Context) error {
    p.ctx, p.cancel = context.WithCancel(ctx)
    p.logChan = make(chan LogEntry, 1000) // Bounded buffer

    // Start batch processor worker
    p.wg.Add(1)
    go p.batchProcessor()

    return nil
}

func (p *Pipeline) Ingest(entry LogEntry) error {
    select {
    case p.logChan <- entry:
        p.metrics.QueueDepth.Set(float64(len(p.logChan)))
        return nil
    case <-p.ctx.Done():
        return fmt.Errorf("pipeline stopped")
    }
    // Note: Blocks when channel full (backpressure)
}

func (p *Pipeline) batchProcessor() {
    defer p.wg.Done()

    batch := make([]LogEntry, 0, p.batchSize)
    ticker := time.NewTicker(1 * time.Second) // Flush timeout
    defer ticker.Stop()

    for {
        select {
        case entry, ok := <-p.logChan:
            if !ok {
                // Channel closed, flush remaining batch
                if len(batch) > 0 {
                    p.sendBatch(batch)
                }
                return
            }

            batch = append(batch, entry)
            p.metrics.QueueDepth.Set(float64(len(p.logChan)))

            // Flush when batch full
            if len(batch) >= p.batchSize {
                p.sendBatch(batch)
                batch = batch[:0] // Clear batch
            }

        case <-ticker.C:
            // Flush partial batch on timeout
            if len(batch) > 0 {
                p.sendBatch(batch)
                batch = batch[:0]
            }

        case <-p.ctx.Done():
            // Graceful shutdown: flush remaining batch
            if len(batch) > 0 {
                p.sendBatch(batch)
            }
            return
        }
    }
}

func (p *Pipeline) sendBatch(batch []LogEntry) {
    err := p.client.IngestBatch(p.ctx, batch)
    if err != nil {
        p.metrics.ErrorsTotal.Inc()
        // Log error but don't crash
        return
    }
    p.metrics.BatchesTotal.Add(float64(len(batch)))
}

func (p *Pipeline) Stop(ctx context.Context) error {
    p.cancel()              // Signal shutdown
    close(p.logChan)        // Close channel to drain

    // Wait for worker to finish with timeout
    done := make(chan struct{})
    go func() {
        p.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return fmt.Errorf("pipeline shutdown timeout")
    }
}
```

**Key insight:** Bounded channels provide natural backpressure without custom logic. Sender blocks when buffer full, preventing memory exhaustion.

### Pattern 6: Prometheus Metrics Instrumentation
**What:** Gauge for queue depth, Counter for throughput and errors
**When to use:** All pipeline operations
**Example:**
```go
// Source: https://prometheus.io/docs/guides/go-application/
// Source: https://betterstack.com/community/guides/monitoring/prometheus-golang/

type Metrics struct {
    QueueDepth   prometheus.Gauge
    BatchesTotal prometheus.Counter
    ErrorsTotal  prometheus.Counter
}

func NewMetrics(reg prometheus.Registerer, instanceName string) *Metrics {
    m := &Metrics{
        QueueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "victorialogs_pipeline_queue_depth",
            Help: "Current number of logs in pipeline buffer",
            ConstLabels: prometheus.Labels{"instance": instanceName},
        }),
        BatchesTotal: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "victorialogs_pipeline_logs_total",
            Help: "Total number of logs sent to VictoriaLogs",
            ConstLabels: prometheus.Labels{"instance": instanceName},
        }),
        ErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "victorialogs_pipeline_errors_total",
            Help: "Total number of pipeline errors",
            ConstLabels: prometheus.Labels{"instance": instanceName},
        }),
    }

    reg.MustRegister(m.QueueDepth, m.BatchesTotal, m.ErrorsTotal)
    return m
}
```

### Anti-Patterns to Avoid
- **Creating HTTP client per request:** Causes connection exhaustion and poor performance. Reuse client across requests.
- **Not reading response body:** Prevents connection reuse even if body is closed. Always `io.ReadAll()` before closing.
- **defer in tight loops:** Defers accumulate on function stack. Use explicit cleanup in loops instead.
- **Unbounded channels:** Causes memory exhaustion under load. Always use bounded channels with explicit buffer size.
- **Ignoring context cancellation:** Pipeline continues processing after shutdown signal. Check `ctx.Done()` in all loops.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP connection pooling | Custom connection manager | `net/http.Client` with tuned `Transport` | Standard library handles connection reuse, keep-alive, TLS handshake caching, and idle connection timeout |
| Request timeout control | Manual timeout tracking | `context.WithTimeout` + `http.NewRequestWithContext` | Context propagation is built into standard library; integrates with graceful shutdown |
| Time parsing/formatting | Custom time parser | `time.Parse(time.RFC3339, ...)` | RFC3339 is ISO 8601-compliant; handles timezone offsets correctly |
| Batch accumulation | Custom batch buffer | Buffered channel + ticker | Channel-based pattern is idiomatic Go; handles backpressure naturally |
| Worker pool shutdown | Custom coordination | `sync.WaitGroup` + context cancellation | Standard library primitives prevent deadlocks and race conditions |
| Metrics registration | Custom metrics tracking | `github.com/prometheus/client_golang` | Industry-standard format; automatic scraping endpoint; type-safe metric operations |

**Key insight:** Go standard library is production-grade for HTTP client patterns. Avoid third-party HTTP libraries unless specific features required (e.g., retries, circuit breaking). For this phase, standard library is sufficient.

## Common Pitfalls

### Pitfall 1: Response Body Resource Leak
**What goes wrong:** Not reading response body to completion causes connection leaks, even if `resp.Body.Close()` is called.
**Why it happens:** Go HTTP client reuses connections only if response body is fully consumed. Closing without reading leaves connection in invalid state.
**How to avoid:** Always `io.ReadAll(resp.Body)` before closing, even for error responses.
**Warning signs:** Growing number of `TIME_WAIT` connections, "too many open files" errors, connection pool exhaustion.

**Example:**
```go
// WRONG: Causes connection leak
resp, err := client.Do(req)
if err != nil {
    return err
}
defer resp.Body.Close() // Not enough!

// RIGHT: Enables connection reuse
resp, err := client.Do(req)
if err != nil {
    return err
}
defer resp.Body.Close()
body, err := io.ReadAll(resp.Body) // Read to completion
if err != nil {
    return err
}
```

**Source:** [Solving Memory Leak Issues in Go HTTP Clients](https://medium.com/@chaewonkong/solving-memory-leak-issues-in-go-http-clients-ba0b04574a83), [Always close the response body!](https://www.j4mcs.dev/posts/golang-response-body/)

### Pitfall 2: Deadlock on Full Buffered Channel
**What goes wrong:** Producer goroutine writes to channel in same goroutine that should read from it, causing deadlock when buffer fills.
**Why it happens:** No concurrent reader exists when producer blocks on full channel.
**How to avoid:** Ensure reader goroutine starts before producer writes, or use non-blocking send with `select`.
**Warning signs:** `fatal error: all goroutines are asleep - deadlock!` panic at runtime.

**Example:**
```go
// WRONG: Deadlocks when buffer fills
ch := make(chan int, 2)
ch <- 1
ch <- 2
ch <- 3 // Blocks forever - no reader!

// RIGHT: Reader started first
ch := make(chan int, 2)
go func() {
    for v := range ch {
        process(v)
    }
}()
ch <- 1
ch <- 2
ch <- 3 // Reader consumes values
```

**Source:** [Golang Channels Simplified](https://medium.com/@raotalha302.rt/golang-channels-simplified-060547830871), [Deadlocks in Go](https://medium.com/@kstntn.lsnk/deadlocks-in-go-understanding-and-preventing-for-production-stability-6084e35050b1)

### Pitfall 3: Low MaxIdleConnsPerHost Causing Connection Churn
**What goes wrong:** Default `MaxIdleConnsPerHost` of 2 causes unnecessary connection closing and TIME_WAIT accumulation under load.
**Why it happens:** Even with `MaxIdleConns: 100`, per-host limit throttles connection reuse for single VictoriaLogs instance.
**How to avoid:** Set `MaxIdleConnsPerHost` to 10-20 for production workloads.
**Warning signs:** High CPU from TLS handshakes, thousands of TIME_WAIT connections, degraded query performance.

**Example:**
```go
// WRONG: Default settings cause churn
client := &http.Client{} // MaxIdleConnsPerHost: 2

// RIGHT: Tune for production
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10, // Increased from default 2
}
client := &http.Client{Transport: transport}
```

**Source:** [HTTP Connection Pooling in Go](https://davidbacisin.com/writing/golang-http-connection-pools-1), [Tuning the HTTP Client in Go](https://medium.com/@indrajeetmishra121/tuning-the-http-client-in-go-8c6062f851d)

### Pitfall 4: Forgetting defer cancel() for Context
**What goes wrong:** Context resources leak when `cancel()` function is not called after `context.WithTimeout()`.
**Why it happens:** Context creates timer that must be explicitly stopped to free resources.
**How to avoid:** Always `defer cancel()` immediately after creating context with timeout or cancellation.
**Warning signs:** Memory leak from accumulated timers, goroutine leak from uncancelled contexts.

**Example:**
```go
// WRONG: Resource leak
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
// Missing defer cancel()

// RIGHT: Proper cleanup
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel() // Always defer immediately
```

**Source:** [Golang Context - Cancellation, Timeout and Propagation](https://golangbot.com/context-timeout-cancellation/), [Context in Go](https://abubakardev0.medium.com/context-in-go-managing-timeouts-and-cancellations-5a7291a59d0f)

### Pitfall 5: Graceful Shutdown Without Timeout
**What goes wrong:** Shutdown waits indefinitely for in-flight requests, preventing restart/redeployment.
**Why it happens:** No timeout on graceful drain period causes hang if worker is stuck.
**How to avoid:** Always use context with timeout for shutdown operations (e.g., 30 seconds).
**Warning signs:** Kubernetes pod termination timeout, force-killed processes, restart delays.

**Example:**
```go
// WRONG: Waits forever
pipeline.Stop(context.Background())

// RIGHT: Bounded shutdown
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := pipeline.Stop(ctx); err != nil {
    // Force stop after timeout
    log.Error("Pipeline shutdown timeout, forcing stop")
}
```

**Source:** [Graceful Shutdown in Go](https://victoriametrics.com/blog/go-graceful-shutdown/), [Implementing Graceful Shutdown in Go](https://www.rudderstack.com/blog/implementing-graceful-shutdown-in-go/)

### Pitfall 6: VictoriaLogs Query Without Time Range
**What goes wrong:** Query without time range filter can attempt to scan entire log history, causing timeout or excessive resource usage.
**Why it happens:** VictoriaLogs defaults to scanning all data if no time constraint specified.
**How to avoid:** Always include `_time:[start, end]` filter. Default to last 1 hour when unspecified.
**Warning signs:** Query timeouts, high VictoriaLogs CPU usage, slow response times.

**Example:**
```go
// WRONG: No time range
query := `namespace:="prod" AND level:="error"`

// RIGHT: Always include time range
query := `namespace:="prod" AND level:="error" AND _time:[1h ago, now]`
```

**Source:** [VictoriaLogs: LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/), [VictoriaLogs: Querying](https://docs.victoriametrics.com/victorialogs/querying/)

## Code Examples

Verified patterns from official sources:

### VictoriaLogs Query Request
```go
// Source: https://docs.victoriametrics.com/victorialogs/querying/

func (c *Client) QueryLogs(ctx context.Context, params QueryParams) (*QueryResponse, error) {
    query := BuildLogsQLQuery(params)

    // Construct request
    form := url.Values{}
    form.Set("query", query)
    if params.Limit > 0 {
        form.Set("limit", strconv.Itoa(params.Limit))
    }

    reqURL := fmt.Sprintf("%s/select/logsql/query", c.baseURL)
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL,
        strings.NewReader(form.Encode()))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    // Execute request
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("execute query: %w", err)
    }
    defer resp.Body.Close()

    // Read response body (critical for connection reuse)
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response: %w", err)
    }

    // Check status code
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("query failed (status %d): %s",
            resp.StatusCode, string(body))
    }

    // Parse JSON line response
    return parseJSONLineResponse(body, params.Limit)
}
```

### VictoriaLogs Histogram Request
```go
// Source: https://docs.victoriametrics.com/victorialogs/querying/

func (c *Client) QueryHistogram(ctx context.Context, params QueryParams, step string) (*HistogramResponse, error) {
    query := BuildLogsQLQuery(params)

    form := url.Values{}
    form.Set("query", query)
    form.Set("start", params.TimeRange.Start.Format(time.RFC3339))
    form.Set("end", params.TimeRange.End.Format(time.RFC3339))
    form.Set("step", step) // e.g., "5m", "1h"

    reqURL := fmt.Sprintf("%s/select/logsql/hits", c.baseURL)
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL,
        strings.NewReader(form.Encode()))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("execute histogram query: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response: %w", err)
    }

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("histogram query failed (status %d): %s",
            resp.StatusCode, string(body))
    }

    return parseHistogramResponse(body)
}
```

### VictoriaLogs Aggregation Request
```go
// Source: https://docs.victoriametrics.com/victorialogs/querying/

func (c *Client) QueryAggregation(ctx context.Context, params QueryParams, groupBy []string) (*AggregationResponse, error) {
    query := BuildAggregationQuery(params, groupBy)

    form := url.Values{}
    form.Set("query", query)
    form.Set("time", params.TimeRange.End.Format(time.RFC3339))

    reqURL := fmt.Sprintf("%s/select/logsql/stats_query", c.baseURL)
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL,
        strings.NewReader(form.Encode()))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("execute aggregation query: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response: %w", err)
    }

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("aggregation query failed (status %d): %s",
            resp.StatusCode, string(body))
    }

    return parseAggregationResponse(body)
}
```

### Parsing VictoriaLogs JSON Line Response
```go
// Source: https://docs.victoriametrics.com/victorialogs/querying/

type LogEntry struct {
    Message   string    `json:"_msg"`
    Stream    string    `json:"_stream"`
    Time      time.Time `json:"_time"`
    Namespace string    `json:"namespace,omitempty"`
    Pod       string    `json:"pod,omitempty"`
    Container string    `json:"container,omitempty"`
    Level     string    `json:"level,omitempty"`
}

func parseJSONLineResponse(body []byte, limit int) (*QueryResponse, error) {
    var entries []LogEntry
    scanner := bufio.NewScanner(bytes.NewReader(body))

    for scanner.Scan() {
        var entry LogEntry
        if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
            return nil, fmt.Errorf("parse log entry: %w", err)
        }
        entries = append(entries, entry)
    }

    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("scan response: %w", err)
    }

    hasMore := limit > 0 && len(entries) >= limit

    return &QueryResponse{
        Logs:    entries,
        Count:   len(entries),
        HasMore: hasMore,
    }, nil
}
```

### Time Format Handling
```go
// Source: https://golang.cafe/blog/how-to-parse-rfc-3339-iso-8601-date-time-string-in-go-golang

func ParseISO8601(s string) (time.Time, error) {
    // RFC3339 is ISO 8601-compliant
    return time.Parse(time.RFC3339, s)
}

func FormatISO8601(t time.Time) string {
    // Format as ISO 8601: "2026-01-21T10:30:00Z"
    return t.UTC().Format(time.RFC3339)
}

// Default time range: last 1 hour
func DefaultTimeRange() TimeRange {
    now := time.Now()
    return TimeRange{
        Start: now.Add(-1 * time.Hour),
        End:   now,
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| VictoriaLogs `/select/logsql/query` only | Added `/select/logsql/hits` and `/select/logsql/stats_query_range` endpoints | Sept 2024 | Enables histogram and time-series aggregation without custom post-processing |
| Drain algorithm (external library) | Built-in template mining (future phase) | Phase 4 (pending) | This phase focuses on basic querying; template mining deferred to Phase 4 |
| `sync.WaitGroup.Wait()` blocking | `sync.WaitGroup.Go()` method added | Go 1.24 (Jan 2026) | Simplified worker spawning pattern, but not critical for this phase |

**Deprecated/outdated:**
- None - VictoriaLogs HTTP API is stable and backward-compatible. LogsQL syntax is actively maintained.

## Open Questions

Things that couldn't be fully resolved:

1. **VictoriaLogs error response format**
   - What we know: HTTP 400 status codes used for query errors; error message in response body
   - What's unclear: Structured error response schema (JSON vs plain text); complete list of HTTP status codes
   - Recommendation: Parse error response body as plain text initially; refine based on actual VictoriaLogs error responses during implementation

2. **stats_query_range API availability**
   - What we know: GitHub issues from Sept 2024 propose `/select/logsql/stats_query_range` endpoint
   - What's unclear: Whether this endpoint is released in current VictoriaLogs versions
   - Recommendation: Use `/select/logsql/hits` for histograms initially; verify `stats_query_range` availability in target VictoriaLogs version

3. **Optimal batch size for ingestion**
   - What we know: 100-item batches are common in Go batching patterns
   - What's unclear: VictoriaLogs ingestion endpoint performance characteristics; whether larger batches improve throughput
   - Recommendation: Start with 100-item batches per requirements; expose as configurable parameter for tuning if needed

## Sources

### Primary (HIGH confidence)
- [VictoriaLogs: Querying](https://docs.victoriametrics.com/victorialogs/querying/) - HTTP API endpoints, query parameters, response format
- [VictoriaLogs: LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/) - Query language syntax, field filtering, time ranges
- [VictoriaLogs: LogsQL Examples](https://docs.victoriametrics.com/victorialogs/logsql-examples/) - Practical query examples
- [The complete guide to Go net/http timeouts](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/) - Production HTTP client configuration
- [HTTP Connection Pooling in Go](https://davidbacisin.com/writing/golang-http-connection-pools-1) - Connection pool tuning
- [Prometheus Go client documentation](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus) - Metrics instrumentation
- [Instrumenting a Go application for Prometheus](https://prometheus.io/docs/guides/go-application/) - Official Prometheus guide

### Secondary (MEDIUM confidence)
- [How to Efficiently Batch Read Data from Go Channels](https://medium.com/@smallnest/how-to-efficiently-batch-read-data-from-go-channels-7fe70774a8a5) - Batching patterns verified with multiple sources
- [Buffered Channels In Go — What Are They Good For?](https://medium.com/capital-one-tech/buffered-channels-in-go-what-are-they-good-for-43703871828) - Backpressure pattern verified with Capital One Tech
- [Graceful Shutdown in Go](https://victoriametrics.com/blog/go-graceful-shutdown/) - VictoriaMetrics team's own shutdown patterns
- [Solving Memory Leak Issues in Go HTTP Clients](https://medium.com/@chaewonkong/solving-memory-leak-issues-in-go-http-clients-ba0b04574a83) - Response body leak verified with multiple sources
- [How to Parse RFC-3339 / ISO-8601 date-time string in Go](https://golang.cafe/blog/how-to-parse-rfc-3339-iso-8601-date-time-string-in-go-golang) - Time format handling

### Tertiary (LOW confidence - flagged for validation)
- [VictoriaLogs stats_query_range GitHub issue #6943](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6943) - Feature proposal from Sept 2024; unclear if released

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Standard library HTTP client patterns are well-documented and battle-tested
- Architecture: HIGH - VictoriaLogs API endpoints verified with official documentation; Go patterns verified with multiple authoritative sources
- Pitfalls: HIGH - Common mistakes documented in multiple sources with clear examples and solutions

**Research date:** 2026-01-21
**Valid until:** 2026-02-21 (30 days - stable ecosystem, slow-moving APIs)

**Key validation notes:**
- VictoriaLogs HTTP API is stable and documented; LogsQL syntax is actively maintained
- Go standard library HTTP patterns are production-grade and sufficient for this phase
- Prometheus client library already available via transitive dependencies
- All architectural decisions from CONTEXT.md are validated by research findings
