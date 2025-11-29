package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestScenarioDynamicConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	testCtx := helpers.SetupE2ETest(t)
	k8sClient := testCtx.K8sClient
	apiClient := testCtx.APIClient

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	testNamespace := "test-config"
	err := k8sClient.CreateNamespace(ctx, testNamespace)
	require.NoError(t, err, "failed to create namespace")
	t.Cleanup(func() {
		if err := k8sClient.DeleteNamespace(context.Background(), testNamespace); err != nil {
			t.Logf("Warning: failed to delete namespace: %v", err)
		}
	})

	ssBuilder := helpers.NewStatefulSetBuilder(t, "test-statefulset", testNamespace)
	statefulSet := ssBuilder.WithReplicas(1).Build()

	ssCreated, err := k8sClient.Clientset.AppsV1().StatefulSets(testNamespace).Create(ctx, statefulSet, metav1.CreateOptions{})
	require.NoError(t, err, "failed to create StatefulSet")

	t.Logf("StatefulSet created: %s/%s", ssCreated.Namespace, ssCreated.Name)

	// Give it time to generate events
	time.Sleep(30 * time.Second)

	// Try to find StatefulSet in API - should NOT be there with default config
	searchResp, err := apiClient.Search(ctx, time.Now().Unix()-60, time.Now().Unix()+10, testNamespace, "StatefulSet")
	require.NoError(t, err)

	foundStatefulSet := false
	for _, r := range searchResp.Resources {
		if r.Name == ssCreated.Name && r.Kind == "StatefulSet" {
			foundStatefulSet = true
			break
		}
	}
	assert.False(t, foundStatefulSet, "StatefulSet should NOT be found with default watch config")

	// Update the watcher ConfigMap directly to include StatefulSet
	// This avoids a Helm upgrade which would trigger a deployment rollout
	configMapName := fmt.Sprintf("%s-k8s-event-monitor", testCtx.ReleaseName)
	newWatcherConfig := `resources:
  - group: "apps"
    version: "v1"
    kind: "StatefulSet"
  - group: "apps"
    version: "v1"
    kind: "Deployment"
`
	err = k8sClient.UpdateConfigMap(ctx, testCtx.Namespace, configMapName, map[string]string{
		"watcher.yaml": newWatcherConfig,
	})
	require.NoError(t, err, "failed to update watcher ConfigMap")
	t.Logf("Waiting for ConfigMap propagation and hot-reload (up to 90 seconds)...")

	// Poll for the StatefulSet to appear in the API, which indicates hot-reload worked
	var foundStatefulSetAfterReload bool
	pollTimeout := time.After(90 * time.Second)
	pollTicker := time.NewTicker(5 * time.Second)
	defer pollTicker.Stop()

pollLoop:
	for {
		select {
		case <-pollTimeout:
			t.Logf("Timeout waiting for StatefulSet to appear after config reload")
			break pollLoop
		case <-pollTicker.C:
			searchRespAfter, err := apiClient.Search(ctx, time.Now().Unix()-500, time.Now().Unix()+10, testNamespace, "StatefulSet")
			if err != nil {
				t.Logf("Search error: %v", err)
				continue
			}
			for _, r := range searchRespAfter.Resources {
				if r.Name == ssCreated.Name && r.Kind == "StatefulSet" {
					foundStatefulSetAfterReload = true
					t.Logf("✓ StatefulSet found in API after config reload!")
					break pollLoop
				}
			}
			t.Logf("  StatefulSet not yet visible, waiting...")
		}
	}

	require.True(t, foundStatefulSetAfterReload, "StatefulSet should be found after config reload - hot-reload may not be working")

	// Create a deployment to verify we can still capture new resources
	deployment, err := helpers.CreateTestDeployment(ctx, t, k8sClient, testNamespace)
	require.NoError(t, err, "failed to create deployment")
	depResource := helpers.EventuallyResourceCreated(t, apiClient, testNamespace, "Deployment", deployment.Name, helpers.DefaultEventuallyOption)
	require.NotNil(t, depResource)
	t.Logf("✓ Deployment also captured after config reload")

	// Verify metadata includes both resource kinds
	metadataStart := time.Now().Unix() - 500
	metadataEnd := time.Now().Unix() + 10
	metadata, err := apiClient.GetMetadata(ctx, &metadataStart, &metadataEnd)
	require.NoError(t, err)
	assert.Contains(t, metadata.Namespaces, testNamespace)
	assert.Contains(t, metadata.Kinds, "StatefulSet", "StatefulSet should be in metadata kinds")
	assert.Contains(t, metadata.Kinds, "Deployment", "Deployment should be in metadata kinds")
	t.Logf("✓ Metadata contains both StatefulSet and Deployment kinds")
}
