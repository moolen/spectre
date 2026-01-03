//go:build integration
// +build integration

package graph

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

// extractTimestampAndPodUID extracts the timestamp from the last event and pod UID from the JSONL file
// The pod UID should be the pod that relates to the resource under test (prefers ReplicaSet/StatefulSet owned pods)
func extractTimestampAndPodUID(jsonlPath string) (int64, string, error) {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()

	var lastTimestamp int64
	var podUID string
	scanner := bufio.NewScanner(file)
	// Increase buffer size to handle large JSONL lines (default is 64KB)
	// Use 10MB buffer to handle very large resource definitions
	buf := make([]byte, 0, 10*1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event models.Event
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		// Track the last timestamp
		if event.Timestamp > lastTimestamp {
			lastTimestamp = event.Timestamp
		}

		// Extract pod UID if this is a Pod resource
		// Prefer pods that are owned by ReplicaSet (which are managed by Deployment/HelmRelease)
		// or StatefulSet (for StatefulSet scenarios)
		if event.Resource.Kind == "Pod" {
			// Try to parse the data field to check for owner
			if len(event.Data) > 0 {
				var data map[string]interface{}
				if err := json.Unmarshal(event.Data, &data); err == nil {
					if metadata, ok := data["metadata"].(map[string]interface{}); ok {
						if ownerRefs, ok := metadata["ownerReferences"].([]interface{}); ok {
							for _, ref := range ownerRefs {
								if refMap, ok := ref.(map[string]interface{}); ok {
									if kind, ok := refMap["kind"].(string); ok {
										// Prefer ReplicaSet-owned pods (for HelmRelease/Deployment scenarios)
										// or StatefulSet-owned pods (for StatefulSet scenarios)
										if kind == "ReplicaSet" || kind == "StatefulSet" {
											podUID = event.Resource.UID
											break
										}
									}
								}
							}
						}
					}
				}
			}
			// If we haven't found a ReplicaSet/StatefulSet-owned pod yet, use any pod as fallback
			if podUID == "" {
				podUID = event.Resource.UID
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, "", err
	}

	return lastTimestamp, podUID, nil
}

// extractInitialConfigTime extracts the timestamp of the first CREATE event for a given resource kind
func extractInitialConfigTime(jsonlPath string, kind string) (time.Time, error) {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return time.Time{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Increase buffer size to handle large JSONL lines
	buf := make([]byte, 0, 10*1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event models.Event
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		// Find first CREATE event for the specified kind
		if event.Resource.Kind == kind && event.Type == models.EventTypeCreate {
			return time.Unix(0, event.Timestamp), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return time.Time{}, err
	}

	return time.Time{}, nil
}

// findNodeByKind finds a node in the graph by its resource kind
func findNodeByKind(rca *analysis.RootCauseAnalysisV2, kind string) *analysis.GraphNode {
	if rca == nil || rca.Incident.Graph.Nodes == nil {
		return nil
	}

	for i := range rca.Incident.Graph.Nodes {
		if rca.Incident.Graph.Nodes[i].Resource.Kind == kind {
			return &rca.Incident.Graph.Nodes[i]
		}
	}
	return nil
}

// assertGraphHasEdgeBetweenKinds verifies that the graph contains an edge of the specified
// relationship type between nodes of the given kinds
func assertGraphHasEdgeBetweenKinds(t testing.TB, rca *analysis.RootCauseAnalysisV2, fromKind, relType, toKind string) {
	req := require.New(t)
	req.NotNil(rca, "Root cause analysis should not be nil")
	req.NotNil(rca.Incident, "Incident should not be nil")
	req.NotNil(rca.Incident.Graph, "Graph should not be nil")

	fromNode := findNodeByKind(rca, fromKind)
	req.NotNil(fromNode, "Graph should contain node of kind %s", fromKind)

	toNode := findNodeByKind(rca, toKind)
	req.NotNil(toNode, "Graph should contain node of kind %s", toKind)

	found := false
	for _, edge := range rca.Incident.Graph.Edges {
		if edge.From == fromNode.ID && edge.To == toNode.ID && edge.RelationshipType == relType {
			found = true
			break
		}
	}

	req.True(found, "Graph should contain edge %s -[%s]-> %s", fromKind, relType, toKind)
}

// assertGraphHasDeploymentOwnsPod verifies the ownership chain from Deployment to Pod
// This is a shared helper used by multiple test files
func assertGraphHasDeploymentOwnsPod(t testing.TB, rca *analysis.RootCauseAnalysisV2) {
	assertGraphHasEdgeBetweenKinds(t, rca, "Deployment", "OWNS", "ReplicaSet")
	assertGraphHasEdgeBetweenKinds(t, rca, "ReplicaSet", "OWNS", "Pod")
	if t, ok := t.(*testing.T); ok {
		t.Logf("✓ Found ownership chain: Deployment -> ReplicaSet -> Pod")
	}
}

// getKeysFromMap is a helper to get keys from a map
func getKeysFromMap(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
