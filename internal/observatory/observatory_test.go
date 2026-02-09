package observatory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignalRole_Constants(t *testing.T) {
	// Verify all expected signal roles exist
	assert.Equal(t, SignalRole("Availability"), SignalAvailability)
	assert.Equal(t, SignalRole("Latency"), SignalLatency)
	assert.Equal(t, SignalRole("Errors"), SignalErrors)
	assert.Equal(t, SignalRole("Traffic"), SignalTraffic)
	assert.Equal(t, SignalRole("Saturation"), SignalSaturation)
	assert.Equal(t, SignalRole("Novelty"), SignalNovelty)
	assert.Equal(t, SignalRole("Unknown"), SignalUnknown)
}

func TestComputeAnomalyScore_NormalValue(t *testing.T) {
	baseline := SignalBaseline{
		Mean:        100,
		StdDev:      10,
		P50:         100,
		P90:         115,
		P99:         120,
		Min:         80,
		Max:         120,
		SampleCount: 100,
	}

	// Value at mean should have low score
	score, err := ComputeAnomalyScore(100, baseline, 0.8)
	require.NoError(t, err)
	assert.Less(t, score.Score, 0.3, "value at mean should have low anomaly score")
	assert.Equal(t, "z-score", score.Method)
}

func TestComputeAnomalyScore_HighValue(t *testing.T) {
	baseline := SignalBaseline{
		Mean:        100,
		StdDev:      10,
		P50:         100,
		P90:         115,
		P99:         120,
		Min:         80,
		Max:         120,
		SampleCount: 100,
	}

	// Value well above P99 should have high score
	score, err := ComputeAnomalyScore(150, baseline, 0.8)
	require.NoError(t, err)
	assert.Greater(t, score.Score, 0.5, "value above P99 should be anomalous")
}

func TestComputeAnomalyScore_ColdStart(t *testing.T) {
	baseline := SignalBaseline{
		Mean:        100,
		StdDev:      10,
		SampleCount: 5, // Below MinSamplesRequired
	}

	_, err := ComputeAnomalyScore(100, baseline, 0.8)
	require.Error(t, err)

	var insufficientErr *InsufficientSamplesError
	require.ErrorAs(t, err, &insufficientErr)
	assert.Equal(t, 5, insufficientErr.Available)
	assert.Equal(t, MinSamplesRequired, insufficientErr.Required)
}

func TestApplyAlertOverride_Firing(t *testing.T) {
	original := &AnomalyScore{
		Score:      0.3,
		Confidence: 0.8,
		Method:     "z-score",
		ZScore:     1.5,
	}

	overridden := ApplyAlertOverride(original, "firing")

	assert.Equal(t, 1.0, overridden.Score)
	assert.Equal(t, 1.0, overridden.Confidence)
	assert.Equal(t, "alert-override", overridden.Method)
	assert.Equal(t, 1.5, overridden.ZScore, "z-score should be preserved")
}

func TestApplyAlertOverride_NotFiring(t *testing.T) {
	original := &AnomalyScore{
		Score:      0.3,
		Confidence: 0.8,
		Method:     "z-score",
		ZScore:     1.5,
	}

	// Normal state should not override
	result := ApplyAlertOverride(original, "normal")
	assert.Equal(t, original, result)

	// Empty state should not override
	result = ApplyAlertOverride(original, "")
	assert.Equal(t, original, result)
}

func TestRegistry_RegisterAndList(t *testing.T) {
	registry := NewRegistry()

	// Create and register a provider
	provider := NewTestProvider("test-provider")
	provider.AddSignal(SignalAnchor{
		MetricName:        "http_requests_total",
		Role:              SignalTraffic,
		Confidence:        0.9,
		QualityScore:      0.85,
		WorkloadNamespace: "prod",
		WorkloadName:      "api-server",
	})

	err := registry.Register(provider)
	require.NoError(t, err)

	// List signals
	ctx := context.Background()
	signals, err := registry.ListAllSignalAnchors(ctx, SignalListOptions{})
	require.NoError(t, err)
	assert.Len(t, signals, 1)
	assert.Equal(t, "http_requests_total", signals[0].MetricName)
	assert.Equal(t, "test-provider", signals[0].SourceProvider)
}

func TestRegistry_DuplicateProvider(t *testing.T) {
	registry := NewRegistry()

	provider1 := NewTestProvider("same-name")
	provider2 := NewTestProvider("same-name")

	err := registry.Register(provider1)
	require.NoError(t, err)

	err = registry.Register(provider2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_ConflictResolution(t *testing.T) {
	registry := NewRegistry()
	ctx := context.Background()

	// Provider 1 with lower quality
	provider1 := NewTestProvider("provider-1")
	provider1.AddSignal(SignalAnchor{
		MetricName:        "http_requests_total",
		Role:              SignalTraffic,
		Confidence:        0.9,
		QualityScore:      0.7, // Lower quality
		WorkloadNamespace: "prod",
		WorkloadName:      "api-server",
	})

	// Provider 2 with higher quality
	provider2 := NewTestProvider("provider-2")
	provider2.AddSignal(SignalAnchor{
		MetricName:        "http_requests_total",
		Role:              SignalTraffic,
		Confidence:        0.9,
		QualityScore:      0.9, // Higher quality
		WorkloadNamespace: "prod",
		WorkloadName:      "api-server",
	})

	require.NoError(t, registry.Register(provider1))
	require.NoError(t, registry.Register(provider2))

	// Should return the higher quality signal
	signals, err := registry.ListAllSignalAnchors(ctx, SignalListOptions{})
	require.NoError(t, err)
	assert.Len(t, signals, 1)
	assert.Equal(t, 0.9, signals[0].QualityScore)
	assert.Equal(t, "provider-2", signals[0].SourceProvider)
}

func TestRegistry_FilterByNamespace(t *testing.T) {
	registry := NewRegistry()
	ctx := context.Background()

	provider := NewTestProvider("test")
	provider.AddSignal(SignalAnchor{
		MetricName:        "metric1",
		WorkloadNamespace: "prod",
		WorkloadName:      "app1",
	})
	provider.AddSignal(SignalAnchor{
		MetricName:        "metric2",
		WorkloadNamespace: "staging",
		WorkloadName:      "app2",
	})

	require.NoError(t, registry.Register(provider))

	// Filter by namespace
	signals, err := registry.ListAllSignalAnchors(ctx, SignalListOptions{
		Namespace: "prod",
	})
	require.NoError(t, err)
	assert.Len(t, signals, 1)
	assert.Equal(t, "metric1", signals[0].MetricName)
}

func TestTestProvider_CurrentValueAndBaseline(t *testing.T) {
	ctx := context.Background()
	provider := NewTestProvider("test")

	// Set up test data
	provider.SetCurrentValue("http_requests", "prod", "api", 1500)
	provider.SetBaseline("http_requests", "prod", "api", &SignalBaseline{
		Mean:        1000,
		StdDev:      100,
		SampleCount: 168,
	})
	provider.SetAlertState("http_requests", "prod", "api", "firing")

	// Retrieve values
	value, found, err := provider.GetCurrentValue(ctx, "http_requests", "prod", "api")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 1500.0, value)

	baseline, err := provider.GetBaseline(ctx, "http_requests", "prod", "api")
	require.NoError(t, err)
	require.NotNil(t, baseline)
	assert.Equal(t, 1000.0, baseline.Mean)

	state, err := provider.GetAlertState(ctx, "http_requests", "prod", "api")
	require.NoError(t, err)
	assert.Equal(t, "firing", state)
}

func TestComputeRollingStatistics(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	stats := ComputeRollingStatistics(values)

	assert.Equal(t, 10, stats.SampleCount)
	assert.Equal(t, 5.5, stats.Mean)
	assert.Equal(t, 1.0, stats.Min)
	assert.Equal(t, 10.0, stats.Max)
	assert.Greater(t, stats.StdDev, 0.0)
}

func TestComputeRollingStatistics_Empty(t *testing.T) {
	stats := ComputeRollingStatistics([]float64{})

	assert.Equal(t, 0, stats.SampleCount)
	assert.Equal(t, 0.0, stats.Mean)
}
