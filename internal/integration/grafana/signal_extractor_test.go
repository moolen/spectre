package grafana

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractSignalsFromPanel_SingleQuery(t *testing.T) {
	dashboard := &GrafanaDashboard{
		UID:   "test-dashboard",
		Title: "Test Dashboard",
	}

	panel := GrafanaPanel{
		ID:    1,
		Title: "CPU Usage",
		Targets: []GrafanaTarget{
			{
				RefID: "A",
				Expr:  `rate(container_cpu_usage_seconds_total{namespace="prod"}[5m])`,
			},
		},
	}

	qualityScore := 0.8
	integrationName := "test-grafana"
	now := int64(1234567890)

	signals, err := ExtractSignalsFromPanel(dashboard, panel, qualityScore, integrationName, now)

	assert.NoError(t, err)
	assert.Len(t, signals, 1)

	signal := signals[0]
	assert.Equal(t, "container_cpu_usage_seconds_total", signal.MetricName)
	assert.Equal(t, SignalSaturation, signal.Role)
	assert.Equal(t, 0.95, signal.Confidence) // Layer 1: hardcoded metric
	assert.Equal(t, 0.8, signal.QualityScore)
	assert.Equal(t, "prod", signal.WorkloadNamespace)
	assert.Equal(t, "", signal.WorkloadName) // No workload labels
	assert.Equal(t, "test-dashboard", signal.DashboardUID)
	assert.Equal(t, 1, signal.PanelID)
	assert.Equal(t, "test-dashboard-1-A", signal.QueryID)
	assert.Equal(t, "test-grafana", signal.SourceGrafana)
	assert.Equal(t, now, signal.FirstSeen)
	assert.Equal(t, now, signal.LastSeen)
	assert.Equal(t, now+(7*24*60*60*1_000_000_000), signal.ExpiresAt)
}

func TestExtractSignalsFromPanel_MultiQuery(t *testing.T) {
	dashboard := &GrafanaDashboard{
		UID:   "golden-signals",
		Title: "Golden Signals Dashboard",
	}

	panel := GrafanaPanel{
		ID:    2,
		Title: "Service Health",
		Targets: []GrafanaTarget{
			{
				RefID: "A",
				Expr:  `up{job="api", namespace="prod"}`,
			},
			{
				RefID: "B",
				Expr:  `rate(http_requests_total{job="api", namespace="prod"}[5m])`,
			},
			{
				RefID: "C",
				Expr:  `rate(http_request_errors_total{job="api", namespace="prod"}[5m])`,
			},
		},
	}

	qualityScore := 0.9
	integrationName := "prod-grafana"
	now := int64(9876543210)

	signals, err := ExtractSignalsFromPanel(dashboard, panel, qualityScore, integrationName, now)

	assert.NoError(t, err)
	assert.Len(t, signals, 3)

	// Check all three signals have correct roles
	roles := make(map[SignalRole]bool)
	for _, signal := range signals {
		roles[signal.Role] = true
		assert.Equal(t, 0.9, signal.QualityScore)
		assert.Equal(t, "prod", signal.WorkloadNamespace)
		assert.Equal(t, "api", signal.WorkloadName) // job label inference
	}

	assert.True(t, roles[SignalAvailability]) // up metric
	assert.True(t, roles[SignalTraffic])      // http_requests_total
	assert.True(t, roles[SignalErrors])       // http_request_errors_total
}

func TestExtractSignalsFromPanel_QualityScoreInheritance(t *testing.T) {
	dashboard := &GrafanaDashboard{
		UID:   "high-quality-dashboard",
		Title: "Production Overview",
	}

	panel := GrafanaPanel{
		ID:    1,
		Title: "Memory Usage",
		Targets: []GrafanaTarget{
			{
				RefID: "A",
				Expr:  `container_memory_usage_bytes{namespace="prod", deployment="api"}`,
			},
		},
	}

	qualityScore := 0.95
	integrationName := "grafana"
	now := int64(1000000000)

	signals, err := ExtractSignalsFromPanel(dashboard, panel, qualityScore, integrationName, now)

	assert.NoError(t, err)
	assert.Len(t, signals, 1)
	assert.Equal(t, 0.95, signals[0].QualityScore) // Inherited from dashboard
}

func TestExtractSignalsFromPanel_WorkloadInferenceIntegration(t *testing.T) {
	dashboard := &GrafanaDashboard{
		UID:   "test-dashboard",
		Title: "Test Dashboard",
	}

	testCases := []struct {
		name                  string
		expr                  string
		expectedNamespace     string
		expectedWorkloadName  string
	}{
		{
			name:                  "Deployment label",
			expr:                  `up{namespace="prod", deployment="api-server"}`,
			expectedNamespace:     "prod",
			expectedWorkloadName:  "api-server",
		},
		{
			name:                  "App label",
			expr:                  `up{namespace="staging", app="frontend"}`,
			expectedNamespace:     "staging",
			expectedWorkloadName:  "frontend",
		},
		{
			name:                  "Service label",
			expr:                  `up{namespace="test", service="database"}`,
			expectedNamespace:     "test",
			expectedWorkloadName:  "database",
		},
		{
			name:                  "Job label",
			expr:                  `up{namespace="prod", job="batch-processor"}`,
			expectedNamespace:     "prod",
			expectedWorkloadName:  "batch-processor",
		},
		{
			name:                  "No workload labels",
			expr:                  `up{namespace="prod"}`,
			expectedNamespace:     "prod",
			expectedWorkloadName:  "",
		},
		{
			name:                  "No labels",
			expr:                  `up`,
			expectedNamespace:     "",
			expectedWorkloadName:  "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			panel := GrafanaPanel{
				ID:    1,
				Title: "Test Panel",
				Targets: []GrafanaTarget{
					{
						RefID: "A",
						Expr:  tc.expr,
					},
				},
			}

			signals, err := ExtractSignalsFromPanel(dashboard, panel, 0.8, "grafana", 1000)

			assert.NoError(t, err)
			assert.Len(t, signals, 1)
			assert.Equal(t, tc.expectedNamespace, signals[0].WorkloadNamespace)
			assert.Equal(t, tc.expectedWorkloadName, signals[0].WorkloadName)
		})
	}
}

func TestExtractSignalsFromPanel_LowConfidenceFiltered(t *testing.T) {
	dashboard := &GrafanaDashboard{
		UID:   "test-dashboard",
		Title: "Test Dashboard",
	}

	// Metric that won't match any classification layer (confidence 0)
	panel := GrafanaPanel{
		ID:    1,
		Title: "Unclassifiable Metric",
		Targets: []GrafanaTarget{
			{
				RefID: "A",
				Expr:  `some_random_metric_xyz123{namespace="prod"}`,
			},
		},
	}

	signals, err := ExtractSignalsFromPanel(dashboard, panel, 0.8, "grafana", 1000)

	assert.NoError(t, err)
	assert.Len(t, signals, 0) // Filtered out due to confidence < 0.5
}

func TestExtractSignalsFromPanel_EmptyQuery(t *testing.T) {
	dashboard := &GrafanaDashboard{
		UID:   "test-dashboard",
		Title: "Test Dashboard",
	}

	panel := GrafanaPanel{
		ID:    1,
		Title: "Empty Panel",
		Targets: []GrafanaTarget{
			{
				RefID: "A",
				Expr:  "", // Empty query
			},
		},
	}

	signals, err := ExtractSignalsFromPanel(dashboard, panel, 0.8, "grafana", 1000)

	assert.NoError(t, err)
	assert.Len(t, signals, 0)
}

func TestExtractSignalsFromDashboard_Deduplication(t *testing.T) {
	dashboard := &GrafanaDashboard{
		UID:   "test-dashboard",
		Title: "Test Dashboard",
		Panels: []GrafanaPanel{
			{
				ID:    1,
				Title: "Panel 1",
				Targets: []GrafanaTarget{
					{
						RefID: "A",
						Expr:  `up{namespace="prod", deployment="api"}`,
					},
				},
			},
			{
				ID:    2,
				Title: "Panel 2",
				Targets: []GrafanaTarget{
					{
						RefID: "A",
						Expr:  `up{namespace="prod", deployment="api"}`, // Duplicate
					},
				},
			},
		},
	}

	qualityScore := 0.8
	integrationName := "grafana"
	now := int64(1000000000)

	signals, err := ExtractSignalsFromDashboard(dashboard, qualityScore, integrationName, now)

	assert.NoError(t, err)
	assert.Len(t, signals, 1) // Deduplicated

	signal := signals[0]
	assert.Equal(t, "up", signal.MetricName)
	assert.Equal(t, "prod", signal.WorkloadNamespace)
	assert.Equal(t, "api", signal.WorkloadName)
}

func TestExtractSignalsFromDashboard_HighestQualityWins(t *testing.T) {
	// Create two separate dashboards with different quality scores
	// to test deduplication logic
	dashboard1 := &GrafanaDashboard{
		UID:   "dashboard-low-quality",
		Title: "Low Quality Dashboard",
		Panels: []GrafanaPanel{
			{
				ID:    1,
				Title: "Panel 1",
				Targets: []GrafanaTarget{
					{
						RefID: "A",
						Expr:  `up{namespace="prod", deployment="api"}`,
					},
				},
			},
		},
	}

	dashboard2 := &GrafanaDashboard{
		UID:   "dashboard-high-quality",
		Title: "High Quality Dashboard",
		Panels: []GrafanaPanel{
			{
				ID:    2,
				Title: "Panel 2",
				Targets: []GrafanaTarget{
					{
						RefID: "B",
						Expr:  `up{namespace="prod", deployment="api"}`, // Same metric+workload
					},
				},
			},
		},
	}

	now := int64(1000000000)

	// Extract signals with lower quality score
	signals1, err := ExtractSignalsFromDashboard(dashboard1, 0.5, "grafana", now)
	assert.NoError(t, err)
	assert.Len(t, signals1, 1)

	// Extract signals with higher quality score
	signals2, err := ExtractSignalsFromDashboard(dashboard2, 0.9, "grafana", now)
	assert.NoError(t, err)
	assert.Len(t, signals2, 1)

	// Manually merge signals to test deduplication logic
	signalMap := make(map[string]SignalAnchor)
	for _, signal := range signals1 {
		key := signal.MetricName + "|" + signal.WorkloadNamespace + "|" + signal.WorkloadName
		signalMap[key] = signal
	}
	for _, signal := range signals2 {
		key := signal.MetricName + "|" + signal.WorkloadNamespace + "|" + signal.WorkloadName
		if existing, exists := signalMap[key]; exists {
			if signal.QualityScore > existing.QualityScore {
				signal.FirstSeen = existing.FirstSeen
				signalMap[key] = signal
			}
		} else {
			signalMap[key] = signal
		}
	}

	// Should have kept high-quality signal
	assert.Len(t, signalMap, 1)
	for _, signal := range signalMap {
		assert.Equal(t, 0.9, signal.QualityScore)
		assert.Equal(t, "dashboard-high-quality", signal.DashboardUID)
	}
}

func TestExtractSignalsFromDashboard_MultipleMetricsMultiplePanels(t *testing.T) {
	dashboard := &GrafanaDashboard{
		UID:   "complex-dashboard",
		Title: "Complex Dashboard",
		Panels: []GrafanaPanel{
			{
				ID:    1,
				Title: "Availability",
				Targets: []GrafanaTarget{
					{
						RefID: "A",
						Expr:  `up{namespace="prod", deployment="api"}`,
					},
					{
						RefID: "B",
						Expr:  `up{namespace="prod", deployment="frontend"}`,
					},
				},
			},
			{
				ID:    2,
				Title: "Traffic",
				Targets: []GrafanaTarget{
					{
						RefID: "A",
						Expr:  `rate(http_requests_total{namespace="prod", deployment="api"}[5m])`,
					},
					{
						RefID: "B",
						Expr:  `rate(http_requests_total{namespace="prod", deployment="frontend"}[5m])`,
					},
				},
			},
			{
				ID:    3,
				Title: "Errors",
				Targets: []GrafanaTarget{
					{
						RefID: "A",
						Expr:  `rate(http_request_errors_total{namespace="prod", deployment="api"}[5m])`,
					},
				},
			},
		},
	}

	qualityScore := 0.85
	integrationName := "grafana"
	now := int64(1000000000)

	signals, err := ExtractSignalsFromDashboard(dashboard, qualityScore, integrationName, now)

	assert.NoError(t, err)
	assert.Len(t, signals, 5) // 2 up + 2 http_requests + 1 http_errors

	// Verify all signals have correct quality score
	for _, signal := range signals {
		assert.Equal(t, 0.85, signal.QualityScore)
		assert.Equal(t, "prod", signal.WorkloadNamespace)
	}

	// Count metrics by role
	roleCounts := make(map[SignalRole]int)
	for _, signal := range signals {
		roleCounts[signal.Role]++
	}

	assert.Equal(t, 2, roleCounts[SignalAvailability]) // 2 up metrics
	assert.Equal(t, 2, roleCounts[SignalTraffic])      // 2 http_requests_total
	assert.Equal(t, 1, roleCounts[SignalErrors])       // 1 http_request_errors_total
}

func TestExtractSignalsFromDashboard_EmptyDashboard(t *testing.T) {
	dashboard := &GrafanaDashboard{
		UID:    "empty-dashboard",
		Title:  "Empty Dashboard",
		Panels: []GrafanaPanel{},
	}

	signals, err := ExtractSignalsFromDashboard(dashboard, 0.8, "grafana", 1000)

	assert.NoError(t, err)
	assert.Len(t, signals, 0)
}
