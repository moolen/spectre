package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Logger interface for retry logging (avoids circular imports with logging package)
type Logger interface {
	Info(msg string, args ...interface{})
}

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
// pageSize of 0 or negative uses the default (100), use a large value like 10000 for unlimited
func (c *SpectreClient) QueryTimeline(startTime, endTime int64, filters map[string]string, pageSize int) (*TimelineResponse, error) {
	q := url.Values{}
	q.Set("start", fmt.Sprintf("%d", startTime))
	q.Set("end", fmt.Sprintf("%d", endTime))

	for k, v := range filters {
		if v != "" {
			q.Set(k, v)
		}
	}

	// Add page_size parameter if specified
	if pageSize > 0 {
		q.Set("page_size", fmt.Sprintf("%d", pageSize))
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

// PingWithRetry pings the Spectre API with exponential backoff retry logic.
// This is useful when starting up alongside the Spectre server container.
// Uses hardcoded defaults: 20 retries, 500ms initial backoff, 10s max backoff.
func (c *SpectreClient) PingWithRetry(logger Logger) error {
	const maxRetries = 20
	const maxBackoff = 10 * time.Second
	initialBackoff := 500 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff calculation - attempt is bounded by maxRetries (20)
			// #nosec G115 -- attempt-1 is bounded by maxRetries and will never overflow
			backoff := initialBackoff * time.Duration(1<<uint(attempt-1))
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			if logger != nil {
				logger.Info("Retrying connection to Spectre API in %v (attempt %d/%d)", backoff, attempt+1, maxRetries)
			}
			time.Sleep(backoff)
		}

		if err := c.Ping(); err != nil {
			lastErr = err
			if attempt == 0 && logger != nil {
				logger.Info("Initial connection to Spectre API failed (server may still be starting): %v", err)
			}
			continue
		}

		// Connection successful
		return nil
	}

	return fmt.Errorf("failed to connect to Spectre API after %d attempts: %w", maxRetries, lastErr)
}

// DetectAnomalies queries the anomalies API to detect anomalies in a resource's causal subgraph
func (c *SpectreClient) DetectAnomalies(resourceUID string, start, end int64) (*AnomalyResponse, error) {
	q := url.Values{}
	q.Set("resourceUID", resourceUID)
	q.Set("start", fmt.Sprintf("%d", start))
	q.Set("end", fmt.Sprintf("%d", end))

	reqURL := fmt.Sprintf("%s/v1/anomalies?%s", c.baseURL, q.Encode())
	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to query anomalies: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anomalies API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result AnomalyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode anomalies response: %w", err)
	}

	return &result, nil
}

// QueryCausalPaths queries the causal paths API
func (c *SpectreClient) QueryCausalPaths(resourceUID string, failureTimestamp int64, lookbackMinutes, maxDepth, maxPaths int) (*CausalPathsResponse, error) {
	q := url.Values{}
	q.Set("resourceUID", resourceUID)
	q.Set("failureTimestamp", fmt.Sprintf("%d", failureTimestamp))

	// Convert lookback minutes to duration string (e.g., "10m")
	if lookbackMinutes > 0 {
		q.Set("lookback", fmt.Sprintf("%dm", lookbackMinutes))
	}
	if maxDepth > 0 {
		q.Set("maxDepth", fmt.Sprintf("%d", maxDepth))
	}
	if maxPaths > 0 {
		q.Set("maxPaths", fmt.Sprintf("%d", maxPaths))
	}

	reqURL := fmt.Sprintf("%s/v1/causal-paths?%s", c.baseURL, q.Encode())
	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to query causal paths: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("causal paths API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result CausalPathsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode causal paths response: %w", err)
	}

	return &result, nil
}
