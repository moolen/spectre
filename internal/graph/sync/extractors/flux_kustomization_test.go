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

func TestFluxKustomizationExtractor_Matches(t *testing.T) {
	extractor := NewFluxKustomizationExtractor()

	tests := []struct {
		name     string
		event    models.Event
		expected bool
	}{
		{
			name: "matches Kustomization",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Kustomization",
					Group: "kustomize.toolkit.fluxcd.io",
				},
			},
			expected: true,
		},
		{
			name: "does not match HelmRelease",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "HelmRelease",
					Group: "helm.toolkit.fluxcd.io",
				},
			},
			expected: false,
		},
		{
			name: "does not match wrong API group",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Kustomization",
					Group: "custom.io",
				},
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

func TestFluxKustomizationExtractor_ExtractSpecReferences(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name               string
		kustomizationData  map[string]interface{}
		mockResources      map[string]*graph.ResourceIdentity // resources to add to mock lookup
		expectedEdgeTypes  []graph.EdgeType
		expectedMinEdges   int
	}{
		{
			name: "Kustomization with sourceRef",
			kustomizationData: map[string]interface{}{
				"spec": map[string]interface{}{
					"interval": "5m",
					"sourceRef": map[string]interface{}{
						"kind": "GitRepository",
						"name": "app-repo",
					},
					"path": "./deploy",
				},
			},
			mockResources: map[string]*graph.ResourceIdentity{
				"flux-system/GitRepository/app-repo": {
					UID:       "gitrepo-uid-1",
					Kind:      "GitRepository",
					Namespace: "flux-system",
					Name:      "app-repo",
				},
			},
			expectedEdgeTypes: []graph.EdgeType{graph.EdgeTypeReferencesSpec},
			expectedMinEdges:  1,
		},
		{
			name: "Kustomization with decryption secretRef",
			kustomizationData: map[string]interface{}{
				"spec": map[string]interface{}{
					"interval": "5m",
					"sourceRef": map[string]interface{}{
						"kind": "GitRepository",
						"name": "app-repo",
					},
					"decryption": map[string]interface{}{
						"provider": "sops",
						"secretRef": map[string]interface{}{
							"name": "sops-gpg",
						},
					},
				},
			},
			mockResources: map[string]*graph.ResourceIdentity{
				"flux-system/GitRepository/app-repo": {
					UID:       "gitrepo-uid-1",
					Kind:      "GitRepository",
					Namespace: "flux-system",
					Name:      "app-repo",
				},
				"flux-system/Secret/sops-gpg": {
					UID:       "secret-uid-1",
					Kind:      "Secret",
					Namespace: "flux-system",
					Name:      "sops-gpg",
				},
			},
			expectedEdgeTypes: []graph.EdgeType{graph.EdgeTypeReferencesSpec, graph.EdgeTypeReferencesSpec},
			expectedMinEdges:  2,
		},
		{
			name: "Kustomization with cross-namespace sourceRef",
			kustomizationData: map[string]interface{}{
				"spec": map[string]interface{}{
					"interval": "5m",
					"sourceRef": map[string]interface{}{
						"kind":      "GitRepository",
						"name":      "app-repo",
						"namespace": "flux-system",
					},
				},
			},
			mockResources: map[string]*graph.ResourceIdentity{
				"flux-system/GitRepository/app-repo": {
					UID:       "gitrepo-uid-1",
					Kind:      "GitRepository",
					Namespace: "flux-system",
					Name:      "app-repo",
				},
			},
			expectedEdgeTypes: []graph.EdgeType{graph.EdgeTypeReferencesSpec},
			expectedMinEdges:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewFluxKustomizationExtractor()

			kustomizationJSON, err := json.Marshal(tt.kustomizationData)
			require.NoError(t, err)

			event := models.Event{
				Resource: models.ResourceMetadata{
					UID:       "kustomization-uid",
					Kind:      "Kustomization",
					Group:     "kustomize.toolkit.fluxcd.io",
					Namespace: "flux-system",
					Name:      "frontend",
				},
				Data: kustomizationJSON,
				Type: models.EventTypeUpdate,
			}

			// Create lookup with mock resources
			lookup := &MockResourceLookup{
				resources: make(map[string]*graph.ResourceIdentity),
				queryResult: &graph.QueryResult{
					Rows: [][]interface{}{}, // No managed resources for this test
				},
			}
			// Add mock resources to the lookup
			for key, resource := range tt.mockResources {
				lookup.resources[key] = resource
			}

			edges, err := extractor.ExtractRelationships(ctx, event, lookup)

			assert.NoError(t, err)
			assert.GreaterOrEqual(t, len(edges), tt.expectedMinEdges)

			// Count REFERENCES_SPEC edges
			refCount := 0
			for _, edge := range edges {
				if edge.Type == graph.EdgeTypeReferencesSpec {
					refCount++
					assert.Equal(t, "kustomization-uid", edge.FromUID)
				}
			}
			assert.Equal(t, len(tt.expectedEdgeTypes), refCount, "Expected number of REFERENCES_SPEC edges")
		})
	}
}

func TestFluxKustomizationExtractor_ExtractManagedResources(t *testing.T) {
	ctx := context.Background()

	kustomizationData := map[string]interface{}{
		"spec": map[string]interface{}{
			"interval": "5m",
			"sourceRef": map[string]interface{}{
				"kind": "GitRepository",
				"name": "app-repo",
			},
			"path": "./deploy",
		},
	}

	kustomizationJSON, err := json.Marshal(kustomizationData)
	require.NoError(t, err)

	event := models.Event{
		Resource: models.ResourceMetadata{
			UID:       "kustomization-uid",
			Kind:      "Kustomization",
			Group:     "kustomize.toolkit.fluxcd.io",
			Namespace: "flux-system",
			Name:      "frontend",
		},
		Data:      kustomizationJSON,
		Type:      models.EventTypeUpdate,
		Timestamp: 1000000000,
	}

	extractor := NewFluxKustomizationExtractor()

	tests := []struct {
		name              string
		queryResult       *graph.QueryResult
		resourceLabels    map[string]string
		expectedMANAGES   int
		minConfidence     float64
	}{
		{
			name: "resource with perfect Kustomize labels",
			queryResult: &graph.QueryResult{
				Rows: [][]interface{}{
					{"managed-resource-uid"},
				},
			},
			resourceLabels: map[string]string{
				kustomizeNameLabel: "frontend",
				kustomizeNsLabel:   "flux-system",
			},
			expectedMANAGES: 1,
			minConfidence:   1.0,
		},
		{
			name: "resource without Kustomize labels (low confidence)",
			queryResult: &graph.QueryResult{
				Rows: [][]interface{}{
					{"managed-resource-uid"},
				},
			},
			resourceLabels: map[string]string{
				"app": "backend", // Different name, so heuristic won't match well
			},
			expectedMANAGES: 0, // Confidence too low without labels
			minConfidence:   0.0,
		},
		{
			name: "no matching resources",
			queryResult: &graph.QueryResult{
				Rows: [][]interface{}{},
			},
			resourceLabels:  map[string]string{},
			expectedMANAGES: 0,
			minConfidence:   0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set resource name and timestamp based on test case
			resourceName := "frontend-deployment"
			firstSeen := int64(1000100000) // Shortly after Kustomization (0.1ms)

			if tt.name == "resource without Kustomize labels (low confidence)" {
				resourceName = "backend-deployment" // Different name to avoid prefix match
				firstSeen = int64(32000000000)      // 31+ seconds after to be outside temporal window
			}

			lookup := &MockResourceLookup{
				queryResult: tt.queryResult,
				resources: map[string]*graph.ResourceIdentity{
					"managed-resource-uid": {
						UID:       "managed-resource-uid",
						Kind:      "Deployment",
						Namespace: "flux-system",
						Name:      resourceName,
						Labels:    tt.resourceLabels,
						FirstSeen: firstSeen,
					},
				},
			}

			edges, err := extractor.ExtractRelationships(ctx, event, lookup)

			assert.NoError(t, err)

			// Count MANAGES edges
			managesCount := 0
			for _, edge := range edges {
				if edge.Type == graph.EdgeTypeManages {
					managesCount++

					// Verify confidence
					var props graph.ManagesEdge
					err := json.Unmarshal(edge.Properties, &props)
					assert.NoError(t, err)

					if tt.minConfidence > 0 {
						assert.GreaterOrEqual(t, props.Confidence, tt.minConfidence)
						assert.NotEmpty(t, props.Evidence)
					}
				}
			}

			assert.Equal(t, tt.expectedMANAGES, managesCount)
		})
	}
}

func TestFluxKustomizationExtractor_DeletedResource(t *testing.T) {
	ctx := context.Background()
	extractor := NewFluxKustomizationExtractor()

	kustomizationData := map[string]interface{}{
		"spec": map[string]interface{}{
			"interval": "5m",
			"sourceRef": map[string]interface{}{
				"kind": "GitRepository",
				"name": "app-repo",
			},
		},
	}

	kustomizationJSON, err := json.Marshal(kustomizationData)
	require.NoError(t, err)

	event := models.Event{
		Resource: models.ResourceMetadata{
			UID:       "kustomization-uid",
			Kind:      "Kustomization",
			Namespace: "flux-system",
			Name:      "frontend",
		},
		Data: kustomizationJSON,
		Type: models.EventTypeDelete, // Deleted event
	}

	lookup := &MockResourceLookup{
		queryResult: &graph.QueryResult{
			Rows: [][]interface{}{
				{"managed-resource-uid"},
			},
		},
	}

	edges, err := extractor.ExtractRelationships(ctx, event, lookup)

	assert.NoError(t, err)

	// Should still extract REFERENCES_SPEC, but no MANAGES edges for deleted resources
	hasManages := false
	for _, edge := range edges {
		if edge.Type == graph.EdgeTypeManages {
			hasManages = true
		}
	}
	assert.False(t, hasManages, "Should not extract MANAGES relationships for deleted resources")
}
