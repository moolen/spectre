package extractors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

const (
	fluxManagedResourceExtractorName = "flux-managed-resource"
)

// FluxManagedResourceExtractor creates MANAGES edges when a resource with Flux labels is created/updated
// This complements the FluxHelmReleaseExtractor and FluxKustomizationExtractor by handling the reverse direction:
// when a managed resource is created, we create the MANAGES edge from the HelmRelease or Kustomization
type FluxManagedResourceExtractor struct {
	logger *logging.Logger
}

// NewFluxManagedResourceExtractor creates a new extractor
func NewFluxManagedResourceExtractor() *FluxManagedResourceExtractor {
	return &FluxManagedResourceExtractor{
		logger: logging.GetLogger("extractors.flux-managed-resource"),
	}
}

// Name returns the extractor name
func (e *FluxManagedResourceExtractor) Name() string {
	return fluxManagedResourceExtractorName
}

// Priority returns execution priority (higher than HelmRelease extractor to run after it)
func (e *FluxManagedResourceExtractor) Priority() int {
	return 15 // Run after FluxHelmReleaseExtractor (10)
}

// Matches checks if this extractor should run for the event
func (e *FluxManagedResourceExtractor) Matches(event models.Event) bool {
	// Only process CREATE events (not UPDATE or DELETE)
	// This extractor handles the case where a new resource is created
	// with Flux labels after the HelmRelease already exists.
	// UPDATE events are handled by FluxHelmReleaseExtractor.
	if event.Type != models.EventTypeCreate {
		return false
	}

	// Only process resources that are NOT HelmReleases or Kustomizations (they're handled by their own extractors)
	if event.Resource.Kind == "HelmRelease" || event.Resource.Kind == "Kustomization" {
		return false
	}

	// Check if the resource has Flux labels (HelmRelease or Kustomization) in its metadata
	return hasFluxLabels(event) || hasKustomizeLabels(event)
}

// ExtractRelationships extracts MANAGES edges from HelmRelease or Kustomization to this resource
func (e *FluxManagedResourceExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup ResourceLookup,
) ([]graph.Edge, error) {
	e.logger.Debug("Processing %s/%s/%s (UID: %s)",
		event.Resource.Namespace, event.Resource.Kind, event.Resource.Name, event.Resource.UID)

	// Extract labels directly from event data instead of querying the graph
	// This is more reliable during batch processing where the graph may not be up-to-date
	labels := extractLabelsFromEvent(event)
	if labels == nil {
		e.logger.Debug("Resource %s has no labels in event data", event.Resource.UID)
		return nil, nil
	}
	e.logger.Debug("Resource %s has %d labels from event data", event.Resource.UID, len(labels))

	// Build a ResourceIdentity from event data for use in edge creation
	resource := &graph.ResourceIdentity{
		UID:       event.Resource.UID,
		Kind:      event.Resource.Kind,
		Namespace: event.Resource.Namespace,
		Name:      event.Resource.Name,
		Labels:    labels,
	}

	// IMPORTANT: Skip resources that have owners (via OWNS edges) to avoid creating
	// transitive MANAGES edges. For example, we want:
	//   HelmRelease --[MANAGES]--> Deployment --[OWNS]--> ReplicaSet --[OWNS]--> Pod
	// NOT:
	//   HelmRelease --[MANAGES]--> Pod (bypassing the ownership chain)
	if hasOwner, err := e.resourceHasOwner(ctx, resource.UID, lookup); err != nil {
		e.logger.Debug("Failed to check if resource %s has owner: %v", resource.UID, err)
		// Continue anyway - better to create duplicate edges than miss valid ones
	} else if hasOwner {
		e.logger.Debug("Skipping MANAGES edge for resource %s/%s/%s - it has an owner",
			resource.Namespace, resource.Kind, resource.Name)
		return nil, nil
	}

	// Try HelmRelease labels first
	if helmReleaseName, hasName := labels[fluxNameLabel]; hasName {
		if helmReleaseNamespace, hasNamespace := labels[fluxNamespaceLabel]; hasNamespace {
			return e.createHelmReleaseEdge(ctx, resource, helmReleaseName, helmReleaseNamespace, lookup)
		}
	}

	// Try Kustomization labels
	if kustomizationName, hasName := labels[kustomizeNameLabel]; hasName {
		if kustomizationNamespace, hasNamespace := labels[kustomizeNsLabel]; hasNamespace {
			e.logger.Debug("Found Kustomization labels: %s/%s for resource %s",
				kustomizationNamespace, kustomizationName, resource.UID)
			return e.createKustomizationEdge(ctx, resource, kustomizationName, kustomizationNamespace, lookup)
		}
	}

	e.logger.Debug("No Flux labels found for resource %s", resource.UID)
	return nil, nil
}

// createFluxManagesEdge creates a MANAGES edge from a Flux controller to a managed resource
func (e *FluxManagedResourceExtractor) createFluxManagesEdge(
	ctx context.Context,
	resource *graph.ResourceIdentity,
	controllerKind, controllerName, controllerNamespace string,
	nameLabel, namespaceLabel string,
	lookup ResourceLookup,
) ([]graph.Edge, error) {
	// Find the Flux controller that manages this resource
	controller, err := lookup.FindResourceByNamespace(ctx, controllerNamespace, controllerKind, controllerName)
	if err != nil {
		e.logger.Debug("%s %s/%s not found for managed resource %s/%s: %v",
			controllerKind, controllerNamespace, controllerName, resource.Namespace, resource.Name, err)
		return nil, nil // Not an error - controller might not be synced yet
	}

	if controller == nil {
		e.logger.Debug("%s %s/%s not found for managed resource %s/%s",
			controllerKind, controllerNamespace, controllerName, resource.Namespace, resource.Name)
		return nil, nil // Controller not synced yet
	}

	// Create MANAGES edge from Flux controller to this resource
	timestamp := time.Now().UnixNano()
	props := graph.ManagesEdge{
		Confidence: 1.0, // Perfect confidence from Flux labels
		Evidence: []graph.EvidenceItem{
			{
				Type:       graph.EvidenceTypeLabel,
				Value:      fmt.Sprintf("Flux %s labels: %s=%s, %s=%s", controllerKind, nameLabel, controllerName, namespaceLabel, controllerNamespace),
				Weight:     1.0,
				Timestamp:  timestamp,
				Key:        nameLabel,
				MatchValue: controllerName,
			},
		},
		FirstObserved:   timestamp,
		LastValidated:   timestamp,
		ValidationState: graph.ValidationStateValid,
	}

	propsJSON, _ := json.Marshal(props)

	edge := graph.Edge{
		Type:       graph.EdgeTypeManages,
		FromUID:    controller.UID,
		ToUID:      resource.UID,
		Properties: propsJSON,
	}

	e.logger.Debug("Created MANAGES edge from %s %s/%s to %s %s/%s (UID: %s)",
		controllerKind, controllerNamespace, controllerName, resource.Kind, resource.Namespace, resource.Name, resource.UID)

	return []graph.Edge{edge}, nil
}

// createHelmReleaseEdge creates a MANAGES edge from HelmRelease to resource
func (e *FluxManagedResourceExtractor) createHelmReleaseEdge(
	ctx context.Context,
	resource *graph.ResourceIdentity,
	helmReleaseName, helmReleaseNamespace string,
	lookup ResourceLookup,
) ([]graph.Edge, error) {
	return e.createFluxManagesEdge(ctx, resource, "HelmRelease", helmReleaseName, helmReleaseNamespace,
		fluxNameLabel, fluxNamespaceLabel, lookup)
}

// createKustomizationEdge creates a MANAGES edge from Kustomization to resource
func (e *FluxManagedResourceExtractor) createKustomizationEdge(
	ctx context.Context,
	resource *graph.ResourceIdentity,
	kustomizationName, kustomizationNamespace string,
	lookup ResourceLookup,
) ([]graph.Edge, error) {
	return e.createFluxManagesEdge(ctx, resource, "Kustomization", kustomizationName, kustomizationNamespace,
		kustomizeNameLabel, kustomizeNsLabel, lookup)
}

// resourceHasOwner checks if the resource has any incoming OWNS edges
func (e *FluxManagedResourceExtractor) resourceHasOwner(
	ctx context.Context,
	resourceUID string,
	lookup ResourceLookup,
) (bool, error) {
	query := graph.GraphQuery{
		Query: `
			MATCH (:ResourceIdentity)-[:OWNS]->(r:ResourceIdentity {uid: $uid})
			RETURN count(*) > 0 as hasOwner
		`,
		Parameters: map[string]interface{}{
			"uid": resourceUID,
		},
	}

	result, err := lookup.QueryGraph(ctx, query)
	if err != nil {
		return false, err
	}

	if len(result.Rows) > 0 && len(result.Rows[0]) > 0 {
		if hasOwner, ok := result.Rows[0][0].(bool); ok {
			return hasOwner, nil
		}
	}

	return false, nil
}

// hasFluxLabels checks if the event data contains Flux HelmRelease labels
func hasFluxLabels(event models.Event) bool {
	// Quick check using event data without full parsing
	// Look for the Flux label keys in the raw JSON data
	dataStr := string(event.Data)
	return strings.Contains(dataStr, fluxNameLabel) && strings.Contains(dataStr, fluxNamespaceLabel)
}

// hasKustomizeLabels checks if the event data contains Flux Kustomization labels
func hasKustomizeLabels(event models.Event) bool {
	// Quick check using event data without full parsing
	// Look for the Kustomize label keys in the raw JSON data
	dataStr := string(event.Data)
	return strings.Contains(dataStr, kustomizeNameLabel) && strings.Contains(dataStr, kustomizeNsLabel)
}

// extractLabelsFromEvent extracts labels from the event's resource data
func extractLabelsFromEvent(event models.Event) map[string]string {
	if len(event.Data) == 0 {
		return nil
	}

	var resourceData map[string]interface{}
	if err := json.Unmarshal(event.Data, &resourceData); err != nil {
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
