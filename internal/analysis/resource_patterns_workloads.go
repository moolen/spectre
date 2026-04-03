package analysis

import (
	"fmt"
	"strings"
)

// detectDeploymentPatterns detects patterns specific to Deployment resources.
func detectDeploymentPatterns(event *ChangeEventInfo) []DetectedPattern {
	var patterns []DetectedPattern

	for _, diff := range event.Diff {
		if diff.Path == "status.unavailableReplicas" {
			if unavailable, ok := diff.NewValue.(float64); ok && unavailable > 0 {
				patterns = append(patterns, DetectedPattern{
					Type:        PatternReplicasUnavailable,
					Severity:    0.35,
					Description: fmt.Sprintf("Deployment has %d unavailable replicas", int(unavailable)),
					Path:        diff.Path,
				})
			}
		}

		if strings.Contains(diff.Path, "status.conditions") && strings.Contains(diff.Path, "reason") {
			if reason, ok := diff.NewValue.(string); ok && strings.Contains(strings.ToLower(reason), "progressdeadlineexceeded") {
				patterns = append(patterns, DetectedPattern{
					Type:        PatternRolloutStalled,
					Severity:    0.40,
					Description: "Deployment rollout stalled: progress deadline exceeded",
					Path:        diff.Path,
				})
			}
		}
	}

	return patterns
}

// detectReplicaSetPatterns detects patterns specific to ReplicaSet resources.
func detectReplicaSetPatterns(event *ChangeEventInfo) []DetectedPattern {
	var patterns []DetectedPattern

	for _, diff := range event.Diff {
		if diff.Path == "status.readyReplicas" {
			if ready, ok := diff.NewValue.(float64); ok && ready == 0 {
				patterns = append(patterns, DetectedPattern{
					Type:        PatternReplicasUnavailable,
					Severity:    0.30,
					Description: "ReplicaSet has 0 ready replicas",
					Path:        diff.Path,
				})
			}
		}
	}

	return patterns
}

// detectStatefulSetPatterns detects patterns specific to StatefulSet resources.
func detectStatefulSetPatterns(event *ChangeEventInfo) []DetectedPattern {
	var patterns []DetectedPattern

	for _, diff := range event.Diff {
		if diff.Path == "status.readyReplicas" {
			if ready, ok := diff.NewValue.(float64); ok && ready == 0 {
				patterns = append(patterns, DetectedPattern{
					Type:        PatternReplicasUnavailable,
					Severity:    0.35,
					Description: "StatefulSet has 0 ready replicas",
					Path:        diff.Path,
				})
			}
		}

		if strings.Contains(diff.Path, "status.conditions") && strings.Contains(diff.Path, "reason") {
			if reason, ok := diff.NewValue.(string); ok {
				reasonLower := strings.ToLower(reason)
				if strings.Contains(reasonLower, "blocked") || strings.Contains(reasonLower, "failed") {
					patterns = append(patterns, DetectedPattern{
						Type:        PatternRolloutStalled,
						Severity:    0.40,
						Description: fmt.Sprintf("StatefulSet update issue: %s", reason),
						Path:        diff.Path,
					})
				}
			}
		}
	}

	return patterns
}
