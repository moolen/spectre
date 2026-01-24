package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/models"
)

// ResourceTimelineTool implements the resource_timeline MCP tool
type ResourceTimelineTool struct {
	timelineService *api.TimelineService
}

// NewResourceTimelineTool creates a new resource_timeline tool using TimelineService
func NewResourceTimelineTool(timelineService *api.TimelineService) *ResourceTimelineTool {
	return &ResourceTimelineTool{
		timelineService: timelineService,
	}
}

// ResourceTimelineInput represents the input for resource_timeline tool
type ResourceTimelineInput struct {
	ResourceKind string `json:"resource_kind"`
	ResourceName string `json:"resource_name,omitempty"` // "*" for all
	Namespace    string `json:"namespace,omitempty"`
	StartTime    int64  `json:"start_time"`
	EndTime      int64  `json:"end_time"`
	MaxResults   int    `json:"max_results,omitempty"` // Max resources to return when using "*", default 20, max 100
}

// ResourceTimelineEvidence represents timeline evidence for a resource
type ResourceTimelineEvidence struct {
	ResourceUID          string             `json:"resource_uid"` // UID for use with other tools (UUID only)
	Kind                 string             `json:"kind"`
	Namespace            string             `json:"namespace"`
	Name                 string             `json:"name"`
	CurrentStatus        string             `json:"current_status"`
	CurrentMessage       string             `json:"current_message"`
	TimelineStart        int64              `json:"timeline_start"`
	TimelineEnd          int64              `json:"timeline_end"`
	StatusSegments       []SegmentSummary   `json:"status_segments"`
	Events               []EventSummary     `json:"events"`
	RawResourceSnapshots []ResourceSnapshot `json:"raw_resource_snapshots,omitempty"`
}

// ResourceTimelineOutput represents the output of resource_timeline tool
type ResourceTimelineOutput struct {
	Timelines       []ResourceTimelineEvidence `json:"timelines"`
	ExecutionTimeMs int64                      `json:"execution_time_ms"`
}

// SegmentSummary represents a status segment summary
type SegmentSummary struct {
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
	Duration  int64  `json:"duration,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// EventSummary represents an event summary
type EventSummary struct {
	Timestamp      int64  `json:"timestamp"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	Type           string `json:"type"`
	Count          int32  `json:"count"`
	Source         string `json:"source"`
	FirstTimestamp int64  `json:"first_timestamp"`
	LastTimestamp  int64  `json:"last_timestamp"`
}

// ResourceSnapshot represents a snapshot of a resource at a point in time
type ResourceSnapshot struct {
	Timestamp  int64    `json:"timestamp"`
	Status     string   `json:"status"`
	Message    string   `json:"message"`
	KeyChanges []string `json:"key_changes"`
}

// Execute runs the resource_timeline tool
func (t *ResourceTimelineTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ResourceTimelineInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	startTime := params.StartTime
	endTime := params.EndTime

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

	start := time.Now()

	// Build filter map for service
	filterParams := make(map[string][]string)
	if params.ResourceKind != "" {
		filterParams["kind"] = []string{params.ResourceKind}
	}
	if params.Namespace != "" {
		filterParams["namespace"] = []string{params.Namespace}
	}

	// Use TimelineService to parse and execute query
	startStr := fmt.Sprintf("%d", startTime)
	endStr := fmt.Sprintf("%d", endTime)

	query, err := t.timelineService.ParseQueryParameters(ctx, startStr, endStr, filterParams)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	// Execute query using service
	queryResult, eventResult, err := t.timelineService.ExecuteConcurrentQueries(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute timeline query: %w", err)
	}

	// Build timeline response using service
	response := t.timelineService.BuildTimelineResponse(queryResult, eventResult)

	timelines := make([]ResourceTimelineEvidence, 0)

	// Apply default limit: 20 (default), max 100
	maxResults := ApplyDefaultLimit(params.MaxResults, 20, 100)
	processedCount := 0

	for _, resource := range response.Resources {
		// Filter by name if specified and not "*"
		if params.ResourceName != "" && params.ResourceName != "*" && resource.Name != params.ResourceName {
			continue
		}

		// Apply limit when using wildcard
		if params.ResourceName == "*" || params.ResourceName == "" {
			if processedCount >= maxResults {
				break
			}
			processedCount++
		}

		evidence := t.buildResourceTimelineEvidence(&resource)
		timelines = append(timelines, evidence)
	}

	output := &ResourceTimelineOutput{
		Timelines:       timelines,
		ExecutionTimeMs: time.Since(start).Milliseconds(),
	}

	return output, nil
}

func (t *ResourceTimelineTool) buildResourceTimelineEvidence(resource *models.Resource) ResourceTimelineEvidence {
	timelineStart := getMinTimestampRT(resource)
	timelineEnd := getMaxTimestampRT(resource)

	// resource.ID is already just the UUID from models.Resource
	evidence := ResourceTimelineEvidence{
		ResourceUID:          resource.ID,
		Kind:                 resource.Kind,
		Namespace:            resource.Namespace,
		Name:                 resource.Name,
		TimelineStart:        timelineStart,
		TimelineEnd:          timelineEnd,
		StatusSegments:       make([]SegmentSummary, 0),
		Events:               make([]EventSummary, 0),
		RawResourceSnapshots: make([]ResourceSnapshot, 0),
	}

	// Get current status from the last segment
	if len(resource.StatusSegments) > 0 {
		lastSegment := resource.StatusSegments[len(resource.StatusSegments)-1]
		evidence.CurrentStatus = lastSegment.Status
		evidence.CurrentMessage = lastSegment.Message
	}

	// Deduplicate and summarize status segments
	evidence.StatusSegments = t.deduplicateStatusSegments(resource.StatusSegments)

	// Add raw snapshots for Error/Warning transitions (after deduplication)
	for _, segment := range evidence.StatusSegments {
		if segment.Status == statusError || segment.Status == statusWarning {
			evidence.RawResourceSnapshots = append(evidence.RawResourceSnapshots, ResourceSnapshot{
				Timestamp:  segment.StartTime,
				Status:     segment.Status,
				Message:    segment.Message,
				KeyChanges: []string{},
			})
		}
	}

	// Summarize events with message truncation
	for _, event := range resource.Events {
		summary := EventSummary{
			Timestamp:      event.Timestamp,
			Reason:         event.Reason,
			Message:        TruncateMessage(event.Message, 256, 256), // Keep first 256 + last 256 chars
			Type:           event.Type,
			Count:          event.Count,
			Source:         event.Source,
			FirstTimestamp: event.FirstTimestamp,
			LastTimestamp:  event.LastTimestamp,
		}
		evidence.Events = append(evidence.Events, summary)
	}

	// Cap events to most recent 25
	if len(evidence.Events) > 25 {
		// Sort by timestamp descending (most recent first)
		sort.Slice(evidence.Events, func(i, j int) bool {
			return evidence.Events[i].Timestamp > evidence.Events[j].Timestamp
		})
		evidence.Events = evidence.Events[:25]
	}

	return evidence
}

// deduplicateStatusSegments merges adjacent segments with the same Status and Message.
// Keeps the earliest StartTime and latest EndTime for merged segments.
func (t *ResourceTimelineTool) deduplicateStatusSegments(segments []models.StatusSegment) []SegmentSummary {
	if len(segments) == 0 {
		return []SegmentSummary{}
	}

	result := make([]SegmentSummary, 0, len(segments))

	// Start with the first segment
	current := SegmentSummary{
		StartTime: segments[0].StartTime,
		EndTime:   segments[0].EndTime,
		Status:    segments[0].Status,
		Message:   segments[0].Message,
	}

	for i := 1; i < len(segments); i++ {
		seg := segments[i]

		// Check if this segment should be merged with the current one
		if seg.Status == current.Status && seg.Message == current.Message {
			// Merge: extend the end time (keep original start time)
			current.EndTime = seg.EndTime
		} else {
			// Different status/message: finalize current and start new
			current.Duration = current.EndTime - current.StartTime
			result = append(result, current)

			current = SegmentSummary{
				StartTime: seg.StartTime,
				EndTime:   seg.EndTime,
				Status:    seg.Status,
				Message:   seg.Message,
			}
		}
	}

	// Don't forget to add the last segment
	current.Duration = current.EndTime - current.StartTime
	result = append(result, current)

	return result
}

// Helper functions with RT suffix to avoid conflicts with existing functions
func getMinTimestampRT(resource *models.Resource) int64 {
	if len(resource.StatusSegments) > 0 {
		return resource.StatusSegments[0].StartTime
	}
	if len(resource.Events) > 0 {
		return resource.Events[0].Timestamp
	}
	return 0
}

func getMaxTimestampRT(resource *models.Resource) int64 {
	maxTimestamp := int64(0)
	if len(resource.StatusSegments) > 0 {
		maxTimestamp = resource.StatusSegments[len(resource.StatusSegments)-1].EndTime
	}
	if len(resource.Events) > 0 {
		lastEvent := resource.Events[len(resource.Events)-1]
		if lastEvent.Timestamp > maxTimestamp {
			maxTimestamp = lastEvent.Timestamp
		}
	}
	return maxTimestamp
}
