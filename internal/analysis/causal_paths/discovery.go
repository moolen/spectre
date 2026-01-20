package causalpaths

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/analysis/anomaly"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

const edgeTypeManages = "MANAGES"

// containsIgnoreCase checks if s contains substr (case insensitive)
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// PathDiscoverer discovers and ranks causal paths from root causes to symptoms
type PathDiscoverer struct {
	graphClient        graph.Client
	analyzer           *analysis.RootCauseAnalyzer
	anomalyDetector    *anomaly.AnomalyDetector
	ranker             *PathRanker
	explanationBuilder *ExplanationBuilder
	logger             *logging.Logger
}

// NewPathDiscoverer creates a new PathDiscoverer instance
func NewPathDiscoverer(graphClient graph.Client) *PathDiscoverer {
	return &PathDiscoverer{
		graphClient:        graphClient,
		analyzer:           analysis.NewRootCauseAnalyzer(graphClient),
		anomalyDetector:    anomaly.NewDetector(graphClient),
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
	analyzeInput := analysis.AnalyzeInput{
		ResourceUID:      input.ResourceUID,
		FailureTimestamp: input.FailureTimestamp,
		LookbackNs:       input.LookbackNs,
		MaxDepth:         input.MaxDepth,
		MinConfidence:    0.5,
		Format:           analysis.FormatDiff,
	}

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

// buildNodeMap creates a lookup map from node ID to GraphNode
func (d *PathDiscoverer) buildNodeMap(graph analysis.CausalGraph) map[string]*analysis.GraphNode {
	nodeMap := make(map[string]*analysis.GraphNode, len(graph.Nodes))
	for i := range graph.Nodes {
		nodeMap[graph.Nodes[i].ID] = &graph.Nodes[i]
	}
	return nodeMap
}

// buildUpstreamAdjacency creates an adjacency map for upstream traversal
// Edges point FROM the downstream node TO the upstream node
// This allows traversing from symptom toward root causes
func (d *PathDiscoverer) buildUpstreamAdjacency(graph analysis.CausalGraph) map[string][]upstreamEdge {
	adjacency := make(map[string][]upstreamEdge)

	d.logger.Debug("buildUpstreamAdjacency: processing %d edges", len(graph.Edges))

	for i := range graph.Edges {
		edge := &graph.Edges[i]

		// Determine edge category for proper direction handling
		edgeCategory := ClassifyEdge(edge.RelationshipType)

		d.logger.Debug("buildUpstreamAdjacency: edge %s -> %s (type=%s, category=%s)",
			edge.From, edge.To, edge.RelationshipType, edgeCategory)

		// Different edge types have different causal directions:
		//
		// 1. Materialization edges (OWNS, SCHEDULED_ON, etc.):
		//    Stored as: parent -> child (e.g., Deployment -> ReplicaSet)
		//    Causal direction: child caused by parent (same)
		//    For upstream traversal: reverse (child -> parent)
		//
		// 2. Cause-introducing edges (REFERENCES_SPEC, BINDS_ROLE):
		//    Stored as: consumer -> dependency (e.g., Pod -> ConfigMap, RoleBinding -> Role)
		//    Causal direction: dependency influences consumer (opposite!)
		//    For upstream traversal: DON'T reverse (already correct for causality)
		//
		// 3. MANAGES edges (special case):
		//    Stored as: manager -> managed (e.g., HelmRelease -> Deployment)
		//    Causal direction: manager influences managed (same as stored)
		//    For upstream traversal: REVERSE (managed -> manager)
		//
		// 4. GRANTS_TO edges (special case, similar to MANAGES):
		//    Stored as: RoleBinding -> ServiceAccount
		//    Causal direction: RoleBinding change affects ServiceAccount permissions
		//    For upstream traversal: REVERSE (ServiceAccount -> RoleBinding)

		// Special handling for MANAGES and GRANTS_TO edges
		if edge.RelationshipType == edgeTypeManages || edge.RelationshipType == "GRANTS_TO" {
			// MANAGES is stored as manager -> managed, but we need managed -> manager for upstream
			// GRANTS_TO is stored as RoleBinding -> SA, but we need SA -> RoleBinding for upstream
			d.logger.Debug("buildUpstreamAdjacency: MANAGES/GRANTS_TO edge: adding adjacency[%s] -> %s", edge.To, edge.From)
			adjacency[edge.To] = append(adjacency[edge.To], upstreamEdge{
				TargetNodeID: edge.From,
				Edge:         edge,
			})
		} else if edgeCategory == EdgeCategoryCauseIntroducing {
			// Other cause-introducing edges: stored edge already represents causal direction
			// Pod -> ConfigMap means ConfigMap influences Pod
			// For upstream from Pod, ConfigMap is upstream
			// RoleBinding -> Role means Role influences RoleBinding
			// For upstream from RoleBinding, Role is upstream
			d.logger.Debug("buildUpstreamAdjacency: CauseIntroducing edge: adding adjacency[%s] -> %s", edge.From, edge.To)
			adjacency[edge.From] = append(adjacency[edge.From], upstreamEdge{
				TargetNodeID: edge.To,
				Edge:         edge,
			})
		} else {
			// Materialization edges: reverse the stored direction
			// Deployment -> ReplicaSet stored, need ReplicaSet -> Deployment for upstream
			d.logger.Debug("buildUpstreamAdjacency: Materialization edge: adding adjacency[%s] -> %s", edge.To, edge.From)
			adjacency[edge.To] = append(adjacency[edge.To], upstreamEdge{
				TargetNodeID: edge.From,
				Edge:         edge,
			})
		}
	}

	return adjacency
}

// detectAnomaliesForAllNodes runs anomaly detection for the causal subgraph
func (d *PathDiscoverer) detectAnomaliesForAllNodes(
	ctx context.Context,
	graph analysis.CausalGraph,
	input CausalPathsInput,
) (map[string][]anomaly.Anomaly, error) {
	nodeAnomalies := make(map[string][]anomaly.Anomaly)

	// Convert timestamps for anomaly detection
	startNs := input.FailureTimestamp - input.LookbackNs
	endNs := input.FailureTimestamp

	// Use existing anomaly detection infrastructure
	// This builds the causal subgraph and detects anomalies for ALL nodes in it
	detectInput := anomaly.DetectInput{
		ResourceUID: input.ResourceUID,
		Start:       startNs / 1_000_000_000, // Convert to seconds
		End:         endNs / 1_000_000_000,
	}

	response, err := d.anomalyDetector.Detect(ctx, detectInput)
	if err != nil {
		return nil, err
	}

	// Build a map from resource UID to graph node ID
	uidToNodeID := make(map[string]string)
	for i := range graph.Nodes {
		uidToNodeID[graph.Nodes[i].Resource.UID] = graph.Nodes[i].ID
	}

	// Group anomalies by graph node ID (not anomaly node UID)
	for _, a := range response.Anomalies {
		if nodeID, ok := uidToNodeID[a.Node.UID]; ok {
			nodeAnomalies[nodeID] = append(nodeAnomalies[nodeID], a)
		}
	}

	return nodeAnomalies, nil
}

// findSymptomNode finds the node matching the symptom resource UID
func (d *PathDiscoverer) findSymptomNode(graph analysis.CausalGraph, resourceUID string) *analysis.GraphNode {
	for i := range graph.Nodes {
		if graph.Nodes[i].Resource.UID == resourceUID {
			return &graph.Nodes[i]
		}
	}
	return nil
}

// identifyFirstFailure determines the first failure timestamp for the symptom
func (d *PathDiscoverer) identifyFirstFailure(
	symptomAnomalies []anomaly.Anomaly,
	fallbackTimestamp int64,
) time.Time {
	// Look for the earliest anomaly on the symptom node
	var earliest time.Time

	for _, a := range symptomAnomalies {
		if earliest.IsZero() || a.Timestamp.Before(earliest) {
			earliest = a.Timestamp
		}
	}

	// If no anomalies, use the fallback timestamp
	if earliest.IsZero() {
		return time.Unix(0, fallbackTimestamp)
	}

	return earliest
}

// traverseUpstream performs DFS from symptom toward root causes
func (d *PathDiscoverer) traverseUpstream(
	symptomNode *analysis.GraphNode,
	adjacency map[string][]upstreamEdge,
	nodeMap map[string]*analysis.GraphNode,
	nodeAnomalies map[string][]anomaly.Anomaly,
	symptomFirstFailure time.Time,
	maxDepth int,
) []CausalPath {
	var paths []CausalPath

	d.logger.Debug("traverseUpstream: starting from %s (Kind=%s, Name=%s), maxDepth=%d",
		symptomNode.ID, symptomNode.Resource.Kind, symptomNode.Resource.Name, maxDepth)
	d.logger.Debug("traverseUpstream: adjacency map has %d entries", len(adjacency))
	for nodeID, edges := range adjacency {
		node := nodeMap[nodeID]
		if node != nil {
			d.logger.Debug("traverseUpstream: adjacency[%s (%s/%s)] has %d upstream edges",
				nodeID, node.Resource.Kind, node.Resource.Name, len(edges))
			for _, e := range edges {
				targetNode := nodeMap[e.TargetNodeID]
				if targetNode != nil {
					d.logger.Debug("traverseUpstream:   -> %s (%s/%s) via %s",
						e.TargetNodeID, targetNode.Resource.Kind, targetNode.Resource.Name, e.Edge.RelationshipType)
				}
			}
		}
	}

	// Initialize stack with symptom node
	initialEntry := traversalEntry{
		CurrentNodeID: symptomNode.ID,
		Path: []pathElement{
			{NodeID: symptomNode.ID, Edge: nil},
		},
		Depth:        0,
		VisitedNodes: map[string]bool{symptomNode.ID: true},
	}

	stack := []traversalEntry{initialEntry}

	for len(stack) > 0 {
		// Pop from stack
		entry := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// Check depth limit
		if entry.Depth >= maxDepth {
			continue
		}

		// Get upstream edges
		upstreamEdges := adjacency[entry.CurrentNodeID]

		d.logger.Debug("traverseUpstream: at node %s (Kind=%s), depth=%d, upstreamEdges=%d",
			entry.CurrentNodeID, nodeMap[entry.CurrentNodeID].Resource.Kind, entry.Depth, len(upstreamEdges))

		// Handle dead-end nodes (no upstream edges) that have cause-introducing anomalies
		// This handles cases like StatefulSet -> Pod where StatefulSet is the root cause
		// with no further upstream managers (unlike Deployment which may have HelmRelease)
		if len(upstreamEdges) == 0 && entry.Depth > 0 {
			currentNode := nodeMap[entry.CurrentNodeID]
			currentAnomalies := nodeAnomalies[entry.CurrentNodeID]
			if currentNode != nil && HasCauseIntroducingAnomaly(currentAnomalies, symptomFirstFailure) {
				path := d.buildCausalPath(entry.Path, nodeMap, nodeAnomalies)
				paths = append(paths, path)
			}
			continue
		}

		for _, upstream := range upstreamEdges {
			// Skip already visited nodes (cycle prevention)
			if entry.VisitedNodes[upstream.TargetNodeID] {
				d.logger.Debug("traverseUpstream: skipping already visited node %s", upstream.TargetNodeID)
				continue
			}

			upstreamNode := nodeMap[upstream.TargetNodeID]
			if upstreamNode == nil {
				d.logger.Debug("traverseUpstream: upstreamNode is nil for %s", upstream.TargetNodeID)
				continue
			}

			upstreamAnomalies := nodeAnomalies[upstream.TargetNodeID]

			d.logger.Debug("traverseUpstream: considering upstream %s/%s via %s, anomalies=%d",
				upstreamNode.Resource.Kind, upstreamNode.Resource.Name, upstream.Edge.RelationshipType, len(upstreamAnomalies))

			// Classify edge type
			edgeCategory := ClassifyEdge(upstream.Edge.RelationshipType)

			// Create new path element
			newElement := pathElement{
				NodeID: upstream.TargetNodeID,
				Edge:   upstream.Edge,
			}

			// Build new path (prepending - we're going upstream)
			newPath := append([]pathElement{newElement}, entry.Path...)

			// Check stopping conditions
			// Use context-aware classification to properly handle Evicted (needs Node pressure context)
			// and ImagePullBackOff (needs upstream ImageChanged context)
			hasCauseAnomaly := HasCauseIntroducingAnomalyWithContext(upstreamAnomalies, nodeAnomalies, symptomFirstFailure)
			isCauseEdge := edgeCategory == EdgeCategoryCauseIntroducing
			isMaterializationEdge := edgeCategory == EdgeCategoryMaterialization

			// Determine if this node has a "definitive" cause anomaly (not just ResourceCreated)
			// ResourceCreated on workloads is often an intermediate effect, not the root cause
			hasDefinitiveCauseAnomaly := hasDefinitiveCauseIntroducingAnomalyWithContext(upstreamAnomalies, nodeAnomalies, symptomFirstFailure)

			// Check if there's a MANAGES edge upstream from this node
			// If so, we should NOT stop here even with definitive anomalies - the manager
			// (e.g., HelmRelease, Kustomization) may be the true root cause
			hasUpstreamManager := d.hasManagesEdgeUpstream(upstream.TargetNodeID, adjacency)

			// Check if there's a REFERENCES_SPEC edge upstream from this node
			// If so, we should NOT stop here - a referenced resource (e.g., ConfigMap, Secret)
			// could be the actual root cause (e.g., deleted ConfigMap causing HelmRelease failure)
			hasUpstreamReferencesSpec := d.hasReferencesSpecEdgeUpstream(upstream.TargetNodeID, adjacency)

			// Check if anomalies on this node are likely reconciliation effects from an upstream manager
			// If so, we should continue traversal to find the true root cause (the manager)
			isReconciliationEffect := IsReconciliationEffectAnomaly(upstreamAnomalies, upstreamNode.Resource.Kind, hasUpstreamManager)

			// Determine if this would normally be a stopping point (definitive cause anomaly)
			// We track this separately from shouldStop to handle the REFERENCES_SPEC case
			isDefinitiveStopCandidate := hasDefinitiveCauseAnomaly &&
				(isCauseEdge || (isMaterializationEdge && entry.Depth > 0)) &&
				!hasUpstreamManager &&
				!isReconciliationEffect

			d.logger.Debug("traverseUpstream: %s/%s: hasCause=%v, hasDefinitive=%v, isCauseEdge=%v, hasUpstreamManager=%v, hasUpstreamRefsSpec=%v, isReconEffect=%v, isDefinitiveStop=%v",
				upstreamNode.Resource.Kind, upstreamNode.Resource.Name,
				hasCauseAnomaly, hasDefinitiveCauseAnomaly, isCauseEdge, hasUpstreamManager, hasUpstreamReferencesSpec, isReconciliationEffect, isDefinitiveStopCandidate)

			// Stop if we found a cause-introducing anomaly at this upstream node
			// This can happen via:
			// 1. Cause-introducing edge (direct causal link) with definitive anomaly
			// 2. Materialization edge (ownership/scheduling) when depth > 0 with definitive anomaly
			//
			// We use "definitive" anomalies to avoid stopping at intermediate nodes with only
			// ResourceCreated - we want to continue upstream to find deeper root causes like
			// ConfigMap deletion or HelmRelease changes.
			//
			// IMPORTANT: Don't stop if:
			// - There's a MANAGES edge upstream - the manager (HelmRelease/Kustomization) may be the true root cause
			// - There's a REFERENCES_SPEC edge upstream - a referenced ConfigMap/Secret could be the root cause
			// - The anomalies are reconciliation effects (ImageChanged, SpecModified, etc.) on a managed workload
			//
			// When there are REFERENCES_SPEC edges upstream, we record this as a candidate but continue
			// exploring to find deeper root causes (like a deleted ConfigMap)
			shouldStop := isDefinitiveStopCandidate && !hasUpstreamReferencesSpec

			d.logger.Debug("traverseUpstream: %s/%s: shouldStop=%v", upstreamNode.Resource.Kind, upstreamNode.Resource.Name, shouldStop)

			if shouldStop {
				// STOP: Found candidate root cause
				path := d.buildCausalPath(newPath, nodeMap, nodeAnomalies)
				paths = append(paths, path)
				// Don't explore further from this node
				continue
			}

			// Even if we don't stop (because of REFERENCES_SPEC), record this as a candidate
			// if it has definitive anomalies. The deeper REFERENCES_SPEC target (ConfigMap)
			// may or may not be a better root cause.
			if isDefinitiveStopCandidate && hasUpstreamReferencesSpec {
				path := d.buildCausalPath(newPath, nodeMap, nodeAnomalies)
				paths = append(paths, path)
				// Continue exploring to find deeper root causes via REFERENCES_SPEC
			}

			// Even if we found a non-definitive cause anomaly (like ResourceCreated),
			// record a path but also continue exploring to find deeper root causes
			if hasCauseAnomaly && !hasDefinitiveCauseAnomaly && (isCauseEdge || (isMaterializationEdge && entry.Depth > 0)) {
				// Record this as a candidate path (may not be the deepest root cause)
				path := d.buildCausalPath(newPath, nodeMap, nodeAnomalies)
				paths = append(paths, path)
				// Continue exploring - don't stop here
			}

			// Determine if we should continue traversal
			// IMPORTANT: If we recorded this node as a candidate but want to continue exploring
			// via REFERENCES_SPEC, we MUST force continue even if shouldContinueTraversal returns false
			shouldContinue := d.shouldContinueTraversal(upstreamAnomalies, edgeCategory)

			// Force continue if this is a definitive stop candidate but has REFERENCES_SPEC upstream
			// This ensures we explore the ConfigMap/Secret that might be the true root cause
			if isDefinitiveStopCandidate && hasUpstreamReferencesSpec {
				shouldContinue = true
			}

			d.logger.Debug("traverseUpstream: %s/%s: shouldContinue=%v", upstreamNode.Resource.Kind, upstreamNode.Resource.Name, shouldContinue)

			if shouldContinue {
				// Copy visited set and add new node
				newVisited := make(map[string]bool, len(entry.VisitedNodes)+1)
				for k, v := range entry.VisitedNodes {
					newVisited[k] = v
				}
				newVisited[upstream.TargetNodeID] = true

				stack = append(stack, traversalEntry{
					CurrentNodeID: upstream.TargetNodeID,
					Path:          newPath,
					Depth:         entry.Depth + 1,
					VisitedNodes:  newVisited,
				})
			}
		}
	}

	return paths
}

// hasManagesEdgeUpstream checks if a node has a MANAGES edge pointing upstream
// This indicates the node is managed by a higher-level resource (e.g., HelmRelease, Kustomization)
func (d *PathDiscoverer) hasManagesEdgeUpstream(nodeID string, adjacency map[string][]upstreamEdge) bool {
	for _, upstream := range adjacency[nodeID] {
		if upstream.Edge != nil && upstream.Edge.RelationshipType == edgeTypeManages {
			return true
		}
	}
	return false
}

// hasReferencesSpecEdgeUpstream checks if a node has REFERENCES_SPEC edges pointing upstream
// This indicates the node references configuration resources (e.g., ConfigMap, Secret)
// that could be the actual root cause (e.g., a deleted ConfigMap causing HelmRelease failure)
func (d *PathDiscoverer) hasReferencesSpecEdgeUpstream(nodeID string, adjacency map[string][]upstreamEdge) bool {
	for _, upstream := range adjacency[nodeID] {
		if upstream.Edge != nil && upstream.Edge.RelationshipType == "REFERENCES_SPEC" {
			return true
		}
	}
	return false
}

// deduplicateByRootCause merges paths that share the same root cause resource UID
// When multiple paths lead to the same root cause (e.g., 5 Pods affected by Node DiskPressure),
// we keep only the best path but preserve metadata about all affected symptoms.
//
// The deduplication key is the root cause resource UID (CandidateRoot.Resource.UID).
// For each unique root cause, we keep the path with:
// 1. Longest path (more complete causal chain) - primary criterion
// 2. Highest ConfidenceScore - tie-breaker
// This ensures we show the full propagation chain (e.g., HelmRelease → Deployment → ReplicaSet → Pod)
// rather than truncated paths (e.g., HelmRelease → Deployment).
func (d *PathDiscoverer) deduplicateByRootCause(paths []CausalPath) []CausalPath {
	if len(paths) <= 1 {
		// Even single paths need AffectedCount initialized
		for i := range paths {
			if paths[i].AffectedCount == 0 {
				paths[i].AffectedCount = 1
				paths[i].AffectedSymptoms = []PathNode{d.extractSymptomNode(paths[i])}
			}
		}
		return paths
	}

	// Map from root cause UID to best path and all symptoms for that root cause
	type rootCauseEntry struct {
		bestPath         CausalPath
		affectedSymptoms []PathNode
	}
	entriesByRoot := make(map[string]*rootCauseEntry)

	for _, path := range paths {
		rootUID := path.CandidateRoot.Resource.UID
		if rootUID == "" {
			// No valid root cause UID - include path as-is
			// Use path ID as fallback key to ensure it's not dropped
			rootUID = path.ID
		}

		symptomNode := d.extractSymptomNode(path)

		existing, exists := entriesByRoot[rootUID]
		if !exists {
			// First path for this root cause
			entriesByRoot[rootUID] = &rootCauseEntry{
				bestPath:         path,
				affectedSymptoms: []PathNode{symptomNode},
			}
		} else {
			// Add this symptom to the list
			existing.affectedSymptoms = append(existing.affectedSymptoms, symptomNode)

			// Determine if this path is better than the existing one
			// Primary criterion: longer path (more complete causal chain)
			// Secondary criterion: higher confidence score (tie-breaker)
			existingLen := len(existing.bestPath.Steps)
			newLen := len(path.Steps)

			shouldReplace := false
			if newLen > existingLen {
				// Longer path shows more of the causal chain - prefer it
				shouldReplace = true
				d.logger.Debug("deduplicateByRootCause: replacing path %s (len=%d) with path %s (len=%d) for root %s/%s - longer path preferred",
					existing.bestPath.ID, existingLen,
					path.ID, newLen,
					path.CandidateRoot.Resource.Kind, path.CandidateRoot.Resource.Name)
			} else if newLen == existingLen && path.ConfidenceScore > existing.bestPath.ConfidenceScore {
				// Same length, use confidence as tie-breaker
				shouldReplace = true
				d.logger.Debug("deduplicateByRootCause: replacing path %s (confidence %.3f) with path %s (confidence %.3f) for root %s/%s - higher confidence",
					existing.bestPath.ID, existing.bestPath.ConfidenceScore,
					path.ID, path.ConfidenceScore,
					path.CandidateRoot.Resource.Kind, path.CandidateRoot.Resource.Name)
			}

			if shouldReplace {
				existing.bestPath = path
			}
		}
	}

	// Collect deduplicated paths with affected symptoms metadata
	result := make([]CausalPath, 0, len(entriesByRoot))
	for _, entry := range entriesByRoot {
		path := entry.bestPath
		path.AffectedSymptoms = entry.affectedSymptoms
		path.AffectedCount = len(entry.affectedSymptoms)

		if path.AffectedCount > 1 {
			d.logger.Debug("deduplicateByRootCause: root cause %s/%s affects %d symptoms",
				path.CandidateRoot.Resource.Kind, path.CandidateRoot.Resource.Name,
				path.AffectedCount)
		}

		result = append(result, path)
	}

	// Re-sort by confidence score (map iteration order is non-deterministic)
	sort.Slice(result, func(i, j int) bool {
		return result[i].ConfidenceScore > result[j].ConfidenceScore
	})

	return result
}

// extractSymptomNode extracts the symptom node (last node in the path) from a CausalPath
func (d *PathDiscoverer) extractSymptomNode(path CausalPath) PathNode {
	if len(path.Steps) == 0 {
		return PathNode{}
	}
	// The symptom is the last step in the path (root -> ... -> symptom order)
	return path.Steps[len(path.Steps)-1].Node
}

// shouldContinueTraversal determines if traversal should continue through this node
func (d *PathDiscoverer) shouldContinueTraversal(
	nodeAnomalies []anomaly.Anomaly,
	edgeCategory string,
) bool {
	// Always continue across materialization edges even if anomalous
	// These edges represent ownership/scheduling, not cause introduction
	if edgeCategory == EdgeCategoryMaterialization {
		return true
	}

	// If node has only derived failure anomalies (symptoms, not causes), continue
	if HasOnlyDerivedAnomalies(nodeAnomalies) {
		return true
	}

	// If node has only intermediate cause-introducing anomalies (like ResourceCreated),
	// continue traversal to find deeper root causes
	if hasOnlyIntermediateCauseAnomalies(nodeAnomalies) {
		return true
	}

	// Otherwise, stop here (cause-introducing edge with definitive cause-introducing anomaly)
	return false
}

// buildCausalPath constructs a CausalPath from the traversal path
func (d *PathDiscoverer) buildCausalPath(
	path []pathElement,
	nodeMap map[string]*analysis.GraphNode,
	nodeAnomalies map[string][]anomaly.Anomaly,
) CausalPath {
	if len(path) == 0 {
		return CausalPath{}
	}

	// Build steps (path is already in root -> symptom order since we prepend)
	steps := make([]PathStep, 0, len(path))
	var firstAnomalyAt time.Time

	for i, elem := range path {
		node := nodeMap[elem.NodeID]
		if node == nil {
			continue
		}

		anomalies := nodeAnomalies[elem.NodeID]

		// Track first anomaly timestamp
		for _, a := range anomalies {
			if firstAnomalyAt.IsZero() || a.Timestamp.Before(firstAnomalyAt) {
				firstAnomalyAt = a.Timestamp
			}
		}

		step := PathStep{
			Node: PathNode{
				ID: node.ID,
				Resource: analysis.SymptomResource{
					UID:       node.Resource.UID,
					Kind:      node.Resource.Kind,
					Namespace: node.Resource.Namespace,
					Name:      node.Resource.Name,
				},
				Anomalies:    anomalies,
				PrimaryEvent: node.ChangeEvent,
			},
		}

		// Add edge info (skip for root node which is first in path)
		if i > 0 && elem.Edge != nil {
			edgeCategory := ClassifyEdge(elem.Edge.RelationshipType)
			step.Edge = &PathEdge{
				ID:               elem.Edge.ID,
				RelationshipType: elem.Edge.RelationshipType,
				EdgeCategory:     edgeCategory,
				CausalWeight:     GetCausalWeight(edgeCategory),
			}
		}

		steps = append(steps, step)
	}

	// Create candidate root from first step
	candidateRoot := PathNode{}
	if len(steps) > 0 {
		candidateRoot = steps[0].Node
	}

	// Generate deterministic ID
	pathID := d.generatePathID(path)

	return CausalPath{
		ID:             pathID,
		CandidateRoot:  candidateRoot,
		FirstAnomalyAt: firstAnomalyAt,
		Steps:          steps,
		// ConfidenceScore and Explanation will be set by ranker and explanation builder
	}
}

// generatePathID creates a deterministic ID for a path based on node IDs and edges
func (d *PathDiscoverer) generatePathID(path []pathElement) string {
	var pathStr string
	for _, elem := range path {
		pathStr += elem.NodeID
		if elem.Edge != nil {
			pathStr += "-" + elem.Edge.RelationshipType + "-"
		}
	}

	hash := sha256.Sum256([]byte(pathStr))
	return fmt.Sprintf("path-%x", hash[:8])
}

// hasNoReadyEndpointsAnomaly checks if anomalies include NoReadyEndpoints
func (d *PathDiscoverer) hasNoReadyEndpointsAnomaly(anomalies []anomaly.Anomaly) bool {
	for _, a := range anomalies {
		if a.Type == "NoReadyEndpoints" {
			return true
		}
	}
	return false
}

// traverseFromServiceSymptom handles causal path discovery when the symptom is a Service
// with NoReadyEndpoints anomaly. Since SELECTS edges go Service → Pod (not reversed),
// we need to:
// 1. Find Pods selected by the Service (forward traversal via SELECTS)
// 2. For each selected Pod, trace upstream to find root causes
// 3. Append the Service as the final symptom in each path
func (d *PathDiscoverer) traverseFromServiceSymptom(
	ctx context.Context,
	serviceNode *analysis.GraphNode,
	input CausalPathsInput,
	nodeAnomalies map[string][]anomaly.Anomaly,
	symptomFirstFailure time.Time,
) []CausalPath {
	var paths []CausalPath

	// Query the graph database directly to find Pods selected by this Service
	selectsTargets, err := d.querySelectsTargets(ctx, serviceNode.Resource.UID)
	if err != nil {
		d.logger.Error("traverseFromServiceSymptom: failed to query SELECTS targets: %v", err)
		return []CausalPath{d.buildServiceOnlyPath(serviceNode, nodeAnomalies)}
	}

	d.logger.Debug("traverseFromServiceSymptom: Service %s has %d SELECTS targets",
		serviceNode.Resource.Name, len(selectsTargets))

	if len(selectsTargets) == 0 {
		// No Pods selected - Service might have selector but no matching Pods
		// Create a path with just the Service as both root and symptom
		d.logger.Debug("traverseFromServiceSymptom: No SELECTS targets, returning Service-only path")
		return []CausalPath{d.buildServiceOnlyPath(serviceNode, nodeAnomalies)}
	}

	// Check if all selected Pods are healthy (no failure anomalies)
	// If so, the Service itself is likely the root cause (selector change)
	allPodsHealthy := true
	anyPodAnalyzed := false

	for _, target := range selectsTargets {
		// Build a minimal graph for anomaly detection
		podAnalyzeInput := analysis.AnalyzeInput{
			ResourceUID:      target.uid,
			FailureTimestamp: input.FailureTimestamp,
			LookbackNs:       input.LookbackNs,
			MaxDepth:         1,
			MinConfidence:    0.5,
			Format:           analysis.FormatDiff,
		}

		podResult, err := d.analyzer.Analyze(ctx, podAnalyzeInput)
		if err != nil {
			d.logger.Debug("traverseFromServiceSymptom: failed to analyze Pod %s for health check: %v", target.name, err)
			continue
		}

		podGraph := podResult.Incident.Graph

		// Find the Pod node
		var podNode *analysis.GraphNode
		for i := range podGraph.Nodes {
			if podGraph.Nodes[i].Resource.UID == target.uid {
				podNode = &podGraph.Nodes[i]
				break
			}
		}

		if podNode == nil {
			continue
		}

		anyPodAnalyzed = true

		// Check the Pod's change event status field
		// Status values like "Error", "Warning" indicate failures
		if podNode.ChangeEvent != nil {
			status := podNode.ChangeEvent.Status
			if status == "Error" || status == "Warning" {
				allPodsHealthy = false
				d.logger.Debug("traverseFromServiceSymptom: Pod %s has failure status: %s", target.name, status)
			}

			// Also check description for common failure patterns
			desc := podNode.ChangeEvent.Description
			if desc != "" {
				failurePatterns := []string{"ImagePullBackOff", "CrashLoopBackOff", "ErrImagePull", "OOMKilled", "Error"}
				for _, pattern := range failurePatterns {
					if containsIgnoreCase(desc, pattern) {
						allPodsHealthy = false
						d.logger.Debug("traverseFromServiceSymptom: Pod %s has failure in description: %s", target.name, desc)
						break
					}
				}
			}
		}

		// Check K8s events for failure indicators (Failed, BackOff, etc.)
		if allPodsHealthy {
			for _, k8sEvent := range podNode.K8sEvents {
				if k8sEvent.Type == "Warning" {
					// Check for failure reasons
					failureReasons := []string{"Failed", "BackOff", "ImagePullBackOff", "CrashLoopBackOff", "ErrImagePull", "Evicted", "OOMKilled"}
					for _, reason := range failureReasons {
						if containsIgnoreCase(k8sEvent.Reason, reason) || containsIgnoreCase(k8sEvent.Message, reason) {
							allPodsHealthy = false
							d.logger.Debug("traverseFromServiceSymptom: Pod %s has failure K8s event: %s - %s", target.name, k8sEvent.Reason, k8sEvent.Message)
							break
						}
					}
				}
				if !allPodsHealthy {
					break
				}
			}
		}

		// Also check via anomaly detection
		if allPodsHealthy {
			podAnomalyInput := CausalPathsInput{
				ResourceUID:      target.uid,
				FailureTimestamp: input.FailureTimestamp,
				LookbackNs:       input.LookbackNs,
				MaxDepth:         1,
				MaxPaths:         1,
			}
			podNodeAnomalies, _ := d.detectAnomaliesForAllNodes(ctx, podGraph, podAnomalyInput)
			podAnomalies := podNodeAnomalies[podNode.ID]

			for _, a := range podAnomalies {
				if anomaly.IsPodFailureAnomaly(a.Type) {
					allPodsHealthy = false
					d.logger.Debug("traverseFromServiceSymptom: Pod %s has failure anomaly: %s", target.name, a.Type)
					break
				}
			}
		}

		if !allPodsHealthy {
			break // No need to check more pods
		}
	}

	// If all Pods are healthy but Service has NoReadyEndpoints, Service itself is the root cause
	// This typically happens when the Service selector was changed to not match any Pods
	// Only return Service-only path if we actually analyzed some Pods and found them healthy
	if allPodsHealthy && anyPodAnalyzed && len(selectsTargets) > 0 {
		d.logger.Debug("traverseFromServiceSymptom: All %d Pods are healthy, Service is root cause (likely selector change)", len(selectsTargets))
		return []CausalPath{d.buildServiceOnlyPath(serviceNode, nodeAnomalies)}
	}

	// For each selected Pod, run the analyzer to get its causal graph and build paths
	for _, target := range selectsTargets {
		d.logger.Debug("traverseFromServiceSymptom: Analyzing Pod %s (UID: %s)", target.name, target.uid)

		// Run the analyzer for this Pod to get its causal graph
		podInput := analysis.AnalyzeInput{
			ResourceUID:      target.uid,
			FailureTimestamp: input.FailureTimestamp,
			LookbackNs:       input.LookbackNs,
			MaxDepth:         input.MaxDepth - 1, // Reserve depth for Service
			MinConfidence:    0.5,
			Format:           analysis.FormatDiff,
		}

		podResult, err := d.analyzer.Analyze(ctx, podInput)
		if err != nil {
			d.logger.Warn("traverseFromServiceSymptom: failed to analyze Pod %s: %v", target.name, err)
			continue
		}

		podGraph := podResult.Incident.Graph
		d.logger.Debug("traverseFromServiceSymptom: Pod %s graph has %d nodes and %d edges",
			target.name, len(podGraph.Nodes), len(podGraph.Edges))

		// Build node map for this Pod's graph
		podNodeMap := d.buildNodeMap(podGraph)

		// Detect anomalies for all nodes in this Pod's graph
		// IMPORTANT: Use Pod UID for anomaly detection, not the Service UID
		podAnomalyInput := CausalPathsInput{
			ResourceUID:      target.uid, // Use Pod UID, not Service UID
			FailureTimestamp: input.FailureTimestamp,
			LookbackNs:       input.LookbackNs,
			MaxDepth:         input.MaxDepth,
			MaxPaths:         input.MaxPaths,
		}
		podNodeAnomalies, err := d.detectAnomaliesForAllNodes(ctx, podGraph, podAnomalyInput)
		if err != nil {
			d.logger.Warn("traverseFromServiceSymptom: failed to detect anomalies for Pod graph: %v", err)
			podNodeAnomalies = make(map[string][]anomaly.Anomaly)
		}

		d.logger.Debug("traverseFromServiceSymptom: Pod %s has %d nodes with anomalies",
			target.name, len(podNodeAnomalies))

		// Find the Pod node in its graph
		var podNode *analysis.GraphNode
		for i := range podGraph.Nodes {
			if podGraph.Nodes[i].Resource.UID == target.uid {
				podNode = &podGraph.Nodes[i]
				break
			}
		}

		if podNode == nil {
			d.logger.Warn("traverseFromServiceSymptom: Pod node not found in its own graph: %s", target.uid)
			continue
		}

		// Build upstream adjacency for this Pod's graph
		podUpstreamAdjacency := d.buildUpstreamAdjacency(podGraph)

		// Traverse upstream from Pod
		podPaths := d.traverseUpstream(
			podNode,
			podUpstreamAdjacency,
			podNodeMap,
			podNodeAnomalies,
			symptomFirstFailure,
			input.MaxDepth-1,
		)

		d.logger.Debug("traverseFromServiceSymptom: Found %d paths from Pod %s",
			len(podPaths), target.name)

		// Create a SELECTS edge for the Service → Pod relationship
		selectsEdge := &analysis.GraphEdge{
			From:             serviceNode.ID,
			To:               podNode.ID,
			RelationshipType: "SELECTS",
		}

		// Append Service to each path as the final symptom
		for _, path := range podPaths {
			extendedPath := d.appendServiceToPath(path, serviceNode, selectsEdge, nodeAnomalies)
			paths = append(paths, extendedPath)
		}

		// If no upstream paths were found from Pod, create a Pod → Service path
		if len(podPaths) == 0 {
			podAnomalies := podNodeAnomalies[podNode.ID]
			if HasCauseIntroducingAnomaly(podAnomalies, symptomFirstFailure) || len(podAnomalies) > 0 {
				simplePath := d.buildPodToServicePath(podNode, serviceNode, selectsEdge, nodeAnomalies)
				paths = append(paths, simplePath)
			}
		}
	}

	return paths
}

// selectsTarget represents a Pod selected by a Service
type selectsTarget struct {
	uid       string
	kind      string
	namespace string
	name      string
}

// querySelectsTargets queries the graph database to find Pods selected by a Service
func (d *PathDiscoverer) querySelectsTargets(ctx context.Context, serviceUID string) ([]selectsTarget, error) {
	query := graph.GraphQuery{
		Timeout: 120000,
		Query: `
			MATCH (service:ResourceIdentity {uid: $serviceUID})-[:SELECTS]->(pod:ResourceIdentity)
			WHERE pod.kind = 'Pod' AND (pod.deleted IS NULL OR pod.deleted = false)
			RETURN pod.uid as uid, pod.kind as kind, pod.namespace as namespace, pod.name as name
		`,
		Parameters: map[string]interface{}{
			"serviceUID": serviceUID,
		},
	}

	result, err := d.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query SELECTS targets: %w", err)
	}

	targets := make([]selectsTarget, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) < 4 {
			continue
		}

		uid, _ := row[0].(string)
		kind, _ := row[1].(string)
		namespace, _ := row[2].(string)
		name, _ := row[3].(string)

		if uid == "" {
			continue
		}

		targets = append(targets, selectsTarget{
			uid:       uid,
			kind:      kind,
			namespace: namespace,
			name:      name,
		})
	}

	// Sort for deterministic ordering
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].uid < targets[j].uid
	})

	return targets, nil
}

// buildServiceOnlyPath creates a path with just the Service when there are no selected Pods
func (d *PathDiscoverer) buildServiceOnlyPath(
	serviceNode *analysis.GraphNode,
	nodeAnomalies map[string][]anomaly.Anomaly,
) CausalPath {
	anomalies := nodeAnomalies[serviceNode.ID]

	var firstAnomalyAt time.Time
	for _, a := range anomalies {
		if firstAnomalyAt.IsZero() || a.Timestamp.Before(firstAnomalyAt) {
			firstAnomalyAt = a.Timestamp
		}
	}

	pathNode := PathNode{
		ID: serviceNode.ID,
		Resource: analysis.SymptomResource{
			UID:       serviceNode.Resource.UID,
			Kind:      serviceNode.Resource.Kind,
			Namespace: serviceNode.Resource.Namespace,
			Name:      serviceNode.Resource.Name,
		},
		Anomalies:    anomalies,
		PrimaryEvent: serviceNode.ChangeEvent,
	}

	step := PathStep{
		Node: pathNode,
	}

	// Generate deterministic ID
	hash := sha256.Sum256([]byte(serviceNode.ID))
	pathID := fmt.Sprintf("path-%x", hash[:8])

	return CausalPath{
		ID:             pathID,
		CandidateRoot:  pathNode,
		FirstAnomalyAt: firstAnomalyAt,
		Steps:          []PathStep{step},
	}
}

// appendServiceToPath adds the Service as the final step in a causal path
func (d *PathDiscoverer) appendServiceToPath(
	path CausalPath,
	serviceNode *analysis.GraphNode,
	selectsEdge *analysis.GraphEdge,
	nodeAnomalies map[string][]anomaly.Anomaly,
) CausalPath {
	serviceAnomalies := nodeAnomalies[serviceNode.ID]

	serviceStep := PathStep{
		Node: PathNode{
			ID: serviceNode.ID,
			Resource: analysis.SymptomResource{
				UID:       serviceNode.Resource.UID,
				Kind:      serviceNode.Resource.Kind,
				Namespace: serviceNode.Resource.Namespace,
				Name:      serviceNode.Resource.Name,
			},
			Anomalies:    serviceAnomalies,
			PrimaryEvent: serviceNode.ChangeEvent,
		},
	}

	// Add the SELECTS edge info (reversed direction: Pod → Service in causal direction)
	if selectsEdge != nil {
		edgeCategory := ClassifyEdge(selectsEdge.RelationshipType)
		serviceStep.Edge = &PathEdge{
			ID:               selectsEdge.ID,
			RelationshipType: selectsEdge.RelationshipType,
			EdgeCategory:     edgeCategory,
			CausalWeight:     GetCausalWeight(edgeCategory),
		}
	}

	// Append Service to existing path
	path.Steps = append(path.Steps, serviceStep)

	// Regenerate path ID with Service included
	var pathStr string
	for _, step := range path.Steps {
		pathStr += step.Node.ID
		if step.Edge != nil {
			pathStr += "-" + step.Edge.RelationshipType + "-"
		}
	}
	hash := sha256.Sum256([]byte(pathStr))
	pathID := fmt.Sprintf("path-%x", hash[:8])

	return CausalPath{
		ID:             pathID,
		CandidateRoot:  path.CandidateRoot,
		FirstAnomalyAt: path.FirstAnomalyAt,
		Steps:          path.Steps,
	}
}

// buildPodToServicePath creates a simple path from Pod to Service
func (d *PathDiscoverer) buildPodToServicePath(
	podNode *analysis.GraphNode,
	serviceNode *analysis.GraphNode,
	selectsEdge *analysis.GraphEdge,
	nodeAnomalies map[string][]anomaly.Anomaly,
) CausalPath {
	podAnomalies := nodeAnomalies[podNode.ID]
	serviceAnomalies := nodeAnomalies[serviceNode.ID]

	var firstAnomalyAt time.Time
	for _, a := range podAnomalies {
		if firstAnomalyAt.IsZero() || a.Timestamp.Before(firstAnomalyAt) {
			firstAnomalyAt = a.Timestamp
		}
	}
	for _, a := range serviceAnomalies {
		if firstAnomalyAt.IsZero() || a.Timestamp.Before(firstAnomalyAt) {
			firstAnomalyAt = a.Timestamp
		}
	}

	podPathNode := PathNode{
		ID: podNode.ID,
		Resource: analysis.SymptomResource{
			UID:       podNode.Resource.UID,
			Kind:      podNode.Resource.Kind,
			Namespace: podNode.Resource.Namespace,
			Name:      podNode.Resource.Name,
		},
		Anomalies:    podAnomalies,
		PrimaryEvent: podNode.ChangeEvent,
	}

	servicePathNode := PathNode{
		ID: serviceNode.ID,
		Resource: analysis.SymptomResource{
			UID:       serviceNode.Resource.UID,
			Kind:      serviceNode.Resource.Kind,
			Namespace: serviceNode.Resource.Namespace,
			Name:      serviceNode.Resource.Name,
		},
		Anomalies:    serviceAnomalies,
		PrimaryEvent: serviceNode.ChangeEvent,
	}

	steps := []PathStep{
		{Node: podPathNode},
		{Node: servicePathNode},
	}

	// Add edge info to Service step
	if selectsEdge != nil {
		edgeCategory := ClassifyEdge(selectsEdge.RelationshipType)
		steps[1].Edge = &PathEdge{
			ID:               selectsEdge.ID,
			RelationshipType: selectsEdge.RelationshipType,
			EdgeCategory:     edgeCategory,
			CausalWeight:     GetCausalWeight(edgeCategory),
		}
	}

	pathStr := podNode.ID + "-SELECTS-" + serviceNode.ID
	hash := sha256.Sum256([]byte(pathStr))
	pathID := fmt.Sprintf("path-%x", hash[:8])

	return CausalPath{
		ID:             pathID,
		CandidateRoot:  podPathNode,
		FirstAnomalyAt: firstAnomalyAt,
		Steps:          steps,
	}
}
