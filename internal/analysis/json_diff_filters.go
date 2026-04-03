package analysis

import "strings"

// FilterNoisyPaths removes paths that are typically noisy and not useful for LLM analysis.
func FilterNoisyPaths(diffs []EventDiff) []EventDiff {
	noisyPrefixes := []string{
		"metadata.managedFields",
		"metadata.resourceVersion",
		"metadata.generation",
		"metadata.uid",
		"metadata.creationTimestamp",
		"status.observedGeneration",
	}

	var filtered []EventDiff
	for _, d := range diffs {
		isNoisy := false
		for _, prefix := range noisyPrefixes {
			if strings.HasPrefix(d.Path, prefix) {
				isNoisy = true
				break
			}
		}
		if !isNoisy {
			filtered = append(filtered, d)
		}
	}

	return filtered
}

// FilterSpecOnly filters diffs to only include spec changes.
func FilterSpecOnly(diffs []EventDiff) []EventDiff {
	excludePrefixes := []string{
		"status",
		"metadata.managedFields",
		"metadata.resourceVersion",
		"metadata.generation",
		"metadata.uid",
		"metadata.creationTimestamp",
	}

	var filtered []EventDiff
	for _, d := range diffs {
		exclude := false
		for _, prefix := range excludePrefixes {
			if d.Path == prefix || strings.HasPrefix(d.Path, prefix+".") {
				exclude = true
				break
			}
		}
		if !exclude {
			filtered = append(filtered, d)
		}
	}

	return filtered
}
