package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/mcp/client"
)

// ResourceExplorerTool implements the resource_explorer MCP tool
type ResourceExplorerTool struct {
	client *client.SpectreClient
}

// NewResourceExplorerTool creates a new resource explorer tool
func NewResourceExplorerTool(client *client.SpectreClient) *ResourceExplorerTool {
	return &ResourceExplorerTool{
		client: client,
	}
}

// ResourceExplorerInput represents the input for resource_explorer tool
type ResourceExplorerInput struct {
	Kind         string `json:"kind,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
	Status       string `json:"status,omitempty"`    // Ready, Warning, Error, Terminating
	Time         int64  `json:"time,omitempty"`      // Snapshot at specific time, 0 = latest
	MaxResources int    `json:"max_resources,omitempty"` // Max resources to return, default 200, max 1000
}

// ResourceInfo represents a resource in the explorer output
type ResourceInfo struct {
	Kind                  string `json:"kind"`
	Namespace             string `json:"namespace"`
	Name                  string `json:"name"`
	CurrentStatus         string `json:"current_status"`
	IssueCount            int    `json:"issue_count"`
	ErrorCount            int    `json:"error_count"`
	WarningCount          int    `json:"warning_count"`
	EventCount            int    `json:"event_count"`
	LastStatusChange      int64  `json:"last_status_change"`
	LastStatusChangeText  string `json:"last_status_change_text,omitempty"` // Human-readable timestamp
}

// AvailableOptions represents available options for filtering
type AvailableOptions struct {
	Kinds      []string `json:"kinds"`
	Namespaces []string `json:"namespaces"`
	Statuses   []string `json:"statuses"`
	TimeRange  struct {
		Start     int64  `json:"start"`
		End       int64  `json:"end"`
		StartText string `json:"start_text,omitempty"` // Human-readable timestamp
		EndText   string `json:"end_text,omitempty"`   // Human-readable timestamp
	} `json:"time_range"`
}

// ResourceExplorerOutput represents the output of resource_explorer tool
type ResourceExplorerOutput struct {
	Resources        []ResourceInfo      `json:"resources"`
	AvailableOptions AvailableOptions    `json:"available_options"`
	ResourceCount    int                 `json:"resource_count"`
	ExplorationTimeMs int64              `json:"exploration_time_ms"`
}

// Execute runs the resource_explorer tool
func (t *ResourceExplorerTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ResourceExplorerInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	start := time.Now()

	// Get metadata for available options
	metadata, err := t.client.GetMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Set time range for query - use metadata if time not specified
	queryStart := params.Time
	queryEnd := params.Time

	if params.Time == 0 {
		// Use full range from metadata
		queryStart = metadata.TimeRange.Start
		queryEnd = metadata.TimeRange.End
	} else {
		// Use 1 hour window around the specified time
		queryStart = params.Time - 1800 // 30 min before
		queryEnd = params.Time + 1800   // 30 min after
	}

	// Query timeline
	filters := make(map[string]string)
	if params.Kind != "" {
		filters["kind"] = params.Kind
	}
	if params.Namespace != "" {
		filters["namespace"] = params.Namespace
	}

	response, err := t.client.QueryTimeline(queryStart, queryEnd, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to query timeline: %w", err)
	}

	// Apply default limit: 200 (default), max 1000
	maxResources := ApplyDefaultLimit(params.MaxResources, 200, 1000)

	output := t.buildExplorerOutput(response, metadata, params, maxResources)
	output.ExplorationTimeMs = time.Since(start).Milliseconds()

	return output, nil
}

func (t *ResourceExplorerTool) buildExplorerOutput(response *client.TimelineResponse, metadata *client.MetadataResponse, params ResourceExplorerInput, maxResources int) *ResourceExplorerOutput {
	output := &ResourceExplorerOutput{
		Resources:        make([]ResourceInfo, 0),
		AvailableOptions: AvailableOptions{},
	}

	output.AvailableOptions.Kinds = metadata.Kinds
	output.AvailableOptions.Namespaces = metadata.Namespaces
	output.AvailableOptions.Statuses = []string{"Ready", "Warning", "Error", "Terminating", "Unknown"}
	output.AvailableOptions.TimeRange.Start = metadata.TimeRange.Start
	output.AvailableOptions.TimeRange.End = metadata.TimeRange.End
	output.AvailableOptions.TimeRange.StartText = FormatTimestamp(metadata.TimeRange.Start)
	output.AvailableOptions.TimeRange.EndText = FormatTimestamp(metadata.TimeRange.End)

	// Process resources
	for _, resource := range response.Resources {
		info := ResourceInfo{
			Kind:      resource.Kind,
			Namespace: resource.Namespace,
			Name:      resource.Name,
			EventCount: len(resource.Events),
		}

		// Determine current status
		currentStatus := "Unknown"
		lastStatusChange := int64(0)

		if len(resource.StatusSegments) > 0 {
			lastSegment := resource.StatusSegments[len(resource.StatusSegments)-1]
			currentStatus = lastSegment.Status
			lastStatusChange = lastSegment.StartTime
		}

		info.CurrentStatus = currentStatus

		// Count issues
		for _, event := range resource.Events {
			if event.Type == "Warning" {
				info.WarningCount++
			}
		}

		// Count error segments
		for _, segment := range resource.StatusSegments {
			if segment.Status == "Error" {
				info.ErrorCount++
			}
		}

		info.IssueCount = info.ErrorCount + info.WarningCount
		info.LastStatusChange = lastStatusChange
		info.LastStatusChangeText = FormatTimestamp(lastStatusChange)

		// Apply status filter if specified
		if params.Status != "" && currentStatus != params.Status {
			continue
		}

		output.Resources = append(output.Resources, info)

		// Apply limit
		if len(output.Resources) >= maxResources {
			break
		}
	}

	// Sort by kind, namespace, name for consistent output
	sort.Slice(output.Resources, func(i, j int) bool {
		if output.Resources[i].Kind != output.Resources[j].Kind {
			return output.Resources[i].Kind < output.Resources[j].Kind
		}
		if output.Resources[i].Namespace != output.Resources[j].Namespace {
			return output.Resources[i].Namespace < output.Resources[j].Namespace
		}
		return output.Resources[i].Name < output.Resources[j].Name
	})

	output.ResourceCount = len(output.Resources)

	return output
}
