package helpers

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// SetupE2ETestShared provisions test infrastructure using a shared Spectre deployment.
// Unlike SetupE2ETest, this function does NOT deploy a new Helm release for each test.
// Instead, it:
//   - Retrieves the shared standard deployment from the registry
//   - Creates a unique test namespace for the test's own resources
//   - Creates a port-forward to the shared Spectre deployment
//   - Returns a TestContext configured to use the shared deployment
//
// Each test still gets full isolation through unique namespaces for their test resources,
// but they share the same Spectre instance for observability and API access.
//
// Cleanup behavior:
//   - Deletes the test namespace (async, no waiting)
//   - Stops the port-forward
//   - Does NOT uninstall the Helm release (shared across tests)
func SetupE2ETestShared(t *testing.T) *TestContext {
	t.Helper()
	return setupWithSharedDeployment(t, "standard")
}

// SetupE2ETestSharedFlux provisions test infrastructure using a shared Spectre deployment
// with Flux CRDs pre-installed. This is for tests that need Flux resources (HelmRelease,
// Kustomization, etc.) to be available.
//
// Flux CRDs are installed once in TestMain, and all Flux tests share the same Spectre
// instance that watches Flux resources.
func SetupE2ETestSharedFlux(t *testing.T) *TestContext {
	t.Helper()
	return setupWithSharedDeployment(t, "flux")
}

// SetupE2ETestSharedMCP provisions test infrastructure using a shared Spectre deployment
// with MCP server enabled. This is for MCP tests that need to interact with the MCP API.
//
// The shared MCP deployment is created once in TestMain with MCP server integrated on port 8080.
// Each test still gets its own namespace for deploying test resources (failing pods, etc.)
// but connects to the shared Spectre MCP endpoint for queries.
func SetupE2ETestSharedMCP(t *testing.T) *TestContext {
	t.Helper()
	return setupWithSharedDeployment(t, "mcp")
}

// setupWithSharedDeployment is the internal implementation for shared deployment setup.
func setupWithSharedDeployment(t *testing.T, deploymentKey string) *TestContext {
	t.Helper()
	setupStartTime := time.Now()

	// Get shared deployment from registry
	sharedDep, err := GetSharedDeployment(deploymentKey)
	if err != nil {
		t.Fatalf("failed to get shared deployment %q: %v", deploymentKey, err)
	}

	// Create unique namespace for this test's resources
	// The test will create its own resources (pods, deployments, etc.) in this namespace
	// but will connect to the shared Spectre deployment for observability
	testNamespace := fmt.Sprintf("test-%s-%06d",
		sanitizeName(t.Name()),
		time.Now().UnixNano()%1_000_000)

	ctx := &TestContext{
		t:                  t,
		Cluster:            sharedDep.Cluster,
		Namespace:          testNamespace, // Test's own namespace for resources
		ReleaseName:        sharedDep.ReleaseName,
		IsSharedDeployment: true,
		SharedDeployment:   sharedDep,
	}

	// Setup cleanup function for shared deployment usage
	// Key difference: We DON'T uninstall Helm, and we delete namespace async
	ctx.cleanupFn = func() {
		if ctx.APIClient != nil {
			if err := ctx.APIClient.Close(); err != nil {
				t.Logf("Warning: failed to close API client: %v", err)
			}
		}
		if ctx.PortForward != nil {
			if err := ctx.PortForward.Stop(); err != nil {
				t.Logf("Warning: failed to stop port-forward: %v", err)
			}
		}

		// Extract audit log from the shared deployment's pod
		if ctx.K8sClient != nil {
			extractCtx, extractCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer extractCancel()

			// Find the Spectre pod in the SHARED deployment namespace
			pods, err := ctx.K8sClient.ListPods(extractCtx, sharedDep.Namespace,
				fmt.Sprintf("app.kubernetes.io/instance=%s", sharedDep.ReleaseName))
			if err == nil && len(pods.Items) > 0 {
				pod := pods.Items[0]
				auditLogPath := "/tmp/audit-logs/audit.jsonl"
				containerName := "spectre"

				testName := sanitizeName(t.Name())

				// Get repo root to ensure we write to .tests/ at repo root
				repoRoot, repoErr := detectRepoRoot()
				var localPath string
				if repoErr != nil {
					t.Logf("Warning: failed to detect repo root, using relative path: %v", repoErr)
					localPath = filepath.Join(".tests", fmt.Sprintf("%s.jsonl", testName))
				} else {
					localPath = filepath.Join(repoRoot, ".tests", fmt.Sprintf("%s.jsonl", testName))
				}

				// Extract audit log
				if err := ctx.K8sClient.ExtractAuditLog(extractCtx, sharedDep.Cluster.GetContext(),
					sharedDep.Namespace, pod.Name, containerName, auditLogPath, localPath); err != nil {
					t.Logf("Warning: failed to extract audit log: %v", err)
				}
			}
		}

		// Delete test namespace (async - no waiting)
		// This is much faster than the synchronous delete with 90s wait
		deleteCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if ctx.K8sClient != nil {
			// Delete namespace (Kubernetes will handle cascading deletion)
			if err := ctx.K8sClient.DeleteNamespace(deleteCtx, testNamespace); err != nil {
				t.Logf("Warning: failed to delete test namespace %s: %v", testNamespace, err)
			} else {
				t.Logf("✓ Test namespace deletion initiated: %s (not waiting)", testNamespace)
			}
		}

		// NOTE: We do NOT uninstall the Helm release - it's shared!
	}

	// Setup K8s client
	k8sClient, err := NewK8sClient(t, sharedDep.Cluster.GetContext())
	if err != nil {
		ctx.Cleanup()
		t.Fatalf("failed to create Kubernetes client: %v", err)
	}
	ctx.K8sClient = k8sClient

	setupCtx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()

	// Create test namespace for this test's resources
	t.Logf("Creating test namespace: %s (shared Spectre in %s)", testNamespace, sharedDep.Namespace)
	if err := k8sClient.CreateNamespace(setupCtx, testNamespace); err != nil {
		ctx.Cleanup()
		t.Fatalf("failed to create test namespace: %v", err)
	}

	// Setup port forwarding to the SHARED deployment
	// Note: We forward to the shared deployment's namespace, not the test namespace
	serviceName := fmt.Sprintf("%s-spectre", sharedDep.ReleaseName)
	portForwarder, err := NewPortForwarder(t, sharedDep.Cluster.GetContext(),
		sharedDep.Namespace, // ← Forward to shared deployment namespace
		serviceName, defaultServicePort)
	if err != nil {
		ctx.Cleanup()
		t.Fatalf("failed to create port-forward to shared deployment: %v", err)
	}
	if err := portForwarder.WaitForReady(30 * time.Second); err != nil {
		ctx.Cleanup()
		t.Fatalf("shared deployment not reachable via port-forward: %v", err)
	}

	ctx.PortForward = portForwarder
	ctx.APIClient = NewAPIClient(t, portForwarder.GetURL())

	t.Cleanup(ctx.Cleanup)
	t.Logf("✓ Test environment ready (shared deployment, test namespace: %s, setup took %v)",
		testNamespace, time.Since(setupStartTime))
	return ctx
}

// DeploySharedDeployment creates a shared Spectre deployment in TestMain.
// This deployment will be reused across multiple tests.
//
// Parameters:
//   - t: Testing context (use &testing.T{} in TestMain)
//   - cluster: The Kind cluster to deploy to
//   - namespace: Persistent namespace for the shared deployment
//   - releaseName: Helm release name
//   - preDeployFn: Optional function to run before deploying (e.g., install Flux)
//
// Returns a SharedDeployment that should be registered in the deployment registry.
func DeploySharedDeployment(
	t *testing.T,
	cluster *TestCluster,
	namespace string,
	releaseName string,
	preDeployFn PreDeployFunc,
) (*SharedDeployment, error) {
	return DeploySharedDeploymentWithValues(t, cluster, namespace, releaseName, preDeployFn, nil)
}

// DeploySharedDeploymentWithValues creates a shared Spectre deployment with custom Helm value overrides.
// This is useful for enabling features like MCP on shared deployments.
//
// Parameters:
//   - t: Testing context (use &testing.T{} in TestMain)
//   - cluster: The Kind cluster to deploy to
//   - namespace: Persistent namespace for the shared deployment
//   - releaseName: Helm release name
//   - preDeployFn: Optional function to run before deploying (e.g., install Flux)
//   - valueOverrides: Optional Helm value overrides (e.g., enable MCP)
//
// Returns a SharedDeployment that should be registered in the deployment registry.
func DeploySharedDeploymentWithValues(
	t *testing.T,
	cluster *TestCluster,
	namespace string,
	releaseName string,
	preDeployFn PreDeployFunc,
	valueOverrides map[string]interface{},
) (*SharedDeployment, error) {
	t.Helper()

	deployStartTime := time.Now()
	t.Logf("Deploying shared Spectre: %s in namespace %s", releaseName, namespace)

	// Setup K8s client
	k8sClient, err := NewK8sClient(t, cluster.GetContext())
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	setupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create persistent namespace for shared deployment
	t.Logf("Creating shared deployment namespace: %s", namespace)
	if err := k8sClient.CreateNamespace(setupCtx, namespace); err != nil {
		return nil, fmt.Errorf("failed to create shared namespace: %w", err)
	}

	// Load Helm values from cached values (loaded in TestMain)
	values, _, err := getCachedHelmValues()
	if err != nil {
		return nil, fmt.Errorf("failed to get cached Helm values: %w", err)
	}

	// Apply value overrides if provided
	if valueOverrides != nil {
		for k, v := range valueOverrides {
			values[k] = v
		}
	}

	// Set the namespace in values
	values["namespace"] = namespace

	// Enable audit log for shared deployment
	auditLogPath := "/tmp/audit-logs/audit.jsonl"
	if extraVolumes, ok := values["extraVolumes"].([]interface{}); ok {
		values["extraVolumes"] = append(extraVolumes, map[string]interface{}{
			"name":     "audit-logs",
			"emptyDir": map[string]interface{}{},
		})
	} else {
		values["extraVolumes"] = []interface{}{
			map[string]interface{}{
				"name":     "audit-logs",
				"emptyDir": map[string]interface{}{},
			},
		}
	}

	if extraVolumeMounts, ok := values["extraVolumeMounts"].([]interface{}); ok {
		values["extraVolumeMounts"] = append(extraVolumeMounts, map[string]interface{}{
			"name":      "audit-logs",
			"mountPath": "/tmp/audit-logs",
		})
	} else {
		values["extraVolumeMounts"] = []interface{}{
			map[string]interface{}{
				"name":      "audit-logs",
				"mountPath": "/tmp/audit-logs",
			},
		}
	}

	if extraArgs, ok := values["extraArgs"].([]interface{}); ok {
		values["extraArgs"] = append(extraArgs, fmt.Sprintf("--audit-log=%s", auditLogPath))
	} else {
		values["extraArgs"] = []interface{}{
			fmt.Sprintf("--audit-log=%s", auditLogPath),
		}
	}

	// Run pre-deploy function if provided (e.g., install Flux CRDs)
	if preDeployFn != nil {
		t.Logf("Running pre-deploy function for shared deployment...")
		if err := preDeployFn(k8sClient, cluster.GetContext()); err != nil {
			return nil, fmt.Errorf("pre-deploy function failed: %w", err)
		}
	}

	// Deploy Helm release in shared namespace
	if err := ensureHelmRelease(t, cluster.GetContext(), namespace, releaseName, values); err != nil {
		return nil, fmt.Errorf("failed to deploy shared Helm release: %w", err)
	}

	// Wait for deployment to be ready
	t.Logf("Waiting for shared deployment to be ready...")
	if err := WaitForAppReady(setupCtx, k8sClient, namespace, releaseName); err != nil {
		return nil, fmt.Errorf("shared deployment not ready: %w", err)
	}

	t.Logf("✓ Shared deployment ready: %s (took %v)", releaseName, time.Since(deployStartTime))

	return &SharedDeployment{
		Name:        releaseName,
		Namespace:   namespace,
		ReleaseName: releaseName,
		Cluster:     cluster,
	}, nil
}

// CleanupSharedDeployment removes a shared Spectre deployment.
// This is typically called in TestMain after all tests complete.
func CleanupSharedDeployment(t *testing.T, dep *SharedDeployment) error {
	if dep == nil {
		return nil
	}

	t.Logf("Cleaning up shared deployment: %s in namespace %s", dep.ReleaseName, dep.Namespace)

	// Uninstall Helm release
	if err := uninstallHelmRelease(t, dep.Cluster.GetContext(), dep.Namespace, dep.ReleaseName); err != nil {
		t.Logf("Warning: failed to uninstall shared Helm release: %v", err)
	}

	// Delete namespace
	k8sClient, err := NewK8sClient(t, dep.Cluster.GetContext())
	if err != nil {
		return fmt.Errorf("failed to create K8s client for cleanup: %w", err)
	}

	deleteCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := k8sClient.DeleteNamespace(deleteCtx, dep.Namespace); err != nil {
		t.Logf("Warning: failed to delete shared namespace: %v", err)
	}

	t.Logf("✓ Shared deployment cleanup initiated: %s", dep.ReleaseName)
	return nil
}
