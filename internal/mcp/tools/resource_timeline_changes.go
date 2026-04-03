package tools

import (
	apptimeline "github.com/moolen/spectre/internal/app/timeline"
)

// ResourceTimelineChangesTool implements the resource_timeline_changes MCP tool
// which returns semantic field-level diffs for specific resources by UID.
type ResourceTimelineChangesTool struct {
	timelineService *apptimeline.Service
}

// NewResourceTimelineChangesTool creates a new resource timeline changes tool.
func NewResourceTimelineChangesTool(timelineService *apptimeline.Service) *ResourceTimelineChangesTool {
	return &ResourceTimelineChangesTool{
		timelineService: timelineService,
	}
}

// ChangeFilter constants for filtering which changes to include.
const (
	// ChangeFilterAll returns all changes (default).
	ChangeFilterAll = "all"
	// ChangeFilterSpecOnly returns only spec changes (excludes status).
	ChangeFilterSpecOnly = "spec_only"
	// ChangeFilterStatusOnly returns only status changes.
	ChangeFilterStatusOnly = "status_only"
)

// ResourceTimelineChangesInput represents the input for resource_timeline_changes tool.
type ResourceTimelineChangesInput struct {
	// ResourceUIDs is a list of resource UIDs to query (required, max 10).
	ResourceUIDs []string `json:"resource_uids"`

	// StartTime is the start of the time window (Unix seconds/ms, optional).
	// Default: 1 hour ago.
	StartTime int64 `json:"start_time,omitempty"`

	// EndTime is the end of the time window (Unix seconds/ms, optional).
	// Default: now.
	EndTime int64 `json:"end_time,omitempty"`

	// IncludeFullSnapshot returns the first segment's full resource JSON (optional).
	// Default: false (for token efficiency).
	IncludeFullSnapshot bool `json:"include_full_snapshot,omitempty"`

	// MaxChangesPerResource limits changes returned per resource (optional).
	// Default: 50, Max: 200.
	MaxChangesPerResource int `json:"max_changes_per_resource,omitempty"`

	// ChangeFilter controls which types of changes to include (optional).
	// Values: "all" (default), "spec_only", "status_only".
	ChangeFilter string `json:"change_filter,omitempty"`
}

// ResourceTimelineChangesOutput represents the output of resource_timeline_changes tool.
type ResourceTimelineChangesOutput struct {
	// Resources contains changes grouped by resource.
	Resources []ResourceTimelineEntry `json:"resources"`

	// Summary provides aggregate statistics.
	Summary TimelineChangesSummary `json:"summary"`

	// ExecutionTimeMs is the query execution time.
	ExecutionTimeMs int64 `json:"execution_time_ms"`
}

// ResourceTimelineEntry contains timeline changes for a single resource.
type ResourceTimelineEntry struct {
	// Resource identification.
	UID       string `json:"uid"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`

	// UnifiedDiff is a git-style unified diff of all changes (compact format).
	UnifiedDiff string `json:"unified_diff,omitempty"`

	// StatusSummary is a condensed view of status transitions.
	StatusSummary StatusConditionSummary `json:"status_summary"`

	// FirstSnapshot is the full resource JSON at first segment (if requested).
	FirstSnapshot map[string]any `json:"first_snapshot,omitempty"`

	// ChangeCount is the total number of changes detected.
	ChangeCount int `json:"change_count"`

	// Error is set if this resource could not be found or processed.
	Error string `json:"error,omitempty"`
}

// StatusConditionSummary provides a condensed view of status condition changes.
type StatusConditionSummary struct {
	// CurrentStatus is the latest status (Ready, Warning, Error, etc.).
	CurrentStatus string `json:"current_status"`

	// Transitions is a list of major status transitions.
	Transitions []StatusTransitionSummary `json:"transitions,omitempty"`

	// ConditionHistory is a per-condition summary (e.g., "Ready: True(5m)->False(2m)").
	ConditionHistory map[string]string `json:"condition_history,omitempty"`
}

// StatusTransitionSummary represents a status state change.
type StatusTransitionSummary struct {
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
	Timestamp  int64  `json:"timestamp"`
	Reason     string `json:"reason,omitempty"`
}

// TimelineChangesSummary provides aggregate statistics.
type TimelineChangesSummary struct {
	TotalResources      int   `json:"total_resources"`
	TotalChanges        int   `json:"total_changes"`
	ResourcesWithErrors int   `json:"resources_with_errors"`
	ResourcesNotFound   int   `json:"resources_not_found"`
	TimeRangeStart      int64 `json:"time_range_start"`
	TimeRangeEnd        int64 `json:"time_range_end"`
}
