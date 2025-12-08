package main

import (
	"github.com/moolen/spectre/internal/models"
)

// EventFilter holds filtering criteria
type EventFilter struct {
	Group     string
	Version   string
	Kind      string
	Namespace string
	Name      string
}

// matchesFilter checks if an event matches all specified filter criteria (AND logic)
func matchesFilter(event *models.Event, filter *EventFilter) bool {
	res := &event.Resource

	// All filters must match (AND logic)
	if filter.Group != "" && res.Group != filter.Group {
		return false
	}
	if filter.Version != "" && res.Version != filter.Version {
		return false
	}
	if filter.Kind != "" && res.Kind != filter.Kind {
		return false
	}
	if filter.Namespace != "" && res.Namespace != filter.Namespace {
		return false
	}
	if filter.Name != "" && res.Name != filter.Name {
		return false
	}

	return true
}

// filterEvents returns only events matching the filter criteria
func filterEvents(events []*models.Event, filter *EventFilter) []*models.Event {
	if isEmptyFilter(filter) {
		return events
	}

	var filtered []*models.Event
	for _, event := range events {
		if matchesFilter(event, filter) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// isEmptyFilter checks if no filters are set
func isEmptyFilter(filter *EventFilter) bool {
	return filter.Group == "" &&
		filter.Version == "" &&
		filter.Kind == "" &&
		filter.Namespace == "" &&
		filter.Name == ""
}

// getUniqueResources returns unique resources from events
func getUniqueResources(events []*models.Event) map[string]*models.Event {
	resources := make(map[string]*models.Event)
	for _, event := range events {
		key := resourceKey(&event.Resource)
		// Keep the most recent event for each resource
		if existing, exists := resources[key]; !exists || event.Timestamp > existing.Timestamp {
			resources[key] = event
		}
	}
	return resources
}

// getUniqueCounts returns counts of unique groups, versions, kinds, namespaces, and names
func getUniqueCounts(events []*models.Event) map[string]int {
	groups := make(map[string]bool)
	versions := make(map[string]bool)
	kinds := make(map[string]bool)
	namespaces := make(map[string]bool)
	names := make(map[string]bool)

	for _, event := range events {
		groups[event.Resource.Group] = true
		versions[event.Resource.Version] = true
		kinds[event.Resource.Kind] = true
		namespaces[event.Resource.Namespace] = true
		names[event.Resource.Name] = true
	}

	return map[string]int{
		"groups":     len(groups),
		"versions":   len(versions),
		"kinds":      len(kinds),
		"namespaces": len(namespaces),
		"names":      len(names),
	}
}

// resourceKey creates a unique key for a resource
func resourceKey(res *models.ResourceMetadata) string {
	if res == nil {
		return ""
	}
	if res.Namespace != "" {
		return res.Group + "/" + res.Version + "/" + res.Kind + "/" + res.Namespace + "/" + res.Name
	}
	return res.Group + "/" + res.Version + "/" + res.Kind + "/" + res.Name
}
