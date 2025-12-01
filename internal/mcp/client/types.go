package client

import "encoding/json"

// TimelineResponse represents the response from the timeline API
type TimelineResponse struct {
	Resources []TimelineResource `json:"resources"`
	Count     int                `json:"count"`
	ExecTimeMs int64            `json:"executionTimeMs"`
}

// TimelineResource represents a resource in the timeline response
type TimelineResource struct {
	ID              string            `json:"id"`
	Group           string            `json:"group"`
	Version         string            `json:"version"`
	Kind            string            `json:"kind"`
	Namespace       string            `json:"namespace"`
	Name            string            `json:"name"`
	StatusSegments  []StatusSegment   `json:"statusSegments"`
	Events          []K8sEvent        `json:"events"`
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
	ID              string `json:"id"`
	Timestamp       int64  `json:"timestamp"`
	Reason          string `json:"reason"`
	Message         string `json:"message"`
	Type            string `json:"type"` // Normal, Warning
	Count           int32  `json:"count"`
	Source          string `json:"source"`
	FirstTimestamp  int64  `json:"firstTimestamp"`
	LastTimestamp   int64  `json:"lastTimestamp"`
}

// MetadataResponse represents cluster metadata
type MetadataResponse struct {
	Namespaces []string `json:"namespaces"`
	Kinds      []string `json:"kinds"`
	Groups     []string `json:"groups"`
	TimeRange  TimeRange `json:"timeRange"`
}

// TimeRange represents the time range of available data
type TimeRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}
