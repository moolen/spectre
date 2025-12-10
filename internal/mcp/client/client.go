package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// SpectreClient handles communication with the Spectre API
type SpectreClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewSpectreClient creates a new Spectre API client
func NewSpectreClient(baseURL string) *SpectreClient {
	return &SpectreClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// QueryTimeline queries the timeline API
func (c *SpectreClient) QueryTimeline(startTime, endTime int64, filters map[string]string) (*TimelineResponse, error) {
	q := url.Values{}
	q.Set("start", fmt.Sprintf("%d", startTime))
	q.Set("end", fmt.Sprintf("%d", endTime))

	for k, v := range filters {
		if v != "" {
			q.Set(k, v)
		}
	}

	url := fmt.Sprintf("%s/v1/timeline?%s", c.baseURL, q.Encode())
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to query timeline: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("timeline API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result TimelineResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode timeline response: %w", err)
	}

	return &result, nil
}

// GetMetadata queries cluster metadata
func (c *SpectreClient) GetMetadata() (*MetadataResponse, error) {
	url := fmt.Sprintf("%s/v1/metadata", c.baseURL)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to query metadata: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("metadata API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result MetadataResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode metadata response: %w", err)
	}

	return &result, nil
}

// Ping checks if the Spectre API is reachable
func (c *SpectreClient) Ping() error {
	url := fmt.Sprintf("%s/health", c.baseURL)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("spectre API unreachable at %s: %w", c.baseURL, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("spectre API health check failed with status %d", resp.StatusCode)
	}

	return nil
}
