package victorialogs

import (
	"fmt"
	"strings"
	"time"
)

// SecretRef references a Kubernetes Secret for sensitive values
type SecretRef struct {
	// SecretName is the name of the Kubernetes Secret in the same namespace as Spectre
	SecretName string `json:"secretName" yaml:"secretName"`

	// Key is the key within the Secret's Data map
	Key string `json:"key" yaml:"key"`
}

// Config represents the VictoriaLogs integration configuration
type Config struct {
	// URL is the base URL for the VictoriaLogs instance
	URL string `json:"url" yaml:"url"`

	// APITokenRef references a Kubernetes Secret containing the API token
	// Mutually exclusive with embedding token in URL
	APITokenRef *SecretRef `json:"apiTokenRef,omitempty" yaml:"apiTokenRef,omitempty"`
}

// Validate checks config for common errors
func (c *Config) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("url is required")
	}

	// Check for mutually exclusive auth methods
	urlHasToken := strings.Contains(c.URL, "@") // Basic auth pattern
	hasSecretRef := c.APITokenRef != nil && c.APITokenRef.SecretName != ""

	if urlHasToken && hasSecretRef {
		return fmt.Errorf("cannot specify both URL-embedded credentials and apiTokenRef")
	}

	// Validate SecretRef if present
	if hasSecretRef {
		if c.APITokenRef.Key == "" {
			return fmt.Errorf("apiTokenRef.key is required when apiTokenRef is specified")
		}
	}

	return nil
}

// UsesSecretRef returns true if config uses Kubernetes Secret for authentication
func (c *Config) UsesSecretRef() bool {
	return c.APITokenRef != nil && c.APITokenRef.SecretName != ""
}

// QueryParams holds structured parameters for VictoriaLogs LogsQL queries.
// These parameters are converted to LogsQL syntax by the query builder.
type QueryParams struct {
	// K8s-focused filter fields
	Namespace string // Exact match for namespace field
	Pod       string // Exact match for pod field
	Container string // Exact match for container field
	Level     string // Exact match for level field (e.g., "error", "warn")

	// TextMatch is a word/phrase to search for in the log message (_msg field)
	// This is used for text-based severity detection when logs don't have structured level fields
	TextMatch string

	// RegexMatch is a regex pattern to match against the log message (_msg field)
	// This is used for complex severity classification patterns
	// Takes precedence over TextMatch if both are set
	RegexMatch string

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
	Message   string    `json:"_msg"`                           // Log message content
	Stream    string    `json:"_stream"`                        // Stream identifier
	Time      time.Time `json:"_time"`                          // Log timestamp
	Namespace string    `json:"kubernetes.pod_namespace,omitempty"` // Kubernetes namespace
	Pod       string    `json:"kubernetes.pod_name,omitempty"`      // Kubernetes pod name
	Container string    `json:"kubernetes.container_name,omitempty"` // Container name
	NodeName  string    `json:"kubernetes.pod_node_name,omitempty"` // Node name where the pod is running
	Level     string    `json:"level,omitempty"`                // Log level (error, warn, info, debug)
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

// statsQueryResponse matches the VictoriaLogs /select/logsql/stats_query JSON response format.
// VictoriaLogs returns a Prometheus-compatible response structure.
type statsQueryResponse struct {
	Status string `json:"status"` // "success" or "error"
	Data   struct {
		ResultType string `json:"resultType"` // "vector" or "matrix"
		Result     []struct {
			Metric map[string]string `json:"metric"` // Labels including the grouped field
			Value  [2]interface{}    `json:"value"`  // [timestamp, count_string]
		} `json:"result"`
	} `json:"data"`
}
