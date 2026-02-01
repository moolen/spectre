package observatorygraph

// AnalyzeInput contains parameters for observatory graph analysis
type AnalyzeInput struct {
	// Integration name to filter (optional, if empty returns all integrations)
	Integration string
	// Namespace to filter SignalAnchors by workload namespace (optional)
	Namespace string
	// WorkloadName to filter SignalAnchors by workload name (optional)
	WorkloadName string
	// IncludeBaselines includes SignalBaseline nodes when true
	IncludeBaselines bool
	// Limit maximum number of SignalAnchor nodes to return (default 100)
	Limit int
}

// ObservatoryGraphResponse contains the graph data for observatory visualization
type ObservatoryGraphResponse struct {
	Graph    Graph         `json:"graph"`
	Metadata GraphMetadata `json:"metadata"`
}

// Graph contains nodes and edges
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Node represents a node in the observatory graph
type Node struct {
	ID         string            `json:"id"`
	Type       NodeType          `json:"type"`
	Label      string            `json:"label"`
	Properties map[string]any    `json:"properties,omitempty"`
}

// NodeType represents the type of observatory graph node
type NodeType string

const (
	NodeTypeSignalAnchor   NodeType = "SignalAnchor"
	NodeTypeSignalBaseline NodeType = "SignalBaseline"
	NodeTypeAlert          NodeType = "Alert"
	NodeTypeDashboard      NodeType = "Dashboard"
	NodeTypePanel          NodeType = "Panel"
	NodeTypeQuery          NodeType = "Query"
	NodeTypeMetric         NodeType = "Metric"
	NodeTypeService        NodeType = "Service"
	NodeTypeWorkload       NodeType = "Workload"
)

// Edge represents an edge in the observatory graph
type Edge struct {
	ID               string   `json:"id"`
	Source           string   `json:"source"`
	Target           string   `json:"target"`
	RelationshipType EdgeType `json:"relationshipType"`
	Properties       map[string]any `json:"properties,omitempty"`
}

// EdgeType represents the type of observatory graph edge
type EdgeType string

const (
	EdgeTypeMonitorsWorkload EdgeType = "MONITORS_WORKLOAD"
	EdgeTypeCorrelatesWith   EdgeType = "CORRELATES_WITH"
	EdgeTypeExtractedFrom    EdgeType = "EXTRACTED_FROM"
	EdgeTypeHasBaseline      EdgeType = "HAS_BASELINE"
	EdgeTypeContains         EdgeType = "CONTAINS"
	EdgeTypeHas              EdgeType = "HAS"
	EdgeTypeUses             EdgeType = "USES"
	EdgeTypeTracks           EdgeType = "TRACKS"
	EdgeTypeMonitors         EdgeType = "MONITORS"
)

// GraphMetadata contains metadata about the graph response
type GraphMetadata struct {
	NodeCount        int   `json:"nodeCount"`
	EdgeCount        int   `json:"edgeCount"`
	QueryExecutionMs int64 `json:"queryExecutionMs"`
}
