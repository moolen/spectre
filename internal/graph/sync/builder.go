package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync/extractors"
	"github.com/moolen/spectre/internal/graph/sync/extractors/argocd"
	"github.com/moolen/spectre/internal/graph/sync/extractors/certmanager"
	"github.com/moolen/spectre/internal/graph/sync/extractors/externalsecrets"
	"github.com/moolen/spectre/internal/graph/sync/extractors/gateway"
	"github.com/moolen/spectre/internal/graph/sync/extractors/native"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// graphBuilder implements the GraphBuilder interface
type graphBuilder struct {
	logger            *logging.Logger
	client            graph.Client // Graph client for querying existing nodes
	extractorRegistry *extractors.ExtractorRegistry
}

// NewGraphBuilder creates a new graph builder
func NewGraphBuilder() GraphBuilder {
	return &graphBuilder{
		logger: logging.GetLogger("graph.sync.builder"),
	}
}

// NewGraphBuilderWithClient creates a new graph builder with client access
func NewGraphBuilderWithClient(client graph.Client) GraphBuilder {
	// Create resource lookup adapter
	lookup := extractors.NewGraphClientLookup(client)

	// Create extractor registry
	registry := extractors.NewExtractorRegistry(lookup)

	// Register native K8s extractors (priority 50-99)
	registry.Register(native.NewServiceExtractor())       // Service→Pod SELECTS
	registry.Register(native.NewIngressExtractor())       // Ingress→Service REFERENCES_SPEC
	registry.Register(native.NewNetworkPolicyExtractor()) // NetworkPolicy→Pod SELECTS

	// Register CRD extractors (priority 100+)
	registry.Register(extractors.NewRBACExtractor())

	// Flux extractors (priority 10-15)
	registry.Register(extractors.NewFluxGitRepositoryExtractor())   // GitRepository→Secret
	registry.Register(extractors.NewFluxHelmReleaseExtractor())     // HelmRelease→Secret, MANAGES
	registry.Register(extractors.NewFluxKustomizationExtractor())   // Kustomization→GitRepository, MANAGES
	registry.Register(extractors.NewFluxManagedResourceExtractor()) // Reverse lookup for Flux-managed resources

	// ArgoCD extractors (priority 20)
	registry.Register(argocd.NewArgoCDApplicationExtractor()) // Application→Secret, MANAGES

	// Gateway API extractors (priority 100)
	registry.Register(gateway.NewGatewayExtractor())   // Gateway→GatewayClass REFERENCES_SPEC
	registry.Register(gateway.NewHTTPRouteExtractor()) // HTTPRoute→Gateway, HTTPRoute→Service REFERENCES_SPEC

	// Secrets & Certs extractors (priority 200)
	registry.Register(certmanager.NewCertificateExtractor())        // Certificate→Issuer/ClusterIssuer, Certificate→Secret
	registry.Register(externalsecrets.NewExternalSecretExtractor()) // ExternalSecret→SecretStore/ClusterSecretStore, ExternalSecret→Secret

	return &graphBuilder{
		logger:            logging.GetLogger("graph.sync.builder"),
		client:            client,
		extractorRegistry: registry,
	}
}

// BuildResourceNodes creates just the resource and event nodes (Phase 1 of two-phase processing)
// This method creates the ResourceIdentity and ChangeEvent/K8sEvent nodes along with their
// immediate structural edges (CHANGED, EMITTED_EVENT). It does NOT extract relationship edges.
func (b *graphBuilder) BuildResourceNodes(event models.Event) (*GraphUpdate, error) {
	update := &GraphUpdate{
		SourceEventID: event.ID,
		Timestamp:     time.Now(),
		ResourceNodes: []graph.ResourceIdentity{},
		EventNodes:    []graph.ChangeEvent{},
		K8sEventNodes: []graph.K8sEvent{},
		Edges:         []graph.Edge{},
	}

	// Create ResourceIdentity node
	resourceNode := b.buildResourceIdentityNode(event)
	update.ResourceNodes = append(update.ResourceNodes, resourceNode)

	// Create ChangeEvent node (unless this is a K8s Event object)
	if event.Resource.Kind != "Event" {
		changeEventNode, err := b.buildChangeEventNode(event)
		if err != nil {
			return nil, fmt.Errorf("failed to build change event: %w", err)
		}
		update.EventNodes = append(update.EventNodes, changeEventNode)

		// Create CHANGED edge (Resource → ChangeEvent)
		changedEdge := b.buildChangedEdge(event.Resource.UID, event.ID)
		update.Edges = append(update.Edges, changedEdge)
	} else {
		// This is a K8s Event object - create K8sEvent node
		k8sEventNode, err := b.buildK8sEventNode(event)
		if err != nil {
			return nil, fmt.Errorf("failed to build k8s event: %w", err)
		}
		update.K8sEventNodes = append(update.K8sEventNodes, k8sEventNode)

		// Create EMITTED_EVENT edge if InvolvedObjectUID is present
		if event.Resource.InvolvedObjectUID != "" {
			// Extract involvedObject metadata and create ResourceIdentity node if possible
			// This ensures the target node exists for the edge, even if we haven't seen
			// a CREATE/UPDATE event for the resource yet (common for long-lived resources)
			if involvedResource := b.extractInvolvedObjectMetadata(event); involvedResource != nil {
				update.ResourceNodes = append(update.ResourceNodes, *involvedResource)
			}

			emittedEdge := b.buildEmittedEventEdge(event.Resource.InvolvedObjectUID, event.ID)
			update.Edges = append(update.Edges, emittedEdge)
		}
	}

	return update, nil
}

// BuildRelationshipEdges extracts relationship edges only (Phase 2 of two-phase processing)
// This method runs extractors to create edges between resources. It assumes all resources
// in the current batch have already been written to the graph by Phase 1.
func (b *graphBuilder) BuildRelationshipEdges(ctx context.Context, event models.Event) (*GraphUpdate, error) {
	update := &GraphUpdate{
		SourceEventID: event.ID,
		Timestamp:     time.Now(),
		Edges:         []graph.Edge{},
	}

	// Extract relationships from the resource data
	relationships, err := b.ExtractRelationships(ctx, event)
	if err != nil {
		b.logger.Warn("Failed to extract relationships from event %s: %v", event.ID, err)
		return update, nil // Return empty update rather than error
	}

	update.Edges = append(update.Edges, relationships...)
	return update, nil
}

// BuildFromEvent creates graph nodes/edges from a Spectre event (combines both phases)
// This method is kept for backward compatibility and single-event processing scenarios.
// For batch processing, use BuildResourceNodes + BuildRelationshipEdges separately.
func (b *graphBuilder) BuildFromEvent(ctx context.Context, event models.Event) (*GraphUpdate, error) {
	// Phase 1: Build resource nodes
	nodeUpdate, err := b.BuildResourceNodes(event)
	if err != nil {
		return nil, err
	}

	// Phase 2: Extract relationships
	edgeUpdate, err := b.BuildRelationshipEdges(ctx, event)
	if err != nil {
		return nil, err
	}

	// Combine both updates
	nodeUpdate.Edges = append(nodeUpdate.Edges, edgeUpdate.Edges...)
	return nodeUpdate, nil
}

// BuildFromBatch processes multiple events and returns graph updates
func (b *graphBuilder) BuildFromBatch(ctx context.Context, events []models.Event) ([]*GraphUpdate, error) {
	updates := make([]*GraphUpdate, 0, len(events))

	for _, event := range events {
		update, err := b.BuildFromEvent(ctx, event)
		if err != nil {
			b.logger.Warn("Failed to build update for event %s: %v", event.ID, err)
			continue
		}
		updates = append(updates, update)
	}

	return updates, nil
}

// ExtractRelationships extracts relationships from resource data
func (b *graphBuilder) ExtractRelationships(ctx context.Context, event models.Event) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	// Skip if no data
	if len(event.Data) == 0 {
		return edges, nil
	}

	// Parse resource data
	var resourceData map[string]interface{}
	if err := json.Unmarshal(event.Data, &resourceData); err != nil {
		return nil, fmt.Errorf("failed to parse resource data: %w", err)
	}

	// Extract ownership relationships (ownerReferences)
	ownershipEdges := b.extractOwnershipRelationships(event.Resource.UID, resourceData)
	edges = append(edges, ownershipEdges...)

	// Extract selector relationships (for Services, Deployments, etc.)
	// Services and Deployments select Pods based on label selectors
	if event.Resource.Kind == "Service" || event.Resource.Kind == "Deployment" || event.Resource.Kind == "ReplicaSet" || event.Resource.Kind == "StatefulSet" || event.Resource.Kind == "DaemonSet" {
		selectsEdges := b.extractSelectorRelationships(event.Resource.UID, event.Resource.Kind, resourceData)
		edges = append(edges, selectsEdges...)
	}

	// Extract scheduling relationships (Pod → Node)
	if event.Resource.Kind == "Pod" {
		if schedEdge := b.extractSchedulingRelationship(event.Resource.UID, resourceData); schedEdge != nil {
			edges = append(edges, *schedEdge)
		}
	}

	// Extract volume relationships (Pod → PVC)
	if event.Resource.Kind == "Pod" {
		volumeEdges := b.extractVolumeRelationships(event.Resource.UID, resourceData)
		edges = append(edges, volumeEdges...)
	}

	// Extract ServiceAccount relationship
	if event.Resource.Kind == "Pod" {
		if saEdge := b.extractServiceAccountRelationship(event.Resource.UID, resourceData); saEdge != nil {
			edges = append(edges, *saEdge)
		}
	}

	// NEW: Apply custom resource extractors
	if b.extractorRegistry != nil {
		crEdges, err := b.extractorRegistry.Extract(ctx, event)
		if err != nil {
			b.logger.Warn("Custom resource extraction failed for event %s: %v", event.ID, err)
		} else {
			b.logger.Debug("Custom resource extractors produced %d edges", len(crEdges))
			edges = append(edges, crEdges...)
		}
	}

	return edges, nil
}

// buildResourceIdentityNode creates a ResourceIdentity node from an event
func (b *graphBuilder) buildResourceIdentityNode(event models.Event) graph.ResourceIdentity {
	now := time.Now().UnixNano()
	deleted := event.Type == models.EventTypeDelete

	// Extract labels from event data
	labels := b.extractLabels(event)

	resource := graph.ResourceIdentity{
		UID:       event.Resource.UID,
		Kind:      event.Resource.Kind,
		APIGroup:  event.Resource.Group,
		Version:   event.Resource.Version,
		Namespace: event.Resource.Namespace,
		Name:      event.Resource.Name,
		Labels:    labels,
		FirstSeen: now,
		LastSeen:  now,
		Deleted:   deleted,
		DeletedAt: func() int64 {
			if deleted {
				return event.Timestamp
			}
			return 0
		}(),
	}

	if deleted {
		b.logger.Debug("Building ResourceIdentity for DELETE event: %s/%s uid=%s",
			resource.Kind, resource.Name, resource.UID)
	}

	return resource
}

// extractLabels extracts labels from the event's resource data
func (b *graphBuilder) extractLabels(event models.Event) map[string]string {
	if len(event.Data) == 0 {
		return nil
	}

	var resourceData map[string]interface{}
	if err := json.Unmarshal(event.Data, &resourceData); err != nil {
		b.logger.Debug("Failed to parse resource data for label extraction: %v", err)
		return nil
	}

	// Extract metadata.labels
	metadata, ok := resourceData["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}

	labelsRaw, ok := metadata["labels"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Convert to map[string]string
	labels := make(map[string]string)
	for key, value := range labelsRaw {
		if strValue, ok := value.(string); ok {
			labels[key] = strValue
		}
	}

	return labels
}

// extractInvolvedObjectMetadata extracts involvedObject metadata from a K8s Event
// and creates a ResourceIdentity node for it. This ensures that EMITTED_EVENT edges
// have a valid target node even if we haven't seen CREATE/UPDATE events for the resource.
func (b *graphBuilder) extractInvolvedObjectMetadata(event models.Event) *graph.ResourceIdentity {
	if len(event.Data) == 0 {
		return nil
	}

	var eventData map[string]interface{}
	if err := json.Unmarshal(event.Data, &eventData); err != nil {
		b.logger.Debug("Failed to parse event data for involvedObject extraction: %v", err)
		return nil
	}

	// Extract involvedObject from the Event
	involvedObj, ok := eventData["involvedObject"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Extract required fields
	uid, _ := involvedObj["uid"].(string)
	kind, _ := involvedObj["kind"].(string)
	name, _ := involvedObj["name"].(string)
	namespace, _ := involvedObj["namespace"].(string)
	apiVersion, _ := involvedObj["apiVersion"].(string)

	// Validate required fields
	if uid == "" || kind == "" || name == "" || apiVersion == "" {
		b.logger.Debug("Incomplete involvedObject metadata in event %s", event.ID)
		return nil
	}

	// Split apiVersion into group and version
	group, version := "", apiVersion
	if idx := strings.Index(apiVersion, "/"); idx != -1 {
		group = apiVersion[:idx]
		version = apiVersion[idx+1:]
	}

	// Create ResourceIdentity node
	// Use event timestamp as both firstSeen and lastSeen since we don't have resource events
	return &graph.ResourceIdentity{
		UID:       uid,
		Kind:      kind,
		APIGroup:  group,
		Version:   version,
		Namespace: namespace,
		Name:      name,
		Labels:    map[string]string{}, // We don't have labels from involvedObject
		FirstSeen: event.Timestamp,
		LastSeen:  event.Timestamp,
		Deleted:   false,
		DeletedAt: 0,
	}
}

// buildChangeEventNode creates a ChangeEvent node from an event
func (b *graphBuilder) buildChangeEventNode(event models.Event) (graph.ChangeEvent, error) {
	// Parse resource data to extract status and errors
	var resourceData *analyzer.ResourceData
	var err error

	if len(event.Data) > 0 {
		resourceData, err = analyzer.ParseResourceData(event.Data)
		if err != nil {
			b.logger.Warn("Failed to parse resource data for event %s: %v", event.ID, err)
		}
	}

	// Infer status
	status := analyzer.InferStatusFromParsedData(event.Resource.Kind, resourceData, string(event.Type))

	// Extract error messages
	errorMessage := ""
	if len(event.Data) > 0 {
		errors := analyzer.InferErrorMessages(event.Resource.Kind, event.Data, status)
		if len(errors) > 0 {
			errorMessage = errors[0] // Use first error message
		}
	}

	// Extract container issues
	containerIssues := []string{}
	if len(event.Data) > 0 && event.Resource.Kind == "Pod" {
		if issues, err := analyzer.GetContainerIssuesFromJSON(event.Data); err == nil && len(issues) > 0 {
			for _, issue := range issues {
				containerIssues = append(containerIssues, issue.Reason)
			}
		}
	}

	// Detect what changed
	configChanged := false
	statusChanged := false
	replicasChanged := false

	if event.Type == models.EventTypeUpdate {
		// For UPDATE events, compare with previous state to detect what changed
		configChanged, statusChanged, replicasChanged = b.detectChanges(event, resourceData)
	}

	return graph.ChangeEvent{
		ID:              event.ID,
		Timestamp:       event.Timestamp,
		EventType:       string(event.Type),
		Status:          status,
		ErrorMessage:    errorMessage,
		ContainerIssues: containerIssues,
		ConfigChanged:   configChanged,
		StatusChanged:   statusChanged,
		ReplicasChanged: replicasChanged,
		ImpactScore:     b.calculateImpactScore(status, containerIssues),
		Data:            string(event.Data), // Store full resource data for timeline reconstruction
	}, nil
}

// detectChanges compares current event with previous event to detect what changed
func (b *graphBuilder) detectChanges(event models.Event, currentData *analyzer.ResourceData) (configChanged, statusChanged, replicasChanged bool) {
	// If no client available, fall back to conservative detection
	if b.client == nil {
		if currentData != nil {
			statusChanged = true // Assume status might have changed
		}
		return configChanged, statusChanged, replicasChanged
	}

	// Query for the previous event for this resource, including its data
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b.logger.Debug("Querying for previous event: resourceUID=%s, timestamp=%d", event.Resource.UID, event.Timestamp)

	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $resourceUID})-[:CHANGED]->(ce:ChangeEvent)
			WHERE ce.timestamp < $currentTimestamp
			  AND ce.eventType IN ["CREATE", "UPDATE"]
			RETURN ce.data, ce.timestamp
			ORDER BY ce.timestamp DESC
			LIMIT 1
		`,
		Parameters: map[string]interface{}{
			"resourceUID":      event.Resource.UID,
			"currentTimestamp": event.Timestamp,
		},
	}

	result, err := b.client.ExecuteQuery(ctx, query)
	if err != nil {
		b.logger.Debug("Failed to query previous event for resource %s: %v", event.Resource.UID, err)
		// For first event (CREATE), or if we can't determine, assume nothing changed from previous
		if currentData != nil {
			statusChanged = true // Conservative: assume status might have changed
		}
		return configChanged, statusChanged, replicasChanged
	}

	if len(result.Rows) == 0 {
		b.logger.Debug("No previous event found for resource %s (this is likely the first event)", event.Resource.UID)
		// For first event (CREATE), or if we can't determine, assume nothing changed from previous
		if currentData != nil {
			statusChanged = true // Conservative: assume status might have changed
		}
		return configChanged, statusChanged, replicasChanged
	}

	b.logger.Debug("Previous event found for resource %s", event.Resource.UID)

	// Previous event exists - now we need to compare spec vs status
	// Parse current event data
	if len(event.Data) == 0 {
		b.logger.Debug("Current event has no data for resource %s, skipping change detection", event.Resource.UID)
		return configChanged, statusChanged, replicasChanged
	}

	var currentResource map[string]interface{}
	if err := json.Unmarshal(event.Data, &currentResource); err != nil {
		b.logger.Debug("Failed to parse current event data for change detection: %v", err)
		statusChanged = true // Conservative fallback
		return configChanged, statusChanged, replicasChanged
	}

	// Parse previous event data with null/empty checks
	var previousResource map[string]interface{}

	// Check if result row exists and has at least one element
	if len(result.Rows) > 0 && len(result.Rows[0]) > 0 {
		// First column is the data (ce.data)
		if dataValue := result.Rows[0][0]; dataValue != nil {
			if dataStr, ok := dataValue.(string); ok && dataStr != "" {
				if err := json.Unmarshal([]byte(dataStr), &previousResource); err != nil {
					b.logger.Debug("Failed to parse previous event data for resource %s: %v", event.Resource.UID, err)
					// Continue with empty previousResource - we'll handle this gracefully
				} else {
					b.logger.Debug("Successfully parsed previous event data for resource %s", event.Resource.UID)
				}
			} else {
				b.logger.Debug("Previous event data is empty or not a string for resource %s", event.Resource.UID)
			}
		} else {
			b.logger.Debug("Previous event data is nil for resource %s", event.Resource.UID)
		}
	}

	// Extract generations from both current and previous events
	var currentGeneration float64 = 0
	var previousGeneration float64 = 0

	if metadata, ok := currentResource["metadata"].(map[string]interface{}); ok {
		if gen, ok := metadata["generation"].(float64); ok {
			currentGeneration = gen
		}
	}

	if len(previousResource) > 0 {
		if metadata, ok := previousResource["metadata"].(map[string]interface{}); ok {
			if gen, ok := metadata["generation"].(float64); ok {
				previousGeneration = gen
			}
		}
	}

	b.logger.Debug("Generation comparison for resource %s: current=%v, previous=%v", event.Resource.UID, currentGeneration, previousGeneration)

	// Config change detection: compare generations
	// Generation increments when spec changes, so if current > previous, config changed
	if currentGeneration > previousGeneration {
		configChanged = true
		b.logger.Debug("Config change detected for resource %s: generation increased from %v to %v", event.Resource.UID, previousGeneration, currentGeneration)
	} else {
		b.logger.Debug("No config change detected for resource %s: current generation %v is not greater than previous %v", event.Resource.UID, currentGeneration, previousGeneration)
	}

	// Detect status changes
	// If status field exists and has changed, statusChanged = true
	// For now, we use a conservative approach: if status exists, assume it might have changed
	if currentResource["status"] != nil {
		statusChanged = true
	}

	// Detect replica changes (for Deployments, StatefulSets, etc.)
	if currentSpec, hasCurrentSpec := currentResource["spec"]; hasCurrentSpec {
		if specMap, ok := currentSpec.(map[string]interface{}); ok {
			if replicas, ok := specMap["replicas"].(float64); ok && replicas >= 0 {
				// We have replicas field, but need previous value to compare
				// For now, mark as potentially changed
				replicasChanged = false // Can't determine without previous value
			}
		}
	}

	return configChanged, statusChanged, replicasChanged
}

// buildK8sEventNode creates a K8sEvent node from a Kubernetes Event object
func (b *graphBuilder) buildK8sEventNode(event models.Event) (graph.K8sEvent, error) {
	var eventData map[string]interface{}
	if err := json.Unmarshal(event.Data, &eventData); err != nil {
		return graph.K8sEvent{}, fmt.Errorf("failed to parse event data: %w", err)
	}

	// Extract event fields
	reason := ""
	message := ""
	eventType := ""
	count := 0
	source := ""

	if r, ok := eventData["reason"].(string); ok {
		reason = r
	}
	if m, ok := eventData["message"].(string); ok {
		message = m
	}
	if t, ok := eventData["type"].(string); ok {
		eventType = t
	}
	if c, ok := eventData["count"].(float64); ok {
		count = int(c)
	}
	if src, ok := eventData["source"].(map[string]interface{}); ok {
		if comp, ok := src["component"].(string); ok {
			source = comp
		}
	}

	return graph.K8sEvent{
		ID:        event.ID,
		Timestamp: event.Timestamp,
		Reason:    reason,
		Message:   message,
		Type:      eventType,
		Count:     count,
		Source:    source,
	}, nil
}

// buildChangedEdge creates a CHANGED edge
func (b *graphBuilder) buildChangedEdge(resourceUID, eventID string) graph.Edge {
	props := graph.ChangedEdge{
		SequenceNumber: 0, // Will be updated by causality engine
	}

	propsJSON, _ := json.Marshal(props)

	return graph.Edge{
		Type:       graph.EdgeTypeChanged,
		FromUID:    resourceUID,
		ToUID:      eventID,
		Properties: propsJSON,
	}
}

// buildEmittedEventEdge creates an EMITTED_EVENT edge
func (b *graphBuilder) buildEmittedEventEdge(resourceUID, k8sEventID string) graph.Edge {
	return graph.Edge{
		Type:       graph.EdgeTypeEmittedEvent,
		FromUID:    resourceUID,
		ToUID:      k8sEventID,
		Properties: json.RawMessage("{}"),
	}
}

// extractOwnershipRelationships extracts OWNS edges from ownerReferences
func (b *graphBuilder) extractOwnershipRelationships(ownedUID string, resourceData map[string]interface{}) []graph.Edge {
	edges := []graph.Edge{}

	metadata, ok := resourceData["metadata"].(map[string]interface{})
	if !ok {
		return edges
	}

	ownerRefsRaw, ok := metadata["ownerReferences"]
	if !ok {
		return edges
	}

	ownerRefs, ok := ownerRefsRaw.([]interface{})
	if !ok {
		return edges
	}

	for _, refRaw := range ownerRefs {
		ref, ok := refRaw.(map[string]interface{})
		if !ok {
			continue
		}

		ownerUID, ok := ref["uid"].(string)
		if !ok {
			continue
		}

		controller := false
		if ctrl, ok := ref["controller"].(bool); ok {
			controller = ctrl
		}

		blockOwnerDeletion := false
		if bod, ok := ref["blockOwnerDeletion"].(bool); ok {
			blockOwnerDeletion = bod
		}

		props := graph.OwnsEdge{
			Controller:         controller,
			BlockOwnerDeletion: blockOwnerDeletion,
		}

		propsJSON, _ := json.Marshal(props)

		edge := graph.Edge{
			Type:       graph.EdgeTypeOwns,
			FromUID:    ownerUID,
			ToUID:      ownedUID,
			Properties: propsJSON,
		}

		edges = append(edges, edge)
	}

	return edges
}

// extractSelectorRelationships extracts SELECTS edges for Services, Deployments, etc. → Pods
func (b *graphBuilder) extractSelectorRelationships(selectorUID, kind string, resourceData map[string]interface{}) []graph.Edge {
	edges := []graph.Edge{}

	// Need client to query for matching Pods
	if b.client == nil {
		return edges
	}

	// Extract selector labels based on resource kind
	var selector map[string]string
	var namespace string

	metadata, ok := resourceData["metadata"].(map[string]interface{})
	if !ok {
		return edges
	}

	namespace, _ = metadata["namespace"].(string)

	spec, ok := resourceData["spec"].(map[string]interface{})
	if !ok {
		return edges
	}

	// Different resources have selectors in different places
	switch kind {
	case "Service":
		// Service selector is at spec.selector
		if selectorRaw, ok := spec["selector"].(map[string]interface{}); ok {
			selector = make(map[string]string)
			for k, v := range selectorRaw {
				if strVal, ok := v.(string); ok {
					selector[k] = strVal
				}
			}
		}

	case "Deployment", "ReplicaSet", "StatefulSet", "DaemonSet":
		// These resources use spec.selector.matchLabels
		if selectorRaw, ok := spec["selector"].(map[string]interface{}); ok {
			if matchLabels, ok := selectorRaw["matchLabels"].(map[string]interface{}); ok {
				selector = make(map[string]string)
				for k, v := range matchLabels {
					if strVal, ok := v.(string); ok {
						selector[k] = strVal
					}
				}
			}
		}
	}

	// If no selector, nothing to do
	if len(selector) == 0 {
		return edges
	}

	// Query for Pods matching these labels
	matchingPodUIDs, err := b.findPodsMatchingLabels(context.Background(), selector, namespace)
	if err != nil {
		b.logger.Debug("Failed to find Pods matching selector for %s %s: %v", kind, selectorUID, err)
		return edges
	}

	// Create SELECTS edges for each matching Pod
	selectorLabelsJSON, _ := json.Marshal(selector)
	for _, podUID := range matchingPodUIDs {
		propsJSON, _ := json.Marshal(graph.SelectsEdge{
			SelectorLabels: selector,
		})

		edge := graph.Edge{
			Type:       graph.EdgeTypeSelects,
			FromUID:    selectorUID,
			ToUID:      podUID,
			Properties: json.RawMessage(propsJSON),
		}

		edges = append(edges, edge)
		b.logger.Debug("Created SELECTS edge: %s %s → Pod %s (selector: %s)", kind, selectorUID, podUID, string(selectorLabelsJSON))
	}

	return edges
}

// findPodsMatchingLabels queries the graph for Pods with labels matching the selector
func (b *graphBuilder) findPodsMatchingLabels(ctx context.Context, selector map[string]string, namespace string) ([]string, error) {
	// Build a Cypher query to find Pods with matching labels
	// For simplicity, we'll query for Pods and check if all selector labels match
	// Note: This requires labels to be stored in the graph - which happens via metadata

	// Build WHERE clause for label matching
	// We need to check the Pod's metadata.labels matches all selector labels
	// Since labels are stored as JSON in the Pod resource data, we'll query for Pods
	// in the namespace and return their UIDs, then we'd need to check labels

	// For now, we'll use a simpler approach: query all Pods in namespace
	// and rely on the fact that labels are part of the resource name/metadata
	// This is a limitation - proper implementation would require storing labels separately

	// Simplified query: Find all Pods in the namespace
	// In a production system, you'd want to denormalize labels into node properties
	var query graph.GraphQuery

	if namespace != "" {
		query = graph.GraphQuery{
			Query: `
				MATCH (p:ResourceIdentity {kind: $kind, namespace: $namespace})
				WHERE NOT p.deleted
				RETURN p.uid as uid
				LIMIT 100
			`,
			Parameters: map[string]interface{}{
				"kind":      "Pod",
				"namespace": namespace,
			},
		}
	} else {
		query = graph.GraphQuery{
			Query: `
				MATCH (p:ResourceIdentity {kind: $kind})
				WHERE NOT p.deleted
				RETURN p.uid as uid
				LIMIT 100
			`,
			Parameters: map[string]interface{}{
				"kind": "Pod",
			},
		}
	}

	result, err := b.client.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Extract UIDs from result
	var podUIDs []string
	for _, row := range result.Rows {
		if len(row) > 0 {
			if uid, ok := row[0].(string); ok {
				podUIDs = append(podUIDs, uid)
			}
		}
	}

	// NOTE: This returns all Pods in the namespace without label filtering
	// Label filtering is now implemented in the custom extractors
	// (e.g., NetworkPolicyExtractor, ServiceExtractor) which query labels properly
	//
	// For now, we return all Pods in the namespace as potential matches
	// The graph will be eventually consistent as Pods get created/updated

	b.logger.Debug("Found %d Pod matches in namespace %s", len(podUIDs), namespace)

	return podUIDs, nil
}

// extractSchedulingRelationship extracts SCHEDULED_ON edge for Pod → Node
func (b *graphBuilder) extractSchedulingRelationship(podUID string, resourceData map[string]interface{}) *graph.Edge {
	spec, ok := resourceData["spec"].(map[string]interface{})
	if !ok {
		return nil
	}

	nodeName, ok := spec["nodeName"].(string)
	if !ok || nodeName == "" {
		return nil
	}

	// If we don't have a client, we can't query for the Node UID
	if b.client == nil {
		b.logger.Debug("No graph client available, cannot create SCHEDULED_ON edge for Pod %s → Node %s", podUID, nodeName)
		return nil
	}

	// Query the graph for the Node with this name
	nodeUID, err := b.findNodeUIDByName(context.Background(), nodeName)
	if err != nil {
		b.logger.Debug("Failed to find Node UID for name %s: %v", nodeName, err)
		return nil
	}

	if nodeUID == "" {
		b.logger.Debug("Node %s not found in graph yet, skipping SCHEDULED_ON edge", nodeName)
		return nil
	}

	// Get the scheduled timestamp from Pod status
	var scheduledAt int64
	if status, ok := resourceData["status"].(map[string]interface{}); ok {
		if conditions, ok := status["conditions"].([]interface{}); ok {
			for _, condRaw := range conditions {
				if cond, ok := condRaw.(map[string]interface{}); ok {
					if condType, ok := cond["type"].(string); ok && condType == "PodScheduled" {
						if lastTransitionTime, ok := cond["lastTransitionTime"].(string); ok {
							if t, err := time.Parse(time.RFC3339, lastTransitionTime); err == nil {
								scheduledAt = t.UnixNano()
							}
						}
					}
				}
			}
		}
	}

	// If we couldn't find the scheduled time, use current time
	if scheduledAt == 0 {
		scheduledAt = time.Now().UnixNano()
	}

	// Create the edge properties
	propsJSON, _ := json.Marshal(graph.ScheduledOnEdge{
		ScheduledAt:  scheduledAt,
		TerminatedAt: 0, // Will be updated if Pod is terminated
	})

	edge := graph.Edge{
		Type:       graph.EdgeTypeScheduledOn,
		FromUID:    podUID,
		ToUID:      nodeUID,
		Properties: json.RawMessage(propsJSON),
	}

	b.logger.Debug("Created SCHEDULED_ON edge: Pod %s → Node %s (name: %s)", podUID, nodeUID, nodeName)
	return &edge
}

// findNodeUIDByName queries the graph for a Node with the given name and returns its UID
func (b *graphBuilder) findNodeUIDByName(ctx context.Context, nodeName string) (string, error) {
	return b.findResourceUIDByName(ctx, "Node", nodeName, "")
}

// findPVCUIDByName queries the graph for a PVC with the given name and namespace and returns its UID
func (b *graphBuilder) findPVCUIDByName(ctx context.Context, pvcName, namespace string) (string, error) {
	return b.findResourceUIDByName(ctx, "PersistentVolumeClaim", pvcName, namespace)
}

// findServiceAccountUIDByName queries the graph for a ServiceAccount with the given name and namespace and returns its UID
func (b *graphBuilder) findServiceAccountUIDByName(ctx context.Context, saName, namespace string) (string, error) {
	return b.findResourceUIDByName(ctx, "ServiceAccount", saName, namespace)
}

// findResourceUIDByName is a generic helper to find a resource UID by kind, name, and optionally namespace
func (b *graphBuilder) findResourceUIDByName(ctx context.Context, kind, name, namespace string) (string, error) {
	// Build query based on whether namespace is required
	var query graph.GraphQuery
	if namespace != "" {
		query = graph.GraphQuery{
			Query: `
				MATCH (n:ResourceIdentity {kind: $kind, name: $name, namespace: $namespace})
				RETURN n.uid as uid
				LIMIT 1
			`,
			Parameters: map[string]interface{}{
				"kind":      kind,
				"name":      name,
				"namespace": namespace,
			},
		}
	} else {
		query = graph.GraphQuery{
			Query: `
				MATCH (n:ResourceIdentity {kind: $kind, name: $name})
				RETURN n.uid as uid
				LIMIT 1
			`,
			Parameters: map[string]interface{}{
				"kind": kind,
				"name": name,
			},
		}
	}

	result, err := b.client.ExecuteQuery(ctx, query)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}

	// Check if we got a result
	if len(result.Rows) == 0 || len(result.Rows[0]) == 0 {
		return "", nil // Resource not found yet
	}

	// Extract the UID from the result
	if uid, ok := result.Rows[0][0].(string); ok {
		return uid, nil
	}

	return "", fmt.Errorf("unexpected result type: %T", result.Rows[0][0])
}

// extractVolumeRelationships extracts MOUNTS edges for Pod → PVC
func (b *graphBuilder) extractVolumeRelationships(podUID string, resourceData map[string]interface{}) []graph.Edge {
	edges := []graph.Edge{}

	// Need client for PVC lookups
	if b.client == nil {
		return edges
	}

	spec, ok := resourceData["spec"].(map[string]interface{})
	if !ok {
		return edges
	}

	// Get Pod namespace for PVC lookup
	metadata, ok := resourceData["metadata"].(map[string]interface{})
	if !ok {
		return edges
	}

	namespace, ok := metadata["namespace"].(string)
	if !ok || namespace == "" {
		return edges
	}

	volumesRaw, ok := spec["volumes"]
	if !ok {
		return edges
	}

	volumes, ok := volumesRaw.([]interface{})
	if !ok {
		return edges
	}

	// Get volume mounts for mount path information
	volumeMounts := b.extractVolumeMounts(resourceData)

	for _, volRaw := range volumes {
		vol, ok := volRaw.(map[string]interface{})
		if !ok {
			continue
		}

		volumeName, _ := vol["name"].(string)

		pvcRaw, ok := vol["persistentVolumeClaim"]
		if !ok {
			continue
		}

		pvc, ok := pvcRaw.(map[string]interface{})
		if !ok {
			continue
		}

		claimName, ok := pvc["claimName"].(string)
		if !ok || claimName == "" {
			continue
		}

		// Look up PVC UID by name and namespace
		pvcUID, err := b.findPVCUIDByName(context.Background(), claimName, namespace)
		if err != nil {
			b.logger.Debug("Failed to find PVC UID for %s/%s: %v", namespace, claimName, err)
			continue
		}

		if pvcUID == "" {
			b.logger.Debug("PVC %s/%s not found in graph yet, skipping MOUNTS edge", namespace, claimName)
			continue
		}

		// Get mount path if available
		mountPath := volumeMounts[volumeName]

		// Create MOUNTS edge
		propsJSON, _ := json.Marshal(graph.MountsEdge{
			VolumeName: volumeName,
			MountPath:  mountPath,
		})

		edge := graph.Edge{
			Type:       graph.EdgeTypeMounts,
			FromUID:    podUID,
			ToUID:      pvcUID,
			Properties: json.RawMessage(propsJSON),
		}

		edges = append(edges, edge)
		b.logger.Debug("Created MOUNTS edge: Pod %s → PVC %s (name: %s/%s)", podUID, pvcUID, namespace, claimName)
	}

	return edges
}

// extractVolumeMounts extracts mount paths from Pod containers
func (b *graphBuilder) extractVolumeMounts(resourceData map[string]interface{}) map[string]string {
	mounts := make(map[string]string)

	spec, ok := resourceData["spec"].(map[string]interface{})
	if !ok {
		return mounts
	}

	containersRaw, ok := spec["containers"]
	if !ok {
		return mounts
	}

	containers, ok := containersRaw.([]interface{})
	if !ok {
		return mounts
	}

	for _, contRaw := range containers {
		cont, ok := contRaw.(map[string]interface{})
		if !ok {
			continue
		}

		volumeMountsRaw, ok := cont["volumeMounts"]
		if !ok {
			continue
		}

		volumeMounts, ok := volumeMountsRaw.([]interface{})
		if !ok {
			continue
		}

		for _, vmRaw := range volumeMounts {
			vm, ok := vmRaw.(map[string]interface{})
			if !ok {
				continue
			}

			name, _ := vm["name"].(string)
			mountPath, _ := vm["mountPath"].(string)

			if name != "" && mountPath != "" {
				mounts[name] = mountPath
			}
		}
	}

	return mounts
}

// extractServiceAccountRelationship extracts USES_SERVICE_ACCOUNT edge
func (b *graphBuilder) extractServiceAccountRelationship(podUID string, resourceData map[string]interface{}) *graph.Edge {
	// Need client for ServiceAccount lookups
	if b.client == nil {
		return nil
	}

	spec, ok := resourceData["spec"].(map[string]interface{})
	if !ok {
		return nil
	}

	serviceAccountName, ok := spec["serviceAccountName"].(string)
	if !ok || serviceAccountName == "" {
		return nil
	}

	// Get Pod namespace for ServiceAccount lookup
	metadata, ok := resourceData["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}

	namespace, ok := metadata["namespace"].(string)
	if !ok || namespace == "" {
		return nil
	}

	// Look up ServiceAccount UID by name and namespace
	saUID, err := b.findServiceAccountUIDByName(context.Background(), serviceAccountName, namespace)
	if err != nil {
		b.logger.Debug("Failed to find ServiceAccount UID for %s/%s: %v", namespace, serviceAccountName, err)
		return nil
	}

	if saUID == "" {
		b.logger.Debug("ServiceAccount %s/%s not found in graph yet, skipping USES_SERVICE_ACCOUNT edge", namespace, serviceAccountName)
		return nil
	}

	// Create USES_SERVICE_ACCOUNT edge
	edge := graph.Edge{
		Type:       graph.EdgeTypeUsesServiceAccount,
		FromUID:    podUID,
		ToUID:      saUID,
		Properties: json.RawMessage("{}"),
	}

	b.logger.Debug("Created USES_SERVICE_ACCOUNT edge: Pod %s → ServiceAccount %s (name: %s/%s)", podUID, saUID, namespace, serviceAccountName)
	return &edge
}

// calculateImpactScore calculates the impact score for a change event
func (b *graphBuilder) calculateImpactScore(status string, containerIssues []string) float64 {
	score := 0.0

	// Base score on status
	switch status {
	case "Error":
		score = 0.8
	case "Warning":
		score = 0.5
	case "Terminating":
		score = 0.6
	case "Unknown":
		score = 0.3
	default:
		score = 0.1
	}

	// Increase score if there are container issues
	if len(containerIssues) > 0 {
		score += 0.2
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}
