package grafana

import (
	"testing"
)

func TestClassifyMetric_Layer1_HardcodedMetrics(t *testing.T) {
	tests := []struct {
		name           string
		metricName     string
		expectedRole   SignalRole
		expectedLayer  int
		expectedConf   float64
	}{
		{
			name:          "up metric → Availability",
			metricName:    "up",
			expectedRole:  SignalAvailability,
			expectedLayer: 1,
			expectedConf:  0.95,
		},
		{
			name:          "kube_pod_status_phase → Availability",
			metricName:    "kube_pod_status_phase",
			expectedRole:  SignalAvailability,
			expectedLayer: 1,
			expectedConf:  0.95,
		},
		{
			name:          "container_cpu_usage_seconds_total → Saturation",
			metricName:    "container_cpu_usage_seconds_total",
			expectedRole:  SignalSaturation,
			expectedLayer: 1,
			expectedConf:  0.95,
		},
		{
			name:          "node_memory_MemAvailable_bytes → Saturation",
			metricName:    "node_memory_MemAvailable_bytes",
			expectedRole:  SignalSaturation,
			expectedLayer: 1,
			expectedConf:  0.95,
		},
		{
			name:          "http_requests_total → Traffic",
			metricName:    "http_requests_total",
			expectedRole:  SignalTraffic,
			expectedLayer: 1,
			expectedConf:  0.95,
		},
		{
			name:          "kube_pod_container_status_restarts_total → Novelty",
			metricName:    "kube_pod_container_status_restarts_total",
			expectedRole:  SignalNovelty,
			expectedLayer: 1,
			expectedConf:  0.95,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyMetric(tt.metricName, nil, "")

			if result.Role != tt.expectedRole {
				t.Errorf("expected role %s, got %s", tt.expectedRole, result.Role)
			}
			if result.Layer != tt.expectedLayer {
				t.Errorf("expected layer %d, got %d", tt.expectedLayer, result.Layer)
			}
			if result.Confidence != tt.expectedConf {
				t.Errorf("expected confidence %.2f, got %.2f", tt.expectedConf, result.Confidence)
			}
			if result.Reason == "" {
				t.Error("expected non-empty reason")
			}
		})
	}
}

func TestClassifyMetric_Layer2_PromQLStructure(t *testing.T) {
	tests := []struct {
		name          string
		metricName    string
		extraction    *QueryExtraction
		expectedRole  SignalRole
		expectedLayer int
		minConf       float64
		maxConf       float64
	}{
		{
			name:       "histogram_quantile → Latency",
			metricName: "http_request_duration_seconds_bucket",
			extraction: &QueryExtraction{
				MetricNames:  []string{"http_request_duration_seconds_bucket"},
				Aggregations: []string{"histogram_quantile"},
			},
			expectedRole:  SignalLatency,
			expectedLayer: 2,
			minConf:       0.9,
			maxConf:       0.9,
		},
		{
			name:       "rate(errors_total) → Errors",
			metricName: "api_errors_total",
			extraction: &QueryExtraction{
				MetricNames:  []string{"api_errors_total"},
				Aggregations: []string{"rate"},
			},
			expectedRole:  SignalErrors,
			expectedLayer: 2,
			minConf:       0.85,
			maxConf:       0.85,
		},
		{
			name:       "increase(failed_total) → Errors",
			metricName: "job_failed_total",
			extraction: &QueryExtraction{
				MetricNames:  []string{"job_failed_total"},
				Aggregations: []string{"increase"},
			},
			expectedRole:  SignalErrors,
			expectedLayer: 2,
			minConf:       0.85,
			maxConf:       0.85,
		},
		{
			name:       "rate(requests_total) → Traffic",
			metricName: "api_requests_total",
			extraction: &QueryExtraction{
				MetricNames:  []string{"api_requests_total"},
				Aggregations: []string{"rate"},
			},
			expectedRole:  SignalTraffic,
			expectedLayer: 2,
			minConf:       0.85,
			maxConf:       0.85,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyMetric(tt.metricName, tt.extraction, "")

			if result.Role != tt.expectedRole {
				t.Errorf("expected role %s, got %s", tt.expectedRole, result.Role)
			}
			if result.Layer != tt.expectedLayer {
				t.Errorf("expected layer %d, got %d", tt.expectedLayer, result.Layer)
			}
			if result.Confidence < tt.minConf || result.Confidence > tt.maxConf {
				t.Errorf("expected confidence between %.2f and %.2f, got %.2f", tt.minConf, tt.maxConf, result.Confidence)
			}
		})
	}
}

func TestClassifyMetric_Layer3_MetricNamePatterns(t *testing.T) {
	tests := []struct {
		name          string
		metricName    string
		expectedRole  SignalRole
		expectedLayer int
		minConf       float64
		maxConf       float64
	}{
		{
			name:          "http_request_duration_seconds → Latency",
			metricName:    "http_request_duration_seconds",
			expectedRole:  SignalLatency,
			expectedLayer: 3,
			minConf:       0.7,
			maxConf:       0.8,
		},
		{
			name:          "api_latency_milliseconds → Latency",
			metricName:    "api_latency_milliseconds",
			expectedRole:  SignalLatency,
			expectedLayer: 3,
			minConf:       0.7,
			maxConf:       0.8,
		},
		{
			name:          "grpc_error_count → Errors",
			metricName:    "grpc_error_count",
			expectedRole:  SignalErrors,
			expectedLayer: 3,
			minConf:       0.7,
			maxConf:       0.8,
		},
		{
			name:          "job_failed_runs → Errors",
			metricName:    "job_failed_runs",
			expectedRole:  SignalErrors,
			expectedLayer: 3,
			minConf:       0.7,
			maxConf:       0.8,
		},
		{
			name:          "api_calls_total → Traffic",
			metricName:    "api_calls_total",
			expectedRole:  SignalTraffic,
			expectedLayer: 3,
			minConf:       0.7,
			maxConf:       0.8,
		},
		{
			name:          "memory_usage_bytes → Saturation",
			metricName:    "memory_usage_bytes",
			expectedRole:  SignalSaturation,
			expectedLayer: 3,
			minConf:       0.7,
			maxConf:       0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyMetric(tt.metricName, nil, "")

			if result.Role != tt.expectedRole {
				t.Errorf("expected role %s, got %s", tt.expectedRole, result.Role)
			}
			if result.Layer != tt.expectedLayer {
				t.Errorf("expected layer %d, got %d", tt.expectedLayer, result.Layer)
			}
			if result.Confidence < tt.minConf || result.Confidence > tt.maxConf {
				t.Errorf("expected confidence between %.2f and %.2f, got %.2f", tt.minConf, tt.maxConf, result.Confidence)
			}
		})
	}
}

func TestClassifyMetric_Layer4_PanelTitle(t *testing.T) {
	tests := []struct {
		name          string
		metricName    string
		panelTitle    string
		expectedRole  SignalRole
		expectedLayer int
		expectedConf  float64
	}{
		{
			name:          "Error Rate panel → Errors",
			metricName:    "my_custom_metric",
			panelTitle:    "Error Rate",
			expectedRole:  SignalErrors,
			expectedLayer: 4,
			expectedConf:  0.5,
		},
		{
			name:          "Latency P95 panel → Latency",
			metricName:    "my_custom_metric",
			panelTitle:    "API Latency P95",
			expectedRole:  SignalLatency,
			expectedLayer: 4,
			expectedConf:  0.5,
		},
		{
			name:          "QPS panel → Traffic",
			metricName:    "my_custom_metric",
			panelTitle:    "Requests QPS",
			expectedRole:  SignalTraffic,
			expectedLayer: 4,
			expectedConf:  0.5,
		},
		{
			name:          "CPU Usage panel → Saturation",
			metricName:    "my_custom_metric",
			panelTitle:    "CPU Usage",
			expectedRole:  SignalSaturation,
			expectedLayer: 4,
			expectedConf:  0.5,
		},
		{
			name:          "Health Status panel → Availability",
			metricName:    "my_custom_metric",
			panelTitle:    "Service Health Status",
			expectedRole:  SignalAvailability,
			expectedLayer: 4,
			expectedConf:  0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyMetric(tt.metricName, nil, tt.panelTitle)

			if result.Role != tt.expectedRole {
				t.Errorf("expected role %s, got %s", tt.expectedRole, result.Role)
			}
			if result.Layer != tt.expectedLayer {
				t.Errorf("expected layer %d, got %d", tt.expectedLayer, result.Layer)
			}
			if result.Confidence != tt.expectedConf {
				t.Errorf("expected confidence %.2f, got %.2f", tt.expectedConf, result.Confidence)
			}
		})
	}
}

func TestClassifyMetric_Layer5_Unknown(t *testing.T) {
	tests := []struct {
		name         string
		metricName   string
		expectedRole SignalRole
		expectedConf float64
	}{
		{
			name:         "completely unknown metric → Unknown",
			metricName:   "my_business_metric_xyz",
			expectedRole: SignalUnknown,
			expectedConf: 0.0,
		},
		{
			name:         "another unknown metric → Unknown",
			metricName:   "foo_bar_baz",
			expectedRole: SignalUnknown,
			expectedConf: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyMetric(tt.metricName, nil, "")

			if result.Role != tt.expectedRole {
				t.Errorf("expected role %s, got %s", tt.expectedRole, result.Role)
			}
			if result.Layer != 5 {
				t.Errorf("expected layer 5, got %d", result.Layer)
			}
			if result.Confidence != tt.expectedConf {
				t.Errorf("expected confidence %.2f, got %.2f", tt.expectedConf, result.Confidence)
			}
			if result.Reason == "" {
				t.Error("expected non-empty reason")
			}
		})
	}
}

func TestClassifyMetric_LayerPriority(t *testing.T) {
	// Test that Layer 1 (hardcoded) takes precedence over Layer 3 (metric name)
	t.Run("Layer 1 takes precedence over Layer 3", func(t *testing.T) {
		// "up" is hardcoded as Availability (Layer 1, 0.95)
		// If Layer 3 tried to classify it, it might be different
		result := ClassifyMetric("up", nil, "")

		if result.Layer != 1 {
			t.Errorf("expected Layer 1 to take precedence, got Layer %d", result.Layer)
		}
		if result.Confidence != 0.95 {
			t.Errorf("expected Layer 1 confidence 0.95, got %.2f", result.Confidence)
		}
	})

	// Test that Layer 2 (PromQL structure) takes precedence over Layer 3 (metric name)
	t.Run("Layer 2 takes precedence over Layer 3", func(t *testing.T) {
		// Metric name has "_total" (Layer 3 would classify as Traffic)
		// But histogram_quantile (Layer 2) should take precedence → Latency
		result := ClassifyMetric("http_request_duration_seconds_bucket", &QueryExtraction{
			MetricNames:  []string{"http_request_duration_seconds_bucket"},
			Aggregations: []string{"histogram_quantile"},
		}, "")

		if result.Layer != 2 {
			t.Errorf("expected Layer 2 to take precedence, got Layer %d", result.Layer)
		}
		if result.Role != SignalLatency {
			t.Errorf("expected Latency, got %s", result.Role)
		}
	})

	// Test that Layer 3 (metric name) takes precedence over Layer 4 (panel title)
	t.Run("Layer 3 takes precedence over Layer 4", func(t *testing.T) {
		// Metric name has "_duration" (Layer 3 → Latency)
		// Panel title says "Error Rate" (Layer 4 → Errors)
		// Layer 3 should win
		result := ClassifyMetric("api_duration_seconds", nil, "Error Rate")

		if result.Layer != 3 {
			t.Errorf("expected Layer 3 to take precedence, got Layer %d", result.Layer)
		}
		if result.Role != SignalLatency {
			t.Errorf("expected Latency, got %s", result.Role)
		}
	})
}

func TestClassifyMetric_AvoidFalsePositives(t *testing.T) {
	// Test that error metrics with "_total" don't get classified as Traffic
	t.Run("error_total should be Errors, not Traffic", func(t *testing.T) {
		result := ClassifyMetric("http_request_errors_total", nil, "")

		// Should match Layer 3 error pattern, not traffic pattern
		if result.Role != SignalErrors {
			t.Errorf("expected Errors, got %s (reason: %s)", result.Role, result.Reason)
		}
	})
}
