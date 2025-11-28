package storage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/models"
	corev1 "k8s.io/api/core/v1"
)

// ResourceBuilder aggregates events into resources with status segments and related Kubernetes events
type ResourceBuilder struct {
	logger *logging.Logger
}

// NewResourceBuilder creates a new ResourceBuilder
func NewResourceBuilder() *ResourceBuilder {
	return &ResourceBuilder{
		logger: logging.GetLogger("resource_builder"),
	}
}

// BuildResourcesFromEvents groups events by resource UID and creates Resource objects
func (rb *ResourceBuilder) BuildResourcesFromEvents(events []models.Event) map[string]*models.Resource {
	resources := make(map[string]*models.Resource)

	// Filter out Kubernetes Event resources; they will be processed separately
	baseEvents := make([]models.Event, 0, len(events))
	for _, event := range events {
		if strings.EqualFold(event.Resource.Kind, "Event") {
			continue
		}
		baseEvents = append(baseEvents, event)
	}

	for _, event := range baseEvents {
		resourceUID := event.Resource.UID
		if resourceUID == "" {
			continue
		}

		if _, exists := resources[resourceUID]; !exists {
			// Create new resource
			resource := rb.CreateResource(event)
			resources[resourceUID] = resource
		}
	}

	// Build status segments for each resource
	for uid, resource := range resources {
		resource.StatusSegments = rb.BuildStatusSegments(uid, baseEvents)
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
		Events:    nil,
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

		segment := models.StatusSegment{
			StartTime:    event.Timestamp / 1e9, // Convert to seconds
			EndTime:      endTime / 1e9,
			Status:       InferStatusFromResource(event.Resource.Kind, event.Data, string(event.Type)),
			Message:      rb.generateMessage(event),
			ResourceData: event.Data,
		}
		segments = append(segments, segment)
	}

	return segments
}

// generateMessage creates a human-readable message for the segment
func (rb *ResourceBuilder) generateMessage(event models.Event) string {
	verb := rb.mapVerb(string(event.Type))

	switch verb {
	case "create":
		return "Resource created"
	case "update":
		return "Resource updated"
	case "delete":
		return "Resource deleted"
	default:
		return "Resource modified"
	}
}

// AttachK8sEvents augments resources with Kubernetes Event data matched by involvedObject UID.
func (rb *ResourceBuilder) AttachK8sEvents(resources map[string]*models.Resource, events []models.Event) {
	if len(resources) == 0 || len(events) == 0 {
		return
	}

	for _, event := range events {
		if !strings.EqualFold(event.Resource.Kind, "Event") {
			continue
		}
		targetUID := event.Resource.InvolvedObjectUID
		if targetUID == "" {
			continue
		}

		resource, exists := resources[targetUID]
		if !exists {
			continue
		}

		k8sEvent, err := rb.convertK8sEvent(event)
		if err != nil {
			rb.logger.Warn("Failed to convert Kubernetes Event %s: %v", event.ID, err)
			continue
		}

		resource.Events = append(resource.Events, k8sEvent)
	}

	for _, resource := range resources {
		if len(resource.Events) == 0 {
			continue
		}
		sort.Slice(resource.Events, func(i, j int) bool {
			return resource.Events[i].Timestamp < resource.Events[j].Timestamp
		})
	}
}

func (rb *ResourceBuilder) convertK8sEvent(event models.Event) (models.K8sEvent, error) {
	if len(event.Data) == 0 {
		return models.K8sEvent{}, fmt.Errorf("event data is empty")
	}

	var kubeEvent corev1.Event
	if err := json.Unmarshal(event.Data, &kubeEvent); err != nil {
		return models.K8sEvent{}, fmt.Errorf("decode kubernetes event: %w", err)
	}

	firstTs := kubeEvent.FirstTimestamp.Unix()
	lastTs := kubeEvent.LastTimestamp.Unix()
	if firstTs == 0 && !kubeEvent.EventTime.Time.IsZero() {
		firstTs = kubeEvent.EventTime.Unix()
	}
	if lastTs == 0 && !kubeEvent.EventTime.Time.IsZero() {
		lastTs = kubeEvent.EventTime.Unix()
	}

	timestamp := event.Timestamp / 1e9
	if lastTs > 0 {
		timestamp = lastTs
	} else if firstTs > 0 {
		timestamp = firstTs
	}

	source := kubeEvent.Source.Component
	if source == "" {
		source = kubeEvent.ReportingController
	}
	if source == "" {
		source = kubeEvent.ReportingInstance
	}

	k8sEvent := models.K8sEvent{
		ID:             event.ID,
		Timestamp:      timestamp,
		Reason:         kubeEvent.Reason,
		Message:        kubeEvent.Message,
		Type:           kubeEvent.Type,
		Count:          kubeEvent.Count,
		Source:         source,
		FirstTimestamp: firstTs,
		LastTimestamp:  lastTs,
	}

	if k8sEvent.Reason == "" {
		k8sEvent.Reason = "Unknown"
	}
	if k8sEvent.Type == "" {
		k8sEvent.Type = "Normal"
	}

	return k8sEvent, nil
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
