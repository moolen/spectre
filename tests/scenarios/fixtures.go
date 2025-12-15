package scenarios

import (
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/mcp/client"
)

// CreateCrashLoopBackOffScenario creates a pod in CrashLoopBackOff state
func CreateCrashLoopBackOffScenario() *client.TimelineResponse {
	return &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID:        "pod/default/crashloop-pod",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "crashloop-pod",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Ready",
						Message:   "Pod started",
						StartTime: time.Now().Add(-10 * time.Minute).Unix(),
						EndTime:   time.Now().Add(-9 * time.Minute).Unix(),
					},
					{
						Status:    "Error",
						Message:   "Container app is in CrashLoopBackOff",
						StartTime: time.Now().Add(-9 * time.Minute).Unix(),
						EndTime:   time.Now().Unix(),
					},
				},
				Events: []client.K8sEvent{
					{
						Reason:         "BackOff",
						Message:        "Back-off restarting failed container app in pod crashloop-pod",
						Type:           "Warning",
						Count:          15,
						FirstTimestamp: time.Now().Add(-9 * time.Minute).Unix(),
						LastTimestamp:  time.Now().Unix(),
					},
				},
			},
		},
	}
}

// CreateImagePullBackOffScenario creates a pod stuck in ImagePullBackOff
func CreateImagePullBackOffScenario() *client.TimelineResponse {
	return &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID: "pod/default/imagepull-pod",
				Kind:       "Pod",
				Namespace:  "default",
				Name:       "imagepull-pod",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Error",
						Message:   "Container nginx is in ImagePullBackOff",
						StartTime: time.Now().Add(-5 * time.Minute).Unix(),
						EndTime:   time.Now().Unix(),
					},
				},
				Events: []client.K8sEvent{
					{
						Reason:         "Failed",
						Message:        "Failed to pull image \"invalid-image:latest\": rpc error: code = Unknown desc = Error response from daemon: manifest for invalid-image:latest not found",
						Type:           "Warning",
						Count:          10,
						FirstTimestamp: time.Now().Add(-5 * time.Minute).Unix(),
						LastTimestamp:  time.Now().Unix(),
					},
					{
						Reason:         "BackOff",
						Message:        "Back-off pulling image \"invalid-image:latest\"",
						Type:           "Warning",
						Count:          8,
						FirstTimestamp: time.Now().Add(-4 * time.Minute).Unix(),
						LastTimestamp:  time.Now().Unix(),
					},
				},
			},
		},
	}
}

// CreateOOMKillScenario creates a pod that was OOMKilled
func CreateOOMKillScenario() *client.TimelineResponse {
	return &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID: "pod/default/oomkill-pod",
				Kind:       "Pod",
				Namespace:  "default",
				Name:       "oomkill-pod",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Ready",
						Message:   "Pod running",
						StartTime: time.Now().Add(-15 * time.Minute).Unix(),
						EndTime:   time.Now().Add(-10 * time.Minute).Unix(),
					},
					{
						Status:    "Error",
						Message:   "Container memory-hog: OOMKilled",
						StartTime: time.Now().Add(-10 * time.Minute).Unix(),
						EndTime:   time.Now().Unix(),
					},
				},
				Events: []client.K8sEvent{
					{
						Reason:         "OOMKilling",
						Message:        "Memory cgroup out of memory: Killed process 1234 (app) total-vm:2097152kB, anon-rss:1048576kB, file-rss:0kB",
						Type:           "Warning",
						Count:          3,
						FirstTimestamp: time.Now().Add(-10 * time.Minute).Unix(),
						LastTimestamp:  time.Now().Add(-2 * time.Minute).Unix(),
					},
				},
			},
		},
	}
}

// CreateReadinessProbeFailureScenario creates a pod failing readiness probes after upgrade
func CreateReadinessProbeFailureScenario() *client.TimelineResponse {
	return &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID: "deployment/default/web",
				Kind:       "Deployment",
				Namespace:  "default",
				Name:       "web",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Ready",
						Message:   "Deployment has minimum availability",
						StartTime: time.Now().Add(-30 * time.Minute).Unix(),
						EndTime:   time.Now().Add(-10 * time.Minute).Unix(),
					},
					{
						Status:    "Warning",
						Message:   "ReplicaSet \"web-new\" has 2 unavailable replicas",
						StartTime: time.Now().Add(-10 * time.Minute).Unix(),
						EndTime:   time.Now().Unix(),
					},
				},
			},
			{
				ID: "pod/default/web-new-abc123",
				Kind:       "Pod",
				Namespace:  "default",
				Name:       "web-new-abc123",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Warning",
						Message:   "Readiness probe failed",
						StartTime: time.Now().Add(-10 * time.Minute).Unix(),
						EndTime:   time.Now().Unix(),
					},
				},
				Events: []client.K8sEvent{
					{
						Reason:         "Unhealthy",
						Message:        "Readiness probe failed: Get http://10.0.0.1:8080/health: dial tcp 10.0.0.1:8080: connect: connection refused",
						Type:           "Warning",
						Count:          20,
						FirstTimestamp: time.Now().Add(-10 * time.Minute).Unix(),
						LastTimestamp:  time.Now().Unix(),
					},
				},
			},
		},
	}
}

// CreateNodePressureScenario creates a node with memory pressure and evicting pods
func CreateNodePressureScenario() *client.TimelineResponse {
	return &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID: "node/worker-1",
				Kind:       "Node",
				Name:       "worker-1",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Ready",
						Message:   "Node is healthy",
						StartTime: time.Now().Add(-20 * time.Minute).Unix(),
						EndTime:   time.Now().Add(-5 * time.Minute).Unix(),
					},
					{
						Status:    "Warning",
						Message:   "Node pressure: MemoryPressure",
						StartTime: time.Now().Add(-5 * time.Minute).Unix(),
						EndTime:   time.Now().Unix(),
					},
				},
				Events: []client.K8sEvent{
					{
						Reason:         "NodeHasInsufficientMemory",
						Message:        "Node worker-1 status is now: NodeHasInsufficientMemory",
						Type:           "Normal",
						Count:          1,
						FirstTimestamp: time.Now().Add(-5 * time.Minute).Unix(),
						LastTimestamp:  time.Now().Add(-5 * time.Minute).Unix(),
					},
				},
			},
			{
				ID: "pod/default/evicted-pod",
				Kind:       "Pod",
				Namespace:  "default",
				Name:       "evicted-pod",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Ready",
						Message:   "Pod running",
						StartTime: time.Now().Add(-10 * time.Minute).Unix(),
						EndTime:   time.Now().Add(-3 * time.Minute).Unix(),
					},
					{
						Status:    "Terminating",
						Message:   "Pod evicted",
						StartTime: time.Now().Add(-3 * time.Minute).Unix(),
						EndTime:   time.Now().Unix(),
					},
				},
				Events: []client.K8sEvent{
					{
						Reason:         "Evicted",
						Message:        "The node was low on resource: memory. Container app was using 512Mi, which exceeds its request of 256Mi.",
						Type:           "Warning",
						Count:          1,
						FirstTimestamp: time.Now().Add(-3 * time.Minute).Unix(),
						LastTimestamp:  time.Now().Add(-3 * time.Minute).Unix(),
					},
				},
			},
		},
	}
}

// CreateUnschedulablePodScenario creates a pod that cannot be scheduled
func CreateUnschedulablePodScenario() *client.TimelineResponse {
	return &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID: "pod/default/unschedulable-pod",
				Kind:       "Pod",
				Namespace:  "default",
				Name:       "unschedulable-pod",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Warning",
						Message:   "Pod pending - unschedulable",
						StartTime: time.Now().Add(-7 * time.Minute).Unix(),
						EndTime:   time.Now().Unix(),
					},
				},
				Events: []client.K8sEvent{
					{
						Reason:         "FailedScheduling",
						Message:        "0/5 nodes are available: 3 Insufficient cpu, 2 node(s) didn't match node selector.",
						Type:           "Warning",
						Count:          12,
						FirstTimestamp: time.Now().Add(-7 * time.Minute).Unix(),
						LastTimestamp:  time.Now().Unix(),
					},
				},
			},
		},
	}
}

// CreateServiceNoEndpointsScenario creates a service with no backing endpoints
func CreateServiceNoEndpointsScenario() *client.TimelineResponse {
	return &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID: "service/default/backend",
				Kind:       "Service",
				Namespace:  "default",
				Name:       "backend",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Ready",
						Message:   "Service created",
						StartTime: time.Now().Add(-15 * time.Minute).Unix(),
						EndTime:   time.Now().Unix(),
					},
				},
			},
			{
				ID: "pod/default/backend-pod",
				Kind:       "Pod",
				Namespace:  "default",
				Name:       "backend-pod",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Error",
						Message:   "CrashLoopBackOff",
						StartTime: time.Now().Add(-10 * time.Minute).Unix(),
						EndTime:   time.Now().Unix(),
					},
				},
			},
		},
	}
}

// CreateNamespaceDeletionScenario creates a namespace being deleted with cascading resources
func CreateNamespaceDeletionScenario() *client.TimelineResponse {
	now := time.Now()
	deletionTime := now.Add(-2 * time.Minute)

	resources := []client.TimelineResource{
		{
			ID: "namespace/test-namespace",
			Kind:       "Namespace",
			Name:       "test-namespace",
			StatusSegments: []client.StatusSegment{
				{
					Status:    "Terminating",
					Message:   "Namespace is being deleted",
					StartTime: deletionTime.Unix(),
					EndTime:   now.Unix(),
				},
			},
		},
	}

	// Add 10 pods being deleted
	for i := 1; i <= 10; i++ {
		resources = append(resources, client.TimelineResource{
			ID: fmt.Sprintf("pod/test-namespace/app-%d", i),
			Kind:       "Pod",
			Namespace:  "test-namespace",
			Name:       fmt.Sprintf("app-%d", i),
			StatusSegments: []client.StatusSegment{
				{
					Status:    "Ready",
					Message:   "Pod running",
					StartTime: deletionTime.Add(-10 * time.Minute).Unix(),
					EndTime:   deletionTime.Unix(),
				},
				{
					Status:    "Terminating",
					Message:   "Pod is being deleted",
					StartTime: deletionTime.Unix(),
					EndTime:   now.Unix(),
				},
			},
		})
	}

	return &client.TimelineResponse{
		Resources: resources,
	}
}

// CreateDaemonSetSchedulingIssuesScenario creates a DaemonSet with scheduling problems
func CreateDaemonSetSchedulingIssuesScenario() *client.TimelineResponse {
	return &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID: "daemonset/kube-system/monitoring-agent",
				Kind:       "DaemonSet",
				Namespace:  "kube-system",
				Name:       "monitoring-agent",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Warning",
						Message:   "DaemonSet has unavailable pods",
						StartTime: time.Now().Add(-15 * time.Minute).Unix(),
						EndTime:   time.Now().Unix(),
					},
				},
				Events: []client.K8sEvent{
					{
						Reason:         "FailedScheduling",
						Message:        "0/3 nodes available: 3 node(s) had taint {node.kubernetes.io/disk-pressure: }, that the pod didn't tolerate.",
						Type:           "Warning",
						Count:          8,
						FirstTimestamp: time.Now().Add(-15 * time.Minute).Unix(),
						LastTimestamp:  time.Now().Unix(),
					},
				},
			},
		},
	}
}

// CreatePVCPendingScenario creates a PVC stuck in Pending state
func CreatePVCPendingScenario() *client.TimelineResponse {
	return &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID: "persistentvolumeclaim/default/data-claim",
				Kind:       "PersistentVolumeClaim",
				Namespace:  "default",
				Name:       "data-claim",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Warning",
						Message:   "PVC is pending",
						StartTime: time.Now().Add(-20 * time.Minute).Unix(),
						EndTime:   time.Now().Unix(),
					},
				},
				Events: []client.K8sEvent{
					{
						Reason:         "FailedBinding",
						Message:        "no persistent volumes available for this claim and no storage class is set",
						Type:           "Warning",
						Count:          25,
						FirstTimestamp: time.Now().Add(-20 * time.Minute).Unix(),
						LastTimestamp:  time.Now().Unix(),
					},
				},
			},
			{
				ID: "pod/default/app-waiting-for-volume",
				Kind:       "Pod",
				Namespace:  "default",
				Name:       "app-waiting-for-volume",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Warning",
						Message:   "Pod pending - waiting for volume",
						StartTime: time.Now().Add(-20 * time.Minute).Unix(),
						EndTime:   time.Now().Unix(),
					},
				},
			},
		},
	}
}
