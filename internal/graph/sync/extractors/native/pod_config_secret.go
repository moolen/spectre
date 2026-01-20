package native

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync/extractors"
	"github.com/moolen/spectre/internal/models"
)

// PodConfigSecretExtractor extracts Pod→ConfigMap and Pod→Secret REFERENCES_SPEC relationships.
// It handles all three Kubernetes reference types:
// - Volume mounts: spec.volumes[].configMap.name, spec.volumes[].secret.secretName
// - EnvFrom: spec.containers[].envFrom[].configMapRef/secretRef
// - Env valueFrom: spec.containers[].env[].valueFrom.configMapKeyRef/secretKeyRef
type PodConfigSecretExtractor struct {
	*extractors.BaseExtractor
}

// NewPodConfigSecretExtractor creates a new Pod ConfigMap/Secret extractor.
func NewPodConfigSecretExtractor() *PodConfigSecretExtractor {
	return &PodConfigSecretExtractor{
		BaseExtractor: extractors.NewBaseExtractor("pod-config-secret", 50),
	}
}

// Matches returns true if the event is for a core/v1 Pod resource.
func (e *PodConfigSecretExtractor) Matches(event models.Event) bool {
	return event.Resource.Kind == "Pod" && event.Resource.Group == ""
}

// ExtractRelationships extracts ConfigMap and Secret references from Pod specs.
func (e *PodConfigSecretExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup extractors.ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	// Skip deleted Pods
	if event.Type == models.EventTypeDelete {
		return edges, nil
	}

	var pod map[string]interface{}
	if err := json.Unmarshal(event.Data, &pod); err != nil {
		return nil, fmt.Errorf("failed to parse Pod: %w", err)
	}

	namespace := event.Resource.Namespace
	podUID := event.Resource.UID

	spec, ok := extractors.GetNestedMap(pod, "spec")
	if !ok {
		return edges, nil
	}

	// Extract from volumes
	volumeEdges := e.extractVolumeReferences(ctx, spec, podUID, namespace, lookup)
	edges = append(edges, volumeEdges...)

	// Extract from containers
	containerEdges := e.extractContainerReferences(ctx, spec, "containers", podUID, namespace, lookup)
	edges = append(edges, containerEdges...)

	// Extract from initContainers
	initContainerEdges := e.extractContainerReferences(ctx, spec, "initContainers", podUID, namespace, lookup)
	edges = append(edges, initContainerEdges...)

	return edges, nil
}

// extractVolumeReferences extracts ConfigMap/Secret references from spec.volumes[].
func (e *PodConfigSecretExtractor) extractVolumeReferences(
	ctx context.Context,
	spec map[string]interface{},
	podUID, namespace string,
	lookup extractors.ResourceLookup,
) []graph.Edge {
	edges := []graph.Edge{}

	volumes, ok := extractors.GetNestedArray(spec, "volumes")
	if !ok {
		return edges
	}

	for idx, volInterface := range volumes {
		vol, ok := volInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// Handle ConfigMap volumes: spec.volumes[].configMap.name
		if configMap, ok := extractors.GetNestedMap(vol, "configMap"); ok {
			if name, ok := extractors.GetNestedString(configMap, "name"); ok {
				fieldPath := fmt.Sprintf("spec.volumes[%d].configMap", idx)
				if edge := e.createEdge(ctx, podUID, namespace, "ConfigMap", name, fieldPath, lookup); edge != nil {
					edges = append(edges, *edge)
				}
			}
		}

		// Handle Secret volumes: spec.volumes[].secret.secretName
		if secret, ok := extractors.GetNestedMap(vol, "secret"); ok {
			if name, ok := extractors.GetNestedString(secret, "secretName"); ok {
				fieldPath := fmt.Sprintf("spec.volumes[%d].secret", idx)
				if edge := e.createEdge(ctx, podUID, namespace, "Secret", name, fieldPath, lookup); edge != nil {
					edges = append(edges, *edge)
				}
			}
		}

		// Handle projected volumes: spec.volumes[].projected.sources[]
		if projected, ok := extractors.GetNestedMap(vol, "projected"); ok {
			projectedEdges := e.extractProjectedVolumeReferences(ctx, projected, idx, podUID, namespace, lookup)
			edges = append(edges, projectedEdges...)
		}
	}

	return edges
}

// extractProjectedVolumeReferences extracts ConfigMap/Secret references from projected volumes.
func (e *PodConfigSecretExtractor) extractProjectedVolumeReferences(
	ctx context.Context,
	projected map[string]interface{},
	volumeIdx int,
	podUID, namespace string,
	lookup extractors.ResourceLookup,
) []graph.Edge {
	edges := []graph.Edge{}

	sources, ok := extractors.GetNestedArray(projected, "sources")
	if !ok {
		return edges
	}

	for srcIdx, srcInterface := range sources {
		src, ok := srcInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// ConfigMap in projected: spec.volumes[].projected.sources[].configMap.name
		if configMap, ok := extractors.GetNestedMap(src, "configMap"); ok {
			if name, ok := extractors.GetNestedString(configMap, "name"); ok {
				fieldPath := fmt.Sprintf("spec.volumes[%d].projected.sources[%d].configMap", volumeIdx, srcIdx)
				if edge := e.createEdge(ctx, podUID, namespace, "ConfigMap", name, fieldPath, lookup); edge != nil {
					edges = append(edges, *edge)
				}
			}
		}

		// Secret in projected: spec.volumes[].projected.sources[].secret.name
		if secret, ok := extractors.GetNestedMap(src, "secret"); ok {
			if name, ok := extractors.GetNestedString(secret, "name"); ok {
				fieldPath := fmt.Sprintf("spec.volumes[%d].projected.sources[%d].secret", volumeIdx, srcIdx)
				if edge := e.createEdge(ctx, podUID, namespace, "Secret", name, fieldPath, lookup); edge != nil {
					edges = append(edges, *edge)
				}
			}
		}
	}

	return edges
}

// extractContainerReferences extracts ConfigMap/Secret references from containers or initContainers.
func (e *PodConfigSecretExtractor) extractContainerReferences(
	ctx context.Context,
	spec map[string]interface{},
	containerField string, // "containers" or "initContainers"
	podUID, namespace string,
	lookup extractors.ResourceLookup,
) []graph.Edge {
	edges := []graph.Edge{}

	containers, ok := extractors.GetNestedArray(spec, containerField)
	if !ok {
		return edges
	}

	for containerIdx, containerInterface := range containers {
		container, ok := containerInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract from envFrom
		envFromEdges := e.extractEnvFromReferences(ctx, container, containerField, containerIdx, podUID, namespace, lookup)
		edges = append(edges, envFromEdges...)

		// Extract from env[].valueFrom
		envEdges := e.extractEnvValueFromReferences(ctx, container, containerField, containerIdx, podUID, namespace, lookup)
		edges = append(edges, envEdges...)
	}

	return edges
}

// extractEnvFromReferences extracts from spec.containers[].envFrom[].
func (e *PodConfigSecretExtractor) extractEnvFromReferences(
	ctx context.Context,
	container map[string]interface{},
	containerField string,
	containerIdx int,
	podUID, namespace string,
	lookup extractors.ResourceLookup,
) []graph.Edge {
	edges := []graph.Edge{}

	envFroms, ok := extractors.GetNestedArray(container, "envFrom")
	if !ok {
		return edges
	}

	for envFromIdx, envFromInterface := range envFroms {
		envFrom, ok := envFromInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// ConfigMap envFrom: spec.containers[].envFrom[].configMapRef.name
		if configMapRef, ok := extractors.GetNestedMap(envFrom, "configMapRef"); ok {
			if name, ok := extractors.GetNestedString(configMapRef, "name"); ok {
				fieldPath := fmt.Sprintf("spec.%s[%d].envFrom[%d].configMapRef", containerField, containerIdx, envFromIdx)
				if edge := e.createEdge(ctx, podUID, namespace, "ConfigMap", name, fieldPath, lookup); edge != nil {
					edges = append(edges, *edge)
				}
			}
		}

		// Secret envFrom: spec.containers[].envFrom[].secretRef.name
		if secretRef, ok := extractors.GetNestedMap(envFrom, "secretRef"); ok {
			if name, ok := extractors.GetNestedString(secretRef, "name"); ok {
				fieldPath := fmt.Sprintf("spec.%s[%d].envFrom[%d].secretRef", containerField, containerIdx, envFromIdx)
				if edge := e.createEdge(ctx, podUID, namespace, "Secret", name, fieldPath, lookup); edge != nil {
					edges = append(edges, *edge)
				}
			}
		}
	}

	return edges
}

// extractEnvValueFromReferences extracts from spec.containers[].env[].valueFrom.
func (e *PodConfigSecretExtractor) extractEnvValueFromReferences(
	ctx context.Context,
	container map[string]interface{},
	containerField string,
	containerIdx int,
	podUID, namespace string,
	lookup extractors.ResourceLookup,
) []graph.Edge {
	edges := []graph.Edge{}

	envVars, ok := extractors.GetNestedArray(container, "env")
	if !ok {
		return edges
	}

	for envIdx, envVarInterface := range envVars {
		envVar, ok := envVarInterface.(map[string]interface{})
		if !ok {
			continue
		}

		valueFrom, ok := extractors.GetNestedMap(envVar, "valueFrom")
		if !ok {
			continue
		}

		// ConfigMap key ref: spec.containers[].env[].valueFrom.configMapKeyRef.name
		if configMapKeyRef, ok := extractors.GetNestedMap(valueFrom, "configMapKeyRef"); ok {
			if name, ok := extractors.GetNestedString(configMapKeyRef, "name"); ok {
				fieldPath := fmt.Sprintf("spec.%s[%d].env[%d].valueFrom.configMapKeyRef", containerField, containerIdx, envIdx)
				if edge := e.createEdge(ctx, podUID, namespace, "ConfigMap", name, fieldPath, lookup); edge != nil {
					edges = append(edges, *edge)
				}
			}
		}

		// Secret key ref: spec.containers[].env[].valueFrom.secretKeyRef.name
		if secretKeyRef, ok := extractors.GetNestedMap(valueFrom, "secretKeyRef"); ok {
			if name, ok := extractors.GetNestedString(secretKeyRef, "name"); ok {
				fieldPath := fmt.Sprintf("spec.%s[%d].env[%d].valueFrom.secretKeyRef", containerField, containerIdx, envIdx)
				if edge := e.createEdge(ctx, podUID, namespace, "Secret", name, fieldPath, lookup); edge != nil {
					edges = append(edges, *edge)
				}
			}
		}
	}

	return edges
}

// createEdge creates a REFERENCES_SPEC edge if the target resource exists.
func (e *PodConfigSecretExtractor) createEdge(
	ctx context.Context,
	podUID, namespace, kind, name, fieldPath string,
	lookup extractors.ResourceLookup,
) *graph.Edge {
	targetResource, err := lookup.FindResourceByNamespace(ctx, namespace, kind, name)
	if err != nil {
		e.Logger().Debug("Failed to lookup target resource: %v (namespace=%s, kind=%s, name=%s)",
			err, namespace, kind, name)
	}

	targetUID := ""
	if targetResource != nil {
		targetUID = targetResource.UID
	} else {
		e.Logger().Debug("Target resource not found in graph (namespace=%s, kind=%s, name=%s)",
			namespace, kind, name)
	}

	edge := e.CreateReferencesSpecEdge(
		podUID,
		targetUID,
		fieldPath,
		kind,
		name,
		namespace,
	)

	return extractors.ValidEdgeOrNil(edge)
}
