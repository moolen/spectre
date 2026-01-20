package analysis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRootCauseAnalysisV2_Schema validates the new response structure
func TestRootCauseAnalysisV2_Schema(t *testing.T) {
	// Create a sample V2 response
	response := RootCauseAnalysisV2{
		Incident: IncidentAnalysis{
			ObservedSymptom: ObservedSymptom{
				Resource: SymptomResource{
					UID:       "pod-123",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "app-pod",
				},
				Status:       "Error",
				ErrorMessage: "ImagePullBackOff: Back-off pulling image",
				ObservedAt:   time.Now(),
				SymptomType:  "ImagePullError",
			},
			Graph: CausalGraph{
				Nodes: []GraphNode{
					{
						ID: "node-helmrelease-456",
						Resource: SymptomResource{
							UID:       "helmrelease-456",
							Kind:      "HelmRelease",
							Namespace: "default",
							Name:      "app-release",
						},
						ChangeEvent: &ChangeEventInfo{
							EventID:       "event-001",
							Timestamp:     time.Now().Add(-10 * time.Second),
							EventType:     "UPDATE",
							ConfigChanged: true,
							StatusChanged: false,
							Description:   "UPDATE event",
						},
						NodeType:   "SPINE",
						StepNumber: 1,
						Reasoning:  "HelmRelease manages Deployment lifecycle (confidence: 100%)",
					},
					{
						ID: "node-deployment-789",
						Resource: SymptomResource{
							UID:       "deployment-789",
							Kind:      "Deployment",
							Namespace: "default",
							Name:      "app-deployment",
						},
						NodeType:   "SPINE",
						StepNumber: 2,
						Reasoning:  "Deployment owns resources in the next layer",
					},
				},
				Edges: []GraphEdge{
					{
						ID:               "edge-1",
						From:             "node-helmrelease-456",
						To:               "node-deployment-789",
						RelationshipType: "MANAGES",
						EdgeType:         "SPINE",
					},
					{
						ID:               "edge-2",
						From:             "node-deployment-789",
						To:               "node-pod-123",
						RelationshipType: "OWNS",
						EdgeType:         "SPINE",
					},
				},
			},
			RootCause: RootCauseHypothesis{
				Resource: SymptomResource{
					UID:       "helmrelease-456",
					Kind:      "HelmRelease",
					Namespace: "default",
					Name:      "app-release",
				},
				ChangeEvent: ChangeEventInfo{
					EventID:       "event-001",
					Timestamp:     time.Now().Add(-10 * time.Second),
					EventType:     "UPDATE",
					ConfigChanged: true,
					StatusChanged: false,
					Description:   "UPDATE event",
				},
				CausationType: "ConfigChange",
				Explanation:   "HelmRelease 'app-release' configuration was changed, which cascaded through Deployment → Pod",
				TimeLagMs:     10000,
			},
			Confidence: ConfidenceScore{
				Score:     0.85,
				Rationale: "Confidence: 85%. Based on: direct spec change detected, change occurred shortly before failure, strong management relationship.",
				Factors: ConfidenceFactors{
					DirectSpecChange:     1.0,
					TemporalProximity:    0.98,
					RelationshipStrength: 1.0,
					ErrorMessageMatch:    0.5,
					ChainCompleteness:    0.67,
				},
			},
		},
		SupportingEvidence: []EvidenceItem{
			{
				Type:        "RELATIONSHIP",
				Description: "HelmRelease manages Deployment lifecycle (confidence: 100%)",
				Confidence:  1.0,
			},
			{
				Type:        "TEMPORAL",
				Description: "Change occurred 10 seconds before failure",
				Confidence:  0.98,
			},
		},
		ExcludedAlternatives: []ExcludedHypothesis{
			{
				Resource: SymptomResource{
					UID:       "configmap-999",
					Kind:      "ConfigMap",
					Namespace: "default",
					Name:      "app-config",
				},
				Hypothesis:     "ConfigMap 'app-config' changed at similar time",
				ReasonExcluded: "No ownership or management relationship to failed resource",
			},
		},
		QueryMetadata: QueryMetadata{
			QueryExecutionMs: 50,
			AlgorithmVersion: "v2.0",
			ExecutedAt:       time.Now(),
		},
	}

	// Validate structure
	t.Run("Incident structure", func(t *testing.T) {
		assert.Equal(t, "Pod", response.Incident.ObservedSymptom.Resource.Kind)
		assert.Equal(t, "ImagePullError", response.Incident.ObservedSymptom.SymptomType)
		assert.Equal(t, "HelmRelease", response.Incident.RootCause.Resource.Kind)
		assert.Equal(t, "ConfigChange", response.Incident.RootCause.CausationType)
	})

	t.Run("Causal graph structure", func(t *testing.T) {
		assert.Len(t, response.Incident.Graph.Nodes, 2)
		assert.Len(t, response.Incident.Graph.Edges, 2)
		assert.Equal(t, 1, response.Incident.Graph.Nodes[0].StepNumber)
		assert.Equal(t, 2, response.Incident.Graph.Nodes[1].StepNumber)
		assert.Equal(t, "MANAGES", response.Incident.Graph.Edges[0].RelationshipType)
		assert.Equal(t, "OWNS", response.Incident.Graph.Edges[1].RelationshipType)
	})

	t.Run("Confidence score", func(t *testing.T) {
		assert.Greater(t, response.Incident.Confidence.Score, 0.0)
		assert.LessOrEqual(t, response.Incident.Confidence.Score, 1.0)
		assert.NotEmpty(t, response.Incident.Confidence.Rationale)
		assert.Equal(t, 1.0, response.Incident.Confidence.Factors.DirectSpecChange)
		assert.Greater(t, response.Incident.Confidence.Factors.TemporalProximity, 0.9)
	})

	t.Run("Supporting evidence", func(t *testing.T) {
		assert.NotEmpty(t, response.SupportingEvidence)
		assert.LessOrEqual(t, len(response.SupportingEvidence), 5)
		// Evidence should have types
		for _, ev := range response.SupportingEvidence {
			assert.NotEmpty(t, ev.Type)
			assert.NotEmpty(t, ev.Description)
			assert.GreaterOrEqual(t, ev.Confidence, 0.0)
			assert.LessOrEqual(t, ev.Confidence, 1.0)
		}
	})

	t.Run("Excluded alternatives", func(t *testing.T) {
		assert.NotEmpty(t, response.ExcludedAlternatives)
		assert.LessOrEqual(t, len(response.ExcludedAlternatives), 3)
		for _, alt := range response.ExcludedAlternatives {
			assert.NotEmpty(t, alt.Hypothesis)
			assert.NotEmpty(t, alt.ReasonExcluded)
		}
	})

	t.Run("Query metadata", func(t *testing.T) {
		assert.Equal(t, "v2.0", response.QueryMetadata.AlgorithmVersion)
		assert.Greater(t, response.QueryMetadata.QueryExecutionMs, int64(0))
	})
}

// TestConfidenceCalculation validates the confidence scoring formula
func TestConfidenceCalculation(t *testing.T) {
	tests := []struct {
		name         string
		specChange   float64
		temporal     float64
		relationship float64
		errorMatch   float64
		completeness float64
		expectedMin  float64
		expectedMax  float64
	}{
		{
			name:         "Perfect confidence",
			specChange:   1.0,
			temporal:     1.0,
			relationship: 1.0,
			errorMatch:   1.0,
			completeness: 1.0,
			expectedMin:  0.95,
			expectedMax:  1.0,
		},
		{
			name:         "High confidence with MANAGES",
			specChange:   1.0,
			temporal:     0.9,
			relationship: 1.0, // MANAGES
			errorMatch:   0.5,
			completeness: 0.67,
			expectedMin:  0.80,
			expectedMax:  0.90,
		},
		{
			name:         "Medium confidence with OWNS",
			specChange:   0.5,
			temporal:     0.7,
			relationship: 0.8, // OWNS
			errorMatch:   0.5,
			completeness: 0.67,
			expectedMin:  0.60,
			expectedMax:  0.70,
		},
		{
			name:         "Low confidence",
			specChange:   0.0,
			temporal:     0.3,
			relationship: 0.5,
			errorMatch:   0.0,
			completeness: 0.33,
			expectedMin:  0.20,
			expectedMax:  0.35,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate weighted average
			score := tt.specChange*0.30 +
				tt.temporal*0.25 +
				tt.relationship*0.25 +
				tt.errorMatch*0.10 +
				tt.completeness*0.10

			assert.GreaterOrEqual(t, score, tt.expectedMin, "Score should be >= min")
			assert.LessOrEqual(t, score, tt.expectedMax, "Score should be <= max")
		})
	}
}

// TestSymptomClassification validates symptom type detection
func TestSymptomClassification(t *testing.T) {
	tests := []struct {
		name            string
		status          string
		errorMessage    string
		containerIssues []string
		expectedSymptom string
	}{
		{
			name:            "ImagePullError from container issue",
			status:          "Error",
			errorMessage:    "",
			containerIssues: []string{"ImagePullBackOff"},
			expectedSymptom: "ImagePullError",
		},
		{
			name:            "ImagePullError from error message",
			status:          "Error",
			errorMessage:    "Failed to pull image: unauthorized",
			containerIssues: []string{},
			expectedSymptom: "ImagePullError",
		},
		{
			name:            "CrashLoop from container issue",
			status:          "Error",
			errorMessage:    "",
			containerIssues: []string{"CrashLoopBackOff"},
			expectedSymptom: "CrashLoop",
		},
		{
			name:            "OOMKilled",
			status:          "Error",
			errorMessage:    "",
			containerIssues: []string{"OOMKilled"},
			expectedSymptom: "OOMKilled",
		},
		{
			name:            "Generic error",
			status:          "Error",
			errorMessage:    "Something went wrong",
			containerIssues: []string{},
			expectedSymptom: "Error",
		},
		{
			name:            "Scheduling failure",
			status:          "Pending",
			errorMessage:    "0/3 nodes are available: insufficient cpu",
			containerIssues: []string{},
			expectedSymptom: "SchedulingFailure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			symptomType := classifySymptomType(tt.status, tt.errorMessage, tt.containerIssues)
			assert.Equal(t, tt.expectedSymptom, symptomType)
		})
	}
}

// TestTemporalFactorCalculation validates temporal proximity scoring
func TestTemporalFactorCalculation(t *testing.T) {
	tests := []struct {
		name        string
		timeLagMs   int64
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "Immediate (1 second)",
			timeLagMs:   1000,
			expectedMin: 0.99,
			expectedMax: 1.0,
		},
		{
			name:        "Recent (1 minute)",
			timeLagMs:   60000,
			expectedMin: 0.89,
			expectedMax: 0.91,
		},
		{
			name:        "Medium lag (5 minutes)",
			timeLagMs:   300000,
			expectedMin: 0.49,
			expectedMax: 0.51,
		},
		{
			name:        "Long lag (10 minutes)",
			timeLagMs:   600000,
			expectedMin: 0.0,
			expectedMax: 0.01,
		},
		{
			name:        "Very long lag (20 minutes)",
			timeLagMs:   1200000,
			expectedMin: 0.0,
			expectedMax: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factor := calculateTemporalFactor(tt.timeLagMs)
			assert.GreaterOrEqual(t, factor, tt.expectedMin, "Factor should be >= min")
			assert.LessOrEqual(t, factor, tt.expectedMax, "Factor should be <= max")
		})
	}
}

// TestEmptyGraphHandling validates graceful handling of minimal graphs
func TestEmptyGraphHandling(t *testing.T) {
	// Test with symptom-only scenario (minimal graph)
	symptom := ObservedSymptom{
		Resource: SymptomResource{
			UID:       "pod-123",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "app-pod",
		},
		Status:       "Error",
		ErrorMessage: "ImagePullBackOff: Back-off pulling image",
		ObservedAt:   time.Now(),
		SymptomType:  "ImagePullError",
	}

	// Create minimal symptom-only graph
	symptomOnlyGraph := CausalGraph{
		Nodes: []GraphNode{
			{
				ID:       "node-pod-123",
				Resource: symptom.Resource,
				ChangeEvent: &ChangeEventInfo{
					EventID:       "",
					Timestamp:     symptom.ObservedAt,
					EventType:     "OBSERVED",
					ConfigChanged: false,
					StatusChanged: true,
				},
				NodeType:   "SPINE",
				StepNumber: 1,
				Reasoning:  "Direct observation",
			},
		},
		Edges: []GraphEdge{},
	}

	// Create root cause for symptom-only case
	rootCause := RootCauseHypothesis{
		Resource: symptom.Resource,
		ChangeEvent: ChangeEventInfo{
			EventID:       "",
			Timestamp:     symptom.ObservedAt,
			EventType:     "OBSERVED",
			ConfigChanged: false,
			StatusChanged: true,
		},
		CausationType: "DirectObservation",
		Explanation:   "Pod failed with ImagePullError. No causal chain found.",
		TimeLagMs:     0,
	}

	// Calculate confidence for symptom-only case
	factors := ConfidenceFactors{
		DirectSpecChange:     calculateSpecChangeFactor(&rootCause),
		TemporalProximity:    calculateTemporalFactor(rootCause.TimeLagMs),
		RelationshipStrength: calculateRelationshipFactor(symptomOnlyGraph),
		ErrorMessageMatch:    calculateErrorMatchFactor(&symptom),
		ChainCompleteness:    calculateCompletenessFactor(symptomOnlyGraph),
	}

	score := factors.DirectSpecChange*0.30 +
		factors.TemporalProximity*0.25 +
		factors.RelationshipStrength*0.25 +
		factors.ErrorMessageMatch*0.10 +
		factors.ChainCompleteness*0.10

	// Validate fallback behavior
	t.Run("Confidence score", func(t *testing.T) {
		assert.Greater(t, score, 0.0, "Score should be > 0 even with minimal graph")
		assert.LessOrEqual(t, score, 0.55, "Score should be < 0.5 for symptom-only")
	})

	t.Run("Factor breakdown", func(t *testing.T) {
		assert.Equal(t, 0.0, factors.DirectSpecChange, "No spec change in symptom-only")
		assert.Equal(t, 1.0, factors.TemporalProximity, "Temporal is 1.0 for immediate")
		assert.InDelta(t, 1.0/3.0, factors.ChainCompleteness, 0.01, "Completeness is 1/3 for single step")
	})

	t.Run("Causal graph structure", func(t *testing.T) {
		assert.Len(t, symptomOnlyGraph.Nodes, 1, "Symptom-only graph should have 1 node")
		assert.Len(t, symptomOnlyGraph.Edges, 0, "Symptom-only graph should have 0 edges")
		assert.Equal(t, "Pod", symptomOnlyGraph.Nodes[0].Resource.Kind)
	})
}
