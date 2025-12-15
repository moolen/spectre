package storage

import (
	"testing"
)

// TestInferStatusFromResource_NodeHealthy tests a healthy node
func TestInferStatusFromResource_NodeHealthy(t *testing.T) {
	data := []byte(`{
		"status": {
			"conditions": [
				{"type": "Ready", "status": "True"},
				{"type": "MemoryPressure", "status": "False"},
				{"type": "DiskPressure", "status": "False"}
			]
		}
	}`)

	status := InferStatusFromResource("Node", data, "UPDATE")
	if status != resourceStatusReady {
		t.Errorf("Expected Ready for healthy node, got %s", status)
	}
}

// TestInferStatusFromResource_NodeNotReady tests a NotReady node
func TestInferStatusFromResource_NodeNotReady(t *testing.T) {
	data := []byte(`{
		"status": {
			"conditions": [
				{"type": "Ready", "status": "False", "reason": "KubeletNotReady", "message": "Node is not ready"}
			]
		}
	}`)

	status := InferStatusFromResource("Node", data, "UPDATE")
	if status != resourceStatusError {
		t.Errorf("Expected Error for NotReady node, got %s", status)
	}
}

// TestInferStatusFromResource_NodeMemoryPressure tests a node with memory pressure
func TestInferStatusFromResource_NodeMemoryPressure(t *testing.T) {
	data := []byte(`{
		"status": {
			"conditions": [
				{"type": "Ready", "status": "True"},
				{"type": "MemoryPressure", "status": "True", "reason": "KubeletHasInsufficientMemory"}
			]
		}
	}`)

	status := InferStatusFromResource("Node", data, "UPDATE")
	if status != resourceStatusWarning {
		t.Errorf("Expected Warning for node with MemoryPressure, got %s", status)
	}
}

// TestInferStatusFromResource_NodeDiskPressure tests a node with disk pressure
func TestInferStatusFromResource_NodeDiskPressure(t *testing.T) {
	data := []byte(`{
		"status": {
			"conditions": [
				{"type": "Ready", "status": "True"},
				{"type": "DiskPressure", "status": "True", "reason": "KubeletHasDiskPressure"}
			]
		}
	}`)

	status := InferStatusFromResource("Node", data, "UPDATE")
	if status != resourceStatusWarning {
		t.Errorf("Expected Warning for node with DiskPressure, got %s", status)
	}
}

// TestInferStatusFromResource_NodePIDPressure tests a node with PID pressure
func TestInferStatusFromResource_NodePIDPressure(t *testing.T) {
	data := []byte(`{
		"status": {
			"conditions": [
				{"type": "Ready", "status": "True"},
				{"type": "PIDPressure", "status": "True"}
			]
		}
	}`)

	status := InferStatusFromResource("Node", data, "UPDATE")
	if status != resourceStatusWarning {
		t.Errorf("Expected Warning for node with PIDPressure, got %s", status)
	}
}

// TestInferStatusFromResource_NodeNetworkUnavailable tests a node with network issues
func TestInferStatusFromResource_NodeNetworkUnavailable(t *testing.T) {
	data := []byte(`{
		"status": {
			"conditions": [
				{"type": "Ready", "status": "True"},
				{"type": "NetworkUnavailable", "status": "True", "reason": "NetworkPluginNotReady"}
			]
		}
	}`)

	status := InferStatusFromResource("Node", data, "UPDATE")
	if status != resourceStatusError {
		t.Errorf("Expected Error for node with NetworkUnavailable, got %s", status)
	}
}

// TestInferStatusFromResource_StatefulSetReady tests a healthy StatefulSet
func TestInferStatusFromResource_StatefulSetReady(t *testing.T) {
	data := []byte(`{
		"spec": {"replicas": 3},
		"status": {
			"replicas": 3,
			"readyReplicas": 3
		}
	}`)

	status := InferStatusFromResource("StatefulSet", data, "UPDATE")
	if status != resourceStatusReady {
		t.Errorf("Expected Ready for healthy StatefulSet, got %s", status)
	}
}

// TestInferStatusFromResource_StatefulSetNotReady tests a StatefulSet with unready replicas
func TestInferStatusFromResource_StatefulSetNotReady(t *testing.T) {
	data := []byte(`{
		"spec": {"replicas": 3},
		"status": {
			"replicas": 3,
			"readyReplicas": 1
		}
	}`)

	status := InferStatusFromResource("StatefulSet", data, "UPDATE")
	if status != resourceStatusWarning {
		t.Errorf("Expected Warning for StatefulSet with unready replicas, got %s", status)
	}
}

// TestInferStatusFromResource_JobComplete tests a completed Job
func TestInferStatusFromResource_JobComplete(t *testing.T) {
	data := []byte(`{
		"status": {
			"conditions": [
				{"type": "Complete", "status": "True"}
			],
			"succeeded": 1
		}
	}`)

	status := InferStatusFromResource("Job", data, "UPDATE")
	if status != resourceStatusReady {
		t.Errorf("Expected Ready for completed Job, got %s", status)
	}
}

// TestInferStatusFromResource_JobFailed tests a failed Job
func TestInferStatusFromResource_JobFailed(t *testing.T) {
	data := []byte(`{
		"status": {
			"conditions": [
				{"type": "Failed", "status": "True", "reason": "BackoffLimitExceeded"}
			],
			"failed": 3
		}
	}`)

	status := InferStatusFromResource("Job", data, "UPDATE")
	if status != resourceStatusError {
		t.Errorf("Expected Error for failed Job, got %s", status)
	}
}

// TestInferStatusFromResource_JobRunning tests a running Job
func TestInferStatusFromResource_JobRunning(t *testing.T) {
	data := []byte(`{
		"status": {
			"active": 1
		}
	}`)

	status := InferStatusFromResource("Job", data, "UPDATE")
	if status != resourceStatusReady {
		t.Errorf("Expected Ready for running Job, got %s", status)
	}
}

// TestInferStatusFromResource_PVCBound tests a bound PVC
func TestInferStatusFromResource_PVCBound(t *testing.T) {
	data := []byte(`{
		"status": {
			"phase": "Bound"
		}
	}`)

	status := InferStatusFromResource("PersistentVolumeClaim", data, "UPDATE")
	if status != resourceStatusReady {
		t.Errorf("Expected Ready for bound PVC, got %s", status)
	}
}

// TestInferStatusFromResource_PVCPending tests a pending PVC
func TestInferStatusFromResource_PVCPending(t *testing.T) {
	data := []byte(`{
		"status": {
			"phase": "Pending"
		}
	}`)

	status := InferStatusFromResource("PersistentVolumeClaim", data, "UPDATE")
	if status != resourceStatusWarning {
		t.Errorf("Expected Warning for pending PVC, got %s", status)
	}
}

// TestInferStatusFromResource_PVCLost tests a lost PVC
func TestInferStatusFromResource_PVCLost(t *testing.T) {
	data := []byte(`{
		"status": {
			"phase": "Lost"
		}
	}`)

	status := InferStatusFromResource("PersistentVolumeClaim", data, "UPDATE")
	if status != resourceStatusError {
		t.Errorf("Expected Error for lost PVC, got %s", status)
	}
}

// TestInferStatusFromResource_PodWithCrashLoopBackOff tests pod with CrashLoopBackOff
func TestInferStatusFromResource_PodWithCrashLoopBackOff(t *testing.T) {
	data := []byte(`{
		"status": {
			"phase": "Running",
			"containerStatuses": [{
				"name": "app",
				"restartCount": 5,
				"state": {
					"waiting": {
						"reason": "CrashLoopBackOff",
						"message": "Back-off restarting failed container"
					}
				}
			}]
		}
	}`)

	status := InferStatusFromResource("Pod", data, "UPDATE")
	if status != resourceStatusError {
		t.Errorf("Expected Error for pod with CrashLoopBackOff, got %s", status)
	}
}

// TestInferStatusFromResource_PodWithImagePullBackOff tests pod with ImagePullBackOff
func TestInferStatusFromResource_PodWithImagePullBackOff(t *testing.T) {
	data := []byte(`{
		"status": {
			"phase": "Pending",
			"containerStatuses": [{
				"name": "nginx",
				"state": {
					"waiting": {
						"reason": "ImagePullBackOff",
						"message": "Back-off pulling image"
					}
				}
			}]
		}
	}`)

	status := InferStatusFromResource("Pod", data, "UPDATE")
	if status != resourceStatusError {
		t.Errorf("Expected Error for pod with ImagePullBackOff, got %s", status)
	}
}

// TestInferStatusFromResource_PodWithOOMKilled tests pod with OOMKilled container
func TestInferStatusFromResource_PodWithOOMKilled(t *testing.T) {
	data := []byte(`{
		"status": {
			"phase": "Running",
			"containerStatuses": [{
				"name": "memory-hog",
				"restartCount": 3,
				"state": {
					"waiting": {
						"reason": "CrashLoopBackOff"
					}
				},
				"lastState": {
					"terminated": {
						"reason": "OOMKilled",
						"exitCode": 137,
						"message": "Container exceeded memory limit"
					}
				}
			}]
		}
	}`)

	status := InferStatusFromResource("Pod", data, "UPDATE")
	if status != resourceStatusError {
		t.Errorf("Expected Error for pod with OOMKilled container, got %s", status)
	}
}

// TestInferStatusFromResource_PodWithReadinessProbeFailure tests pod with failed readiness probe
func TestInferStatusFromResource_PodWithReadinessProbeFailure(t *testing.T) {
	data := []byte(`{
		"status": {
			"phase": "Running",
			"conditions": [
				{"type": "Ready", "status": "False", "reason": "ContainersNotReady"}
			],
			"containerStatuses": [{
				"name": "app",
				"ready": false,
				"state": {
					"running": {
						"startedAt": "2025-01-01T00:00:00Z"
					}
				}
			}]
		}
	}`)

	status := InferStatusFromResource("Pod", data, "UPDATE")
	if status != resourceStatusWarning {
		t.Errorf("Expected Warning for pod with readiness probe failure, got %s", status)
	}
}

// TestInferStatusFromResource_DeploymentUnavailableReplicas tests deployment with unavailable replicas
func TestInferStatusFromResource_DeploymentUnavailableReplicas(t *testing.T) {
	data := []byte(`{
		"spec": {"replicas": 3},
		"status": {
			"replicas": 3,
			"readyReplicas": 2,
			"availableReplicas": 2,
			"unavailableReplicas": 1
		}
	}`)

	status := InferStatusFromResource("Deployment", data, "UPDATE")
	if status != resourceStatusWarning {
		t.Errorf("Expected Warning for deployment with unavailable replicas, got %s", status)
	}
}

// TestInferStatusFromResource_DeploymentProgressingCondition tests deployment progressing
func TestInferStatusFromResource_DeploymentProgressingCondition(t *testing.T) {
	data := []byte(`{
		"spec": {"replicas": 2},
		"status": {
			"replicas": 2,
			"readyReplicas": 2,
			"conditions": [
				{"type": "Available", "status": "True"},
				{"type": "Progressing", "status": "True", "reason": "NewReplicaSetAvailable"}
			]
		}
	}`)

	status := InferStatusFromResource("Deployment", data, "UPDATE")
	if status != resourceStatusWarning {
		t.Errorf("Expected Warning for progressing deployment, got %s", status)
	}
}

// TestInferStatusFromResource_DeploymentProgressingFailed tests deployment with failed progression
func TestInferStatusFromResource_DeploymentProgressingFailed(t *testing.T) {
	data := []byte(`{
		"spec": {"replicas": 2},
		"status": {
			"replicas": 2,
			"readyReplicas": 0,
			"conditions": [
				{"type": "Progressing", "status": "False", "reason": "ProgressDeadlineExceeded"}
			]
		}
	}`)

	status := InferStatusFromResource("Deployment", data, "UPDATE")
	if status != resourceStatusError {
		t.Errorf("Expected Error for deployment with failed progression, got %s", status)
	}
}

// TestInferStatusFromResource_ServiceAlwaysReady tests that Service is always ready
func TestInferStatusFromResource_ServiceAlwaysReady(t *testing.T) {
	data := []byte(`{
		"spec": {
			"clusterIP": "10.0.0.1"
		}
	}`)

	status := InferStatusFromResource("Service", data, "UPDATE")
	if status != resourceStatusReady {
		t.Errorf("Expected Ready for Service, got %s", status)
	}
}

// TestInferStatusFromResource_ConfigMapAlwaysReady tests that ConfigMap is always ready
func TestInferStatusFromResource_ConfigMapAlwaysReady(t *testing.T) {
	data := []byte(`{
		"data": {
			"key": "value"
		}
	}`)

	status := InferStatusFromResource("ConfigMap", data, "UPDATE")
	if status != resourceStatusReady {
		t.Errorf("Expected Ready for ConfigMap, got %s", status)
	}
}

// TestInferStatusFromResource_SecretAlwaysReady tests that Secret is always ready
func TestInferStatusFromResource_SecretAlwaysReady(t *testing.T) {
	data := []byte(`{
		"data": {
			"password": "c2VjcmV0"
		}
	}`)

	status := InferStatusFromResource("Secret", data, "UPDATE")
	if status != resourceStatusReady {
		t.Errorf("Expected Ready for Secret, got %s", status)
	}
}

// TestInferStatusFromResource_PodUnknownPhase tests pod with unknown phase
func TestInferStatusFromResource_PodUnknownPhase(t *testing.T) {
	data := []byte(`{
		"status": {
			"phase": "Unknown"
		}
	}`)

	status := InferStatusFromResource("Pod", data, "UPDATE")
	if status != resourceStatusWarning {
		t.Errorf("Expected Error for pod with Unknown phase, got %s", status)
	}
}

// TestInferStatusFromResource_PodSucceeded tests succeeded pod
func TestInferStatusFromResource_PodSucceeded(t *testing.T) {
	data := []byte(`{
		"status": {
			"phase": "Succeeded"
		}
	}`)

	status := InferStatusFromResource("Pod", data, "UPDATE")
	if status != resourceStatusReady {
		t.Errorf("Expected Ready for succeeded pod, got %s", status)
	}
}

// TestInferStatusFromResource_ReplicaSetReady tests a ready ReplicaSet
func TestInferStatusFromResource_ReplicaSetReady(t *testing.T) {
	data := []byte(`{
		"spec": {"replicas": 2},
		"status": {
			"replicas": 2,
			"readyReplicas": 2
		}
	}`)

	status := InferStatusFromResource("ReplicaSet", data, "UPDATE")
	if status != resourceStatusReady {
		t.Errorf("Expected Ready for healthy ReplicaSet, got %s", status)
	}
}

// TestInferStatusFromResource_ReplicaSetNotReady tests ReplicaSet with unready replicas
func TestInferStatusFromResource_ReplicaSetNotReady(t *testing.T) {
	data := []byte(`{
		"spec": {"replicas": 3},
		"status": {
			"replicas": 3,
			"readyReplicas": 1
		}
	}`)

	status := InferStatusFromResource("ReplicaSet", data, "UPDATE")
	if status != resourceStatusWarning {
		t.Errorf("Expected Warning for ReplicaSet with unready replicas, got %s", status)
	}
}
