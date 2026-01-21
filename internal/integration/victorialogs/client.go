package victorialogs

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// Client is an HTTP client wrapper for VictoriaLogs API.
// It supports log queries, histogram aggregation, stats aggregation, and batch ingestion.
type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     *logging.Logger
}

// NewClient creates a new VictoriaLogs HTTP client with tuned connection pooling.
// baseURL: VictoriaLogs instance URL (e.g., "http://victorialogs:9428")
// queryTimeout: Maximum time for query execution (e.g., 30s)
func NewClient(baseURL string, queryTimeout time.Duration) *Client {
	// Create tuned HTTP transport for high-throughput queries
	transport := &http.Transport{
		// Connection pool settings
		MaxIdleConns:        100,                // Global connection pool
		MaxConnsPerHost:     20,                 // Per-host connection limit
		MaxIdleConnsPerHost: 10,                 // CRITICAL: default 2 causes connection churn
		IdleConnTimeout:     90 * time.Second,   // Keep-alive for idle connections
		TLSHandshakeTimeout: 10 * time.Second,

		// Dialer settings
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,  // Connection establishment timeout
			KeepAlive: 30 * time.Second, // TCP keep-alive interval
		}).DialContext,
	}

	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"), // Remove trailing slash
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   queryTimeout, // Overall request timeout
		},
		logger: logging.GetLogger("victorialogs.client"),
	}
}

// QueryLogs executes a log query and returns matching log entries.
// Uses /select/logsql/query endpoint with JSON line response format.
func (c *Client) QueryLogs(ctx context.Context, params QueryParams) (*QueryResponse, error) {
	// Build LogsQL query from structured parameters
	query := BuildLogsQLQuery(params)

	// Construct form-encoded request body
	form := url.Values{}
	form.Set("query", query)
	if params.Limit > 0 {
		form.Set("limit", strconv.Itoa(params.Limit))
	}

	// Build request URL
	reqURL := fmt.Sprintf("%s/select/logsql/query", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create query request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	defer resp.Body.Close()

	// CRITICAL: Always read response body to completion for connection reuse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("VictoriaLogs query failed: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("query failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse JSON line response
	return c.parseJSONLineResponse(body, params.Limit)
}

// QueryHistogram executes a histogram query and returns time-bucketed log counts.
// Uses /select/logsql/hits endpoint with step parameter for automatic bucketing.
func (c *Client) QueryHistogram(ctx context.Context, params QueryParams, step string) (*HistogramResponse, error) {
	// Build base query (hits endpoint handles bucketing)
	query := BuildHistogramQuery(params)

	// Use default time range if not specified
	timeRange := params.TimeRange
	if timeRange.IsZero() {
		timeRange = DefaultTimeRange()
	}

	// Construct form-encoded request body
	form := url.Values{}
	form.Set("query", query)
	form.Set("start", timeRange.Start.Format(time.RFC3339))
	form.Set("end", timeRange.End.Format(time.RFC3339))
	form.Set("step", step) // e.g., "5m", "1h"

	// Build request URL
	reqURL := fmt.Sprintf("%s/select/logsql/hits", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create histogram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute histogram query: %w", err)
	}
	defer resp.Body.Close()

	// CRITICAL: Always read response body to completion for connection reuse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("VictoriaLogs histogram query failed: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("histogram query failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var result HistogramResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse histogram response: %w", err)
	}

	return &result, nil
}

// QueryAggregation executes an aggregation query and returns grouped counts.
// Uses /select/logsql/stats_query endpoint with stats pipe for grouping.
func (c *Client) QueryAggregation(ctx context.Context, params QueryParams, groupBy []string) (*AggregationResponse, error) {
	// Build aggregation query with stats pipe
	query := BuildAggregationQuery(params, groupBy)

	// Use default time range if not specified
	timeRange := params.TimeRange
	if timeRange.IsZero() {
		timeRange = DefaultTimeRange()
	}

	// Construct form-encoded request body
	form := url.Values{}
	form.Set("query", query)
	form.Set("time", timeRange.End.Format(time.RFC3339))

	// Build request URL
	reqURL := fmt.Sprintf("%s/select/logsql/stats_query", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create aggregation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute aggregation query: %w", err)
	}
	defer resp.Body.Close()

	// CRITICAL: Always read response body to completion for connection reuse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("VictoriaLogs aggregation query failed: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("aggregation query failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var result AggregationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse aggregation response: %w", err)
	}

	return &result, nil
}

// IngestBatch sends a batch of log entries to VictoriaLogs for ingestion.
// Uses /insert/jsonline endpoint with JSON array payload.
func (c *Client) IngestBatch(ctx context.Context, entries []LogEntry) error {
	if len(entries) == 0 {
		return nil // Nothing to ingest
	}

	// Marshal entries as JSON array
	jsonData, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshal log entries: %w", err)
	}

	// Build request URL
	reqURL := fmt.Sprintf("%s/insert/jsonline", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL,
		bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("create ingestion request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute ingestion: %w", err)
	}
	defer resp.Body.Close()

	// CRITICAL: Always read response body to completion for connection reuse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("VictoriaLogs ingestion failed: status=%d body=%s", resp.StatusCode, string(body))
		return fmt.Errorf("ingestion failed (status %d): %s", resp.StatusCode, string(body))
	}

	c.logger.Debug("Ingested %d log entries to VictoriaLogs", len(entries))
	return nil
}

// parseJSONLineResponse parses VictoriaLogs JSON line response into QueryResponse.
// Each line is a separate JSON object representing a log entry.
func (c *Client) parseJSONLineResponse(body []byte, limit int) (*QueryResponse, error) {
	var entries []LogEntry
	scanner := bufio.NewScanner(bytes.NewReader(body))

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue // Skip empty lines
		}

		var entry LogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("parse log entry: %w (line: %s)", err, string(line))
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan response: %w", err)
	}

	// Determine if more results exist beyond the limit
	hasMore := limit > 0 && len(entries) >= limit

	return &QueryResponse{
		Logs:    entries,
		Count:   len(entries),
		HasMore: hasMore,
	}, nil
}
