package extractors

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFluxManagedResourceExtractor_Matches(t *testing.T) {
	extractor := NewFluxManagedResourceExtractor()

	tests := []struct {
		name     string
		event    models.Event
		expected bool
	}{
		{
			name: "matches deployment CREATE with flux labels",
			event: models.Event{
				Type: models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Kind: "Deployment",
				},
				Data: []byte(`{"metadata":{"labels":{"helm.toolkit.fluxcd.io/name":"test","helm.toolkit.fluxcd.io/namespace":"flux-system"}}}`),
			},
			expected: true,
		},
		{
			name: "does not match deployment UPDATE with flux labels",
			event: models.Event{
				Type: models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					Kind: "Deployment",
				},
				Data: []byte(`{"metadata":{"labels":{"helm.toolkit.fluxcd.io/name":"test","helm.toolkit.fluxcd.io/namespace":"flux-system"}}}`),
			},
			expected: false,
		},
		{
			name: "does not match deployment without flux labels",
			event: models.Event{
				Type: models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Kind: "Deployment",
				},
				Data: []byte(`{"metadata":{"labels":{"app":"test"}}}`),
			},
			expected: false,
		},
		{
			name: "does not match delete event",
			event: models.Event{
				Type: models.EventTypeDelete,
				Resource: models.ResourceMetadata{
					Kind: "Deployment",
				},
				Data: []byte(`{"metadata":{"labels":{"helm.toolkit.fluxcd.io/name":"test","helm.toolkit.fluxcd.io/namespace":"flux-system"}}}`),
			},
			expected: false,
		},
		{
			name: "does not match helmrelease (handled by other extractor)",
			event: models.Event{
				Type: models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Kind: "HelmRelease",
				},
				Data: []byte(`{"metadata":{"labels":{"helm.toolkit.fluxcd.io/name":"test","helm.toolkit.fluxcd.io/namespace":"flux-system"}}}`),
			},
			expected: false,
		},
		{
			name: "matches deployment CREATE with kustomize labels",
			event: models.Event{
				Type: models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Kind: "Deployment",
				},
				Data: []byte(`{"metadata":{"labels":{"kustomize.toolkit.fluxcd.io/name":"test","kustomize.toolkit.fluxcd.io/namespace":"flux-system"}}}`),
			},
			expected: true,
		},
		{
			name: "does not match kustomization (handled by other extractor)",
			event: models.Event{
				Type: models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Kind: "Kustomization",
				},
				Data: []byte(`{"metadata":{"labels":{"kustomize.toolkit.fluxcd.io/name":"test","kustomize.toolkit.fluxcd.io/namespace":"flux-system"}}}`),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.Matches(tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFluxManagedResourceExtractor_ExtractRelationships(t *testing.T) {
	extractor := NewFluxManagedResourceExtractor()

	// Create test deployment with Flux labels
	deploymentUID := "deployment-uid-123"
	helmReleaseUID := "helmrelease-uid-456"

	deploymentData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "test-deployment",
			"namespace": "default",
			"labels": map[string]interface{}{
				"app":                              "test",
				"helm.toolkit.fluxcd.io/name":      "test-release",
				"helm.toolkit.fluxcd.io/namespace": "flux-system",
			},
		},
	}

	deploymentJSON, err := json.Marshal(deploymentData)
	require.NoError(t, err)

	event := models.Event{
		Type: models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			UID:       deploymentUID,
			Kind:      "Deployment",
			Name:      "test-deployment",
			Namespace: "default",
		},
		Data: deploymentJSON,
	}

	// Create mock lookup
	lookup := newMockResourceLookup()
	
	// Add deployment resource with labels
	deployment := &graph.ResourceIdentity{
		UID:       deploymentUID,
		Kind:      "Deployment",
		Name:      "test-deployment",
		Namespace: "default",
		Labels: map[string]string{
			"app":                              "test",
			"helm.toolkit.fluxcd.io/name":      "test-release",
			"helm.toolkit.fluxcd.io/namespace": "flux-system",
		},
	}
	lookup.resources[deploymentUID] = deployment
	
	// Add HelmRelease
	helmRelease := &graph.ResourceIdentity{
		UID:       helmReleaseUID,
		Kind:      "HelmRelease",
		Name:      "test-release",
		Namespace: "flux-system",
	}
	lookup.resources[helmReleaseUID] = helmRelease

	// Extract relationships
	edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
	require.NoError(t, err)
	require.Len(t, edges, 1)

	// Verify edge
	edge := edges[0]
	assert.Equal(t, graph.EdgeTypeManages, edge.Type)
	assert.Equal(t, helmReleaseUID, edge.FromUID)
	assert.Equal(t, deploymentUID, edge.ToUID)

	// Verify edge properties
	var props graph.ManagesEdge
	err = json.Unmarshal(edge.Properties, &props)
	require.NoError(t, err)
	assert.Equal(t, 1.0, props.Confidence)
	assert.Len(t, props.Evidence, 1)
	assert.Equal(t, graph.EvidenceTypeLabel, props.Evidence[0].Type)
	assert.Equal(t, graph.ValidationStateValid, props.ValidationState)
}

func TestFluxManagedResourceExtractor_HelmReleaseNotFound(t *testing.T) {
	extractor := NewFluxManagedResourceExtractor()

	deploymentUID := "deployment-uid-123"

	deploymentData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "test-deployment",
			"namespace": "default",
			"labels": map[string]interface{}{
				"helm.toolkit.fluxcd.io/name":      "nonexistent-release",
				"helm.toolkit.fluxcd.io/namespace": "flux-system",
			},
		},
	}

	deploymentJSON, err := json.Marshal(deploymentData)
	require.NoError(t, err)

	event := models.Event{
		Type: models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			UID:       deploymentUID,
			Kind:      "Deployment",
			Name:      "test-deployment",
			Namespace: "default",
		},
		Data: deploymentJSON,
	}

	// Create mock lookup with no HelmRelease
	lookup := newMockResourceLookup()

	deployment := &graph.ResourceIdentity{
		UID:       deploymentUID,
		Kind:      "Deployment",
		Name:      "test-deployment",
		Namespace: "default",
		Labels: map[string]string{
			"helm.toolkit.fluxcd.io/name":      "nonexistent-release",
			"helm.toolkit.fluxcd.io/namespace": "flux-system",
		},
	}
	lookup.resources[deploymentUID] = deployment

	// Extract relationships - should return empty (not error) when HelmRelease not found
	edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
	assert.NoError(t, err)
	assert.Len(t, edges, 0)
}

func TestFluxManagedResourceExtractor_Kustomization(t *testing.T) {
	extractor := NewFluxManagedResourceExtractor()

	// Create test deployment with Kustomize labels
	deploymentUID := "deployment-uid-789"
	kustomizationUID := "kustomization-uid-012"

	deploymentData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "test-deployment",
			"namespace": "default",
			"labels": map[string]interface{}{
				"app":                                   "test",
				"kustomize.toolkit.fluxcd.io/name":      "test-kustomization",
				"kustomize.toolkit.fluxcd.io/namespace": "flux-system",
			},
		},
	}

	deploymentJSON, err := json.Marshal(deploymentData)
	require.NoError(t, err)

	event := models.Event{
		Type: models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			UID:       deploymentUID,
			Kind:      "Deployment",
			Name:      "test-deployment",
			Namespace: "default",
		},
		Data: deploymentJSON,
	}

	// Create mock lookup
	lookup := newMockResourceLookup()

	// Add deployment resource with labels
	deployment := &graph.ResourceIdentity{
		UID:       deploymentUID,
		Kind:      "Deployment",
		Name:      "test-deployment",
		Namespace: "default",
		Labels: map[string]string{
			"app":                                   "test",
			"kustomize.toolkit.fluxcd.io/name":      "test-kustomization",
			"kustomize.toolkit.fluxcd.io/namespace": "flux-system",
		},
	}
	lookup.resources[deploymentUID] = deployment

	// Add Kustomization
	kustomization := &graph.ResourceIdentity{
		UID:       kustomizationUID,
		Kind:      "Kustomization",
		Name:      "test-kustomization",
		Namespace: "flux-system",
	}
	lookup.resources[kustomizationUID] = kustomization

	// Extract relationships
	edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
	require.NoError(t, err)
	require.Len(t, edges, 1)

	// Verify edge
	edge := edges[0]
	assert.Equal(t, graph.EdgeTypeManages, edge.Type)
	assert.Equal(t, kustomizationUID, edge.FromUID)
	assert.Equal(t, deploymentUID, edge.ToUID)

	// Verify edge properties
	var props graph.ManagesEdge
	err = json.Unmarshal(edge.Properties, &props)
	require.NoError(t, err)
	assert.Equal(t, 1.0, props.Confidence)
	assert.Len(t, props.Evidence, 1)
	assert.Equal(t, graph.EvidenceTypeLabel, props.Evidence[0].Type)
	assert.Equal(t, graph.ValidationStateValid, props.ValidationState)
}

func TestFluxManagedResourceExtractor_KustomizationNotFound(t *testing.T) {
	extractor := NewFluxManagedResourceExtractor()

	deploymentUID := "deployment-uid-456"

	deploymentData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "test-deployment",
			"namespace": "default",
			"labels": map[string]interface{}{
				"kustomize.toolkit.fluxcd.io/name":      "nonexistent-kustomization",
				"kustomize.toolkit.fluxcd.io/namespace": "flux-system",
			},
		},
	}

	deploymentJSON, err := json.Marshal(deploymentData)
	require.NoError(t, err)

	event := models.Event{
		Type: models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			UID:       deploymentUID,
			Kind:      "Deployment",
			Name:      "test-deployment",
			Namespace: "default",
		},
		Data: deploymentJSON,
	}

	// Create mock lookup with no Kustomization
	lookup := newMockResourceLookup()

	deployment := &graph.ResourceIdentity{
		UID:       deploymentUID,
		Kind:      "Deployment",
		Name:      "test-deployment",
		Namespace: "default",
		Labels: map[string]string{
			"kustomize.toolkit.fluxcd.io/name":      "nonexistent-kustomization",
			"kustomize.toolkit.fluxcd.io/namespace": "flux-system",
		},
	}
	lookup.resources[deploymentUID] = deployment

	// Extract relationships - should return empty (not error) when Kustomization not found
	edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
	assert.NoError(t, err)
	assert.Len(t, edges, 0)
}

func TestFluxManagedResourceExtractor_Priority(t *testing.T) {
	extractor := NewFluxManagedResourceExtractor()
	
	// Should run after FluxHelmReleaseExtractor (priority 10)
	assert.Equal(t, 15, extractor.Priority())
}

func TestFluxManagedResourceExtractor_Name(t *testing.T) {
	extractor := NewFluxManagedResourceExtractor()
	assert.Equal(t, "flux-managed-resource", extractor.Name())
}
