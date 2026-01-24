package grafana

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCategorizeAlert_Empty(t *testing.T) {
	now := time.Now()
	categories := CategorizeAlert([]StateTransition{}, now, 0.0)

	assert.Equal(t, []string{"stable-normal"}, categories.Onset)
	assert.Equal(t, []string{"stable-normal"}, categories.Pattern)
}

func TestCategorizeAlert_New(t *testing.T) {
	now := time.Now()
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-30 * time.Minute)},
	}

	categories := CategorizeAlert(transitions, now, 0.0)

	assert.Equal(t, []string{"new"}, categories.Onset)
	assert.Contains(t, categories.Pattern, "stable-firing")
}

func TestCategorizeAlert_Recent(t *testing.T) {
	now := time.Now()
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-12 * time.Hour)},
	}

	categories := CategorizeAlert(transitions, now, 0.0)

	assert.Equal(t, []string{"recent"}, categories.Onset)
	assert.Contains(t, categories.Pattern, "stable-firing")
}

func TestCategorizeAlert_Persistent(t *testing.T) {
	now := time.Now()
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-3 * 24 * time.Hour)},
	}

	categories := CategorizeAlert(transitions, now, 0.0)

	assert.Equal(t, []string{"persistent"}, categories.Onset)
	assert.Contains(t, categories.Pattern, "stable-firing")
}

func TestCategorizeAlert_Chronic(t *testing.T) {
	now := time.Now()
	// Alert fired 8 days ago and has been firing 90% of the time
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-8 * 24 * time.Hour)},
		// Brief normal period
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-7*24*time.Hour - 12*time.Hour)},
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-7 * 24 * time.Hour)},
	}

	categories := CategorizeAlert(transitions, now, 0.0)

	assert.Equal(t, []string{"chronic"}, categories.Onset)
	assert.Contains(t, categories.Pattern, "stable-firing")
}

func TestCategorizeAlert_PersistentNotChronic(t *testing.T) {
	now := time.Now()
	// Alert fired 8 days ago but only 50% firing (below chronic threshold)
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-8 * 24 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-4 * 24 * time.Hour)},
	}

	categories := CategorizeAlert(transitions, now, 0.0)

	assert.Equal(t, []string{"persistent"}, categories.Onset)
	assert.Contains(t, categories.Pattern, "stable-normal")
}

func TestCategorizeAlert_Flapping(t *testing.T) {
	now := time.Now()
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-2 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-90 * time.Minute)},
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-80 * time.Minute)},
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-70 * time.Minute)},
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-60 * time.Minute)},
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-50 * time.Minute)},
	}

	categories := CategorizeAlert(transitions, now, 0.8) // High flappiness score

	assert.Equal(t, []string{"recent"}, categories.Onset)
	assert.Equal(t, []string{"flapping"}, categories.Pattern)
}

func TestCategorizeAlert_TrendingWorse(t *testing.T) {
	now := time.Now()
	// Prior 6h: mostly normal
	// Last 1h: mostly firing (trending worse)
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-3 * 24 * time.Hour)}, // 3 days ago (persistent)
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-7 * time.Hour)},
		// Long normal period
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-1 * time.Hour)},
		// Still firing
	}

	categories := CategorizeAlert(transitions, now, 0.0)

	assert.Equal(t, []string{"persistent"}, categories.Onset)
	assert.Equal(t, []string{"trending-worse"}, categories.Pattern)
}

func TestCategorizeAlert_TrendingBetter(t *testing.T) {
	now := time.Now()
	// Prior 6h: mostly firing
	// Last 1h: mostly normal (trending better)
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-3 * 24 * time.Hour)}, // 3 days ago (persistent)
		// Long firing period
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-1 * time.Hour)},
		// Now normal
	}

	categories := CategorizeAlert(transitions, now, 0.0)

	assert.Equal(t, []string{"persistent"}, categories.Onset)
	assert.Equal(t, []string{"trending-better"}, categories.Pattern)
}

func TestCategorizeAlert_StableFiring(t *testing.T) {
	now := time.Now()
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-3 * 24 * time.Hour)},
		// Stable firing for 3 days
	}

	categories := CategorizeAlert(transitions, now, 0.0)

	assert.Equal(t, []string{"persistent"}, categories.Onset)
	assert.Equal(t, []string{"stable-firing"}, categories.Pattern)
}

func TestCategorizeAlert_StableNormal(t *testing.T) {
	now := time.Now()
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-3 * 24 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-2 * 24 * time.Hour)},
		// Stable normal for 2 days
	}

	categories := CategorizeAlert(transitions, now, 0.0)

	assert.Equal(t, []string{"persistent"}, categories.Onset)
	assert.Equal(t, []string{"stable-normal"}, categories.Pattern)
}

func TestCategorizeAlert_MultiLabel_ChronicAndFlapping(t *testing.T) {
	now := time.Now()
	// Alert is chronic (old + high firing %) AND flapping
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-8 * 24 * time.Hour)},
		// Mostly firing but with some flapping
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-7*24*time.Hour - 1*time.Hour)},
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-7 * 24 * time.Hour)},
	}

	categories := CategorizeAlert(transitions, now, 0.8) // High flappiness

	assert.Equal(t, []string{"chronic"}, categories.Onset)
	assert.Equal(t, []string{"flapping"}, categories.Pattern)
}

func TestCategorizeAlert_InsufficientHistoryForTrend(t *testing.T) {
	now := time.Now()
	// Only 30min of history - not enough for trend
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-30 * time.Minute)},
	}

	categories := CategorizeAlert(transitions, now, 0.0)

	assert.Equal(t, []string{"new"}, categories.Onset)
	assert.Equal(t, []string{"stable-firing"}, categories.Pattern) // No trend, use stable-*
}

func TestComputeStateDurations_Simple(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-1 * time.Hour)
	windowEnd := now

	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-30 * time.Minute)},
	}

	durations := computeStateDurations(transitions, windowStart, windowEnd)

	// 30 minutes normal, 30 minutes firing
	assert.InDelta(t, 30*time.Minute, durations["normal"], float64(time.Second))
	assert.InDelta(t, 30*time.Minute, durations["firing"], float64(time.Second))
}

func TestComputeStateDurations_LOCF(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-2 * time.Hour)
	windowEnd := now

	// Transition before window establishes initial state
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-3 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-1 * time.Hour)},
	}

	durations := computeStateDurations(transitions, windowStart, windowEnd)

	// LOCF: firing from windowStart until transition at -1h (1 hour)
	// Then normal from -1h until windowEnd (1 hour)
	assert.InDelta(t, 1*time.Hour, durations["firing"], float64(time.Second))
	assert.InDelta(t, 1*time.Hour, durations["normal"], float64(time.Second))
}

func TestComputeStateDurations_Empty(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-1 * time.Hour)
	windowEnd := now

	durations := computeStateDurations([]StateTransition{}, windowStart, windowEnd)

	assert.Empty(t, durations)
}

func TestGetCurrentState_Default(t *testing.T) {
	now := time.Now()
	state := getCurrentState([]StateTransition{}, now)
	assert.Equal(t, "normal", state)
}

func TestGetCurrentState_MostRecent(t *testing.T) {
	now := time.Now()
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-2 * time.Hour)},
		{FromState: "firing", ToState: "pending", Timestamp: now.Add(-1 * time.Hour)},
		{FromState: "pending", ToState: "normal", Timestamp: now.Add(-30 * time.Minute)},
	}

	state := getCurrentState(transitions, now)
	assert.Equal(t, "normal", state)
}

func TestGetCurrentState_IgnoreFuture(t *testing.T) {
	now := time.Now()
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-1 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(1 * time.Hour)}, // Future
	}

	state := getCurrentState(transitions, now)
	assert.Equal(t, "firing", state) // Should not consider future transition
}
