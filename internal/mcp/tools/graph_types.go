package tools

// Common types shared across graph MCP tools

// GraphResourceInfo contains resource identification for graph tools
type GraphResourceInfo struct {
	UID       string `json:"uid"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// GraphChangeEventInfo contains change event details
type GraphChangeEventInfo struct {
	ID           string `json:"id"`
	Timestamp    int64  `json:"timestamp"`
	EventType    string `json:"eventType"`
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// GraphEvidenceItem represents a piece of evidence for causality
type GraphEvidenceItem struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Confidence  float64                `json:"confidence,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// GraphRelationshipInfo describes how resources are related
type GraphRelationshipInfo struct {
	Type     string `json:"type"`
	Distance int    `json:"distance"` // Hops in graph
	Path     string `json:"path,omitempty"`
}

// GraphImpactEvent describes an event caused by a change
type GraphImpactEvent struct {
	Timestamp      int64  `json:"timestamp"`
	Status         string `json:"status"`
	ErrorMessage   string `json:"errorMessage,omitempty"`
	LagFromTrigger int64  `json:"lagFromTrigger"` // Milliseconds
}

// normalizeTimestamp converts Unix seconds to nanoseconds if needed
func normalizeTimestamp(ts int64) int64 {
	// If timestamp is less than year 2100 in seconds, it's in seconds
	if ts < 4_102_444_800 {
		return ts * 1_000_000_000
	}
	return ts
}
