package analysis

import (
	"strconv"
	"strings"
)

// GetHighestSeverityPattern returns the pattern with the highest severity.
func GetHighestSeverityPattern(patterns []DetectedPattern) *DetectedPattern {
	if len(patterns) == 0 {
		return nil
	}

	highest := &patterns[0]
	for i := range patterns {
		if patterns[i].Severity > highest.Severity {
			highest = &patterns[i]
		}
	}
	return highest
}

// ExtractContainerIndexFromPath extracts the container index from a path like
// "status.containerStatuses.2.state.terminated.exitCode" and returns "2".
func ExtractContainerIndexFromPath(path string) string {
	parts := strings.Split(path, ".")
	for i, part := range parts {
		if (part == "containerStatuses" || part == "initContainerStatuses") && i+1 < len(parts) {
			if _, err := strconv.Atoi(parts[i+1]); err == nil {
				return parts[i+1]
			}
		}
	}
	return ""
}
