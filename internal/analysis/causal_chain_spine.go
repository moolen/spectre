package analysis

import (
	"fmt"

	"github.com/moolen/spectre/internal/graph"
)

// spineNodeBuildResult contains the output of building SPINE nodes.
type spineNodeBuildResult struct {
	nodes          []GraphNode
	nodeMap        map[string]string
	seenResources  map[string]bool
	nextStepNumber int
}

// buildSpineNodes creates all SPINE nodes.
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
		if seenResources[resource.UID] {
			continue
		}
		seenResources[resource.UID] = true

		events := changeEvents[resource.UID]
		k8sEvts := k8sEvents[resource.UID]

		mgrData := managers[resource.UID]
		var manager *graph.ResourceIdentity
		var managesEdge *graph.ManagesEdge
		if mgrData != nil {
			manager = &mgrData.Manager
			managesEdge = &mgrData.ManagesEdge
		}

		primaryEvent := selectPrimaryEvent(events, failureTimestamp)
		reasoning := generateStepReasoning(resource, manager, managesEdge, primaryEvent, spineRelationshipType(symptom, resource, manager))

		nodeID := createNodeID(resource.UID)
		nodeMap[resource.UID] = nodeID
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

		if manager == nil || seenResources[manager.UID] {
			continue
		}

		seenResources[manager.UID] = true
		managerEvents := changeEvents[manager.UID]
		managerPrimaryEvent := selectPrimaryEvent(managerEvents, failureTimestamp)
		managerNodeID := createNodeID(manager.UID)
		nodeMap[manager.UID] = managerNodeID

		confidence := 0.0
		if managesEdge != nil {
			confidence = managesEdge.Confidence
		}

		nodes = append(nodes, createSpineNode(
			managerNodeID,
			resourceIdentityToSymptomResource(*manager),
			managerPrimaryEvent,
			managerEvents,
			nil,
			stepNumber,
			fmt.Sprintf("%s manages %s lifecycle (confidence: %.0f%%)",
				manager.Kind, resource.Kind, confidence*100),
		))
		stepNumber++
	}

	return spineNodeBuildResult{
		nodes:          nodes,
		nodeMap:        nodeMap,
		seenResources:  seenResources,
		nextStepNumber: stepNumber,
	}
}

// buildSpineEdges creates SPINE edges.
func (a *RootCauseAnalyzer) buildSpineEdges(
	symptom *ObservedSymptom,
	sortedChain []ResourceWithDistance,
	managers map[string]*ManagerData,
	nodeMap map[string]string,
) []GraphEdge {
	edges := []GraphEdge{}
	edgeSet := make(map[string]bool)
	ownedResources := collectOwnedResources(symptom, sortedChain)

	for idx, rwd := range sortedChain {
		resource := rwd.Resource
		fromNodeID := nodeMap[resource.UID]
		if fromNodeID == "" {
			continue
		}

		if ownsEdge := buildOwnsSpineEdge(symptom, sortedChain, nodeMap, idx, resource, fromNodeID); ownsEdge != nil && !edgeSet[ownsEdge.ID] {
			edges = append(edges, *ownsEdge)
			edgeSet[ownsEdge.ID] = true
		}

		if managesEdge := buildManagesSpineEdge(resource, fromNodeID, managers, nodeMap, ownedResources); managesEdge != nil && !edgeSet[managesEdge.ID] {
			edges = append(edges, *managesEdge)
			edgeSet[managesEdge.ID] = true
		}
	}

	return edges
}

func collectOwnedResources(symptom *ObservedSymptom, sortedChain []ResourceWithDistance) map[string]bool {
	ownedResources := make(map[string]bool)
	for idx, rwd := range sortedChain {
		if rwd.Resource.UID == symptom.Resource.UID {
			continue
		}
		targetDistance := rwd.Distance - 1
		for nextIdx := idx + 1; nextIdx < len(sortedChain); nextIdx++ {
			if sortedChain[nextIdx].Distance == targetDistance {
				ownedResources[sortedChain[nextIdx].Resource.UID] = true
				break
			}
		}
	}
	return ownedResources
}

func buildOwnsSpineEdge(
	symptom *ObservedSymptom,
	sortedChain []ResourceWithDistance,
	nodeMap map[string]string,
	idx int,
	resource graph.ResourceIdentity,
	fromNodeID string,
) *GraphEdge {
	if resource.UID == symptom.Resource.UID {
		return nil
	}

	targetDistance := sortedChain[idx].Distance - 1
	for nextIdx := idx + 1; nextIdx < len(sortedChain); nextIdx++ {
		if sortedChain[nextIdx].Distance != targetDistance {
			continue
		}

		nextUID := sortedChain[nextIdx].Resource.UID
		toNodeID := nodeMap[nextUID]
		if toNodeID == "" {
			return nil
		}

		return &GraphEdge{
			ID:               createSpineEdgeID(resource.UID, nextUID),
			From:             fromNodeID,
			To:               toNodeID,
			RelationshipType: edgeTypeOwns,
			EdgeType:         "SPINE",
		}
	}

	return nil
}

func buildManagesSpineEdge(
	resource graph.ResourceIdentity,
	fromNodeID string,
	managers map[string]*ManagerData,
	nodeMap map[string]string,
	ownedResources map[string]bool,
) *GraphEdge {
	mgrData := managers[resource.UID]
	if mgrData == nil || ownedResources[resource.UID] {
		return nil
	}

	managerNodeID := nodeMap[mgrData.Manager.UID]
	if managerNodeID == "" {
		return nil
	}

	return &GraphEdge{
		ID:               createSpineEdgeID(mgrData.Manager.UID, resource.UID),
		From:             managerNodeID,
		To:               fromNodeID,
		RelationshipType: "MANAGES",
		EdgeType:         "SPINE",
	}
}
