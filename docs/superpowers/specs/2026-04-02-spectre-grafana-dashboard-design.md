# Spectre Grafana Dashboard Design

## Goal

Add a standalone Grafana dashboard JSON for Spectre that gives operators a clear overview of system health, graph sync throughput, error behavior, cache and index effectiveness, and VictoriaLogs pipeline status.

## Scope

This design covers a single committed Grafana dashboard JSON artifact.

In scope:
- one importable Grafana dashboard JSON file
- Prometheus-backed panels for currently confirmed Spectre metric families
- datasource and label variables so the dashboard is portable across environments
- operator-focused visual layout for quick triage

Out of scope:
- Helm wiring or Grafana provisioning manifests
- alert rules
- dashboard auto-loading
- speculative panels for metrics that do not exist in the codebase

## Current Metric Surface

The metric families confirmed in the current codebase are:

### Graph sync metrics

Defined in `internal/graph/sync/metrics.go`:

- `spectre_graph_sync_state_cache_hits_total`
- `spectre_graph_sync_state_cache_misses_total`
- `spectre_graph_sync_state_cache_size`
- `spectre_graph_sync_label_index_hits_total`
- `spectre_graph_sync_label_index_misses_total`
- `spectre_graph_sync_label_index_size`
- `spectre_graph_sync_label_index_namespaces`
- `spectre_graph_sync_events_processed_total`
- `spectre_graph_sync_events_skipped_total`
- `spectre_graph_sync_nodes_created_total`
- `spectre_graph_sync_edges_created_total`
- `spectre_graph_sync_event_processing_seconds`
- `spectre_graph_sync_batch_size`
- `spectre_graph_sync_batch_duration_seconds`
- `spectre_graph_sync_errors_total`

### VictoriaLogs integration metrics

Defined in `internal/integration/victorialogs/metrics.go`:

- `victorialogs_pipeline_queue_depth{instance=...}`
- `victorialogs_pipeline_logs_total{instance=...}`
- `victorialogs_pipeline_errors_total{instance=...}`

## Important Runtime Caveat

The Helm chart already exposes a `ServiceMonitor` shape that targets `/metrics`, but this design work only produces the dashboard JSON. Live data in Grafana still depends on the Spectre service exposing scrapeable Prometheus metrics in the running environment.

That means:
- the dashboard JSON can be committed and imported immediately
- some or all panels may remain empty until `/metrics` is available and scraped
- the dashboard must therefore avoid hard failures when a metric family is absent

## Dashboard Strategy

Use a hybrid operator-overview design:

- prioritize currently confirmed Spectre metrics as first-class panels
- make rate-based counters the default view
- include compact derived signals that help operators interpret throughput and efficiency
- avoid embedded-only or HTTP-server panels unless a confirmed metric exists

This gives a useful dashboard now without overstating the current telemetry surface.

## Dashboard Layout

### Variables

- `datasource`
  - type: datasource
  - datasource type: Prometheus
  - purpose: portability across Grafana environments

- `instance`
  - type: query
  - source metric: `label_values(victorialogs_pipeline_queue_depth, instance)`
  - include all option
  - purpose: filter VictoriaLogs panels when multiple integration instances exist

### Row 1: Spectre Overview

Purpose: operator-at-a-glance health and activity.

Panels:
- Events processed rate
- Events skipped rate
- Processing errors rate
- Nodes created rate
- Edges created rate
- Event processing latency p95

Representative PromQL:
- `sum(rate(spectre_graph_sync_events_processed_total[$__rate_interval]))`
- `sum(rate(spectre_graph_sync_events_skipped_total[$__rate_interval]))`
- `sum(rate(spectre_graph_sync_errors_total[$__rate_interval]))`
- `sum(rate(spectre_graph_sync_nodes_created_total[$__rate_interval]))`
- `sum(rate(spectre_graph_sync_edges_created_total[$__rate_interval]))`
- `histogram_quantile(0.95, sum by (le) (rate(spectre_graph_sync_event_processing_seconds_bucket[$__rate_interval])))`

### Row 2: Sync Pipeline Efficiency

Purpose: show whether Spectre is doing useful work efficiently.

Panels:
- Batch size p50/p95
- Batch duration p50/p95
- Skip ratio
- Graph mutation intensity

Representative PromQL:
- `histogram_quantile(0.50, sum by (le) (rate(spectre_graph_sync_batch_size_bucket[$__rate_interval])))`
- `histogram_quantile(0.95, sum by (le) (rate(spectre_graph_sync_batch_duration_seconds_bucket[$__rate_interval])))`
- `sum(rate(spectre_graph_sync_events_skipped_total[$__rate_interval])) / clamp_min(sum(rate(spectre_graph_sync_events_processed_total[$__rate_interval])), 1e-9)`
- `(sum(rate(spectre_graph_sync_nodes_created_total[$__rate_interval])) + sum(rate(spectre_graph_sync_edges_created_total[$__rate_interval]))) / clamp_min(sum(rate(spectre_graph_sync_events_processed_total[$__rate_interval])), 1e-9)`

### Row 3: Cache and Label Index Behavior

Purpose: expose whether Spectre’s internal lookup paths are healthy and whether cardinality is drifting.

Panels:
- State cache hit rate
- State cache miss rate
- State cache size
- Label index hit rate
- Label index miss rate
- Label index size
- Label index namespaces

Representative PromQL:
- `sum(rate(spectre_graph_sync_state_cache_hits_total[$__rate_interval]))`
- `sum(rate(spectre_graph_sync_state_cache_misses_total[$__rate_interval]))`
- `sum(spectre_graph_sync_state_cache_size)`
- `sum(rate(spectre_graph_sync_label_index_hits_total[$__rate_interval]))`
- `sum(rate(spectre_graph_sync_label_index_misses_total[$__rate_interval]))`
- `sum(spectre_graph_sync_label_index_size)`
- `sum(spectre_graph_sync_label_index_namespaces)`

### Row 4: VictoriaLogs Pipeline Health

Purpose: expose downstream log-export pressure and failures.

Panels:
- Queue depth
- Logs sent rate
- Pipeline error rate
- Per-instance queue depth breakdown

Representative PromQL:
- `sum(victorialogs_pipeline_queue_depth{instance=~"$instance"})`
- `sum(rate(victorialogs_pipeline_logs_total{instance=~"$instance"}[$__rate_interval]))`
- `sum(rate(victorialogs_pipeline_errors_total{instance=~"$instance"}[$__rate_interval]))`
- `victorialogs_pipeline_queue_depth{instance=~"$instance"}`

### Row 5: Operator Status Strip

Purpose: provide a compact final row of stat panels for dashboards viewed on wallboards or narrow screens.

Panels:
- Processed events rate
- Error rate
- Skip ratio
- State cache size
- Label index size
- Total queue depth

## Visualization Choices

Use:
- `stat` panels for headline rates and gauges
- `timeseries` panels for trends and p95 latency lines
- no pie charts
- no tables unless a per-instance breakdown cannot be expressed clearly as a series panel

Threshold intent:
- error-rate panels should visually highlight any sustained non-zero activity
- queue depth panels should highlight backlog growth
- skip ratio panels should only warn at clearly elevated levels, because some skip behavior is expected

## Portability Requirements

The JSON must:
- use a datasource variable rather than a hardcoded UID
- avoid fixed namespace or cluster labels
- avoid environment-specific titles, URLs, or links
- import cleanly into any Grafana with a Prometheus datasource

## Artifact Location

The dashboard JSON should be committed as a standalone file under:

- `hack/grafana/spectre-operator-overview.json`

This keeps it discoverable, easy to import manually, and separate from Helm packaging concerns.

## Validation

The produced JSON should be checked for:
- valid Grafana dashboard structure
- queries referencing only confirmed metric names
- importability with a Prometheus datasource variable
- graceful behavior when VictoriaLogs metrics are absent

Manual validation after implementation:
- import the dashboard into Grafana
- bind the Prometheus datasource variable
- confirm graph-sync panels render if Spectre metrics are scraped
- confirm VictoriaLogs panels either render correctly or stay empty without query errors

## Recommendation

Implement the hybrid operator overview exactly as described above: it gives Spectre a usable Grafana dashboard immediately while staying honest about the currently confirmed metric surface.
