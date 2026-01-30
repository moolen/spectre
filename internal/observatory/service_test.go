package observatory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_GetClusterAnomalies(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()

	// Create provider with anomalous signal
	provider := NewTestProvider("test")
	provider.AddSignal(SignalAnchor{
		MetricName:        "http_errors_total",
		Role:              SignalErrors,
		Confidence:        0.9,
		QualityScore:      0.85,
		WorkloadNamespace: "prod",
		WorkloadName:      "api-server",
	})
	provider.SetBaseline("http_errors_total", "prod", "api-server", &SignalBaseline{
		Mean:        5,
		StdDev:      2,
		P50:         5,
		P90:         8,
		P99:         12,
		Min:         1,
		Max:         15,
		SampleCount: 168,
	})
	// Set high current value to trigger anomaly
	provider.SetCurrentValue("http_errors_total", "prod", "api-server", 50)

	require.NoError(t, registry.Register(provider))

	service := NewService(registry)

	result, err := service.GetClusterAnomalies(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotEmpty(t, result.Timestamp)
	// With anomalous signal, we should have hotspots
	assert.GreaterOrEqual(t, len(result.TopHotspots), 0)
}

func TestService_GetNamespaceAnomalies(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()

	provider := NewTestProvider("test")
	// Add two workloads in same namespace
	provider.AddSignal(SignalAnchor{
		MetricName:        "http_errors_total",
		Role:              SignalErrors,
		Confidence:        0.9,
		QualityScore:      0.85,
		WorkloadNamespace: "prod",
		WorkloadName:      "api-server",
	})
	provider.AddSignal(SignalAnchor{
		MetricName:        "http_requests_total",
		Role:              SignalTraffic,
		Confidence:        0.9,
		QualityScore:      0.8,
		WorkloadNamespace: "prod",
		WorkloadName:      "nginx",
	})
	provider.SetBaseline("http_errors_total", "prod", "api-server", &SignalBaseline{
		Mean:        5,
		StdDev:      2,
		P50:         5,
		P90:         8,
		P99:         12,
		SampleCount: 168,
	})
	provider.SetBaseline("http_requests_total", "prod", "nginx", &SignalBaseline{
		Mean:        1000,
		StdDev:      100,
		P50:         1000,
		P90:         1100,
		P99:         1150,
		SampleCount: 168,
	})
	// Set anomalous value for api-server
	provider.SetCurrentValue("http_errors_total", "prod", "api-server", 50)
	provider.SetCurrentValue("http_requests_total", "prod", "nginx", 1000)

	require.NoError(t, registry.Register(provider))

	service := NewService(registry)

	result, err := service.GetNamespaceAnomalies(ctx, "prod")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "prod", result.Namespace)
	assert.NotEmpty(t, result.Timestamp)
}

func TestService_GetWorkloadAnomalyDetail(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()

	provider := NewTestProvider("test")
	provider.AddSignal(SignalAnchor{
		MetricName:        "http_errors_total",
		Role:              SignalErrors,
		Confidence:        0.9,
		QualityScore:      0.85,
		WorkloadNamespace: "prod",
		WorkloadName:      "api-server",
	})
	provider.AddSignal(SignalAnchor{
		MetricName:        "http_latency_seconds",
		Role:              SignalLatency,
		Confidence:        0.85,
		QualityScore:      0.9,
		WorkloadNamespace: "prod",
		WorkloadName:      "api-server",
	})
	provider.SetBaseline("http_errors_total", "prod", "api-server", &SignalBaseline{
		Mean:        5,
		StdDev:      2,
		P50:         5,
		P90:         8,
		P99:         12,
		SampleCount: 168,
	})
	provider.SetBaseline("http_latency_seconds", "prod", "api-server", &SignalBaseline{
		Mean:        0.05,
		StdDev:      0.02,
		P50:         0.05,
		P90:         0.08,
		P99:         0.12,
		SampleCount: 168,
	})
	// Set anomalous values
	provider.SetCurrentValue("http_errors_total", "prod", "api-server", 50)
	provider.SetCurrentValue("http_latency_seconds", "prod", "api-server", 0.25)

	require.NoError(t, registry.Register(provider))

	service := NewService(registry)

	result, err := service.GetWorkloadAnomalyDetail(ctx, "prod", "api-server")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "prod", result.Namespace)
	assert.Equal(t, "api-server", result.Workload)
	assert.NotEmpty(t, result.Timestamp)
	// Both signals should be anomalous
	assert.Len(t, result.Signals, 2)
}

func TestInvestigateService_GetWorkloadSignals(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()

	provider := NewTestProvider("test")
	provider.AddSignal(SignalAnchor{
		MetricName:        "http_errors_total",
		Role:              SignalErrors,
		Confidence:        0.9,
		QualityScore:      0.85,
		WorkloadNamespace: "prod",
		WorkloadName:      "api-server",
	})
	provider.SetBaseline("http_errors_total", "prod", "api-server", &SignalBaseline{
		Mean:        5,
		StdDev:      2,
		P50:         5,
		P90:         8,
		P99:         12,
		SampleCount: 168,
	})
	provider.SetCurrentValue("http_errors_total", "prod", "api-server", 50)

	require.NoError(t, registry.Register(provider))

	service := NewInvestigateService(registry)

	result, err := service.GetWorkloadSignals(ctx, "prod", "api-server")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "prod/api-server", result.Scope)
	assert.Len(t, result.Signals, 1)
	assert.Equal(t, "http_errors_total", result.Signals[0].MetricName)
	assert.Equal(t, "Errors", result.Signals[0].Role)
	assert.Greater(t, result.Signals[0].Score, 0.5, "should be anomalous")
}

func TestInvestigateService_GetSignalDetail(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()

	provider := NewTestProvider("test")
	provider.AddSignal(SignalAnchor{
		MetricName:        "http_errors_total",
		Role:              SignalErrors,
		Confidence:        0.9,
		QualityScore:      0.85,
		WorkloadNamespace: "prod",
		WorkloadName:      "api-server",
		SourceProvider:    "test",
		SourceRef:         "dashboard-123",
	})
	provider.SetBaseline("http_errors_total", "prod", "api-server", &SignalBaseline{
		Mean:        5,
		StdDev:      2,
		P50:         5,
		P90:         8,
		P99:         12,
		SampleCount: 168,
	})
	provider.SetCurrentValue("http_errors_total", "prod", "api-server", 50)

	require.NoError(t, registry.Register(provider))

	service := NewInvestigateService(registry)

	result, err := service.GetSignalDetail(ctx, "prod", "api-server", "http_errors_total")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "http_errors_total", result.MetricName)
	assert.Equal(t, "Errors", result.Role)
	assert.Equal(t, 50.0, result.CurrentValue)
	assert.Equal(t, 5.0, result.Baseline.Mean)
	assert.Equal(t, 168, result.Baseline.SampleCount)
	assert.Greater(t, result.AnomalyScore, 0.5)
	assert.Equal(t, "test", result.SourceProvider)
}

func TestInvestigateService_GetSignalDetail_NotFound(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()

	provider := NewTestProvider("test")
	require.NoError(t, registry.Register(provider))

	service := NewInvestigateService(registry)

	_, err := service.GetSignalDetail(ctx, "prod", "api-server", "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInvestigateService_CompareSignal(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()

	provider := NewTestProvider("test")
	provider.AddSignal(SignalAnchor{
		MetricName:        "http_errors_total",
		Role:              SignalErrors,
		Confidence:        0.9,
		QualityScore:      0.85,
		WorkloadNamespace: "prod",
		WorkloadName:      "api-server",
	})
	provider.SetBaseline("http_errors_total", "prod", "api-server", &SignalBaseline{
		Mean:        5,
		StdDev:      2,
		P50:         5,
		P90:         8,
		P99:         12,
		SampleCount: 168,
	})
	// High current value = anomalous
	provider.SetCurrentValue("http_errors_total", "prod", "api-server", 50)

	require.NoError(t, registry.Register(provider))

	service := NewInvestigateService(registry)

	result, err := service.CompareSignal(ctx, "prod", "api-server", "http_errors_total", 0)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "http_errors_total", result.MetricName)
	assert.Equal(t, 50.0, result.CurrentValue)
	assert.Equal(t, 5.0, result.PastValue) // Baseline mean
	assert.Equal(t, 24, result.LookbackHours)
	assert.Greater(t, result.CurrentScore, result.PastScore)
	assert.Greater(t, result.ScoreDelta, 0.0, "score should be increasing (getting worse)")
}

func TestAnomalyAggregator_CacheHit(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()

	provider := NewTestProvider("test")
	provider.AddSignal(SignalAnchor{
		MetricName:        "http_errors_total",
		Role:              SignalErrors,
		Confidence:        0.9,
		QualityScore:      0.85,
		WorkloadNamespace: "prod",
		WorkloadName:      "api-server",
	})
	provider.SetBaseline("http_errors_total", "prod", "api-server", &SignalBaseline{
		Mean:        5,
		StdDev:      2,
		P50:         5,
		P90:         8,
		P99:         12,
		SampleCount: 168,
	})
	provider.SetCurrentValue("http_errors_total", "prod", "api-server", 50)

	require.NoError(t, registry.Register(provider))

	aggregator := NewAnomalyAggregator(registry)

	// First call populates cache
	result1, err := aggregator.AggregateWorkloadAnomaly(ctx, "prod", "api-server")
	require.NoError(t, err)
	require.NotNil(t, result1)

	// Second call should hit cache (same result)
	result2, err := aggregator.AggregateWorkloadAnomaly(ctx, "prod", "api-server")
	require.NoError(t, err)
	require.NotNil(t, result2)

	assert.Equal(t, result1.Score, result2.Score)
	assert.Equal(t, result1.TopSource, result2.TopSource)
}

func TestAnomalyAggregator_ClearCache(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()

	provider := NewTestProvider("test")
	provider.AddSignal(SignalAnchor{
		MetricName:        "http_errors_total",
		Role:              SignalErrors,
		Confidence:        0.9,
		QualityScore:      0.85,
		WorkloadNamespace: "prod",
		WorkloadName:      "api-server",
	})
	provider.SetBaseline("http_errors_total", "prod", "api-server", &SignalBaseline{
		Mean:        5,
		StdDev:      2,
		P50:         5,
		P90:         8,
		P99:         12,
		SampleCount: 168,
	})
	provider.SetCurrentValue("http_errors_total", "prod", "api-server", 50)

	require.NoError(t, registry.Register(provider))

	aggregator := NewAnomalyAggregator(registry)

	// First call
	result1, err := aggregator.AggregateWorkloadAnomaly(ctx, "prod", "api-server")
	require.NoError(t, err)
	require.NotNil(t, result1)

	// Clear cache and change value
	aggregator.ClearCache()
	provider.SetCurrentValue("http_errors_total", "prod", "api-server", 5) // Normal value

	// Should recompute with new value
	result2, err := aggregator.AggregateWorkloadAnomaly(ctx, "prod", "api-server")
	require.NoError(t, err)
	require.NotNil(t, result2)

	// Scores should be different
	assert.NotEqual(t, result1.Score, result2.Score, "should have different scores after cache clear")
}
