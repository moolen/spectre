package embeddedstore

import (
	"sort"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
)

func mergeChangeEvents(configEvents, recentEvents []analysisstore.ChangeEventInfo) []analysisstore.ChangeEventInfo {
	if len(configEvents) == 0 && len(recentEvents) == 0 {
		return nil
	}

	combined := make([]analysisstore.ChangeEventInfo, 0, len(configEvents)+len(recentEvents))
	seen := make(map[string]bool, len(configEvents)+len(recentEvents))
	appendUnique := func(events []analysisstore.ChangeEventInfo) {
		for _, event := range events {
			if seen[event.EventID] {
				continue
			}
			seen[event.EventID] = true
			combined = append(combined, event)
		}
	}

	appendUnique(configEvents)
	appendUnique(recentEvents)

	sort.Slice(combined, func(i, j int) bool {
		if combined[i].Timestamp.Equal(combined[j].Timestamp) {
			return combined[i].EventID > combined[j].EventID
		}
		return combined[i].Timestamp.After(combined[j].Timestamp)
	})

	return combined
}
