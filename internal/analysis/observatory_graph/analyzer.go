package observatorygraph

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
)

// DefaultLimit is the default maximum number of SignalAnchors to return
const DefaultLimit = 100

// MaxLimit is the maximum allowed limit
const MaxLimit = 500

// RelationshipLimitMultiplier increases the limit for relationship queries
// since each SignalAnchor can have many relationships (e.g., universal metrics)
const RelationshipLimitMultiplier = 50

// Analyzer provides observatory graph analysis functionality
type Analyzer struct {
	graphClient graph.Client
}

// NewAnalyzer creates a new observatory graph analyzer
func NewAnalyzer(graphClient graph.Client) *Analyzer {
	return &Analyzer{
		graphClient: graphClient,
	}
}

// Analyze returns the observatory graph data
func (a *Analyzer) Analyze(ctx context.Context, input AnalyzeInput) (*ObservatoryGraphResponse, error) {
	startTime := time.Now()

	// Apply defaults
	if input.Limit <= 0 || input.Limit > MaxLimit {
		input.Limit = DefaultLimit
	}

	nodes := make([]Node, 0)
	edges := make([]Edge, 0)

	// Track node IDs to avoid duplicates
	nodeIDs := make(map[string]bool)

	// 1. Query SignalAnchors and their relationships
	signalNodes, signalEdges, err := a.querySignalAnchors(ctx, input, nodeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query signal anchors: %w", err)
	}
	nodes = append(nodes, signalNodes...)
	edges = append(edges, signalEdges...)

	// 2. Query related Dashboards, Panels, Queries
	dashboardNodes, dashboardEdges, err := a.queryDashboardHierarchy(ctx, input, nodeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query dashboard hierarchy: %w", err)
	}
	nodes = append(nodes, dashboardNodes...)
	edges = append(edges, dashboardEdges...)

	// 3. Query Alerts and their relationships
	alertNodes, alertEdges, err := a.queryAlerts(ctx, input, nodeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query alerts: %w", err)
	}
	nodes = append(nodes, alertNodes...)
	edges = append(edges, alertEdges...)

	executionMs := time.Since(startTime).Milliseconds()

	return &ObservatoryGraphResponse{
		Graph: Graph{
			Nodes: nodes,
			Edges: edges,
		},
		Metadata: GraphMetadata{
			NodeCount:        len(nodes),
			EdgeCount:        len(edges),
			QueryExecutionMs: executionMs,
		},
	}, nil
}

// querySignalAnchors queries SignalAnchor nodes and their related nodes/edges
// Uses separate queries to avoid FalkorDB crashes with complex OPTIONAL MATCH patterns
func (a *Analyzer) querySignalAnchors(ctx context.Context, input AnalyzeInput, nodeIDs map[string]bool) ([]Node, []Edge, error) {
	now := time.Now().Unix()

	params := map[string]any{
		"now":   now,
		"limit": input.Limit,
	}

	// Build WHERE clause
	whereClause := "WHERE s.expires_at > $now"
	if input.Integration != "" {
		whereClause += " AND s.integration = $integration"
		params["integration"] = input.Integration
	}
	if input.Namespace != "" {
		whereClause += " AND s.workload_namespace = $namespace"
		params["namespace"] = input.Namespace
	}
	if input.WorkloadName != "" {
		whereClause += " AND s.workload_name = $workload"
		params["workload"] = input.WorkloadName
	}

	nodes := make([]Node, 0)
	edges := make([]Edge, 0)

	// Query 1: Get SignalAnchors
	signalQuery := `
		MATCH (s:SignalAnchor)
		` + whereClause + `
		RETURN
			s.metric_name AS metric_name,
			s.workload_namespace AS workload_namespace,
			s.workload_name AS workload_name,
			s.role AS role,
			s.confidence AS confidence,
			s.quality_score AS quality_score,
			s.integration AS integration,
			s.dashboard_uid AS dashboard_uid,
			s.panel_id AS panel_id
		LIMIT $limit
	`

	result, err := a.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query:      signalQuery,
		Parameters: params,
	})
	if err != nil {
		return nil, nil, err
	}

	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col] = i
	}

	// Build signal ID map for relationship queries
	signalIDs := make(map[string]string) // metric:ns:wl -> signalID

	for _, row := range result.Rows {
		metricName := getStringValue(colIdx, row, "metric_name")
		ns := getStringValue(colIdx, row, "workload_namespace")
		wl := getStringValue(colIdx, row, "workload_name")
		signalKey := fmt.Sprintf("%s:%s:%s", metricName, ns, wl)
		signalID := fmt.Sprintf("signal:%s", signalKey)

		if !nodeIDs[signalID] {
			nodeIDs[signalID] = true
			signalIDs[signalKey] = signalID
			nodes = append(nodes, Node{
				ID:    signalID,
				Type:  NodeTypeSignalAnchor,
				Label: metricName,
				Properties: map[string]any{
					"metricName":        metricName,
					"workloadNamespace": ns,
					"workloadName":      wl,
					"role":              getStringValue(colIdx, row, "role"),
					"confidence":        getFloatValue(colIdx, row, "confidence"),
					"qualityScore":      getFloatValue(colIdx, row, "quality_score"),
					"integration":       getStringValue(colIdx, row, "integration"),
					"dashboardUID":      getStringValue(colIdx, row, "dashboard_uid"),
					"panelID":           getIntValue(colIdx, row, "panel_id"),
				},
			})
		}
	}

	// Query 2: Get MONITORS_WORKLOAD relationships
	// Use a higher limit for relationship queries since each SignalAnchor can have many relationships
	relationshipLimit := input.Limit * RelationshipLimitMultiplier
	workloadQuery := `
		MATCH (s:SignalAnchor)-[:MONITORS_WORKLOAD]->(w:ResourceIdentity)
		` + whereClause + `
		RETURN
			s.metric_name AS metric_name,
			s.workload_namespace AS workload_namespace,
			s.workload_name AS workload_name,
			w.uid AS workload_uid,
			w.kind AS workload_kind,
			w.name AS workload_name_full,
			w.namespace AS workload_ns_full
		LIMIT $relationshipLimit
	`

	workloadParams := make(map[string]any)
	for k, v := range params {
		workloadParams[k] = v
	}
	workloadParams["relationshipLimit"] = relationshipLimit

	workloadResult, err := a.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query:      workloadQuery,
		Parameters: workloadParams,
	})
	if err == nil && workloadResult != nil {
		wColIdx := make(map[string]int)
		for i, col := range workloadResult.Columns {
			wColIdx[col] = i
		}

		for _, row := range workloadResult.Rows {
			metricName := getStringValue(wColIdx, row, "metric_name")
			ns := getStringValue(wColIdx, row, "workload_namespace")
			wl := getStringValue(wColIdx, row, "workload_name")
			signalKey := fmt.Sprintf("%s:%s:%s", metricName, ns, wl)
			signalID := signalIDs[signalKey]
			if signalID == "" {
				signalID = fmt.Sprintf("signal:%s", signalKey)
			}

			workloadUID := getStringValue(wColIdx, row, "workload_uid")
			if workloadUID != "" && !nodeIDs[workloadUID] {
				nodeIDs[workloadUID] = true
				nodes = append(nodes, Node{
					ID:    workloadUID,
					Type:  NodeTypeWorkload,
					Label: getStringValue(wColIdx, row, "workload_name_full"),
					Properties: map[string]any{
						"kind":      getStringValue(wColIdx, row, "workload_kind"),
						"namespace": getStringValue(wColIdx, row, "workload_ns_full"),
					},
				})
			}
			if workloadUID != "" {
				edgeID := fmt.Sprintf("%s->%s", signalID, workloadUID)
				edges = append(edges, Edge{
					ID:               edgeID,
					Source:           signalID,
					Target:           workloadUID,
					RelationshipType: EdgeTypeMonitorsWorkload,
				})
			}
		}
	}

	// Query 3: Get CORRELATES_WITH relationships
	alertQuery := `
		MATCH (s:SignalAnchor)-[:CORRELATES_WITH]->(a:Alert)
		` + whereClause + `
		RETURN
			s.metric_name AS metric_name,
			s.workload_namespace AS workload_namespace,
			s.workload_name AS workload_name,
			a.uid AS alert_uid,
			a.title AS alert_title
		LIMIT $limit
	`

	alertResult, err := a.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query:      alertQuery,
		Parameters: params,
	})
	if err == nil && alertResult != nil {
		aColIdx := make(map[string]int)
		for i, col := range alertResult.Columns {
			aColIdx[col] = i
		}

		for _, row := range alertResult.Rows {
			metricName := getStringValue(aColIdx, row, "metric_name")
			ns := getStringValue(aColIdx, row, "workload_namespace")
			wl := getStringValue(aColIdx, row, "workload_name")
			signalKey := fmt.Sprintf("%s:%s:%s", metricName, ns, wl)
			signalID := signalIDs[signalKey]
			if signalID == "" {
				signalID = fmt.Sprintf("signal:%s", signalKey)
			}

			alertUID := getStringValue(aColIdx, row, "alert_uid")
			if alertUID != "" && !nodeIDs[alertUID] {
				nodeIDs[alertUID] = true
				nodes = append(nodes, Node{
					ID:    alertUID,
					Type:  NodeTypeAlert,
					Label: getStringValue(aColIdx, row, "alert_title"),
					Properties: map[string]any{
						"uid":   alertUID,
						"title": getStringValue(aColIdx, row, "alert_title"),
					},
				})
			}
			if alertUID != "" {
				edgeID := fmt.Sprintf("%s->%s", signalID, alertUID)
				edges = append(edges, Edge{
					ID:               edgeID,
					Source:           signalID,
					Target:           alertUID,
					RelationshipType: EdgeTypeCorrelatesWith,
				})
			}
		}
	}

	// Query 4: Get HAS_BASELINE relationships (only if requested)
	if input.IncludeBaselines {
		baselineQuery := `
			MATCH (s:SignalAnchor)-[:HAS_BASELINE]->(b:SignalBaseline)
			` + whereClause + `
			RETURN
				s.metric_name AS metric_name,
				s.workload_namespace AS workload_namespace,
				s.workload_name AS workload_name,
				b.metric_name AS baseline_metric,
				b.mean AS baseline_mean,
				b.stddev AS baseline_stddev
			LIMIT $limit
		`

		baselineResult, err := a.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
			Query:      baselineQuery,
			Parameters: params,
		})
		if err == nil && baselineResult != nil {
			bColIdx := make(map[string]int)
			for i, col := range baselineResult.Columns {
				bColIdx[col] = i
			}

			for _, row := range baselineResult.Rows {
				metricName := getStringValue(bColIdx, row, "metric_name")
				ns := getStringValue(bColIdx, row, "workload_namespace")
				wl := getStringValue(bColIdx, row, "workload_name")
				signalKey := fmt.Sprintf("%s:%s:%s", metricName, ns, wl)
				signalID := signalIDs[signalKey]
				if signalID == "" {
					signalID = fmt.Sprintf("signal:%s", signalKey)
				}

				baselineMetric := getStringValue(bColIdx, row, "baseline_metric")
				baselineID := fmt.Sprintf("baseline:%s:%s:%s", baselineMetric, ns, wl)
				if !nodeIDs[baselineID] {
					nodeIDs[baselineID] = true
					nodes = append(nodes, Node{
						ID:    baselineID,
						Type:  NodeTypeSignalBaseline,
						Label: fmt.Sprintf("Baseline: %s", baselineMetric),
						Properties: map[string]any{
							"metricName": baselineMetric,
							"mean":       getFloatValue(bColIdx, row, "baseline_mean"),
							"stddev":     getFloatValue(bColIdx, row, "baseline_stddev"),
						},
					})
				}
				edgeID := fmt.Sprintf("%s->%s", signalID, baselineID)
				edges = append(edges, Edge{
					ID:               edgeID,
					Source:           signalID,
					Target:           baselineID,
					RelationshipType: EdgeTypeHasBaseline,
				})
			}
		}
	}

	return nodes, edges, nil
}

// queryDashboardHierarchy queries Dashboard, Panel, Query, Metric nodes
func (a *Analyzer) queryDashboardHierarchy(ctx context.Context, input AnalyzeInput, nodeIDs map[string]bool) ([]Node, []Edge, error) {
	params := map[string]any{
		"limit": input.Limit,
	}

	whereClause := ""
	if input.Integration != "" {
		whereClause = "WHERE d.integration = $integration"
		params["integration"] = input.Integration
	}

	query := `
		MATCH (d:Dashboard)
		` + whereClause + `
		OPTIONAL MATCH (d)-[:CONTAINS]->(p:Panel)
		OPTIONAL MATCH (p)-[:HAS]->(q:Query)
		OPTIONAL MATCH (q)-[:USES]->(m:Metric)
		RETURN DISTINCT
			d.uid AS dashboard_uid,
			d.title AS dashboard_title,
			d.folder AS dashboard_folder,
			p.id AS panel_id,
			p.title AS panel_title,
			p.type AS panel_type,
			q.id AS query_id,
			q.refId AS query_refid,
			q.rawPromQL AS query_promql,
			m.name AS metric_name
		LIMIT $limit
	`

	result, err := a.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query:      query,
		Parameters: params,
	})
	if err != nil {
		return nil, nil, err
	}

	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col] = i
	}

	nodes := make([]Node, 0)
	edges := make([]Edge, 0)

	for _, row := range result.Rows {
		// Dashboard node
		dashboardUID := getStringValue(colIdx, row, "dashboard_uid")
		if dashboardUID != "" && !nodeIDs[dashboardUID] {
			nodeIDs[dashboardUID] = true
			nodes = append(nodes, Node{
				ID:    dashboardUID,
				Type:  NodeTypeDashboard,
				Label: getStringValue(colIdx, row, "dashboard_title"),
				Properties: map[string]any{
					"uid":    dashboardUID,
					"title":  getStringValue(colIdx, row, "dashboard_title"),
					"folder": getStringValue(colIdx, row, "dashboard_folder"),
				},
			})
		}

		// Panel node
		panelID := getStringValue(colIdx, row, "panel_id")
		if panelID != "" && !nodeIDs[panelID] {
			nodeIDs[panelID] = true
			nodes = append(nodes, Node{
				ID:    panelID,
				Type:  NodeTypePanel,
				Label: getStringValue(colIdx, row, "panel_title"),
				Properties: map[string]any{
					"title": getStringValue(colIdx, row, "panel_title"),
					"type":  getStringValue(colIdx, row, "panel_type"),
				},
			})
			if dashboardUID != "" {
				edges = append(edges, Edge{
					ID:               fmt.Sprintf("%s->%s", dashboardUID, panelID),
					Source:           dashboardUID,
					Target:           panelID,
					RelationshipType: EdgeTypeContains,
				})
			}
		}

		// Query node
		queryID := getStringValue(colIdx, row, "query_id")
		if queryID != "" && !nodeIDs[queryID] {
			nodeIDs[queryID] = true
			promQL := getStringValue(colIdx, row, "query_promql")
			label := getStringValue(colIdx, row, "query_refid")
			if label == "" {
				label = "Query"
			}
			nodes = append(nodes, Node{
				ID:    queryID,
				Type:  NodeTypeQuery,
				Label: label,
				Properties: map[string]any{
					"refId":   getStringValue(colIdx, row, "query_refid"),
					"promQL":  promQL,
				},
			})
			if panelID != "" {
				edges = append(edges, Edge{
					ID:               fmt.Sprintf("%s->%s", panelID, queryID),
					Source:           panelID,
					Target:           queryID,
					RelationshipType: EdgeTypeHas,
				})
			}
		}

		// Metric node
		metricName := getStringValue(colIdx, row, "metric_name")
		metricID := fmt.Sprintf("metric:%s", metricName)
		if metricName != "" && !nodeIDs[metricID] {
			nodeIDs[metricID] = true
			nodes = append(nodes, Node{
				ID:    metricID,
				Type:  NodeTypeMetric,
				Label: metricName,
				Properties: map[string]any{
					"name": metricName,
				},
			})
		}
		if queryID != "" && metricName != "" {
			edges = append(edges, Edge{
				ID:               fmt.Sprintf("%s->%s", queryID, metricID),
				Source:           queryID,
				Target:           metricID,
				RelationshipType: EdgeTypeUses,
			})
		}
	}

	return nodes, edges, nil
}

// queryAlerts queries Alert nodes and their relationships
func (a *Analyzer) queryAlerts(ctx context.Context, input AnalyzeInput, nodeIDs map[string]bool) ([]Node, []Edge, error) {
	params := map[string]any{
		"limit": input.Limit,
	}

	whereClause := ""
	if input.Integration != "" {
		whereClause = "WHERE a.integration = $integration"
		params["integration"] = input.Integration
	}

	query := `
		MATCH (a:Alert)
		` + whereClause + `
		OPTIONAL MATCH (a)-[:MONITORS]->(m:Metric)
		OPTIONAL MATCH (a)-[:MONITORS]->(s:Service)
		RETURN DISTINCT
			a.uid AS alert_uid,
			a.title AS alert_title,
			a.folderTitle AS alert_folder,
			a.ruleGroup AS alert_group,
			m.name AS metric_name,
			s.name AS service_name,
			s.namespace AS service_namespace
		LIMIT $limit
	`

	result, err := a.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query:      query,
		Parameters: params,
	})
	if err != nil {
		return nil, nil, err
	}

	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col] = i
	}

	nodes := make([]Node, 0)
	edges := make([]Edge, 0)

	for _, row := range result.Rows {
		// Alert node
		alertUID := getStringValue(colIdx, row, "alert_uid")
		if alertUID != "" && !nodeIDs[alertUID] {
			nodeIDs[alertUID] = true
			nodes = append(nodes, Node{
				ID:    alertUID,
				Type:  NodeTypeAlert,
				Label: getStringValue(colIdx, row, "alert_title"),
				Properties: map[string]any{
					"uid":       alertUID,
					"title":     getStringValue(colIdx, row, "alert_title"),
					"folder":    getStringValue(colIdx, row, "alert_folder"),
					"ruleGroup": getStringValue(colIdx, row, "alert_group"),
				},
			})
		}

		// Metric relationship
		metricName := getStringValue(colIdx, row, "metric_name")
		if metricName != "" {
			metricID := fmt.Sprintf("metric:%s", metricName)
			if !nodeIDs[metricID] {
				nodeIDs[metricID] = true
				nodes = append(nodes, Node{
					ID:    metricID,
					Type:  NodeTypeMetric,
					Label: metricName,
					Properties: map[string]any{
						"name": metricName,
					},
				})
			}
			edges = append(edges, Edge{
				ID:               fmt.Sprintf("%s->%s", alertUID, metricID),
				Source:           alertUID,
				Target:           metricID,
				RelationshipType: EdgeTypeMonitors,
			})
		}

		// Service relationship
		serviceName := getStringValue(colIdx, row, "service_name")
		if serviceName != "" {
			serviceNs := getStringValue(colIdx, row, "service_namespace")
			serviceID := fmt.Sprintf("service:%s:%s", serviceNs, serviceName)
			if !nodeIDs[serviceID] {
				nodeIDs[serviceID] = true
				nodes = append(nodes, Node{
					ID:    serviceID,
					Type:  NodeTypeService,
					Label: serviceName,
					Properties: map[string]any{
						"name":      serviceName,
						"namespace": serviceNs,
					},
				})
			}
			edges = append(edges, Edge{
				ID:               fmt.Sprintf("%s->%s", alertUID, serviceID),
				Source:           alertUID,
				Target:           serviceID,
				RelationshipType: EdgeTypeMonitors,
			})
		}
	}

	return nodes, edges, nil
}

// Helper functions for extracting values from query results

func getStringValue(colIdx map[string]int, row []any, col string) string {
	if idx, ok := colIdx[col]; ok && idx < len(row) {
		if v, ok := row[idx].(string); ok {
			return v
		}
	}
	return ""
}

func getFloatValue(colIdx map[string]int, row []any, col string) float64 {
	if idx, ok := colIdx[col]; ok && idx < len(row) {
		switch v := row[idx].(type) {
		case float64:
			return v
		case int64:
			return float64(v)
		case int:
			return float64(v)
		}
	}
	return 0
}

func getIntValue(colIdx map[string]int, row []any, col string) int {
	if idx, ok := colIdx[col]; ok && idx < len(row) {
		switch v := row[idx].(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}
