package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenarioDefaultResources(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	testCtx := helpers.SetupE2ETest(t)
	k8sClient := testCtx.K8sClient
	apiClient := testCtx.APIClient

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	version, err := k8sClient.GetClusterVersion(ctx)
	require.NoError(t, err, "failed to get cluster version")
	t.Logf("Kubernetes version: %s", version)

	testNamespace1 := "test-default"
	testNamespace2 := "test-alternate"

	for _, ns := range []string{testNamespace1, testNamespace2} {
		ns := ns // capture loop variable for closure
		err := k8sClient.CreateNamespace(ctx, ns)
		require.NoError(t, err, "failed to create namespace %s", ns)
		t.Cleanup(func() {
			if err := k8sClient.DeleteNamespace(context.Background(), ns); err != nil {
				t.Logf("Warning: failed to delete namespace %s: %v", ns, err)
			}
		})
	}

	deployment, err := helpers.CreateTestDeployment(ctx, t, k8sClient, testNamespace1)
	require.NoError(t, err, "failed to create test deployment")
	require.NotNil(t, deployment)

	t.Logf("Deployment created: %s/%s (UID: %s)", deployment.Namespace, deployment.Name, deployment.UID)

	// Wait for deployment to be ready with all replicas available
	err = k8sClient.WaitForDeploymentReady(ctx, testNamespace1, deployment.Name, 2*time.Minute)
	if err != nil {
		t.Logf("Warning: deployment ready check failed (expected in e2e): %v", err)
	}

	resource := helpers.EventuallyResourceCreated(t, apiClient, testNamespace1, "Deployment", deployment.Name, helpers.SlowEventuallyOption)
	require.NotNil(t, resource)
	assert.Equal(t, deployment.Name, resource.Name)
	assert.Equal(t, "Deployment", resource.Kind, "Resource kind mismatch")
	assert.Equal(t, testNamespace1, resource.Namespace, "Resource namespace mismatch")
	assert.NotEmpty(t, resource.ID, "Resource ID should not be empty")
	assert.NotEmpty(t, resource.Name, "Resource name should not be empty")
	t.Logf("✓ Resource found in API: %s/%s (ID: %s)", resource.Namespace, resource.Kind, resource.ID)

	// Test 1: Query with namespace filter
	searchResp, err := apiClient.Search(ctx, time.Now().Unix()-90, time.Now().Unix()+10, testNamespace1, "Deployment")
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
	searchRespAll, err := apiClient.Search(ctx, time.Now().Unix()-90, time.Now().Unix()+10, "", "Deployment")
	require.NoError(t, err)
	assert.Greater(t, searchRespAll.Count, 0, "Should find deployments across all namespaces")
	t.Logf("✓ Unfiltered query works: Found %d total resources", searchRespAll.Count)

	// Test 3: Query with different namespace filter (should return empty or different resources)
	searchRespWrong, err := apiClient.Search(ctx, time.Now().Unix()-90, time.Now().Unix()+10, testNamespace2, "Deployment")
	require.NoError(t, err)
	foundInWrongNamespace := false
	for _, r := range searchRespWrong.Resources {
		if r.Name == deployment.Name {
			foundInWrongNamespace = true
			break
		}
	}
	assert.False(t, foundInWrongNamespace, "Should NOT find test-default deployment when filtering by test-alternate namespace")

	metadata, err := apiClient.GetMetadata(ctx, nil, nil)
	require.NoError(t, err)

	helpers.AssertNamespaceInMetadata(t, metadata, testNamespace1)
	helpers.AssertKindInMetadata(t, metadata, "Deployment")
}
