package analysis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassifyCausationType validates causation type classification
func TestClassifyCausationType(t *testing.T) {
	tests := []struct {
		name             string
		event            *ChangeEventInfo
		relationshipType string
		expectedType     string
	}{
		{
			name: "ConfigChanged=true returns ConfigChange",
			event: &ChangeEventInfo{
				ConfigChanged: true,
				EventType:     "UPDATE",
			},
			relationshipType: "OWNS",
			expectedType:     "ConfigChange",
		},
		{
			name: "ConfigChanged overrides event type",
			event: &ChangeEventInfo{
				ConfigChanged: true,
				EventType:     "CREATE",
			},
			relationshipType: "OWNS",
			expectedType:     "ConfigChange",
		},
		{
			name: "CREATE event without config change",
			event: &ChangeEventInfo{
				ConfigChanged: false,
				EventType:     "CREATE",
			},
			relationshipType: "OWNS",
			expectedType:     "ResourceCreation",
		},
		{
			name: "UPDATE with MANAGES relationship",
			event: &ChangeEventInfo{
				ConfigChanged: false,
				EventType:     "UPDATE",
			},
			relationshipType: "MANAGES",
			expectedType:     "DeploymentUpdate",
		},
		{
			name: "UPDATE with OWNS relationship",
			event: &ChangeEventInfo{
				ConfigChanged: false,
				EventType:     "UPDATE",
			},
			relationshipType: "OWNS",
			expectedType:     "ResourceUpdate",
		},
		{
			name: "UPDATE with no relationship",
			event: &ChangeEventInfo{
				ConfigChanged: false,
				EventType:     "UPDATE",
			},
			relationshipType: "",
			expectedType:     "ResourceUpdate",
		},
		{
			name: "DELETE event",
			event: &ChangeEventInfo{
				ConfigChanged: false,
				EventType:     "DELETE",
			},
			relationshipType: "OWNS",
			expectedType:     "ResourceDeletion",
		},
		{
			name: "Unknown event type",
			event: &ChangeEventInfo{
				ConfigChanged: false,
				EventType:     "OBSERVED",
			},
			relationshipType: "OWNS",
			expectedType:     "Unknown",
		},
		{
			name: "Empty event type",
			event: &ChangeEventInfo{
				ConfigChanged: false,
				EventType:     "",
			},
			relationshipType: "OWNS",
			expectedType:     "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyCausationType(tt.event, tt.relationshipType)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

// TestGenerateRootCauseExplanation validates explanation generation
func TestGenerateRootCauseExplanation(t *testing.T) {
	tests := []struct {
		name              string
		rootNode          GraphNode
		event             *ChangeEventInfo
		causationType     string
		spineNodes        []GraphNode
		expectedSubstring []string
	}{
		{
			name: "ConfigChange explanation",
			rootNode: GraphNode{
				Resource: SymptomResource{
					Kind: "HelmRelease",
					Name: "my-app",
				},
			},
			event: &ChangeEventInfo{
				ConfigChanged: true,
			},
			causationType: "ConfigChange",
			spineNodes: []GraphNode{
				{Resource: SymptomResource{Kind: "HelmRelease", Name: "my-app"}},
			},
			expectedSubstring: []string{
				"HelmRelease",
				"my-app",
				"configuration was changed",
			},
		},
		{
			name: "DeploymentUpdate explanation",
			rootNode: GraphNode{
				Resource: SymptomResource{
					Kind: "Deployment",
					Name: "web-server",
				},
			},
			event:         &ChangeEventInfo{},
			causationType: "DeploymentUpdate",
			spineNodes: []GraphNode{
				{Resource: SymptomResource{Kind: "Deployment", Name: "web-server"}},
			},
			expectedSubstring: []string{
				"Deployment",
				"web-server",
				"was updated (deployment)",
			},
		},
		{
			name: "ResourceCreation explanation",
			rootNode: GraphNode{
				Resource: SymptomResource{
					Kind: "ConfigMap",
					Name: "app-config",
				},
			},
			event:         &ChangeEventInfo{},
			causationType: "ResourceCreation",
			spineNodes: []GraphNode{
				{Resource: SymptomResource{Kind: "ConfigMap", Name: "app-config"}},
			},
			expectedSubstring: []string{
				"ConfigMap",
				"app-config",
				"was created",
			},
		},
		{
			name: "ResourceUpdate explanation",
			rootNode: GraphNode{
				Resource: SymptomResource{
					Kind: "Service",
					Name: "api-service",
				},
			},
			event:         &ChangeEventInfo{},
			causationType: "ResourceUpdate",
			spineNodes: []GraphNode{
				{Resource: SymptomResource{Kind: "Service", Name: "api-service"}},
			},
			expectedSubstring: []string{
				"Service",
				"api-service",
				"was updated",
			},
		},
		{
			name: "ResourceDeletion explanation",
			rootNode: GraphNode{
				Resource: SymptomResource{
					Kind: "Pod",
					Name: "worker-pod",
				},
			},
			event:         &ChangeEventInfo{},
			causationType: "ResourceDeletion",
			spineNodes: []GraphNode{
				{Resource: SymptomResource{Kind: "Pod", Name: "worker-pod"}},
			},
			expectedSubstring: []string{
				"Pod",
				"worker-pod",
				"was deleted",
			},
		},
		{
			name: "Unknown causation type",
			rootNode: GraphNode{
				Resource: SymptomResource{
					Kind: "Unknown",
					Name: "mystery",
				},
			},
			event:         &ChangeEventInfo{},
			causationType: "Unknown",
			spineNodes: []GraphNode{
				{Resource: SymptomResource{Kind: "Unknown", Name: "mystery"}},
			},
			expectedSubstring: []string{
				"Unknown",
				"mystery",
				"changed",
			},
		},
		{
			name: "Explanation with propagation path",
			rootNode: GraphNode{
				Resource: SymptomResource{
					Kind: "HelmRelease",
					Name: "my-app",
				},
			},
			event:         &ChangeEventInfo{},
			causationType: "ConfigChange",
			spineNodes: []GraphNode{
				{Resource: SymptomResource{Kind: "HelmRelease", Name: "my-app"}},
				{Resource: SymptomResource{Kind: "Deployment", Name: "my-app-deploy"}},
				{Resource: SymptomResource{Kind: "ReplicaSet", Name: "my-app-rs"}},
				{Resource: SymptomResource{Kind: "Pod", Name: "my-app-pod"}},
			},
			expectedSubstring: []string{
				"HelmRelease",
				"configuration was changed",
				"cascaded through",
				// Don't check exact order since the function reverses the path
			},
		},
		{
			name: "Single node chain (no propagation)",
			rootNode: GraphNode{
				Resource: SymptomResource{
					Kind: "Pod",
					Name: "standalone-pod",
				},
			},
			event:         &ChangeEventInfo{},
			causationType: "ConfigChange",
			spineNodes: []GraphNode{
				{Resource: SymptomResource{Kind: "Pod", Name: "standalone-pod"}},
			},
			expectedSubstring: []string{
				"Pod",
				"standalone-pod",
				"configuration was changed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateRootCauseExplanation(tt.rootNode, tt.event, tt.causationType, tt.spineNodes)

			// Check that explanation is not empty
			assert.NotEmpty(t, result)

			// Check for expected substrings
			for _, substr := range tt.expectedSubstring {
				assert.Contains(t, result, substr, "Expected explanation to contain: %s", substr)
			}
		})
	}
}

// TestIdentifyRootCause validates root cause identification logic
func TestIdentifyRootCause(t *testing.T) {
	analyzer := &RootCauseAnalyzer{}
	failureTime := time.Now().UnixNano()

	tests := []struct {
		name          string
		graph         CausalGraph
		expectedError bool
		validate      func(t *testing.T, result *RootCauseHypothesis)
	}{
		{
			name: "Empty graph returns error",
			graph: CausalGraph{
				Nodes: []GraphNode{},
			},
			expectedError: true,
		},
		{
			name: "No SPINE nodes returns error",
			graph: CausalGraph{
				Nodes: []GraphNode{
					{NodeType: "RELATED"},
					{NodeType: "RELATED"},
				},
			},
			expectedError: true,
		},
		{
			name: "Single SPINE node with event",
			graph: CausalGraph{
				Nodes: []GraphNode{
					{
						ID:         "node-1",
						NodeType:   "SPINE",
						StepNumber: 1,
						Resource: SymptomResource{
							UID:  "pod-123",
							Kind: "Pod",
							Name: "my-pod",
						},
						ChangeEvent: &ChangeEventInfo{
							EventID:       "event-1",
							Timestamp:     time.Unix(0, failureTime-1000000000),
							EventType:     "UPDATE",
							ConfigChanged: true,
						},
					},
				},
				Edges: []GraphEdge{},
			},
			expectedError: false,
			validate: func(t *testing.T, result *RootCauseHypothesis) {
				assert.Equal(t, "pod-123", result.Resource.UID)
				assert.Equal(t, "Pod", result.Resource.Kind)
				assert.Equal(t, "event-1", result.ChangeEvent.EventID)
				assert.Equal(t, "ConfigChange", result.CausationType)
				assert.Equal(t, int64(1000), result.TimeLagMs) // 1 second in milliseconds
			},
		},
		{
			name: "Multiple SPINE nodes - highest step number is root",
			graph: CausalGraph{
				Nodes: []GraphNode{
					{
						ID:         "node-pod",
						NodeType:   "SPINE",
						StepNumber: 1,
						Resource: SymptomResource{
							Kind: "Pod",
							Name: "my-pod",
						},
						ChangeEvent: &ChangeEventInfo{
							EventID:   "event-pod",
							Timestamp: time.Unix(0, failureTime-1000000000),
							EventType: "UPDATE",
						},
					},
					{
						ID:         "node-rs",
						NodeType:   "SPINE",
						StepNumber: 2,
						Resource: SymptomResource{
							Kind: "ReplicaSet",
							Name: "my-rs",
						},
						ChangeEvent: &ChangeEventInfo{
							EventID:   "event-rs",
							Timestamp: time.Unix(0, failureTime-2000000000),
							EventType: "UPDATE",
						},
					},
					{
						ID:         "node-deploy",
						NodeType:   "SPINE",
						StepNumber: 3,
						Resource: SymptomResource{
							Kind: "Deployment",
							Name: "my-deploy",
						},
						ChangeEvent: &ChangeEventInfo{
							EventID:       "event-deploy",
							Timestamp:     time.Unix(0, failureTime-5000000000),
							EventType:     "UPDATE",
							ConfigChanged: true,
						},
					},
				},
				Edges: []GraphEdge{
					{
						From:             "node-deploy",
						To:               "node-rs",
						RelationshipType: "OWNS",
					},
				},
			},
			expectedError: false,
			validate: func(t *testing.T, result *RootCauseHypothesis) {
				// Should select node-deploy (highest step number)
				assert.Equal(t, "Deployment", result.Resource.Kind)
				assert.Equal(t, "my-deploy", result.Resource.Name)
				assert.Equal(t, "event-deploy", result.ChangeEvent.EventID)
				assert.Equal(t, "ConfigChange", result.CausationType)
				assert.Equal(t, int64(5000), result.TimeLagMs)
			},
		},
		{
			name: "No change event at root - uses first node with event",
			graph: CausalGraph{
				Nodes: []GraphNode{
					{
						ID:         "node-1",
						NodeType:   "SPINE",
						StepNumber: 1,
						Resource: SymptomResource{
							Kind: "Pod",
						},
						ChangeEvent: &ChangeEventInfo{
							EventID:   "event-1",
							Timestamp: time.Unix(0, failureTime-1000000000),
							EventType: "UPDATE",
						},
					},
					{
						ID:          "node-2",
						NodeType:    "SPINE",
						StepNumber:  2,
						Resource:    SymptomResource{Kind: "ReplicaSet"},
						ChangeEvent: nil, // No event
					},
				},
			},
			expectedError: false,
			validate: func(t *testing.T, result *RootCauseHypothesis) {
				// Should use node-1 since node-2 has no event
				assert.Equal(t, "Pod", result.Resource.Kind)
				assert.Equal(t, "event-1", result.ChangeEvent.EventID)
			},
		},
		{
			name: "All nodes have no events - returns error",
			graph: CausalGraph{
				Nodes: []GraphNode{
					{
						ID:          "node-1",
						NodeType:    "SPINE",
						StepNumber:  1,
						ChangeEvent: nil,
					},
					{
						ID:          "node-2",
						NodeType:    "SPINE",
						StepNumber:  2,
						ChangeEvent: nil,
					},
				},
			},
			expectedError: true,
		},
		{
			name: "RELATED nodes are ignored",
			graph: CausalGraph{
				Nodes: []GraphNode{
					{
						ID:         "node-spine",
						NodeType:   "SPINE",
						StepNumber: 1,
						Resource: SymptomResource{
							Kind: "Pod",
						},
						ChangeEvent: &ChangeEventInfo{
							EventID:   "event-spine",
							Timestamp: time.Unix(0, failureTime-1000000000),
							EventType: "UPDATE",
						},
					},
					{
						ID:       "node-related",
						NodeType: "RELATED",
						Resource: SymptomResource{
							Kind: "Node",
						},
						ChangeEvent: &ChangeEventInfo{
							EventID:   "event-related",
							Timestamp: time.Unix(0, failureTime-10000000000),
							EventType: "UPDATE",
						},
					},
				},
			},
			expectedError: false,
			validate: func(t *testing.T, result *RootCauseHypothesis) {
				// Should use SPINE node, not RELATED
				assert.Equal(t, "Pod", result.Resource.Kind)
				assert.Equal(t, "event-spine", result.ChangeEvent.EventID)
			},
		},
		{
			name: "Relationship type affects causation type",
			graph: CausalGraph{
				Nodes: []GraphNode{
					{
						ID:         "node-hr",
						NodeType:   "SPINE",
						StepNumber: 2,
						Resource: SymptomResource{
							Kind: "HelmRelease",
						},
						ChangeEvent: &ChangeEventInfo{
							EventID:   "event-hr",
							Timestamp: time.Unix(0, failureTime-5000000000),
							EventType: "UPDATE",
						},
					},
					{
						ID:         "node-deploy",
						NodeType:   "SPINE",
						StepNumber: 1,
						Resource:   SymptomResource{Kind: "Deployment"},
					},
				},
				Edges: []GraphEdge{
					{
						From:             "node-hr",
						To:               "node-deploy",
						RelationshipType: "MANAGES",
					},
				},
			},
			expectedError: false,
			validate: func(t *testing.T, result *RootCauseHypothesis) {
				// UPDATE + MANAGES = DeploymentUpdate
				assert.Equal(t, "DeploymentUpdate", result.CausationType)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := analyzer.identifyRootCause(tt.graph, failureTime)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Basic validations
				assert.NotEmpty(t, result.Explanation)
				assert.GreaterOrEqual(t, result.TimeLagMs, int64(0))

				// Custom validation
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// TestIdentifyRootCauseTimeLagCalculation validates time lag calculation
func TestIdentifyRootCauseTimeLagCalculation(t *testing.T) {
	analyzer := &RootCauseAnalyzer{}
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		failureTime     time.Time
		eventTime       time.Time
		expectedLagMs   int64
		expectedLagDesc string
	}{
		{
			name:            "1 second lag",
			failureTime:     baseTime,
			eventTime:       baseTime.Add(-1 * time.Second),
			expectedLagMs:   1000,
			expectedLagDesc: "1 second",
		},
		{
			name:            "1 minute lag",
			failureTime:     baseTime,
			eventTime:       baseTime.Add(-1 * time.Minute),
			expectedLagMs:   60000,
			expectedLagDesc: "1 minute",
		},
		{
			name:            "5 minutes lag",
			failureTime:     baseTime,
			eventTime:       baseTime.Add(-5 * time.Minute),
			expectedLagMs:   300000,
			expectedLagDesc: "5 minutes",
		},
		{
			name:            "10 minutes lag",
			failureTime:     baseTime,
			eventTime:       baseTime.Add(-10 * time.Minute),
			expectedLagMs:   600000,
			expectedLagDesc: "10 minutes",
		},
		{
			name:            "Same time (0 lag)",
			failureTime:     baseTime,
			eventTime:       baseTime,
			expectedLagMs:   0,
			expectedLagDesc: "0 seconds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph := CausalGraph{
				Nodes: []GraphNode{
					{
						ID:         "node-1",
						NodeType:   "SPINE",
						StepNumber: 1,
						Resource:   SymptomResource{Kind: "Pod"},
						ChangeEvent: &ChangeEventInfo{
							EventID:   "event-1",
							Timestamp: tt.eventTime,
							EventType: "UPDATE",
						},
					},
				},
			}

			result, err := analyzer.identifyRootCause(graph, tt.failureTime.UnixNano())
			require.NoError(t, err)
			assert.Equal(t, tt.expectedLagMs, result.TimeLagMs, "Time lag should be %s", tt.expectedLagDesc)
		})
	}
}

// TestIdentifyRootCauseExplanationQuality validates explanation quality
func TestIdentifyRootCauseExplanationQuality(t *testing.T) {
	analyzer := &RootCauseAnalyzer{}
	failureTime := time.Now().UnixNano()

	t.Run("Explanation includes resource details", func(t *testing.T) {
		graph := CausalGraph{
			Nodes: []GraphNode{
				{
					ID:         "node-1",
					NodeType:   "SPINE",
					StepNumber: 1,
					Resource: SymptomResource{
						Kind: "HelmRelease",
						Name: "my-application",
					},
					ChangeEvent: &ChangeEventInfo{
						EventID:       "event-1",
						Timestamp:     time.Unix(0, failureTime-5000000000),
						EventType:     "UPDATE",
						ConfigChanged: true,
					},
				},
			},
		}

		result, err := analyzer.identifyRootCause(graph, failureTime)
		require.NoError(t, err)

		assert.Contains(t, result.Explanation, "HelmRelease")
		assert.Contains(t, result.Explanation, "my-application")
		assert.NotEmpty(t, result.Explanation)
	})

	t.Run("Explanation describes propagation path", func(t *testing.T) {
		graph := CausalGraph{
			Nodes: []GraphNode{
				{
					ID:         "node-pod",
					NodeType:   "SPINE",
					StepNumber: 1,
					Resource:   SymptomResource{Kind: "Pod"},
					ChangeEvent: &ChangeEventInfo{
						Timestamp: time.Unix(0, failureTime-1000000000),
						EventType: "UPDATE",
					},
				},
				{
					ID:         "node-rs",
					NodeType:   "SPINE",
					StepNumber: 2,
					Resource:   SymptomResource{Kind: "ReplicaSet"},
					ChangeEvent: &ChangeEventInfo{
						Timestamp: time.Unix(0, failureTime-2000000000),
						EventType: "UPDATE",
					},
				},
				{
					ID:         "node-deploy",
					NodeType:   "SPINE",
					StepNumber: 3,
					Resource:   SymptomResource{Kind: "Deployment"},
					ChangeEvent: &ChangeEventInfo{
						Timestamp:     time.Unix(0, failureTime-5000000000),
						EventType:     "UPDATE",
						ConfigChanged: true,
					},
				},
			},
		}

		result, err := analyzer.identifyRootCause(graph, failureTime)
		require.NoError(t, err)

		// Should mention the root cause and cascading
		explanation := result.Explanation
		assert.Contains(t, explanation, "Deployment")
		assert.Contains(t, explanation, "cascaded", "Should mention cascading")

		// Explanation should be non-empty and meaningful
		assert.NotEmpty(t, explanation)
		assert.Greater(t, len(explanation), 20, "Explanation should be reasonably detailed")
	})
}
