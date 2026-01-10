package analysis

import (
	"strings"
	"time"
)

// ============================================================================
// EVENT SIGNIFICANCE WEIGHTS
// ============================================================================
// These weights determine the relative importance of different factors
// when calculating event significance scores.

const (
	// EventWeightCausalSpine is the weight for events on the causal path.
	// Events on the causal spine are most relevant for root cause analysis.
	EventWeightCausalSpine = 0.15

	// EventWeightConfigChange is the weight for spec/config modifications.
	// Configuration changes are high-signal events.
	EventWeightConfigChange = 0.30

	// EventWeightStatusChange is the weight for status changes.
	// Status changes are less impactful but still relevant.
	EventWeightStatusChange = 0.05

	// EventWeightTemporalProximity is the weight for temporal closeness to failure.
	// Events closer to the failure are more likely to be relevant.
	EventWeightTemporalProximity = 0.20

	// EventWeightErrorCorrelation is the weight for error pattern matching.
	// Events that correlate with the error message are important.
	EventWeightErrorCorrelation = 0.10

	// EventWeightResourcePattern is the weight for detected resource-specific patterns.
	// Patterns like OOMKills, container crashes, and probe failures are critical indicators.
	EventWeightResourcePattern = 0.20

	// EventWeightEventType is the weight for event type significance.
	// DELETE and CREATE events have higher impact than UPDATE.
	EventWeightEventType = 0.05
)

// SignificantK8sEventReasons maps Kubernetes event reasons to their significance boosts.
// These represent well-known failure indicators in Kubernetes.
var SignificantK8sEventReasons = map[string]float64{
	// Critical failures
	"FailedScheduling":   0.5,
	"ImagePullBackOff":   0.5,
	"CrashLoopBackOff":   0.5,
	"OOMKilled":          0.5,
	"Failed":             0.4,
	"Unhealthy":          0.4,
	"BackOff":            0.4,
	"FailedMount":        0.4,
	"FailedAttachVolume": 0.4,
	"NodeNotReady":       0.4,
	"NetworkNotReady":    0.4,

	// Resource issues
	"Evicted":             0.3,
	"FreeDiskSpaceFailed": 0.3,
	"InsufficientMemory":  0.3,
	"InsufficientCPU":     0.3,

	// Normal operational events with lower significance
	"Killing":           0.2,
	"Preempting":        0.2,
	"SuccessfulCreate":  0.0,
	"Scheduled":         0.0,
	"Pulled":            0.0,
	"Started":           0.0,
	"Created":           0.0,
	"ScalingReplicaSet": 0.0,
	"SuccessfulDelete":  0.0,
	"SuccessfulUpdate":  0.0,
}

// CalculateChangeEventSignificance scores a change event based on multiple factors.
// The score is a weighted combination of:
// - Causal spine position (15%): Is this event on the causal path?
// - Config change (30%): Does this event modify the resource spec?
// - Temporal proximity (20%): How close is this event to the failure time?
// - Resource patterns (20%): Does this event contain significant resource-specific patterns?
// - Error correlation (10%): Does this event correlate with error patterns?
// - Event type (5%): Is this a DELETE/CREATE vs UPDATE event?
func CalculateChangeEventSignificance(
	event *ChangeEventInfo,
	resourceKind string,
	isOnCausalSpine bool,
	failureTime time.Time,
	errorPatterns []string,
) *EventSignificance {
	score := 0.0
	reasons := []string{}

	// Factor 1: Causal spine position (35%)
	if isOnCausalSpine {
		score += EventWeightCausalSpine
		reasons = append(reasons, "on causal path")
	}

	// Factor 2: Config change (30%)
	if event.ConfigChanged {
		score += EventWeightConfigChange
		reasons = append(reasons, "spec changed")
	} else if event.StatusChanged {
		// Status changes are less impactful but still relevant
		score += EventWeightStatusChange
		reasons = append(reasons, "status changed")
	}

	// Factor 3: Resource-specific patterns (20%)
	// Detect significant patterns like OOMKills, container crashes, probe failures
	patterns := DetectResourcePatterns(event, resourceKind)
	if len(patterns) > 0 {
		// Use the highest severity pattern
		highestPattern := GetHighestSeverityPattern(patterns)
		if highestPattern != nil {
			// Scale pattern severity to our weight
			patternScore := EventWeightResourcePattern * highestPattern.Severity
			score += patternScore
			reasons = append(reasons, highestPattern.Description)
		}
	}

	// Factor 4: Temporal proximity (20%)
	if !failureTime.IsZero() {
		timeDelta := failureTime.Sub(event.Timestamp)
		if timeDelta < 0 {
			timeDelta = -timeDelta // Handle future events (shouldn't happen but be safe)
		}

		if timeDelta < 5*time.Minute {
			score += EventWeightTemporalProximity
			reasons = append(reasons, "within 5min of failure")
		} else if timeDelta < 30*time.Minute {
			score += EventWeightTemporalProximity * 0.5
			reasons = append(reasons, "within 30min of failure")
		} else if timeDelta < time.Hour {
			score += EventWeightTemporalProximity * 0.2
		}
	}

	// Factor 5: Error correlation (10%)
	if len(errorPatterns) > 0 && event.Description != "" {
		descLower := strings.ToLower(event.Description)
		for _, pattern := range errorPatterns {
			if strings.Contains(descLower, strings.ToLower(pattern)) {
				score += EventWeightErrorCorrelation
				reasons = append(reasons, "matches error pattern")
				break
			}
		}
	}

	// Factor 6: Event type significance (5%)
	switch event.EventType {
	case "DELETE":
		score += EventWeightEventType
		reasons = append(reasons, "resource deleted")
	case "CREATE":
		score += EventWeightEventType * 0.6
		reasons = append(reasons, "resource created")
	}

	// Normalize score to [0, 1]
	if score > 1.0 {
		score = 1.0
	}

	return &EventSignificance{
		Score:   score,
		Reasons: reasons,
	}
}

// CalculateK8sEventSignificance scores a Kubernetes event based on its type,
// reason, and relation to the failure.
func CalculateK8sEventSignificance(
	event *K8sEventInfo,
	isOnCausalSpine bool,
	failureTime time.Time,
) *EventSignificance {
	score := 0.0
	reasons := []string{}

	// Warning events are more significant than Normal events
	if event.Type == "Warning" {
		score += 0.2
		reasons = append(reasons, "warning event")
	} else if event.Type == "Error" {
		score += 0.3
		reasons = append(reasons, "error event")
	}

	// Check for known significant reasons
	if boost, ok := SignificantK8sEventReasons[event.Reason]; ok {
		score += boost
		reasons = append(reasons, event.Reason)
	}

	// Causal spine boost
	if isOnCausalSpine {
		score += 0.15
		reasons = append(reasons, "on causal path")
	}

	// Temporal proximity boost
	if !failureTime.IsZero() {
		timeDelta := failureTime.Sub(event.Timestamp)
		if timeDelta < 0 {
			timeDelta = -timeDelta
		}

		if timeDelta < 5*time.Minute {
			score += 0.1
		} else if timeDelta < 30*time.Minute {
			score += 0.05
		}
	}

	// High event count indicates persistent issues
	if event.Count > 5 {
		score += 0.1
		reasons = append(reasons, "repeated event")
	}

	// Normalize score to [0, 1]
	if score > 1.0 {
		score = 1.0
	}

	return &EventSignificance{
		Score:   score,
		Reasons: reasons,
	}
}

// ExtractErrorPatterns extracts keywords from an error message that can be used
// for correlation with events.
func ExtractErrorPatterns(errorMessage string) []string {
	if errorMessage == "" {
		return nil
	}

	// Common error keywords to look for in events
	keywords := []string{
		"image", "pull", "config", "configmap", "secret", "volume", "mount",
		"permission", "timeout", "connection", "refused", "denied",
		"memory", "cpu", "oom", "killed", "crash", "restart",
		"unhealthy", "probe", "liveness", "readiness",
		"schedule", "node", "taint", "affinity",
	}

	errorLower := strings.ToLower(errorMessage)
	var patterns []string

	for _, kw := range keywords {
		if strings.Contains(errorLower, kw) {
			patterns = append(patterns, kw)
		}
	}

	return patterns
}

// ScoreEvents applies significance scoring to all events in a graph node.
// This is typically called during graph building when the format is "diff".
func ScoreEvents(
	node *GraphNode,
	isOnCausalSpine bool,
	failureTime time.Time,
	errorPatterns []string,
) {
	resourceKind := node.Resource.Kind

	// Score change events
	if node.ChangeEvent != nil {
		node.ChangeEvent.Significance = CalculateChangeEventSignificance(
			node.ChangeEvent, resourceKind, isOnCausalSpine, failureTime, errorPatterns,
		)
	}

	for i := range node.AllEvents {
		node.AllEvents[i].Significance = CalculateChangeEventSignificance(
			&node.AllEvents[i], resourceKind, isOnCausalSpine, failureTime, errorPatterns,
		)
	}

	// Score K8s events
	for i := range node.K8sEvents {
		node.K8sEvents[i].Significance = CalculateK8sEventSignificance(
			&node.K8sEvents[i], isOnCausalSpine, failureTime,
		)
	}
}
