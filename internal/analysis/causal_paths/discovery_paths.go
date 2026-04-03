package causalpaths

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/analysis/anomaly"
)

// deduplicateByRootCause merges paths that share the same root cause resource UID.
func (d *PathDiscoverer) deduplicateByRootCause(paths []CausalPath) []CausalPath {
	if len(paths) <= 1 {
		for i := range paths {
			if paths[i].AffectedCount == 0 {
				paths[i].AffectedCount = 1
				paths[i].AffectedSymptoms = []PathNode{d.extractSymptomNode(paths[i])}
			}
		}
		return paths
	}

	type rootCauseEntry struct {
		bestPath         CausalPath
		affectedSymptoms []PathNode
	}

	entriesByRoot := make(map[string]*rootCauseEntry)

	for _, path := range paths {
		rootUID := path.CandidateRoot.Resource.UID
		if rootUID == "" {
			rootUID = path.ID
		}

		symptomNode := d.extractSymptomNode(path)
		existing, exists := entriesByRoot[rootUID]
		if !exists {
			entriesByRoot[rootUID] = &rootCauseEntry{
				bestPath:         path,
				affectedSymptoms: []PathNode{symptomNode},
			}
			continue
		}

		existing.affectedSymptoms = append(existing.affectedSymptoms, symptomNode)

		existingLen := len(existing.bestPath.Steps)
		newLen := len(path.Steps)

		shouldReplace := false
		if newLen > existingLen {
			shouldReplace = true
			d.logger.Debug("deduplicateByRootCause: replacing path %s (len=%d) with path %s (len=%d) for root %s/%s - longer path preferred",
				existing.bestPath.ID, existingLen,
				path.ID, newLen,
				path.CandidateRoot.Resource.Kind, path.CandidateRoot.Resource.Name)
		} else if newLen == existingLen && path.ConfidenceScore > existing.bestPath.ConfidenceScore {
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

	sort.Slice(result, func(i, j int) bool {
		return result[i].ConfidenceScore > result[j].ConfidenceScore
	})

	return result
}

// extractSymptomNode extracts the symptom node (last node in the path) from a CausalPath.
func (d *PathDiscoverer) extractSymptomNode(path CausalPath) PathNode {
	if len(path.Steps) == 0 {
		return PathNode{}
	}
	return path.Steps[len(path.Steps)-1].Node
}

// buildCausalPath constructs a CausalPath from the traversal path.
func (d *PathDiscoverer) buildCausalPath(
	path []pathElement,
	nodeMap map[string]*analysis.GraphNode,
	nodeAnomalies map[string][]anomaly.Anomaly,
) CausalPath {
	if len(path) == 0 {
		return CausalPath{}
	}

	steps := make([]PathStep, 0, len(path))
	var firstAnomalyAt time.Time

	for i, elem := range path {
		node := nodeMap[elem.NodeID]
		if node == nil {
			continue
		}

		anomalies := nodeAnomalies[elem.NodeID]
		for _, detected := range anomalies {
			if firstAnomalyAt.IsZero() || detected.Timestamp.Before(firstAnomalyAt) {
				firstAnomalyAt = detected.Timestamp
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

	candidateRoot := PathNode{}
	if len(steps) > 0 {
		candidateRoot = steps[0].Node
	}

	return CausalPath{
		ID:             d.generatePathID(path),
		CandidateRoot:  candidateRoot,
		FirstAnomalyAt: firstAnomalyAt,
		Steps:          steps,
	}
}

// generatePathID creates a deterministic ID for a path based on node IDs and edges.
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
