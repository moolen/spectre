package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/mcp/client"
)

// EventPattern represents a detected pattern in Kubernetes events
// TODO: Reimplement this type based on previous storage.EventPattern
type EventPattern struct {
	PatternType string  `json:"pattern_type"` // FailedScheduling, Evicted, Preempted, DNSFailure
	Reason      string  `json:"reason"`
	Message     string  `json:"message"`
	Details     string  `json:"details"` // Parsed details from the message
	ImpactScore float64 `json:"impact_score"`
}

// K8sEventData represents event data structure from Kubernetes
type K8sEventData struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

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
	Namespace        string  `json:"namespace,omitempty"`      // namespace filter
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
	ResourceID        string                   `json:"resource_id"`
	Kind              string                   `json:"kind"`
	Namespace         string                   `json:"namespace"`
	Name              string                   `json:"name"`
	ImpactScore       float64                  `json:"impact_score"` // 0-1.0
	Changes           []ChangeDetail           `json:"changes"`
	ChangeCount       int                      `json:"change_count"`
	EventCount        int                      `json:"event_count"`
	ErrorEvents       int                      `json:"error_events"`
	WarningEvents     int                      `json:"warning_events"`
	StatusTransitions []StatusTransition       `json:"status_transitions,omitempty"`
	ContainerIssues   []analyzer.ContainerIssue `json:"container_issues,omitempty"` // Container-level problems
	EventPatterns     []EventPattern            `json:"event_patterns,omitempty"`   // Detected event patterns (TODO: reimplement)
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

	// Validate required parameters
	if startTime == 0 || endTime == 0 {
		return nil, fmt.Errorf("start_time and end_time are required")
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

	// Validate impact threshold
	if params.ImpactThreshold < 0 || params.ImpactThreshold > 1.0 {
		return nil, fmt.Errorf("impact_threshold must be between 0 and 1.0")
	}

	start := time.Now()

	// Parse filters
	filters := make(map[string]string)
	if params.Kinds != "" {
		// Only the first kind can be queried per API call
		kinds := strings.Split(params.Kinds, ",")
		filters["kind"] = strings.TrimSpace(kinds[0])
	}
	if params.Namespace != "" {
		filters["namespace"] = params.Namespace
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

	// Debug: log resources received
	if len(response.Resources) == 0 {
		// No resources in response - this might be the issue
		return output
	}

	for _, resource := range response.Resources {
		summary := ResourceChangeSummary{
			ResourceID:        resource.ID,
			Kind:              resource.Kind,
			Namespace:         resource.Namespace,
			Name:              resource.Name,
			Changes:           make([]ChangeDetail, 0),
			StatusTransitions: make([]StatusTransition, 0),
			ContainerIssues:   make([]analyzer.ContainerIssue, 0),
			EventPatterns:     make([]EventPattern, 0),
		}

		// Count events and analyze patterns
		for _, event := range resource.Events {
			summary.EventCount++
			if event.Type == "Warning" {
				summary.WarningEvents++
			}
			if strings.Contains(event.Reason, "Error") || strings.Contains(event.Reason, "Failed") {
				summary.ErrorEvents++
			}
		}

		// TODO: Reimplement event pattern analysis
		// Previously used storage.AnalyzeEventPatterns - needs graph-based implementation
		summary.EventPatterns = []EventPattern{}

		// Analyze status transitions and extract container issues from Pod resources
		segments := resource.StatusSegments
		
		// Track status transitions (process in forward chronological order)
		previousStatus := ""
		for _, segment := range segments {
			if previousStatus != "" && segment.Status != previousStatus {
				summary.StatusTransitions = append(summary.StatusTransitions, StatusTransition{
					FromStatus:    previousStatus,
					ToStatus:      segment.Status,
					Timestamp:     segment.StartTime,
					TimestampText: FormatTimestamp(segment.StartTime),
					Message:       segment.Message,
				})
			}
			previousStatus = segment.Status
		}
		
		// For Pod resources, check for container issues in segments
		// Process segments in REVERSE order (most recent first) to get latest container state
		if strings.EqualFold(resource.Kind, "Pod") {
			// Start from the most recent segment and work backwards
			// Collect issues from all segments with ResourceData, but prefer the most recent
			var latestIssues []analyzer.ContainerIssue
			var latestSegmentTime int64
			
			for i := len(segments) - 1; i >= 0; i-- {
				segment := segments[i]
				// Check if ResourceData is present and non-empty (not just "{}" or "null")
				if len(segment.ResourceData) > 0 {
					// Skip empty JSON objects
					resourceDataStr := strings.TrimSpace(string(segment.ResourceData))
					if resourceDataStr == "{}" || resourceDataStr == "null" || resourceDataStr == "" {
						continue
					}
					
					issues, err := analyzer.GetContainerIssuesFromJSON(segment.ResourceData)
					if err != nil {
						// Log error but continue - might be a parsing issue
						continue
					}
					if len(issues) > 0 {
						// Use the most recent segment (highest StartTime) with issues
						if segment.StartTime >= latestSegmentTime {
							latestIssues = issues
							latestSegmentTime = segment.StartTime
						}
					}
				}
			}
			
			// Fallback: If no issues found in ResourceData, try to infer from status messages and events
			if len(latestIssues) == 0 {
				var message, statusMessage string
				if len(segments) > 0 {
					// Check the most recent segment's message for container issue indicators
					mostRecentSegment := segments[len(segments)-1]
					message = strings.ToLower(mostRecentSegment.Message)
					statusMessage = strings.ToLower(mostRecentSegment.Status)
				}
				reason := ""
				var issueMessage string
				if len(segments) > 0 {
					issueMessage = segments[len(segments)-1].Message
				}
				
				// First, check events for container-related reasons (check both Reason and Message)
				// Events are more reliable than status messages
				for _, event := range resource.Events {
					eventReason := strings.ToLower(event.Reason)
					eventMessage := strings.ToLower(event.Message)
					
					// Check for CrashLoopBackOff patterns (most common)
					if strings.Contains(eventReason, "crashloopbackoff") || 
					   strings.Contains(eventMessage, "crashloopbackoff") ||
					   strings.Contains(eventMessage, "back-off") ||
					   strings.Contains(eventMessage, "backoff") ||
					   strings.Contains(eventMessage, "restarting failed") {
						reason = "CrashLoopBackOff"
						if issueMessage == "" {
							issueMessage = event.Message
						}
						break
					}
					// Check for ImagePullBackOff patterns
					if strings.Contains(eventReason, "imagepullbackoff") || 
					   strings.Contains(eventReason, "errimagepull") ||
					   strings.Contains(eventMessage, "imagepullbackoff") ||
					   strings.Contains(eventMessage, "errimagepull") ||
					   strings.Contains(eventMessage, "failed to pull") {
						reason = "ImagePullBackOff"
						if issueMessage == "" {
							issueMessage = event.Message
						}
						break
					}
					// Check for OOMKilled patterns
					if strings.Contains(eventReason, "oomkilled") || 
					   strings.Contains(eventReason, "oom") ||
					   strings.Contains(eventMessage, "oomkilled") ||
					   strings.Contains(eventMessage, "out of memory") {
						reason = "OOMKilled"
						if issueMessage == "" {
							issueMessage = event.Message
						}
						break
					}
				}
				
				// If no reason found in events, check segment messages (they often contain the issue details)
				if reason == "" {
					// Check all segments for container issue patterns (not just the most recent)
					for i := len(segments) - 1; i >= 0; i-- {
						segment := segments[i]
						segmentMsg := strings.ToLower(segment.Message)
						segmentStatus := strings.ToLower(segment.Status)
						combinedMsg := segmentMsg + " " + segmentStatus
						
						if strings.Contains(combinedMsg, "crashloopbackoff") || 
						   strings.Contains(combinedMsg, "back-off") ||
						   strings.Contains(combinedMsg, "backoff") ||
						   strings.Contains(combinedMsg, "restarting failed") {
							reason = "CrashLoopBackOff"
							issueMessage = segment.Message
							break
						} else if strings.Contains(combinedMsg, "imagepullbackoff") || 
						          strings.Contains(combinedMsg, "errimagepull") ||
						          strings.Contains(combinedMsg, "failed to pull image") {
							reason = "ImagePullBackOff"
							issueMessage = segment.Message
							break
						} else if strings.Contains(combinedMsg, "oomkilled") ||
						          strings.Contains(combinedMsg, "out of memory") {
							reason = "OOMKilled"
							issueMessage = segment.Message
							break
						}
					}
				}
				
				// If we found a reason (from events or segments), create a container issue
				if reason != "" {
					latestIssues = []analyzer.ContainerIssue{
						{
							IssueType:   reason,
							Message:     issueMessage,
							Reason:      reason,
							ImpactScore: getImpactScoreForIssueType(reason),
						},
					}
				} else if (len(segments) == 0 || statusMessage == "error" || statusMessage == "warning") && (summary.ErrorEvents > 0 || summary.WarningEvents > 0) {
					// Last resort: if status is Error/Warning (or segments empty) and we have error events, create a generic issue
					// This ensures we at least detect that there's a problem
					issueType := "CrashLoopBackOff" // Default to most common issue
					impactScore := 0.35
					fallbackReason := "CrashLoopBackOff"
					fallbackMessage := issueMessage
					if fallbackMessage == "" {
						fallbackMessage = message
					}
					
					for _, event := range resource.Events {
						eventReasonLower := strings.ToLower(event.Reason)
						eventMessageLower := strings.ToLower(event.Message)
						
						// Check for specific issue types in event reason/message
						if strings.Contains(eventReasonLower, "imagepull") || strings.Contains(eventMessageLower, "imagepull") {
							issueType = "ImagePullBackOff"
							impactScore = 0.25
							fallbackReason = event.Reason
							fallbackMessage = event.Message
							break
						} else if strings.Contains(eventReasonLower, "oom") || strings.Contains(eventMessageLower, "oom") {
							issueType = "OOMKilled"
							impactScore = 0.40
							fallbackReason = event.Reason
							fallbackMessage = event.Message
							break
						} else if event.Type == "Warning" || strings.Contains(eventReasonLower, "error") ||
						   strings.Contains(eventReasonLower, "failed") {
							fallbackReason = event.Reason
							if fallbackMessage == "" {
								fallbackMessage = event.Message
							}
							break
						}
					}
					
					// Create a container issue based on the error
					latestIssues = []analyzer.ContainerIssue{
						{
							IssueType:   issueType,
							Message:     fallbackMessage,
							Reason:      fallbackReason,
							ImpactScore: impactScore,
						},
					}
				}
			}
			
			// If we found issues, use them
			if len(latestIssues) > 0 {
				summary.ContainerIssues = latestIssues
			}
		}

		// Calculate impact score
		summary.ImpactScore = calculateImpactScore(&summary)

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

// getImpactScoreForIssueType returns the impact score for a given issue type
func getImpactScoreForIssueType(issueType string) float64 {
	switch issueType {
	case "OOMKilled":
		return 0.40
	case "CrashLoopBackOff":
		return 0.35
	case "ImagePullBackOff":
		return 0.25
	default:
		return 0.20
	}
}

// calculateImpactScore calculates the impact score for a resource change summary
func calculateImpactScore(summary *ResourceChangeSummary) float64 {
	score := 0.0

	// Container-level issues (Phase 1)
	for _, issue := range summary.ContainerIssues {
		switch issue.IssueType {
		case "OOMKilled":
			score += 0.40 // High impact - resource issue
		case "CrashLoopBackOff":
			score += 0.35 // High impact - app issue
		case "ImagePullBackOff":
			score += 0.25 // Medium impact - config issue
		case "VeryHighRestartCount":
			score += 0.35 // High restart count
		case "HighRestartCount":
			score += 0.20 // Moderate restart count
		}
	}

	// Event patterns (Phase 2 & 3)
	for _, pattern := range summary.EventPatterns {
		switch pattern.PatternType {
		case "Evicted":
			score += 0.35 // High impact - node pressure
		case "LivenessProbe":
			score += 0.35 // High impact - will cause restarts
		case "FailedScheduling", "DNSFailure", "StartupProbe":
			score += 0.30 // Medium-high impact
		case "ReadinessProbe", "Preempted", "ProbeFailure":
			score += 0.25 // Medium impact
		}
	}

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

	// Very high event volume
	if summary.EventCount > 50 {
		score += 0.2
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}
