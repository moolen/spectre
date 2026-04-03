package analyzer

import (
	"encoding/json"
	"strings"
)

const podPhasePending = "pending"

// InferErrorMessages extracts detailed error messages from resource data.
// Returns a slice of error strings describing what is wrong with the resource.
func InferErrorMessages(kind string, data json.RawMessage, status string) []string {
	if len(data) == 0 {
		return nil
	}

	// Only extract errors for Warning and Error statuses.
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
		return inferGenericErrors(obj)
	}
}
