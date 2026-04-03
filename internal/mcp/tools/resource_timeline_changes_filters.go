package tools

import (
	"strings"

	"github.com/moolen/spectre/internal/analysis"
)

// filterDiffsByPath filters diffs based on the change filter setting.
func filterDiffsByPath(diffs []analysis.EventDiff, changeFilter string) []analysis.EventDiff {
	if changeFilter == ChangeFilterAll {
		return diffs
	}

	filtered := make([]analysis.EventDiff, 0, len(diffs))
	for _, diff := range diffs {
		isStatusPath := strings.HasPrefix(diff.Path, ".status") || strings.HasPrefix(diff.Path, "status")

		switch changeFilter {
		case ChangeFilterSpecOnly:
			if !isStatusPath {
				filtered = append(filtered, diff)
			}
		case ChangeFilterStatusOnly:
			if isStatusPath {
				filtered = append(filtered, diff)
			}
		}
	}

	return filtered
}
