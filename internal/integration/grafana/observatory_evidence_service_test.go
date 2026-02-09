package grafana

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEvidenceGraphClient implements graph.Client for evidence service tests.
type mockEvidenceGraphClient struct {
	executeQueryFunc func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error)
	queries          []graph.GraphQuery
}

func newMockEvidenceGraphClient() *mockEvidenceGraphClient {
	return &mockEvidenceGraphClient{
		queries: make([]graph.GraphQuery, 0),
	}
}

func (m *mockEvidenceGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)
	if m.executeQueryFunc != nil {
		return m.executeQueryFunc(ctx, query)
	}
	return &graph.QueryResult{}, nil
}

// Implement remaining graph.Client interface methods
func (m *mockEvidenceGraphClient) Connect(ctx context.Context) error { return nil }
func (m *mockEvidenceGraphClient) Close() error                      { return nil }
func (m *mockEvidenceGraphClient) Ping(ctx context.Context) error    { return nil }
func (m *mockEvidenceGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockEvidenceGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockEvidenceGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockEvidenceGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockEvidenceGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockEvidenceGraphClient) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockEvidenceGraphClient) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockEvidenceGraphClient) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockEvidenceGraphClient) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockEvidenceGraphClient) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

// TestEvidenceService_GetCandidateCauses_WithUpstream tests returning upstream dependencies.
func TestEvidenceService_GetCandidateCauses_WithUpstream(t *testing.T) {
	logger := logging.GetLogger("test.evidence")
	mockGraph := newMockEvidenceGraphClient()

	// Mock returns upstream dependencies (1-hop and 2-hop)
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Check if this is the upstream deps query or recent changes query
		if query.Parameters["workload"] != nil {
			// Upstream dependencies query
			return &graph.QueryResult{
				Columns: []string{"hops1", "hops2"},
				Rows: [][]interface{}{
					{
						// 1-hop dependencies
						[]interface{}{
							map[string]interface{}{
								"kind":      "Service",
								"namespace": "default",
								"name":      "nginx-svc",
								"hops":      int64(1),
							},
						},
						// 2-hop dependencies
						[]interface{}{
							map[string]interface{}{
								"kind":      "Ingress",
								"namespace": "default",
								"name":      "nginx-ingress",
								"hops":      int64(2),
							},
						},
					},
				},
			}, nil
		}
		// Recent changes query - return empty
		return &graph.QueryResult{
			Columns: []string{"kind", "namespace", "name", "reason", "timestamp"},
			Rows:    [][]interface{}{},
		}, nil
	}

	service := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetCandidateCauses(ctx, "default", "nginx", "container_cpu_usage")

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify upstream dependencies
	assert.Len(t, result.UpstreamDeps, 2)

	// Check 1-hop dependency
	found1Hop := false
	for _, dep := range result.UpstreamDeps {
		if dep.HopsAway == 1 {
			assert.Equal(t, "Service", dep.Kind)
			assert.Equal(t, "default", dep.Namespace)
			assert.Equal(t, "nginx-svc", dep.Name)
			found1Hop = true
		}
	}
	assert.True(t, found1Hop, "should have 1-hop dependency")

	// Check 2-hop dependency
	found2Hop := false
	for _, dep := range result.UpstreamDeps {
		if dep.HopsAway == 2 {
			assert.Equal(t, "Ingress", dep.Kind)
			assert.Equal(t, "nginx-ingress", dep.Name)
			found2Hop = true
		}
	}
	assert.True(t, found2Hop, "should have 2-hop dependency")

	// Timestamp should be set
	assert.NotEmpty(t, result.Timestamp)
}

// TestEvidenceService_GetCandidateCauses_WithRecentChanges tests returning recent K8s changes.
func TestEvidenceService_GetCandidateCauses_WithRecentChanges(t *testing.T) {
	logger := logging.GetLogger("test.evidence")
	mockGraph := newMockEvidenceGraphClient()

	// Mock returns recent changes
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Check if this is the upstream deps query or recent changes query
		if query.Parameters["workload"] != nil {
			// Upstream dependencies query - return empty
			return &graph.QueryResult{
				Columns: []string{"hops1", "hops2"},
				Rows: [][]interface{}{
					{[]interface{}{}, []interface{}{}},
				},
			}, nil
		}
		// Recent changes query
		return &graph.QueryResult{
			Columns: []string{"kind", "namespace", "name", "reason", "timestamp"},
			Rows: [][]interface{}{
				{"Deployment", "default", "nginx", "DeploymentUpdated", "2026-01-30T00:00:00Z"},
				{"ConfigMap", "default", "nginx-config", "ConfigChanged", "2026-01-30T00:05:00Z"},
			},
		}, nil
	}

	service := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetCandidateCauses(ctx, "default", "nginx", "container_cpu_usage")

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify recent changes
	assert.Len(t, result.RecentChanges, 2)

	// Check first change (Deployment)
	assert.Equal(t, "Deployment", result.RecentChanges[0].Kind)
	assert.Equal(t, "default", result.RecentChanges[0].Namespace)
	assert.Equal(t, "nginx", result.RecentChanges[0].Name)
	assert.Equal(t, "DeploymentUpdated", result.RecentChanges[0].Reason)

	// Check second change (ConfigMap)
	assert.Equal(t, "ConfigMap", result.RecentChanges[1].Kind)
	assert.Equal(t, "nginx-config", result.RecentChanges[1].Name)
}

// TestEvidenceService_GetCandidateCauses_Empty tests returning empty when no deps or changes.
func TestEvidenceService_GetCandidateCauses_Empty(t *testing.T) {
	logger := logging.GetLogger("test.evidence")
	mockGraph := newMockEvidenceGraphClient()

	// Mock returns empty for both queries
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		if query.Parameters["workload"] != nil {
			// Upstream dependencies query - return empty
			return &graph.QueryResult{
				Columns: []string{"hops1", "hops2"},
				Rows: [][]interface{}{
					{[]interface{}{}, []interface{}{}},
				},
			}, nil
		}
		// Recent changes query - return empty
		return &graph.QueryResult{
			Columns: []string{"kind", "namespace", "name", "reason", "timestamp"},
			Rows:    [][]interface{}{},
		}, nil
	}

	service := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetCandidateCauses(ctx, "default", "nginx", "container_cpu_usage")

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should return empty arrays, not nil
	assert.Empty(t, result.UpstreamDeps)
	assert.Empty(t, result.RecentChanges)
	assert.NotEmpty(t, result.Timestamp)
}

// TestEvidenceService_GetSignalEvidence_Success tests successful evidence aggregation.
func TestEvidenceService_GetSignalEvidence_Success(t *testing.T) {
	logger := logging.GetLogger("test.evidence")
	mockGraph := newMockEvidenceGraphClient()

	// Mock returns metric values and alert states
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Identify query type by parameters
		if query.Parameters["metric_name"] != nil {
			// Metric values query (SignalBaseline)
			return &graph.QueryResult{
				Columns: []string{"mean", "std_dev", "min", "max", "p50", "p90", "p99", "window_start", "window_end"},
				Rows: [][]interface{}{
					{100.5, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, int64(1706572800), int64(1706659200)},
				},
			}, nil
		}
		if query.Parameters["start"] != nil && query.Parameters["end"] != nil {
			// Alert states query
			return &graph.QueryResult{
				Columns: []string{"title", "state", "since"},
				Rows: [][]interface{}{
					{"High CPU Alert", "firing", "2026-01-30T00:10:00Z"},
				},
			}, nil
		}
		if query.Parameters["since"] != nil {
			// Log excerpts query - return empty (graceful degradation)
			return &graph.QueryResult{
				Columns: []string{"timestamp", "level", "message", "source"},
				Rows:    [][]interface{}{},
			}, nil
		}
		return &graph.QueryResult{}, nil
	}

	service := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetSignalEvidence(ctx, "default", "nginx", "container_cpu_usage", 1*time.Hour)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify metric values from baseline
	assert.Len(t, result.MetricValues, 1)
	assert.Equal(t, 100.5, result.MetricValues[0].Value)

	// Verify alert states
	assert.Len(t, result.AlertStates, 1)
	assert.Equal(t, "High CPU Alert", result.AlertStates[0].AlertName)
	assert.Equal(t, "firing", result.AlertStates[0].State)
	assert.Equal(t, "2026-01-30T00:10:00Z", result.AlertStates[0].Since)

	// Timestamp should be set
	assert.NotEmpty(t, result.Timestamp)
}

// TestEvidenceService_GetSignalEvidence_NoLogs tests graceful handling when logs unavailable.
func TestEvidenceService_GetSignalEvidence_NoLogs(t *testing.T) {
	logger := logging.GetLogger("test.evidence")
	mockGraph := newMockEvidenceGraphClient()

	// Mock returns metric and alert data, but log query returns error (simulating no log integration)
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		if query.Parameters["metric_name"] != nil {
			// Metric values query
			return &graph.QueryResult{
				Columns: []string{"mean", "std_dev", "min", "max", "p50", "p90", "p99", "window_start", "window_end"},
				Rows: [][]interface{}{
					{50.0, 5.0, 40.0, 60.0, 50.0, 55.0, 58.0, int64(1706572800), int64(1706659200)},
				},
			}, nil
		}
		if query.Parameters["start"] != nil && query.Parameters["end"] != nil {
			// Alert states query - empty
			return &graph.QueryResult{
				Columns: []string{"title", "state", "since"},
				Rows:    [][]interface{}{},
			}, nil
		}
		if query.Parameters["since"] != nil {
			// Log excerpts query - return empty (log integration not configured)
			return &graph.QueryResult{
				Columns: []string{"timestamp", "level", "message", "source"},
				Rows:    [][]interface{}{},
			}, nil
		}
		return &graph.QueryResult{}, nil
	}

	service := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetSignalEvidence(ctx, "default", "nginx", "container_memory_usage", 1*time.Hour)

	// Should succeed despite no logs
	require.NoError(t, err)
	require.NotNil(t, result)

	// Metric values should still work
	assert.Len(t, result.MetricValues, 1)
	assert.Equal(t, 50.0, result.MetricValues[0].Value)

	// Log excerpts should be empty (graceful degradation)
	assert.Empty(t, result.LogExcerpts)
}

// TestEvidenceService_GetSignalEvidence_AlertStates tests including firing/pending alerts.
func TestEvidenceService_GetSignalEvidence_AlertStates(t *testing.T) {
	logger := logging.GetLogger("test.evidence")
	mockGraph := newMockEvidenceGraphClient()

	// Mock returns multiple alert states
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		if query.Parameters["metric_name"] != nil {
			// Metric values query - empty (no baseline)
			return &graph.QueryResult{
				Columns: []string{"mean", "std_dev", "min", "max", "p50", "p90", "p99", "window_start", "window_end"},
				Rows:    [][]interface{}{},
			}, nil
		}
		if query.Parameters["start"] != nil && query.Parameters["end"] != nil {
			// Alert states query - multiple alerts with different states
			return &graph.QueryResult{
				Columns: []string{"title", "state", "since"},
				Rows: [][]interface{}{
					{"Critical Memory Alert", "firing", "2026-01-30T00:05:00Z"},
					{"High CPU Alert", "pending", "2026-01-30T00:08:00Z"},
					{"Network Latency Alert", "normal", "2026-01-29T23:00:00Z"},
				},
			}, nil
		}
		if query.Parameters["since"] != nil {
			// Log excerpts query
			return &graph.QueryResult{
				Columns: []string{"timestamp", "level", "message", "source"},
				Rows:    [][]interface{}{},
			}, nil
		}
		return &graph.QueryResult{}, nil
	}

	service := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetSignalEvidence(ctx, "default", "nginx", "container_memory_usage", 2*time.Hour)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify all alert states are returned
	assert.Len(t, result.AlertStates, 3)

	// Check firing alert
	foundFiring := false
	for _, alert := range result.AlertStates {
		if alert.State == "firing" {
			assert.Equal(t, "Critical Memory Alert", alert.AlertName)
			foundFiring = true
		}
	}
	assert.True(t, foundFiring, "should include firing alert")

	// Check pending alert
	foundPending := false
	for _, alert := range result.AlertStates {
		if alert.State == "pending" {
			assert.Equal(t, "High CPU Alert", alert.AlertName)
			foundPending = true
		}
	}
	assert.True(t, foundPending, "should include pending alert")

	// Check normal alert
	foundNormal := false
	for _, alert := range result.AlertStates {
		if alert.State == "normal" {
			assert.Equal(t, "Network Latency Alert", alert.AlertName)
			foundNormal = true
		}
	}
	assert.True(t, foundNormal, "should include normal alert")
}

// TestNewObservatoryEvidenceService tests service constructor.
func TestNewObservatoryEvidenceService(t *testing.T) {
	logger := logging.GetLogger("test.evidence")
	mockGraph := newMockEvidenceGraphClient()

	service := NewObservatoryEvidenceService(mockGraph, nil, "test-integration", logger)

	assert.NotNil(t, service)
	assert.Equal(t, "test-integration", service.integrationName)
	assert.NotNil(t, service.graphClient)
	assert.NotNil(t, service.logger)
}

// TestEvidenceService_GetCandidateCauses_GracefulDegradation tests error handling.
func TestEvidenceService_GetCandidateCauses_GracefulDegradation(t *testing.T) {
	logger := logging.GetLogger("test.evidence")
	mockGraph := newMockEvidenceGraphClient()

	// Mock returns error for upstream deps but success for recent changes
	callCount := 0
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		callCount++
		if query.Parameters["workload"] != nil {
			// Upstream dependencies query - simulate error
			return nil, assert.AnError
		}
		// Recent changes query - return data
		return &graph.QueryResult{
			Columns: []string{"kind", "namespace", "name", "reason", "timestamp"},
			Rows: [][]interface{}{
				{"Deployment", "default", "nginx", "Updated", "2026-01-30T00:00:00Z"},
			},
		}, nil
	}

	service := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetCandidateCauses(ctx, "default", "nginx", "cpu_metric")

	// Should succeed despite upstream deps error (graceful degradation)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Upstream deps should be empty due to error
	assert.Empty(t, result.UpstreamDeps)

	// Recent changes should still be populated
	assert.Len(t, result.RecentChanges, 1)
	assert.Equal(t, "Deployment", result.RecentChanges[0].Kind)
}
