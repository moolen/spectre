package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/models"
)

// Execute runs the resource_timeline_changes tool.
func (t *ResourceTimelineChangesTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	params, err := parseResourceTimelineChangesInput(input)
	if err != nil {
		return nil, err
	}

	startTime, endTime, maxChanges, changeFilter, err := normalizeResourceTimelineChangesParams(params)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	response, err := t.queryTimelineResponse(ctx, startTime, endTime)
	if err != nil {
		return nil, err
	}

	output := &ResourceTimelineChangesOutput{
		Resources: make([]ResourceTimelineEntry, 0, len(params.ResourceUIDs)),
		Summary: TimelineChangesSummary{
			TimeRangeStart: startTime,
			TimeRangeEnd:   endTime,
		},
	}

	foundUIDs := make(map[string]bool)
	uidSet := buildUIDSet(params.ResourceUIDs)
	for _, resource := range response.Resources {
		if !uidSet[resource.ID] {
			continue
		}
		foundUIDs[resource.ID] = true

		entry := t.processResource(resource, maxChanges, params.IncludeFullSnapshot, changeFilter)
		output.Resources = append(output.Resources, entry)
		output.Summary.TotalChanges += entry.ChangeCount
		if entry.StatusSummary.CurrentStatus == "Error" {
			output.Summary.ResourcesWithErrors++
		}
	}

	appendMissingTimelineResources(output, params.ResourceUIDs, foundUIDs)
	output.Summary.TotalResources = len(output.Resources)
	output.ExecutionTimeMs = time.Since(start).Milliseconds()

	return output, nil
}

func parseResourceTimelineChangesInput(input json.RawMessage) (ResourceTimelineChangesInput, error) {
	var params ResourceTimelineChangesInput
	if err := json.Unmarshal(input, &params); err != nil {
		return ResourceTimelineChangesInput{}, fmt.Errorf("invalid input: %w", err)
	}
	if len(params.ResourceUIDs) == 0 {
		return ResourceTimelineChangesInput{}, fmt.Errorf("resource_uids is required and must contain at least one UID")
	}
	if len(params.ResourceUIDs) > 10 {
		return ResourceTimelineChangesInput{}, fmt.Errorf("resource_uids cannot contain more than 10 UIDs")
	}

	return params, nil
}

func normalizeResourceTimelineChangesParams(params ResourceTimelineChangesInput) (startTime, endTime int64, maxChanges int, changeFilter string, err error) {
	startTime, endTime = normalizeTimelineChangeRange(params.StartTime, params.EndTime)
	if startTime >= endTime {
		return 0, 0, 0, "", fmt.Errorf("start_time must be before end_time")
	}

	maxChanges = ApplyDefaultLimit(params.MaxChangesPerResource, 50, 200)
	changeFilter = params.ChangeFilter
	if changeFilter == "" {
		changeFilter = ChangeFilterAll
	}
	if changeFilter != ChangeFilterAll && changeFilter != ChangeFilterSpecOnly && changeFilter != ChangeFilterStatusOnly {
		return 0, 0, 0, "", fmt.Errorf("change_filter must be one of: %q, %q, %q", ChangeFilterAll, ChangeFilterSpecOnly, ChangeFilterStatusOnly)
	}

	return startTime, endTime, maxChanges, changeFilter, nil
}

func normalizeTimelineChangeRange(startTime, endTime int64) (int64, int64) {
	now := time.Now().Unix()
	if startTime == 0 {
		startTime = now - 3600
	}
	if endTime == 0 {
		endTime = now
	}
	if startTime > 10000000000 {
		startTime /= 1000
	}
	if endTime > 10000000000 {
		endTime /= 1000
	}

	return startTime, endTime
}

func (t *ResourceTimelineChangesTool) queryTimelineResponse(ctx context.Context, startTime, endTime int64) (*models.SearchResponse, error) {
	startStr := fmt.Sprintf("%d", startTime)
	endStr := fmt.Sprintf("%d", endTime)

	query, err := t.timelineService.ParseQueryParameters(ctx, startStr, endStr, map[string][]string{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse query parameters: %w", err)
	}

	queryResult, eventResult, err := t.timelineService.ExecuteConcurrentQueries(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query timeline: %w", err)
	}

	return t.timelineService.BuildTimelineResponse(queryResult, eventResult), nil
}

func buildUIDSet(resourceUIDs []string) map[string]bool {
	uidSet := make(map[string]bool, len(resourceUIDs))
	for _, uid := range resourceUIDs {
		uidSet[uid] = true
	}

	return uidSet
}

func appendMissingTimelineResources(output *ResourceTimelineChangesOutput, resourceUIDs []string, foundUIDs map[string]bool) {
	for _, uid := range resourceUIDs {
		if foundUIDs[uid] {
			continue
		}
		output.Resources = append(output.Resources, ResourceTimelineEntry{
			UID:   uid,
			Error: "resource not found in time window",
		})
		output.Summary.ResourcesNotFound++
	}
}
