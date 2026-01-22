# Pitfalls Research: Logz.io Integration + Secret Management

**Domain:** Logz.io integration for Kubernetes observability with API token secret management
**Researched:** 2026-01-22
**Confidence:** MEDIUM (WebSearch verified with official docs, existing VictoriaLogs patterns examined)

## Executive Summary

Adding Logz.io integration and secret management introduces complexity across multiple dimensions: Elasticsearch DSL query limitations, multi-region configuration, rate limiting, scroll API lifecycle, fsnotify edge cases, and Kubernetes secret refresh mechanics. Critical pitfalls cluster around three areas:

1. **Elasticsearch DSL query constraints** - Leading wildcards disabled, analyzed field limitations, scroll API expiration
2. **Secret rotation mechanics** - Kubernetes subPath breaks hot-reload, fsnotify misses atomic writes, race conditions during rotation
3. **Multi-region correctness** - Hard-coded endpoints, region-specific rate limits, credential scope confusion

Many of these are subtle correctness issues that manifest in production under load, not during development. This research identifies early warning signs and prevention strategies for each.

---

## Critical Pitfalls

Mistakes that cause rewrites, data loss, or security incidents.

### Pitfall 1: Kubernetes Secret subPath Breaks Hot-Reload

**What goes wrong:**
When secrets are mounted with `subPath`, Kubernetes updates the volume symlink but NOT the actual file bind-mounted into the container. Your fsnotify watcher detects no changes, application never reloads credentials, and you get authentication failures after secret rotation.

**Why it happens:**
Kubernetes atomic writer uses symlinks for volume updates. With `subPath`, the symlink update happens at the mount point, not at the file level. The existing VictoriaLogs fsnotify watcher (`.planning/research/integration_watcher.go`) watches the file directly, which becomes stale with `subPath` mounts.

**Consequences:**
- Secret rotation causes downtime (authentication fails)
- Monitoring alerts fire during rotation windows
- Manual pod restarts required to pick up new secrets
- Violates zero-downtime rotation requirement

**Prevention:**
1. **DO NOT use `subPath` for secret mounts** - Mount entire secret volume, reference by path
2. Document in deployment YAML with explicit comment warning against subPath
3. Add integration test that verifies hot-reload with volume mount (not subPath)
4. Consider Secrets Store CSI Driver with Reloader for external vaults

**Detection:**
- Warning sign: fsnotify events stop after first secret rotation
- Warning sign: Pod logs show "authentication failed" after secret update
- Test: Update secret, watch for fsnotify event within 60s (kubelet sync period)

**References:**
- [Kubernetes Secrets and Pod Restarts](https://blog.ascendingdc.com/kubernetes-secrets-and-pod-restarts)
- [Known Limitations - Secrets Store CSI Driver](https://secrets-store-csi-driver.sigs.k8s.io/known-limitations)
- [K8s Deployment Automatic Rollout Restart](https://igboie.medium.com/k8s-deployment-automatic-rollout-restart-when-referenced-secrets-and-configmaps-are-updated-0c74c85c1b4a)

**Which phase:**
Phase 2 (Logz.io API Client) - Must establish secret loading pattern before MCP tools implementation

---

### Pitfall 2: Atomic Editor Saves Cause fsnotify Watch Loss

**What goes wrong:**
Text editors (vim, VSCode, kubectl) use atomic writes: write temp file → rename to target. fsnotify watches the inode, which changes on rename. Watch is automatically removed, fsnotify stops receiving events, and config changes are silently ignored.

**Why it happens:**
The existing `integration_watcher.go` handles this with Remove/Rename event re-watching (lines 140-148), BUT there's a 50ms sleep gap where the file might be written and you miss the event. Kubernetes Secret volume updates are atomic writes. VSCode triggers 5 events per save, creating race conditions in the debounce logic.

**Consequences:**
- Secret rotation silently fails (no reload triggered)
- Integration continues using expired credentials until health check fails
- Gap between rotation and detection creates security window
- Difficult to debug (no error, just missing events)

**Prevention:**
1. **Verify existing re-watch logic handles Kubernetes volume updates** - Test with actual Secret mount
2. **Increase re-watch delay from 50ms to 200ms** for Kubernetes atomic writes (slower than editor saves)
3. **Watch parent directory, not file** - Recommended by fsnotify docs (avoids inode change problem)
4. **Add file existence check after re-watch** - Verify file exists before continuing
5. **Log all watch removals and re-additions** - Make missing events visible

**Detection:**
- Warning sign: Watcher logs show Remove/Rename events but no subsequent reload
- Warning sign: Time gap between secret update and reload > 500ms
- Test: Simulate atomic write (write temp → rename), verify fsnotify event within 200ms

**References:**
- [fsnotify Issue #372: Robustly watching a single file](https://github.com/fsnotify/fsnotify/issues/372)
- [Building a cross-platform File Watcher in Go](https://dev.to/asoseil/building-a-cross-platform-file-watcher-in-go-what-i-learned-from-scratch-1dbj)

**Which phase:**
Phase 2 (Logz.io API Client) - Critical for secret hot-reload, blocks production deployment

---

### Pitfall 3: Leading Wildcard Queries Disabled by Logz.io

**What goes wrong:**
Logz.io API enforces `allow_leading_wildcard: false` on query_string queries. User tries query like `*-service` to find all services, gets error. This is NOT documented clearly in their API docs, only buried in their UI help.

**Why it happens:**
Leading wildcards require scanning every term in the index (extremely expensive). Logz.io disables this for performance/cost reasons. Standard Elasticsearch clients default to allowing it, creating mismatch with Logz.io API expectations.

**Consequences:**
- MCP tool queries fail with cryptic errors
- Users familiar with Elasticsearch expect this to work
- Workarounds (use filters, analyzed fields) are non-obvious
- Degrades user experience of AI assistant tools

**Prevention:**
1. **Document leading wildcard limitation prominently** in MCP tool descriptions
2. **Validate queries before sending to API** - Reject leading wildcards with helpful error
3. **Suggest alternatives in error message** - "Use field filters instead of leading wildcards"
4. **Pre-query field mapping check** - Identify analyzed fields that support tokenized search
5. **Add query builder helper** that constructs valid Logz.io queries

**Detection:**
- Warning sign: API returns 400 errors on wildcard queries
- Test: Attempt query with leading wildcard, verify helpful error message

**References:**
- [Logz.io Wildcard Searches](https://docs.logz.io/kibana/wildcards/)
- [Logz.io Search Logs API](https://api-docs.logz.io/docs/logz/search/)
- [Elasticsearch Query DSL Guide](https://logz.io/blog/elasticsearch-queries/)

**Which phase:**
Phase 3 (MCP Tool Implementation) - Query validation layer before API client calls

---

### Pitfall 4: Scroll API Context Expiration After 20 Minutes

**What goes wrong:**
Logz.io scroll API contexts expire after 20 minutes. If MCP tool takes >20min to process results (e.g., pattern mining large dataset), scroll_id becomes invalid. Subsequent scroll requests fail with "expired scroll ID" error, and you lose your pagination state.

**Why it happens:**
Scroll contexts hold cluster resources (search state, results cache). 20-minute timeout is aggressive compared to Elasticsearch default (1 minute, but adjustable). The project context mentions this limit but doesn't explain implications for long-running operations.

**Consequences:**
- Pattern mining tool fails mid-operation on large namespaces
- Partial results without clear indication of incompleteness
- User retries query, hits rate limit, degrades service
- Cannot paginate through large result sets (>10,000 logs)

**Prevention:**
1. **Use scroll API only for result sets needing >1,000 logs** (Logz.io aggregation limit)
2. **Set aggressive internal timeout (15 min)** - Leave 5min buffer before API expiration
3. **Implement checkpoint/resume** - Save last processed position, allow restart
4. **Consider Point-in-Time API** if Logz.io supports it (newer alternative to scroll)
5. **Stream results to caller incrementally** - Don't buffer entire dataset in memory
6. **Clear scroll context after use** - Free resources promptly

**Detection:**
- Warning sign: Long-running queries (>10min) fail with scroll errors
- Warning sign: Memory usage grows unbounded during pattern mining
- Test: Query with scroll, sleep 21 minutes, attempt next page (expect error handling)

**References:**
- [Elasticsearch Scroll API](https://www.elastic.co/guide/en/elasticsearch/reference/current/scroll-api.html)
- [Elasticsearch Error: Cannot retrieve scroll context](https://pulse.support/kb/elasticsearch-cannot-retrieve-scroll-context-expired-scroll-id)
- [Elasticsearch Pagination by Scroll API](https://medium.com/eatclub-tech/elasticsearch-pagination-by-scroll-api-68d36b8f4972)

**Which phase:**
Phase 3 (MCP Tool Implementation) - Affects patterns tool querying large datasets

---

### Pitfall 5: Hard-Coded API Region Endpoint

**What goes wrong:**
Logz.io uses different API endpoints per region (us-east-1, eu-central-1, ap-southeast-2, etc.). If you hard-code the endpoint URL in config or default to US region, users in other regions get authentication failures or routing errors.

**Why it happens:**
Multi-region architecture is common in observability SaaS, but not obvious to new integrators. The project context mentions "multi-region: different API endpoints" but doesn't specify how to determine correct endpoint. Users expect a single API domain.

**Consequences:**
- Authentication fails for non-US users (wrong region, token not valid)
- Higher latency for users far from hard-coded region
- Data sovereignty violations (EU data routed through US)
- Support burden ("integration doesn't work for me")

**Prevention:**
1. **Require region as explicit config parameter** - No defaults, force user to specify
2. **Validate region against known list** - Reject invalid regions early with helpful message
3. **Construct endpoint URL from region** - `https://api-{region}.logz.io` pattern
4. **Document region discovery process** - Link to Logz.io docs showing how to find your region
5. **Add region to MCP tool descriptions** - Make it visible which instance serves which region

**Detection:**
- Warning sign: Authentication works in staging but fails in production (different regions)
- Warning sign: High latency in API calls (cross-region routing)
- Test: Configure integration for each known region, verify correct endpoint construction

**References:**
- [Azure APIM Multi-Region Concepts](https://github.com/MicrosoftDocs/azure-docs/blob/main/includes/api-management-multi-region-concepts.md)
- [Multi-Region API Gateway Deployment Guide](https://www.eyer.ai/blog/multi-region-api-gateway-deployment-guide/)

**Which phase:**
Phase 1 (Planning & Research) - Architecture decision before implementation starts

---

### Pitfall 6: Secret Value Logging During Debug

**What goes wrong:**
During development/debugging, developers add log statements that inadvertently log secret values (API tokens, connection strings with passwords). These end up in pod logs, aggregated into VictoriaLogs/Logz.io, and become searchable by anyone with log access.

**Why it happens:**
Secrets are just strings, no type-level protection. Generic error messages include full context ("failed to connect with token=abc123..."). Structured logging makes it easy to log entire config objects. Existing VictoriaLogs integration has generic logging, no secret scrubbing.

**Consequences:**
- Credential leakage to logs (security incident)
- Compliance violation (secrets in plaintext in log storage)
- Difficult to detect/remediate (secrets may be in historical logs)
- Incident response requires log deletion (may violate retention policies)

**Prevention:**
1. **Mark secret fields with struct tags** - `json:"-" yaml:"api_token"` prevents marshaling
2. **Implement String() method for config** - Return redacted version for logging
3. **Log config validation only** - Log "token present: yes" not token value
4. **Add linter rule** - Detect `log.*config` patterns in code review
5. **Sanitize error messages** - Wrap API errors, strip credentials from strings
6. **Log audit** - Search existing logs for exposed tokens before production

**Detection:**
- Warning sign: Log entries contain "token=" or "api_key=" followed by values
- Test: Grep application logs for known test secret values
- Test: Log config object, verify secrets are redacted

**References:**
- [Kubernetes Secrets Management Best Practices](https://www.cncf.io/blog/2023/09/28/kubernetes-security-best-practices-for-kubernetes-secrets-management/)
- [Kubernetes Secrets: Best Practices](https://blog.gitguardian.com/how-to-handle-secrets-in-kubernetes/)

**Which phase:**
Phase 2 (Logz.io API Client) - Establish logging patterns before building MCP tools

---

## Moderate Pitfalls

Mistakes that cause delays, technical debt, or intermittent issues.

### Pitfall 7: Rate Limit Handling Without Exponential Backoff

**What goes wrong:**
Logz.io enforces 100 concurrent requests per account. Without exponential backoff, multiple MCP tools hitting rate limit will retry immediately, amplifying the problem. Fixed-delay retries create thundering herd when rate limit resets.

**Why it happens:**
Rate limiting is account-wide, not per-instance. Multiple users running Claude Code simultaneously share the same rate limit. Naive retry logic uses fixed delays or immediate retries. HTTP 429 responses don't include Retry-After header (not documented).

**Consequences:**
- Request storms during rate limit periods
- Degraded service for all users sharing account
- MCP tools time out waiting for responses
- Support tickets for "integration randomly fails"

**Prevention:**
1. **Implement exponential backoff with jitter** - Start at 1s, double each retry, max 32s
2. **Track rate limit globally per instance** - Share state across tool invocations
3. **Fail fast after 3 retries** - Return clear error to user, don't hang
4. **Add rate limit metrics** - Expose `logzio_rate_limit_hits_total` counter
5. **Document concurrent request limit** in integration configuration
6. **Consider request queuing** - Serialize requests to stay under limit

**Detection:**
- Warning sign: Bursts of 429 errors in logs
- Warning sign: Request latency spikes during high usage
- Test: Send 101 concurrent requests, verify graceful handling

**References:**
- [Logz.io Metrics Throttling](https://docs.logz.io/docs/user-guide/infrastructure-monitoring/metric-throttling/)
- [API Rate Limiting Strategies](https://nhonvo.github.io/posts/2025-09-07-api-rate-limiting-and-throttling-strategies/)
- [Exponential Backoff Strategy](https://substack.thewebscraping.club/p/rate-limit-scraping-exponential-backoff)

**Which phase:**
Phase 2 (Logz.io API Client) - HTTP client configuration with retry middleware

---

### Pitfall 8: Result Limit Confusion (1,000 vs 10,000)

**What goes wrong:**
Logz.io has TWO result limits: 1,000 for aggregated results, 10,000 for non-aggregated. MCP tool tries to fetch 5,000 log messages (non-aggregated), expects it to work based on 10,000 limit, but uses aggregation query by accident and gets 1,000-row limit error.

**Why it happens:**
The distinction between aggregated vs non-aggregated is subtle. Aggregation happens implicitly when grouping by fields. Project context mentions both limits but doesn't explain which queries use which. Easy to hit wrong limit during development.

**Consequences:**
- Pattern mining tool silently truncates results at 1,000 (uses aggregation)
- Raw logs tool works fine (non-aggregated, 10,000 limit)
- Inconsistent behavior across MCP tools confuses users
- Testing with small datasets misses the problem

**Prevention:**
1. **Document which MCP tools hit which limit** in tool descriptions
2. **Validate limit parameter against query type** - Reject invalid combinations early
3. **Warn user when approaching limit** - "Returning 1,000 of 50,000 matching logs"
4. **Use scroll API for large result sets** - Avoid hitting limits entirely
5. **Test with large datasets** - Ensure limits are enforced correctly

**Detection:**
- Warning sign: Tool returns exactly 1,000 or 10,000 results (suspicious)
- Test: Query returning >1,000 aggregated results, verify error handling

**References:**
- Project context: "Result limits: 1,000 aggregated, 10,000 non-aggregated"
- [Elasticsearch Query DSL Guide](https://logz.io/blog/elasticsearch-queries/)

**Which phase:**
Phase 3 (MCP Tool Implementation) - Query construction and result handling

---

### Pitfall 9: Analyzed Field Sorting/Aggregation Failure

**What goes wrong:**
Elasticsearch analyzed fields (like `message`) are tokenized for full-text search. You cannot sort or aggregate on them. MCP tool tries `"sort": [{"message": "asc"}]` and gets cryptic error about "fielddata disabled on text fields."

**Why it happens:**
Field mapping determines whether field is analyzed (full-text) or keyword (exact match). Logz.io auto-maps many fields, but behavior may differ from self-hosted Elasticsearch. Sorting/aggregation requires keyword fields. Error messages are Elasticsearch internals, not user-friendly.

**Consequences:**
- MCP tools fail with confusing errors
- Query construction logic becomes brittle (needs field mapping knowledge)
- Different behavior between environments (mapping differences)

**Prevention:**
1. **Fetch field mappings during integration Start()** - Cache them
2. **Validate sort/aggregation fields against mappings** - Only allow keyword fields
3. **Provide user-friendly error** - "Cannot sort on 'message' (text field). Use 'message.keyword' instead."
4. **Document common field suffixes** - `.keyword` for exact match, base field for search
5. **Add field mapping explorer tool** - Let users discover available fields

**Detection:**
- Warning sign: Queries fail with "fielddata" or "aggregation not supported" errors
- Test: Attempt sort on known text field, verify helpful error message

**References:**
- [Elasticsearch Query DSL Guide](https://logz.io/blog/elasticsearch-queries/)
- [Understanding Common Elasticsearch Query Errors](https://moldstud.com/articles/p-understanding-common-causes-of-elasticsearch-query-errors-and-how-to-effectively-resolve-them)

**Which phase:**
Phase 3 (MCP Tool Implementation) - Query builder needs field mapping awareness

---

### Pitfall 10: fsnotify File Descriptor Exhaustion on macOS

**What goes wrong:**
On macOS, fsnotify uses kqueue, which requires one file descriptor per watched file. If you watch many integration config files (or watch a directory with many files), you hit the OS limit (default 256) and get "too many open files" error.

**Why it happens:**
macOS kqueue is more resource-intensive than Linux inotify. The existing integration watcher watches a single file, but if deployment pattern involves watching multiple config files (one per integration instance), the problem scales. This is a platform-specific behavior.

**Consequences:**
- Watcher fails to start on macOS (development machines)
- Error is cryptic ("too many open files" doesn't mention fsnotify)
- Works fine on Linux (CI/production), fails on developer laptops
- Blocks local testing of multi-instance scenarios

**Prevention:**
1. **Watch parent directory, not individual files** - Single file descriptor for entire directory
2. **Filter events by filename** - Ignore irrelevant files in directory
3. **Document macOS ulimit requirement** - `ulimit -n 4096` in setup docs
4. **Add startup check** - Verify file descriptor limit is sufficient
5. **Log clear error** - "fsnotify failed: increase file descriptor limit (ulimit -n 4096)"

**Detection:**
- Warning sign: "too many open files" error during watcher startup
- Warning sign: Watcher works on Linux CI, fails on macOS laptops
- Test: Create 300 watched files, verify watcher starts successfully (or errors helpfully)

**References:**
- [fsnotify GitHub README](https://github.com/fsnotify/fsnotify)
- [Building a cross-platform File Watcher in Go](https://dev.to/asoseil/building-a-cross-platform-file-watcher-in-go-what-i-learned-from-scratch-1dbj)

**Which phase:**
Phase 2 (Logz.io API Client) - File watching infrastructure setup

---

### Pitfall 11: Dual-Phase Secret Rotation Not Implemented

**What goes wrong:**
Old secret is invalidated immediately when new secret is generated. During rotation, there's a window where application has old secret cached but it's already invalid. Requests fail with 401 errors until hot-reload completes.

**Why it happens:**
Simple rotation (generate new → invalidate old) assumes instant propagation. File-based hot-reload takes time (fsnotify event → reload → HTTP client update). Kubernetes kubelet syncs volumes every 60s by default. Secret provider may not support overlapping active versions.

**Consequences:**
- Downtime during secret rotation (seconds to minutes)
- 401 errors visible to users during rotation window
- Rotation becomes risky, teams avoid doing it regularly
- Security posture degrades (stale secrets stay active)

**Prevention:**
1. **Use dual-phase rotation** - Generate new, wait for propagation, invalidate old
2. **Support multiple active tokens** - Application accepts both old and new during transition
3. **Implement grace period** - Keep old secret valid for 5 minutes after new one deployed
4. **Monitor rotation health** - Alert if 401 errors spike during rotation
5. **Document rotation procedure** - Step-by-step with verification checkpoints
6. **Test rotation in staging** - Verify zero-downtime before production

**Detection:**
- Warning sign: 401 errors during known rotation windows
- Test: Rotate secret while load testing, verify no 401s

**References:**
- [Zero Downtime Secrets Rotation: 10-Step Guide](https://www.doppler.com/blog/10-step-secrets-rotation-guide)
- [AWS: Rotate database credentials without restarting containers](https://docs.aws.amazon.com/prescriptive-guidance/latest/patterns/rotate-database-credentials-without-restarting-containers.html)
- [Secrets rotation strategies for long-lived services](https://technori.com/news/secrets-rotation-long-lived-services/)

**Which phase:**
Phase 2 (Logz.io API Client) - Client initialization and credential refresh logic

---

## Minor Pitfalls

Mistakes that cause inconvenience but are easily fixable.

### Pitfall 12: Time Range Default Confusion (Seconds vs Milliseconds)

**What goes wrong:**
Logz.io API accepts Unix timestamps in milliseconds. Developer defaults to Go's `time.Now().Unix()` (seconds), queries return no results. Error is silent (valid query, just wrong time range).

**Why it happens:**
Go standard library uses seconds for Unix timestamps. JavaScript uses milliseconds. Elasticsearch can accept both but prefers milliseconds. Easy to forget conversion. Project context doesn't specify which format to use.

**Consequences:**
- MCP tools return empty results for valid queries
- Confusing user experience ("I know there are logs in that timeframe")
- Hard to debug (no error, just wrong results)

**Prevention:**
1. **Use milliseconds consistently** - Convert at input boundary
2. **Add unit tests** - Verify timestamp format in queries
3. **Validate time ranges** - Reject timestamps in the future or too far past
4. **Log effective time range** - "Querying logs from 2024-01-20T10:00:00Z to 2024-01-20T11:00:00Z"
5. **Accept both formats, normalize internally** - Check magnitude, convert if needed

**Detection:**
- Warning sign: Queries return 0 results when logs exist
- Test: Query with known log entry timestamp, verify it's found

**References:**
- [Logz.io Search API](https://api-docs.logz.io/docs/logz/search/)

**Which phase:**
Phase 3 (MCP Tool Implementation) - Time range parameter handling

---

### Pitfall 13: Integration Name Used Directly in Tool Names

**What goes wrong:**
If integration name contains spaces or special characters (e.g., "Logz.io Production"), tool name becomes `logzio_Logz.io Production_overview` (invalid MCP tool name). Registration fails.

**Why it happens:**
The existing VictoriaLogs integration uses name directly in tool name construction: `fmt.Sprintf("victorialogs_%s_overview", v.name)`. Assumes name is kebab-case or snake_case. No validation of integration name format.

**Consequences:**
- Tool registration fails silently or with cryptic error
- Integration starts but MCP tools don't work
- Hard to debug (error is far from name definition)

**Prevention:**
1. **Sanitize name for tool construction** - Replace spaces with underscores, lowercase
2. **Validate name at config load** - Reject names with special characters
3. **Document name format requirement** - "Name must be lowercase alphanumeric with hyphens"
4. **Add test case** - Verify tool registration with various name formats
5. **Log generated tool names** - Make it visible what names were registered

**Detection:**
- Warning sign: Integration starts but `mcp tools list` doesn't show expected tools
- Test: Configure integration with name containing spaces, verify error or sanitization

**References:**
- Existing code: `/home/moritz/dev/spectre-via-ssh/internal/integration/victorialogs/victorialogs.go` line 163

**Which phase:**
Phase 3 (MCP Tool Implementation) - Tool registration logic

---

### Pitfall 14: Debounce Too Short for Kubernetes Secret Updates

**What goes wrong:**
Integration watcher uses 500ms debounce (existing code line 59). Kubernetes Secret volume updates trigger multiple events (Remove → Create → Write) within 1 second as kubelet syncs. Reload triggers multiple times, causing unnecessary restarts.

**Why it happens:**
Kubelet sync isn't atomic from fsnotify's perspective. Atomic writer updates symlink, then rewrites target file. 500ms debounce is tuned for editor saves (many fast events), not Kubernetes volume updates (slower but still multiple events).

**Consequences:**
- Secret reload triggers 2-3 times for single update
- Unnecessary churn in HTTP client reconnection
- Metrics show inflated reload counts
- Log noise

**Prevention:**
1. **Increase debounce to 2 seconds** for Kubernetes environments
2. **Make debounce configurable** - Different values for dev (editor) vs prod (K8s)
3. **Add reload deduplication** - Track content hash, skip if unchanged
4. **Log debounce behavior** - "Received 3 events, coalesced into 1 reload"
5. **Test with real Kubernetes Secret updates** - Not just local file edits

**Detection:**
- Warning sign: Multiple reload log entries within seconds
- Test: Update secret once, verify exactly one reload (after debounce period)

**References:**
- Existing code: `/home/moritz/dev/spectre-via-ssh/internal/config/integration_watcher.go` line 59
- [fsnotify Issue #372](https://github.com/fsnotify/fsnotify/issues/372)

**Which phase:**
Phase 2 (Logz.io API Client) - Watcher configuration tuning

---

### Pitfall 15: No Index Specification (Defaults May Surprise)

**What goes wrong:**
Logz.io search API documentation says "two consecutive indexes only (today + yesterday default)." If user expects to query logs from 3 days ago, they get empty results. API silently ignores logs outside the default index range.

**Why it happens:**
Elasticsearch uses date-based index rotation. Logz.io default is recent 2 days for performance. Querying older logs requires explicit index specification. This is mentioned in project context but not enforced in API client.

**Consequences:**
- Historical log queries return incomplete results
- Users don't understand why old logs aren't visible
- Workaround (specify indexes) is not discoverable

**Prevention:**
1. **Validate time range against index coverage** - Warn if querying >2 days
2. **Auto-calculate index names from time range** - `logzio-YYYY-MM-DD` pattern
3. **Document index limitation prominently** - In MCP tool descriptions
4. **Add index parameter to MCP tools** - Advanced users can override
5. **Log effective index range** - "Querying indexes: logzio-2024-01-20, logzio-2024-01-21"

**Detection:**
- Warning sign: Historical queries (>2 days ago) return 0 results
- Test: Query with 3-day-old timestamp, verify warning or index specification

**References:**
- Project context: "Two consecutive indexes only (today + yesterday default)"
- [Logz.io Search API](https://api-docs.logz.io/docs/logz/search/)

**Which phase:**
Phase 3 (MCP Tool Implementation) - Query construction with index awareness

---

## Secret Management Pitfalls

Security-specific issues to avoid.

### Pitfall 16: Secret Leakage in Error Messages

**What goes wrong:**
HTTP client error includes full request details: `GET https://api.logz.io/logs?X-API-TOKEN=abc123...`. Error is logged, bubbles up to MCP tool response, ends up in Claude Code conversation history.

**Why it happens:**
Standard HTTP libraries include full request in errors for debugging. Headers contain credentials. Error wrapping preserves original error. No sanitization layer between HTTP client and caller.

**Consequences:**
- API token visible in application logs
- Token visible in MCP tool error responses
- Token may be transmitted to Anthropic via Claude Code (conversation history)
- Credential rotation required if leak detected

**Prevention:**
1. **Implement HTTP client error wrapper** - Strip `X-API-TOKEN` header from errors
2. **Redact credentials in request logs** - `X-API-TOKEN: [REDACTED]`
3. **Never log full HTTP requests** - Log method + path only, not headers
4. **Sanitize errors before MCP response** - Generic "authentication failed" message
5. **Add security test** - Simulate auth failure, verify token not in error

**Detection:**
- Warning sign: Grep logs for "X-API-TOKEN" finds matches
- Test: Trigger auth error, verify token not in error message

**References:**
- [Kubernetes Secrets Management Best Practices](https://www.cncf.io/blog/2023/09/28/kubernetes-security-best-practices-for-kubernetes-secrets-management/)

**Which phase:**
Phase 2 (Logz.io API Client) - HTTP client error handling

---

### Pitfall 17: Base64 Encoding Is Not Encryption

**What goes wrong:**
Kubernetes Secrets are base64-encoded, not encrypted. Developer assumes this provides security, stores API token in Secret without enabling encryption-at-rest in etcd. Anyone with etcd access can decode secrets.

**Why it happens:**
Base64 looks like encryption (random characters). Kubernetes documentation mentions "Secrets" which implies security. Encryption-at-rest is not enabled by default. This is a Kubernetes platform issue, but affects integration security.

**Consequences:**
- Secrets vulnerable to etcd compromise
- Compliance violations (secrets stored in plaintext)
- Cluster-wide security issue (affects all secrets)

**Prevention:**
1. **Document encryption-at-rest requirement** - In deployment docs
2. **Recommend External Secrets Operator** - Fetch from Vault/AWS Secrets Manager
3. **Verify encryption during setup** - Check etcd encryption config
4. **Use least-privilege RBAC** - Limit who can read Secrets
5. **Consider sealed secrets** - Encrypt before committing to Git

**Detection:**
- Check: `kubectl describe secret` shows base64 data (not encrypted)
- Check: etcd encryption provider config exists
- Audit: Review who has `get secrets` RBAC permission

**References:**
- [Kubernetes Secrets Good Practices](https://kubernetes.io/docs/concepts/security/secrets-good-practices/)
- [Kubernetes Secrets Management Limitations](https://www.groundcover.com/blog/kubernetes-secret-management)

**Which phase:**
Phase 1 (Planning & Research) - Security architecture decision, documented before implementation

---

### Pitfall 18: Secret Rotation Without Monitoring

**What goes wrong:**
Secret is rotated (new token deployed), but no monitoring verifies that rotation succeeded. Old token expired, new token has typo, all API calls fail silently until next health check (could be minutes).

**Why it happens:**
Rotation is treated as deployment task, not operational concern. No metrics track rotation events. Health checks run infrequently (default 30s-60s). Gap between rotation and detection creates downtime.

**Consequences:**
- Undetected authentication failures during rotation
- Users experience intermittent errors
- Difficult to correlate errors with rotation events

**Prevention:**
1. **Add rotation event metric** - `logzio_secret_reload_total{status="success|failure"}`
2. **Trigger health check immediately after reload** - Don't wait for next periodic check
3. **Alert on reload failures** - Prometheus alert: `rate(logzio_secret_reload_total{status="failure"}) > 0`
4. **Log before/after token prefix** - "Reloaded token: old=abc123..., new=def456..." (first 6 chars only)
5. **Test connection after reload** - Verify new credentials work before considering reload successful

**Detection:**
- Warning sign: No metrics for secret reload events
- Test: Rotate to invalid token, verify immediate health check failure

**References:**
- [Zero Downtime Secrets Rotation](https://www.doppler.com/blog/10-step-secrets-rotation-guide)

**Which phase:**
Phase 2 (Logz.io API Client) - Metrics and health check integration

---

## Phase-Specific Warnings

Recommendations for which phases need deeper investigation or risk mitigation.

| Phase | Likely Pitfall | Mitigation Strategy |
|-------|---------------|---------------------|
| **Phase 1: Planning** | Multi-region config complexity | Research region discovery, document region parameter requirement explicitly |
| **Phase 2: API Client** | Kubernetes Secret subPath + fsnotify atomic writes | Prototype secret hot-reload early, test with real K8s Secret volume (not local file) |
| **Phase 2: API Client** | Secret leakage in logs | Implement sanitization/redaction before any MCP tool integration |
| **Phase 2: API Client** | Rate limiting without backoff | Add retry middleware to HTTP client with exponential backoff + jitter |
| **Phase 3: MCP Tools** | Leading wildcard queries fail | Add query validator that rejects leading wildcards with helpful error |
| **Phase 3: MCP Tools** | Scroll API expiration on large datasets | Set 15min timeout for pattern mining, implement checkpoint/resume |
| **Phase 3: MCP Tools** | Result limit confusion (1K vs 10K) | Document which tools use aggregation, validate limits against query type |
| **Phase 4: Testing** | Integration tests miss K8s-specific issues | Add E2E test with real Kubernetes Secret mount (not mocked file) |
| **Phase 4: Testing** | Rate limit testing requires shared state | Mock rate limiter in tests, verify backoff behavior without hitting real API |

---

## Open Questions for Further Research

1. **Does Logz.io API return Retry-After header on 429 responses?** - Not documented, need to test
2. **What's the exact index naming pattern?** - `logzio-YYYY-MM-DD` is assumed, need to verify
3. **Can we use Point-in-Time API instead of scroll?** - Newer Elasticsearch feature, may not be available
4. **Does Logz.io support multiple active API tokens?** - Critical for dual-phase rotation
5. **What's the actual kubelet Secret sync period?** - Default is 60s, but can be configured
6. **How to discover user's Logz.io region programmatically?** - May need to parse account details

---

## Confidence Assessment

| Area | Confidence | Source | Notes |
|------|-----------|--------|-------|
| **Elasticsearch DSL limitations** | HIGH | Official Logz.io docs, Elasticsearch reference | Leading wildcard restriction confirmed in docs |
| **Kubernetes Secret mechanics** | HIGH | Kubernetes docs, community blog posts | subPath limitation well-documented |
| **fsnotify edge cases** | HIGH | fsnotify GitHub issues, community experiences | Atomic write problem is known issue #372 |
| **Scroll API behavior** | MEDIUM | Elasticsearch docs, Stack Overflow | 20min timeout from project context, not directly verified |
| **Rate limiting details** | LOW | Logz.io docs (metrics only, not logs API) | 100 concurrent requests from project context, needs verification |
| **Multi-region configuration** | MEDIUM | Generic multi-region patterns, not Logz.io-specific | Need to verify exact endpoint format |
| **Secret rotation patterns** | HIGH | Multiple authoritative sources (AWS, HashiCorp, Doppler) | Dual-phase rotation well-established pattern |
| **Result limits** | MEDIUM | Project context states 1K/10K | Need to verify if aggregation detection is automatic |

---

## Summary: Top 5 Pitfalls to Address First

1. **Kubernetes Secret subPath breaks hot-reload** - Critical for production deployments, affects security posture
2. **fsnotify atomic write edge cases** - Silent failures hard to debug, blocks reliable secret rotation
3. **Leading wildcard queries disabled** - User-facing errors, degrades MCP tool experience
4. **Secret value leakage in logs/errors** - Security incident risk, compliance violation
5. **Multi-region endpoint hard-coding** - Breaks integration for non-US users, support burden

These five pitfalls represent the highest risk and should be addressed in Phase 2 (API Client) before implementing MCP tools in Phase 3.

---

## Sources

**Logz.io-Specific:**
- [Logz.io Wildcard Searches](https://docs.logz.io/kibana/wildcards/)
- [Logz.io Search Logs API](https://api-docs.logz.io/docs/logz/search/)
- [Elasticsearch Query DSL Guide by Logz.io](https://logz.io/blog/elasticsearch-queries/)
- [Logz.io Metrics Throttling](https://docs.logz.io/docs/user-guide/infrastructure-monitoring/metric-throttling/)

**Elasticsearch DSL:**
- [Elasticsearch Query DSL](https://www.elastic.co/docs/explore-analyze/query-filter/languages/querydsl)
- [Query string query Reference](https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-query-string-query.html)
- [Understanding Elasticsearch Query Errors](https://moldstud.com/articles/p-understanding-common-causes-of-elasticsearch-query-errors-and-how-to-effectively-resolve-them)
- [Elasticsearch Scroll API](https://www.elastic.co/guide/en/elasticsearch/reference/current/scroll-api.html)
- [Elasticsearch Error: Expired Scroll ID](https://pulse.support/kb/elasticsearch-cannot-retrieve-scroll-context-expired-scroll-id)

**Kubernetes Secrets:**
- [Kubernetes Secrets Good Practices](https://kubernetes.io/docs/concepts/security/secrets-good-practices/)
- [Secrets Management in Kubernetes Best Practices](https://dev.to/rubixkube/secrets-management-in-kubernetes-best-practices-for-security-1df0)
- [Kubernetes Secret Management Limitations](https://www.groundcover.com/blog/kubernetes-secret-management)
- [Kubernetes Secrets: Best Practices (GitGuardian)](https://blog.gitguardian.com/how-to-handle-secrets-in-kubernetes/)
- [Kubernetes CNCF: Secrets Management Best Practices](https://www.cncf.io/blog/2023/09/28/kubernetes-security-best-practices-for-kubernetes-secrets-management/)
- [Kubernetes Secrets and Pod Restarts](https://blog.ascendingdc.com/kubernetes-secrets-and-pod-restarts)
- [K8s Deployment Automatic Rollout Restart](https://igboie.medium.com/k8s-deployment-automatic-rollout-restart-when-referenced-secrets-and-configmaps-are-updated-0c74c85c1b4a)
- [Secrets Store CSI Driver Known Limitations](https://secrets-store-csi-driver.sigs.k8s.io/known-limitations)

**Secret Rotation:**
- [Zero Downtime Secrets Rotation: 10-Step Guide (Doppler)](https://www.doppler.com/blog/10-step-secrets-rotation-guide)
- [AWS: Rotate database credentials without restarting containers](https://docs.aws.amazon.com/prescriptive-guidance/latest/patterns/rotate-database-credentials-without-restarting-containers.html)
- [Secrets rotation strategies for long-lived services](https://technori.com/news/secrets-rotation-long-lived-services/)
- [Orchestrating Automated Secret Rotation](https://medium.com/@eren.c.uysal/orchestrating-automated-secret-rotation-for-custom-applications-67d0869d6c5f)
- [HashiCorp: Automated secrets rotation](https://developer.hashicorp.com/hcp/docs/vault-secrets/auto-rotation)

**fsnotify:**
- [fsnotify Issue #372: Robustly watching a single file](https://github.com/fsnotify/fsnotify/issues/372)
- [fsnotify GitHub Repository](https://github.com/fsnotify/fsnotify)
- [Building a cross-platform File Watcher in Go](https://dev.to/asoseil/building-a-cross-platform-file-watcher-in-go-what-i-learned-from-scratch-1dbj)

**Rate Limiting:**
- [API Rate Limiting and Throttling Strategies](https://nhonvo.github.io/posts/2025-09-07-api-rate-limiting-and-throttling-strategies/)
- [Exponential Backoff Strategy](https://substack.thewebscraping.club/p/rate-limit-scraping-exponential-backoff)
- [API Rate Limits Best Practices 2025](https://orq.ai/blog/api-rate-limit)

**Multi-Region:**
- [Azure APIM Multi-Region Concepts](https://github.com/MicrosoftDocs/azure-docs/blob/main/includes/api-management-multi-region-concepts.md)
- [Multi-Region API Gateway Deployment Guide](https://www.eyer.ai/blog/multi-region-api-gateway-deployment-guide/)
- [Google Cloud: Multi-region deployments for API Gateway](https://cloud.google.com/api-gateway/docs/multi-region-deployment)
