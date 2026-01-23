package grafana

import (
	"math"
	"sort"
	"time"

	"gonum.org/v1/gonum/stat"
)

// StateTransition represents a single state change for an alert
type StateTransition struct {
	FromState string    // "normal", "pending", "firing"
	ToState   string    // "normal", "pending", "firing"
	Timestamp time.Time // RFC3339 timestamp from graph edge
}

// ComputeFlappinessScore calculates a normalized flappiness score (0.0-1.0) for an alert
// based on state transitions within a time window. Higher scores indicate more flapping.
//
// The score combines two factors:
// - Frequency: how many transitions occurred relative to maximum possible
// - Duration penalty: preference for long-lived states over short-lived states
//
// Parameters:
//   - transitions: slice of state transitions (will be filtered to window)
//   - windowSize: time window to analyze (e.g., 6 hours)
//   - currentTime: end of analysis window
//
// Returns:
//   - score between 0.0 (stable) and 1.0 (extremely flapping)
func ComputeFlappinessScore(transitions []StateTransition, windowSize time.Duration, currentTime time.Time) float64 {
	// Filter transitions to window
	windowStart := currentTime.Add(-windowSize)
	var windowTransitions []StateTransition
	for _, t := range transitions {
		if t.Timestamp.After(windowStart) && !t.Timestamp.After(currentTime) {
			windowTransitions = append(windowTransitions, t)
		}
	}

	// Empty or stable (0-1 transitions) gets 0.0 score
	if len(windowTransitions) == 0 {
		return 0.0
	}

	// Sort transitions chronologically
	sort.Slice(windowTransitions, func(i, j int) bool {
		return windowTransitions[i].Timestamp.Before(windowTransitions[j].Timestamp)
	})

	// Calculate frequency component
	// Use a sigmoid-like scaling to make scores more sensitive
	// 5 transitions in 6h should score ~0.5, 10+ should approach 1.0
	transitionCount := float64(len(windowTransitions))

	// Base frequency score (exponential scaling for sensitivity)
	// Formula: 1 - exp(-k * count) where k controls sensitivity
	k := 0.15 // Tuned so 5 transitions ≈ 0.5, 10 transitions ≈ 0.8
	frequencyScore := 1.0 - math.Exp(-k*transitionCount)

	// Calculate duration penalty component
	// Compute average state duration
	var durations []float64
	for i := 0; i < len(windowTransitions); i++ {
		var duration time.Duration
		if i < len(windowTransitions)-1 {
			// Duration until next transition
			duration = windowTransitions[i+1].Timestamp.Sub(windowTransitions[i].Timestamp)
		} else {
			// Last transition: duration until current time
			duration = currentTime.Sub(windowTransitions[i].Timestamp)
		}
		durations = append(durations, float64(duration))
	}

	avgStateDuration := stat.Mean(durations, nil)

	// Duration penalty: penalize short-lived states
	// avgDuration / windowSize gives ratio (0 = very short, 1 = full window)
	// We want short durations to increase score
	durationRatio := avgStateDuration / float64(windowSize)

	// Apply multiplier based on duration
	// Short durations (< 10% of window) get 1.3x multiplier
	// Long durations (> 50% of window) get 0.7x multiplier
	var durationMultiplier float64
	if durationRatio < 0.1 {
		durationMultiplier = 1.3
	} else if durationRatio < 0.3 {
		durationMultiplier = 1.1
	} else if durationRatio < 0.5 {
		durationMultiplier = 1.0
	} else {
		durationMultiplier = 0.8
	}

	// Combined score with duration multiplier
	score := frequencyScore * durationMultiplier

	// Cap at 1.0 (normalize extreme cases)
	return math.Min(1.0, score)
}
