package analyzer

import (
	"testing"
)

func TestInspectContainerStates_CrashLoopBackOff(t *testing.T) {
	data := []byte(`{
		"status": {
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

	obj, err := newResourceData(data)
	if err != nil {
		t.Fatalf("Failed to parse data: %v", err)
	}

	issues := InspectContainerStates(obj)

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issues))
	}

	issue := issues[0]
	if issue.IssueType != "CrashLoopBackOff" {
		t.Errorf("Expected CrashLoopBackOff, got %s", issue.IssueType)
	}
	if issue.ContainerName != "app" {
		t.Errorf("Expected container name 'app', got %s", issue.ContainerName)
	}
	if issue.RestartCount != 5 {
		t.Errorf("Expected restart count 5, got %d", issue.RestartCount)
	}
	if issue.ImpactScore != 0.35 {
		t.Errorf("Expected impact score 0.35, got %f", issue.ImpactScore)
	}
}

func TestInspectContainerStates_ImagePullBackOff(t *testing.T) {
	data := []byte(`{
		"status": {
			"containerStatuses": [{
				"name": "nginx",
				"restartCount": 0,
				"state": {
					"waiting": {
						"reason": "ImagePullBackOff",
						"message": "Back-off pulling image \"invalid-image:latest\""
					}
				}
			}]
		}
	}`)

	obj, err := newResourceData(data)
	if err != nil {
		t.Fatalf("Failed to parse data: %v", err)
	}

	issues := InspectContainerStates(obj)

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issues))
	}

	issue := issues[0]
	if issue.IssueType != "ImagePullBackOff" {
		t.Errorf("Expected ImagePullBackOff, got %s", issue.IssueType)
	}
	if issue.ImpactScore != 0.25 {
		t.Errorf("Expected impact score 0.25, got %f", issue.ImpactScore)
	}
}

func TestInspectContainerStates_ErrImagePull(t *testing.T) {
	data := []byte(`{
		"status": {
			"containerStatuses": [{
				"name": "app",
				"restartCount": 0,
				"state": {
					"waiting": {
						"reason": "ErrImagePull",
						"message": "Failed to pull image"
					}
				}
			}]
		}
	}`)

	obj, err := newResourceData(data)
	if err != nil {
		t.Fatalf("Failed to parse data: %v", err)
	}

	issues := InspectContainerStates(obj)

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issues))
	}

	if issues[0].IssueType != "ImagePullBackOff" {
		t.Errorf("Expected ImagePullBackOff for ErrImagePull, got %s", issues[0].IssueType)
	}
}

func TestInspectContainerStates_OOMKilled(t *testing.T) {
	data := []byte(`{
		"status": {
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

	obj, err := newResourceData(data)
	if err != nil {
		t.Fatalf("Failed to parse data: %v", err)
	}

	issues := InspectContainerStates(obj)

	// Should detect both OOMKilled and CrashLoopBackOff
	if len(issues) != 2 {
		t.Fatalf("Expected 2 issues, got %d", len(issues))
	}

	// Find OOMKilled issue
	var oomIssue *ContainerIssue
	for i := range issues {
		if issues[i].IssueType == "OOMKilled" {
			oomIssue = &issues[i]
			break
		}
	}

	if oomIssue == nil {
		t.Fatal("Expected to find OOMKilled issue")
	}

	if oomIssue.ImpactScore != 0.40 {
		t.Errorf("Expected impact score 0.40, got %f", oomIssue.ImpactScore)
	}
	if oomIssue.ExitCode != 137 {
		t.Errorf("Expected exit code 137, got %d", oomIssue.ExitCode)
	}
}

func TestInspectContainerStates_OOMKilledByExitCodeOnly(t *testing.T) {
	data := []byte(`{
		"status": {
			"containerStatuses": [{
				"name": "app",
				"restartCount": 1,
				"lastState": {
					"terminated": {
						"reason": "Error",
						"exitCode": 137,
						"message": "Container terminated"
					}
				}
			}]
		}
	}`)

	obj, err := newResourceData(data)
	if err != nil {
		t.Fatalf("Failed to parse data: %v", err)
	}

	issues := InspectContainerStates(obj)

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issues))
	}

	if issues[0].IssueType != "OOMKilled" {
		t.Errorf("Expected OOMKilled (detected by exit code), got %s", issues[0].IssueType)
	}
}

func TestInspectContainerStates_HighRestartCount(t *testing.T) {
	data := []byte(`{
		"status": {
			"containerStatuses": [{
				"name": "flaky-app",
				"restartCount": 8,
				"state": {
					"running": {
						"startedAt": "2024-01-01T12:00:00Z"
					}
				}
			}]
		}
	}`)

	obj, err := newResourceData(data)
	if err != nil {
		t.Fatalf("Failed to parse data: %v", err)
	}

	issues := InspectContainerStates(obj)

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issues))
	}

	issue := issues[0]
	if issue.IssueType != "HighRestartCount" {
		t.Errorf("Expected HighRestartCount, got %s", issue.IssueType)
	}
	if issue.ImpactScore != 0.20 {
		t.Errorf("Expected impact score 0.20 for 8 restarts, got %f", issue.ImpactScore)
	}
}

func TestInspectContainerStates_VeryHighRestartCount(t *testing.T) {
	data := []byte(`{
		"status": {
			"containerStatuses": [{
				"name": "unstable-app",
				"restartCount": 15,
				"state": {
					"running": {}
				}
			}]
		}
	}`)

	obj, err := newResourceData(data)
	if err != nil {
		t.Fatalf("Failed to parse data: %v", err)
	}

	issues := InspectContainerStates(obj)

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issues))
	}

	issue := issues[0]
	if issue.IssueType != "VeryHighRestartCount" {
		t.Errorf("Expected VeryHighRestartCount, got %s", issue.IssueType)
	}
	if issue.ImpactScore != 0.35 {
		t.Errorf("Expected impact score 0.35 for 15 restarts, got %f", issue.ImpactScore)
	}
}

func TestInspectContainerStates_NoRestartCountWithCrashLoop(t *testing.T) {
	// When CrashLoopBackOff is present, don't also report HighRestartCount
	data := []byte(`{
		"status": {
			"containerStatuses": [{
				"name": "app",
				"restartCount": 12,
				"state": {
					"waiting": {
						"reason": "CrashLoopBackOff"
					}
				}
			}]
		}
	}`)

	obj, err := newResourceData(data)
	if err != nil {
		t.Fatalf("Failed to parse data: %v", err)
	}

	issues := InspectContainerStates(obj)

	// Should only have CrashLoopBackOff, not also HighRestartCount
	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue (CrashLoopBackOff only), got %d", len(issues))
	}

	if issues[0].IssueType != "CrashLoopBackOff" {
		t.Errorf("Expected CrashLoopBackOff, got %s", issues[0].IssueType)
	}
}

func TestInspectContainerStates_InitContainers(t *testing.T) {
	data := []byte(`{
		"status": {
			"initContainerStatuses": [{
				"name": "init-db",
				"restartCount": 2,
				"state": {
					"waiting": {
						"reason": "CrashLoopBackOff",
						"message": "Init container failed"
					}
				}
			}]
		}
	}`)

	obj, err := newResourceData(data)
	if err != nil {
		t.Fatalf("Failed to parse data: %v", err)
	}

	issues := InspectContainerStates(obj)

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue from init container, got %d", len(issues))
	}

	if issues[0].ContainerName != "init-db" {
		t.Errorf("Expected container name 'init-db', got %s", issues[0].ContainerName)
	}
}

func TestInspectContainerStates_MultipleContainers(t *testing.T) {
	data := []byte(`{
		"status": {
			"containerStatuses": [
				{
					"name": "app",
					"restartCount": 0,
					"state": {"running": {}}
				},
				{
					"name": "sidecar",
					"restartCount": 3,
					"state": {
						"waiting": {
							"reason": "CrashLoopBackOff"
						}
					}
				}
			]
		}
	}`)

	obj, err := newResourceData(data)
	if err != nil {
		t.Fatalf("Failed to parse data: %v", err)
	}

	issues := InspectContainerStates(obj)

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue (from sidecar), got %d", len(issues))
	}

	if issues[0].ContainerName != "sidecar" {
		t.Errorf("Expected issue from 'sidecar' container, got %s", issues[0].ContainerName)
	}
}

func TestInspectContainerStates_NoIssues(t *testing.T) {
	data := []byte(`{
		"status": {
			"containerStatuses": [{
				"name": "healthy-app",
				"restartCount": 0,
				"state": {
					"running": {
						"startedAt": "2024-01-01T12:00:00Z"
					}
				}
			}]
		}
	}`)

	obj, err := newResourceData(data)
	if err != nil {
		t.Fatalf("Failed to parse data: %v", err)
	}

	issues := InspectContainerStates(obj)

	if len(issues) != 0 {
		t.Errorf("Expected no issues for healthy container, got %d", len(issues))
	}
}

func TestHasCriticalContainerIssues(t *testing.T) {
	tests := []struct {
		name     string
		issues   []ContainerIssue
		expected bool
	}{
		{
			name: "OOMKilled is critical",
			issues: []ContainerIssue{
				{IssueType: "OOMKilled"},
			},
			expected: true,
		},
		{
			name: "CrashLoopBackOff is critical",
			issues: []ContainerIssue{
				{IssueType: "CrashLoopBackOff"},
			},
			expected: true,
		},
		{
			name: "ImagePullBackOff is not critical",
			issues: []ContainerIssue{
				{IssueType: "ImagePullBackOff"},
			},
			expected: false,
		},
		{
			name: "HighRestartCount is not critical",
			issues: []ContainerIssue{
				{IssueType: "HighRestartCount"},
			},
			expected: false,
		},
		{
			name:     "No issues",
			issues:   []ContainerIssue{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasCriticalContainerIssues(tt.issues)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetHighestImpactScore(t *testing.T) {
	issues := []ContainerIssue{
		{IssueType: "ImagePullBackOff", ImpactScore: 0.25},
		{IssueType: "OOMKilled", ImpactScore: 0.40},
		{IssueType: "HighRestartCount", ImpactScore: 0.20},
	}

	score := GetHighestImpactScore(issues)
	if score != 0.40 {
		t.Errorf("Expected highest score 0.40, got %f", score)
	}
}

func TestGetHighestImpactScore_NoIssues(t *testing.T) {
	issues := []ContainerIssue{}
	score := GetHighestImpactScore(issues)
	if score != 0.0 {
		t.Errorf("Expected 0.0 for no issues, got %f", score)
	}
}

func TestInferPodStatus_WithContainerIssues(t *testing.T) {
	// Test that pod status reflects container issues
	data := []byte(`{
		"status": {
			"phase": "Running",
			"conditions": [{"type": "Ready", "status": "False"}],
			"containerStatuses": [{
				"name": "app",
				"restartCount": 5,
				"state": {
					"waiting": {
						"reason": "CrashLoopBackOff"
					}
				}
			}]
		}
	}`)

	status := InferStatusFromResource("Pod", data, "UPDATE")
	if status != resourceStatusError {
		t.Errorf("Expected Error status for pod with CrashLoopBackOff, got %s", status)
	}
}

func TestInferPodStatus_WithOOMKilled(t *testing.T) {
	data := []byte(`{
		"status": {
			"phase": "Running",
			"containerStatuses": [{
				"name": "app",
				"restartCount": 2,
				"lastState": {
					"terminated": {
						"reason": "OOMKilled",
						"exitCode": 137
					}
				}
			}]
		}
	}`)

	status := InferStatusFromResource("Pod", data, "UPDATE")
	if status != resourceStatusError {
		t.Errorf("Expected Error status for pod with OOMKilled, got %s", status)
	}
}

func TestInferPodStatus_WithImagePullBackOff(t *testing.T) {
	data := []byte(`{
		"status": {
			"phase": "Pending",
			"containerStatuses": [{
				"name": "app",
				"restartCount": 0,
				"state": {
					"waiting": {
						"reason": "ImagePullBackOff"
					}
				}
			}]
		}
	}`)

	status := InferStatusFromResource("Pod", data, "UPDATE")
	if status != resourceStatusError {
		t.Errorf("Expected Error status for pod with ImagePullBackOff, got %s", status)
	}
}
