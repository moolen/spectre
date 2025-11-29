package storage

import "testing"

func TestInferStatusFromResource_DeploymentReady(t *testing.T) {
	data := []byte(`
{
  "metadata": {"name": "demo"},
  "spec": {"replicas": 2},
  "status": {
    "replicas": 2,
    "readyReplicas": 2,
    "availableReplicas": 2,
    "unavailableReplicas": 0
  }
}`)

	status := InferStatusFromResource("Deployment", data, "UPDATE")
	if status != resourceStatusReady {
		t.Fatalf("expected Ready, got %s", status)
	}
}

func TestInferStatusFromResource_DeploymentErrorCondition(t *testing.T) {
	data := []byte(`
{
  "metadata": {"name": "demo"},
  "spec": {"replicas": 2},
  "status": {
    "replicas": 2,
    "readyReplicas": 1,
    "availableReplicas": 1,
    "conditions": [
      {"type": "Available", "status": "False", "reason": "ErrorDeploying"}
    ]
  }
}`)

	status := InferStatusFromResource("Deployment", data, "UPDATE")
	if status != resourceStatusError {
		t.Fatalf("expected Error, got %s", status)
	}
}

func TestInferStatusFromResource_PodStates(t *testing.T) {
	pending := []byte(`{"status":{"phase":"Pending"}}`)
	if status := InferStatusFromResource("Pod", pending, "UPDATE"); status != resourceStatusWarning {
		t.Fatalf("expected Warning for pending pod, got %s", status)
	}

	running := []byte(`
{
  "status": {
    "phase": "Running",
    "conditions": [
      {"type": "Ready", "status": "True"}
    ]
  }
}`)
	if status := InferStatusFromResource("Pod", running, "UPDATE"); status != resourceStatusReady {
		t.Fatalf("expected Ready for running pod, got %s", status)
	}

	failed := []byte(`{"status":{"phase":"Failed"}}`)
	if status := InferStatusFromResource("Pod", failed, "UPDATE"); status != resourceStatusError {
		t.Fatalf("expected Error for failed pod, got %s", status)
	}
}

func TestInferStatusFromResource_CustomConditions(t *testing.T) {
	data := []byte(`
{
  "status": {
    "conditions": [
      {
        "type": "Ready",
        "status": "False",
        "reason": "ReconciliationFailed",
        "message": "apply failed"
      }
    ]
  }
}`)

	status := InferStatusFromResource("Kustomization", data, "UPDATE")
	if status != resourceStatusError {
		t.Fatalf("expected Error from Ready condition failure, got %s", status)
	}

	success := []byte(`
{
  "status": {
    "conditions": [
      {
        "type": "Ready",
        "status": "True"
      }
    ]
  }
}`)

	if status := InferStatusFromResource("ExternalSecret", success, "UPDATE"); status != resourceStatusReady {
		t.Fatalf("expected Ready from Ready=True condition, got %s", status)
	}
}

func TestInferStatusFromResource_DeletionSignalsTerminating(t *testing.T) {
	data := []byte(`
{
  "metadata": {
    "deletionTimestamp": "2025-01-01T00:00:00Z"
  }
}`)

	status := InferStatusFromResource("Deployment", data, "UPDATE")
	if status != resourceStatusTerminating {
		t.Fatalf("expected Terminating when deletionTimestamp is set, got %s", status)
	}
}

func TestInferStatusFromResource_DeleteEventFallback(t *testing.T) {
	if status := InferStatusFromResource("Deployment", nil, "DELETE"); status != resourceStatusTerminating {
		t.Fatalf("expected Terminating for delete event, got %s", status)
	}
}
