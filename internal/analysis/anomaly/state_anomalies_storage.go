package anomaly

import (
	"encoding/json"
	"fmt"
	"time"
)

// detectPVCStateAnomalies detects PersistentVolumeClaim binding failures.
func (d *StateAnomalyDetector) detectPVCStateAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	const pendingThreshold = 1 * time.Minute
	var firstPending time.Time
	var lastPending time.Time
	var pendingReason string

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

		phase, _ := status["phase"].(string)
		if phase == "Pending" {
			if firstPending.IsZero() {
				firstPending = event.Timestamp
			}
			lastPending = event.Timestamp

			if conditions, ok := status["conditions"].([]interface{}); ok {
				for _, condInterface := range conditions {
					cond, ok := condInterface.(map[string]interface{})
					if !ok {
						continue
					}

					condType, _ := cond["type"].(string)
					condStatus, _ := cond["status"].(string)
					condReason, _ := cond["reason"].(string)
					condMessage, _ := cond["message"].(string)

					if condStatus == conditionFalse || condType == "Resizing" && condStatus == "True" {
						if condReason != "" {
							pendingReason = condReason
						}
						if condMessage != "" && pendingReason == "" {
							pendingReason = condMessage
						}
					}
				}
			}
		}

		if phase == "Lost" {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryState,
				Type:      "PVCBindingFailed",
				Severity:  SeverityCritical,
				Timestamp: event.Timestamp,
				Summary:   "PersistentVolumeClaim lost its bound PersistentVolume",
				Details: map[string]interface{}{
					"phase":  phase,
					"reason": "PersistentVolume was deleted",
				},
			})
		}
	}

	if !firstPending.IsZero() && !lastPending.IsZero() {
		duration := lastPending.Sub(firstPending)
		if duration >= pendingThreshold {
			summary := "PersistentVolumeClaim failed to bind"
			if pendingReason != "" {
				summary = fmt.Sprintf("PersistentVolumeClaim failed to bind: %s", pendingReason)
			}

			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryState,
				Type:      "PVCBindingFailed",
				Severity:  SeverityCritical,
				Timestamp: lastPending,
				Summary:   summary,
				Details: map[string]interface{}{
					"phase":            "Pending",
					"duration_seconds": int64(duration.Seconds()),
					"reason":           pendingReason,
				},
			})
		}
	}

	return anomalies
}
