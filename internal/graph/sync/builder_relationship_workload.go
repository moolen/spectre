package sync

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moolen/spectre/internal/graph"
)

// extractSchedulingRelationship extracts SCHEDULED_ON edge for Pod -> Node
func (b *graphBuilder) extractSchedulingRelationship(podUID string, resourceData map[string]interface{}) *graph.Edge {
	spec, ok := resourceData["spec"].(map[string]interface{})
	if !ok {
		return nil
	}

	nodeName, ok := spec["nodeName"].(string)
	if !ok || nodeName == "" {
		return nil
	}

	if b.client == nil {
		b.logger.Debug("No graph client available, cannot create SCHEDULED_ON edge for Pod %s -> Node %s", podUID, nodeName)
		return nil
	}

	nodeUID, err := b.findNodeUIDByName(context.Background(), nodeName)
	if err != nil {
		b.logger.Debug("Failed to find Node UID for name %s: %v", nodeName, err)
		return nil
	}

	if nodeUID == "" {
		b.logger.Debug("Node %s not found in graph yet, skipping SCHEDULED_ON edge", nodeName)
		return nil
	}

	var scheduledAt int64
	if status, ok := resourceData["status"].(map[string]interface{}); ok {
		if conditions, ok := status["conditions"].([]interface{}); ok {
			for _, condRaw := range conditions {
				if cond, ok := condRaw.(map[string]interface{}); ok {
					if condType, ok := cond["type"].(string); ok && condType == "PodScheduled" {
						if lastTransitionTime, ok := cond["lastTransitionTime"].(string); ok {
							if parsedTime, err := time.Parse(time.RFC3339, lastTransitionTime); err == nil {
								scheduledAt = parsedTime.UnixNano()
							}
						}
					}
				}
			}
		}
	}

	if scheduledAt == 0 {
		scheduledAt = time.Now().UnixNano()
	}

	propsJSON, _ := json.Marshal(graph.ScheduledOnEdge{
		ScheduledAt:  scheduledAt,
		TerminatedAt: 0,
	})

	edge := graph.Edge{
		Type:       graph.EdgeTypeScheduledOn,
		FromUID:    podUID,
		ToUID:      nodeUID,
		Properties: json.RawMessage(propsJSON),
	}

	b.logger.Debug("Created SCHEDULED_ON edge: Pod %s -> Node %s (name: %s)", podUID, nodeUID, nodeName)
	return &edge
}

// extractVolumeRelationships extracts MOUNTS edges for Pod -> PVC
func (b *graphBuilder) extractVolumeRelationships(podUID string, resourceData map[string]interface{}) []graph.Edge {
	edges := []graph.Edge{}

	if b.client == nil {
		return edges
	}

	spec, ok := resourceData["spec"].(map[string]interface{})
	if !ok {
		return edges
	}

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

		pvcUID, err := b.findPVCUIDByName(context.Background(), claimName, namespace)
		if err != nil {
			b.logger.Debug("Failed to find PVC UID for %s/%s: %v", namespace, claimName, err)
			continue
		}

		if pvcUID == "" {
			b.logger.Debug("PVC %s/%s not found in graph yet, skipping MOUNTS edge", namespace, claimName)
			continue
		}

		mountPath := volumeMounts[volumeName]

		propsJSON, _ := json.Marshal(graph.MountsEdge{
			VolumeName: volumeName,
			MountPath:  mountPath,
		})

		edges = append(edges, graph.Edge{
			Type:       graph.EdgeTypeMounts,
			FromUID:    podUID,
			ToUID:      pvcUID,
			Properties: json.RawMessage(propsJSON),
		})

		b.logger.Debug("Created MOUNTS edge: Pod %s -> PVC %s (name: %s/%s)", podUID, pvcUID, namespace, claimName)
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

	metadata, ok := resourceData["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}

	namespace, ok := metadata["namespace"].(string)
	if !ok || namespace == "" {
		return nil
	}

	saUID, err := b.findServiceAccountUIDByName(context.Background(), serviceAccountName, namespace)
	if err != nil {
		b.logger.Debug("Failed to find ServiceAccount UID for %s/%s: %v", namespace, serviceAccountName, err)
		return nil
	}

	if saUID == "" {
		b.logger.Debug("ServiceAccount %s/%s not found in graph yet, skipping USES_SERVICE_ACCOUNT edge", namespace, serviceAccountName)
		return nil
	}

	edge := graph.Edge{
		Type:       graph.EdgeTypeUsesServiceAccount,
		FromUID:    podUID,
		ToUID:      saUID,
		Properties: json.RawMessage("{}"),
	}

	b.logger.Debug("Created USES_SERVICE_ACCOUNT edge: Pod %s -> ServiceAccount %s (name: %s/%s)", podUID, saUID, namespace, serviceAccountName)
	return &edge
}
