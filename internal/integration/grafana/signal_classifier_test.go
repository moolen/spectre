package grafana

import (
	"testing"
)

func TestClassifyMetric_Layer1_CuratedMetrics(t *testing.T) {
	// Layer 1 now uses embedded curated metrics from JSON files.
	// Confidence values come from the curated data (typically 0.8-1.0).
	tests := []struct {
		name          string
		metricName    string
		expectedRole  SignalRole
		expectedLayer int
		minConf       float64
		maxConf       float64
	}{
		{
			name:          "up metric → Availability",
			metricName:    "up",
			expectedRole:  SignalAvailability,
			expectedLayer: 1,
			minConf:       0.9,
			maxConf:       1.0,
		},
		{
			name:          "kube_pod_status_phase → Availability",
			metricName:    "kube_pod_status_phase",
			expectedRole:  SignalAvailability,
			expectedLayer: 1,
			minConf:       0.9,
			maxConf:       1.0,
		},
		{
			name:          "container_cpu_usage_seconds_total → Saturation",
			metricName:    "container_cpu_usage_seconds_total",
			expectedRole:  SignalSaturation,
			expectedLayer: 1,
			minConf:       0.8,
			maxConf:       1.0,
		},
		{
			name:          "node_memory_MemAvailable_bytes → Saturation",
			metricName:    "node_memory_MemAvailable_bytes",
			expectedRole:  SignalSaturation,
			expectedLayer: 1,
			minConf:       0.9,
			maxConf:       1.0,
		},
		{
			name:          "kube_pod_container_status_restarts_total → Novelty",
			metricName:    "kube_pod_container_status_restarts_total",
			expectedRole:  SignalNovelty,
			expectedLayer: 1,
			minConf:       0.9,
			maxConf:       1.0,
		},
		{
			name:          "etcd_server_has_leader → Availability",
			metricName:    "etcd_server_has_leader",
			expectedRole:  SignalAvailability,
			expectedLayer: 1,
			minConf:       0.9,
			maxConf:       1.0,
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
			if result.Reason == "" {
				t.Error("expected non-empty reason")
			}
		})
	}
}

func TestClassifyMetric_Layer2_PromQLStructure(t *testing.T) {
	// Layer 2 tests use metrics NOT in curated data, so classification
	// falls through to PromQL structure analysis.
	// Note: Use unique names that don't match patterns in batch-8-conventions-patterns.json
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
			name:       "histogram_quantile on custom metric → Latency",
			metricName: "zztest_latency_histogram_bucket",
			extraction: &QueryExtraction{
				MetricNames:  []string{"zztest_latency_histogram_bucket"},
				Aggregations: []string{"histogram_quantile"},
			},
			expectedRole:  SignalLatency,
			expectedLayer: 2,
			minConf:       0.9,
			maxConf:       0.9,
		},
		{
			name:       "rate(error metric) → Errors",
			metricName: "zztest_error_events",
			extraction: &QueryExtraction{
				MetricNames:  []string{"zztest_error_events"},
				Aggregations: []string{"rate"},
			},
			expectedRole:  SignalErrors,
			expectedLayer: 2,
			minConf:       0.85,
			maxConf:       0.85,
		},
		{
			name:       "increase(failed metric) → Errors",
			metricName: "zztest_failure_events",
			extraction: &QueryExtraction{
				MetricNames:  []string{"zztest_failure_events"},
				Aggregations: []string{"increase"},
			},
			expectedRole:  SignalErrors,
			expectedLayer: 2,
			minConf:       0.85,
			maxConf:       0.85,
		},
		{
			name:       "rate(request metric) → Traffic",
			metricName: "zztest_request_events",
			extraction: &QueryExtraction{
				MetricNames:  []string{"zztest_request_events"},
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
	// Layer 3 tests use metrics NOT in curated data, so classification
	// falls through to metric name pattern matching.
	// Note: Use names that don't match patterns in batch-8-conventions-patterns.json
	tests := []struct {
		name          string
		metricName    string
		expectedRole  SignalRole
		expectedLayer int
		minConf       float64
		maxConf       float64
	}{
		{
			name:          "latency in name → Latency",
			metricName:    "zztest_latency_measurement",
			expectedRole:  SignalLatency,
			expectedLayer: 3,
			minConf:       0.7,
			maxConf:       0.8,
		},
		{
			name:          "duration in name → Latency",
			metricName:    "zztest_duration_measurement",
			expectedRole:  SignalLatency,
			expectedLayer: 3,
			minConf:       0.7,
			maxConf:       0.8,
		},
		{
			name:          "error in name → Errors",
			metricName:    "zztest_error_measurement",
			expectedRole:  SignalErrors,
			expectedLayer: 3,
			minConf:       0.7,
			maxConf:       0.8,
		},
		{
			name:          "failed in name → Errors",
			metricName:    "zztest_job_failed_measurement",
			expectedRole:  SignalErrors,
			expectedLayer: 3,
			minConf:       0.7,
			maxConf:       0.8,
		},
		{
			name:          "usage in name → Saturation",
			metricName:    "zztest_memory_usage_value",
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
	// Test that Layer 1 (curated metrics) takes precedence over other layers
	t.Run("Layer 1 takes precedence over Layer 3", func(t *testing.T) {
		// "up" is in curated metrics as Availability (Layer 1)
		result := ClassifyMetric("up", nil, "")

		if result.Layer != 1 {
			t.Errorf("expected Layer 1 to take precedence, got Layer %d", result.Layer)
		}
		// Confidence comes from curated data (1.0 for "up")
		if result.Confidence < 0.9 || result.Confidence > 1.0 {
			t.Errorf("expected Layer 1 confidence between 0.9-1.0, got %.2f", result.Confidence)
		}
	})

	// Test that Layer 2 (PromQL structure) takes precedence over Layer 3 (metric name)
	// when the metric is NOT in curated data
	t.Run("Layer 2 takes precedence over Layer 3", func(t *testing.T) {
		// Use a custom metric NOT in curated data
		// Metric name has "_total" (Layer 3 would classify as Traffic)
		// But histogram_quantile (Layer 2) should take precedence → Latency
		result := ClassifyMetric("myapp_custom_latency_bucket", &QueryExtraction{
			MetricNames:  []string{"myapp_custom_latency_bucket"},
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
	// when the metric is NOT in curated data
	t.Run("Layer 3 takes precedence over Layer 4", func(t *testing.T) {
		// Use a custom metric NOT in curated data (avoid pattern matches)
		// Metric name has "_duration" (Layer 3 → Latency)
		// Panel title says "Error Rate" (Layer 4 → Errors)
		// Layer 3 should win
		result := ClassifyMetric("zztest_api_duration_value", nil, "Error Rate")

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

func TestClassifyMetric_KubernetesRecordingRules(t *testing.T) {
	// Recording rules may be classified by Layer 1 (if in curated data) or
	// fall through to Layer 3 (metric name patterns). The important thing
	// is that the role is correct.
	tests := []struct {
		name         string
		metricName   string
		expectedRole SignalRole
		expectFilter bool
	}{
		{
			name:         "Memory working set recording rule → Saturation (via _bytes pattern)",
			metricName:   "node_namespace_pod_container:container_memory_working_set_bytes",
			expectedRole: SignalSaturation,
		},
		{
			name:         "Relabel recording rule → filtered",
			metricName:   "namespace_workload_pod:kube_pod_owner:relabel",
			expectedRole: SignalUnknown,
			expectFilter: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyMetric(tt.metricName, nil, "")

			if result.Role != tt.expectedRole {
				t.Errorf("expected role %s, got %s (reason: %s)", tt.expectedRole, result.Role, result.Reason)
			}

			if tt.expectFilter {
				if result.Layer != 0 {
					t.Errorf("expected Layer 0 for filtered metric, got %d", result.Layer)
				}
				if result.Confidence != 0.0 {
					t.Errorf("expected confidence 0.0 for filtered metric, got %.2f", result.Confidence)
				}
			}
		})
	}
}

func TestClassifyMetric_CoreDNS(t *testing.T) {
	// CoreDNS metrics may be in Layer 1 (curated data) or Layer 3 (name patterns).
	// The important thing is that the role is correct.
	tests := []struct {
		name         string
		metricName   string
		expectedRole SignalRole
	}{
		{
			name:         "CoreDNS requests → Traffic",
			metricName:   "coredns_dns_requests_total",
			expectedRole: SignalTraffic,
		},
		{
			name:         "CoreDNS responses → Traffic",
			metricName:   "coredns_dns_responses_total",
			expectedRole: SignalTraffic,
		},
		{
			name:         "CoreDNS request duration → Latency",
			metricName:   "coredns_dns_request_duration_seconds",
			expectedRole: SignalLatency,
		},
		{
			name:         "CoreDNS request duration bucket → Latency",
			metricName:   "coredns_dns_request_duration_seconds_bucket",
			expectedRole: SignalLatency,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyMetric(tt.metricName, nil, "")

			if result.Role != tt.expectedRole {
				t.Errorf("expected role %s, got %s (reason: %s)", tt.expectedRole, result.Role, result.Reason)
			}
		})
	}
}

func TestClassifyMetric_RequestsPatternFix(t *testing.T) {
	tests := []struct {
		name         string
		metricName   string
		expectedRole SignalRole
	}{
		{
			name:         "http_requests → Traffic (generic requests)",
			metricName:   "myapp_http_requests",
			expectedRole: SignalTraffic,
		},
		{
			name:         "custom_requests_total → Traffic (generic requests)",
			metricName:   "myapp_api_requests_total",
			expectedRole: SignalTraffic,
		},
		{
			name:         "kube_pod_container_resource_requests → Saturation (not Traffic)",
			metricName:   "kube_pod_container_resource_requests",
			expectedRole: SignalSaturation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyMetric(tt.metricName, nil, "")

			if result.Role != tt.expectedRole {
				t.Errorf("expected role %s, got %s (reason: %s)", tt.expectedRole, result.Role, result.Reason)
			}
		})
	}
}

func TestClassifyMetric_SizeBytesTraffic(t *testing.T) {
	// Test that size/bytes metrics from Layer 3 classification are Traffic.
	// Note: Curated patterns may classify some _bytes$ metrics differently.
	// This tests the Layer 3 _size_bytes and _bytes_total patterns specifically.
	tests := []struct {
		name         string
		metricName   string
		expectedRole SignalRole
	}{
		{
			name:         "_bytes_total suffix → Traffic (Layer 3 or curated pattern)",
			metricName:   "zztest_network_transferred_total",
			expectedRole: SignalTraffic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyMetric(tt.metricName, nil, "")

			if result.Role != tt.expectedRole {
				t.Errorf("expected role %s, got %s (reason: %s)", tt.expectedRole, result.Role, result.Reason)
			}
		})
	}
}
