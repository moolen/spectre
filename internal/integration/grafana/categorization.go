package grafana

import (
	"sort"
	"time"
)

// AlertCategories represents multi-label categorization for an alert
// Onset categories are time-based (when alert started)
// Pattern categories are behavior-based (how alert behaves)
type AlertCategories struct {
	Onset   []string // "new", "recent", "persistent", "chronic"
	Pattern []string // "stable-firing", "stable-normal", "flapping", "trending-worse", "trending-better"
}

// CategorizeAlert performs multi-label categorization of an alert based on
// state transition history and flappiness score.
//
// Onset categorization (time-based):
//   - "new": first firing < 1h ago
//   - "recent": first firing < 24h ago
//   - "persistent": first firing < 7d ago
//   - "chronic": first firing >= 7d ago AND >80% time firing
//   - "stable-normal": never fired
//
// Pattern categorization (behavior-based):
//   - "flapping": flappinessScore > 0.7
//   - "trending-worse": firing % increased >20% in last 1h vs prior 6h
//   - "trending-better": firing % decreased >20% in last 1h vs prior 6h
//   - "stable-firing": currently firing, not flapping, no trend
//   - "stable-normal": currently normal, not flapping, no trend
//
// Uses LOCF (Last Observation Carried Forward) interpolation to compute
// state durations for chronic threshold and trend analysis.
//
// Parameters:
//   - transitions: historical state transitions (should be sorted chronologically)
//   - currentTime: reference time for analysis
//   - flappinessScore: score from ComputeFlappinessScore (0.0-1.0)
//
// Returns:
//   - AlertCategories with onset and pattern labels
func CategorizeAlert(
	transitions []StateTransition,
	currentTime time.Time,
	flappinessScore float64,
) AlertCategories {
	// Handle empty transitions
	if len(transitions) == 0 {
		return AlertCategories{
			Onset:   []string{"stable-normal"},
			Pattern: []string{"stable-normal"},
		}
	}

	// Sort transitions chronologically (defensive)
	sortedTransitions := make([]StateTransition, len(transitions))
	copy(sortedTransitions, transitions)
	sort.Slice(sortedTransitions, func(i, j int) bool {
		return sortedTransitions[i].Timestamp.Before(sortedTransitions[j].Timestamp)
	})

	// Compute onset categories
	onsetCategories := categorizeOnset(sortedTransitions, currentTime)

	// Compute pattern categories
	patternCategories := categorizePattern(sortedTransitions, currentTime, flappinessScore)

	return AlertCategories{
		Onset:   onsetCategories,
		Pattern: patternCategories,
	}
}

// categorizeOnset determines onset categories based on when alert first fired
func categorizeOnset(transitions []StateTransition, currentTime time.Time) []string {
	// Find first firing state
	var firstFiringTime *time.Time
	for _, t := range transitions {
		if t.ToState == "firing" {
			firstFiringTime = &t.Timestamp
			break
		}
	}

	// Never fired
	if firstFiringTime == nil {
		return []string{"stable-normal"}
	}

	// Time since first firing
	timeSinceFiring := currentTime.Sub(*firstFiringTime)

	// Apply time-based thresholds
	if timeSinceFiring < 1*time.Hour {
		return []string{"new"}
	}

	if timeSinceFiring < 24*time.Hour {
		return []string{"recent"}
	}

	if timeSinceFiring < 7*24*time.Hour {
		return []string{"persistent"}
	}

	// Check chronic threshold (>80% firing over 7 days)
	sevenDaysAgo := currentTime.Add(-7 * 24 * time.Hour)
	durations := computeStateDurations(transitions, sevenDaysAgo, currentTime)
	totalDuration := 7 * 24 * time.Hour
	firingDuration := durations["firing"]

	firingRatio := float64(firingDuration) / float64(totalDuration)
	if firingRatio > 0.8 {
		return []string{"chronic"}
	}

	// >= 7d but not chronic threshold
	return []string{"persistent"}
}

// categorizePattern determines pattern categories based on behavior
func categorizePattern(transitions []StateTransition, currentTime time.Time, flappinessScore float64) []string {
	patterns := make([]string, 0, 2)

	// Check flapping first (independent of other patterns)
	if flappinessScore > 0.7 {
		patterns = append(patterns, "flapping")
		return patterns // Flapping overrides other pattern categories
	}

	// Insufficient data for trend analysis (need at least 2h history)
	if len(transitions) == 0 {
		return []string{"stable-normal"}
	}

	earliestTime := transitions[0].Timestamp
	availableHistory := currentTime.Sub(earliestTime)
	if availableHistory < 2*time.Hour {
		// Not enough history for trend - use stable-* based on current state
		currentState := getCurrentState(transitions, currentTime)
		if currentState == "firing" {
			return []string{"stable-firing"}
		}
		return []string{"stable-normal"}
	}

	// Compute trend: compare last 1h to prior 6h
	oneHourAgo := currentTime.Add(-1 * time.Hour)
	sevenHoursAgo := currentTime.Add(-7 * time.Hour)

	// Recent window (last 1h)
	recentDurations := computeStateDurations(transitions, oneHourAgo, currentTime)
	recentTotal := 1 * time.Hour
	recentFiringPercent := float64(recentDurations["firing"]) / float64(recentTotal)

	// Prior window (6h before that)
	priorDurations := computeStateDurations(transitions, sevenHoursAgo, oneHourAgo)
	priorTotal := 6 * time.Hour
	priorFiringPercent := float64(priorDurations["firing"]) / float64(priorTotal)

	// Compute change in firing percentage
	change := recentFiringPercent - priorFiringPercent

	// Threshold: >20% change indicates trend
	if change > 0.2 {
		patterns = append(patterns, "trending-worse")
		return patterns
	}

	if change < -0.2 {
		patterns = append(patterns, "trending-better")
		return patterns
	}

	// No flapping, no trend - use stable-* based on current state
	currentState := getCurrentState(transitions, currentTime)
	if currentState == "firing" {
		patterns = append(patterns, "stable-firing")
	} else {
		patterns = append(patterns, "stable-normal")
	}

	return patterns
}

// computeStateDurations computes time spent in each state within a time window
// using LOCF (Last Observation Carried Forward) interpolation.
//
// This fills gaps by carrying forward the last known state until the next transition.
//
// Parameters:
//   - transitions: all state transitions (may span beyond window)
//   - windowStart: start of analysis window
//   - windowEnd: end of analysis window
//
// Returns:
//   - map of state -> duration spent in that state within window
func computeStateDurations(transitions []StateTransition, windowStart, windowEnd time.Time) map[string]time.Duration {
	durations := make(map[string]time.Duration)

	if len(transitions) == 0 {
		return durations
	}

	// Find initial state for window (LOCF from before window if available)
	var currentState string = "normal" // Default if no prior history
	var currentTime time.Time = windowStart

	// Find last transition before window to establish initial state
	for i, t := range transitions {
		if t.Timestamp.Before(windowStart) {
			currentState = t.ToState
		} else if !t.Timestamp.After(windowEnd) {
			// First transition in window
			if i > 0 {
				// Use previous transition's ToState as initial state
				currentState = transitions[i-1].ToState
			}
			break
		}
	}

	// Process transitions within window
	for _, t := range transitions {
		// Skip transitions before window
		if t.Timestamp.Before(windowStart) {
			continue
		}

		// Stop at transitions after window
		if t.Timestamp.After(windowEnd) {
			break
		}

		// Add duration in current state until this transition
		if t.Timestamp.After(currentTime) {
			duration := t.Timestamp.Sub(currentTime)
			durations[currentState] += duration
			currentTime = t.Timestamp
		}

		// Update state
		currentState = t.ToState
	}

	// Add remaining time in final state until window end
	if currentTime.Before(windowEnd) {
		duration := windowEnd.Sub(currentTime)
		durations[currentState] += duration
	}

	return durations
}

// getCurrentState determines the current alert state based on most recent transition
func getCurrentState(transitions []StateTransition, currentTime time.Time) string {
	if len(transitions) == 0 {
		return "normal"
	}

	// Find most recent transition at or before currentTime
	var currentState string = "normal"
	for _, t := range transitions {
		if !t.Timestamp.After(currentTime) {
			currentState = t.ToState
		} else {
			break
		}
	}

	return currentState
}
