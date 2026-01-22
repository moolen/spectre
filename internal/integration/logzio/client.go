package logzio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/integration/victorialogs"
	"github.com/moolen/spectre/internal/logging"
)

// Client is an HTTP client wrapper for Logz.io API.
// It supports log queries and aggregation queries using Elasticsearch DSL.
type Client struct {
	baseURL       string
	httpClient    *http.Client
	secretWatcher *victorialogs.SecretWatcher // Optional: for dynamic token fetch
	logger        *logging.Logger
}

// NewClient creates a new Logz.io HTTP client.
// baseURL: Logz.io regional endpoint (e.g., "https://api.logz.io")
// httpClient: Configured HTTP client with timeout
// secretWatcher: Optional SecretWatcher for dynamic token authentication (may be nil)
// logger: Logger for observability
func NewClient(baseURL string, httpClient *http.Client, secretWatcher *victorialogs.SecretWatcher, logger *logging.Logger) *Client {
	return &Client{
		baseURL:       strings.TrimSuffix(baseURL, "/"), // Remove trailing slash
		httpClient:    httpClient,
		secretWatcher: secretWatcher,
		logger:        logger,
	}
}

// QueryLogs executes a log query and returns matching log entries.
// Uses /v1/search endpoint with Elasticsearch DSL.
func (c *Client) QueryLogs(ctx context.Context, params QueryParams) (*QueryResponse, error) {
	// Build Elasticsearch DSL query
	query := BuildLogsQuery(params)

	// Marshal to JSON
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	// Build request URL
	reqURL := fmt.Sprintf("%s/v1/search", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(string(queryJSON)))
	if err != nil {
		return nil, fmt.Errorf("create query request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Add authentication header if using secret watcher
	if c.secretWatcher != nil {
		token, err := c.secretWatcher.GetToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get API token: %w", err)
		}
		// CRITICAL: Logz.io uses X-API-TOKEN header (not Authorization: Bearer)
		req.Header.Set("X-API-TOKEN", token)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		c.logger.Error("Logz.io authentication failed: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("authentication failed (status %d): check API token", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		c.logger.Error("Logz.io rate limit exceeded: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("rate limit exceeded (status 429): please retry later")
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Logz.io query failed: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("query failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse Elasticsearch response
	var esResp elasticsearchResponse
	if err := json.Unmarshal(body, &esResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// Normalize hits to LogEntry
	entries := make([]LogEntry, 0, len(esResp.Hits.Hits))
	for _, hit := range esResp.Hits.Hits {
		entry := parseLogzioHit(hit)
		entries = append(entries, entry)
	}

	return &QueryResponse{
		Logs: entries,
	}, nil
}

// QueryAggregation executes an aggregation query and returns grouped counts.
// Uses /v1/search endpoint with Elasticsearch aggregations.
func (c *Client) QueryAggregation(ctx context.Context, params QueryParams, groupByFields []string) (*AggregationResponse, error) {
	// Build Elasticsearch DSL aggregation query
	query := BuildAggregationQuery(params, groupByFields)

	// Marshal to JSON
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	// Build request URL
	reqURL := fmt.Sprintf("%s/v1/search", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(string(queryJSON)))
	if err != nil {
		return nil, fmt.Errorf("create aggregation request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Add authentication header if using secret watcher
	if c.secretWatcher != nil {
		token, err := c.secretWatcher.GetToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get API token: %w", err)
		}
		// CRITICAL: Logz.io uses X-API-TOKEN header (not Authorization: Bearer)
		req.Header.Set("X-API-TOKEN", token)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute aggregation query: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		c.logger.Error("Logz.io authentication failed: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("authentication failed (status %d): check API token", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		c.logger.Error("Logz.io rate limit exceeded: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("rate limit exceeded (status 429): please retry later")
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Logz.io aggregation query failed: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("aggregation query failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse Elasticsearch aggregation response
	var esResp elasticsearchAggResponse
	if err := json.Unmarshal(body, &esResp); err != nil {
		return nil, fmt.Errorf("parse aggregation response: %w", err)
	}

	// Convert buckets to AggregationGroup
	groups := make([]AggregationGroup, 0)
	if len(groupByFields) > 0 {
		// Extract buckets from the aggregation (uses first groupByField as aggregation name)
		aggName := groupByFields[0]
		if agg, ok := esResp.Aggregations[aggName]; ok {
			for _, bucket := range agg.Buckets {
				groups = append(groups, AggregationGroup{
					Value: bucket.Key,
					Count: bucket.DocCount,
				})
			}
		}
	}

	return &AggregationResponse{
		Groups: groups,
	}, nil
}

// parseLogzioHit extracts a LogEntry from an Elasticsearch hit
func parseLogzioHit(hit elasticsearchHit) LogEntry {
	source := hit.Source

	// Extract timestamp
	var timestamp time.Time
	if tsStr, ok := source["@timestamp"].(string); ok {
		timestamp, _ = time.Parse(time.RFC3339, tsStr)
	}

	// Extract fields - map Logz.io field names to common schema
	// Note: Field extraction uses base field names (no .keyword suffix)
	entry := LogEntry{
		Time: timestamp,
	}

	if msg, ok := source["message"].(string); ok {
		entry.Message = msg
	}

	if ns, ok := source["kubernetes.namespace"].(string); ok {
		entry.Namespace = ns
	} else if ns, ok := source["kubernetes_namespace"].(string); ok {
		entry.Namespace = ns
	}

	if pod, ok := source["kubernetes.pod_name"].(string); ok {
		entry.Pod = pod
	} else if pod, ok := source["kubernetes_pod_name"].(string); ok {
		entry.Pod = pod
	}

	if container, ok := source["kubernetes.container_name"].(string); ok {
		entry.Container = container
	} else if container, ok := source["kubernetes_container_name"].(string); ok {
		entry.Container = container
	}

	if level, ok := source["level"].(string); ok {
		entry.Level = level
	}

	return entry
}

// Elasticsearch response structures

type elasticsearchResponse struct {
	Hits struct {
		Hits []elasticsearchHit `json:"hits"`
	} `json:"hits"`
}

type elasticsearchHit struct {
	Source map[string]interface{} `json:"_source"`
}

type elasticsearchAggResponse struct {
	Aggregations map[string]struct {
		Buckets []struct {
			Key      string `json:"key"`
			DocCount int    `json:"doc_count"`
		} `json:"buckets"`
	} `json:"aggregations"`
}
