package api

import (
	"sort"

	apptimeline "github.com/moolen/spectre/internal/app/timeline"
	"github.com/moolen/spectre/internal/models"
)

// GroupedResources groups resources by kind
type GroupedResources struct {
	Kind      string
	Resources []models.Resource
}

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

// groupResourcesByKind groups resources by kind and sorts kinds alphabetically
func groupResourcesByKind(resources []models.Resource) []*GroupedResources {
	groups := make(map[string][]models.Resource)

	for _, r := range resources {
		groups[r.Kind] = append(groups[r.Kind], r)
	}

	// Convert to slice and sort kinds alphabetically
	kinds := make([]string, 0, len(groups))
	for k := range groups {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)

	result := make([]*GroupedResources, 0, len(groups))
	for _, kind := range kinds {
		result = append(result, &GroupedResources{
			Kind:      kind,
			Resources: groups[kind],
		})
	}

	return result
}

// sortResourcesByNamespaceAndName sorts resources within a group by namespace, then name
func sortResourcesByNamespaceAndName(resources []models.Resource) {
	sort.Slice(resources, func(i, j int) bool {
		return resourceIdentityLess(
			resources[i].Kind, resources[i].Namespace, resources[i].Name,
			resources[j].Kind, resources[j].Namespace, resources[j].Name,
		)
	})
}

// groupAndSortResources groups resources by kind and sorts them within each group
func groupAndSortResources(resources []models.Resource) []*GroupedResources {
	grouped := groupResourcesByKind(resources)

	// Sort resources within each kind group
	for _, group := range grouped {
		sortResourcesByNamespaceAndName(group.Resources)
	}

	return grouped
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
