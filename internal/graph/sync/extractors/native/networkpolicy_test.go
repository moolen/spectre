package native

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkPolicyExtractor_Matches(t *testing.T) {
	extractor := NewNetworkPolicyExtractor()

	tests := []struct {
		name     string
		event    models.Event
		expected bool
	}{
		{
			name: "matches NetworkPolicy",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "NetworkPolicy",
					Group: "networking.k8s.io",
				},
			},
			expected: true,
		},
		{
			name: "does not match Service",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Service",
					Group: "",
				},
			},
			expected: false,
		},
		{
			name: "does not match NetworkPolicy with wrong group",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "NetworkPolicy",
					Group: "custom.io",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.Matches(tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNetworkPolicyExtractor_ExtractRelationships(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		networkPolicyData map[string]interface{}
		mockQueryResult *graph.QueryResult
		expectedEdges   int
		expectError     bool
	}{
		{
			name: "network policy with specific pod selector",
			networkPolicyData: map[string]interface{}{
				"spec": map[string]interface{}{
					"podSelector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "backend",
						},
					},
				},
			},
			mockQueryResult: &graph.QueryResult{
				Rows: [][]interface{}{
					{"pod-1"},
					{"pod-2"},
				},
			},
			expectedEdges: 2,
			expectError:   false,
		},
		{
			name: "network policy with empty pod selector (selects all)",
			networkPolicyData: map[string]interface{}{
				"spec": map[string]interface{}{
					"podSelector": map[string]interface{}{},
				},
			},
			mockQueryResult: &graph.QueryResult{
				Rows: [][]interface{}{
					{"pod-1"},
					{"pod-2"},
					{"pod-3"},
				},
			},
			expectedEdges: 3,
			expectError:   false,
		},
		{
			name: "network policy with multiple selector labels",
			networkPolicyData: map[string]interface{}{
				"spec": map[string]interface{}{
					"podSelector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app":  "backend",
							"tier": "db",
						},
					},
				},
			},
			mockQueryResult: &graph.QueryResult{
				Rows: [][]interface{}{
					{"pod-1"},
				},
			},
			expectedEdges: 1,
			expectError:   false,
		},
		{
			name: "network policy with no matching pods",
			networkPolicyData: map[string]interface{}{
				"spec": map[string]interface{}{
					"podSelector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "nonexistent",
						},
					},
				},
			},
			mockQueryResult: &graph.QueryResult{
				Rows: [][]interface{}{},
			},
			expectedEdges: 0,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewNetworkPolicyExtractor()

			netpolJSON, err := json.Marshal(tt.networkPolicyData)
			require.NoError(t, err)

			event := models.Event{
				Resource: models.ResourceMetadata{
					UID:       "netpol-uid",
					Kind:      "NetworkPolicy",
					Namespace: "default",
					Name:      "test-policy",
				},
				Data: netpolJSON,
				Type: models.EventTypeUpdate,
			}

			lookup := &MockResourceLookup{
				queryResult: tt.mockQueryResult,
			}

			edges, err := extractor.ExtractRelationships(ctx, event, lookup)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, edges, tt.expectedEdges)

				// Verify edge types and direction
				for _, edge := range edges {
					assert.Equal(t, graph.EdgeTypeSelects, edge.Type)
					assert.Equal(t, "netpol-uid", edge.FromUID)

					// Verify properties
					var props graph.SelectsEdge
					err := json.Unmarshal(edge.Properties, &props)
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestNetworkPolicyExtractor_DeletedResource(t *testing.T) {
	ctx := context.Background()
	extractor := NewNetworkPolicyExtractor()

	networkPolicyData := map[string]interface{}{
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"app": "backend",
				},
			},
		},
	}

	netpolJSON, err := json.Marshal(networkPolicyData)
	require.NoError(t, err)

	event := models.Event{
		Resource: models.ResourceMetadata{
			UID:       "netpol-uid",
			Kind:      "NetworkPolicy",
			Namespace: "default",
			Name:      "test-policy",
		},
		Data: netpolJSON,
		Type: models.EventTypeDelete, // Deleted event
	}

	lookup := &MockResourceLookup{
		queryResult: &graph.QueryResult{
			Rows: [][]interface{}{
				{"pod-1"},
			},
		},
	}

	edges, err := extractor.ExtractRelationships(ctx, event, lookup)

	assert.NoError(t, err)
	assert.Len(t, edges, 0, "Should not extract relationships for deleted resources")
}
