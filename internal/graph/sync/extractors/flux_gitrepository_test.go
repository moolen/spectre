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

func TestFluxGitRepositoryExtractor_Matches(t *testing.T) {
	extractor := NewFluxGitRepositoryExtractor()

	tests := []struct {
		name     string
		event    models.Event
		expected bool
	}{
		{
			name: "matches GitRepository",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "GitRepository",
					Group: "source.toolkit.fluxcd.io",
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
					Kind:  "GitRepository",
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

func TestFluxGitRepositoryExtractor_ExtractRelationships(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		gitRepoData   map[string]interface{}
		mockSecrets   map[string]*graph.ResourceIdentity // secrets to add to mock lookup
		expectedEdges int
	}{
		{
			name: "GitRepository with secretRef",
			gitRepoData: map[string]interface{}{
				"spec": map[string]interface{}{
					"url": "https://github.com/example/repo",
					"secretRef": map[string]interface{}{
						"name": "git-credentials",
					},
				},
			},
			mockSecrets: map[string]*graph.ResourceIdentity{
				"flux-system/Secret/git-credentials": {
					UID:       "secret-uid-1",
					Kind:      "Secret",
					Namespace: "flux-system",
					Name:      "git-credentials",
				},
			},
			expectedEdges: 1,
		},
		{
			name: "GitRepository with verify secretRef",
			gitRepoData: map[string]interface{}{
				"spec": map[string]interface{}{
					"url": "https://github.com/example/repo",
					"verify": map[string]interface{}{
						"mode": "head",
						"secretRef": map[string]interface{}{
							"name": "gpg-key",
						},
					},
				},
			},
			mockSecrets: map[string]*graph.ResourceIdentity{
				"flux-system/Secret/gpg-key": {
					UID:       "secret-uid-2",
					Kind:      "Secret",
					Namespace: "flux-system",
					Name:      "gpg-key",
				},
			},
			expectedEdges: 1,
		},
		{
			name: "GitRepository with both secretRefs",
			gitRepoData: map[string]interface{}{
				"spec": map[string]interface{}{
					"url": "https://github.com/example/repo",
					"secretRef": map[string]interface{}{
						"name": "git-credentials",
					},
					"verify": map[string]interface{}{
						"mode": "head",
						"secretRef": map[string]interface{}{
							"name": "gpg-key",
						},
					},
				},
			},
			mockSecrets: map[string]*graph.ResourceIdentity{
				"flux-system/Secret/git-credentials": {
					UID:       "secret-uid-1",
					Kind:      "Secret",
					Namespace: "flux-system",
					Name:      "git-credentials",
				},
				"flux-system/Secret/gpg-key": {
					UID:       "secret-uid-2",
					Kind:      "Secret",
					Namespace: "flux-system",
					Name:      "gpg-key",
				},
			},
			expectedEdges: 2,
		},
		{
			name: "GitRepository without secretRefs",
			gitRepoData: map[string]interface{}{
				"spec": map[string]interface{}{
					"url": "https://github.com/example/repo",
				},
			},
			mockSecrets:   nil,
			expectedEdges: 0,
		},
		{
			name: "GitRepository with secretRef but target not found",
			gitRepoData: map[string]interface{}{
				"spec": map[string]interface{}{
					"url": "https://github.com/example/repo",
					"secretRef": map[string]interface{}{
						"name": "missing-secret",
					},
				},
			},
			mockSecrets:   nil, // No secrets in lookup
			expectedEdges: 0,   // Edge should not be created when target doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewFluxGitRepositoryExtractor()

			gitRepoJSON, err := json.Marshal(tt.gitRepoData)
			require.NoError(t, err)

			event := models.Event{
				Resource: models.ResourceMetadata{
					UID:       "gitrepo-uid",
					Kind:      "GitRepository",
					Group:     "source.toolkit.fluxcd.io",
					Namespace: "flux-system",
					Name:      "test-repo",
				},
				Data: gitRepoJSON,
				Type: models.EventTypeUpdate,
			}

			// Create lookup with mock secrets
			lookup := &MockResourceLookup{
				resources: make(map[string]*graph.ResourceIdentity),
			}
			// Add mock secrets to the lookup
			for key, secret := range tt.mockSecrets {
				lookup.resources[key] = secret
			}

			edges, err := extractor.ExtractRelationships(ctx, event, lookup)

			assert.NoError(t, err)
			assert.Len(t, edges, tt.expectedEdges)

			// Verify edge types
			for _, edge := range edges {
				assert.Equal(t, graph.EdgeTypeReferencesSpec, edge.Type)
				assert.Equal(t, "gitrepo-uid", edge.FromUID)
				assert.NotEmpty(t, edge.ToUID, "Edge ToUID should not be empty")

				// Verify properties
				var props graph.ReferencesSpecEdge
				err := json.Unmarshal(edge.Properties, &props)
				assert.NoError(t, err)
				assert.Equal(t, "Secret", props.RefKind)
			}
		})
	}
}
