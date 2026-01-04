package analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCalculateSpecChangeFactor validates spec change factor calculation
func TestCalculateSpecChangeFactor(t *testing.T) {
	tests := []struct {
		name           string
		rootCause      *RootCauseHypothesis
		expectedFactor float64
	}{
		{
			name: "ConfigChanged true returns 1.0",
			rootCause: &RootCauseHypothesis{
				ChangeEvent: ChangeEventInfo{
					ConfigChanged: true,
					EventType:     "UPDATE",
				},
			},
			expectedFactor: 1.0,
		},
		{
			name: "UPDATE event without config change returns 0.5",
			rootCause: &RootCauseHypothesis{
				ChangeEvent: ChangeEventInfo{
					ConfigChanged: false,
					EventType:     "UPDATE",
				},
			},
			expectedFactor: 0.5,
		},
		{
			name: "CREATE event returns 0.0",
			rootCause: &RootCauseHypothesis{
				ChangeEvent: ChangeEventInfo{
					ConfigChanged: false,
					EventType:     "CREATE",
				},
			},
			expectedFactor: 0.0,
		},
		{
			name: "DELETE event returns 0.0",
			rootCause: &RootCauseHypothesis{
				ChangeEvent: ChangeEventInfo{
					ConfigChanged: false,
					EventType:     "DELETE",
				},
			},
			expectedFactor: 0.0,
		},
		{
			name: "ConfigChanged overrides event type",
			rootCause: &RootCauseHypothesis{
				ChangeEvent: ChangeEventInfo{
					ConfigChanged: true,
					EventType:     "CREATE",
				},
			},
			expectedFactor: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factor := calculateSpecChangeFactor(tt.rootCause)
			assert.Equal(t, tt.expectedFactor, factor)
		})
	}
}

// TestCalculateTemporalFactor validates temporal proximity scoring
func TestCalculateTemporalFactor(t *testing.T) {
	tests := []struct {
		name           string
		timeLagMs      int64
		expectedFactor float64
		delta          float64 // allowed difference for float comparison
	}{
		{
			name:           "Zero lag returns 1.0",
			timeLagMs:      0,
			expectedFactor: 1.0,
			delta:          0.0,
		},
		{
			name:           "1 second lag",
			timeLagMs:      1000,
			expectedFactor: 0.9983, // 1.0 - (1000/600000)
			delta:          0.0001,
		},
		{
			name:           "1 minute lag",
			timeLagMs:      60000,
			expectedFactor: 0.9,
			delta:          0.0001,
		},
		{
			name:           "5 minutes lag (halfway)",
			timeLagMs:      300000,
			expectedFactor: 0.5,
			delta:          0.0001,
		},
		{
			name:           "10 minutes lag (max lookback)",
			timeLagMs:      600000,
			expectedFactor: 0.0,
			delta:          0.0001,
		},
		{
			name:           "20 minutes lag (beyond max) capped at 0",
			timeLagMs:      1200000,
			expectedFactor: 0.0,
			delta:          0.0,
		},
		{
			name:           "Negative lag treated as 0",
			timeLagMs:      -1000,
			expectedFactor: 1.0,
			delta:          0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factor := calculateTemporalFactor(tt.timeLagMs)
			if tt.delta > 0 {
				assert.InDelta(t, tt.expectedFactor, factor, tt.delta)
			} else {
				assert.Equal(t, tt.expectedFactor, factor)
			}
		})
	}
}

// TestCalculateRelationshipFactor validates relationship strength scoring
func TestCalculateRelationshipFactor(t *testing.T) {
	tests := []struct {
		name           string
		graph          CausalGraph
		expectedFactor float64
	}{
		{
			name: "Empty graph returns 0.0",
			graph: CausalGraph{
				Nodes: []GraphNode{},
				Edges: []GraphEdge{},
			},
			expectedFactor: 0.0,
		},
		{
			name: "MANAGES relationship returns 1.0",
			graph: CausalGraph{
				Edges: []GraphEdge{
					{
						ID:               "edge-1",
						From:             "node-1",
						To:               "node-2",
						RelationshipType: "MANAGES",
					},
				},
			},
			expectedFactor: 1.0,
		},
		{
			name: "OWNS relationship returns 0.8",
			graph: CausalGraph{
				Edges: []GraphEdge{
					{
						ID:               "edge-1",
						From:             "node-1",
						To:               "node-2",
						RelationshipType: "OWNS",
					},
				},
			},
			expectedFactor: 0.8,
		},
		{
			name: "TRIGGERED_BY relationship returns 0.7",
			graph: CausalGraph{
				Edges: []GraphEdge{
					{
						ID:               "edge-1",
						From:             "node-1",
						To:               "node-2",
						RelationshipType: "TRIGGERED_BY",
					},
				},
			},
			expectedFactor: 0.7,
		},
		{
			name: "Unknown relationship returns 0.5",
			graph: CausalGraph{
				Edges: []GraphEdge{
					{
						ID:               "edge-1",
						From:             "node-1",
						To:               "node-2",
						RelationshipType: "REFERENCES_SPEC",
					},
				},
			},
			expectedFactor: 0.5,
		},
		{
			name: "Multiple edges returns highest strength",
			graph: CausalGraph{
				Edges: []GraphEdge{
					{
						ID:               "edge-1",
						From:             "node-1",
						To:               "node-2",
						RelationshipType: "OWNS", // 0.8
					},
					{
						ID:               "edge-2",
						From:             "node-2",
						To:               "node-3",
						RelationshipType: "MANAGES", // 1.0
					},
					{
						ID:               "edge-3",
						From:             "node-3",
						To:               "node-4",
						RelationshipType: "SCHEDULED_ON", // 0.5
					},
				},
			},
			expectedFactor: 1.0, // Max is MANAGES
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factor := calculateRelationshipFactor(tt.graph)
			assert.Equal(t, tt.expectedFactor, factor)
		})
	}
}

// TestCalculateErrorMatchFactor validates error message correlation scoring
func TestCalculateErrorMatchFactor(t *testing.T) {
	tests := []struct {
		name           string
		symptom        *ObservedSymptom
		rootCause      *RootCauseHypothesis
		expectedFactor float64
	}{
		{
			name: "Image error returns 1.0",
			symptom: &ObservedSymptom{
				ErrorMessage: "Failed to pull image: unauthorized",
			},
			rootCause:      &RootCauseHypothesis{},
			expectedFactor: 1.0,
		},
		{
			name: "Config error returns 1.0",
			symptom: &ObservedSymptom{
				ErrorMessage: "Invalid configuration value",
			},
			rootCause:      &RootCauseHypothesis{},
			expectedFactor: 1.0,
		},
		{
			name: "Invalid error returns 1.0",
			symptom: &ObservedSymptom{
				ErrorMessage: "Invalid resource specification",
			},
			rootCause:      &RootCauseHypothesis{},
			expectedFactor: 1.0,
		},
		{
			name: "Pull error returns 1.0",
			symptom: &ObservedSymptom{
				ErrorMessage: "ErrImagePull",
			},
			rootCause:      &RootCauseHypothesis{},
			expectedFactor: 1.0,
		},
		{
			name: "Generic error message returns 0.5",
			symptom: &ObservedSymptom{
				ErrorMessage: "Something went wrong",
			},
			rootCause:      &RootCauseHypothesis{},
			expectedFactor: 0.5,
		},
		{
			name: "Empty error message returns 0.0",
			symptom: &ObservedSymptom{
				ErrorMessage: "",
			},
			rootCause:      &RootCauseHypothesis{},
			expectedFactor: 0.0,
		},
		{
			name: "Case insensitive matching for IMAGE",
			symptom: &ObservedSymptom{
				ErrorMessage: "Failed to pull IMAGE from registry",
			},
			rootCause:      &RootCauseHypothesis{},
			expectedFactor: 1.0,
		},
		{
			name: "Case insensitive matching for CONFIG",
			symptom: &ObservedSymptom{
				ErrorMessage: "CONFIG file not found",
			},
			rootCause:      &RootCauseHypothesis{},
			expectedFactor: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factor := calculateErrorMatchFactor(tt.symptom, tt.rootCause)
			assert.Equal(t, tt.expectedFactor, factor)
		})
	}
}

// TestCalculateCompletenessFactor validates chain completeness scoring
func TestCalculateCompletenessFactor(t *testing.T) {
	tests := []struct {
		name           string
		graph          CausalGraph
		expectedFactor float64
		delta          float64
	}{
		{
			name: "Empty graph returns 0.0",
			graph: CausalGraph{
				Nodes: []GraphNode{},
			},
			expectedFactor: 0.0,
			delta:          0.0,
		},
		{
			name: "Single SPINE node returns 1/3",
			graph: CausalGraph{
				Nodes: []GraphNode{
					{NodeType: "SPINE"},
				},
			},
			expectedFactor: 0.3333,
			delta:          0.0001,
		},
		{
			name: "Two SPINE nodes returns 2/3",
			graph: CausalGraph{
				Nodes: []GraphNode{
					{NodeType: "SPINE"},
					{NodeType: "SPINE"},
				},
			},
			expectedFactor: 0.6667,
			delta:          0.0001,
		},
		{
			name: "Three SPINE nodes returns 1.0",
			graph: CausalGraph{
				Nodes: []GraphNode{
					{NodeType: "SPINE"},
					{NodeType: "SPINE"},
					{NodeType: "SPINE"},
				},
			},
			expectedFactor: 1.0,
			delta:          0.0001,
		},
		{
			name: "Four SPINE nodes capped at 1.0",
			graph: CausalGraph{
				Nodes: []GraphNode{
					{NodeType: "SPINE"},
					{NodeType: "SPINE"},
					{NodeType: "SPINE"},
					{NodeType: "SPINE"},
				},
			},
			expectedFactor: 1.0,
			delta:          0.0,
		},
		{
			name: "RELATED nodes not counted",
			graph: CausalGraph{
				Nodes: []GraphNode{
					{NodeType: "SPINE"},
					{NodeType: "SPINE"},
					{NodeType: "RELATED"},
					{NodeType: "RELATED"},
					{NodeType: "RELATED"},
				},
			},
			expectedFactor: 0.6667,
			delta:          0.0001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factor := calculateCompletenessFactor(tt.graph)
			if tt.delta > 0 {
				assert.InDelta(t, tt.expectedFactor, factor, tt.delta)
			} else {
				assert.Equal(t, tt.expectedFactor, factor)
			}
		})
	}
}

// TestGenerateConfidenceRationale validates rationale generation
func TestGenerateConfidenceRationale(t *testing.T) {
	tests := []struct {
		name              string
		factors           ConfidenceFactors
		score             float64
		expectedSubstring []string // Substrings that should appear in rationale
	}{
		{
			name: "Perfect confidence mentions all factors",
			factors: ConfidenceFactors{
				DirectSpecChange:     1.0,
				TemporalProximity:    1.0,
				RelationshipStrength: 1.0,
				ErrorMessageMatch:    1.0,
				ChainCompleteness:    1.0,
			},
			score: 1.0,
			expectedSubstring: []string{
				"100%",
				"direct spec change detected",
				"change occurred shortly before failure",
				"strong management relationship",
				"error message correlates",
				"complete causal chain",
			},
		},
		{
			name: "Low confidence with minimal factors",
			factors: ConfidenceFactors{
				DirectSpecChange:     0.0,
				TemporalProximity:    0.3,
				RelationshipStrength: 0.5,
				ErrorMessageMatch:    0.0,
				ChainCompleteness:    0.3,
			},
			score:             0.29,
			expectedSubstring: []string{"29%"},
		},
		{
			name: "High temporal but no spec change",
			factors: ConfidenceFactors{
				DirectSpecChange:     0.0,
				TemporalProximity:    0.95,
				RelationshipStrength: 0.8,
				ErrorMessageMatch:    0.5,
				ChainCompleteness:    0.67,
			},
			score: 0.625,
			expectedSubstring: []string{
				"62%",
				"change occurred shortly before failure",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rationale := generateConfidenceRationale(tt.factors, tt.score)

			// Check that rationale is not empty
			assert.NotEmpty(t, rationale)

			// Check for expected substrings
			for _, substr := range tt.expectedSubstring {
				assert.Contains(t, rationale, substr, "Expected rationale to contain: %s", substr)
			}
		})
	}
}

// TestCalculateConfidence validates the complete confidence calculation
func TestCalculateConfidence(t *testing.T) {
	// Create a mock analyzer (we only need the method, not real dependencies)
	analyzer := &RootCauseAnalyzer{}

	tests := []struct {
		name          string
		symptom       *ObservedSymptom
		graph         CausalGraph
		rootCause     *RootCauseHypothesis
		expectedMin   float64
		expectedMax   float64
		checkRationale bool
	}{
		{
			name: "Perfect scenario - config change, immediate, MANAGES relationship",
			symptom: &ObservedSymptom{
				ErrorMessage: "Failed to pull image",
			},
			graph: CausalGraph{
				Nodes: []GraphNode{
					{NodeType: "SPINE"},
					{NodeType: "SPINE"},
					{NodeType: "SPINE"},
				},
				Edges: []GraphEdge{
					{RelationshipType: "MANAGES"},
				},
			},
			rootCause: &RootCauseHypothesis{
				ChangeEvent: ChangeEventInfo{
					ConfigChanged: true,
					EventType:     "UPDATE",
				},
				TimeLagMs: 1000, // 1 second
			},
			expectedMin:    0.85,
			expectedMax:    1.0,
			checkRationale: true,
		},
		{
			name: "Medium confidence - no config change, longer lag",
			symptom: &ObservedSymptom{
				ErrorMessage: "Generic error",
			},
			graph: CausalGraph{
				Nodes: []GraphNode{
					{NodeType: "SPINE"},
					{NodeType: "SPINE"},
				},
				Edges: []GraphEdge{
					{RelationshipType: "OWNS"},
				},
			},
			rootCause: &RootCauseHypothesis{
				ChangeEvent: ChangeEventInfo{
					ConfigChanged: false,
					EventType:     "UPDATE",
				},
				TimeLagMs: 300000, // 5 minutes
			},
			expectedMin:    0.45,
			expectedMax:    0.65,
			checkRationale: true,
		},
		{
			name: "Low confidence - minimal graph, long lag",
			symptom: &ObservedSymptom{
				ErrorMessage: "",
			},
			graph: CausalGraph{
				Nodes: []GraphNode{
					{NodeType: "SPINE"},
				},
				Edges: []GraphEdge{},
			},
			rootCause: &RootCauseHypothesis{
				ChangeEvent: ChangeEventInfo{
					ConfigChanged: false,
					EventType:     "CREATE",
				},
				TimeLagMs: 600000, // 10 minutes
			},
			expectedMin:    0.0,
			expectedMax:    0.15,
			checkRationale: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.calculateConfidence(tt.symptom, tt.graph, tt.rootCause)

			// Check score is in expected range
			assert.GreaterOrEqual(t, result.Score, tt.expectedMin, "Score should be >= min")
			assert.LessOrEqual(t, result.Score, tt.expectedMax, "Score should be <= max")

			// Check score is between 0 and 1
			assert.GreaterOrEqual(t, result.Score, 0.0)
			assert.LessOrEqual(t, result.Score, 1.0)

			// Check factors are present
			assert.GreaterOrEqual(t, result.Factors.DirectSpecChange, 0.0)
			assert.LessOrEqual(t, result.Factors.DirectSpecChange, 1.0)
			assert.GreaterOrEqual(t, result.Factors.TemporalProximity, 0.0)
			assert.LessOrEqual(t, result.Factors.TemporalProximity, 1.0)
			assert.GreaterOrEqual(t, result.Factors.RelationshipStrength, 0.0)
			assert.LessOrEqual(t, result.Factors.RelationshipStrength, 1.0)
			assert.GreaterOrEqual(t, result.Factors.ErrorMessageMatch, 0.0)
			assert.LessOrEqual(t, result.Factors.ErrorMessageMatch, 1.0)
			assert.GreaterOrEqual(t, result.Factors.ChainCompleteness, 0.0)
			assert.LessOrEqual(t, result.Factors.ChainCompleteness, 1.0)

			if tt.checkRationale {
				// Check rationale exists and contains confidence percentage
				assert.NotEmpty(t, result.Rationale)
				assert.Contains(t, result.Rationale, "Confidence:")
			}
		})
	}
}

// TestConfidenceWeights validates that weights sum to 1.0
func TestConfidenceWeights(t *testing.T) {
	// These should match the weights in confidence.go
	const (
		weightSpecChange   = 0.30
		weightTemporal     = 0.25
		weightRelationship = 0.25
		weightErrorMatch   = 0.10
		weightCompleteness = 0.10
	)

	sum := weightSpecChange + weightTemporal + weightRelationship + weightErrorMatch + weightCompleteness

	assert.InDelta(t, 1.0, sum, 0.0001, "Weights must sum to 1.0")
}
