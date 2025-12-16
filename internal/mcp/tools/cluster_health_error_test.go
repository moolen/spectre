package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/mcp/client"
)

func TestClusterHealth_ErrorMessageExtraction(t *testing.T) {
	// Create a mock timeline response with a Pod in CrashLoopBackOff
	podJSON := json.RawMessage(`{
		"metadata": {"name": "test-pod", "namespace": "default"},
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
	}`)

	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID:        "pod-1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test-pod",
				StatusSegments: []client.StatusSegment{
					{
						StartTime:    1000,
						EndTime:      2000,
						Status:       "Error",
						Message:      "Resource updated", // Old generic message
						ResourceData: podJSON,
					},
				},
			},
		},
	}

	output := analyzeHealth(response, 100)

	// Verify overall status is Critical (has errors)
	if output.OverallStatus != overallStatusCritical {
		t.Errorf("Expected overall status to be Critical, got %s", output.OverallStatus)
	}

	// Verify error count
	if output.ErrorResourceCount != 1 {
		t.Errorf("Expected 1 error resource, got %d", output.ErrorResourceCount)
	}

	// Verify top issues contains the pod with proper error message
	if len(output.TopIssues) == 0 {
		t.Fatal("Expected at least one issue in TopIssues")
	}

	issue := output.TopIssues[0]
	if issue.Name != "test-pod" {
		t.Errorf("Expected issue name to be 'test-pod', got %s", issue.Name)
	}

	// Verify error message contains CrashLoopBackOff details
	if !strings.Contains(issue.ErrorMessage, "CrashLoopBackOff") {
		t.Errorf("Expected error message to contain 'CrashLoopBackOff', got: %s", issue.ErrorMessage)
	}
	if !strings.Contains(issue.ErrorMessage, "app") {
		t.Errorf("Expected error message to contain container name 'app', got: %s", issue.ErrorMessage)
	}
	if !strings.Contains(issue.ErrorMessage, "15") {
		t.Errorf("Expected error message to contain restart count '15', got: %s", issue.ErrorMessage)
	}

	t.Logf("Successfully extracted error message: %s", issue.ErrorMessage)
}

func TestClusterHealth_MultipleErrors(t *testing.T) {
	// Create a deployment with insufficient replicas
	deploymentJSON := json.RawMessage(`{
		"metadata": {"name": "test-deployment", "namespace": "default"},
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
	}`)

	// Create a node with NotReady condition
	nodeJSON := json.RawMessage(`{
		"metadata": {"name": "node-1"},
		"status": {
			"conditions": [
				{
					"type": "Ready",
					"status": "False",
					"reason": "KubeletNotReady",
					"message": "container runtime network not ready"
				}
			]
		}
	}`)

	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID:        "deployment-1",
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "test-deployment",
				StatusSegments: []client.StatusSegment{
					{
						StartTime:    1000,
						EndTime:      2000,
						Status:       "Error",
						Message:      "Resource updated",
						ResourceData: deploymentJSON,
					},
				},
			},
			{
				ID:        "node-1",
				Kind:      "Node",
				Namespace: "",
				Name:      "node-1",
				StatusSegments: []client.StatusSegment{
					{
						StartTime:    1000,
						EndTime:      2000,
						Status:       "Error",
						Message:      "Resource updated",
						ResourceData: nodeJSON,
					},
				},
			},
		},
	}

	output := analyzeHealth(response, 100)

	// Verify we have 2 error resources
	if output.ErrorResourceCount != 2 {
		t.Errorf("Expected 2 error resources, got %d", output.ErrorResourceCount)
	}

	// Verify both issues have detailed error messages
	if len(output.TopIssues) < 2 {
		t.Fatalf("Expected at least 2 issues, got %d", len(output.TopIssues))
	}

	// Check deployment error
	var deploymentIssue *Issue
	var nodeIssue *Issue
	for i := range output.TopIssues {
		if output.TopIssues[i].Kind == "Deployment" {
			deploymentIssue = &output.TopIssues[i]
		}
		if output.TopIssues[i].Kind == "Node" {
			nodeIssue = &output.TopIssues[i]
		}
	}

	if deploymentIssue == nil {
		t.Fatal("Expected to find Deployment issue")
	}
	if !strings.Contains(deploymentIssue.ErrorMessage, "Insufficient replicas") {
		t.Errorf("Expected deployment error to contain 'Insufficient replicas', got: %s", deploymentIssue.ErrorMessage)
	}
	if !strings.Contains(deploymentIssue.ErrorMessage, "MinimumReplicasUnavailable") {
		t.Errorf("Expected deployment error to contain 'MinimumReplicasUnavailable', got: %s", deploymentIssue.ErrorMessage)
	}

	if nodeIssue == nil {
		t.Fatal("Expected to find Node issue")
	}
	if !strings.Contains(nodeIssue.ErrorMessage, "NotReady") {
		t.Errorf("Expected node error to contain 'NotReady', got: %s", nodeIssue.ErrorMessage)
	}
	if !strings.Contains(nodeIssue.ErrorMessage, "KubeletNotReady") {
		t.Errorf("Expected node error to contain 'KubeletNotReady', got: %s", nodeIssue.ErrorMessage)
	}

	t.Logf("Deployment error: %s", deploymentIssue.ErrorMessage)
	t.Logf("Node error: %s", nodeIssue.ErrorMessage)
}

func TestClusterHealth_FallbackToSegmentMessage(t *testing.T) {
	// Create a resource with empty ResourceData - should fallback to segment message
	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID:        "pod-1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test-pod",
				StatusSegments: []client.StatusSegment{
					{
						StartTime:    1000,
						EndTime:      2000,
						Status:       "Error",
						Message:      "Fallback message",
						ResourceData: nil,
					},
				},
			},
		},
	}

	output := analyzeHealth(response, 100)

	if len(output.TopIssues) == 0 {
		t.Fatal("Expected at least one issue")
	}

	issue := output.TopIssues[0]
	if issue.ErrorMessage != "Fallback message" {
		t.Errorf("Expected fallback to segment message, got: %s", issue.ErrorMessage)
	}
}
