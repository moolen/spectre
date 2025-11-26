package storage

import (
	"sort"
	"strings"

	"github.com/moritz/rpk/internal/models"
)

// ResourceBuilder aggregates events into resources with status segments and audit events
type ResourceBuilder struct{}

// NewResourceBuilder creates a new ResourceBuilder
func NewResourceBuilder() *ResourceBuilder {
	return &ResourceBuilder{}
}

// BuildResourcesFromEvents groups events by resource UID and creates Resource objects
func (rb *ResourceBuilder) BuildResourcesFromEvents(events []models.Event) map[string]*models.Resource {
	resources := make(map[string]*models.Resource)

	for _, event := range events {
		resourceUID := event.Resource.UID
		if resourceUID == "" {
			continue
		}

		if _, exists := resources[resourceUID]; !exists {
			// Create new resource
			resource := rb.CreateResource(event)
			resources[resourceUID] = resource
		}

		// Add event to resource
		if resources[resourceUID].Events == nil {
			resources[resourceUID].Events = []models.AuditEvent{}
		}
		resources[resourceUID].Events = append(resources[resourceUID].Events, rb.transformEvent(event))
	}

	// Build status segments for each resource
	for uid, resource := range resources {
		if len(resource.Events) > 0 {
			resource.StatusSegments = rb.BuildStatusSegments(uid, events)
		}
	}

	return resources
}

// CreateResource extracts metadata from an Event and creates a Resource object
func (rb *ResourceBuilder) CreateResource(event models.Event) *models.Resource {
	return &models.Resource{
		ID:        event.Resource.UID,
		Group:     event.Resource.Group,
		Version:   event.Resource.Version,
		Kind:      event.Resource.Kind,
		Namespace: event.Resource.Namespace,
		Name:      event.Resource.Name,
		Events:    []models.AuditEvent{},
	}
}

// BuildStatusSegments derives status segments from event timeline
func (rb *ResourceBuilder) BuildStatusSegments(resourceUID string, allEvents []models.Event) []models.StatusSegment {
	// Filter events for this resource and sort by timestamp
	var resourceEvents []models.Event
	for _, event := range allEvents {
		if event.Resource.UID == resourceUID {
			resourceEvents = append(resourceEvents, event)
		}
	}

	// Sort by timestamp ascending
	sort.Slice(resourceEvents, func(i, j int) bool {
		return resourceEvents[i].Timestamp < resourceEvents[j].Timestamp
	})

	var segments []models.StatusSegment

	for i, event := range resourceEvents {
		var endTime int64
		if i+1 < len(resourceEvents) {
			endTime = resourceEvents[i+1].Timestamp
		} else {
			// For the last event, end time is current time
			endTime = event.Timestamp + 3600*1e9 // 1 hour after
		}

		status := rb.inferStatus(string(event.Type))
		segment := models.StatusSegment{
			StartTime: event.Timestamp / 1e9, // Convert to seconds
			EndTime:   endTime / 1e9,
			Status:    status,
			Message:   "",
			Config:    make(map[string]interface{}),
		}
		segments = append(segments, segment)
	}

	return segments
}

// BuildAuditEvents converts storage Event objects to AuditEvent objects
func (rb *ResourceBuilder) BuildAuditEvents(events []models.Event) []models.AuditEvent {
	var auditEvents []models.AuditEvent
	for _, event := range events {
		auditEvents = append(auditEvents, rb.transformEvent(event))
	}
	return auditEvents
}

// transformEvent converts a models.Event to an AuditEvent
func (rb *ResourceBuilder) transformEvent(event models.Event) models.AuditEvent {
	return models.AuditEvent{
		ID:        event.ID,
		Timestamp: event.Timestamp / 1e9, // Convert nanoseconds to seconds
		Verb:      rb.mapVerb(string(event.Type)),
		User:      "", // Will be populated from audit logs in production
		Message:   "",
		Details:   "",
	}
}

// mapVerb maps event type to standard API verb
func (rb *ResourceBuilder) mapVerb(eventType string) string {
	verbMap := map[string]string{
		"CREATE": "create",
		"UPDATE": "update",
		"DELETE": "delete",
	}
	if mapped, exists := verbMap[strings.ToUpper(eventType)]; exists {
		return mapped
	}
	return strings.ToLower(eventType)
}

// inferStatus infers resource status from event type
func (rb *ResourceBuilder) inferStatus(eventType string) string {
	typeUpper := strings.ToUpper(eventType)
	switch typeUpper {
	case "CREATE", "UPDATE":
		return "Ready"
	case "DELETE":
		return "Terminating"
	default:
		return "Unknown"
	}
}
