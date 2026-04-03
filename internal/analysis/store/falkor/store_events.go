package falkor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/analysis/store"
	analyzerpkg "github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/graph"
)

func (s *Store) GetChangeEvents(
	ctx context.Context,
	resourceUIDs []string,
	window store.ResourceWindow,
) (map[string][]store.ChangeEventInfo, error) {
	if len(resourceUIDs) == 0 {
		return map[string][]store.ChangeEventInfo{}, nil
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query: `
			MATCH (resource:ResourceIdentity)
			WHERE resource.uid IN $resourceUIDs
			OPTIONAL MATCH (resource)-[:CHANGED]->(event:ChangeEvent)
			WHERE event.timestamp <= $failureTimestamp
			  AND event.timestamp >= $failureTimestamp - $lookback
			WITH resource.uid as resourceUID, event
			ORDER BY event.timestamp DESC
			WITH resourceUID, collect(event) as allEvents
			WITH resourceUID,
			     [e IN allEvents WHERE e.configChanged = true] as configEvents,
			     allEvents[0..$maxEvents] as recentEvents
			WITH resourceUID,
			     configEvents + [e IN recentEvents WHERE NOT e.id IN [ce IN configEvents | ce.id]] as combinedEvents
			UNWIND CASE WHEN size(combinedEvents) > 0 THEN combinedEvents ELSE [null] END as event
			WITH resourceUID, event
			WHERE event IS NOT NULL
			WITH resourceUID, event
			ORDER BY event.timestamp DESC
			RETURN resourceUID, collect(DISTINCT event) as events
		`,
		Parameters: map[string]interface{}{
			"resourceUIDs":     resourceUIDs,
			"failureTimestamp": window.FailureTimestampNs,
			"lookback":         window.LookbackNs,
			"maxEvents":        maxRecentEvents,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query change events: %w", err)
	}

	events := make(map[string][]store.ChangeEventInfo)
	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}

		resourceUID, ok := row[0].(string)
		if !ok {
			continue
		}
		events[resourceUID] = []store.ChangeEventInfo{}

		eventList, ok := row[1].([]interface{})
		if !ok {
			continue
		}

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
			if seenEventIDs[event.ID] {
				continue
			}
			seenEventIDs[event.ID] = true

			status := analyzerpkg.InferStatusFromResource("", json.RawMessage(event.Data), event.EventType)
			events[resourceUID] = append(events[resourceUID], store.ChangeEventInfo{
				EventID:       event.ID,
				Timestamp:     time.Unix(0, event.Timestamp),
				EventType:     event.EventType,
				Status:        status,
				ConfigChanged: event.ConfigChanged,
				StatusChanged: event.StatusChanged,
				Description:   fmt.Sprintf("%s event", event.EventType),
				Data:          []byte(event.Data),
			})
		}
	}

	return events, nil
}

func (s *Store) GetK8sEvents(
	ctx context.Context,
	resourceUIDs []string,
	window store.ResourceWindow,
) (map[string][]store.K8sEventInfo, error) {
	if len(resourceUIDs) == 0 {
		return map[string][]store.K8sEventInfo{}, nil
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query: `
			MATCH (resource:ResourceIdentity)
			WHERE resource.uid IN $resourceUIDs
			OPTIONAL MATCH (resource)-[:EMITTED_EVENT]->(k8sEvent:K8sEvent)
			WHERE k8sEvent.timestamp <= $failureTimestamp
			  AND k8sEvent.timestamp >= $failureTimestamp - $lookback
			WITH resource.uid as resourceUID, k8sEvent
			ORDER BY k8sEvent.timestamp DESC
			WITH resourceUID, collect(k8sEvent)[0..$maxEvents] as events
			RETURN resourceUID, events
		`,
		Parameters: map[string]interface{}{
			"resourceUIDs":     resourceUIDs,
			"failureTimestamp": window.FailureTimestampNs,
			"lookback":         window.LookbackNs,
			"maxEvents":        maxK8sEvents,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query K8s events: %w", err)
	}

	events := make(map[string][]store.K8sEventInfo)
	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}

		resourceUID, ok := row[0].(string)
		if !ok {
			continue
		}
		events[resourceUID] = []store.K8sEventInfo{}

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

			events[resourceUID] = append(events[resourceUID], store.K8sEventInfo{
				EventID:   event.ID,
				Timestamp: time.Unix(0, event.Timestamp),
				Reason:    event.Reason,
				Message:   event.Message,
				Type:      event.Type,
				Count:     event.Count,
				Source:    event.Source,
			})
		}
	}

	return events, nil
}
