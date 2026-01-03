package validation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockGraphClient implements graph.Client for testing
type MockGraphClient struct {
	queryResults map[string]*graph.QueryResult
	queries      []graph.GraphQuery
}

func NewMockGraphClient() *MockGraphClient {
	return &MockGraphClient{
		queryResults: make(map[string]*graph.QueryResult),
		queries:      make([]graph.GraphQuery, 0),
	}
}

func (m *MockGraphClient) Connect(ctx context.Context) error {
	return nil
}

func (m *MockGraphClient) Ping(ctx context.Context) error {
	return nil
}

func (m *MockGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)
	
	// Return mock result if available
	if result, ok := m.queryResults[query.Query]; ok {
		return result, nil
	}
	
	return &graph.QueryResult{Rows: [][]interface{}{}}, nil
}

func (m *MockGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}

func (m *MockGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}

func (m *MockGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}

func (m *MockGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}

func (m *MockGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}

func (m *MockGraphClient) InitializeSchema(ctx context.Context) error {
	return nil
}

func (m *MockGraphClient) Close() error {
	return nil
}

func (m *MockGraphClient) DeleteGraph(ctx context.Context) error {
	return nil
}

func TestEdgeRevalidator_DefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	assert.Equal(t, 5*time.Minute, config.Interval)
	assert.Equal(t, 1*time.Hour, config.MaxAge)
	assert.Equal(t, 7*24*time.Hour, config.StaleThreshold)
	assert.True(t, config.DecayEnabled)
	assert.Equal(t, 0.9, config.DecayFactor6h)
	assert.Equal(t, 0.7, config.DecayFactor24h)
}

func TestEdgeRevalidator_ApplyConfidenceDecay(t *testing.T) {
	client := NewMockGraphClient()
	config := DefaultConfig()
	revalidator := NewEdgeRevalidator(client, config)
	
	tests := []struct {
		name               string
		originalConfidence float64
		ageHours           int
		expectedDecayed    bool
		expectedConfidence float64
	}{
		{
			name:               "no decay for 100% confidence",
			originalConfidence: 1.0,
			ageHours:           48,
			expectedDecayed:    false,
			expectedConfidence: 1.0,
		},
		{
			name:               "no decay within 6 hours",
			originalConfidence: 0.8,
			ageHours:           3,
			expectedDecayed:    false,
			expectedConfidence: 0.8,
		},
		{
			name:               "6-hour decay applied",
			originalConfidence: 0.8,
			ageHours:           12,
			expectedDecayed:    true,
			expectedConfidence: 0.72, // 0.8 * 0.9
		},
		{
			name:               "24-hour decay applied",
			originalConfidence: 0.8,
			ageHours:           48,
			expectedDecayed:    true,
			expectedConfidence: 0.56, // 0.8 * 0.7
		},
		{
			name:               "minimum confidence threshold",
			originalConfidence: 0.15,
			ageHours:           48,
			expectedDecayed:    true,
			expectedConfidence: 0.1, // Floor at 0.1
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edge := &graph.ManagesEdge{
				Confidence: tt.originalConfidence,
			}
			
			ageNs := time.Duration(tt.ageHours) * time.Hour
			decayed, newConfidence := revalidator.applyConfidenceDecay(edge, ageNs.Nanoseconds())
			
			assert.Equal(t, tt.expectedDecayed, decayed)
			if tt.expectedDecayed {
				assert.InDelta(t, tt.expectedConfidence, newConfidence, 0.01)
			}
		})
	}
}

func TestEdgeRevalidator_ValidateEdge(t *testing.T) {
	client := NewMockGraphClient()
	config := DefaultConfig()
	revalidator := NewEdgeRevalidator(client, config)
	
	tests := []struct {
		name           string
		source         map[string]interface{}
		target         map[string]interface{}
		expectedValid  bool
	}{
		{
			name: "both resources exist",
			source: map[string]interface{}{
				"uid":     "source-uid",
				"deleted": false,
			},
			target: map[string]interface{}{
				"uid":     "target-uid",
				"deleted": false,
			},
			expectedValid: true,
		},
		{
			name: "source deleted",
			source: map[string]interface{}{
				"uid":     "source-uid",
				"deleted": true,
			},
			target: map[string]interface{}{
				"uid":     "target-uid",
				"deleted": false,
			},
			expectedValid: false,
		},
		{
			name: "target deleted",
			source: map[string]interface{}{
				"uid":     "source-uid",
				"deleted": false,
			},
			target: map[string]interface{}{
				"uid":     "target-uid",
				"deleted": true,
			},
			expectedValid: false,
		},
		{
			name: "both deleted",
			source: map[string]interface{}{
				"uid":     "source-uid",
				"deleted": true,
			},
			target: map[string]interface{}{
				"uid":     "target-uid",
				"deleted": true,
			},
			expectedValid: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edge := &graph.ManagesEdge{
				Confidence: 0.8,
			}
			
			valid := revalidator.validateEdge(context.Background(), tt.source, tt.target, edge)
			assert.Equal(t, tt.expectedValid, valid)
		})
	}
}

func TestEdgeRevalidator_ParseEdgeProperties(t *testing.T) {
	client := NewMockGraphClient()
	config := DefaultConfig()
	revalidator := NewEdgeRevalidator(client, config)
	
	now := time.Now().UnixNano()
	edge := graph.ManagesEdge{
		Confidence:      0.85,
		FirstObserved:   now,
		LastValidated:   now,
		ValidationState: graph.ValidationStateValid,
		Evidence: []graph.EvidenceItem{
			{
				Type:   graph.EvidenceTypeLabel,
				Value:  "test-label",
				Weight: 0.5,
			},
		},
	}
	
	edgeJSON, err := json.Marshal(edge)
	require.NoError(t, err)
	
	edgeData := map[string]interface{}{
		"type":       "MANAGES",
		"fromUID":    "source-uid",
		"toUID":      "target-uid",
		"properties": string(edgeJSON),
	}
	
	parsed, err := revalidator.parseEdgeProperties(edgeData)
	require.NoError(t, err)
	assert.Equal(t, 0.85, parsed.Confidence)
	assert.Equal(t, graph.ValidationStateValid, parsed.ValidationState)
	assert.Len(t, parsed.Evidence, 1)
}

func TestEdgeRevalidator_GetStats(t *testing.T) {
	client := NewMockGraphClient()
	config := DefaultConfig()
	revalidator := NewEdgeRevalidator(client, config)
	
	stats := revalidator.GetStats()
	
	assert.Equal(t, "5m0s", stats["interval"])
	assert.Equal(t, "1h0m0s", stats["maxAge"])
	assert.Equal(t, "168h0m0s", stats["staleThreshold"])
	assert.Equal(t, true, stats["decayEnabled"])
	assert.Equal(t, 0.9, stats["decayFactor6h"])
	assert.Equal(t, 0.7, stats["decayFactor24h"])
}

func TestEdgeRevalidator_RevalidationCycle(t *testing.T) {
	client := NewMockGraphClient()
	config := Config{
		Interval:         1 * time.Second,
		MaxAge:           10 * time.Millisecond,
		StaleThreshold:   1 * time.Hour,
		DecayEnabled:     true,
		DecayInterval6h:  6 * time.Hour,
		DecayInterval24h: 24 * time.Hour,
		DecayFactor6h:    0.9,
		DecayFactor24h:   0.7,
	}
	
	now := time.Now().UnixNano()
	edge := graph.ManagesEdge{
		Confidence:      0.8,
		FirstObserved:   now - (12 * time.Hour).Nanoseconds(),
		LastValidated:   now - (12 * time.Hour).Nanoseconds(),
		ValidationState: graph.ValidationStateValid,
	}
	edgeJSON, _ := json.Marshal(edge)
	
	// Mock query result
	client.queryResults[`
			MATCH (source:ResourceIdentity)-[edge]->(target:ResourceIdentity)
			WHERE (type(edge) = 'MANAGES' OR type(edge) = 'CREATES_OBSERVED')
			  AND source.deleted = false
			  AND target.deleted = false
			RETURN source, edge, target
			LIMIT 1000
		`] = &graph.QueryResult{
		Rows: [][]interface{}{
			{
				map[string]interface{}{
					"uid":     "source-uid",
					"deleted": false,
				},
				map[string]interface{}{
					"type":       "MANAGES",
					"fromUID":    "source-uid",
					"toUID":      "target-uid",
					"properties": string(edgeJSON),
				},
				map[string]interface{}{
					"uid":     "target-uid",
					"deleted": false,
				},
			},
		},
	}
	
	revalidator := NewEdgeRevalidator(client, config)
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// Run revalidation cycle
	err := revalidator.revalidateEdges(ctx)
	assert.NoError(t, err)
	
	// Verify queries were executed
	assert.GreaterOrEqual(t, len(client.queries), 1)
}
