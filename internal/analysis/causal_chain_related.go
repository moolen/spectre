package analysis

// buildRelatedGraph creates RELATED nodes and ATTACHMENT edges for supporting resources.
func (a *RootCauseAnalyzer) buildRelatedGraph(
	sortedChain []ResourceWithDistance,
	managers map[string]*ManagerData,
	related map[string][]RelatedResourceData,
	nodeMap map[string]string,
) relatedGraphBuildResult {
	nodes := []GraphNode{}
	edges := []GraphEdge{}
	edgeSet := make(map[string]bool)

	resourcesWithRelated := collectResourcesWithRelated(sortedChain, managers, a)

	for _, resourceUID := range resourcesWithRelated {
		parentNodeID := nodeMap[resourceUID]
		if parentNodeID == "" {
			a.logger.Debug("buildRelatedGraph: skipping related resources for %s - no node in map", resourceUID)
			continue
		}

		relatedList := related[resourceUID]
		a.logger.Debug("buildRelatedGraph: pass 1 - creating nodes for %d related resources of %s", len(relatedList), resourceUID)
		nodes = append(nodes, a.buildRelatedNodes(relatedList, nodeMap)...)
	}

	for _, resourceUID := range resourcesWithRelated {
		parentNodeID := nodeMap[resourceUID]
		if parentNodeID == "" {
			continue
		}

		relatedList := related[resourceUID]
		a.logger.Debug("buildRelatedGraph: pass 2 - creating edges for %d related resources of %s", len(relatedList), resourceUID)
		edges = append(edges, a.buildRelatedEdges(parentNodeID, relatedList, nodeMap, edgeSet)...)
	}

	return relatedGraphBuildResult{nodes: nodes, edges: edges}
}

func collectResourcesWithRelated(
	sortedChain []ResourceWithDistance,
	managers map[string]*ManagerData,
	analyzer *RootCauseAnalyzer,
) []string {
	resourcesWithRelated := make([]string, 0, len(sortedChain)+len(managers))
	for _, rwd := range sortedChain {
		resourcesWithRelated = append(resourcesWithRelated, rwd.Resource.UID)
	}

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
			analyzer.logger.Debug("buildRelatedGraph: adding manager %s/%s (UID: %s) to resources with related",
				mgrData.Manager.Kind, mgrData.Manager.Name, managerUID)
			resourcesWithRelated = append(resourcesWithRelated, managerUID)
		}
	}

	return resourcesWithRelated
}

func (a *RootCauseAnalyzer) buildRelatedNodes(relatedList []RelatedResourceData, nodeMap map[string]string) []GraphNode {
	nodes := []GraphNode{}

	for _, relData := range relatedList {
		if !shouldIncludeRelatedResource(relData, a) {
			continue
		}

		relatedUID := relData.Resource.UID
		if nodeMap[relatedUID] != "" {
			continue
		}

		relatedNodeID := createNodeID(relatedUID)
		nodeMap[relatedUID] = relatedNodeID
		nodes = append(nodes, createRelatedNode(
			relatedNodeID,
			resourceIdentityToSymptomResource(relData.Resource),
			relData.Events,
		))
		a.logger.Debug("buildRelatedGraph: created node for %s/%s (type=%s)",
			relData.Resource.Kind, relData.Resource.Name, relData.RelationshipType)
	}

	return nodes
}

func shouldIncludeRelatedResource(relData RelatedResourceData, analyzer *RootCauseAnalyzer) bool {
	hasChanges := len(relData.Events) > 0
	if relData.RelationshipType != edgeTypeScheduledOn &&
		relData.RelationshipType != edgeTypeGrantsTo &&
		relData.RelationshipType != edgeTypeBindsRole &&
		relData.RelationshipType != edgeTypeReferencesSpec &&
		relData.RelationshipType != edgeTypeIngressRef &&
		!hasChanges {
		analyzer.logger.Debug("buildRelatedGraph: skipping %s (type=%s) - no changes", relData.Resource.Name, relData.RelationshipType)
		return false
	}

	if relData.RelationshipType == edgeTypeReferencesSpec || relData.RelationshipType == edgeTypeIngressRef {
		analyzer.logger.Debug("buildRelatedGraph: including %s %s/%s (hasChanges=%v)",
			relData.RelationshipType, relData.Resource.Kind, relData.Resource.Name, hasChanges)
	}

	return true
}

func (a *RootCauseAnalyzer) buildRelatedEdges(
	parentNodeID string,
	relatedList []RelatedResourceData,
	nodeMap map[string]string,
	edgeSet map[string]bool,
) []GraphEdge {
	edges := []GraphEdge{}

	for _, relData := range relatedList {
		if !shouldIncludeRelatedResource(relData, a) {
			continue
		}

		edge := a.buildRelatedEdge(parentNodeID, relatedList, relData, nodeMap)
		if edge == nil || edgeSet[edge.ID] {
			continue
		}

		edges = append(edges, *edge)
		edgeSet[edge.ID] = true
	}

	return edges
}

func (a *RootCauseAnalyzer) buildRelatedEdge(
	parentNodeID string,
	relatedList []RelatedResourceData,
	relData RelatedResourceData,
	nodeMap map[string]string,
) *GraphEdge {
	relatedNodeID := nodeMap[relData.Resource.UID]

	switch relData.RelationshipType {
	case edgeTypeGrantsTo:
		return a.buildGrantsToEdge(relatedList, relData, relatedNodeID, nodeMap)
	case edgeTypeBindsRole:
		return a.buildBindsRoleEdge(relatedList, relData, relatedNodeID, nodeMap)
	default:
		return a.buildAttachmentEdge(parentNodeID, relData, relatedNodeID, nodeMap)
	}
}

func (a *RootCauseAnalyzer) buildGrantsToEdge(
	relatedList []RelatedResourceData,
	relData RelatedResourceData,
	relatedNodeID string,
	nodeMap map[string]string,
) *GraphEdge {
	serviceAccountNodeID := relatedNodeIDForType(relatedList, "USES_SERVICE_ACCOUNT", nodeMap)
	if serviceAccountNodeID == "" {
		return nil
	}

	a.logger.Debug("buildRelatedGraph: created GRANTS_TO edge from RoleBinding %s to ServiceAccount", relData.Resource.Name)
	return &GraphEdge{
		ID:               createAttachmentEdgeID(relatedNodeID, serviceAccountNodeID),
		From:             relatedNodeID,
		To:               serviceAccountNodeID,
		RelationshipType: edgeTypeGrantsTo,
		EdgeType:         "ATTACHMENT",
	}
}

func (a *RootCauseAnalyzer) buildBindsRoleEdge(
	relatedList []RelatedResourceData,
	relData RelatedResourceData,
	relatedNodeID string,
	nodeMap map[string]string,
) *GraphEdge {
	roleBindingNodeID := relatedNodeIDForType(relatedList, edgeTypeGrantsTo, nodeMap)
	if roleBindingNodeID == "" {
		a.logger.Debug("buildRelatedGraph: skipping BINDS_ROLE edge - RoleBinding node not found")
		return nil
	}

	a.logger.Debug("buildRelatedGraph: created BINDS_ROLE edge from RoleBinding to Role %s/%s",
		relData.Resource.Kind, relData.Resource.Name)
	return &GraphEdge{
		ID:               createAttachmentEdgeID(roleBindingNodeID, relatedNodeID),
		From:             roleBindingNodeID,
		To:               relatedNodeID,
		RelationshipType: edgeTypeBindsRole,
		EdgeType:         "ATTACHMENT",
	}
}

func (a *RootCauseAnalyzer) buildAttachmentEdge(
	parentNodeID string,
	relData RelatedResourceData,
	relatedNodeID string,
	nodeMap map[string]string,
) *GraphEdge {
	fromNode, toNode, ok := a.attachmentEndpoints(parentNodeID, relData, relatedNodeID, nodeMap)
	if !ok {
		return nil
	}

	relType := relData.RelationshipType
	if relType == edgeTypeIngressRef {
		relType = edgeTypeReferencesSpec
	}

	return &GraphEdge{
		ID:               createAttachmentEdgeID(fromNode, toNode),
		From:             fromNode,
		To:               toNode,
		RelationshipType: relType,
		EdgeType:         "ATTACHMENT",
	}
}

func (a *RootCauseAnalyzer) attachmentEndpoints(
	parentNodeID string,
	relData RelatedResourceData,
	relatedNodeID string,
	nodeMap map[string]string,
) (string, string, bool) {
	if relData.RelationshipType == "SELECTS" {
		return relatedNodeID, parentNodeID, true
	}

	if relData.RelationshipType == edgeTypeIngressRef {
		if relData.ReferenceTargetUID == "" {
			a.logger.Debug("buildRelatedGraph: skipping INGRESS_REF edge - no ReferenceTargetUID")
			return "", "", false
		}

		targetNodeID := nodeMap[relData.ReferenceTargetUID]
		if targetNodeID == "" {
			a.logger.Debug("buildRelatedGraph: skipping INGRESS_REF edge - Service node not found for UID %s",
				relData.ReferenceTargetUID)
			return "", "", false
		}
		return relatedNodeID, targetNodeID, true
	}

	return parentNodeID, relatedNodeID, true
}

func relatedNodeIDForType(relatedList []RelatedResourceData, relationshipType string, nodeMap map[string]string) string {
	for _, otherRelData := range relatedList {
		if otherRelData.RelationshipType == relationshipType {
			return nodeMap[otherRelData.Resource.UID]
		}
	}
	return ""
}
