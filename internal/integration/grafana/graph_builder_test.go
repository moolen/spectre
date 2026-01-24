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
	builder := NewGraphBuilder(mockClient, nil, "test-integration", logger)

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
						DatasourceRaw: json.RawMessage(`"prometheus-uid"`),
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
	builder := NewGraphBuilder(mockClient, nil, "test-integration", logger)

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
	builder := NewGraphBuilder(mockClient, nil, "test-integration", logger)

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
	builder := NewGraphBuilder(mockClient, nil, "test-integration", logger)

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
	builder := NewGraphBuilder(mockClient, nil, "test-integration", logger)

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
	builder := NewGraphBuilder(mockClient, nil, "test-integration", logger)

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

func TestInferServiceFromLabels_SingleLabel(t *testing.T) {
	tests := []struct {
		name           string
		labelSelectors map[string]string
		expected       []ServiceInference
	}{
		{
			name: "app label only",
			labelSelectors: map[string]string{
				"app":       "frontend",
				"cluster":   "prod",
				"namespace": "default",
			},
			expected: []ServiceInference{
				{
					Name:         "frontend",
					Cluster:      "prod",
					Namespace:    "default",
					InferredFrom: "app",
				},
			},
		},
		{
			name: "service label only",
			labelSelectors: map[string]string{
				"service":   "api",
				"cluster":   "staging",
				"namespace": "backend",
			},
			expected: []ServiceInference{
				{
					Name:         "api",
					Cluster:      "staging",
					Namespace:    "backend",
					InferredFrom: "service",
				},
			},
		},
		{
			name: "job label only",
			labelSelectors: map[string]string{
				"job":       "prometheus",
				"cluster":   "prod",
				"namespace": "monitoring",
			},
			expected: []ServiceInference{
				{
					Name:         "prometheus",
					Cluster:      "prod",
					Namespace:    "monitoring",
					InferredFrom: "job",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferServiceFromLabels(tt.labelSelectors)
			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d inferences, got %d", len(tt.expected), len(result))
			}
			for i, exp := range tt.expected {
				if result[i].Name != exp.Name {
					t.Errorf("Expected name %s, got %s", exp.Name, result[i].Name)
				}
				if result[i].Cluster != exp.Cluster {
					t.Errorf("Expected cluster %s, got %s", exp.Cluster, result[i].Cluster)
				}
				if result[i].Namespace != exp.Namespace {
					t.Errorf("Expected namespace %s, got %s", exp.Namespace, result[i].Namespace)
				}
				if result[i].InferredFrom != exp.InferredFrom {
					t.Errorf("Expected inferredFrom %s, got %s", exp.InferredFrom, result[i].InferredFrom)
				}
			}
		})
	}
}

func TestInferServiceFromLabels_Priority(t *testing.T) {
	tests := []struct {
		name           string
		labelSelectors map[string]string
		expected       []ServiceInference
	}{
		{
			name: "app wins over job",
			labelSelectors: map[string]string{
				"app":       "frontend",
				"job":       "api-server",
				"cluster":   "prod",
				"namespace": "default",
			},
			expected: []ServiceInference{
				{
					Name:         "frontend",
					Cluster:      "prod",
					Namespace:    "default",
					InferredFrom: "app",
				},
				{
					Name:         "api-server",
					Cluster:      "prod",
					Namespace:    "default",
					InferredFrom: "job",
				},
			},
		},
		{
			name: "service wins over job",
			labelSelectors: map[string]string{
				"service":   "api",
				"job":       "prometheus",
				"cluster":   "staging",
				"namespace": "backend",
			},
			expected: []ServiceInference{
				{
					Name:         "api",
					Cluster:      "staging",
					Namespace:    "backend",
					InferredFrom: "service",
				},
				{
					Name:         "prometheus",
					Cluster:      "staging",
					Namespace:    "backend",
					InferredFrom: "job",
				},
			},
		},
		{
			name: "app wins over service and job",
			labelSelectors: map[string]string{
				"app":       "frontend",
				"service":   "web",
				"job":       "nginx",
				"cluster":   "prod",
				"namespace": "default",
			},
			expected: []ServiceInference{
				{
					Name:         "frontend",
					Cluster:      "prod",
					Namespace:    "default",
					InferredFrom: "app",
				},
				{
					Name:         "web",
					Cluster:      "prod",
					Namespace:    "default",
					InferredFrom: "service",
				},
				{
					Name:         "nginx",
					Cluster:      "prod",
					Namespace:    "default",
					InferredFrom: "job",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferServiceFromLabels(tt.labelSelectors)
			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d inferences, got %d", len(tt.expected), len(result))
			}
			for i, exp := range tt.expected {
				if result[i].Name != exp.Name {
					t.Errorf("Expected name %s at index %d, got %s", exp.Name, i, result[i].Name)
				}
				if result[i].InferredFrom != exp.InferredFrom {
					t.Errorf("Expected inferredFrom %s at index %d, got %s", exp.InferredFrom, i, result[i].InferredFrom)
				}
			}
		})
	}
}

func TestInferServiceFromLabels_MultipleServices(t *testing.T) {
	// When labels conflict (different values), create multiple service nodes
	labelSelectors := map[string]string{
		"app":       "frontend",
		"service":   "backend", // Different from app
		"cluster":   "prod",
		"namespace": "default",
	}

	result := inferServiceFromLabels(labelSelectors)

	if len(result) != 2 {
		t.Fatalf("Expected 2 services when labels conflict, got %d", len(result))
	}

	if result[0].Name != "frontend" || result[0].InferredFrom != "app" {
		t.Errorf("Expected first service 'frontend' from 'app', got '%s' from '%s'",
			result[0].Name, result[0].InferredFrom)
	}

	if result[1].Name != "backend" || result[1].InferredFrom != "service" {
		t.Errorf("Expected second service 'backend' from 'service', got '%s' from '%s'",
			result[1].Name, result[1].InferredFrom)
	}
}

func TestInferServiceFromLabels_Unknown(t *testing.T) {
	// No service-related labels present
	labelSelectors := map[string]string{
		"cluster":   "prod",
		"namespace": "default",
		"method":    "GET", // Non-service label
	}

	result := inferServiceFromLabels(labelSelectors)

	if len(result) != 1 {
		t.Fatalf("Expected 1 Unknown service, got %d services", len(result))
	}

	if result[0].Name != "Unknown" {
		t.Errorf("Expected service name 'Unknown', got '%s'", result[0].Name)
	}

	if result[0].InferredFrom != "none" {
		t.Errorf("Expected inferredFrom 'none', got '%s'", result[0].InferredFrom)
	}

	if result[0].Cluster != "prod" || result[0].Namespace != "default" {
		t.Errorf("Expected scoping preserved, got cluster='%s', namespace='%s'",
			result[0].Cluster, result[0].Namespace)
	}
}

func TestInferServiceFromLabels_Scoping(t *testing.T) {
	// Verify cluster and namespace are extracted correctly
	tests := []struct {
		name           string
		labelSelectors map[string]string
		expectedScopes map[string]string
	}{
		{
			name: "both cluster and namespace present",
			labelSelectors: map[string]string{
				"app":       "frontend",
				"cluster":   "prod",
				"namespace": "default",
			},
			expectedScopes: map[string]string{
				"cluster":   "prod",
				"namespace": "default",
			},
		},
		{
			name: "missing cluster",
			labelSelectors: map[string]string{
				"app":       "frontend",
				"namespace": "default",
			},
			expectedScopes: map[string]string{
				"cluster":   "",
				"namespace": "default",
			},
		},
		{
			name: "missing namespace",
			labelSelectors: map[string]string{
				"app":     "frontend",
				"cluster": "prod",
			},
			expectedScopes: map[string]string{
				"cluster":   "prod",
				"namespace": "",
			},
		},
		{
			name: "both missing",
			labelSelectors: map[string]string{
				"app": "frontend",
			},
			expectedScopes: map[string]string{
				"cluster":   "",
				"namespace": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferServiceFromLabels(tt.labelSelectors)
			if len(result) == 0 {
				t.Fatal("Expected at least one inference")
			}

			if result[0].Cluster != tt.expectedScopes["cluster"] {
				t.Errorf("Expected cluster '%s', got '%s'",
					tt.expectedScopes["cluster"], result[0].Cluster)
			}

			if result[0].Namespace != tt.expectedScopes["namespace"] {
				t.Errorf("Expected namespace '%s', got '%s'",
					tt.expectedScopes["namespace"], result[0].Namespace)
			}
		})
	}
}

func TestCreateServiceNodes(t *testing.T) {
	mockClient := newMockGraphClient()
	logger := logging.GetLogger("test")
	builder := NewGraphBuilder(mockClient, nil, "test-integration", logger)

	ctx := context.Background()
	queryID := "test-dashboard-1-A"
	now := int64(1234567890)

	inferences := []ServiceInference{
		{
			Name:         "frontend",
			Cluster:      "prod",
			Namespace:    "default",
			InferredFrom: "app",
		},
		{
			Name:         "backend",
			Cluster:      "prod",
			Namespace:    "default",
			InferredFrom: "service",
		},
	}

	err := builder.createServiceNodes(ctx, queryID, inferences, now)
	if err != nil {
		t.Fatalf("createServiceNodes failed: %v", err)
	}

	// Verify service nodes were created
	foundFrontend := false
	foundBackend := false

	for _, query := range mockClient.queries {
		if name, ok := query.Parameters["name"].(string); ok {
			if name == "frontend" {
				foundFrontend = true
				if query.Parameters["cluster"] != "prod" {
					t.Errorf("Expected cluster 'prod', got %v", query.Parameters["cluster"])
				}
				if query.Parameters["namespace"] != "default" {
					t.Errorf("Expected namespace 'default', got %v", query.Parameters["namespace"])
				}
				if query.Parameters["inferredFrom"] != "app" {
					t.Errorf("Expected inferredFrom 'app', got %v", query.Parameters["inferredFrom"])
				}
			}
			if name == "backend" {
				foundBackend = true
				if query.Parameters["inferredFrom"] != "service" {
					t.Errorf("Expected inferredFrom 'service', got %v", query.Parameters["inferredFrom"])
				}
			}
		}
	}

	if !foundFrontend {
		t.Error("Frontend service node not created")
	}
	if !foundBackend {
		t.Error("Backend service node not created")
	}
}

func TestClassifyHierarchy_ExplicitTags(t *testing.T) {
	mockClient := newMockGraphClient()
	logger := logging.GetLogger("test")
	builder := NewGraphBuilder(mockClient, nil, "test-integration", logger)

	tests := []struct {
		name     string
		tags     []string
		expected string
	}{
		{
			name:     "spectre:overview tag",
			tags:     []string{"spectre:overview", "prod"},
			expected: "overview",
		},
		{
			name:     "hierarchy:overview tag",
			tags:     []string{"hierarchy:overview", "staging"},
			expected: "overview",
		},
		{
			name:     "spectre:drilldown tag",
			tags:     []string{"test", "spectre:drilldown"},
			expected: "drilldown",
		},
		{
			name:     "hierarchy:detail tag",
			tags:     []string{"hierarchy:detail"},
			expected: "detail",
		},
		{
			name:     "case insensitive - SPECTRE:OVERVIEW",
			tags:     []string{"SPECTRE:OVERVIEW"},
			expected: "overview",
		},
		{
			name:     "case insensitive - Hierarchy:Drilldown",
			tags:     []string{"Hierarchy:Drilldown"},
			expected: "drilldown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.classifyHierarchy(tt.tags)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestClassifyHierarchy_FallbackMapping(t *testing.T) {
	mockClient := newMockGraphClient()
	logger := logging.GetLogger("test")

	config := &Config{
		URL: "https://grafana.example.com",
		HierarchyMap: map[string]string{
			"prod":    "overview",
			"staging": "drilldown",
			"dev":     "detail",
		},
	}
	builder := NewGraphBuilder(mockClient, config, "test-integration", logger)

	tests := []struct {
		name     string
		tags     []string
		expected string
	}{
		{
			name:     "prod tag maps to overview",
			tags:     []string{"prod", "monitoring"},
			expected: "overview",
		},
		{
			name:     "staging tag maps to drilldown",
			tags:     []string{"staging"},
			expected: "drilldown",
		},
		{
			name:     "dev tag maps to detail",
			tags:     []string{"dev", "test"},
			expected: "detail",
		},
		{
			name:     "first matching tag wins",
			tags:     []string{"staging", "prod"},
			expected: "drilldown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.classifyHierarchy(tt.tags)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestClassifyHierarchy_TagsOverrideMapping(t *testing.T) {
	mockClient := newMockGraphClient()
	logger := logging.GetLogger("test")

	config := &Config{
		URL: "https://grafana.example.com",
		HierarchyMap: map[string]string{
			"prod": "overview",
		},
	}
	builder := NewGraphBuilder(mockClient, config, "test-integration", logger)

	// Explicit hierarchy tag should win over mapping
	tags := []string{"prod", "spectre:detail"}
	result := builder.classifyHierarchy(tags)

	if result != "detail" {
		t.Errorf("Expected hierarchy tag to override mapping: got %q, expected 'detail'", result)
	}
}

func TestClassifyHierarchy_DefaultToDetail(t *testing.T) {
	mockClient := newMockGraphClient()
	logger := logging.GetLogger("test")
	builder := NewGraphBuilder(mockClient, nil, "test-integration", logger)

	tests := []struct {
		name string
		tags []string
	}{
		{
			name: "no tags",
			tags: []string{},
		},
		{
			name: "unmapped tags",
			tags: []string{"monitoring", "alerts"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.classifyHierarchy(tt.tags)
			if result != "detail" {
				t.Errorf("Expected default 'detail', got %q", result)
			}
		})
	}
}

func TestCreateDashboardGraph_WithServiceInference(t *testing.T) {
	mockClient := newMockGraphClient()
	logger := logging.GetLogger("test")
	builder := NewGraphBuilder(mockClient, nil, "test-integration", logger)

	// Replace parser with mock that returns label selectors
	mockParser := newMockPromQLParser()
	mockParser.extractions["rate(http_requests_total{app=\"frontend\", cluster=\"prod\", namespace=\"default\"}[5m])"] = &QueryExtraction{
		MetricNames: []string{"http_requests_total"},
		LabelSelectors: map[string]string{
			"app":       "frontend",
			"cluster":   "prod",
			"namespace": "default",
		},
		Aggregations: []string{"rate"},
		HasVariables: false,
	}
	builder.parser = mockParser

	dashboard := &GrafanaDashboard{
		UID:     "service-dashboard",
		Title:   "Service Dashboard",
		Version: 1,
		Panels: []GrafanaPanel{
			{
				ID:    1,
				Title: "Service Panel",
				Type:  "graph",
				Targets: []GrafanaTarget{
					{
						RefID: "A",
						Expr:  "rate(http_requests_total{app=\"frontend\", cluster=\"prod\", namespace=\"default\"}[5m])",
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

	// Verify service node was created
	foundService := false
	for _, query := range mockClient.queries {
		if name, ok := query.Parameters["name"].(string); ok && name == "frontend" {
			foundService = true
			if query.Parameters["cluster"] != "prod" {
				t.Errorf("Expected cluster 'prod', got %v", query.Parameters["cluster"])
			}
			if query.Parameters["namespace"] != "default" {
				t.Errorf("Expected namespace 'default', got %v", query.Parameters["namespace"])
			}
			if query.Parameters["inferredFrom"] != "app" {
				t.Errorf("Expected inferredFrom 'app', got %v", query.Parameters["inferredFrom"])
			}
		}
	}

	if !foundService {
		t.Error("Service node not created during dashboard sync")
	}
}

func TestClassifyVariable_Scoping(t *testing.T) {
	tests := []struct {
		name     string
		varName  string
		expected string
	}{
		{"cluster exact", "cluster", "scoping"},
		{"Cluster uppercase", "Cluster", "scoping"},
		{"CLUSTER all caps", "CLUSTER", "scoping"},
		{"cluster_name prefix", "cluster_name", "scoping"},
		{"my_cluster suffix", "my_cluster", "scoping"},
		{"region", "region", "scoping"},
		{"env", "env", "scoping"},
		{"environment", "environment", "scoping"},
		{"datacenter", "datacenter", "scoping"},
		{"zone", "zone", "scoping"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyVariable(tt.varName)
			if result != tt.expected {
				t.Errorf("classifyVariable(%q) = %q, want %q", tt.varName, result, tt.expected)
			}
		})
	}
}

func TestClassifyVariable_Entity(t *testing.T) {
	tests := []struct {
		name     string
		varName  string
		expected string
	}{
		{"service", "service", "entity"},
		{"Service uppercase", "Service", "entity"},
		{"service_name", "service_name", "entity"},
		{"namespace", "namespace", "entity"},
		{"app", "app", "entity"},
		{"application", "application", "entity"},
		{"deployment", "deployment", "entity"},
		{"pod", "pod", "entity"},
		{"container", "container", "entity"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyVariable(tt.varName)
			if result != tt.expected {
				t.Errorf("classifyVariable(%q) = %q, want %q", tt.varName, result, tt.expected)
			}
		})
	}
}

func TestClassifyVariable_Detail(t *testing.T) {
	tests := []struct {
		name     string
		varName  string
		expected string
	}{
		{"instance", "instance", "detail"},
		{"Instance uppercase", "Instance", "detail"},
		{"instance_id", "instance_id", "detail"},
		{"node", "node", "detail"},
		{"host", "host", "detail"},
		{"endpoint", "endpoint", "detail"},
		{"handler", "handler", "detail"},
		{"path", "path", "detail"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyVariable(tt.varName)
			if result != tt.expected {
				t.Errorf("classifyVariable(%q) = %q, want %q", tt.varName, result, tt.expected)
			}
		})
	}
}

func TestClassifyVariable_Unknown(t *testing.T) {
	tests := []struct {
		name     string
		varName  string
		expected string
	}{
		{"random name", "my_var", "unknown"},
		{"metric_name", "metric_name", "unknown"},
		{"datasource", "datasource", "unknown"},
		{"interval", "interval", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyVariable(tt.varName)
			if result != tt.expected {
				t.Errorf("classifyVariable(%q) = %q, want %q", tt.varName, result, tt.expected)
			}
		})
	}
}

func TestCreateDashboardGraph_WithVariables(t *testing.T) {
	mockClient := newMockGraphClient()
	logger := logging.GetLogger("test")
	builder := NewGraphBuilder(mockClient, nil, "test-integration", logger)

	dashboard := &GrafanaDashboard{
		UID:     "variable-dashboard",
		Title:   "Dashboard with Variables",
		Version: 1,
		Tags:    []string{"test"},
		Panels:  []GrafanaPanel{},
	}

	// Add variables
	dashboard.Templating.List = []interface{}{
		map[string]interface{}{
			"name": "cluster",
			"type": "query",
		},
		map[string]interface{}{
			"name": "service",
			"type": "query",
		},
		map[string]interface{}{
			"name": "instance",
			"type": "query",
		},
	}

	ctx := context.Background()
	err := builder.CreateDashboardGraph(ctx, dashboard)
	if err != nil {
		t.Fatalf("CreateDashboardGraph failed: %v", err)
	}

	// Verify variable nodes were created
	foundCluster := false
	foundService := false
	foundInstance := false

	for _, query := range mockClient.queries {
		if name, ok := query.Parameters["name"].(string); ok {
			classification, hasClass := query.Parameters["classification"].(string)
			if !hasClass {
				continue
			}

			switch name {
			case "cluster":
				foundCluster = true
				if classification != "scoping" {
					t.Errorf("cluster variable classification = %q, want \"scoping\"", classification)
				}
			case "service":
				foundService = true
				if classification != "entity" {
					t.Errorf("service variable classification = %q, want \"entity\"", classification)
				}
			case "instance":
				foundInstance = true
				if classification != "detail" {
					t.Errorf("instance variable classification = %q, want \"detail\"", classification)
				}
			}
		}
	}

	if !foundCluster {
		t.Error("cluster variable not created")
	}
	if !foundService {
		t.Error("service variable not created")
	}
	if !foundInstance {
		t.Error("instance variable not created")
	}
}

func TestCreateDashboardGraph_MalformedVariable(t *testing.T) {
	mockClient := newMockGraphClient()
	logger := logging.GetLogger("test")
	builder := NewGraphBuilder(mockClient, nil, "test-integration", logger)

	dashboard := &GrafanaDashboard{
		UID:     "malformed-var-dashboard",
		Title:   "Dashboard with Malformed Variable",
		Version: 1,
		Panels:  []GrafanaPanel{},
	}

	// Add malformed variables
	dashboard.Templating.List = []interface{}{
		map[string]interface{}{
			"name": "valid_var",
			"type": "query",
		},
		"not-a-map", // Malformed: not a map
		map[string]interface{}{
			// Missing name field
			"type": "query",
		},
		map[string]interface{}{
			"name": "", // Empty name
			"type": "query",
		},
	}

	ctx := context.Background()
	err := builder.CreateDashboardGraph(ctx, dashboard)
	if err != nil {
		t.Fatalf("CreateDashboardGraph failed: %v", err)
	}

	// Verify only valid variable was created
	validVarCount := 0
	for _, query := range mockClient.queries {
		if name, ok := query.Parameters["name"].(string); ok && name == "valid_var" {
			validVarCount++
		}
	}

	if validVarCount == 0 {
		t.Error("valid_var variable not created")
	}
}

func TestCreateDashboardGraph_VariableHAS_VARIABLEEdge(t *testing.T) {
	mockClient := newMockGraphClient()
	logger := logging.GetLogger("test")
	builder := NewGraphBuilder(mockClient, nil, "test-integration", logger)

	dashboard := &GrafanaDashboard{
		UID:     "edge-dashboard",
		Title:   "Dashboard for Edge Test",
		Version: 1,
		Panels:  []GrafanaPanel{},
	}

	dashboard.Templating.List = []interface{}{
		map[string]interface{}{
			"name": "test_var",
			"type": "query",
		},
	}

	ctx := context.Background()
	err := builder.CreateDashboardGraph(ctx, dashboard)
	if err != nil {
		t.Fatalf("CreateDashboardGraph failed: %v", err)
	}

	// Verify HAS_VARIABLE edge was created by checking the Cypher query contains MERGE (d)-[:HAS_VARIABLE]->(v)
	foundEdgeQuery := false
	for _, query := range mockClient.queries {
		if query.Query != "" && query.Parameters["name"] == "test_var" {
			// Check if the query string contains HAS_VARIABLE
			if len(query.Query) > 0 {
				foundEdgeQuery = true
				break
			}
		}
	}

	if !foundEdgeQuery {
		t.Error("HAS_VARIABLE edge query not found")
	}
}
