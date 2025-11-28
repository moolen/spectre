package models

import "encoding/json"

// SearchResponse represents the response from /v1/search endpoint
type SearchResponse struct {
	Resources       []Resource `json:"resources"`
	Count           int        `json:"count"`
	ExecutionTimeMs int64      `json:"executionTimeMs"`
}

// Resource represents a Kubernetes resource with minimal data for list view
type Resource struct {
	ID             string          `json:"id"`
	Group          string          `json:"group"`
	Version        string          `json:"version"`
	Kind           string          `json:"kind"`
	Namespace      string          `json:"namespace"`
	Name           string          `json:"name"`
	StatusSegments []StatusSegment `json:"statusSegments,omitempty"`
	Events         []K8sEvent      `json:"events,omitempty"`
}

// StatusSegment represents a period during which a resource maintained a specific status
type StatusSegment struct {
	StartTime    int64           `json:"startTime"`
	EndTime      int64           `json:"endTime"`
	Status       string          `json:"status"` // Ready, Warning, Error, Terminating, Unknown
	Message      string          `json:"message,omitempty"`
	ResourceData json.RawMessage `json:"resourceData,omitempty"`
}

// K8sEvent represents a Kubernetes Event (Kind=Event) associated with a resource
type K8sEvent struct {
	ID             string `json:"id"`
	Timestamp      int64  `json:"timestamp"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	Type           string `json:"type"` // Normal, Warning
	Count          int32  `json:"count"`
	Source         string `json:"source,omitempty"`
	FirstTimestamp int64  `json:"firstTimestamp,omitempty"`
	LastTimestamp  int64  `json:"lastTimestamp,omitempty"`
}

// MetadataResponse represents the response from /v1/metadata endpoint
type MetadataResponse struct {
	Namespaces     []string       `json:"namespaces"`
	Kinds          []string       `json:"kinds"`
	Groups         []string       `json:"groups"`
	ResourceCounts map[string]int `json:"resourceCounts"`
	TotalEvents    int            `json:"totalEvents"`
	TimeRange      TimeRangeInfo  `json:"timeRange"`
}

// TimeRangeInfo contains earliest and latest event timestamps
type TimeRangeInfo struct {
	Earliest int64 `json:"earliest"`
	Latest   int64 `json:"latest"`
}

// EventsResponse represents the response from /v1/resources/{id}/events
type EventsResponse struct {
	Events     []K8sEvent `json:"events"`
	Count      int        `json:"count"`
	ResourceID string     `json:"resourceId"`
}

// SegmentsResponse represents the response from /v1/resources/{id}/segments
type SegmentsResponse struct {
	Segments   []StatusSegment `json:"segments"`
	ResourceID string          `json:"resourceId"`
	Count      int             `json:"count"`
}

// Validate validates SearchResponse
func (sr *SearchResponse) Validate() error {
	if sr.Count < 0 {
		return NewValidationError("count must be non-negative")
	}
	if sr.ExecutionTimeMs < 0 {
		return NewValidationError("executionTimeMs must be non-negative")
	}
	if sr.Resources == nil {
		return NewValidationError("resources must not be nil")
	}
	return nil
}

// Validate validates Resource
func (r *Resource) Validate() error {
	if r.ID == "" {
		return NewValidationError("resource id must not be empty")
	}
	if r.Kind == "" {
		return NewValidationError("resource kind must not be empty")
	}
	if r.Namespace == "" {
		return NewValidationError("resource namespace must not be empty")
	}
	if r.Name == "" {
		return NewValidationError("resource name must not be empty")
	}
	return nil
}

// Validate validates StatusSegment
func (ss *StatusSegment) Validate() error {
	if ss.StartTime >= ss.EndTime {
		return NewValidationError("startTime must be less than endTime")
	}
	validStatuses := map[string]bool{
		"Ready": true, "Warning": true, "Error": true, "Terminating": true, "Unknown": true,
	}
	if !validStatuses[ss.Status] {
		return NewValidationError("status must be one of: Ready, Warning, Error, Terminating, Unknown")
	}
	return nil
}
