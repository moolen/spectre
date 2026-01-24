# Stack Research: Logz.io Integration + K8s Secret Management

**Project:** Spectre v1.2 - Logz.io Integration
**Researched:** 2026-01-22
**Confidence:** HIGH for libraries, MEDIUM for Logz.io client patterns

## Executive Summary

For v1.2 milestone, add Logz.io integration using official Elasticsearch client + query builder, and implement file-based secret management with hot-reload using existing fsnotify infrastructure.

**Key Decision:** Use `elastic/go-elasticsearch/v8` (official) + `effdsl/v2` (query builder) instead of deprecated `olivere/elastic`. No official Logz.io Go SDK exists - build custom client using Elasticsearch DSL patterns.

**Secret Management:** Extend existing `fsnotify`-based config watcher pattern (already in use at `internal/config/integration_watcher.go`) to watch Kubernetes Secret mount paths.

---

## Recommended Stack

### Core HTTP Client for Logz.io

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `net/http` (stdlib) | Go 1.24.4 | HTTP client for Logz.io API | Standard library, already used in VictoriaLogs integration, sufficient for custom headers (X-API-TOKEN) |
| `elastic/go-elasticsearch` | v9.2.1 (or v8.18.0) | Type definitions for Elasticsearch responses | Official client provides mature JSON unmarshaling for ES responses, forward-compatible with Logz.io's Elasticsearch-compatible API |

**Rationale:** Logz.io has NO official Go SDK. Their API is Elasticsearch DSL over HTTP with custom auth header. Use stdlib HTTP client with custom `RoundTripper` for auth injection, leverage `go-elasticsearch` types for response parsing only (not transport).

### Elasticsearch DSL Query Building

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/sdqri/effdsl/v2` | v2.2.0 | Type-safe Elasticsearch query builder | Actively maintained (last release Sept 2024), supports go-elasticsearch v8, provides functional API for programmatic query construction, MIT license |

**Alternatives Considered:**
- `aquasecurity/esquery`: **REJECTED** - Only supports go-elasticsearch v7, stale (last release March 2021), marked as "early release" with API instability warnings
- `olivere/elastic`: **REJECTED** - Officially deprecated, author abandoned v8+ support
- Raw `map[string]interface{}`: **REJECTED** - Error-prone for complex queries, no compile-time safety, maintenance burden

### Secret Management

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/fsnotify/fsnotify` | v1.9.0 | File system change notifications | Already in `go.mod`, proven in production at `internal/config/integration_watcher.go`, cross-platform, handles K8s Secret atomic writes (RENAME events) |
| `os.ReadFile` (stdlib) | Go 1.24.4 | Read secret file contents | Standard library, sufficient for reading mounted Secret files |

**Rationale:** Kubernetes mounts Secrets as files with automatic updates via atomic writes (RENAME events). Existing `IntegrationWatcher` pattern already handles debouncing, atomic write detection, and hot-reload callbacks. Reuse this infrastructure.

---

## Implementation Patterns

### 1. Logz.io Client Architecture

**Pattern:** Custom HTTP client with regional endpoint support + query builder

```go
// Client structure (similar to VictoriaLogs pattern)
type LogzioClient struct {
    baseURL    string              // Regional API endpoint
    apiToken   string              // X-API-TOKEN value
    httpClient *http.Client        // Configured with timeout
    region     string              // us, eu, uk, au, ca
}

// Regional endpoints (from official docs)
var RegionEndpoints = map[string]string{
    "us": "https://api.logz.io",
    "eu": "https://api-eu.logz.io",
    "uk": "https://api-uk.logz.io",
    "au": "https://api-au.logz.io",
    "ca": "https://api-ca.logz.io",
}

// HTTP transport with auth injection
type logzioTransport struct {
    base     http.RoundTripper
    apiToken string
}

func (t *logzioTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    req.Header.Set("X-API-TOKEN", t.apiToken)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept-Encoding", "gzip, deflate") // Compression recommended
    return t.base.RoundTrip(req)
}
```

**Why this pattern:**
- Follows VictoriaLogs client architecture (consistency)
- Centralized auth header injection via RoundTripper
- Regional endpoint selection at client creation
- Enables middleware (metrics, logging, circuit breaker) via transport chain

**Sources:**
- [Logz.io API Authentication](https://api-docs.logz.io/docs/logz/logz-io-api/)
- [Logz.io Regions](https://docs.logz.io/docs/user-guide/admin/hosting-regions/account-region/)
- [Go HTTP Client Best Practices](https://blog.logrocket.com/configuring-the-go-http-client/)

### 2. Query Building with effdsl

**Pattern:** Type-safe query construction with effdsl

```go
import (
    "github.com/elastic/go-elasticsearch/v8"
    "github.com/sdqri/effdsl/v2"
    "github.com/sdqri/effdsl/v2/queries/boolquery"
    "github.com/sdqri/effdsl/v2/queries/rangequery"
)

// Example: Build time-range + namespace filter query
func buildLogQuery(namespace string, startTime, endTime int64) (string, error) {
    query, err := effdsl.Define(
        effdsl.WithQuery(
            boolquery.BoolQuery(
                boolquery.WithMust(
                    rangequery.RangeQuery("@timestamp",
                        rangequery.WithGte(startTime),
                        rangequery.WithLte(endTime),
                    ),
                ),
                boolquery.WithFilter(
                    termquery.TermQuery("kubernetes.namespace", namespace),
                ),
            ),
        ),
        effdsl.WithSize(1000), // Logz.io limit: 10k non-aggregated
    )
    return query, err
}
```

**Why effdsl:**
- Type-safe: Compile-time validation prevents DSL syntax errors
- Functional API: Easy to build queries programmatically (critical for dynamic MCP tool parameters)
- Low abstraction: Close to Elasticsearch JSON, easy to debug
- Actively maintained: v2.2.0 released Sept 2024, 117 commits

**Alternatives rejected:**
- Raw JSON strings: No validation, string manipulation complexity
- `map[string]interface{}`: Runtime errors, no autocomplete, brittle

**Sources:**
- [effdsl GitHub](https://github.com/sdqri/effdsl)
- [Elasticsearch Query DSL Docs](https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl.html)

### 3. Kubernetes Secret File Management

**Pattern:** Extend existing `IntegrationWatcher` for secret files

```go
// Reuse existing config watcher pattern
type SecretWatcher struct {
    config   config.IntegrationWatcherConfig
    callback ReloadCallback
    // ... (same fields as IntegrationWatcher)
}

// Integration config references secret file path
type LogzioConfig struct {
    URL             string `yaml:"url"`               // Regional API endpoint
    Region          string `yaml:"region"`            // us, eu, uk, au, ca
    APITokenFile    string `yaml:"api_token_file"`    // /var/run/secrets/logzio/api-token
}

// Load secret from file
func loadAPIToken(path string) (string, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return "", fmt.Errorf("failed to read API token: %w", err)
    }
    return strings.TrimSpace(string(data)), nil
}

// Hot-reload callback updates client
func (l *LogzioIntegration) reloadSecret(path string) error {
    token, err := loadAPIToken(path)
    if err != nil {
        return err
    }

    // Atomically update client with new token
    newClient := NewLogzioClient(l.config.URL, token, l.config.Region)

    l.mu.Lock()
    oldClient := l.client
    l.client = newClient
    l.mu.Unlock()

    // Gracefully drain old client
    // (optional: wait for in-flight requests)

    return nil
}
```

**Why this pattern:**
- Proven in production: `internal/config/integration_watcher.go` uses fsnotify with 500ms debounce
- K8s atomic writes: fsnotify detects RENAME events when kubelet updates Secret symlinks
- Zero-downtime reload: New client replaces old without dropping requests
- Fail-open: Invalid secret file logged but watcher continues (matches existing behavior)

**K8s Secret Mount Details:**
- Secrets mounted as volumes: `/var/run/secrets/<secret-name>/<key-name>`
- Kubelet updates: Every sync period (default 1 minute) + local cache TTL
- File permissions: 0400 (read-only)
- Atomic updates: Old symlink replaced, triggers fsnotify.Rename event

**Sources:**
- [K8s Secrets as Files](https://kubernetes.io/docs/concepts/configuration/secret/)
- [fsnotify GitHub](https://github.com/fsnotify/fsnotify)
- [Go Secrets Management for K8s](https://oneuptime.com/blog/post/2026-01-07-go-secrets-management-kubernetes/view)
- Existing code: `/home/moritz/dev/spectre-via-ssh/internal/config/integration_watcher.go`

### 4. Multi-Region Failover (Future Enhancement)

**NOT REQUIRED for v1.2**, but documented for future:

```go
// Optional: Client with regional failover
type MultiRegionClient struct {
    clients  []*LogzioClient  // Primary + fallback regions
    current  int              // Active client index
    mu       sync.RWMutex
}

// Circuit breaker pattern for auto-failover
func (m *MultiRegionClient) executeWithFailover(fn func(*LogzioClient) error) error {
    // Try primary, fall back to secondary on failure
    // Requires: github.com/sony/gobreaker or similar
}
```

**Defer to post-v1.2:** User specifies region in config, single-region client sufficient for MVP.

**Sources:**
- [Multi-Region Failover Strategies](https://systemdr.substack.com/p/multi-region-failover-strategies)
- [Resilient HTTP Client in Go](https://dev.to/rafaeljesus/resilient-http-client-in-go-ho6)

---

## Logz.io API Specifics

### Search Endpoint

**Endpoint:** `POST /v1/search`

**Request Body (Elasticsearch DSL):**
```json
{
  "query": {
    "bool": {
      "must": [
        { "range": { "@timestamp": { "gte": 1640000000000, "lte": 1640086400000 } } }
      ],
      "filter": [
        { "term": { "kubernetes.namespace": "production" } }
      ]
    }
  },
  "size": 1000,
  "sort": [{ "@timestamp": "desc" }]
}
```

**Authentication:**
- Header: `X-API-TOKEN: <api-token>`
- Token location: K8s Secret mounted at `/var/run/secrets/logzio/api-token`

**Rate Limits:**
- 100 concurrent requests per account
- Result limits: 1,000 aggregated, 10,000 non-aggregated
- Pagination: Use Scroll API (`/v1/scroll`) for large result sets

**Compression:**
- Strongly recommended: `Accept-Encoding: gzip, deflate`
- Large responses (10k results) can be multiple MB

**Regional Endpoints:**
| Region | API Base URL |
|--------|--------------|
| US East (default) | `https://api.logz.io` |
| EU (Frankfurt) | `https://api-eu.logz.io` |
| UK (London) | `https://api-uk.logz.io` |
| Australia (Sydney) | `https://api-au.logz.io` |
| Canada (Central) | `https://api-ca.logz.io` |

**Sources:**
- [Logz.io API Docs](https://api-docs.logz.io/docs/logz/logz-io-api/)
- [Logz.io Regions](https://docs.logz.io/docs/user-guide/admin/hosting-regions/account-region/)

### Scroll API (for large result sets)

**Endpoint:** `POST /v1/scroll`

**Use case:** Paginate through >10,000 results

**Pattern:**
1. Initial search request with `scroll=5m` parameter
2. Extract `_scroll_id` from response
3. Subsequent scroll requests with `_scroll_id` in body
4. Stop when no results returned

**Implementation note:** Defer to post-MVP unless MCP tools require >10k log retrieval (unlikely for AI assistant use cases).

---

## What NOT to Use

### AVOID: olivere/elastic

**Status:** Officially deprecated (Jan 2026)

**Why deprecated:**
- Author abandoned project (no v8+ support planned)
- GitHub README: "Deprecated: Use the official Elasticsearch client"
- Community moving to official client

**If found in code:** Migrate to `elastic/go-elasticsearch` + `effdsl`

**Sources:**
- [olivere/elastic GitHub](https://github.com/olivere/elastic)
- [Official vs olivere discussion](https://discuss.elastic.co/t/go-elasticsearch-versus-olivere-golang-client/252248)

### AVOID: aquasecurity/esquery

**Status:** Stale, limited support

**Why avoid:**
- Only supports go-elasticsearch v7 (v8/v9 incompatible)
- Last release: March 2021 (3+ years stale)
- README warns: "early release, API may still change"
- 21 commits total, low activity

**Use instead:** effdsl (v2.2.0, Sept 2024, 117 commits, v8 support)

**Sources:**
- [esquery GitHub](https://github.com/aquasecurity/esquery)
- [effdsl GitHub](https://github.com/sdqri/effdsl)

### AVOID: Environment Variables for Secrets

**Why avoid:**
- K8s best practice: Prefer file-based secrets over env vars
- Security: Env vars visible in `/proc`, logs, error dumps
- Hot-reload: Env vars require pod restart, files update automatically
- Audit: File access auditable via RBAC, env vars not

**Use instead:** K8s Secret mounted as file at `/var/run/secrets/logzio/api-token`

**Sources:**
- [K8s Secrets Documentation](https://kubernetes.io/docs/concepts/configuration/secret/)
- [File-based vs Env Vars](https://itnext.io/how-to-mount-secrets-as-files-or-environment-variables-in-kubernetes-f03d545dcd89)

### AVOID: Building Custom Elasticsearch DSL JSON Strings

**Why avoid:**
- Error-prone: Typos in field names, invalid syntax
- No validation: Errors discovered at runtime
- Brittle: Hard to refactor, test, or extend
- Maintenance burden: String manipulation complexity

**Use instead:** effdsl type-safe query builder

**Example of BAD pattern:**
```go
// DON'T DO THIS
query := fmt.Sprintf(`{
  "query": {
    "bool": {
      "must": [
        { "range": { "@timestamp": { "gte": %d } } }
      ]
    }
  }
}`, startTime) // Easy to break, no validation
```

**Example of GOOD pattern:**
```go
// DO THIS
query, err := effdsl.Define(
    effdsl.WithQuery(
        boolquery.BoolQuery(
            boolquery.WithMust(
                rangequery.RangeQuery("@timestamp",
                    rangequery.WithGte(startTime),
                ),
            ),
        ),
    ),
)
```

---

## Installation Instructions

### 1. Add Dependencies to go.mod

```bash
# Elasticsearch official client (for types/responses)
go get github.com/elastic/go-elasticsearch/v8@v8.18.0

# Query builder
go get github.com/sdqri/effdsl/v2@v2.2.0

# fsnotify already in go.mod (v1.9.0)
```

**Note:** Choose `v8` (stable, v8.18.0) or `v9` (latest, v9.2.1) based on compatibility needs. v8 recommended for stability, v9 if features required.

### 2. Helm Chart Updates (for K8s Secret mount)

```yaml
# templates/deployment.yaml
spec:
  containers:
  - name: spectre
    volumeMounts:
    - name: logzio-api-token
      mountPath: /var/run/secrets/logzio
      readOnly: true

  volumes:
  - name: logzio-api-token
    secret:
      secretName: logzio-api-token
      items:
      - key: token
        path: api-token
        mode: 0400  # Read-only
```

```yaml
# Example Secret (applied separately, NOT in Helm chart)
apiVersion: v1
kind: Secret
metadata:
  name: logzio-api-token
  namespace: spectre
type: Opaque
stringData:
  token: "your-api-token-here"
```

### 3. Integration Config Schema

```yaml
# config/integrations.yaml
integrations:
  - name: logzio-prod
    type: logzio
    config:
      region: us                                    # or eu, uk, au, ca
      api_token_file: /var/run/secrets/logzio/api-token
      timeout_seconds: 60                           # HTTP client timeout
      compression: true                             # Enable gzip/deflate
```

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Elasticsearch Client Choice | HIGH | Official go-elasticsearch is well-documented, actively maintained, forward-compatible. v9.2.1 released Dec 2025. |
| Query Builder Choice | MEDIUM-HIGH | effdsl is actively maintained (Sept 2024), good API design, but smaller community (34 stars). Production usage not widely documented. Recommend wrapping in abstraction layer. |
| Secret Management Pattern | HIGH | fsnotify proven in Spectre codebase (`integration_watcher.go`), K8s Secret mounting is standard practice, pattern well-documented. |
| Logz.io API Compatibility | MEDIUM | No official Go SDK means custom implementation. Elasticsearch DSL compatibility verified via docs, but edge cases may exist. Recommend comprehensive integration tests. |
| Regional Endpoints | HIGH | Official Logz.io docs list 5 regions with explicit API URLs. Straightforward URL mapping. |

## Risk Mitigation

### Risk: effdsl stability in production
**Mitigation:**
- Wrap effdsl in internal abstraction (`internal/logzio/query.go`)
- If effdsl fails, fallback to raw Elasticsearch JSON via `json.Marshal`
- Comprehensive unit tests for query generation
- Document all query patterns used

### Risk: Logz.io API changes
**Mitigation:**
- Pin to Elasticsearch DSL version in documentation
- Version integration API responses
- Comprehensive error handling for API changes
- Monitor Logz.io API changelog (https://api-docs.logz.io/)

### Risk: Secret file hot-reload race conditions
**Mitigation:**
- Reuse proven debounce logic from `IntegrationWatcher` (500ms)
- Atomic client swap with mutex
- Graceful degradation: Old secret continues working until new validated
- Integration test with K8s Secret update simulation

---

## Research Gaps

### LOW Priority (defer to implementation phase):
- Logz.io Scroll API pagination details (only if MCP tools need >10k results)
- Circuit breaker library selection (only if multi-region failover required)
- Compression benchmark (gzip vs deflate performance)

### Addressed in this research:
- ~~Which Elasticsearch Go client to use~~ → elastic/go-elasticsearch v8/v9
- ~~Query builder library selection~~ → effdsl/v2
- ~~Secret management pattern~~ → fsnotify + K8s Secret files
- ~~Regional endpoint mapping~~ → Documented 5 regions
- ~~Authentication mechanism~~ → X-API-TOKEN header via RoundTripper

---

## Sources

### Official Documentation
- [Logz.io API Documentation](https://api-docs.logz.io/docs/logz/logz-io-api/)
- [Logz.io Account Regions](https://docs.logz.io/docs/user-guide/admin/hosting-regions/account-region/)
- [Elasticsearch Query DSL](https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl.html)
- [Kubernetes Secrets](https://kubernetes.io/docs/concepts/configuration/secret/)
- [go-elasticsearch GitHub](https://github.com/elastic/go-elasticsearch)
- [go-elasticsearch Examples](https://www.elastic.co/guide/en/elasticsearch/client/go-api/current/examples.html)

### Libraries
- [fsnotify GitHub](https://github.com/fsnotify/fsnotify)
- [effdsl GitHub](https://github.com/sdqri/effdsl)
- [aquasecurity/esquery GitHub](https://github.com/aquasecurity/esquery) (rejected)
- [olivere/elastic GitHub](https://github.com/olivere/elastic) (deprecated)

### Community Resources
- [Go Secrets Management for Kubernetes (Jan 2026)](https://oneuptime.com/blog/post/2026-01-07-go-secrets-management-kubernetes/view)
- [Configuring Go HTTP Client](https://blog.logrocket.com/configuring-the-go-http-client/)
- [Go HTTP Client Middleware](https://echorand.me/posts/go-http-client-middleware/)
- [Mounting K8s Secrets as Files](https://itnext.io/how-to-mount-secrets-as-files-or-environment-variables-in-kubernetes-f03d545dcd89)
- [Elasticsearch Clients Comparison](https://medium.com/a-journey-with-go/go-elasticsearch-clients-study-case-dbaee1e02c7)
- [Multi-Region Failover Strategies](https://systemdr.substack.com/p/multi-region-failover-strategies)

### Stack Overflow / Discussions
- [Go-Elasticsearch vs Olivere](https://discuss.elastic.co/t/go-elasticsearch-versus-olivere-golang-client/252248)
- [olivere/elastic v8 Support Issue](https://github.com/olivere/elastic/issues/1240)

---

## Next Steps for Roadmap Creation

Based on this stack research, recommended phase structure for v1.2:

1. **Phase 1: Logz.io Client Foundation**
   - Addresses: HTTP client with regional endpoints, X-API-TOKEN auth
   - Uses: stdlib `net/http`, custom RoundTripper
   - Avoids: Premature multi-region failover complexity

2. **Phase 2: Query DSL Integration**
   - Addresses: Type-safe query building for Search API
   - Uses: effdsl/v2, wrap in abstraction layer
   - Avoids: Raw JSON string manipulation

3. **Phase 3: Secret File Management**
   - Addresses: K8s Secret mounting, hot-reload
   - Uses: Existing fsnotify infrastructure, extend IntegrationWatcher
   - Avoids: Environment variable approach

4. **Phase 4: MCP Tool Registration**
   - Addresses: logzio_{name}_overview, logzio_{name}_logs tools
   - Uses: Existing integration.ToolRegistry pattern
   - Avoids: Premature patterns tool (defer to v1.3)

**Likely research flags:**
- Phase 2: May need deeper research if effdsl doesn't cover required query types (e.g., aggregations, nested queries)
- Phase 3: Standard pattern, unlikely to need additional research

**Estimated complexity:**
- Phase 1: Medium (custom client, similar to VictoriaLogs)
- Phase 2: Low-Medium (query builder wrapper)
- Phase 3: Low (reuse existing pattern)
- Phase 4: Low (copy VictoriaLogs tool pattern)
