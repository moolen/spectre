package namespacegraph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
)

// FetchLatestEvents fetches the latest change event for each resource
func (f *ResourceFetcher) FetchLatestEvents(
	ctx context.Context,
	resourceUIDs []string,
	timestamp int64,
) (map[string]*ChangeEventInfo, error) {
	if len(resourceUIDs) == 0 {
		return make(map[string]*ChangeEventInfo), nil
	}

	query := graph.GraphQuery{
		Timeout: QueryTimeoutMs,
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
	}

	result, err := f.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest events: %w", err)
	}

	events := make(map[string]*ChangeEventInfo)
	for _, row := range result.Rows {
		resourceUID := parseStringCell(row, 0)
		if resourceUID == "" {
			continue
		}

		event := &ChangeEventInfo{
			Timestamp:    parseInt64Cell(row, 1),
			EventType:    parseStringCell(row, 2),
			Status:       parseStringCell(row, 3),
			ErrorMessage: parseStringCell(row, 4),
			ImpactScore:  parseFloat64Cell(row, 6),
		}
		event.ContainerIssues = parseContainerIssuesCell(row, 5)

		dataStr := parseStringCell(row, 7)
		if dataStr != "" {
			event.SpecReplicas = extractSpecReplicas(dataStr)
		}

		events[resourceUID] = event
	}

	return events, nil
}

// specChangeResult holds spec data for diff computation
type specChangeResult struct {
	ResourceUID     string
	LatestData      []byte
	EarliestData    []byte
	LatestTimestamp int64
}

// FetchSpecChanges fetches spec data for resources within a lookback window to compute diffs.
func (f *ResourceFetcher) FetchSpecChanges(
	ctx context.Context,
	resourceUIDs []string,
	timestamp int64,
	lookbackNs int64,
) (map[string]*specChangeResult, error) {
	if len(resourceUIDs) == 0 {
		return make(map[string]*specChangeResult), nil
	}

	startTimestamp := timestamp - lookbackNs
	query := graph.GraphQuery{
		Timeout: QueryTimeoutMs,
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
			"startTimestamp": startTimestamp,
		},
	}

	result, err := f.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch spec changes: %w", err)
	}

	specChanges := make(map[string]*specChangeResult)
	for _, row := range result.Rows {
		resourceUID := parseStringCell(row, 0)
		if resourceUID == "" {
			continue
		}

		sc := &specChangeResult{
			ResourceUID:     resourceUID,
			EarliestData:    []byte(parseStringCell(row, 1)),
			LatestData:      []byte(parseStringCell(row, 2)),
			LatestTimestamp: parseInt64Cell(row, 3),
		}

		if len(sc.EarliestData) > 0 && len(sc.LatestData) > 0 {
			specChanges[resourceUID] = sc
		}
	}

	return specChanges, nil
}

// extractSpecReplicas extracts the spec.replicas field from resource JSON data
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
	default:
		return nil
	}
}

func parseContainerIssuesCell(row []interface{}, index int) []string {
	if index >= len(row) {
		return nil
	}

	if issues, ok := row[index].([]interface{}); ok {
		containerIssues := make([]string, 0, len(issues))
		for _, issue := range issues {
			if s, ok := issue.(string); ok {
				containerIssues = append(containerIssues, s)
			}
		}
		return containerIssues
	}

	if issuesStr, ok := row[index].(string); ok && issuesStr != "" {
		var issues []string
		if err := json.Unmarshal([]byte(issuesStr), &issues); err == nil {
			return issues
		}
	}

	return nil
}
