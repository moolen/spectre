package causalpaths

import (
	"context"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/analysis/anomaly"
)

// containsIgnoreCase checks if s contains substr (case insensitive).
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// selectsTarget represents a Pod selected by a Service.
type selectsTarget struct {
	uid       string
	kind      string
	namespace string
	name      string
}

// traverseFromServiceSymptom handles causal path discovery when the symptom is a Service
// with NoReadyEndpoints anomaly.
func (d *PathDiscoverer) traverseFromServiceSymptom(
	ctx context.Context,
	serviceNode *analysis.GraphNode,
	input CausalPathsInput,
	nodeAnomalies map[string][]anomaly.Anomaly,
	symptomFirstFailure time.Time,
) []CausalPath {
	var paths []CausalPath

	selectsTargets, err := d.querySelectsTargets(ctx, serviceNode.Resource.UID, input.FailureTimestamp)
	if err != nil {
		d.logger.Error("traverseFromServiceSymptom: failed to query SELECTS targets: %v", err)
		return []CausalPath{d.buildServiceOnlyPath(serviceNode, nodeAnomalies)}
	}

	d.logger.Debug("traverseFromServiceSymptom: Service %s has %d SELECTS targets",
		serviceNode.Resource.Name, len(selectsTargets))

	if len(selectsTargets) == 0 {
		d.logger.Debug("traverseFromServiceSymptom: No SELECTS targets, returning Service-only path")
		return []CausalPath{d.buildServiceOnlyPath(serviceNode, nodeAnomalies)}
	}

	allPodsHealthy := true
	anyPodAnalyzed := false

	for _, target := range selectsTargets {
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

		if podNode.ChangeEvent != nil {
			status := podNode.ChangeEvent.Status
			if status == "Error" || status == "Warning" {
				allPodsHealthy = false
				d.logger.Debug("traverseFromServiceSymptom: Pod %s has failure status: %s", target.name, status)
			}

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

		if allPodsHealthy {
			for _, k8sEvent := range podNode.K8sEvents {
				if k8sEvent.Type == "Warning" {
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

			for _, detected := range podAnomalies {
				if anomaly.IsPodFailureAnomaly(detected.Type) {
					allPodsHealthy = false
					d.logger.Debug("traverseFromServiceSymptom: Pod %s has failure anomaly: %s", target.name, detected.Type)
					break
				}
			}
		}

		if !allPodsHealthy {
			break
		}
	}

	if allPodsHealthy && anyPodAnalyzed && len(selectsTargets) > 0 {
		d.logger.Debug("traverseFromServiceSymptom: All %d Pods are healthy, Service is root cause (likely selector change)", len(selectsTargets))
		return []CausalPath{d.buildServiceOnlyPath(serviceNode, nodeAnomalies)}
	}

	for _, target := range selectsTargets {
		d.logger.Debug("traverseFromServiceSymptom: Analyzing Pod %s (UID: %s)", target.name, target.uid)

		podInput := analysis.AnalyzeInput{
			ResourceUID:      target.uid,
			FailureTimestamp: input.FailureTimestamp,
			LookbackNs:       input.LookbackNs,
			MaxDepth:         input.MaxDepth - 1,
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

		podNodeMap := d.buildNodeMap(podGraph)
		podAnomalyInput := CausalPathsInput{
			ResourceUID:      target.uid,
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

		podUpstreamAdjacency := d.buildUpstreamAdjacency(podGraph)
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

		selectsEdge := &analysis.GraphEdge{
			From:             serviceNode.ID,
			To:               podNode.ID,
			RelationshipType: "SELECTS",
		}

		for _, path := range podPaths {
			paths = append(paths, d.appendServiceToPath(path, serviceNode, selectsEdge, nodeAnomalies))
		}

		if len(podPaths) == 0 {
			podAnomalies := podNodeAnomalies[podNode.ID]
			if HasCauseIntroducingAnomaly(podAnomalies, symptomFirstFailure) || len(podAnomalies) > 0 {
				paths = append(paths, d.buildPodToServicePath(podNode, serviceNode, selectsEdge, nodeAnomalies))
			}
		}
	}

	return paths
}
