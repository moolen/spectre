package tools

import (
	"sort"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/models"
)

// processResource computes semantic changes for a single resource.
func (t *ResourceTimelineChangesTool) processResource(resource models.Resource, maxChanges int, includeSnapshot bool, changeFilter string) ResourceTimelineEntry {
	entry := ResourceTimelineEntry{
		UID:       resource.ID,
		Kind:      resource.Kind,
		Namespace: resource.Namespace,
		Name:      resource.Name,
		StatusSummary: StatusConditionSummary{
			Transitions:      make([]StatusTransitionSummary, 0),
			ConditionHistory: make(map[string]string),
		},
	}

	segments := resource.StatusSegments
	if len(segments) == 0 {
		return entry
	}

	sort.Slice(segments, func(i, j int) bool {
		return segments[i].StartTime < segments[j].StartTime
	})

	entry.StatusSummary.CurrentStatus = segments[len(segments)-1].Status
	if includeSnapshot {
		entry.FirstSnapshot = firstResourceSnapshot(segments)
	}

	allDiffs := collectTimelineDiffs(segments, changeFilter, &entry)
	if len(allDiffs) > maxChanges {
		allDiffs = allDiffs[:maxChanges]
	}

	entry.UnifiedDiff = analysis.FormatUnifiedDiff(allDiffs)
	entry.ChangeCount = len(allDiffs)
	entry.StatusSummary.ConditionHistory = t.summarizeConditions(segments)

	return entry
}

func firstResourceSnapshot(segments []models.StatusSegment) map[string]any {
	if len(segments) == 0 || len(segments[0].ResourceData) == 0 {
		return nil
	}

	snapshot, err := analysis.ParseJSONToMap(segments[0].ResourceData)
	if err != nil || snapshot == nil {
		return nil
	}

	return snapshot
}

func collectTimelineDiffs(segments []models.StatusSegment, changeFilter string, entry *ResourceTimelineEntry) []analysis.EventDiff {
	allDiffs := make([]analysis.EventDiff, 0)
	for i := 1; i < len(segments); i++ {
		prevSegment := segments[i-1]
		currSegment := segments[i]

		diffs, err := analysis.ComputeJSONDiff(prevSegment.ResourceData, currSegment.ResourceData)
		if err != nil {
			continue
		}

		diffs = analysis.FilterNoisyPaths(diffs)
		diffs = filterDiffsByPath(diffs, changeFilter)
		allDiffs = append(allDiffs, diffs...)

		if prevSegment.Status != currSegment.Status {
			entry.StatusSummary.Transitions = append(entry.StatusSummary.Transitions, StatusTransitionSummary{
				FromStatus: prevSegment.Status,
				ToStatus:   currSegment.Status,
				Timestamp:  currSegment.StartTime,
				Reason:     currSegment.Message,
			})
		}
	}

	return allDiffs
}
