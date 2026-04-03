package anomaly

import (
	"github.com/moolen/spectre/internal/analysis"
)

// detectSecretMissingAnomalies checks if a Pod has MOUNTS edges to Secrets/ConfigMaps
// that don't exist in the graph (indicating missing configuration)
func (d *AnomalyDetector) detectSecretMissingAnomalies(
	podNode *analysis.GraphNode,
	nodeByID map[string]*analysis.GraphNode,
	edgesBySource map[string][]analysis.GraphEdge,
	timeWindow TimeWindow,
) []Anomaly {
	var anomalies []Anomaly

	for _, edge := range edgesBySource[podNode.ID] {
		if edge.RelationshipType != "MOUNTS" {
			continue
		}

		if nodeByID[edge.To] != nil {
			continue
		}

		anomalies = append(anomalies, Anomaly{
			Node:      NodeFromGraphNode(podNode),
			Category:  CategoryState,
			Type:      "SecretMissing",
			Severity:  SeverityCritical,
			Timestamp: timeWindow.End,
			Summary:   "Pod references a Secret or ConfigMap that doesn't exist",
			Details: map[string]interface{}{
				"target_id": edge.To,
				"reason":    "referenced_resource_not_found",
			},
		})
	}

	d.checkPodSpecForMissingReferences(podNode, nodeByID)

	return anomalies
}

// checkPodSpecForMissingReferences parses Pod spec to find Secret/ConfigMap references
func (d *AnomalyDetector) checkPodSpecForMissingReferences(
	podNode *analysis.GraphNode,
	nodeByID map[string]*analysis.GraphNode,
) {
	podData := latestSnapshotData(podNode)
	if podData == nil {
		return
	}

	spec, ok := podData["spec"].(map[string]interface{})
	if !ok {
		return
	}

	volumes, ok := spec["volumes"].([]interface{})
	if !ok {
		return
	}

	for _, volume := range volumes {
		volMap, ok := volume.(map[string]interface{})
		if !ok {
			continue
		}

		if secretVol, ok := volMap["secret"].(map[string]interface{}); ok {
			if secretName, ok := secretVol["secretName"].(string); ok &&
				!d.resourceExistsInGraph(nodeByID, "Secret", podNode.Resource.Namespace, secretName) {
				d.logger.Debug("Pod %s references missing Secret %s", podNode.Resource.Name, secretName)
			}
		}

		if configMapVol, ok := volMap["configMap"].(map[string]interface{}); ok {
			if cmName, ok := configMapVol["name"].(string); ok &&
				!d.resourceExistsInGraph(nodeByID, "ConfigMap", podNode.Resource.Namespace, cmName) {
				d.logger.Debug("Pod %s references missing ConfigMap %s", podNode.Resource.Name, cmName)
			}
		}
	}
}

// detectServiceAccountMissingAnomalies checks if a Pod references a ServiceAccount
// that doesn't exist in the graph (indicating missing RBAC configuration)
func (d *AnomalyDetector) detectServiceAccountMissingAnomalies(
	podNode *analysis.GraphNode,
	nodeByID map[string]*analysis.GraphNode,
	edgesBySource map[string][]analysis.GraphEdge,
	timeWindow TimeWindow,
) []Anomaly {
	var anomalies []Anomaly

	for _, edge := range edgesBySource[podNode.ID] {
		if edge.RelationshipType != "USES_SERVICE_ACCOUNT" {
			continue
		}

		if nodeByID[edge.To] != nil {
			continue
		}

		anomalies = append(anomalies, Anomaly{
			Node:      NodeFromGraphNode(podNode),
			Category:  CategoryState,
			Type:      "ServiceAccountMissing",
			Severity:  SeverityCritical,
			Timestamp: timeWindow.End,
			Summary:   "Pod references a ServiceAccount that doesn't exist",
			Details: map[string]interface{}{
				"target_id": edge.To,
				"reason":    "serviceaccount_not_found",
			},
		})
	}

	d.checkPodSpecForMissingServiceAccount(podNode, nodeByID)

	return anomalies
}

// checkPodSpecForMissingServiceAccount parses Pod spec to find ServiceAccount references
func (d *AnomalyDetector) checkPodSpecForMissingServiceAccount(
	podNode *analysis.GraphNode,
	nodeByID map[string]*analysis.GraphNode,
) {
	podData := latestSnapshotData(podNode)
	if podData == nil {
		return
	}

	spec, ok := podData["spec"].(map[string]interface{})
	if !ok {
		return
	}

	saName, ok := spec["serviceAccountName"].(string)
	if !ok || saName == "" || saName == "default" {
		return
	}

	if !d.resourceExistsInGraph(nodeByID, "ServiceAccount", podNode.Resource.Namespace, saName) {
		d.logger.Debug("Pod %s references missing ServiceAccount %s", podNode.Resource.Name, saName)
	}
}

// resourceExistsInGraph checks if a resource exists in the graph nodes
func (d *AnomalyDetector) resourceExistsInGraph(
	nodeByID map[string]*analysis.GraphNode,
	kind, namespace, name string,
) bool {
	for _, node := range nodeByID {
		if node.Resource.Kind == kind &&
			node.Resource.Namespace == namespace &&
			node.Resource.Name == name {
			return true
		}
	}
	return false
}
