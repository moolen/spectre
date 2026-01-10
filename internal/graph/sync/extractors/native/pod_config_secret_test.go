package native

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync/extractors"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPodConfigSecretExtractor_Matches(t *testing.T) {
	extractor := NewPodConfigSecretExtractor()

	tests := []struct {
		name     string
		event    models.Event
		expected bool
	}{
		{
			name: "matches Pod",
			event: models.Event{
				Resource: models.ResourceMetadata{Kind: "Pod", Group: ""},
			},
			expected: true,
		},
		{
			name: "does not match Deployment",
			event: models.Event{
				Resource: models.ResourceMetadata{Kind: "Deployment", Group: "apps"},
			},
			expected: false,
		},
		{
			name: "does not match Pod with non-empty group",
			event: models.Event{
				Resource: models.ResourceMetadata{Kind: "Pod", Group: "custom.io"},
			},
			expected: false,
		},
		{
			name: "does not match Service",
			event: models.Event{
				Resource: models.ResourceMetadata{Kind: "Service", Group: ""},
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

func TestPodConfigSecretExtractor_VolumeReferences(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		podData       map[string]interface{}
		mockResources []*graph.ResourceIdentity
		expectedEdges int
		expectedKinds []string
		expectedPaths []string
	}{
		{
			name: "configMap volume",
			podData: map[string]interface{}{
				"spec": map[string]interface{}{
					"volumes": []interface{}{
						map[string]interface{}{
							"name": "config-vol",
							"configMap": map[string]interface{}{
								"name": "my-config",
							},
						},
					},
				},
			},
			mockResources: []*graph.ResourceIdentity{
				{UID: "cm-uid", Kind: "ConfigMap", Namespace: "default", Name: "my-config"},
			},
			expectedEdges: 1,
			expectedKinds: []string{"ConfigMap"},
			expectedPaths: []string{"spec.volumes[0].configMap"},
		},
		{
			name: "secret volume",
			podData: map[string]interface{}{
				"spec": map[string]interface{}{
					"volumes": []interface{}{
						map[string]interface{}{
							"name": "secret-vol",
							"secret": map[string]interface{}{
								"secretName": "my-secret",
							},
						},
					},
				},
			},
			mockResources: []*graph.ResourceIdentity{
				{UID: "secret-uid", Kind: "Secret", Namespace: "default", Name: "my-secret"},
			},
			expectedEdges: 1,
			expectedKinds: []string{"Secret"},
			expectedPaths: []string{"spec.volumes[0].secret"},
		},
		{
			name: "multiple volumes with configMap and secret",
			podData: map[string]interface{}{
				"spec": map[string]interface{}{
					"volumes": []interface{}{
						map[string]interface{}{
							"name": "config-vol",
							"configMap": map[string]interface{}{
								"name": "my-config",
							},
						},
						map[string]interface{}{
							"name": "secret-vol",
							"secret": map[string]interface{}{
								"secretName": "my-secret",
							},
						},
					},
				},
			},
			mockResources: []*graph.ResourceIdentity{
				{UID: "cm-uid", Kind: "ConfigMap", Namespace: "default", Name: "my-config"},
				{UID: "secret-uid", Kind: "Secret", Namespace: "default", Name: "my-secret"},
			},
			expectedEdges: 2,
			expectedKinds: []string{"ConfigMap", "Secret"},
			expectedPaths: []string{"spec.volumes[0].configMap", "spec.volumes[1].secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewPodConfigSecretExtractor()
			lookup := extractors.NewMockResourceLookup()

			for _, res := range tt.mockResources {
				lookup.AddResource(res)
			}

			podJSON, err := json.Marshal(tt.podData)
			require.NoError(t, err)

			event := models.Event{
				Resource: models.ResourceMetadata{
					UID: "pod-uid", Kind: "Pod", Namespace: "default", Name: "test-pod",
				},
				Data: podJSON,
				Type: models.EventTypeUpdate,
			}

			edges, err := extractor.ExtractRelationships(ctx, event, lookup)
			assert.NoError(t, err)
			assert.Len(t, edges, tt.expectedEdges)

			for i, edge := range edges {
				assert.Equal(t, graph.EdgeTypeReferencesSpec, edge.Type)
				var props graph.ReferencesSpecEdge
				err := json.Unmarshal(edge.Properties, &props)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedKinds[i], props.RefKind)
				assert.Equal(t, tt.expectedPaths[i], props.FieldPath)
			}
		})
	}
}

func TestPodConfigSecretExtractor_ProjectedVolumes(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		podData       map[string]interface{}
		mockResources []*graph.ResourceIdentity
		expectedEdges int
		expectedKinds []string
	}{
		{
			name: "projected volume with configMap and secret",
			podData: map[string]interface{}{
				"spec": map[string]interface{}{
					"volumes": []interface{}{
						map[string]interface{}{
							"name": "projected-vol",
							"projected": map[string]interface{}{
								"sources": []interface{}{
									map[string]interface{}{
										"configMap": map[string]interface{}{"name": "cm1"},
									},
									map[string]interface{}{
										"secret": map[string]interface{}{"name": "secret1"},
									},
								},
							},
						},
					},
				},
			},
			mockResources: []*graph.ResourceIdentity{
				{UID: "cm1-uid", Kind: "ConfigMap", Namespace: "default", Name: "cm1"},
				{UID: "secret1-uid", Kind: "Secret", Namespace: "default", Name: "secret1"},
			},
			expectedEdges: 2,
			expectedKinds: []string{"ConfigMap", "Secret"},
		},
		{
			name: "projected volume with multiple configMaps",
			podData: map[string]interface{}{
				"spec": map[string]interface{}{
					"volumes": []interface{}{
						map[string]interface{}{
							"name": "projected-vol",
							"projected": map[string]interface{}{
								"sources": []interface{}{
									map[string]interface{}{
										"configMap": map[string]interface{}{"name": "cm1"},
									},
									map[string]interface{}{
										"configMap": map[string]interface{}{"name": "cm2"},
									},
								},
							},
						},
					},
				},
			},
			mockResources: []*graph.ResourceIdentity{
				{UID: "cm1-uid", Kind: "ConfigMap", Namespace: "default", Name: "cm1"},
				{UID: "cm2-uid", Kind: "ConfigMap", Namespace: "default", Name: "cm2"},
			},
			expectedEdges: 2,
			expectedKinds: []string{"ConfigMap", "ConfigMap"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewPodConfigSecretExtractor()
			lookup := extractors.NewMockResourceLookup()

			for _, res := range tt.mockResources {
				lookup.AddResource(res)
			}

			podJSON, err := json.Marshal(tt.podData)
			require.NoError(t, err)

			event := models.Event{
				Resource: models.ResourceMetadata{
					UID: "pod-uid", Kind: "Pod", Namespace: "default", Name: "test-pod",
				},
				Data: podJSON,
				Type: models.EventTypeUpdate,
			}

			edges, err := extractor.ExtractRelationships(ctx, event, lookup)
			assert.NoError(t, err)
			assert.Len(t, edges, tt.expectedEdges)

			for i, edge := range edges {
				assert.Equal(t, graph.EdgeTypeReferencesSpec, edge.Type)
				var props graph.ReferencesSpecEdge
				err := json.Unmarshal(edge.Properties, &props)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedKinds[i], props.RefKind)
			}
		})
	}
}

func TestPodConfigSecretExtractor_EnvFromReferences(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		podData       map[string]interface{}
		mockResources []*graph.ResourceIdentity
		expectedEdges int
		expectedKinds []string
	}{
		{
			name: "envFrom configMapRef",
			podData: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name": "app",
							"envFrom": []interface{}{
								map[string]interface{}{
									"configMapRef": map[string]interface{}{"name": "env-config"},
								},
							},
						},
					},
				},
			},
			mockResources: []*graph.ResourceIdentity{
				{UID: "cm-uid", Kind: "ConfigMap", Namespace: "default", Name: "env-config"},
			},
			expectedEdges: 1,
			expectedKinds: []string{"ConfigMap"},
		},
		{
			name: "envFrom secretRef",
			podData: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name": "app",
							"envFrom": []interface{}{
								map[string]interface{}{
									"secretRef": map[string]interface{}{"name": "env-secret"},
								},
							},
						},
					},
				},
			},
			mockResources: []*graph.ResourceIdentity{
				{UID: "secret-uid", Kind: "Secret", Namespace: "default", Name: "env-secret"},
			},
			expectedEdges: 1,
			expectedKinds: []string{"Secret"},
		},
		{
			name: "envFrom with both configMapRef and secretRef",
			podData: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name": "app",
							"envFrom": []interface{}{
								map[string]interface{}{
									"configMapRef": map[string]interface{}{"name": "env-config"},
								},
								map[string]interface{}{
									"secretRef": map[string]interface{}{"name": "env-secret"},
								},
							},
						},
					},
				},
			},
			mockResources: []*graph.ResourceIdentity{
				{UID: "cm-uid", Kind: "ConfigMap", Namespace: "default", Name: "env-config"},
				{UID: "secret-uid", Kind: "Secret", Namespace: "default", Name: "env-secret"},
			},
			expectedEdges: 2,
			expectedKinds: []string{"ConfigMap", "Secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewPodConfigSecretExtractor()
			lookup := extractors.NewMockResourceLookup()

			for _, res := range tt.mockResources {
				lookup.AddResource(res)
			}

			podJSON, err := json.Marshal(tt.podData)
			require.NoError(t, err)

			event := models.Event{
				Resource: models.ResourceMetadata{
					UID: "pod-uid", Kind: "Pod", Namespace: "default", Name: "test-pod",
				},
				Data: podJSON,
				Type: models.EventTypeUpdate,
			}

			edges, err := extractor.ExtractRelationships(ctx, event, lookup)
			assert.NoError(t, err)
			assert.Len(t, edges, tt.expectedEdges)

			for i, edge := range edges {
				assert.Equal(t, graph.EdgeTypeReferencesSpec, edge.Type)
				var props graph.ReferencesSpecEdge
				err := json.Unmarshal(edge.Properties, &props)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedKinds[i], props.RefKind)
			}
		})
	}
}

func TestPodConfigSecretExtractor_EnvValueFromReferences(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		podData       map[string]interface{}
		mockResources []*graph.ResourceIdentity
		expectedEdges int
		expectedKinds []string
	}{
		{
			name: "env valueFrom configMapKeyRef",
			podData: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name": "app",
							"env": []interface{}{
								map[string]interface{}{
									"name": "CONFIG_VAL",
									"valueFrom": map[string]interface{}{
										"configMapKeyRef": map[string]interface{}{
											"name": "config-values",
											"key":  "some-key",
										},
									},
								},
							},
						},
					},
				},
			},
			mockResources: []*graph.ResourceIdentity{
				{UID: "cm-uid", Kind: "ConfigMap", Namespace: "default", Name: "config-values"},
			},
			expectedEdges: 1,
			expectedKinds: []string{"ConfigMap"},
		},
		{
			name: "env valueFrom secretKeyRef",
			podData: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name": "app",
							"env": []interface{}{
								map[string]interface{}{
									"name": "SECRET_VAL",
									"valueFrom": map[string]interface{}{
										"secretKeyRef": map[string]interface{}{
											"name": "secret-values",
											"key":  "password",
										},
									},
								},
							},
						},
					},
				},
			},
			mockResources: []*graph.ResourceIdentity{
				{UID: "secret-uid", Kind: "Secret", Namespace: "default", Name: "secret-values"},
			},
			expectedEdges: 1,
			expectedKinds: []string{"Secret"},
		},
		{
			name: "multiple env vars with different references",
			podData: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name": "app",
							"env": []interface{}{
								map[string]interface{}{
									"name": "CONFIG_VAL",
									"valueFrom": map[string]interface{}{
										"configMapKeyRef": map[string]interface{}{
											"name": "config-values",
											"key":  "key1",
										},
									},
								},
								map[string]interface{}{
									"name": "SECRET_VAL",
									"valueFrom": map[string]interface{}{
										"secretKeyRef": map[string]interface{}{
											"name": "secret-values",
											"key":  "key2",
										},
									},
								},
							},
						},
					},
				},
			},
			mockResources: []*graph.ResourceIdentity{
				{UID: "cm-uid", Kind: "ConfigMap", Namespace: "default", Name: "config-values"},
				{UID: "secret-uid", Kind: "Secret", Namespace: "default", Name: "secret-values"},
			},
			expectedEdges: 2,
			expectedKinds: []string{"ConfigMap", "Secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewPodConfigSecretExtractor()
			lookup := extractors.NewMockResourceLookup()

			for _, res := range tt.mockResources {
				lookup.AddResource(res)
			}

			podJSON, err := json.Marshal(tt.podData)
			require.NoError(t, err)

			event := models.Event{
				Resource: models.ResourceMetadata{
					UID: "pod-uid", Kind: "Pod", Namespace: "default", Name: "test-pod",
				},
				Data: podJSON,
				Type: models.EventTypeUpdate,
			}

			edges, err := extractor.ExtractRelationships(ctx, event, lookup)
			assert.NoError(t, err)
			assert.Len(t, edges, tt.expectedEdges)

			for i, edge := range edges {
				assert.Equal(t, graph.EdgeTypeReferencesSpec, edge.Type)
				var props graph.ReferencesSpecEdge
				err := json.Unmarshal(edge.Properties, &props)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedKinds[i], props.RefKind)
			}
		})
	}
}

func TestPodConfigSecretExtractor_InitContainers(t *testing.T) {
	ctx := context.Background()
	extractor := NewPodConfigSecretExtractor()
	lookup := extractors.NewMockResourceLookup()

	lookup.AddResource(&graph.ResourceIdentity{
		UID: "init-cm-uid", Kind: "ConfigMap", Namespace: "default", Name: "init-config",
	})
	lookup.AddResource(&graph.ResourceIdentity{
		UID: "app-cm-uid", Kind: "ConfigMap", Namespace: "default", Name: "app-config",
	})

	podData := map[string]interface{}{
		"spec": map[string]interface{}{
			"initContainers": []interface{}{
				map[string]interface{}{
					"name": "init",
					"envFrom": []interface{}{
						map[string]interface{}{
							"configMapRef": map[string]interface{}{"name": "init-config"},
						},
					},
				},
			},
			"containers": []interface{}{
				map[string]interface{}{
					"name": "app",
					"envFrom": []interface{}{
						map[string]interface{}{
							"configMapRef": map[string]interface{}{"name": "app-config"},
						},
					},
				},
			},
		},
	}

	podJSON, err := json.Marshal(podData)
	require.NoError(t, err)

	event := models.Event{
		Resource: models.ResourceMetadata{
			UID: "pod-uid", Kind: "Pod", Namespace: "default", Name: "test-pod",
		},
		Data: podJSON,
		Type: models.EventTypeUpdate,
	}

	edges, err := extractor.ExtractRelationships(ctx, event, lookup)
	assert.NoError(t, err)
	assert.Len(t, edges, 2, "Should extract edges from both initContainers and containers")

	// Verify one edge references init-config and one references app-config
	var foundInit, foundApp bool
	for _, edge := range edges {
		var props graph.ReferencesSpecEdge
		err := json.Unmarshal(edge.Properties, &props)
		require.NoError(t, err)

		if props.RefName == "init-config" {
			foundInit = true
			assert.Contains(t, props.FieldPath, "initContainers[0]")
		}
		if props.RefName == "app-config" {
			foundApp = true
			assert.Contains(t, props.FieldPath, "containers[0]")
		}
	}

	assert.True(t, foundInit, "Should find edge for init container config")
	assert.True(t, foundApp, "Should find edge for app container config")
}

func TestPodConfigSecretExtractor_EdgeCases(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		podData       map[string]interface{}
		eventType     models.EventType
		mockResources []*graph.ResourceIdentity
		expectedEdges int
	}{
		{
			name:          "deleted pod skipped",
			podData:       map[string]interface{}{},
			eventType:     models.EventTypeDelete,
			mockResources: []*graph.ResourceIdentity{},
			expectedEdges: 0,
		},
		{
			name: "missing target resource (not in graph yet)",
			podData: map[string]interface{}{
				"spec": map[string]interface{}{
					"volumes": []interface{}{
						map[string]interface{}{
							"name": "config-vol",
							"configMap": map[string]interface{}{"name": "non-existent"},
						},
					},
				},
			},
			eventType:     models.EventTypeUpdate,
			mockResources: []*graph.ResourceIdentity{}, // No mock resource added
			expectedEdges: 0,                            // Edge is filtered out
		},
		{
			name: "empty spec",
			podData: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "empty-pod"},
			},
			eventType:     models.EventTypeUpdate,
			mockResources: []*graph.ResourceIdentity{},
			expectedEdges: 0,
		},
		{
			name: "duplicate references to same configmap",
			podData: map[string]interface{}{
				"spec": map[string]interface{}{
					"volumes": []interface{}{
						map[string]interface{}{
							"name": "vol1",
							"configMap": map[string]interface{}{"name": "shared-config"},
						},
					},
					"containers": []interface{}{
						map[string]interface{}{
							"name": "app",
							"envFrom": []interface{}{
								map[string]interface{}{
									"configMapRef": map[string]interface{}{"name": "shared-config"},
								},
							},
						},
					},
				},
			},
			eventType: models.EventTypeUpdate,
			mockResources: []*graph.ResourceIdentity{
				{UID: "cm-uid", Kind: "ConfigMap", Namespace: "default", Name: "shared-config"},
			},
			expectedEdges: 2, // Two edges with different fieldPaths
		},
		{
			name: "no volumes or containers",
			podData: map[string]interface{}{
				"spec": map[string]interface{}{},
			},
			eventType:     models.EventTypeUpdate,
			mockResources: []*graph.ResourceIdentity{},
			expectedEdges: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewPodConfigSecretExtractor()
			lookup := extractors.NewMockResourceLookup()

			for _, res := range tt.mockResources {
				lookup.AddResource(res)
			}

			podJSON, err := json.Marshal(tt.podData)
			require.NoError(t, err)

			event := models.Event{
				Resource: models.ResourceMetadata{
					UID: "pod-uid", Kind: "Pod", Namespace: "default", Name: "test-pod",
				},
				Data: podJSON,
				Type: tt.eventType,
			}

			edges, err := extractor.ExtractRelationships(ctx, event, lookup)
			assert.NoError(t, err)
			assert.Len(t, edges, tt.expectedEdges)
		})
	}
}

func TestPodConfigSecretExtractor_FieldPaths(t *testing.T) {
	ctx := context.Background()
	extractor := NewPodConfigSecretExtractor()
	lookup := extractors.NewMockResourceLookup()

	lookup.AddResource(&graph.ResourceIdentity{
		UID: "cm-uid", Kind: "ConfigMap", Namespace: "default", Name: "my-cm",
	})

	podData := map[string]interface{}{
		"spec": map[string]interface{}{
			"volumes": []interface{}{
				map[string]interface{}{
					"name": "vol",
					"configMap": map[string]interface{}{"name": "my-cm"},
				},
			},
		},
	}

	podJSON, err := json.Marshal(podData)
	require.NoError(t, err)

	event := models.Event{
		Resource: models.ResourceMetadata{
			UID: "pod-uid", Kind: "Pod", Namespace: "default", Name: "test-pod",
		},
		Data: podJSON,
		Type: models.EventTypeUpdate,
	}

	edges, err := extractor.ExtractRelationships(ctx, event, lookup)
	require.NoError(t, err)
	require.Len(t, edges, 1)

	var props graph.ReferencesSpecEdge
	err = json.Unmarshal(edges[0].Properties, &props)
	require.NoError(t, err)

	assert.Equal(t, "spec.volumes[0].configMap", props.FieldPath)
	assert.Equal(t, "ConfigMap", props.RefKind)
	assert.Equal(t, "my-cm", props.RefName)
	assert.Equal(t, "default", props.RefNamespace)
}
