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
	fluxAPIGroup    = "helm.toolkit.fluxcd.io"
	helmReleaseKind = "HelmRelease"

	// Flux standard labels
	fluxNameLabel      = "helm.toolkit.fluxcd.io/name"
	fluxNamespaceLabel = "helm.toolkit.fluxcd.io/namespace"

	// Confidence scoring weights
	labelMatchWeight        = 0.4
	namespaceMatchWeight    = 0.1
	temporalProximityWeight = 0.3
	reconcileEventWeight    = 0.2
)

// FluxHelmReleaseExtractor extracts relationships from Flux HelmRelease resources
type FluxHelmReleaseExtractor struct {
	logger *logging.Logger
}

// NewFluxHelmReleaseExtractor creates a new Flux HelmRelease extractor
func NewFluxHelmReleaseExtractor() *FluxHelmReleaseExtractor {
	return &FluxHelmReleaseExtractor{
		logger: logging.GetLogger("extractors.flux-helmrelease"),
	}
}

func (e *FluxHelmReleaseExtractor) Name() string {
	return "flux-helmrelease"
}

func (e *FluxHelmReleaseExtractor) Priority() int {
	return 100 // Run after native K8s extractors (priority 0-99)
}

func (e *FluxHelmReleaseExtractor) Matches(event models.Event) bool {
	return event.Resource.Group == fluxAPIGroup &&
		event.Resource.Kind == helmReleaseKind
}

func (e *FluxHelmReleaseExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup ResourceLookup,
) ([]graph.Edge, error) {

	fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: ExtractRelationships called for %s/%s\n", event.Resource.Namespace, event.Resource.Name)

	edges := []graph.Edge{}

	// Parse HelmRelease spec
	var helmRelease map[string]interface{}
	if err := json.Unmarshal(event.Data, &helmRelease); err != nil {
		return nil, fmt.Errorf("failed to parse HelmRelease: %w", err)
	}

	spec, ok := helmRelease["spec"].(map[string]interface{})
	if !ok {
		e.logger.Debug("HelmRelease %s/%s has no spec", event.Resource.Namespace, event.Resource.Name)
		fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: no spec found\n")
		return edges, nil // No spec, skip
	}

	// 1. Extract explicit spec references (REFERENCES_SPEC)
	refEdges, err := e.extractSpecReferences(ctx, event.Resource, spec, lookup)
	if err != nil {
		e.logger.Warn("Failed to extract spec references: %v", err)
		fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: error extracting spec references: %v\n", err)
	} else {
		fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: extracted %d spec reference edges\n", len(refEdges))
		edges = append(edges, refEdges...)
	}

	// 2. Extract managed resources (MANAGES) - only on CREATE/UPDATE
	if event.Type != models.EventTypeDelete {
		managedEdges, err := e.extractManagedResources(ctx, event, helmRelease, lookup)
		if err != nil {
			e.logger.Warn("Failed to extract managed resources: %v", err)
			fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: error extracting managed resources: %v\n", err)
		} else {
			fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: extracted %d managed resource edges\n", len(managedEdges))
			edges = append(edges, managedEdges...)
		}
	}

	fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: returning %d total edges\n", len(edges))
	return edges, nil
}

// extractSpecReferences extracts REFERENCES_SPEC edges from spec fields
func (e *FluxHelmReleaseExtractor) extractSpecReferences(
	ctx context.Context,
	resource models.ResourceMetadata,
	spec map[string]interface{},
	lookup ResourceLookup,
) ([]graph.Edge, error) {

	edges := []graph.Edge{}

	fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: extracting spec references for %s/%s\n", resource.Namespace, resource.Name)

	// Extract valuesFrom references
	// spec.valuesFrom: [{ kind: Secret, name: my-values }]
	valuesFrom, ok := spec["valuesFrom"].([]interface{})
	if !ok {
		fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: no valuesFrom found in spec\n")
	} else {
		fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: found valuesFrom with %d entries\n", len(valuesFrom))
	}

	if ok {
		for i, vf := range valuesFrom {
			vfMap, ok := vf.(map[string]interface{})
			if !ok {
				fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: valuesFrom[%d] is not a map\n", i)
				continue
			}

			kind, _ := vfMap["kind"].(string)
			name, _ := vfMap["name"].(string)

			fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: valuesFrom[%d]: kind=%s, name=%s\n", i, kind, name)

			if kind == "" || name == "" {
				fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: skipping valuesFrom[%d] - empty kind or name\n", i)
				continue
			}

			// Determine namespace (defaults to HelmRelease namespace)
			namespace := resource.Namespace
			if ns, ok := vfMap["targetNamespace"].(string); ok && ns != "" {
				namespace = ns
			}

			fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: looking up %s/%s/%s\n", namespace, kind, name)

			// Look up referenced resource
			targetResource, err := lookup.FindResourceByNamespace(ctx, namespace, kind, name)
			targetUID := ""
			if err != nil {
				e.logger.Debug("Referenced resource not found: %s/%s/%s (will create pending edge)", namespace, kind, name)
				fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: resource not found: %v (will create pending edge)\n", err)
				// Create edge anyway - this handles race conditions where target isn't indexed yet
			} else {
				targetUID = targetResource.UID
				fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: found target resource with UID: %s\n", targetUID)
			}

			fieldPath := fmt.Sprintf("spec.valuesFrom[%d]", i)

			edge := e.createReferencesSpecEdge(
				resource.UID,
				targetUID,
				fieldPath,
				kind,
				name,
				namespace,
			)

			fmt.Printf("[DEBUG] FluxHelmReleaseExtractor: created REFERENCES_SPEC edge from %s to %s (target: %s/%s/%s)\n",
				resource.UID, targetUID, namespace, kind, name)

			edges = append(edges, edge)
		}
	}

	// Extract chart.spec.sourceRef (reference to HelmRepository or GitRepository)
	if chart, ok := spec["chart"].(map[string]interface{}); ok {
		if chartSpec, ok := chart["spec"].(map[string]interface{}); ok {
			if sourceRef, ok := chartSpec["sourceRef"].(map[string]interface{}); ok {
				kind, _ := sourceRef["kind"].(string)
				name, _ := sourceRef["name"].(string)
				namespace, _ := sourceRef["namespace"].(string)

				if namespace == "" {
					namespace = resource.Namespace
				}

				if kind != "" && name != "" {
					targetResource, _ := lookup.FindResourceByNamespace(ctx, namespace, kind, name)
					targetUID := ""
					if targetResource != nil {
						targetUID = targetResource.UID
					}

					edge := e.createReferencesSpecEdge(
						resource.UID,
						targetUID,
						"spec.chart.spec.sourceRef",
						kind,
						name,
						namespace,
					)

					edges = append(edges, edge)
				}
			}
		}
	}

	// Extract kubeConfig.secretRef (reference to Secret)
	if kubeConfig, ok := spec["kubeConfig"].(map[string]interface{}); ok {
		if secretRef, ok := kubeConfig["secretRef"].(map[string]interface{}); ok {
			name, _ := secretRef["name"].(string)

			if name != "" {
				targetResource, _ := lookup.FindResourceByNamespace(ctx, resource.Namespace, "Secret", name)
				targetUID := ""
				if targetResource != nil {
					targetUID = targetResource.UID
				}

				edge := e.createReferencesSpecEdge(
					resource.UID,
					targetUID,
					"spec.kubeConfig.secretRef",
					"Secret",
					name,
					resource.Namespace,
				)

				edges = append(edges, edge)
			}
		}
	}

	return edges, nil
}

// extractManagedResources infers MANAGES edges by finding resources with Flux labels
func (e *FluxHelmReleaseExtractor) extractManagedResources(
	ctx context.Context,
	event models.Event,
	helmRelease map[string]interface{},
	lookup ResourceLookup,
) ([]graph.Edge, error) {

	edges := []graph.Edge{}

	releaseName := event.Resource.Name
	releaseNamespace := event.Resource.Namespace

	// Determine target namespace (where resources are deployed)
	targetNamespace := releaseNamespace
	if spec, ok := helmRelease["spec"].(map[string]interface{}); ok {
		if ns, ok := spec["targetNamespace"].(string); ok && ns != "" {
			targetNamespace = ns
		}
	}

	// Find resources in target namespace or cluster-scoped (excluding the HelmRelease itself)
	// Cluster-scoped resources (ClusterRoles, ClusterRoleBindings, CRDs, etc.) have empty namespace
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity)
			WHERE (r.namespace = $namespace OR r.namespace = "")
			  AND r.deleted = false
			  AND r.uid <> $helmReleaseUID
			RETURN r
			LIMIT 500
		`,
		Parameters: map[string]interface{}{
			"namespace":      targetNamespace,
			"helmReleaseUID": event.Resource.UID,
		},
	}

	result, err := lookup.QueryGraph(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query potential managed resources: %w", err)
	}

	e.logger.Debug("Found %d potential managed resources in namespace %s", len(result.Rows), targetNamespace)

	// For each candidate resource, check if it's managed by this HelmRelease
	for _, row := range result.Rows {
		candidateUID := ExtractUID(row)
		if candidateUID == "" {
			continue
		}
		// Score this relationship
		confidence, evidence := e.scoreManagementRelationship(
			ctx,
			event,
			candidateUID,
			releaseName,
			releaseNamespace,
			targetNamespace,
			lookup,
		)

		// Only create edge if confidence meets threshold
		if confidence >= 0.5 {
			edge := e.createManagesEdge(
				event.Resource.UID,
				candidateUID,
				confidence,
				evidence,
				event.Timestamp,
			)

			edges = append(edges, edge)
			e.logger.Debug("Created MANAGES edge to %s with confidence %.2f", candidateUID, confidence)
		}
	}

	return edges, nil
}

// scoreManagementRelationship calculates confidence score for MANAGES relationship
func (e *FluxHelmReleaseExtractor) scoreManagementRelationship(
	ctx context.Context,
	helmReleaseEvent models.Event,
	candidateUID string,
	releaseName string,
	releaseNamespace string,
	targetNamespace string,
	lookup ResourceLookup,
) (float64, []graph.EvidenceItem) {

	evidence := []graph.EvidenceItem{}
	totalWeight := 0.0
	earnedWeight := 0.0

	// Get candidate resource details
	candidate, err := lookup.FindResourceByUID(ctx, candidateUID)
	if err != nil {
		e.logger.Debug("Failed to lookup candidate resource %s: %v", candidateUID, err)
		return 0.0, evidence
	}

	// PRIORITY CHECK: Flux standard labels (100% confidence if both match)
	fluxLabels := e.checkFluxLabels(ctx, candidateUID, releaseName, releaseNamespace, lookup)
	if fluxLabels.hasNameLabel && fluxLabels.hasNamespaceLabel {
		// Perfect match - Flux definitely manages this resource
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeLabel,
			Value:     fmt.Sprintf("Flux labels match: %s=%s, %s=%s", fluxNameLabel, releaseName, fluxNamespaceLabel, releaseNamespace),
			Weight:    1.0,
			Timestamp: time.Now().UnixNano(),
		})
		e.logger.Debug("Found Flux standard labels on %s - 100%% confidence", candidateUID)
		return 1.0, evidence
	}

	// Fall back to heuristic scoring if Flux labels aren't present

	// EVIDENCE 1: Label match (weight: 0.4)
	// Check if resource name starts with release name (heuristic)
	totalWeight += labelMatchWeight
	labelMatch := e.checkLabelMatch(candidate, releaseName)
	if labelMatch {
		earnedWeight += labelMatchWeight
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeLabel,
			Value:     fmt.Sprintf("name prefix matches: %s", releaseName),
			Weight:    labelMatchWeight,
			Timestamp: time.Now().UnixNano(),
		})
	}

	// EVIDENCE 2: Same namespace (weight: 0.1)
	totalWeight += namespaceMatchWeight
	if candidate.Namespace == targetNamespace {
		earnedWeight += namespaceMatchWeight
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeNamespace,
			Value:     targetNamespace,
			Weight:    namespaceMatchWeight,
			Timestamp: time.Now().UnixNano(),
		})
	}

	// EVIDENCE 3: Temporal proximity (weight: 0.3)
	// Check if candidate was created shortly after HelmRelease reconcile
	totalWeight += temporalProximityWeight
	lagMs := (candidate.FirstSeen - helmReleaseEvent.Timestamp) / 1_000_000
	if lagMs >= 0 && lagMs <= 30000 { // Within 30 seconds
		// Scale confidence based on proximity (closer = higher confidence)
		proximityScore := 1.0 - (float64(lagMs) / 30000.0)
		earnedWeight += temporalProximityWeight * proximityScore

		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeTemporal,
			Value:     fmt.Sprintf("created %dms after reconcile", lagMs),
			Weight:    temporalProximityWeight * proximityScore,
			Timestamp: time.Now().UnixNano(),
		})
	}

	// EVIDENCE 4: Reconcile event correlation (weight: 0.2)
	// Check if HelmRelease has a recent reconcile event
	totalWeight += reconcileEventWeight
	windowNs := int64(60 * time.Second.Nanoseconds())
	helmEvents, err := lookup.FindRecentEvents(ctx, helmReleaseEvent.Resource.UID, windowNs)
	if err == nil && len(helmEvents) > 0 {
		// Check for reconcile success
		for _, evt := range helmEvents {
			if evt.Status == "Ready" || evt.EventType == "UPDATE" {
				earnedWeight += reconcileEventWeight
				evidence = append(evidence, graph.EvidenceItem{
					Type:      graph.EvidenceTypeReconcile,
					Value:     fmt.Sprintf("HelmRelease reconciled at %d", evt.Timestamp),
					Weight:    reconcileEventWeight,
					Timestamp: evt.Timestamp,
				})
				break
			}
		}
	}

	// Calculate final confidence
	confidence := 0.0
	if totalWeight > 0 {
		confidence = earnedWeight / totalWeight
	}

	return confidence, evidence
}

// fluxLabelCheck represents the result of checking for Flux standard labels
type fluxLabelCheck struct {
	hasNameLabel      bool
	hasNamespaceLabel bool
	nameValue         string
	namespaceValue    string
}

// checkFluxLabels checks if the resource has Flux standard labels
// Returns true if both helm.toolkit.fluxcd.io/name and helm.toolkit.fluxcd.io/namespace match
func (e *FluxHelmReleaseExtractor) checkFluxLabels(
	ctx context.Context,
	candidateUID string,
	expectedName string,
	expectedNamespace string,
	lookup ResourceLookup,
) fluxLabelCheck {
	result := fluxLabelCheck{}

	// Get the resource to access its labels
	candidate, err := lookup.FindResourceByUID(ctx, candidateUID)
	if err != nil {
		e.logger.Debug("Failed to lookup candidate resource %s: %v", candidateUID, err)
		return result
	}

	// Check if labels map contains the Flux standard labels
	if candidate.Labels == nil {
		return result
	}

	// Check helm.toolkit.fluxcd.io/name label
	if nameValue, ok := candidate.Labels[fluxNameLabel]; ok {
		if nameValue == expectedName {
			result.hasNameLabel = true
			result.nameValue = nameValue
		}
	}

	// Check helm.toolkit.fluxcd.io/namespace label
	if namespaceValue, ok := candidate.Labels[fluxNamespaceLabel]; ok {
		if namespaceValue == expectedNamespace {
			result.hasNamespaceLabel = true
			result.namespaceValue = namespaceValue
		}
	}

	return result
}

// checkLabelMatch checks if a resource name matches the HelmRelease name pattern
// This is a heuristic since we don't have direct access to labels in the graph
func (e *FluxHelmReleaseExtractor) checkLabelMatch(
	candidate *graph.ResourceIdentity,
	releaseName string,
) bool {
	// Heuristic: Check if resource name starts with release name
	// This covers common patterns like:
	//   HelmRelease: frontend
	//   Deployment: frontend
	//   Service: frontend-svc
	//   ConfigMap: frontend-config
	candidateName := strings.ToLower(candidate.Name)
	releaseNameLower := strings.ToLower(releaseName)

	return strings.HasPrefix(candidateName, releaseNameLower)
}

// Helper functions to create edges

func (e *FluxHelmReleaseExtractor) createReferencesSpecEdge(
	sourceUID, targetUID, fieldPath, kind, name, namespace string,
) graph.Edge {
	props := graph.ReferencesSpecEdge{
		FieldPath:    fieldPath,
		RefKind:      kind,
		RefName:      name,
		RefNamespace: namespace,
	}

	propsJSON, _ := json.Marshal(props)

	return graph.Edge{
		Type:       graph.EdgeTypeReferencesSpec,
		FromUID:    sourceUID,
		ToUID:      targetUID,
		Properties: propsJSON,
	}
}

func (e *FluxHelmReleaseExtractor) createManagesEdge(
	managerUID, managedUID string,
	confidence float64,
	evidence []graph.EvidenceItem,
	timestamp int64,
) graph.Edge {
	props := graph.ManagesEdge{
		Confidence:      confidence,
		Evidence:        evidence,
		FirstObserved:   timestamp,
		LastValidated:   timestamp,
		ValidationState: graph.ValidationStateValid,
	}

	propsJSON, _ := json.Marshal(props)

	return graph.Edge{
		Type:       graph.EdgeTypeManages,
		FromUID:    managerUID,
		ToUID:      managedUID,
		Properties: propsJSON,
	}
}
