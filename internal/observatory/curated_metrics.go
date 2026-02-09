package observatory

import (
	"embed"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
)

//go:embed curated/*.json
var curatedFS embed.FS

// CuratedMetric represents a curated metric definition with classification metadata.
type CuratedMetric struct {
	// Name is the exact metric name (mutually exclusive with NamePattern)
	Name string `json:"name"`

	// NamePattern is a regex pattern for matching metric names (mutually exclusive with Name)
	NamePattern *string `json:"name_pattern"`

	// SignalRole is the classified signal role (availability, latency, errors, traffic, saturation, novelty, churn)
	SignalRole string `json:"signal_role"`

	// Confidence is the classification confidence (0.0-1.0)
	Confidence float64 `json:"confidence"`

	// Importance is the relative importance of this metric (0.0-1.0)
	Importance float64 `json:"importance"`

	// Source is the metric source (e.g., "kubernetes/kube-state-metrics", "prometheus/scrape")
	Source string `json:"source"`

	// MetricType is the Prometheus metric type (counter, gauge, histogram, summary, info)
	MetricType string `json:"metric_type"`

	// LabelsOfInterest are the labels commonly used with this metric
	LabelsOfInterest []string `json:"labels_of_interest"`

	// CommonPromQLPatterns are example PromQL queries using this metric
	CommonPromQLPatterns []string `json:"common_promql_patterns"`

	// Notes provides context and usage guidance for this metric
	Notes string `json:"notes"`

	// Deprecated indicates if this metric is deprecated
	Deprecated bool `json:"deprecated"`

	// DisabledByDefault indicates if this metric is disabled by default in its exporter
	DisabledByDefault bool `json:"disabled_by_default"`

	// compiledPattern caches the compiled regex for pattern-based metrics
	compiledPattern *regexp.Regexp
}

// CuratedBatch represents a batch of curated metrics from a JSON file.
type CuratedBatch struct {
	// Batch is the batch identifier (can be string or int in JSON, stored as any)
	Batch any `json:"batch"`

	// Name is the human-readable batch name
	Name string `json:"name"`

	// Description describes the batch contents
	Description string `json:"description"`

	// ResearchedAt is the timestamp when this batch was researched
	ResearchedAt string `json:"researched_at"`

	// SourcesConsulted lists the documentation sources used
	SourcesConsulted []string `json:"sources_consulted"`

	// Sources is an alternative field name for sources
	Sources []string `json:"sources"`

	// Metrics is the list of curated metrics in this batch
	Metrics []CuratedMetric `json:"metrics"`
}

// curatedMetricsRegistry holds all loaded curated metrics.
type curatedMetricsRegistry struct {
	// exactMatch maps exact metric names to their curated definitions
	exactMatch map[string]*CuratedMetric

	// patternMatch holds metrics with regex patterns
	patternMatch []*CuratedMetric

	// allMetrics holds all loaded metrics for iteration
	allMetrics []*CuratedMetric
}

var (
	registry     *curatedMetricsRegistry
	registryOnce sync.Once
	registryErr  error
)

// loadCuratedMetrics loads and parses all curated metric JSON files.
func loadCuratedMetrics() (*curatedMetricsRegistry, error) {
	reg := &curatedMetricsRegistry{
		exactMatch:   make(map[string]*CuratedMetric),
		patternMatch: make([]*CuratedMetric, 0),
		allMetrics:   make([]*CuratedMetric, 0),
	}

	entries, err := curatedFS.ReadDir("curated")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := curatedFS.ReadFile("curated/" + entry.Name())
		if err != nil {
			return nil, err
		}

		var batch CuratedBatch
		if err := json.Unmarshal(data, &batch); err != nil {
			// Skip files that don't match the expected format (e.g., methodology-only files)
			continue
		}

		for i := range batch.Metrics {
			metric := &batch.Metrics[i]

			// Determine the metric name - use name_pattern as literal name if name is empty
			effectiveName := metric.Name
			if effectiveName == "" && metric.NamePattern != nil && *metric.NamePattern != "" {
				effectiveName = *metric.NamePattern
				metric.Name = effectiveName // Normalize: copy to Name field
			}

			// Skip metrics with no usable name
			if effectiveName == "" {
				continue
			}

			// Check if name_pattern looks like a regex (contains special chars)
			// Most patterns in the curated data are literal names, not regexes
			isRegexPattern := metric.NamePattern != nil && *metric.NamePattern != "" &&
				(strings.ContainsAny(*metric.NamePattern, ".*+?^${}[]|()\\"))

			if isRegexPattern {
				compiled, err := regexp.Compile(*metric.NamePattern)
				if err == nil {
					metric.compiledPattern = compiled
					reg.patternMatch = append(reg.patternMatch, metric)
				}
				// Also add to exact match if the pattern equals the name
				if metric.Name != "" && metric.Name != *metric.NamePattern {
					reg.exactMatch[metric.Name] = metric
				}
			} else {
				reg.exactMatch[effectiveName] = metric
			}

			reg.allMetrics = append(reg.allMetrics, metric)
		}
	}

	return reg, nil
}

// getCuratedRegistry returns the singleton curated metrics registry.
func getCuratedRegistry() (*curatedMetricsRegistry, error) {
	registryOnce.Do(func() {
		registry, registryErr = loadCuratedMetrics()
	})
	return registry, registryErr
}

// LookupCuratedMetric looks up a metric name in the curated metrics registry.
// It first tries exact match, then falls back to pattern matching.
// Returns nil if no match is found.
func LookupCuratedMetric(metricName string) *CuratedMetric {
	reg, err := getCuratedRegistry()
	if err != nil || reg == nil {
		return nil
	}

	// Try exact match first
	if metric, ok := reg.exactMatch[metricName]; ok {
		return metric
	}

	// Try pattern match
	for _, metric := range reg.patternMatch {
		if metric.compiledPattern != nil && metric.compiledPattern.MatchString(metricName) {
			return metric
		}
	}

	return nil
}

// GetAllCuratedMetrics returns all loaded curated metrics.
func GetAllCuratedMetrics() []*CuratedMetric {
	reg, err := getCuratedRegistry()
	if err != nil || reg == nil {
		return nil
	}
	return reg.allMetrics
}

// GetCuratedMetricCount returns the total number of curated metrics loaded.
func GetCuratedMetricCount() int {
	reg, err := getCuratedRegistry()
	if err != nil || reg == nil {
		return 0
	}
	return len(reg.allMetrics)
}

// signalRoleFromString converts a JSON signal_role string to a SignalRole constant.
func signalRoleFromString(role string) SignalRole {
	switch strings.ToLower(role) {
	case "availability":
		return SignalAvailability
	case "latency":
		return SignalLatency
	case "errors":
		return SignalErrors
	case "traffic":
		return SignalTraffic
	case "saturation":
		return SignalSaturation
	case "novelty", "churn":
		// Both "novelty" and "churn" map to SignalNovelty
		return SignalNovelty
	default:
		return SignalUnknown
	}
}

// ToSignalRole converts the metric's signal_role string to a SignalRole constant.
func (m *CuratedMetric) ToSignalRole() SignalRole {
	return signalRoleFromString(m.SignalRole)
}
