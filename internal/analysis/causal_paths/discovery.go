package causalpaths

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/analysis/anomaly"
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/logging"
)

const edgeTypeManages = "MANAGES"

// PathDiscoverer discovers and ranks causal paths from root causes to symptoms
type PathDiscoverer struct {
	store              analysisstore.AnalysisStore
	analyzer           *analysis.RootCauseAnalyzer
	anomalyDetector    *anomaly.AnomalyDetector
	ranker             *PathRanker
	explanationBuilder *ExplanationBuilder
	logger             *logging.Logger
}

// NewPathDiscoverer creates a new PathDiscoverer instance
func NewPathDiscoverer(store analysisstore.AnalysisStore) *PathDiscoverer {
	return &PathDiscoverer{
		store:              store,
		analyzer:           analysis.NewRootCauseAnalyzer(store),
		anomalyDetector:    anomaly.NewDetector(store),
		ranker:             NewPathRanker(),
		explanationBuilder: NewExplanationBuilder(),
		logger:             logging.GetLogger("causalpaths.discovery"),
	}
}

// upstreamEdge represents an edge pointing upstream (toward potential root cause)
type upstreamEdge struct {
	TargetNodeID string // The upstream node ID
	Edge         *analysis.GraphEdge
}

// traversalEntry represents a state in the DFS traversal
type traversalEntry struct {
	CurrentNodeID string
	Path          []pathElement
	Depth         int
	VisitedNodes  map[string]bool
}

// pathElement represents a node and its incoming edge in the path
type pathElement struct {
	NodeID string
	Edge   *analysis.GraphEdge // Edge that leads TO this node (nil for root)
}

// DiscoverCausalPaths finds all causal paths from anomalous upstream resources to the symptom
func (d *PathDiscoverer) DiscoverCausalPaths(ctx context.Context, input CausalPathsInput) (*CausalPathsResponse, error) {
	startTime := time.Now()

	// Apply defaults
	if input.LookbackNs == 0 {
		input.LookbackNs = DefaultLookbackNs
	}
	if input.MaxDepth == 0 {
		input.MaxDepth = DefaultMaxDepth
	}
	if input.MaxPaths == 0 {
		input.MaxPaths = DefaultMaxPaths
	}

	d.logger.Debug("DiscoverCausalPaths: resourceUID=%s, failureTimestamp=%d, lookbackNs=%d, maxDepth=%d, maxPaths=%d",
		input.ResourceUID, input.FailureTimestamp, input.LookbackNs, input.MaxDepth, input.MaxPaths)

	// Step 1: Fetch causal subgraph using the existing analyzer
	analyzeInput := analysis.PrepareAnalyzeInput(analysis.AnalyzeInput{
		ResourceUID:      input.ResourceUID,
		FailureTimestamp: input.FailureTimestamp,
		LookbackNs:       input.LookbackNs,
		MaxDepth:         input.MaxDepth,
		MinConfidence:    0.5,
		Format:           analysis.FormatDiff,
	})

	result, err := d.analyzer.Analyze(ctx, analyzeInput)
	if err != nil {
		d.logger.Error("DiscoverCausalPaths: failed to analyze causal graph: %v", err)
		return nil, fmt.Errorf("failed to build causal graph: %w", err)
	}

	causalGraph := result.Incident.Graph
	d.logger.Debug("DiscoverCausalPaths: causal graph has %d nodes and %d edges",
		len(causalGraph.Nodes), len(causalGraph.Edges))

	// Step 2: Build node lookup map
	nodeMap := d.buildNodeMap(causalGraph)

	// Step 3: Detect anomalies for all nodes
	nodeAnomalies, err := d.detectAnomaliesForAllNodes(ctx, causalGraph, input)
	if err != nil {
		d.logger.Warn("DiscoverCausalPaths: failed to detect anomalies: %v", err)
		// Continue without anomalies - paths will still be discovered
		nodeAnomalies = make(map[string][]anomaly.Anomaly)
	}

	// Step 4: Build upstream adjacency map (edges point from child to parent)
	upstreamAdjacency := d.buildUpstreamAdjacency(causalGraph)

	// Step 5: Find symptom node and its first failure time
	symptomNode := d.findSymptomNode(causalGraph, input.ResourceUID)
	if symptomNode == nil {
		d.logger.Error("DiscoverCausalPaths: symptom node not found: %s", input.ResourceUID)
		return nil, fmt.Errorf("symptom node not found: %s", input.ResourceUID)
	}

	symptomFirstFailure := d.identifyFirstFailure(nodeAnomalies[symptomNode.ID], input.FailureTimestamp)
	d.logger.Debug("DiscoverCausalPaths: symptom first failure time: %v", symptomFirstFailure)

	// Step 6: DFS traversal upstream from symptom
	// Special handling for Service symptoms with NoReadyEndpoints anomaly:
	// - Service cannot be traversed upstream via SELECTS (direction is Service → Pod)
	// - Instead, find selected Pods and trace upstream from them
	// - Append Service as the final symptom in each path
	var rawPaths []CausalPath

	if symptomNode.Resource.Kind == "Service" && d.hasNoReadyEndpointsAnomaly(nodeAnomalies[symptomNode.ID]) {
		d.logger.Debug("DiscoverCausalPaths: Service with NoReadyEndpoints - using bidirectional traversal")
		rawPaths = d.traverseFromServiceSymptom(
			ctx,
			symptomNode,
			input,
			nodeAnomalies,
			symptomFirstFailure,
		)
	} else {
		rawPaths = d.traverseUpstream(
			symptomNode,
			upstreamAdjacency,
			nodeMap,
			nodeAnomalies,
			symptomFirstFailure,
			input.MaxDepth,
		)
	}

	d.logger.Debug("DiscoverCausalPaths: discovered %d raw paths", len(rawPaths))

	// Step 7: Rank paths
	rankedPaths := d.ranker.RankPaths(rawPaths, symptomFirstFailure)

	// Step 7.5: Deduplicate paths by root cause
	// Multiple paths may lead to the same root cause (e.g., 5 Pods affected by same Node DiskPressure)
	// Keep only the highest-confidence path for each unique root cause
	dedupedPaths := d.deduplicateByRootCause(rankedPaths)
	d.logger.Debug("DiscoverCausalPaths: deduplicated %d paths to %d unique root causes",
		len(rankedPaths), len(dedupedPaths))

	// Step 8: Take top N paths
	topPaths := dedupedPaths
	if len(topPaths) > input.MaxPaths {
		topPaths = topPaths[:input.MaxPaths]
	}

	// Step 9: Generate explanations
	for i := range topPaths {
		topPaths[i].Explanation = d.explanationBuilder.GenerateExplanation(topPaths[i])
	}

	return &CausalPathsResponse{
		Paths: topPaths,
		Metadata: ResponseMetadata{
			QueryExecutionMs: time.Since(startTime).Milliseconds(),
			AlgorithmVersion: AlgorithmVersion,
			ExecutedAt:       time.Now(),
			NodesExplored:    len(causalGraph.Nodes),
			PathsDiscovered:  len(rawPaths),
			PathsReturned:    len(topPaths),
		},
	}, nil
}
