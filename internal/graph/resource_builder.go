package graph

import (
	"sort"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// ResourceBuilder converts ChangeEvent sequences into StatusSegments
// This is the graph equivalent of storage/resource_builder.go
type ResourceBuilder struct {
	logger *logging.Logger
}

// NewResourceBuilder creates a new ResourceBuilder
func NewResourceBuilder() *ResourceBuilder {
	return &ResourceBuilder{
		logger: logging.GetLogger("graph.resource_builder"),
	}
}

// BuildStatusSegments converts a sequence of ChangeEvents into StatusSegments
// This merges consecutive events with the same status into single segments
func (rb *ResourceBuilder) BuildStatusSegments(events []ChangeEvent, queryStartNs, queryEndNs int64) []models.StatusSegment {
	if len(events) == 0 {
		return nil
	}

	// Sort events by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp < events[j].Timestamp
	})

	var segments []models.StatusSegment
	var currentSegment *models.StatusSegment

	for _, event := range events {
		// Skip events outside query range
		if event.Timestamp < queryStartNs || event.Timestamp > queryEndNs {
			continue
		}

		// Create or extend segment
		if currentSegment == nil || currentSegment.Status != event.Status {
			// Finalize previous segment
			if currentSegment != nil {
				currentSegment.EndTime = event.Timestamp
				segments = append(segments, *currentSegment)
			}

			// Start new segment
			currentSegment = &models.StatusSegment{
				StartTime: event.Timestamp,
				EndTime:   queryEndNs, // Will be updated by next event or remain as query end
				Status:    event.Status,
				Message:   event.ErrorMessage,
			}
		} else if event.ErrorMessage != "" {
			// Same status - extend current segment
			// Update message if more recent event has one
			currentSegment.Message = event.ErrorMessage
		}
	}

	// Finalize last segment
	if currentSegment != nil {
		segments = append(segments, *currentSegment)
	}

	return segments
}

// BuildResourcesFromChangeEvents converts graph query results into Resource objects
// This groups events by resource and builds status segments for each
func (rb *ResourceBuilder) BuildResourcesFromChangeEvents(
	resourceNodes []ResourceIdentity,
	changeEventsByResource map[string][]ChangeEvent,
	k8sEventsByResource map[string][]K8sEvent,
	queryStartNs, queryEndNs int64,
) []models.Resource {
	resources := make([]models.Resource, 0, len(resourceNodes))

	for _, resNode := range resourceNodes {
		changeEvents := changeEventsByResource[resNode.UID]
		k8sEvents := k8sEventsByResource[resNode.UID]

		resource := models.Resource{
			ID:        resNode.UID,
			Group:     resNode.APIGroup,
			Version:   resNode.Version,
			Kind:      resNode.Kind,
			Namespace: resNode.Namespace,
			Name:      resNode.Name,
		}

		// Build status segments
		resource.StatusSegments = rb.BuildStatusSegments(changeEvents, queryStartNs, queryEndNs)

		// Check if pre-existing (has events before query start)
		resource.PreExisting = rb.isPreExisting(resNode, changeEvents, queryStartNs)

		// Convert K8s events
		resource.Events = rb.convertK8sEvents(k8sEvents)

		resources = append(resources, resource)
	}

	return resources
}

// isPreExisting checks if a resource existed before the query start time
func (rb *ResourceBuilder) isPreExisting(resource ResourceIdentity, events []ChangeEvent, queryStartNs int64) bool {
	// If firstSeen is before query start, it's pre-existing
	if resource.FirstSeen < queryStartNs {
		return true
	}

	// If there are events before query start, it's pre-existing
	for _, event := range events {
		if event.Timestamp < queryStartNs {
			return true
		}
	}

	return false
}

// convertK8sEvents converts graph K8sEvent nodes to models.K8sEvent
func (rb *ResourceBuilder) convertK8sEvents(events []K8sEvent) []models.K8sEvent {
	result := make([]models.K8sEvent, 0, len(events))

	for _, event := range events {
		result = append(result, models.K8sEvent{
			ID:        event.ID,
			Timestamp: event.Timestamp,
			Reason:    event.Reason,
			Message:   event.Message,
			Type:      event.Type,
			// K8s event counts are typically small (< 1000 for repeated events)
			// #nosec G115 -- Event count from K8s API fits in int32
			Count:  int32(event.Count),
			Source: event.Source,
		})
	}

	// Sort by timestamp
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp < result[j].Timestamp
	})

	return result
}

// CreateLeadingSegment creates a synthetic segment for pre-existing resources
// This fills the gap between query start and the first event
func (rb *ResourceBuilder) CreateLeadingSegment(
	resource ResourceIdentity,
	firstEvent *ChangeEvent,
	queryStartNs int64,
) *models.StatusSegment {
	if firstEvent == nil || firstEvent.Timestamp <= queryStartNs {
		return nil
	}

	// Determine status from the first event or default to Unknown
	status := "Unknown"
	if firstEvent.Status != "" {
		status = firstEvent.Status
	}

	return &models.StatusSegment{
		StartTime: queryStartNs,
		EndTime:   firstEvent.Timestamp,
		Status:    status,
		Message:   "Pre-existing resource",
	}
}
