package anomaly

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/analysis"
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
		case "Error":
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
	case "Pod":
		anomalies = append(anomalies, d.detectPodStateAnomalies(input)...)
	case "Node":
		anomalies = append(anomalies, d.detectNodeStateAnomalies(input)...)
	case "Deployment":
		anomalies = append(anomalies, d.detectDeploymentStateAnomalies(input)...)
	case "Service":
		anomalies = append(anomalies, d.detectServiceStateAnomalies(input)...)
	case "EndpointSlice":
		anomalies = append(anomalies, d.detectEndpointSliceStateAnomalies(input)...)
	case "Ingress":
		anomalies = append(anomalies, d.detectIngressStateAnomalies(input)...)
	case "StatefulSet":
		anomalies = append(anomalies, d.detectStatefulSetStateAnomalies(input)...)
	case "ConfigMap", "Secret":
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

func (d *StateAnomalyDetector) classifyContainerIssue(kind string, event analysis.ChangeEventInfo, issue string) *Anomaly {
	issueLower := strings.ToLower(issue)

	type issueRule struct {
		contains string
		anomType string
		severity Severity
		summary  string
	}

	rules := []issueRule{
		{"crashloopbackoff", "CrashLoopBackOff", SeverityCritical, "Container in CrashLoopBackOff"},
		{"imagepullbackoff", "ImagePullBackOff", SeverityCritical, "Container cannot pull image"},
		{"oomkilled", "OOMKilled", SeverityHigh, "Container killed due to OOM"},
		{"errimagepull", "ErrImagePull", SeverityHigh, "Failed to pull container image"},
		{"createcontainererror", "ContainerCreateError", SeverityHigh, "Failed to create container"},
		{"createcontainerconfig", "CreateContainerConfigError", SeverityHigh, "Failed to create container config"},
		{"invalidimagenameerror", "InvalidImageNameError", SeverityHigh, "Invalid container image name"},
		{"initcontainerfailed", "InitContainerFailed", SeverityHigh, "Init container failed"},
	}

	for _, rule := range rules {
		if strings.Contains(issueLower, rule.contains) {
			return &Anomaly{
				Node: AnomalyNode{
					UID:       event.EventID, // Use event ID as unique identifier
					Kind:      kind,
					Namespace: "", // Will be set by caller if needed
					Name:      "",
				},
				Category:  CategoryState,
				Type:      rule.anomType,
				Severity:  rule.severity,
				Timestamp: event.Timestamp,
				Summary:   rule.summary,
				Details: map[string]interface{}{
					"container_issue": issue,
				},
			}
		}
	}

	return nil
}

func (d *StateAnomalyDetector) detectPodStateAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	// Check for long-running Pending state
	const pendingThreshold = 5 * time.Minute
	var firstPending time.Time
	var lastPending time.Time

	for _, event := range input.AllEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		// Track Pending status duration
		if event.Status == "Pending" || strings.Contains(strings.ToLower(event.EventType), "pending") {
			if firstPending.IsZero() {
				firstPending = event.Timestamp
			}
			lastPending = event.Timestamp
		}

		// Parse resource data from either FullSnapshot or Data field
		var resourceData map[string]interface{}
		if event.FullSnapshot != nil {
			resourceData = event.FullSnapshot
		} else if len(event.Data) > 0 {
			if err := json.Unmarshal(event.Data, &resourceData); err == nil {
				// Successfully parsed
			}
		}

		if resourceData != nil {
			if status, ok := resourceData["status"].(map[string]interface{}); ok {
				// Check for Failed phase
				if phase, ok := status["phase"].(string); ok && strings.ToLower(phase) == "failed" {
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

				// Check for Evicted reason
				if reason, ok := status["reason"].(string); ok {
					if strings.ToLower(reason) == "evicted" {
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

				// Check pod conditions for Unschedulable
				if conditions, ok := status["conditions"].([]interface{}); ok {
					for _, conditionInterface := range conditions {
						if condition, ok := conditionInterface.(map[string]interface{}); ok {
							condType, _ := condition["type"].(string)
							condStatus, _ := condition["status"].(string)
							condReason, _ := condition["reason"].(string)

							// Check for PodScheduled = False with Unschedulable reason
							if condType == "PodScheduled" && condStatus == "False" && condReason == "Unschedulable" {
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

	// Report long-running Pending state
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

func (d *StateAnomalyDetector) detectNodeStateAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	for _, event := range input.AllEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		// Analyze node conditions from full snapshot
		if event.FullSnapshot != nil {
			if status, ok := event.FullSnapshot["status"].(map[string]interface{}); ok {
				if conditions, ok := status["conditions"].([]interface{}); ok {
					for _, conditionInterface := range conditions {
						if condition, ok := conditionInterface.(map[string]interface{}); ok {
							condType, _ := condition["type"].(string)
							condStatus, _ := condition["status"].(string)

							// Check for NotReady
							if condType == "Ready" && condStatus == "False" {
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

							// Check for pressure conditions
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

// extractContainerIssues extracts container issue strings from an event
func (d *StateAnomalyDetector) extractContainerIssues(event analysis.ChangeEventInfo) []string {
	var issues []string

	// Check description for common issues
	desc := strings.ToLower(event.Description)
	knownIssues := []string{
		"CrashLoopBackOff", "ImagePullBackOff", "OOMKilled",
		"ErrImagePull", "CreateContainerError", "InvalidImageNameError",
	}

	for _, issue := range knownIssues {
		if strings.Contains(desc, strings.ToLower(issue)) {
			issues = append(issues, issue)
		}
	}

	// Also check full snapshot for container statuses (used for first event after conversion)
	if event.FullSnapshot != nil {
		issues = append(issues, d.extractIssuesFromStatus(event.FullSnapshot)...)
	}

	// Also check Data field for container statuses (used when FullSnapshot not available)
	// This handles UPDATE events that only have Diff (not FullSnapshot)
	if len(event.Data) > 0 {
		var resourceData map[string]interface{}
		if err := json.Unmarshal(event.Data, &resourceData); err == nil {
			statusIssues := d.extractIssuesFromStatus(resourceData)
			issues = append(issues, statusIssues...)
		}
	}

	return issues
}

// extractIssuesFromStatus extracts container issues from a resource status object
func (d *StateAnomalyDetector) extractIssuesFromStatus(resourceData map[string]interface{}) []string {
	var issues []string

	if status, ok := resourceData["status"].(map[string]interface{}); ok {
		// Check containerStatuses
		if containerStatuses, ok := status["containerStatuses"].([]interface{}); ok {
			for _, csInterface := range containerStatuses {
				if cs, ok := csInterface.(map[string]interface{}); ok {
					// Check waiting state
					if state, ok := cs["state"].(map[string]interface{}); ok {
						if waiting, ok := state["waiting"].(map[string]interface{}); ok {
							if reason, ok := waiting["reason"].(string); ok {
								issues = append(issues, reason)
							}
						}
						// Check terminated state
						if terminated, ok := state["terminated"].(map[string]interface{}); ok {
							if reason, ok := terminated["reason"].(string); ok {
								issues = append(issues, reason)
							}
						}
					}
				}
			}
		}

		// Check initContainerStatuses for init container failures
		if initContainerStatuses, ok := status["initContainerStatuses"].([]interface{}); ok {
			for _, icsInterface := range initContainerStatuses {
				if ics, ok := icsInterface.(map[string]interface{}); ok {
					if state, ok := ics["state"].(map[string]interface{}); ok {
						// Check waiting state for init container failures
						if waiting, ok := state["waiting"].(map[string]interface{}); ok {
							if reason, ok := waiting["reason"].(string); ok {
								issues = append(issues, reason)
								// Also mark as init container failure for specific reasons
								if reason == "CrashLoopBackOff" || reason == "Error" || reason == "ImagePullBackOff" || reason == "ErrImagePull" {
									issues = append(issues, "InitContainerFailed")
								}
							}
						}
						// Check terminated state with non-zero exit code or Error reason
						if terminated, ok := state["terminated"].(map[string]interface{}); ok {
							if reason, ok := terminated["reason"].(string); ok {
								issues = append(issues, reason)
							}
							// Non-zero exit code indicates init container failure
							if exitCode, ok := terminated["exitCode"].(float64); ok && exitCode != 0 {
								issues = append(issues, "InitContainerFailed")
							}
							// Error reason also indicates failure
							if reason, ok := terminated["reason"].(string); ok && reason == "Error" {
								issues = append(issues, "InitContainerFailed")
							}
						}
					}
				}
			}
		}
	}

	return issues
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

		// Check if there's a config change anywhere in the events
		if event.ConfigChanged {
			hasConfigChange = true
		}

		// Parse resource data from either FullSnapshot or Data field
		var resourceData map[string]interface{}
		if event.FullSnapshot != nil {
			resourceData = event.FullSnapshot
		} else if len(event.Data) > 0 {
			// Parse from Data field (used when querying from graph)
			if err := json.Unmarshal(event.Data, &resourceData); err != nil {
				continue
			}
		}

		if resourceData != nil {
			if status, ok := resourceData["status"].(map[string]interface{}); ok {
				// Check unavailable replicas
				if unavailable, ok := status["unavailableReplicas"].(float64); ok && unavailable > 0 {
					// Track the most recent unavailable state
					if event.Timestamp.After(lastUnavailable) {
						lastUnavailable = event.Timestamp
						unavailableCount = int32(unavailable)
					}
				}

				// Check Progressing condition
				if conditions, ok := status["conditions"].([]interface{}); ok {
					for _, conditionInterface := range conditions {
						if condition, ok := conditionInterface.(map[string]interface{}); ok {
							condType, _ := condition["type"].(string)
							condStatus, _ := condition["status"].(string)
							condReason, _ := condition["reason"].(string)

							// Check for ProgressDeadlineExceeded
							if condType == "Progressing" && condStatus == "False" && condReason == "ProgressDeadlineExceeded" {
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

	// Report unavailable replicas after config change
	// If there's a config change (in any event) and unavailable replicas appear (in any event), report as stuck
	// Note: ConfigChanged and unavailableReplicas may be in different events since config change happens
	// on the UPDATE event where spec changes, but unavailableReplicas appears on later UPDATE events
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

func (d *StateAnomalyDetector) detectServiceStateAnomalies(input DetectorInput) []Anomaly {
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

		// Check for services with no ready endpoints
		// This is indicated by examining the service spec/status
		// We look for annotation or status indicating no endpoints
		if resourceData != nil {
			// Check description for endpoint issues
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
			// Check endpoints for ready status
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

				// If we have endpoints but none are ready, report anomaly
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

		// Check description for backend issues
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

func (d *StateAnomalyDetector) detectStatefulSetStateAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	var hasConfigChange bool
	var hasUpdateRevisionRollback bool

	for _, event := range input.AllEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		// Check if there's a config change
		if event.ConfigChanged {
			hasConfigChange = true
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
			if status, ok := resourceData["status"].(map[string]interface{}); ok {
				// Check if currentRevision != updateRevision (rollback indicator)
				currentRev, hasCurrentRev := status["currentRevision"].(string)
				updateRev, hasUpdateRev := status["updateRevision"].(string)

				if hasCurrentRev && hasUpdateRev && currentRev != "" && updateRev != "" && currentRev != updateRev {
					// Check if updateRevision is older than currentRevision (indicates rollback)
					// For now, just check if they're different after a config change
					if hasConfigChange {
						hasUpdateRevisionRollback = true
					}
				}
			}
		}
	}

	// Report rollback if detected
	if hasUpdateRevisionRollback {
		anomalies = append(anomalies, Anomaly{
			Node:      NodeFromGraphNode(input.Node),
			Category:  CategoryState,
			Type:      "UpdateRollback",
			Severity:  SeverityHigh,
			Timestamp: input.TimeWindow.End, // Use end of window as timestamp
			Summary:   "StatefulSet update has been rolled back",
			Details:   map[string]interface{}{},
		})
	}

	return anomalies
}

func (d *StateAnomalyDetector) detectConfigResourceStateAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	// Check if the ConfigMap or Secret was deleted
	for _, event := range input.AllEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		// Check for DELETE events
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

// detectHelmReleaseStateAnomalies detects Flux HelmRelease failure states
// It checks status.conditions for Ready=False or Released=False conditions
func (d *StateAnomalyDetector) detectHelmReleaseStateAnomalies(input DetectorInput) []Anomaly {
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

		if resourceData == nil {
			continue
		}

		status, ok := resourceData["status"].(map[string]interface{})
		if !ok {
			continue
		}

		// Check conditions for failure states
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

			// Detect Ready=False (general failure state)
			if condType == "Ready" && condStatus == "False" {
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

			// Detect Released=False (Helm install/upgrade failed)
			if condType == "Released" && condStatus == "False" {
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

// detectKustomizationStateAnomalies detects Flux Kustomization failure states
// It checks status.conditions for Ready=False conditions
func (d *StateAnomalyDetector) detectKustomizationStateAnomalies(input DetectorInput) []Anomaly {
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

		if resourceData == nil {
			continue
		}

		status, ok := resourceData["status"].(map[string]interface{})
		if !ok {
			continue
		}

		// Check conditions for failure states
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

			// Detect Ready=False (general failure state)
			if condType == "Ready" && condStatus == "False" {
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

			// Detect Reconciling=False with failure reasons (build/apply failed)
			if condType == "Reconciling" && condStatus == "False" {
				// Check if it's actually a failure reason, not just successful reconciliation complete
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

// detectPVCStateAnomalies detects PersistentVolumeClaim binding failures
// It checks status.phase for Pending state which indicates binding failed
func (d *StateAnomalyDetector) detectPVCStateAnomalies(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	// Track if PVC has been in Pending state for extended time
	const pendingThreshold = 1 * time.Minute
	var firstPending time.Time
	var lastPending time.Time
	var pendingReason string

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

		if resourceData == nil {
			continue
		}

		status, ok := resourceData["status"].(map[string]interface{})
		if !ok {
			continue
		}

		// Check PVC phase
		phase, _ := status["phase"].(string)
		if phase == "Pending" {
			if firstPending.IsZero() {
				firstPending = event.Timestamp
			}
			lastPending = event.Timestamp

			// Check conditions for more specific failure reasons
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

					// Check for specific failure conditions
					if condStatus == "False" || condType == "Resizing" && condStatus == "True" {
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

		// Check for Lost phase (PV was deleted while bound)
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

	// Report PVC stuck in Pending state
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
