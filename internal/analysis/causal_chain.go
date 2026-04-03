package analysis

import "github.com/moolen/spectre/internal/graph"

const (
	edgeTypeOwns           = "OWNS"
	edgeTypeScheduledOn    = "SCHEDULED_ON"
	edgeTypeGrantsTo       = "GRANTS_TO"
	edgeTypeBindsRole      = "BINDS_ROLE"
	edgeTypeReferencesSpec = "REFERENCES_SPEC"
	edgeTypeIngressRef     = "INGRESS_REF"
)

// relatedGraphBuildResult contains the output of building RELATED nodes and edges.
type relatedGraphBuildResult struct {
	nodes []GraphNode
	edges []GraphEdge
}

// mergeIntoCausalGraph combines the split query results into a CausalGraph.
func (a *RootCauseAnalyzer) mergeIntoCausalGraph(
	symptom *ObservedSymptom,
	chain []ResourceWithDistance,
	managers map[string]*ManagerData,
	related map[string][]RelatedResourceData,
	changeEvents map[string][]ChangeEventInfo,
	k8sEvents map[string][]K8sEventInfo,
	failureTimestamp int64,
) (CausalGraph, error) {
	edges := []GraphEdge{}

	sortedChain := sortChainByDistanceDesc(chain)

	spineResult := a.buildSpineNodes(symptom, sortedChain, managers, changeEvents, k8sEvents, failureTimestamp)
	nodes := spineResult.nodes
	nodeMap := spineResult.nodeMap

	spineEdges := a.buildSpineEdges(symptom, sortedChain, managers, nodeMap)
	edges = append(edges, spineEdges...)

	relatedResult := a.buildRelatedGraph(sortedChain, managers, related, nodeMap)
	nodes = append(nodes, relatedResult.nodes...)
	edges = append(edges, relatedResult.edges...)

	a.logger.Debug("Built causal graph with %d nodes and %d edges", len(nodes), len(edges))
	return CausalGraph{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

func sortChainByDistanceDesc(chain []ResourceWithDistance) []ResourceWithDistance {
	sortedChain := make([]ResourceWithDistance, len(chain))
	copy(sortedChain, chain)

	for i := 0; i < len(sortedChain); i++ {
		for j := i + 1; j < len(sortedChain); j++ {
			if sortedChain[j].Distance > sortedChain[i].Distance {
				sortedChain[i], sortedChain[j] = sortedChain[j], sortedChain[i]
			}
		}
	}

	return sortedChain
}

func spineRelationshipType(symptom *ObservedSymptom, resource graph.ResourceIdentity, manager *graph.ResourceIdentity) string {
	if manager != nil {
		return "MANAGED_BY"
	}
	if resource.UID != symptom.Resource.UID {
		return edgeTypeOwns
	}
	return "SYMPTOM"
}
