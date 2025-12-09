package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/moolen/spectre/internal/models"
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
	pvcName := testCtx.ReleaseName + "-spectre"
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
	importReq.Header.Set("Content-Type", "application/vnd.spectre.events.v1+bin")

	importResp, err := http.DefaultClient.Do(importReq)
	require.NoError(t, err, "failed to execute import request")
	defer importResp.Body.Close()

	// Parse import response
	var importReport map[string]interface{}
	err = json.NewDecoder(importResp.Body).Decode(&importReport)
	require.NoError(t, err, "failed to decode import response")
	t.Logf("Import report: %+v", importReport)

	require.Equal(t, http.StatusOK, importResp.StatusCode, "import request failed")

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

// TestJSONEventBatchImport validates the JSON event batch import functionality:
// 1. Deploy Spectre via Helm
// 2. Generate 30-40 test events of different kinds across multiple namespaces
// 3. Call the JSON import endpoint
// 4. Verify imported events are present via search and metadata APIs
func TestJSONEventBatchImport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Step 1: Deploy Spectre via Helm
	t.Log("Step 1: Deploying Spectre via Helm")
	testCtx := helpers.SetupE2ETest(t)
	apiClient := testCtx.APIClient

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Step 2: Generate test events
	t.Log("Step 2: Generating test events")
	now := time.Now()
	testNamespaces := []string{"e2e-import-json-1", "e2e-import-json-2", "e2e-import-json-3", "e2e-import-json-4"}

	events := generateTestEvents(now, testNamespaces)
	t.Logf("Generated %d test events", len(events))

	// Step 3: Prepare JSON payload and call import endpoint
	t.Log("Step 3: Calling JSON import endpoint")

	importPayload := map[string]interface{}{
		"events": events,
	}

	payloadJSON, err := json.Marshal(importPayload)
	require.NoError(t, err, "failed to marshal import payload")

	importURL := fmt.Sprintf("%s/v1/storage/import?validate=true&overwrite=true", apiClient.BaseURL)
	importReq, err := http.NewRequestWithContext(ctx, "POST", importURL, bytes.NewReader(payloadJSON))
	require.NoError(t, err, "failed to create import request")

	// Set custom content-type for JSON event batch
	importReq.Header.Set("Content-Type", "application/vnd.spectre.events.v1+json")

	importResp, err := http.DefaultClient.Do(importReq)
	require.NoError(t, err, "failed to execute import request")
	defer importResp.Body.Close()

	require.Equal(t, http.StatusOK, importResp.StatusCode, "import request failed with status %d", importResp.StatusCode)

	// Parse import response
	var importReport map[string]interface{}
	err = json.NewDecoder(importResp.Body).Decode(&importReport)
	require.NoError(t, err, "failed to decode import response")

	t.Logf("Import report: %+v", importReport)

	// Verify import was successful
	totalEventsImported := int64(0)
	if totalEvents, ok := importReport["total_events"].(float64); ok {
		assert.Greater(t, totalEvents, float64(0), "Import should have imported events")
		totalEventsImported = int64(totalEvents)
		t.Logf("✓ Imported %.0f events", totalEvents)
	}

	require.Greater(t, totalEventsImported, int64(0), "No events were imported")

	// Step 4: Verify imported data is present
	t.Log("Step 4: Verifying imported data via search and metadata APIs")

	// Wait for data to be indexed
	time.Sleep(3 * time.Second)

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

	t.Log("✓ All test namespaces appear in metadata")

	// Verify all resource kinds are present
	helpers.EventuallyCondition(t, func() bool {
		metadataCtx, metadataCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer metadataCancel()

		metadata, err := apiClient.GetMetadata(metadataCtx, nil, nil)
		if err != nil {
			t.Logf("GetMetadata failed: %v", err)
			return false
		}

		expectedKinds := map[string]bool{
			"Deployment": false,
			"Pod":        false,
			"Service":    false,
			"ConfigMap":  false,
		}

		for _, kind := range metadata.Kinds {
			if _, exists := expectedKinds[kind]; exists {
				expectedKinds[kind] = true
			}
		}

		allFound := true
		for kind, found := range expectedKinds {
			if !found {
				t.Logf("Kind %s not yet in metadata", kind)
				allFound = false
			}
		}

		return allFound
	}, helpers.SlowEventuallyOption)

	t.Log("✓ All expected resource kinds appear in metadata")

	// Verify resources can be queried by namespace and kind
	resourceKinds := []string{"Deployment", "Pod", "Service", "ConfigMap"}
	startTime := now.Unix() - 300 // 5 minutes before
	endTime := now.Unix() + 300   // 5 minutes after

	for _, ns := range testNamespaces {
		for _, kind := range resourceKinds {
			helpers.EventuallyCondition(t, func() bool {
				searchCtx, searchCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer searchCancel()

				resp, err := apiClient.Search(searchCtx, startTime, endTime, ns, kind)
				if err != nil {
					t.Logf("Search failed for namespace %s, kind %s: %v", ns, kind, err)
					return false
				}

				if resp.Count > 0 {
					t.Logf("Found %d %s resources in namespace %s", resp.Count, kind, ns)
					return true
				}

				return false
			}, helpers.SlowEventuallyOption)
		}
	}

	t.Log("✓ All resource kinds queryable in all test namespaces")

	// Spot-check specific resources by name
	// Note: kindIdx in generation is 0=Deployment, 1=Pod, 2=Service, 3=ConfigMap
	expectedResources := []struct {
		namespace string
		name      string
		kind      string
	}{
		{"e2e-import-json-1", "test-deployment-0", "Deployment"}, // kindIdx=0
		{"e2e-import-json-1", "test-pod-1", "Pod"},               // kindIdx=1
		{"e2e-import-json-2", "test-service-2", "Service"},       // kindIdx=2
		{"e2e-import-json-2", "test-configmap-3", "ConfigMap"},   // kindIdx=3
	}

	for _, expected := range expectedResources {
		helpers.EventuallyCondition(t, func() bool {
			searchCtx, searchCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer searchCancel()

			resp, err := apiClient.Search(searchCtx, startTime, endTime, expected.namespace, expected.kind)
			if err != nil {
				t.Logf("Search failed for %s/%s: %v", expected.namespace, expected.name, err)
				return false
			}

			for _, r := range resp.Resources {
				if r.Name == expected.name && r.Kind == expected.kind {
					t.Logf("✓ Found expected resource: %s/%s (%s)", r.Namespace, r.Name, r.Kind)
					return true
				}
			}

			t.Logf("Resource %s/%s not yet found", expected.namespace, expected.name)
			return false
		}, helpers.SlowEventuallyOption)
	}

	t.Log("✓ JSON event batch import test completed successfully!")
}

// TestBatchImportWithResourceTimeline validates batch import with multiple update events for a single resource:
// 1. Deploy Spectre via Helm
// 2. Generate a single Service resource with 10 update events in short succession (5-30 seconds apart)
// 3. Push the JSON data to the batch import endpoint
// 4. Verify the data is available through the timeline API
func TestBatchImportWithResourceTimeline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Step 1: Deploy Spectre via Helm
	t.Log("Step 1: Deploying Spectre via Helm")
	testCtx := helpers.SetupE2ETest(t)
	apiClient := testCtx.APIClient

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Step 2: Generate test events for a single resource with multiple updates
	t.Log("Step 2: Generating Service resource with 10 update events")

	now := time.Now()
	namespace := "e2e-timeline-test"
	serviceName := "test-service-timeline"
	serviceUID := uuid.New().String()

	var events []*models.Event

	// Create initial CREATE event
	createEvent := &models.Event{
		ID:        uuid.New().String(),
		Timestamp: now.UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Service",
			Namespace: namespace,
			Name:      serviceName,
			UID:       serviceUID,
		},
		Data: []byte(fmt.Sprintf(
			`{"apiVersion":"v1","kind":"Service","metadata":{"name":"%s","namespace":"%s","uid":"%s"},"spec":{"ports":[{"port":80,"targetPort":8080}],"selector":{"app":"test"}}}`,
			serviceName, namespace, serviceUID,
		)),
	}
	createEvent.DataSize = int32(len(createEvent.Data))
	events = append(events, createEvent)

	// Create 10 UPDATE events with 5-30 seconds between them
	baseInterval := 5 * time.Second
	for i := 0; i < 10; i++ {
		// Vary the interval between 5-30 seconds
		interval := baseInterval + time.Duration(i*2)*time.Second
		timestamp := now.Add(interval)

		updateEvent := &models.Event{
			ID:        uuid.New().String(),
			Timestamp: timestamp.UnixNano(),
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				Group:     "",
				Version:   "v1",
				Kind:      "Service",
				Namespace: namespace,
				Name:      serviceName,
				UID:       serviceUID,
			},
			Data: []byte(fmt.Sprintf(
				`{"apiVersion":"v1","kind":"Service","metadata":{"name":"%s","namespace":"%s","uid":"%s","resourceVersion":"%d"},"spec":{"ports":[{"port":80,"targetPort":%d}],"selector":{"app":"test","version":"v%d"}}}`,
				serviceName, namespace, serviceUID, i+2, 8080+i, i+1,
			)),
		}
		updateEvent.DataSize = int32(len(updateEvent.Data))
		events = append(events, updateEvent)
	}

	t.Logf("Generated %d events (1 CREATE + 10 UPDATE) for Service %s/%s", len(events), namespace, serviceName)

	// Step 3: Push JSON data to batch import endpoint
	t.Log("Step 3: Importing events via JSON batch import endpoint")

	importPayload := map[string]interface{}{
		"events": events,
	}

	payloadJSON, err := json.Marshal(importPayload)
	require.NoError(t, err, "failed to marshal import payload")

	importURL := fmt.Sprintf("%s/v1/storage/import?validate=true&overwrite=true", apiClient.BaseURL)
	importReq, err := http.NewRequestWithContext(ctx, "POST", importURL, bytes.NewReader(payloadJSON))
	require.NoError(t, err, "failed to create import request")
	importReq.Header.Set("Content-Type", "application/vnd.spectre.events.v1+json")

	importResp, err := http.DefaultClient.Do(importReq)
	require.NoError(t, err, "failed to execute import request")
	defer importResp.Body.Close()

	require.Equal(t, http.StatusOK, importResp.StatusCode, "import request failed with status %d", importResp.StatusCode)

	// Parse import response
	var importReport map[string]interface{}
	err = json.NewDecoder(importResp.Body).Decode(&importReport)
	require.NoError(t, err, "failed to decode import response")
	t.Logf("Import report: %+v", importReport)

	// Verify all events were imported
	if totalEvents, ok := importReport["total_events"].(float64); ok {
		assert.Equal(t, float64(11), totalEvents, "Expected 11 events to be imported (1 CREATE + 10 UPDATE)")
		t.Logf("✓ Imported %.0f events", totalEvents)
	} else {
		t.Fatal("total_events not found in import report")
	}

	// Step 4: Verify data is available through the timeline API
	t.Log("Step 4: Verifying data via timeline API")

	// Wait for data to be indexed
	time.Sleep(3 * time.Second)

	// Verify namespace appears in metadata
	helpers.EventuallyCondition(t, func() bool {
		metadataCtx, metadataCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer metadataCancel()

		metadata, err := apiClient.GetMetadata(metadataCtx, nil, nil)
		if err != nil {
			t.Logf("GetMetadata failed: %v", err)
			return false
		}

		for _, ns := range metadata.Namespaces {
			if ns == namespace {
				t.Logf("✓ Namespace %s found in metadata", namespace)
				return true
			}
		}

		t.Logf("Namespace %s not yet in metadata", namespace)
		return false
	}, helpers.SlowEventuallyOption)

	// Verify Service kind appears in metadata
	helpers.EventuallyCondition(t, func() bool {
		metadataCtx, metadataCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer metadataCancel()

		metadata, err := apiClient.GetMetadata(metadataCtx, nil, nil)
		if err != nil {
			t.Logf("GetMetadata failed: %v", err)
			return false
		}

		for _, kind := range metadata.Kinds {
			if kind == "Service" {
				t.Logf("✓ Service kind found in metadata")
				return true
			}
		}

		t.Logf("Service kind not yet in metadata")
		return false
	}, helpers.SlowEventuallyOption)

	// Verify resource can be found via search
	startTime := now.Unix() - 300
	endTime := now.Unix() + 300

	var resourceID string
	helpers.EventuallyCondition(t, func() bool {
		searchCtx, searchCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer searchCancel()

		resp, err := apiClient.Search(searchCtx, startTime, endTime, namespace, "Service")
		if err != nil {
			t.Logf("Search failed: %v", err)
			return false
		}

		for _, r := range resp.Resources {
			if r.Name == serviceName {
				resourceID = r.ID
				t.Logf("✓ Found Service %s/%s with ID %s", namespace, serviceName, resourceID)
				return true
			}
		}

		t.Logf("Service %s/%s not yet found in search results", namespace, serviceName)
		return false
	}, helpers.SlowEventuallyOption)

	require.NotEmpty(t, resourceID, "Resource ID should be set after search")
	require.Equal(t, serviceUID, resourceID, "Resource ID should match the UID we created")

	// Verify timeline shows all status segments for this resource
	// The timeline endpoint returns resources with their full status history
	var timelineResource *helpers.Resource
	helpers.EventuallyCondition(t, func() bool {
		timelineCtx, timelineCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer timelineCancel()

		timelineURL := fmt.Sprintf("%s/v1/timeline?start=%d&end=%d&namespace=%s&kind=Service",
			apiClient.BaseURL, startTime, endTime, namespace)

		req, err := http.NewRequestWithContext(timelineCtx, "GET", timelineURL, nil)
		if err != nil {
			t.Logf("Failed to create timeline request: %v", err)
			return false
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Logf("Timeline request failed: %v", err)
			return false
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Logf("Timeline request returned status %d", resp.StatusCode)
			return false
		}

		var timelineResp helpers.SearchResponse
		if err := json.NewDecoder(resp.Body).Decode(&timelineResp); err != nil {
			t.Logf("Failed to decode timeline response: %v", err)
			return false
		}

		// Find our specific resource
		for i := range timelineResp.Resources {
			if timelineResp.Resources[i].ID == resourceID {
				timelineResource = &timelineResp.Resources[i]
				break
			}
		}

		if timelineResource == nil {
			t.Logf("Resource %s not found in timeline response", resourceID)
			return false
		}

		// The timeline should show status segments for all the updates
		// With 11 events (1 CREATE + 10 UPDATE), we should have at least some status segments
		if len(timelineResource.StatusSegments) < 1 {
			t.Logf("Expected status segments in timeline, got %d", len(timelineResource.StatusSegments))
			return false
		}

		t.Logf("✓ Timeline contains %d status segments for resource", len(timelineResource.StatusSegments))
		return true
	}, helpers.SlowEventuallyOption)

	require.NotNil(t, timelineResource, "Timeline resource should be found")

	// Verify status segments exist and are ordered
	assert.Greater(t, len(timelineResource.StatusSegments), 0, "Should have status segments")
	for i := 1; i < len(timelineResource.StatusSegments); i++ {
		assert.LessOrEqual(t, timelineResource.StatusSegments[i-1].StartTime, timelineResource.StatusSegments[i].StartTime,
			"Status segments should be ordered by start time")
	}

	t.Log("✓ Batch import with resource timeline test completed successfully!")
}

// generateTestEvents creates a batch of test events with variety across namespaces and kinds
func generateTestEvents(baseTime time.Time, namespaces []string) []*models.Event {
	var events []*models.Event

	kinds := []string{"Deployment", "Pod", "Service", "ConfigMap"}
	eventTypes := []models.EventType{models.EventTypeCreate, models.EventTypeUpdate, models.EventTypeDelete}

	for nsIdx, ns := range namespaces {
		for kindIdx, kind := range kinds {
			// Create 2-3 events per kind per namespace
			// All events for the same resource should share the same UID
			resourceUID := fmt.Sprintf("uid-%s-%d-%d", ns, nsIdx, kindIdx)
			// Use lowercase kind in resource name to match test expectations
			resourceName := fmt.Sprintf("test-%s-%d", strings.ToLower(kind), kindIdx)

			for eventIdx := 0; eventIdx < 3; eventIdx++ {
				timestamp := baseTime.Add(time.Duration(eventIdx*10) * time.Second)
				eventType := eventTypes[eventIdx%len(eventTypes)]

				event := &models.Event{
					ID:        uuid.New().String(),
					Timestamp: timestamp.UnixNano(),
					Type:      eventType,
					Resource: models.ResourceMetadata{
						Group:     "apps",
						Version:   "v1",
						Kind:      kind,
						Namespace: ns,
						Name:      resourceName,
						UID:       resourceUID,
					},
					// Include minimal JSON data for CREATE/UPDATE events
					Data: []byte(fmt.Sprintf(
						`{"apiVersion":"apps/v1","kind":"%s","metadata":{"name":"%s","namespace":"%s"}}`,
						kind, resourceName, ns,
					)),
					DataSize: int32(len([]byte(fmt.Sprintf(
						`{"apiVersion":"apps/v1","kind":"%s","metadata":{"name":"%s","namespace":"%s"}}`,
						kind, resourceName, ns,
					)))),
				}

				events = append(events, event)
			}
		}
	}

	return events
}
