package anomaly

import (
	"fmt"
	"strings"
)

// FrequencyAnomalyDetector detects high-frequency patterns indicating instability
type FrequencyAnomalyDetector struct{}

// NewFrequencyAnomalyDetector creates a new frequency anomaly detector
func NewFrequencyAnomalyDetector() *FrequencyAnomalyDetector {
	return &FrequencyAnomalyDetector{}
}

// Detect analyzes event frequency patterns for anomalies
func (d *FrequencyAnomalyDetector) Detect(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	// Filter events within time window
	eventsInWindow := 0
	for _, event := range input.AllEvents {
		if !event.Timestamp.Before(input.TimeWindow.Start) && !event.Timestamp.After(input.TimeWindow.End) {
			eventsInWindow++
		}
	}

	// Count status transitions
	statusTransitions := d.countStatusTransitions(input)

	// Detect flapping (status oscillation)
	if statusTransitions >= 3 {
		anomalies = append(anomalies, Anomaly{
			Node:      NodeFromGraphNode(input.Node),
			Category:  CategoryFrequency,
			Type:      "FlappingState",
			Severity:  SeverityHigh,
			Timestamp: input.TimeWindow.End,
			Summary:   fmt.Sprintf("Resource status changed %d times", statusTransitions),
			Details: map[string]interface{}{
				"transition_count": statusTransitions,
			},
		})
	}

	// Pod-specific: restart count
	if input.Node.Resource.Kind == "Pod" {
		restartCount := d.extractRestartCount(input)
		if restartCount > 3 {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryFrequency,
				Type:      "HighRestartCount",
				Severity:  SeverityHigh,
				Timestamp: input.TimeWindow.End,
				Summary:   fmt.Sprintf("Pod restarted %d times", restartCount),
				Details: map[string]interface{}{
					"restart_count": restartCount,
				},
			})
		}
	}

	return anomalies
}

func (d *FrequencyAnomalyDetector) countStatusTransitions(input DetectorInput) int {
	transitions := 0
	lastStatus := ""

	for _, event := range input.AllEvents {
		// Skip events outside time window
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		if event.Status != "" && event.Status != lastStatus {
			if lastStatus != "" {
				transitions++
			}
			lastStatus = event.Status
		}
	}

	return transitions
}

func (d *FrequencyAnomalyDetector) extractRestartCount(input DetectorInput) int {
	maxRestarts := 0

	for _, event := range input.AllEvents {
		// Skip events outside time window
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		// Look for restart count in diffs
		for _, diff := range event.Diff {
			if strings.Contains(diff.Path, "restartCount") {
				if count, ok := diff.NewValue.(float64); ok {
					if int(count) > maxRestarts {
						maxRestarts = int(count)
					}
				}
			}
		}

		// Also check in full snapshot for containerStatuses
		if event.FullSnapshot != nil {
			if status, ok := event.FullSnapshot["status"].(map[string]interface{}); ok {
				maxRestarts = max(maxRestarts, d.extractRestartCountFromStatus(status, "containerStatuses"))
				maxRestarts = max(maxRestarts, d.extractRestartCountFromStatus(status, "initContainerStatuses"))
			}
		}
	}

	return maxRestarts
}

func (d *FrequencyAnomalyDetector) extractRestartCountFromStatus(status map[string]interface{}, field string) int {
	maxRestarts := 0

	if containerStatuses, ok := status[field].([]interface{}); ok {
		for _, csInterface := range containerStatuses {
			if cs, ok := csInterface.(map[string]interface{}); ok {
				if restartCount, ok := cs["restartCount"].(float64); ok {
					if int(restartCount) > maxRestarts {
						maxRestarts = int(restartCount)
					}
				}
			}
		}
	}

	return maxRestarts
}

func (d *FrequencyAnomalyDetector) isControllerKind(kind string) bool {
	controllerKinds := []string{
		"Deployment", "StatefulSet", "DaemonSet", "ReplicaSet",
		"HelmRelease", "Kustomization", "Application",
	}

	for _, ck := range controllerKinds {
		if kind == ck {
			return true
		}
	}

	return false
}

func (d *FrequencyAnomalyDetector) countReconciles(input DetectorInput) int {
	reconcileCount := 0

	for _, event := range input.AllEvents {
		// Skip events outside time window
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		// Count UPDATE events as potential reconciles
		if event.EventType == "UPDATE" {
			reconcileCount++
		}
	}

	return reconcileCount
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
