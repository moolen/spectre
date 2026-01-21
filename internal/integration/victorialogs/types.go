package victorialogs

import (
	"fmt"
	"time"
)

// QueryParams holds structured parameters for VictoriaLogs LogsQL queries.
// These parameters are converted to LogsQL syntax by the query builder.
type QueryParams struct {
	// K8s-focused filter fields
	Namespace string // Exact match for namespace field
	Pod       string // Exact match for pod field
	Container string // Exact match for container field
	Level     string // Exact match for level field (e.g., "error", "warn")

	// Time range for query (defaults to last 1 hour if zero)
	TimeRange TimeRange

	// Maximum number of log entries to return (max 1000)
	Limit int
}

// TimeRange represents a time window for log queries.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// IsZero returns true if the time range is not set (both Start and End are zero).
func (tr TimeRange) IsZero() bool {
	return tr.Start.IsZero() && tr.End.IsZero()
}

// ValidateMinimumDuration checks that the time range duration meets the minimum requirement.
// Returns an error if the duration is less than the specified minimum.
func (tr TimeRange) ValidateMinimumDuration(minDuration time.Duration) error {
	if tr.IsZero() {
		return nil // Zero time ranges use defaults, no validation needed
	}

	duration := tr.End.Sub(tr.Start)
	if duration < minDuration {
		return fmt.Errorf("time range duration %v is below minimum %v", duration, minDuration)
	}

	return nil
}

// Duration returns the duration of the time range (End - Start).
func (tr TimeRange) Duration() time.Duration {
	return tr.End.Sub(tr.Start)
}

// DefaultTimeRange returns a TimeRange for the last 1 hour.
func DefaultTimeRange() TimeRange {
	now := time.Now()
	return TimeRange{
		Start: now.Add(-1 * time.Hour),
		End:   now,
	}
}

// LogEntry represents a single log entry returned from VictoriaLogs.
// JSON tags match VictoriaLogs field names (underscore-prefixed for system fields).
type LogEntry struct {
	Message   string    `json:"_msg"`              // Log message content
	Stream    string    `json:"_stream"`           // Stream identifier
	Time      time.Time `json:"_time"`             // Log timestamp
	Namespace string    `json:"namespace,omitempty"` // Kubernetes namespace
	Pod       string    `json:"pod,omitempty"`     // Kubernetes pod name
	Container string    `json:"container,omitempty"` // Container name
	Level     string    `json:"level,omitempty"`   // Log level (error, warn, info, debug)
}

// QueryResponse holds the result of a log query.
type QueryResponse struct {
	Logs    []LogEntry // Log entries returned by the query
	Count   int        // Number of log entries in this response
	HasMore bool       // True if more results exist beyond the limit
}

// HistogramBucket represents a single time bucket in a histogram.
type HistogramBucket struct {
	Timestamp time.Time `json:"timestamp"` // Bucket timestamp
	Count     int       `json:"count"`     // Number of logs in this bucket
}

// HistogramResponse holds the result of a histogram query.
type HistogramResponse struct {
	Buckets []HistogramBucket `json:"buckets"` // Time-series histogram data
}

// AggregationGroup represents aggregated log counts by dimension.
type AggregationGroup struct {
	Dimension string `json:"dimension"` // Dimension name (e.g., "namespace", "level")
	Value     string `json:"value"`     // Dimension value (e.g., "prod", "error")
	Count     int    `json:"count"`     // Number of logs for this dimension value
}

// AggregationResponse holds the result of an aggregation query.
type AggregationResponse struct {
	Groups []AggregationGroup `json:"groups"` // Aggregated groups
}
