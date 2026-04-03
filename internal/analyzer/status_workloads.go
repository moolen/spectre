package analyzer

import "strings"

func inferDeploymentStatus(obj *resourceData) string {
	status := obj.status()
	if status == nil {
		return ""
	}

	desired := firstNonZero(obj.specInt("replicas"), obj.statusInt("replicas"))
	ready := obj.statusInt("readyReplicas")
	available := obj.statusInt("availableReplicas")
	unavailable := obj.statusInt("unavailableReplicas")

	if desired > 0 && ready >= desired && available >= desired && unavailable == 0 {
		return resourceStatusReady
	}

	if cond := obj.condition("Available"); cond != nil && cond.isFalse() {
		if cond.isErrorLike() {
			return resourceStatusError
		}
		return resourceStatusWarning
	}

	if unavailable > 0 {
		return resourceStatusWarning
	}

	if cond := obj.condition("Progressing"); cond != nil {
		if cond.isFalse() && cond.isErrorLike() {
			return resourceStatusError
		}
		if cond.isTrue() {
			return resourceStatusWarning
		}
	}

	if desired > 0 && available < desired {
		return resourceStatusWarning
	}

	return ""
}

func inferStatefulSetStatus(obj *resourceData) string {
	status := obj.status()
	if status == nil {
		return ""
	}

	desired := firstNonZero(obj.specInt("replicas"), obj.statusInt("replicas"))
	ready := obj.statusInt("readyReplicas")
	current := obj.statusInt("currentReplicas")

	if desired > 0 && ready >= desired {
		return resourceStatusReady
	}
	if desired > 0 && current < desired {
		return resourceStatusWarning
	}

	return ""
}

func inferDaemonSetStatus(obj *resourceData) string {
	status := obj.status()
	if status == nil {
		return ""
	}

	desired := obj.statusInt("desiredNumberScheduled")
	ready := obj.statusInt("numberReady")
	unavailable := obj.statusInt("numberUnavailable")
	misscheduled := obj.statusInt("numberMisscheduled")

	if desired > 0 && ready >= desired && unavailable == 0 && misscheduled == 0 {
		return resourceStatusReady
	}
	if unavailable > 0 || misscheduled > 0 {
		return resourceStatusWarning
	}

	return ""
}

func inferReplicaSetStatus(obj *resourceData) string {
	status := obj.status()
	if status == nil {
		return ""
	}

	desired := obj.specInt("replicas")
	ready := obj.statusInt("readyReplicas")
	available := obj.statusInt("availableReplicas")

	if desired > 0 && ready >= desired {
		if available > 0 && available >= desired {
			return resourceStatusReady
		}
		if available == 0 {
			return resourceStatusReady
		}
	}

	if desired > 0 && ready < desired {
		return resourceStatusWarning
	}
	if desired > 0 && available > 0 && available < desired {
		return resourceStatusWarning
	}

	return ""
}

func inferPodStatus(obj *resourceData) string {
	status := obj.status()
	if status == nil {
		return ""
	}

	containerIssues := InspectContainerStates(obj)
	for _, issue := range containerIssues {
		if issue.IssueType == issueTypeImagePullBackOff {
			return resourceStatusError
		}
	}

	if HasCriticalContainerIssues(containerIssues) {
		return resourceStatusError
	}
	if len(containerIssues) > 0 {
		return resourceStatusWarning
	}

	switch strings.ToLower(obj.statusString("phase")) {
	case "running":
		if cond := obj.condition("Ready"); cond != nil && cond.isTrue() {
			return resourceStatusReady
		}
		return resourceStatusWarning
	case "pending":
		return resourceStatusWarning
	case "succeeded":
		return resourceStatusReady
	case "failed":
		return resourceStatusError
	case "unknown":
		return resourceStatusWarning
	default:
		return ""
	}
}

func inferJobStatus(obj *resourceData) string {
	status := obj.status()
	if status == nil {
		return ""
	}

	if cond := obj.condition("Complete"); cond != nil && cond.isTrue() {
		return resourceStatusReady
	}
	if cond := obj.condition("Failed"); cond != nil && cond.isTrue() {
		return resourceStatusError
	}

	if obj.statusInt("succeeded") > 0 {
		return resourceStatusReady
	}
	if obj.statusInt("failed") > 0 {
		return resourceStatusError
	}
	if obj.statusInt("active") > 0 {
		return resourceStatusReady
	}

	return ""
}
