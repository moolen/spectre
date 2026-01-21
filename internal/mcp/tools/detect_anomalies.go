package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/analysis/anomaly"
	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/mcp/client"
)

// DetectAnomaliesTool implements the detect_anomalies MCP tool
type DetectAnomaliesTool struct {
	graphService    *api.GraphService
	timelineService *api.TimelineService
	client          *client.SpectreClient
}

// NewDetectAnomaliesTool creates a new detect anomalies tool with direct services
func NewDetectAnomaliesTool(graphService *api.GraphService, timelineService *api.TimelineService) *DetectAnomaliesTool {
	return &DetectAnomaliesTool{
		graphService:    graphService,
		timelineService: timelineService,
		client:          nil,
	}
}

// NewDetectAnomaliesToolWithClient creates a new detect anomalies tool with HTTP client (backward compatibility)
func NewDetectAnomaliesToolWithClient(client *client.SpectreClient) *DetectAnomaliesTool {
	return &DetectAnomaliesTool{
		graphService:    nil,
		timelineService: nil,
		client:          client,
	}
}

// DetectAnomaliesInput represents the input for detect_anomalies tool (MCP-style naming)
// Either resource_uid OR (namespace + kind) must be provided
type DetectAnomaliesInput struct {
	ResourceUID string `json:"resource_uid,omitempty"` // UID of the resource to analyze (alternative to namespace+kind)
	Namespace   string `json:"namespace,omitempty"`    // Namespace to filter by (use with kind as alternative to resource_uid)
	Kind        string `json:"kind,omitempty"`         // Resource kind to filter by (use with namespace as alternative to resource_uid)
	StartTime   int64  `json:"start_time"`             // Required: Unix timestamp (seconds or milliseconds)
	EndTime     int64  `json:"end_time"`               // Required: Unix timestamp (seconds or milliseconds)
	MaxResults  int    `json:"max_results,omitempty"`  // Max resources to analyze when using namespace/kind filter (default: 10, max: 50)
}

// DetectAnomaliesOutput represents the LLM-optimized output of detect_anomalies tool
type DetectAnomaliesOutput struct {
	Anomalies           []AnomalySummary   `json:"anomalies"`
	AnomalyCount        int                `json:"anomaly_count"`
	AnomaliesBySeverity map[string]int     `json:"anomalies_by_severity"` // e.g., {"critical":2,"high":5}
	AnomaliesByCategory map[string]int     `json:"anomalies_by_category"` // e.g., {"State":3,"Event":4}
	Metadata            AnomalyMetadataOut `json:"metadata"`
}

// AnomalySummary represents a single anomaly with human-readable timestamps
type AnomalySummary struct {
	Node          AnomalyNodeInfo        `json:"node"`
	Category      string                 `json:"category"`
	Type          string                 `json:"type"`
	Severity      string                 `json:"severity"`
	Timestamp     int64                  `json:"timestamp"`
	TimestampText string                 `json:"timestamp_text"` // Human-readable timestamp
	Summary       string                 `json:"summary"`
	Details       map[string]interface{} `json:"details,omitempty"`
}

// AnomalyNodeInfo identifies the resource exhibiting the anomaly
type AnomalyNodeInfo struct {
	UID       string `json:"uid"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// AnomalyMetadataOut provides context about the analysis
type AnomalyMetadataOut struct {
	ResourceUID      string   `json:"resource_uid,omitempty"`
	ResourceUIDs     []string `json:"resource_uids,omitempty"` // When using namespace/kind filter
	Namespace        string   `json:"namespace,omitempty"`
	Kind             string   `json:"kind,omitempty"`
	StartTime        int64    `json:"start_time"`
	EndTime          int64    `json:"end_time"`
	StartTimeText    string   `json:"start_time_text"` // Human-readable timestamp
	EndTimeText      string   `json:"end_time_text"`   // Human-readable timestamp
	NodesAnalyzed    int      `json:"nodes_analyzed"`
	ResourcesQueried int      `json:"resources_queried,omitempty"` // When using namespace/kind filter
	ExecutionTimeMs  int64    `json:"execution_time_ms"`
}

// Execute runs the detect_anomalies tool
func (t *DetectAnomaliesTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params DetectAnomaliesInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Validate input: either resource_uid OR (namespace + kind) must be provided
	hasResourceUID := params.ResourceUID != ""
	hasNamespaceKind := params.Namespace != "" && params.Kind != ""

	if !hasResourceUID && !hasNamespaceKind {
		return nil, fmt.Errorf("either resource_uid OR both namespace and kind must be provided")
	}

	if params.StartTime == 0 {
		return nil, fmt.Errorf("start_time is required")
	}
	if params.EndTime == 0 {
		return nil, fmt.Errorf("end_time is required")
	}

	// Convert milliseconds to seconds if needed
	startTime := params.StartTime
	endTime := params.EndTime
	if startTime > 10000000000 {
		startTime /= 1000
	}
	if endTime > 10000000000 {
		endTime /= 1000
	}

	// Validate time range
	if startTime >= endTime {
		return nil, fmt.Errorf("start_time must be before end_time")
	}

	// If resource_uid is provided, use direct lookup
	if hasResourceUID {
		return t.executeByUID(ctx, params.ResourceUID, startTime, endTime)
	}

	// Otherwise, use namespace/kind filter to discover resources first
	return t.executeByNamespaceKind(ctx, params.Namespace, params.Kind, startTime, endTime, params.MaxResults)
}

// executeByUID performs anomaly detection for a single resource by UID
func (t *DetectAnomaliesTool) executeByUID(ctx context.Context, resourceUID string, startTime, endTime int64) (*DetectAnomaliesOutput, error) {
	// Use GraphService if available (direct service call), otherwise HTTP client
	if t.graphService != nil {
		// Direct service call
		input := anomaly.DetectInput{
			ResourceUID: resourceUID,
			Start:       startTime,
			End:         endTime,
		}
		result, err := t.graphService.DetectAnomalies(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to detect anomalies: %w", err)
		}

		// Transform to MCP output format
		output := t.transformAnomalyResponse(result, startTime, endTime)
		output.Metadata.ResourceUID = resourceUID
		return output, nil
	}

	// Fallback to HTTP client
	response, err := t.client.DetectAnomalies(resourceUID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to detect anomalies: %w", err)
	}

	output := t.transformResponse(response, startTime, endTime)
	output.Metadata.ResourceUID = resourceUID
	return output, nil
}

// executeByNamespaceKind discovers resources by namespace/kind and runs anomaly detection on each
func (t *DetectAnomaliesTool) executeByNamespaceKind(ctx context.Context, namespace, kind string, startTime, endTime int64, maxResults int) (*DetectAnomaliesOutput, error) {
	// Apply default limit: 10 (default), max 50
	if maxResults <= 0 {
		maxResults = 10
	}
	if maxResults > 50 {
		maxResults = 50
	}

	// Query timeline to discover resources in the namespace/kind
	// Use TimelineService via HTTP client (timeline service integration is more complex, defer to future iteration)
	filters := map[string]string{
		"namespace": namespace,
		"kind":      kind,
	}

	var resources []interface{ GetID() string }
	// For now, always use HTTP client for timeline queries in detect_anomalies
	// TODO: Integrate TimelineService properly in future iteration
	timelineResponse, err := t.client.QueryTimeline(startTime, endTime, filters, 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to query timeline for resource discovery: %w", err)
	}
	for _, r := range timelineResponse.Resources {
		resources = append(resources, &resourceWithID{id: r.ID})
	}

	if len(resources) == 0 {
		return &DetectAnomaliesOutput{
			Anomalies:           make([]AnomalySummary, 0),
			AnomalyCount:        0,
			AnomaliesBySeverity: make(map[string]int),
			AnomaliesByCategory: make(map[string]int),
			Metadata: AnomalyMetadataOut{
				Namespace:        namespace,
				Kind:             kind,
				StartTime:        startTime,
				EndTime:          endTime,
				StartTimeText:    FormatTimestamp(startTime),
				EndTimeText:      FormatTimestamp(endTime),
				NodesAnalyzed:    0,
				ResourcesQueried: 0,
			},
		}, nil
	}

	// Limit the number of resources to analyze
	if len(resources) > maxResults {
		resources = resources[:maxResults]
	}

	// Aggregate results from all resources
	aggregatedOutput := &DetectAnomaliesOutput{
		Anomalies:           make([]AnomalySummary, 0),
		AnomalyCount:        0,
		AnomaliesBySeverity: make(map[string]int),
		AnomaliesByCategory: make(map[string]int),
		Metadata: AnomalyMetadataOut{
			Namespace:        namespace,
			Kind:             kind,
			ResourceUIDs:     make([]string, 0, len(resources)),
			StartTime:        startTime,
			EndTime:          endTime,
			StartTimeText:    FormatTimestamp(startTime),
			EndTimeText:      FormatTimestamp(endTime),
			NodesAnalyzed:    0,
			ResourcesQueried: len(resources),
		},
	}

	// Run anomaly detection for each discovered resource
	for _, resource := range resources {
		resourceID := resource.GetID()
		aggregatedOutput.Metadata.ResourceUIDs = append(aggregatedOutput.Metadata.ResourceUIDs, resourceID)

		// Use GraphService if available
		if t.graphService != nil {
			input := anomaly.DetectInput{
				ResourceUID: resourceID,
				Start:       startTime,
				End:         endTime,
			}
			result, err := t.graphService.DetectAnomalies(ctx, input)
			if err != nil {
				// Log error but continue with other resources
				continue
			}

			// Merge results
			singleOutput := t.transformAnomalyResponse(result, startTime, endTime)
			aggregatedOutput.Anomalies = append(aggregatedOutput.Anomalies, singleOutput.Anomalies...)
			aggregatedOutput.AnomalyCount += singleOutput.AnomalyCount
			aggregatedOutput.Metadata.NodesAnalyzed += singleOutput.Metadata.NodesAnalyzed

			// Merge severity counts
			for severity, count := range singleOutput.AnomaliesBySeverity {
				aggregatedOutput.AnomaliesBySeverity[severity] += count
			}

			// Merge category counts
			for category, count := range singleOutput.AnomaliesByCategory {
				aggregatedOutput.AnomaliesByCategory[category] += count
			}
		} else {
			// HTTP client fallback
			response, err := t.client.DetectAnomalies(resourceID, startTime, endTime)
			if err != nil {
				// Log error but continue with other resources
				continue
			}

			// Merge results
			singleOutput := t.transformResponse(response, startTime, endTime)
			aggregatedOutput.Anomalies = append(aggregatedOutput.Anomalies, singleOutput.Anomalies...)
			aggregatedOutput.AnomalyCount += singleOutput.AnomalyCount
			aggregatedOutput.Metadata.NodesAnalyzed += singleOutput.Metadata.NodesAnalyzed

			// Merge severity counts
			for severity, count := range singleOutput.AnomaliesBySeverity {
				aggregatedOutput.AnomaliesBySeverity[severity] += count
			}

			// Merge category counts
			for category, count := range singleOutput.AnomaliesByCategory {
				aggregatedOutput.AnomaliesByCategory[category] += count
			}
		}
	}

	return aggregatedOutput, nil
}

// resourceWithID is a helper type to unify resource ID access
type resourceWithID struct {
	id string
}

func (r *resourceWithID) GetID() string {
	return r.id
}

// transformAnomalyResponse transforms anomaly.AnomalyResponse to MCP output format
func (t *DetectAnomaliesTool) transformAnomalyResponse(response *anomaly.AnomalyResponse, startTime, endTime int64) *DetectAnomaliesOutput {
	output := &DetectAnomaliesOutput{
		Anomalies:           make([]AnomalySummary, 0, len(response.Anomalies)),
		AnomalyCount:        len(response.Anomalies),
		AnomaliesBySeverity: make(map[string]int),
		AnomaliesByCategory: make(map[string]int),
		Metadata: AnomalyMetadataOut{
			ResourceUID:     response.Metadata.ResourceUID,
			StartTime:       startTime,
			EndTime:         endTime,
			StartTimeText:   FormatTimestamp(startTime),
			EndTimeText:     FormatTimestamp(endTime),
			NodesAnalyzed:   response.Metadata.NodesAnalyzed,
			ExecutionTimeMs: response.Metadata.ExecutionTimeMs,
		},
	}

	// Transform each anomaly
	for _, a := range response.Anomalies {
		timestamp := a.Timestamp.Unix()
		timestampText := FormatTimestamp(timestamp)

		summary := AnomalySummary{
			Node: AnomalyNodeInfo{
				UID:       a.Node.UID,
				Kind:      a.Node.Kind,
				Namespace: a.Node.Namespace,
				Name:      a.Node.Name,
			},
			Category:      string(a.Category),
			Type:          a.Type,
			Severity:      string(a.Severity),
			Timestamp:     timestamp,
			TimestampText: timestampText,
			Summary:       a.Summary,
			Details:       a.Details,
		}
		output.Anomalies = append(output.Anomalies, summary)

		// Count by severity
		output.AnomaliesBySeverity[string(a.Severity)]++

		// Count by category
		output.AnomaliesByCategory[string(a.Category)]++
	}

	return output
}

// transformResponse converts the HTTP API response to LLM-optimized output
func (t *DetectAnomaliesTool) transformResponse(response *client.AnomalyResponse, startTime, endTime int64) *DetectAnomaliesOutput {
	output := &DetectAnomaliesOutput{
		Anomalies:           make([]AnomalySummary, 0, len(response.Anomalies)),
		AnomalyCount:        len(response.Anomalies),
		AnomaliesBySeverity: make(map[string]int),
		AnomaliesByCategory: make(map[string]int),
		Metadata: AnomalyMetadataOut{
			ResourceUID:     response.Metadata.ResourceUID,
			StartTime:       startTime,
			EndTime:         endTime,
			StartTimeText:   FormatTimestamp(startTime),
			EndTimeText:     FormatTimestamp(endTime),
			NodesAnalyzed:   response.Metadata.NodesAnalyzed,
			ExecutionTimeMs: response.Metadata.ExecTimeMs,
		},
	}

	// Transform each anomaly
	for _, a := range response.Anomalies {
		// Parse the timestamp from RFC3339 format
		ts, err := time.Parse(time.RFC3339, a.Timestamp)
		var timestamp int64
		var timestampText string
		if err == nil {
			timestamp = ts.Unix()
			timestampText = FormatTimestamp(timestamp)
		} else {
			// Fallback if parsing fails
			timestampText = a.Timestamp
		}

		summary := AnomalySummary{
			Node: AnomalyNodeInfo{
				UID:       a.Node.UID,
				Kind:      a.Node.Kind,
				Namespace: a.Node.Namespace,
				Name:      a.Node.Name,
			},
			Category:      string(a.Category),
			Type:          a.Type,
			Severity:      string(a.Severity),
			Timestamp:     timestamp,
			TimestampText: timestampText,
			Summary:       a.Summary,
			Details:       a.Details,
		}
		output.Anomalies = append(output.Anomalies, summary)

		// Count by severity
		output.AnomaliesBySeverity[string(a.Severity)]++

		// Count by category
		output.AnomaliesByCategory[string(a.Category)]++
	}

	return output
}
