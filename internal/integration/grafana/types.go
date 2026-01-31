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

	// SignalValidation configures the signal validation job that correlates
	// alert state transitions with signal behavior to build confidence scores.
	SignalValidation *SignalValidationConfig `json:"signalValidation,omitempty" yaml:"signalValidation,omitempty"`
}

// SignalValidationConfig configures the signal validation job.
type SignalValidationConfig struct {
	// Enabled controls whether the job runs.
	// Default: true when PrometheusURL is configured
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// RunInterval is how often to run the validation job.
	// Format: Go duration string (e.g., "24h", "12h")
	// Default: "24h"
	RunInterval string `json:"runInterval,omitempty" yaml:"runInterval,omitempty"`

	// LookbackPeriod is how far back to look for alert transitions.
	// Format: Go duration string (e.g., "30d", "7d")
	// Default: "30d" (limited by STATE_TRANSITION TTL of 7 days)
	LookbackPeriod string `json:"lookbackPeriod,omitempty" yaml:"lookbackPeriod,omitempty"`

	// WindowSize is the time window for metric comparison (before/after transition).
	// Format: Go duration string (e.g., "15m", "30m")
	// Default: "15m"
	WindowSize string `json:"windowSize,omitempty" yaml:"windowSize,omitempty"`

	// MinSampleCount is the minimum samples required in each window.
	// Default: 5
	MinSampleCount int `json:"minSampleCount,omitempty" yaml:"minSampleCount,omitempty"`

	// FlappingMaxTransitionsPerDay is the maximum transitions per day before alert is considered flapping.
	// Default: 50
	FlappingMaxTransitionsPerDay int `json:"flappingMaxTransitionsPerDay,omitempty" yaml:"flappingMaxTransitionsPerDay,omitempty"`

	// FlappingMaxDuration is the maximum continuous flapping duration before alert is excluded.
	// Format: Go duration string (e.g., "2h")
	// Default: "2h"
	FlappingMaxDuration string `json:"flappingMaxDuration,omitempty" yaml:"flappingMaxDuration,omitempty"`

	// DecayPeriod is how long before correlation observations fully decay.
	// Format: Go duration string (e.g., "90d")
	// Default: "90d"
	DecayPeriod string `json:"decayPeriod,omitempty" yaml:"decayPeriod,omitempty"`

	// PValueThreshold is the p-value threshold for t-test significance.
	// Default: 0.05
	PValueThreshold float64 `json:"pValueThreshold,omitempty" yaml:"pValueThreshold,omitempty"`

	// CohensDThreshold is the Cohen's d threshold for effect size significance.
	// Default: 0.8 (large effect)
	CohensDThreshold float64 `json:"cohensDThreshold,omitempty" yaml:"cohensDThreshold,omitempty"`

	// SigmaThreshold is the number of standard deviations for threshold-based detection.
	// Default: 2.0
	SigmaThreshold float64 `json:"sigmaThreshold,omitempty" yaml:"sigmaThreshold,omitempty"`

	// QueryRateLimit is the minimum interval between Prometheus queries.
	// Format: Go duration string (e.g., "100ms")
	// Default: "100ms"
	QueryRateLimit string `json:"queryRateLimit,omitempty" yaml:"queryRateLimit,omitempty"`
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

// IsSignalValidationEnabled returns whether signal validation is enabled.
// Returns true if PrometheusURL is set and not explicitly disabled.
func (c *Config) IsSignalValidationEnabled() bool {
	// Signal validation requires Prometheus URL
	if c.PrometheusURL == "" {
		return false
	}
	// If config exists and Enabled is explicitly set, use that value
	if c.SignalValidation != nil && c.SignalValidation.Enabled != nil {
		return *c.SignalValidation.Enabled
	}
	// Default: enabled when PrometheusURL is configured
	return true
}

// GetSignalValidationConfig returns the signal validation config with defaults applied.
func (c *Config) GetSignalValidationConfig() SignalValidationConfig {
	if c.SignalValidation == nil {
		return DefaultSignalValidationConfig()
	}
	return c.SignalValidation.WithDefaults()
}

// DefaultSignalValidationConfig returns default signal validation configuration.
func DefaultSignalValidationConfig() SignalValidationConfig {
	enabled := true
	return SignalValidationConfig{
		Enabled:                      &enabled,
		RunInterval:                  "24h",
		LookbackPeriod:               "7d", // Limited by STATE_TRANSITION TTL
		WindowSize:                   "15m",
		MinSampleCount:               5,
		FlappingMaxTransitionsPerDay: 50,
		FlappingMaxDuration:          "2h",
		DecayPeriod:                  "90d",
		PValueThreshold:              0.05,
		CohensDThreshold:             0.8,
		SigmaThreshold:               2.0,
		QueryRateLimit:               "100ms",
	}
}

// WithDefaults returns a copy of the config with defaults applied for unset values.
func (c *SignalValidationConfig) WithDefaults() SignalValidationConfig {
	defaults := DefaultSignalValidationConfig()
	result := *c

	if result.Enabled == nil {
		result.Enabled = defaults.Enabled
	}
	if result.RunInterval == "" {
		result.RunInterval = defaults.RunInterval
	}
	if result.LookbackPeriod == "" {
		result.LookbackPeriod = defaults.LookbackPeriod
	}
	if result.WindowSize == "" {
		result.WindowSize = defaults.WindowSize
	}
	if result.MinSampleCount == 0 {
		result.MinSampleCount = defaults.MinSampleCount
	}
	if result.FlappingMaxTransitionsPerDay == 0 {
		result.FlappingMaxTransitionsPerDay = defaults.FlappingMaxTransitionsPerDay
	}
	if result.FlappingMaxDuration == "" {
		result.FlappingMaxDuration = defaults.FlappingMaxDuration
	}
	if result.DecayPeriod == "" {
		result.DecayPeriod = defaults.DecayPeriod
	}
	if result.PValueThreshold == 0 {
		result.PValueThreshold = defaults.PValueThreshold
	}
	if result.CohensDThreshold == 0 {
		result.CohensDThreshold = defaults.CohensDThreshold
	}
	if result.SigmaThreshold == 0 {
		result.SigmaThreshold = defaults.SigmaThreshold
	}
	if result.QueryRateLimit == "" {
		result.QueryRateLimit = defaults.QueryRateLimit
	}

	return result
}

// GetRunInterval returns the run interval as a Duration.
func (c *SignalValidationConfig) GetRunInterval() time.Duration {
	if c.RunInterval == "" {
		return 24 * time.Hour
	}
	d, err := time.ParseDuration(c.RunInterval)
	if err != nil {
		return 24 * time.Hour
	}
	return d
}

// GetLookbackPeriod returns the lookback period as a Duration.
func (c *SignalValidationConfig) GetLookbackPeriod() time.Duration {
	if c.LookbackPeriod == "" {
		return 7 * 24 * time.Hour
	}
	d, err := time.ParseDuration(c.LookbackPeriod)
	if err != nil {
		return 7 * 24 * time.Hour
	}
	return d
}

// GetWindowSize returns the window size as a Duration.
func (c *SignalValidationConfig) GetWindowSize() time.Duration {
	if c.WindowSize == "" {
		return 15 * time.Minute
	}
	d, err := time.ParseDuration(c.WindowSize)
	if err != nil {
		return 15 * time.Minute
	}
	return d
}

// GetFlappingMaxDuration returns the flapping max duration as a Duration.
func (c *SignalValidationConfig) GetFlappingMaxDuration() time.Duration {
	if c.FlappingMaxDuration == "" {
		return 2 * time.Hour
	}
	d, err := time.ParseDuration(c.FlappingMaxDuration)
	if err != nil {
		return 2 * time.Hour
	}
	return d
}

// GetDecayPeriod returns the decay period as a Duration.
func (c *SignalValidationConfig) GetDecayPeriod() time.Duration {
	if c.DecayPeriod == "" {
		return 90 * 24 * time.Hour
	}
	d, err := time.ParseDuration(c.DecayPeriod)
	if err != nil {
		return 90 * 24 * time.Hour
	}
	return d
}

// GetQueryRateLimit returns the query rate limit as a Duration.
func (c *SignalValidationConfig) GetQueryRateLimit() time.Duration {
	if c.QueryRateLimit == "" {
		return 100 * time.Millisecond
	}
	d, err := time.ParseDuration(c.QueryRateLimit)
	if err != nil {
		return 100 * time.Millisecond
	}
	return d
}
