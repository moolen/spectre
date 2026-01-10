package anomaly

import (
	"time"

	"github.com/moolen/spectre/internal/analysis"
)

// AnomalyCategory classifies the type of anomaly detection
type AnomalyCategory string

const (
	CategoryEvent     AnomalyCategory = "Event"
	CategoryState     AnomalyCategory = "State"
	CategoryChange    AnomalyCategory = "Change"
	CategoryFrequency AnomalyCategory = "Frequency"
	CategoryConfig    AnomalyCategory = "Config"
	CategoryNetwork   AnomalyCategory = "Network"
)

// Severity indicates the impact level of an anomaly
type Severity string

const (
	SeverityLow      Severity = "low"      // Informational or historical
	SeverityMedium   Severity = "medium"   // Potential contributor
	SeverityHigh     Severity = "high"     // Likely contributor
	SeverityCritical Severity = "critical" // Actively breaking workloads
)

// AnomalyNode identifies the resource exhibiting the anomaly
type AnomalyNode struct {
	UID       string `json:"uid"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// Anomaly represents a single detected anomaly
type Anomaly struct {
	Node      AnomalyNode            `json:"node"`
	Category  AnomalyCategory        `json:"category"`
	Type      string                 `json:"type"`      // Specific anomaly type
	Severity  Severity               `json:"severity"`
	Timestamp time.Time              `json:"timestamp"`
	Summary   string                 `json:"summary"`   // Human-readable description
	Details   map[string]interface{} `json:"details"`   // Anomaly-specific data
}

// AnomalyResponse is the API response structure
type AnomalyResponse struct {
	Anomalies []Anomaly        `json:"anomalies"`
	Metadata  ResponseMetadata `json:"metadata"`
}

// ResponseMetadata provides context about the analysis
type ResponseMetadata struct {
	ResourceUID     string     `json:"resource_uid"`
	TimeWindow      TimeWindow `json:"time_window"`
	NodesAnalyzed   int        `json:"nodes_analyzed"`
	ExecutionTimeMs int64      `json:"execution_time_ms"`
}

// TimeWindow represents the analysis time range
type TimeWindow struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// DetectorInput provides context for anomaly detectors
type DetectorInput struct {
	Node       *analysis.GraphNode
	TimeWindow TimeWindow
	AllEvents  []analysis.ChangeEventInfo
	K8sEvents  []analysis.K8sEventInfo
}

// Detector is the interface for all anomaly detectors
type Detector interface {
	Detect(input DetectorInput) []Anomaly
}

// nodeFromGraphNode converts a GraphNode to an AnomalyNode
func NodeFromGraphNode(node *analysis.GraphNode) AnomalyNode {
	return AnomalyNode{
		UID:       node.Resource.UID,
		Kind:      node.Resource.Kind,
		Namespace: node.Resource.Namespace,
		Name:      node.Resource.Name,
	}
}
