package anomaly

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (d *StateAnomalyDetector) detectPodStateAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	const pendingThreshold = 5 * time.Minute
	var firstPending time.Time
	var lastPending time.Time

	for _, event := range input.AllEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		if event.Status == "Pending" || strings.Contains(strings.ToLower(event.EventType), "pending") {
			if firstPending.IsZero() {
				firstPending = event.Timestamp
			}
			lastPending = event.Timestamp
		}

		var resourceData map[string]interface{}
		if event.FullSnapshot != nil {
			resourceData = event.FullSnapshot
		} else if len(event.Data) > 0 {
			if err := json.Unmarshal(event.Data, &resourceData); err == nil {
			}
		}

		if resourceData != nil {
			if status, ok := resourceData["status"].(map[string]interface{}); ok {
				if phase, ok := status["phase"].(string); ok && strings.EqualFold(phase, "failed") {
					anomalies = append(anomalies, Anomaly{
						Node:      NodeFromGraphNode(input.Node),
						Category:  CategoryState,
						Type:      "PodFailed",
						Severity:  SeverityCritical,
						Timestamp: event.Timestamp,
						Summary:   "Pod is in Failed phase",
						Details: map[string]interface{}{
							"phase": phase,
						},
					})
				}

				if reason, ok := status["reason"].(string); ok {
					if strings.EqualFold(reason, "evicted") {
						anomalies = append(anomalies, Anomaly{
							Node:      NodeFromGraphNode(input.Node),
							Category:  CategoryState,
							Type:      "Evicted",
							Severity:  SeverityHigh,
							Timestamp: event.Timestamp,
							Summary:   "Pod has been evicted",
							Details: map[string]interface{}{
								"reason": reason,
							},
						})
					}
				}

				if conditions, ok := status["conditions"].([]interface{}); ok {
					for _, conditionInterface := range conditions {
						if condition, ok := conditionInterface.(map[string]interface{}); ok {
							condType, _ := condition["type"].(string)
							condStatus, _ := condition["status"].(string)
							condReason, _ := condition["reason"].(string)

							if condType == "PodScheduled" && condStatus == conditionFalse && condReason == "Unschedulable" {
								anomalies = append(anomalies, Anomaly{
									Node:      NodeFromGraphNode(input.Node),
									Category:  CategoryState,
									Type:      "Unschedulable",
									Severity:  SeverityHigh,
									Timestamp: event.Timestamp,
									Summary:   "Pod cannot be scheduled",
									Details: map[string]interface{}{
										"condition_type":   condType,
										"condition_status": condStatus,
										"condition_reason": condReason,
									},
								})
							}
						}
					}
				}
			}
		}
	}

	if !firstPending.IsZero() && !lastPending.IsZero() {
		duration := lastPending.Sub(firstPending)
		if duration >= pendingThreshold {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryState,
				Type:      "PodPending",
				Severity:  SeverityHigh,
				Timestamp: lastPending,
				Summary:   fmt.Sprintf("Pod has been Pending for %v", duration.Round(time.Second)),
				Details: map[string]interface{}{
					"duration_seconds": int64(duration.Seconds()),
				},
			})
		}
	}

	return anomalies
}

func (d *StateAnomalyDetector) detectDeploymentStateAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	var lastUnavailable time.Time
	var unavailableCount int32
	var hasConfigChange bool

	for _, event := range input.AllEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		if event.ConfigChanged {
			hasConfigChange = true
		}

		var resourceData map[string]interface{}
		if event.FullSnapshot != nil {
			resourceData = event.FullSnapshot
		} else if len(event.Data) > 0 {
			if err := json.Unmarshal(event.Data, &resourceData); err != nil {
				continue
			}
		}

		if resourceData != nil {
			if status, ok := resourceData["status"].(map[string]interface{}); ok {
				if unavailable, ok := status["unavailableReplicas"].(float64); ok && unavailable > 0 {
					if event.Timestamp.After(lastUnavailable) {
						lastUnavailable = event.Timestamp
						unavailableCount = int32(unavailable)
					}
				}

				if conditions, ok := status["conditions"].([]interface{}); ok {
					for _, conditionInterface := range conditions {
						if condition, ok := conditionInterface.(map[string]interface{}); ok {
							condType, _ := condition["type"].(string)
							condStatus, _ := condition["status"].(string)
							condReason, _ := condition["reason"].(string)

							if condType == "Progressing" && condStatus == conditionFalse && condReason == "ProgressDeadlineExceeded" {
								anomalies = append(anomalies, Anomaly{
									Node:      NodeFromGraphNode(input.Node),
									Category:  CategoryState,
									Type:      "RolloutStuck",
									Severity:  SeverityHigh,
									Timestamp: event.Timestamp,
									Summary:   "Deployment rollout has exceeded progress deadline",
									Details: map[string]interface{}{
										"condition_type":   condType,
										"condition_status": condStatus,
										"condition_reason": condReason,
									},
								})
							}
						}
					}
				}
			}
		}
	}

	if hasConfigChange && !lastUnavailable.IsZero() && unavailableCount > 0 {
		anomalies = append(anomalies, Anomaly{
			Node:      NodeFromGraphNode(input.Node),
			Category:  CategoryState,
			Type:      "RolloutStuck",
			Severity:  SeverityHigh,
			Timestamp: lastUnavailable,
			Summary:   fmt.Sprintf("Deployment rollout stuck with %d unavailable replicas", unavailableCount),
			Details: map[string]interface{}{
				"unavailable_replicas": unavailableCount,
			},
		})
	}

	return anomalies
}

func (d *StateAnomalyDetector) detectStatefulSetStateAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	var hasConfigChange bool
	var hasUpdateRevisionRollback bool

	for _, event := range input.AllEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		if event.ConfigChanged {
			hasConfigChange = true
		}

		var resourceData map[string]interface{}
		if event.FullSnapshot != nil {
			resourceData = event.FullSnapshot
		} else if len(event.Data) > 0 {
			if err := json.Unmarshal(event.Data, &resourceData); err != nil {
				continue
			}
		}

		if resourceData != nil {
			if status, ok := resourceData["status"].(map[string]interface{}); ok {
				currentRev, hasCurrentRev := status["currentRevision"].(string)
				updateRev, hasUpdateRev := status["updateRevision"].(string)

				if hasCurrentRev && hasUpdateRev && currentRev != "" && updateRev != "" && currentRev != updateRev {
					if hasConfigChange {
						hasUpdateRevisionRollback = true
					}
				}
			}
		}
	}

	if hasUpdateRevisionRollback {
		anomalies = append(anomalies, Anomaly{
			Node:      NodeFromGraphNode(input.Node),
			Category:  CategoryState,
			Type:      "UpdateRollback",
			Severity:  SeverityHigh,
			Timestamp: input.TimeWindow.End,
			Summary:   "StatefulSet update has been rolled back",
			Details:   map[string]interface{}{},
		})
	}

	return anomalies
}
