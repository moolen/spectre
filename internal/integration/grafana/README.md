Complete Grafana Graph Overview Query

  redis-cli -p 6379 GRAPH.QUERY spectre "
  MATCH (d:Dashboard)
  OPTIONAL MATCH (d)-[:CONTAINS]->(p:Panel)-[:HAS]->(q:Query)
  OPTIONAL MATCH (q)-[:USES]->(m:Metric)
  OPTIONAL MATCH (d)-[:HAS_VARIABLE]->(v:Variable)
  WITH d, count(DISTINCT p) AS panels, count(DISTINCT q) AS queries,
       count(DISTINCT m) AS metrics, count(DISTINCT v) AS vars
  RETURN
    'DASHBOARD' AS type,
    d.title AS name,
    coalesce(d.hierarchy, '-') AS state_or_hierarchy,
    panels, queries, metrics, vars
  ORDER BY d.title

  UNION ALL

  MATCH (a:Alert)
  RETURN
    'ALERT' AS type,
    a.title AS name,
    coalesce(a.state, 'unknown') AS state_or_hierarchy,
    0 AS panels, 0 AS queries, 0 AS metrics, 0 AS vars
  ORDER BY a.state DESC, a.title
  "

  ---
  Quick Stats Query

  redis-cli -p 6379 GRAPH.QUERY spectre "
  MATCH (d:Dashboard) WITH count(d) AS dashboards
  MATCH (p:Panel) WITH dashboards, count(p) AS panels
  MATCH (q:Query) WITH dashboards, panels, count(q) AS queries
  MATCH (m:Metric) WITH dashboards, panels, queries, count(m) AS metrics
  MATCH (v:Variable) WITH dashboards, panels, queries, metrics, count(v) AS variables
  MATCH (a:Alert) WITH dashboards, panels, queries, metrics, variables, count(a) AS alerts
  MATCH (a2:Alert) WHERE a2.state = 'firing' WITH dashboards, panels, queries, metrics, variables, alerts, count(a2) AS firing
  MATCH (a3:Alert) WHERE a3.state = 'pending'
  RETURN dashboards, panels, queries, metrics, variables, alerts, firing AS alerts_firing, count(a3) AS alerts_pending
  "

● Bash(redis-cli -p 6379 GRAPH.QUERY spectre "
      MATCH (d:Dashboard) WITH count(d) AS dashboards…)
  ⎿  dashboards
     panels
     queries
     … +15 lines (ctrl+o to expand)

● Summary

  | Entity     | Count |
  |------------|-------|
  | Dashboards | 73    |
  | Panels     | 830   |
  | Queries    | 931   |
  | Metrics    | 12    |
  | Variables  | 269   |
  | Alerts     | 30    |
  | └─ Firing  | 20    |
  | └─ Pending | 10    |

  Graph Structure:
  Dashboard ──CONTAINS──> Panel ──HAS──> Query ──USES──> Metric
      │
      └──HAS_VARIABLE──> Variable

  Alert (standalone, with state/labels/condition)