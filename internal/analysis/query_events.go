package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/graph"
)

// getChangeEvents retrieves change events for resources within the time window.
// The query ensures ALL configChanged events are included (important for root cause)
// plus the most recent events for context, avoiding truncation of important config changes.
//
// This two-phase approach ensures we never miss critical configuration changes that
// triggered a failure, even if there are many subsequent status-only events.
func (a *RootCauseAnalyzer) getChangeEvents(
	ctx context.Context,
	resourceUIDs []string,
	failureTimestamp int64,
	lookbackNs int64,
) (map[string][]ChangeEventInfo, error) {
	if len(resourceUIDs) == 0 {
		return make(map[string][]ChangeEventInfo), nil
	}

	// Query that combines:
	// 1. ALL events with configChanged=true (critical for root cause analysis)
	// 2. Up to 10 most recent events (for status context)
	// This ensures we never miss the important config change that triggered a failure,
	// even if there are many subsequent status-only events.
	query := graph.GraphQuery{
		Timeout: 5000,
		Query: `
			UNWIND $resourceUIDs as uid
			MATCH (resource:ResourceIdentity {uid: uid})
			OPTIONAL MATCH (resource)-[:CHANGED]->(event:ChangeEvent)
			WHERE event.timestamp <= $failureTimestamp
			  AND event.timestamp >= $failureTimestamp - $lookback
			WITH resource.uid as resourceUID, event
			ORDER BY event.timestamp DESC
			WITH resourceUID, collect(event) as allEvents
			WITH resourceUID,
			     [e IN allEvents WHERE e.configChanged = true] as configEvents,
			     allEvents[0..10] as recentEvents
			WITH resourceUID,
			     configEvents + [e IN recentEvents WHERE NOT e.id IN [ce IN configEvents | ce.id]] as combinedEvents
			UNWIND combinedEvents as event
			WITH resourceUID, event
			ORDER BY event.timestamp DESC
			RETURN resourceUID, collect(DISTINCT event) as events
		`,
		Parameters: map[string]interface{}{
			"resourceUIDs":     resourceUIDs,
			"failureTimestamp": failureTimestamp,
			"lookback":         lookbackNs,
		},
	}

	a.logger.Debug("getChangeEvents: executing query for %d resources with lookback %d ns (%.1f minutes)",
		len(resourceUIDs), lookbackNs, float64(lookbackNs)/float64(time.Minute.Nanoseconds()))
	result, err := a.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query change events: %w", err)
	}

	events := make(map[string][]ChangeEventInfo)

	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}

		resourceUID, ok := row[0].(string)
		if !ok {
			continue
		}

		events[resourceUID] = []ChangeEventInfo{}

		// Parse events array
		if row[1] == nil {
			continue
		}

		eventList, ok := row[1].([]interface{})
		if !ok {
			continue
		}

		// Track seen event IDs to deduplicate (safety check)
		seenEventIDs := make(map[string]bool)

		for _, eventNode := range eventList {
			if eventNode == nil {
				continue
			}

			eventProps, err := graph.ParseNodeFromResult(eventNode)
			if err != nil || eventProps == nil || len(eventProps) == 0 {
				continue
			}

			event := graph.ParseChangeEventFromNode(eventProps)

			// Deduplicate by event ID (safety check)
			if seenEventIDs[event.ID] {
				a.logger.Debug("getChangeEvents: skipping duplicate event %s for resource %s", event.ID, resourceUID)
				continue
			}
			seenEventIDs[event.ID] = true

			// Get kind for status inference (we don't have it here, use empty)
			status := analyzer.InferStatusFromResource("", json.RawMessage(event.Data), event.EventType)

			changeEvent := ChangeEventInfo{
				EventID:       event.ID,
				Timestamp:     time.Unix(0, event.Timestamp),
				EventType:     event.EventType,
				Status:        status,
				ConfigChanged: event.ConfigChanged,
				StatusChanged: event.StatusChanged,
				Description:   fmt.Sprintf("%s event", event.EventType),
				Data:          []byte(event.Data),
			}

			events[resourceUID] = append(events[resourceUID], changeEvent)
		}
	}

	// Log event counts for debugging
	for uid, evts := range events {
		configCount := 0
		for _, e := range evts {
			if e.ConfigChanged {
				configCount++
			}
		}
		a.logger.Debug("getChangeEvents: resource %s has %d events (%d with configChanged=true)", uid, len(evts), configCount)
	}
	a.logger.Debug("getChangeEvents: found events for %d resources", len(events))
	return events, nil
}

// getK8sEvents retrieves Kubernetes events (kind: Event) for resources within the time window.
// These are different from ChangeEvents - they represent K8s Events like "FailedScheduling",
// "BackOff", etc. that are emitted by Kubernetes components.
func (a *RootCauseAnalyzer) getK8sEvents(
	ctx context.Context,
	resourceUIDs []string,
	failureTimestamp int64,
	lookbackNs int64,
) (map[string][]K8sEventInfo, error) {
	if len(resourceUIDs) == 0 {
		return make(map[string][]K8sEventInfo), nil
	}

	query := graph.GraphQuery{
		Timeout: 5000,
		Query: `
			UNWIND $resourceUIDs as uid
			MATCH (resource:ResourceIdentity {uid: uid})
			OPTIONAL MATCH (resource)-[:EMITTED_EVENT]->(k8sEvent:K8sEvent)
			WHERE k8sEvent.timestamp <= $failureTimestamp
			  AND k8sEvent.timestamp >= $failureTimestamp - $lookback
			WITH resource.uid as resourceUID, k8sEvent
			ORDER BY k8sEvent.timestamp DESC
			WITH resourceUID, collect(k8sEvent)[0..20] as events
			RETURN resourceUID, events
		`,
		Parameters: map[string]interface{}{
			"resourceUIDs":     resourceUIDs,
			"failureTimestamp": failureTimestamp,
			"lookback":         lookbackNs,
		},
	}

	a.logger.Debug("getK8sEvents: executing query for %d resources", len(resourceUIDs))
	result, err := a.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query K8s events: %w", err)
	}

	events := make(map[string][]K8sEventInfo)

	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}

		resourceUID, ok := row[0].(string)
		if !ok {
			continue
		}

		events[resourceUID] = []K8sEventInfo{}

		// Parse events array
		if row[1] == nil {
			continue
		}

		eventList, ok := row[1].([]interface{})
		if !ok {
			continue
		}

		for _, eventNode := range eventList {
			if eventNode == nil {
				continue
			}

			eventProps, err := graph.ParseNodeFromResult(eventNode)
			if err != nil || eventProps == nil || len(eventProps) == 0 {
				continue
			}

			event := graph.ParseK8sEventFromNode(eventProps)

			k8sEventInfo := K8sEventInfo{
				EventID:   event.ID,
				Timestamp: time.Unix(0, event.Timestamp),
				Reason:    event.Reason,
				Message:   event.Message,
				Type:      event.Type,
				Count:     event.Count,
				Source:    event.Source,
			}

			events[resourceUID] = append(events[resourceUID], k8sEventInfo)
		}
	}

	a.logger.Debug("getK8sEvents: found events for %d resources", len(events))
	return events, nil
}

// getChangeEventsForRelated retrieves change events for related resources and populates
// the Events field in the RelatedResourceData structures.
//
// This is more efficient than querying events individually for each related resource,
// as it batches all related resource UIDs into a single query.
func (a *RootCauseAnalyzer) getChangeEventsForRelated(
	ctx context.Context,
	relatedByResource map[string][]RelatedResourceData,
	failureTimestamp int64,
	lookbackNs int64,
) error {
	// Collect all related resource UIDs
	relatedUIDs := []string{}
	uidToParent := make(map[string][]struct {
		parentUID string
		index     int
	})

	for parentUID, relatedList := range relatedByResource {
		for i, rel := range relatedList {
			relatedUIDs = append(relatedUIDs, rel.Resource.UID)
			uidToParent[rel.Resource.UID] = append(uidToParent[rel.Resource.UID], struct {
				parentUID string
				index     int
			}{parentUID, i})
		}
	}

	if len(relatedUIDs) == 0 {
		return nil
	}

	// Get events for all related resources
	events, err := a.getChangeEvents(ctx, relatedUIDs, failureTimestamp, lookbackNs)
	if err != nil {
		return err
	}

	// Distribute events back to related resources
	for uid, eventList := range events {
		for _, parent := range uidToParent[uid] {
			if parent.index < len(relatedByResource[parent.parentUID]) {
				relatedByResource[parent.parentUID][parent.index].Events = eventList
			}
		}
	}

	return nil
}
