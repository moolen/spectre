package analysis

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"golang.org/x/sync/errgroup"
)

// buildCausalGraph constructs the causal graph from symptom to root cause.
// Uses split queries for better performance and maintainability.
//
// Error Handling:
// - If ownership chain query fails, returns error (cannot proceed without chain)
// - If any parallel query fails, returns error (fail-fast approach)
// - Individual query timeouts are handled by the query functions
//
// Performance: Executes 5 graph queries total:
// 1. Ownership chain (sequential)
// 2. Managers, Related resources, Change events, K8s events (parallel)
// 3. Manager events, Related events (sequential)
func (a *RootCauseAnalyzer) buildCausalGraph(
	ctx context.Context,
	symptom *ObservedSymptom,
	failureTimestamp int64,
	lookbackNs int64,
) (CausalGraph, error) {
	queryStart := time.Now()

	// Step 1: Get ownership chain (must succeed first)
	a.logger.Debug("buildCausalGraph: getting ownership chain for symptom %s", symptom.Resource.UID)
	chainStart := time.Now()
	chain, err := a.getOwnershipChain(ctx, symptom.Resource.UID)
	chainDuration := time.Since(chainStart)

	if err != nil {
		a.logger.Error("buildCausalGraph: ownership chain query failed after %v: %v", chainDuration, err)
		return CausalGraph{}, fmt.Errorf("failed to get ownership chain: %w", err)
	}

	a.logger.Debug("buildCausalGraph: ownership chain query completed in %v", chainDuration)
	if chainDuration.Milliseconds() > SlowQueryThresholdMs {
		a.logger.Warn("buildCausalGraph: slow ownership chain query: %v (threshold: %dms)",
			chainDuration, SlowQueryThresholdMs)
	}

	// Collect all resource UIDs for batch queries
	resourceUIDs := extractUIDs(chain)
	a.logger.Debug("buildCausalGraph: found %d resources in chain", len(resourceUIDs))

	// Step 2-5: Execute remaining queries in parallel with fail-fast
	// First get managers, then query for related resources including managers
	managers, err := a.getManagers(ctx, resourceUIDs)
	if err != nil {
		return CausalGraph{}, fmt.Errorf("failed to get managers: %w", err)
	}

	a.logger.Debug("buildCausalGraph: found %d managers", len(managers))

	// Collect all UIDs (chain + managers) for related resources query
	allUIDs := append([]string{}, resourceUIDs...)
	for managerUID, mgr := range managers {
		a.logger.Debug("buildCausalGraph: found manager %s/%s (UID: %s) for resource %s",
			mgr.Manager.Kind, mgr.Manager.Name, mgr.Manager.UID, managerUID)

		// Add manager UID if not already in the list
		found := false
		for _, uid := range resourceUIDs {
			if uid == mgr.Manager.UID {
				found = true
				break
			}
		}
		if !found {
			a.logger.Debug("buildCausalGraph: adding manager UID %s to query list", mgr.Manager.UID)
			allUIDs = append(allUIDs, mgr.Manager.UID)
		} else {
			a.logger.Debug("buildCausalGraph: manager UID %s already in chain", mgr.Manager.UID)
		}
	}
	a.logger.Debug("buildCausalGraph: querying for related resources of %d resources (chain + managers): %v", len(allUIDs), allUIDs)

	// Now run the remaining queries in parallel
	a.logger.Debug("buildCausalGraph: executing parallel queries (related resources, change events, k8s events)")
	parallelStart := time.Now()
	g, gctx := errgroup.WithContext(ctx)
	var related map[string][]RelatedResourceData
	var changeEvents map[string][]ChangeEventInfo
	var k8sEvents map[string][]K8sEventInfo

	g.Go(func() error {
		start := time.Now()
		var err error
		related, err = a.getRelatedResources(gctx, allUIDs)
		duration := time.Since(start)
		if err != nil {
			a.logger.Error("buildCausalGraph: related resources query failed after %v: %v", duration, err)
		} else {
			a.logger.Debug("buildCausalGraph: related resources query completed in %v (%d resources)", duration, len(related))
		}
		return err
	})

	g.Go(func() error {
		start := time.Now()
		var err error
		changeEvents, err = a.getChangeEvents(gctx, resourceUIDs, failureTimestamp, lookbackNs)
		duration := time.Since(start)
		if err != nil {
			a.logger.Error("buildCausalGraph: change events query failed after %v: %v", duration, err)
		} else {
			a.logger.Debug("buildCausalGraph: change events query completed in %v (%d resources)", duration, len(changeEvents))
		}
		return err
	})

	g.Go(func() error {
		start := time.Now()
		var err error
		k8sEvents, err = a.getK8sEvents(gctx, resourceUIDs, failureTimestamp, lookbackNs)
		duration := time.Since(start)
		if err != nil {
			a.logger.Error("buildCausalGraph: k8s events query failed after %v: %v", duration, err)
		} else {
			a.logger.Debug("buildCausalGraph: k8s events query completed in %v (%d resources)", duration, len(k8sEvents))
		}
		return err
	})

	// Fail fast: if any query fails, return immediately
	if err := g.Wait(); err != nil {
		a.logger.Error("buildCausalGraph: parallel queries failed after %v: %v", time.Since(parallelStart), err)
		return CausalGraph{}, fmt.Errorf("failed to build causal graph: %w", err)
	}

	parallelDuration := time.Since(parallelStart)
	a.logger.Debug("buildCausalGraph: all parallel queries completed in %v", parallelDuration)

	// Step 5b: Get events for related resources (managers and related)
	// Collect manager UIDs
	managerUIDs := []string{}
	for _, mgr := range managers {
		managerUIDs = append(managerUIDs, mgr.Manager.UID)
	}
	if len(managerUIDs) > 0 {
		managerEvents, err := a.getChangeEvents(ctx, managerUIDs, failureTimestamp, lookbackNs)
		if err != nil {
			return CausalGraph{}, fmt.Errorf("failed to get manager events: %w", err)
		}
		// Merge manager events into the main changeEvents map
		for uid, events := range managerEvents {
			changeEvents[uid] = events
			a.logger.Debug("buildCausalGraph: merged %d events for manager %s", len(events), uid)
		}
	}

	// Get events for related resources
	if err := a.getChangeEventsForRelated(ctx, related, failureTimestamp, lookbackNs); err != nil {
		return CausalGraph{}, fmt.Errorf("failed to get related resource events: %w", err)
	}

	queryDuration := time.Since(queryStart)
	a.logger.Debug("buildCausalGraph: all queries completed in %v", queryDuration)

	// Step 6: Merge results into CausalGraph
	return a.mergeIntoCausalGraph(symptom, chain, managers, related, changeEvents, k8sEvents, failureTimestamp)
}

// spineNodeBuildResult contains the output of building SPINE nodes
type spineNodeBuildResult struct {
	nodes          []GraphNode
	nodeMap        map[string]string // resource.UID -> node.ID
	seenResources  map[string]bool
	nextStepNumber int
}

// buildSpineNodes creates all SPINE nodes (resources in the ownership chain and their managers)
// This is STEP 1 of graph building: establishing the main causal chain
func (a *RootCauseAnalyzer) buildSpineNodes(
	symptom *ObservedSymptom,
	sortedChain []ResourceWithDistance,
	managers map[string]*ManagerData,
	changeEvents map[string][]ChangeEventInfo,
	k8sEvents map[string][]K8sEventInfo,
	failureTimestamp int64,
) spineNodeBuildResult {
	nodes := []GraphNode{}
	nodeMap := make(map[string]string)
	seenResources := make(map[string]bool)
	stepNumber := 1

	for _, rwd := range sortedChain {
		resource := rwd.Resource

		// Skip if we've already seen this resource
		if seenResources[resource.UID] {
			continue
		}
		seenResources[resource.UID] = true

		// Get events for this resource
		events := changeEvents[resource.UID]
		k8sEvts := k8sEvents[resource.UID]

		// Get manager info
		mgrData := managers[resource.UID]
		var manager *graph.ResourceIdentity
		var managesEdge *graph.ManagesEdge
		if mgrData != nil {
			manager = &mgrData.Manager
			managesEdge = &mgrData.ManagesEdge
		}

		// Select primary event from collected events
		primaryEvent := selectPrimaryEvent(events, failureTimestamp)

		// Determine relationship type for reasoning
		var relationshipType string
		if resource.UID != symptom.Resource.UID {
			relationshipType = "OWNS"
		} else {
			relationshipType = "SYMPTOM"
		}
		if manager != nil {
			relationshipType = "MANAGED_BY"
		}

		// Generate reasoning for this node
		reasoning := generateStepReasoning(resource, manager, managesEdge, primaryEvent, relationshipType)

		// Create node ID and add to map
		nodeID := createNodeID(resource.UID)
		nodeMap[resource.UID] = nodeID

		// Create SPINE node using factory function
		nodes = append(nodes, createSpineNode(
			nodeID,
			resourceIdentityToSymptomResource(resource),
			primaryEvent,
			events,
			k8sEvts,
			stepNumber,
			reasoning,
		))
		stepNumber++

		// If this resource has a manager, add the manager as a separate node
		if manager != nil && !seenResources[manager.UID] {
			seenResources[manager.UID] = true

			// Get manager events
			managerEvents := changeEvents[manager.UID]
			managerPrimaryEvent := selectPrimaryEvent(managerEvents, failureTimestamp)

			managerNodeID := createNodeID(manager.UID)
			nodeMap[manager.UID] = managerNodeID

			confidence := 0.0
			if managesEdge != nil {
				confidence = managesEdge.Confidence
			}

			// Create manager SPINE node using factory function
			nodes = append(nodes, createSpineNode(
				managerNodeID,
				resourceIdentityToSymptomResource(*manager),
				managerPrimaryEvent,
				managerEvents,
				nil, // No K8s events for managers
				stepNumber,
				fmt.Sprintf("%s manages %s lifecycle (confidence: %.0f%%)",
					manager.Kind, resource.Kind, confidence*100),
			))
			stepNumber++
		}
	}

	return spineNodeBuildResult{
		nodes:          nodes,
		nodeMap:        nodeMap,
		seenResources:  seenResources,
		nextStepNumber: stepNumber,
	}
}

// buildSpineEdges creates SPINE edges (OWNS and MANAGES relationships in the main chain)
// This is STEP 2 of graph building: connecting the causal chain nodes
func (a *RootCauseAnalyzer) buildSpineEdges(
	symptom *ObservedSymptom,
	sortedChain []ResourceWithDistance,
	managers map[string]*ManagerData,
	nodeMap map[string]string,
) []GraphEdge {
	edges := []GraphEdge{}
	edgeSet := make(map[string]bool) // Deduplicate edges

	// Build a map of which resources are owned (to avoid duplicate MANAGES edges)
	ownedResources := make(map[string]bool)
	for idx, rwd := range sortedChain {
		if rwd.Resource.UID != symptom.Resource.UID {
			targetDistance := rwd.Distance - 1
			for nextIdx := idx + 1; nextIdx < len(sortedChain); nextIdx++ {
				if sortedChain[nextIdx].Distance == targetDistance {
					ownedResources[sortedChain[nextIdx].Resource.UID] = true
					break
				}
			}
		}
	}

	// Create edges for each resource in the chain
	for idx, rwd := range sortedChain {
		resource := rwd.Resource
		fromNodeID := nodeMap[resource.UID]
		if fromNodeID == "" {
			continue
		}

		// OWNS edge to next resource in chain (parent -> child)
		if resource.UID != symptom.Resource.UID {
			targetDistance := rwd.Distance - 1
			for nextIdx := idx + 1; nextIdx < len(sortedChain); nextIdx++ {
				if sortedChain[nextIdx].Distance == targetDistance {
					nextUID := sortedChain[nextIdx].Resource.UID
					toNodeID := nodeMap[nextUID]
					if toNodeID != "" {
						edgeID := createSpineEdgeID(resource.UID, nextUID)
						if !edgeSet[edgeID] {
							edges = append(edges, GraphEdge{
								ID:               edgeID,
								From:             fromNodeID,
								To:               toNodeID,
								RelationshipType: "OWNS",
								EdgeType:         "SPINE",
							})
							edgeSet[edgeID] = true
						}
					}
					break
				}
			}
		}

		// MANAGES edge from manager to resource (only if not already owned)
		mgrData := managers[resource.UID]
		if mgrData != nil && !ownedResources[resource.UID] {
			managerNodeID := nodeMap[mgrData.Manager.UID]
			if managerNodeID != "" {
				edgeID := createSpineEdgeID(mgrData.Manager.UID, resource.UID)
				if !edgeSet[edgeID] {
					edges = append(edges, GraphEdge{
						ID:               edgeID,
						From:             managerNodeID,
						To:               fromNodeID,
						RelationshipType: "MANAGES",
						EdgeType:         "SPINE",
					})
					edgeSet[edgeID] = true
				}
			}
		}
	}

	return edges
}

// relatedGraphBuildResult contains the output of building RELATED nodes and edges
type relatedGraphBuildResult struct {
	nodes []GraphNode
	edges []GraphEdge
}

// buildRelatedGraph creates RELATED nodes and ATTACHMENT edges for supporting resources
// This is STEP 3 of graph building: adding context nodes like Nodes, ServiceAccounts, etc.
func (a *RootCauseAnalyzer) buildRelatedGraph(
	sortedChain []ResourceWithDistance,
	managers map[string]*ManagerData,
	related map[string][]RelatedResourceData,
	nodeMap map[string]string, // Will be modified to add new nodes
) relatedGraphBuildResult {
	nodes := []GraphNode{}
	edges := []GraphEdge{}
	edgeSet := make(map[string]bool) // Deduplicate edges

	// Collect all resource UIDs that may have related resources
	resourcesWithRelated := []string{}
	for _, rwd := range sortedChain {
		resourcesWithRelated = append(resourcesWithRelated, rwd.Resource.UID)
	}

	// Also add manager UIDs (note: managers map is keyed by managed resource UID)
	for _, mgrData := range managers {
		managerUID := mgrData.Manager.UID
		found := false
		for _, uid := range resourcesWithRelated {
			if uid == managerUID {
				found = true
				break
			}
		}
		if !found {
			a.logger.Debug("buildRelatedGraph: adding manager %s/%s (UID: %s) to resources with related",
				mgrData.Manager.Kind, mgrData.Manager.Name, managerUID)
			resourcesWithRelated = append(resourcesWithRelated, managerUID)
		}
	}

	// Process each resource's related resources
	for _, resourceUID := range resourcesWithRelated {
		parentNodeID := nodeMap[resourceUID]
		if parentNodeID == "" {
			a.logger.Debug("buildRelatedGraph: skipping related resources for %s - no node in map", resourceUID)
			continue
		}

		relatedList := related[resourceUID]
		a.logger.Debug("buildRelatedGraph: processing %d related resources for %s", len(relatedList), resourceUID)

		for _, relData := range relatedList {
			hasChanges := len(relData.Events) > 0

			// Filter: only include certain relationship types without changes
			// Always include SCHEDULED_ON, GRANTS_TO, REFERENCES_SPEC, and INGRESS_REF
			// These are important for understanding configuration dependencies
			if relData.RelationshipType != "SCHEDULED_ON" &&
				relData.RelationshipType != "GRANTS_TO" &&
				relData.RelationshipType != "REFERENCES_SPEC" &&
				relData.RelationshipType != "INGRESS_REF" &&
				!hasChanges {
				a.logger.Debug("buildRelatedGraph: skipping %s (type=%s) - no changes", relData.Resource.Name, relData.RelationshipType)
				continue
			}

			if relData.RelationshipType == "REFERENCES_SPEC" || relData.RelationshipType == "INGRESS_REF" {
				a.logger.Debug("buildRelatedGraph: including %s %s/%s (hasChanges=%v)",
					relData.RelationshipType, relData.Resource.Kind, relData.Resource.Name, hasChanges)
			}

			relatedUID := relData.Resource.UID

			// Create or get node for related resource
			relatedNodeID := nodeMap[relatedUID]
			if relatedNodeID == "" {
				relatedNodeID = createNodeID(relatedUID)
				nodeMap[relatedUID] = relatedNodeID

				// Create RELATED node using factory function
				nodes = append(nodes, createRelatedNode(
					relatedNodeID,
					resourceIdentityToSymptomResource(relData.Resource),
					relData.Events,
				))
			}

			// Create attachment edge based on relationship type
			if relData.RelationshipType == "GRANTS_TO" {
				// Special case: GRANTS_TO connects RoleBinding to ServiceAccount
				// Find the ServiceAccount node
				var serviceAccountNodeID string
				for _, otherRelData := range relatedList {
					if otherRelData.RelationshipType == "USES_SERVICE_ACCOUNT" {
						serviceAccountNodeID = nodeMap[otherRelData.Resource.UID]
						break
					}
				}

				if serviceAccountNodeID != "" {
					edgeID := createAttachmentEdgeID(relatedNodeID, serviceAccountNodeID)
					if !edgeSet[edgeID] {
						edges = append(edges, GraphEdge{
							ID:               edgeID,
							From:             relatedNodeID,
							To:               serviceAccountNodeID,
							RelationshipType: "GRANTS_TO",
							EdgeType:         "ATTACHMENT",
						})
						edgeSet[edgeID] = true
					}
				}
			} else {
				// Determine edge direction based on relationship type
				var fromNode, toNode string

				if relData.RelationshipType == "SELECTS" {
					// Reverse direction: selector (Service/NetworkPolicy) -> resource (Pod)
					fromNode = relatedNodeID
					toNode = parentNodeID
				} else if relData.RelationshipType == "INGRESS_REF" {
					// Ingress -> Service: use the ReferenceTargetUID
					fromNode = relatedNodeID // Ingress
					if relData.ReferenceTargetUID != "" {
						toNode = nodeMap[relData.ReferenceTargetUID]
						if toNode == "" {
							a.logger.Debug("buildRelatedGraph: skipping INGRESS_REF edge - Service node not found for UID %s",
								relData.ReferenceTargetUID)
							continue
						}
					} else {
						a.logger.Debug("buildRelatedGraph: skipping INGRESS_REF edge - no ReferenceTargetUID")
						continue
					}
				} else {
					// Normal direction: resource -> related (SCHEDULED_ON, USES_SERVICE_ACCOUNT, REFERENCES_SPEC)
					fromNode = parentNodeID
					toNode = relatedNodeID
				}

				edgeID := createAttachmentEdgeID(fromNode, toNode)
				if !edgeSet[edgeID] {
					// Map internal relationship types to canonical ones
					relType := relData.RelationshipType
					if relType == "INGRESS_REF" {
						relType = "REFERENCES_SPEC"
					}

					edges = append(edges, GraphEdge{
						ID:               edgeID,
						From:             fromNode,
						To:               toNode,
						RelationshipType: relType,
						EdgeType:         "ATTACHMENT",
					})
					edgeSet[edgeID] = true
				}
			}
		}
	}

	return relatedGraphBuildResult{
		nodes: nodes,
		edges: edges,
	}
}

// mergeIntoCausalGraph combines the split query results into a CausalGraph
func (a *RootCauseAnalyzer) mergeIntoCausalGraph(
	symptom *ObservedSymptom,
	chain []ResourceWithDistance,
	managers map[string]*ManagerData,
	related map[string][]RelatedResourceData,
	changeEvents map[string][]ChangeEventInfo,
	k8sEvents map[string][]K8sEventInfo,
	failureTimestamp int64,
) (CausalGraph, error) {
	// Build graph structure: edges will be built in separate steps
	edges := []GraphEdge{}

	// Sort chain by distance DESC (furthest from symptom = root cause side)
	sortedChain := make([]ResourceWithDistance, len(chain))
	copy(sortedChain, chain)
	for i := 0; i < len(sortedChain); i++ {
		for j := i + 1; j < len(sortedChain); j++ {
			if sortedChain[j].Distance > sortedChain[i].Distance {
				sortedChain[i], sortedChain[j] = sortedChain[j], sortedChain[i]
			}
		}
	}

	// STEP 1: Create all SPINE nodes
	spineResult := a.buildSpineNodes(symptom, sortedChain, managers, changeEvents, k8sEvents, failureTimestamp)
	nodes := spineResult.nodes
	nodeMap := spineResult.nodeMap

	// STEP 2: Create SPINE edges (main chain relationships)
	spineEdges := a.buildSpineEdges(symptom, sortedChain, managers, nodeMap)
	edges = append(edges, spineEdges...)

	// STEP 3: Create RELATED nodes and ATTACHMENT edges
	relatedResult := a.buildRelatedGraph(sortedChain, managers, related, nodeMap)
	nodes = append(nodes, relatedResult.nodes...)
	edges = append(edges, relatedResult.edges...)

	a.logger.Debug("Built causal graph with %d nodes and %d edges", len(nodes), len(edges))
	return CausalGraph{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// selectPrimaryEvent chooses the most relevant event from a collection
// Priority: configChanged > CREATE > statusChanged (closest to failure)
func selectPrimaryEvent(events []ChangeEventInfo, failureTimestamp int64) *ChangeEventInfo {
	if len(events) == 0 {
		return nil
	}

	// Priority 1: configChanged=true events (earliest timestamp = trigger)
	var earliestConfigChange *ChangeEventInfo
	for i := range events {
		if events[i].ConfigChanged {
			if earliestConfigChange == nil || events[i].Timestamp.Before(earliestConfigChange.Timestamp) {
				earliestConfigChange = &events[i]
			}
		}
	}
	if earliestConfigChange != nil {
		return earliestConfigChange
	}

	// Priority 2: CREATE events
	var earliestCreate *ChangeEventInfo
	for i := range events {
		if events[i].EventType == "CREATE" {
			if earliestCreate == nil || events[i].Timestamp.Before(earliestCreate.Timestamp) {
				earliestCreate = &events[i]
			}
		}
	}
	if earliestCreate != nil {
		return earliestCreate
	}

	// Priority 3: Status changes closest to failure
	var closestStatus *ChangeEventInfo
	minDelta := int64(1<<63 - 1) // Max int64
	for i := range events {
		if events[i].StatusChanged {
			delta := failureTimestamp - events[i].Timestamp.UnixNano()
			if delta < 0 {
				delta = -delta
			}
			if delta < minDelta {
				minDelta = delta
				closestStatus = &events[i]
			}
		}
	}
	if closestStatus != nil {
		return closestStatus
	}

	// Fallback: earliest event
	earliest := &events[0]
	for i := range events {
		if events[i].Timestamp.Before(earliest.Timestamp) {
			earliest = &events[i]
		}
	}
	return earliest
}

// generateStepReasoning creates a human-readable explanation for a causal step
func generateStepReasoning(
	resource graph.ResourceIdentity,
	manager *graph.ResourceIdentity,
	managesEdge *graph.ManagesEdge,
	changeEvent *ChangeEventInfo,
	relationshipType string,
) string {
	switch relationshipType {
	case "MANAGES":
		if manager != nil {
			confidence := 0.0
			if managesEdge != nil {
				confidence = managesEdge.Confidence
			}
			return fmt.Sprintf("%s manages %s lifecycle (confidence: %.0f%%)",
				manager.Kind, resource.Kind, confidence*100)
		}
		return "Lifecycle management relationship"

	case "MANAGED_BY":
		if manager != nil {
			confidence := 0.0
			if managesEdge != nil {
				confidence = managesEdge.Confidence
			}
			return fmt.Sprintf("Managed by %s (confidence: %.0f%%)",
				manager.Kind, confidence*100)
		}
		return "Managed resource"

	case "OWNS":
		return fmt.Sprintf("%s owns resources in the next layer", resource.Kind)

	case "SYMPTOM":
		if changeEvent != nil && changeEvent.ConfigChanged {
			return "Configuration change triggered the failure"
		}
		return "Observed failure symptom"

	default:
		if changeEvent != nil {
			return fmt.Sprintf("%s occurred in this resource", changeEvent.EventType)
		}
		return "Part of the causal chain"
	}
}
