package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/integration/grafana"
	"github.com/moolen/spectre/internal/logging"
)

// SimpleGrafanaClient is a minimal client for fetching Grafana data
type SimpleGrafanaClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewSimpleGrafanaClient(baseURL, token string) *SimpleGrafanaClient {
	return &SimpleGrafanaClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: nil, // Will use default, may need InsecureSkipVerify for self-signed
			},
		},
	}
}

func (c *SimpleGrafanaClient) doRequest(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var body []byte
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			body = append(body, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	return body, nil
}

type DashboardSearchResult struct {
	UID         string   `json:"uid"`
	Title       string   `json:"title"`
	FolderTitle string   `json:"folderTitle"`
	Tags        []string `json:"tags"`
	URL         string   `json:"url"`
}

type DashboardResponse struct {
	Dashboard json.RawMessage `json:"dashboard"`
	Meta      struct {
		FolderTitle string `json:"folderTitle"`
		Updated     string `json:"updated"`
	} `json:"meta"`
}

type AlertRule struct {
	UID       string            `json:"uid"`
	Title     string            `json:"title"`
	FolderUID string            `json:"folderUID"`
	RuleGroup string            `json:"ruleGroup"`
	Labels    map[string]string `json:"labels"`
}

// Signal represents an extracted signal for reporting
type Signal struct {
	MetricName   string  `json:"metric_name"`
	Role         string  `json:"role"`
	Namespace    string  `json:"namespace"`
	Workload     string  `json:"workload"`
	DashboardUID string  `json:"dashboard_uid"`
	PanelTitle   string  `json:"panel_title"`
	Quality      float64 `json:"quality"`
}

func main() {
	grafanaURL := os.Getenv("GRAFANA_URL")
	grafanaToken := os.Getenv("GRAFANA_TOKEN")

	if grafanaURL == "" || grafanaToken == "" {
		fmt.Println("Usage: GRAFANA_URL=https://grafana.lab GRAFANA_TOKEN=xxx go run ./cmd/grafana-observatory-report/")
		os.Exit(1)
	}

	ctx := context.Background()
	client := NewSimpleGrafanaClient(grafanaURL, grafanaToken)
	logger := logging.GetLogger("report")

	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println("OBSERVATORY GRAFANA REPORT")
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Printf("Grafana URL: %s\n", grafanaURL)
	fmt.Printf("Generated:   %s\n", time.Now().Format(time.RFC3339))
	fmt.Println()

	// 1. Fetch dashboards
	fmt.Println("## DASHBOARDS")
	fmt.Println("-" + strings.Repeat("-", 79))

	dashboardsJSON, err := client.doRequest(ctx, "/api/search?type=dash-db&limit=100")
	if err != nil {
		fmt.Printf("ERROR: Failed to fetch dashboards: %v\n", err)
		os.Exit(1)
	}

	var dashboards []DashboardSearchResult
	if err := json.Unmarshal(dashboardsJSON, &dashboards); err != nil {
		fmt.Printf("ERROR: Failed to parse dashboards: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d dashboards\n\n", len(dashboards))
	for i, d := range dashboards {
		if i >= 20 {
			fmt.Printf("  ... and %d more\n", len(dashboards)-20)
			break
		}
		folder := d.FolderTitle
		if folder == "" {
			folder = "General"
		}
		fmt.Printf("  [%s] %s (uid: %s)\n", folder, d.Title, d.UID)
	}
	fmt.Println()

	// 2. Fetch alert rules
	fmt.Println("## ALERT RULES")
	fmt.Println("-" + strings.Repeat("-", 79))

	alertsJSON, err := client.doRequest(ctx, "/api/v1/provisioning/alert-rules")
	if err != nil {
		fmt.Printf("Warning: Could not fetch alert rules: %v\n", err)
	} else {
		var alerts []AlertRule
		if err := json.Unmarshal(alertsJSON, &alerts); err != nil {
			fmt.Printf("Warning: Failed to parse alerts: %v\n", err)
		} else {
			fmt.Printf("Found %d alert rules\n\n", len(alerts))
			for i, a := range alerts {
				if i >= 15 {
					fmt.Printf("  ... and %d more\n", len(alerts)-15)
					break
				}
				severity := a.Labels["severity"]
				if severity == "" {
					severity = "unknown"
				}
				fmt.Printf("  [%s] %s (group: %s)\n", severity, a.Title, a.RuleGroup)
			}
		}
	}
	fmt.Println()

	// 3. Extract signals from dashboards
	fmt.Println("## EXTRACTED SIGNALS")
	fmt.Println("-" + strings.Repeat("-", 79))

	var allSignals []Signal
	now := time.Now().UnixNano()
	_ = logger // silence unused

	for _, d := range dashboards {
		// Fetch full dashboard
		dashJSON, err := client.doRequest(ctx, "/api/dashboards/uid/"+d.UID)
		if err != nil {
			continue
		}

		var dashResp DashboardResponse
		if err := json.Unmarshal(dashJSON, &dashResp); err != nil {
			continue
		}

		// Parse dashboard
		var dashboardData map[string]interface{}
		if err := json.Unmarshal(dashResp.Dashboard, &dashboardData); err != nil {
			continue
		}

		// Build GrafanaDashboard for signal extraction
		gd := &grafana.GrafanaDashboard{
			UID:   d.UID,
			Title: d.Title,
		}

		// Extract panels
		if panels, ok := dashboardData["panels"].([]interface{}); ok {
			for _, p := range panels {
				if panel, ok := p.(map[string]interface{}); ok {
					gp := grafana.GrafanaPanel{
						ID:    int(getFloat(panel, "id")),
						Title: getString(panel, "title"),
						Type:  getString(panel, "type"),
					}

					// Extract targets (queries)
					if targets, ok := panel["targets"].([]interface{}); ok {
						for _, t := range targets {
							if target, ok := t.(map[string]interface{}); ok {
								gt := grafana.GrafanaTarget{
									Expr:   getString(target, "expr"),
									RefID:  getString(target, "refId"),
								}
								gp.Targets = append(gp.Targets, gt)
							}
						}
					}

					gd.Panels = append(gd.Panels, gp)
				}
			}
		}

		// Extract signals using the real extractor
		signals, err := grafana.ExtractSignalsFromDashboard(gd, 0.7, "grafana-report", now)
		if err != nil {
			continue
		}
		for _, sig := range signals {
			allSignals = append(allSignals, Signal{
				MetricName:   sig.MetricName,
				Role:         string(sig.Role),
				Namespace:    sig.WorkloadNamespace,
				Workload:     sig.WorkloadName,
				DashboardUID: d.UID,
				PanelTitle:   fmt.Sprintf("Panel %d", sig.PanelID),
				Quality:      sig.QualityScore,
			})
		}
	}

	// Group signals by namespace/workload
	signalsByWorkload := make(map[string][]Signal)
	for _, s := range allSignals {
		key := s.Namespace + "/" + s.Workload
		if s.Namespace == "" || s.Workload == "" {
			key = "unlinked"
		}
		signalsByWorkload[key] = append(signalsByWorkload[key], s)
	}

	fmt.Printf("Extracted %d total signals\n\n", len(allSignals))

	// Sort keys for consistent output
	var keys []string
	for k := range signalsByWorkload {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		signals := signalsByWorkload[key]
		if key == "unlinked" {
			fmt.Printf("### Unlinked Signals (%d)\n", len(signals))
		} else {
			fmt.Printf("### %s (%d signals)\n", key, len(signals))
		}

		// Show up to 10 signals per workload
		for i, s := range signals {
			if i >= 10 {
				fmt.Printf("    ... and %d more\n", len(signals)-10)
				break
			}
			fmt.Printf("    - %s [%s] (from: %s)\n", s.MetricName, s.Role, s.PanelTitle)
		}
		fmt.Println()
	}

	// 4. Simulate MCP tool responses
	fmt.Println("## SIMULATED MCP TOOL RESPONSES")
	fmt.Println("-" + strings.Repeat("-", 79))

	// observatory_status simulation
	fmt.Println("\n### observatory_status {}")
	fmt.Println("```json")

	// Build hotspots from extracted data
	type Hotspot struct {
		Namespace   string  `json:"namespace"`
		Score       float64 `json:"score"`
		Confidence  float64 `json:"confidence"`
		SignalCount int     `json:"signal_count"`
	}

	var hotspots []Hotspot
	namespaceSignals := make(map[string]int)
	for _, s := range allSignals {
		if s.Namespace != "" {
			namespaceSignals[s.Namespace]++
		}
	}

	for ns, count := range namespaceSignals {
		hotspots = append(hotspots, Hotspot{
			Namespace:   ns,
			Score:       0.0, // Would need actual metrics to compute
			Confidence:  0.8,
			SignalCount: count,
		})
	}

	// Sort by signal count
	sort.Slice(hotspots, func(i, j int) bool {
		return hotspots[i].SignalCount > hotspots[j].SignalCount
	})

	// Limit to top 5
	if len(hotspots) > 5 {
		hotspots = hotspots[:5]
	}

	statusResp := map[string]interface{}{
		"top_hotspots":           hotspots,
		"total_anomalous_signals": 0, // Would need metrics to determine
		"timestamp":              time.Now().Format(time.RFC3339),
		"note":                   "Scores are 0 because no metric queries were executed. In production, these would reflect actual anomaly detection.",
	}
	statusJSON, _ := json.MarshalIndent(statusResp, "", "  ")
	fmt.Println(string(statusJSON))
	fmt.Println("```")

	// observatory_signals simulation for top namespace
	if len(hotspots) > 0 {
		topNS := hotspots[0].Namespace
		fmt.Printf("\n### observatory_scope {\"namespace\": \"%s\"}\n", topNS)
		fmt.Println("```json")

		var workloadAnomalies []map[string]interface{}
		workloadCounts := make(map[string]int)

		for _, s := range allSignals {
			if s.Namespace == topNS && s.Workload != "" {
				workloadCounts[s.Workload]++
			}
		}

		for workload, count := range workloadCounts {
			workloadAnomalies = append(workloadAnomalies, map[string]interface{}{
				"workload":     workload,
				"score":        0.0,
				"confidence":   0.8,
				"signal_count": count,
			})
		}

		// Sort by signal count
		sort.Slice(workloadAnomalies, func(i, j int) bool {
			return workloadAnomalies[i]["signal_count"].(int) > workloadAnomalies[j]["signal_count"].(int)
		})

		scopeResp := map[string]interface{}{
			"anomalies": workloadAnomalies,
			"scope":     topNS,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		scopeJSON, _ := json.MarshalIndent(scopeResp, "", "  ")
		fmt.Println(string(scopeJSON))
		fmt.Println("```")

		// Show signals for top workload
		if len(workloadAnomalies) > 0 {
			topWorkload := workloadAnomalies[0]["workload"].(string)
			fmt.Printf("\n### observatory_signals {\"namespace\": \"%s\", \"workload\": \"%s\"}\n", topNS, topWorkload)
			fmt.Println("```json")

			var signalStates []map[string]interface{}
			for _, s := range allSignals {
				if s.Namespace == topNS && s.Workload == topWorkload {
					signalStates = append(signalStates, map[string]interface{}{
						"metric_name":   s.MetricName,
						"role":          s.Role,
						"score":         0.0,
						"confidence":    0.8,
						"quality_score": s.Quality,
					})
				}
			}

			signalsResp := map[string]interface{}{
				"signals":   signalStates,
				"scope":     fmt.Sprintf("%s/%s", topNS, topWorkload),
				"timestamp": time.Now().Format(time.RFC3339),
			}
			signalsJSON, _ := json.MarshalIndent(signalsResp, "", "  ")
			fmt.Println(string(signalsJSON))
			fmt.Println("```")
		}
	}

	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println("NOTE: Anomaly scores are 0 because this report does not query actual metrics.")
	fmt.Println("In production, Observatory would:")
	fmt.Println("  1. Query current metric values from Grafana/Prometheus")
	fmt.Println("  2. Compare against historical baselines stored in FalkorDB")
	fmt.Println("  3. Compute anomaly scores using z-score + percentile hybrid")
	fmt.Println("=" + strings.Repeat("=", 79))
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}
