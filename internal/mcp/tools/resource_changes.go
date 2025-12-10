package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/mcp/client"
)

// ResourceChangesTool implements the resource_changes MCP tool
type ResourceChangesTool struct {
	client *client.SpectreClient
}

// NewResourceChangesTool creates a new resource changes tool
func NewResourceChangesTool(client *client.SpectreClient) *ResourceChangesTool {
	return &ResourceChangesTool{
		client: client,
	}
}

// ResourceChangesInput represents the input for resource_changes tool
type ResourceChangesInput struct {
	StartTime        int64   `json:"start_time"`
	EndTime          int64   `json:"end_time"`
	Kinds            string  `json:"kinds,omitempty"`         // comma-separated list
	ImpactThreshold  float64 `json:"impact_threshold,omitempty"`
	MaxResources     int     `json:"max_resources,omitempty"` // Max resources to return, default 50, max 500
}

// ChangeDetail represents a single change
type ChangeDetail struct {
	Category      string      `json:"category"` // Config, Status, Replicas, Scheduling, Security
	Field         string      `json:"field"`
	Before        interface{} `json:"before"`
	After         interface{} `json:"after"`
	Timestamp     int64       `json:"timestamp"`
	TimestampText string      `json:"timestamp_text,omitempty"` // Human-readable timestamp
}

// ResourceChangeSummary represents changes for a single resource
type ResourceChangeSummary struct {
	ResourceID        string          `json:"resource_id"`
	Kind              string          `json:"kind"`
	Namespace         string          `json:"namespace"`
	Name              string          `json:"name"`
	ImpactScore       float64         `json:"impact_score"` // 0-1.0
	Changes           []ChangeDetail  `json:"changes"`
	ChangeCount       int             `json:"change_count"`
	EventCount        int             `json:"event_count"`
	ErrorEvents       int             `json:"error_events"`
	WarningEvents     int             `json:"warning_events"`
	StatusTransitions []StatusTransition `json:"status_transitions,omitempty"`
}

// StatusTransition represents a change in resource status
type StatusTransition struct {
	FromStatus    string `json:"from_status"`
	ToStatus      string `json:"to_status"`
	Timestamp     int64  `json:"timestamp"`
	Message       string `json:"message"`
	TimestampText string `json:"timestamp_text,omitempty"` // Human-readable timestamp
}

// ResourceChangesOutput represents the output of resource_changes tool
type ResourceChangesOutput struct {
	Changes           []ResourceChangeSummary `json:"changes"`
	TotalChanges      int                     `json:"total_changes"`
	ResourcesAffected int                     `json:"resources_affected"`
	AggregationTimeMs int64                   `json:"aggregation_time_ms"`
}

// Execute runs the resource_changes tool
func (t *ResourceChangesTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ResourceChangesInput
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

	// Parse kinds filter
	filters := make(map[string]string)
	if params.Kinds != "" {
		// Only the first kind can be queried per API call
		kinds := strings.Split(params.Kinds, ",")
		filters["kind"] = strings.TrimSpace(kinds[0])
	}

	response, err := t.client.QueryTimeline(startTime, endTime, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to query timeline: %w", err)
	}

	// Apply default limit: 50 (default), max 500
	maxResources := ApplyDefaultLimit(params.MaxResources, 50, 500)

	output := t.analyzeChanges(response, params.ImpactThreshold, maxResources)
	output.AggregationTimeMs = time.Since(start).Milliseconds()

	return output, nil
}

func (t *ResourceChangesTool) analyzeChanges(response *client.TimelineResponse, impactThreshold float64, maxResources int) *ResourceChangesOutput {
	output := &ResourceChangesOutput{
		Changes: make([]ResourceChangeSummary, 0),
	}

	for _, resource := range response.Resources {
		summary := ResourceChangeSummary{
			ResourceID:        resource.ID,
			Kind:              resource.Kind,
			Namespace:         resource.Namespace,
			Name:              resource.Name,
			Changes:           make([]ChangeDetail, 0),
			StatusTransitions: make([]StatusTransition, 0),
		}

		// Count events
		for _, event := range resource.Events {
			summary.EventCount++
			if event.Type == "Warning" {
				summary.WarningEvents++
			}
			if strings.Contains(event.Reason, "Error") || strings.Contains(event.Reason, "Failed") {
				summary.ErrorEvents++
			}
		}

		// Analyze status transitions
		previousStatus := ""
		for _, segment := range resource.StatusSegments {
			if previousStatus != "" && segment.Status != previousStatus {
				message := segment.Message
				summary.StatusTransitions = append(summary.StatusTransitions, StatusTransition{
					FromStatus:    previousStatus,
					ToStatus:      segment.Status,
					Timestamp:     segment.StartTime,
					TimestampText: FormatTimestamp(segment.StartTime),
					Message:       message,
				})
			}
			previousStatus = segment.Status
		}

		// Calculate impact score
		summary.ImpactScore = t.calculateImpactScore(&summary)

		// Only include if above threshold
		if impactThreshold > 0 && summary.ImpactScore < impactThreshold {
			continue
		}

		summary.ChangeCount = len(summary.Changes)
		output.Changes = append(output.Changes, summary)
	}

	// Sort by impact score (descending)
	sort.Slice(output.Changes, func(i, j int) bool {
		return output.Changes[i].ImpactScore > output.Changes[j].ImpactScore
	})

	// Apply limit to top results
	if len(output.Changes) > maxResources {
		output.Changes = output.Changes[:maxResources]
	}

	output.ResourcesAffected = len(output.Changes)
	output.TotalChanges = output.ResourcesAffected // Simplified for now

	return output
}

func (t *ResourceChangesTool) calculateImpactScore(summary *ResourceChangeSummary) float64 {
	score := 0.0

	// Error events have high impact
	if summary.ErrorEvents > 0 {
		score += 0.3
	}

	// Warning events
	if summary.WarningEvents > 0 {
		score += 0.15
	}

	// Status transitions (especially to Error)
	for _, transition := range summary.StatusTransitions {
		if transition.ToStatus == "Error" {
			score += 0.3
		} else if transition.ToStatus == "Warning" {
			score += 0.15
		}
	}

	// High event count indicates churn
	if summary.EventCount > 10 {
		score += 0.1
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}
