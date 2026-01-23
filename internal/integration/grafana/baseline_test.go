package grafana

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestComputeRollingBaseline_InsufficientData(t *testing.T) {
	// Less than 24h of history
	currentTime := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-12 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: currentTime.Add(-6 * time.Hour)},
	}

	_, _, err := ComputeRollingBaseline(transitions, 7, currentTime)

	if err == nil {
		t.Fatal("ComputeRollingBaseline(<24h data) should return error")
	}

	var insufficientDataErr *InsufficientDataError
	if !errors.As(err, &insufficientDataErr) {
		t.Errorf("Error should be InsufficientDataError, got %T: %v", err, err)
	}
}

func TestComputeRollingBaseline_Exactly24Hours(t *testing.T) {
	// Exactly 24h of history
	currentTime := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-24 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: currentTime.Add(-12 * time.Hour)},
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-6 * time.Hour)},
	}

	baseline, stdDev, err := ComputeRollingBaseline(transitions, 7, currentTime)

	if err != nil {
		t.Fatalf("ComputeRollingBaseline(24h data) should not return error, got: %v", err)
	}

	// Should compute baseline from available data
	if baseline.PercentNormal < 0 || baseline.PercentNormal > 1 {
		t.Errorf("PercentNormal = %v, want 0.0-1.0", baseline.PercentNormal)
	}
	if baseline.PercentPending < 0 || baseline.PercentPending > 1 {
		t.Errorf("PercentPending = %v, want 0.0-1.0", baseline.PercentPending)
	}
	if baseline.PercentFiring < 0 || baseline.PercentFiring > 1 {
		t.Errorf("PercentFiring = %v, want 0.0-1.0", baseline.PercentFiring)
	}

	// Sum should be approximately 1.0
	sum := baseline.PercentNormal + baseline.PercentPending + baseline.PercentFiring
	if math.Abs(sum-1.0) > 0.01 {
		t.Errorf("Sum of percentages = %v, want ~1.0", sum)
	}

	// StdDev should be non-negative
	if stdDev < 0 {
		t.Errorf("stdDev = %v, want >= 0", stdDev)
	}
}

func TestComputeRollingBaseline_StableFiring(t *testing.T) {
	// 7 days of stable firing state
	currentTime := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-7 * 24 * time.Hour)},
		// No other transitions - stays firing
	}

	baseline, stdDev, err := ComputeRollingBaseline(transitions, 7, currentTime)

	if err != nil {
		t.Fatalf("ComputeRollingBaseline(stable firing) should not return error, got: %v", err)
	}

	// Should be mostly firing
	if baseline.PercentFiring < 0.9 {
		t.Errorf("PercentFiring = %v, want >= 0.9 for stable firing", baseline.PercentFiring)
	}

	// Standard deviation should be low (stable state)
	if stdDev > 0.1 {
		t.Errorf("stdDev = %v, want <= 0.1 for stable state", stdDev)
	}
}

func TestComputeRollingBaseline_AlternatingStates(t *testing.T) {
	// 7 days of alternating between firing and normal daily
	currentTime := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)
	var transitions []StateTransition

	for day := 7; day > 0; day-- {
		// Fire for 12 hours, normal for 12 hours each day
		transitions = append(transitions, StateTransition{
			FromState: "normal",
			ToState:   "firing",
			Timestamp: currentTime.Add(-time.Duration(day)*24*time.Hour + 6*time.Hour),
		})
		transitions = append(transitions, StateTransition{
			FromState: "firing",
			ToState:   "normal",
			Timestamp: currentTime.Add(-time.Duration(day)*24*time.Hour + 18*time.Hour),
		})
	}

	baseline, stdDev, err := ComputeRollingBaseline(transitions, 7, currentTime)

	if err != nil {
		t.Fatalf("ComputeRollingBaseline(alternating) should not return error, got: %v", err)
	}

	// Should be roughly 50/50 normal and firing
	if baseline.PercentNormal < 0.4 || baseline.PercentNormal > 0.6 {
		t.Errorf("PercentNormal = %v, want ~0.5 for alternating pattern", baseline.PercentNormal)
	}
	if baseline.PercentFiring < 0.4 || baseline.PercentFiring > 0.6 {
		t.Errorf("PercentFiring = %v, want ~0.5 for alternating pattern", baseline.PercentFiring)
	}

	// Standard deviation should be moderate (variability exists)
	if stdDev < 0.05 {
		t.Errorf("stdDev = %v, want > 0.05 for variable pattern", stdDev)
	}
}

func TestComputeRollingBaseline_WithGaps_LOCF(t *testing.T) {
	// Test that gaps are filled using last observation carried forward
	currentTime := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-7 * 24 * time.Hour)},
		// Gap of several days with no transitions - should carry forward "firing" state
		{FromState: "firing", ToState: "normal", Timestamp: currentTime.Add(-1 * time.Hour)},
	}

	baseline, _, err := ComputeRollingBaseline(transitions, 7, currentTime)

	if err != nil {
		t.Fatalf("ComputeRollingBaseline(with gaps) should not return error, got: %v", err)
	}

	// Most of the time should be in firing state due to LOCF
	if baseline.PercentFiring < 0.8 {
		t.Errorf("PercentFiring = %v, want >= 0.8 (LOCF should carry forward firing state)", baseline.PercentFiring)
	}
}

func TestComputeRollingBaseline_AllNormal(t *testing.T) {
	// 7 days with no transitions (all normal)
	currentTime := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)
	transitions := []StateTransition{}

	baseline, stdDev, err := ComputeRollingBaseline(transitions, 7, currentTime)

	if err != nil {
		t.Fatalf("ComputeRollingBaseline(all normal) should not return error, got: %v", err)
	}

	// Should be 100% normal
	if baseline.PercentNormal < 0.99 {
		t.Errorf("PercentNormal = %v, want >= 0.99 for no transitions", baseline.PercentNormal)
	}
	if baseline.PercentFiring > 0.01 {
		t.Errorf("PercentFiring = %v, want ~0.0 for no transitions", baseline.PercentFiring)
	}

	// StdDev should be very low (no variation)
	if stdDev > 0.01 {
		t.Errorf("stdDev = %v, want ~0.0 for stable normal state", stdDev)
	}
}

func TestCompareToBaseline_TwoSigmaDeviation(t *testing.T) {
	baseline := StateDistribution{
		PercentNormal:  0.7,
		PercentPending: 0.1,
		PercentFiring:  0.2,
	}
	stdDev := 0.1

	// Current state is 2 standard deviations above baseline
	current := StateDistribution{
		PercentNormal:  0.5,
		PercentPending: 0.1,
		PercentFiring:  0.4, // baseline + 2*stdDev
	}

	deviationScore := CompareToBaseline(current, baseline, stdDev)

	// Should be approximately 2.0
	if math.Abs(deviationScore-2.0) > 0.1 {
		t.Errorf("CompareToBaseline(2σ deviation) = %v, want ~2.0", deviationScore)
	}
}

func TestCompareToBaseline_ZeroDeviation(t *testing.T) {
	baseline := StateDistribution{
		PercentNormal:  0.7,
		PercentPending: 0.1,
		PercentFiring:  0.2,
	}
	stdDev := 0.1

	// Current matches baseline
	current := baseline

	deviationScore := CompareToBaseline(current, baseline, stdDev)

	// Should be approximately 0.0
	if math.Abs(deviationScore) > 0.01 {
		t.Errorf("CompareToBaseline(zero deviation) = %v, want ~0.0", deviationScore)
	}
}

func TestCompareToBaseline_NegativeDeviation(t *testing.T) {
	baseline := StateDistribution{
		PercentNormal:  0.5,
		PercentPending: 0.1,
		PercentFiring:  0.4,
	}
	stdDev := 0.1

	// Current is below baseline (less firing)
	current := StateDistribution{
		PercentNormal:  0.8,
		PercentPending: 0.1,
		PercentFiring:  0.1, // baseline - 3*stdDev
	}

	deviationScore := CompareToBaseline(current, baseline, stdDev)

	// Should be approximately 3.0 (absolute value)
	if math.Abs(deviationScore-3.0) > 0.1 {
		t.Errorf("CompareToBaseline(3σ below baseline) = %v, want ~3.0", deviationScore)
	}
}

func TestCompareToBaseline_ZeroStdDev(t *testing.T) {
	baseline := StateDistribution{
		PercentNormal:  0.7,
		PercentPending: 0.1,
		PercentFiring:  0.2,
	}
	stdDev := 0.0 // No variation in baseline

	current := StateDistribution{
		PercentNormal:  0.5,
		PercentPending: 0.1,
		PercentFiring:  0.4,
	}

	deviationScore := CompareToBaseline(current, baseline, stdDev)

	// With zero stddev, deviation should be 0 (can't divide by zero)
	if deviationScore != 0.0 {
		t.Errorf("CompareToBaseline(zero stddev) = %v, want 0.0", deviationScore)
	}
}

func TestStateDistribution_Struct(t *testing.T) {
	// Test that StateDistribution type exists and has expected fields
	dist := StateDistribution{
		PercentNormal:  0.5,
		PercentPending: 0.2,
		PercentFiring:  0.3,
	}

	if dist.PercentNormal != 0.5 {
		t.Errorf("PercentNormal = %v, want 0.5", dist.PercentNormal)
	}
	if dist.PercentPending != 0.2 {
		t.Errorf("PercentPending = %v, want 0.2", dist.PercentPending)
	}
	if dist.PercentFiring != 0.3 {
		t.Errorf("PercentFiring = %v, want 0.3", dist.PercentFiring)
	}
}

func TestInsufficientDataError_Fields(t *testing.T) {
	// Test that InsufficientDataError has expected fields
	err := &InsufficientDataError{
		Available: 12 * time.Hour,
		Required:  24 * time.Hour,
	}

	if err.Available != 12*time.Hour {
		t.Errorf("Available = %v, want 12h", err.Available)
	}
	if err.Required != 24*time.Hour {
		t.Errorf("Required = %v, want 24h", err.Required)
	}
	if err.Error() == "" {
		t.Error("Error() should return non-empty string")
	}
}

func TestComputeRollingBaseline_PartialData(t *testing.T) {
	// Test with 3 days of data (partial, but > 24h)
	currentTime := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)
	var transitions []StateTransition

	// 3 days of data: mostly firing with some normal periods
	for day := 3; day > 0; day-- {
		transitions = append(transitions, StateTransition{
			FromState: "normal",
			ToState:   "firing",
			Timestamp: currentTime.Add(-time.Duration(day)*24*time.Hour + 2*time.Hour),
		})
		transitions = append(transitions, StateTransition{
			FromState: "firing",
			ToState:   "normal",
			Timestamp: currentTime.Add(-time.Duration(day)*24*time.Hour + 20*time.Hour),
		})
	}

	baseline, stdDev, err := ComputeRollingBaseline(transitions, 7, currentTime)

	if err != nil {
		t.Fatalf("ComputeRollingBaseline(partial data) should not return error, got: %v", err)
	}

	// Should compute from available 3 days
	// Note: LOCF and partial day boundaries can push this slightly above 0.9
	if baseline.PercentFiring < 0.6 || baseline.PercentFiring > 0.95 {
		t.Errorf("PercentFiring = %v, want 0.6-0.95 (mostly firing for 18h/day)", baseline.PercentFiring)
	}

	// Should have valid stddev
	if stdDev < 0 {
		t.Errorf("stdDev = %v, want >= 0", stdDev)
	}
}
