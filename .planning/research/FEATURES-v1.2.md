# Features Research: Logz.io Integration

**Domain:** Log Management & Observability Platform (Kubernetes-focused)
**Researched:** 2026-01-22
**Target:** v1.2 milestone — Add Logz.io as second log backend

## Executive Summary

Logz.io provides a managed ELK (Elasticsearch-based) platform with **native log patterns** (clustering algorithms built-in), superior to VictoriaLogs which requires custom Drain algorithm implementation. For Spectre's progressive disclosure UX (overview → patterns → logs), Logz.io offers:

1. **Overview:** Terms aggregation for namespace grouping + query_string filters for severity
2. **Patterns:** Built-in Patterns Engine (automatically clusters logs, no mining needed)
3. **Logs:** Standard search with scroll API for >1000 results

**Key differentiator:** Logz.io patterns are **pre-computed and indexed** during ingestion, eliminating the need for pattern mining and TemplateStore infrastructure.

**Key constraint:** Search API requires **Enterprise or Pro plan** (not Community). Rate limited to 100 concurrent requests per account.

---

## Table Stakes (Parity with VictoriaLogs)

These features are **required** to match the existing VictoriaLogs MCP tool capabilities.

### 1. Overview Tool — Namespace-Level Severity Summary

**VictoriaLogs approach:** 3 parallel aggregation queries (total, errors, warnings) grouped by namespace.

**Logz.io equivalent:**
- **API:** `/v1/search` with `terms` aggregation on `kubernetes.namespace` field
- **Severity filtering:** Use `query_string` with boolean operators:
  - Errors: `(level:error OR level:fatal OR _msg:*ERROR* OR _msg:*FATAL*)`
  - Warnings: `(level:warn OR level:warning OR _msg:*WARN*)`
- **Parallel execution:** Run 3 concurrent Search API calls like VictoriaLogs
- **Result format:** Return `NamespaceSeverity` array sorted by total desc

**Complexity:** Medium
- Elasticsearch DSL aggregations are more complex than LogsQL
- Must handle nested JSON response structure
- Field mapping: `kubernetes.pod_namespace` vs VictoriaLogs `kubernetes.namespace`

**Sources:**
- [Logz.io Search API](https://api-docs.logz.io/docs/logz/search/)
- [Elasticsearch Aggregations Guide](https://logz.io/blog/elasticsearch-aggregations/)

### 2. Patterns Tool — Log Template Clustering

**VictoriaLogs approach:** Fetch raw logs, mine patterns with Drain algorithm in TemplateStore, detect novelty.

**Logz.io equivalent:**
- **Built-in feature:** Logz.io Patterns Engine pre-clusters logs during ingestion
- **No mining needed:** Patterns are automatically indexed and queryable
- **Access method:**
  - Option A: Use OpenSearch Dashboards Patterns API (if exposed)
  - Option B: Fetch raw logs and filter by pattern field (if exposed in documents)
  - Option C: Search API with aggregation on pattern metadata fields

**CRITICAL LIMITATION:** Patterns Engine is **UI-only** feature. API access unclear from documentation.

**Implementation options:**

| Option | Approach | Complexity | Confidence |
|--------|----------|------------|------------|
| A | Use dedicated Patterns API if exists | Low | **LOW** — Not documented |
| B | Aggregate on `logzio.pattern` field | Medium | **LOW** — Field name unverified |
| C | Fallback to VictoriaLogs-style mining | High | **HIGH** — Known working approach |

**Recommendation:** Start with Search API exploration to check if pattern metadata exists in log documents. If not, implement **fallback pattern mining** using existing TemplateStore code (reusable across backends).

**Complexity:** High (uncertainty about API exposure)

**Sources:**
- [Understanding Log Patterns](https://docs.logz.io/docs/user-guide/log-management/opensearch-dashboards/opensearch-patterns/)
- [Announcing Log Patterns](https://logz.io/blog/announcing-log-patterns-saving-time-and-money-for-engineers/)

### 3. Logs Tool — Raw Log Retrieval with Filters

**VictoriaLogs approach:** Query with namespace/pod/container/level filters, limit 500.

**Logz.io equivalent:**
- **API:** `/v1/search` with `query_string` filters
- **Filters:**
  - Namespace: `kubernetes.namespace:"value"`
  - Pod: `kubernetes.pod_name:"value"` (note: pod_name not pod)
  - Container: `kubernetes.container_name:"value"`
  - Level: `level:"error"` OR `_msg:~"pattern"`
- **Result limits:**
  - Non-aggregated: max 10,000 results per request
  - Paginated: default 10, max 1,000 per page
  - For >1,000 results: Use Scroll API
- **Sort:** Chronological (newest first) via `sort` parameter

**Complexity:** Medium
- Query_string syntax more flexible than LogsQL
- Must handle pagination/scroll for large result sets
- Field name mapping required

**Sources:**
- [Logz.io Search API](https://api-docs.logz.io/docs/logz/search/)
- [Kubernetes Log Fields](https://docs.logz.io/docs/shipping/containers/kubernetes/)

### 4. Time Range Filtering

**VictoriaLogs approach:** `_time:duration` syntax (e.g., `_time:1h`).

**Logz.io equivalent:**
- **API parameter:** `dayOffset` (2-day window, moveable)
- **Custom range:** Use `@timestamp` field with range filter in query
- **Format:** Unix timestamp (milliseconds) or ISO8601

**Example:**
```json
{
  "query": {
    "bool": {
      "filter": [
        {
          "range": {
            "@timestamp": {
              "gte": "2026-01-22T00:00:00Z",
              "lte": "2026-01-22T23:59:59Z"
            }
          }
        }
      ]
    }
  }
}
```

**Complexity:** Low

**Sources:**
- [Logz.io Search API](https://api-docs.logz.io/docs/logz/search/)

---

## Differentiators (Logz.io-Specific)

Features unique to Logz.io that could enhance Spectre's capabilities.

### 1. Pre-Computed Patterns (No Mining Required)

**Value proposition:** Eliminate CPU-intensive Drain algorithm execution during queries.

**How it works:**
- Logz.io Patterns Engine runs clustering at **ingestion time**
- Patterns are stored as indexed metadata
- Real-time pattern updates as new logs arrive
- Continuous algorithm improvement based on usage

**Benefit for Spectre:**
- Faster pattern queries (pre-computed vs on-demand)
- No TemplateStore state management needed
- Consistent patterns across multiple queries
- Reduced memory footprint (no in-process pattern cache)

**Implementation requirement:** Pattern metadata must be exposed via Search API. If not exposed, this differentiator is **unavailable**.

**Confidence:** LOW (API exposure unverified)

**Sources:**
- [Log Patterns Feature](https://logz.io/blog/troubleshooting-on-steroids-with-logz-io-log-patterns/)
- [Patterns Technology](https://logz.io/platform/features/log-patterns/)

### 2. Scroll API for Large Result Sets

**Value proposition:** Retrieve >10,000 logs efficiently with server-side pagination.

**How it works:**
- Initial request returns `scrollId` + first batch
- Subsequent requests use `scroll_id` for next batches
- Scroll expires after 20 minutes
- Time search limited to 5 minutes per scroll

**Benefit for Spectre:**
- VictoriaLogs hard limit: 500 logs per query
- Logz.io: Unlimited (paginated via scroll)
- Better support for deep investigations

**Use case:** When AI assistant needs comprehensive log analysis beyond initial sample.

**Complexity:** Medium (state management for scroll_id)

**Confidence:** HIGH

**Sources:**
- [Logz.io Scroll API](https://api-docs.logz.io/docs/logz/scroll/)

### 3. Advanced Aggregations (Cardinality, Stats, Percentiles)

**Value proposition:** Richer metrics beyond simple counts.

**Elasticsearch aggregations supported:**
- `cardinality`: Unique value counts (e.g., distinct error types)
- `stats`: min/max/avg/sum/count in single query
- `percentiles`: Distribution analysis (p50, p95, p99)
- `date_histogram`: Time-series bucketing

**Benefit for Spectre:**
- Enhanced overview tool with percentile-based insights
- Cardinality for "number of unique pods with errors"
- Stats for numeric log fields (latency, response codes)

**Use case:** Future tool like "performance_overview" showing latency percentiles by namespace.

**Complexity:** Low (Elasticsearch DSL well-documented)

**Confidence:** HIGH

**Sources:**
- [Elasticsearch Aggregations Guide](https://logz.io/blog/elasticsearch-aggregations/)

### 4. Lookup Lists for Query Simplification

**Value proposition:** Reusable filter sets for complex queries.

**How it works:**
- Admin creates named lists (e.g., "production-namespaces")
- Queries use `in lookups` operator instead of long OR chains
- Centralized management in OpenSearch Dashboards

**Benefit for Spectre:**
- Simplified namespace filtering for multi-tenant clusters
- User-defined groupings (e.g., "critical-services")

**Limitation:** Requires OpenSearch Dashboards setup (admin overhead).

**Complexity:** Medium (requires Lookup API integration)

**Confidence:** MEDIUM

**Sources:**
- [Lookup Lists Documentation](https://docs.logz.io/user-guide/lookups/)

---

## Anti-Features

Things to **deliberately NOT build** and why.

### 1. Custom Pattern Mining When Native Patterns Available

**What not to do:** Implement Drain algorithm for Logz.io if Patterns Engine is accessible via API.

**Why avoid:**
- Duplicates built-in functionality
- Inferior to Logz.io's continuously-learning algorithms
- Increases maintenance burden
- Wastes computational resources

**Do instead:**
- First, thoroughly investigate Pattern API exposure
- If exposed: Use native patterns directly
- If not exposed: Document as limitation, consider feedback to Logz.io

**Exception:** Fallback mining acceptable if Pattern API definitively unavailable.

### 2. Sub-Account Management Features

**What not to do:** Build tools for creating/managing Logz.io sub-accounts, adjusting quotas, or managing API tokens.

**Why avoid:**
- Spectre is a read-only observability tool (by design)
- Account management is admin/ops function, not AI assistant task
- Increases security surface (requires admin-level tokens)
- Out of scope for "log exploration" use case

**Do instead:**
- Document required permissions in integration setup
- Assume single account or read-only sub-account access

### 3. Real-Time Alerting/Monitoring

**What not to do:** Build alert creation, alert management, or continuous monitoring features.

**Why avoid:**
- Logz.io Alert API already provides comprehensive alerting
- Spectre is query-driven (pull), not event-driven (push)
- AI assistant use case is investigation, not proactive monitoring
- Adds complexity without value (alerts should stay in Logz.io UI)

**Do instead:**
- AI assistant can query existing logs to understand **why** an alert fired
- Focus on diagnostic/investigative queries

### 4. Wildcard-Leading Searches

**What not to do:** Support queries like `_msg:*error` (leading wildcard).

**Why avoid:**
- Logz.io API explicitly prohibits `allow_leading_wildcard: true`
- Leading wildcards are inefficient (full index scans)
- Elasticsearch best practice: avoid leading wildcards

**Do instead:**
- Use full-text search: `_msg:error` (matches anywhere in string)
- Use regex when specific patterns needed: `_msg:~"pattern"`
- Document limitation in tool descriptions

**Sources:**
- [Logz.io Search API Restrictions](https://api-docs.logz.io/docs/logz/search/)

### 5. Multi-Account Parallel Querying

**What not to do:** Query multiple Logz.io accounts simultaneously and merge results.

**Why avoid:**
- Scroll API limited to token's account (no cross-account)
- Merging results requires complex deduplication
- Users should configure single account for Spectre
- Adds latency and complexity

**Do instead:**
- Single account per integration config
- If multi-account needed, create separate integrations (each appears as distinct log source)

---

## Secret Management Features

Requirements for Spectre's secret infrastructure to support Logz.io integration.

### 1. API Token Storage (Required)

**What to store:**
- `api_token`: Logz.io API token (string, sensitive)
- `region`: Logz.io region (e.g., "us", "eu", "au") for URL construction

**Secret sensitivity:** HIGH
- API tokens grant read access to all logs in account
- Enterprise tokens have elevated permissions
- Compromise = unauthorized log access

**Rotation support:**
- Tokens don't expire automatically (manual rotation)
- Must support token update without integration reconfiguration
- UI should show token creation date (if available from API)

**Format validation:**
- Token format: Not documented (appears to be opaque string)
- No client-side validation possible

**Sources:**
- [Manage API Tokens](https://docs.logz.io/docs/user-guide/admin/authentication-tokens/api-tokens/)

### 2. Region-Specific Endpoint Configuration (Required)

**What to configure:**
- Base API URL varies by region:
  - US: `https://api.logz.io`
  - EU: `https://api-eu.logz.io`
  - AU: `https://api-au.logz.io`
  - CA: `https://api-ca.logz.io`

**Implementation:**
- Store region as enum: `["us", "eu", "au", "ca"]`
- Construct URL: `https://api-{region}.logz.io` (if not "us")
- Default: "us"

**UI consideration:**
- Dropdown for region selection during integration setup
- Validate region + token combo with test query

**Sources:**
- [Logz.io API Documentation](https://api-docs.logz.io/docs/logz/logz-io-api/)

### 3. Account ID Storage (Optional, but Recommended)

**What to store:**
- `account_id`: Numeric account identifier (not secret, but useful)

**Why useful:**
- Some API endpoints require account ID in URL path
- Helps troubleshoot multi-account scenarios
- Can display in UI for verification

**How to obtain:**
- Visible in Logz.io Settings > Account
- May be returned by token validation endpoint

**Sensitivity:** LOW (not secret, but scoped to account)

### 4. Token Validation Endpoint (Required)

**Purpose:** Test token validity during integration setup.

**Implementation:**
- Make simple Search API call (e.g., count logs in last 1m)
- Success = token valid + region correct
- Failure codes:
  - 401: Invalid token
  - 403: Community plan (no API access)
  - 429: Rate limit exceeded
  - 5xx: Logz.io service issue

**Example validation query:**
```json
POST https://api.logz.io/v1/search
{
  "query": {
    "query_string": {
      "query": "*"
    }
  },
  "size": 0,
  "from": 0
}
```

**Sources:**
- [Logz.io Search API](https://api-docs.logz.io/docs/logz/search/)

### 5. Rate Limit Handling (Required)

**Logz.io limits:**
- 100 concurrent API requests per account
- No documented per-second/per-minute limits

**Required features:**
- Retry logic with exponential backoff on 429
- Circuit breaker to prevent overwhelming account
- Log rate limit errors for debugging

**UI consideration:**
- Warn users about Enterprise/Pro plan requirement
- Show error message on 403 (Community plan)

**Implementation detail:**
- Share rate limiter across all tools in integration
- Don't spawn 100 concurrent requests (be conservative)

**Sources:**
- [API Tokens and Restrictions](https://docs.logz.io/docs/user-guide/admin/authentication-tokens/api-tokens/)

### 6. Secret Encryption at Rest (Existing Requirement)

**Assumption:** Spectre already encrypts integration secrets.

**Logz.io-specific:**
- No special encryption requirements
- Standard secret storage sufficient
- Token is opaque string (no embedded metadata to leak)

### 7. Connection Test Feature (Required)

**UI Flow:**
1. User enters API token + region
2. Click "Test Connection"
3. Backend validates:
   - Token format (non-empty)
   - Region valid
   - API reachable (network)
   - Token authenticated (Search API call)
   - Plan supports API (not 403)
4. Display result:
   - Success: "Connected to Logz.io {region} account"
   - Failure: Specific error message

**Sources:**
- [Logz.io API Authentication](https://api-docs.logz.io/docs/logz/logz-io-api/)

---

## Implementation Phases (Recommended)

Suggested order for feature development to match VictoriaLogs parity.

### Phase 1: Foundation (MVP)
**Goal:** Basic query capability without parity.

Features:
- Secret storage (token + region)
- Connection validation
- Single tool: `logzio_logs` (raw log search with filters)

**Rationale:** Proves API integration works before building complex features.

### Phase 2: Overview (Table Stakes)
**Goal:** Namespace-level severity summary.

Features:
- `logzio_overview` tool
- Terms aggregation by namespace
- Parallel queries (total, errors, warnings)
- Response format matching VictoriaLogs

**Rationale:** Most valuable tool for high-level cluster health.

### Phase 3: Patterns (Complex)
**Goal:** Log template clustering.

Features:
- Investigate Pattern API exposure
- If available: `logzio_patterns` with native patterns
- If not: Fallback pattern mining with TemplateStore

**Rationale:** Most complex feature due to API uncertainty. Build last to avoid blocking other work.

### Phase 4: Scroll API (Enhancement)
**Goal:** Support >1,000 log results.

Features:
- Scroll API integration in `logzio_logs`
- State management for scroll_id
- Automatic pagination for large queries

**Rationale:** Differentiator over VictoriaLogs, but not blocking for parity.

---

## Field Name Mapping Reference

Logz.io uses different field names than VictoriaLogs for Kubernetes metadata.

| Concept | VictoriaLogs | Logz.io | Notes |
|---------|--------------|---------|-------|
| Namespace | `kubernetes.pod_namespace` | `kubernetes.namespace` | Logz.io shorter |
| Pod Name | `kubernetes.pod_name` | `kubernetes.pod_name` | Same |
| Container | `kubernetes.container_name` | `kubernetes.container_name` | Same |
| Log Level | `level` | `level` | Same (if structured) |
| Message | `_msg` | `message` | Logz.io uses standard field |
| Timestamp | `_time` | `@timestamp` | Elasticsearch convention |

**Implementation note:** Create field mapping layer to abstract differences.

**Sources:**
- [Kubernetes Log Fields](https://docs.logz.io/docs/shipping/containers/kubernetes/)
- VictoriaLogs query.go code review

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| **Overview Tool** | HIGH | Terms aggregation well-documented, parallel queries proven pattern |
| **Logs Tool** | HIGH | Standard Search API, field mapping straightforward |
| **Patterns Tool** | LOW | API exposure unclear, may require fallback mining |
| **Scroll API** | HIGH | Documented endpoint, known limitations |
| **Secret Management** | HIGH | Requirements clear from API docs |
| **Field Names** | MEDIUM | Based on Kubernetes shipper docs, not verified in actual API responses |
| **Rate Limits** | MEDIUM | 100 concurrent documented, but per-second limits unknown |
| **Enterprise Access** | HIGH | Clearly documented (Enterprise/Pro only for Search API) |

---

## Open Questions for Phase-Specific Research

These questions **cannot be answered** without hands-on API testing. Flag for deeper research during Phase 3 (Patterns).

### 1. Pattern API Exposure
**Question:** Is Logz.io Patterns Engine accessible via Search API?

**How to answer:**
- Run Search API query, inspect response for pattern-related fields
- Check if `logzio.pattern`, `pattern_id`, or similar fields exist
- Test aggregation on pattern field
- Review Elasticsearch index mapping (if accessible)

**Fallback:** Implement Drain-based mining if patterns not exposed.

### 2. Kubernetes Field Names in Practice
**Question:** Do actual log documents use `kubernetes.namespace` or `kubernetes.pod_namespace`?

**How to answer:**
- Fetch sample logs from test Logz.io account
- Inspect JSON structure
- Verify field names match documentation

**Risk:** Documentation may differ from reality (fluentd config variations).

### 3. Novelty Detection Without Previous Window Query
**Question:** Does Logz.io expose pattern creation timestamps to detect "new" patterns?

**How to answer:**
- Inspect pattern metadata for `first_seen` or `created_at` field
- Test if pattern count history is available
- Check if Logz.io has built-in "rare patterns" feature

**Fallback:** Implement time-window comparison like VictoriaLogs.

### 4. Real-World Rate Limit Behavior
**Question:** How aggressive is the 100 concurrent request limit in practice?

**How to answer:**
- Load test with parallel Overview queries (3 concurrent per request)
- Measure retry/throttle frequency
- Determine safe concurrency level

**Impact:** May need request queuing if limit too strict.

---

## Sources Summary

### HIGH Confidence (Official Documentation)
- [Logz.io Search API](https://api-docs.logz.io/docs/logz/search/)
- [Logz.io Scroll API](https://api-docs.logz.io/docs/logz/scroll/)
- [Manage API Tokens](https://docs.logz.io/docs/user-guide/admin/authentication-tokens/api-tokens/)
- [Kubernetes Log Shipping](https://docs.logz.io/docs/shipping/containers/kubernetes/)
- [OpenSearch Dashboards Best Practices](https://docs.logz.io/docs/user-guide/log-management/opensearch-dashboards/opensearch-best-practices/)

### MEDIUM Confidence (Official Guides & Blogs)
- [Elasticsearch Aggregations Guide](https://logz.io/blog/elasticsearch-aggregations/)
- [Elasticsearch Queries Guide](https://logz.io/blog/elasticsearch-queries/)
- [Understanding Log Patterns](https://docs.logz.io/docs/user-guide/log-management/opensearch-dashboards/opensearch-patterns/)

### LOW Confidence (Unverified for API)
- [Log Patterns Feature Announcement](https://logz.io/blog/announcing-log-patterns-saving-time-and-money-for-engineers/)
- [Troubleshooting with Log Patterns](https://logz.io/blog/troubleshooting-on-steroids-with-logz-io-log-patterns/)

---

## Recommendations for Roadmap

1. **Phase 1 (Foundation):** Quick win — basic Search API integration with single tool
2. **Phase 2 (Overview):** High value — namespace severity summary matches VictoriaLogs
3. **Phase 3 (Patterns):** Research flag — investigate Pattern API, plan fallback
4. **Phase 4 (Scroll):** Enhancement — differentiate from VictoriaLogs limitations

**Overall assessment:** Logz.io integration is **feasible** for v1.2. Patterns tool requires deeper research but has known fallback (mining). Enterprise plan requirement is **blocking** for Community users.
