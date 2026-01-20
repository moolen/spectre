package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/mcp/client"
)

// ResourceTimelineChangesTool implements the resource_timeline_changes MCP tool
// which returns semantic field-level diffs for specific resources by UID.
type ResourceTimelineChangesTool struct {
	client *client.SpectreClient
}

// NewResourceTimelineChangesTool creates a new resource timeline changes tool
func NewResourceTimelineChangesTool(client *client.SpectreClient) *ResourceTimelineChangesTool {
	return &ResourceTimelineChangesTool{
		client: client,
	}
}

// ChangeFilter constants for filtering which changes to include
const (
	// ChangeFilterAll returns all changes (default)
	ChangeFilterAll = "all"
	// ChangeFilterSpecOnly returns only spec changes (excludes status)
	ChangeFilterSpecOnly = "spec_only"
	// ChangeFilterStatusOnly returns only status changes
	ChangeFilterStatusOnly = "status_only"
)

// ResourceTimelineChangesInput represents the input for resource_timeline_changes tool
type ResourceTimelineChangesInput struct {
	// ResourceUIDs is a list of resource UIDs to query (required, max 10)
	ResourceUIDs []string `json:"resource_uids"`

	// StartTime is the start of the time window (Unix seconds/ms, optional)
	// Default: 1 hour ago
	StartTime int64 `json:"start_time,omitempty"`

	// EndTime is the end of the time window (Unix seconds/ms, optional)
	// Default: now
	EndTime int64 `json:"end_time,omitempty"`

	// IncludeFullSnapshot returns the first segment's full resource JSON (optional)
	// Default: false (for token efficiency)
	IncludeFullSnapshot bool `json:"include_full_snapshot,omitempty"`

	// MaxChangesPerResource limits changes returned per resource (optional)
	// Default: 50, Max: 200
	MaxChangesPerResource int `json:"max_changes_per_resource,omitempty"`

	// ChangeFilter controls which types of changes to include (optional)
	// Values: "all" (default), "spec_only", "status_only"
	// - "all": returns all changes
	// - "spec_only": returns only spec changes (excludes .status.* paths)
	// - "status_only": returns only status changes (only .status.* paths)
	ChangeFilter string `json:"change_filter,omitempty"`
}

// ResourceTimelineChangesOutput represents the output of resource_timeline_changes tool
type ResourceTimelineChangesOutput struct {
	// Resources contains changes grouped by resource
	Resources []ResourceTimelineEntry `json:"resources"`

	// Summary provides aggregate statistics
	Summary TimelineChangesSummary `json:"summary"`

	// ExecutionTimeMs is the query execution time
	ExecutionTimeMs int64 `json:"execution_time_ms"`
}

// ResourceTimelineEntry contains timeline changes for a single resource
type ResourceTimelineEntry struct {
	// Resource identification
	UID       string `json:"uid"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`

	// UnifiedDiff is a git-style unified diff of all changes (compact format)
	UnifiedDiff string `json:"unified_diff,omitempty"`

	// StatusSummary is a condensed view of status transitions
	StatusSummary StatusConditionSummary `json:"status_summary"`

	// FirstSnapshot is the full resource JSON at first segment (if requested)
	FirstSnapshot map[string]any `json:"first_snapshot,omitempty"`

	// ChangeCount is the total number of changes detected
	ChangeCount int `json:"change_count"`

	// Error is set if this resource could not be found or processed
	Error string `json:"error,omitempty"`
}

// StatusConditionSummary provides a condensed view of status condition changes
type StatusConditionSummary struct {
	// CurrentStatus is the latest status (Ready, Warning, Error, etc.)
	CurrentStatus string `json:"current_status"`

	// Transitions is a list of major status transitions
	Transitions []StatusTransitionSummary `json:"transitions,omitempty"`

	// ConditionHistory is a per-condition summary (e.g., "Ready: True(5m)->False(2m)")
	ConditionHistory map[string]string `json:"condition_history,omitempty"`
}

// StatusTransitionSummary represents a status state change
type StatusTransitionSummary struct {
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
	Timestamp  int64  `json:"timestamp"`
	Reason     string `json:"reason,omitempty"`
}

// TimelineChangesSummary provides aggregate statistics
type TimelineChangesSummary struct {
	TotalResources      int   `json:"total_resources"`
	TotalChanges        int   `json:"total_changes"`
	ResourcesWithErrors int   `json:"resources_with_errors"`
	ResourcesNotFound   int   `json:"resources_not_found"`
	TimeRangeStart      int64 `json:"time_range_start"`
	TimeRangeEnd        int64 `json:"time_range_end"`
}

// Execute runs the resource_timeline_changes tool
func (t *ResourceTimelineChangesTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ResourceTimelineChangesInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Validate required parameters
	if len(params.ResourceUIDs) == 0 {
		return nil, fmt.Errorf("resource_uids is required and must contain at least one UID")
	}
	if len(params.ResourceUIDs) > 10 {
		return nil, fmt.Errorf("resource_uids cannot contain more than 10 UIDs")
	}

	// Apply default time window (1 hour ago to now)
	now := time.Now().Unix()
	startTime := params.StartTime
	endTime := params.EndTime

	if startTime == 0 {
		startTime = now - 3600 // 1 hour ago
	}
	if endTime == 0 {
		endTime = now
	}

	// Convert milliseconds to seconds if needed
	if startTime > 10000000000 {
		startTime /= 1000
	}
	if endTime > 10000000000 {
		endTime /= 1000
	}

	if startTime >= endTime {
		return nil, fmt.Errorf("start_time must be before end_time")
	}

	// Apply defaults for max changes
	maxChanges := ApplyDefaultLimit(params.MaxChangesPerResource, 50, 200)

	// Validate and apply default for change filter
	changeFilter := params.ChangeFilter
	if changeFilter == "" {
		changeFilter = ChangeFilterAll
	}
	if changeFilter != ChangeFilterAll && changeFilter != ChangeFilterSpecOnly && changeFilter != ChangeFilterStatusOnly {
		return nil, fmt.Errorf("change_filter must be one of: %q, %q, %q", ChangeFilterAll, ChangeFilterSpecOnly, ChangeFilterStatusOnly)
	}

	start := time.Now()

	// Query timeline API - we need to query without filters and match by UID
	// since the API doesn't support direct UID filtering
	response, err := t.client.QueryTimeline(startTime, endTime, nil, 10000) // Large page size to search all resources by UID
	if err != nil {
		return nil, fmt.Errorf("failed to query timeline: %w", err)
	}

	// Build UID lookup set for efficient filtering
	uidSet := make(map[string]bool)
	for _, uid := range params.ResourceUIDs {
		uidSet[uid] = true
	}

	// Process each requested UID
	output := &ResourceTimelineChangesOutput{
		Resources: make([]ResourceTimelineEntry, 0, len(params.ResourceUIDs)),
		Summary: TimelineChangesSummary{
			TimeRangeStart: startTime,
			TimeRangeEnd:   endTime,
		},
	}

	// Find matching resources and compute changes
	foundUIDs := make(map[string]bool)
	for _, resource := range response.Resources {
		if !uidSet[resource.ID] {
			continue
		}
		foundUIDs[resource.ID] = true

		entry := t.processResource(resource, maxChanges, params.IncludeFullSnapshot, changeFilter)
		output.Resources = append(output.Resources, entry)
		output.Summary.TotalChanges += entry.ChangeCount

		// Check if resource has errors
		if entry.StatusSummary.CurrentStatus == "Error" {
			output.Summary.ResourcesWithErrors++
		}
	}

	// Add entries for UIDs that were not found
	for _, uid := range params.ResourceUIDs {
		if !foundUIDs[uid] {
			output.Resources = append(output.Resources, ResourceTimelineEntry{
				UID:   uid,
				Error: "resource not found in time window",
			})
			output.Summary.ResourcesNotFound++
		}
	}

	output.Summary.TotalResources = len(output.Resources)
	output.ExecutionTimeMs = time.Since(start).Milliseconds()

	return output, nil
}

// processResource computes semantic changes for a single resource
func (t *ResourceTimelineChangesTool) processResource(resource client.TimelineResource, maxChanges int, includeSnapshot bool, changeFilter string) ResourceTimelineEntry {
	entry := ResourceTimelineEntry{
		UID:       resource.ID,
		Kind:      resource.Kind,
		Namespace: resource.Namespace,
		Name:      resource.Name,
		StatusSummary: StatusConditionSummary{
			Transitions:      make([]StatusTransitionSummary, 0),
			ConditionHistory: make(map[string]string),
		},
	}

	segments := resource.StatusSegments
	if len(segments) == 0 {
		return entry
	}

	// Sort segments by start time (ascending) to ensure chronological order
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].StartTime < segments[j].StartTime
	})

	// Set current status from last segment
	entry.StatusSummary.CurrentStatus = segments[len(segments)-1].Status

	// Include first snapshot if requested
	if includeSnapshot && len(segments[0].ResourceData) > 0 {
		snapshot, err := analysis.ParseJSONToMap(segments[0].ResourceData)
		if err == nil && snapshot != nil {
			entry.FirstSnapshot = snapshot
		}
	}

	// Collect all diffs between consecutive segments
	var allDiffs []analysis.EventDiff
	for i := 1; i < len(segments); i++ {
		prevSegment := segments[i-1]
		currSegment := segments[i]

		// Compute JSON diff between segments
		diffs, err := analysis.ComputeJSONDiff(prevSegment.ResourceData, currSegment.ResourceData)
		if err != nil {
			continue
		}

		// Filter noisy paths
		diffs = analysis.FilterNoisyPaths(diffs)

		// Apply change filter based on path
		diffs = filterDiffsByPath(diffs, changeFilter)

		allDiffs = append(allDiffs, diffs...)

		// Track status transitions
		if prevSegment.Status != currSegment.Status {
			entry.StatusSummary.Transitions = append(entry.StatusSummary.Transitions, StatusTransitionSummary{
				FromStatus: prevSegment.Status,
				ToStatus:   currSegment.Status,
				Timestamp:  currSegment.StartTime,
				Reason:     currSegment.Message,
			})
		}
	}

	// Apply max changes limit (limit number of diffs, not output size)
	if len(allDiffs) > maxChanges {
		allDiffs = allDiffs[:maxChanges]
	}

	// Convert all diffs to a single unified diff string
	entry.UnifiedDiff = analysis.FormatUnifiedDiff(allDiffs)
	entry.ChangeCount = len(allDiffs)

	// Generate condition history summary
	entry.StatusSummary.ConditionHistory = t.summarizeConditions(segments)

	return entry
}

// summarizeConditions extracts and summarizes status conditions across segments
func (t *ResourceTimelineChangesTool) summarizeConditions(segments []client.StatusSegment) map[string]string {
	result := make(map[string]string)

	// Track condition states over time
	type conditionState struct {
		status    string
		startTime int64
	}
	conditionTimeline := make(map[string][]conditionState)

	for _, segment := range segments {
		if len(segment.ResourceData) == 0 {
			continue
		}

		// Parse resource data to extract conditions
		var resourceData map[string]any
		if err := json.Unmarshal(segment.ResourceData, &resourceData); err != nil {
			continue
		}

		// Try to find conditions in status
		status, ok := resourceData["status"].(map[string]any)
		if !ok {
			continue
		}

		conditions, ok := status["conditions"].([]any)
		if !ok {
			continue
		}

		// Extract each condition
		for _, cond := range conditions {
			condMap, ok := cond.(map[string]any)
			if !ok {
				continue
			}

			condType, _ := condMap["type"].(string)
			condStatus, _ := condMap["status"].(string)

			if condType == "" {
				continue
			}

			// Check if this is a state change
			timeline := conditionTimeline[condType]
			if len(timeline) == 0 || timeline[len(timeline)-1].status != condStatus {
				conditionTimeline[condType] = append(timeline, conditionState{
					status:    condStatus,
					startTime: segment.StartTime,
				})
			}
		}
	}

	// Generate summary strings for important conditions
	importantConditions := []string{"Ready", "Available", "Progressing", "Initialized", "ContainersReady", "PodScheduled"}

	for _, condType := range importantConditions {
		timeline, exists := conditionTimeline[condType]
		if !exists || len(timeline) == 0 {
			continue
		}

		// Build summary string
		var parts []string
		for i, state := range timeline {
			duration := ""
			if i < len(timeline)-1 {
				durationSec := timeline[i+1].startTime - state.startTime
				duration = fmt.Sprintf("(%s)", formatDuration(durationSec))
			}
			parts = append(parts, fmt.Sprintf("%s%s", state.status, duration))
		}

		if len(parts) > 0 {
			result[condType] = strings.Join(parts, " -> ")
		}
	}

	return result
}

// formatDuration is defined in cluster_health.go in the same package

// filterDiffsByPath filters diffs based on the change filter setting
func filterDiffsByPath(diffs []analysis.EventDiff, changeFilter string) []analysis.EventDiff {
	if changeFilter == ChangeFilterAll {
		return diffs
	}

	filtered := make([]analysis.EventDiff, 0, len(diffs))
	for _, diff := range diffs {
		isStatusPath := strings.HasPrefix(diff.Path, ".status") || strings.HasPrefix(diff.Path, "status")

		switch changeFilter {
		case ChangeFilterSpecOnly:
			// Include only non-status paths
			if !isStatusPath {
				filtered = append(filtered, diff)
			}
		case ChangeFilterStatusOnly:
			// Include only status paths
			if isStatusPath {
				filtered = append(filtered, diff)
			}
		}
	}

	return filtered
}
