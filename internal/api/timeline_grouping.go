package api

import (
	"sort"

	apptimeline "github.com/moolen/spectre/internal/app/timeline"
)

type GroupedTimelineEntries struct {
	Kind    string
	Entries []*apptimeline.TimelineResourceEntry
}

func resourceIdentityLess(kindA, namespaceA, nameA, kindB, namespaceB, nameB string) bool {
	if kindA != kindB {
		return kindA < kindB
	}
	if namespaceA != namespaceB {
		return namespaceA < namespaceB
	}
	return nameA < nameB
}

func sortTimelineEntries(entries []*apptimeline.TimelineResourceEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return resourceIdentityLess(
			entries[i].Kind, entries[i].Namespace, entries[i].Name,
			entries[j].Kind, entries[j].Namespace, entries[j].Name,
		)
	})
}

func groupAndSortTimelineEntries(entries []*apptimeline.TimelineResourceEntry) []*GroupedTimelineEntries {
	groups := make(map[string][]*apptimeline.TimelineResourceEntry)
	for _, entry := range entries {
		groups[entry.Kind] = append(groups[entry.Kind], entry)
	}

	kinds := make([]string, 0, len(groups))
	for kind := range groups {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)

	grouped := make([]*GroupedTimelineEntries, 0, len(groups))
	for _, kind := range kinds {
		groupEntries := groups[kind]
		sortTimelineEntries(groupEntries)
		grouped = append(grouped, &GroupedTimelineEntries{
			Kind:    kind,
			Entries: groupEntries,
		})
	}

	return grouped
}
