package grafana

import (
	"math"
	"testing"
	"time"
)

func TestComputeFlappinessScore_EmptyTransitions(t *testing.T) {
	transitions := []StateTransition{}
	windowSize := 6 * time.Hour
	currentTime := time.Now()

	score := ComputeFlappinessScore(transitions, windowSize, currentTime)

	if score != 0.0 {
		t.Errorf("ComputeFlappinessScore(empty) = %v, want 0.0", score)
	}
}

func TestComputeFlappinessScore_SingleTransition(t *testing.T) {
	currentTime := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)
	transitions := []StateTransition{
		{
			FromState: "normal",
			ToState:   "firing",
			Timestamp: currentTime.Add(-1 * time.Hour),
		},
	}
	windowSize := 6 * time.Hour

	score := ComputeFlappinessScore(transitions, windowSize, currentTime)

	// Single transition should have low score
	if score <= 0.0 || score > 0.2 {
		t.Errorf("ComputeFlappinessScore(single transition) = %v, want between 0.0-0.2", score)
	}
}

func TestComputeFlappinessScore_ModerateFlapping(t *testing.T) {
	currentTime := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)

	// 5 transitions in 6 hours (one every ~1.5 hours)
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-5 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: currentTime.Add(-4 * time.Hour)},
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-3 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: currentTime.Add(-2 * time.Hour)},
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-1 * time.Hour)},
	}
	windowSize := 6 * time.Hour

	score := ComputeFlappinessScore(transitions, windowSize, currentTime)

	// Moderate flapping should have moderate score around 0.5
	if score < 0.3 || score > 0.7 {
		t.Errorf("ComputeFlappinessScore(moderate flapping) = %v, want between 0.3-0.7", score)
	}
}

func TestComputeFlappinessScore_HighFlapping_ShortStates(t *testing.T) {
	currentTime := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)

	// 10 transitions with short durations (every 30 minutes)
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-5 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: currentTime.Add(-270 * time.Minute)},
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-240 * time.Minute)},
		{FromState: "firing", ToState: "normal", Timestamp: currentTime.Add(-210 * time.Minute)},
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-180 * time.Minute)},
		{FromState: "firing", ToState: "normal", Timestamp: currentTime.Add(-150 * time.Minute)},
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-120 * time.Minute)},
		{FromState: "firing", ToState: "normal", Timestamp: currentTime.Add(-90 * time.Minute)},
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-60 * time.Minute)},
		{FromState: "firing", ToState: "normal", Timestamp: currentTime.Add(-30 * time.Minute)},
	}
	windowSize := 6 * time.Hour

	score := ComputeFlappinessScore(transitions, windowSize, currentTime)

	// High flapping with short states should have high score
	if score < 0.7 || score > 1.0 {
		t.Errorf("ComputeFlappinessScore(high flapping) = %v, want between 0.7-1.0", score)
	}
}

func TestComputeFlappinessScore_ManyTransitions_LongLivedStates(t *testing.T) {
	currentTime := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)

	// 5 transitions but with longer durations (less flappy than same count with short durations)
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-6 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: currentTime.Add(-5 * time.Hour)},
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-4 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: currentTime.Add(-2 * time.Hour)},
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-1 * time.Hour)},
	}
	windowSize := 6 * time.Hour

	// For comparison, create the same number of transitions but with shorter durations
	shortTransitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-5 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: currentTime.Add(-4*time.Hour - 30*time.Minute)},
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-4 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: currentTime.Add(-3*time.Hour - 30*time.Minute)},
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-3 * time.Hour)},
	}

	longScore := ComputeFlappinessScore(transitions, windowSize, currentTime)
	shortScore := ComputeFlappinessScore(shortTransitions, windowSize, currentTime)

	// Long-lived states should have lower score than short-lived states with same transition count
	if longScore >= shortScore {
		t.Errorf("Long-lived states score (%v) should be lower than short-lived states score (%v)", longScore, shortScore)
	}
}

func TestComputeFlappinessScore_TransitionsOutsideWindow(t *testing.T) {
	currentTime := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)
	windowSize := 6 * time.Hour

	// Mix of transitions inside and outside window
	transitions := []StateTransition{
		// Outside window (should be ignored)
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-10 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: currentTime.Add(-8 * time.Hour)},
		// Inside window (should be counted)
		{FromState: "normal", ToState: "firing", Timestamp: currentTime.Add(-3 * time.Hour)},
	}

	score := ComputeFlappinessScore(transitions, windowSize, currentTime)

	// Should behave like single transition case
	if score <= 0.0 || score > 0.2 {
		t.Errorf("ComputeFlappinessScore(transitions outside window) = %v, want between 0.0-0.2", score)
	}
}

func TestComputeFlappinessScore_NormalizedRange(t *testing.T) {
	currentTime := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)
	windowSize := 6 * time.Hour

	// Create extreme flapping scenario (transition every 5 minutes)
	var transitions []StateTransition
	for i := 0; i < 72; i++ { // 72 transitions in 6 hours
		fromState := "normal"
		toState := "firing"
		if i%2 == 1 {
			fromState = "firing"
			toState = "normal"
		}
		transitions = append(transitions, StateTransition{
			FromState: fromState,
			ToState:   toState,
			Timestamp: currentTime.Add(-time.Duration(6*60-i*5) * time.Minute),
		})
	}

	score := ComputeFlappinessScore(transitions, windowSize, currentTime)

	// Score should be capped at 1.0
	if score < 0.0 || score > 1.0 {
		t.Errorf("ComputeFlappinessScore(extreme flapping) = %v, want between 0.0-1.0 (capped)", score)
	}

	// Extreme flapping should be close to 1.0
	if score < 0.9 {
		t.Errorf("ComputeFlappinessScore(extreme flapping) = %v, want >= 0.9", score)
	}
}

func TestComputeFlappinessScore_ScoreMonotonicity(t *testing.T) {
	// Test that more transitions generally lead to higher scores
	currentTime := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)
	windowSize := 6 * time.Hour

	// Create scenarios with increasing transition counts
	scenarios := []struct {
		name  string
		count int
	}{
		{"zero", 0},
		{"one", 1},
		{"three", 3},
		{"five", 5},
		{"ten", 10},
	}

	var prevScore float64
	for i, scenario := range scenarios {
		var transitions []StateTransition
		if scenario.count > 0 {
			// Distribute transitions evenly across window
			interval := windowSize / time.Duration(scenario.count)
			for j := 0; j < scenario.count; j++ {
				fromState := "normal"
				toState := "firing"
				if j%2 == 1 {
					fromState = "firing"
					toState = "normal"
				}
				transitions = append(transitions, StateTransition{
					FromState: fromState,
					ToState:   toState,
					Timestamp: currentTime.Add(-windowSize + time.Duration(j+1)*interval),
				})
			}
		}

		score := ComputeFlappinessScore(transitions, windowSize, currentTime)

		t.Logf("%s transitions: score = %v", scenario.name, score)

		// Scores should generally increase (allowing for small numerical variations)
		if i > 0 && score < prevScore-0.01 {
			t.Errorf("Score decreased with more transitions: %d transitions = %v, %d transitions = %v",
				scenarios[i-1].count, prevScore, scenario.count, score)
		}

		prevScore = score
	}
}

func TestStateTransition_Struct(t *testing.T) {
	// Test that StateTransition type exists and has expected fields
	transition := StateTransition{
		FromState: "normal",
		ToState:   "firing",
		Timestamp: time.Now(),
	}

	if transition.FromState != "normal" {
		t.Errorf("FromState = %v, want normal", transition.FromState)
	}
	if transition.ToState != "firing" {
		t.Errorf("ToState = %v, want firing", transition.ToState)
	}
	if transition.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

// Helper function to check if a value is within a range
func withinRange(value, min, max float64) bool {
	return value >= min && value <= max
}

// Test helper to compare floats with tolerance
func floatsEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}
