// Package helpers provides API client utilities for e2e testing.
package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
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
	Namespaces     []string       `json:"namespaces"`
	Kinds          []string       `json:"kinds"`
	Groups         []string       `json:"groups"`
	ResourceCounts map[string]int `json:"resourceCounts"`
	TotalEvents    int            `json:"totalEvents"`
	TimeRange      TimeRange      `json:"timeRange"`
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
func NewAPIClient(t *testing.T, baseURL string) *APIClient {
	t.Logf("Creating API client for: %s", baseURL)

	return &APIClient{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
		t: t,
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
