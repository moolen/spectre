package grafana

import (
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComputeAnomalyScore_NormalValue tests that normal values produce low scores.
// A value within 1 stddev of mean should score below 0.5.
func TestComputeAnomalyScore_NormalValue(t *testing.T) {
	baseline := SignalBaseline{
		Mean:        100.0,
		StdDev:      10.0,
		Min:         80.0,
		Max:         120.0,
		P50:         100.0,
		P90:         115.0,
		P99:         118.0,
		SampleCount: 100,
	}

	// Value is exactly at the mean
	score, err := ComputeAnomalyScore(100.0, baseline, 1.0)
	require.NoError(t, err)
	assert.Less(t, score.Score, 0.5, "value at mean should score < 0.5")
	assert.Equal(t, "z-score", score.Method)

	// Value within 1 stddev of mean
	score, err = ComputeAnomalyScore(105.0, baseline, 1.0)
	require.NoError(t, err)
	assert.Less(t, score.Score, 0.5, "value within 1 stddev should score < 0.5")
}

// TestComputeAnomalyScore_HighZScore tests that high z-scores produce high scores.
// A value 3 stddev above mean should score > 0.7.
func TestComputeAnomalyScore_HighZScore(t *testing.T) {
	baseline := SignalBaseline{
		Mean:        100.0,
		StdDev:      10.0,
		Min:         80.0,
		Max:         120.0,
		P50:         100.0,
		P90:         115.0,
		P99:         125.0, // Higher P99 to avoid percentile score dominating
		SampleCount: 100,
	}

	// Value 3 stddev above mean (z=3)
	score, err := ComputeAnomalyScore(130.0, baseline, 1.0)
	require.NoError(t, err)
	assert.Greater(t, score.Score, 0.7, "z=3 should score > 0.7")
	assert.InDelta(t, 3.0, score.ZScore, 0.01, "z-score should be ~3.0")

	// Value 2 stddev above mean (z=2)
	score, err = ComputeAnomalyScore(120.0, baseline, 1.0)
	require.NoError(t, err)
	assert.Greater(t, score.Score, 0.6, "z=2 should score > 0.6")
}

// TestComputeAnomalyScore_AboveP99 tests percentile-based scoring.
// Value above P99 should trigger percentile score > 0.5.
func TestComputeAnomalyScore_AboveP99(t *testing.T) {
	baseline := SignalBaseline{
		Mean:        100.0,
		StdDev:      10.0,
		Min:         80.0,
		Max:         120.0,
		P50:         100.0,
		P90:         115.0,
		P99:         119.0,
		SampleCount: 100,
	}

	// Value just above P99
	score, err := ComputeAnomalyScore(125.0, baseline, 1.0)
	require.NoError(t, err)
	assert.Greater(t, score.Score, 0.5, "value above P99 should score > 0.5")
}

// TestComputeAnomalyScore_BelowMin tests low value detection.
// Value below historical minimum should trigger anomaly.
func TestComputeAnomalyScore_BelowMin(t *testing.T) {
	baseline := SignalBaseline{
		Mean:        100.0,
		StdDev:      10.0,
		Min:         80.0,
		Max:         120.0,
		P50:         100.0,
		P90:         115.0,
		P99:         119.0,
		SampleCount: 100,
	}

	// Value below historical minimum
	score, err := ComputeAnomalyScore(70.0, baseline, 1.0)
	require.NoError(t, err)
	assert.Greater(t, score.Score, 0.5, "value below min should score > 0.5")
}

// TestComputeAnomalyScore_ColdStart tests cold start handling.
// With < 10 samples, should return InsufficientSamplesError.
func TestComputeAnomalyScore_ColdStart(t *testing.T) {
	baseline := SignalBaseline{
		Mean:        100.0,
		StdDev:      10.0,
		Min:         80.0,
		Max:         120.0,
		P50:         100.0,
		P90:         115.0,
		P99:         119.0,
		SampleCount: 5, // Below MinSamplesRequired (10)
	}

	score, err := ComputeAnomalyScore(100.0, baseline, 1.0)
	assert.Nil(t, score, "should return nil score on cold start")
	require.Error(t, err, "should return error on cold start")

	var insufficientErr *InsufficientSamplesError
	assert.True(t, errors.As(err, &insufficientErr), "error should be InsufficientSamplesError")
	assert.Equal(t, 5, insufficientErr.Available)
	assert.Equal(t, MinSamplesRequired, insufficientErr.Required)
}

// TestComputeAnomalyScore_ExactlyMinSamples tests the boundary at 10 samples.
func TestComputeAnomalyScore_ExactlyMinSamples(t *testing.T) {
	baseline := SignalBaseline{
		Mean:        100.0,
		StdDev:      10.0,
		Min:         80.0,
		Max:         120.0,
		P50:         100.0,
		P90:         115.0,
		P99:         119.0,
		SampleCount: 10, // Exactly at MinSamplesRequired
	}

	score, err := ComputeAnomalyScore(100.0, baseline, 1.0)
	require.NoError(t, err, "exactly 10 samples should be valid")
	assert.NotNil(t, score)
}

// TestComputeAnomalyScore_ZeroStdDev tests handling of zero standard deviation.
// When stddev=0, z-score should be 0 and percentile method should be used.
func TestComputeAnomalyScore_ZeroStdDev(t *testing.T) {
	baseline := SignalBaseline{
		Mean:        100.0,
		StdDev:      0.0, // Zero stddev (all values identical)
		Min:         100.0,
		Max:         100.0,
		P50:         100.0,
		P90:         100.0,
		P99:         100.0,
		SampleCount: 100,
	}

	// Value at mean - should score low
	score, err := ComputeAnomalyScore(100.0, baseline, 1.0)
	require.NoError(t, err)
	assert.Equal(t, 0.0, score.ZScore, "z-score should be 0 when stddev=0")
	assert.Less(t, score.Score, 0.5, "value at mean with zero stddev should score low")

	// Value above all observations - percentile should detect
	score, err = ComputeAnomalyScore(110.0, baseline, 1.0)
	require.NoError(t, err)
	assert.Equal(t, 0.0, score.ZScore, "z-score should still be 0 when stddev=0")
	// Note: percentile score should kick in for above P99
}

// TestComputeAnomalyScore_HybridMAX tests that final score is MAX of both methods.
func TestComputeAnomalyScore_HybridMAX(t *testing.T) {
	// Setup where z-score and percentile give different scores
	baseline := SignalBaseline{
		Mean:        100.0,
		StdDev:      5.0,  // Small stddev - z-score will be high
		Min:         90.0,
		Max:         110.0,
		P50:         100.0,
		P90:         108.0,
		P99:         109.0, // Low P99 - percentile will also be high
		SampleCount: 100,
	}

	// Value that triggers both methods
	score, err := ComputeAnomalyScore(115.0, baseline, 1.0)
	require.NoError(t, err)

	// Calculate expected z-score
	expectedZ := (115.0 - 100.0) / 5.0 // z = 3
	assert.InDelta(t, expectedZ, score.ZScore, 0.01)

	// Final score should be MAX of both methods
	// For z=3, normalized score is ~0.78
	// For 115 > P99(109), percentile score > 0.5
	assert.Greater(t, score.Score, 0.5, "hybrid score should be > 0.5")
}

// TestComputeAnomalyScore_Confidence tests confidence calculation.
// confidence = MIN(sampleConfidence, qualityScore)
func TestComputeAnomalyScore_Confidence(t *testing.T) {
	// High sample count, high quality
	baseline := SignalBaseline{
		Mean:        100.0,
		StdDev:      10.0,
		Min:         80.0,
		Max:         120.0,
		P50:         100.0,
		P90:         115.0,
		P99:         119.0,
		SampleCount: 200, // High sample count
	}

	score, err := ComputeAnomalyScore(100.0, baseline, 1.0)
	require.NoError(t, err)
	assert.Greater(t, score.Confidence, 0.9, "high samples + high quality = high confidence")

	// Same baseline, low quality
	score, err = ComputeAnomalyScore(100.0, baseline, 0.3)
	require.NoError(t, err)
	assert.LessOrEqual(t, score.Confidence, 0.3, "confidence capped by quality score")

	// Low sample count (just above minimum)
	baselineLowSamples := SignalBaseline{
		Mean:        100.0,
		StdDev:      10.0,
		Min:         80.0,
		Max:         120.0,
		P50:         100.0,
		P90:         115.0,
		P99:         119.0,
		SampleCount: 15, // Just above minimum
	}

	score, err = ComputeAnomalyScore(100.0, baselineLowSamples, 1.0)
	require.NoError(t, err)
	assert.Less(t, score.Confidence, 0.6, "low sample count = low sample confidence")
}

// TestComputeAnomalyScore_ZScoreNormalization tests the sigmoid-like z-score mapping.
// z=2 -> ~0.63, z=3 -> ~0.78
func TestComputeAnomalyScore_ZScoreNormalization(t *testing.T) {
	baseline := SignalBaseline{
		Mean:        100.0,
		StdDev:      10.0,
		Min:         50.0,
		Max:         150.0,
		P50:         100.0,
		P90:         115.0,
		P99:         145.0, // High P99 so percentile doesn't dominate
		SampleCount: 100,
	}

	// z=2: value = mean + 2*stddev = 120
	score, err := ComputeAnomalyScore(120.0, baseline, 1.0)
	require.NoError(t, err)
	// zScoreNormalized = 1.0 - exp(-|z|/2.0) = 1.0 - exp(-1) = ~0.632
	expectedNormalized := 1.0 - math.Exp(-2.0/2.0)
	assert.InDelta(t, expectedNormalized, score.Score, 0.05, "z=2 should normalize to ~0.63")

	// z=3: value = mean + 3*stddev = 130
	score, err = ComputeAnomalyScore(130.0, baseline, 1.0)
	require.NoError(t, err)
	// zScoreNormalized = 1.0 - exp(-|z|/2.0) = 1.0 - exp(-1.5) = ~0.777
	expectedNormalized = 1.0 - math.Exp(-3.0/2.0)
	assert.InDelta(t, expectedNormalized, score.Score, 0.05, "z=3 should normalize to ~0.78")
}

// TestComputeAnomalyScore_NegativeZScore tests that negative z-scores work correctly.
// Value below mean should also trigger z-score scoring.
func TestComputeAnomalyScore_NegativeZScore(t *testing.T) {
	baseline := SignalBaseline{
		Mean:        100.0,
		StdDev:      10.0,
		Min:         50.0,  // Low min so percentile doesn't trigger
		Max:         150.0,
		P50:         100.0,
		P90:         115.0,
		P99:         145.0,
		SampleCount: 100,
	}

	// z=-3: value = mean - 3*stddev = 70
	score, err := ComputeAnomalyScore(70.0, baseline, 1.0)
	require.NoError(t, err)
	assert.InDelta(t, -3.0, score.ZScore, 0.01, "z-score should be -3.0")
	// Normalized uses absolute value
	expectedNormalized := 1.0 - math.Exp(-3.0/2.0)
	assert.InDelta(t, expectedNormalized, score.Score, 0.05, "z=-3 should normalize same as z=3")
}

// TestComputeAnomalyScore_ScoreBounds tests that score is bounded 0.0-1.0.
func TestComputeAnomalyScore_ScoreBounds(t *testing.T) {
	baseline := SignalBaseline{
		Mean:        100.0,
		StdDev:      10.0,
		Min:         80.0,
		Max:         120.0,
		P50:         100.0,
		P90:         115.0,
		P99:         119.0,
		SampleCount: 100,
	}

	testValues := []float64{100.0, 0.0, 200.0, -100.0, 1000.0}
	for _, value := range testValues {
		score, err := ComputeAnomalyScore(value, baseline, 1.0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, score.Score, 0.0, "score should be >= 0.0 for value %f", value)
		assert.LessOrEqual(t, score.Score, 1.0, "score should be <= 1.0 for value %f", value)
	}
}

// TestApplyAlertOverride_Firing tests that firing alerts override score to 1.0.
func TestApplyAlertOverride_Firing(t *testing.T) {
	originalScore := &AnomalyScore{
		Score:      0.3,
		Confidence: 0.8,
		Method:     "z-score",
		ZScore:     1.5,
	}

	overridden := ApplyAlertOverride(originalScore, "firing")

	assert.Equal(t, 1.0, overridden.Score, "firing alert should override score to 1.0")
	assert.Equal(t, 1.0, overridden.Confidence, "firing alert should override confidence to 1.0")
	assert.Equal(t, "alert-override", overridden.Method)
	assert.Equal(t, 1.5, overridden.ZScore, "original z-score should be preserved")
}

// TestApplyAlertOverride_Pending tests that pending alerts don't override.
func TestApplyAlertOverride_Pending(t *testing.T) {
	originalScore := &AnomalyScore{
		Score:      0.3,
		Confidence: 0.8,
		Method:     "z-score",
		ZScore:     1.5,
	}

	result := ApplyAlertOverride(originalScore, "pending")

	assert.Equal(t, originalScore.Score, result.Score, "pending should not override")
	assert.Equal(t, originalScore.Confidence, result.Confidence)
	assert.Equal(t, originalScore.Method, result.Method)
}

// TestApplyAlertOverride_Normal tests that normal alert state doesn't override.
func TestApplyAlertOverride_Normal(t *testing.T) {
	originalScore := &AnomalyScore{
		Score:      0.3,
		Confidence: 0.8,
		Method:     "z-score",
		ZScore:     1.5,
	}

	result := ApplyAlertOverride(originalScore, "normal")

	assert.Equal(t, originalScore.Score, result.Score, "normal should not override")
}

// TestApplyAlertOverride_EmptyState tests that empty alert state doesn't override.
func TestApplyAlertOverride_EmptyState(t *testing.T) {
	originalScore := &AnomalyScore{
		Score:      0.3,
		Confidence: 0.8,
		Method:     "z-score",
		ZScore:     1.5,
	}

	result := ApplyAlertOverride(originalScore, "")

	assert.Equal(t, originalScore.Score, result.Score, "empty state should not override")
}

// TestAnomalyScoreType tests that AnomalyScore struct has all required fields.
func TestAnomalyScoreType(t *testing.T) {
	score := AnomalyScore{
		Score:      0.75,
		Confidence: 0.9,
		Method:     "z-score",
		ZScore:     2.5,
	}

	assert.Equal(t, 0.75, score.Score)
	assert.Equal(t, 0.9, score.Confidence)
	assert.Equal(t, "z-score", score.Method)
	assert.Equal(t, 2.5, score.ZScore)
}

// TestComputeAnomalyScore_MethodSelection tests correct method attribution.
func TestComputeAnomalyScore_MethodSelection(t *testing.T) {
	// Setup where z-score dominates
	baselineZScoreDominates := SignalBaseline{
		Mean:        100.0,
		StdDev:      5.0, // Small stddev
		Min:         50.0,
		Max:         150.0,
		P50:         100.0,
		P90:         110.0,
		P99:         145.0, // High P99 - percentile won't trigger
		SampleCount: 100,
	}

	// z=3 with high P99
	score, err := ComputeAnomalyScore(115.0, baselineZScoreDominates, 1.0)
	require.NoError(t, err)
	assert.Equal(t, "z-score", score.Method, "z-score should dominate when percentile doesn't trigger")

	// Setup where percentile dominates
	baselinePercentileDominates := SignalBaseline{
		Mean:        100.0,
		StdDev:      50.0, // Large stddev - z-score will be low
		Min:         90.0,
		Max:         110.0,
		P50:         100.0,
		P90:         105.0,
		P99:         108.0, // Low P99
		SampleCount: 100,
	}

	// Small z-score but above P99
	score, err = ComputeAnomalyScore(115.0, baselinePercentileDominates, 1.0)
	require.NoError(t, err)
	assert.Equal(t, "percentile", score.Method, "percentile should dominate when z-score is low")
}
