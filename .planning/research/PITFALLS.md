# Domain Pitfalls: MCP Plugin System + VictoriaLogs Integration

**Domain:** MCP server plugin architecture, log template mining, config hot-reload, progressive disclosure
**Researched:** 2026-01-20
**Confidence:** MEDIUM (verified with official sources and production reports where available)

## Executive Summary

This research identifies critical pitfalls across four domains: Go plugin systems, log template mining, configuration hot-reload, and progressive disclosure UIs. The most severe risks involve Go's stdlib plugin versioning constraints, template mining instability with variable-starting logs, race conditions in hot-reload without atomic updates, and state loss during progressive disclosure navigation.

**Key finding:** The stdlib `plugin` package has severe production limitations. HashiCorp's go-plugin (RPC-based) is the production-proven alternative, used by Terraform, Vault, Nomad, and Packer for 4+ years.

---

## Critical Pitfalls

Mistakes that cause rewrites or major production issues.

### CRITICAL-1: Go Stdlib Plugin Versioning Hell

**What goes wrong:**
Using Go's stdlib `plugin` package creates brittle deployment where plugins crash with "plugin was built with a different version of package" errors. All plugins and the host must be built with:
- Exact same Go toolchain version
- Exact same dependency versions (including transitive deps)
- Exact same GOPATH
- Exact same build flags (`-trimpath`, `-buildmode=plugin`, etc.)

**Why it happens:**
Go's plugin system loads `.so` files into the same process space. The runtime performs strict version checking on all shared packages. Even patch version differences in dependencies cause panics.

**Consequences:**
- Plugin updates require rebuilding ALL plugins and host
- Cannot distribute third-party plugins (users can't build with your exact toolchain)
- Go version upgrades become coordination nightmares
- Production deployment requires lock-step versioning

**Prevention:**
Use HashiCorp's `go-plugin` instead of stdlib `plugin`:
- RPC-based: plugins run as separate processes
- Protocol versioning: increment protocol version to invalidate incompatible plugins
- Cross-language: plugins don't need to be written in Go
- Production-proven: 4+ years in Terraform, Vault, Nomad, Packer
- Human-friendly errors when version mismatches occur

**Detection:**
Early warning signs:
- Investigating stdlib `plugin` package documentation
- Planning to distribute plugins to users
- Considering Go version upgrades with existing plugins

**Phase mapping:**
Phase 1 (Plugin Architecture) must decide: stdlib `plugin` vs `go-plugin`. Wrong choice here forces a rewrite.

**Confidence:** HIGH (verified by Go issue tracker, HashiCorp docs, production reports)

**Sources:**
- [Go issue #27751: plugin panic with different package versions](https://github.com/golang/go/issues/27751)
- [Go issue #31354: plugin versions in modules](https://github.com/golang/go/issues/31354)
- [Things to avoid while using Golang plugins](https://alperkose.medium.com/things-to-avoid-while-using-golang-plugins-f34c0a636e8)
- [HashiCorp go-plugin](https://github.com/hashicorp/go-plugin)
- [RPC-based plugins in Go](https://eli.thegreenplace.net/2023/rpc-based-plugins-in-go/)

---

### CRITICAL-2: Template Mining Instability with Variable-Starting Logs

**What goes wrong:**
Drain and similar tree-based parsers fail when log messages start with variables instead of constants. Example:
- "cupsd shutdown succeeded"
- "irqbalance shutdown succeeded"

These should map to template "{service} shutdown succeeded" but Drain creates separate templates because the first token differs.

**Why it happens:**
Drain uses a fixed-depth tree where the first few tokens determine which branch to follow. When constants appear AFTER variables, the tree structure breaks down. Log messages with different variable values at the start get routed to different branches, preventing template consolidation.

**Consequences:**
- Template explosion: thousands of unique templates for the same pattern
- Inaccurate "new pattern" detection (false positives)
- High memory usage from redundant templates
- Degraded anomaly detection (signal lost in noise)
- Production accuracy drops below 90% on variable-starting logs

**Prevention:**
1. **Pre-tokenize with masking:** Replace known variable patterns (IPs, UUIDs, numbers) BEFORE feeding to Drain
2. **Use Drain3 with masking:** The Drain3 implementation includes built-in masking for common patterns
3. **Consider XDrain:** Uses fixed-depth forest (not tree) with majority voting for better stability
4. **Sampling + validation:** Sample 10K logs from each namespace, validate template count is reasonable (<1000 for typical app logs)
5. **Fallback detection:** If template count exceeds threshold, flag namespace for manual review

**Detection:**
Warning signs:
- Template count growing unbounded (monitor templates-per-1000-logs metric)
- Most templates have only 1-5 instances (indicates over-fragmentation)
- "New pattern" alerts firing constantly
- High cardinality in first token position during analysis

**Phase mapping:**
- Phase 2 (Template Mining): Algorithm selection must account for variable-starting logs
- Phase 3 (VictoriaLogs Integration): Need sampling and validation before production use
- Phase 4 (Progressive Disclosure): Template count explosion breaks aggregated view

**Confidence:** HIGH (verified by academic papers, Drain3 documentation, production stability reports)

**Sources:**
- [Investigating and Improving Log Parsing in Practice](https://yanmeng.github.io/papers/FSE221.pdf)
- [Drain3: Robust streaming log template miner](https://github.com/logpai/Drain3)
- [XDrain: Effective log parsing with fixed-depth forest](https://www.sciencedirect.com/science/article/abs/pii/S0950584924001514)
- [Tools and Benchmarks for Automated Log Parsing](https://arxiv.org/pdf/1811.03509)

---

### CRITICAL-3: Race Conditions in Config Hot-Reload Without Atomic Swap

**What goes wrong:**
Naive hot-reload implementations use `sync.RWMutex` to guard a config struct, then modify it in place during reload. This creates race conditions:
1. Goroutine A reads `config.VictoriaLogsURL`
2. Reload happens, sets `config.VictoriaLogsURL = newURL`
3. Goroutine A reads `config.VictoriaLogsAPIKey` (now inconsistent with URL)
4. Request goes to newURL with oldAPIKey → authentication failure

Even with RWMutex, readers can observe partially-updated config state.

**Why it happens:**
RWMutex only prevents concurrent reads/writes, not partial reads across field updates. If reload updates multiple fields, readers may see:
- Some old fields, some new fields (torn reads)
- Config in invalid intermediate state (e.g., URL points to prod but timeout is still dev value)

**Consequences:**
- Intermittent request failures during config reload
- Authentication errors with mismatched credentials
- Timeouts with wrong timeout values
- Silent data corruption if config fields are interdependent
- Difficult to reproduce (timing-sensitive)

**Prevention:**
Use **atomic pointer swap pattern**:

```go
type Config struct {
    // config fields
}

type HotReloadable struct {
    config atomic.Value // stores *Config
}

func (h *HotReloadable) Get() *Config {
    return h.config.Load().(*Config)
}

func (h *HotReloadable) Reload(newConfig *Config) {
    // Validate newConfig first
    if err := newConfig.Validate(); err != nil {
        log.Warn("Config validation failed, keeping old config", "error", err)
        return
    }

    // Single atomic swap - readers see old OR new, never partial
    h.config.Store(newConfig)
}
```

Additional safeguards:
1. **Validate before swap:** Never store invalid config
2. **Deep copy on read if mutating:** Prevent readers from mutating shared config
3. **Version numbering:** Include config version for debugging
4. **Rollback on partial failure:** If plugin initialization fails with new config, revert to old

**Detection:**
Warning signs:
- Planning to use `sync.RWMutex` with multi-field config struct
- Reload logic updates fields one-by-one
- No validation before applying new config
- No rollback mechanism for failed reloads

**Phase mapping:**
Phase 1 (Config Hot-Reload) must use atomic swap from day 1. Retrofitting is painful.

**Confidence:** HIGH (verified by Go docs, production guidance, atomic package documentation)

**Sources:**
- [Golang Hot Configuration Reload](https://www.openmymind.net/Golang-Hot-Configuration-Reload/)
- [Mastering Go Atomic Operations](https://jsschools.com/golang/mastering-go-atomic-operations-build-high-perform/)
- [aah framework hot-reload implementation](https://github.com/go-aah/docs/blob/v0.12/configuration-hot-reload.md)

---

### CRITICAL-4: Template Drift Without Rebalancing Mechanism

**What goes wrong:**
Log formats evolve over time (syntactic drift): new services start emitting logs, existing services change log formats during deployments, dependencies upgrade and change message structure. Template miners trained on old logs fail to recognize new patterns, causing:
- Template explosion as drift occurs
- Accuracy degradation from 90%+ to <70%
- False "new pattern" alerts (actually old pattern with new format)
- Stale templates never merged with similar new ones

**Why it happens:**
Initial clustering creates boundaries. New logs that are semantically similar but syntactically different (e.g., "ERROR: connection timeout" becomes "ERROR connection timeout" after log library upgrade) land in separate clusters. Without rebalancing, these never merge.

**Consequences:**
- Production accuracy drops from 90% to <70% after 30-60 days
- Template count grows unbounded (memory leak)
- "New pattern" detection becomes useless (too many false positives)
- Pattern comparison vs previous window breaks (formats don't match)
- Requires manual intervention or service restart to fix

**Prevention:**
1. **Periodic rebalancing:** Drain3's HELP implementation includes iterative rebalancing that merges similar clusters
2. **Similarity threshold tuning:** Monitor template count growth and adjust similarity threshold if growing too fast
3. **Template TTL:** Expire templates not seen in N days (configurable, default 30d)
4. **Ensemble adaptation:** Use directed lifelong learning (maintain ensemble of parsers, add new one when drift detected)
5. **Drift detection metrics:** Track templates-per-1000-logs ratio, alert if ratio exceeds threshold

**Detection:**
Warning signs:
- Template count growing linearly over time (not plateau)
- Most templates created in last 7 days (indicates old templates not being reused)
- Monitoring Population Stability Index (PSI) shows distribution shift
- "New pattern" alerts correlate with service deployments (expected) AND with time (drift)

**Phase mapping:**
- Phase 2 (Template Mining): Must include rebalancing mechanism from start
- Phase 3 (Monitoring): Track drift metrics (template count, PSI, creation timestamps)
- Phase 4 (Production hardening): Add template TTL and ensemble adaptation

**Confidence:** HIGH (verified by academic research, production log analysis systems, Drain3 documentation)

**Sources:**
- [Adaptive Log Anomaly Detection through Drift Characterization](https://openreview.net/pdf?id=6QXrawkcrX)
- [HELP: Hierarchical Embeddings-based Log Parsing](https://www.themoonlight.io/en/review/help-hierarchical-embeddings-based-log-parsing)
- [System Log Parsing with LLMs: A Review](https://arxiv.org/pdf/2504.04877)

---

## Moderate Pitfalls

Mistakes that cause delays or technical debt.

### MODERATE-1: MCP Protocol Version Mismatch Without Graceful Degradation

**What goes wrong:**
MCP protocol evolves rapidly (2025-03-26, 2025-06-18, 2025-11-25 releases). Plugin built against 2025-06-18 fails to connect to client supporting only 2025-03-26. Instead of graceful degradation, connection fails silently or with cryptic error.

**Why it happens:**
MCP protocol version negotiation happens during initialization. If server declares only newer protocol version and client supports only older version, they cannot agree on common version. Without explicit handling, this manifests as connection timeout or protocol error.

**Prevention:**
1. **Multi-version support:** Server declares all supported protocol versions: `["2025-11-25", "2025-06-18", "2025-03-26"]`
2. **Feature detection, not version checking:** Check for specific capabilities (e.g., async tasks) rather than version string
3. **Graceful fallback:** If client only supports old version, use subset of features available in that version
4. **Clear error messages:** If no common version, return human-friendly error: "Server requires MCP 2025-06-18+, client supports 2025-03-26"
5. **Version in health endpoint:** Expose supported protocol versions in status endpoint for debugging

**Detection:**
- Planning single protocol version support
- Hard-coding protocol version checks
- No fallback for missing features
- Connection errors without version information

**Phase mapping:**
Phase 1 (Plugin Architecture): Design plugin interface to support multiple MCP protocol versions.

**Confidence:** MEDIUM (MCP spec documentation, production deployment reports)

**Sources:**
- [MCP Versioning Specification](https://modelcontextprotocol.io/specification/versioning)
- [MCP 2025-11-25 Release](https://blog.modelcontextprotocol.io/posts/2025-11-25-first-mcp-anniversary/)
- [MCP Best Practices](https://modelcontextprotocol.info/docs/best-practices/)

---

### MODERATE-2: Cross-Client Template Inconsistency Without Canonical Storage

**What goes wrong:**
Two clients (IDE plugin, CLI) mine templates independently. Same log message gets template ID "a7b3c4" in IDE but "f9e2d1" in CLI. User asks "show me instances of template a7b3c4" in CLI → no results (CLI doesn't have that ID).

**Why it happens:**
Template mining is sensitive to:
- Processing order (first-seen logs influence tree structure)
- Sampling (if sampling differently, see different representative logs)
- Algorithm parameters (similarity threshold, max depth)
- Initialization state (empty tree vs pre-populated)

**Prevention:**
1. **Canonical storage in MCP server:** Server mines templates once, stores in local cache, serves template IDs to all clients
2. **Deterministic template IDs:** Hash normalized template string (lowercase, sorted params) → consistent ID across clients
3. **Template sync protocol:** Clients periodically fetch template mapping from MCP server
4. **Lazy mining:** Client sends raw logs to MCP server, server returns template ID (mines if new)
5. **Template versioning:** Include timestamp or version in template ID to track evolution

**Detection:**
- Planning client-side template mining without coordination
- Using random IDs or sequential counters for templates
- No shared storage for template definitions
- Template IDs in URLs or saved queries (implies long-term identity)

**Phase mapping:**
Phase 2 (Template Mining): Must decide storage location (MCP server vs client)
Phase 4 (Multi-client support): Cross-client consistency becomes critical

**Confidence:** MEDIUM (distributed caching research, log analysis best practices)

**Sources:**
- [Distributed caching with strong consistency](https://www.frontiersin.org/journals/computer-science/articles/10.3389/fcomp.2025.1511161/full)
- [Cache consistency patterns](https://redis.io/blog/three-ways-to-maintain-cache-consistency/)

---

### MODERATE-3: Plugin Testing Without Process Isolation

**What goes wrong:**
Testing plugin lifecycle (load, execute, reload, unload) in-process using Go's stdlib `plugin` package. Test crashes take down entire test suite. Flaky tests due to global state pollution between plugin loads.

**Why it happens:**
Stdlib `plugin.Open()` loads `.so` into current process. Cannot unload. Global variables in plugin persist across test cases. Panic in plugin panics test runner.

**Prevention:**
1. **Use go-plugin (RPC):** Plugins run as subprocesses, crashes are isolated
2. **Test containers:** Run each plugin test in separate container
3. **Test utilities:** Use testify suites for setup/teardown
4. **Resource limits:** Apply cgroups or containers to limit plugin resource usage during tests
5. **Timeout protection:** Wrap plugin operations in timeouts

Example test structure with go-plugin:
```go
func TestPluginLifecycle(t *testing.T) {
    client := plugin.NewClient(&plugin.ClientConfig{
        HandshakeConfig: handshake,
        Plugins:        pluginMap,
        Cmd:            exec.Command("./my-plugin"),
    })
    defer client.Kill()  // Clean shutdown

    rpcClient, err := client.Client()
    require.NoError(t, err)

    // Test plugin operations - crash won't affect test runner
}
```

**Detection:**
- Using stdlib `plugin` for testing
- No process isolation in tests
- Tests share global state
- Flaky tests that pass individually but fail in suite

**Phase mapping:**
Phase 1 (Plugin Architecture): Test strategy must align with plugin implementation choice.

**Confidence:** MEDIUM (Go testing best practices, go-plugin documentation)

**Sources:**
- [Building a Plugin System in Go](https://skoredin.pro/blog/golang/go-plugin-system)
- [Go integration testing guide](https://mortenvistisen.com/posts/integration-tests-with-docker-and-go)
- [go-plugin test examples](https://github.com/hashicorp/go-plugin/blob/main/grpc_client_test.go)

---

### MODERATE-4: VictoriaLogs Live Tailing Without Rate Limiting

**What goes wrong:**
Implementing live tail (streaming logs in real-time) with aggressive refresh intervals (e.g., 100ms). High-volume namespaces emit thousands of logs per second. UI becomes unusable, VictoriaLogs CPU spikes, websocket connections timeout.

**Why it happens:**
VictoriaLogs documentation explicitly warns: "It isn't recommended setting too low value for refresh_interval query arg, since this may increase load on VictoriaLogs without measurable benefits." Live tailing is optimized for human inspection (up to 1K logs/sec), not machine processing.

**Consequences:**
- VictoriaLogs CPU usage spikes
- UI freezes trying to render thousands of log lines
- Websocket connections saturate network
- False impression that VictoriaLogs is slow (actually client abuse)
- User cannot read logs scrolling at 10K/sec anyway

**Prevention:**
1. **Minimum refresh interval:** 1 second minimum, recommend 5 seconds
2. **Rate limiting in UI:** If logs exceed 1K/sec, show warning and suggest adding filters
3. **Auto-pause on high rate:** Pause streaming if rate exceeds threshold, require user action to resume
4. **Sampling for preview:** Show sampled logs (1 in N) during high-volume periods
5. **Filter-first UX:** Require namespace + severity filter before enabling live tail

**Detection:**
- Planning live tail feature
- No refresh_interval limits in UI
- No rate detection or warnings
- Testing with low-volume logs only

**Phase mapping:**
Phase 3 (VictoriaLogs Integration): Live tail is nice-to-have, defer to later phase.
Phase 4 (Progressive Disclosure): Focus on aggregated view first, raw logs last.

**Confidence:** HIGH (VictoriaLogs official documentation)

**Sources:**
- [VictoriaLogs Querying Documentation](https://docs.victoriametrics.com/victorialogs/querying/)
- [VictoriaLogs FAQ](https://docs.victoriametrics.com/victorialogs/faq/)

---

### MODERATE-5: UI State Loss During Progressive Disclosure Navigation

**What goes wrong:**
User is in "Aggregated View" for namespace "api-gateway", drills into a specific template, clicks browser back button → loses all state, returns to global overview. Expected: return to namespace "api-gateway" aggregated view.

**Why it happens:**
Progressive disclosure creates three view levels (global → aggregated → full logs). If state is component-local (React useState), navigation resets it. Browser back/forward buttons don't restore component state.

**Consequences:**
- Poor UX: users must manually navigate back through levels
- Lost context: selected filters, time ranges, templates
- Frustration: "I was just looking at that namespace, now I have to find it again"
- Users avoid drilling down (defeats purpose of progressive disclosure)

**Prevention:**
1. **URL state:** Encode state in URL query params: `?view=aggregated&namespace=api-gateway&template=a7b3c4`
2. **React Router with state:** Use `location.state` to pass context between routes
3. **Global state manager:** Zustand or Context API for cross-component state
4. **Session storage fallback:** Persist state to sessionStorage as backup
5. **Breadcrumb navigation:** Show "Global > api-gateway > template-a7b3c4" with clickable links

Example URL structure:
```
/logs                                    # Global overview
/logs?ns=api-gateway                    # Aggregated view for namespace
/logs?ns=api-gateway&tpl=a7b3c4         # Full logs for template
```

**Detection:**
- Planning multi-level navigation without URL state
- Using component-local state for navigation context
- No breadcrumb UI
- Browser back button not tested

**Phase mapping:**
Phase 4 (Progressive Disclosure UI): URL-based state from day 1, hard to retrofit.

**Confidence:** MEDIUM (SPA best practices, React state management guidance)

**Sources:**
- [State is hard: why SPAs will persist](https://nolanlawson.com/2022/05/29/state-is-hard-why-spas-will-persist/)
- [React State Management 2025](https://www.developerway.com/posts/react-state-management-2025)
- [State Management in SPAs](https://blog.pixelfreestudio.com/state-management-in-single-page-applications-spas/)

---

## Minor Pitfalls

Mistakes that cause annoyance but are fixable.

### MINOR-1: No Config Validation Before Hot-Reload

**What goes wrong:**
Hot-reload picks up new config file with typo in VictoriaLogs URL: `http://victorialogs:8428/selec` (missing 't' in 'select'). MCP server reloads config, tools break with 404 errors. No warning logged, just silent failure.

**Prevention:**
1. **Validate before swap:** Parse and validate config completely before applying
2. **Health check endpoints:** For integrations with base URLs, ping health endpoint before activating
3. **Dry-run mode:** Test config without applying (config validate command)
4. **Schema validation:** Use JSON schema or struct tags to enforce required fields
5. **Keep old config on failure:** Log warning, continue using old config

**Detection:**
- No validation in reload path
- Assuming config is always valid
- No health checks for external services

**Phase mapping:**
Phase 1 (Config Hot-Reload): Add validation in initial implementation.

---

### MINOR-2: Overly Deep Progressive Disclosure (>2 Levels)

**What goes wrong:**
Designing 4+ levels: Global → Namespace → Service → Pod → Template → Instance. User gets lost in navigation, clicks back 5 times to start over.

**Prevention:**
UX research shows "more than two levels of information disclosure usually negatively affect the user experience." Limit to 3 levels maximum:
1. Global overview (signals by namespace)
2. Aggregated view (templates in selected namespace)
3. Full logs (instances of selected template)

**Detection:**
- UI mockups showing 4+ navigation levels
- No breadcrumb UI (indicates too many levels)
- User testing shows confusion

**Phase mapping:**
Phase 4 (Progressive Disclosure): Design review before implementation.

**Confidence:** HIGH (UX research on progressive disclosure)

**Sources:**
- [Progressive Disclosure Examples](https://medium.com/@Flowmapp/progressive-disclosure-10-great-examples-to-check-5e54c5e0b5b6)
- [Progressive Disclosure in UX Design](https://blog.logrocket.com/ux-design/progressive-disclosure-ux-types-use-cases/)

---

### MINOR-3: Template Normalization Inconsistency

**What goes wrong:**
Normalizing UUIDs to wildcards: `req-550e8400-e29b-41d4-a716-446655440000` → `req-{uuid}`. But IPv6 addresses also have hyphens: `2001:0db8:85a3:0000:0000:8a2e:0370:7334`. Naive UUID regex matches IPv6, breaks template.

**Prevention:**
1. **Order normalization rules:** Most specific first (IPv6 before UUID)
2. **Use proven masking libraries:** Don't write regex from scratch
3. **Test with edge cases:** IPv6, scientific notation, negative numbers, etc.
4. **Drain3 built-in masking:** Includes battle-tested patterns
5. **Validate templates:** Sample 1000 logs, ensure template coverage is reasonable (>80%)

**Detection:**
- Writing custom normalization regex
- No test cases for edge cases
- Template validation shows unexpected patterns

**Phase mapping:**
Phase 2 (Template Mining): Use proven library from start.

---

### MINOR-4: Ignoring VictoriaLogs Time Filter Optimization

**What goes wrong:**
Querying "show logs with severity=ERROR for the last 7 days" without explicit time filter, relying only on day_range. VictoriaLogs scans all time partitions unnecessarily.

**Prevention:**
VictoriaLogs docs recommend: "it is recommended to specify a regular time filter additionally to the day_range filter." Combine both:
```
_time:[now-7d, now] AND day_range[now-7d, now] AND severity:ERROR
```

**Detection:**
- Using day_range without _time filter
- Slow queries despite correct day_range

**Phase mapping:**
Phase 3 (VictoriaLogs Integration): Query construction must follow docs.

**Confidence:** HIGH (VictoriaLogs official documentation)

**Sources:**
- [VictoriaLogs Querying Documentation](https://docs.victoriametrics.com/victorialogs/querying/)

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Plugin Architecture | Using stdlib `plugin` instead of go-plugin | Research go-plugin first, understand RPC trade-offs |
| Config Hot-Reload | RWMutex instead of atomic.Value | Use atomic pointer swap pattern from day 1 |
| Template Mining | Choosing Drain without understanding variable-starting logs | Test with production log samples, validate template count |
| VictoriaLogs API | Hardcoding protocol version, no multi-version support | Support multiple MCP protocol versions |
| Progressive Disclosure | Component-local state without URL persistence | Encode state in URL from day 1 |
| Cross-Client Consistency | Client-side template mining without canonical storage | Store templates in MCP server, use deterministic IDs |
| Testing Strategy | In-process plugin testing without isolation | Align testing with plugin architecture (RPC = subprocess tests) |
| Live Tailing | No rate limiting on websocket streaming | Min 1s refresh, warn at >1K logs/sec |
| Template Stability | No rebalancing mechanism for drift | Use Drain3 with iterative rebalancing |
| Config Validation | Accepting invalid config during hot-reload | Validate before swap, keep old config on failure |

---

## Research Confidence Assessment

| Area | Confidence | Notes |
|------|-----------|-------|
| Go Plugin Systems | HIGH | Verified with Go issue tracker, HashiCorp docs, production reports |
| Template Mining | HIGH | Verified with academic papers, Drain3 docs, production stability reports |
| Config Hot-Reload | HIGH | Verified with Go atomic package docs, production guides |
| Progressive Disclosure | MEDIUM | Verified with UX research, React state management guides (web search only) |
| VictoriaLogs | HIGH | Verified with official documentation |
| MCP Protocol | MEDIUM | Verified with spec documentation (web search only) |
| Cross-Client Caching | MEDIUM | Verified with distributed systems research (web search only) |

---

## Sources

### Go Plugin Systems
- [Go issue #27751: plugin panic with different package versions](https://github.com/golang/go/issues/27751)
- [Go issue #31354: plugin versions in modules](https://github.com/golang/go/issues/31354)
- [Things to avoid while using Golang plugins](https://alperkose.medium.com/things-to-avoid-while-using-golang-plugins-f34c0a636e8)
- [HashiCorp go-plugin](https://github.com/hashicorp/go-plugin)
- [RPC-based plugins in Go](https://eli.thegreenplace.net/2023/rpc-based-plugins-in-go/)
- [HashiCorp Plugin System Design](https://zerofruit-web3.medium.com/hashicorp-plugin-system-design-and-implementation-5f939f09e3b3)

### Log Template Mining
- [Investigating and Improving Log Parsing in Practice](https://yanmeng.github.io/papers/FSE221.pdf)
- [Drain3: Robust streaming log template miner](https://github.com/logpai/Drain3)
- [XDrain: Effective log parsing with fixed-depth forest](https://www.sciencedirect.com/science/article/abs/pii/S0950584924001514)
- [Tools and Benchmarks for Automated Log Parsing](https://arxiv.org/pdf/1811.03509)
- [Adaptive Log Anomaly Detection through Drift Characterization](https://openreview.net/pdf?id=6QXrawkcrX)
- [HELP: Hierarchical Embeddings-based Log Parsing](https://www.themoonlight.io/en/review/help-hierarchical-embeddings-based-log-parsing)
- [System Log Parsing with LLMs: A Review](https://arxiv.org/pdf/2504.04877)

### Configuration Hot-Reload
- [Golang Hot Configuration Reload](https://www.openmymind.net/Golang-Hot-Configuration-Reload/)
- [Mastering Go Atomic Operations](https://jsschools.com/golang/mastering-go-atomic-operations-build-high-perform/)
- [aah framework hot-reload implementation](https://github.com/go-aah/docs/blob/v0.12/configuration-hot-reload.md)

### Progressive Disclosure & State Management
- [State is hard: why SPAs will persist](https://nolanlawson.com/2022/05/29/state-is-hard-why-spas-will-persist/)
- [React State Management 2025](https://www.developerway.com/posts/react-state-management-2025)
- [State Management in SPAs](https://blog.pixelfreestudio.com/state-management-in-single-page-applications-spas/)
- [Progressive Disclosure Examples](https://medium.com/@Flowmapp/progressive-disclosure-10-great-examples-to-check-5e54c5e0b5b6)
- [Progressive Disclosure in UX Design](https://blog.logrocket.com/ux-design/progressive-disclosure-ux-types-use-cases/)

### VictoriaLogs
- [VictoriaLogs Documentation](https://docs.victoriametrics.com/victorialogs/)
- [VictoriaLogs Querying](https://docs.victoriametrics.com/victorialogs/querying/)
- [VictoriaLogs FAQ](https://docs.victoriametrics.com/victorialogs/faq/)
- [VictoriaLogs vs Loki Benchmarks](https://www.truefoundry.com/blog/victorialogs-vs-loki)

### MCP Protocol
- [MCP Versioning Specification](https://modelcontextprotocol.io/specification/versioning)
- [MCP 2025-11-25 Release](https://blog.modelcontextprotocol.io/posts/2025-11-25-first-mcp-anniversary/)
- [MCP Best Practices](https://modelcontextprotocol.info/docs/best-practices/)

### Distributed Caching & Consistency
- [Distributed caching with strong consistency](https://www.frontiersin.org/journals/computer-science/articles/10.3389/fcomp.2025.1511161/full)
- [Cache consistency patterns](https://redis.io/blog/three-ways-to-maintain-cache-consistency/)
- [Comparative Analysis of Distributed Caching Algorithms](https://arxiv.org/html/2504.02220v1)

### Testing & Development
- [Building a Plugin System in Go](https://skoredin.pro/blog/golang/go-plugin-system)
- [Go integration testing guide](https://mortenvistisen.com/posts/integration-tests-with-docker-and-go)
- [go-plugin test examples](https://github.com/hashicorp/go-plugin/blob/main/grpc_client_test.go)
