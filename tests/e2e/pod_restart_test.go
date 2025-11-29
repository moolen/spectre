package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenarioPodRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	testCtx := helpers.SetupE2ETest(t)
	k8sClient := testCtx.K8sClient
	apiClient := testCtx.APIClient

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	testNamespace := "test-restart"
	err := k8sClient.CreateNamespace(ctx, testNamespace)
	require.NoError(t, err, "failed to create namespace")
	t.Cleanup(func() {
		if err := k8sClient.DeleteNamespace(context.Background(), testNamespace); err != nil {
			t.Logf("Warning: failed to delete namespace: %v", err)
		}
	})

	deployment1, err := helpers.CreateTestDeployment(ctx, t, k8sClient, testNamespace)
	require.NoError(t, err, "failed to create first deployment")
	resource1 := helpers.EventuallyResourceCreated(t, apiClient, testNamespace, "Deployment", deployment1.Name, helpers.DefaultEventuallyOption)
	require.NotNil(t, resource1)

	// delete pod to trigger restart
	podList, err := testCtx.K8sClient.ListPods(ctx, "monitoring", "app.kubernetes.io/instance="+testCtx.ReleaseName)
	require.NoError(t, err, "failed to list pods")
	require.Greater(t, len(podList.Items), 0, "should have at least one pod")
	podName := podList.Items[0].Name
	if err := testCtx.K8sClient.DeletePod(ctx, "monitoring", podName); err != nil {
		t.Logf("Warning: failed to delete pod %s: %v", podName, err)
	}

	err = helpers.WaitForAppReady(ctx, testCtx.K8sClient, "monitoring", testCtx.ReleaseName)
	require.NoError(t, err, "failed to wait for app to be ready")

	// Re-establish port-forward to the new pod (the old one was connected to the deleted pod)
	err = testCtx.ReconnectPortForward()
	require.NoError(t, err, "failed to reconnect port-forward after pod restart")

	// Update the apiClient reference since ReconnectPortForward creates a new one
	apiClient = testCtx.APIClient

	resource1 = helpers.EventuallyResourceCreated(t, apiClient, testNamespace, "Deployment", deployment1.Name, helpers.DefaultEventuallyOption)
	require.NotNil(t, resource1)

	deployment2Builder := helpers.NewDeploymentBuilder(t, "test-deployment-2", testNamespace)
	deployment2 := deployment2Builder.WithReplicas(1).Build()
	deployment2Created, err := k8sClient.CreateDeployment(ctx, testNamespace, deployment2)
	require.NoError(t, err, "failed to create second deployment")
	resource2 := helpers.EventuallyResourceCreated(t, apiClient, testNamespace, "Deployment", deployment2Created.Name, helpers.DefaultEventuallyOption)
	require.NotNil(t, resource2)

	searchResp, err := apiClient.Search(ctx, time.Now().Unix()-120, time.Now().Unix()+10, testNamespace, "Deployment")
	require.NoError(t, err)

	foundDeployment1 := false
	foundDeployment2 := false

	for _, r := range searchResp.Resources {
		if r.Name == deployment1.Name {
			foundDeployment1 = true
		}
		if r.Name == deployment2Created.Name {
			foundDeployment2 = true
		}
	}

	assert.True(t, foundDeployment1, "First deployment should still be found in search")
	assert.True(t, foundDeployment2, "Second deployment should be found in search")
}
