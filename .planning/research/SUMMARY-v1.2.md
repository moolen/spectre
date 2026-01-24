# Project Research Summary: v1.2 Logz.io Integration

**Project:** Spectre v1.2 - Logz.io Integration + Secret Management
**Researched:** 2026-01-22
**Confidence:** HIGH (stack, architecture), MEDIUM (patterns API exposure)

## Executive Summary

Spectre v1.2 adds Logz.io as a second log backend with production-grade secret management. The integration follows the proven VictoriaLogs plugin pattern but introduces three architectural extensions: multi-region API client, file-based secret hot-reload via fsnotify, and Elasticsearch DSL query building. Research confirms feasibility with clear implementation path and identified risks.

**Core technology decision:** Use stdlib `net/http` with `elastic/go-elasticsearch` types + `effdsl/v2` query builder. Logz.io has no official Go SDK—build custom HTTP client following Elasticsearch compatibility patterns. Extend existing fsnotify-based config watcher to support Kubernetes Secret file mounts with atomic write handling.

**Critical findings:** Logz.io's Patterns Engine (pre-computed log clustering) has unclear API exposure—research recommends investigating pattern metadata fields during Phase 1, with fallback to VictoriaLogs-style Drain mining if unavailable. Secret management requires careful fsnotify handling due to Kubernetes atomic symlink rotation (subPath volumes break hot-reload). Multi-region support is table stakes (5 regional endpoints with different URLs).

**Key risk:** Kubernetes Secret subPath incompatibility with hot-reload. This is a critical pitfall that blocks zero-downtime credential rotation. Prevention requires volume-level mounts (not file-level subPath) and re-establishing fsnotify watches after atomic write events.

**Roadmap readiness:** Clear 5-phase structure emerges from research. Phase 1-2 (client foundation + secret management) are low-risk with proven patterns. Phase 3-4 (pattern mining + MCP tools) need targeted research flags for Pattern API verification and scroll lifecycle management. Overall confidence: HIGH for delivery, MEDIUM for timeline estimation (patterns uncertainty).

## Key Findings

### Recommended Stack

**HTTP Client Layer:**
- `net/http` (stdlib) - Custom HTTP client with regional endpoint mapping, sufficient for bearer auth
- `elastic/go-elasticsearch` v8.18.0 or v9.2.1 - Type definitions for response unmarshaling (not transport)
- `effdsl/v2` v2.2.0 - Type-safe Elasticsearch DSL query builder, actively maintained

**Secret Management:**
- `fsnotify` v1.9.0 - Already in go.mod, proven in `internal/config/integration_watcher.go`
- `os.ReadFile` (stdlib) - Read API token from Kubernetes Secret volume mount
- File-based pattern: `/var/run/secrets/logzio/api-token` (no environment variables)

**Regional Endpoints:**
| Region | API Base URL |
|--------|--------------|
| US | `https://api.logz.io` |
| EU | `https://api-eu.logz.io` |
| UK | `https://api-uk.logz.io` |
| AU | `https://api-au.logz.io` |
| CA | `https://api-ca.logz.io` |

**Why this stack:**
- Consistency: Mirrors VictoriaLogs HTTP client pattern (custom transport for auth injection)
- Type safety: effdsl prevents Elasticsearch DSL syntax errors at compile time
- Hot-reload: fsnotify proven in production for config watching, extends to secret files
- Kubernetes-native: Volume-mounted secrets work with any secret backend (Vault, AWS, manual)

**Rejected alternatives:**
- `olivere/elastic` - Officially deprecated (author abandoned v8+ support)
- `aquasecurity/esquery` - Stale (last release March 2021), only supports go-elasticsearch v7
- Environment variables for secrets - No hot-reload support (requires pod restart)
- Raw JSON query strings - Error-prone, no compile-time validation

### Expected Features

**Table Stakes (VictoriaLogs Parity):**

1. **Overview Tool** - Namespace-level severity summary
   - API: `/v1/search` with terms aggregation on `kubernetes.namespace`
   - Parallel queries: total, errors, warnings (same pattern as VictoriaLogs)
   - Confidence: HIGH - Standard Elasticsearch aggregations, well-documented

2. **Logs Tool** - Raw log retrieval with filters
   - Filters: namespace, pod, container, severity, time range
   - Result limits: 1,000 per page (aggregated), 10,000 total (non-aggregated)
   - Scroll API available for pagination beyond limits
   - Confidence: HIGH - Core Search API functionality

3. **Patterns Tool** - Log template clustering
   - Logz.io has built-in Patterns Engine (pre-computed during ingestion)
   - **CRITICAL UNCERTAINTY:** Pattern metadata API exposure unclear
   - Fallback: Reuse VictoriaLogs Drain algorithm + TemplateStore if API unavailable
   - Confidence: LOW for native patterns, HIGH for fallback mining

**Differentiators (Logz.io-Specific):**

1. **Pre-Computed Patterns** - No CPU-intensive mining required if API exposes pattern metadata
2. **Scroll API** - Unlimited pagination vs VictoriaLogs 500-log hard limit
3. **Advanced Aggregations** - Cardinality, percentiles, stats (richer than LogsQL)
4. **Multi-Region Support** - Geographic data locality, compliance requirements

**Anti-Features (Deliberately Excluded):**

1. **Custom pattern mining when native patterns available** - Duplicates built-in functionality
2. **Sub-account management** - Out of scope for read-only observability tool
3. **Real-time alerting** - Logz.io Alert API handles this, Spectre is query-driven
4. **Leading wildcard searches** - Explicitly prohibited by Logz.io API
5. **Multi-account parallel querying** - Scroll API limited to single account

**Secret Management Requirements:**

- API token storage (sensitive, no expiration, manual rotation)
- Region configuration (5 options, affects endpoint URL)
- Connection validation (test query during setup)
- Rate limit handling (100 concurrent requests per account)
- Hot-reload support (zero-downtime credential rotation)
- Encryption at rest (Kubernetes-level, not application-level)

### Architecture Approach

**Component Structure:**

```
LogzioIntegration (internal/integration/logzio/logzio.go)
├── RegionalClient (client.go) - HTTP client with regional endpoints
│   ├── Region endpoint mapping (5 regions)
│   ├── Bearer token authentication (X-API-TOKEN header)
│   └── Thread-safe token updates (RWMutex for hot-reload)
├── QueryBuilder (query.go) - Elasticsearch DSL generation via effdsl
│   ├── SearchParams → Elasticsearch JSON
│   ├── Time range conversion (Unix ms)
│   └── Kubernetes field mapping
├── SecretWatcher (secret_watcher.go) - fsnotify file monitoring
│   ├── Watch secret file path
│   ├── Detect atomic writes (Kubernetes symlink rotation)
│   ├── Callback to client.UpdateToken()
│   └── Re-establish watch after IN_DELETE_SELF events
└── Tools (tools_*.go) - MCP tool implementations
    ├── logzio_{name}_overview
    ├── logzio_{name}_logs
    └── logzio_{name}_patterns (Phase 2, pending API research)
```

**Integration with Existing Systems:**

- **Factory Registration:** Uses existing `integration.RegisterFactory("logzio", ...)` pattern
- **Lifecycle Management:** Implements `integration.Integration` interface (no changes needed)
- **Config Hot-Reload:** Managed by existing `IntegrationWatcher` (integrations.yaml level)
- **Secret Hot-Reload:** New `SecretWatcher` at integration instance level (file-level)
- **MCP Tool Registry:** Uses existing `ToolRegistry.RegisterTool()` adapter

**Data Flow Patterns:**

1. **Query Flow:** MCP Client → MCP Server → Tool → RegionalClient → Logz.io API
2. **Secret Rotation:** K8s Secret update → fsnotify event → SecretWatcher → client.UpdateToken() → next query uses new token
3. **Error Recovery:** 401 error → Health check detects Degraded → Auto-recovery via Start() with new token

**Build Order (Dependency-Driven):**

1. **Phase 1: Core Client** - HTTP client, regional endpoints, query builder, basic health checks
2. **Phase 2: Secret File Reading** - Initial token load from file, config parsing, error handling
3. **Phase 3: Secret Hot-Reload** - fsnotify integration, atomic write handling, thread-safe updates
4. **Phase 4: MCP Tools** - Tool registration, overview/logs/patterns implementations
5. **Phase 5: Helm Chart + Docs** - extraVolumes config, rotation workflow docs, setup guide

**Key Architecture Decisions:**

- **File-based secrets over env vars:** Enables hot-reload without pod restart
- **Watch parent directory, not file:** Avoids fsnotify inode change issues
- **RWMutex for token updates:** Queries read concurrently, rotation locks briefly for write
- **No multi-region failover:** Single region per integration (defer to v2+)
- **effdsl wrapped in abstraction:** Allows fallback to raw JSON if library issues arise

### Critical Pitfalls

**Top 5 Risks (Ordered by Impact):**

**1. Kubernetes Secret subPath Breaks Hot-Reload** (CRITICAL)
- **Problem:** subPath mounts bypass Kubernetes atomic writer, fsnotify never detects updates
- **Impact:** Secret rotation causes downtime, authentication failures, manual pod restarts required
- **Prevention:** Volume-level mounts only (not subPath), document explicitly in deployment YAML
- **Phase:** Phase 2 (Secret Management) - Must validate before MCP tools

**2. Atomic Editor Saves Cause fsnotify Watch Loss** (CRITICAL)
- **Problem:** Kubernetes Secret updates use rename → fsnotify watch on inode breaks → events missed
- **Impact:** Silent secret reload failures, security window between rotation and detection
- **Prevention:** Re-establish watch after Remove/Rename events, increase debounce to 200ms, watch parent directory
- **Phase:** Phase 2 (Secret Management) - Core hot-reload reliability

**3. Leading Wildcard Queries Disabled by Logz.io** (MODERATE)
- **Problem:** API enforces `allow_leading_wildcard: false`, queries like `*-service` fail
- **Impact:** User-facing errors, degrades MCP tool experience
- **Prevention:** Query validation layer, reject leading wildcards with helpful error message
- **Phase:** Phase 3 (MCP Tools) - Query construction validation

**4. Scroll API Context Expiration After 20 Minutes** (MODERATE)
- **Problem:** Long-running pattern mining operations lose scroll context mid-operation
- **Impact:** Incomplete results, user retries hit rate limit
- **Prevention:** 15-minute internal timeout, checkpoint/resume for large datasets, stream results incrementally
- **Phase:** Phase 3 (MCP Tools) - Pattern mining implementation

**5. Secret Value Logging During Debug** (CRITICAL - SECURITY)
- **Problem:** API tokens logged in error messages, config dumps, HTTP request logs
- **Impact:** Credential leakage to logs, compliance violation, incident response burden
- **Prevention:** Struct tags for secret fields, redact tokens in String() methods, sanitize HTTP errors
- **Phase:** Phase 2 (Secret Management) - Establish logging patterns before MCP tools

**Additional Moderate Pitfalls:**

- **Rate limit handling without exponential backoff** - 100 concurrent requests per account, need jitter retry
- **Result limit confusion (1K vs 10K)** - Aggregated queries have 1K limit, non-aggregated 10K
- **Analyzed field sorting/aggregation failure** - Text fields don't support sorting, need `.keyword` suffix
- **Multi-region endpoint hard-coding** - Must construct URL from region config, no defaults
- **Dual-phase rotation not implemented** - Brief window where old token invalid, new not loaded yet

**Early Warning Signs:**

- fsnotify events stop after first secret rotation → subPath mount detected
- "Authentication failed" after Secret update → watch loss or rotation window issue
- Queries return 0 results when logs exist → timestamp format (seconds vs milliseconds)
- 429 errors in bursts → rate limit without backoff
- Grep logs for "token=" or "X-API-TOKEN" → secret leakage

## Implications for Roadmap

### Suggested Phase Structure

**Phase 1: Logz.io Client Foundation (2-3 days)**
- **Delivers:** HTTP client with regional endpoints, query builder, connection validation
- **Components:** RegionalClient, QueryBuilder, health checks
- **Dependencies:** None (uses existing plugin interfaces)
- **Rationale:** Prove API integration works before adding secret complexity
- **Research flag:** NO - Standard HTTP client patterns, well-documented API

**Phase 2: Secret File Management (3-4 days)**
- **Delivers:** File-based token storage, hot-reload via fsnotify, thread-safe updates
- **Components:** SecretWatcher, config parsing for `api_token_path`, RWMutex in client
- **Dependencies:** Phase 1 complete
- **Rationale:** Most complex component due to fsnotify edge cases, blocks production deployment
- **Research flag:** YES - Prototype with real Kubernetes Secret mount, test atomic write handling

**Phase 3: MCP Tools - Overview + Logs (2-3 days)**
- **Delivers:** `logzio_{name}_overview` and `logzio_{name}_logs` tools
- **Components:** Tool registration, Elasticsearch DSL aggregations, result formatting
- **Dependencies:** Phase 2 complete
- **Rationale:** High-value tools with proven patterns (mirrors VictoriaLogs)
- **Research flag:** NO - Standard Search API, well-documented aggregations

**Phase 4: MCP Tools - Patterns (3-5 days)**
- **Delivers:** `logzio_{name}_patterns` tool with native or fallback mining
- **Components:** Pattern API investigation, fallback to Drain algorithm if needed
- **Dependencies:** Phase 3 complete
- **Rationale:** Uncertain API exposure requires investigation, has fallback option
- **Research flag:** YES - Test query for pattern metadata fields, plan fallback if unavailable

**Phase 5: Helm Chart + Documentation (1-2 days)**
- **Delivers:** extraVolumes config, rotation workflow docs, troubleshooting guide
- **Components:** deployment.yaml updates, README sections, example manifests
- **Dependencies:** Phase 4 complete
- **Rationale:** Documentation should reflect actual implementation
- **Research flag:** NO - Standard Kubernetes patterns

**Total Estimate:** 11-17 days (assuming no major blockers)

### Roadmap Decision Points

**Decision Point 1: Pattern Mining Approach** (End of Phase 4)
- **If pattern metadata exposed:** Implement native pattern tool (fast, pre-computed)
- **If pattern metadata not exposed:** Fallback to Drain mining (proven, but CPU-intensive)
- **Impact:** Native patterns save 2-3 days development time, better performance

**Decision Point 2: Scroll API Implementation** (During Phase 3)
- **If MCP tools need >1,000 logs:** Implement scroll pagination with checkpoint/resume
- **If 1,000-log limit sufficient:** Defer scroll API to v1.3 (enhancement, not blocker)
- **Impact:** Scroll adds 1-2 days complexity, but differentiates from VictoriaLogs

**Decision Point 3: Multi-Token Support** (During Phase 2)
- **If Logz.io supports multiple active tokens:** Implement dual-phase rotation (zero downtime)
- **If single active token only:** Accept brief rotation window, document carefully
- **Impact:** Dual-phase rotation adds 1 day complexity, improves production safety

### Research Flags

**Phases Needing Deeper Research:**

1. **Phase 2 (Secret Management)** - HIGH PRIORITY
   - Validate fsnotify behavior with real Kubernetes Secret mount (not local file simulation)
   - Test atomic write event sequence (Remove → Create → Write)
   - Verify debounce timing (500ms may be too short for kubelet sync)
   - Confirm watch re-establishment works after IN_DELETE_SELF

2. **Phase 4 (Patterns Tool)** - MEDIUM PRIORITY
   - Query Logz.io API for pattern metadata fields (`logzio.pattern`, `pattern_id`)
   - Test aggregation on pattern field if exists
   - Benchmark Drain fallback performance (CPU/memory) if needed
   - Determine novelty detection approach (timestamp-based vs count-based)

**Phases with Standard Patterns (Skip Research):**

- Phase 1: HTTP client patterns proven in VictoriaLogs
- Phase 3: Search API and aggregations well-documented by Logz.io
- Phase 5: Standard Helm chart extraVolumes pattern

### Success Criteria by Phase

**Phase 1:**
- [ ] Client connects to all 5 regional endpoints
- [ ] Health check validates token with test query
- [ ] Query builder generates valid Elasticsearch DSL
- [ ] Unit tests cover region mapping and auth injection

**Phase 2:**
- [ ] Token loaded from file at startup
- [ ] fsnotify detects Kubernetes Secret rotation within 2 seconds
- [ ] Token updates don't block concurrent queries (RWMutex)
- [ ] Integration test simulates atomic write, verifies hot-reload

**Phase 3:**
- [ ] Overview tool returns namespace severity summary
- [ ] Logs tool supports all filter parameters (namespace, pod, container, level)
- [ ] MCP tools handle rate limits gracefully (exponential backoff)
- [ ] Leading wildcard queries rejected with helpful error

**Phase 4:**
- [ ] Pattern metadata investigation complete (native or fallback decision)
- [ ] Patterns tool returns log templates with occurrence counts
- [ ] Large dataset queries complete within 15 minutes (scroll timeout buffer)
- [ ] Fallback mining matches VictoriaLogs pattern quality if used

**Phase 5:**
- [ ] Helm chart includes extraVolumes example
- [ ] Documentation covers rotation workflow end-to-end
- [ ] Troubleshooting guide addresses top 5 pitfalls
- [ ] Example Kubernetes Secret manifest provided

## Confidence Assessment

| Area | Confidence | Source Quality | Notes |
|------|------------|---------------|-------|
| **Stack (HTTP Client)** | HIGH | Official docs, stdlib patterns | `net/http` + custom transport proven in VictoriaLogs |
| **Stack (Query Builder)** | MEDIUM-HIGH | effdsl actively maintained | Smaller community (34 stars), recommend abstraction wrapper |
| **Stack (Secret Management)** | HIGH | fsnotify proven in Spectre | Existing `integration_watcher.go` handles similar use case |
| **Features (Overview Tool)** | HIGH | Official API docs | Standard Elasticsearch aggregations, well-documented |
| **Features (Logs Tool)** | HIGH | Official API docs | Core Search API functionality |
| **Features (Patterns Tool)** | LOW | UI feature, API unclear | Pattern Engine exists, API exposure unverified |
| **Architecture (Regional Client)** | HIGH | Official region docs | 5 regions with explicit API URLs |
| **Architecture (Hot-Reload)** | MEDIUM | Community patterns | fsnotify + Kubernetes has known edge cases, needs testing |
| **Pitfalls (subPath Issue)** | HIGH | Multiple authoritative sources | Well-documented Kubernetes limitation |
| **Pitfalls (fsnotify Events)** | HIGH | fsnotify GitHub issue #372 | Known problem with atomic writes |
| **Pitfalls (Rate Limits)** | MEDIUM | Project context, not verified | 100 concurrent from context, need to test in practice |

**Overall Confidence:** HIGH for delivery, MEDIUM for timeline (patterns uncertainty adds 1-3 days variance)

### Research Gaps Requiring Validation

**During Phase 2 (Prototyping):**
1. Kubernetes field names in actual API responses (`kubernetes.namespace` vs `k8s_namespace`)
2. fsnotify event sequence with real Secret rotation (not simulated)
3. Effective debounce timing for kubelet sync period (500ms vs 2000ms)

**During Phase 4 (Pattern Investigation):**
1. Pattern metadata field names (`logzio.pattern`, `pattern_id`, or other)
2. Pattern aggregation API support (terms aggregation on pattern field)
3. Novelty detection mechanism (timestamp-based or frequency-based)
4. Scroll API behavior with large pattern datasets (20-minute timeout handling)

**Low Priority (Defer to Post-MVP):**
1. Point-in-Time API availability (newer alternative to scroll)
2. Retry-After header on 429 responses (affects backoff strategy)
3. Multiple active token support (affects dual-phase rotation)
4. Exact index naming pattern (`logzio-YYYY-MM-DD` assumed)

## Sources

### Stack Research
- [Logz.io API Documentation](https://api-docs.logz.io/docs/logz/logz-io-api/)
- [go-elasticsearch GitHub](https://github.com/elastic/go-elasticsearch)
- [effdsl GitHub](https://github.com/sdqri/effdsl)
- [fsnotify GitHub](https://github.com/fsnotify/fsnotify)
- [Kubernetes Secrets Documentation](https://kubernetes.io/docs/concepts/configuration/secret/)

### Features Research
- [Logz.io Search API](https://api-docs.logz.io/docs/logz/search/)
- [Logz.io Scroll API](https://api-docs.logz.io/docs/logz/scroll/)
- [Understanding Log Patterns](https://docs.logz.io/docs/user-guide/log-management/opensearch-dashboards/opensearch-patterns/)
- [Elasticsearch Aggregations Guide](https://logz.io/blog/elasticsearch-aggregations/)
- [Manage API Tokens](https://docs.logz.io/docs/user-guide/admin/authentication-tokens/api-tokens/)

### Architecture Research
- [Logz.io Account Regions](https://docs.logz.io/docs/user-guide/admin/hosting-regions/account-region/)
- [Kubernetes Secret Volume Mount Behavior](https://kubernetes.io/docs/concepts/configuration/secret/)
- [fsnotify with Kubernetes Secrets](https://ahmet.im/blog/kubernetes-inotify/)
- [Secrets Store CSI Driver Auto Rotation](https://secrets-store-csi-driver.sigs.k8s.io/topics/secret-auto-rotation/)
- Existing code: `internal/config/integration_watcher.go`, `internal/integration/victorialogs/`

### Pitfalls Research
- [fsnotify Issue #372: Robustly watching a single file](https://github.com/fsnotify/fsnotify/issues/372)
- [Kubernetes Secrets and Pod Restarts](https://blog.ascendingdc.com/kubernetes-secrets-and-pod-restarts)
- [Zero Downtime Secrets Rotation: 10-Step Guide](https://www.doppler.com/blog/10-step-secrets-rotation-guide)
- [Kubernetes Security Best Practices for Secrets Management](https://www.cncf.io/blog/2023/09/28/kubernetes-security-best-practices-for-kubernetes-secrets-management/)
- [Elasticsearch Query DSL Guide](https://logz.io/blog/elasticsearch-queries/)

---

*Research completed: 2026-01-22*
*Ready for roadmap: YES*
*Next step: Phase 1 implementation (Logz.io Client Foundation)*
