package causalpaths

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/analysis/anomaly"
)

// buildServiceOnlyPath creates a path with just the Service when there are no selected Pods.
func (d *PathDiscoverer) buildServiceOnlyPath(
	serviceNode *analysis.GraphNode,
	nodeAnomalies map[string][]anomaly.Anomaly,
) CausalPath {
	anomalies := nodeAnomalies[serviceNode.ID]

	var firstAnomalyAt time.Time
	for _, detected := range anomalies {
		if firstAnomalyAt.IsZero() || detected.Timestamp.Before(firstAnomalyAt) {
			firstAnomalyAt = detected.Timestamp
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

	step := PathStep{Node: pathNode}
	hash := sha256.Sum256([]byte(serviceNode.ID))

	return CausalPath{
		ID:             fmt.Sprintf("path-%x", hash[:8]),
		CandidateRoot:  pathNode,
		FirstAnomalyAt: firstAnomalyAt,
		Steps:          []PathStep{step},
	}
}

// appendServiceToPath adds the Service as the final step in a causal path.
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

	if selectsEdge != nil {
		edgeCategory := ClassifyEdge(selectsEdge.RelationshipType)
		serviceStep.Edge = &PathEdge{
			ID:               selectsEdge.ID,
			RelationshipType: selectsEdge.RelationshipType,
			EdgeCategory:     edgeCategory,
			CausalWeight:     GetCausalWeight(edgeCategory),
		}
	}

	path.Steps = append(path.Steps, serviceStep)

	var pathStr string
	for _, step := range path.Steps {
		pathStr += step.Node.ID
		if step.Edge != nil {
			pathStr += "-" + step.Edge.RelationshipType + "-"
		}
	}
	hash := sha256.Sum256([]byte(pathStr))

	return CausalPath{
		ID:             fmt.Sprintf("path-%x", hash[:8]),
		CandidateRoot:  path.CandidateRoot,
		FirstAnomalyAt: path.FirstAnomalyAt,
		Steps:          path.Steps,
	}
}

// buildPodToServicePath creates a simple path from Pod to Service.
func (d *PathDiscoverer) buildPodToServicePath(
	podNode *analysis.GraphNode,
	serviceNode *analysis.GraphNode,
	selectsEdge *analysis.GraphEdge,
	nodeAnomalies map[string][]anomaly.Anomaly,
) CausalPath {
	podAnomalies := nodeAnomalies[podNode.ID]
	serviceAnomalies := nodeAnomalies[serviceNode.ID]

	var firstAnomalyAt time.Time
	for _, detected := range podAnomalies {
		if firstAnomalyAt.IsZero() || detected.Timestamp.Before(firstAnomalyAt) {
			firstAnomalyAt = detected.Timestamp
		}
	}
	for _, detected := range serviceAnomalies {
		if firstAnomalyAt.IsZero() || detected.Timestamp.Before(firstAnomalyAt) {
			firstAnomalyAt = detected.Timestamp
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

	return CausalPath{
		ID:             fmt.Sprintf("path-%x", hash[:8]),
		CandidateRoot:  podPathNode,
		FirstAnomalyAt: firstAnomalyAt,
		Steps:          steps,
	}
}
