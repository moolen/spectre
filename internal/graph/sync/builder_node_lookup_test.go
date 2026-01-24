package sync

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/stretchr/testify/assert"
)

// mockGraphClient implements graph.Client for testing Node lookups
type mockGraphClient struct {
	nodes map[string]string // name -> UID mapping
}

func (m *mockGraphClient) Connect(ctx context.Context) error { return nil }
func (m *mockGraphClient) Close() error                      { return nil }
func (m *mockGraphClient) Ping(ctx context.Context) error    { return nil }
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
	return true, nil
}

func (m *mockGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	// Check if this is a Node lookup query
	if kind, ok := query.Parameters["kind"].(string); ok && kind == "Node" {
		if name, ok := query.Parameters["name"].(string); ok {
			if uid, exists := m.nodes[name]; exists {
				return &graph.QueryResult{
					Columns: []string{"uid"},
					Rows:    [][]interface{}{{uid}},
				}, nil
			}
			// Node not found
			return &graph.QueryResult{
				Columns: []string{"uid"},
				Rows:    [][]interface{}{},
			}, nil
		}
	}
	return &graph.QueryResult{}, nil
}

func TestGraphBuilder_SchedulingRelationship(t *testing.T) {
	tests := []struct {
		name           string
		mockNodes      map[string]string
		podSpec        map[string]interface{}
		expectEdge     bool
		expectedNodeUID string
	}{
		{
			name: "Pod scheduled on existing Node",
			mockNodes: map[string]string{
				"node-1": "node-uid-123",
			},
			podSpec: map[string]interface{}{
				"spec": map[string]interface{}{
					"nodeName": "node-1",
				},
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":               "PodScheduled",
							"lastTransitionTime": time.Now().Format(time.RFC3339),
						},
					},
				},
			},
			expectEdge:      true,
			expectedNodeUID: "node-uid-123",
		},
		{
			name:      "Pod scheduled on non-existent Node",
			mockNodes: map[string]string{},
			podSpec: map[string]interface{}{
				"spec": map[string]interface{}{
					"nodeName": "node-2",
				},
			},
			expectEdge: false,
		},
		{
			name: "Pod without nodeName",
			mockNodes: map[string]string{
				"node-1": "node-uid-123",
			},
			podSpec: map[string]interface{}{
				"spec": map[string]interface{}{},
			},
			expectEdge: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client with test nodes
			mockClient := &mockGraphClient{
				nodes: tt.mockNodes,
			}

			// Create builder with client
			builder := NewGraphBuilderWithClient(mockClient)
			gb := builder.(*graphBuilder)

			// Marshal pod spec to JSON
			podData, _ := json.Marshal(tt.podSpec)
			var resourceData map[string]interface{}
			json.Unmarshal(podData, &resourceData)

			// Extract scheduling relationship
			edge := gb.extractSchedulingRelationship("pod-uid-456", resourceData)

			if tt.expectEdge {
				assert.NotNil(t, edge, "Expected SCHEDULED_ON edge to be created")
				assert.Equal(t, graph.EdgeTypeScheduledOn, edge.Type)
				assert.Equal(t, "pod-uid-456", edge.FromUID)
				assert.Equal(t, tt.expectedNodeUID, edge.ToUID)

				// Verify edge properties
				var props graph.ScheduledOnEdge
				err := json.Unmarshal(edge.Properties, &props)
				assert.NoError(t, err)
				assert.NotZero(t, props.ScheduledAt, "ScheduledAt should be set")
			} else {
				assert.Nil(t, edge, "Expected no SCHEDULED_ON edge")
			}
		})
	}
}

func TestGraphBuilder_WithoutClient(t *testing.T) {
	// Create builder without client (legacy mode)
	builder := NewGraphBuilder()
	gb := builder.(*graphBuilder)

	podSpec := map[string]interface{}{
		"spec": map[string]interface{}{
			"nodeName": "node-1",
		},
	}

	// Should return nil when no client is available
	edge := gb.extractSchedulingRelationship("pod-uid-789", podSpec)
	assert.Nil(t, edge, "Should not create edge without client")
}
