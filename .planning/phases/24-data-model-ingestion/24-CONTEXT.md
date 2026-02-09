# Phase 24: Data Model & Ingestion - Context

**Gathered:** 2026-01-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Create SignalAnchor nodes that extract "what matters" from Grafana dashboards. Each anchor links a metric query to a classified signal role (Availability, Latency, Errors, Traffic, Saturation, Churn, Novelty) with quality scoring and K8s workload linkage. Baseline storage and anomaly detection are Phase 25.

</domain>

<decisions>
## Implementation Decisions

### Role Classification

**Layered classification with decreasing confidence:**

1. **Layer 1: Hardcoded Known Metrics** (confidence ~0.95)
   - `container_cpu_usage_seconds_total` → Saturation
   - `kube_pod_status_phase` → Availability
   - `up` → Availability

2. **Layer 2: PromQL Structure** (confidence ~0.85-0.9)
   - `histogram_quantile(*_bucket)` → Latency
   - `increase(*_total)` where name contains error → Errors
   - `rate(*_total)` where name matches request/query/call → Traffic

3. **Layer 3: Metric Name Patterns** (confidence ~0.7-0.8)
   - `*_latency*`, `*_duration*`, `*_time*` → Latency
   - `*_error*`, `*_failed*`, `*_fault*` → Errors
   - `*_total`, `*_count` (not error) → Traffic

4. **Layer 4: Panel Title/Description** (confidence ~0.5)
   - "Error Rate", "Failures" → Errors
   - "Latency", "Response Time" → Latency
   - "QPS", "Throughput" → Traffic

5. **Layer 5: Unclassified** (confidence 0)
   - Mark as Unknown, include in `uncertain` response section

**Multi-role handling:** Create separate SignalAnchor per detected role from the same query. Anchor links to Query node, not just Metric.

**No overrides initially:** Trust the algorithm, fix classification bugs in code.

### Confidence Thresholds

**Show all signals, structured by confidence tier:**

```go
type WorkloadSignals struct {
    Signals   map[SignalRole][]SignalSummary  // High confidence (>= threshold)
    Uncertain []UncertainSignal               // Below threshold but detected
    Unmapped  []string                        // Couldn't classify at all
}
```

- Default threshold: 0.7
- Agent can override via tool parameter (`min_confidence`, `include_uncertain`, `include_unmapped`)
- Never filter/hide signals completely — agent needs to know what it doesn't know

### Quality Scoring

**Five factors, normalized 0-1, simple average with alert boost:**

1. **Freshness:** Last modified within 90 days = 1.0, linear decay to 0.0 at 365 days
2. **RecentUsage:** Has any views in last 30 days = 1.0, else 0.0 (from Grafana Stats API)
3. **HasAlerts:** At least one alert rule attached = 1.0, else 0.0
4. **Ownership:** Lives in team folder (not "General") = 1.0, else 0.5
5. **Completeness:** Has description + meaningful panel titles = 0-1.0 (partial credit)

**Formula:**
```go
base := (Freshness + RecentUsage + Ownership + Completeness) / 4.0
alertBoost := HasAlerts * 0.2
quality := min(1.0, base + alertBoost)
```

**Tier mapping:** >= 0.7 = high, >= 0.4 = medium, < 0.4 = low

**Propagation:** SignalAnchor inherits quality score from source dashboard directly.

### K8s Workload Linkage

**Hybrid approach, layered:**

1. Try direct K8s label match (namespace, deployment, service, pod) to existing K8s graph nodes
2. Fall back to Service node inference (reuse v1.3 Service nodes from job/service/app labels)
3. If no match: create signal as orphan node, mark as `unlinked`

**Label priority (standard K8s):** namespace > deployment > service > pod > container

**Workload node creation:**
- Link to existing K8s graph nodes if Spectre has K8s integration enabled
- Create Workload nodes from PromQL labels if K8s integration not available

### Ingestion Behavior

**Trigger:** Piggyback on existing DashboardSyncer — extract signals whenever dashboards sync

**Conflict resolution:** Same metric in multiple dashboards → keep anchor from highest-quality dashboard source (deduplicate by metric+workload, highest quality wins)

**Progress reporting:** Counts only — dashboards processed, signals created, errors

**Stale signal handling:** TTL expiration via `expires_at` timestamp with query-time filtering (matches existing pattern from v1.4)

### Claude's Discretion

- Exact Layer 1 hardcoded metric list (start small, expand based on real data)
- PromQL parsing depth for Layer 2 (extend existing parser or use regex patterns)
- TTL duration for signal expiration
- Whether to log classification decisions at debug level

</decisions>

<specifics>
## Specific Ideas

- Confidence decreases as classification moves down layers — Layer 1 = 0.95, Layer 4 = 0.5
- Panel title is "human intent" signal — leverage it as fallback
- "Golden signals" dashboards pack multiple metrics in one panel — handle multi-query panels correctly
- Usage data from Grafana Stats API may not exist in all deployments — handle gracefully

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 24-data-model-ingestion*
*Context gathered: 2026-01-29*
