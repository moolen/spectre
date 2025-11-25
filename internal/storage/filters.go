package storage

import (
	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/models"
)

// FilterEngine handles filtering of events based on query filters
type FilterEngine struct {
	logger *logging.Logger
}

// NewFilterEngine creates a new filter engine
func NewFilterEngine() *FilterEngine {
	return &FilterEngine{
		logger: logging.GetLogger("filter"),
	}
}

// FilterEvents filters a list of events based on the given filters
func (fe *FilterEngine) FilterEvents(events []models.Event, filters models.QueryFilters) []models.Event {
	if filters.IsEmpty() {
		// No filters, return all events
		return events
	}

	var filtered []models.Event

	for _, event := range events {
		if fe.MatchesFilters(&event, filters) {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// MatchesFilters checks if an event matches the given filters
func (fe *FilterEngine) MatchesFilters(event *models.Event, filters models.QueryFilters) bool {
	return filters.Matches(event.Resource)
}

// FilterByNamespace filters events by namespace
func (fe *FilterEngine) FilterByNamespace(events []models.Event, namespace string) []models.Event {
	if namespace == "" {
		return events
	}

	var filtered []models.Event
	for _, event := range events {
		if event.Resource.Namespace == namespace {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// FilterByKind filters events by resource kind
func (fe *FilterEngine) FilterByKind(events []models.Event, kind string) []models.Event {
	if kind == "" {
		return events
	}

	var filtered []models.Event
	for _, event := range events {
		if event.Resource.Kind == kind {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// FilterByGroup filters events by API group
func (fe *FilterEngine) FilterByGroup(events []models.Event, group string) []models.Event {
	if group == "" {
		return events
	}

	var filtered []models.Event
	for _, event := range events {
		if event.Resource.Group == group {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// FilterByVersion filters events by API version
func (fe *FilterEngine) FilterByVersion(events []models.Event, version string) []models.Event {
	if version == "" {
		return events
	}

	var filtered []models.Event
	for _, event := range events {
		if event.Resource.Version == version {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// FilterByTimeRange filters events by timestamp range
func (fe *FilterEngine) FilterByTimeRange(events []models.Event, startTime, endTime int64) []models.Event {
	var filtered []models.Event

	for _, event := range events {
		if event.Timestamp >= startTime && event.Timestamp <= endTime {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// FilterByType filters events by event type (CREATE, UPDATE, DELETE)
func (fe *FilterEngine) FilterByType(events []models.Event, eventType models.EventType) []models.Event {
	if eventType == "" {
		return events
	}

	var filtered []models.Event
	for _, event := range events {
		if event.Type == eventType {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// GetFilterSummary returns a summary of the filters
func (fe *FilterEngine) GetFilterSummary(filters models.QueryFilters) map[string]string {
	summary := make(map[string]string)

	if filters.Group != "" {
		summary["group"] = filters.Group
	}
	if filters.Version != "" {
		summary["version"] = filters.Version
	}
	if filters.Kind != "" {
		summary["kind"] = filters.Kind
	}
	if filters.Namespace != "" {
		summary["namespace"] = filters.Namespace
	}

	if len(summary) == 0 {
		summary["filters"] = "none"
	}

	return summary
}

// AreFiltersCompatible checks if two filter sets are compatible (no conflicting criteria)
func (fe *FilterEngine) AreFiltersCompatible(filters1, filters2 models.QueryFilters) bool {
	// Filters are compatible if they don't contradict each other
	// For example, same kind filter values must match

	if filters1.Group != "" && filters2.Group != "" && filters1.Group != filters2.Group {
		return false
	}
	if filters1.Version != "" && filters2.Version != "" && filters1.Version != filters2.Version {
		return false
	}
	if filters1.Kind != "" && filters2.Kind != "" && filters1.Kind != filters2.Kind {
		return false
	}
	if filters1.Namespace != "" && filters2.Namespace != "" && filters1.Namespace != filters2.Namespace {
		return false
	}

	return true
}
