package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"
)

const podPhasePending = "pending"

// InferErrorMessages extracts detailed error messages from resource data
// Returns a slice of error strings describing what is wrong with the resource
func InferErrorMessages(kind string, data json.RawMessage, status string) []string {
	if len(data) == 0 {
		return nil
	}

	// Only extract errors for Warning and Error statuses
	if status != resourceStatusWarning && status != resourceStatusError {
		return nil
	}

	obj, err := newResourceData(data)
	if err != nil {
		return nil
	}

	return inferResourceSpecificErrors(strings.ToLower(kind), obj)
}

func inferResourceSpecificErrors(kind string, obj *resourceData) []string {
	switch kind {
	case "pod":
		return inferPodErrors(obj)
	case "deployment":
		return inferDeploymentErrors(obj)
	case "statefulset":
		return inferStatefulSetErrors(obj)
	case "daemonset":
		return inferDaemonSetErrors(obj)
	case "replicaset":
		return inferReplicaSetErrors(obj)
	case "node":
		return inferNodeErrors(obj)
	case "job":
		return inferJobErrors(obj)
	case "persistentvolumeclaim":
		return inferPVCErrors(obj)
	default:
		// For unknown resource types, try to extract from conditions
		return inferGenericErrors(obj)
	}
}

func inferPodErrors(obj *resourceData) []string {
	errors := make([]string, 0)

	// Check container states first - most specific errors
	containerIssues := InspectContainerStates(obj)
	for _, issue := range containerIssues {
		var msg string
		switch issue.IssueType {
		case issueTypeCrashLoopBackOff:
			msg = fmt.Sprintf("CrashLoopBackOff (container: %s, restarts: %d)", issue.ContainerName, issue.RestartCount)
			if issue.Message != "" {
				msg += fmt.Sprintf(": %s", issue.Message)
			}
		case issueTypeImagePullBackOff:
			msg = fmt.Sprintf("ImagePullBackOff (container: %s)", issue.ContainerName)
			if issue.Reason == "ErrImagePull" {
				msg = fmt.Sprintf("ErrImagePull (container: %s)", issue.ContainerName)
			}
			if issue.Message != "" {
				msg += fmt.Sprintf(": %s", issue.Message)
			}
		case issueTypeOOMKilled:
			msg = fmt.Sprintf("OOMKilled (container: %s, exit code: %d)", issue.ContainerName, issue.ExitCode)
		case issueTypeHighRestartCount:
			msg = fmt.Sprintf("High restart count (container: %s, restarts: %d)", issue.ContainerName, issue.RestartCount)
		case issueTypeVeryHighRestartCount:
			msg = fmt.Sprintf("Very high restart count (container: %s, restarts: %d)", issue.ContainerName, issue.RestartCount)
		default:
			msg = fmt.Sprintf("%s (container: %s)", issue.IssueType, issue.ContainerName)
		}
		errors = append(errors, msg)
	}

	// Check pod phase
	phase := strings.ToLower(obj.statusString("phase"))
	switch phase {
	case podPhasePending:
		// Check for scheduling issues
		conditions := obj.conditions()
		if cond := findCondition(conditions, "PodScheduled"); cond != nil && cond.isFalse() {
			msg := fmt.Sprintf("Pod scheduling failed: %s", cond.Reason)
			if cond.Message != "" {
				msg += fmt.Sprintf(" - %s", cond.Message)
			}
			errors = append(errors, msg)
		} else if len(containerIssues) == 0 {
			// Only add generic pending if no specific issues found
			errors = append(errors, "Pod pending")
		}
	case "failed":
		if cond := findCondition(obj.conditions(), "Ready"); cond != nil && cond.Reason != "" {
			errors = append(errors, fmt.Sprintf("Pod failed: %s", cond.Reason))
		} else if len(containerIssues) == 0 {
			errors = append(errors, "Pod failed")
		}
	case "unknown":
		errors = append(errors, "Pod status unknown")
	}

	// Check if pod is not ready despite running
	if phase == "running" {
		if cond := obj.condition("Ready"); cond != nil && cond.isFalse() {
			if cond.Reason != "" && len(containerIssues) == 0 {
				msg := fmt.Sprintf("Pod not ready: %s", cond.Reason)
				if cond.Message != "" {
					msg += fmt.Sprintf(" - %s", cond.Message)
				}
				errors = append(errors, msg)
			}
		}
	}

	return errors
}

func inferDeploymentErrors(obj *resourceData) []string {
	errors := make([]string, 0)

	status := obj.status()
	if status == nil {
		return errors
	}

	desired := firstNonZero(obj.specInt("replicas"), obj.statusInt("replicas"))
	ready := obj.statusInt("readyReplicas")
	available := obj.statusInt("availableReplicas")
	unavailable := obj.statusInt("unavailableReplicas")

	// Check replica counts
	if desired > 0 && ready < desired {
		errors = append(errors, fmt.Sprintf("Insufficient replicas (%d/%d ready)", ready, desired))
	}

	if unavailable > 0 {
		errors = append(errors, fmt.Sprintf("%d unavailable replicas", unavailable))
	}

	if desired > 0 && available < desired {
		errors = append(errors, fmt.Sprintf("Only %d/%d replicas available", available, desired))
	}

	// Check conditions
	if cond := obj.condition("Available"); cond != nil && cond.isFalse() {
		msg := fmt.Sprintf("Not available: %s", cond.Reason)
		if cond.Message != "" {
			msg += fmt.Sprintf(" - %s", cond.Message)
		}
		errors = append(errors, msg)
	}

	if cond := obj.condition("Progressing"); cond != nil && cond.isFalse() {
		msg := fmt.Sprintf("Not progressing: %s", cond.Reason)
		if cond.Message != "" {
			msg += fmt.Sprintf(" - %s", cond.Message)
		}
		errors = append(errors, msg)
	}

	return errors
}

func inferStatefulSetErrors(obj *resourceData) []string {
	errors := make([]string, 0)

	status := obj.status()
	if status == nil {
		return errors
	}

	desired := firstNonZero(obj.specInt("replicas"), obj.statusInt("replicas"))
	ready := obj.statusInt("readyReplicas")
	current := obj.statusInt("currentReplicas")

	if desired > 0 && ready < desired {
		errors = append(errors, fmt.Sprintf("Insufficient replicas (%d/%d ready)", ready, desired))
	}

	if desired > 0 && current < desired {
		errors = append(errors, fmt.Sprintf("Only %d/%d current replicas", current, desired))
	}

	// Check conditions
	errors = append(errors, extractConditionErrors(obj)...)

	return errors
}

func inferDaemonSetErrors(obj *resourceData) []string {
	errors := make([]string, 0)

	status := obj.status()
	if status == nil {
		return errors
	}

	desired := obj.statusInt("desiredNumberScheduled")
	ready := obj.statusInt("numberReady")
	unavailable := obj.statusInt("numberUnavailable")
	misscheduled := obj.statusInt("numberMisscheduled")

	if desired > 0 && ready < desired {
		errors = append(errors, fmt.Sprintf("Only %d/%d pods ready", ready, desired))
	}

	if unavailable > 0 {
		errors = append(errors, fmt.Sprintf("%d pods unavailable", unavailable))
	}

	if misscheduled > 0 {
		errors = append(errors, fmt.Sprintf("%d pods misscheduled", misscheduled))
	}

	return errors
}

func inferReplicaSetErrors(obj *resourceData) []string {
	errors := make([]string, 0)

	status := obj.status()
	if status == nil {
		return errors
	}

	desired := obj.specInt("replicas")
	ready := obj.statusInt("readyReplicas")
	available := obj.statusInt("availableReplicas")

	if desired > 0 && ready < desired {
		errors = append(errors, fmt.Sprintf("Insufficient replicas (%d/%d ready)", ready, desired))
	}

	if available > 0 && available < desired {
		errors = append(errors, fmt.Sprintf("Only %d/%d replicas available", available, desired))
	}

	// Check conditions
	errors = append(errors, extractConditionErrors(obj)...)

	return errors
}

func inferNodeErrors(obj *resourceData) []string {
	errors := make([]string, 0)

	// Check Ready condition
	if cond := obj.condition("Ready"); cond != nil {
		if cond.isFalse() {
			msg := fmt.Sprintf("NotReady: %s", cond.Reason)
			if cond.Message != "" {
				msg += fmt.Sprintf(" - %s", cond.Message)
			}
			errors = append(errors, msg)
		} else if cond.isUnknown() {
			errors = append(errors, "Node status unknown")
		}
	}

	// Check network condition
	if cond := obj.condition("NetworkUnavailable"); cond != nil && cond.isTrue() {
		msg := "Network unavailable"
		if cond.Reason != "" {
			msg += fmt.Sprintf(": %s", cond.Reason)
		}
		errors = append(errors, msg)
	}

	// Check pressure conditions
	pressureConditions := []string{"MemoryPressure", "DiskPressure", "PIDPressure"}
	for _, condType := range pressureConditions {
		if cond := obj.condition(condType); cond != nil && cond.isTrue() {
			msg := condType
			if cond.Reason != "" {
				msg += fmt.Sprintf(": %s", cond.Reason)
			}
			if cond.Message != "" {
				msg += fmt.Sprintf(" - %s", cond.Message)
			}
			errors = append(errors, msg)
		}
	}

	return errors
}

func inferJobErrors(obj *resourceData) []string {
	errors := make([]string, 0)

	status := obj.status()
	if status == nil {
		return errors
	}

	// Check failure condition
	if cond := obj.condition("Failed"); cond != nil && cond.isTrue() {
		msg := fmt.Sprintf("Job failed: %s", cond.Reason)
		if cond.Message != "" {
			msg += fmt.Sprintf(" - %s", cond.Message)
		}
		errors = append(errors, msg)
	}

	// Check failed count
	failed := obj.statusInt("failed")
	if failed > 0 {
		errors = append(errors, fmt.Sprintf("%d failed pods", failed))
	}

	// Check for completion timeout or backoff
	if cond := obj.condition("Complete"); cond != nil && cond.isFalse() {
		if cond.Reason != "" {
			errors = append(errors, fmt.Sprintf("Not complete: %s", cond.Reason))
		}
	}

	return errors
}

func inferPVCErrors(obj *resourceData) []string {
	errors := make([]string, 0)

	phase := strings.ToLower(obj.statusString("phase"))
	switch phase {
	case podPhasePending:
		// Check for specific reasons
		conditions := obj.conditions()
		if len(conditions) > 0 {
			for _, cond := range conditions {
				if cond.isFalse() && cond.Reason != "" {
					msg := fmt.Sprintf("PVC pending: %s", cond.Reason)
					if cond.Message != "" {
						msg += fmt.Sprintf(" - %s", cond.Message)
					}
					errors = append(errors, msg)
				}
			}
		}
		if len(errors) == 0 {
			errors = append(errors, "PVC pending - waiting for volume provisioning")
		}
	case "lost":
		errors = append(errors, "PVC lost - volume no longer accessible")
	}

	return errors
}

func inferGenericErrors(obj *resourceData) []string {
	return extractConditionErrors(obj)
}

// extractConditionErrors extracts error messages from resource conditions
func extractConditionErrors(obj *resourceData) []string {
	errors := make([]string, 0)

	conditions := obj.conditions()
	if len(conditions) == 0 {
		return errors
	}

	// Check for common error conditions
	errorConditionTypes := []string{"Failed", "Failing", "Stalled", "Degraded"}
	for _, condType := range errorConditionTypes {
		if cond := findCondition(conditions, condType); cond != nil && cond.isTrue() {
			msg := fmt.Sprintf("%s: %s", condType, cond.Reason)
			if cond.Message != "" {
				msg += fmt.Sprintf(" - %s", cond.Message)
			}
			errors = append(errors, msg)
		}
	}

	// Check Ready/Healthy conditions that are false
	if cond := findCondition(conditions, "Ready"); cond != nil && cond.isFalse() {
		if cond.Reason != "" {
			msg := fmt.Sprintf("Not ready: %s", cond.Reason)
			if cond.Message != "" {
				msg += fmt.Sprintf(" - %s", cond.Message)
			}
			errors = append(errors, msg)
		}
	}

	if cond := findCondition(conditions, "Healthy"); cond != nil && cond.isFalse() {
		if cond.Reason != "" {
			msg := fmt.Sprintf("Not healthy: %s", cond.Reason)
			if cond.Message != "" {
				msg += fmt.Sprintf(" - %s", cond.Message)
			}
			errors = append(errors, msg)
		}
	}

	return errors
}
