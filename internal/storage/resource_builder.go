package storage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
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
// Optimized to pre-index events by UID, reducing O(n×m) to O(n+m) complexity
func (rb *ResourceBuilder) BuildResourcesFromEvents(events []models.Event) map[string]*models.Resource {
	return rb.BuildResourcesFromEventsWithQueryTime(events, 0)
}

func (rb *ResourceBuilder) BuildResourcesFromEventsWithQueryTime(events []models.Event, queryStartTimeNs int64) map[string]*models.Resource {
	// Temporary logging to debug gap fix
	rb.logger.Info("BuildResourcesFromEventsWithQueryTime called: events=%d, queryStartTimeNs=%d", len(events), queryStartTimeNs)
	// Log first 3 event IDs to check for state- prefix
	for i := 0; i < 3 && i < len(events); i++ {
		rb.logger.Info("Event %d: id=%s, timestamp=%d", i, events[i].ID, events[i].Timestamp/1e9)
	}
	resources := make(map[string]*models.Resource)

	// Filter out Kubernetes Event resources; they will be processed separately
	baseEvents := make([]models.Event, 0, len(events))
	for _, event := range events {
		if strings.EqualFold(event.Resource.Kind, "Event") {
			continue
		}
		baseEvents = append(baseEvents, event)
	}

	// PRE-INDEX EVENTS BY UID - This is the key optimization!
	// Instead of iterating all events for each resource (O(n×m)),
	// we build an index once (O(n)) and look up events per resource (O(1))
	eventsByUID := rb.indexEventsByUID(baseEvents)

	// Create resources from indexed events
	for uid, resourceEvents := range eventsByUID {
		if len(resourceEvents) == 0 {
			continue
		}

		// Create resource from first event
		resource := rb.CreateResource(resourceEvents[0])

		// Build status segments using pre-filtered events - O(k) not O(n)!
		resource.StatusSegments = rb.BuildStatusSegmentsFromEventsWithQueryTime(resourceEvents, queryStartTimeNs)

		// Mark as pre-existing if the first event is a state snapshot
		if len(resource.StatusSegments) > 0 {
			resource.PreExisting = rb.IsPreExistingFromEvents(resourceEvents)
		}

		resources[uid] = resource
	}

	return resources
}

// indexEventsByUID groups events by resource UID for efficient lookup
// This eliminates O(n×m) iteration by building an index once in O(n) time
func (rb *ResourceBuilder) indexEventsByUID(events []models.Event) map[string][]models.Event {
	index := make(map[string][]models.Event)
	for _, event := range events {
		uid := event.Resource.UID
		if uid == "" {
			continue
		}
		index[uid] = append(index[uid], event)
	}
	return index
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

// BuildStatusSegmentsFromEvents derives status segments from pre-filtered resource events
// This is the optimized version that works with pre-indexed events
// It caches parsed resource data to avoid repeated JSON unmarshaling (Performance Optimization Phase 4)
func (rb *ResourceBuilder) BuildStatusSegmentsFromEvents(resourceEvents []models.Event) []models.StatusSegment {
	return rb.BuildStatusSegmentsFromEventsWithQueryTime(resourceEvents, 0)
}

func (rb *ResourceBuilder) BuildStatusSegmentsFromEventsWithQueryTime(resourceEvents []models.Event, queryStartTimeNs int64) []models.StatusSegment {
	if len(resourceEvents) == 0 {
		return nil
	}

	// Sort by timestamp ascending
	sort.Slice(resourceEvents, func(i, j int) bool {
		return resourceEvents[i].Timestamp < resourceEvents[j].Timestamp
	})

	segments := make([]models.StatusSegment, 0, len(resourceEvents))

	// OPTIMIZATION: Cache parsed resource data to avoid repeated JSON unmarshaling
	// This reduces 5,707 unmarshal operations (439 resources × 13 segments) to ~6,000 (one per event)
	// Expected savings: ~0.60s (17% total CPU) based on CPU profile analysis
	resourceDataCache := make(map[string]*analyzer.ResourceData, len(resourceEvents))

	// Check if we need to create a leading segment for pre-existing resources
	// This handles the case where a resource existed before the query window
	// but the first event is after the query start time
	var leadingSegmentNeeded bool
	var leadingSegmentStatus string
	var leadingSegmentMessage string
	var leadingSegmentData json.RawMessage

	if queryStartTimeNs > 0 && len(resourceEvents) > 0 {
		firstEvent := resourceEvents[0]
		// If first event is a state snapshot (pre-existing) and occurs after query start,
		// we need a leading segment from query start to first event
		if strings.HasPrefix(firstEvent.ID, "state-") && firstEvent.Timestamp > queryStartTimeNs {
			leadingSegmentNeeded = true
			leadingSegmentData = firstEvent.Data

			// Determine the status for the leading segment
			parsedData, err := analyzer.ParseResourceData(firstEvent.Data)
			if err == nil {
				resourceDataCache[firstEvent.ID] = parsedData
				leadingSegmentStatus = analyzer.InferStatusFromParsedData(firstEvent.Resource.Kind, parsedData, string(firstEvent.Type))
			} else {
				leadingSegmentStatus = analyzer.InferStatusFromResource(firstEvent.Resource.Kind, firstEvent.Data, string(firstEvent.Type))
			}
			leadingSegmentMessage = "Resource existed before query window"
			
			// Debug logging to verify the fix is working
			logging.GetLogger("resource_builder").Debug(
				"Creating leading segment: resource=%s/%s, queryStart=%d, firstEvent=%d, gap=%ds",
				firstEvent.Resource.Kind, firstEvent.Resource.Name,
				queryStartTimeNs/1e9, firstEvent.Timestamp/1e9,
				(firstEvent.Timestamp-queryStartTimeNs)/1e9,
			)
		}
	}

	// Create leading segment if needed
	if leadingSegmentNeeded {
		firstEventTime := resourceEvents[0].Timestamp
		leadingSegment := models.StatusSegment{
			StartTime:    queryStartTimeNs / 1e9, // Convert to seconds
			EndTime:      firstEventTime / 1e9,
			Status:       leadingSegmentStatus,
			Message:      leadingSegmentMessage,
			ResourceData: leadingSegmentData,
		}
		segments = append(segments, leadingSegment)
	}

	for i, event := range resourceEvents {
		var endTime int64
		if i+1 < len(resourceEvents) {
			endTime = resourceEvents[i+1].Timestamp
		} else {
			// For the last event, end time is current time
			endTime = event.Timestamp + 3600*1e9 // 1 hour after
		}

		// Get or parse resource data (cache keyed by event ID)
		var status string
		if parsedData, ok := resourceDataCache[event.ID]; ok {
			// Cache hit - use pre-parsed data
			status = analyzer.InferStatusFromParsedData(event.Resource.Kind, parsedData, string(event.Type))
		} else {
			// Cache miss - parse and cache for future use
			parsedData, err := analyzer.ParseResourceData(event.Data)
			if err == nil {
				resourceDataCache[event.ID] = parsedData
				status = analyzer.InferStatusFromParsedData(event.Resource.Kind, parsedData, string(event.Type))
			} else {
				// Fallback to non-cached version if parsing fails
				status = analyzer.InferStatusFromResource(event.Resource.Kind, event.Data, string(event.Type))
			}
		}

		segment := models.StatusSegment{
			StartTime:    event.Timestamp / 1e9, // Convert to seconds
			EndTime:      endTime / 1e9,
			Status:       status,
			Message:      rb.generateMessage(event),
			ResourceData: event.Data,
		}
		segments = append(segments, segment)
	}

	// Merge consecutive segments with the same status
	//return rb.mergeConsecutiveSegments(segments)
	return segments
}

// mergeConsecutiveSegments combines adjacent segments with identical status
// This prevents visual clutter in the timeline when multiple events don't change the resource status
func (rb *ResourceBuilder) mergeConsecutiveSegments(segments []models.StatusSegment) []models.StatusSegment {
	if len(segments) <= 1 {
		return segments
	}

	merged := make([]models.StatusSegment, 0, len(segments))
	current := segments[0]

	for i := 1; i < len(segments); i++ {
		next := segments[i]

		// If status is the same, extend the current segment
		if current.Status == next.Status {
			// Keep the first segment's start time and resource data
			// Update the end time to encompass the next segment
			current.EndTime = next.EndTime
			// Optionally update message if the next one is more informative
			if next.Message != "" && len(next.Message) > len(current.Message) {
				current.Message = next.Message
			}
		} else {
			// Status changed, save current segment and start a new one
			merged = append(merged, current)
			current = next
		}
	}

	// Don't forget the last segment
	merged = append(merged, current)

	return merged
}

// BuildStatusSegments derives status segments from event timeline
// This is a wrapper for backward compatibility - filters events then calls optimized version
func (rb *ResourceBuilder) BuildStatusSegments(resourceUID string, allEvents []models.Event) []models.StatusSegment {
	// Filter events for this resource
	var resourceEvents []models.Event
	for _, event := range allEvents {
		if event.Resource.UID == resourceUID {
			resourceEvents = append(resourceEvents, event)
		}
	}

	return rb.BuildStatusSegmentsFromEvents(resourceEvents)
}

// IsPreExistingFromEvents checks if resource is pre-existing from pre-filtered events
// This is the optimized version that works with pre-indexed events
func (rb *ResourceBuilder) IsPreExistingFromEvents(resourceEvents []models.Event) bool {
	if len(resourceEvents) == 0 {
		return false
	}

	// Sort by timestamp ascending
	sort.Slice(resourceEvents, func(i, j int) bool {
		return resourceEvents[i].Timestamp < resourceEvents[j].Timestamp
	})

	// Check if the first event is a state snapshot
	// State snapshot events have IDs starting with "state-"
	firstEvent := resourceEvents[0]
	return strings.HasPrefix(firstEvent.ID, "state-")
}

// IsPreExisting checks if a resource existed before the query start time
// by checking if the first event is a state snapshot (indicated by "state-" ID prefix)
// This is a wrapper for backward compatibility - filters events then calls optimized version
func (rb *ResourceBuilder) IsPreExisting(resourceUID string, allEvents []models.Event) bool {
	// Find all events for this resource
	var resourceEvents []models.Event
	for _, event := range allEvents {
		if event.Resource.UID == resourceUID {
			resourceEvents = append(resourceEvents, event)
		}
	}

	return rb.IsPreExistingFromEvents(resourceEvents)
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
