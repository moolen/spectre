package storage

import (
	"encoding/json"
	"testing"
)

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

func TestInferStatusFromResource_DaemonSet(t *testing.T) {
	tests := []struct {
		name                   string
		metadataName           string
		desiredNumberScheduled int
		numberReady            int
		numberUnavailable      int
		numberMisscheduled     int
		currentNumberScheduled *int
		numberAvailable        *int
		updatedNumberScheduled *int
		hasStatus              bool
		expected               string
	}{
		{
			name:                   "Ready - healthy DaemonSet",
			metadataName:           "kube-prometheus-stack-prometheus-node-exporter",
			desiredNumberScheduled: 3,
			numberReady:            3,
			numberUnavailable:      0,
			numberMisscheduled:     0,
			currentNumberScheduled: intPtr(3),
			numberAvailable:        intPtr(3),
			updatedNumberScheduled: intPtr(3),
			hasStatus:              true,
			expected:               resourceStatusReady,
		},
		{
			name:                   "Warning - unavailable pods",
			metadataName:           "test-daemonset",
			desiredNumberScheduled: 3,
			numberReady:            2,
			numberUnavailable:      1,
			numberMisscheduled:     0,
			hasStatus:              true,
			expected:               resourceStatusWarning,
		},
		{
			name:                   "Warning - misscheduled pods",
			metadataName:           "test-daemonset",
			desiredNumberScheduled: 3,
			numberReady:            3,
			numberUnavailable:      0,
			numberMisscheduled:     1,
			hasStatus:              true,
			expected:               resourceStatusWarning,
		},
		{
			name:         "Ready (fallback) - no status",
			metadataName: "test-daemonset",
			hasStatus:    false,
			expected:     resourceStatusReady,
		},
		{
			name:                   "Ready (fallback) - ready less than desired but no unavailable/misscheduled",
			metadataName:           "test-daemonset",
			desiredNumberScheduled: 3,
			numberReady:            2,
			numberUnavailable:      0,
			numberMisscheduled:     0,
			hasStatus:              true,
			expected:               resourceStatusReady,
		},
		{
			name:                   "Warning - both unavailable and misscheduled",
			metadataName:           "test-daemonset",
			desiredNumberScheduled: 3,
			numberReady:            2,
			numberUnavailable:      1,
			numberMisscheduled:     1,
			hasStatus:              true,
			expected:               resourceStatusWarning,
		},
		{
			name:                   "Ready (fallback) - zero desired pods",
			metadataName:           "test-daemonset",
			desiredNumberScheduled: 0,
			numberReady:            0,
			numberUnavailable:      0,
			numberMisscheduled:     0,
			hasStatus:              true,
			expected:               resourceStatusReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": tt.metadataName,
				},
			}

			if tt.hasStatus {
				status := map[string]interface{}{
					"desiredNumberScheduled": tt.desiredNumberScheduled,
					"numberReady":            tt.numberReady,
					"numberUnavailable":      tt.numberUnavailable,
					"numberMisscheduled":     tt.numberMisscheduled,
				}
				if tt.currentNumberScheduled != nil {
					status["currentNumberScheduled"] = *tt.currentNumberScheduled
				}
				if tt.numberAvailable != nil {
					status["numberAvailable"] = *tt.numberAvailable
				}
				if tt.updatedNumberScheduled != nil {
					status["updatedNumberScheduled"] = *tt.updatedNumberScheduled
				}
				resource["status"] = status
			}

			data, err := json.Marshal(resource)
			if err != nil {
				t.Fatalf("failed to marshal test data: %v", err)
			}

			status := InferStatusFromResource("DaemonSet", data, "UPDATE")
			if status != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, status)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
