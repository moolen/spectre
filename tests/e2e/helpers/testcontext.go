package helpers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"gopkg.in/yaml.v3"
)

const (
	auditLogPath          = "/tmp/audit-logs/audit.jsonl"
	containerNameSpectre  = "spectre"
	defaultNamespace      = "monitoring"
	defaultServicePort    = 8080
	helmValuesFixturePath = "tests/e2e/fixtures/helm-values-test.yaml"
	chartDirectory        = "chart"
)

var (
	// dockerBuildMutex ensures only one Docker build happens at a time
	dockerBuildMutex sync.Mutex
	// builtImages tracks which images have been built to avoid rebuilding
	builtImages = make(map[string]bool)
	// builtImagesMutex protects access to builtImages map
	builtImagesMutex sync.RWMutex

	// cachedHelmValues stores Helm values loaded once in TestMain
	cachedHelmValues  map[string]interface{}
	cachedImageRef    string
	cachedValuesMutex sync.RWMutex
	helmValuesLoaded  bool
)

// TestContext bundles shared infrastructure needed by scenario tests.
type TestContext struct {
	t           *testing.T
	Cluster     *TestCluster
	K8sClient   *K8sClient
	APIClient   *APIClient
	PortForward *PortForwarder

	ReleaseName string
	Namespace   string

	// IsSharedDeployment indicates if this test is using a shared Spectre deployment
	// (true) or an isolated deployment (false). When true, cleanup will NOT uninstall
	// the Helm release.
	IsSharedDeployment bool

	// SharedDeployment holds reference to the shared deployment if IsSharedDeployment=true.
	// This is used to extract audit logs from the shared deployment's pod.
	SharedDeployment *SharedDeployment

	cleanupOnce sync.Once
	cleanupFn   func()
}

// Cleanup releases all resources created for the test context.
func (tc *TestContext) Cleanup() {
	tc.cleanupOnce.Do(func() {
		if tc.cleanupFn != nil {
			tc.cleanupFn()
		}
	})
}

// ReconnectPortForward stops the existing port-forward and creates a new one
// to a new pod. This is useful after pod restarts when the original port-forward
// connection is lost. It also updates the APIClient with the new URL.
func (tc *TestContext) ReconnectPortForward() error {
	tc.t.Helper()

	// Stop the old port-forward if it exists
	if tc.PortForward != nil {
		if err := tc.PortForward.Stop(); err != nil {
			tc.t.Logf("Warning: failed to stop old port-forward: %v", err)
		}
	}

	// Determine the correct namespace and service name based on deployment type
	namespace := tc.Namespace
	serviceName := fmt.Sprintf("%s-spectre", tc.ReleaseName)

	if tc.IsSharedDeployment && tc.SharedDeployment != nil {
		// Using shared deployment - reconnect to shared namespace
		namespace = tc.SharedDeployment.Namespace
		serviceName = fmt.Sprintf("%s-spectre", tc.SharedDeployment.ReleaseName)
	}

	// Create new port-forward
	portForwarder, err := NewPortForwarder(tc.t, tc.Cluster.GetContext(), namespace, serviceName, defaultServicePort)
	if err != nil {
		return fmt.Errorf("failed to create new port-forward: %w", err)
	}
	if err := portForwarder.WaitForReady(30 * time.Second); err != nil {
		return fmt.Errorf("service not reachable via new port-forward: %w", err)
	}

	tc.PortForward = portForwarder
	tc.APIClient = NewAPIClient(tc.t, portForwarder.GetURL())

	tc.t.Logf("✓ Port-forward reconnected to new pod")
	return nil
}

// SetupE2ETestWithValuesFile provisions test infrastructure using a custom Helm values file.
// This is useful for tests that need specific configurations (e.g., minimal watcher config).
func SetupE2ETestWithValuesFile(t *testing.T, valuesFilePath string) *TestContext {
	t.Helper()
	return setupE2ETestWithCustomValues(t, valuesFilePath, nil)
}

// SetupE2ETest provisions test infrastructure using the shared Kind cluster.
// Each test gets its own unique namespace for isolation.
func SetupE2ETest(t *testing.T) *TestContext {
	t.Helper()
	return setupE2ETestWithCustomValues(t, helmValuesFixturePath, nil)
}

// SetupE2ETestWithFlux provisions test infrastructure with Flux installed BEFORE Spectre.
// This is required for tests that use Flux CRDs (HelmRelease, Kustomization, etc.)
// because Spectre's watcher needs the CRDs to exist when it starts.
func SetupE2ETestWithFlux(t *testing.T) *TestContext {
	t.Helper()
	preDeployFn := func(k8sClient *K8sClient, kubeContext string) error {
		return EnsureFluxInstalled(t, k8sClient, kubeContext)
	}
	return setupE2ETestWithCustomValues(t, helmValuesFixturePath, preDeployFn)
}

// PreDeployFunc is a function that runs before Spectre is deployed.
// It can be used to install dependencies like Flux CRDs.
type PreDeployFunc func(k8sClient *K8sClient, kubeContext string) error

// setupE2ETestWithCustomValues is the internal implementation that accepts a custom values file path.
// The optional preDeployFn is called after the K8s client is created but before Spectre is deployed.
func setupE2ETestWithCustomValues(t *testing.T, valuesFilePath string, preDeployFn PreDeployFunc) *TestContext {
	t.Helper()
	setupStartTime := time.Now()

	// Get shared cluster from package-level variable
	// This will be set by TestMain in shared_cluster.go
	sharedCluster := getSharedCluster(t)

	// Create unique namespace for this test
	namespace := fmt.Sprintf("test-%s-%06d",
		sanitizeName(t.Name()),
		time.Now().UnixNano()%1_000_000)

	releaseName := buildReleaseName(namespace)

	ctx := &TestContext{
		t:           t,
		Cluster:     sharedCluster,
		Namespace:   namespace,
		ReleaseName: releaseName,
	}

	// Cleanup: Delete namespace (NOT cluster!)
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

		// Extract audit log before cleanup
		if ctx.K8sClient != nil {
			extractCtx, extractCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer extractCancel()

			// Find the Spectre pod
			pods, err := ctx.K8sClient.ListPods(extractCtx, namespace, fmt.Sprintf("app.kubernetes.io/instance=%s", releaseName))
			if err == nil && len(pods.Items) > 0 {
				// Use the first pod (typically there's only one)
				pod := pods.Items[0]
				containerName := containerNameSpectre

				// Create test name for file (sanitize test name)
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
				if err := ctx.K8sClient.ExtractAuditLog(extractCtx, sharedCluster.GetContext(), namespace, pod.Name, containerName, auditLogPath, localPath); err != nil {
					t.Logf("Warning: failed to extract audit log: %v", err)
				}
			}
		}

		// Delete namespace (cascading delete of all resources)
		deleteCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if ctx.K8sClient != nil {
			// Uninstall Helm release first
			if err := uninstallHelmRelease(t, sharedCluster.GetContext(), namespace, releaseName); err != nil {
				t.Logf("Warning: failed to uninstall Helm release: %v", err)
			}

			// Delete namespace
			if err := ctx.K8sClient.DeleteNamespace(deleteCtx, namespace); err != nil {
				t.Logf("Warning: failed to delete namespace: %v", err)
			} else {
				// Wait for namespace deletion
				if err := ctx.K8sClient.WaitForNamespaceDeleted(deleteCtx, namespace, 90*time.Second); err != nil {
					t.Logf("Warning: timeout waiting for namespace deletion: %v", err)
				} else {
					t.Logf("✓ Test namespace cleaned up: %s", namespace)
				}
			}
		}
	}

	// Setup K8s client
	k8sClient, err := NewK8sClient(t, sharedCluster.GetContext())
	if err != nil {
		ctx.Cleanup()
		t.Fatalf("failed to create Kubernetes client: %v", err)
	}
	ctx.K8sClient = k8sClient

	setupCtx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()

	// Create test namespace
	t.Logf("Creating test namespace: %s", namespace)
	if err := k8sClient.CreateNamespace(setupCtx, namespace); err != nil {
		ctx.Cleanup()
		t.Fatalf("failed to create namespace: %v", err)
	}

	// Load Helm values from the specified file
	var values map[string]interface{}

	// Use cached values only if using default path
	if valuesFilePath == helmValuesFixturePath {
		var loadErr error
		values, _, loadErr = getCachedHelmValues()
		if loadErr != nil {
			// Fallback to loading if not cached
			values, _, loadErr = loadHelmValues()
			if loadErr != nil {
				ctx.Cleanup()
				t.Fatalf("failed to load Helm values: %v", loadErr)
			}
		}
	} else {
		// Load custom values file
		var loadErr error
		values, _, loadErr = loadHelmValuesFromFile(valuesFilePath)
		if loadErr != nil {
			ctx.Cleanup()
			t.Fatalf("failed to load custom Helm values from %s: %v", valuesFilePath, loadErr)
		}
	}

	// Set the namespace in values to match the test namespace
	values["namespace"] = namespace

	// Enable audit log for e2e tests
	// Create an emptyDir volume for audit logs
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

	// Add volume mount for audit logs
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

	// Add audit log flag to extraArgs
	if extraArgs, ok := values["extraArgs"].([]interface{}); ok {
		values["extraArgs"] = append(extraArgs, fmt.Sprintf("--audit-log=%s", auditLogPath))
	} else {
		values["extraArgs"] = []interface{}{
			fmt.Sprintf("--audit-log=%s", auditLogPath),
		}
	}

	// Run pre-deploy function if provided (e.g., install Flux CRDs)
	if preDeployFn != nil {
		t.Logf("Running pre-deploy function...")
		if err := preDeployFn(k8sClient, sharedCluster.GetContext()); err != nil {
			ctx.Cleanup()
			t.Fatalf("pre-deploy function failed: %v", err)
		}
	}

	// Deploy Helm release in test namespace
	t.Logf("Deploying Helm release: %s in namespace %s", releaseName, namespace)
	if err := ensureHelmRelease(t, sharedCluster.GetContext(), namespace, releaseName, values); err != nil {
		ctx.Cleanup()
		t.Fatalf("failed to deploy: %v", err)
	}

	// Wait for deployment
	t.Logf("Waiting for app to be ready in namespace %s", namespace)
	if err := WaitForAppReady(setupCtx, k8sClient, namespace, releaseName); err != nil {
		ctx.Cleanup()
		t.Fatalf("app deployment not ready: %v", err)
	}

	// Setup port forwarding
	serviceName := fmt.Sprintf("%s-spectre", releaseName)
	portForwarder, err := NewPortForwarder(t, sharedCluster.GetContext(), namespace, serviceName, defaultServicePort)
	if err != nil {
		ctx.Cleanup()
		t.Fatalf("failed to create port-forward: %v", err)
	}
	if err := portForwarder.WaitForReady(30 * time.Second); err != nil {
		ctx.Cleanup()
		t.Fatalf("service not reachable via port-forward: %v", err)
	}

	ctx.PortForward = portForwarder
	ctx.APIClient = NewAPIClient(t, portForwarder.GetURL())

	t.Cleanup(ctx.Cleanup)
	t.Logf("✓ Test environment ready in namespace: %s (total setup took %v)", namespace, time.Since(setupStartTime))
	return ctx
}

func UpdateHelmRelease(tc *TestContext, overrides map[string]interface{}) error {
	values, _, err := loadHelmValues()
	if err != nil {
		return fmt.Errorf("failed to load Helm values: %w", err)
	}
	values = mergeMaps(values, overrides)

	// Inject the namespace to match the test namespace
	values["namespace"] = tc.Namespace

	if err := ensureHelmRelease(tc.t, tc.Cluster.GetContext(), tc.Namespace, tc.ReleaseName, values); err != nil {
		return fmt.Errorf("failed to deploy: %w", err)
	}
	if err := WaitForAppReady(tc.t.Context(), tc.K8sClient, tc.Namespace, tc.ReleaseName); err != nil {
		return fmt.Errorf("deployment not ready: %w", err)
	}
	return nil
}

func mergeMaps(maps ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

func ensureHelmRelease(t *testing.T, context, namespace, releaseName string, values map[string]interface{}) error {
	helm, err := NewHelmDeployer(t, context, namespace)
	if err != nil {
		return err
	}

	chartPath, err := repoPath(chartDirectory)
	if err != nil {
		return err
	}

	return helm.InstallOrUpgrade(releaseName, chartPath, values)
}

// LoadHelmValues loads the test Helm values file and returns the values map and image reference
func LoadHelmValues() (map[string]interface{}, string, error) {
	return loadHelmValues()
}

func loadHelmValues() (map[string]interface{}, string, error) {
	valuesPath, err := repoPath(helmValuesFixturePath)
	if err != nil {
		return nil, "", err
	}

	data, err := os.ReadFile(valuesPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read Helm values file: %w", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal Helm values: %w", err)
	}

	imageRef := extractImageReference(values)
	return values, imageRef, nil
}

// loadHelmValuesFromFile loads a Helm values file from a custom path
func loadHelmValuesFromFile(relativePath string) (map[string]interface{}, string, error) {
	valuesPath, err := repoPath(relativePath)
	if err != nil {
		return nil, "", err
	}

	data, err := os.ReadFile(valuesPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read Helm values file: %w", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal Helm values: %w", err)
	}

	imageRef := extractImageReference(values)
	return values, imageRef, nil
}

func extractImageReference(values map[string]interface{}) string {
	repo := containerNameSpectre
	tag := "latest"

	if imageSection, ok := values["image"].(map[string]interface{}); ok {
		if r, ok := imageSection["repository"].(string); ok && r != "" {
			repo = r
		}
		if t, ok := imageSection["tag"].(string); ok && t != "" {
			tag = t
		}
	}

	return fmt.Sprintf("%s:%s", repo, tag)
}

// BuildAndLoadTestImage builds and loads a Docker image into the Kind cluster
func BuildAndLoadTestImage(t *testing.T, clusterName, imageRef string) error {
	return buildAndLoadTestImage(t, clusterName, imageRef)
}

func buildAndLoadTestImage(t *testing.T, clusterName, imageRef string) error {
	root, err := detectRepoRoot()
	if err != nil {
		return err
	}

	// Use mutex to ensure only one Docker build happens at a time across all tests
	dockerBuildMutex.Lock()
	defer dockerBuildMutex.Unlock()

	// Check if image was already built in this test run
	builtImagesMutex.RLock()
	alreadyBuilt := builtImages[imageRef]
	builtImagesMutex.RUnlock()

	if !alreadyBuilt {
		t.Logf("Building Docker image %s", imageRef)
		buildCmd := exec.Command("docker", "build", "-t", imageRef, root)
		if err := runCommand(buildCmd); err != nil {
			return err
		}

		// Mark image as built
		builtImagesMutex.Lock()
		builtImages[imageRef] = true
		builtImagesMutex.Unlock()
	} else {
		t.Logf("Reusing already built Docker image %s", imageRef)
	}

	t.Logf("Loading Docker image %s into Kind cluster %s", imageRef, clusterName)
	loadCmd := exec.Command("kind", "load", "docker-image", "--name", clusterName, imageRef)
	if err := runCommand(loadCmd); err != nil {
		return err
	}

	return nil
}

func runCommand(cmd *exec.Cmd) error {
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command %s failed: %w\n%s", strings.Join(cmd.Args, " "), err, string(output))
	}
	return nil
}

func WaitForAppReady(ctx context.Context, client *K8sClient, namespace, releaseName string) error {
	labelSelector := fmt.Sprintf("app.kubernetes.io/instance=%s", releaseName)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for pods to be ready: %w", ctx.Err())
		case <-ticker.C:
			pods, err := client.ListPods(ctx, namespace, labelSelector)
			if err != nil {
				continue
			}
			if len(pods.Items) == 0 {
				continue
			}
			for i := range pods.Items {
				if isPodReady(&pods.Items[i]) {
					return nil
				}
			}
		}
	}
}

func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// RepoPath returns the absolute path to a file relative to the repository root
func RepoPath(relative string) (string, error) {
	return repoPath(relative)
}

func repoPath(relative string) (string, error) {
	root, err := detectRepoRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, relative), nil
}

var (
	repoRootOnce sync.Once
	repoRootPath string
	repoRootErr  error
)

func DetectRepoRoot() (string, error) {
	return detectRepoRoot()
}

func detectRepoRoot() (string, error) {
	repoRootOnce.Do(func() {
		wd, err := os.Getwd()
		if err != nil {
			repoRootErr = fmt.Errorf("failed to determine working directory: %w", err)
			return
		}

		current := wd
		for {
			if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
				repoRootPath = current
				return
			}
			parent := filepath.Dir(current)
			if parent == current {
				repoRootErr = fmt.Errorf("failed to locate go.mod from %s", wd)
				return
			}
			current = parent
		}
	})

	return repoRootPath, repoRootErr
}

var sanitizeRegexp = regexp.MustCompile(`[^a-zA-Z0-9-]+`)

func buildReleaseName(clusterName string) string {
	return clusterName
}

func sanitizeName(input string) string {
	name := sanitizeRegexp.ReplaceAllString(strings.ToLower(input), "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "scenario"
	}
	if len(name) > 40 {
		return name[:40]
	}
	return name
}

// SetCachedHelmValues stores Helm values for reuse across tests
func SetCachedHelmValues(values map[string]interface{}, imageRef string) {
	cachedValuesMutex.Lock()
	defer cachedValuesMutex.Unlock()
	cachedHelmValues = values
	cachedImageRef = imageRef
	helmValuesLoaded = true
}

// getCachedHelmValues retrieves cached Helm values
func getCachedHelmValues() (map[string]interface{}, string, error) {
	cachedValuesMutex.RLock()
	defer cachedValuesMutex.RUnlock()

	if !helmValuesLoaded {
		return nil, "", fmt.Errorf("helm values not cached")
	}

	// Return a copy to avoid concurrent modifications
	valuesCopy := make(map[string]interface{})
	for k, v := range cachedHelmValues {
		valuesCopy[k] = v
	}

	return valuesCopy, cachedImageRef, nil
}

// getSharedCluster retrieves the shared cluster or fails the test
func getSharedCluster(t *testing.T) *TestCluster {
	t.Helper()

	if sharedClusterInstance == nil {
		t.Fatal("Shared cluster not initialized. TestMain should have called SetSharedCluster(). " +
			"Make sure you're running tests via 'go test' and not individually.")
	}

	return sharedClusterInstance
}

// sharedClusterInstance is set by the e2e package's TestMain
var sharedClusterInstance *TestCluster

// SetSharedCluster sets the shared cluster instance (called by TestMain)
func SetSharedCluster(cluster *TestCluster) {
	sharedClusterInstance = cluster
}

// uninstallHelmRelease removes a Helm release
func uninstallHelmRelease(t *testing.T, context, namespace, releaseName string) error {
	t.Helper()

	cmd := exec.Command("helm", "uninstall", releaseName,
		"--kube-context", context,
		"--namespace", namespace,
		"--wait",
		"--timeout", "2m",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Log but don't fail if release doesn't exist
		if !strings.Contains(string(output), "not found") {
			return fmt.Errorf("helm uninstall failed: %w\nOutput: %s", err, output)
		}
	}

	return nil
}
