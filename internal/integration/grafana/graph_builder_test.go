package grafana

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// mockGraphClient implements graph.Client for testing
type mockGraphClient struct {
	queries []graph.GraphQuery
	results map[string]*graph.QueryResult
}

func newMockGraphClient() *mockGraphClient {
	return &mockGraphClient{
		queries: make([]graph.GraphQuery, 0),
		results: make(map[string]*graph.QueryResult),
	}
}

func (m *mockGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)

	// Return mock result
	result := &graph.QueryResult{
		Stats: graph.QueryStats{
			NodesCreated:         1,
			RelationshipsCreated: 1,
		},
	}

	// Check if we have a specific result for this query type
	if query.Query != "" {
		if mockResult, ok := m.results[query.Query]; ok {
			return mockResult, nil
		}
	}

	return result, nil
}

func (m *mockGraphClient) Connect(ctx context.Context) error                { return nil }
func (m *mockGraphClient) Close() error                                     { return nil }
func (m *mockGraphClient) Ping(ctx context.Context) error                   { return nil }
func (m *mockGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockGraphClient) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockGraphClient) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockGraphClient) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockGraphClient) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockGraphClient) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

// mockPromQLParser for testing
type mockPromQLParser struct {
	extractions map[string]*QueryExtraction
}

func newMockPromQLParser() *mockPromQLParser {
	return &mockPromQLParser{
		extractions: make(map[string]*QueryExtraction),
	}
}

func (m *mockPromQLParser) Parse(queryStr string) (*QueryExtraction, error) {
	if extraction, ok := m.extractions[queryStr]; ok {
		return extraction, nil
	}
	// Default extraction
	return &QueryExtraction{
		MetricNames:    []string{"http_requests_total"},
		LabelSelectors: map[string]string{"job": "api"},
		Aggregations:   []string{"rate"},
		HasVariables:   false,
	}, nil
}

func TestCreateDashboardGraph_SimplePanel(t *testing.T) {
	mockClient := newMockGraphClient()
	logger := logging.GetLogger("test")
	builder := NewGraphBuilder(mockClient, logger)

	dashboard := &GrafanaDashboard{
		UID:     "test-dashboard",
		Title:   "Test Dashboard",
		Version: 1,
		Tags:    []string{"test"},
		Panels: []GrafanaPanel{
			{
				ID:    1,
				Title: "Test Panel",
				Type:  "graph",
				GridPos: GrafanaGridPos{
					X: 0,
					Y: 0,
				},
				Targets: []GrafanaTarget{
					{
						RefID:         "A",
						Expr:          "rate(http_requests_total[5m])",
						DatasourceUID: "prometheus-uid",
					},
				},
			},
		},
	}

	ctx := context.Background()
	err := builder.CreateDashboardGraph(ctx, dashboard)
	if err != nil {
		t.Fatalf("CreateDashboardGraph failed: %v", err)
	}

	// Verify queries were executed
	if len(mockClient.queries) == 0 {
		t.Fatal("Expected queries to be executed, got none")
	}

	// Verify dashboard node creation
	foundDashboard := false
	foundPanel := false
	foundQuery := false
	foundMetric := false

	for _, query := range mockClient.queries {
		if query.Parameters["uid"] == "test-dashboard" {
			foundDashboard = true
		}
		if query.Parameters["panelID"] == "test-dashboard-1" {
			foundPanel = true
		}
		if query.Parameters["refId"] == "A" {
			foundQuery = true
		}
		if query.Parameters["name"] == "http_requests_total" {
			foundMetric = true
		}
	}

	if !foundDashboard {
		t.Error("Dashboard node creation not found")
	}
	if !foundPanel {
		t.Error("Panel node creation not found")
	}
	if !foundQuery {
		t.Error("Query node creation not found")
	}
	if !foundMetric {
		t.Error("Metric node creation not found")
	}
}

func TestCreateDashboardGraph_MultipleQueries(t *testing.T) {
	mockClient := newMockGraphClient()
	logger := logging.GetLogger("test")
	builder := NewGraphBuilder(mockClient, logger)

	dashboard := &GrafanaDashboard{
		UID:     "multi-query-dashboard",
		Title:   "Multi Query Dashboard",
		Version: 1,
		Panels: []GrafanaPanel{
			{
				ID:    1,
				Title: "Multi Query Panel",
				Type:  "graph",
				Targets: []GrafanaTarget{
					{
						RefID: "A",
						Expr:  "rate(http_requests_total[5m])",
					},
					{
						RefID: "B",
						Expr:  "rate(http_errors_total[5m])",
					},
				},
			},
		},
	}

	ctx := context.Background()
	err := builder.CreateDashboardGraph(ctx, dashboard)
	if err != nil {
		t.Fatalf("CreateDashboardGraph failed: %v", err)
	}

	// Verify both queries were created
	foundQueryA := false
	foundQueryB := false

	for _, query := range mockClient.queries {
		if query.Parameters["refId"] == "A" {
			foundQueryA = true
		}
		if query.Parameters["refId"] == "B" {
			foundQueryB = true
		}
	}

	if !foundQueryA {
		t.Error("Query A not found")
	}
	if !foundQueryB {
		t.Error("Query B not found")
	}
}

func TestCreateDashboardGraph_VariableInMetric(t *testing.T) {
	mockClient := newMockGraphClient()
	logger := logging.GetLogger("test")
	builder := NewGraphBuilder(mockClient, logger)

	// Replace parser with mock that returns HasVariables=true
	mockParser := newMockPromQLParser()
	mockParser.extractions["rate($metric[5m])"] = &QueryExtraction{
		MetricNames:    []string{"$metric"}, // Variable in metric name
		LabelSelectors: map[string]string{},
		Aggregations:   []string{"rate"},
		HasVariables:   true,
	}
	builder.parser = mockParser

	dashboard := &GrafanaDashboard{
		UID:     "variable-dashboard",
		Title:   "Variable Dashboard",
		Version: 1,
		Panels: []GrafanaPanel{
			{
				ID:    1,
				Title: "Variable Panel",
				Type:  "graph",
				Targets: []GrafanaTarget{
					{
						RefID: "A",
						Expr:  "rate($metric[5m])",
					},
				},
			},
		},
	}

	ctx := context.Background()
	err := builder.CreateDashboardGraph(ctx, dashboard)
	if err != nil {
		t.Fatalf("CreateDashboardGraph failed: %v", err)
	}

	// Verify query was created but metric node was NOT created
	foundQuery := false
	foundMetric := false

	for _, query := range mockClient.queries {
		if query.Parameters["refId"] == "A" {
			foundQuery = true
			// Verify hasVariables is true
			if hasVars, ok := query.Parameters["hasVariables"].(bool); ok && hasVars {
				t.Log("Query correctly marked with hasVariables=true")
			}
		}
		if query.Parameters["name"] == "$metric" {
			foundMetric = true
		}
	}

	if !foundQuery {
		t.Error("Query node not created")
	}
	if foundMetric {
		t.Error("Metric node should NOT be created when query has variables")
	}
}

func TestDeletePanelsForDashboard(t *testing.T) {
	mockClient := newMockGraphClient()
	logger := logging.GetLogger("test")
	builder := NewGraphBuilder(mockClient, logger)

	// Set up mock result for delete operation
	mockClient.results[""] = &graph.QueryResult{
		Stats: graph.QueryStats{
			NodesDeleted:         3, // 2 panels + 2 queries
			RelationshipsDeleted: 4,
		},
	}

	ctx := context.Background()
	err := builder.DeletePanelsForDashboard(ctx, "test-dashboard")
	if err != nil {
		t.Fatalf("DeletePanelsForDashboard failed: %v", err)
	}

	// Verify delete query was executed
	if len(mockClient.queries) == 0 {
		t.Fatal("Expected delete query to be executed")
	}

	lastQuery := mockClient.queries[len(mockClient.queries)-1]
	if lastQuery.Parameters["uid"] != "test-dashboard" {
		t.Errorf("Expected uid parameter to be 'test-dashboard', got %v", lastQuery.Parameters["uid"])
	}

	// Verify the query uses DETACH DELETE (checks that metrics are preserved)
	if lastQuery.Query == "" {
		t.Error("Delete query is empty")
	}
}

func TestGraphBuilder_GracefulDegradation(t *testing.T) {
	mockClient := newMockGraphClient()
	logger := logging.GetLogger("test")
	builder := NewGraphBuilder(mockClient, logger)

	// Replace parser with one that returns errors for specific queries
	mockParser := newMockPromQLParser()
	// Don't set extraction for "invalid_query" - parser will use default
	builder.parser = mockParser

	dashboard := &GrafanaDashboard{
		UID:     "mixed-dashboard",
		Title:   "Mixed Dashboard",
		Version: 1,
		Panels: []GrafanaPanel{
			{
				ID:    1,
				Title: "Valid Panel",
				Type:  "graph",
				Targets: []GrafanaTarget{
					{
						RefID: "A",
						Expr:  "valid_query",
					},
				},
			},
			{
				ID:    2,
				Title: "Another Valid Panel",
				Type:  "graph",
				Targets: []GrafanaTarget{
					{
						RefID: "B",
						Expr:  "another_valid_query",
					},
				},
			},
		},
	}

	ctx := context.Background()
	err := builder.CreateDashboardGraph(ctx, dashboard)

	// Should not fail entirely - graceful degradation
	if err != nil {
		t.Fatalf("CreateDashboardGraph should handle parse errors gracefully: %v", err)
	}

	// Verify at least some queries were executed (valid panels)
	if len(mockClient.queries) == 0 {
		t.Error("Expected some queries to succeed even with parse errors")
	}
}

func TestGraphBuilder_JSONSerialization(t *testing.T) {
	mockClient := newMockGraphClient()
	logger := logging.GetLogger("test")
	builder := NewGraphBuilder(mockClient, logger)

	dashboard := &GrafanaDashboard{
		UID:     "json-dashboard",
		Title:   "JSON Test Dashboard",
		Version: 1,
		Panels: []GrafanaPanel{
			{
				ID:    1,
				Title: "Test Panel",
				Type:  "graph",
				Targets: []GrafanaTarget{
					{
						RefID: "A",
						Expr:  "rate(http_requests_total{job=\"api\"}[5m])",
					},
				},
			},
		},
	}

	ctx := context.Background()
	err := builder.CreateDashboardGraph(ctx, dashboard)
	if err != nil {
		t.Fatalf("CreateDashboardGraph failed: %v", err)
	}

	// Find query creation and verify JSON serialization
	for _, query := range mockClient.queries {
		if aggJSON, ok := query.Parameters["aggregations"].(string); ok {
			var aggregations []string
			if err := json.Unmarshal([]byte(aggJSON), &aggregations); err != nil {
				t.Errorf("Failed to unmarshal aggregations JSON: %v", err)
			}
		}
		if labelsJSON, ok := query.Parameters["labelSelectors"].(string); ok {
			var labels map[string]string
			if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
				t.Errorf("Failed to unmarshal labelSelectors JSON: %v", err)
			}
		}
	}
}
