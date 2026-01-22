package grafana

import (
	"context"
	"encoding/json"
	"fmt"
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
	graphClient graph.Client
	parser      PromQLParserInterface
	logger      *logging.Logger
}

// NewGraphBuilder creates a new GraphBuilder instance
func NewGraphBuilder(graphClient graph.Client, logger *logging.Logger) *GraphBuilder {
	return &GraphBuilder{
		graphClient: graphClient,
		parser:      &defaultPromQLParser{},
		logger:      logger,
	}
}

// defaultPromQLParser wraps ExtractFromPromQL for production use
type defaultPromQLParser struct{}

// Parse extracts semantic information from a PromQL query
func (p *defaultPromQLParser) Parse(queryStr string) (*QueryExtraction, error) {
	return ExtractFromPromQL(queryStr)
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

	dashboardQuery := `
		MERGE (d:Dashboard {uid: $uid})
		ON CREATE SET
			d.title = $title,
			d.version = $version,
			d.tags = $tags,
			d.firstSeen = $now,
			d.lastSeen = $now,
			d.variables = $variables
		ON MATCH SET
			d.title = $title,
			d.version = $version,
			d.tags = $tags,
			d.lastSeen = $now,
			d.variables = $variables
	`

	_, err = gb.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: dashboardQuery,
		Parameters: map[string]interface{}{
			"uid":       dashboard.UID,
			"title":     dashboard.Title,
			"version":   dashboard.Version,
			"tags":      dashboard.Tags,
			"now":       now,
			"variables": string(variablesJSON),
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
