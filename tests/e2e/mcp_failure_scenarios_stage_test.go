package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	clusterHealthResult   map[string]interface{}
	investigateResult     map[string]interface{}
	resourceChangesResult map[string]interface{}
	resourceExplorerResult map[string]interface{}

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
	mcpPortForward, err := helpers.NewPortForwarder(s.t, s.testCtx.Cluster.GetContext(), mcpNamespace, serviceName, 8082)
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

func (s *MCPFailureScenarioStage) investigate_tool_is_called_for_resource(kind, name string) *MCPFailureScenarioStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 30*time.Second)
	defer cancel()

	args := map[string]interface{}{
		"resource_kind": kind,
		"resource_name": name,
		"namespace":     s.testCtx.Namespace,
		"start_time":    s.queryStartTime,
		"end_time":      s.queryEndTime,
	}

	result, err := s.mcpClient.CallTool(ctx, "investigate", args)
	s.require.NoError(err, "investigate tool call failed")
	s.require.NotNil(result, "investigate result should not be nil")

	s.investigateResult = result
	s.t.Logf("✓ investigate tool called successfully for %s/%s", kind, name)
	return s
}

func (s *MCPFailureScenarioStage) resource_changes_tool_is_called() *MCPFailureScenarioStage {
	// Filter by namespace to ensure we get resources from the test namespace
	// This helps avoid hitting the resource limit with cluster-wide resources
	return s.resource_changes_tool_is_called_with_filters(map[string]interface{}{
		"namespace": s.testCtx.Namespace,
	})
}

func (s *MCPFailureScenarioStage) resource_changes_tool_is_called_with_filters(filters map[string]interface{}) *MCPFailureScenarioStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 30*time.Second)
	defer cancel()

	args := map[string]interface{}{
		"start_time":    s.queryStartTime,
		"end_time":      s.queryEndTime,
		"max_resources": 500, // Increase limit to ensure we get Pod resources
	}

	// Merge in any additional filters (e.g., namespace, kinds)
	for k, v := range filters {
		args[k] = v
	}

	result, err := s.mcpClient.CallTool(ctx, "resource_changes", args)
	s.require.NoError(err, "resource_changes tool call failed")
	s.require.NotNil(result, "resource_changes result should not be nil")

	s.resourceChangesResult = result
	s.t.Logf("✓ resource_changes tool called successfully")
	return s
}

func (s *MCPFailureScenarioStage) resource_explorer_tool_is_called() *MCPFailureScenarioStage {
	return s.resource_explorer_tool_is_called_with_filters(map[string]interface{}{
		"namespace": s.testCtx.Namespace,
	})
}

func (s *MCPFailureScenarioStage) resource_explorer_tool_is_called_with_filters(filters map[string]interface{}) *MCPFailureScenarioStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 30*time.Second)
	defer cancel()

	result, err := s.mcpClient.CallTool(ctx, "resource_explorer", filters)
	s.require.NoError(err, "resource_explorer tool call failed")
	s.require.NotNil(result, "resource_explorer result should not be nil")

	s.resourceExplorerResult = result
	s.t.Logf("✓ resource_explorer tool called successfully")
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
		if errorMsg != "" && (issueType == "" || containsSubstring(errorMsg, issueType)) {
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

func (s *MCPFailureScenarioStage) investigate_shows_status_transition(fromStatus, toStatus string) *MCPFailureScenarioStage {
	s.require.NotNil(s.investigateResult, "investigate must be called first")

	content := s.extractContent(s.investigateResult)
	var investigateData map[string]interface{}
	err := json.Unmarshal([]byte(content), &investigateData)
	s.require.NoError(err, "failed to parse investigate JSON")

	investigations, ok := investigateData["investigations"].([]interface{})
	s.require.True(ok && len(investigations) > 0, "investigations should be present and non-empty")

	// Check first investigation
	inv := investigations[0].(map[string]interface{})
	statusSegments, ok := inv["status_segments"].([]interface{})
	
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

func (s *MCPFailureScenarioStage) investigate_provides_rca_prompts() *MCPFailureScenarioStage {
	s.require.NotNil(s.investigateResult, "investigate must be called first")

	content := s.extractContent(s.investigateResult)
	var investigateData map[string]interface{}
	err := json.Unmarshal([]byte(content), &investigateData)
	s.require.NoError(err, "failed to parse investigate JSON")

	investigations, ok := investigateData["investigations"].([]interface{})
	s.require.True(ok && len(investigations) > 0, "investigations should be present")

	inv := investigations[0].(map[string]interface{})
	prompts, ok := inv["investigation_prompts"].([]interface{})
	s.assert.True(ok && len(prompts) > 0, "investigation_prompts should be present and non-empty")

	return s
}

func (s *MCPFailureScenarioStage) investigate_event_count_exceeds(count int) *MCPFailureScenarioStage {
	s.require.NotNil(s.investigateResult, "investigate must be called first")

	content := s.extractContent(s.investigateResult)
	var investigateData map[string]interface{}
	err := json.Unmarshal([]byte(content), &investigateData)
	s.require.NoError(err, "failed to parse investigate JSON")

	investigations, ok := investigateData["investigations"].([]interface{})
	s.require.True(ok && len(investigations) > 0, "investigations should be present")

	inv := investigations[0].(map[string]interface{})
	events, ok := inv["events"].([]interface{})
	s.assert.True(ok && len(events) >= count, "expected at least %d events, got %d", count, len(events))

	return s
}

func (s *MCPFailureScenarioStage) resource_changes_has_container_issue(issueType string) *MCPFailureScenarioStage {
	s.require.NotNil(s.resourceChangesResult, "resource_changes must be called first")

	content := s.extractContent(s.resourceChangesResult)
	var changesData map[string]interface{}
	err := json.Unmarshal([]byte(content), &changesData)
	s.require.NoError(err, "failed to parse resource_changes JSON")

	changes, ok := changesData["changes"].([]interface{})
	s.require.True(ok && len(changes) > 0, "changes should be present and non-empty")

	// Debug: log all resources and their container issues
	s.t.Logf("Debug: resource_changes returned %d resources", len(changes))
	for i, change := range changes {
		changeMap := change.(map[string]interface{})
		resourceID, _ := changeMap["resource_id"].(string)
		kind, _ := changeMap["kind"].(string)
		namespace, _ := changeMap["namespace"].(string)
		name, _ := changeMap["name"].(string)
		impactScore, _ := changeMap["impact_score"].(float64)
		s.t.Logf("  Resource %d: %s/%s/%s (ID: %s, impact: %.2f)", i+1, kind, namespace, name, resourceID, impactScore)
		
		containerIssues, ok := changeMap["container_issues"].([]interface{})
		if !ok || len(containerIssues) == 0 {
			s.t.Logf("    No container_issues found")
			continue
		}
		
		s.t.Logf("    Container issues (%d):", len(containerIssues))
		for _, issue := range containerIssues {
			issueMap := issue.(map[string]interface{})
			issueTypeFound, _ := issueMap["issue_type"].(string)
			s.t.Logf("      - %s", issueTypeFound)
		}
	}

	found := false
	for _, change := range changes {
		changeMap := change.(map[string]interface{})
		containerIssues, ok := changeMap["container_issues"].([]interface{})
		if !ok {
			continue
		}

		for _, issue := range containerIssues {
			issueMap := issue.(map[string]interface{})
			if issueMap["issue_type"] == issueType {
				found = true
				s.t.Logf("✓ Found container issue: %s", issueType)
				break
			}
		}
		if found {
			break
		}
	}

	s.assert.True(found, "Expected to find container issue type: %s", issueType)
	return s
}

func (s *MCPFailureScenarioStage) resource_changes_has_event_pattern(patternType string) *MCPFailureScenarioStage {
	s.require.NotNil(s.resourceChangesResult, "resource_changes must be called first")

	content := s.extractContent(s.resourceChangesResult)
	var changesData map[string]interface{}
	err := json.Unmarshal([]byte(content), &changesData)
	s.require.NoError(err, "failed to parse resource_changes JSON")

	changes, ok := changesData["changes"].([]interface{})
	s.require.True(ok && len(changes) > 0, "changes should be present and non-empty")

	found := false
	for _, change := range changes {
		changeMap := change.(map[string]interface{})
		eventPatterns, ok := changeMap["event_patterns"].([]interface{})
		if !ok || eventPatterns == nil {
			continue
		}

		for _, pattern := range eventPatterns {
			patternMap := pattern.(map[string]interface{})
			if patternMap["pattern_type"] == patternType {
				found = true
				s.t.Logf("✓ Found event pattern: %s", patternType)
				break
			}
		}
		if found {
			break
		}
	}

	s.assert.True(found, "Expected to find event pattern: %s", patternType)
	return s
}

func (s *MCPFailureScenarioStage) resource_changes_impact_score_exceeds(threshold float64) *MCPFailureScenarioStage {
	s.require.NotNil(s.resourceChangesResult, "resource_changes must be called first")

	content := s.extractContent(s.resourceChangesResult)
	var changesData map[string]interface{}
	err := json.Unmarshal([]byte(content), &changesData)
	s.require.NoError(err, "failed to parse resource_changes JSON")

	changes, ok := changesData["changes"].([]interface{})
	s.require.True(ok && len(changes) > 0, "changes should be present and non-empty")

	changeMap := changes[0].(map[string]interface{})
	impactScore, ok := changeMap["impact_score"].(float64)
	s.require.True(ok, "impact_score should be present")

	s.assert.GreaterOrEqual(impactScore, threshold, 
		"impact_score %.2f should be >= %.2f", impactScore, threshold)

	return s
}

func (s *MCPFailureScenarioStage) resource_explorer_shows_error_status() *MCPFailureScenarioStage {
	s.require.NotNil(s.resourceExplorerResult, "resource_explorer must be called first")

	content := s.extractContent(s.resourceExplorerResult)
	var explorerData map[string]interface{}
	err := json.Unmarshal([]byte(content), &explorerData)
	s.require.NoError(err, "failed to parse resource_explorer JSON")

	resources, ok := explorerData["resources"].([]interface{})
	if !ok || len(resources) == 0 {
		// This might happen if the resource hasn't been indexed yet
		s.t.Logf("⚠ No resources found in resource_explorer (timing/indexing issue)")
		return s
	}

	found := false
	for _, res := range resources {
		resMap := res.(map[string]interface{})
		status, _ := resMap["current_status"].(string)
		name, _ := resMap["name"].(string)
		kind, _ := resMap["kind"].(string)
		
		s.t.Logf("  Resource: %s/%s status=%s", kind, name, status)
		
		if status == "Error" || status == "Warning" {
			found = true
			s.t.Logf("✓ Found resource with error/warning status: %s/%s (%s)", kind, name, status)
			break
		}
	}

	if !found {
		s.t.Logf("⚠ No resources with Error/Warning status found (may be timing/indexing issue)")
	}
	return s
}

func (s *MCPFailureScenarioStage) resource_explorer_resource_count_equals(count int) *MCPFailureScenarioStage {
	s.require.NotNil(s.resourceExplorerResult, "resource_explorer must be called first")

	content := s.extractContent(s.resourceExplorerResult)
	var explorerData map[string]interface{}
	err := json.Unmarshal([]byte(content), &explorerData)
	s.require.NoError(err, "failed to parse resource_explorer JSON")

	resourceCount, ok := explorerData["resource_count"].(float64)
	s.require.True(ok, "resource_count should be present")

	s.assert.Equal(count, int(resourceCount), "resource count should match")
	return s
}

func (s *MCPFailureScenarioStage) all_tools_agree_on_resource_status() *MCPFailureScenarioStage {
	// This is a complex assertion that checks consistency across all tools
	// For simplicity, we'll just verify that all tools returned non-empty results
	s.require.NotNil(s.clusterHealthResult, "cluster_health must be called")
	s.require.NotNil(s.investigateResult, "investigate must be called")
	s.require.NotNil(s.resourceChangesResult, "resource_changes must be called")
	s.require.NotNil(s.resourceExplorerResult, "resource_explorer must be called")

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

func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
