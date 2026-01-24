package grafana

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAnalysisGraphClient implements graph.Client for alert analysis testing
type mockAnalysisGraphClient struct {
	queryResponses map[string]*graph.QueryResult
	lastQuery      string
}

func (m *mockAnalysisGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.lastQuery = query.Query

	// Detect query type by pattern matching
	if strings.Contains(query.Query, "STATE_TRANSITION") {
		// Return appropriate mock data based on test scenario
		if result, ok := m.queryResponses["STATE_TRANSITION"]; ok {
			return result, nil
		}
		// Default: return empty result (no transitions)
		return &graph.QueryResult{
			Columns: []string{"from_state", "to_state", "timestamp"},
			Rows:    [][]interface{}{},
		}, nil
	}

	return &graph.QueryResult{}, nil
}

func (m *mockAnalysisGraphClient) Connect(ctx context.Context) error              { return nil }
func (m *mockAnalysisGraphClient) Close() error                                   { return nil }
func (m *mockAnalysisGraphClient) Ping(ctx context.Context) error                 { return nil }
func (m *mockAnalysisGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockAnalysisGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockAnalysisGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockAnalysisGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockAnalysisGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockAnalysisGraphClient) InitializeSchema(ctx context.Context) error       { return nil }
func (m *mockAnalysisGraphClient) DeleteGraph(ctx context.Context) error            { return nil }
func (m *mockAnalysisGraphClient) CreateGraph(ctx context.Context, graphName string) error { return nil }
func (m *mockAnalysisGraphClient) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockAnalysisGraphClient) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

func TestAlertAnalysisService_AnalyzeAlert_Success(t *testing.T) {
	now := time.Now()

	// Mock 7-day stable firing history
	mockClient := &mockAnalysisGraphClient{
		queryResponses: map[string]*graph.QueryResult{
			"STATE_TRANSITION": {
				Columns: []string{"from_state", "to_state", "timestamp"},
				Rows: [][]interface{}{
					{"normal", "firing", now.Add(-7 * 24 * time.Hour).Format(time.RFC3339)},
				},
			},
		},
	}

	logger := logging.GetLogger("test")
	service := NewAlertAnalysisService(mockClient, "test-grafana", logger)

	result, err := service.AnalyzeAlert(context.Background(), "alert-123")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.FlappinessScore, 0.0)
	assert.LessOrEqual(t, result.FlappinessScore, 1.0)
	assert.NotEmpty(t, result.Categories.Onset)
	assert.NotEmpty(t, result.Categories.Pattern)
	assert.GreaterOrEqual(t, result.DataAvailable, 7*24*time.Hour)
}

func TestAlertAnalysisService_AnalyzeAlert_PartialData(t *testing.T) {
	now := time.Now()

	// Mock 2-day history (between 24h and 7d - should succeed)
	mockClient := &mockAnalysisGraphClient{
		queryResponses: map[string]*graph.QueryResult{
			"STATE_TRANSITION": {
				Columns: []string{"from_state", "to_state", "timestamp"},
				Rows: [][]interface{}{
					{"normal", "firing", now.Add(-2 * 24 * time.Hour).Format(time.RFC3339)},
				},
			},
		},
	}

	logger := logging.GetLogger("test")
	service := NewAlertAnalysisService(mockClient, "test-grafana", logger)

	result, err := service.AnalyzeAlert(context.Background(), "alert-456")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.DataAvailable, 24*time.Hour)
	assert.LessOrEqual(t, result.DataAvailable, 7*24*time.Hour)
}

func TestAlertAnalysisService_AnalyzeAlert_InsufficientData(t *testing.T) {
	now := time.Now()

	// Mock < 24h history (should error)
	mockClient := &mockAnalysisGraphClient{
		queryResponses: map[string]*graph.QueryResult{
			"STATE_TRANSITION": {
				Columns: []string{"from_state", "to_state", "timestamp"},
				Rows: [][]interface{}{
					{"normal", "firing", now.Add(-12 * time.Hour).Format(time.RFC3339)},
				},
			},
		},
	}

	logger := logging.GetLogger("test")
	service := NewAlertAnalysisService(mockClient, "test-grafana", logger)

	result, err := service.AnalyzeAlert(context.Background(), "new-alert")

	require.Error(t, err)
	assert.Nil(t, result)

	var insufficientErr ErrInsufficientData
	assert.ErrorAs(t, err, &insufficientErr)
	assert.Less(t, insufficientErr.Available, 24*time.Hour)
	assert.Equal(t, 24*time.Hour, insufficientErr.Required)
}

func TestAlertAnalysisService_AnalyzeAlert_EmptyTransitions(t *testing.T) {
	// Mock empty transitions (new alert with no history)
	mockClient := &mockAnalysisGraphClient{
		queryResponses: map[string]*graph.QueryResult{
			"STATE_TRANSITION": {
				Columns: []string{"from_state", "to_state", "timestamp"},
				Rows:    [][]interface{}{}, // Empty
			},
		},
	}

	logger := logging.GetLogger("test")
	service := NewAlertAnalysisService(mockClient, "test-grafana", logger)

	result, err := service.AnalyzeAlert(context.Background(), "brand-new-alert")

	require.Error(t, err)
	assert.Nil(t, result)

	var insufficientErr ErrInsufficientData
	assert.ErrorAs(t, err, &insufficientErr)
	assert.Equal(t, time.Duration(0), insufficientErr.Available)
}

func TestAlertAnalysisService_AnalyzeAlert_CacheHit(t *testing.T) {
	now := time.Now()

	mockClient := &mockAnalysisGraphClient{
		queryResponses: map[string]*graph.QueryResult{
			"STATE_TRANSITION": {
				Columns: []string{"from_state", "to_state", "timestamp"},
				Rows: [][]interface{}{
					{"normal", "firing", now.Add(-3 * 24 * time.Hour).Format(time.RFC3339)},
				},
			},
		},
	}

	logger := logging.GetLogger("test")
	service := NewAlertAnalysisService(mockClient, "test-grafana", logger)

	// First call - should query graph
	result1, err1 := service.AnalyzeAlert(context.Background(), "alert-cached")
	require.NoError(t, err1)
	firstComputedAt := result1.ComputedAt

	// Second call - should use cache
	result2, err2 := service.AnalyzeAlert(context.Background(), "alert-cached")
	require.NoError(t, err2)

	// Verify same cached result (ComputedAt should match)
	assert.Equal(t, firstComputedAt, result2.ComputedAt)
	assert.Equal(t, result1.FlappinessScore, result2.FlappinessScore)
	assert.Equal(t, result1.DeviationScore, result2.DeviationScore)
}

func TestAlertAnalysisService_AnalyzeAlert_Flapping(t *testing.T) {
	now := time.Now()

	// Mock flapping alert (many transitions)
	rows := [][]interface{}{
		{"normal", "firing", now.Add(-3 * 24 * time.Hour).Format(time.RFC3339)},
	}
	// Add 10 transitions in last 6 hours to trigger high flappiness
	for i := 0; i < 10; i++ {
		timestamp := now.Add(-time.Duration(5-i/2) * time.Hour)
		if i%2 == 0 {
			rows = append(rows, []interface{}{"firing", "normal", timestamp.Format(time.RFC3339)})
		} else {
			rows = append(rows, []interface{}{"normal", "firing", timestamp.Format(time.RFC3339)})
		}
	}

	mockClient := &mockAnalysisGraphClient{
		queryResponses: map[string]*graph.QueryResult{
			"STATE_TRANSITION": {
				Columns: []string{"from_state", "to_state", "timestamp"},
				Rows:    rows,
			},
		},
	}

	logger := logging.GetLogger("test")
	service := NewAlertAnalysisService(mockClient, "test-grafana", logger)

	result, err := service.AnalyzeAlert(context.Background(), "flapping-alert")

	require.NoError(t, err)
	assert.NotNil(t, result)

	// Should have high flappiness score
	assert.Greater(t, result.FlappinessScore, 0.5)

	// Should be categorized as flapping
	assert.Contains(t, result.Categories.Pattern, "flapping")
}

func TestAlertAnalysisService_AnalyzeAlert_Chronic(t *testing.T) {
	now := time.Now()

	// Mock chronic alert (old + mostly firing)
	mockClient := &mockAnalysisGraphClient{
		queryResponses: map[string]*graph.QueryResult{
			"STATE_TRANSITION": {
				Columns: []string{"from_state", "to_state", "timestamp"},
				Rows: [][]interface{}{
					{"normal", "firing", now.Add(-8 * 24 * time.Hour).Format(time.RFC3339)},
					// Brief normal period
					{"firing", "normal", now.Add(-7*24*time.Hour - 1*time.Hour).Format(time.RFC3339)},
					{"normal", "firing", now.Add(-7 * 24 * time.Hour).Format(time.RFC3339)},
					// Firing for rest of 7 days (>80%)
				},
			},
		},
	}

	logger := logging.GetLogger("test")
	service := NewAlertAnalysisService(mockClient, "test-grafana", logger)

	result, err := service.AnalyzeAlert(context.Background(), "chronic-alert")

	require.NoError(t, err)
	assert.NotNil(t, result)

	// Should be categorized as chronic
	assert.Contains(t, result.Categories.Onset, "chronic")
}

func TestFetchStateTransitions_QueryFormat(t *testing.T) {
	now := time.Now()

	mockClient := &mockAnalysisGraphClient{
		queryResponses: map[string]*graph.QueryResult{
			"STATE_TRANSITION": {
				Columns: []string{"from_state", "to_state", "timestamp"},
				Rows: [][]interface{}{
					{"normal", "firing", now.Add(-1 * time.Hour).Format(time.RFC3339)},
				},
			},
		},
	}

	startTime := now.Add(-2 * time.Hour)
	endTime := now

	transitions, err := FetchStateTransitions(
		context.Background(),
		mockClient,
		"test-alert",
		"test-integration",
		startTime,
		endTime,
	)

	require.NoError(t, err)
	assert.Len(t, transitions, 1)

	// Verify query contains expected clauses
	assert.Contains(t, mockClient.lastQuery, "STATE_TRANSITION")
	assert.Contains(t, mockClient.lastQuery, "WHERE")
	assert.Contains(t, mockClient.lastQuery, "timestamp >=")
	assert.Contains(t, mockClient.lastQuery, "expires_at >")
	assert.Contains(t, mockClient.lastQuery, "ORDER BY")
}

func TestFilterTransitions(t *testing.T) {
	now := time.Now()

	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-3 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-2 * time.Hour)},
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-1 * time.Hour)},
	}

	// Filter to last 1.5 hours
	filtered := filterTransitions(transitions, now.Add(-90*time.Minute), now)

	assert.Len(t, filtered, 1)
	assert.Equal(t, "firing", filtered[0].ToState)
}

func TestComputeCurrentDistribution(t *testing.T) {
	now := time.Now()

	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-1 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-30 * time.Minute)},
	}

	dist := computeCurrentDistribution(transitions, now, 1*time.Hour)

	// 30 minutes firing, 30 minutes normal
	assert.InDelta(t, 0.5, dist.PercentFiring, 0.01)
	assert.InDelta(t, 0.5, dist.PercentNormal, 0.01)
}
