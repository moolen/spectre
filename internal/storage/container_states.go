package storage

import (
	"encoding/json"
	"strings"
)

const (
	issueTypeCrashLoopBackOff = "CrashLoopBackOff"
	issueTypeImagePullBackOff = "ImagePullBackOff"
	issueTypeOOMKilled        = "OOMKilled"
)

// ContainerIssue represents a detected container-level problem
type ContainerIssue struct {
	ContainerName string  `json:"container_name"`
	IssueType     string  `json:"issue_type"` // CrashLoopBackOff, ImagePullBackOff, OOMKilled, HighRestartCount
	RestartCount  int32   `json:"restart_count"`
	Message       string  `json:"message"`
	Reason        string  `json:"reason"`
	ExitCode      int32   `json:"exit_code,omitempty"`
	ImpactScore   float64 `json:"impact_score"` // Individual container issue impact
}

// InspectContainerStates analyzes pod data to detect container-level issues
// Returns a list of detected issues with their impact scores
func InspectContainerStates(obj *resourceData) []ContainerIssue {
	issues := make([]ContainerIssue, 0)

	status := obj.status()
	if status == nil {
		return issues
	}

	// Check both containerStatuses and initContainerStatuses
	containerStatuses := getSliceValue(status, "containerStatuses")
	initContainerStatuses := getSliceValue(status, "initContainerStatuses")

	issues = append(issues, inspectContainerStatusList(containerStatuses)...)
	issues = append(issues, inspectContainerStatusList(initContainerStatuses)...)

	return issues
}

func inspectContainerStatusList(containerStatuses []any) []ContainerIssue {
	issues := make([]ContainerIssue, 0)

	for _, cs := range containerStatuses {
		containerStatus, ok := cs.(map[string]any)
		if !ok {
			continue
		}

		containerName := getStringValue(containerStatus, "name")
		restartCount := int32(getIntValue(containerStatus, "restartCount")) //nolint:gosec

		// Check current state (Waiting)
		if state := getMapValue(containerStatus, "state"); state != nil {
			if waiting := getMapValue(state, "waiting"); waiting != nil {
				reason := getStringValue(waiting, "reason")
				message := getStringValue(waiting, "message")

				// CrashLoopBackOff detection
				if strings.EqualFold(reason, "CrashLoopBackOff") {
					issues = append(issues, ContainerIssue{
						ContainerName: containerName,
						IssueType:     issueTypeCrashLoopBackOff,
						RestartCount:  restartCount,
						Reason:        reason,
						Message:       message,
						ImpactScore:   0.35,
					})
				}

				// ImagePullBackOff detection
				if strings.EqualFold(reason, "ImagePullBackOff") || strings.EqualFold(reason, "ErrImagePull") {
					issues = append(issues, ContainerIssue{
						ContainerName: containerName,
						IssueType:     issueTypeImagePullBackOff,
						RestartCount:  restartCount,
						Reason:        reason,
						Message:       message,
						ImpactScore:   0.25,
					})
				}
			}
		}

		// Check last termination state for OOMKilled
		if lastState := getMapValue(containerStatus, "lastState"); lastState != nil {
			if terminated := getMapValue(lastState, "terminated"); terminated != nil {
				reason := getStringValue(terminated, "reason")
				exitCode := int32(getIntValue(terminated, "exitCode")) //nolint:gosec
				message := getStringValue(terminated, "message")

				// OOMKilled detection (either by reason or exit code 137)
				if strings.EqualFold(reason, "OOMKilled") || exitCode == 137 {
					issues = append(issues, ContainerIssue{
						ContainerName: containerName,
						IssueType:     issueTypeOOMKilled,
						RestartCount:  restartCount,
						Reason:        reason,
						Message:       message,
						ExitCode:      exitCode,
						ImpactScore:   0.40,
					})
				}
			}
		}

		// High restart count detection
		// >5 restarts is Warning, >10 is higher impact
		if restartCount > 5 {
			impactScore := 0.20
			issueType := "HighRestartCount"
			message := "Container has restarted multiple times"

			if restartCount > 10 {
				impactScore = 0.35
				issueType = "VeryHighRestartCount"
			}

			// Only add this if we haven't already detected a more specific issue
			// (CrashLoopBackOff or ImagePullBackOff would already explain the restarts)
			hasSpecificIssue := false
			for _, issue := range issues {
				if issue.ContainerName == containerName &&
					(issue.IssueType == issueTypeCrashLoopBackOff || issue.IssueType == issueTypeImagePullBackOff) {
					hasSpecificIssue = true
					break
				}
			}

			if !hasSpecificIssue {
				issues = append(issues, ContainerIssue{
					ContainerName: containerName,
					IssueType:     issueType,
					RestartCount:  restartCount,
					Message:       message,
					ImpactScore:   impactScore,
				})
			}
		}
	}

	return issues
}

// GetContainerIssuesFromJSON is a convenience function to inspect container states from raw JSON
func GetContainerIssuesFromJSON(data json.RawMessage) ([]ContainerIssue, error) {
	obj, err := newResourceData(data)
	if err != nil {
		return nil, err
	}
	return InspectContainerStates(obj), nil
}

// HasCriticalContainerIssues returns true if any container has a critical issue
func HasCriticalContainerIssues(issues []ContainerIssue) bool {
	for _, issue := range issues {
		if issue.IssueType == issueTypeOOMKilled || issue.IssueType == issueTypeCrashLoopBackOff {
			return true
		}
	}
	return false
}

// GetHighestImpactScore returns the highest impact score among container issues
func GetHighestImpactScore(issues []ContainerIssue) float64 {
	maxScore := 0.0
	for _, issue := range issues {
		if issue.ImpactScore > maxScore {
			maxScore = issue.ImpactScore
		}
	}
	return maxScore
}
