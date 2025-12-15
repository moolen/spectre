package storage

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInferErrorMessages_Pod_CrashLoopBackOff(t *testing.T) {
	podJSON := `{
		"metadata": {"name": "test-pod"},
		"status": {
			"phase": "Running",
			"containerStatuses": [
				{
					"name": "app",
					"restartCount": 15,
					"state": {
						"waiting": {
							"reason": "CrashLoopBackOff",
							"message": "back-off 5m0s restarting failed container"
						}
					}
				}
			]
		}
	}`

	errors := InferErrorMessages("Pod", json.RawMessage(podJSON), resourceStatusError)

	if len(errors) == 0 {
		t.Fatal("Expected errors, got none")
	}

	errorStr := strings.Join(errors, "; ")
	if !strings.Contains(errorStr, "CrashLoopBackOff") {
		t.Errorf("Expected CrashLoopBackOff in error, got: %s", errorStr)
	}
	if !strings.Contains(errorStr, "app") {
		t.Errorf("Expected container name 'app' in error, got: %s", errorStr)
	}
	if !strings.Contains(errorStr, "15") {
		t.Errorf("Expected restart count '15' in error, got: %s", errorStr)
	}
}

func TestInferErrorMessages_Pod_ImagePullBackOff(t *testing.T) {
	podJSON := `{
		"metadata": {"name": "test-pod"},
		"status": {
			"phase": "Pending",
			"containerStatuses": [
				{
					"name": "nginx",
					"restartCount": 0,
					"state": {
						"waiting": {
							"reason": "ImagePullBackOff",
							"message": "Back-off pulling image \"nginx:nonexistent\""
						}
					}
				}
			]
		}
	}`

	errors := InferErrorMessages("Pod", json.RawMessage(podJSON), resourceStatusError)

	if len(errors) == 0 {
		t.Fatal("Expected errors, got none")
	}

	errorStr := strings.Join(errors, "; ")
	if !strings.Contains(errorStr, "ImagePullBackOff") {
		t.Errorf("Expected ImagePullBackOff in error, got: %s", errorStr)
	}
	if !strings.Contains(errorStr, "nginx") {
		t.Errorf("Expected container name 'nginx' in error, got: %s", errorStr)
	}
}

func TestInferErrorMessages_Pod_OOMKilled(t *testing.T) {
	podJSON := `{
		"metadata": {"name": "test-pod"},
		"status": {
			"phase": "Running",
			"containerStatuses": [
				{
					"name": "memory-hog",
					"restartCount": 3,
					"lastState": {
						"terminated": {
							"reason": "OOMKilled",
							"exitCode": 137,
							"message": "Container exceeded memory limit"
						}
					}
				}
			]
		}
	}`

	errors := InferErrorMessages("Pod", json.RawMessage(podJSON), resourceStatusError)

	if len(errors) == 0 {
		t.Fatal("Expected errors, got none")
	}

	errorStr := strings.Join(errors, "; ")
	if !strings.Contains(errorStr, "OOMKilled") {
		t.Errorf("Expected OOMKilled in error, got: %s", errorStr)
	}
	if !strings.Contains(errorStr, "memory-hog") {
		t.Errorf("Expected container name 'memory-hog' in error, got: %s", errorStr)
	}
	if !strings.Contains(errorStr, "137") {
		t.Errorf("Expected exit code '137' in error, got: %s", errorStr)
	}
}

func TestInferErrorMessages_Pod_MultipleIssues(t *testing.T) {
	podJSON := `{
		"metadata": {"name": "test-pod"},
		"status": {
			"phase": "Running",
			"containerStatuses": [
				{
					"name": "app",
					"restartCount": 15,
					"state": {
						"waiting": {
							"reason": "CrashLoopBackOff",
							"message": "back-off 5m0s restarting failed container"
						}
					}
				},
				{
					"name": "sidecar",
					"restartCount": 0,
					"state": {
						"waiting": {
							"reason": "ImagePullBackOff",
							"message": "Failed to pull image"
						}
					}
				}
			]
		}
	}`

	errors := InferErrorMessages("Pod", json.RawMessage(podJSON), resourceStatusError)

	if len(errors) < 2 {
		t.Fatalf("Expected at least 2 errors, got %d", len(errors))
	}

	errorStr := strings.Join(errors, "; ")
	if !strings.Contains(errorStr, "CrashLoopBackOff") {
		t.Errorf("Expected CrashLoopBackOff in error, got: %s", errorStr)
	}
	if !strings.Contains(errorStr, "ImagePullBackOff") {
		t.Errorf("Expected ImagePullBackOff in error, got: %s", errorStr)
	}
}

func TestInferErrorMessages_Pod_Pending(t *testing.T) {
	podJSON := `{
		"metadata": {"name": "test-pod"},
		"status": {
			"phase": "Pending",
			"conditions": [
				{
					"type": "PodScheduled",
					"status": "False",
					"reason": "Unschedulable",
					"message": "0/3 nodes are available: 3 Insufficient cpu"
				}
			]
		}
	}`

	errors := InferErrorMessages("Pod", json.RawMessage(podJSON), resourceStatusWarning)

	if len(errors) == 0 {
		t.Fatal("Expected errors, got none")
	}

	errorStr := strings.Join(errors, "; ")
	if !strings.Contains(errorStr, "scheduling failed") {
		t.Errorf("Expected 'scheduling failed' in error, got: %s", errorStr)
	}
	if !strings.Contains(errorStr, "Unschedulable") {
		t.Errorf("Expected 'Unschedulable' in error, got: %s", errorStr)
	}
}

func TestInferErrorMessages_Deployment_InsufficientReplicas(t *testing.T) {
	deploymentJSON := `{
		"metadata": {"name": "test-deployment"},
		"spec": {"replicas": 3},
		"status": {
			"replicas": 3,
			"readyReplicas": 1,
			"availableReplicas": 1,
			"unavailableReplicas": 2,
			"conditions": [
				{
					"type": "Available",
					"status": "False",
					"reason": "MinimumReplicasUnavailable",
					"message": "Deployment does not have minimum availability"
				}
			]
		}
	}`

	errors := InferErrorMessages("Deployment", json.RawMessage(deploymentJSON), resourceStatusError)

	if len(errors) == 0 {
		t.Fatal("Expected errors, got none")
	}

	errorStr := strings.Join(errors, "; ")
	if !strings.Contains(errorStr, "Insufficient replicas") || !strings.Contains(errorStr, "1/3") {
		t.Errorf("Expected 'Insufficient replicas (1/3 ready)' in error, got: %s", errorStr)
	}
	if !strings.Contains(errorStr, "2 unavailable replicas") {
		t.Errorf("Expected '2 unavailable replicas' in error, got: %s", errorStr)
	}
	if !strings.Contains(errorStr, "MinimumReplicasUnavailable") {
		t.Errorf("Expected 'MinimumReplicasUnavailable' in error, got: %s", errorStr)
	}
}

func TestInferErrorMessages_Node_NotReady(t *testing.T) {
	nodeJSON := `{
		"metadata": {"name": "node-1"},
		"status": {
			"conditions": [
				{
					"type": "Ready",
					"status": "False",
					"reason": "KubeletNotReady",
					"message": "container runtime network not ready"
				},
				{
					"type": "DiskPressure",
					"status": "True",
					"reason": "KubeletHasDiskPressure",
					"message": "kubelet has disk pressure"
				}
			]
		}
	}`

	errors := InferErrorMessages("Node", json.RawMessage(nodeJSON), resourceStatusError)

	if len(errors) < 2 {
		t.Fatalf("Expected at least 2 errors, got %d", len(errors))
	}

	errorStr := strings.Join(errors, "; ")
	if !strings.Contains(errorStr, "NotReady") {
		t.Errorf("Expected 'NotReady' in error, got: %s", errorStr)
	}
	if !strings.Contains(errorStr, "KubeletNotReady") {
		t.Errorf("Expected 'KubeletNotReady' in error, got: %s", errorStr)
	}
	if !strings.Contains(errorStr, "DiskPressure") {
		t.Errorf("Expected 'DiskPressure' in error, got: %s", errorStr)
	}
}

func TestInferErrorMessages_Job_Failed(t *testing.T) {
	jobJSON := `{
		"metadata": {"name": "test-job"},
		"status": {
			"failed": 3,
			"conditions": [
				{
					"type": "Failed",
					"status": "True",
					"reason": "BackoffLimitExceeded",
					"message": "Job has reached the specified backoff limit"
				}
			]
		}
	}`

	errors := InferErrorMessages("Job", json.RawMessage(jobJSON), resourceStatusError)

	if len(errors) == 0 {
		t.Fatal("Expected errors, got none")
	}

	errorStr := strings.Join(errors, "; ")
	if !strings.Contains(errorStr, "Job failed") {
		t.Errorf("Expected 'Job failed' in error, got: %s", errorStr)
	}
	if !strings.Contains(errorStr, "BackoffLimitExceeded") {
		t.Errorf("Expected 'BackoffLimitExceeded' in error, got: %s", errorStr)
	}
	if !strings.Contains(errorStr, "3 failed pods") {
		t.Errorf("Expected '3 failed pods' in error, got: %s", errorStr)
	}
}

func TestInferErrorMessages_PVC_Pending(t *testing.T) {
	pvcJSON := `{
		"metadata": {"name": "test-pvc"},
		"status": {
			"phase": "Pending"
		}
	}`

	errors := InferErrorMessages("PersistentVolumeClaim", json.RawMessage(pvcJSON), resourceStatusWarning)

	if len(errors) == 0 {
		t.Fatal("Expected errors, got none")
	}

	errorStr := strings.Join(errors, "; ")
	if !strings.Contains(errorStr, "PVC pending") {
		t.Errorf("Expected 'PVC pending' in error, got: %s", errorStr)
	}
}

func TestInferErrorMessages_ReadyStatus_NoErrors(t *testing.T) {
	podJSON := `{
		"metadata": {"name": "test-pod"},
		"status": {
			"phase": "Running",
			"conditions": [
				{
					"type": "Ready",
					"status": "True"
				}
			],
			"containerStatuses": [
				{
					"name": "app",
					"ready": true,
					"restartCount": 0
				}
			]
		}
	}`

	errors := InferErrorMessages("Pod", json.RawMessage(podJSON), resourceStatusReady)

	if len(errors) != 0 {
		t.Errorf("Expected no errors for Ready status, got: %v", errors)
	}
}

func TestInferErrorMessages_EmptyData(t *testing.T) {
	errors := InferErrorMessages("Pod", json.RawMessage(""), resourceStatusError)

	if len(errors) != 0 {
		t.Errorf("Expected no errors for empty data, got: %v", errors)
	}
}

func TestInferErrorMessages_InvalidJSON(t *testing.T) {
	errors := InferErrorMessages("Pod", json.RawMessage("{invalid json"), resourceStatusError)

	if len(errors) != 0 {
		t.Errorf("Expected no errors for invalid JSON, got: %v", errors)
	}
}

func TestInferErrorMessages_DaemonSet_Unavailable(t *testing.T) {
	daemonSetJSON := `{
		"metadata": {"name": "test-ds"},
		"status": {
			"desiredNumberScheduled": 5,
			"numberReady": 3,
			"numberUnavailable": 2,
			"numberMisscheduled": 1
		}
	}`

	errors := InferErrorMessages("DaemonSet", json.RawMessage(daemonSetJSON), resourceStatusWarning)

	if len(errors) == 0 {
		t.Fatal("Expected errors, got none")
	}

	errorStr := strings.Join(errors, "; ")
	if !strings.Contains(errorStr, "3/5 pods ready") {
		t.Errorf("Expected '3/5 pods ready' in error, got: %s", errorStr)
	}
	if !strings.Contains(errorStr, "2 pods unavailable") {
		t.Errorf("Expected '2 pods unavailable' in error, got: %s", errorStr)
	}
	if !strings.Contains(errorStr, "1 pods misscheduled") {
		t.Errorf("Expected '1 pods misscheduled' in error, got: %s", errorStr)
	}
}

func TestInferErrorMessages_StatefulSet_NotReady(t *testing.T) {
	statefulSetJSON := `{
		"metadata": {"name": "test-sts"},
		"spec": {"replicas": 3},
		"status": {
			"replicas": 3,
			"readyReplicas": 2,
			"currentReplicas": 2
		}
	}`

	errors := InferErrorMessages("StatefulSet", json.RawMessage(statefulSetJSON), resourceStatusWarning)

	if len(errors) == 0 {
		t.Fatal("Expected errors, got none")
	}

	errorStr := strings.Join(errors, "; ")
	if !strings.Contains(errorStr, "Insufficient replicas") || !strings.Contains(errorStr, "2/3") {
		t.Errorf("Expected 'Insufficient replicas (2/3 ready)' in error, got: %s", errorStr)
	}
}
