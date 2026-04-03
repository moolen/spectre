package analyzer

import (
	"encoding/json"
	"strings"
)

const (
	resourceStatusReady       = "Ready"
	resourceStatusWarning     = "Warning"
	resourceStatusError       = "Error"
	resourceStatusTerminating = "Terminating"
	resourceStatusUnknown     = "Unknown"
)

// InferStatusFromResource inspects the resource payload and produces a best-effort status.
func InferStatusFromResource(kind string, data json.RawMessage, eventType string) string {
	if strings.EqualFold(eventType, "DELETE") {
		return resourceStatusTerminating
	}

	if len(data) == 0 {
		return inferStatusFromEventType(eventType)
	}

	obj, err := newResourceData(data)
	if err != nil {
		return inferStatusFromEventType(eventType)
	}

	return InferStatusFromParsedData(kind, obj, eventType)
}

// InferStatusFromParsedData inspects pre-parsed resource data and produces a best-effort status.
func InferStatusFromParsedData(kind string, obj *ResourceData, eventType string) string {
	if strings.EqualFold(eventType, "DELETE") {
		return resourceStatusTerminating
	}

	if obj == nil {
		return inferStatusFromEventType(eventType)
	}
	if obj.isDeleting() {
		return resourceStatusTerminating
	}

	status := inferResourceSpecificStatus(strings.ToLower(kind), obj)
	if status != "" {
		return status
	}

	conditionStatus := inferStatusFromConditions(obj.conditions())
	if conditionStatus != "" {
		return conditionStatus
	}

	return inferStatusFromEventType(eventType)
}

func inferResourceSpecificStatus(kind string, obj *resourceData) string {
	switch kind {
	case "deployment":
		return inferDeploymentStatus(obj)
	case "statefulset":
		return inferStatefulSetStatus(obj)
	case "daemonset":
		return inferDaemonSetStatus(obj)
	case "replicaset":
		return inferReplicaSetStatus(obj)
	case "pod":
		return inferPodStatus(obj)
	case "persistentvolumeclaim":
		return inferPVCStatus(obj)
	case "node":
		return inferNodeStatus(obj)
	case "job":
		return inferJobStatus(obj)
	case "service", "configmap", "secret":
		return resourceStatusReady
	default:
		return ""
	}
}
