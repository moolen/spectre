package causalpaths

import (
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/analysis/anomaly"
)

// CausalPathsInput defines input parameters for causal path discovery
type CausalPathsInput struct {
	ResourceUID      string // Symptom resource UID (required)
	FailureTimestamp int64  // Unix nanoseconds (required)
	LookbackNs       int64  // Lookback window in nanoseconds (default: 10 minutes)
	MaxDepth         int    // Maximum traversal depth (default: 5)
	MaxPaths         int    // Maximum paths to return (default: 5)
}

// CausalPathsResponse is the API response structure
type CausalPathsResponse struct {
	Paths    []CausalPath     `json:"paths"`
	Metadata ResponseMetadata `json:"metadata"`
}

// CausalPath represents a single causal path from root cause to symptom
type CausalPath struct {
	ID              string      `json:"id"`              // Unique path identifier (deterministic hash)
	CandidateRoot   PathNode    `json:"candidateRoot"`   // The identified root cause node
	FirstAnomalyAt  time.Time   `json:"firstAnomalyAt"`  // Timestamp of first anomaly in this path
	Steps           []PathStep  `json:"steps"`           // Ordered sequence: root -> ... -> symptom
	ConfidenceScore float64     `json:"confidenceScore"` // 0.0-1.0, deterministic
	Explanation     string      `json:"explanation"`     // Human-readable explanation
	Ranking         PathRanking `json:"ranking"`         // Breakdown of ranking factors

	// AffectedSymptoms contains all symptoms that share this root cause.
	// When multiple paths lead to the same root cause (e.g., 5 Pods affected by Node DiskPressure),
	// deduplication keeps one representative path but preserves info about all affected symptoms.
	// The first element is always the primary symptom from the representative path.
	AffectedSymptoms []PathNode `json:"affectedSymptoms,omitempty"`

	// AffectedCount is the total number of symptoms affected by this root cause.
	// This is always >= 1 (the primary symptom from Steps).
	AffectedCount int `json:"affectedCount"`
}

// PathStep represents one hop in the causal path
type PathStep struct {
	Node PathNode  `json:"node"`
	Edge *PathEdge `json:"edge,omitempty"` // Edge TO this node (nil for root)
}

// PathNode represents a node in the causal path
type PathNode struct {
	ID           string                    `json:"id"`
	Resource     analysis.SymptomResource  `json:"resource"`
	Anomalies    []anomaly.Anomaly         `json:"anomalies"`              // Detected anomalies at this node
	PrimaryEvent *analysis.ChangeEventInfo `json:"primaryEvent,omitempty"` // Most relevant change event
}

// PathEdge represents an edge in the causal path
type PathEdge struct {
	ID               string  `json:"id"`
	RelationshipType string  `json:"relationshipType"` // OWNS, MANAGES, SCHEDULED_ON, etc.
	EdgeCategory     string  `json:"edgeCategory"`     // "CAUSE_INTRODUCING" or "MATERIALIZATION"
	CausalWeight     float64 `json:"causalWeight"`     // 1.0 for cause-introducing, 0.0 for materialization
}

// PathRanking contains the breakdown of ranking factors
type PathRanking struct {
	TemporalScore           float64 `json:"temporalScore"`           // Earlier = higher (0.0-1.0)
	EffectiveCausalDistance int     `json:"effectiveCausalDistance"` // Count of cause-introducing edges
	MaxAnomalySeverity      string  `json:"maxAnomalySeverity"`      // Highest severity in path
	SeverityScore           float64 `json:"severityScore"`           // Numeric severity (0.0-1.0)

	// RankingExplanation provides a human-readable explanation of why this path
	// was ranked at its position. This helps users understand the confidence score.
	RankingExplanation string `json:"rankingExplanation,omitempty"`

	// TemporalExplanation describes the temporal relationship between the anomaly and failure
	TemporalExplanation string `json:"temporalExplanation,omitempty"`

	// DistanceExplanation describes the causal distance (number of cause-introducing edges)
	DistanceExplanation string `json:"distanceExplanation,omitempty"`

	// SeverityExplanation describes the severity of anomalies in the path
	SeverityExplanation string `json:"severityExplanation,omitempty"`
}

// ResponseMetadata provides execution information
type ResponseMetadata struct {
	QueryExecutionMs int64     `json:"queryExecutionMs"`
	AlgorithmVersion string    `json:"algorithmVersion"`
	ExecutedAt       time.Time `json:"executedAt"`
	NodesExplored    int       `json:"nodesExplored"`
	PathsDiscovered  int       `json:"pathsDiscovered"`
	PathsReturned    int       `json:"pathsReturned"`
}
