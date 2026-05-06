package namespacegraph

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/analysis/anomaly"
	causalpaths "github.com/moolen/spectre/internal/analysis/causal_paths"
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/logging"
)

// Analyzer orchestrates namespace graph analysis
type Analyzer struct {
	store           analysisstore.AnalysisStore
	anomalyDetector *anomaly.AnomalyDetector
	pathDiscoverer  *causalpaths.PathDiscoverer
	logger          *logging.Logger
}

// NewAnalyzer creates a new namespace graph Analyzer
func NewAnalyzer(store analysisstore.AnalysisStore) *Analyzer {
	return &Analyzer{
		store:           store,
		anomalyDetector: anomaly.NewDetector(store),
		pathDiscoverer:  causalpaths.NewPathDiscoverer(store),
		logger:          logging.GetLogger("namespacegraph.analyzer"),
	}
}

// Analyze fetches the namespace graph at a point in time with optional enrichment
func (a *Analyzer) Analyze(ctx context.Context, input AnalyzeInput) (*NamespaceGraphResponse, error) {
	startTime := time.Now()

	prepared, err := PrepareAnalyzeInput(input, startTime)
	if err != nil {
		return nil, err
	}
	input = prepared

	a.logger.Debug("Analyzing namespace graph: namespace=%s, timestamp=%d, limit=%d, maxDepth=%d",
		input.Namespace, input.Timestamp, input.Limit, input.MaxDepth)

	storeGraph, err := a.store.GetNamespaceGraph(ctx, analysisstore.NamespaceGraphQuery{
		Namespace:   input.Namespace,
		TimestampNs: input.Timestamp,
		LookbackNs:  input.Lookback.Nanoseconds(),
		MaxDepth:    input.MaxDepth,
		Limit:       input.Limit,
		Cursor:      input.Cursor,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch namespace graph: %w", err)
	}

	nodes := buildNodesFromStore(storeGraph.Graph.Nodes)
	edges := buildEdgesFromStore(storeGraph.Graph.Edges)
	resources := buildResourceResultsFromStore(storeGraph.Graph.Nodes)

	// Step 6: Full-fledged anomaly detection (runs per-resource anomaly analysis)
	var anomalies []anomaly.Anomaly
	if input.IncludeAnomalies {
		anomalies = a.detectAnomalies(ctx, resources, input)
		a.logger.Debug("Detected %d anomalies", len(anomalies))
	}

	// Step 7: Optional causal path discovery (only if anomalies found and requested)
	var causalPaths []causalpaths.CausalPath
	if input.IncludeCausalPaths && len(anomalies) > 0 {
		causalPaths = a.discoverCausalPaths(ctx, anomalies, input)
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
			Namespace:        storeGraph.Metadata.Namespace,
			Timestamp:        storeGraph.Metadata.TimestampNs,
			NodeCount:        storeGraph.Metadata.NodeCount,
			EdgeCount:        storeGraph.Metadata.EdgeCount,
			QueryExecutionMs: time.Since(startTime).Milliseconds(),
			HasMore:          storeGraph.Metadata.HasMore,
			NextCursor:       storeGraph.Metadata.NextCursor,
			Cached:           storeGraph.Metadata.Cached,
			CacheAge:         storeGraph.Metadata.CacheAgeMs,
		},
	}

	return response, nil
}

func buildNodesFromStore(nodes []analysisstore.NamespaceGraphNode) []Node {
	result := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		node := Node{
			UID:       n.UID,
			Kind:      n.Kind,
			APIGroup:  n.APIGroup,
			Namespace: n.Namespace,
			Name:      n.Name,
			Status:    n.Status,
			Labels:    n.Labels,
		}
		if n.LatestEvent != nil {
			node.LatestEvent = &ChangeEventInfo{
				Timestamp:       n.LatestEvent.TimestampNs,
				EventType:       n.LatestEvent.EventType,
				Status:          n.LatestEvent.Status,
				ErrorMessage:    n.LatestEvent.ErrorMessage,
				ContainerIssues: n.LatestEvent.ContainerIssues,
				ImpactScore:     n.LatestEvent.ImpactScore,
				SpecChanges:     n.LatestEvent.SpecChanges,
				SpecReplicas:    n.LatestEvent.SpecReplicas,
			}
		}
		if node.Status == "" {
			node.Status = StatusUnknown
		}
		result = append(result, node)
	}
	return result
}

func buildEdgesFromStore(edges []analysisstore.NamespaceGraphEdge) []Edge {
	result := make([]Edge, 0, len(edges))
	for _, e := range edges {
		result = append(result, Edge{
			ID:               e.ID,
			Source:           e.Source,
			Target:           e.Target,
			RelationshipType: e.RelationshipType,
		})
	}
	return result
}

func buildResourceResultsFromStore(nodes []analysisstore.NamespaceGraphNode) []resourceResult {
	result := make([]resourceResult, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, resourceResult{
			UID:       n.UID,
			Kind:      n.Kind,
			APIGroup:  n.APIGroup,
			Namespace: n.Namespace,
			Name:      n.Name,
			Labels:    n.Labels,
		})
	}
	return result
}

// buildNodes converts resource results to Node structs
// It filters out resources that have been deleted (latest event type is DELETE)
func (a *Analyzer) buildNodes(resources []resourceResult, latestEvents map[string]*ChangeEventInfo) []Node {
	nodes := make([]Node, 0, len(resources))

	for _, r := range resources {
		// Check if the latest event is a DELETE - if so, skip this resource
		// This provides a safety filter in case the r.deleted flag wasn't set correctly
		// (e.g., due to race conditions or incomplete data)
		if event, ok := latestEvents[r.UID]; ok && event.EventType == "DELETE" {
			a.logger.Debug("Skipping deleted resource: %s/%s (latest event is DELETE at %d)",
				r.Kind, r.Name, event.Timestamp)
			continue
		}

		// Also skip if the resource is marked as deleted in the graph
		if r.Deleted {
			a.logger.Debug("Skipping deleted resource: %s/%s (marked as deleted at %d)",
				r.Kind, r.Name, r.DeletedAt)
			continue
		}

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
) []anomaly.Anomaly {
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

	return allAnomalies
}

// discoverCausalPaths runs causal path discovery for anomalous resources
func (a *Analyzer) discoverCausalPaths(
	ctx context.Context,
	anomalies []anomaly.Anomaly,
	input AnalyzeInput,
) []causalpaths.CausalPath {
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

	return allPaths
}
