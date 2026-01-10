package namespacegraph

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/analysis/anomaly"
	causalpaths "github.com/moolen/spectre/internal/analysis/causal_paths"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// Analyzer orchestrates namespace graph analysis
type Analyzer struct {
	graphClient         graph.Client
	resourceFetcher     *ResourceFetcher
	relationshipFetcher *RelationshipFetcher
	anomalyDetector     *anomaly.AnomalyDetector
	lightweightDetector *LightweightAnomalyDetector
	pathDiscoverer      *causalpaths.PathDiscoverer
	logger              *logging.Logger
}

// NewAnalyzer creates a new namespace graph Analyzer
func NewAnalyzer(graphClient graph.Client) *Analyzer {
	return &Analyzer{
		graphClient:         graphClient,
		resourceFetcher:     NewResourceFetcher(graphClient),
		relationshipFetcher: NewRelationshipFetcher(graphClient),
		anomalyDetector:     anomaly.NewDetector(graphClient),
		lightweightDetector: NewLightweightAnomalyDetector(),
		pathDiscoverer:      causalpaths.NewPathDiscoverer(graphClient),
		logger:              logging.GetLogger("namespacegraph.analyzer"),
	}
}

// Analyze fetches the namespace graph at a point in time with optional enrichment
func (a *Analyzer) Analyze(ctx context.Context, input AnalyzeInput) (*NamespaceGraphResponse, error) {
	startTime := time.Now()

	// Apply defaults
	if input.Limit <= 0 {
		input.Limit = DefaultLimit
	}
	if input.Limit > MaxLimit {
		input.Limit = MaxLimit
	}
	if input.MaxDepth <= 0 {
		input.MaxDepth = DefaultMaxDepth
	}
	if input.MaxDepth > MaxMaxDepth {
		input.MaxDepth = MaxMaxDepth
	}
	if input.Lookback <= 0 {
		input.Lookback = DefaultLookback
	}
	if input.Lookback > MaxLookback {
		input.Lookback = MaxLookback
	}

	a.logger.Debug("Analyzing namespace graph: namespace=%s, timestamp=%d, limit=%d, maxDepth=%d",
		input.Namespace, input.Timestamp, input.Limit, input.MaxDepth)

	// Step 1: Fetch namespaced resources
	namespacedResources, hasMore, nextCursor, err := a.resourceFetcher.FetchNamespacedResources(
		ctx, input.Namespace, input.Timestamp, input.Limit, input.Cursor)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch namespaced resources: %w", err)
	}

	a.logger.Debug("Fetched %d namespaced resources", len(namespacedResources))

	// Extract UIDs for further queries
	namespacedUIDs := make([]string, len(namespacedResources))
	for i, r := range namespacedResources {
		namespacedUIDs[i] = r.UID
	}

	// Step 2: Fetch cluster-scoped resources related to namespaced resources
	clusterScopedResources, err := a.resourceFetcher.FetchClusterScopedResources(
		ctx, namespacedUIDs, input.Timestamp, input.MaxDepth)
	if err != nil {
		a.logger.Warn("Failed to fetch cluster-scoped resources: %v", err)
		// Continue without cluster-scoped resources
		clusterScopedResources = nil
	}

	a.logger.Debug("Fetched %d cluster-scoped resources", len(clusterScopedResources))

	// Combine all resources
	allResources := append(namespacedResources, clusterScopedResources...)

	// Collect all UIDs for relationship query
	allUIDs := make([]string, len(allResources))
	for i, r := range allResources {
		allUIDs[i] = r.UID
	}

	// Step 3: Fetch latest events for all resources
	latestEvents, err := a.resourceFetcher.FetchLatestEvents(ctx, allUIDs, input.Timestamp)
	if err != nil {
		a.logger.Warn("Failed to fetch latest events: %v", err)
		latestEvents = make(map[string]*ChangeEventInfo)
	}

	a.logger.Info("Fetched %d latest events for %d resources", len(latestEvents), len(allUIDs))

	// Step 4: Fetch relationships between all resources
	edgeResults, err := a.relationshipFetcher.FetchRelationships(ctx, allUIDs)
	if err != nil {
		a.logger.Warn("Failed to fetch relationships: %v", err)
		edgeResults = nil
	}

	a.logger.Info("Fetched %d relationships for %d resources", len(edgeResults), len(allUIDs))

	// Step 5: Build graph response
	nodes := a.buildNodes(allResources, latestEvents)
	edges := a.buildEdges(edgeResults)

	// Step 6: Lightweight anomaly detection (uses already-fetched data, no extra queries)
	var anomalies []anomaly.Anomaly
	if input.IncludeAnomalies {
		anomalies = a.lightweightDetector.DetectFromNodes(nodes, input.Timestamp)
		a.logger.Debug("Detected %d anomalies (lightweight)", len(anomalies))
	}

	// Step 7: Optional causal path discovery (only if anomalies found and requested)
	var causalPaths []causalpaths.CausalPath
	if input.IncludeCausalPaths && len(anomalies) > 0 {
		causalPaths, err = a.discoverCausalPaths(ctx, anomalies, input)
		if err != nil {
			a.logger.Warn("Failed to discover causal paths: %v", err)
			// Continue without causal paths
		}
		a.logger.Debug("Discovered %d causal paths", len(causalPaths))
	}

	// Build response
	response := &NamespaceGraphResponse{
		Graph: Graph{
			Nodes: nodes,
			Edges: edges,
		},
		Anomalies:   anomalies,
		CausalPaths: causalPaths,
		Metadata: Metadata{
			Namespace:        input.Namespace,
			Timestamp:        input.Timestamp,
			NodeCount:        len(nodes),
			EdgeCount:        len(edges),
			QueryExecutionMs: time.Since(startTime).Milliseconds(),
			HasMore:          hasMore,
			NextCursor:       nextCursor,
		},
	}

	return response, nil
}

// buildNodes converts resource results to Node structs
func (a *Analyzer) buildNodes(resources []resourceResult, latestEvents map[string]*ChangeEventInfo) []Node {
	nodes := make([]Node, 0, len(resources))

	for _, r := range resources {
		node := Node{
			UID:       r.UID,
			Kind:      r.Kind,
			APIGroup:  r.APIGroup,
			Namespace: r.Namespace,
			Name:      r.Name,
			Status:    StatusUnknown,
			Labels:    r.Labels,
		}

		// Attach latest event and derive status from it
		if event, ok := latestEvents[r.UID]; ok {
			node.LatestEvent = event
			// Use the status from the latest change event
			if event.Status != "" {
				node.Status = event.Status
			}
		}

		nodes = append(nodes, node)
	}

	return nodes
}

// buildEdges converts edge results to Edge structs
func (a *Analyzer) buildEdges(edgeResults []edgeResult) []Edge {
	edges := make([]Edge, 0, len(edgeResults))

	for _, e := range edgeResults {
		edge := Edge{
			ID:               e.EdgeID,
			Source:           e.SourceUID,
			Target:           e.TargetUID,
			RelationshipType: e.RelationshipType,
		}
		edges = append(edges, edge)
	}

	return edges
}

// detectAnomalies runs anomaly detection on resources that show signs of issues
func (a *Analyzer) detectAnomalies(
	ctx context.Context,
	resources []resourceResult,
	input AnalyzeInput,
) ([]anomaly.Anomaly, error) {
	var allAnomalies []anomaly.Anomaly
	seen := make(map[string]bool) // Deduplicate by anomaly key

	// Calculate time window for anomaly detection
	lookbackNs := input.Lookback.Nanoseconds()
	startSeconds := (input.Timestamp - lookbackNs) / 1_000_000_000
	endSeconds := input.Timestamp / 1_000_000_000

	// Run anomaly detection on each resource
	// In practice, we might want to be more selective about which resources to analyze
	// (e.g., only Pods, Deployments, etc.)
	candidateKinds := map[string]bool{
		"Pod":         true,
		"Deployment":  true,
		"StatefulSet": true,
		"DaemonSet":   true,
		"ReplicaSet":  true,
		"Job":         true,
		"CronJob":     true,
		"Service":     true,
		"Ingress":     true,
		"Node":        true,
	}

	for _, r := range resources {
		// Only analyze workload-type resources
		if !candidateKinds[r.Kind] {
			continue
		}

		detectInput := anomaly.DetectInput{
			ResourceUID: r.UID,
			Start:       startSeconds,
			End:         endSeconds,
		}

		result, err := a.anomalyDetector.Detect(ctx, detectInput)
		if err != nil {
			a.logger.Debug("Failed to detect anomalies for %s/%s: %v", r.Kind, r.Name, err)
			continue
		}

		// Add unique anomalies
		for _, anom := range result.Anomalies {
			key := fmt.Sprintf("%s:%s:%s:%d", anom.Node.UID, anom.Category, anom.Type, anom.Timestamp.Unix())
			if !seen[key] {
				seen[key] = true
				allAnomalies = append(allAnomalies, anom)
			}
		}
	}

	return allAnomalies, nil
}

// discoverCausalPaths runs causal path discovery for anomalous resources
func (a *Analyzer) discoverCausalPaths(
	ctx context.Context,
	anomalies []anomaly.Anomaly,
	input AnalyzeInput,
) ([]causalpaths.CausalPath, error) {
	var allPaths []causalpaths.CausalPath
	seen := make(map[string]bool) // Deduplicate by path ID

	// Group anomalies by resource UID to avoid duplicate analysis
	resourceUIDs := make(map[string]bool)
	for _, anom := range anomalies {
		resourceUIDs[anom.Node.UID] = true
	}

	lookbackNs := input.Lookback.Nanoseconds()

	// Use CausalPathMaxDepth for path discovery instead of input.MaxDepth
	// Causal paths need to traverse full chains like:
	// HelmRelease -> Deployment -> ReplicaSet -> Pod -> Node (5+ hops)
	// The input.MaxDepth is optimized for resource fetching (default 1) which is too shallow
	maxDepth := CausalPathMaxDepth
	if input.MaxDepth > maxDepth {
		maxDepth = input.MaxDepth // Allow explicit override to go higher
	}

	for uid := range resourceUIDs {
		pathInput := causalpaths.CausalPathsInput{
			ResourceUID:      uid,
			FailureTimestamp: input.Timestamp,
			LookbackNs:       lookbackNs,
			MaxDepth:         maxDepth,
			MaxPaths:         5, // Limit paths per resource
		}

		result, err := a.pathDiscoverer.DiscoverCausalPaths(ctx, pathInput)
		if err != nil {
			a.logger.Debug("Failed to discover causal paths for %s: %v", uid, err)
			continue
		}

		// Add unique paths
		for _, path := range result.Paths {
			if !seen[path.ID] {
				seen[path.ID] = true
				allPaths = append(allPaths, path)
			}
		}
	}

	return allPaths, nil
}
