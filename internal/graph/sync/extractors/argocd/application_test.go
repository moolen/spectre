package argocd

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockResourceLookup provides a mock implementation for testing
type MockResourceLookup struct {
	resources   map[string]*graph.ResourceIdentity
	queryResult *graph.QueryResult
	queryError  error
}

func (m *MockResourceLookup) FindResourceByUID(_ context.Context, uid string) (*graph.ResourceIdentity, error) {
	if res, ok := m.resources[uid]; ok {
		return res, nil
	}
	return nil, nil
}

func (m *MockResourceLookup) FindResourceByNamespace(_ context.Context, namespace, kind, name string) (*graph.ResourceIdentity, error) {
	key := namespace + "/" + kind + "/" + name
	if res, ok := m.resources[key]; ok {
		return res, nil
	}
	return nil, nil
}

func (m *MockResourceLookup) FindRecentEvents(_ context.Context, _ string, _ int64) ([]graph.ChangeEvent, error) {
	return nil, nil
}

func (m *MockResourceLookup) QueryGraph(_ context.Context, _ graph.GraphQuery) (*graph.QueryResult, error) {
	if m.queryError != nil {
		return nil, m.queryError
	}
	return m.queryResult, nil
}

func TestArgoCDApplicationExtractor_Matches(t *testing.T) {
	extractor := NewArgoCDApplicationExtractor()

	tests := []struct {
		name     string
		event    models.Event
		expected bool
	}{
		{
			name: "matches ArgoCD Application",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Application",
					Group: "argoproj.io",
				},
			},
			expected: true,
		},
		{
			name: "does not match Flux Kustomization",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Kustomization",
					Group: "kustomize.toolkit.fluxcd.io",
				},
			},
			expected: false,
		},
		{
			name: "does not match wrong API group",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Application",
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

func TestArgoCDApplicationExtractor_ExtractSecretReferences(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		appData       map[string]interface{}
		expectedEdges int
	}{
		{
			name: "Application with Git password secret",
			appData: map[string]interface{}{
				"spec": map[string]interface{}{
					"source": map[string]interface{}{
						"repoURL": "https://github.com/example/repo",
						"passwordSecret": map[string]interface{}{
							"name": "git-credentials",
						},
					},
					"destination": map[string]interface{}{
						"namespace": "default",
					},
				},
			},
			expectedEdges: 1,
		},
		{
			name: "Application with multiple secrets",
			appData: map[string]interface{}{
				"spec": map[string]interface{}{
					"source": map[string]interface{}{
						"repoURL": "https://github.com/example/repo",
						"usernameSecret": map[string]interface{}{
							"name": "git-username",
						},
						"passwordSecret": map[string]interface{}{
							"name": "git-password",
						},
						"tokenSecret": map[string]interface{}{
							"name": "git-token",
						},
					},
					"destination": map[string]interface{}{
						"namespace": "default",
					},
				},
			},
			expectedEdges: 3,
		},
		{
			name: "Application with Helm values from secret",
			appData: map[string]interface{}{
				"spec": map[string]interface{}{
					"source": map[string]interface{}{
						"repoURL": "https://charts.example.com",
						"helm": map[string]interface{}{
							"valuesFrom": []interface{}{
								map[string]interface{}{
									"secretKeyRef": map[string]interface{}{
										"name": "helm-values",
										"key":  "values.yaml",
									},
								},
							},
						},
					},
					"destination": map[string]interface{}{
						"namespace": "default",
					},
				},
			},
			expectedEdges: 1,
		},
		{
			name: "Application with multiple sources",
			appData: map[string]interface{}{
				"spec": map[string]interface{}{
					"sources": []interface{}{
						map[string]interface{}{
							"repoURL": "https://github.com/example/repo1",
							"passwordSecret": map[string]interface{}{
								"name": "repo1-creds",
							},
						},
						map[string]interface{}{
							"repoURL": "https://github.com/example/repo2",
							"tokenSecret": map[string]interface{}{
								"name": "repo2-token",
							},
						},
					},
					"destination": map[string]interface{}{
						"namespace": "default",
					},
				},
			},
			expectedEdges: 2,
		},
		{
			name: "Application without secrets",
			appData: map[string]interface{}{
				"spec": map[string]interface{}{
					"source": map[string]interface{}{
						"repoURL": "https://github.com/example/public-repo",
					},
					"destination": map[string]interface{}{
						"namespace": "default",
					},
				},
			},
			expectedEdges: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewArgoCDApplicationExtractor()

			appJSON, err := json.Marshal(tt.appData)
			require.NoError(t, err)

			event := models.Event{
				Resource: models.ResourceMetadata{
					UID:       "app-uid",
					Kind:      "Application",
					Group:     "argoproj.io",
					Namespace: "argocd",
					Name:      "test-app",
				},
				Data: appJSON,
				Type: models.EventTypeUpdate,
			}

			lookup := &MockResourceLookup{
				resources: map[string]*graph.ResourceIdentity{
					// Add all the referenced secrets so they can be found
					"argocd/Secret/git-credentials": {UID: "secret-git-credentials"},
					"argocd/Secret/git-creds":       {UID: "secret-git-creds"},
					"argocd/Secret/git-username":    {UID: "secret-git-username"},
					"argocd/Secret/git-password":    {UID: "secret-git-password"},
					"argocd/Secret/git-token":       {UID: "secret-git-token"},
					"argocd/Secret/repo1-creds":     {UID: "secret-repo1-creds"},
					"argocd/Secret/repo2-token":     {UID: "secret-repo2-token"},
					"argocd/Secret/helm-values":     {UID: "secret-helm-values"},
				},
				queryResult: &graph.QueryResult{
					Rows: [][]interface{}{}, // No managed resources for this test
				},
			}

			edges, err := extractor.ExtractRelationships(ctx, event, lookup)

			assert.NoError(t, err)
			assert.Len(t, edges, tt.expectedEdges)

			// Verify all edges are REFERENCES_SPEC to Secret
			for _, edge := range edges {
				assert.Equal(t, graph.EdgeTypeReferencesSpec, edge.Type)
				assert.Equal(t, "app-uid", edge.FromUID)

				var props graph.ReferencesSpecEdge
				err := json.Unmarshal(edge.Properties, &props)
				assert.NoError(t, err)
				assert.Equal(t, "Secret", props.RefKind)
			}
		})
	}
}

func TestArgoCDApplicationExtractor_ExtractManagedResources(t *testing.T) {
	ctx := context.Background()

	appData := map[string]interface{}{
		"spec": map[string]interface{}{
			"source": map[string]interface{}{
				"repoURL": "https://github.com/example/repo",
			},
			"destination": map[string]interface{}{
				"namespace": "default",
			},
		},
	}

	appJSON, err := json.Marshal(appData)
	require.NoError(t, err)

	event := models.Event{
		Resource: models.ResourceMetadata{
			UID:       "app-uid",
			Kind:      "Application",
			Group:     "argoproj.io",
			Namespace: "argocd",
			Name:      "frontend",
		},
		Data:      appJSON,
		Type:      models.EventTypeUpdate,
		Timestamp: 1000000000,
	}

	extractor := NewArgoCDApplicationExtractor()

	tests := []struct {
		name            string
		queryResult     *graph.QueryResult
		resourceLabels  map[string]string
		resourceName    string
		firstSeen       int64
		expectedMANAGES int
		minConfidence   float64
	}{
		{
			name: "resource with perfect ArgoCD label",
			queryResult: &graph.QueryResult{
				Rows: [][]interface{}{
					{"managed-resource-uid"},
				},
			},
			resourceLabels: map[string]string{
				argoCDInstanceLabel: "frontend",
			},
			resourceName:    "frontend-deployment",
			firstSeen:       1000100000,
			expectedMANAGES: 1,
			minConfidence:   1.0,
		},
		{
			name: "resource without ArgoCD label (low confidence)",
			queryResult: &graph.QueryResult{
				Rows: [][]interface{}{
					{"managed-resource-uid"},
				},
			},
			resourceLabels: map[string]string{
				"app": "backend",
			},
			resourceName:    "backend-deployment",
			firstSeen:       int64(130000000000), // 2+ minutes after, outside temporal window
			expectedMANAGES: 0,                   // Confidence too low
			minConfidence:   0.0,
		},
		{
			name: "no matching resources",
			queryResult: &graph.QueryResult{
				Rows: [][]interface{}{},
			},
			resourceLabels:  map[string]string{},
			resourceName:    "test-deployment",
			firstSeen:       1000100000,
			expectedMANAGES: 0,
			minConfidence:   0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := &MockResourceLookup{
				queryResult: tt.queryResult,
				resources: map[string]*graph.ResourceIdentity{
					"managed-resource-uid": {
						UID:       "managed-resource-uid",
						Kind:      "Deployment",
						Namespace: "default",
						Name:      tt.resourceName,
						Labels:    tt.resourceLabels,
						FirstSeen: tt.firstSeen,
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

func TestArgoCDApplicationExtractor_CrossNamespaceDeployment(t *testing.T) {
	ctx := context.Background()
	extractor := NewArgoCDApplicationExtractor()

	// Application in argocd namespace, deploying to production namespace
	appData := map[string]interface{}{
		"spec": map[string]interface{}{
			"source": map[string]interface{}{
				"repoURL": "https://github.com/example/app",
			},
			"destination": map[string]interface{}{
				"namespace": "production", // Different from Application namespace
			},
		},
	}

	appJSON, err := json.Marshal(appData)
	require.NoError(t, err)

	event := models.Event{
		Resource: models.ResourceMetadata{
			UID:       "app-uid",
			Kind:      "Application",
			Group:     "argoproj.io",
			Namespace: "argocd", // Application lives here
			Name:      "backend-app",
		},
		Data:      appJSON,
		Type:      models.EventTypeUpdate,
		Timestamp: 1000000000,
	}

	lookup := &MockResourceLookup{
		queryResult: &graph.QueryResult{
			Rows: [][]interface{}{
				{"deployment-uid"},
			},
		},
		resources: map[string]*graph.ResourceIdentity{
			"deployment-uid": {
				UID:       "deployment-uid",
				Kind:      "Deployment",
				Namespace: "production", // Deployed to target namespace
				Name:      "backend-deployment",
				Labels: map[string]string{
					argoCDInstanceLabel: "backend-app",
				},
				FirstSeen: 1000100000,
			},
		},
	}

	edges, err := extractor.ExtractRelationships(ctx, event, lookup)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(edges), 1)

	// Find MANAGES edge
	var managesEdge *graph.Edge
	for i := range edges {
		if edges[i].Type == graph.EdgeTypeManages {
			managesEdge = &edges[i]
			break
		}
	}

	require.NotNil(t, managesEdge, "Should create MANAGES edge for cross-namespace deployment")
	assert.Equal(t, "app-uid", managesEdge.FromUID)
	assert.Equal(t, "deployment-uid", managesEdge.ToUID)

	// Verify high confidence from label match
	var props graph.ManagesEdge
	err = json.Unmarshal(managesEdge.Properties, &props)
	require.NoError(t, err)
	assert.Equal(t, 1.0, props.Confidence)
}

func TestArgoCDApplicationExtractor_DeletedApplication(t *testing.T) {
	ctx := context.Background()
	extractor := NewArgoCDApplicationExtractor()

	appData := map[string]interface{}{
		"spec": map[string]interface{}{
			"source": map[string]interface{}{
				"repoURL": "https://github.com/example/repo",
			},
			"destination": map[string]interface{}{
				"namespace": "default",
			},
		},
	}

	appJSON, err := json.Marshal(appData)
	require.NoError(t, err)

	event := models.Event{
		Resource: models.ResourceMetadata{
			UID:       "app-uid",
			Kind:      "Application",
			Namespace: "argocd",
			Name:      "frontend",
		},
		Data: appJSON,
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

	// Should not extract MANAGES relationships for deleted Applications
	hasManages := false
	for _, edge := range edges {
		if edge.Type == graph.EdgeTypeManages {
			hasManages = true
		}
	}
	assert.False(t, hasManages, "Should not extract MANAGES relationships for deleted Applications")
}

func TestArgoCDApplicationExtractor_Priority(t *testing.T) {
	extractor := NewArgoCDApplicationExtractor()
	assert.Equal(t, 20, extractor.Priority())
}

func TestArgoCDApplicationExtractor_Name(t *testing.T) {
	extractor := NewArgoCDApplicationExtractor()
	assert.Equal(t, "argocd-application", extractor.Name())
}
