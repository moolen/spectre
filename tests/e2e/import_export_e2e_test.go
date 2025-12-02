package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestImportExportRoundTrip validates the full import/export workflow:
// 1. Deploy Spectre via Helm
// 2. Generate test data in two namespaces (import-1, import-2)
// 3. Export the data to a file
// 4. Uninstall Spectre and ensure no PV/PVC remains
// 5. Delete the generated resources and namespaces
// 6. Redeploy Spectre
// 7. Ensure old data is gone
// 8. Import the previously exported data
// 9. Verify the imported data is present
func TestImportExportRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Step 1: Deploy Spectre via Helm
	t.Log("Step 1: Deploying Spectre via Helm")
	testCtx := helpers.SetupE2ETest(t)
	k8sClient := testCtx.K8sClient
	apiClient := testCtx.APIClient

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Step 2: Generate test data in two namespaces
	t.Log("Step 2: Generating test data in two namespaces")
	testNamespaces := []string{"import-1", "import-2"}
	deploymentNames := make(map[string][]string) // namespace -> deployment names

	for _, ns := range testNamespaces {
		err := k8sClient.CreateNamespace(ctx, ns)
		require.NoError(t, err, "failed to create namespace %s", ns)

		deploymentNames[ns] = []string{}

		// Create 25 deployments per namespace (50 total)
		for i := 0; i < 25; i++ {
			deployName := fmt.Sprintf("import-deploy-%d", i)
			deployment := helpers.NewDeploymentBuilder(t, deployName, ns).
				WithImage("nginx:latest").
				WithReplicas(1).
				Build()

			_, err := k8sClient.CreateDeployment(ctx, ns, deployment)
			require.NoError(t, err, "failed to create deployment %s in namespace %s", deployName, ns)

			deploymentNames[ns] = append(deploymentNames[ns], deployName)
		}

		t.Logf("Created 25 deployments in namespace %s", ns)
	}

	// Wait for some resources to be indexed by Spectre
	t.Log("Waiting for resources to be indexed by Spectre")
	time.Sleep(10 * time.Second)

	// Verify data is present in both namespaces
	for _, ns := range testNamespaces {
		helpers.EventuallyCondition(t, func() bool {
			searchCtx, searchCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer searchCancel()

			now := time.Now().Unix()
			resp, err := apiClient.Search(searchCtx, now-300, now+10, ns, "Deployment")
			if err != nil {
				t.Logf("Search failed for namespace %s: %v", ns, err)
				return false
			}

			t.Logf("Found %d resources in namespace %s", resp.Count, ns)
			return resp.Count > 0
		}, helpers.SlowEventuallyOption)
	}

	t.Log("✓ Test data successfully indexed in both namespaces")

	// Step 3: Export data to a temp file
	t.Log("Step 3: Exporting data to temp file")
	exportPath := filepath.Join(t.TempDir(), "export.tar.gz")

	now := time.Now().Unix()
	exportURL := fmt.Sprintf("%s/v1/storage/export?from=%d&to=%d&include_open_hour=true&compression=true",
		apiClient.BaseURL, now-900, now+60) // Last 15 minutes + 1 minute buffer

	exportResp, err := http.Get(exportURL)
	require.NoError(t, err, "failed to request export")
	require.Equal(t, http.StatusOK, exportResp.StatusCode, "export request failed")

	exportFile, err := os.Create(exportPath)
	require.NoError(t, err, "failed to create export file")

	written, err := io.Copy(exportFile, exportResp.Body)
	exportResp.Body.Close()
	exportFile.Close()
	require.NoError(t, err, "failed to write export data")
	require.Greater(t, written, int64(0), "export file is empty")

	t.Logf("✓ Exported %d bytes to %s", written, exportPath)

	// Step 4: Uninstall Spectre and ensure no PV/PVC remains
	t.Log("Step 4: Uninstalling Spectre and verifying PV/PVC cleanup")

	helmDeployer, err := helpers.NewHelmDeployer(t, testCtx.Cluster.GetKubeConfig(), testCtx.Namespace)
	require.NoError(t, err, "failed to create Helm deployer")

	err = helmDeployer.UninstallChart(testCtx.ReleaseName)
	require.NoError(t, err, "failed to uninstall Helm release")

	t.Log("Waiting for resources to be cleaned up")
	time.Sleep(10 * time.Second)

	// Verify PVC is gone
	pvcName := testCtx.ReleaseName + "-k8s-event-monitor"
	_, err = k8sClient.Clientset.CoreV1().PersistentVolumeClaims(testCtx.Namespace).Get(ctx, pvcName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "PVC should be deleted after uninstall")

	t.Log("✓ Spectre uninstalled and PVC cleaned up")

	// Step 5: Delete generated resources and namespaces
	t.Log("Step 5: Deleting generated resources and namespaces")
	for _, ns := range testNamespaces {
		err := k8sClient.DeleteNamespace(ctx, ns)
		require.NoError(t, err, "failed to delete namespace %s", ns)
	}

	// Wait for namespaces to be fully deleted
	for _, ns := range testNamespaces {
		helpers.EventuallyCondition(t, func() bool {
			_, err := k8sClient.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
			return apierrors.IsNotFound(err)
		}, helpers.SlowEventuallyOption)
	}

	t.Log("✓ Test namespaces deleted")

	// Step 6: Redeploy Spectre
	t.Log("Step 6: Redeploying Spectre")

	// Load Helm values
	values, imageRef, err := helpers.LoadHelmValues()
	require.NoError(t, err, "failed to load Helm values")

	// Build and load image again (in case it's not cached)
	err = helpers.BuildAndLoadTestImage(t, testCtx.Cluster.Name, imageRef)
	require.NoError(t, err, "failed to build/load image")

	// Install Helm release
	chartPath, err := helpers.RepoPath("chart")
	require.NoError(t, err, "failed to get chart path")

	err = helmDeployer.InstallOrUpgrade(testCtx.ReleaseName, chartPath, values)
	require.NoError(t, err, "failed to reinstall Helm release")

	// Wait for deployment to be ready
	t.Log("Waiting for Spectre to be ready after redeployment")
	err = helpers.WaitForAppReady(ctx, k8sClient, testCtx.Namespace, testCtx.ReleaseName)
	require.NoError(t, err, "Spectre not ready after redeployment")

	// Reconnect port-forward
	err = testCtx.ReconnectPortForward()
	require.NoError(t, err, "failed to reconnect port-forward")

	// Update API client reference
	apiClient = testCtx.APIClient

	t.Log("✓ Spectre redeployed and ready")

	// Step 7: Ensure old data is gone
	t.Log("Step 7: Verifying old data is not present")

	// Wait a bit for the system to stabilize
	time.Sleep(5 * time.Second)

	metadata, err := apiClient.GetMetadata(ctx, nil, nil)
	require.NoError(t, err, "failed to get metadata")

	for _, ns := range testNamespaces {
		assert.NotContains(t, metadata.Namespaces, ns, "Namespace %s should not be in metadata before import", ns)
	}

	// Verify searches return no results
	for _, ns := range testNamespaces {
		searchResp, err := apiClient.Search(ctx, now-900, now+60, ns, "Deployment")
		require.NoError(t, err, "search failed for namespace %s", ns)
		assert.Equal(t, 0, searchResp.Count, "Should find no resources in namespace %s before import", ns)
	}

	t.Log("✓ Confirmed old data is not present")

	// Step 8: Import the previously exported data
	t.Log("Step 8: Importing previously exported data")

	exportFile, err = os.Open(exportPath)
	require.NoError(t, err, "failed to open export file")
	defer exportFile.Close()

	importURL := fmt.Sprintf("%s/v1/storage/import?validate=true&overwrite=true", apiClient.BaseURL)
	importReq, err := http.NewRequestWithContext(ctx, "POST", importURL, exportFile)
	require.NoError(t, err, "failed to create import request")
	importReq.Header.Set("Content-Type", "application/gzip")

	importResp, err := http.DefaultClient.Do(importReq)
	require.NoError(t, err, "failed to execute import request")
	defer importResp.Body.Close()

	require.Equal(t, http.StatusOK, importResp.StatusCode, "import request failed")

	// Parse import response
	var importReport map[string]interface{}
	err = json.NewDecoder(importResp.Body).Decode(&importReport)
	require.NoError(t, err, "failed to decode import response")

	t.Logf("Import report: %+v", importReport)

	// Verify import was successful
	if failedFiles, ok := importReport["failed_files"].(float64); ok {
		assert.Equal(t, float64(0), failedFiles, "Import should have no failed files")
	}

	if totalEvents, ok := importReport["total_events"].(float64); ok {
		assert.Greater(t, totalEvents, float64(0), "Import should have imported events")
		t.Logf("✓ Imported %.0f events", totalEvents)
	}

	// Step 9: Verify imported data is present
	t.Log("Step 9: Verifying imported data is present")

	// Wait for data to be queryable
	time.Sleep(5 * time.Second)

	// Verify namespaces appear in metadata
	helpers.EventuallyCondition(t, func() bool {
		metadataCtx, metadataCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer metadataCancel()

		metadata, err := apiClient.GetMetadata(metadataCtx, nil, nil)
		if err != nil {
			t.Logf("GetMetadata failed: %v", err)
			return false
		}

		for _, ns := range testNamespaces {
			found := false
			for _, metaNs := range metadata.Namespaces {
				if metaNs == ns {
					found = true
					break
				}
			}
			if !found {
				t.Logf("Namespace %s not yet in metadata", ns)
				return false
			}
		}

		return true
	}, helpers.SlowEventuallyOption)

	t.Log("✓ Namespaces appear in metadata")

	// Verify resources can be queried
	for _, ns := range testNamespaces {
		helpers.EventuallyCondition(t, func() bool {
			searchCtx, searchCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer searchCancel()

			resp, err := apiClient.Search(searchCtx, now-900, now+60, ns, "Deployment")
			if err != nil {
				t.Logf("Search failed for namespace %s: %v", ns, err)
				return false
			}

			t.Logf("Found %d resources in namespace %s after import", resp.Count, ns)
			return resp.Count > 0
		}, helpers.SlowEventuallyOption)
	}

	// Spot-check a specific deployment
	searchResp, err := apiClient.Search(ctx, now-900, now+60, "import-1", "Deployment")
	require.NoError(t, err, "search failed")
	require.Greater(t, searchResp.Count, 0, "should find deployments in import-1")

	foundSpecificDeploy := false
	for _, r := range searchResp.Resources {
		if r.Name == "import-deploy-0" {
			foundSpecificDeploy = true
			t.Logf("✓ Found specific deployment: %s/%s", r.Namespace, r.Name)
			break
		}
	}
	assert.True(t, foundSpecificDeploy, "Should find import-deploy-0 after import")

	t.Log("✓ Import/Export round-trip test completed successfully!")
}
