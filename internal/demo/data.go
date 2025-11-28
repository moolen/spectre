package demo

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/moritz/rpk/internal/models"
)

// GetDemoEvents returns a comprehensive set of demo events showcasing various Kubernetes scenarios
func GetDemoEvents(startUnixTimestamp int64) []models.Event {
	startTime := time.Unix(startUnixTimestamp, 0)
	var events []models.Event

	// Scenario 1: Deployment with ImagePullBackOff (misconfigured image)
	deploymentName := "api-service"
	deploymentUID := "550e8400-e29b-41d4-a716-446655440001"
	imageErrors := createImagePullBackOffScenario(startTime, deploymentName, deploymentUID)
	events = append(events, imageErrors...)

	// Scenario 2: Node with disk pressure
	nodeName := "worker-node-2"
	nodeUID := "550e8400-e29b-41d4-a716-446655440002"
	diskPressureEvents := createDiskPressureScenario(startTime, nodeName, nodeUID)
	events = append(events, diskPressureEvents...)

	// Scenario 3: HelmRelease failed update
	helmReleaseName := "prometheus-stack"
	helmReleaseUID := "550e8400-e29b-41d4-a716-446655440003"
	helmEvents := createHelmReleaseFailureScenario(startTime, helmReleaseName, helmReleaseUID)
	events = append(events, helmEvents...)

	// Scenario 4: Pod restart loop
	podName := "worker-queue-processor-abc123"
	podUID := "550e8400-e29b-41d4-a716-446655440004"
	crashLoopEvents := createPodRestartLoopScenario(startTime, podName, podUID)
	events = append(events, crashLoopEvents...)

	// Scenario 5: StatefulSet successful deployment
	statefulSetName := "postgres-database"
	statefulSetUID := "550e8400-e29b-41d4-a716-446655440005"
	successfulEvents := createSuccessfulDeploymentScenario(startTime, statefulSetName, statefulSetUID)
	events = append(events, successfulEvents...)

	// Scenario 6: Service exposing multiple replicas with transitions
	serviceName := "web-frontend"
	serviceUID := "550e8400-e29b-41d4-a716-446655440006"
	scaleEvents := createScalingScenario(startTime, serviceName, serviceUID)
	events = append(events, scaleEvents...)

	// Scenario 7: ConfigMap update with pod reconciliation
	configMapName := "app-config"
	configMapUID := "550e8400-e29b-41d4-a716-446655440007"
	configEvents := createConfigMapUpdateScenario(startTime, configMapName, configMapUID)
	events = append(events, configEvents...)

	return events
}

// createImagePullBackOffScenario creates a deployment with image pull errors that eventually resolves
func createImagePullBackOffScenario(startTime time.Time, deploymentName, deploymentUID string) []models.Event {
	var events []models.Event
	ts := startTime

	// CREATE event
	createData := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      deploymentName,
			"namespace": "default",
			"uid":       deploymentUID,
		},
		"spec": map[string]interface{}{
			"replicas": 3,
			"selector": map[string]interface{}{
				"matchLabels": map[string]string{"app": "api-service"},
			},
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{
						{
							"name":  "api",
							"image": "myregistry.azurecr.io/api:typo", // Typo in image name
						},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(createData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-1", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeCreate,
		Resource:       makeResourceMetadata("apps", "v1", "Deployment", "default", deploymentName, deploymentUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	// UPDATE event - pods failing with ImagePullBackOff
	ts = ts.Add(2 * time.Second)
	updateData := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []map[string]interface{}{
				{
					"type":    "Progressing",
					"status":  "False",
					"reason":  "ProgressDeadlineExceeded",
					"message": "ReplicaSet \"api-service-5d4f7b\" has timed out progressing.",
				},
			},
			"unavailableReplicas": 3,
		},
	}
	data, _ = json.Marshal(updateData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-2", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeUpdate,
		Resource:       makeResourceMetadata("apps", "v1", "Deployment", "default", deploymentName, deploymentUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	// Kubernetes Events for the failing pods
	for i := 0; i < 3; i++ {
		ts = ts.Add(1 * time.Second)
		podName := fmt.Sprintf("%s-pod-%d", deploymentName, i)
		podUID := fmt.Sprintf("550e8400-e29b-41d4-a716-4466554400%02d", 10+i)
		eventData := map[string]interface{}{
			"reason":  "ImagePullBackOff",
			"message": "Back-off pulling image \"myregistry.azurecr.io/api:typo\"",
			"type":    "Warning",
		}
		data, _ = json.Marshal(eventData)
		events = append(events, models.Event{
			ID:        fmt.Sprintf("evt-%d-pod-%d", ts.Unix(), i),
			Timestamp: ts.UnixNano(),
			Type:      models.EventTypeCreate,
			Resource:  makeResourceMetadata("", "v1", "Event", "default", podName, podUID),
			Data:      data,
			DataSize:  int32(len(data)),
		})
	}

	// Fix: Update deployment with correct image
	ts = ts.Add(5 * time.Minute)
	fixData := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{
						{
							"name":  "api",
							"image": "myregistry.azurecr.io/api:v1.2.3",
						},
					},
				},
			},
		},
	}
	data, _ = json.Marshal(fixData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-fixed", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeUpdate,
		Resource:       makeResourceMetadata("apps", "v1", "Deployment", "default", deploymentName, deploymentUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	// Success: All replicas ready
	ts = ts.Add(30 * time.Second)
	successData := map[string]interface{}{
		"status": map[string]interface{}{
			"replicas":          3,
			"readyReplicas":     3,
			"updatedReplicas":   3,
			"availableReplicas": 3,
			"conditions": []map[string]interface{}{
				{
					"type":    "Available",
					"status":  "True",
					"reason":  "MinimumReplicasAvailable",
					"message": "Deployment has minimum availability.",
				},
			},
		},
	}
	data, _ = json.Marshal(successData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-success", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeUpdate,
		Resource:       makeResourceMetadata("apps", "v1", "Deployment", "default", deploymentName, deploymentUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	return events
}

// createDiskPressureScenario creates a node condition with disk pressure
func createDiskPressureScenario(startTime time.Time, nodeName, nodeUID string) []models.Event {
	var events []models.Event
	ts := startTime

	// CREATE node (cluster-scoped, no namespace)
	createData := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Node",
		"metadata": map[string]interface{}{
			"name": nodeName,
			"uid":  nodeUID,
		},
		"status": map[string]interface{}{
			"conditions": []map[string]interface{}{
				{
					"type":   "Ready",
					"status": "True",
				},
			},
		},
	}
	data, _ := json.Marshal(createData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-node-create", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeCreate,
		Resource:       makeResourceMetadata("", "v1", "Node", "", nodeName, nodeUID), // cluster-scoped
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	// UPDATE: Disk pressure detected
	ts = ts.Add(1 * time.Hour)
	updateData := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []map[string]interface{}{
				{
					"type":    "DiskPressure",
					"status":  "True",
					"reason":  "KubeletHasDiskPressure",
					"message": "/var/lib/kubelet is at 90% capacity",
				},
				{
					"type":   "Ready",
					"status": "True",
				},
			},
		},
	}
	data, _ = json.Marshal(updateData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-disk-pressure", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeUpdate,
		Resource:       makeResourceMetadata("", "v1", "Node", "", nodeName, nodeUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	// Kubernetes Event warning
	ts = ts.Add(5 * time.Second)
	eventData := map[string]interface{}{
		"reason":  "DiskPressure",
		"message": "Node has disk pressure condition",
		"type":    "Warning",
	}
	data, _ = json.Marshal(eventData)
	events = append(events, models.Event{
		ID:        fmt.Sprintf("evt-%d-disk-warning", ts.Unix()),
		Timestamp: ts.UnixNano(),
		Type:      models.EventTypeCreate,
		Resource:  makeResourceMetadata("", "v1", "Event", "", fmt.Sprintf("%s.disk", nodeName), nodeUID),
		Data:      data,
		DataSize:  int32(len(data)),
	})

	// Resolution: Cleanup and disk pressure resolves
	ts = ts.Add(2 * time.Hour)
	resolveData := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []map[string]interface{}{
				{
					"type":    "DiskPressure",
					"status":  "False",
					"reason":  "KubeletHasNoDiskPressure",
					"message": "kubelet has no disk pressure",
				},
				{
					"type":   "Ready",
					"status": "True",
				},
			},
		},
	}
	data, _ = json.Marshal(resolveData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-disk-resolved", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeUpdate,
		Resource:       makeResourceMetadata("", "v1", "Node", "", nodeName, nodeUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	return events
}

// createHelmReleaseFailureScenario creates a HelmRelease with failed update
func createHelmReleaseFailureScenario(startTime time.Time, releaseName, releaseUID string) []models.Event {
	var events []models.Event
	ts := startTime

	// CREATE HelmRelease
	createData := map[string]interface{}{
		"apiVersion": "helm.toolkit.fluxcd.io/v2beta1",
		"kind":       "HelmRelease",
		"metadata": map[string]interface{}{
			"name":      releaseName,
			"namespace": "monitoring",
			"uid":       releaseUID,
		},
		"spec": map[string]interface{}{
			"chart": "prometheus-community/kube-prometheus-stack",
			"values": map[string]interface{}{
				"prometheus": map[string]interface{}{
					"retention": "15d",
				},
			},
		},
	}
	data, _ := json.Marshal(createData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-helm-create", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeCreate,
		Resource:       makeResourceMetadata("helm.toolkit.fluxcd.io", "v2beta1", "HelmRelease", "monitoring", releaseName, releaseUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	// UPDATE: Helm install in progress
	ts = ts.Add(5 * time.Second)
	createData["status"] = map[string]interface{}{
		"conditions": []map[string]interface{}{
			{
				"type":    "Ready",
				"status":  "Unknown",
				"reason":  "Progressing",
				"message": "installing Helm release",
			},
		},
	}
	data, _ = json.Marshal(createData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-helm-progress", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeUpdate,
		Resource:       makeResourceMetadata("helm.toolkit.fluxcd.io", "v2beta1", "HelmRelease", "monitoring", releaseName, releaseUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	// UPDATE: Helm release update triggered
	ts = ts.Add(10 * time.Second)
	createData["spec"] = map[string]interface{}{
		"chart": "prometheus-community/kube-prometheus-stack",
		"values": map[string]interface{}{
			"prometheus": map[string]interface{}{
				"retention": "30d", // Changed from 15d
			},
		},
	}
	data, _ = json.Marshal(createData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-helm-update", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeUpdate,
		Resource:       makeResourceMetadata("helm.toolkit.fluxcd.io", "v2beta1", "HelmRelease", "monitoring", releaseName, releaseUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	// UPDATE: Helm upgrade fails due to CRD mismatch
	ts = ts.Add(30 * time.Second)
	createData["status"] = map[string]interface{}{
		"conditions": []map[string]interface{}{
			{
				"type":    "Ready",
				"status":  "False",
				"reason":  "HelmInstallFailed",
				"message": "Helm upgrade failed: could not find a ready Kubernetes api-server to connect to",
			},
		},
		"lastAttemptedRevision": "54.0.0",
	}
	data, _ = json.Marshal(createData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-helm-failure", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeUpdate,
		Resource:       makeResourceMetadata("helm.toolkit.fluxcd.io", "v2beta1", "HelmRelease", "monitoring", releaseName, releaseUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	// Recovery: Manual fix and retry
	ts = ts.Add(15 * time.Minute)
	createData["status"] = map[string]interface{}{
		"conditions": []map[string]interface{}{
			{
				"type":    "Ready",
				"status":  "True",
				"reason":  "ReconciliationSucceeded",
				"message": "Release reconciliation succeeded",
			},
		},
		"lastAppliedRevision": "54.0.0",
	}
	data, _ = json.Marshal(createData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-helm-recovery", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeUpdate,
		Resource:       makeResourceMetadata("helm.toolkit.fluxcd.io", "v2beta1", "HelmRelease", "monitoring", releaseName, releaseUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	return events
}

// createPodRestartLoopScenario creates a pod in a crash loop
func createPodRestartLoopScenario(startTime time.Time, podName, podUID string) []models.Event {
	var events []models.Event
	ts := startTime

	// CREATE pod
	createData := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      podName,
			"namespace": "default",
			"uid":       podUID,
		},
		"spec": map[string]interface{}{
			"containers": []map[string]interface{}{
				{
					"name":  "worker",
					"image": "myapp:v1",
				},
			},
			"restartPolicy": "Always",
		},
	}
	data, _ := json.Marshal(createData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-pod-create", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeCreate,
		Resource:       makeResourceMetadata("", "v1", "Pod", "default", podName, podUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	// Multiple restart cycles
	for cycle := 0; cycle < 3; cycle++ {
		// Running
		ts = ts.Add(2 * time.Second)
		runningData := map[string]interface{}{
			"status": map[string]interface{}{
				"phase": "Running",
				"containerStatuses": []map[string]interface{}{
					{
						"name": "worker",
						"state": map[string]interface{}{
							"running": map[string]interface{}{
								"startedAt": ts.Format(time.RFC3339),
							},
						},
					},
				},
			},
		}
		data, _ = json.Marshal(runningData)
		events = append(events, models.Event{
			ID:             fmt.Sprintf("evt-%d-pod-running-%d", ts.Unix(), cycle),
			Timestamp:      ts.UnixNano(),
			Type:           models.EventTypeUpdate,
			Resource:       makeResourceMetadata("", "v1", "Pod", "default", podName, podUID),
			Data:           data,
			DataSize:       int32(len(data)),
			CompressedSize: int32(len(data) / 2),
		})

		// Crashed
		ts = ts.Add(10 * time.Second)
		crashData := map[string]interface{}{
			"status": map[string]interface{}{
				"phase": "Failed",
				"containerStatuses": []map[string]interface{}{
					{
						"name":         "worker",
						"restartCount": cycle + 1,
						"state": map[string]interface{}{
							"terminated": map[string]interface{}{
								"exitCode": 1,
								"reason":   "Error",
								"message":  "Process died with exit code 1",
							},
						},
					},
				},
			},
		}
		data, _ = json.Marshal(crashData)
		events = append(events, models.Event{
			ID:             fmt.Sprintf("evt-%d-pod-crashed-%d", ts.Unix(), cycle),
			Timestamp:      ts.UnixNano(),
			Type:           models.EventTypeUpdate,
			Resource:       makeResourceMetadata("", "v1", "Pod", "default", podName, podUID),
			Data:           data,
			DataSize:       int32(len(data)),
			CompressedSize: int32(len(data) / 2),
		})

		// BackOffRe-starting
		ts = ts.Add(2 * time.Second)
		backoffData := map[string]interface{}{
			"status": map[string]interface{}{
				"phase": "Unknown",
				"containerStatuses": []map[string]interface{}{
					{
						"name":         "worker",
						"restartCount": cycle + 1,
						"state": map[string]interface{}{
							"waiting": map[string]interface{}{
								"reason":  "CrashLoopBackOff",
								"message": "back-off restarting failed container worker",
							},
						},
					},
				},
			},
		}
		data, _ = json.Marshal(backoffData)
		events = append(events, models.Event{
			ID:             fmt.Sprintf("evt-%d-pod-backoff-%d", ts.Unix(), cycle),
			Timestamp:      ts.UnixNano(),
			Type:           models.EventTypeUpdate,
			Resource:       makeResourceMetadata("", "v1", "Pod", "default", podName, podUID),
			Data:           data,
			DataSize:       int32(len(data)),
			CompressedSize: int32(len(data) / 2),
		})

		// Event logged
		ts = ts.Add(1 * time.Second)
		eventData := map[string]interface{}{
			"reason":  "BackOff",
			"message": "Back-off restarting failed container worker in pod " + podName,
			"type":    "Warning",
		}
		data, _ = json.Marshal(eventData)
		events = append(events, models.Event{
			ID:        fmt.Sprintf("evt-%d-pod-event-%d", ts.Unix(), cycle),
			Timestamp: ts.UnixNano(),
			Type:      models.EventTypeCreate,
			Resource:  makeResourceMetadata("", "v1", "Event", "default", podName, podUID),
			Data:      data,
			DataSize:  int32(len(data)),
		})
	}

	return events
}

// createSuccessfulDeploymentScenario creates a StatefulSet that deploys successfully
func createSuccessfulDeploymentScenario(startTime time.Time, statefulSetName, statefulSetUID string) []models.Event {
	var events []models.Event
	ts := startTime

	// CREATE
	createData := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "StatefulSet",
		"metadata": map[string]interface{}{
			"name":      statefulSetName,
			"namespace": "data",
			"uid":       statefulSetUID,
		},
		"spec": map[string]interface{}{
			"serviceName": "postgres",
			"replicas":    3,
			"selector": map[string]interface{}{
				"matchLabels": map[string]string{"app": statefulSetName},
			},
		},
	}
	data, _ := json.Marshal(createData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-sts-create", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeCreate,
		Resource:       makeResourceMetadata("apps", "v1", "StatefulSet", "data", statefulSetName, statefulSetUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	// Pods scaling up
	for i := 0; i < 3; i++ {
		ts = ts.Add(15 * time.Second)
		statusData := map[string]interface{}{
			"status": map[string]interface{}{
				"replicas":           i + 1,
				"readyReplicas":      i,
				"observedGeneration": 1,
			},
		}
		data, _ = json.Marshal(statusData)
		events = append(events, models.Event{
			ID:             fmt.Sprintf("evt-%d-sts-scale-%d", ts.Unix(), i),
			Timestamp:      ts.UnixNano(),
			Type:           models.EventTypeUpdate,
			Resource:       makeResourceMetadata("apps", "v1", "StatefulSet", "data", statefulSetName, statefulSetUID),
			Data:           data,
			DataSize:       int32(len(data)),
			CompressedSize: int32(len(data) / 2),
		})
	}

	// Final ready state
	ts = ts.Add(30 * time.Second)
	readyData := map[string]interface{}{
		"status": map[string]interface{}{
			"replicas":           3,
			"readyReplicas":      3,
			"currentReplicas":    3,
			"observedGeneration": 1,
			"conditions": []map[string]interface{}{
				{
					"type":    "Ready",
					"status":  "True",
					"reason":  "AllReplicasReady",
					"message": "StatefulSet replicas are ready",
				},
			},
		},
	}
	data, _ = json.Marshal(readyData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-sts-ready", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeUpdate,
		Resource:       makeResourceMetadata("apps", "v1", "StatefulSet", "data", statefulSetName, statefulSetUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	return events
}

// createScalingScenario creates scaling events for a Service
func createScalingScenario(startTime time.Time, serviceName, serviceUID string) []models.Event {
	var events []models.Event
	ts := startTime

	// CREATE Service
	createData := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": map[string]interface{}{
			"name":      serviceName,
			"namespace": "default",
			"uid":       serviceUID,
		},
		"spec": map[string]interface{}{
			"type": "LoadBalancer",
			"selector": map[string]string{
				"app": serviceName,
			},
			"ports": []map[string]interface{}{
				{
					"port":       80,
					"targetPort": 8080,
				},
			},
		},
	}
	data, _ := json.Marshal(createData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-svc-create", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeCreate,
		Resource:       makeResourceMetadata("", "v1", "Service", "default", serviceName, serviceUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	// UPDATE: Endpoint appearing
	ts = ts.Add(5 * time.Second)
	endpointData := map[string]interface{}{
		"status": map[string]interface{}{
			"loadBalancer": map[string]interface{}{
				"ingress": []map[string]interface{}{
					{
						"hostname": "web-frontend.elb.amazonaws.com",
					},
				},
			},
		},
	}
	data, _ = json.Marshal(endpointData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-svc-endpoint", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeUpdate,
		Resource:       makeResourceMetadata("", "v1", "Service", "default", serviceName, serviceUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	return events
}

// createConfigMapUpdateScenario creates ConfigMap and updates triggering pod reconciliation
func createConfigMapUpdateScenario(startTime time.Time, configMapName, configMapUID string) []models.Event {
	var events []models.Event
	ts := startTime

	// CREATE ConfigMap
	createData := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      configMapName,
			"namespace": "default",
			"uid":       configMapUID,
		},
		"data": map[string]interface{}{
			"app.conf": "debug=false\nlogLevel=info",
		},
	}
	data, _ := json.Marshal(createData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-cm-create", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeCreate,
		Resource:       makeResourceMetadata("", "v1", "ConfigMap", "default", configMapName, configMapUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	// UPDATE ConfigMap
	ts = ts.Add(2 * time.Hour)
	updateData := map[string]interface{}{
		"data": map[string]interface{}{
			"app.conf": "debug=false\nlogLevel=debug",
		},
	}
	data, _ = json.Marshal(updateData)
	events = append(events, models.Event{
		ID:             fmt.Sprintf("evt-%d-cm-update", ts.Unix()),
		Timestamp:      ts.UnixNano(),
		Type:           models.EventTypeUpdate,
		Resource:       makeResourceMetadata("", "v1", "ConfigMap", "default", configMapName, configMapUID),
		Data:           data,
		DataSize:       int32(len(data)),
		CompressedSize: int32(len(data) / 2),
	})

	// Pod reconciliation triggered
	ts = ts.Add(5 * time.Second)
	podUID := "550e8400-e29b-41d4-a716-446655440099"
	podReconciliationData := map[string]interface{}{
		"reason":  "TriggeredByConfigMap",
		"message": "ConfigMap app-config updated, triggering pod restart",
		"type":    "Normal",
	}
	data, _ = json.Marshal(podReconciliationData)
	events = append(events, models.Event{
		ID:        fmt.Sprintf("evt-%d-pod-reconcile", ts.Unix()),
		Timestamp: ts.UnixNano(),
		Type:      models.EventTypeCreate,
		Resource:  makeResourceMetadata("", "v1", "Event", "default", "app-pod", podUID),
		Data:      data,
		DataSize:  int32(len(data)),
	})

	return events
}

// makeResourceMetadata is a helper to create ResourceMetadata
func makeResourceMetadata(group, version, kind, namespace, name, uid string) models.ResourceMetadata {
	return models.ResourceMetadata{
		Group:     group,
		Version:   version,
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
		UID:       uid,
	}
}
