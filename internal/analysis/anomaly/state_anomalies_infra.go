package anomaly

import (
	"encoding/json"
	"strings"
)

func (d *StateAnomalyDetector) detectNodeStateAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	for _, event := range input.AllEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		if event.FullSnapshot != nil {
			if status, ok := event.FullSnapshot["status"].(map[string]interface{}); ok {
				if conditions, ok := status["conditions"].([]interface{}); ok {
					for _, conditionInterface := range conditions {
						if condition, ok := conditionInterface.(map[string]interface{}); ok {
							condType, _ := condition["type"].(string)
							condStatus, _ := condition["status"].(string)

							if condType == conditionReady && condStatus == conditionFalse {
								anomalies = append(anomalies, Anomaly{
									Node:      NodeFromGraphNode(input.Node),
									Category:  CategoryState,
									Type:      "NodeNotReady",
									Severity:  SeverityCritical,
									Timestamp: event.Timestamp,
									Summary:   "Node is NotReady",
									Details: map[string]interface{}{
										"condition_type":   condType,
										"condition_status": condStatus,
									},
								})
							}

							if condStatus == "True" {
								switch condType {
								case "DiskPressure":
									anomalies = append(anomalies, Anomaly{
										Node:      NodeFromGraphNode(input.Node),
										Category:  CategoryState,
										Type:      "DiskPressure",
										Severity:  SeverityMedium,
										Timestamp: event.Timestamp,
										Summary:   "Node has DiskPressure",
										Details: map[string]interface{}{
											"condition_type": condType,
										},
									})
								case "MemoryPressure":
									anomalies = append(anomalies, Anomaly{
										Node:      NodeFromGraphNode(input.Node),
										Category:  CategoryState,
										Type:      "NodeMemoryPressure",
										Severity:  SeverityHigh,
										Timestamp: event.Timestamp,
										Summary:   "Node has MemoryPressure",
										Details: map[string]interface{}{
											"condition_type": condType,
										},
									})
								case "PIDPressure":
									anomalies = append(anomalies, Anomaly{
										Node:      NodeFromGraphNode(input.Node),
										Category:  CategoryState,
										Type:      "NodePIDPressure",
										Severity:  SeverityHigh,
										Timestamp: event.Timestamp,
										Summary:   "Node has PIDPressure",
										Details: map[string]interface{}{
											"condition_type": condType,
										},
									})
								}
							}
						}
					}
				}
			}
		}
	}

	return anomalies
}

func (d *StateAnomalyDetector) detectServiceStateAnomalies(input DetectorInput) []Anomaly {
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

		if resourceData != nil {
			desc := strings.ToLower(event.Description)
			if strings.Contains(desc, "no endpoint") || strings.Contains(desc, "no ready endpoint") {
				anomalies = append(anomalies, Anomaly{
					Node:      NodeFromGraphNode(input.Node),
					Category:  CategoryState,
					Type:      "NoReadyEndpoints",
					Severity:  SeverityHigh,
					Timestamp: event.Timestamp,
					Summary:   "Service has no ready endpoints",
					Details: map[string]interface{}{
						"description": event.Description,
					},
				})
			}
		}
	}

	return anomalies
}

func (d *StateAnomalyDetector) detectEndpointSliceStateAnomalies(input DetectorInput) []Anomaly {
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

		if resourceData != nil {
			if endpoints, ok := resourceData["endpoints"].([]interface{}); ok {
				hasReadyEndpoint := false
				for _, epInterface := range endpoints {
					if ep, ok := epInterface.(map[string]interface{}); ok {
						if conditions, ok := ep["conditions"].(map[string]interface{}); ok {
							if ready, ok := conditions["ready"].(bool); ok && ready {
								hasReadyEndpoint = true
								break
							}
						}
					}
				}

				if len(endpoints) > 0 && !hasReadyEndpoint {
					anomalies = append(anomalies, Anomaly{
						Node:      NodeFromGraphNode(input.Node),
						Category:  CategoryState,
						Type:      "NoReadyEndpoints",
						Severity:  SeverityHigh,
						Timestamp: event.Timestamp,
						Summary:   "EndpointSlice has no ready endpoints",
						Details: map[string]interface{}{
							"endpoint_count": len(endpoints),
						},
					})
				}
			}
		}
	}

	return anomalies
}

func (d *StateAnomalyDetector) detectIngressStateAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	for _, event := range input.AllEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		desc := strings.ToLower(event.Description)
		if strings.Contains(desc, "backend") && (strings.Contains(desc, "down") || strings.Contains(desc, "unavailable") || strings.Contains(desc, "no endpoint")) {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryState,
				Type:      "BackendDown",
				Severity:  SeverityHigh,
				Timestamp: event.Timestamp,
				Summary:   "Ingress backend is down or unavailable",
				Details: map[string]interface{}{
					"description": event.Description,
				},
			})
		}
	}

	return anomalies
}
