package falkor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
)

func (s *Store) fetchLatestEvents(
	ctx context.Context,
	resourceUIDs []string,
	timestamp int64,
) (map[string]*store.NamespaceGraphChangeEvent, error) {
	if len(resourceUIDs) == 0 {
		return map[string]*store.NamespaceGraphChangeEvent{}, nil
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query: `
			MATCH (r:ResourceIdentity)-[:CHANGED]->(e:ChangeEvent)
			WHERE r.uid IN $uids
			  AND e.timestamp <= $timestamp
			WITH r.uid as resourceUID, e
			ORDER BY e.timestamp DESC
			WITH resourceUID, collect(e)[0] as latestEvent
			WHERE latestEvent IS NOT NULL
			RETURN resourceUID,
			       latestEvent.timestamp as timestamp,
			       latestEvent.eventType as eventType,
			       latestEvent.status as status,
			       latestEvent.errorMessage as errorMessage,
			       latestEvent.containerIssues as containerIssues,
			       latestEvent.impactScore as impactScore,
			       latestEvent.data as data
		`,
		Parameters: map[string]interface{}{
			"uids":      resourceUIDs,
			"timestamp": timestamp,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest events: %w", err)
	}

	events := make(map[string]*store.NamespaceGraphChangeEvent)
	for _, row := range result.Rows {
		if len(row) < 7 {
			continue
		}

		resourceUID, _ := row[0].(string)
		if resourceUID == "" {
			continue
		}

		event := &store.NamespaceGraphChangeEvent{}
		switch ts := row[1].(type) {
		case int64:
			event.TimestampNs = ts
		case float64:
			event.TimestampNs = int64(ts)
		}
		if et, ok := row[2].(string); ok {
			event.EventType = et
		}
		if status, ok := row[3].(string); ok {
			event.Status = status
		}
		if errMsg, ok := row[4].(string); ok {
			event.ErrorMessage = errMsg
		}
		if issues, ok := row[5].([]interface{}); ok {
			for _, issue := range issues {
				if issueString, ok := issue.(string); ok {
					event.ContainerIssues = append(event.ContainerIssues, issueString)
				}
			}
		} else if issuesStr, ok := row[5].(string); ok && issuesStr != "" {
			var issues []string
			if err := json.Unmarshal([]byte(issuesStr), &issues); err == nil {
				event.ContainerIssues = issues
			}
		}
		if score, ok := row[6].(float64); ok {
			event.ImpactScore = score
		}
		if len(row) > 7 {
			if dataStr, ok := row[7].(string); ok && dataStr != "" {
				event.SpecReplicas = extractSpecReplicas(dataStr)
			}
		}

		events[resourceUID] = event
	}

	return events, nil
}

func (s *Store) fetchSpecChanges(
	ctx context.Context,
	resourceUIDs []string,
	timestamp int64,
	lookbackNs int64,
) (map[string]*specChangeResult, error) {
	if len(resourceUIDs) == 0 {
		return map[string]*specChangeResult{}, nil
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query: `
			MATCH (r:ResourceIdentity)-[:CHANGED]->(e:ChangeEvent)
			WHERE r.uid IN $uids
			  AND e.timestamp >= $startTimestamp AND e.timestamp <= $timestamp
			WITH r.uid as resourceUID, e
			ORDER BY e.timestamp ASC
			WITH resourceUID, collect(e) as events
			WHERE size(events) > 0
			RETURN resourceUID,
			       events[0].data as earliestData,
			       events[-1].data as latestData,
			       events[-1].timestamp as latestTimestamp
		`,
		Parameters: map[string]interface{}{
			"uids":           resourceUIDs,
			"timestamp":      timestamp,
			"startTimestamp": timestamp - lookbackNs,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch spec changes: %w", err)
	}

	specChanges := make(map[string]*specChangeResult)
	for _, row := range result.Rows {
		if len(row) < 4 {
			continue
		}
		resourceUID, _ := row[0].(string)
		if resourceUID == "" {
			continue
		}

		sc := &specChangeResult{ResourceUID: resourceUID}
		if data, ok := row[1].(string); ok {
			sc.EarliestData = []byte(data)
		}
		if data, ok := row[2].(string); ok {
			sc.LatestData = []byte(data)
		}
		switch ts := row[3].(type) {
		case int64:
			sc.LatestTimestamp = ts
		case float64:
			sc.LatestTimestamp = int64(ts)
		}

		if len(sc.EarliestData) > 0 && len(sc.LatestData) > 0 {
			specChanges[resourceUID] = sc
		}
	}

	return specChanges, nil
}

func extractSpecReplicas(data string) *int {
	var resource map[string]interface{}
	if err := json.Unmarshal([]byte(data), &resource); err != nil {
		return nil
	}

	spec, ok := resource["spec"].(map[string]interface{})
	if !ok {
		return nil
	}

	replicas, ok := spec["replicas"]
	if !ok {
		return nil
	}

	switch v := replicas.(type) {
	case float64:
		r := int(v)
		return &r
	case int:
		return &v
	case int64:
		r := int(v)
		return &r
	}
	return nil
}
