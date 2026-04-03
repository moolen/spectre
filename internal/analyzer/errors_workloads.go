package analyzer

import (
	"fmt"
	"strings"
)

func inferPodErrors(obj *resourceData) []string {
	errors := make([]string, 0)

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

	phase := strings.ToLower(obj.statusString("phase"))
	switch phase {
	case podPhasePending:
		conditions := obj.conditions()
		if cond := findCondition(conditions, "PodScheduled"); cond != nil && cond.isFalse() {
			msg := fmt.Sprintf("Pod scheduling failed: %s", cond.Reason)
			if cond.Message != "" {
				msg += fmt.Sprintf(" - %s", cond.Message)
			}
			errors = append(errors, msg)
		} else if len(containerIssues) == 0 {
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

	if desired > 0 && ready < desired {
		errors = append(errors, fmt.Sprintf("Insufficient replicas (%d/%d ready)", ready, desired))
	}
	if unavailable > 0 {
		errors = append(errors, fmt.Sprintf("%d unavailable replicas", unavailable))
	}
	if desired > 0 && available < desired {
		errors = append(errors, fmt.Sprintf("Only %d/%d replicas available", available, desired))
	}

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

	errors = append(errors, extractConditionErrors(obj)...)
	return errors
}

func inferJobErrors(obj *resourceData) []string {
	errors := make([]string, 0)

	status := obj.status()
	if status == nil {
		return errors
	}

	if cond := obj.condition("Failed"); cond != nil && cond.isTrue() {
		msg := fmt.Sprintf("Job failed: %s", cond.Reason)
		if cond.Message != "" {
			msg += fmt.Sprintf(" - %s", cond.Message)
		}
		errors = append(errors, msg)
	}

	failed := obj.statusInt("failed")
	if failed > 0 {
		errors = append(errors, fmt.Sprintf("%d failed pods", failed))
	}

	if cond := obj.condition("Complete"); cond != nil && cond.isFalse() {
		if cond.Reason != "" {
			errors = append(errors, fmt.Sprintf("Not complete: %s", cond.Reason))
		}
	}

	return errors
}
