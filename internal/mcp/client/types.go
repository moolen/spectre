package client

import "encoding/json"

// TimelineResponse represents the response from the timeline API
type TimelineResponse struct {
	Resources  []TimelineResource `json:"resources"`
	Count      int                `json:"count"`
	ExecTimeMs int64              `json:"executionTimeMs"`
}

// TimelineResource represents a resource in the timeline response
type TimelineResource struct {
	ID             string          `json:"id"`
	Group          string          `json:"group"`
	Version        string          `json:"version"`
	Kind           string          `json:"kind"`
	Namespace      string          `json:"namespace"`
	Name           string          `json:"name"`
	StatusSegments []StatusSegment `json:"statusSegments"`
	Events         []K8sEvent      `json:"events"`
}

// StatusSegment represents a time period with a specific status
type StatusSegment struct {
	StartTime    int64           `json:"startTime"`
	EndTime      int64           `json:"endTime"`
	Status       string          `json:"status"` // Ready, Warning, Error, Terminating, Unknown
	Message      string          `json:"message"`
	ResourceData json.RawMessage `json:"resourceData"`
}

// K8sEvent represents a Kubernetes event
type K8sEvent struct {
	ID             string `json:"id"`
	Timestamp      int64  `json:"timestamp"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	Type           string `json:"type"` // Normal, Warning
	Count          int32  `json:"count"`
	Source         string `json:"source"`
	FirstTimestamp int64  `json:"firstTimestamp"`
	LastTimestamp  int64  `json:"lastTimestamp"`
}

// MetadataResponse represents cluster metadata
type MetadataResponse struct {
	Namespaces []string  `json:"namespaces"`
	Kinds      []string  `json:"kinds"`
	TimeRange  TimeRange `json:"timeRange"`
}

// TimeRange represents the time range of available data
type TimeRange struct {
	Start int64 `json:"earliest"`
	End   int64 `json:"latest"`
}

// AnomalyResponse represents the response from the anomalies API
type AnomalyResponse struct {
	Anomalies []Anomaly       `json:"anomalies"`
	Metadata  AnomalyMetadata `json:"metadata"`
}

// Anomaly represents a single detected anomaly
type Anomaly struct {
	Node      AnomalyNode            `json:"node"`
	Category  string                 `json:"category"`
	Type      string                 `json:"type"`
	Severity  string                 `json:"severity"`
	Timestamp string                 `json:"timestamp"` // RFC3339 format from API
	Summary   string                 `json:"summary"`
	Details   map[string]interface{} `json:"details"`
}

// AnomalyNode identifies the resource exhibiting the anomaly
type AnomalyNode struct {
	UID       string `json:"uid"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// AnomalyMetadata provides context about the analysis
type AnomalyMetadata struct {
	ResourceUID   string            `json:"resource_uid"`
	TimeWindow    AnomalyTimeWindow `json:"time_window"`
	NodesAnalyzed int               `json:"nodes_analyzed"`
	ExecTimeMs    int64             `json:"execution_time_ms"`
}

// AnomalyTimeWindow represents the analysis time range
type AnomalyTimeWindow struct {
	Start string `json:"start"` // RFC3339 format
	End   string `json:"end"`   // RFC3339 format
}

// CausalPathsResponse represents the response from the causal paths API
type CausalPathsResponse struct {
	Paths    []CausalPath        `json:"paths"`
	Metadata CausalPathsMetadata `json:"metadata"`
}

// CausalPath represents a single causal path from root cause to symptom
type CausalPath struct {
	ID               string            `json:"id"`
	CandidateRoot    CausalPathNode    `json:"candidateRoot"`
	FirstAnomalyAt   string            `json:"firstAnomalyAt"` // RFC3339 format
	Steps            []CausalPathStep  `json:"steps"`
	ConfidenceScore  float64           `json:"confidenceScore"`
	Explanation      string            `json:"explanation"`
	Ranking          CausalPathRanking `json:"ranking"`
	AffectedSymptoms []CausalPathNode  `json:"affectedSymptoms,omitempty"`
	AffectedCount    int               `json:"affectedCount"`
}

// CausalPathStep represents one hop in the causal path
type CausalPathStep struct {
	Node CausalPathNode  `json:"node"`
	Edge *CausalPathEdge `json:"edge,omitempty"`
}

// CausalPathNode represents a node in the causal path
type CausalPathNode struct {
	ID           string                 `json:"id"`
	Resource     CausalPathResource     `json:"resource"`
	Anomalies    []interface{}          `json:"anomalies"`
	PrimaryEvent map[string]interface{} `json:"primaryEvent,omitempty"`
}

// CausalPathResource represents resource information
type CausalPathResource struct {
	UID       string `json:"uid"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// CausalPathEdge represents an edge in the causal path
type CausalPathEdge struct {
	ID               string  `json:"id"`
	RelationshipType string  `json:"relationshipType"`
	EdgeCategory     string  `json:"edgeCategory"`
	CausalWeight     float64 `json:"causalWeight"`
}

// CausalPathRanking contains ranking factors
type CausalPathRanking struct {
	TemporalScore           float64 `json:"temporalScore"`
	EffectiveCausalDistance int     `json:"effectiveCausalDistance"`
	MaxAnomalySeverity      string  `json:"maxAnomalySeverity"`
	SeverityScore           float64 `json:"severityScore"`
	RankingExplanation      string  `json:"rankingExplanation,omitempty"`
	TemporalExplanation     string  `json:"temporalExplanation,omitempty"`
	DistanceExplanation     string  `json:"distanceExplanation,omitempty"`
	SeverityExplanation     string  `json:"severityExplanation,omitempty"`
}

// CausalPathsMetadata provides execution information
type CausalPathsMetadata struct {
	QueryExecutionMs int64  `json:"queryExecutionMs"`
	AlgorithmVersion string `json:"algorithmVersion"`
	ExecutedAt       string `json:"executedAt"` // RFC3339 format
	NodesExplored    int    `json:"nodesExplored"`
	PathsDiscovered  int    `json:"pathsDiscovered"`
	PathsReturned    int    `json:"pathsReturned"`
}
