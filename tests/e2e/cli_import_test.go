package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestCLIImportOnStartup validates the CLI import functionality:
// 1. Create a Kind cluster and namespace
// 2. Generate test JSON event data
// 3. Create a ConfigMap containing the JSON events
// 4. Deploy Spectre with deployment patch to mount ConfigMap and add --import flag
// 5. Verify the imported data is accessible via search and metadata APIs
func TestCLIImportOnStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Step 1: Generate test JSON event data
	t.Log("Step 1: Generating test JSON event data")
	now := time.Now()
	testNamespaces := []string{"cli-import-1", "cli-import-2"}

	events := generateTestEvents(now, testNamespaces)
	require.Greater(t, len(events), 0, "Should generate test events")
	t.Logf("Generated %d test events", len(events))

	// Convert events to JSON
	importPayload := map[string]interface{}{
		"events": events,
	}
	payloadJSON, err := json.MarshalIndent(importPayload, "", "  ")
	require.NoError(t, err, "failed to marshal events to JSON")

	t.Logf("JSON payload size: %d bytes", len(payloadJSON))

	// Step 2: Set up test cluster and namespace (but don't deploy Spectre yet)
	t.Log("Step 2: Creating Kind cluster and namespace")

	clusterName := fmt.Sprintf("cli-test-%d", time.Now().Unix()%1000000)
	testCluster, err := helpers.CreateKindCluster(t, clusterName)
	require.NoError(t, err, "failed to create Kind cluster")

	defer func() {
		if err := testCluster.Delete(); err != nil {
			t.Logf("Warning: failed to delete Kind cluster: %v", err)
		}
	}()

	k8sClient, err := helpers.NewK8sClient(t, testCluster.GetKubeConfig())
	require.NoError(t, err, "failed to create Kubernetes client")

	namespace := "monitoring"
	err = k8sClient.CreateNamespace(ctx, namespace)
	require.NoError(t, err, "failed to create namespace")

	// Step 3: Create ConfigMap with the JSON event data
	t.Log("Step 3: Creating ConfigMap with JSON event data")
	configMapName := "import-events"

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"events.json": string(payloadJSON),
		},
	}

	_, err = k8sClient.Clientset.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
	require.NoError(t, err, "failed to create ConfigMap")
	t.Logf("✓ ConfigMap created: %s/%s", namespace, configMapName)

	// Step 4: Load and customize Helm values with import configuration
	t.Log("Step 4: Preparing Helm deployment with import configuration")

	values, imageRef, err := helpers.LoadHelmValues()
	require.NoError(t, err, "failed to load Helm values")

	// Build and load image
	err = helpers.BuildAndLoadTestImage(t, testCluster.Name, imageRef)
	require.NoError(t, err, "failed to build/load image")

	// Configure import via Helm values
	importMountPath := "/import-data"

	// Add extra volumes, volume mounts, and args for import functionality
	values["extraVolumes"] = []map[string]interface{}{
		{
			"name": "import-data",
			"configMap": map[string]string{
				"name": configMapName,
			},
		},
	}

	values["extraVolumeMounts"] = []map[string]interface{}{
		{
			"name":      "import-data",
			"mountPath": importMountPath,
			"readOnly":  true,
		},
	}

	values["extraArgs"] = []string{
		fmt.Sprintf("--import=%s", importMountPath),
	}

	// Step 5: Deploy Spectre with Helm including import configuration
	t.Log("Step 5: Deploying Spectre via Helm with import configuration")

	releaseName := clusterName
	helmDeployer, err := helpers.NewHelmDeployer(t, testCluster.GetKubeConfig(), namespace)
	require.NoError(t, err, "failed to create Helm deployer")

	chartPath, err := helpers.RepoPath("chart")
	require.NoError(t, err, "failed to get chart path")

	err = helmDeployer.InstallOrUpgrade(releaseName, chartPath, values)
	require.NoError(t, err, "failed to install Helm release")

	// Step 6: Wait for deployment to be ready
	t.Log("Step 6: Waiting for Spectre to become ready")
	err = helpers.WaitForAppReady(ctx, k8sClient, namespace, releaseName)
	require.NoError(t, err, "Spectre not ready")

	// Step 7: Set up port forwarding to access the API
	t.Log("Step 7: Setting up port forwarding")

	serviceName := fmt.Sprintf("%s-spectre", releaseName)
	portForwarder, err := helpers.NewPortForwarder(t, testCluster.GetKubeConfig(), namespace, serviceName, 8080)
	require.NoError(t, err, "failed to create port-forward")
	defer portForwarder.Stop()

	err = portForwarder.WaitForReady(30 * time.Second)
	require.NoError(t, err, "service not reachable via port-forward")

	apiClient := helpers.NewAPIClient(t, portForwarder.GetURL())

	t.Log("✓ Spectre deployed and ready with import configuration")

	// Step 8: Wait a bit for import to complete (it runs on startup)
	t.Log("Step 8: Waiting for import to complete")
	time.Sleep(10 * time.Second)

	// Define time range for queries (matches the event generation time)
	startTime := now.Unix() - 300 // 5 minutes before
	endTime := now.Unix() + 300   // 5 minutes after

	// Step 9: Verify imported data is present via metadata API
	t.Log("Step 9: Verifying imported data via metadata API")

	helpers.EventuallyCondition(t, func() bool {
		metadataCtx, metadataCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer metadataCancel()

		metadata, err := apiClient.GetMetadata(metadataCtx, &startTime, &endTime)
		if err != nil {
			t.Logf("GetMetadata failed: %v", err)
			return false
		}

		// Check if our test namespaces are present
		foundNamespaces := make(map[string]bool)
		for _, ns := range metadata.Namespaces {
			for _, testNs := range testNamespaces {
				if ns == testNs {
					foundNamespaces[testNs] = true
				}
			}
		}

		if len(foundNamespaces) != len(testNamespaces) {
			t.Logf("Not all namespaces found in metadata yet. Found: %v, all namespaces: %v", foundNamespaces, metadata.Namespaces)
			return false
		}

		return true
	}, helpers.DefaultEventuallyOption)

	t.Log("✓ All test namespaces appear in metadata")

	// Step 10: Verify resources can be queried via search API
	t.Log("Step 10: Verifying resources via search API")

	resourceKinds := []string{"Deployment", "Pod", "Service", "ConfigMap"}

	for _, ns := range testNamespaces {
		for _, kind := range resourceKinds {
			helpers.EventuallyCondition(t, func() bool {
				searchCtx, searchCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer searchCancel()

				resp, err := apiClient.Search(searchCtx, startTime, endTime, ns, kind)
				if err != nil {
					t.Logf("Search failed for %s/%s: %v", ns, kind, err)
					return false
				}

				if resp.Count > 0 {
					t.Logf("Found %d %s resources in namespace %s", resp.Count, kind, ns)
					return true
				}

				t.Logf("No %s resources found yet in namespace %s", kind, ns)
				return false
			}, helpers.SlowEventuallyOption)
		}
	}

	t.Log("✓ All resource kinds queryable in all test namespaces")

	// Step 11: Verify specific resources by name
	t.Log("Step 11: Verifying specific resources by name")

	expectedResources := []struct {
		namespace string
		name      string
		kind      string
	}{
		{"cli-import-1", "test-deployment-0", "Deployment"},
		{"cli-import-1", "test-pod-1", "Pod"},
		{"cli-import-2", "test-service-2", "Service"},
		{"cli-import-2", "test-configmap-3", "ConfigMap"},
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

	// Step 12: Verify import report in logs
	t.Log("Step 12: Checking pod logs for import confirmation")

	pods, err := k8sClient.ListPods(ctx, namespace, fmt.Sprintf("app.kubernetes.io/instance=%s", releaseName))
	require.NoError(t, err, "failed to list pods")
	require.Greater(t, len(pods.Items), 0, "no pods found")

	podName := pods.Items[0].Name
	logs, err := k8sClient.Clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{}).DoRaw(ctx)
	require.NoError(t, err, "failed to get pod logs")

	logsStr := string(logs)

	// Check for import-related log messages
	assert.True(t, strings.Contains(logsStr, "Importing events from") || strings.Contains(logsStr, "Import"),
		"Pod logs should contain import-related messages")
	assert.True(t, strings.Contains(logsStr, "Import Summary") || strings.Contains(logsStr, "Import completed"),
		"Pod logs should contain import summary or completion message")

	t.Logf("✓ Pod logs confirm import execution")

	t.Log("✓ CLI import on startup test completed successfully!")
}
