package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// GrafanaDashboard represents the structure of a Grafana dashboard
type GrafanaDashboard struct {
	UID     string            `json:"uid"`
	Title   string            `json:"title"`
	Version int               `json:"version"`
	Tags    []string          `json:"tags"`
	Panels  []GrafanaPanel    `json:"panels"`
	Templating struct {
		List []interface{} `json:"list"` // Variable definitions as JSON
	} `json:"templating"`
}

// GrafanaPanel represents a panel within a Grafana dashboard
type GrafanaPanel struct {
	ID      int             `json:"id"`
	Title   string          `json:"title"`
	Type    string          `json:"type"`
	GridPos GrafanaGridPos  `json:"gridPos"`
	Targets []GrafanaTarget `json:"targets"`
}

// GrafanaGridPos represents the position of a panel in the dashboard grid
type GrafanaGridPos struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"` // width
	H int `json:"h"` // height
}

// GrafanaTarget represents a query target within a panel
type GrafanaTarget struct {
	RefID        string `json:"refId"`
	Expr         string `json:"expr"`         // PromQL expression
	DatasourceUID string `json:"datasource"`  // Can be UID or other identifier
}

// PromQLParserInterface defines the interface for PromQL parsing
type PromQLParserInterface interface {
	Parse(queryStr string) (*QueryExtraction, error)
}

// GraphBuilder creates graph nodes and edges from Grafana dashboard structure
type GraphBuilder struct {
	graphClient     graph.Client
	parser          PromQLParserInterface
	config          *Config
	integrationName string
	logger          *logging.Logger
}

// ServiceInference represents an inferred service from label selectors
type ServiceInference struct {
	Name         string
	Cluster      string
	Namespace    string
	InferredFrom string // Label name used (app/service/job)
}

// NewGraphBuilder creates a new GraphBuilder instance
func NewGraphBuilder(graphClient graph.Client, config *Config, integrationName string, logger *logging.Logger) *GraphBuilder {
	return &GraphBuilder{
		graphClient:     graphClient,
		parser:          &defaultPromQLParser{},
		config:          config,
		integrationName: integrationName,
		logger:          logger,
	}
}

// defaultPromQLParser wraps ExtractFromPromQL for production use
type defaultPromQLParser struct{}

// Parse extracts semantic information from a PromQL query
func (p *defaultPromQLParser) Parse(queryStr string) (*QueryExtraction, error) {
	return ExtractFromPromQL(queryStr)
}

// classifyHierarchy determines the hierarchy level of a dashboard based on tags and config mapping
// Priority: 1) explicit hierarchy tags (spectre:* or hierarchy:*), 2) HierarchyMap lookup, 3) default to "detail"
func (gb *GraphBuilder) classifyHierarchy(tags []string) string {
	// 1. Check for explicit hierarchy tags (primary signal)
	for _, tag := range tags {
		tagLower := strings.ToLower(tag)
		// Support both spectre:* and hierarchy:* formats
		if tagLower == "spectre:overview" || tagLower == "hierarchy:overview" {
			return "overview"
		}
		if tagLower == "spectre:drilldown" || tagLower == "hierarchy:drilldown" {
			return "drilldown"
		}
		if tagLower == "spectre:detail" || tagLower == "hierarchy:detail" {
			return "detail"
		}
	}

	// 2. Fallback to HierarchyMap lookup (if config available)
	if gb.config != nil && len(gb.config.HierarchyMap) > 0 {
		for _, tag := range tags {
			if level, exists := gb.config.HierarchyMap[tag]; exists {
				return level
			}
		}
	}

	// 3. Default to "detail" when no signals present
	return "detail"
}

// classifyVariable classifies a dashboard variable by its name pattern
// Returns: "scoping", "entity", "detail", or "unknown"
func classifyVariable(name string) string {
	// Convert to lowercase for case-insensitive matching
	lowerName := strings.ToLower(name)

	// Scoping variables: cluster, region, env, environment, datacenter, zone
	scopingPatterns := []string{"cluster", "region", "env", "environment", "datacenter", "zone"}
	for _, pattern := range scopingPatterns {
		if strings.Contains(lowerName, pattern) {
			return "scoping"
		}
	}

	// Entity variables: service, namespace, app, application, deployment, pod, container
	entityPatterns := []string{"service", "namespace", "app", "application", "deployment", "pod", "container"}
	for _, pattern := range entityPatterns {
		if strings.Contains(lowerName, pattern) {
			return "entity"
		}
	}

	// Detail variables: instance, node, host, endpoint, handler, path
	detailPatterns := []string{"instance", "node", "host", "endpoint", "handler", "path"}
	for _, pattern := range detailPatterns {
		if strings.Contains(lowerName, pattern) {
			return "detail"
		}
	}

	// Unknown if no pattern matches
	return "unknown"
}

// createVariableNodes creates Variable nodes from dashboard Templating.List
// Returns the number of variables created
func (gb *GraphBuilder) createVariableNodes(ctx context.Context, dashboardUID string, variables []interface{}, now int64) int {
	if len(variables) == 0 {
		return 0
	}

	variableCount := 0
	for _, v := range variables {
		// Parse variable as JSON map
		varMap, ok := v.(map[string]interface{})
		if !ok {
			gb.logger.Warn("Skipping malformed variable in dashboard %s: not a map", dashboardUID)
			continue
		}

		// Extract name and type fields
		name, hasName := varMap["name"].(string)
		if !hasName || name == "" {
			gb.logger.Warn("Skipping variable in dashboard %s: missing name field", dashboardUID)
			continue
		}

		// Type is optional, default to "unknown"
		varType := "unknown"
		if typeVal, hasType := varMap["type"].(string); hasType {
			varType = typeVal
		}

		// Classify the variable
		classification := classifyVariable(name)

		// Create Variable node with MERGE (upsert semantics)
		variableQuery := `
			MERGE (v:Variable {dashboardUID: $dashboardUID, name: $name})
			ON CREATE SET
				v.type = $type,
				v.classification = $classification,
				v.firstSeen = $now,
				v.lastSeen = $now
			ON MATCH SET
				v.type = $type,
				v.classification = $classification,
				v.lastSeen = $now
			WITH v
			MATCH (d:Dashboard {uid: $dashboardUID})
			MERGE (d)-[:HAS_VARIABLE]->(v)
		`

		_, err := gb.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
			Query: variableQuery,
			Parameters: map[string]interface{}{
				"dashboardUID":  dashboardUID,
				"name":          name,
				"type":          varType,
				"classification": classification,
				"now":           now,
			},
		})
		if err != nil {
			gb.logger.Warn("Failed to create variable node %s for dashboard %s: %v", name, dashboardUID, err)
			continue
		}

		variableCount++
	}

	return variableCount
}

// CreateDashboardGraph creates or updates dashboard nodes and all related structure in the graph
func (gb *GraphBuilder) CreateDashboardGraph(ctx context.Context, dashboard *GrafanaDashboard) error {
	now := time.Now().UnixNano()

	// 1. Update Dashboard node with MERGE (upsert semantics)
	gb.logger.Debug("Creating/updating Dashboard node: %s (version: %d)", dashboard.UID, dashboard.Version)

	// Marshal variables to JSON string for storage
	variablesJSON, err := json.Marshal(dashboard.Templating.List)
	if err != nil {
		gb.logger.Warn("Failed to marshal dashboard variables: %v", err)
		variablesJSON = []byte("[]")
	}

	// Classify dashboard hierarchy level
	hierarchyLevel := gb.classifyHierarchy(dashboard.Tags)

	dashboardQuery := `
		MERGE (d:Dashboard {uid: $uid})
		ON CREATE SET
			d.title = $title,
			d.version = $version,
			d.tags = $tags,
			d.hierarchyLevel = $hierarchyLevel,
			d.firstSeen = $now,
			d.lastSeen = $now,
			d.variables = $variables
		ON MATCH SET
			d.title = $title,
			d.version = $version,
			d.tags = $tags,
			d.hierarchyLevel = $hierarchyLevel,
			d.lastSeen = $now,
			d.variables = $variables
	`

	_, err = gb.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: dashboardQuery,
		Parameters: map[string]interface{}{
			"uid":            dashboard.UID,
			"title":          dashboard.Title,
			"version":        dashboard.Version,
			"tags":           dashboard.Tags,
			"hierarchyLevel": hierarchyLevel,
			"now":            now,
			"variables":      string(variablesJSON),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create dashboard node: %w", err)
	}

	// 2. Process each panel
	for _, panel := range dashboard.Panels {
		if err := gb.createPanelGraph(ctx, dashboard, panel, now); err != nil {
			// Log error but continue with other panels (graceful degradation)
			gb.logger.Warn("Failed to create panel graph for dashboard %s, panel %d: %v",
				dashboard.UID, panel.ID, err)
			continue
		}
	}

	// 3. Process dashboard variables
	variableCount := gb.createVariableNodes(ctx, dashboard.UID, dashboard.Templating.List, now)
	if variableCount > 0 {
		gb.logger.Debug("Created %d variables for dashboard %s", variableCount, dashboard.UID)
	}

	gb.logger.Debug("Successfully created dashboard graph for %s with %d panels",
		dashboard.UID, len(dashboard.Panels))
	return nil
}

// createPanelGraph creates a panel node and all its queries
func (gb *GraphBuilder) createPanelGraph(ctx context.Context, dashboard *GrafanaDashboard, panel GrafanaPanel, now int64) error {
	// Create unique panel ID: dashboardUID + panelID
	panelID := fmt.Sprintf("%s-%d", dashboard.UID, panel.ID)

	// 1. Create Panel node with MERGE
	panelQuery := `
		MATCH (d:Dashboard {uid: $dashboardUID})
		MERGE (p:Panel {id: $panelID})
		ON CREATE SET
			p.dashboardUID = $dashboardUID,
			p.title = $title,
			p.type = $type,
			p.gridPosX = $gridPosX,
			p.gridPosY = $gridPosY
		ON MATCH SET
			p.dashboardUID = $dashboardUID,
			p.title = $title,
			p.type = $type,
			p.gridPosX = $gridPosX,
			p.gridPosY = $gridPosY
		MERGE (d)-[:CONTAINS]->(p)
	`

	_, err := gb.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: panelQuery,
		Parameters: map[string]interface{}{
			"dashboardUID": dashboard.UID,
			"panelID":      panelID,
			"title":        panel.Title,
			"type":         panel.Type,
			"gridPosX":     panel.GridPos.X,
			"gridPosY":     panel.GridPos.Y,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create panel node: %w", err)
	}

	// 2. Process each query target
	for _, target := range panel.Targets {
		if err := gb.createQueryGraph(ctx, dashboard.UID, panelID, target, now); err != nil {
			// Log error but continue with other queries (graceful degradation)
			gb.logger.Warn("Failed to parse PromQL for query %s: %v (skipping query)", target.RefID, err)
			continue
		}
	}

	return nil
}

// inferServiceFromLabels infers service nodes from PromQL label selectors
// Label priority: app > service > job
// Service identity = {name, cluster, namespace}
func inferServiceFromLabels(labelSelectors map[string]string) []ServiceInference {
	// Extract cluster and namespace for scoping
	cluster := labelSelectors["cluster"]
	namespace := labelSelectors["namespace"]

	// Apply label priority: app > service > job
	// Check each label in priority order
	var inferences []ServiceInference

	if appName, hasApp := labelSelectors["app"]; hasApp {
		inferences = append(inferences, ServiceInference{
			Name:         appName,
			Cluster:      cluster,
			Namespace:    namespace,
			InferredFrom: "app",
		})
	}

	if serviceName, hasService := labelSelectors["service"]; hasService {
		// Only add if different from app (if app was present)
		if len(inferences) == 0 || inferences[0].Name != serviceName {
			inferences = append(inferences, ServiceInference{
				Name:         serviceName,
				Cluster:      cluster,
				Namespace:    namespace,
				InferredFrom: "service",
			})
		}
	}

	if jobName, hasJob := labelSelectors["job"]; hasJob {
		// Only add if different from already inferred services
		isDuplicate := false
		for _, inf := range inferences {
			if inf.Name == jobName {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			inferences = append(inferences, ServiceInference{
				Name:         jobName,
				Cluster:      cluster,
				Namespace:    namespace,
				InferredFrom: "job",
			})
		}
	}

	// If no service labels found, return Unknown service
	if len(inferences) == 0 {
		inferences = append(inferences, ServiceInference{
			Name:         "Unknown",
			Cluster:      cluster,
			Namespace:    namespace,
			InferredFrom: "none",
		})
	}

	return inferences
}

// createServiceNodes creates or updates Service nodes and TRACKS edges
func (gb *GraphBuilder) createServiceNodes(ctx context.Context, queryID string, inferences []ServiceInference, now int64) error {
	for _, inference := range inferences {
		// Use MERGE for upsert semantics
		// Service identity = {name, cluster, namespace}
		serviceQuery := `
			MATCH (q:Query {id: $queryID})
			MATCH (q)-[:USES]->(m:Metric)
			MERGE (s:Service {name: $name, cluster: $cluster, namespace: $namespace})
			ON CREATE SET
				s.inferredFrom = $inferredFrom,
				s.firstSeen = $now,
				s.lastSeen = $now
			ON MATCH SET
				s.inferredFrom = $inferredFrom,
				s.lastSeen = $now
			MERGE (m)-[:TRACKS]->(s)
		`

		_, err := gb.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
			Query: serviceQuery,
			Parameters: map[string]interface{}{
				"queryID":      queryID,
				"name":         inference.Name,
				"cluster":      inference.Cluster,
				"namespace":    inference.Namespace,
				"inferredFrom": inference.InferredFrom,
				"now":          now,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create service node %s: %w", inference.Name, err)
		}
	}

	return nil
}

// createQueryGraph creates a query node and its metric relationships
func (gb *GraphBuilder) createQueryGraph(ctx context.Context, dashboardUID, panelID string, target GrafanaTarget, now int64) error {
	// Create unique query ID: dashboardUID-panelID-refID
	queryID := fmt.Sprintf("%s-%s", panelID, target.RefID)

	// Parse PromQL to extract semantic information
	extraction, err := gb.parser.Parse(target.Expr)
	if err != nil {
		// If parsing fails completely, skip this query
		return fmt.Errorf("failed to parse PromQL: %w", err)
	}

	// Marshal aggregations and label selectors to JSON
	aggregationsJSON, _ := json.Marshal(extraction.Aggregations)
	labelSelectorsJSON, _ := json.Marshal(extraction.LabelSelectors)

	// 1. Create Query node with MERGE
	queryQuery := `
		MATCH (p:Panel {id: $panelID})
		MERGE (q:Query {id: $queryID})
		ON CREATE SET
			q.refId = $refId,
			q.rawPromQL = $rawPromQL,
			q.datasourceUID = $datasourceUID,
			q.aggregations = $aggregations,
			q.labelSelectors = $labelSelectors,
			q.hasVariables = $hasVariables
		ON MATCH SET
			q.refId = $refId,
			q.rawPromQL = $rawPromQL,
			q.datasourceUID = $datasourceUID,
			q.aggregations = $aggregations,
			q.labelSelectors = $labelSelectors,
			q.hasVariables = $hasVariables
		MERGE (p)-[:HAS]->(q)
	`

	_, err = gb.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: queryQuery,
		Parameters: map[string]interface{}{
			"panelID":        panelID,
			"queryID":        queryID,
			"refId":          target.RefID,
			"rawPromQL":      target.Expr,
			"datasourceUID":  target.DatasourceUID,
			"aggregations":   string(aggregationsJSON),
			"labelSelectors": string(labelSelectorsJSON),
			"hasVariables":   extraction.HasVariables,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create query node: %w", err)
	}

	// 2. Create Metric nodes and relationships
	// Skip if query has variables (metric names may be templated)
	if !extraction.HasVariables {
		for _, metricName := range extraction.MetricNames {
			if err := gb.createMetricNode(ctx, queryID, metricName, now); err != nil {
				gb.logger.Warn("Failed to create metric node %s: %v", metricName, err)
				// Continue with other metrics
				continue
			}
		}

		// 3. Infer Service nodes from label selectors
		inferences := inferServiceFromLabels(extraction.LabelSelectors)
		gb.logger.Debug("Inferred %d services from query %s", len(inferences), queryID)

		// 4. Create Service nodes and TRACKS edges
		if err := gb.createServiceNodes(ctx, queryID, inferences, now); err != nil {
			gb.logger.Warn("Failed to create service nodes for query %s: %v", queryID, err)
			// Continue despite error (graceful degradation)
		}
	}

	return nil
}

// createMetricNode creates or updates a metric node and links it to a query
func (gb *GraphBuilder) createMetricNode(ctx context.Context, queryID, metricName string, now int64) error {
	// Use MERGE for upsert semantics - Metric nodes are shared across dashboards
	metricQuery := `
		MATCH (q:Query {id: $queryID})
		MERGE (m:Metric {name: $name})
		ON CREATE SET
			m.firstSeen = $now,
			m.lastSeen = $now
		ON MATCH SET
			m.lastSeen = $now
		MERGE (q)-[:USES]->(m)
	`

	_, err := gb.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: metricQuery,
		Parameters: map[string]interface{}{
			"queryID": queryID,
			"name":    metricName,
			"now":     now,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create metric node: %w", err)
	}

	return nil
}

// DeletePanelsForDashboard removes all panels and queries for a dashboard
// Metric nodes are preserved (shared across dashboards)
func (gb *GraphBuilder) DeletePanelsForDashboard(ctx context.Context, dashboardUID string) error {
	gb.logger.Debug("Deleting panels for dashboard: %s", dashboardUID)

	// Delete panels and queries, but preserve metrics
	deleteQuery := `
		MATCH (d:Dashboard {uid: $uid})-[:CONTAINS]->(p:Panel)
		OPTIONAL MATCH (p)-[:HAS]->(q:Query)
		DETACH DELETE p, q
	`

	result, err := gb.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: deleteQuery,
		Parameters: map[string]interface{}{
			"uid": dashboardUID,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete panels: %w", err)
	}

	gb.logger.Debug("Deleted %d panels and %d queries for dashboard %s",
		result.Stats.NodesDeleted, result.Stats.RelationshipsDeleted, dashboardUID)
	return nil
}

// BuildAlertGraph creates or updates an Alert node and its metric relationships
func (gb *GraphBuilder) BuildAlertGraph(alertRule AlertRule) error {
	now := time.Now().UnixNano()

	gb.logger.Debug("Creating/updating Alert node: %s", alertRule.UID)

	// Extract first PromQL expression for condition display
	var firstCondition string
	for _, query := range alertRule.Data {
		if query.QueryType == "prometheus" && len(query.Model) > 0 {
			// Parse Model JSON to extract expr field
			var modelData map[string]interface{}
			if err := json.Unmarshal(query.Model, &modelData); err == nil {
				if expr, ok := modelData["expr"].(string); ok && expr != "" {
					firstCondition = expr
					break
				}
			}
		}
	}

	// Marshal labels and annotations to JSON
	labelsJSON, err := json.Marshal(alertRule.Labels)
	if err != nil {
		gb.logger.Warn("Failed to marshal alert labels: %v", err)
		labelsJSON = []byte("{}")
	}

	annotationsJSON, err := json.Marshal(alertRule.Annotations)
	if err != nil {
		gb.logger.Warn("Failed to marshal alert annotations: %v", err)
		annotationsJSON = []byte("{}")
	}

	// 1. Create/update Alert node with MERGE
	alertQuery := `
		MERGE (a:Alert {uid: $uid, integration: $integration})
		ON CREATE SET
			a.title = $title,
			a.folderTitle = $folderTitle,
			a.ruleGroup = $ruleGroup,
			a.condition = $condition,
			a.labels = $labels,
			a.annotations = $annotations,
			a.updated = $updated,
			a.firstSeen = $now,
			a.lastSeen = $now
		ON MATCH SET
			a.title = $title,
			a.folderTitle = $folderTitle,
			a.ruleGroup = $ruleGroup,
			a.condition = $condition,
			a.labels = $labels,
			a.annotations = $annotations,
			a.updated = $updated,
			a.lastSeen = $now
	`

	_, err = gb.graphClient.ExecuteQuery(context.Background(), graph.GraphQuery{
		Query: alertQuery,
		Parameters: map[string]interface{}{
			"uid":         alertRule.UID,
			"integration": gb.integrationName,
			"title":       alertRule.Title,
			"folderTitle": alertRule.FolderUID,
			"ruleGroup":   alertRule.RuleGroup,
			"condition":   firstCondition,
			"labels":      string(labelsJSON),
			"annotations": string(annotationsJSON),
			"updated":     alertRule.Updated.Format(time.RFC3339),
			"now":         now,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create alert node: %w", err)
	}

	// 2. Extract PromQL expressions and parse for metrics
	for _, query := range alertRule.Data {
		// Only process Prometheus queries
		if query.QueryType != "prometheus" {
			continue
		}

		// Parse Model JSON to extract expr field
		var modelData map[string]interface{}
		if err := json.Unmarshal(query.Model, &modelData); err != nil {
			gb.logger.Warn("Failed to parse alert query model for alert %s, query %s: %v (skipping query)",
				alertRule.UID, query.RefID, err)
			continue
		}

		expr, ok := modelData["expr"].(string)
		if !ok || expr == "" {
			gb.logger.Debug("No expr field in alert query model for alert %s, query %s (skipping)",
				alertRule.UID, query.RefID)
			continue
		}

		// Parse PromQL expression
		extraction, err := gb.parser.Parse(expr)
		if err != nil {
			// Log error but continue with other queries (graceful degradation)
			gb.logger.Warn("Failed to parse PromQL for alert %s, query %s: %v (skipping query)",
				alertRule.UID, query.RefID, err)
			continue
		}

		// Skip if query has variables (metric names may be templated)
		if extraction.HasVariables {
			gb.logger.Debug("Alert query %s has variables, skipping metric extraction", query.RefID)
			continue
		}

		// 3. Create Metric nodes and MONITORS edges
		for _, metricName := range extraction.MetricNames {
			if err := gb.createAlertMetricEdge(alertRule.UID, metricName, now); err != nil {
				// Log error but continue with other metrics (graceful degradation)
				gb.logger.Warn("Failed to create MONITORS edge for alert %s, metric %s: %v",
					alertRule.UID, metricName, err)
				continue
			}
		}
	}

	gb.logger.Debug("Successfully created alert graph for %s", alertRule.UID)
	return nil
}

// createAlertMetricEdge creates a Metric node and MONITORS edge from Alert to Metric
func (gb *GraphBuilder) createAlertMetricEdge(alertUID, metricName string, now int64) error {
	// Use MERGE for both Metric node and MONITORS edge
	query := `
		MATCH (a:Alert {uid: $alertUID, integration: $integration})
		MERGE (m:Metric {name: $metricName})
		ON CREATE SET
			m.firstSeen = $now,
			m.lastSeen = $now
		ON MATCH SET
			m.lastSeen = $now
		MERGE (a)-[:MONITORS]->(m)
	`

	_, err := gb.graphClient.ExecuteQuery(context.Background(), graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"alertUID":    alertUID,
			"integration": gb.integrationName,
			"metricName":  metricName,
			"now":         now,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create metric node and MONITORS edge: %w", err)
	}

	return nil
}
