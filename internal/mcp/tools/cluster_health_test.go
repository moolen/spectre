package tools

import (
	"fmt"
	"testing"

	"github.com/moolen/spectre/internal/mcp/client"
)

const kindPod = "Pod"

func TestAnalyzeHealth_AllHealthyCluster(t *testing.T) {
	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID:        "pod-1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "app-1",
				StatusSegments: []client.StatusSegment{
					{Status: "Ready", Message: "Pod is running"},
				},
			},
			{
				ID:        "pod-2",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "app-2",
				StatusSegments: []client.StatusSegment{
					{Status: "Ready", Message: "Pod is running"},
				},
			},
			{
				ID:        "deploy-1",
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "web",
				StatusSegments: []client.StatusSegment{
					{Status: "Ready", Message: "All replicas ready"},
				},
			},
		},
	}

	output := analyzeHealth(response, 100)

	if output.OverallStatus != "Healthy" {
		t.Errorf("Expected overall status Healthy, got %s", output.OverallStatus)
	}

	if output.TotalResources != 3 {
		t.Errorf("Expected 3 total resources, got %d", output.TotalResources)
	}

	if output.HealthyResourceCount != 3 {
		t.Errorf("Expected 3 healthy resources, got %d", output.HealthyResourceCount)
	}

	if output.ErrorResourceCount != 0 {
		t.Errorf("Expected 0 error resources, got %d", output.ErrorResourceCount)
	}

	if output.WarningResourceCount != 0 {
		t.Errorf("Expected 0 warning resources, got %d", output.WarningResourceCount)
	}
}

func TestAnalyzeHealth_CriticalCluster(t *testing.T) {
	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID:        "pod-1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "app-1",
				StatusSegments: []client.StatusSegment{
					{Status: "Error", Message: "CrashLoopBackOff"},
				},
			},
			{
				ID:        "pod-2",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "app-2",
				StatusSegments: []client.StatusSegment{
					{Status: "Error", Message: "ImagePullBackOff"},
				},
			},
		},
	}

	output := analyzeHealth(response, 100)

	if output.OverallStatus != "Critical" {
		t.Errorf("Expected overall status Critical, got %s", output.OverallStatus)
	}

	if output.ErrorResourceCount != 2 {
		t.Errorf("Expected 2 error resources, got %d", output.ErrorResourceCount)
	}

	if output.HealthyResourceCount != 0 {
		t.Errorf("Expected 0 healthy resources, got %d", output.HealthyResourceCount)
	}
}

func TestAnalyzeHealth_DegradedCluster(t *testing.T) {
	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID:        "pod-1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "app-1",
				StatusSegments: []client.StatusSegment{
					{Status: "Ready", Message: "Pod is running"},
				},
			},
			{
				ID:        "pod-2",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "app-2",
				StatusSegments: []client.StatusSegment{
					{Status: "Warning", Message: "Pending"},
				},
			},
		},
	}

	output := analyzeHealth(response, 100)

	if output.OverallStatus != "Degraded" {
		t.Errorf("Expected overall status Degraded, got %s", output.OverallStatus)
	}

	if output.WarningResourceCount != 1 {
		t.Errorf("Expected 1 warning resource, got %d", output.WarningResourceCount)
	}

	if output.HealthyResourceCount != 1 {
		t.Errorf("Expected 1 healthy resource, got %d", output.HealthyResourceCount)
	}
}

func TestAnalyzeHealth_MixedHealthCluster(t *testing.T) {
	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID:        "pod-1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "healthy",
				StatusSegments: []client.StatusSegment{
					{Status: "Ready"},
				},
			},
			{
				ID:        "pod-2",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "warning",
				StatusSegments: []client.StatusSegment{
					{Status: "Warning"},
				},
			},
			{
				ID:        "pod-3",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "error",
				StatusSegments: []client.StatusSegment{
					{Status: "Error"},
				},
			},
			{
				ID:        "deploy-1",
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "app",
				StatusSegments: []client.StatusSegment{
					{Status: "Ready"},
				},
			},
		},
	}

	output := analyzeHealth(response, 100)

	// Critical because there's at least one Error
	if output.OverallStatus != "Critical" {
		t.Errorf("Expected overall status Critical, got %s", output.OverallStatus)
	}

	if output.TotalResources != 4 {
		t.Errorf("Expected 4 total resources, got %d", output.TotalResources)
	}

	if output.HealthyResourceCount != 2 {
		t.Errorf("Expected 2 healthy resources, got %d", output.HealthyResourceCount)
	}

	if output.WarningResourceCount != 1 {
		t.Errorf("Expected 1 warning resource, got %d", output.WarningResourceCount)
	}

	if output.ErrorResourceCount != 1 {
		t.Errorf("Expected 1 error resource, got %d", output.ErrorResourceCount)
	}
}

func TestAnalyzeHealth_EmptyCluster(t *testing.T) {
	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{},
	}

	output := analyzeHealth(response, 100)

	if output.OverallStatus != "Healthy" {
		t.Errorf("Expected overall status Healthy for empty cluster, got %s", output.OverallStatus)
	}

	if output.TotalResources != 0 {
		t.Errorf("Expected 0 total resources, got %d", output.TotalResources)
	}
}

func TestAnalyzeHealth_ResourceCountsByKind(t *testing.T) {
	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID:   "pod-1",
				Kind: "Pod",
				StatusSegments: []client.StatusSegment{
					{Status: "Ready"},
				},
			},
			{
				ID:   "pod-2",
				Kind: "Pod",
				StatusSegments: []client.StatusSegment{
					{Status: "Error"},
				},
			},
			{
				ID:   "deploy-1",
				Kind: "Deployment",
				StatusSegments: []client.StatusSegment{
					{Status: "Ready"},
				},
			},
		},
	}

	output := analyzeHealth(response, 100)

	if len(output.ResourcesByKind) != 2 {
		t.Fatalf("Expected 2 resource kinds, got %d", len(output.ResourcesByKind))
	}

	// Find Pod kind
	var podCount *ResourceStatusCount
	for i := range output.ResourcesByKind {
		if output.ResourcesByKind[i].Kind == kindPod {
			podCount = &output.ResourcesByKind[i]
			break
		}
	}

	if podCount == nil {
		t.Fatal("Pod kind not found in output")
	}

	if podCount.TotalCount != 2 {
		t.Errorf("Expected 2 total Pods, got %d", podCount.TotalCount)
	}

	if podCount.Ready != 1 {
		t.Errorf("Expected 1 ready Pod, got %d", podCount.Ready)
	}

	if podCount.Error != 1 {
		t.Errorf("Expected 1 error Pod, got %d", podCount.Error)
	}
}

func TestAnalyzeHealth_ErrorRateCalculation(t *testing.T) {
	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{Kind: "Pod", StatusSegments: []client.StatusSegment{{Status: "Ready"}}},
			{Kind: "Pod", StatusSegments: []client.StatusSegment{{Status: "Ready"}}},
			{Kind: "Pod", StatusSegments: []client.StatusSegment{{Status: "Error"}}},
			{Kind: "Pod", StatusSegments: []client.StatusSegment{{Status: "Error"}}},
		},
	}

	output := analyzeHealth(response, 100)

	var podCount *ResourceStatusCount
	for i := range output.ResourcesByKind {
		if output.ResourcesByKind[i].Kind == kindPod {
			podCount = &output.ResourcesByKind[i]
			break
		}
	}

	if podCount == nil {
		t.Fatal("Pod kind not found")
	}

	// Error rate = 2 errors / 4 total = 0.5
	expectedErrorRate := 0.5
	if podCount.ErrorRate != expectedErrorRate {
		t.Errorf("Expected error rate %f, got %f", expectedErrorRate, podCount.ErrorRate)
	}
}

func TestAnalyzeHealth_TopIssuesSorting(t *testing.T) {
	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID:   "pod-1",
				Kind: "Pod",
				Name: "short-error",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Error",
						Message:   "Error 1",
						StartTime: 1000,
						EndTime:   1100, // 100s duration
					},
				},
			},
			{
				ID:   "pod-2",
				Kind: "Pod",
				Name: "long-error",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Error",
						Message:   "Error 2",
						StartTime: 1000,
						EndTime:   1500, // 500s duration (longest)
					},
				},
			},
			{
				ID:   "pod-3",
				Kind: "Pod",
				Name: "medium-error",
				StatusSegments: []client.StatusSegment{
					{
						Status:    "Error",
						Message:   "Error 3",
						StartTime: 1000,
						EndTime:   1300, // 300s duration
					},
				},
			},
		},
	}

	output := analyzeHealth(response, 100)

	if len(output.TopIssues) != 3 {
		t.Fatalf("Expected 3 top issues, got %d", len(output.TopIssues))
	}

	// Should be sorted by duration descending
	if output.TopIssues[0].Name != "long-error" {
		t.Errorf("Expected first issue to be 'long-error', got %s", output.TopIssues[0].Name)
	}

	if output.TopIssues[1].Name != "medium-error" {
		t.Errorf("Expected second issue to be 'medium-error', got %s", output.TopIssues[1].Name)
	}

	if output.TopIssues[2].Name != "short-error" {
		t.Errorf("Expected third issue to be 'short-error', got %s", output.TopIssues[2].Name)
	}

	// Check error durations
	if output.TopIssues[0].ErrorDuration != 500 {
		t.Errorf("Expected error duration 500s, got %d", output.TopIssues[0].ErrorDuration)
	}
}

func TestAnalyzeHealth_TerminatingResources(t *testing.T) {
	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID:   "pod-1",
				Kind: "Pod",
				Name: "terminating-pod",
				StatusSegments: []client.StatusSegment{
					{Status: "Terminating", Message: "Pod is being deleted"},
				},
			},
			{
				ID:   "pod-2",
				Kind: "Pod",
				Name: "healthy-pod",
				StatusSegments: []client.StatusSegment{
					{Status: "Ready"},
				},
			},
		},
	}

	output := analyzeHealth(response, 100)

	var podCount *ResourceStatusCount
	for i := range output.ResourcesByKind {
		if output.ResourcesByKind[i].Kind == kindPod {
			podCount = &output.ResourcesByKind[i]
			break
		}
	}

	if podCount == nil {
		t.Fatal("Pod kind not found")
	}

	if podCount.Terminating != 1 {
		t.Errorf("Expected 1 terminating Pod, got %d", podCount.Terminating)
	}

	if podCount.Ready != 1 {
		t.Errorf("Expected 1 ready Pod, got %d", podCount.Ready)
	}
}

func TestAnalyzeHealth_UnknownStatus(t *testing.T) {
	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID:   "pod-1",
				Kind: "Pod",
				StatusSegments: []client.StatusSegment{
					{Status: "Unknown", Message: "Status cannot be determined"},
				},
			},
		},
	}

	output := analyzeHealth(response, 100)

	var podCount *ResourceStatusCount
	for i := range output.ResourcesByKind {
		if output.ResourcesByKind[i].Kind == kindPod {
			podCount = &output.ResourcesByKind[i]
			break
		}
	}

	if podCount == nil {
		t.Fatal("Pod kind not found")
	}

	if podCount.Unknown != 1 {
		t.Errorf("Expected 1 unknown Pod, got %d", podCount.Unknown)
	}
}

func TestAnalyzeHealth_MaxResourcesLimit(t *testing.T) {
	// Create 10 error resources
	resources := make([]client.TimelineResource, 10)
	for i := 0; i < 10; i++ {
		resources[i] = client.TimelineResource{
			ID:   fmt.Sprintf("pod-%d", i),
			Kind: "Pod",
			Name: fmt.Sprintf("error-pod-%d", i),
			StatusSegments: []client.StatusSegment{
				{Status: "Error", Message: "Test error"},
			},
		}
	}

	response := &client.TimelineResponse{
		Resources: resources,
	}

	// Limit to 5 resources
	output := analyzeHealth(response, 5)

	var podCount *ResourceStatusCount
	for i := range output.ResourcesByKind {
		if output.ResourcesByKind[i].Kind == kindPod {
			podCount = &output.ResourcesByKind[i]
			break
		}
	}

	if podCount == nil {
		t.Fatal("Pod kind not found")
	}

	// Should still count all 10 errors
	if podCount.Error != 10 {
		t.Errorf("Expected 10 error Pods counted, got %d", podCount.Error)
	}

	// But only list 5 resources + 1 "(+N more)" entry = 6 items total
	if len(podCount.ErrorResources) != 6 {
		t.Errorf("Expected 6 items (5 resources + 1 summary), got %d", len(podCount.ErrorResources))
	}

	// Last entry should indicate there are more
	if len(podCount.ErrorResources) > 0 {
		lastEntry := podCount.ErrorResources[len(podCount.ErrorResources)-1]
		if lastEntry != "(+5 more)" {
			t.Errorf("Expected last entry to be '(+5 more)', got '%s'", lastEntry)
		}
	}
}

func TestAnalyzeHealth_MultipleResourceKinds(t *testing.T) {
	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{Kind: "Pod", StatusSegments: []client.StatusSegment{{Status: "Ready"}}},
			{Kind: "Pod", StatusSegments: []client.StatusSegment{{Status: "Error"}}},
			{Kind: "Deployment", StatusSegments: []client.StatusSegment{{Status: "Ready"}}},
			{Kind: "Service", StatusSegments: []client.StatusSegment{{Status: "Ready"}}},
			{Kind: "Node", StatusSegments: []client.StatusSegment{{Status: "Warning"}}},
		},
	}

	output := analyzeHealth(response, 100)

	// Should have 4 different kinds
	expectedKinds := map[string]bool{
		"Pod":        true,
		"Deployment": true,
		"Service":    true,
		"Node":       true,
	}

	if len(output.ResourcesByKind) != 4 {
		t.Errorf("Expected 4 resource kinds, got %d", len(output.ResourcesByKind))
	}

	for _, kindCount := range output.ResourcesByKind {
		if !expectedKinds[kindCount.Kind] {
			t.Errorf("Unexpected kind: %s", kindCount.Kind)
		}
	}
}

func TestAnalyzeHealth_NoStatusSegments(t *testing.T) {
	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID:             "pod-1",
				Kind:           "Pod",
				Name:           "no-segments",
				StatusSegments: []client.StatusSegment{}, // Empty
			},
		},
	}

	output := analyzeHealth(response, 100)

	var podCount *ResourceStatusCount
	for i := range output.ResourcesByKind {
		if output.ResourcesByKind[i].Kind == kindPod {
			podCount = &output.ResourcesByKind[i]
			break
		}
	}

	if podCount == nil {
		t.Fatal("Pod kind not found")
	}

	// Should be counted as Unknown
	if podCount.Unknown != 1 {
		t.Errorf("Expected 1 unknown Pod (no status segments), got %d", podCount.Unknown)
	}
}

func TestAnalyzeHealth_EventCounting(t *testing.T) {
	response := &client.TimelineResponse{
		Resources: []client.TimelineResource{
			{
				ID:   "pod-1",
				Kind: "Pod",
				Name: "high-event-pod",
				StatusSegments: []client.StatusSegment{
					{Status: "Error", Message: "CrashLoopBackOff"},
				},
				Events: []client.K8sEvent{
					{Reason: "BackOff"},
					{Reason: "BackOff"},
					{Reason: "BackOff"},
					{Reason: "BackOff"},
					{Reason: "BackOff"},
				},
			},
		},
	}

	output := analyzeHealth(response, 100)

	if len(output.TopIssues) != 1 {
		t.Fatalf("Expected 1 top issue, got %d", len(output.TopIssues))
	}

	if output.TopIssues[0].EventCount != 5 {
		t.Errorf("Expected 5 events, got %d", output.TopIssues[0].EventCount)
	}
}
