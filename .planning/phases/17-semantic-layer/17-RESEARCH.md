# Phase 17: Semantic Layer - Research

**Researched:** 2026-01-22
**Domain:** Grafana dashboard ingestion semantics (service inference, hierarchy classification, variable typing) in Go + React
**Confidence:** MEDIUM-HIGH

## Summary

Phase 17 builds on the existing Grafana integration pipeline (`internal/integration/grafana`) that already ingests dashboards, parses PromQL, and writes Dashboard/Panel/Query/Metric nodes. The missing work is entirely semantic: infer Service nodes from PromQL label selectors, classify dashboards by hierarchy tags, and parse Grafana variables from dashboard JSON into typed Variable nodes. The UI already exposes Grafana configuration; Phase 17 adds hierarchy mapping fallback configuration to the integration form (UICF-04) and stores mapping in integration config for use during sync.

Implementation should stay inside the Grafana sync pipeline (GraphBuilder + Syncer) to keep semantic extraction at ingestion time. This keeps MCP tools fast later and aligns with Phase 16’s decision to extract PromQL during sync. Use the existing PromQL parser (`prometheus/promql/parser`) and graph client utilities; don’t build new parsers or schema systems.

**Primary recommendation:** extend `GraphBuilder` to (1) classify dashboards by tags with config fallback, (2) parse templating variables into Variable nodes with classification, and (3) infer Service nodes from label selectors and link via Metric-[:TRACKS]->Service using label priority rules from CONTEXT.md.

## Standard Stack

### Core
| Library/Component | Version | Purpose | Why Standard |
|---|---|---|---|
| `github.com/prometheus/prometheus/promql/parser` | already in repo | PromQL parsing and label selector extraction | Official parser already used in Phase 16 (`internal/integration/grafana/promql_parser.go`). |
| FalkorDB client (`github.com/FalkorDB/falkordb-go/v2`) | v2.0.2 (go.mod) | Graph storage | Existing graph client + schema patterns in `internal/graph`. |
| Grafana API via `net/http` | stdlib | Dashboard retrieval | Current client in `internal/integration/grafana/client.go`. |
| React UI | existing | Integration config UI | `ui/src/components/IntegrationConfigForm.tsx` provides Grafana form fields. |

### Supporting
| Library/Component | Version | Purpose | When to Use |
|---|---|---|---|
| `encoding/json` | stdlib | Parse Grafana dashboard JSON/templating variables | Already used for dashboard parsing and variable storage. |
| `regexp` | stdlib | Variable name classification patterns | Works for classification rules (cluster, region, service, etc.). |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|---|---|---|
| PromQL regex parsing | Custom regex | Brittle and already avoided by Phase 16; stick with official parser. |
| Separate semantic service | Standalone pipeline | Extra moving parts; existing `GraphBuilder` is already the ingestion stage. |

**Installation:**
```bash
# No new dependencies required for Phase 17
```

## Architecture Patterns

### Recommended Project Structure
```
internal/integration/grafana/
├── graph_builder.go         # Add service inference + variable parsing + hierarchy tagging
├── promql_parser.go         # Reuse label selectors for service inference
├── dashboard_syncer.go      # Pass integration config fallback mapping into graph builder
└── types.go                 # Extend Config with hierarchy mapping

ui/src/components/
└── IntegrationConfigForm.tsx # Add hierarchy mapping UI fields
```

### Pattern 1: Ingestion-Time Semantic Extraction
**What:** Parse service labels, dashboard hierarchy, and variables during sync, not at query time.
**When to use:** Always for semantic graph metadata that powers MCP tools.
**Example:**
```go
// Source: internal/integration/grafana/graph_builder.go
// Extend CreateDashboardGraph to derive hierarchy + variables + services.
func (gb *GraphBuilder) CreateDashboardGraph(ctx context.Context, dashboard *GrafanaDashboard) error {
    // 1) Determine hierarchy level from tags or fallback config
    // 2) Extract variables from dashboard.Templating.List
    // 3) Create Service nodes inferred from QueryExtraction.LabelSelectors
}
```

### Pattern 2: Config-Driven Fallbacks
**What:** Use integration config to provide fallback mapping for hierarchy when tags are missing.
**When to use:** If dashboard tags don’t include `spectre:overview`, `spectre:drilldown`, `spectre:detail`.
**Example:**
```go
// Source: internal/integration/grafana/types.go
type Config struct {
    URL          string     `json:"url" yaml:"url"`
    APITokenRef  *SecretRef `json:"apiTokenRef,omitempty" yaml:"apiTokenRef,omitempty"`
    HierarchyMap map[string][]string `json:"hierarchyMap,omitempty" yaml:"hierarchyMap,omitempty"`
}
```

### Anti-Patterns to Avoid
- **Parsing PromQL with regex:** unreliable for label extraction and conflicts with Phase 16’s AST parser.
- **Creating service nodes without scoping:** service identity must include cluster and namespace per CONTEXT.md.
- **Skipping unknown classifications:** store explicit `unknown` values so tools can reason about gaps.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---|---|---|---|
| PromQL parsing | Regex/hand parser | `prometheus/promql/parser` | Already used in `promql_parser.go`, robust AST access. |
| Graph writes | Custom bolt client | `graph.Client` + `graph.GraphQuery` | Keeps schema and logging consistent with existing graph code. |
| Integration config UI | New settings page | `IntegrationConfigForm` + existing modal workflow | Consistent UX and validation flow. |

**Key insight:** Phase 17 is data modeling and extraction, not new infrastructure—reuse existing parsers, graph client, and UI forms.

## Common Pitfalls

### Pitfall 1: Variable syntax breaks PromQL parsing
**What goes wrong:** Grafana variables (`$var`, `${var}`) make PromQL unparseable; metrics skipped.
**Why it happens:** `parser.ParseExpr` fails on variable syntax.
**How to avoid:** Keep `HasVariables` flag and use label selectors only; avoid metric name creation when variable is present (current behavior).
**Warning signs:** PromQL parse errors in sync logs, no Metric nodes for variable-heavy dashboards.

### Pitfall 2: Dashboard tags missing or inconsistent
**What goes wrong:** Hierarchy level is undefined or incorrect.
**Why it happens:** Grafana tags are optional and user-controlled.
**How to avoid:** Apply tag-first logic and fallback mapping with default `detail` when no match (per CONTEXT.md).
**Warning signs:** Dashboards missing `hierarchyLevel` property, unexpected tool ordering.

### Pitfall 3: Service inference over-matches labels
**What goes wrong:** Metrics link to incorrect services or explode into many Service nodes.
**Why it happens:** Using any label as service name or not enforcing whitelist.
**How to avoid:** Use label whitelist (job, service, app, namespace, cluster) and priority `app > service > job`; split when conflicting.
**Warning signs:** High cardinality of Service nodes with empty cluster/namespace.

### Pitfall 4: Variable classification too implicit
**What goes wrong:** Tools can’t decide what variables are for scoping vs entity.
**Why it happens:** Variables stored raw JSON only (`Dashboard.variables` string).
**How to avoid:** Create Variable nodes with explicit classification `scoping|entity|detail|unknown` and link to dashboards.
**Warning signs:** Variable data only stored in `Dashboard.variables` string and not queryable.

## Code Examples

### Extract PromQL labels for service inference
```go
// Source: internal/integration/grafana/promql_parser.go
parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
    if n, ok := node.(*parser.VectorSelector); ok {
        for _, matcher := range n.LabelMatchers {
            if matcher.Name == "__name__" {
                continue
            }
            extraction.LabelSelectors[matcher.Name] = matcher.Value
        }
    }
    return nil
})
```

### Dashboard/Panel/Query/Metric graph insertion
```go
// Source: internal/integration/grafana/graph_builder.go
MERGE (d:Dashboard {uid: $uid})
MERGE (p:Panel {id: $panelID})
MERGE (q:Query {id: $queryID})
MERGE (m:Metric {name: $name})
MERGE (q)-[:USES]->(m)
```

### Integration config UI entry point
```tsx
// Source: ui/src/components/IntegrationConfigForm.tsx
{config.type === 'grafana' && (
  <input id="integration-grafana-url" value={config.config.url || ''} />
)}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|---|---|---|---|
| No Grafana metrics graph | Dashboard→Panel→Query→Metric nodes | Phase 16 | Enables semantic expansion in Phase 17. |
| Raw variable JSON in Dashboard node | Variable nodes + classification | Phase 17 | Enables smart defaults for tools. |

**Deprecated/outdated:**
- None in Phase 17 scope; continue using existing Grafana client and parser.

## Open Questions

1. **Hierarchy mapping granularity**
   - What we know: UI should allow fallback mapping when tags are absent (UICF-04).
   - What's unclear: per-tag vs per-dashboard vs per-folder overrides (left to Claude’s discretion).
   - Recommendation: pick one granularity early in planning; keep config structure simple and forward-compatible.

## Sources

### Primary (HIGH confidence)
- `internal/integration/grafana/graph_builder.go` - current graph ingestion flow.
- `internal/integration/grafana/promql_parser.go` - PromQL parsing and label extraction.
- `internal/integration/grafana/dashboard_syncer.go` - sync lifecycle + dashboard parsing.
- `internal/integration/grafana/types.go` - integration config structure.
- `ui/src/components/IntegrationConfigForm.tsx` - Grafana UI configuration entry point.
- `.planning/phases/17-semantic-layer/17-CONTEXT.md` - locked decisions for service inference, hierarchy, variable classification.

### Secondary (MEDIUM confidence)
- `.planning/research/STACK-v1.3-grafana.md` - stack recommendations, existing architecture notes.
- `.planning/research/ARCHITECTURE-grafana-v1.3.md` - ingestion-time semantic extraction guidance.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - existing code already uses required stack.
- Architecture: HIGH - GraphBuilder/Syncer already in place.
- Pitfalls: MEDIUM - inferred from code behavior and existing patterns.

**Research date:** 2026-01-22
**Valid until:** 2026-02-21
