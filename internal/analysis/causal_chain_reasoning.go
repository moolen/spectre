package analysis

import (
	"fmt"

	"github.com/moolen/spectre/internal/graph"
)

// selectPrimaryEvent chooses the most relevant event from a collection.
func selectPrimaryEvent(events []ChangeEventInfo, failureTimestamp int64) *ChangeEventInfo {
	if len(events) == 0 {
		return nil
	}

	if event := earliestConfigChange(events); event != nil {
		return event
	}
	if event := earliestCreateEvent(events); event != nil {
		return event
	}
	if event := closestStatusChange(events, failureTimestamp); event != nil {
		return event
	}

	earliest := &events[0]
	for i := range events {
		if events[i].Timestamp.Before(earliest.Timestamp) {
			earliest = &events[i]
		}
	}
	return earliest
}

func earliestConfigChange(events []ChangeEventInfo) *ChangeEventInfo {
	var selected *ChangeEventInfo
	for i := range events {
		if !events[i].ConfigChanged {
			continue
		}
		if selected == nil || events[i].Timestamp.Before(selected.Timestamp) {
			selected = &events[i]
		}
	}
	return selected
}

func earliestCreateEvent(events []ChangeEventInfo) *ChangeEventInfo {
	var selected *ChangeEventInfo
	for i := range events {
		if events[i].EventType != "CREATE" {
			continue
		}
		if selected == nil || events[i].Timestamp.Before(selected.Timestamp) {
			selected = &events[i]
		}
	}
	return selected
}

func closestStatusChange(events []ChangeEventInfo, failureTimestamp int64) *ChangeEventInfo {
	var selected *ChangeEventInfo
	minDelta := int64(1<<63 - 1)
	for i := range events {
		if !events[i].StatusChanged {
			continue
		}
		delta := failureTimestamp - events[i].Timestamp.UnixNano()
		if delta < 0 {
			delta = -delta
		}
		if delta < minDelta {
			minDelta = delta
			selected = &events[i]
		}
	}
	return selected
}

// generateStepReasoning creates a human-readable explanation for a causal step.
func generateStepReasoning(
	resource graph.ResourceIdentity,
	manager *graph.ResourceIdentity,
	managesEdge *graph.ManagesEdge,
	changeEvent *ChangeEventInfo,
	relationshipType string,
) string {
	switch relationshipType {
	case "MANAGES":
		return managementReasoning(resource, manager, managesEdge, true)
	case "MANAGED_BY":
		return managementReasoning(resource, manager, managesEdge, false)
	case edgeTypeOwns:
		return fmt.Sprintf("%s owns resources in the next layer", resource.Kind)
	case "SYMPTOM":
		if changeEvent != nil && changeEvent.ConfigChanged {
			return "Configuration change triggered the failure"
		}
		return "Observed failure symptom"
	default:
		if changeEvent != nil {
			return fmt.Sprintf("%s occurred in this resource", changeEvent.EventType)
		}
		return "Part of the causal chain"
	}
}

func managementReasoning(
	resource graph.ResourceIdentity,
	manager *graph.ResourceIdentity,
	managesEdge *graph.ManagesEdge,
	isManagerView bool,
) string {
	if manager == nil {
		if isManagerView {
			return "Lifecycle management relationship"
		}
		return "Managed resource"
	}

	confidence := 0.0
	if managesEdge != nil {
		confidence = managesEdge.Confidence
	}

	if isManagerView {
		return fmt.Sprintf("%s manages %s lifecycle (confidence: %.0f%%)",
			manager.Kind, resource.Kind, confidence*100)
	}

	return fmt.Sprintf("Managed by %s (confidence: %.0f%%)",
		manager.Kind, confidence*100)
}
