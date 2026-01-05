package extractors

import (
	"testing"

	"github.com/moolen/spectre/internal/graph"
	"github.com/stretchr/testify/assert"
)

func TestAbsInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected int64
	}{
		{
			name:     "zero",
			input:    0,
			expected: 0,
		},
		{
			name:     "positive number",
			input:    42,
			expected: 42,
		},
		{
			name:     "negative number",
			input:    -42,
			expected: 42,
		},
		{
			name:     "large positive",
			input:    9223372036854775807, // max int64
			expected: 9223372036854775807,
		},
		{
			name:     "large negative",
			input:    -9223372036854775807,
			expected: 9223372036854775807,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AbsInt64(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidEdgeOrNil(t *testing.T) {
	tests := []struct {
		name     string
		edge     graph.Edge
		expected *graph.Edge
	}{
		{
			name: "valid edge with non-empty ToUID",
			edge: graph.Edge{
				Type:    graph.EdgeTypeReferencesSpec,
				FromUID: "from-123",
				ToUID:   "to-456",
			},
			expected: &graph.Edge{
				Type:    graph.EdgeTypeReferencesSpec,
				FromUID: "from-123",
				ToUID:   "to-456",
			},
		},
		{
			name: "invalid edge with empty ToUID",
			edge: graph.Edge{
				Type:    graph.EdgeTypeReferencesSpec,
				FromUID: "from-123",
				ToUID:   "",
			},
			expected: nil,
		},
		{
			name: "edge with FromUID but no ToUID",
			edge: graph.Edge{
				Type:    graph.EdgeTypeManages,
				FromUID: "manager-789",
				ToUID:   "",
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidEdgeOrNil(tt.edge)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.Type, result.Type)
				assert.Equal(t, tt.expected.FromUID, result.FromUID)
				assert.Equal(t, tt.expected.ToUID, result.ToUID)
			}
		})
	}
}

func TestHasReadyCondition(t *testing.T) {
	tests := []struct {
		name     string
		resource map[string]interface{}
		expected bool
	}{
		{
			name: "has Ready=True condition",
			resource: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":   "Ready",
							"status": "True",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "has Ready=False condition",
			resource: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":   "Ready",
							"status": "False",
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "has multiple conditions with Ready=True",
			resource: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":   "Progressing",
							"status": "True",
						},
						map[string]interface{}{
							"type":   "Ready",
							"status": "True",
						},
						map[string]interface{}{
							"type":   "Available",
							"status": "True",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "has other conditions but no Ready",
			resource: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":   "Progressing",
							"status": "True",
						},
					},
				},
			},
			expected: false,
		},
		{
			name:     "no status field",
			resource: map[string]interface{}{},
			expected: false,
		},
		{
			name: "status field but no conditions",
			resource: map[string]interface{}{
				"status": map[string]interface{}{},
			},
			expected: false,
		},
		{
			name: "empty conditions array",
			resource: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{},
				},
			},
			expected: false,
		},
		{
			name: "malformed conditions (not array)",
			resource: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": "not an array",
				},
			},
			expected: false,
		},
		{
			name: "malformed condition entry (not map)",
			resource: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						"not a map",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasReadyCondition(tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}
