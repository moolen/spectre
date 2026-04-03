package analyzer

import "strings"

func inferStatusFromConditions(conditions []condition) string {
	if len(conditions) == 0 {
		return ""
	}

	if cond := findCondition(conditions, "Ready"); cond != nil {
		if cond.isTrue() {
			return resourceStatusReady
		}
		if cond.isFalse() {
			if cond.isErrorLike() {
				return resourceStatusError
			}
			return resourceStatusWarning
		}
		if cond.isUnknown() {
			return resourceStatusWarning
		}
	}

	if cond := findCondition(conditions, "Healthy"); cond != nil {
		if cond.isTrue() {
			return resourceStatusReady
		}
		if cond.isFalse() {
			if cond.isErrorLike() {
				return resourceStatusError
			}
			return resourceStatusWarning
		}
	}

	for _, name := range []string{"Stalled", "Degraded", "Failing", "Failed"} {
		if cond := findCondition(conditions, name); cond != nil && cond.isTrue() {
			if name == "Degraded" {
				return resourceStatusWarning
			}
			return resourceStatusError
		}
	}

	for _, name := range []string{"Reconciling", "Progressing"} {
		if cond := findCondition(conditions, name); cond != nil && cond.isTrue() {
			return resourceStatusWarning
		}
	}

	return ""
}

func inferStatusFromEventType(eventType string) string {
	switch strings.ToUpper(eventType) {
	case "CREATE", "UPDATE":
		return resourceStatusReady
	case "DELETE":
		return resourceStatusTerminating
	default:
		return resourceStatusUnknown
	}
}

func findCondition(conditions []condition, condType string) *condition {
	for _, cond := range conditions {
		if strings.EqualFold(cond.Type, condType) {
			c := cond
			return &c
		}
	}
	return nil
}

func containsErrorKeyword(text string) bool {
	textLower := strings.ToLower(text)
	if textLower == "" {
		return false
	}

	for _, keyword := range []string{"error", "fail", "invalid", "crash", "timeout", "stalled", "deadline", "exceeded"} {
		if strings.Contains(textLower, keyword) {
			return true
		}
	}
	return false
}

func firstNonZero(values ...int64) int64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}
