package causalpaths

import (
	"testing"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/analysis/anomaly"
	"github.com/stretchr/testify/assert"
)

func TestPathRanker_CalculateTemporalScore(t *testing.T) {
	ranker := NewPathRanker()
	now := time.Now()

	// New scoring rationale (bell-curve, extended for GitOps):
	// - Anomalies < 5s before failure: low score (0.3-0.5) - likely symptoms, not causes
	// - Anomalies 5s-30s before failure: ramping up (0.5-1.0) - may be propagation delay
	// - Anomalies 30s-5min before failure: optimal (1.0) - typical root cause window (extended for GitOps)
	// - Anomalies > 5min before failure: declining (0.6-1.0) - still plausible, gentler decay
	// - Anomalies after failure: 0 - cannot be a cause

	tests := []struct {
		name        string
		anomalyTime time.Time
		failureTime time.Time
		minExpected float64
		maxExpected float64
	}{
		{
			name:        "Same time = low score (likely symptom)",
			anomalyTime: now,
			failureTime: now,
			minExpected: 0.25,
			maxExpected: 0.35,
		},
		{
			name:        "1 minute before = optimal score (in optimal range)",
			anomalyTime: now.Add(-1 * time.Minute),
			failureTime: now,
			minExpected: 0.95,
			maxExpected: 1.0,
		},
		{
			name:        "5 minutes before = optimal score (at edge of extended optimal range)",
			anomalyTime: now.Add(-5 * time.Minute),
			failureTime: now,
			minExpected: 0.95,
			maxExpected: 1.0,
		},
		{
			name:        "10 minutes before = good score (gentler decay, higher floor)",
			anomalyTime: now.Add(-10 * time.Minute),
			failureTime: now,
			minExpected: 0.55,
			maxExpected: 0.75,
		},
		{
			name:        "After failure = zero score",
			anomalyTime: now.Add(1 * time.Minute),
			failureTime: now,
			minExpected: 0.0,
			maxExpected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ranker.calculateTemporalScore(tt.anomalyTime, tt.failureTime)
			assert.GreaterOrEqual(t, result, tt.minExpected, "Score should be >= %f", tt.minExpected)
			assert.LessOrEqual(t, result, tt.maxExpected, "Score should be <= %f", tt.maxExpected)
		})
	}
}

func TestPathRanker_CalculateEffectiveCausalDistance(t *testing.T) {
	ranker := NewPathRanker()

	tests := []struct {
		name     string
		path     CausalPath
		expected int
	}{
		{
			name: "No edges (single node)",
			path: CausalPath{
				Steps: []PathStep{
					{Node: PathNode{ID: "root"}, Edge: nil},
				},
			},
			expected: 0,
		},
		{
			name: "One cause-introducing edge",
			path: CausalPath{
				Steps: []PathStep{
					{Node: PathNode{ID: "root"}, Edge: nil},
					{Node: PathNode{ID: "child"}, Edge: &PathEdge{
						EdgeCategory: EdgeCategoryCauseIntroducing,
					}},
				},
			},
			expected: 1,
		},
		{
			name: "One materialization edge",
			path: CausalPath{
				Steps: []PathStep{
					{Node: PathNode{ID: "root"}, Edge: nil},
					{Node: PathNode{ID: "child"}, Edge: &PathEdge{
						EdgeCategory: EdgeCategoryMaterialization,
					}},
				},
			},
			expected: 0,
		},
		{
			name: "Mixed edges - only cause-introducing counted",
			path: CausalPath{
				Steps: []PathStep{
					{Node: PathNode{ID: "helmrelease"}, Edge: nil},
					{Node: PathNode{ID: "deployment"}, Edge: &PathEdge{
						EdgeCategory: EdgeCategoryCauseIntroducing, // MANAGES
					}},
					{Node: PathNode{ID: "replicaset"}, Edge: &PathEdge{
						EdgeCategory: EdgeCategoryMaterialization, // OWNS
					}},
					{Node: PathNode{ID: "pod"}, Edge: &PathEdge{
						EdgeCategory: EdgeCategoryMaterialization, // OWNS
					}},
				},
			},
			expected: 1, // Only MANAGES counts
		},
		{
			name: "Multiple cause-introducing edges",
			path: CausalPath{
				Steps: []PathStep{
					{Node: PathNode{ID: "configmap"}, Edge: nil},
					{Node: PathNode{ID: "deployment"}, Edge: &PathEdge{
						EdgeCategory: EdgeCategoryCauseIntroducing, // REFERENCES_SPEC
					}},
					{Node: PathNode{ID: "controller"}, Edge: &PathEdge{
						EdgeCategory: EdgeCategoryCauseIntroducing, // MANAGES
					}},
					{Node: PathNode{ID: "pod"}, Edge: &PathEdge{
						EdgeCategory: EdgeCategoryMaterialization, // OWNS
					}},
				},
			},
			expected: 2, // Two cause-introducing edges
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ranker.calculateEffectiveCausalDistance(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPathRanker_CalculateSeverityScore(t *testing.T) {
	ranker := NewPathRanker()

	tests := []struct {
		name             string
		path             CausalPath
		expectedSeverity string
		expectedScore    float64
	}{
		{
			name: "Critical severity",
			path: CausalPath{
				Steps: []PathStep{
					{Node: PathNode{
						Anomalies: []anomaly.Anomaly{
							{Severity: anomaly.SeverityCritical},
						},
					}},
				},
			},
			expectedSeverity: "critical",
			expectedScore:    1.0,
		},
		{
			name: "High severity",
			path: CausalPath{
				Steps: []PathStep{
					{Node: PathNode{
						Anomalies: []anomaly.Anomaly{
							{Severity: anomaly.SeverityHigh},
						},
					}},
				},
			},
			expectedSeverity: "high",
			expectedScore:    0.75,
		},
		{
			name: "Mixed severities - returns max",
			path: CausalPath{
				Steps: []PathStep{
					{Node: PathNode{
						Anomalies: []anomaly.Anomaly{
							{Severity: anomaly.SeverityLow},
						},
					}},
					{Node: PathNode{
						Anomalies: []anomaly.Anomaly{
							{Severity: anomaly.SeverityHigh},
							{Severity: anomaly.SeverityMedium},
						},
					}},
				},
			},
			expectedSeverity: "high",
			expectedScore:    0.75,
		},
		{
			name: "No anomalies defaults to low",
			path: CausalPath{
				Steps: []PathStep{
					{Node: PathNode{Anomalies: []anomaly.Anomaly{}}},
				},
			},
			expectedSeverity: "low",
			expectedScore:    0.25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			severity, score := ranker.calculateSeverityScore(tt.path)
			assert.Equal(t, tt.expectedSeverity, severity)
			assert.Equal(t, tt.expectedScore, score)
		})
	}
}

func TestPathRanker_RankPaths(t *testing.T) {
	ranker := NewPathRanker()
	now := time.Now()

	// Create paths with different characteristics
	paths := []CausalPath{
		{
			// Path 1: Old anomaly, short distance, high severity
			ID:             "path-1",
			FirstAnomalyAt: now.Add(-8 * time.Minute),
			Steps: []PathStep{
				{Node: PathNode{
					ID:        "root1",
					Resource:  analysis.SymptomResource{UID: "root1"},
					Anomalies: []anomaly.Anomaly{{Severity: anomaly.SeverityHigh}},
				}, Edge: nil},
				{Node: PathNode{ID: "symptom"}, Edge: &PathEdge{EdgeCategory: EdgeCategoryCauseIntroducing}},
			},
		},
		{
			// Path 2: Recent anomaly, short distance, critical severity (should rank highest)
			ID:             "path-2",
			FirstAnomalyAt: now.Add(-1 * time.Minute),
			Steps: []PathStep{
				{Node: PathNode{
					ID:        "root2",
					Resource:  analysis.SymptomResource{UID: "root2"},
					Anomalies: []anomaly.Anomaly{{Severity: anomaly.SeverityCritical}},
				}, Edge: nil},
				{Node: PathNode{ID: "symptom"}, Edge: &PathEdge{EdgeCategory: EdgeCategoryCauseIntroducing}},
			},
		},
		{
			// Path 3: Recent anomaly, long distance, low severity
			ID:             "path-3",
			FirstAnomalyAt: now.Add(-2 * time.Minute),
			Steps: []PathStep{
				{Node: PathNode{
					ID:        "root3",
					Resource:  analysis.SymptomResource{UID: "root3"},
					Anomalies: []anomaly.Anomaly{{Severity: anomaly.SeverityLow}},
				}, Edge: nil},
				{Node: PathNode{ID: "n1"}, Edge: &PathEdge{EdgeCategory: EdgeCategoryCauseIntroducing}},
				{Node: PathNode{ID: "n2"}, Edge: &PathEdge{EdgeCategory: EdgeCategoryCauseIntroducing}},
				{Node: PathNode{ID: "n3"}, Edge: &PathEdge{EdgeCategory: EdgeCategoryCauseIntroducing}},
				{Node: PathNode{ID: "symptom"}, Edge: &PathEdge{EdgeCategory: EdgeCategoryCauseIntroducing}},
			},
		},
	}

	rankedPaths := ranker.RankPaths(paths, now)

	// Verify paths are sorted by confidence score (descending)
	assert.Len(t, rankedPaths, 3)

	// Path 2 should rank highest (recent + critical + short)
	assert.Equal(t, "path-2", rankedPaths[0].ID)
	assert.Greater(t, rankedPaths[0].ConfidenceScore, rankedPaths[1].ConfidenceScore)
	assert.Greater(t, rankedPaths[1].ConfidenceScore, rankedPaths[2].ConfidenceScore)

	// Verify confidence scores are in valid range
	for _, p := range rankedPaths {
		assert.GreaterOrEqual(t, p.ConfidenceScore, 0.0)
		assert.LessOrEqual(t, p.ConfidenceScore, 1.0)
	}
}

func TestPathRanker_ConfidenceScoreWeights(t *testing.T) {
	// Verify that weights sum to approximately 1.0
	totalWeight := WeightTemporal + WeightDistance + WeightSeverity
	assert.InDelta(t, 1.0, totalWeight, 0.001, "Weights should sum to 1.0")
}
