package anomaly

import (
	"github.com/moolen/spectre/internal/analysis"
)

// detectGraphLevelAnomalies detects anomalies that require graph context
// (e.g., Service with no ready endpoints based on SELECTS edges)
func (d *AnomalyDetector) detectGraphLevelAnomalies(
	graph analysis.CausalGraph,
	timeWindow TimeWindow,
	nodeAnomalies map[string][]Anomaly,
) []Anomaly {
	var anomalies []Anomaly

	nodeByID := make(map[string]*analysis.GraphNode)
	for i := range graph.Nodes {
		nodeByID[graph.Nodes[i].ID] = &graph.Nodes[i]
	}

	edgesBySource := make(map[string][]analysis.GraphEdge)
	for _, edge := range graph.Edges {
		edgesBySource[edge.From] = append(edgesBySource[edge.From], edge)
	}

	for i := range graph.Nodes {
		node := &graph.Nodes[i]

		switch node.Resource.Kind {
		case "Service":
			anomalies = append(anomalies,
				d.detectServiceEndpointAnomalies(node, nodeByID, edgesBySource, nodeAnomalies, timeWindow)...)
		case "Pod":
			anomalies = append(anomalies,
				d.detectSecretMissingAnomalies(node, nodeByID, edgesBySource, timeWindow)...)
			anomalies = append(anomalies,
				d.detectServiceAccountMissingAnomalies(node, nodeByID, edgesBySource, timeWindow)...)
		case "Certificate":
			if d.isCertManagerCertificate(node) {
				anomalies = append(anomalies, d.detectCertificateExpiredAnomalies(node, timeWindow)...)
			}
		}
	}

	return anomalies
}

// detectServiceEndpointAnomalies checks if a Service has no ready endpoints
func (d *AnomalyDetector) detectServiceEndpointAnomalies(
	serviceNode *analysis.GraphNode,
	nodeByID map[string]*analysis.GraphNode,
	edgesBySource map[string][]analysis.GraphEdge,
	nodeAnomalies map[string][]Anomaly,
	timeWindow TimeWindow,
) []Anomaly {
	var anomalies []Anomaly
	var selectsEdges []analysis.GraphEdge

	for _, edge := range edgesBySource[serviceNode.ID] {
		if edge.RelationshipType == "SELECTS" {
			selectsEdges = append(selectsEdges, edge)
		}
	}

	d.logger.Debug("Service %s has %d SELECTS edges", serviceNode.Resource.Name, len(selectsEdges))
	if len(selectsEdges) == 0 {
		if d.serviceHasSelector(serviceNode) {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(serviceNode),
				Category:  CategoryState,
				Type:      "NoReadyEndpoints",
				Severity:  SeverityHigh,
				Timestamp: timeWindow.End,
				Summary:   "Service has no matching endpoints",
				Details: map[string]interface{}{
					"reason": "no_pods_match_selector",
				},
			})
		}
		return anomalies
	}

	healthyPodCount := 0
	failedPodCount := 0
	podFailureTypes := make(map[string]bool)

	for _, edge := range selectsEdges {
		targetNode := nodeByID[edge.To]
		if targetNode == nil {
			continue
		}

		hasFailure := false
		for _, a := range nodeAnomalies[edge.To] {
			if IsPodFailureAnomaly(a.Type) {
				hasFailure = true
				podFailureTypes[a.Type] = true
			}
		}

		if hasFailure {
			failedPodCount++
		} else {
			healthyPodCount++
		}
	}

	d.logger.Debug("Service %s: %d healthy pods, %d failed pods",
		serviceNode.Resource.Name, healthyPodCount, failedPodCount)

	if healthyPodCount == 0 && failedPodCount > 0 {
		failureTypesList := make([]string, 0, len(podFailureTypes))
		for failureType := range podFailureTypes {
			failureTypesList = append(failureTypesList, failureType)
		}

		anomalies = append(anomalies, Anomaly{
			Node:      NodeFromGraphNode(serviceNode),
			Category:  CategoryState,
			Type:      "NoReadyEndpoints",
			Severity:  SeverityHigh,
			Timestamp: timeWindow.End,
			Summary:   "Service has no ready endpoints due to pod failures",
			Details: map[string]interface{}{
				"reason":            "all_pods_failing",
				"failed_pod_count":  failedPodCount,
				"pod_failure_types": failureTypesList,
			},
		})
	}

	return anomalies
}

// serviceHasSelector checks if a Service has a selector defined
func (d *AnomalyDetector) serviceHasSelector(serviceNode *analysis.GraphNode) bool {
	serviceData := latestSnapshotData(serviceNode)
	if serviceData == nil {
		return false
	}

	spec, ok := serviceData["spec"].(map[string]interface{})
	if !ok {
		return false
	}

	selector, ok := spec["selector"].(map[string]interface{})
	return ok && len(selector) > 0
}
