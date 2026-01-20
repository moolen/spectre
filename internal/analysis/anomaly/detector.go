package anomaly

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// AnomalyDetector orchestrates anomaly detection across the causal subgraph
type AnomalyDetector struct {
	graphClient graph.Client
	analyzer    *analysis.RootCauseAnalyzer
	logger      *logging.Logger

	// Sub-detectors
	eventDetector     *EventAnomalyDetector
	stateDetector     *StateAnomalyDetector
	changeDetector    *ChangeAnomalyDetector
	frequencyDetector *FrequencyAnomalyDetector
	configDetector    *ConfigAnomalyDetector
	networkDetector   *NetworkAnomalyDetector
}

// NewDetector creates a new anomaly detector
func NewDetector(graphClient graph.Client) *AnomalyDetector {
	return &AnomalyDetector{
		graphClient:       graphClient,
		analyzer:          analysis.NewRootCauseAnalyzer(graphClient),
		logger:            logging.GetLogger("anomaly.detector"),
		eventDetector:     NewEventAnomalyDetector(),
		stateDetector:     NewStateAnomalyDetector(),
		changeDetector:    NewChangeAnomalyDetector(),
		frequencyDetector: NewFrequencyAnomalyDetector(),
		configDetector:    NewConfigAnomalyDetector(),
		networkDetector:   NewNetworkAnomalyDetector(),
	}
}

// DetectInput contains the parameters for anomaly detection
type DetectInput struct {
	ResourceUID string
	Start       int64 // Unix seconds
	End         int64 // Unix seconds
}

// Detect analyzes a resource's causal subgraph for anomalies
func (d *AnomalyDetector) Detect(ctx context.Context, input DetectInput) (*AnomalyResponse, error) {
	timeWindow := TimeWindow{
		Start: time.Unix(input.Start, 0),
		End:   time.Unix(input.End, 0),
	}

	// Convert to nanoseconds for analyzer
	failureTimestampNs := input.End * 1_000_000_000
	lookbackNs := (input.End - input.Start) * 1_000_000_000

	d.logger.Debug("Detecting anomalies for resource %s, time window: %v to %v",
		input.ResourceUID, timeWindow.Start, timeWindow.End)

	// Use the existing analyzer to fetch the causal subgraph
	analyzeInput := analysis.AnalyzeInput{
		ResourceUID:      input.ResourceUID,
		FailureTimestamp: failureTimestampNs,
		LookbackNs:       lookbackNs,
		MaxDepth:         5,
		MinConfidence:    0.5,
		Format:           analysis.FormatDiff,
	}

	result, err := d.analyzer.Analyze(ctx, analyzeInput)
	if err != nil {
		// Check if this is a "no data in range" error with a hint
		var noDataErr *analysis.ErrNoChangeEventInRange
		if errors.As(err, &noDataErr) {
			// Return success with empty anomalies and a hint
			d.logger.Debug("No data in requested time range, returning hint: %s", noDataErr.Hint())
			return &AnomalyResponse{
				Anomalies: []Anomaly{},
				Metadata: ResponseMetadata{
					ResourceUID: input.ResourceUID,
					TimeWindow:  timeWindow,
					Hint:        noDataErr.Hint(),
				},
			}, nil
		}

		d.logger.Error("Failed to analyze causal graph: %v", err)
		return nil, fmt.Errorf("failed to analyze causal graph: %w", err)
	}

	d.logger.Debug("Causal graph has %d nodes", len(result.Incident.Graph.Nodes))

	// Collect all anomalies from all nodes
	var allAnomalies []Anomaly

	// Build a map of node anomalies for graph-level detection
	nodeAnomaliesMap := make(map[string][]Anomaly)

	for i := range result.Incident.Graph.Nodes {
		node := &result.Incident.Graph.Nodes[i]

		detectorInput := DetectorInput{
			Node:       node,
			TimeWindow: timeWindow,
			AllEvents:  node.AllEvents,
			K8sEvents:  node.K8sEvents,
		}

		var nodeAnomalies []Anomaly

		// Run all detectors
		eventAnomalies := d.eventDetector.Detect(detectorInput)
		d.logger.Debug("Node %s (%s): %d event anomalies",
			node.Resource.Name, node.Resource.Kind, len(eventAnomalies))
		nodeAnomalies = append(nodeAnomalies, eventAnomalies...)

		stateAnomalies := d.stateDetector.Detect(detectorInput)
		d.logger.Debug("Node %s (%s): %d state anomalies",
			node.Resource.Name, node.Resource.Kind, len(stateAnomalies))
		nodeAnomalies = append(nodeAnomalies, stateAnomalies...)

		changeAnomalies := d.changeDetector.Detect(detectorInput)
		d.logger.Debug("Node %s (%s): %d change anomalies",
			node.Resource.Name, node.Resource.Kind, len(changeAnomalies))
		nodeAnomalies = append(nodeAnomalies, changeAnomalies...)

		frequencyAnomalies := d.frequencyDetector.Detect(detectorInput)
		d.logger.Debug("Node %s (%s): %d frequency anomalies",
			node.Resource.Name, node.Resource.Kind, len(frequencyAnomalies))
		nodeAnomalies = append(nodeAnomalies, frequencyAnomalies...)

		configAnomalies := d.configDetector.Detect(detectorInput)
		d.logger.Debug("Node %s (%s): %d config anomalies",
			node.Resource.Name, node.Resource.Kind, len(configAnomalies))
		nodeAnomalies = append(nodeAnomalies, configAnomalies...)

		networkAnomalies := d.networkDetector.Detect(detectorInput)
		d.logger.Debug("Node %s (%s): %d network anomalies",
			node.Resource.Name, node.Resource.Kind, len(networkAnomalies))
		nodeAnomalies = append(nodeAnomalies, networkAnomalies...)

		// Store for graph-level detection
		nodeAnomaliesMap[node.ID] = nodeAnomalies
		allAnomalies = append(allAnomalies, nodeAnomalies...)
	}

	// Run graph-level detection (e.g., Service with no ready endpoints)
	graphAnomalies := d.detectGraphLevelAnomalies(result.Incident.Graph, timeWindow, nodeAnomaliesMap)
	d.logger.Debug("Graph-level anomalies: %d", len(graphAnomalies))
	allAnomalies = append(allAnomalies, graphAnomalies...)

	// Deduplicate anomalies (same node + type + timestamp)
	anomalies := deduplicateAnomalies(allAnomalies)

	d.logger.Debug("Total anomalies detected: %d (after deduplication)", len(anomalies))

	return &AnomalyResponse{
		Anomalies: anomalies,
		Metadata: ResponseMetadata{
			ResourceUID:   input.ResourceUID,
			TimeWindow:    timeWindow,
			NodesAnalyzed: len(result.Incident.Graph.Nodes),
		},
	}, nil
}

// detectGraphLevelAnomalies detects anomalies that require graph context
// (e.g., Service with no ready endpoints based on SELECTS edges)
func (d *AnomalyDetector) detectGraphLevelAnomalies(
	graph analysis.CausalGraph,
	timeWindow TimeWindow,
	nodeAnomalies map[string][]Anomaly,
) []Anomaly {
	var anomalies []Anomaly

	// Build lookup maps
	nodeByID := make(map[string]*analysis.GraphNode)
	for i := range graph.Nodes {
		nodeByID[graph.Nodes[i].ID] = &graph.Nodes[i]
	}

	// Build edge lookup: source node ID -> edges from that node
	edgesBySource := make(map[string][]analysis.GraphEdge)
	for _, edge := range graph.Edges {
		edgesBySource[edge.From] = append(edgesBySource[edge.From], edge)
	}

	// Detect Service anomalies based on SELECTS edges
	for i := range graph.Nodes {
		node := &graph.Nodes[i]
		if node.Resource.Kind != "Service" {
			continue
		}

		serviceAnomalies := d.detectServiceEndpointAnomalies(node, nodeByID, edgesBySource, nodeAnomalies, timeWindow)
		anomalies = append(anomalies, serviceAnomalies...)
	}

	// Detect SecretMissing for Pods with MOUNTS edges to non-existent Secrets/ConfigMaps
	for i := range graph.Nodes {
		node := &graph.Nodes[i]
		if node.Resource.Kind != "Pod" {
			continue
		}

		secretMissingAnomalies := d.detectSecretMissingAnomalies(node, nodeByID, edgesBySource, timeWindow)
		anomalies = append(anomalies, secretMissingAnomalies...)
	}

	// Detect CertExpired for Certificate resources (cert-manager)
	for i := range graph.Nodes {
		node := &graph.Nodes[i]
		if node.Resource.Kind != "Certificate" {
			continue
		}
		// Verify it's a cert-manager Certificate by checking apiVersion in the data
		if !d.isCertManagerCertificate(node) {
			continue
		}

		certAnomalies := d.detectCertificateExpiredAnomalies(node, timeWindow)
		anomalies = append(anomalies, certAnomalies...)
	}

	// Detect ServiceAccountMissing for Pods with USES_SERVICE_ACCOUNT edges to non-existent ServiceAccounts
	for i := range graph.Nodes {
		node := &graph.Nodes[i]
		if node.Resource.Kind != "Pod" {
			continue
		}

		saAnomalies := d.detectServiceAccountMissingAnomalies(node, nodeByID, edgesBySource, timeWindow)
		anomalies = append(anomalies, saAnomalies...)
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

	// Find SELECTS edges from this Service
	selectsEdges := []analysis.GraphEdge{}
	for _, edge := range edgesBySource[serviceNode.ID] {
		if edge.RelationshipType == "SELECTS" {
			selectsEdges = append(selectsEdges, edge)
		}
	}

	d.logger.Debug("Service %s has %d SELECTS edges", serviceNode.Resource.Name, len(selectsEdges))

	// If Service has no SELECTS edges, it might have no matching Pods
	// This could be due to selector mismatch or no Pods in namespace
	if len(selectsEdges) == 0 {
		// Check if Service has a selector (from latest event data)
		hasSelector := d.serviceHasSelector(serviceNode)
		if hasSelector {
			// Service has selector but no matching Pods - NoReadyEndpoints
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(serviceNode),
				Category:  CategoryState,
				Type:      "NoReadyEndpoints",
				Severity:  SeverityHigh,
				Timestamp: timeWindow.End, // Use end of time window
				Summary:   "Service has no matching endpoints",
				Details: map[string]interface{}{
					"reason": "no_pods_match_selector",
				},
			})
		}
		return anomalies
	}

	// Check if any selected Pods are healthy (no failure anomalies)
	healthyPodCount := 0
	failedPodCount := 0
	podFailureTypes := make(map[string]bool)

	for _, edge := range selectsEdges {
		targetNode := nodeByID[edge.To]
		if targetNode == nil {
			continue
		}

		// Check if this Pod has failure anomalies
		podAnomalies := nodeAnomalies[edge.To]
		hasFailure := false
		for _, a := range podAnomalies {
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

	// If all selected Pods have failures, Service has no ready endpoints
	if healthyPodCount == 0 && failedPodCount > 0 {
		failureTypesList := make([]string, 0, len(podFailureTypes))
		for ft := range podFailureTypes {
			failureTypesList = append(failureTypesList, ft)
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
	// Check the latest event for selector
	if len(serviceNode.AllEvents) == 0 {
		return false
	}

	// Get the latest event
	latestEvent := serviceNode.AllEvents[len(serviceNode.AllEvents)-1]

	// Try to parse FullSnapshot first
	if latestEvent.FullSnapshot != nil {
		if spec, ok := latestEvent.FullSnapshot["spec"].(map[string]interface{}); ok {
			if selector, ok := spec["selector"].(map[string]interface{}); ok {
				return len(selector) > 0
			}
		}
	}

	return false
}

// IsPodFailureAnomaly checks if an anomaly type indicates pod failure
func IsPodFailureAnomaly(anomalyType string) bool {
	failureTypes := map[string]bool{
		"CrashLoopBackOff":           true,
		"ImagePullBackOff":           true,
		"ErrImagePull":               true,
		"OOMKilled":                  true,
		"ContainerCreateError":       true,
		"CreateContainerConfigError": true,
		"InvalidImageNameError":      true,
		"PodPending":                 true,
		"Evicted":                    true,
		"ErrorStatus":                true,
		"InitContainerFailed":        true,
	}
	return failureTypes[anomalyType]
}

// deduplicateAnomalies removes duplicate anomalies based on node+type+timestamp
func deduplicateAnomalies(anomalies []Anomaly) []Anomaly {
	seen := make(map[string]bool)
	result := make([]Anomaly, 0, len(anomalies))

	for _, a := range anomalies {
		// Create a unique key from node UID, category, type, and timestamp
		key := fmt.Sprintf("%s:%s:%s:%d",
			a.Node.UID, a.Category, a.Type, a.Timestamp.Unix())

		if !seen[key] {
			seen[key] = true
			result = append(result, a)
		}
	}

	return result
}

// detectSecretMissingAnomalies checks if a Pod has MOUNTS edges to Secrets/ConfigMaps
// that don't exist in the graph (indicating missing configuration)
func (d *AnomalyDetector) detectSecretMissingAnomalies(
	podNode *analysis.GraphNode,
	nodeByID map[string]*analysis.GraphNode,
	edgesBySource map[string][]analysis.GraphEdge,
	timeWindow TimeWindow,
) []Anomaly {
	var anomalies []Anomaly

	// Find MOUNTS edges from this Pod
	for _, edge := range edgesBySource[podNode.ID] {
		if edge.RelationshipType != "MOUNTS" {
			continue
		}

		// Check if target exists in the graph
		targetNode := nodeByID[edge.To]
		if targetNode == nil {
			// Target doesn't exist in graph - this indicates a missing resource
			// We need to determine the kind from the edge or Pod spec
			// For now, we'll report it as a generic SecretMissing
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
	}

	// Also check Pod spec for volume references that might not have edges
	d.checkPodSpecForMissingReferences(podNode, nodeByID, timeWindow, &anomalies)

	return anomalies
}

// checkPodSpecForMissingReferences parses Pod spec to find Secret/ConfigMap references
func (d *AnomalyDetector) checkPodSpecForMissingReferences(
	podNode *analysis.GraphNode,
	nodeByID map[string]*analysis.GraphNode,
	_ TimeWindow,
	_ *[]Anomaly,
) {
	if len(podNode.AllEvents) == 0 {
		return
	}

	// Get latest event data
	latestEvent := podNode.AllEvents[len(podNode.AllEvents)-1]

	var podData map[string]interface{}
	if latestEvent.FullSnapshot != nil {
		podData = latestEvent.FullSnapshot
	} else if len(latestEvent.Data) > 0 {
		if err := json.Unmarshal(latestEvent.Data, &podData); err != nil {
			return
		}
	}

	if podData == nil {
		return
	}

	spec, ok := podData["spec"].(map[string]interface{})
	if !ok {
		return
	}

	// Check volumes for secret/configMap references
	volumes, ok := spec["volumes"].([]interface{})
	if ok {
		for _, vol := range volumes {
			volMap, ok := vol.(map[string]interface{})
			if !ok {
				continue
			}

			// Check for secret volume
			if secretVol, ok := volMap["secret"].(map[string]interface{}); ok {
				if secretName, ok := secretVol["secretName"].(string); ok {
					if !d.resourceExistsInGraph(nodeByID, "Secret", podNode.Resource.Namespace, secretName) {
						d.logger.Debug("Pod %s references missing Secret %s", podNode.Resource.Name, secretName)
						// Note: This might generate duplicates with MOUNTS edge check,
						// but deduplication will handle it
					}
				}
			}

			// Check for configMap volume
			if cmVol, ok := volMap["configMap"].(map[string]interface{}); ok {
				if cmName, ok := cmVol["name"].(string); ok {
					if !d.resourceExistsInGraph(nodeByID, "ConfigMap", podNode.Resource.Namespace, cmName) {
						d.logger.Debug("Pod %s references missing ConfigMap %s", podNode.Resource.Name, cmName)
					}
				}
			}
		}
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

// isCertManagerCertificate checks if a Certificate node is from cert-manager
func (d *AnomalyDetector) isCertManagerCertificate(certNode *analysis.GraphNode) bool {
	if len(certNode.AllEvents) == 0 {
		return false
	}

	// Check the apiVersion in the latest event data
	latestEvent := certNode.AllEvents[len(certNode.AllEvents)-1]

	var certData map[string]interface{}
	if latestEvent.FullSnapshot != nil {
		certData = latestEvent.FullSnapshot
	} else if len(latestEvent.Data) > 0 {
		if err := json.Unmarshal(latestEvent.Data, &certData); err != nil {
			return false
		}
	}

	if certData == nil {
		return false
	}

	// Check apiVersion contains cert-manager.io
	if apiVersion, ok := certData["apiVersion"].(string); ok {
		return apiVersion == "cert-manager.io/v1" ||
			apiVersion == "cert-manager.io/v1alpha2" ||
			apiVersion == "cert-manager.io/v1alpha3" ||
			apiVersion == "cert-manager.io/v1beta1"
	}

	return false
}

// detectCertificateExpiredAnomalies checks if a cert-manager Certificate has expired
func (d *AnomalyDetector) detectCertificateExpiredAnomalies(
	certNode *analysis.GraphNode,
	timeWindow TimeWindow,
) []Anomaly {
	var anomalies []Anomaly

	if len(certNode.AllEvents) == 0 {
		return anomalies
	}

	// Get latest event data
	latestEvent := certNode.AllEvents[len(certNode.AllEvents)-1]

	var certData map[string]interface{}
	if latestEvent.FullSnapshot != nil {
		certData = latestEvent.FullSnapshot
	} else if len(latestEvent.Data) > 0 {
		if err := json.Unmarshal(latestEvent.Data, &certData); err != nil {
			return anomalies
		}
	}

	if certData == nil {
		return anomalies
	}

	// Check status.notAfter field
	status, ok := certData["status"].(map[string]interface{})
	if !ok {
		return anomalies
	}

	notAfterStr, ok := status["notAfter"].(string)
	if !ok {
		return anomalies
	}

	// Parse the notAfter timestamp (RFC3339 format)
	notAfter, err := time.Parse(time.RFC3339, notAfterStr)
	if err != nil {
		d.logger.Debug("Failed to parse Certificate notAfter time: %v", err)
		return anomalies
	}

	// Check if certificate has expired
	now := time.Now()
	if notAfter.Before(now) {
		anomalies = append(anomalies, Anomaly{
			Node:      NodeFromGraphNode(certNode),
			Category:  CategoryState,
			Type:      "CertExpired",
			Severity:  SeverityCritical,
			Timestamp: timeWindow.End,
			Summary:   fmt.Sprintf("Certificate expired on %s", notAfter.Format(time.RFC3339)),
			Details: map[string]interface{}{
				"not_after":   notAfterStr,
				"expired_for": now.Sub(notAfter).String(),
			},
		})
	}

	return anomalies
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

	// Find USES_SERVICE_ACCOUNT edges from this Pod
	for _, edge := range edgesBySource[podNode.ID] {
		if edge.RelationshipType != "USES_SERVICE_ACCOUNT" {
			continue
		}

		// Check if target ServiceAccount exists in the graph
		targetNode := nodeByID[edge.To]
		if targetNode == nil {
			// ServiceAccount doesn't exist in graph - this indicates a missing resource
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
	}

	// Also check Pod spec for serviceAccountName that might not have edges
	d.checkPodSpecForMissingServiceAccount(podNode, nodeByID)

	return anomalies
}

// checkPodSpecForMissingServiceAccount parses Pod spec to find ServiceAccount references
func (d *AnomalyDetector) checkPodSpecForMissingServiceAccount(
	podNode *analysis.GraphNode,
	nodeByID map[string]*analysis.GraphNode,
) {
	if len(podNode.AllEvents) == 0 {
		return
	}

	// Get latest event data
	latestEvent := podNode.AllEvents[len(podNode.AllEvents)-1]

	var podData map[string]interface{}
	if latestEvent.FullSnapshot != nil {
		podData = latestEvent.FullSnapshot
	} else if len(latestEvent.Data) > 0 {
		if err := json.Unmarshal(latestEvent.Data, &podData); err != nil {
			return
		}
	}

	if podData == nil {
		return
	}

	spec, ok := podData["spec"].(map[string]interface{})
	if !ok {
		return
	}

	// Check serviceAccountName field
	if saName, ok := spec["serviceAccountName"].(string); ok && saName != "" && saName != "default" {
		if !d.resourceExistsInGraph(nodeByID, "ServiceAccount", podNode.Resource.Namespace, saName) {
			d.logger.Debug("Pod %s references missing ServiceAccount %s", podNode.Resource.Name, saName)
			// Note: This might generate duplicates with USES_SERVICE_ACCOUNT edge check,
			// but deduplication will handle it
		}
	}
}
