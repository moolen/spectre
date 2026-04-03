package sync

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

// buildResourceIdentityNode creates a ResourceIdentity node from an event
func (b *graphBuilder) buildResourceIdentityNode(event models.Event) graph.ResourceIdentity {
	now := time.Now().UnixNano()
	deleted := event.Type == models.EventTypeDelete

	// Extract labels from event data
	labels := b.extractLabels(event)

	resource := graph.ResourceIdentity{
		UID:       event.Resource.UID,
		Kind:      event.Resource.Kind,
		APIGroup:  event.Resource.Group,
		Version:   event.Resource.Version,
		Namespace: event.Resource.Namespace,
		Name:      event.Resource.Name,
		Labels:    labels,
		FirstSeen: now,
		LastSeen:  now,
		Deleted:   deleted,
		DeletedAt: func() int64 {
			if deleted {
				return event.Timestamp
			}
			return 0
		}(),
	}

	// Update label index for Pod resources (enables fast selector lookups)
	if b.labelIndex != nil && event.Resource.Kind == kindPod {
		if deleted {
			b.labelIndex.Remove(event.Resource.Namespace, kindPod, event.Resource.UID)
			b.logger.Debug("Removed Pod from label index: %s/%s", event.Resource.Namespace, event.Resource.Name)
		} else {
			b.labelIndex.Update(event.Resource.Namespace, kindPod, event.Resource.UID, labels)
			b.logger.Debug("Updated label index for Pod: %s/%s with %d labels",
				event.Resource.Namespace, event.Resource.Name, len(labels))
		}
	}

	if deleted {
		b.logger.Debug("Building ResourceIdentity for DELETE event: %s/%s uid=%s",
			resource.Kind, resource.Name, resource.UID)
	}

	return resource
}

// extractLabels extracts labels from the event's resource data
func (b *graphBuilder) extractLabels(event models.Event) map[string]string {
	if len(event.Data) == 0 {
		return nil
	}

	var resourceData map[string]interface{}
	if err := json.Unmarshal(event.Data, &resourceData); err != nil {
		b.logger.Debug("Failed to parse resource data for label extraction: %v", err)
		return nil
	}

	metadata, ok := resourceData["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}

	labelsRaw, ok := metadata["labels"].(map[string]interface{})
	if !ok {
		return nil
	}

	labels := make(map[string]string)
	for key, value := range labelsRaw {
		if strValue, ok := value.(string); ok {
			labels[key] = strValue
		}
	}

	return labels
}

// extractInvolvedObjectMetadata extracts involvedObject metadata from a K8s Event
// and creates a ResourceIdentity node for it. This ensures that EMITTED_EVENT edges
// have a valid target node even if we haven't seen CREATE/UPDATE events for the resource.
func (b *graphBuilder) extractInvolvedObjectMetadata(event models.Event) *graph.ResourceIdentity {
	if len(event.Data) == 0 {
		return nil
	}

	var eventData map[string]interface{}
	if err := json.Unmarshal(event.Data, &eventData); err != nil {
		b.logger.Debug("Failed to parse event data for involvedObject extraction: %v", err)
		return nil
	}

	involvedObj, ok := eventData["involvedObject"].(map[string]interface{})
	if !ok {
		return nil
	}

	uid, _ := involvedObj["uid"].(string)
	kind, _ := involvedObj["kind"].(string)
	name, _ := involvedObj["name"].(string)
	namespace, _ := involvedObj["namespace"].(string)
	apiVersion, _ := involvedObj["apiVersion"].(string)

	if uid == "" || kind == "" || name == "" || apiVersion == "" {
		b.logger.Debug("Incomplete involvedObject metadata in event %s", event.ID)
		return nil
	}

	group, version := "", apiVersion
	if idx := strings.Index(apiVersion, "/"); idx != -1 {
		group = apiVersion[:idx]
		version = apiVersion[idx+1:]
	}

	return &graph.ResourceIdentity{
		UID:       uid,
		Kind:      kind,
		APIGroup:  group,
		Version:   version,
		Namespace: namespace,
		Name:      name,
		Labels:    map[string]string{},
		FirstSeen: event.Timestamp,
		LastSeen:  event.Timestamp,
		Deleted:   false,
		DeletedAt: 0,
	}
}

// buildChangeEventNode creates a ChangeEvent node from an event
func (b *graphBuilder) buildChangeEventNode(event models.Event) graph.ChangeEvent {
	var resourceData *analyzer.ResourceData
	var err error

	if len(event.Data) > 0 {
		resourceData, err = analyzer.ParseResourceData(event.Data)
		if err != nil {
			b.logger.Warn("Failed to parse resource data for event %s: %v", event.ID, err)
		}
	}

	status := analyzer.InferStatusFromParsedData(event.Resource.Kind, resourceData, string(event.Type))

	errorMessage := ""
	if len(event.Data) > 0 {
		errors := analyzer.InferErrorMessages(event.Resource.Kind, event.Data, status)
		if len(errors) > 0 {
			errorMessage = errors[0]
		}
	}

	containerIssues := []string{}
	if len(event.Data) > 0 && event.Resource.Kind == kindPod {
		if issues, err := analyzer.GetContainerIssuesFromJSON(event.Data); err == nil && len(issues) > 0 {
			for _, issue := range issues {
				containerIssues = append(containerIssues, issue.IssueType)
			}
		}
	}

	configChanged := false
	statusChanged := false
	replicasChanged := false

	switch event.Type {
	case models.EventTypeUpdate:
		configChanged, statusChanged, replicasChanged = b.detectChanges(event, resourceData)
	case models.EventTypeCreate, models.EventTypeDelete:
	default:
	}

	if b.stateCache != nil {
		if event.Type == models.EventTypeDelete {
			b.stateCache.Remove(event.Resource.UID)
		} else if len(event.Data) > 0 {
			b.stateCache.Put(event.Resource.UID, event.Data, event.Timestamp, string(event.Type))
		}
	}

	return graph.ChangeEvent{
		ID:              event.ID,
		Timestamp:       event.Timestamp,
		EventType:       string(event.Type),
		Status:          status,
		ErrorMessage:    errorMessage,
		ContainerIssues: containerIssues,
		ConfigChanged:   configChanged,
		StatusChanged:   statusChanged,
		ReplicasChanged: replicasChanged,
		ImpactScore:     b.calculateImpactScore(status, containerIssues),
		Data:            string(event.Data),
	}
}

// buildK8sEventNode creates a K8sEvent node from a Kubernetes Event object
func (b *graphBuilder) buildK8sEventNode(event models.Event) (graph.K8sEvent, error) {
	var eventData map[string]interface{}
	if err := json.Unmarshal(event.Data, &eventData); err != nil {
		return graph.K8sEvent{}, fmt.Errorf("failed to parse event data: %w", err)
	}

	reason := ""
	message := ""
	eventType := ""
	count := 0
	source := ""

	if r, ok := eventData["reason"].(string); ok {
		reason = r
	}
	if m, ok := eventData["message"].(string); ok {
		message = m
	}
	if t, ok := eventData["type"].(string); ok {
		eventType = t
	}
	if c, ok := eventData["count"].(float64); ok {
		count = int(c)
	}
	if src, ok := eventData["source"].(map[string]interface{}); ok {
		if comp, ok := src["component"].(string); ok {
			source = comp
		}
	}

	return graph.K8sEvent{
		ID:        event.ID,
		Timestamp: event.Timestamp,
		Reason:    reason,
		Message:   message,
		Type:      eventType,
		Count:     count,
		Source:    source,
	}, nil
}

// buildChangedEdge creates a CHANGED edge
func (b *graphBuilder) buildChangedEdge(resourceUID, eventID string) graph.Edge {
	props := graph.ChangedEdge{
		SequenceNumber: 0,
	}

	propsJSON, _ := json.Marshal(props)

	return graph.Edge{
		Type:       graph.EdgeTypeChanged,
		FromUID:    resourceUID,
		ToUID:      eventID,
		Properties: propsJSON,
	}
}

// buildEmittedEventEdge creates an EMITTED_EVENT edge
func (b *graphBuilder) buildEmittedEventEdge(resourceUID, k8sEventID string) graph.Edge {
	return graph.Edge{
		Type:       graph.EdgeTypeEmittedEvent,
		FromUID:    resourceUID,
		ToUID:      k8sEventID,
		Properties: json.RawMessage("{}"),
	}
}

// calculateImpactScore calculates the impact score for a change event
func (b *graphBuilder) calculateImpactScore(status string, containerIssues []string) float64 {
	score := 0.0

	switch status {
	case "Error":
		score = 0.8
	case "Warning":
		score = 0.5
	case "Terminating":
		score = 0.6
	case "Unknown":
		score = 0.3
	default:
		score = 0.1
	}

	if len(containerIssues) > 0 {
		score += 0.2
	}

	if score > 1.0 {
		score = 1.0
	}

	return score
}
