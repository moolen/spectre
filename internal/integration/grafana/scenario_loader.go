package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/observatory"
)

// Scenario represents a complete test scenario with seed data and expected outputs
type Scenario struct {
	Name        string
	Description string
	SeedData    SeedData
	Topology    TopologyData
	Expected    map[string][]byte // tool name -> expected JSON
}

// SeedData contains all data to seed into FalkorDB for a test scenario
type SeedData struct {
	SignalAnchors  []SignalAnchorSeed  `json:"signal_anchors"`
	SignalBaselines []SignalBaselineSeed `json:"signal_baselines"`
	Dashboards     []DashboardSeed     `json:"dashboards"`
	CurrentValues  map[string]float64  `json:"current_values"`  // metric|ns|workload -> value
	AlertStates    map[string]string   `json:"alert_states"`    // metric|ns|workload -> state
}

// SignalAnchorSeed represents a signal anchor to seed
type SignalAnchorSeed struct {
	MetricName        string  `json:"metric_name"`
	Role              string  `json:"role"`
	Confidence        float64 `json:"confidence"`
	QualityScore      float64 `json:"quality_score"`
	WorkloadNamespace string  `json:"workload_namespace"`
	WorkloadName      string  `json:"workload_name"`
	DashboardUID      string  `json:"dashboard_uid"`
	PanelID           int     `json:"panel_id"`
}

// SignalBaselineSeed represents a signal baseline to seed
type SignalBaselineSeed struct {
	MetricName        string  `json:"metric_name"`
	WorkloadNamespace string  `json:"workload_namespace"`
	WorkloadName      string  `json:"workload_name"`
	Mean              float64 `json:"mean"`
	StdDev            float64 `json:"std_dev"`
	Min               float64 `json:"min"`
	Max               float64 `json:"max"`
	P50               float64 `json:"p50"`
	P90               float64 `json:"p90"`
	P99               float64 `json:"p99"`
	SampleCount       int     `json:"sample_count"`
}

// DashboardSeed represents a dashboard to seed
type DashboardSeed struct {
	UID         string  `json:"uid"`
	Title       string  `json:"title"`
	QualityScore float64 `json:"quality_score"`
	FolderTitle string  `json:"folder_title"`
}

// TopologyData contains K8s resource topology for evidence tools
type TopologyData struct {
	Resources    []ResourceSeed    `json:"resources"`
	Dependencies []DependencySeed  `json:"dependencies"`
	Events       []EventSeed       `json:"events"`
}

// ResourceSeed represents a K8s resource identity
type ResourceSeed struct {
	UID       string `json:"uid"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// DependencySeed represents a dependency edge
type DependencySeed struct {
	FromUID      string `json:"from_uid"`
	ToUID        string `json:"to_uid"`
	Relationship string `json:"relationship"`
}

// EventSeed represents a K8s event
type EventSeed struct {
	UID             string `json:"uid"`
	Kind            string `json:"kind"`
	Namespace       string `json:"namespace"`
	Name            string `json:"name"`
	Reason          string `json:"reason"`
	TimestampOffset string `json:"timestamp_offset"` // e.g., "-30m", "-1h"
	AffectsUID      string `json:"affects_uid"`
}

// LoadScenario loads a test scenario from a directory
func LoadScenario(scenarioPath string) (*Scenario, error) {
	scenario := &Scenario{
		Name:     filepath.Base(scenarioPath),
		Expected: make(map[string][]byte),
	}

	// Load seed.json
	seedPath := filepath.Join(scenarioPath, "seed.json")
	seedData, err := os.ReadFile(seedPath)
	if err != nil {
		return nil, fmt.Errorf("read seed.json: %w", err)
	}
	if err := json.Unmarshal(seedData, &scenario.SeedData); err != nil {
		return nil, fmt.Errorf("parse seed.json: %w", err)
	}

	// Validate seed data
	if err := validateSeedData(&scenario.SeedData); err != nil {
		return nil, fmt.Errorf("validate seed data: %w", err)
	}

	// Load topology.json (optional)
	topologyPath := filepath.Join(scenarioPath, "topology.json")
	if topologyData, err := os.ReadFile(topologyPath); err == nil {
		if err := json.Unmarshal(topologyData, &scenario.Topology); err != nil {
			return nil, fmt.Errorf("parse topology.json: %w", err)
		}
	}

	// Load expected golden files
	expectedDir := filepath.Join(scenarioPath, "expected")
	entries, err := os.ReadDir(expectedDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			toolName := entry.Name()[:len(entry.Name())-len(".golden.json")]
			data, err := os.ReadFile(filepath.Join(expectedDir, entry.Name()))
			if err != nil {
				return nil, fmt.Errorf("read expected/%s: %w", entry.Name(), err)
			}
			scenario.Expected[toolName] = data
		}
	}

	return scenario, nil
}

// validateSeedData validates seed data has required properties
func validateSeedData(seed *SeedData) error {
	for i, signal := range seed.SignalAnchors {
		if signal.MetricName == "" {
			return fmt.Errorf("signal_anchors[%d]: metric_name is required", i)
		}
		if signal.WorkloadNamespace == "" {
			return fmt.Errorf("signal_anchors[%d]: workload_namespace is required", i)
		}
		if signal.WorkloadName == "" {
			return fmt.Errorf("signal_anchors[%d]: workload_name is required", i)
		}
		if signal.Role == "" {
			return fmt.Errorf("signal_anchors[%d]: role is required", i)
		}
	}

	for i, baseline := range seed.SignalBaselines {
		if baseline.MetricName == "" {
			return fmt.Errorf("signal_baselines[%d]: metric_name is required", i)
		}
		if baseline.SampleCount < 0 {
			return fmt.Errorf("signal_baselines[%d]: sample_count must be non-negative", i)
		}
	}

	return nil
}

// SeedScenario seeds a scenario into the graph and configures the harness.
// Seeds data into both FalkorDB (for graph queries) and the testProvider (for registry-based services).
func SeedScenario(ctx context.Context, harness *ObservatoryTestHarness, scenario *Scenario) error {
	// Clear previous test state
	harness.ClearTestState()

	now := time.Now().Unix()
	expiresAt := now + (7 * 24 * 60 * 60) // 7 days from now

	// Get test provider for registry-based seeding
	testProvider := harness.GetTestProvider()

	// Seed signal anchors into graph AND testProvider
	for _, anchor := range scenario.SeedData.SignalAnchors {
		// Seed into FalkorDB for graph-based queries
		if err := seedSignalAnchor(ctx, harness.GetGraphClient(), harness.integrationName, anchor, now, expiresAt); err != nil {
			return fmt.Errorf("seed signal anchor %s: %w", anchor.MetricName, err)
		}

		// Seed into testProvider for registry-based services
		testProvider.AddSignal(observatory.SignalAnchor{
			MetricName:        anchor.MetricName,
			Role:              observatory.SignalRole(anchor.Role),
			Confidence:        anchor.Confidence,
			QualityScore:      anchor.QualityScore,
			WorkloadNamespace: anchor.WorkloadNamespace,
			WorkloadName:      anchor.WorkloadName,
			SourceRef:         anchor.DashboardUID,
		})
	}

	// Seed signal baselines into graph AND testProvider
	for _, baseline := range scenario.SeedData.SignalBaselines {
		// Seed into FalkorDB with HAS_BASELINE edges
		if err := seedSignalBaseline(ctx, harness.GetGraphClient(), harness.integrationName, baseline); err != nil {
			return fmt.Errorf("seed signal baseline %s: %w", baseline.MetricName, err)
		}

		// Seed into testProvider for registry-based services
		testProvider.SetBaseline(baseline.MetricName, baseline.WorkloadNamespace, baseline.WorkloadName, &observatory.SignalBaseline{
			Mean:        baseline.Mean,
			StdDev:      baseline.StdDev,
			Min:         baseline.Min,
			Max:         baseline.Max,
			P50:         baseline.P50,
			P90:         baseline.P90,
			P99:         baseline.P99,
			SampleCount: baseline.SampleCount,
		})
	}

	// Seed dashboards (graph only - not needed for registry)
	for _, dashboard := range scenario.SeedData.Dashboards {
		if err := seedDashboard(ctx, harness.GetGraphClient(), harness.integrationName, dashboard, now, expiresAt); err != nil {
			return fmt.Errorf("seed dashboard %s: %w", dashboard.UID, err)
		}
	}

	// Set current values in harness (syncs to testProvider automatically)
	for key, value := range scenario.SeedData.CurrentValues {
		parts := parseKey(key)
		if len(parts) == 3 {
			harness.SetCurrentValue(parts[0], parts[1], parts[2], value)
		}
	}

	// Set alert states in harness (syncs to testProvider automatically)
	for key, state := range scenario.SeedData.AlertStates {
		parts := parseKey(key)
		if len(parts) == 3 {
			harness.SetAlertState(parts[0], parts[1], parts[2], state)
		}
	}

	// Seed topology if present (graph only - for evidence tools)
	if err := seedTopology(ctx, harness.GetGraphClient(), &scenario.Topology, now, expiresAt); err != nil {
		return fmt.Errorf("seed topology: %w", err)
	}

	return nil
}

// parseKey parses "metric|namespace|workload" format
func parseKey(key string) []string {
	var parts []string
	current := ""
	for _, c := range key {
		if c == '|' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	parts = append(parts, current)
	return parts
}

// seedSignalAnchor creates a SignalAnchor node
func seedSignalAnchor(ctx context.Context, client graph.Client, integration string, anchor SignalAnchorSeed, now, expiresAt int64) error {
	uid := fmt.Sprintf("%s/%s/%s", anchor.WorkloadNamespace, anchor.WorkloadName, anchor.MetricName)

	query := `
		MERGE (s:SignalAnchor {uid: $uid})
		SET s.metric_name = $metric_name,
		    s.role = $role,
		    s.confidence = $confidence,
		    s.quality_score = $quality_score,
		    s.workload_namespace = $workload_namespace,
		    s.workload_name = $workload_name,
		    s.dashboard_uid = $dashboard_uid,
		    s.panel_id = $panel_id,
		    s.integration = $integration,
		    s.first_seen = $first_seen,
		    s.last_seen = $last_seen,
		    s.expires_at = $expires_at
	`

	_, err := client.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]any{
			"uid":                uid,
			"metric_name":        anchor.MetricName,
			"role":               anchor.Role,
			"confidence":         anchor.Confidence,
			"quality_score":      anchor.QualityScore,
			"workload_namespace": anchor.WorkloadNamespace,
			"workload_name":      anchor.WorkloadName,
			"dashboard_uid":      anchor.DashboardUID,
			"panel_id":           anchor.PanelID,
			"integration":        integration,
			"first_seen":         now - (24 * 60 * 60), // 1 day ago
			"last_seen":          now,
			"expires_at":         expiresAt,
		},
	})
	return err
}

// seedSignalBaseline creates a SignalBaseline node and HAS_BASELINE edge
func seedSignalBaseline(ctx context.Context, client graph.Client, integration string, baseline SignalBaselineSeed) error {
	anchorUID := fmt.Sprintf("%s/%s/%s", baseline.WorkloadNamespace, baseline.WorkloadName, baseline.MetricName)
	baselineUID := anchorUID + "/baseline"

	query := `
		MATCH (s:SignalAnchor {uid: $anchor_uid})
		MERGE (b:SignalBaseline {uid: $baseline_uid})
		SET b.mean = $mean,
		    b.std_dev = $std_dev,
		    b.min = $min,
		    b.max = $max,
		    b.p50 = $p50,
		    b.p90 = $p90,
		    b.p99 = $p99,
		    b.sample_count = $sample_count
		MERGE (s)-[:HAS_BASELINE]->(b)
	`

	_, err := client.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]any{
			"anchor_uid":   anchorUID,
			"baseline_uid": baselineUID,
			"mean":         baseline.Mean,
			"std_dev":      baseline.StdDev,
			"min":          baseline.Min,
			"max":          baseline.Max,
			"p50":          baseline.P50,
			"p90":          baseline.P90,
			"p99":          baseline.P99,
			"sample_count": baseline.SampleCount,
		},
	})
	return err
}

// seedDashboard creates a Dashboard node
func seedDashboard(ctx context.Context, client graph.Client, integration string, dashboard DashboardSeed, now, expiresAt int64) error {
	query := `
		MERGE (d:Dashboard {uid: $uid})
		SET d.title = $title,
		    d.quality_score = $quality_score,
		    d.folder_title = $folder_title,
		    d.integration = $integration,
		    d.first_seen = $first_seen,
		    d.last_seen = $last_seen,
		    d.expires_at = $expires_at
	`

	_, err := client.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]any{
			"uid":           dashboard.UID,
			"title":         dashboard.Title,
			"quality_score": dashboard.QualityScore,
			"folder_title":  dashboard.FolderTitle,
			"integration":   integration,
			"first_seen":    now - (24 * 60 * 60),
			"last_seen":     now,
			"expires_at":    expiresAt,
		},
	})
	return err
}

// seedTopology seeds K8s resource topology
func seedTopology(ctx context.Context, client graph.Client, topology *TopologyData, now, expiresAt int64) error {
	// Seed resources
	for _, resource := range topology.Resources {
		query := `
			MERGE (r:ResourceIdentity {uid: $uid})
			SET r.kind = $kind,
			    r.namespace = $namespace,
			    r.name = $name,
			    r.first_seen = $first_seen,
			    r.last_seen = $last_seen,
			    r.expires_at = $expires_at
		`
		_, err := client.ExecuteQuery(ctx, graph.GraphQuery{
			Query: query,
			Parameters: map[string]any{
				"uid":        resource.UID,
				"kind":       resource.Kind,
				"namespace":  resource.Namespace,
				"name":       resource.Name,
				"first_seen": now - (24 * 60 * 60),
				"last_seen":  now,
				"expires_at": expiresAt,
			},
		})
		if err != nil {
			return fmt.Errorf("seed resource %s: %w", resource.UID, err)
		}
	}

	// Seed dependencies
	for _, dep := range topology.Dependencies {
		query := `
			MATCH (from:ResourceIdentity {uid: $from_uid})
			MATCH (to:ResourceIdentity {uid: $to_uid})
			MERGE (from)-[:DEPENDS_ON]->(to)
		`
		_, err := client.ExecuteQuery(ctx, graph.GraphQuery{
			Query: query,
			Parameters: map[string]any{
				"from_uid": dep.FromUID,
				"to_uid":   dep.ToUID,
			},
		})
		if err != nil {
			return fmt.Errorf("seed dependency %s->%s: %w", dep.FromUID, dep.ToUID, err)
		}
	}

	// Seed events
	for _, event := range topology.Events {
		eventTime := parseTimestampOffset(event.TimestampOffset, now)
		query := `
			MERGE (e:Event {uid: $uid})
			SET e.kind = $kind,
			    e.namespace = $namespace,
			    e.name = $name,
			    e.reason = $reason,
			    e.timestamp = $timestamp
			WITH e
			MATCH (r:ResourceIdentity {uid: $affects_uid})
			MERGE (e)-[:AFFECTS]->(r)
		`
		_, err := client.ExecuteQuery(ctx, graph.GraphQuery{
			Query: query,
			Parameters: map[string]any{
				"uid":         event.UID,
				"kind":        event.Kind,
				"namespace":   event.Namespace,
				"name":        event.Name,
				"reason":      event.Reason,
				"timestamp":   eventTime,
				"affects_uid": event.AffectsUID,
			},
		})
		if err != nil {
			return fmt.Errorf("seed event %s: %w", event.UID, err)
		}
	}

	return nil
}

// parseTimestampOffset parses "-30m", "-1h", etc. relative to now
func parseTimestampOffset(offset string, now int64) int64 {
	if offset == "" {
		return now
	}

	duration, err := time.ParseDuration(offset)
	if err != nil {
		return now
	}

	return now + int64(duration.Seconds())
}
