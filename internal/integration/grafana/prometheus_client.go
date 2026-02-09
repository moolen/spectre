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

// ScrapeTarget represents a Prometheus scrape target with its metadata labels.
type ScrapeTarget struct {
	Labels             map[string]string // namespace, pod, job, app, etc.
	ScrapePool         string            // job name
	Health             string            // "up" | "down" | "unknown"
	LastScrape         time.Time
	LastScrapeDuration time.Duration
}

// PrometheusClient is an HTTP client for direct Prometheus API access.
// It supports fetching scrape targets for linking SignalAnchors to K8s workloads.
type PrometheusClient struct {
	baseURL       string
	client        *http.Client
	secretWatcher *SecretWatcher
	secretRef     *SecretRef
	logger        *logging.Logger
}

// prometheusTargetsResponse represents the response from /api/v1/targets
type prometheusTargetsResponse struct {
	Status string `json:"status"`
	Data   struct {
		ActiveTargets []prometheusTarget `json:"activeTargets"`
	} `json:"data"`
}

// prometheusTarget represents a single target in the targets response
type prometheusTarget struct {
	Labels             map[string]string `json:"labels"`
	ScrapePool         string            `json:"scrapePool"`
	ScrapeURL          string            `json:"scrapeUrl"`
	Health             string            `json:"health"`
	LastScrape         string            `json:"lastScrape"`
	LastScrapeDuration float64           `json:"lastScrapeDuration"` // seconds
}

// NewPrometheusClient creates a new Prometheus HTTP client with tuned connection pooling.
// baseURL: Prometheus API base URL (e.g., http://prometheus:9090)
// secretRef: Optional SecretRef for token authentication (may be nil)
// secretWatcher: Optional SecretWatcher for dynamic token authentication (may be nil)
// logger: Logger for observability
func NewPrometheusClient(baseURL string, secretRef *SecretRef, secretWatcher *SecretWatcher, logger *logging.Logger) *PrometheusClient {
	// Create tuned HTTP transport (same pattern as GrafanaClient)
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxConnsPerHost:     20,
		MaxIdleConnsPerHost: 10,               // CRITICAL: default 2 causes connection churn
		IdleConnTimeout:     90 * time.Second, // Keep-alive for idle connections
		TLSHandshakeTimeout: 10 * time.Second,

		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	return &PrometheusClient{
		baseURL: baseURL,
		client: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		secretWatcher: secretWatcher,
		secretRef:     secretRef,
		logger:        logger,
	}
}

// GetTargets fetches active scrape targets from Prometheus.
// Only returns healthy targets (health="up") to ensure accurate linking.
func (c *PrometheusClient) GetTargets(ctx context.Context) ([]ScrapeTarget, error) {
	// Build request URL
	reqURL := fmt.Sprintf("%s/api/v1/targets?state=active", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create targets request: %w", err)
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
		return nil, fmt.Errorf("execute targets request: %w", err)
	}
	defer resp.Body.Close()

	// CRITICAL: Always read response body to completion for connection reuse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Prometheus get targets failed: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("get targets failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var result prometheusTargetsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse targets response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus API returned non-success status: %s", result.Status)
	}

	// Convert to ScrapeTarget structs, filtering for healthy targets
	var targets []ScrapeTarget
	for _, t := range result.Data.ActiveTargets {
		// Only include healthy targets
		if t.Health != "up" {
			continue
		}

		// Parse lastScrape timestamp
		var lastScrape time.Time
		if t.LastScrape != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, t.LastScrape); err == nil {
				lastScrape = parsed
			}
		}

		targets = append(targets, ScrapeTarget{
			Labels:             t.Labels,
			ScrapePool:         t.ScrapePool,
			Health:             t.Health,
			LastScrape:         lastScrape,
			LastScrapeDuration: time.Duration(t.LastScrapeDuration * float64(time.Second)),
		})
	}

	c.logger.Debug("Fetched %d healthy scrape targets from Prometheus", len(targets))
	return targets, nil
}

// TestConnection tests connectivity to Prometheus by fetching targets.
// Returns nil if successful, error otherwise.
func (c *PrometheusClient) TestConnection(ctx context.Context) error {
	_, err := c.GetTargets(ctx)
	return err
}
