// Package helpers provides API client utilities for e2e testing.
package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/moolen/spectre/internal/analysis"
	pb "github.com/moolen/spectre/internal/api/pb"
	"github.com/moolen/spectre/internal/api/pb/pbconnect"
)

type APIClient struct {
	BaseURL string
	Client  *http.Client
	t       *testing.T
}

// SearchResponse matches the /v1/search endpoint response.
type SearchResponse struct {
	Resources       []Resource `json:"resources"`
	Count           int        `json:"count"`
	ExecutionTimeMs int        `json:"executionTimeMs"`
}

// Resource represents a Kubernetes resource with audit events.
type Resource struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	APIVersion     string          `json:"apiVersion"`
	Namespace      string          `json:"namespace"`
	StatusSegments []StatusSegment `json:"statusSegments"`
	Events         []K8sEvent      `json:"events"`
}

// StatusSegment represents a period of consistent resource status.
type StatusSegment struct {
	Status    string                 `json:"status"`
	StartTime int64                  `json:"startTime"`
	EndTime   int64                  `json:"endTime"`
	Message   string                 `json:"message"`
	Config    map[string]interface{} `json:"config"`
}

// K8sEvent represents a Kubernetes Event (Kind=Event).
type K8sEvent struct {
	ID             string `json:"id"`
	Timestamp      int64  `json:"timestamp"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	Type           string `json:"type"`
	Count          int32  `json:"count"`
	Source         string `json:"source,omitempty"`
	FirstTimestamp int64  `json:"firstTimestamp,omitempty"`
	LastTimestamp  int64  `json:"lastTimestamp,omitempty"`
}

// MetadataResponse matches the /v1/metadata endpoint response.
type MetadataResponse struct {
	Namespaces []string  `json:"namespaces"`
	Kinds      []string  `json:"kinds"`
	TimeRange  TimeRange `json:"timeRange"`
}

// TimeRange represents earliest and latest timestamps.
type TimeRange struct {
	Earliest int64 `json:"earliest"`
	Latest   int64 `json:"latest"`
}

// SegmentsResponse represents the /v1/resources/{id}/segments response.
type SegmentsResponse struct {
	Segments   []StatusSegment `json:"segments"`
	ResourceID string          `json:"resourceId"`
	Count      int             `json:"count"`
}

// EventsResponse represents the /v1/resources/{id}/events response.
type EventsResponse struct {
	Events     []K8sEvent `json:"events"`
	Count      int        `json:"count"`
	ResourceID string     `json:"resourceId"`
}

// NewAPIClient creates a new API client.
// baseURL is the HTTP REST API endpoint (supports HTTP, Connect, gRPC, and gRPC-Web)
func NewAPIClient(t *testing.T, baseURL string) *APIClient {
	t.Logf("Creating API client for: %s (supports HTTP, Connect, gRPC, and gRPC-Web)", baseURL)

	return &APIClient{
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: 10 * time.Second},
		t:       t,
	}
}

// Search queries the /v1/search endpoint.
func (a *APIClient) Search(ctx context.Context, startTime, endTime int64, namespace, kind string) (*SearchResponse, error) {
	url := fmt.Sprintf("%s/v1/search?start=%d&end=%d", a.BaseURL, startTime, endTime)

	if namespace != "" {
		url += fmt.Sprintf("&namespace=%s", namespace)
	}
	if kind != "" {
		url += fmt.Sprintf("&kind=%s", kind)
	}

	resp, err := a.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return &result, nil
}

// GetMetadata queries the /v1/metadata endpoint.
func (a *APIClient) GetMetadata(ctx context.Context, startTime, endTime *int64) (*MetadataResponse, error) {
	url := a.BaseURL + "/v1/metadata"

	if startTime != nil && endTime != nil {
		url += fmt.Sprintf("?start=%d&end=%d", *startTime, *endTime)
	}

	resp, err := a.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result MetadataResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode metadata response: %w", err)
	}

	return &result, nil
}

// GetResource queries the /v1/resources/{id} endpoint.
func (a *APIClient) GetResource(ctx context.Context, resourceID string) (*Resource, error) {
	url := fmt.Sprintf("%s/v1/resources/%s", a.BaseURL, resourceID)

	resp, err := a.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result Resource
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode resource response: %w", err)
	}

	return &result, nil
}

// GetSegments queries the /v1/resources/{id}/segments endpoint.
func (a *APIClient) GetSegments(ctx context.Context, resourceID string, startTime, endTime *int64) (*SegmentsResponse, error) {
	url := fmt.Sprintf("%s/v1/resources/%s/segments", a.BaseURL, resourceID)

	if startTime != nil && endTime != nil {
		url += fmt.Sprintf("?start=%d&end=%d", *startTime, *endTime)
	}

	resp, err := a.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SegmentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode segments response: %w", err)
	}

	return &result, nil
}

// GetEvents queries the /v1/resources/{id}/events endpoint.
func (a *APIClient) GetEvents(ctx context.Context, resourceID string, startTime, endTime *int64, limit *int) (*EventsResponse, error) {
	url := fmt.Sprintf("%s/v1/resources/%s/events", a.BaseURL, resourceID)

	first := true
	if startTime != nil && endTime != nil {
		url += fmt.Sprintf("?start=%d&end=%d", *startTime, *endTime)
		first = false
	}

	if limit != nil {
		if first {
			url += "?"
		} else {
			url += "&"
		}
		url += fmt.Sprintf("limit=%d", *limit)
	}

	resp, err := a.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result EventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode events response: %w", err)
	}

	return &result, nil
}

// doRequest executes an HTTP request with context support.
func (a *APIClient) doRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// Timeline queries the /v1/timeline endpoint.
// This uses the same REST API as Search but with timeline-specific logic.
func (a *APIClient) Timeline(ctx context.Context, startTime, endTime int64, namespace, kind string) (*SearchResponse, error) {
	url := fmt.Sprintf("%s/v1/timeline?start=%d&end=%d", a.BaseURL, startTime, endTime)

	if namespace != "" {
		url += fmt.Sprintf("&namespace=%s", namespace)
	}
	if kind != "" {
		url += fmt.Sprintf("&kind=%s", kind)
	}

	resp, err := a.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode timeline response: %w", err)
	}

	return &result, nil
}

// Health checks if the API is healthy.
func (a *APIClient) Health(ctx context.Context) error {
	url := a.BaseURL + "/health"
	resp, err := a.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// RootCause queries the /v1/causal-graph endpoint.
func (a *APIClient) RootCause(ctx context.Context, resourceUID string, failureTimestamp int64, lookback time.Duration, maxDepth int, minConfidence float64) (*analysis.RootCauseAnalysisV2, error) {
	baseURL := fmt.Sprintf("%s/v1/causal-graph", a.BaseURL)
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	q := u.Query()
	q.Set("resourceUID", resourceUID)
	q.Set("failureTimestamp", fmt.Sprintf("%d", failureTimestamp))

	if lookback > 0 {
		q.Set("lookback", lookback.String())
	}
	if maxDepth > 0 {
		q.Set("maxDepth", fmt.Sprintf("%d", maxDepth))
	}
	if minConfidence > 0 {
		q.Set("minConfidence", fmt.Sprintf("%.2f", minConfidence))
	}

	u.RawQuery = q.Encode()
	urlStr := u.String()

	resp, err := a.doRequest(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read the full response body for better error reporting
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result analysis.RootCauseAnalysisV2
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		// Include first 500 chars of response in error for debugging
		preview := string(bodyBytes)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		return nil, fmt.Errorf("failed to decode root cause response: %w. Response body: %s", err, preview)
	}

	return &result, nil
}

// TimelineGRPC queries the timeline using the Connect streaming API.
// Supports Connect, gRPC, and gRPC-Web protocols on the same HTTP port.
func (a *APIClient) TimelineGRPC(ctx context.Context, startTime, endTime int64, namespace, kind string) (*SearchResponse, error) {
	// Create Connect Timeline service client
	// Using connect.WithGRPCWeb() to use gRPC-Web protocol over HTTP
	client := pbconnect.NewTimelineServiceClient(
		a.Client,
		a.BaseURL,
		connect.WithGRPCWeb(), // Use gRPC-Web protocol over HTTP
	)

	// Prepare request
	req := connect.NewRequest(&pb.TimelineRequest{
		StartTimestamp: startTime,
		EndTimestamp:   endTime,
		Namespace:      namespace,
		Kind:           kind,
	})

	// Call the streaming endpoint
	stream, err := client.GetTimeline(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to call GetTimeline: %w", err)
	}

	// Collect results
	var resources []Resource
	var executionTimeMs int64
	var totalCount int

	for stream.Receive() {
		chunk := stream.Msg()

		// Check chunk type
		if metadata := chunk.GetMetadata(); metadata != nil {
			// First chunk contains metadata
			totalCount = int(metadata.TotalCount)
			executionTimeMs = metadata.QueryExecutionTimeMs
			a.t.Logf("Timeline metadata: count=%d, execution_time=%dms", totalCount, executionTimeMs)
		} else if batch := chunk.GetBatch(); batch != nil {
			// Subsequent chunks contain resource batches
			a.t.Logf("Received batch of %d resources (kind=%s)", len(batch.Resources), batch.Kind)

			// Convert protobuf resources to API response format
			for _, pbResource := range batch.Resources {
				resource := convertTimelineResource(pbResource)
				resources = append(resources, resource)
			}
		}
	}

	// Check for errors after stream completes
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("stream error: %w", err)
	}

	return &SearchResponse{
		Resources:       resources,
		Count:           totalCount,
		ExecutionTimeMs: int(executionTimeMs),
	}, nil
}

// convertTimelineResource converts a protobuf TimelineResource to the API Resource format
func convertTimelineResource(pbResource *pb.TimelineResource) Resource {
	resource := Resource{
		ID:             pbResource.Id,
		Name:           pbResource.Name,
		Kind:           pbResource.Kind,
		APIVersion:     pbResource.ApiVersion,
		Namespace:      pbResource.Namespace,
		StatusSegments: []StatusSegment{},
		Events:         []K8sEvent{},
	}

	// Convert status segments
	for _, pbSegment := range pbResource.StatusSegments {
		segment := StatusSegment{
			Status:    pbSegment.Status,
			StartTime: pbSegment.StartTime,
			EndTime:   pbSegment.EndTime,
			Message:   pbSegment.Message,
		}
		resource.StatusSegments = append(resource.StatusSegments, segment)
	}

	// Convert events
	for _, pbEvent := range pbResource.Events {
		event := K8sEvent{
			ID:        pbEvent.Uid,
			Timestamp: pbEvent.Timestamp,
			Reason:    pbEvent.Reason,
			Message:   pbEvent.Message,
			Type:      pbEvent.Type,
		}
		resource.Events = append(resource.Events, event)
	}

	return resource
}

// Close cleans up API client resources.
func (a *APIClient) Close() error {
	// No cleanup needed for ConnectRPC client (uses standard http.Client)
	return nil
}
