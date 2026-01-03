package graph

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/moolen/spectre/internal/models"
)

// CreatePodEvent creates a Pod event with realistic Kubernetes data
func CreatePodEvent(uid, name, namespace string, timestamp time.Time, eventType models.EventType, status string, data map[string]interface{}) models.Event {
	if data == nil {
		data = map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"uid":       uid,
			},
			"spec": map[string]interface{}{
				"containers": []map[string]interface{}{
					{
						"name":  "main",
						"image": "nginx:latest",
					},
				},
			},
			"status": map[string]interface{}{
				"phase": status,
			},
		}
	}

	dataBytes, _ := json.Marshal(data)

	event := models.Event{
		ID:        uuid.New().String(),
		Timestamp: timestamp.UnixNano(),
		Type:      eventType,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: namespace,
			Name:      name,
			UID:       uid,
		},
		Data:     dataBytes,
		DataSize: int32(len(dataBytes)),
	}

	return event
}

// CreateDeploymentEvent creates a Deployment event with realistic Kubernetes data
func CreateDeploymentEvent(uid, name, namespace string, timestamp time.Time, eventType models.EventType, replicas int32) models.Event {
	data := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"uid":       uid,
		},
		"spec": map[string]interface{}{
			"replicas": replicas,
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"app": name,
				},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app": name,
					},
				},
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{
						{
							"name":  "main",
							"image": "nginx:latest",
						},
					},
				},
			},
		},
	}

	dataBytes, _ := json.Marshal(data)

	event := models.Event{
		ID:        uuid.New().String(),
		Timestamp: timestamp.UnixNano(),
		Type:      eventType,
		Resource: models.ResourceMetadata{
			Group:     "apps",
			Version:   "v1",
			Kind:      "Deployment",
			Namespace: namespace,
			Name:      name,
			UID:       uid,
		},
		Data:     dataBytes,
		DataSize: int32(len(dataBytes)),
	}

	return event
}

// CreateServiceEvent creates a Service event
func CreateServiceEvent(uid, name, namespace string, timestamp time.Time, eventType models.EventType) models.Event {
	data := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"uid":       uid,
		},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{
				"app": name,
			},
			"ports": []map[string]interface{}{
				{
					"port":     80,
					"protocol": "TCP",
				},
			},
		},
	}

	dataBytes, _ := json.Marshal(data)

	event := models.Event{
		ID:        uuid.New().String(),
		Timestamp: timestamp.UnixNano(),
		Type:      eventType,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Service",
			Namespace: namespace,
			Name:      name,
			UID:       uid,
		},
		Data:     dataBytes,
		DataSize: int32(len(dataBytes)),
	}

	return event
}

// CreateHelmReleaseEvent creates a HelmRelease event (Flux)
func CreateHelmReleaseEvent(uid, name, namespace string, timestamp time.Time, eventType models.EventType, chart string) models.Event {
	data := map[string]interface{}{
		"apiVersion": "helm.toolkit.fluxcd.io/v2beta1",
		"kind":       "HelmRelease",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"uid":       uid,
		},
		"spec": map[string]interface{}{
			"chart": map[string]interface{}{
				"spec": map[string]interface{}{
					"chart": chart,
				},
			},
		},
	}

	dataBytes, _ := json.Marshal(data)

	event := models.Event{
		ID:        uuid.New().String(),
		Timestamp: timestamp.UnixNano(),
		Type:      eventType,
		Resource: models.ResourceMetadata{
			Group:     "helm.toolkit.fluxcd.io",
			Version:   "v2beta1",
			Kind:      "HelmRelease",
			Namespace: namespace,
			Name:      name,
			UID:       uid,
		},
		Data:     dataBytes,
		DataSize: int32(len(dataBytes)),
	}

	return event
}

// CreateConfigMapEvent creates a ConfigMap event
func CreateConfigMapEvent(uid, name, namespace string, timestamp time.Time, eventType models.EventType, data map[string]string) models.Event {
	cmData := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"uid":       uid,
		},
	}

	if data != nil {
		cmData["data"] = data
	}

	dataBytes, _ := json.Marshal(cmData)

	event := models.Event{
		ID:        uuid.New().String(),
		Timestamp: timestamp.UnixNano(),
		Type:      eventType,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "ConfigMap",
			Namespace: namespace,
			Name:      name,
			UID:       uid,
		},
		Data:     dataBytes,
		DataSize: int32(len(dataBytes)),
	}

	return event
}

// CreateReplicaSetEvent creates a ReplicaSet event
func CreateReplicaSetEvent(uid, name, namespace string, timestamp time.Time, eventType models.EventType, replicas int32, ownerUID string) models.Event {
	metadata := map[string]interface{}{
		"name":      name,
		"namespace": namespace,
		"uid":       uid,
	}

	if ownerUID != "" {
		metadata["ownerReferences"] = []map[string]interface{}{
			{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"name":       "parent-deployment",
				"uid":        ownerUID,
				"controller": true,
			},
		}
	}

	data := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "ReplicaSet",
		"metadata":   metadata,
		"spec": map[string]interface{}{
			"replicas": replicas,
		},
	}

	dataBytes, _ := json.Marshal(data)

	event := models.Event{
		ID:        uuid.New().String(),
		Timestamp: timestamp.UnixNano(),
		Type:      eventType,
		Resource: models.ResourceMetadata{
			Group:     "apps",
			Version:   "v1",
			Kind:      "ReplicaSet",
			Namespace: namespace,
			Name:      name,
			UID:       uid,
		},
		Data:     dataBytes,
		DataSize: int32(len(dataBytes)),
	}

	return event
}

// CreateEventSequence creates a sequence of events for a single resource
func CreateEventSequence(resource models.ResourceMetadata, timestamps []time.Time, statuses []string) []models.Event {
	events := make([]models.Event, 0, len(timestamps))

	for i, ts := range timestamps {
		var eventType models.EventType
		if i == 0 {
			eventType = models.EventTypeCreate
		} else if i == len(timestamps)-1 && len(statuses) > 0 && statuses[i] == "Deleted" {
			eventType = models.EventTypeDelete
		} else {
			eventType = models.EventTypeUpdate
		}

		var data map[string]interface{}
		switch resource.Kind {
		case "Pod":
			podData := map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name":      resource.Name,
					"namespace": resource.Namespace,
					"uid":       resource.UID,
				},
			}
			if len(statuses) > i && statuses[i] != "" {
				podData["status"] = map[string]interface{}{
					"phase": statuses[i],
				}
			}
			data = podData
		case "Deployment":
			data = map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      resource.Name,
					"namespace": resource.Namespace,
					"uid":       resource.UID,
				},
				"spec": map[string]interface{}{
					"replicas": 1,
				},
			}
		default:
			data = map[string]interface{}{
				"apiVersion": "v1",
				"kind":       resource.Kind,
				"metadata": map[string]interface{}{
					"name":      resource.Name,
					"namespace": resource.Namespace,
					"uid":       resource.UID,
				},
			}
		}

		dataBytes, _ := json.Marshal(data)

		event := models.Event{
			ID:        uuid.New().String(),
			Timestamp: ts.UnixNano(),
			Type:      eventType,
			Resource:  resource,
			Data:      dataBytes,
			DataSize:  int32(len(dataBytes)),
		}

		events = append(events, event)
	}

	return events
}

// CreateLifecycleEvents creates CREATE, UPDATE, DELETE sequence for a resource
func CreateLifecycleEvents(resource models.ResourceMetadata, startTime time.Time, duration time.Duration) []models.Event {
	createTime := startTime
	updateTime := startTime.Add(duration / 3)
	deleteTime := startTime.Add(2 * duration / 3)

	statuses := []string{"", "", "Deleted"}
	return CreateEventSequence(resource, []time.Time{createTime, updateTime, deleteTime}, statuses)
}

// CreateOwnershipChain creates events for an ownership chain (e.g., Deployment -> ReplicaSet -> Pod)
func CreateOwnershipChain(ownerUID, ownedUID string, timestamps []time.Time) []models.Event {
	events := make([]models.Event, 0)

	if len(timestamps) < 2 {
		return events
	}

	// Create owner (Deployment) first
	ownerEvent := CreateDeploymentEvent(
		ownerUID,
		"parent-deployment",
		"default",
		timestamps[0],
		models.EventTypeCreate,
		1,
	)
	events = append(events, ownerEvent)

	// Create owned (ReplicaSet) with owner reference
	ownedEvent := CreateReplicaSetEvent(
		ownedUID,
		"child-replicaset",
		"default",
		timestamps[1],
		models.EventTypeCreate,
		1,
		ownerUID,
	)
	events = append(events, ownedEvent)

	return events
}

// CreateFailureScenario creates a sequence of events representing a failure scenario
func CreateFailureScenario(baseTime time.Time) []models.Event {
	deploymentUID := uuid.New().String()
	replicasetUID := uuid.New().String()
	podUID := uuid.New().String()

	events := []models.Event{
		// Deployment created
		CreateDeploymentEvent(deploymentUID, "failing-deployment", "default", baseTime, models.EventTypeCreate, 1),
		// ReplicaSet created
		CreateReplicaSetEvent(replicasetUID, "failing-replicaset", "default", baseTime.Add(1*time.Second), models.EventTypeCreate, 1, deploymentUID),
		// Pod created and running
		CreatePodEvent(podUID, "failing-pod", "default", baseTime.Add(2*time.Second), models.EventTypeCreate, "Running", nil),
		// Pod fails
		CreatePodEvent(podUID, "failing-pod", "default", baseTime.Add(10*time.Second), models.EventTypeUpdate, "Error", nil),
	}

	return events
}
