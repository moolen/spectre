package anomaly

import (
	"encoding/json"
	"strings"

	"github.com/moolen/spectre/internal/analysis"
)

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
					UID:       event.EventID,
					Kind:      kind,
					Namespace: "",
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

// extractContainerIssues extracts container issue strings from an event.
func (d *StateAnomalyDetector) extractContainerIssues(event analysis.ChangeEventInfo) []string {
	var issues []string

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

	if event.FullSnapshot != nil {
		issues = append(issues, d.extractIssuesFromStatus(event.FullSnapshot)...)
	}

	if len(event.Data) > 0 {
		var resourceData map[string]interface{}
		if err := json.Unmarshal(event.Data, &resourceData); err == nil {
			issues = append(issues, d.extractIssuesFromStatus(resourceData)...)
		}
	}

	return issues
}

// extractIssuesFromStatus extracts container issues from a resource status object.
func (d *StateAnomalyDetector) extractIssuesFromStatus(resourceData map[string]interface{}) []string {
	var issues []string

	if status, ok := resourceData["status"].(map[string]interface{}); ok {
		if containerStatuses, ok := status["containerStatuses"].([]interface{}); ok {
			for _, csInterface := range containerStatuses {
				if cs, ok := csInterface.(map[string]interface{}); ok {
					if state, ok := cs["state"].(map[string]interface{}); ok {
						if waiting, ok := state["waiting"].(map[string]interface{}); ok {
							if reason, ok := waiting["reason"].(string); ok {
								issues = append(issues, reason)
							}
						}
						if terminated, ok := state["terminated"].(map[string]interface{}); ok {
							if reason, ok := terminated["reason"].(string); ok {
								issues = append(issues, reason)
							}
						}
					}
				}
			}
		}

		if initContainerStatuses, ok := status["initContainerStatuses"].([]interface{}); ok {
			for _, icsInterface := range initContainerStatuses {
				if ics, ok := icsInterface.(map[string]interface{}); ok {
					if state, ok := ics["state"].(map[string]interface{}); ok {
						if waiting, ok := state["waiting"].(map[string]interface{}); ok {
							if reason, ok := waiting["reason"].(string); ok {
								issues = append(issues, reason)
								if reason == "CrashLoopBackOff" || reason == statusError || reason == "ImagePullBackOff" || reason == "ErrImagePull" {
									issues = append(issues, "InitContainerFailed")
								}
							}
						}
						if terminated, ok := state["terminated"].(map[string]interface{}); ok {
							if reason, ok := terminated["reason"].(string); ok {
								issues = append(issues, reason)
							}
							if exitCode, ok := terminated["exitCode"].(float64); ok && exitCode != 0 {
								issues = append(issues, "InitContainerFailed")
							}
							if reason, ok := terminated["reason"].(string); ok && reason == statusError {
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
