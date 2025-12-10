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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gopkg.in/yaml.v3"
)

const (
	defaultNamespace      = "monitoring"
	defaultServicePort    = 8080
	helmValuesFixturePath = "tests/e2e/fixtures/helm-values-test.yaml"
	chartDirectory        = "chart"
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

	// Create a new port-forward
	serviceName := fmt.Sprintf("%s-spectre", tc.ReleaseName)
	portForwarder, err := NewPortForwarder(tc.t, tc.Cluster.GetKubeConfig(), tc.Namespace, serviceName, defaultServicePort)
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

// SetupE2ETest provisions an isolated Kind cluster, deploys the app via Helm, and
// returns a fully configured test context for scenarios to build upon.
func SetupE2ETest(t *testing.T) *TestContext {
	t.Helper()

	clusterName := newClusterName(t.Name())
	testCluster, err := CreateKindCluster(t, clusterName)
	if err != nil {
		t.Fatalf("failed to create Kind cluster: %v", err)
	}

	ctx := &TestContext{
		t:           t,
		Cluster:     testCluster,
		Namespace:   defaultNamespace,
		ReleaseName: buildReleaseName(clusterName),
	}

	ctx.cleanupFn = func() {
		if ctx.PortForward != nil {
			if err := ctx.PortForward.Stop(); err != nil {
				t.Logf("Warning: failed to stop port-forward: %v", err)
			}
		}
		if ctx.Cluster != nil {
			if err := ctx.Cluster.Delete(); err != nil {
				t.Logf("Warning: failed to delete Kind cluster: %v", err)
			}
		}
	}

	k8sClient, err := NewK8sClient(t, testCluster.GetKubeConfig())
	if err != nil {
		ctx.Cleanup()
		t.Fatalf("failed to create Kubernetes client: %v", err)
	}
	ctx.K8sClient = k8sClient

	setupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := ensureNamespace(setupCtx, k8sClient, ctx.Namespace); err != nil {
		ctx.Cleanup()
		t.Fatalf("failed to ensure namespace %s: %v", ctx.Namespace, err)
	}

	values, imageRef, err := loadHelmValues()
	if err != nil {
		ctx.Cleanup()
		t.Fatalf("failed to load Helm values: %v", err)
	}

	if err := buildAndLoadTestImage(t, testCluster.Name, imageRef); err != nil {
		ctx.Cleanup()
		t.Fatalf("failed to build/load test image: %v", err)
	}

	if err := ensureHelmRelease(t, testCluster.GetKubeConfig(), ctx.Namespace, ctx.ReleaseName, values); err != nil {
		ctx.Cleanup()
		t.Fatalf("failed to deploy: %v", err)
	}

	t.Logf("Waiting for app to be ready")
	if err := WaitForAppReady(setupCtx, k8sClient, ctx.Namespace, ctx.ReleaseName); err != nil {
		ctx.Cleanup()
		t.Fatalf("app deployment not ready: %v", err)
	}

	serviceName := fmt.Sprintf("%s-spectre", ctx.ReleaseName)
	portForwarder, err := NewPortForwarder(t, testCluster.GetKubeConfig(), ctx.Namespace, serviceName, defaultServicePort)
	if err != nil {
		ctx.Cleanup()
		t.Fatalf("failed to create port-forward: %v", err)
	}
	if err := portForwarder.WaitForReady(30 * time.Second); err != nil {
		ctx.Cleanup()
		t.Fatalf("app service not reachable via port-forward: %v", err)
	}

	ctx.PortForward = portForwarder
	ctx.APIClient = NewAPIClient(t, portForwarder.GetURL())

	t.Cleanup(ctx.Cleanup)
	return ctx
}

func UpdateHelmRelease(tc *TestContext, overrides map[string]interface{}) error {
	values, _, err := loadHelmValues()
	if err != nil {
		return fmt.Errorf("failed to load Helm values: %w", err)
	}
	values = mergeMaps(values, overrides)
	if err := ensureHelmRelease(tc.t, tc.Cluster.GetKubeConfig(), tc.Namespace, tc.ReleaseName, values); err != nil {
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

func ensureNamespace(ctx context.Context, client *K8sClient, name string) error {
	if _, err := client.Clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{}); err == nil {
		return nil
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get namespace %s: %w", name, err)
	}
	return client.CreateNamespace(ctx, name)
}

func ensureHelmRelease(t *testing.T, kubeConfig, namespace, releaseName string, values map[string]interface{}) error {
	helm, err := NewHelmDeployer(t, kubeConfig, namespace)
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

func extractImageReference(values map[string]interface{}) string {
	repo := "spectre"
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

	t.Logf("Building Docker image %s", imageRef)
	buildCmd := exec.Command("docker", "build", "-t", imageRef, root)
	if err := runCommand(buildCmd); err != nil {
		return err
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

func newClusterName(testName string) string {
	base := sanitizeName(testName)
	if len(base) > 8 {
		base = base[:8]
	}
	if base == "" {
		base = "test"
	}

	suffix := time.Now().UnixNano() % 1_000_000
	return fmt.Sprintf("%s-%06d", base, suffix)
}
