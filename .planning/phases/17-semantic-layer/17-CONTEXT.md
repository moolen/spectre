# Phase 17: Semantic Layer - Context

**Gathered:** 2026-01-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Classify dashboards by hierarchy level, infer services from PromQL labels, and categorize Grafana variables by type. Includes UI for hierarchy mapping fallback configuration when tags are missing.

</domain>

<decisions>
## Implementation Decisions

### Service inference rules
- Label priority: app > service > job.
- Service identity includes both cluster and namespace scoping.
- If multiple labels disagree, split into multiple service nodes.
- If no service-related labels exist, attach metrics to an Unknown service node.

### Dashboard hierarchy classification
- Primary signal: tags first; naming heuristics only as fallback.
- Tag values for level: overview / drilldown / detail.
- Tags are authoritative when they conflict with name heuristics.
- If no signals present, default to detail.

### Variable classification
- Primary signal: variable name patterns (e.g., cluster, region, service).
- Scoping variables include cluster, region, env.
- Entity variables include service, namespace, app.
- Unknown variables get explicit unknown classification.

### Fallback mapping UI
- If tags are absent, default classification to detail.
- Validation on save is warning-only (allow save).
- No preview of classification results in the UI.

### Claude's Discretion
- User override granularity for fallback mapping UI (per tag, per dashboard, per folder).

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 17-semantic-layer*
*Context gathered: 2026-01-22*
