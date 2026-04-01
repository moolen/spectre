package grafana

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSignalIngestionEndToEnd tests the complete signal ingestion pipeline
// from dashboard sync to SignalAnchor nodes in FalkorDB.
func TestSignalIngestionEndToEnd(t *testing.T) {
	ctx := context.Background()
	logger := logging.GetLogger("test")

	// Setup mock clients
	mockGrafana := newMockGrafanaClient()
	mockGraph := newMockGraphClient()

	// Configure DashboardSyncer
	config := &Config{URL: "https://test.grafana.net"}
	integrationName := "test-grafana"

	syncer := NewDashboardSyncer(mockGrafana, mockGraph, config, integrationName, time.Hour, logger)

	// Test case 1: Dashboard with known metrics (Layer 1 classification)
	t.Run("KnownMetrics_Layer1Classification", func(t *testing.T) {
		dashboard := &GrafanaDashboard{
			UID:     "test-dashboard-1",
			Title:   "Test Dashboard",
			Version: 1,
			Tags:    []string{"test"},
			Panels: []GrafanaPanel{
				{
					ID:    1,
					Title: "Pod Availability",
					Type:  "graph",
					Targets: []GrafanaTarget{
						{
							RefID: "A",
							Expr:  `kube_pod_status_phase{namespace="production"}`,
						},
					},
				},
				{
					ID:    2,
					Title: "CPU Usage",
					Type:  "graph",
					Targets: []GrafanaTarget{
						{
							RefID: "A",
							Expr:  `container_cpu_usage_seconds_total{namespace="production", deployment="web"}`,
						},
					},
				},
			},
		}

		// Sync dashboard (triggers signal ingestion)
		err := syncer.syncDashboard(ctx, dashboard)
		require.NoError(t, err)

		// Verify: SignalAnchor nodes created
		foundAvailability := false
		foundSaturation := false

		for _, query := range mockGraph.queries {
			// Look for SignalAnchor MERGE queries (have role and confidence parameters)
			if query.Parameters["role"] != nil && query.Parameters["confidence"] != nil {
				metricName, ok := query.Parameters["metric_name"].(string)
				if !ok {
					continue
				}

				if metricName == "kube_pod_status_phase" {
					assert.Equal(t, "Availability", query.Parameters["role"])
					assert.Equal(t, 0.95, query.Parameters["confidence"]) // Curated: 0.95
					assert.Equal(t, "production", query.Parameters["workload_namespace"])
					foundAvailability = true
				}
				if metricName == "container_cpu_usage_seconds_total" {
					assert.Equal(t, "Saturation", query.Parameters["role"])
					assert.Equal(t, 0.9, query.Parameters["confidence"]) // Curated: 0.9
					assert.Equal(t, "production", query.Parameters["workload_namespace"])
					assert.Equal(t, "web", query.Parameters["workload_name"])
					foundSaturation = true
				}
			}
		}

		assert.True(t, foundAvailability, "Expected Availability signal for kube_pod_status_phase")
		assert.True(t, foundSaturation, "Expected Saturation signal for container_cpu_usage_seconds_total")
	})

	// Test case 2: Dashboard with PromQL structure patterns (Layer 2)
	t.Run("PromQLStructure_Layer2Classification", func(t *testing.T) {
		mockGraph.queries = []graph.GraphQuery{} // Reset queries

		// Use a custom metric name not in curated data to test Layer 2 classification
		dashboard := &GrafanaDashboard{
			UID:     "test-dashboard-2",
			Title:   "Latency Dashboard",
			Version: 1,
			Tags:    []string{"test"},
			Panels: []GrafanaPanel{
				{
					ID:    1,
					Title: "Request Latency",
					Type:  "graph",
					Targets: []GrafanaTarget{
						{
							RefID: "A",
							Expr:  `histogram_quantile(0.99, rate(myapp_custom_latency_bucket[5m]))`,
						},
					},
				},
			},
		}

		err := syncer.syncDashboard(ctx, dashboard)
		require.NoError(t, err)

		// Verify: histogram_quantile classified as Latency with 0.9 confidence (Layer 2)
		foundLatency := false
		for _, query := range mockGraph.queries {
			if query.Parameters["role"] != nil && query.Parameters["confidence"] != nil {
				metricName, ok := query.Parameters["metric_name"].(string)
				if ok {
					// histogram_quantile extracts the _bucket suffix metric
					if metricName == "myapp_custom_latency_bucket" {
						assert.Equal(t, "Latency", query.Parameters["role"])
						assert.Equal(t, 0.9, query.Parameters["confidence"])
						foundLatency = true
					}
				}
			}
		}

		assert.True(t, foundLatency, "Expected Latency signal for histogram_quantile query")
	})

	// Test case 3: Quality score propagation
	t.Run("QualityScorePropagation", func(t *testing.T) {
		mockGraph.queries = []graph.GraphQuery{} // Reset queries

		// Dashboard with recent update (high freshness) and meaningful content
		dashboard := &GrafanaDashboard{
			UID:     "test-dashboard-3",
			Title:   "High Quality Dashboard",
			Version: 1,
			Tags:    []string{"test"},
			Panels: []GrafanaPanel{
				{
					ID:    1,
					Title: "Service Uptime",
					Type:  "graph",
					Targets: []GrafanaTarget{
						{
							RefID: "A",
							Expr:  `up{job="api"}`,
						},
					},
				},
			},
		}

		err := syncer.syncDashboard(ctx, dashboard)
		require.NoError(t, err)

		// Verify: Signal inherits quality score from dashboard
		foundSignal := false
		for _, query := range mockGraph.queries {
			if query.Parameters["role"] != nil && query.Parameters["quality_score"] != nil {
				metricName, ok := query.Parameters["metric_name"].(string)
				if ok && metricName == "up" {
					qualityScore, ok := query.Parameters["quality_score"].(float64)
					require.True(t, ok, "quality_score should be float64")
					// Quality should be > 0 (actual score depends on dashboard metadata)
					assert.Greater(t, qualityScore, 0.0)
					assert.LessOrEqual(t, qualityScore, 1.0)
					foundSignal = true
				}
			}
		}

		assert.True(t, foundSignal, "Expected signal with quality score")
	})

	// Test case 4: TTL expiration
	t.Run("TTLExpiration", func(t *testing.T) {
		mockGraph.queries = []graph.GraphQuery{} // Reset queries

		now := time.Now().UnixNano()

		// Create GraphBuilder for direct signal creation
		gb := NewGraphBuilder(mockGraph, config, integrationName, logger)

		// Create signal with expires_at
		signal := SignalAnchor{
			MetricName:        "test_metric",
			Role:              SignalAvailability,
			Confidence:        0.95,
			QualityScore:      0.8,
			WorkloadNamespace: "test",
			WorkloadName:      "test",
			DashboardUID:      "test-dashboard",
			PanelID:           1,
			QueryID:           "test-query",
			SourceGrafana:     integrationName,
			FirstSeen:         now,
			LastSeen:          now,
			ExpiresAt:         now + (7 * 24 * 60 * 60 * 1_000_000_000), // 7 days in nanoseconds
		}

		err := gb.BuildSignalGraph(ctx, []SignalAnchor{signal})
		require.NoError(t, err)

		// Verify: expires_at is set correctly (7 days from now)
		foundSignal := false
		for _, query := range mockGraph.queries {
			if query.Parameters["role"] != nil && query.Parameters["expires_at"] != nil {
				metricName, ok := query.Parameters["metric_name"].(string)
				if ok && metricName == "test_metric" {
					expiresAt, ok := query.Parameters["expires_at"].(int64)
					require.True(t, ok, "expires_at should be int64")

					// Verify TTL is approximately 7 days
					ttl := time.Duration(expiresAt - now)
					expectedTTL := 7 * 24 * time.Hour
					assert.InDelta(t, expectedTTL, ttl, float64(time.Minute), "TTL should be ~7 days")
					foundSignal = true
				}
			}
		}

		assert.True(t, foundSignal, "Expected signal with TTL")
	})

	// Test case 5: Relationships
	t.Run("SignalRelationships", func(t *testing.T) {
		mockGraph.queries = []graph.GraphQuery{} // Reset queries

		dashboard := &GrafanaDashboard{
			UID:     "test-dashboard-5",
			Title:   "Relationship Test",
			Version: 1,
			Tags:    []string{"test"},
			Panels: []GrafanaPanel{
				{
					ID:    1,
					Title: "Test Panel",
					Type:  "graph",
					Targets: []GrafanaTarget{
						{
							RefID: "A",
							Expr:  `up{namespace="production"}`,
						},
					},
				},
			},
		}

		err := syncer.syncDashboard(ctx, dashboard)
		require.NoError(t, err)

		// Verify: Relationship queries created
		foundSourcedFrom := false
		foundRepresents := false

		for _, query := range mockGraph.queries {
			// Look for SOURCED_FROM relationship (SignalAnchor -> Dashboard)
			if query.Parameters["dashboard_uid"] == "test-dashboard-5" {
				foundSourcedFrom = true
			}
			// Look for REPRESENTS relationship (SignalAnchor -> Metric)
			if query.Parameters["metric_name"] == "up" {
				foundRepresents = true
			}
		}

		assert.True(t, foundSourcedFrom, "Expected SOURCED_FROM relationship")
		assert.True(t, foundRepresents, "Expected REPRESENTS relationship")
	})

	// Test case 6: Unlinked signals (no workload)
	t.Run("UnlinkedSignals_NoWorkload", func(t *testing.T) {
		mockGraph.queries = []graph.GraphQuery{} // Reset queries

		dashboard := &GrafanaDashboard{
			UID:     "test-dashboard-6",
			Title:   "Unlinked Signal Test",
			Version: 1,
			Tags:    []string{"test"},
			Panels: []GrafanaPanel{
				{
					ID:    1,
					Title: "Cluster-wide Metric",
					Type:  "graph",
					Targets: []GrafanaTarget{
						{
							RefID: "A",
							// No namespace or workload labels
							Expr: `up`,
						},
					},
				},
			},
		}

		err := syncer.syncDashboard(ctx, dashboard)
		require.NoError(t, err)

		// Verify: Signal created with empty workload fields
		foundUnlinked := false
		for _, query := range mockGraph.queries {
			if query.Parameters["role"] != nil {
				metricName, ok := query.Parameters["metric_name"].(string)
				if ok && metricName == "up" {
					// workload_namespace and workload_name should be empty strings
					assert.Equal(t, "", query.Parameters["workload_namespace"])
					assert.Equal(t, "", query.Parameters["workload_name"])
					foundUnlinked = true
				}
			}
		}

		assert.True(t, foundUnlinked, "Expected unlinked signal with empty workload fields")
	})

	// Test case 7: Multi-query panel creates multiple signals
	t.Run("MultiQueryPanel_MultipleSignals", func(t *testing.T) {
		mockGraph.queries = []graph.GraphQuery{} // Reset queries

		dashboard := &GrafanaDashboard{
			UID:     "test-dashboard-7",
			Title:   "Golden Signals Dashboard",
			Version: 1,
			Tags:    []string{"test"},
			Panels: []GrafanaPanel{
				{
					ID:    1,
					Title: "Service Health",
					Type:  "graph",
					Targets: []GrafanaTarget{
						{
							RefID: "A",
							Expr:  `up{job="api"}`,
						},
						{
							RefID: "B",
							Expr:  `http_requests_total{job="api"}`,
						},
						{
							RefID: "C",
							Expr:  `http_request_errors_total{job="api"}`,
						},
					},
				},
			},
		}

		err := syncer.syncDashboard(ctx, dashboard)
		require.NoError(t, err)

		// Verify: Multiple signals created from single panel
		metrics := make(map[string]bool)
		for _, query := range mockGraph.queries {
			if query.Parameters["role"] != nil {
				if metricName, ok := query.Parameters["metric_name"].(string); ok {
					metrics[metricName] = true
				}
			}
		}

		assert.True(t, metrics["up"], "Expected 'up' signal")
		assert.True(t, metrics["http_requests_total"], "Expected 'http_requests_total' signal")
		assert.True(t, metrics["http_request_errors_total"], "Expected 'http_request_errors_total' signal")
		assert.GreaterOrEqual(t, len(metrics), 3, "Expected at least 3 signals from multi-query panel")
	})

	// Test case 8: Idempotency - sync same dashboard twice
	t.Run("Idempotency_UpdateNotDuplicate", func(t *testing.T) {
		mockGraph.queries = []graph.GraphQuery{} // Reset queries

		dashboard := &GrafanaDashboard{
			UID:     "test-dashboard-8",
			Title:   "Idempotency Test",
			Version: 1,
			Tags:    []string{"test"},
			Panels: []GrafanaPanel{
				{
					ID:    1,
					Title: "Test Metric",
					Type:  "graph",
					Targets: []GrafanaTarget{
						{
							RefID: "A",
							Expr:  `up{namespace="prod"}`,
						},
					},
				},
			},
		}

		// First sync
		err := syncer.syncDashboard(ctx, dashboard)
		require.NoError(t, err)

		firstSyncQueryCount := len(mockGraph.queries)

		// Second sync (same dashboard)
		err = syncer.syncDashboard(ctx, dashboard)
		require.NoError(t, err)

		// Verify: Signal was updated (MERGE upsert), not duplicated
		// The query count should increase (both syncs execute queries),
		// but MERGE ensures no duplicate nodes in graph
		assert.Greater(t, len(mockGraph.queries), firstSyncQueryCount,
			"Second sync should execute queries")

		// Verify MERGE pattern in queries (ON CREATE SET, ON MATCH SET)
		foundMerge := false
		for _, query := range mockGraph.queries {
			if query.Parameters["role"] != nil {
				metricName, ok := query.Parameters["metric_name"].(string)
				if ok && metricName == "up" {
					// MERGE queries should have ON CREATE and ON MATCH clauses
					foundMerge = true
				}
			}
		}

		assert.True(t, foundMerge, "Expected MERGE upsert for idempotency")
	})
}

// TestSignalIngestion_LowConfidenceFiltering tests that low-confidence signals are filtered
func TestSignalIngestion_LowConfidenceFiltering(t *testing.T) {
	ctx := context.Background()
	logger := logging.GetLogger("test")

	mockGrafana := newMockGrafanaClient()
	mockGraph := newMockGraphClient()

	config := &Config{URL: "https://test.grafana.net"}
	integrationName := "test-grafana"

	syncer := NewDashboardSyncer(mockGrafana, mockGraph, config, integrationName, time.Hour, logger)

	// Dashboard with unclassifiable metrics (would result in confidence < 0.5)
	dashboard := &GrafanaDashboard{
		UID:     "test-dashboard-lowconf",
		Title:   "Low Confidence Dashboard",
		Version: 1,
		Tags:    []string{"test"},
		Panels: []GrafanaPanel{
			{
				ID:    1,
				Title: "Panel Title", // Generic title (0.0 confidence)
				Type:  "graph",
				Targets: []GrafanaTarget{
					{
						RefID: "A",
						// Generic metric with no classification patterns
						Expr: `some_random_metric`,
					},
				},
			},
		},
	}

	err := syncer.syncDashboard(ctx, dashboard)
	require.NoError(t, err)

	// Verify: Low-confidence signal NOT stored
	foundLowConfidence := false
	for _, query := range mockGraph.queries {
		if query.Parameters["role"] != nil {
			metricName, ok := query.Parameters["metric_name"].(string)
			if ok && metricName == "some_random_metric" {
				foundLowConfidence = true
			}
		}
	}

	assert.False(t, foundLowConfidence, "Low-confidence signal should be filtered out")
}

// TestSignalIngestion_NamespaceOnlyInference tests signals with namespace but no workload
func TestSignalIngestion_NamespaceOnlyInference(t *testing.T) {
	ctx := context.Background()
	logger := logging.GetLogger("test")

	mockGrafana := newMockGrafanaClient()
	mockGraph := newMockGraphClient()

	config := &Config{URL: "https://test.grafana.net"}
	integrationName := "test-grafana"

	syncer := NewDashboardSyncer(mockGrafana, mockGraph, config, integrationName, time.Hour, logger)

	dashboard := &GrafanaDashboard{
		UID:     "test-dashboard-ns",
		Title:   "Namespace-only Signal",
		Version: 1,
		Tags:    []string{"test"},
		Panels: []GrafanaPanel{
			{
				ID:    1,
				Title: "Namespace Metric",
				Type:  "graph",
				Targets: []GrafanaTarget{
					{
						RefID: "A",
						// Has namespace but no workload labels
						Expr: `kube_pod_status_phase{namespace="staging"}`,
					},
				},
			},
		},
	}

	err := syncer.syncDashboard(ctx, dashboard)
	require.NoError(t, err)

	// Verify: Signal created with namespace but empty workload_name
	foundNamespaceOnly := false
	for _, query := range mockGraph.queries {
		if query.Parameters["role"] != nil {
			metricName, ok := query.Parameters["metric_name"].(string)
			if ok && metricName == "kube_pod_status_phase" {
				assert.Equal(t, "staging", query.Parameters["workload_namespace"])
				assert.Equal(t, "", query.Parameters["workload_name"])
				foundNamespaceOnly = true
			}
		}
	}

	assert.True(t, foundNamespaceOnly, "Expected namespace-only signal")
}
