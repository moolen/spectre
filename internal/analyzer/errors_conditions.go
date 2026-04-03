package analyzer

import "fmt"

func inferGenericErrors(obj *resourceData) []string {
	return extractConditionErrors(obj)
}

// extractConditionErrors extracts error messages from resource conditions.
func extractConditionErrors(obj *resourceData) []string {
	errors := make([]string, 0)

	conditions := obj.conditions()
	if len(conditions) == 0 {
		return errors
	}

	for _, condType := range []string{"Failed", "Failing", "Stalled", "Degraded"} {
		if cond := findCondition(conditions, condType); cond != nil && cond.isTrue() {
			msg := fmt.Sprintf("%s: %s", condType, cond.Reason)
			if cond.Message != "" {
				msg += fmt.Sprintf(" - %s", cond.Message)
			}
			errors = append(errors, msg)
		}
	}

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
