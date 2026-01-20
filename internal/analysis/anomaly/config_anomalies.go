package anomaly

import (
	"encoding/json"
	"strings"
)

// ConfigAnomalyDetector detects configuration-related anomalies
type ConfigAnomalyDetector struct{}

// NewConfigAnomalyDetector creates a new config anomaly detector
func NewConfigAnomalyDetector() *ConfigAnomalyDetector {
	return &ConfigAnomalyDetector{}
}

// Detect analyzes configuration for anomalies
func (d *ConfigAnomalyDetector) Detect(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	switch input.Node.Resource.Kind {
	case "NetworkPolicy":
		anomalies = append(anomalies, d.detectNetworkPolicyConfigAnomalies(input)...)
	case "ServiceAccount":
		anomalies = append(anomalies, d.detectServiceAccountConfigAnomalies(input)...)
	}

	return anomalies
}

func (d *ConfigAnomalyDetector) detectNetworkPolicyConfigAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	for _, event := range input.AllEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		// Parse resource data from either FullSnapshot or Data field
		var resourceData map[string]interface{}
		if event.FullSnapshot != nil {
			resourceData = event.FullSnapshot
		} else if len(event.Data) > 0 {
			if err := json.Unmarshal(event.Data, &resourceData); err != nil {
				continue
			}
		}

		if resourceData != nil {
			if spec, ok := resourceData["spec"].(map[string]interface{}); ok {
				// Check policyTypes for Ingress restriction
				// Any NetworkPolicy that includes "Ingress" in policyTypes is restrictive by definition
				// because it means the policy is actively controlling ingress traffic
				if policyTypes, ok := spec["policyTypes"].([]interface{}); ok {
					for _, pt := range policyTypes {
						if ptStr, ok := pt.(string); ok && ptStr == "Ingress" {
							// This NetworkPolicy affects ingress traffic
							// Report it as an ingress restriction regardless of specific rules
							anomalies = append(anomalies, Anomaly{
								Node:      NodeFromGraphNode(input.Node),
								Category:  CategoryConfig,
								Type:      "IngressRestriction",
								Severity:  SeverityMedium,
								Timestamp: event.Timestamp,
								Summary:   "NetworkPolicy restricts ingress traffic",
								Details: map[string]interface{}{
									"policy_types": policyTypes,
								},
							})
							break // Only report once per event
						}
					}
				}
			}
		}
	}

	return anomalies
}

func (d *ConfigAnomalyDetector) detectServiceAccountConfigAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	// Check K8s events for RBAC-related errors
	for _, event := range input.K8sEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		// Check for RBAC error messages
		msgLower := strings.ToLower(event.Message)
		reasonLower := strings.ToLower(event.Reason)

		if strings.Contains(msgLower, "forbidden") ||
		   strings.Contains(msgLower, "insufficient permission") ||
		   strings.Contains(msgLower, "unauthorized") ||
		   strings.Contains(reasonLower, "failedcreate") && strings.Contains(msgLower, "forbidden") {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryConfig,
				Type:      "InsufficientPermissions",
				Severity:  SeverityMedium,
				Timestamp: event.Timestamp,
				Summary:   "ServiceAccount has insufficient permissions",
				Details: map[string]interface{}{
					"event_reason":  event.Reason,
					"event_message": event.Message,
				},
			})
		}
	}

	return anomalies
}
