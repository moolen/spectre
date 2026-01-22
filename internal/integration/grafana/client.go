package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// GrafanaClient is an HTTP client wrapper for Grafana API.
// It supports listing dashboards and retrieving dashboard JSON with Bearer token authentication.
type GrafanaClient struct {
	config        *Config
	client        *http.Client
	secretWatcher *SecretWatcher
	logger        *logging.Logger
}

// DashboardMeta represents a dashboard in the list response
type DashboardMeta struct {
	UID         string   `json:"uid"`
	Title       string   `json:"title"`
	Tags        []string `json:"tags"`
	FolderTitle string   `json:"folderTitle"`
	URL         string   `json:"url"`
}

// NewGrafanaClient creates a new Grafana HTTP client with tuned connection pooling.
// config: Grafana configuration (URL)
// secretWatcher: Optional SecretWatcher for dynamic token authentication (may be nil)
// logger: Logger for observability
func NewGrafanaClient(config *Config, secretWatcher *SecretWatcher, logger *logging.Logger) *GrafanaClient {
	// Create tuned HTTP transport for high-throughput queries
	transport := &http.Transport{
		// Connection pool settings
		MaxIdleConns:        100,              // Global connection pool
		MaxConnsPerHost:     20,               // Per-host connection limit
		MaxIdleConnsPerHost: 10,               // CRITICAL: default 2 causes connection churn
		IdleConnTimeout:     90 * time.Second, // Keep-alive for idle connections
		TLSHandshakeTimeout: 10 * time.Second,

		// Dialer settings
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,  // Connection establishment timeout
			KeepAlive: 30 * time.Second, // TCP keep-alive interval
		}).DialContext,
	}

	return &GrafanaClient{
		config: config,
		client: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second, // Overall request timeout
		},
		secretWatcher: secretWatcher,
		logger:        logger,
	}
}

// ListDashboards retrieves all dashboards from Grafana.
// Uses /api/search endpoint with type=dash-db filter and limit=5000 (handles most deployments).
func (c *GrafanaClient) ListDashboards(ctx context.Context) ([]DashboardMeta, error) {
	// Build request URL with query parameters
	reqURL := fmt.Sprintf("%s/api/search?type=dash-db&limit=5000", c.config.URL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create list dashboards request: %w", err)
	}

	// Add Bearer token authentication if using secret watcher
	if c.secretWatcher != nil {
		token, err := c.secretWatcher.GetToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get API token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute list dashboards request: %w", err)
	}
	defer resp.Body.Close()

	// CRITICAL: Always read response body to completion for connection reuse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Grafana list dashboards failed: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("list dashboards failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var dashboards []DashboardMeta
	if err := json.Unmarshal(body, &dashboards); err != nil {
		return nil, fmt.Errorf("parse dashboards response: %w", err)
	}

	c.logger.Debug("Listed %d dashboards from Grafana", len(dashboards))
	return dashboards, nil
}

// GetDashboard retrieves a dashboard's full JSON by UID.
// Uses /api/dashboards/uid/{uid} endpoint.
// Returns the complete dashboard structure as a map for flexible parsing.
func (c *GrafanaClient) GetDashboard(ctx context.Context, uid string) (map[string]interface{}, error) {
	// Build request URL
	reqURL := fmt.Sprintf("%s/api/dashboards/uid/%s", c.config.URL, uid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create get dashboard request: %w", err)
	}

	// Add Bearer token authentication if using secret watcher
	if c.secretWatcher != nil {
		token, err := c.secretWatcher.GetToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get API token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute get dashboard request: %w", err)
	}
	defer resp.Body.Close()

	// CRITICAL: Always read response body to completion for connection reuse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Grafana get dashboard failed: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("get dashboard failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var dashboard map[string]interface{}
	if err := json.Unmarshal(body, &dashboard); err != nil {
		return nil, fmt.Errorf("parse dashboard response: %w", err)
	}

	c.logger.Debug("Retrieved dashboard %s from Grafana", uid)
	return dashboard, nil
}

// ListDatasources retrieves all datasources from Grafana.
// Uses /api/datasources endpoint.
// Returns the datasources list as a slice of maps for flexible parsing.
func (c *GrafanaClient) ListDatasources(ctx context.Context) ([]map[string]interface{}, error) {
	// Build request URL
	reqURL := fmt.Sprintf("%s/api/datasources", c.config.URL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create list datasources request: %w", err)
	}

	// Add Bearer token authentication if using secret watcher
	if c.secretWatcher != nil {
		token, err := c.secretWatcher.GetToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get API token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute list datasources request: %w", err)
	}
	defer resp.Body.Close()

	// CRITICAL: Always read response body to completion for connection reuse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("Grafana list datasources failed: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("list datasources failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var datasources []map[string]interface{}
	if err := json.Unmarshal(body, &datasources); err != nil {
		return nil, fmt.Errorf("parse datasources response: %w", err)
	}

	c.logger.Debug("Listed %d datasources from Grafana", len(datasources))
	return datasources, nil
}
