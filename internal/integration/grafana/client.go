package grafana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// AlertRule represents a Grafana alert rule from the Alerting Provisioning API
type AlertRule struct {
	UID         string            `json:"uid"`         // Alert rule UID
	Title       string            `json:"title"`       // Alert rule title
	FolderUID   string            `json:"folderUID"`   // Folder UID
	RuleGroup   string            `json:"ruleGroup"`   // Rule group name
	Data        []AlertQuery      `json:"data"`        // Alert queries (PromQL expressions)
	Labels      map[string]string `json:"labels"`      // Alert labels
	Annotations map[string]string `json:"annotations"` // Annotations including severity
	Updated     time.Time         `json:"updated"`     // Last update timestamp
}

// AlertQuery represents a query within an alert rule
type AlertQuery struct {
	RefID         string          `json:"refId"`         // Query reference ID
	Model         json.RawMessage `json:"model"`         // Query model (contains PromQL)
	DatasourceUID string          `json:"datasourceUID"` // Datasource UID
	QueryType     string          `json:"queryType"`     // Query type (typically "prometheus")
}

// AlertState represents an alert rule with its current state and instances
type AlertState struct {
	UID       string          `json:"-"`      // Extracted from rule
	Title     string          `json:"-"`      // Extracted from rule
	State     string          `json:"state"`  // Alert rule evaluation state
	Instances []AlertInstance `json:"alerts"` // Active alert instances
}

// AlertInstance represents a single alert instance (specific label combination)
type AlertInstance struct {
	Labels   map[string]string `json:"labels"`   // Alert instance labels
	State    string            `json:"state"`    // firing, pending, normal
	ActiveAt *time.Time        `json:"activeAt"` // When instance became active (nil if normal)
	Value    string            `json:"value"`    // Current metric value
}

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

// ListAlertRules retrieves all alert rules from Grafana Alerting Provisioning API.
// Uses /api/v1/provisioning/alert-rules endpoint.
func (c *GrafanaClient) ListAlertRules(ctx context.Context) ([]AlertRule, error) {
	// Build request URL
	reqURL := fmt.Sprintf("%s/api/v1/provisioning/alert-rules", c.config.URL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create list alert rules request: %w", err)
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
		return nil, fmt.Errorf("execute list alert rules request: %w", err)
	}
	defer resp.Body.Close()

	// CRITICAL: Always read response body to completion for connection reuse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Grafana list alert rules failed: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("list alert rules failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var alertRules []AlertRule
	if err := json.Unmarshal(body, &alertRules); err != nil {
		return nil, fmt.Errorf("parse alert rules response: %w", err)
	}

	c.logger.Debug("Listed %d alert rules from Grafana", len(alertRules))
	return alertRules, nil
}

// GetAlertRule retrieves a single alert rule by UID from Grafana Alerting Provisioning API.
// Uses /api/v1/provisioning/alert-rules/{uid} endpoint.
func (c *GrafanaClient) GetAlertRule(ctx context.Context, uid string) (*AlertRule, error) {
	// Build request URL
	reqURL := fmt.Sprintf("%s/api/v1/provisioning/alert-rules/%s", c.config.URL, uid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create get alert rule request: %w", err)
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
		return nil, fmt.Errorf("execute get alert rule request: %w", err)
	}
	defer resp.Body.Close()

	// CRITICAL: Always read response body to completion for connection reuse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Grafana get alert rule failed: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("get alert rule failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var alertRule AlertRule
	if err := json.Unmarshal(body, &alertRule); err != nil {
		return nil, fmt.Errorf("parse alert rule response: %w", err)
	}

	c.logger.Debug("Retrieved alert rule %s from Grafana", uid)
	return &alertRule, nil
}

// PrometheusRulesResponse represents the response from /api/prometheus/grafana/api/v1/rules
type PrometheusRulesResponse struct {
	Status string `json:"status"`
	Data   struct {
		Groups []PrometheusRuleGroup `json:"groups"`
	} `json:"data"`
}

// PrometheusRuleGroup represents a rule group in Prometheus format
type PrometheusRuleGroup struct {
	Name  string             `json:"name"`
	File  string             `json:"file"`
	Rules []PrometheusRule   `json:"rules"`
}

// PrometheusRule represents a rule with its current state and instances
type PrometheusRule struct {
	Name   string            `json:"name"`   // Alert rule name
	Query  string            `json:"query"`  // PromQL expression
	Labels map[string]string `json:"labels"` // Rule labels
	State  string            `json:"state"`  // Alert rule evaluation state
	Alerts []AlertInstance   `json:"alerts"` // Active alert instances
}

// GetAlertStates retrieves current alert states from Grafana using Prometheus-compatible API.
// Uses /api/prometheus/grafana/api/v1/rules endpoint which returns alert rules with instances.
// Maps Grafana state values: "alerting" -> "firing", normalizes to lowercase.
func (c *GrafanaClient) GetAlertStates(ctx context.Context) ([]AlertState, error) {
	// Build request URL
	reqURL := fmt.Sprintf("%s/api/prometheus/grafana/api/v1/rules", c.config.URL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create get alert states request: %w", err)
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
		return nil, fmt.Errorf("execute get alert states request: %w", err)
	}
	defer resp.Body.Close()

	// CRITICAL: Always read response body to completion for connection reuse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Grafana get alert states failed: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("get alert states failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var result PrometheusRulesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse alert states response: %w", err)
	}

	// Extract alert states from nested structure
	var alertStates []AlertState
	for _, group := range result.Data.Groups {
		for _, rule := range group.Rules {
			// Extract UID from labels (uid label injected by Grafana)
			uid := rule.Labels["grafana_uid"]
			if uid == "" {
				// Skip rules without UID (not Grafana-managed alerts)
				continue
			}

			// Normalize state: "alerting" -> "firing", lowercase
			state := rule.State
			if state == "alerting" {
				state = "firing"
			}
			state = strings.ToLower(state)

			alertStates = append(alertStates, AlertState{
				UID:       uid,
				Title:     rule.Name,
				State:     state,
				Instances: rule.Alerts,
			})
		}
	}

	c.logger.Debug("Retrieved %d alert states from Grafana", len(alertStates))
	return alertStates, nil
}

// QueryRequest represents a request to Grafana's /api/ds/query endpoint
type QueryRequest struct {
	Queries []Query `json:"queries"`
	From    string  `json:"from"` // epoch milliseconds as string
	To      string  `json:"to"`   // epoch milliseconds as string
}

// Query represents a single query within a QueryRequest
type Query struct {
	RefID         string              `json:"refId"`
	Datasource    QueryDatasource     `json:"datasource"`
	Expr          string              `json:"expr"`
	Format        string              `json:"format"`        // "time_series"
	MaxDataPoints int                 `json:"maxDataPoints"` // 100
	IntervalMs    int                 `json:"intervalMs"`    // 1000
	ScopedVars    map[string]ScopedVar `json:"scopedVars,omitempty"`
}

// QueryDatasource identifies a datasource in a query
type QueryDatasource struct {
	UID string `json:"uid"`
}

// ScopedVar represents a scoped variable for Grafana variable substitution
type ScopedVar struct {
	Text  string `json:"text"`
	Value string `json:"value"`
}

// QueryResponse represents the response from Grafana's /api/ds/query endpoint
type QueryResponse struct {
	Results map[string]QueryResult `json:"results"`
}

// QueryResult represents a single result in the query response
type QueryResult struct {
	Frames []DataFrame `json:"frames"`
	Error  string      `json:"error,omitempty"`
}

// DataFrame represents a Grafana data frame
type DataFrame struct {
	Schema DataFrameSchema `json:"schema"`
	Data   DataFrameData   `json:"data"`
}

// DataFrameSchema contains metadata about a data frame
type DataFrameSchema struct {
	Name   string              `json:"name,omitempty"`
	Fields []DataFrameField    `json:"fields"`
}

// DataFrameField represents a field in a data frame schema
type DataFrameField struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Labels map[string]string `json:"labels,omitempty"`
	Config *FieldConfig      `json:"config,omitempty"`
}

// FieldConfig contains field configuration like unit
type FieldConfig struct {
	Unit string `json:"unit,omitempty"`
}

// DataFrameData contains the actual data values
type DataFrameData struct {
	Values [][]interface{} `json:"values"` // First array is timestamps, second is values
}

// QueryDataSource executes a PromQL query via Grafana's /api/ds/query endpoint.
// datasourceUID: the UID of the datasource to query
// expr: the PromQL expression to execute
// from, to: time range as epoch milliseconds (as strings)
// scopedVars: variables for server-side substitution (e.g., cluster, region)
func (c *GrafanaClient) QueryDataSource(ctx context.Context, datasourceUID string, expr string, from string, to string, scopedVars map[string]ScopedVar) (*QueryResponse, error) {
	// Build query request
	reqBody := QueryRequest{
		Queries: []Query{
			{
				RefID:         "A",
				Datasource:    QueryDatasource{UID: datasourceUID},
				Expr:          expr,
				Format:        "time_series",
				MaxDataPoints: 100,
				IntervalMs:    1000,
				ScopedVars:    scopedVars,
			},
		},
		From: from,
		To:   to,
	}

	// Marshal request body
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal query request: %w", err)
	}

	// Build HTTP request
	reqURL := fmt.Sprintf("%s/api/ds/query", c.config.URL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("create query request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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
		return nil, fmt.Errorf("execute query request: %w", err)
	}
	defer resp.Body.Close()

	// CRITICAL: Always read response body to completion for connection reuse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Grafana query failed: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("query failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var result QueryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse query response: %w", err)
	}

	c.logger.Debug("Executed query against datasource %s", datasourceUID)
	return &result, nil
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
