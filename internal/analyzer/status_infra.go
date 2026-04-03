package analyzer

func inferPVCStatus(obj *resourceData) string {
	switch phase := obj.statusString("phase"); phase {
	case "Bound", "bound":
		return resourceStatusReady
	case "Pending", "pending":
		return resourceStatusWarning
	case "Lost", "lost":
		return resourceStatusError
	default:
		return ""
	}
}

func inferNodeStatus(obj *resourceData) string {
	readyCond := obj.condition("Ready")
	if readyCond == nil {
		return ""
	}
	if readyCond.isFalse() || readyCond.isUnknown() {
		return resourceStatusError
	}

	if cond := obj.condition("NetworkUnavailable"); cond != nil && cond.isTrue() {
		return resourceStatusError
	}

	for _, condType := range []string{"MemoryPressure", "DiskPressure", "PIDPressure"} {
		if cond := obj.condition(condType); cond != nil && cond.isTrue() {
			return resourceStatusWarning
		}
	}

	return resourceStatusReady
}
