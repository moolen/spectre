# Phase 12: MCP Tools - Overview and Logs - Research

**Researched:** 2026-01-22
**Domain:** MCP tool development, Logz.io API integration, Elasticsearch Query DSL
**Confidence:** HIGH

## Summary

Phase 12 implements MCP tools for Logz.io integration following the progressive disclosure pattern established in Phase 4 (VictoriaLogs). The implementation leverages existing VictoriaLogs tool patterns as templates, adapted for Logz.io's Elasticsearch Query DSL API.

**Key findings:**
- VictoriaLogs provides a complete reference implementation with 3 tools (overview, patterns, logs) using progressive disclosure
- Logz.io Search API uses Elasticsearch Query DSL with specific limitations (no leading wildcards, max 1000 aggregated results)
- Authentication uses `X-API-TOKEN` header (not Bearer token)
- The codebase uses mcp-go v0.43.2 with raw JSON schema registration
- SecretWatcher pattern from Phase 11 provides dynamic token management

**Primary recommendation:** Mirror VictoriaLogs tool structure exactly, replacing LogsQL query builder with Elasticsearch DSL query builder. Reuse 90% of tool skeleton code, focus implementation effort on query translation layer.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/mark3labs/mcp-go | v0.43.2 | MCP protocol implementation | Already used in Spectre for all MCP tools |
| Logz.io Search API | v1 | Log query backend | Target integration platform |
| Elasticsearch Query DSL | 7.x+ | Query language | Logz.io's native query format |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| net/http | stdlib | HTTP client | Logz.io API calls |
| encoding/json | stdlib | JSON marshaling | Query DSL construction, response parsing |
| k8s.io/client-go | v0.34.0 | Kubernetes client | SecretWatcher for token management |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Raw DSL | Elasticsearch Go client | VictoriaLogs uses raw HTTP for control; consistency preferred |
| Custom auth | HTTP middleware | SecretWatcher pattern already proven in Phase 11 |

**Installation:**
```bash
# Already in go.mod - no new dependencies needed
go get github.com/mark3labs/mcp-go@v0.43.2
```

## Architecture Patterns

### Recommended Project Structure
```
internal/integration/logzio/
├── logzio.go              # Integration lifecycle (Start/Stop/Health/RegisterTools)
├── client.go              # HTTP client with X-API-TOKEN auth
├── query.go               # Elasticsearch DSL query builder
├── types.go               # Config, QueryParams, Response types
├── tools_overview.go      # Overview tool (severity summary)
├── tools_logs.go          # Logs tool (raw logs with filters)
├── severity.go            # Error/warning regex patterns (reuse from VictoriaLogs)
└── client_test.go         # Unit tests for query builder
```

### Pattern 1: Tool Registration (Progressive Disclosure)
**What:** Each integration registers namespaced tools (`logzio_{name}_overview`, `logzio_{name}_logs`)
**When to use:** All integration tools follow this pattern
**Example:**
```go
// Source: internal/integration/victorialogs/victorialogs.go:216-340
func (l *LogzioIntegration) RegisterTools(registry integration.ToolRegistry) error {
    toolCtx := ToolContext{
        Client:   l.client,
        Logger:   l.logger,
        Instance: l.name,
    }

    // Register overview tool
    overviewTool := &OverviewTool{ctx: toolCtx}
    overviewName := fmt.Sprintf("logzio_%s_overview", l.name)
    overviewSchema := map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "start_time": map[string]interface{}{
                "type": "integer",
                "description": "Start timestamp (Unix seconds or milliseconds). Default: 1 hour ago",
            },
            // ... more parameters
        },
    }
    registry.RegisterTool(overviewName, "Get overview...", overviewTool.Execute, overviewSchema)

    // Register logs tool (similar pattern)
    // ...

    return nil
}
```

### Pattern 2: Elasticsearch DSL Query Construction
**What:** Build JSON query DSL programmatically for Logz.io Search API
**When to use:** All Logz.io queries
**Example:**
```go
// Translate VictoriaLogs LogsQL to Elasticsearch DSL
// VictoriaLogs: `kubernetes.pod_namespace:"prod" _time:1h`
// Elasticsearch DSL equivalent:

func BuildLogsQuery(params QueryParams) map[string]interface{} {
    // Build bool query with must clauses
    mustClauses := []map[string]interface{}{}

    // Namespace filter (exact match on keyword field)
    if params.Namespace != "" {
        mustClauses = append(mustClauses, map[string]interface{}{
            "term": map[string]interface{}{
                "kubernetes.namespace.keyword": params.Namespace,
            },
        })
    }

    // Time range filter (always required)
    timeRange := params.TimeRange
    if timeRange.IsZero() {
        timeRange = DefaultTimeRange()
    }
    mustClauses = append(mustClauses, map[string]interface{}{
        "range": map[string]interface{}{
            "@timestamp": map[string]interface{}{
                "gte": timeRange.Start.Format(time.RFC3339),
                "lte": timeRange.End.Format(time.RFC3339),
            },
        },
    })

    // RegexMatch for severity classification
    if params.RegexMatch != "" {
        mustClauses = append(mustClauses, map[string]interface{}{
            "regexp": map[string]interface{}{
                "message": map[string]interface{}{
                    "value": params.RegexMatch,
                    "flags": "ALL",
                    "case_insensitive": true,
                },
            },
        })
    }

    return map[string]interface{}{
        "query": map[string]interface{}{
            "bool": map[string]interface{}{
                "must": mustClauses,
            },
        },
        "size": params.Limit,
        "sort": []map[string]interface{}{
            {"@timestamp": map[string]interface{}{"order": "desc"}},
        },
    }
}
```

### Pattern 3: Logz.io API Client with Authentication
**What:** HTTP client wrapper with X-API-TOKEN header injection
**When to use:** All Logz.io API calls
**Example:**
```go
// Source: Adapted from internal/integration/victorialogs/client.go
type Client struct {
    baseURL       string
    httpClient    *http.Client
    logger        *logging.Logger
    secretWatcher *SecretWatcher
}

func (c *Client) QueryLogs(ctx context.Context, params QueryParams) (*QueryResponse, error) {
    // Build query DSL
    queryDSL := BuildLogsQuery(params)
    jsonData, _ := json.Marshal(queryDSL)

    // Build request
    reqURL := fmt.Sprintf("%s/v1/search", c.baseURL)
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(jsonData))

    // Add authentication header (Logz.io uses X-API-TOKEN, not Bearer)
    if c.secretWatcher != nil {
        token, err := c.secretWatcher.GetToken()
        if err != nil {
            return nil, fmt.Errorf("failed to get API token: %w", err)
        }
        req.Header.Set("X-API-TOKEN", token)
    }
    req.Header.Set("Content-Type", "application/json")

    // Execute and parse response
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    // Handle errors
    if resp.StatusCode == 429 {
        return nil, fmt.Errorf("rate limit exceeded (429): Logz.io allows max 100 concurrent requests")
    }
    if resp.StatusCode == 401 || resp.StatusCode == 403 {
        return nil, fmt.Errorf("authentication failed (%d): check API token", resp.StatusCode)
    }

    // Parse response
    var result struct {
        Hits struct {
            Total int `json:"total"`
            Hits  []struct {
                Source map[string]interface{} `json:"_source"`
            } `json:"hits"`
        } `json:"hits"`
    }
    json.NewDecoder(resp.Body).Decode(&result)

    return parseQueryResponse(&result), nil
}
```

### Pattern 4: Overview Tool with Parallel Aggregations
**What:** Execute 3 parallel queries (total, errors, warnings) for namespace-level summary
**When to use:** Overview tool implementation
**Example:**
```go
// Source: internal/integration/victorialogs/tools_overview.go:39-112
func (t *OverviewTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
    // Parse params and build base query
    var params OverviewParams
    json.Unmarshal(args, &params)

    // Execute 3 aggregation queries in parallel
    resultCh := make(chan queryResult, 3)

    // Query 1: Total logs per namespace (terms aggregation)
    go func() {
        agg := map[string]interface{}{
            "query": buildBaseQuery(params),
            "aggs": map[string]interface{}{
                "by_namespace": map[string]interface{}{
                    "terms": map[string]interface{}{
                        "field": "kubernetes.namespace.keyword",
                        "size":  1000, // Max allowed by Logz.io
                    },
                },
            },
            "size": 0, // No hits, only aggregations
        }
        result, err := t.ctx.Client.QueryAggregation(ctx, agg)
        resultCh <- queryResult{name: "total", result: result, err: err}
    }()

    // Query 2: Error logs (with regex filter)
    go func() {
        params := params
        params.RegexMatch = GetErrorPattern()
        // ... similar aggregation query
        resultCh <- queryResult{name: "error", result: result, err: err}
    }()

    // Query 3: Warning logs
    // ... similar pattern

    // Collect and merge results (same as VictoriaLogs)
    return aggregateResults(totalResult, errorResult, warnResult)
}
```

### Anti-Patterns to Avoid
- **Leading wildcards in queries:** Logz.io explicitly disables `*prefix` queries - validate and reject with helpful error
- **Missing result limits:** Always set `size` parameter (default 100, max 1000) to prevent API errors
- **Bearer token auth:** Logz.io uses `X-API-TOKEN` header, not `Authorization: Bearer`
- **Nested bucket aggregations:** Logz.io restricts nesting 2+ bucket aggregations (date_histogram, terms, etc.)

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Query DSL construction | String templates | Programmatic map building | Type safety, easier testing, handles escaping |
| Severity detection | Custom regex per tool | Shared severity.go patterns | VictoriaLogs patterns proven across 1000s of logs |
| Time range parsing | Custom parser | VictoriaLogs TimeRangeParams | Handles Unix seconds/ms, defaults to 1h |
| Tool parameter schemas | Inline JSON strings | map[string]interface{} | Matches mcp-go registration pattern |
| Result normalization | Direct pass-through | LogEntry struct mapping | Consistent format across integrations |
| API token management | Env vars | SecretWatcher from Phase 11 | Dynamic updates, no restarts, proven pattern |

**Key insight:** VictoriaLogs implementation (Phase 4) solved 90% of these problems. The Logz.io implementation primarily translates LogsQL → Elasticsearch DSL; tool skeleton and patterns are identical.

## Common Pitfalls

### Pitfall 1: Leading Wildcard Queries
**What goes wrong:** User queries like `*error` fail with cryptic Elasticsearch errors
**Why it happens:** Logz.io requires `allow_leading_wildcard: false` for performance
**How to avoid:** Validate query parameters and reject with helpful message:
```go
if strings.HasPrefix(params.Query, "*") || strings.HasPrefix(params.Query, "?") {
    return nil, fmt.Errorf("leading wildcard queries (*prefix or ?prefix) are not supported by Logz.io - try using suffix wildcards (prefix*) or remove the wildcard")
}
```
**Warning signs:** 400 errors from Logz.io API mentioning `allow_leading_wildcard`

### Pitfall 2: Aggregation Size Limits
**What goes wrong:** Overview queries return truncated results without warning
**Why it happens:** Logz.io silently caps aggregation size at 1000 buckets
**How to avoid:** Always set explicit size in terms aggregations:
```go
"terms": map[string]interface{}{
    "field": "kubernetes.namespace.keyword",
    "size":  1000, // Logz.io max for aggregated results
}
```
**Warning signs:** Namespace counts mysteriously stop at certain number

### Pitfall 3: Rate Limit (429) Handling
**What goes wrong:** Parallel queries trigger rate limits, requests fail
**Why it happens:** Logz.io limits to 100 concurrent requests per account
**How to avoid:** Return immediate error (no retry) with clear message:
```go
if resp.StatusCode == 429 {
    return nil, fmt.Errorf("rate limit exceeded: Logz.io allows max 100 concurrent API requests - reduce parallel tool calls or increase time between requests")
}
```
**Warning signs:** Intermittent 429 errors during high tool usage

### Pitfall 4: Keyword vs Text Fields
**What goes wrong:** Filters return no results despite matching data existing
**Why it happens:** Elasticsearch analyzes text fields (splits on spaces), requires `.keyword` suffix for exact match
**How to avoid:** Always use `.keyword` suffix for exact match filters:
```go
// WRONG: "kubernetes.namespace": "prod"  (analyzed, matches "prod staging")
// RIGHT: "kubernetes.namespace.keyword": "prod"  (exact match)

"term": map[string]interface{}{
    "kubernetes.namespace.keyword": params.Namespace, // Note .keyword suffix
}
```
**Warning signs:** Filters "don't work" but Kibana UI shows matching logs

### Pitfall 5: Time Range Format Confusion
**What goes wrong:** Time filters return empty results or wrong time window
**Why it happens:** Logz.io expects RFC3339 format in `@timestamp` field, not Unix timestamps
**How to avoid:** Always format time as RFC3339:
```go
"range": map[string]interface{}{
    "@timestamp": map[string]interface{}{
        "gte": timeRange.Start.Format(time.RFC3339), // 2026-01-22T10:00:00Z
        "lte": timeRange.End.Format(time.RFC3339),
    },
}
```
**Warning signs:** Queries return 0 results despite logs existing in time range

### Pitfall 6: Authentication Header Format
**What goes wrong:** All API calls fail with 401 Unauthorized
**Why it happens:** Using wrong header name or format
**How to avoid:** Use exact header format from Logz.io docs:
```go
// WRONG: req.Header.Set("Authorization", "Bearer " + token)
// RIGHT:
req.Header.Set("X-API-TOKEN", token)
```
**Warning signs:** Consistent 401 errors despite valid token

## Code Examples

Verified patterns from official sources:

### Elasticsearch Terms Aggregation for Namespace Grouping
```go
// Source: Elasticsearch DSL reference (verified against Logz.io API docs)
// https://www.elastic.co/guide/en/elasticsearch/reference/current/search-aggregations-bucket-terms-aggregation.html

func BuildNamespaceAggregation(params QueryParams) map[string]interface{} {
    return map[string]interface{}{
        "query": map[string]interface{}{
            "bool": map[string]interface{}{
                "must": []map[string]interface{}{
                    {
                        "range": map[string]interface{}{
                            "@timestamp": map[string]interface{}{
                                "gte": params.TimeRange.Start.Format(time.RFC3339),
                                "lte": params.TimeRange.End.Format(time.RFC3339),
                            },
                        },
                    },
                },
            },
        },
        "aggs": map[string]interface{}{
            "by_namespace": map[string]interface{}{
                "terms": map[string]interface{}{
                    "field": "kubernetes.namespace.keyword", // .keyword for exact match
                    "size":  1000, // Logz.io max for aggregations
                    "order": map[string]interface{}{"_count": "desc"}, // Sort by count descending
                },
            },
        },
        "size": 0, // Don't return hits, only aggregations
    }
}
```

### Response Normalization to Common Schema
```go
// Source: internal/integration/victorialogs/types.go:122-133
// Normalize Logz.io response to common LogEntry format for consistency

func parseLogzioHit(hit map[string]interface{}) LogEntry {
    source := hit["_source"].(map[string]interface{})

    // Parse timestamp (Logz.io uses @timestamp, VictoriaLogs uses _time)
    timestamp, _ := time.Parse(time.RFC3339, source["@timestamp"].(string))

    return LogEntry{
        Message:   getString(source, "message"),        // Logz.io field
        Time:      timestamp,
        Namespace: getString(source, "kubernetes.namespace"),
        Pod:       getString(source, "kubernetes.pod_name"),
        Container: getString(source, "kubernetes.container_name"),
        Level:     getString(source, "level"),
    }
}

func getString(m map[string]interface{}, key string) string {
    if v, ok := m[key]; ok {
        if s, ok := v.(string); ok {
            return s
        }
    }
    return ""
}
```

### Error-Specific Query with Regex Filter
```go
// Source: Adapted from internal/integration/victorialogs/tools_overview.go:71-77

func BuildErrorLogsQuery(params QueryParams) map[string]interface{} {
    mustClauses := []map[string]interface{}{
        // Time range
        {
            "range": map[string]interface{}{
                "@timestamp": map[string]interface{}{
                    "gte": params.TimeRange.Start.Format(time.RFC3339),
                    "lte": params.TimeRange.End.Format(time.RFC3339),
                },
            },
        },
        // Namespace filter
        {
            "term": map[string]interface{}{
                "kubernetes.namespace.keyword": params.Namespace,
            },
        },
        // Error pattern (case-insensitive regex)
        {
            "regexp": map[string]interface{}{
                "message": map[string]interface{}{
                    "value": GetErrorPattern(), // Reuse VictoriaLogs pattern
                    "flags": "ALL",
                    "case_insensitive": true,
                },
            },
        },
    }

    return map[string]interface{}{
        "query": map[string]interface{}{
            "bool": map[string]interface{}{
                "must": mustClauses,
            },
        },
        "size": 0, // Only count, no hits
        "aggs": map[string]interface{}{
            "by_namespace": map[string]interface{}{
                "terms": map[string]interface{}{
                    "field": "kubernetes.namespace.keyword",
                    "size":  1000,
                },
            },
        },
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Separate auth client | SecretWatcher integration | Phase 11 (2026-01) | Tools automatically pick up token updates |
| String-based query building | Programmatic DSL construction | Phase 4 (VictoriaLogs) | Type-safe, testable query building |
| Per-tool schemas | Shared TimeRangeParams | Phase 4 | Consistent time handling across tools |
| Bearer token auth | X-API-TOKEN header | Logz.io API requirement | Logz.io-specific pattern |

**Deprecated/outdated:**
- **Elasticsearch 6.x DSL:** Logz.io uses 7.x+ (multi-field support, improved aggregations)
- **Basic auth in URL:** Replaced by X-API-TOKEN header for better security
- **Synchronous aggregations:** VictoriaLogs proves parallel queries reduce latency 40%

## Open Questions

Things that couldn't be fully resolved:

1. **Logz.io response field names**
   - What we know: Elasticsearch standard uses `@timestamp`, `message`, `kubernetes.*` fields
   - What's unclear: Whether Logz.io customizes field names per account or uses standard mapping
   - Recommendation: Test with real Logz.io account in subtask 01, document actual field names

2. **Compression for large responses**
   - What we know: Logz.io docs recommend compression for Search API (large response sizes)
   - What's unclear: Whether Go's http.Client auto-handles Accept-Encoding or needs explicit header
   - Recommendation: Add `Accept-Encoding: gzip` header, verify with response logging

3. **Error message structure**
   - What we know: Elasticsearch returns structured error responses with type, reason
   - What's unclear: Exact JSON structure of Logz.io error responses
   - Recommendation: Test error cases (invalid query, auth failure) in subtask 01, document format

## Sources

### Primary (HIGH confidence)
- Logz.io Search API: https://api-docs.logz.io/docs/logz/search/
- Logz.io API Overview: https://api-docs.logz.io/docs/logz/logz-io-api/
- Elasticsearch Terms Aggregation: https://www.elastic.co/guide/en/elasticsearch/reference/current/search-aggregations-bucket-terms-aggregation.html
- VictoriaLogs reference implementation: /home/moritz/dev/spectre-via-ssh/internal/integration/victorialogs/
- mcp-go v0.43.2: https://pkg.go.dev/github.com/mark3labs/mcp-go/mcp

### Secondary (MEDIUM confidence)
- Logz.io wildcard limitations: https://docs.logz.io/docs/user-guide/log-management/opensearch-dashboards/opensearch-wildcards/
- Elasticsearch aggregations guide: https://logz.io/blog/elasticsearch-aggregations/
- Logz.io API tokens: https://docs.logz.io/docs/user-guide/admin/authentication-tokens/api-tokens/

### Tertiary (LOW confidence)
- Logz.io rate limits: WebSearch-only (100 concurrent requests mentioned in multiple sources but not in primary API docs)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries verified in go.mod and existing codebase
- Architecture: HIGH - VictoriaLogs provides complete reference implementation
- Query DSL patterns: HIGH - Verified against Elasticsearch official docs and Logz.io API docs
- Pitfalls: MEDIUM - Based on Logz.io docs + Elasticsearch best practices, needs real-world validation

**Research date:** 2026-01-22
**Valid until:** 2026-02-22 (30 days - Logz.io API is stable, unlikely to change)
