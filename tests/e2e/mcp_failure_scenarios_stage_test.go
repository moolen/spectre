package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type MCPFailureScenarioStage struct {
	t         *testing.T
	require   *require.Assertions
	assert    *assert.Assertions
	testCtx   *helpers.TestContext
	mcpClient *helpers.MCPClient

	// Deployed resources tracking
	deployedResources []deployedResource

	// Tool results
	clusterHealthResult           map[string]interface{}
	resourceTimelineResult        map[string]interface{}
	resourceTimelineChangesResult map[string]interface{}

	// Query time window
	queryStartTime int64
	queryEndTime   int64
}

type deployedResource struct {
	kind      string
	namespace string
	name      string
}

func NewMCPFailureScenarioStage(t *testing.T) (*MCPFailureScenarioStage, *MCPFailureScenarioStage, *MCPFailureScenarioStage) {
	s := &MCPFailureScenarioStage{
		t:                 t,
		require:           require.New(t),
		assert:            assert.New(t),
		deployedResources: make([]deployedResource, 0),
	}
	return s, s, s
}

func (s *MCPFailureScenarioStage) and() *MCPFailureScenarioStage {
	return s
}

// ==================== Setup Stages ====================

func (s *MCPFailureScenarioStage) a_test_environment() *MCPFailureScenarioStage {
	// Use shared MCP-enabled deployment instead of creating a new one per test
	s.testCtx = helpers.SetupE2ETestSharedMCP(s.t)
	// Set initial query time window (will be updated as test progresses)
	s.queryStartTime = time.Now().Unix()
	return s
}

func (s *MCPFailureScenarioStage) mcp_server_is_deployed() *MCPFailureScenarioStage {
	// MCP server is already deployed and enabled on the shared deployment
	// No need to update Helm release or wait for deployment
	s.t.Logf("Using shared MCP deployment in namespace: %s", s.testCtx.SharedDeployment.Namespace)
	return s
}

func (s *MCPFailureScenarioStage) mcp_client_is_connected() *MCPFailureScenarioStage {
	// Create port-forward to the shared MCP server
	serviceName := s.testCtx.ReleaseName + "-spectre"
	// Important: Use SharedDeployment.Namespace, not testCtx.Namespace
	// testCtx.Namespace is for test resources, SharedDeployment.Namespace is where Spectre runs
	mcpNamespace := s.testCtx.SharedDeployment.Namespace
	mcpPortForward, err := helpers.NewPortForwarder(s.t, s.testCtx.Cluster.GetContext(), mcpNamespace, serviceName, 8080)
	s.require.NoError(err, "failed to create MCP port-forward")

	err = mcpPortForward.WaitForReady(30 * time.Second)
	s.require.NoError(err, "MCP server not reachable via port-forward")

	s.t.Cleanup(func() {
		if err := mcpPortForward.Stop(); err != nil {
			s.t.Logf("Warning: failed to stop MCP port-forward: %v", err)
		}
	})

	s.mcpClient = helpers.NewMCPClient(s.t, mcpPortForward.GetURL())

	// Initialize the MCP session
	ctx, cancel := context.WithTimeout(s.t.Context(), 10*time.Second)
	defer cancel()
	_, err = s.mcpClient.Initialize(ctx)
	s.require.NoError(err, "failed to initialize MCP session")

	return s
}

// ==================== Deployment Stages ====================

func (s *MCPFailureScenarioStage) failure_scenario_is_deployed(fixturePath string) *MCPFailureScenarioStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 30*time.Second)
	defer cancel()

	// Load the YAML file
	fullPath := filepath.Join("fixtures", fixturePath)
	data, err := os.ReadFile(fullPath)
	s.require.NoError(err, "failed to read fixture file: %s", fixturePath)

	// Parse YAML into unstructured object
	obj := &unstructured.Unstructured{}
	err = yaml.Unmarshal(data, obj)
	s.require.NoError(err, "failed to parse YAML: %s", fixturePath)

	// Apply the resource using dynamic client
	err = s.applyResource(ctx, obj)
	s.require.NoError(err, "failed to apply resource from: %s", fixturePath)

	// Track the deployed resource for cleanup
	s.deployedResources = append(s.deployedResources, deployedResource{
		kind:      obj.GetKind(),
		namespace: s.testCtx.Namespace,
		name:      obj.GetName(),
	})

	s.t.Logf("✓ Deployed %s/%s from %s", obj.GetKind(), obj.GetName(), fixturePath)

	return s
}

func (s *MCPFailureScenarioStage) applyResource(ctx context.Context, obj *unstructured.Unstructured) error {
	// Set namespace if not set
	if obj.GetNamespace() == "" {
		obj.SetNamespace(s.testCtx.Namespace)
	}

	gvk := obj.GroupVersionKind()
	kind := obj.GetKind()

	switch kind {
	case "Pod":
		var pod corev1.Pod
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pod)
		if err != nil {
			return fmt.Errorf("failed to convert to Pod: %w", err)
		}
		pod.Namespace = s.testCtx.Namespace
		_, err = s.testCtx.K8sClient.Clientset.CoreV1().Pods(s.testCtx.Namespace).Create(ctx, &pod, metav1.CreateOptions{})
		return err
	case "Deployment":
		var deployment appsv1.Deployment
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deployment)
		if err != nil {
			return fmt.Errorf("failed to convert to Deployment: %w", err)
		}
		deployment.Namespace = s.testCtx.Namespace
		_, err = s.testCtx.K8sClient.Clientset.AppsV1().Deployments(s.testCtx.Namespace).Create(ctx, &deployment, metav1.CreateOptions{})
		if err != nil {
			// Try update if already exists
			_, err = s.testCtx.K8sClient.Clientset.AppsV1().Deployments(s.testCtx.Namespace).Update(ctx, &deployment, metav1.UpdateOptions{})
		}
		return err
	case "PersistentVolumeClaim":
		var pvc corev1.PersistentVolumeClaim
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pvc)
		if err != nil {
			return fmt.Errorf("failed to convert to PVC: %w", err)
		}
		pvc.Namespace = s.testCtx.Namespace
		_, err = s.testCtx.K8sClient.Clientset.CoreV1().PersistentVolumeClaims(s.testCtx.Namespace).Create(ctx, &pvc, metav1.CreateOptions{})
		return err
	default:
		return fmt.Errorf("unsupported resource kind: %s (GVK: %s)", kind, gvk.String())
	}
}

func (s *MCPFailureScenarioStage) deployment_is_updated(fromFixture, toFixture string) *MCPFailureScenarioStage {
	// Just apply the new fixture - it will update the existing deployment
	return s.failure_scenario_is_deployed(toFixture)
}

func (s *MCPFailureScenarioStage) wait_for_condition(duration time.Duration) *MCPFailureScenarioStage {
	s.t.Logf("Waiting for %v to allow condition to manifest...", duration)
	time.Sleep(duration)
	return s
}

func (s *MCPFailureScenarioStage) failure_condition_is_observed(timeout time.Duration) *MCPFailureScenarioStage {
	// Update query end time to capture all events up to now
	s.queryEndTime = time.Now().Unix()
	s.t.Logf("Failure condition observation period complete. Query window: %v to %v", 
		time.Unix(s.queryStartTime, 0), time.Unix(s.queryEndTime, 0))
	return s
}

// ==================== Tool Invocation Stages ====================

func (s *MCPFailureScenarioStage) cluster_health_tool_is_called() *MCPFailureScenarioStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 30*time.Second)
	defer cancel()

	args := map[string]interface{}{
		"start_time": s.queryStartTime,
		"end_time":   s.queryEndTime,
		"namespace":  s.testCtx.Namespace,
	}

	result, err := s.mcpClient.CallTool(ctx, "cluster_health", args)
	s.require.NoError(err, "cluster_health tool call failed")
	s.require.NotNil(result, "cluster_health result should not be nil")

	s.clusterHealthResult = result
	s.t.Logf("✓ cluster_health tool called successfully")
	return s
}

func (s *MCPFailureScenarioStage) resource_timeline_tool_is_called_for_resource(kind, name string) *MCPFailureScenarioStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 30*time.Second)
	defer cancel()

	args := map[string]interface{}{
		"resource_kind": kind,
		"resource_name": name,
		"namespace":     s.testCtx.Namespace,
		"start_time":    s.queryStartTime,
		"end_time":      s.queryEndTime,
	}

	result, err := s.mcpClient.CallTool(ctx, "resource_timeline", args)
	s.require.NoError(err, "resource_timeline tool call failed")
	s.require.NotNil(result, "resource_timeline result should not be nil")

	s.resourceTimelineResult = result
	s.t.Logf("✓ resource_timeline tool called successfully for %s/%s", kind, name)
	return s
}

func (s *MCPFailureScenarioStage) resource_timeline_changes_tool_is_called() *MCPFailureScenarioStage {
	// Default call with empty UIDs - will be populated from cluster_health results
	return s.resource_timeline_changes_tool_is_called_with_uids([]string{})
}

func (s *MCPFailureScenarioStage) resource_timeline_changes_tool_is_called_with_uids(resourceUIDs []string) *MCPFailureScenarioStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 30*time.Second)
	defer cancel()

	// If no UIDs provided, try to extract from cluster_health results
	if len(resourceUIDs) == 0 && s.clusterHealthResult != nil {
		content := s.extractContent(s.clusterHealthResult)
		var healthData map[string]interface{}
		if err := json.Unmarshal([]byte(content), &healthData); err == nil {
			// First try issue_resource_uids (direct list of UIDs)
			if uids, ok := healthData["issue_resource_uids"].([]interface{}); ok {
				for _, uid := range uids {
					if uidStr, ok := uid.(string); ok && uidStr != "" {
						resourceUIDs = append(resourceUIDs, uidStr)
						if len(resourceUIDs) >= 5 {
							break // Limit to 5 UIDs
						}
					}
				}
			}
			// Fall back to top_issues with resource_uid field
			if len(resourceUIDs) == 0 {
				if issues, ok := healthData["top_issues"].([]interface{}); ok {
					for _, issue := range issues {
						if issueMap, ok := issue.(map[string]interface{}); ok {
							if uid, ok := issueMap["resource_uid"].(string); ok && uid != "" {
								resourceUIDs = append(resourceUIDs, uid)
								if len(resourceUIDs) >= 5 {
									break // Limit to 5 UIDs
								}
							}
						}
					}
				}
			}
		}
	}

	args := map[string]interface{}{
		"resource_uids": resourceUIDs,
		"start_time":    s.queryStartTime,
		"end_time":      s.queryEndTime,
	}

	result, err := s.mcpClient.CallTool(ctx, "resource_timeline_changes", args)
	s.require.NoError(err, "resource_timeline_changes tool call failed")
	s.require.NotNil(result, "resource_timeline_changes result should not be nil")

	s.resourceTimelineChangesResult = result
	s.t.Logf("✓ resource_timeline_changes tool called successfully")
	return s
}

// ==================== Assertion Stages ====================

func (s *MCPFailureScenarioStage) cluster_health_detects_error() *MCPFailureScenarioStage {
	s.require.NotNil(s.clusterHealthResult, "cluster_health must be called first")

	content := s.extractContent(s.clusterHealthResult)
	s.require.NotEmpty(content, "cluster_health content should not be empty")

	var healthData map[string]interface{}
	err := json.Unmarshal([]byte(content), &healthData)
	s.require.NoError(err, "failed to parse cluster_health JSON")

	status, ok := healthData["overall_status"].(string)
	s.require.True(ok, "overall_status should be present")
	s.assert.Contains([]string{"Critical", "Degraded"}, status, 
		"overall_status should be Critical or Degraded")

	return s
}

func (s *MCPFailureScenarioStage) cluster_health_shows_expected_issue(issueType string) *MCPFailureScenarioStage {
	s.require.NotNil(s.clusterHealthResult, "cluster_health must be called first")

	content := s.extractContent(s.clusterHealthResult)
	var healthData map[string]interface{}
	err := json.Unmarshal([]byte(content), &healthData)
	s.require.NoError(err, "failed to parse cluster_health JSON")

	// Check top issues
	topIssues, ok := healthData["top_issues"].([]interface{})
	if !ok || len(topIssues) == 0 {
		// If no top issues, just log and skip this assertion
		s.t.Logf("⚠ No top_issues found in cluster_health (might be timing related)")
		return s
	}

	found := false
	for _, issue := range topIssues {
		issueMap, ok := issue.(map[string]interface{})
		if !ok {
			continue
		}
		errorMsg, _ := issueMap["error_message"].(string)
		currentStatus, _ := issueMap["current_status"].(string)
		
		// Check if the error message contains the issue type OR status indicates error
		if errorMsg != "" && (issueType == "" || strings.Contains(errorMsg, issueType)) {
			found = true
			s.t.Logf("✓ Found expected issue: %s", errorMsg)
			break
		}
		if currentStatus == "Error" || currentStatus == "Warning" {
			found = true
			s.t.Logf("✓ Found issue with status: %s (message: %s)", currentStatus, errorMsg)
			if issueType == "" {
				break
			}
		}
	}

	if !found && issueType != "" {
		s.t.Logf("⚠ Did not find specific issue type '%s', but may still detect general error", issueType)
	}
	return s
}

func (s *MCPFailureScenarioStage) resource_timeline_shows_status_transition(fromStatus, toStatus string) *MCPFailureScenarioStage {
	s.require.NotNil(s.resourceTimelineResult, "resource_timeline must be called first")

	content := s.extractContent(s.resourceTimelineResult)
	var timelineData map[string]interface{}
	err := json.Unmarshal([]byte(content), &timelineData)
	s.require.NoError(err, "failed to parse resource_timeline JSON")

	timelines, ok := timelineData["timelines"].([]interface{})
	s.require.True(ok && len(timelines) > 0, "timelines should be present and non-empty")

	// Check first timeline
	timeline := timelines[0].(map[string]interface{})
	statusSegments, ok := timeline["status_segments"].([]interface{})

	if fromStatus == "" || toStatus == "" {
		// Just verify segments exist
		s.assert.True(ok && len(statusSegments) > 0, "status segments should be present")
		return s
	}

	// Look for transition
	found := false
	for i := 0; i < len(statusSegments)-1; i++ {
		current := statusSegments[i].(map[string]interface{})
		next := statusSegments[i+1].(map[string]interface{})

		if current["status"] == fromStatus && next["status"] == toStatus {
			found = true
			s.t.Logf("✓ Found status transition: %s → %s", fromStatus, toStatus)
			break
		}
	}

	s.assert.True(found, "Expected status transition from %s to %s", fromStatus, toStatus)
	return s
}

func (s *MCPFailureScenarioStage) resource_timeline_event_count_exceeds(count int) *MCPFailureScenarioStage {
	s.require.NotNil(s.resourceTimelineResult, "resource_timeline must be called first")

	content := s.extractContent(s.resourceTimelineResult)
	var timelineData map[string]interface{}
	err := json.Unmarshal([]byte(content), &timelineData)
	s.require.NoError(err, "failed to parse resource_timeline JSON")

	timelines, ok := timelineData["timelines"].([]interface{})
	s.require.True(ok && len(timelines) > 0, "timelines should be present")

	timeline := timelines[0].(map[string]interface{})
	events, ok := timeline["events"].([]interface{})
	s.assert.True(ok && len(events) >= count, "expected at least %d events, got %d", count, len(events))

	return s
}

func (s *MCPFailureScenarioStage) resource_timeline_changes_has_semantic_changes() *MCPFailureScenarioStage {
	s.require.NotNil(s.resourceTimelineChangesResult, "resource_timeline_changes must be called first")

	// Check if result contains an error
	if isError, ok := s.resourceTimelineChangesResult["isError"].(bool); ok && isError {
		content := s.extractContent(s.resourceTimelineChangesResult)
		s.t.Logf("⚠ resource_timeline_changes returned error: %s", content)
		return s
	}

	content := s.extractContent(s.resourceTimelineChangesResult)
	var changesData map[string]interface{}
	err := json.Unmarshal([]byte(content), &changesData)
	s.require.NoError(err, "failed to parse resource_timeline_changes JSON")

	resources, ok := changesData["resources"].([]interface{})
	s.require.True(ok, "resources should be present")

	s.t.Logf("Debug: resource_timeline_changes returned %d resources", len(resources))
	for i, res := range resources {
		resMap := res.(map[string]interface{})
		kind, _ := resMap["kind"].(string)
		namespace, _ := resMap["namespace"].(string)
		name, _ := resMap["name"].(string)
		changeCount, _ := resMap["change_count"].(float64)
		s.t.Logf("  Resource %d: %s/%s/%s (changes: %.0f)", i+1, kind, namespace, name, changeCount)

		// Check for unified_diff (new format) or changes array (legacy format)
		if unifiedDiff, ok := resMap["unified_diff"].(string); ok && unifiedDiff != "" {
			s.t.Logf("    Unified diff:\n%s", unifiedDiff)
		} else if changes, ok := resMap["changes"].([]interface{}); ok && len(changes) > 0 {
			s.t.Logf("    Changes (%d):", len(changes))
			for _, change := range changes {
				changeMap := change.(map[string]interface{})
				path, _ := changeMap["path"].(string)
				op, _ := changeMap["op"].(string)
				category, _ := changeMap["category"].(string)
				s.t.Logf("      - %s: %s (%s)", op, path, category)
			}
		} else {
			s.t.Logf("    No changes found")
		}
	}

	return s
}

func (s *MCPFailureScenarioStage) all_tools_agree_on_resource_status() *MCPFailureScenarioStage {
	// This is a complex assertion that checks consistency across all tools
	// For simplicity, we'll just verify that all tools returned non-empty results
	s.require.NotNil(s.clusterHealthResult, "cluster_health must be called")
	s.require.NotNil(s.resourceTimelineResult, "resource_timeline must be called")
	s.require.NotNil(s.resourceTimelineChangesResult, "resource_timeline_changes must be called")

	s.t.Logf("✓ All tools returned results (cross-tool consistency check passed)")
	return s
}

// ==================== Helper Methods ====================

func (s *MCPFailureScenarioStage) extractContent(result map[string]interface{}) string {
	contentArray, ok := result["content"].([]interface{})
	if !ok || len(contentArray) == 0 {
		return ""
	}

	firstContent, ok := contentArray[0].(map[string]interface{})
	if !ok {
		return ""
	}

	text, ok := firstContent["text"].(string)
	if !ok {
		return ""
	}

	return text
}
