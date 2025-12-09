package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/mcp/client"
)

// ClusterHealthTool implements the cluster_health MCP tool
type ClusterHealthTool struct {
	client *client.SpectreClient
}

// NewClusterHealthTool creates a new cluster health tool
func NewClusterHealthTool(client *client.SpectreClient) *ClusterHealthTool {
	return &ClusterHealthTool{
		client: client,
	}
}

// Input represents the input for cluster_health tool
type ClusterHealthInput struct {
	StartTime    int64  `json:"start_time"`
	EndTime      int64  `json:"end_time"`
	Namespace    string `json:"namespace,omitempty"`
	MaxResources int    `json:"max_resources,omitempty"` // Max resources to list per status, default 100, max 500
}

// ResourceStatusCount represents count of resources in each status
type ResourceStatusCount struct {
	Kind                string   `json:"kind"`
	Ready               int      `json:"ready"`
	Warning             int      `json:"warning"`
	Error               int      `json:"error"`
	Terminating         int      `json:"terminating"`
	Unknown             int      `json:"unknown"`
	TotalCount          int      `json:"total_count"`
	ErrorRate           float64  `json:"error_rate"`
	WarningResources    []string `json:"warning_resources,omitempty"`
	ErrorResources      []string `json:"error_resources,omitempty"`
	TerminatingResources []string `json:"terminating_resources,omitempty"`
	UnknownResources    []string `json:"unknown_resources,omitempty"`
}

// Issue represents a resource with persistent issues
type Issue struct {
	ResourceID        string `json:"resource_id"`
	Kind              string `json:"kind"`
	Namespace         string `json:"namespace"`
	Name              string `json:"name"`
	CurrentStatus     string `json:"current_status"`
	ErrorDuration     int64  `json:"error_duration_seconds"`
	ErrorMessage      string `json:"error_message"`
	EventCount        int    `json:"event_count"`
	ErrorDurationText string `json:"error_duration_text,omitempty"` // Human-readable duration
}

// ClusterHealthOutput represents the output of cluster_health tool
type ClusterHealthOutput struct {
	OverallStatus       string                  `json:"overall_status"` // Healthy, Degraded, Critical
	TotalResources      int                     `json:"total_resources"`
	ResourcesByKind     []ResourceStatusCount   `json:"resources_by_kind"`
	TopIssues           []Issue                 `json:"top_issues,omitempty"`
	ErrorResourceCount  int                     `json:"error_resource_count"`
	WarningResourceCount int                    `json:"warning_resource_count"`
	HealthyResourceCount int                    `json:"healthy_resource_count"`
	AggregationTimeMs   int64                   `json:"aggregation_time_ms"`
}

// Execute runs the cluster_health tool
func (t *ClusterHealthTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ClusterHealthInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Parse timestamps - if they're in milliseconds, convert to seconds
	startTime := params.StartTime
	endTime := params.EndTime

	// If timestamps look like milliseconds, convert to seconds
	if startTime > 10000000000 {
		startTime = startTime / 1000
	}
	if endTime > 10000000000 {
		endTime = endTime / 1000
	}

	// Validate time range
	if startTime >= endTime {
		return nil, fmt.Errorf("start_time must be before end_time")
	}

	start := time.Now()
	filters := make(map[string]string)
	if params.Namespace != "" {
		filters["namespace"] = params.Namespace
	}

	response, err := t.client.QueryTimeline(startTime, endTime, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to query timeline: %w", err)
	}

	// Apply default limit: 100 (default), max 500
	maxResources := ApplyDefaultLimit(params.MaxResources, 100, 500)

	output := t.analyzeHealth(response, maxResources)
	output.AggregationTimeMs = time.Since(start).Milliseconds()

	return output, nil
}

func (t *ClusterHealthTool) analyzeHealth(response *client.TimelineResponse, maxResources int) *ClusterHealthOutput {
	output := &ClusterHealthOutput{
		ResourcesByKind: make([]ResourceStatusCount, 0),
	}

	// Count resources by kind and status
	kindStatusMap := make(map[string]map[string]int)
	// Track resource names by kind and status
	kindResourceNamesMap := make(map[string]map[string][]string)
	issues := make([]Issue, 0)

	for _, resource := range response.Resources {
		// Initialize kind map if not exists
		if kindStatusMap[resource.Kind] == nil {
			kindStatusMap[resource.Kind] = make(map[string]int)
			kindResourceNamesMap[resource.Kind] = make(map[string][]string)
		}

		// Get current status from last segment
		currentStatus := "Unknown"
		errorDuration := int64(0)
		errorMessage := ""

		if len(resource.StatusSegments) > 0 {
			lastSegment := resource.StatusSegments[len(resource.StatusSegments)-1]
			currentStatus = lastSegment.Status
			errorMessage = lastSegment.Message

			// Calculate error duration if in error state
			if lastSegment.Status == "Error" || lastSegment.Status == "Warning" {
				errorDuration = lastSegment.EndTime - lastSegment.StartTime
			}
		}

		kindStatusMap[resource.Kind][currentStatus]++
		// Track resource name for non-ready statuses
		if currentStatus != "Ready" {
			resourceFullName := fmt.Sprintf("%s/%s", resource.Namespace, resource.Name)
			kindResourceNamesMap[resource.Kind][currentStatus] = append(kindResourceNamesMap[resource.Kind][currentStatus], resourceFullName)
		}
		output.TotalResources++

		// Track issues
		if currentStatus == "Error" || currentStatus == "Warning" {
			issues = append(issues, Issue{
				ResourceID:        resource.ID,
				Kind:              resource.Kind,
				Namespace:         resource.Namespace,
				Name:              resource.Name,
				CurrentStatus:     currentStatus,
				ErrorDuration:     errorDuration,
				ErrorMessage:      errorMessage,
				EventCount:        len(resource.Events),
				ErrorDurationText: formatDuration(errorDuration),
			})
		}

		// Count by status for overall calculation
		if currentStatus == "Ready" {
			output.HealthyResourceCount++
		} else if currentStatus == "Error" {
			output.ErrorResourceCount++
		} else if currentStatus == "Warning" {
			output.WarningResourceCount++
		}
	}

	// Build resource status counts
	for kind, statuses := range kindStatusMap {
		count := ResourceStatusCount{
			Kind:                 kind,
			Ready:                statuses["Ready"],
			Warning:              statuses["Warning"],
			Error:                statuses["Error"],
			Terminating:          statuses["Terminating"],
			Unknown:              statuses["Unknown"],
			WarningResources:     TruncateList(kindResourceNamesMap[kind]["Warning"], maxResources),
			ErrorResources:       TruncateList(kindResourceNamesMap[kind]["Error"], maxResources),
			TerminatingResources: TruncateList(kindResourceNamesMap[kind]["Terminating"], maxResources),
			UnknownResources:     TruncateList(kindResourceNamesMap[kind]["Unknown"], maxResources),
		}
		count.TotalCount = count.Ready + count.Warning + count.Error + count.Terminating + count.Unknown

		if count.TotalCount > 0 {
			count.ErrorRate = float64(count.Error+count.Warning) / float64(count.TotalCount)
		}

		output.ResourcesByKind = append(output.ResourcesByKind, count)
	}

	// Sort by kind name for consistent output
	sort.Slice(output.ResourcesByKind, func(i, j int) bool {
		return output.ResourcesByKind[i].Kind < output.ResourcesByKind[j].Kind
	})

	// Determine overall status
	if output.ErrorResourceCount > 0 {
		output.OverallStatus = "Critical"
	} else if output.WarningResourceCount > 0 {
		output.OverallStatus = "Degraded"
	} else {
		output.OverallStatus = "Healthy"
	}

	// Sort issues by error duration (descending) and take top 10
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].ErrorDuration != issues[j].ErrorDuration {
			return issues[i].ErrorDuration > issues[j].ErrorDuration
		}
		return issues[i].EventCount > issues[j].EventCount
	})

	if len(issues) > 10 {
		output.TopIssues = issues[:10]
	} else {
		output.TopIssues = issues
	}

	return output
}

// formatDuration converts seconds to a human-readable format (e.g., "2h 30m 45s")
func formatDuration(seconds int64) string {
	if seconds == 0 {
		return ""
	}

	duration := time.Duration(seconds) * time.Second
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	secs := int(duration.Seconds()) % 60

	parts := []string{}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if secs > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", secs))
	}

	result := ""
	for i, part := range parts {
		if i > 0 {
			result += " "
		}
		result += part
	}
	return result
}
