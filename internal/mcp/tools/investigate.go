package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/mcp/client"
)

// InvestigateTool implements the investigate MCP tool
type InvestigateTool struct {
	client *client.SpectreClient
}

// NewInvestigateTool creates a new investigate tool
func NewInvestigateTool(client *client.SpectreClient) *InvestigateTool {
	return &InvestigateTool{
		client: client,
	}
}

// InvestigateInput represents the input for investigate tool
type InvestigateInput struct {
	ResourceKind       string `json:"resource_kind"`
	ResourceName       string `json:"resource_name,omitempty"`  // "*" for all
	Namespace          string `json:"namespace,omitempty"`
	StartTime          int64  `json:"start_time"`
	EndTime            int64  `json:"end_time"`
	InvestigationType  string `json:"investigation_type,omitempty"` // incident, post-mortem, auto
	MaxInvestigations  int    `json:"max_investigations,omitempty"` // Max resources to investigate when using "*", default 20, max 100
}

// InvestigationEvidence represents evidence for investigation
type InvestigationEvidence struct {
	ResourceID            string            `json:"resource_id"`
	Kind                  string            `json:"kind"`
	Namespace             string            `json:"namespace"`
	Name                  string            `json:"name"`
	CurrentStatus         string            `json:"current_status"`
	CurrentMessage        string            `json:"current_message"`
	TimelineStart         int64             `json:"timeline_start"`
	TimelineEnd           int64             `json:"timeline_end"`
	TimelineStartText     string            `json:"timeline_start_text,omitempty"` // Human-readable timestamp
	TimelineEndText       string            `json:"timeline_end_text,omitempty"`   // Human-readable timestamp
	StatusSegments        []SegmentSummary  `json:"status_segments"`
	Events                []EventSummary    `json:"events"`
	InvestigationPrompts  []string          `json:"investigation_prompts"`
	RawResourceSnapshots  []ResourceSnapshot `json:"raw_resource_snapshots,omitempty"`
}

// SegmentSummary summarizes a status segment
type SegmentSummary struct {
	StartTime     int64  `json:"start_time"`
	EndTime       int64  `json:"end_time"`
	Duration      int64  `json:"duration_seconds"`
	Status        string `json:"status"`
	Message       string `json:"message"`
	StartTimeText string `json:"start_time_text,omitempty"` // Human-readable timestamp
	EndTimeText   string `json:"end_time_text,omitempty"`   // Human-readable timestamp
}

// EventSummary summarizes an event
type EventSummary struct {
	Timestamp         int64  `json:"timestamp"`
	Reason            string `json:"reason"`
	Message           string `json:"message"`
	Type              string `json:"type"` // Normal, Warning
	Count             int32  `json:"count"`
	Source            string `json:"source"`
	FirstTimestamp    int64  `json:"first_timestamp"`
	LastTimestamp     int64  `json:"last_timestamp"`
	TimestampText     string `json:"timestamp_text,omitempty"`       // Human-readable timestamp
	FirstTimestampText string `json:"first_timestamp_text,omitempty"` // Human-readable timestamp
	LastTimestampText  string `json:"last_timestamp_text,omitempty"`  // Human-readable timestamp
}

// ResourceSnapshot represents a snapshot of a resource at a point in time
type ResourceSnapshot struct {
	Timestamp     int64    `json:"timestamp"`
	Status        string   `json:"status"`
	Message       string   `json:"message"`
	KeyChanges    []string `json:"key_changes,omitempty"`    // Summary of important changes, e.g., ["replicas: 3->1", "image: v1.2->v1.3"]
	TimestampText string   `json:"timestamp_text,omitempty"` // Human-readable timestamp
}

// InvestigateOutput represents the output of investigate tool
type InvestigateOutput struct {
	Investigations       []InvestigationEvidence `json:"investigations"`
	InvestigationTimeMs  int64                   `json:"investigation_time_ms"`
}

// Execute runs the investigate tool
func (t *InvestigateTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params InvestigateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	startTime := params.StartTime
	endTime := params.EndTime

	// Convert milliseconds to seconds if needed
	if startTime > 10000000000 {
		startTime = startTime / 1000
	}
	if endTime > 10000000000 {
		endTime = endTime / 1000
	}

	if startTime >= endTime {
		return nil, fmt.Errorf("start_time must be before end_time")
	}

	start := time.Now()

	filters := make(map[string]string)
	if params.ResourceKind != "" {
		filters["kind"] = params.ResourceKind
	}
	if params.Namespace != "" {
		filters["namespace"] = params.Namespace
	}

	response, err := t.client.QueryTimeline(startTime, endTime, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to query timeline: %w", err)
	}

	investigations := make([]InvestigationEvidence, 0)

	// Apply default limit: 20 (default), max 100
	maxInvestigations := ApplyDefaultLimit(params.MaxInvestigations, 20, 100)
	processedCount := 0

	for _, resource := range response.Resources {
		// Filter by name if specified and not "*"
		if params.ResourceName != "" && params.ResourceName != "*" && resource.Name != params.ResourceName {
			continue
		}

		// Apply limit when using wildcard
		if params.ResourceName == "*" || params.ResourceName == "" {
			if processedCount >= maxInvestigations {
				break
			}
			processedCount++
		}

		evidence := t.buildInvestigationEvidence(&resource, params.InvestigationType)
		investigations = append(investigations, evidence)
	}

	output := &InvestigateOutput{
		Investigations:      investigations,
		InvestigationTimeMs: time.Since(start).Milliseconds(),
	}

	return output, nil
}

func (t *InvestigateTool) buildInvestigationEvidence(resource *client.TimelineResource, investigationType string) InvestigationEvidence {
	timelineStart := getMinTimestamp(resource)
	timelineEnd := getMaxTimestamp(resource)
	evidence := InvestigationEvidence{
		ResourceID:        resource.ID,
		Kind:              resource.Kind,
		Namespace:         resource.Namespace,
		Name:              resource.Name,
		TimelineStart:     timelineStart,
		TimelineEnd:       timelineEnd,
		TimelineStartText: FormatTimestamp(timelineStart),
		TimelineEndText:   FormatTimestamp(timelineEnd),
		StatusSegments:    make([]SegmentSummary, 0),
		Events:            make([]EventSummary, 0),
		InvestigationPrompts: make([]string, 0),
		RawResourceSnapshots: make([]ResourceSnapshot, 0),
	}

	// Get current status
	if len(resource.StatusSegments) > 0 {
		lastSegment := resource.StatusSegments[len(resource.StatusSegments)-1]
		evidence.CurrentStatus = lastSegment.Status
		evidence.CurrentMessage = lastSegment.Message
	}

	// Summarize status segments
	for _, segment := range resource.StatusSegments {
		summary := SegmentSummary{
			StartTime:     segment.StartTime,
			EndTime:       segment.EndTime,
			Duration:      segment.EndTime - segment.StartTime,
			Status:        segment.Status,
			Message:       segment.Message,
			StartTimeText: FormatTimestamp(segment.StartTime),
			EndTimeText:   FormatTimestamp(segment.EndTime),
		}
		evidence.StatusSegments = append(evidence.StatusSegments, summary)

		// Include snapshot summary for key transitions (Error/Warning only)
		// Skip raw resource data to save tokens - message contains key information
		if segment.Status == "Error" || segment.Status == "Warning" {
			evidence.RawResourceSnapshots = append(evidence.RawResourceSnapshots, ResourceSnapshot{
				Timestamp:     segment.StartTime,
				Status:        segment.Status,
				Message:       segment.Message,
				KeyChanges:    []string{}, // Could be enhanced with diff analysis in the future
				TimestampText: FormatTimestamp(segment.StartTime),
			})
		}
	}

	// Summarize events
	for _, event := range resource.Events {
		summary := EventSummary{
			Timestamp:         event.Timestamp,
			Reason:            event.Reason,
			Message:           event.Message,
			Type:              event.Type,
			Count:             event.Count,
			Source:            event.Source,
			FirstTimestamp:    event.FirstTimestamp,
			LastTimestamp:     event.LastTimestamp,
			TimestampText:     FormatTimestamp(event.Timestamp),
			FirstTimestampText: FormatTimestamp(event.FirstTimestamp),
			LastTimestampText:  FormatTimestamp(event.LastTimestamp),
		}
		evidence.Events = append(evidence.Events, summary)
	}

	// Generate investigation prompts
	evidence.InvestigationPrompts = t.generatePrompts(&evidence, resource, investigationType)

	return evidence
}

func (t *InvestigateTool) generatePrompts(evidence *InvestigationEvidence, resource *client.TimelineResource, investigationType string) []string {
	prompts := make([]string, 0)

	// Default investigation type
	if investigationType == "" {
		if evidence.CurrentStatus == "Error" {
			investigationType = "incident"
		} else {
			investigationType = "post-mortem"
		}
	}

	// Common prompts for all types
	prompts = append(prompts,
		fmt.Sprintf("Analyze the status transitions for %s/%s. What caused the transition from %s to %s?",
			evidence.Kind, evidence.Name, getPreviousStatus(evidence), evidence.CurrentStatus),
	)

	// Count error and warning events
	errorEventCount := 0
	warningEventCount := 0
	for _, event := range evidence.Events {
		if event.Type == "Warning" {
			warningEventCount++
		}
		if strings.Contains(event.Reason, "Error") || strings.Contains(event.Reason, "Failed") {
			errorEventCount++
		}
	}

	// Incident-specific prompts
	if investigationType == "incident" {
		if evidence.CurrentStatus == "Error" {
			prompts = append(prompts,
				fmt.Sprintf("The %s %s is currently in Error state since %d seconds. What are the immediate mitigation steps?",
					evidence.Kind, evidence.Name, getErrorDuration(evidence)),
				fmt.Sprintf("Based on the events, what is the root cause of the current error in %s?", evidence.Name),
			)
		} else if evidence.CurrentStatus == "Warning" {
			prompts = append(prompts,
				fmt.Sprintf("The %s %s is in Warning state. What should we monitor to prevent escalation to Error?", evidence.Kind, evidence.Name),
			)
		}

		if errorEventCount > 0 {
			prompts = append(prompts,
				fmt.Sprintf("There are %d error events. Summarize the error pattern and suggest preventive measures.", errorEventCount),
			)
		}
	}

	// Post-mortem specific prompts
	if investigationType == "post-mortem" {
		prompts = append(prompts,
			fmt.Sprintf("Create a timeline of what happened to %s/%s during this period.", evidence.Kind, evidence.Name),
			fmt.Sprintf("What were the contributing factors that led to the status changes in %s?", evidence.Name),
		)

		if len(evidence.StatusSegments) > 1 {
			prompts = append(prompts,
				fmt.Sprintf("How long did %s remain in each problematic state? Suggest improvements.", evidence.Name),
			)
		}

		if errorEventCount > 0 {
			prompts = append(prompts,
				fmt.Sprintf("Document the %d error events and identify any patterns that could be prevented.", errorEventCount),
			)
		}
	}

	// Additional context prompts
	if evidence.CurrentMessage != "" {
		prompts = append(prompts,
			fmt.Sprintf("Interpret the current message: '%s'. What does it mean?", evidence.CurrentMessage),
		)
	}

	return prompts
}

func getMinTimestamp(resource *client.TimelineResource) int64 {
	if len(resource.StatusSegments) > 0 {
		return resource.StatusSegments[0].StartTime
	}
	if len(resource.Events) > 0 {
		return resource.Events[0].Timestamp
	}
	return 0
}

func getMaxTimestamp(resource *client.TimelineResource) int64 {
	max := int64(0)
	if len(resource.StatusSegments) > 0 {
		max = resource.StatusSegments[len(resource.StatusSegments)-1].EndTime
	}
	if len(resource.Events) > 0 {
		lastEvent := resource.Events[len(resource.Events)-1]
		if lastEvent.Timestamp > max {
			max = lastEvent.Timestamp
		}
	}
	return max
}

func getPreviousStatus(evidence *InvestigationEvidence) string {
	if len(evidence.StatusSegments) > 1 {
		return evidence.StatusSegments[len(evidence.StatusSegments)-2].Status
	}
	return "Unknown"
}

func getErrorDuration(evidence *InvestigationEvidence) int64 {
	if len(evidence.StatusSegments) > 0 {
		lastSegment := evidence.StatusSegments[len(evidence.StatusSegments)-1]
		if lastSegment.Status == "Error" || lastSegment.Status == "Warning" {
			return lastSegment.Duration
		}
	}
	return 0
}
