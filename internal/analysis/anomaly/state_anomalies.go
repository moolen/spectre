package anomaly

import (
	"fmt"
)

const (
	statusError            = "Error"
	conditionFalse         = "False"
	conditionReady         = "Ready"
	kindStatefulSet        = "StatefulSet"
	kindSecret             = "Secret"
	reasonImagePullBackOff = "ImagePullBackOff"
)

// StateAnomalyDetector detects abnormal resource states
type StateAnomalyDetector struct{}

// NewStateAnomalyDetector creates a new state anomaly detector
func NewStateAnomalyDetector() *StateAnomalyDetector {
	return &StateAnomalyDetector{}
}

// Detect analyzes resource states for anomalies
func (d *StateAnomalyDetector) Detect(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	for _, event := range input.AllEvents {
		// Skip events outside time window
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		// Extract container issues from description and full snapshot
		containerIssues := d.extractContainerIssues(event)
		for _, issue := range containerIssues {
			anomaly := d.classifyContainerIssue(input.Node.Resource.Kind, event, issue)
			if anomaly != nil {
				anomalies = append(anomalies, *anomaly)
			}
		}

		// Check status field
		switch event.Status {
		case statusError:
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryState,
				Type:      "ErrorStatus",
				Severity:  SeverityHigh,
				Timestamp: event.Timestamp,
				Summary:   fmt.Sprintf("%s in Error state", input.Node.Resource.Kind),
				Details: map[string]interface{}{
					"description": event.Description,
					"status":      event.Status,
				},
			})
		case "Terminating":
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryState,
				Type:      "TerminatingStatus",
				Severity:  SeverityMedium,
				Timestamp: event.Timestamp,
				Summary:   fmt.Sprintf("%s is terminating", input.Node.Resource.Kind),
				Details: map[string]interface{}{
					"status": event.Status,
				},
			})
		}
	}

	// Kind-specific state checks
	switch input.Node.Resource.Kind {
	case kindPod:
		anomalies = append(anomalies, d.detectPodStateAnomalies(input)...)
	case "Node":
		anomalies = append(anomalies, d.detectNodeStateAnomalies(input)...)
	case kindDeployment:
		anomalies = append(anomalies, d.detectDeploymentStateAnomalies(input)...)
	case "Service":
		anomalies = append(anomalies, d.detectServiceStateAnomalies(input)...)
	case "EndpointSlice":
		anomalies = append(anomalies, d.detectEndpointSliceStateAnomalies(input)...)
	case "Ingress":
		anomalies = append(anomalies, d.detectIngressStateAnomalies(input)...)
	case kindStatefulSet:
		anomalies = append(anomalies, d.detectStatefulSetStateAnomalies(input)...)
	case "ConfigMap", kindSecret:
		anomalies = append(anomalies, d.detectConfigResourceStateAnomalies(input)...)
	case "HelmRelease":
		anomalies = append(anomalies, d.detectHelmReleaseStateAnomalies(input)...)
	case "Kustomization":
		anomalies = append(anomalies, d.detectKustomizationStateAnomalies(input)...)
	case "PersistentVolumeClaim":
		anomalies = append(anomalies, d.detectPVCStateAnomalies(input)...)
	}

	return anomalies
}
