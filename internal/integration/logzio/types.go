package logzio

import (
	"fmt"
	"time"
)

// SecretRef references a Kubernetes Secret for sensitive values
type SecretRef struct {
	// SecretName is the name of the Kubernetes Secret in the same namespace as Spectre
	SecretName string `json:"secretName" yaml:"secretName"`

	// Key is the key within the Secret's Data map
	Key string `json:"key" yaml:"key"`
}

// Config represents the Logz.io integration configuration
type Config struct {
	// Region determines the Logz.io API endpoint
	// Valid values: us, eu, uk, au, ca
	Region string `json:"region" yaml:"region"`

	// APITokenRef references a Kubernetes Secret containing the API token
	APITokenRef *SecretRef `json:"apiTokenRef,omitempty" yaml:"apiTokenRef,omitempty"`
}

// Validate checks config for common errors
func (c *Config) Validate() error {
	if c.Region == "" {
		return fmt.Errorf("region is required")
	}

	// Validate region value
	validRegions := map[string]bool{
		"us": true,
		"eu": true,
		"uk": true,
		"au": true,
		"ca": true,
	}
	if !validRegions[c.Region] {
		return fmt.Errorf("invalid region %q, must be one of: us, eu, uk, au, ca", c.Region)
	}

	// Validate SecretRef if present
	if c.APITokenRef != nil {
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

// GetBaseURL returns the Logz.io API endpoint for the configured region
func (c *Config) GetBaseURL() string {
	regionURLs := map[string]string{
		"us": "https://api.logz.io",
		"eu": "https://api-eu.logz.io",
		"uk": "https://api-uk.logz.io",
		"au": "https://api-au.logz.io",
		"ca": "https://api-ca.logz.io",
	}
	return regionURLs[c.Region]
}

// QueryParams holds structured parameters for Logz.io Elasticsearch queries.
type QueryParams struct {
	// K8s-focused filter fields
	Namespace string // Exact match for namespace field
	Pod       string // Exact match for pod field
	Container string // Exact match for container field
	Level     string // Exact match for level field (e.g., "error", "warn")

	// RegexMatch is a regex pattern to match against the log message (message field)
	// This is used for complex severity classification patterns
	RegexMatch string

	// Time range for query (defaults to last 1 hour if zero)
	TimeRange TimeRange

	// Maximum number of log entries to return (max 500)
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

// LogEntry represents a single log entry returned from Logz.io.
// Normalized to match common schema across backends.
type LogEntry struct {
	Message   string    `json:"message"`   // Log message content
	Time      time.Time `json:"@timestamp"` // Log timestamp
	Namespace string    `json:"namespace,omitempty"` // Kubernetes namespace
	Pod       string    `json:"pod,omitempty"`       // Kubernetes pod name
	Container string    `json:"container,omitempty"` // Container name
	Level     string    `json:"level,omitempty"`     // Log level (error, warn, info, debug)
}

// QueryResponse holds the result of a log query.
type QueryResponse struct {
	Logs []LogEntry // Log entries returned by the query
}

// AggregationGroup represents aggregated log counts by dimension.
type AggregationGroup struct {
	Value string `json:"value"` // Dimension value (e.g., "prod", "error")
	Count int    `json:"count"` // Number of logs for this dimension value
}

// AggregationResponse holds the result of an aggregation query.
type AggregationResponse struct {
	Groups []AggregationGroup `json:"groups"` // Aggregated groups
}
