package analyzer

import (
	"fmt"
	"strings"
)

func inferNodeErrors(obj *resourceData) []string {
	errors := make([]string, 0)

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

	if cond := obj.condition("NetworkUnavailable"); cond != nil && cond.isTrue() {
		msg := "Network unavailable"
		if cond.Reason != "" {
			msg += fmt.Sprintf(": %s", cond.Reason)
		}
		errors = append(errors, msg)
	}

	for _, condType := range []string{"MemoryPressure", "DiskPressure", "PIDPressure"} {
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

func inferPVCErrors(obj *resourceData) []string {
	errors := make([]string, 0)

	phase := strings.ToLower(obj.statusString("phase"))
	switch phase {
	case podPhasePending:
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
