package grafana

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

// Config represents the Grafana integration configuration
type Config struct {
	// URL is the base URL for the Grafana instance (Cloud or self-hosted)
	// Examples: https://myorg.grafana.net or https://grafana.internal:3000
	URL string `json:"url" yaml:"url"`

	// APITokenRef references a Kubernetes Secret containing the API token
	APITokenRef *SecretRef `json:"apiTokenRef,omitempty" yaml:"apiTokenRef,omitempty"`

	// HierarchyMap maps Grafana tags to hierarchy levels (overview/drilldown/detail)
	// Used as fallback when dashboard lacks explicit hierarchy tags (spectre:* or hierarchy:*)
	// Example: {"prod": "overview", "staging": "drilldown"}
	// Optional: if not specified, dashboards default to "detail" when no hierarchy tags found
	HierarchyMap map[string]string `json:"hierarchyMap,omitempty" yaml:"hierarchyMap,omitempty"`

	// MetricsSyncEnabled enables automatic curated metric ingestion.
	// When enabled, metrics are fetched from Prometheus and matched against curated definitions.
	// Default: true
	MetricsSyncEnabled *bool `json:"metricsSyncEnabled,omitempty" yaml:"metricsSyncEnabled,omitempty"`

	// MetricsSyncInterval is how often to run metrics sync.
	// Format: Go duration string (e.g., "1h", "30m")
	// Default: "1h"
	MetricsSyncInterval string `json:"metricsSyncInterval,omitempty" yaml:"metricsSyncInterval,omitempty"`

	// MetricsDatasourceUID is the Prometheus datasource UID to query for metrics.
	// If empty, the default Prometheus datasource is used.
	// Default: "" (use default)
	MetricsDatasourceUID string `json:"metricsDatasourceUID,omitempty" yaml:"metricsDatasourceUID,omitempty"`

	// PrometheusURL is the direct Prometheus API URL for scrape target discovery.
	// This enables linking SignalAnchors to K8s workloads via scrape target metadata.
	// Example: http://prometheus:9090
	PrometheusURL string `json:"prometheusUrl,omitempty" yaml:"prometheusUrl,omitempty"`

	// PrometheusAPITokenRef references a Kubernetes Secret containing the Prometheus API token.
	// Optional: only needed if Prometheus requires authentication.
	PrometheusAPITokenRef *SecretRef `json:"prometheusApiTokenRef,omitempty" yaml:"prometheusApiTokenRef,omitempty"`

	// ScrapeTargetLinkingEnabled enables linking SignalAnchors to K8s workloads.
	// Default: true when PrometheusURL is set
	ScrapeTargetLinkingEnabled *bool `json:"scrapeTargetLinkingEnabled,omitempty" yaml:"scrapeTargetLinkingEnabled,omitempty"`

	// ScrapeTargetLinkingInterval is how often to refresh scrape target links.
	// Format: Go duration string (e.g., "5m", "10m")
	// Default: "5m"
	ScrapeTargetLinkingInterval string `json:"scrapeTargetLinkingInterval,omitempty" yaml:"scrapeTargetLinkingInterval,omitempty"`
}

// Validate checks config for common errors
func (c *Config) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("url is required")
	}

	// Normalize URL: remove trailing slash for consistency
	c.URL = strings.TrimSuffix(c.URL, "/")

	// Normalize PrometheusURL: remove trailing slash for consistency
	if c.PrometheusURL != "" {
		c.PrometheusURL = strings.TrimSuffix(c.PrometheusURL, "/")
	}

	// Validate SecretRef if present
	if c.APITokenRef != nil && c.APITokenRef.SecretName != "" {
		if c.APITokenRef.Key == "" {
			return fmt.Errorf("apiTokenRef.key is required when apiTokenRef is specified")
		}
	}

	// Validate PrometheusAPITokenRef if present
	if c.PrometheusAPITokenRef != nil && c.PrometheusAPITokenRef.SecretName != "" {
		if c.PrometheusAPITokenRef.Key == "" {
			return fmt.Errorf("prometheusApiTokenRef.key is required when prometheusApiTokenRef is specified")
		}
	}

	// Validate HierarchyMap if present
	if len(c.HierarchyMap) > 0 {
		validLevels := map[string]bool{
			"overview":  true,
			"drilldown": true,
			"detail":    true,
		}
		for tag, level := range c.HierarchyMap {
			if !validLevels[level] {
				return fmt.Errorf("hierarchyMap contains invalid level %q for tag %q, must be overview/drilldown/detail", level, tag)
			}
		}
	}

	return nil
}

// UsesSecretRef returns true if config uses Kubernetes Secret for authentication
func (c *Config) UsesSecretRef() bool {
	return c.APITokenRef != nil && c.APITokenRef.SecretName != ""
}

// IsMetricsSyncEnabled returns whether metrics sync is enabled.
// Defaults to true if not specified.
func (c *Config) IsMetricsSyncEnabled() bool {
	if c.MetricsSyncEnabled == nil {
		return true // Default: enabled
	}
	return *c.MetricsSyncEnabled
}

// GetMetricsSyncInterval returns the metrics sync interval.
// Defaults to 1 hour if not specified or invalid.
func (c *Config) GetMetricsSyncInterval() time.Duration {
	if c.MetricsSyncInterval == "" {
		return time.Hour
	}
	d, err := time.ParseDuration(c.MetricsSyncInterval)
	if err != nil {
		return time.Hour // Default on parse error
	}
	return d
}

// IsScrapeTargetLinkingEnabled returns whether scrape target linking is enabled.
// Returns true if PrometheusURL is set and not explicitly disabled.
func (c *Config) IsScrapeTargetLinkingEnabled() bool {
	// If explicitly set, use that value
	if c.ScrapeTargetLinkingEnabled != nil {
		return *c.ScrapeTargetLinkingEnabled
	}
	// Default: enabled when PrometheusURL is configured
	return c.PrometheusURL != ""
}

// GetScrapeTargetLinkingInterval returns the scrape target linking sync interval.
// Defaults to 5 minutes if not specified or invalid.
func (c *Config) GetScrapeTargetLinkingInterval() time.Duration {
	if c.ScrapeTargetLinkingInterval == "" {
		return 5 * time.Minute
	}
	d, err := time.ParseDuration(c.ScrapeTargetLinkingInterval)
	if err != nil {
		return 5 * time.Minute // Default on parse error
	}
	return d
}

// UsesPrometheusSecretRef returns true if config uses Kubernetes Secret for Prometheus authentication
func (c *Config) UsesPrometheusSecretRef() bool {
	return c.PrometheusAPITokenRef != nil && c.PrometheusAPITokenRef.SecretName != ""
}
