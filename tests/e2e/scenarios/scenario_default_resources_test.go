// Package scenarios contains e2e test scenarios for KEM.
package scenarios

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moritz/rpk/tests/e2e/helpers"
)

// TestScenarioDefaultResources validates default resource event capture and namespace filtering.
//
// This scenario tests:
// 1. Default KEM configuration watches Deployments and Pods
// 2. Create a Deployment in a test namespace
// 3. Verify the deployment and events appear in the API
// 4. Test namespace filtering (with filter, without filter, wrong namespace)
// 5. Test cross-namespace isolation
//
// Expected behavior:
// - Deployment create event captured within 5 seconds
// - Namespace filter returns only matching resources
// - Unfiltered query returns all namespaces
// - Cross-namespace filter returns no results
func TestScenarioDefaultResources(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// ===== PHASE 1: Setup Test Cluster =====
	t.Log("\n========== PHASE 1: Setup Test Cluster ==========")

	// Create Kind cluster
	clusterName := fmt.Sprintf("kem-e2e-test-%d", time.Now().UnixNano())
	testCluster, err := helpers.CreateKindCluster(t, clusterName)
	require.NoError(t, err, "failed to create Kind cluster")
	defer func() {
		if err := testCluster.Delete(); err != nil {
			t.Logf("Warning: failed to delete cluster: %v", err)
		}
	}()

	// Create Kubernetes client
	k8sClient, err := helpers.NewK8sClient(t, testCluster.GetKubeConfig())
	require.NoError(t, err, "failed to create Kubernetes client")

	// ===== PHASE 2: Verify Cluster =====
	t.Log("\n========== PHASE 2: Verify Cluster ==========")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	version, err := k8sClient.GetClusterVersion(ctx)
	require.NoError(t, err, "failed to get cluster version")
	t.Logf("Kubernetes version: %s", version)

	// ===== PHASE 3: Deploy KEM =====
	t.Log("\n========== PHASE 3: Deploy KEM ==========")

	// TODO: Deploy KEM via Helm when chart is available
	// For now, we skip this - the actual KEM should be running in the cluster
	t.Log("Note: KEM deployment would happen here when Helm chart is available")

	// ===== PHASE 4: Create Test Namespaces =====
	t.Log("\n========== PHASE 4: Create Test Namespaces ==========")

	testNamespace1 := "test-default"
	testNamespace2 := "test-alternate"

	for _, ns := range []string{testNamespace1, testNamespace2} {
		err := k8sClient.CreateNamespace(ctx, ns)
		require.NoError(t, err, "failed to create namespace %s", ns)
		defer func(namespace string) {
			if err := k8sClient.DeleteNamespace(context.Background(), namespace); err != nil {
				t.Logf("Warning: failed to delete namespace %s: %v", namespace, err)
			}
		}(ns)
	}

	// ===== PHASE 5: Create Test Deployment =====
	t.Log("\n========== PHASE 5: Create Test Deployment ==========")

	deployment, err := helpers.CreateTestDeployment(ctx, t, k8sClient, testNamespace1)
	require.NoError(t, err, "failed to create test deployment")
	require.NotNil(t, deployment)

	t.Logf("Deployment created: %s/%s (UID: %s)", deployment.Namespace, deployment.Name, deployment.UID)

	// Wait for deployment to be available
	err = k8sClient.WaitForPodReady(ctx, testNamespace1, "test-deployment", 2*time.Minute)
	if err != nil {
		t.Logf("Warning: pod ready check failed (expected in e2e): %v", err)
	}

	// ===== PHASE 6: Verify API Access =====
	t.Log("\n========== PHASE 6: Verify API Access ==========")

	// Create port-forward (this would normally be done by KEM deployment)
	// For now, we assume KEM is accessible at localhost:8080
	apiClient := helpers.NewAPIClient(t, "http://localhost:8080")

	// Check API health with retry
	helpers.EventuallyAPIAvailable(t, apiClient, helpers.DefaultEventuallyOption)

	// ===== PHASE 7: Test Event Capture =====
	t.Log("\n========== PHASE 7: Test Event Capture ==========")

	// Wait for resource to appear in API
	resource := helpers.EventuallyResourceCreated(t, apiClient, testNamespace1, "Deployment", deployment.Name, helpers.DefaultEventuallyOption)
	require.NotNil(t, resource)

	helpers.AssertResourceExists(t, resource, "Deployment", testNamespace1)
	assert.Equal(t, deployment.Name, resource.Name)

	t.Logf("✓ Resource found in API: %s/%s (ID: %s)", resource.Namespace, resource.Kind, resource.ID)

	// Verify events exist
	helpers.EventuallyEventCount(t, apiClient, resource.ID, 1, helpers.DefaultEventuallyOption)

	// ===== PHASE 8: Test Namespace Filtering =====
	t.Log("\n========== PHASE 8: Test Namespace Filtering ==========")

	// Test 1: Query with namespace filter
	t.Log("Test 1: Query with matching namespace filter")
	searchResp, err := apiClient.Search(ctx, time.Now().Unix()-60, time.Now().Unix()+10, testNamespace1, "Deployment")
	require.NoError(t, err)
	assert.Greater(t, searchResp.Count, 0, "Should find deployments in test-default namespace")
	foundInNamespace := false
	for _, r := range searchResp.Resources {
		if r.Name == deployment.Name && r.Namespace == testNamespace1 {
			foundInNamespace = true
			break
		}
	}
	assert.True(t, foundInNamespace, "Deployment should be found with namespace filter")
	t.Logf("✓ Namespace filter works: Found %d resources", searchResp.Count)

	// Test 2: Query without namespace filter (should return all)
	t.Log("Test 2: Query without namespace filter")
	searchRespAll, err := apiClient.Search(ctx, time.Now().Unix()-60, time.Now().Unix()+10, "", "Deployment")
	require.NoError(t, err)
	assert.Greater(t, searchRespAll.Count, 0, "Should find deployments across all namespaces")
	t.Logf("✓ Unfiltered query works: Found %d total resources", searchRespAll.Count)

	// Test 3: Query with different namespace filter (should return empty or different resources)
	t.Log("Test 3: Query with non-matching namespace filter")
	searchRespWrong, err := apiClient.Search(ctx, time.Now().Unix()-60, time.Now().Unix()+10, testNamespace2, "Deployment")
	require.NoError(t, err)
	foundInWrongNamespace := false
	for _, r := range searchRespWrong.Resources {
		if r.Name == deployment.Name {
			foundInWrongNamespace = true
			break
		}
	}
	assert.False(t, foundInWrongNamespace, "Should NOT find test-default deployment when filtering by test-alternate namespace")
	t.Logf("✓ Cross-namespace filter works correctly: Found %d resources in alternate namespace (expected 0 for test deployment)", searchRespWrong.Count)

	// ===== PHASE 9: Test Metadata Endpoint =====
	t.Log("\n========== PHASE 9: Test Metadata Endpoint ==========")

	metadata, err := apiClient.GetMetadata(ctx, nil, nil)
	require.NoError(t, err)

	helpers.AssertNamespaceInMetadata(t, metadata, testNamespace1)
	helpers.AssertKindInMetadata(t, metadata, "Deployment")

	t.Logf("✓ Metadata contains test namespace and Deployment kind")
	t.Logf("  - Namespaces: %v", metadata.Namespaces)
	t.Logf("  - Kinds: %v", metadata.Kinds)
	t.Logf("  - Total events: %d", metadata.TotalEvents)

	// ===== PHASE 10: Test Event Details =====
	t.Log("\n========== PHASE 10: Test Event Details ==========")

	eventsResp, err := apiClient.GetEvents(ctx, resource.ID, nil, nil, nil)
	require.NoError(t, err)
	assert.Greater(t, len(eventsResp.Events), 0, "Should have at least one event")

	if len(eventsResp.Events) > 0 {
		event := eventsResp.Events[0]
		t.Logf("First event: %s (verb: %s, user: %s, timestamp: %d)", event.ID, event.Verb, event.User, event.Timestamp)

		// Verify event has required fields
		assert.NotEmpty(t, event.ID)
		assert.NotZero(t, event.Timestamp)
		assert.NotEmpty(t, event.Verb)
		assert.NotEmpty(t, event.User)
	}

	t.Log("\n========== TEST COMPLETE: All assertions passed ==========")
}
