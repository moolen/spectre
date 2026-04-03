package anomaly

import (
	"encoding/json"
	"fmt"
)

func (d *StateAnomalyDetector) detectConfigResourceStateAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	for _, event := range input.AllEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		if event.EventType == "DELETE" {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryState,
				Type:      "Deleted",
				Severity:  SeverityHigh,
				Timestamp: event.Timestamp,
				Summary:   fmt.Sprintf("%s has been deleted", input.Node.Resource.Kind),
				Details: map[string]interface{}{
					"event_type": event.EventType,
				},
			})
		}
	}

	return anomalies
}

// detectHelmReleaseStateAnomalies detects Flux HelmRelease failure states.
func (d *StateAnomalyDetector) detectHelmReleaseStateAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	for _, event := range input.AllEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		var resourceData map[string]interface{}
		if event.FullSnapshot != nil {
			resourceData = event.FullSnapshot
		} else if len(event.Data) > 0 {
			if err := json.Unmarshal(event.Data, &resourceData); err != nil {
				continue
			}
		}

		if resourceData == nil {
			continue
		}

		status, ok := resourceData["status"].(map[string]interface{})
		if !ok {
			continue
		}

		conditions, ok := status["conditions"].([]interface{})
		if !ok {
			continue
		}

		for _, condInterface := range conditions {
			cond, ok := condInterface.(map[string]interface{})
			if !ok {
				continue
			}

			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			condReason, _ := cond["reason"].(string)
			condMessage, _ := cond["message"].(string)

			if condType == conditionReady && condStatus == conditionFalse {
				anomalies = append(anomalies, Anomaly{
					Node:      NodeFromGraphNode(input.Node),
					Category:  CategoryState,
					Type:      "HelmReleaseFailed",
					Severity:  SeverityCritical,
					Timestamp: event.Timestamp,
					Summary:   fmt.Sprintf("HelmRelease is not ready: %s", condReason),
					Details: map[string]interface{}{
						"condition_type":    condType,
						"condition_status":  condStatus,
						"condition_reason":  condReason,
						"condition_message": condMessage,
					},
				})
			}

			if condType == "Released" && condStatus == conditionFalse {
				anomalies = append(anomalies, Anomaly{
					Node:      NodeFromGraphNode(input.Node),
					Category:  CategoryState,
					Type:      "HelmReleaseFailed",
					Severity:  SeverityCritical,
					Timestamp: event.Timestamp,
					Summary:   fmt.Sprintf("HelmRelease failed: %s", condReason),
					Details: map[string]interface{}{
						"condition_type":    condType,
						"condition_status":  condStatus,
						"condition_reason":  condReason,
						"condition_message": condMessage,
					},
				})
			}
		}
	}

	return anomalies
}

// detectKustomizationStateAnomalies detects Flux Kustomization failure states.
func (d *StateAnomalyDetector) detectKustomizationStateAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	for _, event := range input.AllEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		var resourceData map[string]interface{}
		if event.FullSnapshot != nil {
			resourceData = event.FullSnapshot
		} else if len(event.Data) > 0 {
			if err := json.Unmarshal(event.Data, &resourceData); err != nil {
				continue
			}
		}

		if resourceData == nil {
			continue
		}

		status, ok := resourceData["status"].(map[string]interface{})
		if !ok {
			continue
		}

		conditions, ok := status["conditions"].([]interface{})
		if !ok {
			continue
		}

		for _, condInterface := range conditions {
			cond, ok := condInterface.(map[string]interface{})
			if !ok {
				continue
			}

			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			condReason, _ := cond["reason"].(string)
			condMessage, _ := cond["message"].(string)

			if condType == conditionReady && condStatus == conditionFalse {
				anomalies = append(anomalies, Anomaly{
					Node:      NodeFromGraphNode(input.Node),
					Category:  CategoryState,
					Type:      "KustomizationFailed",
					Severity:  SeverityCritical,
					Timestamp: event.Timestamp,
					Summary:   fmt.Sprintf("Kustomization is not ready: %s", condReason),
					Details: map[string]interface{}{
						"condition_type":    condType,
						"condition_status":  condStatus,
						"condition_reason":  condReason,
						"condition_message": condMessage,
					},
				})
			}

			if condType == "Reconciling" && condStatus == conditionFalse {
				failureReasons := []string{"BuildFailed", "ArtifactFailed", "DependencyNotReady", "ReconciliationFailed"}
				for _, failReason := range failureReasons {
					if condReason == failReason {
						anomalies = append(anomalies, Anomaly{
							Node:      NodeFromGraphNode(input.Node),
							Category:  CategoryState,
							Type:      "KustomizationFailed",
							Severity:  SeverityCritical,
							Timestamp: event.Timestamp,
							Summary:   fmt.Sprintf("Kustomization failed: %s", condReason),
							Details: map[string]interface{}{
								"condition_type":    condType,
								"condition_status":  condStatus,
								"condition_reason":  condReason,
								"condition_message": condMessage,
							},
						})
						break
					}
				}
			}
		}
	}

	return anomalies
}
