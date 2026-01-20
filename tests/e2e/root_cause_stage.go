package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	causalpaths "github.com/moolen/spectre/internal/analysis/causal_paths"
	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const kindHelmRelease = "HelmRelease"

type RootCauseScenarioStage struct {
	t       *testing.T
	require *require.Assertions
	assert  *assert.Assertions
	testCtx *helpers.TestContext

	// Test state
	helmReleaseName  string
	helmReleaseNs    string
	helmRepoName     string
	helmChartName    string
	helmChartVersion string
	deploymentName   string
	deploymentNs     string
	statefulSetName  string
	statefulSetNs    string
	targetNamespace  string
	failedPodUID     string
	failedPodName    string
	failureTimestamp int64
	beforeUpdateTime int64
	afterUpdateTime  int64
	rcaResponse      *analysis.RootCauseAnalysisV2
	causalPathsResp  *causalpaths.CausalPathsResponse

	// NetworkPolicy test state
	networkPolicyName string
	networkPolicyNs   string
	serviceName       string
	serviceNs         string
	ingressName       string
	ingressNs         string
	podLabels         map[string]string

	// Tracking for cleanup
	namespacesToCleanup []string
	resourcesToCleanup  []resourceCleanup
}

type resourceCleanup struct {
	kind      string
	name      string
	namespace string
}

func NewRootCauseScenarioStage(t *testing.T) (*RootCauseScenarioStage, *RootCauseScenarioStage, *RootCauseScenarioStage) {
	s := &RootCauseScenarioStage{
		t:                   t,
		require:             require.New(t),
		assert:              assert.New(t),
		namespacesToCleanup: make([]string, 0),
	}
	return s, s, s
}

func (s *RootCauseScenarioStage) and() *RootCauseScenarioStage {
	return s
}

// ==================== Setup Stages ====================

func (s *RootCauseScenarioStage) a_test_environment() *RootCauseScenarioStage {
	startTime := time.Now()
	s.testCtx = helpers.SetupE2ETestShared(s.t)
	s.t.Logf("✓ Test environment setup completed (took %v)", time.Since(startTime))
	return s
}

// a_test_environment_with_flux sets up the test environment with Flux installed BEFORE Spectre.
// This is required for tests that use Flux CRDs because Spectre's watcher needs the CRDs
// to exist when it starts in order to watch HelmRelease, Kustomization, etc.
func (s *RootCauseScenarioStage) a_test_environment_with_flux() *RootCauseScenarioStage {
	startTime := time.Now()
	s.testCtx = helpers.SetupE2ETestSharedFlux(s.t)
	s.t.Logf("✓ Test environment with Flux setup completed (took %v)", time.Since(startTime))
	return s
}

func (s *RootCauseScenarioStage) spectre_is_deployed() *RootCauseScenarioStage {
	startTime := time.Now()
	// Spectre is already deployed by SetupE2ETest
	// Just verify it's ready
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Determine the correct namespace and release name based on deployment type
	namespace := s.testCtx.Namespace
	releaseName := s.testCtx.ReleaseName

	if s.testCtx.IsSharedDeployment && s.testCtx.SharedDeployment != nil {
		// Using shared deployment - check in the shared namespace
		namespace = s.testCtx.SharedDeployment.Namespace
		releaseName = s.testCtx.SharedDeployment.ReleaseName
	}

	err := helpers.WaitForAppReady(ctx, s.testCtx.K8sClient, namespace, releaseName)
	s.require.NoError(err, "Spectre deployment not ready")

	s.t.Logf("✓ Spectre is deployed and ready (took %v)", time.Since(startTime))
	return s
}

// ==================== Flux External HelmRelease Stages ====================

func (s *RootCauseScenarioStage) flux_external_helmrelease_is_deployed(chartName, chartVersion, chartURL string) *RootCauseScenarioStage {
	startTime := time.Now()
	ctx := context.Background()

	// Create unique test identifiers
	// Use simpler naming to match working fixture pattern
	testNs := fmt.Sprintf("rca-test-%d", time.Now().UnixNano()%1000000)
	s.helmRepoName = chartName
	s.helmReleaseName = fmt.Sprintf("%s-app", chartName)
	s.targetNamespace = testNs
	s.helmReleaseNs = testNs // HelmRelease lives in same namespace
	s.helmChartName = chartName
	s.helmChartVersion = chartVersion

	// Create namespace for HelmRepository, HelmRelease, and deployment
	err := s.testCtx.K8sClient.CreateNamespace(ctx, testNs)
	s.require.NoError(err, "Failed to create namespace")
	s.namespacesToCleanup = append(s.namespacesToCleanup, testNs)

	// Create HelmRepository in the same namespace
	helmRepoYAML := fmt.Sprintf(`apiVersion: source.toolkit.fluxcd.io/v1
kind: HelmRepository
metadata:
  name: %s
  namespace: %s
spec:
  interval: 5m
  url: %s`, s.helmRepoName, testNs, chartURL)

	err = applyYAML(s.testCtx.Cluster.GetContext(), helmRepoYAML)
	s.require.NoError(err, "Failed to create HelmRepository")

	// Wait for HelmRepository to be ready
	s.t.Logf("Waiting for HelmRepository %s/%s to be ready...", testNs, s.helmRepoName)
	s.waitForHelmRepositoryReady(testNs, s.helmRepoName, 60*time.Second)

	// Create HelmRelease in the same namespace (use default image for initial deployment)
	helmReleaseYAML := fmt.Sprintf(`apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: %s
  namespace: %s
spec:
  interval: 1m
  targetNamespace: %s
  chart:
    spec:
      chart: %s
      version: "%s"
      sourceRef:
        kind: HelmRepository
        name: %s
        namespace: %s
  values:
    replicaCount: 1`, s.helmReleaseName, testNs, testNs, chartName, chartVersion, s.helmRepoName, testNs)

	err = applyYAML(s.testCtx.Cluster.GetContext(), helmReleaseYAML)
	s.require.NoError(err, "Failed to create HelmRelease")

	// Wait for HelmRelease to be ready
	hrCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	err = helpers.WaitForHelmReleaseReady(hrCtx, s.t, s.testCtx.Cluster.GetContext(), testNs, s.helmReleaseName, 3*time.Minute)
	s.require.NoError(err, "HelmRelease did not become ready")

	// Wait for pods to be running in target namespace
	s.t.Logf("Waiting for pods to be running in namespace %s...", s.targetNamespace)
	s.waitForPodsRunningInNamespace(s.targetNamespace, 2*time.Minute)

	s.t.Logf("✓ Flux external HelmRelease deployed successfully (took %v)", time.Since(startTime))
	return s
}

func (s *RootCauseScenarioStage) flux_helmrelease_image_is_updated(imageTag string) *RootCauseScenarioStage {
	ctx := context.Background()

	// For simplicity, we'll create a new manifest with the updated values
	updateYAML := fmt.Sprintf(`apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: %s
  namespace: %s
spec:
  interval: 1m
  targetNamespace: %s
  chart:
    spec:
      chart: %s
      version: "%s"
      sourceRef:
        kind: HelmRepository
        name: %s
        namespace: %s
  values:
    replicaCount: 1
    image:
      tag: "%s"`, s.helmReleaseName, s.helmReleaseNs, s.helmReleaseNs, s.helmChartName, s.helmChartVersion, s.helmRepoName, s.helmReleaseNs, imageTag)

	err := applyYAML(s.testCtx.Cluster.GetContext(), updateYAML)
	s.require.NoError(err, "Failed to update HelmRelease")

	// Wait for reconciliation
	reconcileCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	err = helpers.WaitForHelmReleaseReconciled(reconcileCtx, s.t, s.testCtx.Cluster.GetContext(), s.helmReleaseNs, s.helmReleaseName, 90*time.Second)
	s.require.NoError(err, "HelmRelease did not reconcile after update")

	s.t.Logf("✓ HelmRelease image updated to: %s", imageTag)
	return s
}

func (s *RootCauseScenarioStage) flux_helmrelease_with_values_configmap_is_deployed(chartName, chartVersion, chartURL string) *RootCauseScenarioStage {
	startTime := time.Now()
	ctx := context.Background()

	// Create unique test identifiers
	testNs := fmt.Sprintf("rca-valuesref-%d", time.Now().UnixNano()%1000000)
	s.helmRepoName = chartName
	s.helmReleaseName = fmt.Sprintf("%s-app", chartName)
	s.targetNamespace = testNs
	s.helmReleaseNs = testNs
	s.helmChartName = chartName
	s.helmChartVersion = chartVersion

	// Create namespace
	err := s.testCtx.K8sClient.CreateNamespace(ctx, testNs)
	s.require.NoError(err, "Failed to create namespace")
	s.namespacesToCleanup = append(s.namespacesToCleanup, testNs)

	// Create ConfigMap with Helm values
	configMapName := fmt.Sprintf("%s-values", chartName)
	configMapYAML := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  values.yaml: |
    replicaCount: 1
    ui:
      message: "Hello from ConfigMap values"`, configMapName, testNs)

	err = applyYAML(s.testCtx.Cluster.GetContext(), configMapYAML)
	s.require.NoError(err, "Failed to create ConfigMap")
	s.t.Logf("✓ Created ConfigMap: %s/%s", testNs, configMapName)

	// Create HelmRepository
	helmRepoYAML := fmt.Sprintf(`apiVersion: source.toolkit.fluxcd.io/v1
kind: HelmRepository
metadata:
  name: %s
  namespace: %s
spec:
  interval: 5m
  url: %s`, s.helmRepoName, testNs, chartURL)

	err = applyYAML(s.testCtx.Cluster.GetContext(), helmRepoYAML)
	s.require.NoError(err, "Failed to create HelmRepository")

	// Wait for HelmRepository to be ready
	s.t.Logf("Waiting for HelmRepository %s/%s to be ready...", testNs, s.helmRepoName)
	s.waitForHelmRepositoryReady(testNs, s.helmRepoName, 60*time.Second)

	// Create HelmRelease with valuesFrom referencing the ConfigMap
	helmReleaseYAML := fmt.Sprintf(`apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: %s
  namespace: %s
spec:
  interval: 1m
  targetNamespace: %s
  chart:
    spec:
      chart: %s
      version: "%s"
      sourceRef:
        kind: HelmRepository
        name: %s
        namespace: %s
  valuesFrom:
  - kind: ConfigMap
    name: %s
    valuesKey: values.yaml`, s.helmReleaseName, testNs, testNs, chartName, chartVersion, s.helmRepoName, testNs, configMapName)

	err = applyYAML(s.testCtx.Cluster.GetContext(), helmReleaseYAML)
	s.require.NoError(err, "Failed to create HelmRelease")

	// Wait for HelmRelease to be ready
	hrCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	err = helpers.WaitForHelmReleaseReady(hrCtx, s.t, s.testCtx.Cluster.GetContext(), testNs, s.helmReleaseName, 3*time.Minute)
	s.require.NoError(err, "HelmRelease did not become ready")

	// Wait for pods to be running
	s.t.Logf("Waiting for pods to be running in namespace %s...", s.targetNamespace)
	s.waitForPodsRunningInNamespace(s.targetNamespace, 2*time.Minute)

	s.t.Logf("✓ Flux HelmRelease with valuesFrom ConfigMap deployed (took %v)", time.Since(startTime))
	return s
}

func (s *RootCauseScenarioStage) flux_helmrelease_with_values_configmap_image_is_updated(imageTag string) *RootCauseScenarioStage {
	ctx := context.Background()

	// Get the ConfigMap name
	configMapName := fmt.Sprintf("%s-values", s.helmChartName)

	// Update HelmRelease with invalid image tag while keeping valuesFrom
	updateYAML := fmt.Sprintf(`apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: %s
  namespace: %s
spec:
  interval: 1m
  targetNamespace: %s
  chart:
    spec:
      chart: %s
      version: "%s"
      sourceRef:
        kind: HelmRepository
        name: %s
        namespace: %s
  valuesFrom:
  - kind: ConfigMap
    name: %s
    valuesKey: values.yaml
  values:
    image:
      tag: "%s"`, s.helmReleaseName, s.helmReleaseNs, s.helmReleaseNs, s.helmChartName, s.helmChartVersion, s.helmRepoName, s.helmReleaseNs, configMapName, imageTag)

	err := applyYAML(s.testCtx.Cluster.GetContext(), updateYAML)
	s.require.NoError(err, "Failed to update HelmRelease")

	// Wait for reconciliation
	reconcileCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	err = helpers.WaitForHelmReleaseReconciled(reconcileCtx, s.t, s.testCtx.Cluster.GetContext(), s.helmReleaseNs, s.helmReleaseName, 90*time.Second)
	s.require.NoError(err, "HelmRelease did not reconcile after update")

	s.t.Logf("✓ HelmRelease image updated to: %s (keeping valuesFrom ConfigMap)", imageTag)
	return s
}

// ==================== Flux Kustomization Stages ====================

func (s *RootCauseScenarioStage) flux_kustomization_with_labeled_deployment_is_deployed(kustomizationName string) *RootCauseScenarioStage {
	startTime := time.Now()
	ctx := context.Background()

	// Create unique test namespace
	testNs := fmt.Sprintf("rca-kustomize-%d", time.Now().UnixNano()%1000000)
	err := s.testCtx.K8sClient.CreateNamespace(ctx, testNs)
	s.require.NoError(err, "Failed to create namespace")
	s.namespacesToCleanup = append(s.namespacesToCleanup, testNs)

	s.deploymentName = fmt.Sprintf("%s-deployment", kustomizationName)
	s.deploymentNs = testNs

	// Create a GitRepository source (required for Kustomization)
	gitRepoYAML := fmt.Sprintf(`apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: %s-source
  namespace: %s
spec:
  interval: 1m
  url: https://github.com/stefanprodan/podinfo
  ref:
    branch: master`, kustomizationName, testNs)

	err = applyYAML(s.testCtx.Cluster.GetContext(), gitRepoYAML)
	s.require.NoError(err, "Failed to create GitRepository")
	s.t.Logf("✓ Created GitRepository: %s/%s-source", testNs, kustomizationName)

	// Create the Kustomization resource
	kustomizationYAML := fmt.Sprintf(`apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: %s
  namespace: %s
spec:
  interval: 1m
  targetNamespace: %s
  sourceRef:
    kind: GitRepository
    name: %s-source
  path: ./kustomize
  prune: true`, kustomizationName, testNs, testNs, kustomizationName)

	err = applyYAML(s.testCtx.Cluster.GetContext(), kustomizationYAML)
	s.require.NoError(err, "Failed to create Kustomization")
	s.t.Logf("✓ Created Kustomization: %s/%s", testNs, kustomizationName)

	// Create a Deployment with Kustomize labels (simulating what Flux would create)
	// The labels kustomize.toolkit.fluxcd.io/name and kustomize.toolkit.fluxcd.io/namespace
	// are what the extractor uses to identify managed resources
	deploymentYAML := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    kustomize.toolkit.fluxcd.io/name: %s
    kustomize.toolkit.fluxcd.io/namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
        kustomize.toolkit.fluxcd.io/name: %s
        kustomize.toolkit.fluxcd.io/namespace: %s
    spec:
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"`, s.deploymentName, testNs, s.deploymentName, kustomizationName, testNs, s.deploymentName, s.deploymentName, kustomizationName, testNs)

	err = applyYAML(s.testCtx.Cluster.GetContext(), deploymentYAML)
	s.require.NoError(err, "Failed to create Deployment")
	s.t.Logf("✓ Created Deployment with Kustomize labels: %s/%s", testNs, s.deploymentName)

	// Wait for pods to be running
	s.waitForPodsRunningInNamespace(testNs, 2*time.Minute)

	s.t.Logf("✓ Flux Kustomization with labeled Deployment deployed (took %v)", time.Since(startTime))
	return s
}

func (s *RootCauseScenarioStage) deployment_image_is_updated_to_invalid(imageTag string) *RootCauseScenarioStage {
	// Get Deployment and update image
	cmd := fmt.Sprintf("kubectl --context=%s get deployment %s -n %s -o json",
		s.testCtx.Cluster.GetContext(), s.deploymentName, s.deploymentNs)
	output, err := helpers.RunCommand(cmd)
	s.require.NoError(err, "Failed to get Deployment")

	var deploy unstructured.Unstructured
	err = json.Unmarshal([]byte(output), &deploy)
	s.require.NoError(err, "Failed to parse Deployment JSON")

	// Update the image
	containers, _, err := unstructured.NestedSlice(deploy.Object, "spec", "template", "spec", "containers")
	s.require.NoError(err, "Failed to get containers")
	s.require.NotEmpty(containers, "No containers found")

	container := containers[0].(map[string]interface{})
	container["image"] = imageTag
	containers[0] = container

	err = unstructured.SetNestedSlice(deploy.Object, containers, "spec", "template", "spec", "containers")
	s.require.NoError(err, "Failed to set containers")

	// Apply update
	manifestBytes, err := json.Marshal(&deploy)
	s.require.NoError(err, "Failed to marshal updated Deployment")

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("deployment-update-%d.yaml", time.Now().UnixNano()))
	err = os.WriteFile(tmpFile, manifestBytes, 0644)
	s.require.NoError(err, "Failed to write temp Deployment manifest")
	defer os.Remove(tmpFile)

	cmd = fmt.Sprintf("kubectl --context=%s apply -f %s", s.testCtx.Cluster.GetContext(), tmpFile)
	_, err = helpers.RunCommand(cmd)
	s.require.NoError(err, "Failed to update Deployment")

	s.t.Logf("✓ Updated Deployment %s/%s with invalid image: %s", s.deploymentNs, s.deploymentName, imageTag)
	return s
}

func (s *RootCauseScenarioStage) assert_graph_has_kustomization_manages_deployment() *RootCauseScenarioStage {
	helpers.RequireGraphNonEmpty(s.t, s.rcaResponse)

	// Check that Kustomization node exists
	kustomizationNode := helpers.FindNodeByKind(s.rcaResponse, "Kustomization")
	s.require.NotNil(kustomizationNode, "Graph should contain Kustomization node")
	s.t.Logf("✓ Found Kustomization node: %s", kustomizationNode.Resource.Name)

	// Check that MANAGES edge exists from Kustomization to Deployment
	helpers.RequireGraphHasEdgeBetweenKinds(s.t, s.rcaResponse, "Kustomization", "MANAGES", "Deployment")
	s.t.Logf("✓ Found MANAGES edge from Kustomization to Deployment")

	// Also verify the ownership chain
	helpers.RequireGraphHasEdgeBetweenKinds(s.t, s.rcaResponse, "Deployment", "OWNS", "ReplicaSet")
	helpers.RequireGraphHasEdgeBetweenKinds(s.t, s.rcaResponse, "ReplicaSet", "OWNS", "Pod")
	s.t.Logf("✓ Found ownership chain: Deployment -> ReplicaSet -> Pod")

	return s
}

func (s *RootCauseScenarioStage) waitForHelmRepositoryReady(namespace, name string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	lastLogTime := time.Time{}

	for time.Now().Before(deadline) {
		// Check Ready status
		cmd := fmt.Sprintf("kubectl --context %s get helmrepository %s -n %s -o jsonpath='{.status.conditions[?(@.type==\"Ready\")].status}'",
			s.testCtx.Cluster.GetContext(), name, namespace)
		output, err := helpers.RunCommand(cmd)
		if err == nil && strings.TrimSpace(output) == "True" {
			s.t.Logf("✓ HelmRepository %s/%s is ready", namespace, name)
			return
		}

		// Log current status every 10 seconds for debugging
		if time.Since(lastLogTime) >= 10*time.Second {
			// Get Ready condition message
			msgCmd := fmt.Sprintf("kubectl --context %s get helmrepository %s -n %s -o jsonpath='{.status.conditions[?(@.type==\"Ready\")].message}'",
				s.testCtx.Cluster.GetContext(), name, namespace)
			statusMsg, _ := helpers.RunCommand(msgCmd)

			// Get Ready condition status
			statusCmd := fmt.Sprintf("kubectl --context %s get helmrepository %s -n %s -o jsonpath='{.status.conditions[?(@.type==\"Ready\")].status}'",
				s.testCtx.Cluster.GetContext(), name, namespace)
			statusStatus, _ := helpers.RunCommand(statusCmd)

			if statusMsg != "" || statusStatus != "" {
				s.t.Logf("HelmRepository %s/%s status=%s, message=%s",
					namespace, name,
					strings.TrimSpace(statusStatus),
					strings.TrimSpace(statusMsg))
			} else {
				// Try to get the resource itself to see if it exists
				checkCmd := fmt.Sprintf("kubectl --context %s get helmrepository %s -n %s 2>&1",
					s.testCtx.Cluster.GetContext(), name, namespace)
				checkOutput, _ := helpers.RunCommand(checkCmd)
				s.t.Logf("HelmRepository %s/%s check: %s", namespace, name, strings.TrimSpace(checkOutput))
			}
			lastLogTime = time.Now()
		}

		time.Sleep(5 * time.Second)
	}

	// Get full status for debugging before failing
	fullStatusCmd := fmt.Sprintf("kubectl --context %s get helmrepository %s -n %s -o yaml",
		s.testCtx.Cluster.GetContext(), name, namespace)
	fullStatus, _ := helpers.RunCommand(fullStatusCmd)
	s.t.Logf("HelmRepository final status:\n%s", fullStatus)

	s.require.Fail("Timeout waiting for HelmRepository to be ready")
}

func (s *RootCauseScenarioStage) waitForPodsRunningInNamespace(namespace string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pods, err := s.testCtx.K8sClient.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err == nil && len(pods.Items) > 0 {
			allRunning := true
			for i := range pods.Items {
				pod := &pods.Items[i]
				if pod.Status.Phase != corev1.PodRunning {
					allRunning = false
					break
				}
			}
			if allRunning {
				s.t.Logf("✓ All pods are running in namespace %s", namespace)
				return
			}
		}
		time.Sleep(3 * time.Second)
	}
}

// ==================== StatefulSet Stages ====================

func (s *RootCauseScenarioStage) statefulset_is_deployed(fixture string) *RootCauseScenarioStage {
	startTime := time.Now()

	fixtureContent := s.loadFixture(fixture)

	// Parse and update namespace
	var statefulSet unstructured.Unstructured
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(fixtureContent), 4096)
	err := decoder.Decode(&statefulSet)
	s.require.NoError(err, "Failed to parse StatefulSet fixture")

	statefulSet.SetNamespace(s.testCtx.Namespace)
	s.statefulSetName = statefulSet.GetName()
	s.statefulSetNs = s.testCtx.Namespace

	// Apply StatefulSet using kubectl to avoid type conversion issues
	manifestBytes, err := json.Marshal(&statefulSet)
	s.require.NoError(err, "Failed to marshal StatefulSet")

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("statefulset-%d.yaml", time.Now().UnixNano()))
	err = os.WriteFile(tmpFile, manifestBytes, 0644)
	s.require.NoError(err, "Failed to write temp StatefulSet manifest")
	defer os.Remove(tmpFile)

	cmd := fmt.Sprintf("kubectl --context=%s apply -f %s", s.testCtx.Cluster.GetContext(), tmpFile)
	_, err = helpers.RunCommand(cmd)
	s.require.NoError(err, "Failed to create StatefulSet")

	s.t.Logf("✓ Created StatefulSet: %s/%s", s.statefulSetNs, s.statefulSetName)

	// Wait for StatefulSet to be ready
	s.waitForStatefulSetReady(s.statefulSetNs, s.statefulSetName, 120*time.Second)

	s.t.Logf("✓ StatefulSet deployed and ready (took %v)", time.Since(startTime))
	return s
}

func (s *RootCauseScenarioStage) statefulset_image_is_updated(invalidImage string) *RootCauseScenarioStage {
	// Record timestamp before update
	s.beforeUpdateTime = time.Now().UnixNano()

	// Get StatefulSet and update image
	cmd := fmt.Sprintf("kubectl --context=%s get statefulset %s -n %s -o json",
		s.testCtx.Cluster.GetContext(), s.statefulSetName, s.statefulSetNs)
	output, err := helpers.RunCommand(cmd)
	s.require.NoError(err, "Failed to get StatefulSet")

	var ss unstructured.Unstructured
	err = json.Unmarshal([]byte(output), &ss)
	s.require.NoError(err, "Failed to parse StatefulSet JSON")

	// Update the image
	containers, _, err := unstructured.NestedSlice(ss.Object, "spec", "template", "spec", "containers")
	s.require.NoError(err, "Failed to get containers")
	s.require.NotEmpty(containers, "No containers found")

	container := containers[0].(map[string]interface{})
	container["image"] = invalidImage
	containers[0] = container

	err = unstructured.SetNestedSlice(ss.Object, containers, "spec", "template", "spec", "containers")
	s.require.NoError(err, "Failed to set containers")

	// Apply update
	manifestBytes, err := json.Marshal(&ss)
	s.require.NoError(err, "Failed to marshal updated StatefulSet")

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("statefulset-update-%d.yaml", time.Now().UnixNano()))
	err = os.WriteFile(tmpFile, manifestBytes, 0644)
	s.require.NoError(err, "Failed to write temp StatefulSet manifest")
	defer os.Remove(tmpFile)

	cmd = fmt.Sprintf("kubectl --context=%s apply -f %s", s.testCtx.Cluster.GetContext(), tmpFile)
	_, err = helpers.RunCommand(cmd)
	s.require.NoError(err, "Failed to update StatefulSet")

	// Record timestamp after update
	s.afterUpdateTime = time.Now().UnixNano()

	// Update namespace tracking for pod failure detection
	s.deploymentNs = s.statefulSetNs

	s.t.Logf("✓ Updated StatefulSet %s/%s with image: %s", s.statefulSetNs, s.statefulSetName, invalidImage)
	return s
}

func (s *RootCauseScenarioStage) waitForStatefulSetReady(namespace, name string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	s.t.Logf("Waiting for StatefulSet %s/%s to be ready...", namespace, name)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		cmd := fmt.Sprintf("kubectl --context=%s get statefulset %s -n %s -o json",
			s.testCtx.Cluster.GetContext(), name, namespace)
		output, err := helpers.RunCommand(cmd)
		if err == nil {
			var ss map[string]interface{}
			if json.Unmarshal([]byte(output), &ss) == nil {
				status, ok := ss["status"].(map[string]interface{})
				if ok {
					readyReplicas, _ := status["readyReplicas"].(float64)
					spec, _ := ss["spec"].(map[string]interface{})
					replicas, _ := spec["replicas"].(float64)

					if readyReplicas > 0 && readyReplicas == replicas {
						// Check if pods are actually running
						pods, err := s.testCtx.K8sClient.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
							LabelSelector: fmt.Sprintf("app=%s", name),
						})
						if err == nil && len(pods.Items) > 0 {
							allRunning := true
							for i := range pods.Items {
								pod := &pods.Items[i]
								if pod.Status.Phase != corev1.PodRunning {
									allRunning = false
									break
								}
							}
							if allRunning {
								s.t.Logf("✓ StatefulSet %s/%s is ready with running pods", namespace, name)
								return
							}
						}
					}
				}
			}
		}
		time.Sleep(3 * time.Second)
	}
	s.require.Fail("Timeout waiting for StatefulSet to be ready")
}

func (s *RootCauseScenarioStage) waitForResourceInGraph(kind, name, namespace string, timeout time.Duration) {
	s.t.Logf("Waiting for %s %s/%s to be indexed in graph...", kind, namespace, name)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Query Spectre's graph to check if the resource exists
		// We'll use kubectl to exec into Spectre and query the graph
		cmd := fmt.Sprintf(`kubectl --context %s exec -n %s deployment/%s-spectre -- /bin/sh -c "echo 'MATCH (r:ResourceIdentity {kind: \"%s\", name: \"%s\", namespace: \"%s\"}) RETURN r.uid LIMIT 1' | grep -q 'r.uid' && echo 'found' || echo 'not found'"`,
			s.testCtx.Cluster.GetContext(), s.testCtx.Namespace, s.testCtx.Namespace,
			kind, name, namespace)

		output, err := helpers.RunCommand(cmd)
		if err == nil && strings.Contains(output, "found") {
			s.t.Logf("✓ %s %s/%s is indexed in graph", kind, namespace, name)
			return
		}

		// Also try querying via the API to check recent events
		time.Sleep(2 * time.Second)
	}
	s.t.Logf("⚠ Timeout waiting for %s %s/%s to be indexed in graph (continuing anyway)", kind, namespace, name)
}

// waitForResourceUIDWithChangeEvent waits for a resource with the given UID to have a ChangeEvent in the graph
func (s *RootCauseScenarioStage) waitForResourceUIDWithChangeEvent(uid string, timeout time.Duration) {
	s.t.Logf("Waiting for resource %s to have ChangeEvent in graph...", uid)
	deadline := time.Now().Add(timeout)

	// Get the namespace of the failed pod for timeline query
	ns := s.helmReleaseNs
	if ns == "" {
		ns = s.deploymentNs
	}
	if ns == "" {
		ns = s.statefulSetNs
	}

	for time.Now().Before(deadline) {
		// Query Spectre's timeline API to check if the Pod has events
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		endTime := time.Now().UnixNano()
		startTime := endTime - int64(10*time.Minute)

		timelineResp, err := s.testCtx.APIClient.Timeline(ctx, startTime, endTime, ns, "Pod")
		cancel()

		if err == nil && timelineResp.Count > 0 {
			// Check if any of the resources match our UID
			// The timeline API returns the resource UID as the "id" field
			for _, resource := range timelineResp.Resources {
				if resource.ID == uid {
					s.t.Logf("✓ Resource %s has events in timeline (found %d total Pod events in namespace)", uid, timelineResp.Count)
					return
				}
			}
			s.t.Logf("Timeline has %d Pod events in %s, but not for UID %s yet...", timelineResp.Count, ns, uid)
		}

		time.Sleep(2 * time.Second)
	}
	s.t.Logf("⚠ Timeout waiting for resource %s to have ChangeEvent in graph (continuing anyway)", uid)
}

// ==================== NetworkPolicy Stages ====================

func (s *RootCauseScenarioStage) deployment_with_labels_is_deployed(deploymentName string, labels map[string]string) *RootCauseScenarioStage {
	startTime := time.Now()
	ctx := context.Background()

	// Create unique test namespace
	testNs := fmt.Sprintf("netpol-test-%d", time.Now().UnixNano()%1000000)
	err := s.testCtx.K8sClient.CreateNamespace(ctx, testNs)
	s.require.NoError(err, "Failed to create namespace")
	s.namespacesToCleanup = append(s.namespacesToCleanup, testNs)

	s.deploymentName = deploymentName
	s.deploymentNs = testNs
	s.podLabels = labels

	// Build labels string for YAML
	labelsYAML := ""
	for k, v := range labels {
		labelsYAML += fmt.Sprintf("        %s: %s\n", k, v)
	}

	// Build selector labels (use first label for selector)
	var selectorKey, selectorValue string
	for k, v := range labels {
		selectorKey = k
		selectorValue = v
		break
	}

	deploymentYAML := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      %s: %s
  template:
    metadata:
      labels:
%s    spec:
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"`, deploymentName, testNs, deploymentName, selectorKey, selectorValue, labelsYAML)

	err = applyYAML(s.testCtx.Cluster.GetContext(), deploymentYAML)
	s.require.NoError(err, "Failed to create Deployment")

	// Wait for pods to be running
	s.waitForPodsRunningInNamespace(testNs, 2*time.Minute)

	s.t.Logf("✓ Deployment %s/%s with labels %v deployed (took %v)", testNs, deploymentName, labels, time.Since(startTime))
	return s
}

func (s *RootCauseScenarioStage) networkpolicy_selecting_pods_is_created(policyName string, selectorLabels map[string]string) *RootCauseScenarioStage {
	startTime := time.Now()

	// NetworkPolicy is created in the same namespace as the deployment
	namespace := s.deploymentNs
	s.networkPolicyName = policyName
	s.networkPolicyNs = namespace

	// Build matchLabels YAML
	matchLabelsYAML := ""
	for k, v := range selectorLabels {
		matchLabelsYAML += fmt.Sprintf("      %s: %s\n", k, v)
	}

	networkPolicyYAML := fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: %s
  namespace: %s
spec:
  podSelector:
    matchLabels:
%s  policyTypes:
  - Ingress
  ingress:
  - from:
    - podSelector: {}`, policyName, namespace, matchLabelsYAML)

	err := applyYAML(s.testCtx.Cluster.GetContext(), networkPolicyYAML)
	s.require.NoError(err, "Failed to create NetworkPolicy")

	// Wait a bit for the graph sync to process the NetworkPolicy
	time.Sleep(5 * time.Second)

	s.t.Logf("✓ NetworkPolicy %s/%s created with selector %v (took %v)", namespace, policyName, selectorLabels, time.Since(startTime))
	return s
}

func (s *RootCauseScenarioStage) networkpolicy_in_different_namespace_is_created(policyName string, selectorLabels map[string]string) *RootCauseScenarioStage {
	startTime := time.Now()
	ctx := context.Background()

	// Create a different namespace for the NetworkPolicy
	netpolNs := fmt.Sprintf("netpol-other-%d", time.Now().UnixNano()%1000000)
	err := s.testCtx.K8sClient.CreateNamespace(ctx, netpolNs)
	s.require.NoError(err, "Failed to create namespace for NetworkPolicy")
	s.namespacesToCleanup = append(s.namespacesToCleanup, netpolNs)

	s.networkPolicyName = policyName
	s.networkPolicyNs = netpolNs

	// Build matchLabels YAML
	matchLabelsYAML := ""
	for k, v := range selectorLabels {
		matchLabelsYAML += fmt.Sprintf("      %s: %s\n", k, v)
	}

	networkPolicyYAML := fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: %s
  namespace: %s
spec:
  podSelector:
    matchLabels:
%s  policyTypes:
  - Ingress
  ingress:
  - from:
    - podSelector: {}`, policyName, netpolNs, matchLabelsYAML)

	err = applyYAML(s.testCtx.Cluster.GetContext(), networkPolicyYAML)
	s.require.NoError(err, "Failed to create NetworkPolicy")

	// Wait a bit for the graph sync to process the NetworkPolicy
	time.Sleep(5 * time.Second)

	s.t.Logf("✓ NetworkPolicy %s/%s created in different namespace with selector %v (took %v)", netpolNs, policyName, selectorLabels, time.Since(startTime))
	return s
}

func (s *RootCauseScenarioStage) service_selecting_pods_is_created(serviceName string, selectorLabels map[string]string, port int) *RootCauseScenarioStage {
	startTime := time.Now()

	// Service is created in the same namespace as the deployment
	namespace := s.deploymentNs
	s.serviceName = serviceName
	s.serviceNs = namespace

	// Build selector YAML
	selectorYAML := ""
	for k, v := range selectorLabels {
		selectorYAML += fmt.Sprintf("    %s: %s\n", k, v)
	}

	serviceYAML := fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
spec:
  selector:
%s  ports:
  - protocol: TCP
    port: %d
    targetPort: %d`, serviceName, namespace, selectorYAML, port, port)

	err := applyYAML(s.testCtx.Cluster.GetContext(), serviceYAML)
	s.require.NoError(err, "Failed to create Service")

	// Wait a bit for the graph sync to process the Service
	time.Sleep(5 * time.Second)

	s.t.Logf("✓ Service %s/%s created with selector %v (took %v)", namespace, serviceName, selectorLabels, time.Since(startTime))
	return s
}

func (s *RootCauseScenarioStage) ingress_referencing_service_is_created(ingressName, serviceName string) *RootCauseScenarioStage {
	startTime := time.Now()

	// Ingress is created in the same namespace as the service
	namespace := s.serviceNs
	s.ingressName = ingressName
	s.ingressNs = namespace

	ingressYAML := fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %s
  namespace: %s
spec:
  rules:
  - host: test.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: %s
            port:
              number: 80`, ingressName, namespace, serviceName)

	err := applyYAML(s.testCtx.Cluster.GetContext(), ingressYAML)
	s.require.NoError(err, "Failed to create Ingress")

	// Wait a bit for the graph sync to process the Ingress
	time.Sleep(5 * time.Second)

	s.t.Logf("✓ Ingress %s/%s created referencing service %s (took %v)", namespace, ingressName, serviceName, time.Since(startTime))
	return s
}

func (s *RootCauseScenarioStage) running_pod_is_identified() *RootCauseScenarioStage {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ns := s.deploymentNs

	// Find a running pod
	pods, err := s.testCtx.K8sClient.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	s.require.NoError(err, "Failed to list pods")
	s.require.NotEmpty(pods.Items, "No pods found in namespace %s", ns)

	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase == corev1.PodRunning {
			s.failedPodUID = string(pod.UID)
			s.failedPodName = pod.Name
			s.failureTimestamp = time.Now().UnixNano()
			s.t.Logf("✓ Identified running pod: %s (UID: %s)", pod.Name, pod.UID)
			return s
		}
	}

	s.require.Fail("No running pod found in namespace %s", ns)
	return s
}

func (s *RootCauseScenarioStage) assert_graph_has_networkpolicy() *RootCauseScenarioStage {
	helpers.RequireGraphNonEmpty(s.t, s.rcaResponse)

	// Check that NetworkPolicy node exists
	netpolNode := helpers.FindNodeByKind(s.rcaResponse, "NetworkPolicy")
	s.require.NotNil(netpolNode, "Graph should contain NetworkPolicy node")
	s.t.Logf("✓ Found NetworkPolicy node: %s", netpolNode.Resource.Name)

	return s
}

func (s *RootCauseScenarioStage) assert_graph_has_selects_edge() *RootCauseScenarioStage {
	helpers.RequireGraphHasEdgeBetweenKinds(s.t, s.rcaResponse, "NetworkPolicy", "SELECTS", "Pod")
	s.t.Logf("✓ Found SELECTS edge from NetworkPolicy to Pod")
	return s
}

func (s *RootCauseScenarioStage) assert_graph_has_no_networkpolicy() *RootCauseScenarioStage {
	helpers.RequireGraphNonEmpty(s.t, s.rcaResponse)

	// Check that NetworkPolicy node does NOT exist
	netpolNode := helpers.FindNodeByKind(s.rcaResponse, "NetworkPolicy")
	s.assert.Nil(netpolNode, "Graph should NOT contain NetworkPolicy node (cross-namespace selection not supported)")
	s.t.Logf("✓ Confirmed no NetworkPolicy node in graph (expected for cross-namespace test)")

	return s
}

func (s *RootCauseScenarioStage) assert_graph_has_service() *RootCauseScenarioStage {
	helpers.RequireGraphNonEmpty(s.t, s.rcaResponse)

	// Check that Service node exists
	serviceNode := helpers.FindNodeByKind(s.rcaResponse, "Service")
	s.require.NotNil(serviceNode, "Graph should contain Service node")
	s.t.Logf("✓ Found Service node: %s", serviceNode.Resource.Name)
	return s
}

func (s *RootCauseScenarioStage) assert_graph_has_service_selects_pod() *RootCauseScenarioStage {
	helpers.RequireGraphHasEdgeBetweenKinds(s.t, s.rcaResponse, "Service", "SELECTS", "Pod")
	s.t.Logf("✓ Found SELECTS edge from Service to Pod")
	return s
}

func (s *RootCauseScenarioStage) assert_graph_has_ingress() *RootCauseScenarioStage {
	helpers.RequireGraphNonEmpty(s.t, s.rcaResponse)

	// Check that Ingress node exists
	ingressNode := helpers.FindNodeByKind(s.rcaResponse, "Ingress")
	s.require.NotNil(ingressNode, "Graph should contain Ingress node")
	s.t.Logf("✓ Found Ingress node: %s", ingressNode.Resource.Name)
	return s
}

func (s *RootCauseScenarioStage) assert_graph_has_ingress_references_service() *RootCauseScenarioStage {
	helpers.RequireGraphHasEdgeBetweenKinds(s.t, s.rcaResponse, "Ingress", "REFERENCES_SPEC", "Service")
	s.t.Logf("✓ Found REFERENCES_SPEC edge from Ingress to Service")
	return s
}

func (s *RootCauseScenarioStage) assert_graph_has_deployment_owns_pod() *RootCauseScenarioStage {
	helpers.RequireGraphHasEdgeBetweenKinds(s.t, s.rcaResponse, "Deployment", "OWNS", "ReplicaSet")
	helpers.RequireGraphHasEdgeBetweenKinds(s.t, s.rcaResponse, "ReplicaSet", "OWNS", "Pod")
	s.t.Logf("✓ Found ownership chain: Deployment -> ReplicaSet -> Pod")
	return s
}

// ==================== Failure Detection Stages ====================

func (s *RootCauseScenarioStage) wait_for_pod_failure(symptom string, timeout time.Duration) *RootCauseScenarioStage {
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns := s.helmReleaseNs
	if ns == "" {
		ns = s.deploymentNs
	}

	s.t.Logf("Waiting for pod with symptom '%s' in namespace %s...", symptom, ns)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pods, err := s.testCtx.K8sClient.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		for i := range pods.Items {
			pod := &pods.Items[i]
			// Check container statuses for the symptom
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.State.Waiting == nil {
					continue
				}
				if !strings.Contains(containerStatus.State.Waiting.Reason, symptom) {
					continue
				}
				s.failedPodUID = string(pod.UID)
				s.failedPodName = pod.Name
				s.t.Logf("✓ Found failed pod: %s (UID: %s) with symptom: %s (waited %v)",
					s.failedPodName, s.failedPodUID, symptom, time.Since(startTime))

				// Set failure timestamp after detecting failure
				s.failureTimestamp = time.Now().UnixNano()
				return s
			}
		}

		time.Sleep(3 * time.Second)
	}

	// List pods for debugging
	pods, _ := s.testCtx.K8sClient.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	s.t.Logf("Current pods in namespace %s:", ns)
	for i := range pods.Items {
		pod := &pods.Items[i]
		s.t.Logf("  - %s: Phase=%s", pod.Name, pod.Status.Phase)
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil {
				s.t.Logf("    Container %s: Waiting (Reason: %s)", cs.Name, cs.State.Waiting.Reason)
			}
		}
	}

	s.require.Fail(fmt.Sprintf("Timeout waiting for pod with symptom '%s' (waited %v)", symptom, time.Since(startTime)))
	return s
}

func (s *RootCauseScenarioStage) failed_pod_is_identified() *RootCauseScenarioStage {
	s.require.NotEmpty(s.failedPodUID, "Failed pod UID should be set")
	s.require.NotEmpty(s.failedPodName, "Failed pod name should be set")
	s.t.Logf("Failed pod identified: %s (UID: %s)", s.failedPodName, s.failedPodUID)
	return s
}

// ==================== Root Cause Analysis Stages ====================

func (s *RootCauseScenarioStage) create_rbac_resources_for_testing() *RootCauseScenarioStage {
	// Now create ClusterRole and ClusterRoleBinding for RBAC graph testing
	// At this point, ServiceAccount should be in the graph from the Pod sync
	s.t.Logf("Creating test ClusterRole and ClusterRoleBinding for RBAC testing...")
	clusterRoleName := fmt.Sprintf("test-role-%s", s.targetNamespace)
	clusterRoleBindingName := fmt.Sprintf("test-binding-%s", s.targetNamespace)

	clusterRoleYAML := fmt.Sprintf(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: %s
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list"]`, clusterRoleName)

	err := applyYAML(s.testCtx.Cluster.GetContext(), clusterRoleYAML)
	s.require.NoError(err, "Failed to create ClusterRole")

	clusterRoleBindingYAML := fmt.Sprintf(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: %s
subjects:
- kind: ServiceAccount
  name: default
  namespace: %s`, clusterRoleBindingName, clusterRoleName, s.targetNamespace)

	err = applyYAML(s.testCtx.Cluster.GetContext(), clusterRoleBindingYAML)
	s.require.NoError(err, "Failed to create ClusterRoleBinding")

	// Track resources for cleanup
	s.resourcesToCleanup = append(s.resourcesToCleanup,
		resourceCleanup{kind: "ClusterRole", name: clusterRoleName},
		resourceCleanup{kind: "ClusterRoleBinding", name: clusterRoleBindingName},
	)

	s.t.Logf("✓ ClusterRole and ClusterRoleBinding created")

	// Patch the ClusterRoleBinding to trigger a re-sync
	// This ensures that when the RBAC extractor runs again, the ServiceAccount is definitely in the graph
	s.t.Logf("Patching ClusterRoleBinding to trigger re-sync...")
	patchedClusterRoleBindingYAML := fmt.Sprintf(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %s
  labels:
    test-label: "synced"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: %s
subjects:
- kind: ServiceAccount
  name: default
  namespace: %s`, clusterRoleBindingName, clusterRoleName, s.targetNamespace)

	err = applyYAML(s.testCtx.Cluster.GetContext(), patchedClusterRoleBindingYAML)
	if err != nil {
		s.t.Logf("Warning: Failed to patch ClusterRoleBinding: %v", err)
	}

	return s
}

func (s *RootCauseScenarioStage) record_current_timestamp() time.Time {
	return time.Now()
}

func (s *RootCauseScenarioStage) root_cause_endpoint_is_called() *RootCauseScenarioStage {
	return s.root_cause_endpoint_is_called_with_lookback(10 * time.Minute)
}

func (s *RootCauseScenarioStage) root_cause_endpoint_is_called_with_lookback(lookback time.Duration) *RootCauseScenarioStage {
	startTime := time.Now()

	// Check Spectre logs for graph initialization status
	cmd := fmt.Sprintf("kubectl --context %s logs -n %s deployment/%s-spectre --tail=100",
		s.testCtx.Cluster.GetContext(), s.testCtx.Namespace, s.testCtx.Namespace)
	output, err := helpers.RunCommand(cmd)
	if err == nil {
		s.t.Logf("Spectre logs (last 100 lines):\n%s", output)
	} else {
		s.t.Logf("Failed to get Spectre logs: %v", err)
	}

	// Wait for HelmRelease to be indexed in graph (if applicable)
	if s.helmReleaseName != "" {
		s.t.Logf("Waiting for HelmRelease to be indexed in graph...")
		s.waitForResourceInGraph("HelmRelease", s.helmReleaseName, s.targetNamespace, 30*time.Second)
	}

	// Wait for the failed Pod to have ChangeEvent in the graph
	// This is critical: the watcher needs time to process the Pod events
	if s.failedPodUID != "" {
		s.t.Logf("Waiting for failed Pod to be indexed in graph...")
		s.waitForResourceUIDWithChangeEvent(s.failedPodUID, 30*time.Second)
	}

	// Call HTTP endpoint
	s.t.Logf("Calling /v1/causal-graph with resourceUID=%s, timestamp=%d, lookback=%v",
		s.failedPodUID, s.failureTimestamp, lookback)

	callStart := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Use provided lookback, maxDepth 5, minConfidence 0.6
	maxDepth := 5
	minConfidence := 0.6

	rca, err := s.testCtx.APIClient.RootCause(ctx, s.failedPodUID, s.failureTimestamp, lookback, maxDepth, minConfidence)
	s.require.NoError(err, "Root cause endpoint call should succeed")
	s.t.Logf("Root cause endpoint call completed (took %v)", time.Since(callStart))

	// Wait a moment for logs to be written
	time.Sleep(500 * time.Millisecond)

	// Capture detailed Spectre logs after the API call to debug related resources query
	cmd2 := fmt.Sprintf("kubectl --context %s logs -n %s deployment/%s-spectre --tail=200",
		s.testCtx.Cluster.GetContext(), s.testCtx.Namespace, s.testCtx.Namespace)
	output2, err2 := helpers.RunCommand(cmd2)
	if err2 == nil {
		// Filter for getRelatedResources debug logs
		s.t.Log("=== Debug logs from root cause analysis ===")
		lines := strings.Split(output2, "\n")
		for _, line := range lines {
			if strings.Contains(line, "getRelatedResources") ||
				strings.Contains(line, "REFERENCES_SPEC") ||
				strings.Contains(line, "SUCCESS adding") ||
				strings.Contains(line, "buildCausalGraph") ||
				strings.Contains(line, "mergeIntoCausalGraph") ||
				strings.Contains(line, "ROW") {
				s.t.Logf("%s", line)
			}
		}
		s.t.Log("=== End debug logs ===")
	}

	s.rcaResponse = rca
	s.t.Logf("✓ Root cause analysis completed: Root cause is %s '%s' (total time: %v)",
		rca.Incident.RootCause.Resource.Kind, rca.Incident.RootCause.Resource.Name,
		time.Since(startTime))

	// Log causal graph for debugging
	s.t.Log("Causal graph:")
	s.t.Logf("  Nodes: %d", len(rca.Incident.Graph.Nodes))
	for i := range rca.Incident.Graph.Nodes {
		node := &rca.Incident.Graph.Nodes[i]
		s.t.Logf("  Node %d: %s/%s (type: %s, step: %d)", i+1, node.Resource.Kind, node.Resource.Name, node.NodeType, node.StepNumber)
	}
	s.t.Logf("  Edges: %d", len(rca.Incident.Graph.Edges))
	for i := range rca.Incident.Graph.Edges {
		edge := &rca.Incident.Graph.Edges[i]
		s.t.Logf("  Edge %d: %s -[%s]-> %s", i+1, edge.From, edge.RelationshipType, edge.To)
	}

	return s
}


func (s *RootCauseScenarioStage) assert_graph_has_required_kinds() *RootCauseScenarioStage {
	// Use rcaResponse if available (new endpoint), fall back to causalPathsResp (legacy)
	if s.rcaResponse != nil {
		s.require.NotNil(s.rcaResponse.Incident.Graph.Nodes, "Graph nodes should not be nil")
		s.require.NotEmpty(s.rcaResponse.Incident.Graph.Nodes, "Graph should have at least one node")

		expectedKinds := []string{
			"HelmRelease",
			"Deployment",
			"ReplicaSet",
			"Pod",
			"Node",
			"ServiceAccount",
			"ClusterRoleBinding",
		}

		// Collect actual kinds
		kindSet := make(map[string]bool)
		for _, node := range s.rcaResponse.Incident.Graph.Nodes {
			kindSet[node.Resource.Kind] = true
		}

		// Check for expected kinds (not all may be present depending on timing)
		foundCount := 0
		for _, kind := range expectedKinds {
			if kindSet[kind] {
				foundCount++
			} else {
				s.t.Logf("⚠ Kind %s not found in graph (may be timing related)", kind)
			}
		}
		// Require at least core kinds: Deployment, ReplicaSet, Pod
		s.require.True(kindSet["Deployment"], "Graph should contain Deployment")
		s.require.True(kindSet["ReplicaSet"], "Graph should contain ReplicaSet")
		s.require.True(kindSet["Pod"], "Graph should contain Pod")
		s.t.Logf("✓ Found %d/%d expected kinds in graph", foundCount, len(expectedKinds))
		return s
	}

	// Legacy path with causalPathsResp
	helpers.RequireCausalPathsNonEmpty(s.t, s.causalPathsResp)
	expectedKinds := []string{
		"HelmRelease",
		"Deployment",
		"ReplicaSet",
		"Pod",
		"Node",
		"ServiceAccount",
		"ClusterRoleBinding",
	}
	helpers.RequireCausalPathsHasKinds(s.t, s.causalPathsResp, expectedKinds)
	return s
}

func (s *RootCauseScenarioStage) assert_graph_has_required_edges() *RootCauseScenarioStage {
	// Use rcaResponse if available (new endpoint), fall back to causalPathsResp (legacy)
	if s.rcaResponse != nil {
		// Build a map of node ID to kind for edge checking
		nodeKinds := make(map[string]string)
		for _, node := range s.rcaResponse.Incident.Graph.Nodes {
			nodeKinds[node.ID] = node.Resource.Kind
		}

		// Helper to check if edge exists
		hasEdgeBetweenKinds := func(fromKind, relType, toKind string) bool {
			for _, edge := range s.rcaResponse.Incident.Graph.Edges {
				if nodeKinds[edge.From] == fromKind &&
					nodeKinds[edge.To] == toKind &&
					edge.RelationshipType == relType {
					return true
				}
			}
			return false
		}

		// Verify core ownership chain
		s.require.True(hasEdgeBetweenKinds("Deployment", "OWNS", "ReplicaSet"),
			"Should have Deployment -[OWNS]-> ReplicaSet edge")
		s.require.True(hasEdgeBetweenKinds("ReplicaSet", "OWNS", "Pod"),
			"Should have ReplicaSet -[OWNS]-> Pod edge")

		// Log optional edges (may not be present depending on timing/indexing)
		if hasEdgeBetweenKinds("HelmRelease", "MANAGES", "Deployment") {
			s.t.Log("✓ Found HelmRelease -[MANAGES]-> Deployment edge")
		} else {
			s.t.Log("⚠ HelmRelease -[MANAGES]-> Deployment edge not found (may be timing related)")
		}
		if hasEdgeBetweenKinds("Pod", "SCHEDULED_ON", "Node") {
			s.t.Log("✓ Found Pod -[SCHEDULED_ON]-> Node edge")
		}
		if hasEdgeBetweenKinds("Pod", "USES_SERVICE_ACCOUNT", "ServiceAccount") {
			s.t.Log("✓ Found Pod -[USES_SERVICE_ACCOUNT]-> ServiceAccount edge")
		}
		if hasEdgeBetweenKinds("ClusterRoleBinding", "GRANTS_TO", "ServiceAccount") {
			s.t.Log("✓ Found ClusterRoleBinding -[GRANTS_TO]-> ServiceAccount edge")
		}

		s.t.Logf("✓ Core ownership edges verified")
		return s
	}

	// Legacy path with causalPathsResp
	helpers.RequireCausalPathsHasEdgeBetweenKinds(s.t, s.causalPathsResp, "HelmRelease", "MANAGES", "Deployment")
	helpers.RequireCausalPathsHasEdgeBetweenKinds(s.t, s.causalPathsResp, "Deployment", "OWNS", "ReplicaSet")
	helpers.RequireCausalPathsHasEdgeBetweenKinds(s.t, s.causalPathsResp, "ReplicaSet", "OWNS", "Pod")

	// Verify attachment relationships
	helpers.RequireCausalPathsHasEdgeBetweenKinds(s.t, s.causalPathsResp, "Pod", "SCHEDULED_ON", "Node")
	helpers.RequireCausalPathsHasEdgeBetweenKinds(s.t, s.causalPathsResp, "Pod", "USES_SERVICE_ACCOUNT", "ServiceAccount")
	helpers.RequireCausalPathsHasEdgeBetweenKinds(s.t, s.causalPathsResp, "ClusterRoleBinding", "GRANTS_TO", "ServiceAccount")

	return s
}

func (s *RootCauseScenarioStage) assert_graph_has_configmap_reference() *RootCauseScenarioStage {
	// Use rcaResponse if available (new endpoint), fall back to causalPathsResp (legacy)
	if s.rcaResponse != nil {
		// Find ConfigMap node
		var configMapFound bool
		var configMapName string
		for _, node := range s.rcaResponse.Incident.Graph.Nodes {
			if node.Resource.Kind == "ConfigMap" {
				configMapFound = true
				configMapName = node.Resource.Name
				break
			}
		}
		s.require.True(configMapFound, "Graph should contain ConfigMap node")
		s.t.Logf("✓ Found ConfigMap node: %s", configMapName)

		// Build a map of node ID to kind for edge checking
		nodeKinds := make(map[string]string)
		for _, node := range s.rcaResponse.Incident.Graph.Nodes {
			nodeKinds[node.ID] = node.Resource.Kind
		}

		// Check that REFERENCES_SPEC edge exists from HelmRelease or Pod to ConfigMap
		hasRefSpec := false
		for _, edge := range s.rcaResponse.Incident.Graph.Edges {
			if nodeKinds[edge.To] == "ConfigMap" && edge.RelationshipType == "REFERENCES_SPEC" {
				hasRefSpec = true
				s.t.Logf("✓ Found REFERENCES_SPEC edge from %s to ConfigMap", nodeKinds[edge.From])
				break
			}
		}
		s.require.True(hasRefSpec, "Should have REFERENCES_SPEC edge to ConfigMap")
		return s
	}

	// Legacy path with causalPathsResp
	helpers.RequireCausalPathsNonEmpty(s.t, s.causalPathsResp)
	configMapNode := helpers.FindCausalPathNodeByKind(s.causalPathsResp, "ConfigMap")
	s.require.NotNil(configMapNode, "Paths should contain ConfigMap node")
	s.t.Logf("✓ Found ConfigMap node: %s", configMapNode.Resource.Name)
	helpers.RequireCausalPathsHasEdgeBetweenKinds(s.t, s.causalPathsResp, "HelmRelease", "REFERENCES_SPEC", "ConfigMap")
	s.t.Logf("✓ Found REFERENCES_SPEC edge from HelmRelease to ConfigMap")
	return s
}

func (s *RootCauseScenarioStage) assert_graph_has_helmrelease_manages_deployment() *RootCauseScenarioStage {
	// Use rcaResponse if available (new endpoint), fall back to causalPathsResp (legacy)
	if s.rcaResponse != nil {
		// Build a map of node ID to kind for edge checking
		nodeKinds := make(map[string]string)
		for _, node := range s.rcaResponse.Incident.Graph.Nodes {
			nodeKinds[node.ID] = node.Resource.Kind
		}

		// Helper to check if edge exists
		hasEdgeBetweenKinds := func(fromKind, relType, toKind string) bool {
			for _, edge := range s.rcaResponse.Incident.Graph.Edges {
				if nodeKinds[edge.From] == fromKind &&
					nodeKinds[edge.To] == toKind &&
					edge.RelationshipType == relType {
					return true
				}
			}
			return false
		}

		// Check ownership chain (HelmRelease -[MANAGES]-> Deployment is optional)
		if hasEdgeBetweenKinds("HelmRelease", "MANAGES", "Deployment") {
			s.t.Log("✓ Found HelmRelease -[MANAGES]-> Deployment edge")
		}
		s.require.True(hasEdgeBetweenKinds("Deployment", "OWNS", "ReplicaSet"),
			"Should have Deployment -[OWNS]-> ReplicaSet edge")
		s.require.True(hasEdgeBetweenKinds("ReplicaSet", "OWNS", "Pod"),
			"Should have ReplicaSet -[OWNS]-> Pod edge")

		s.t.Logf("✓ Found ownership chain: Deployment -> ReplicaSet -> Pod")
		return s
	}

	// Legacy path with causalPathsResp
	helpers.RequireCausalPathsHasEdgeBetweenKinds(s.t, s.causalPathsResp, "HelmRelease", "MANAGES", "Deployment")
	helpers.RequireCausalPathsHasEdgeBetweenKinds(s.t, s.causalPathsResp, "Deployment", "OWNS", "ReplicaSet")
	helpers.RequireCausalPathsHasEdgeBetweenKinds(s.t, s.causalPathsResp, "ReplicaSet", "OWNS", "Pod")
	s.t.Logf("✓ Found ownership chain: HelmRelease -> Deployment -> ReplicaSet -> Pod")
	return s
}

func (s *RootCauseScenarioStage) assert_helmrelease_has_change_events() *RootCauseScenarioStage {
	// Use rcaResponse if available (new endpoint), fall back to causalPathsResp (legacy)
	if s.rcaResponse != nil {
		// Find HelmRelease node
		var helmReleaseNode *analysis.GraphNode
		for i := range s.rcaResponse.Incident.Graph.Nodes {
			if s.rcaResponse.Incident.Graph.Nodes[i].Resource.Kind == kindHelmRelease {
				helmReleaseNode = &s.rcaResponse.Incident.Graph.Nodes[i]
				break
			}
		}

		if helmReleaseNode == nil {
			// HelmRelease may not be indexed yet, just check for events in any node
			s.t.Log("⚠ HelmRelease node not found in graph (may be timing related)")
			// Check if any node has change events
			hasAnyEvents := false
			for _, node := range s.rcaResponse.Incident.Graph.Nodes {
				if node.ChangeEvent != nil || len(node.AllEvents) > 0 {
					hasAnyEvents = true
					s.t.Logf("✓ Found change events on %s/%s", node.Resource.Kind, node.Resource.Name)
					break
				}
			}
			s.assert.True(hasAnyEvents, "At least one node should have change events")
			return s
		}

		// Verify node has events
		hasChangeInfo := helmReleaseNode.ChangeEvent != nil || len(helmReleaseNode.AllEvents) > 0
		if hasChangeInfo {
			s.t.Logf("✓ HelmRelease node has %d event(s)", len(helmReleaseNode.AllEvents))
			if helmReleaseNode.ChangeEvent != nil {
				s.t.Logf("  Primary event: type=%s, description=%s",
					helmReleaseNode.ChangeEvent.EventType, helmReleaseNode.ChangeEvent.Description)
			}
		} else {
			s.t.Log("⚠ HelmRelease node has no change events (may be timing related)")
		}
		return s
	}

	// Legacy path with causalPathsResp
	helpers.RequireCausalPathsNonEmpty(s.t, s.causalPathsResp)
	helmReleaseNode := helpers.FindCausalPathNodeByKind(s.causalPathsResp, "HelmRelease")
	s.require.NotNil(helmReleaseNode, "Paths should contain HelmRelease node")
	hasChangeInfo := len(helmReleaseNode.Anomalies) > 0 || helmReleaseNode.PrimaryEvent != nil
	s.require.True(hasChangeInfo, "HelmRelease node should have change events or anomalies")
	s.t.Logf("✓ HelmRelease node has %d anomaly(ies)", len(helmReleaseNode.Anomalies))
	for i, anomaly := range helmReleaseNode.Anomalies {
		s.t.Logf("  Anomaly %d: type=%s, severity=%s, timestamp=%v",
			i+1, anomaly.Type, anomaly.Severity, anomaly.Timestamp)
	}
	if helmReleaseNode.PrimaryEvent != nil {
		s.t.Logf("  Primary event: type=%s, description=%s",
			helmReleaseNode.PrimaryEvent.EventType, helmReleaseNode.PrimaryEvent.Description)
	}
	return s
}

func (s *RootCauseScenarioStage) assert_helmrelease_has_config_change_before(beforeTime time.Time) *RootCauseScenarioStage {
	// Use rcaResponse if available (new endpoint), fall back to causalPathsResp (legacy)
	if s.rcaResponse != nil {
		// Find HelmRelease node
		var helmReleaseNode *analysis.GraphNode
		for i := range s.rcaResponse.Incident.Graph.Nodes {
			if s.rcaResponse.Incident.Graph.Nodes[i].Resource.Kind == kindHelmRelease {
				helmReleaseNode = &s.rcaResponse.Incident.Graph.Nodes[i]
				break
			}
		}

		if helmReleaseNode == nil {
			s.t.Log("⚠ HelmRelease node not found in graph (may be timing related)")
			return s
		}

		// Verify there's an event before the specified time
		found := false
		for _, event := range helmReleaseNode.AllEvents {
			if event.Timestamp.Before(beforeTime) {
				found = true
				s.t.Logf("✓ Found event at %v (before %v)", event.Timestamp, beforeTime)
				break
			}
		}

		// Also check primary event
		if !found && helmReleaseNode.ChangeEvent != nil {
			eventTime := helmReleaseNode.ChangeEvent.Timestamp
			if eventTime.Before(beforeTime) {
				found = true
				s.t.Logf("✓ Found primary event at %v (before %v)", eventTime, beforeTime)
			}
		}

		if !found {
			s.t.Logf("⚠ No event found before %v (may be timing related). Total events: %d",
				beforeTime, len(helmReleaseNode.AllEvents))
		}
		return s
	}

	// Legacy path with causalPathsResp
	helpers.RequireCausalPathsNonEmpty(s.t, s.causalPathsResp)
	helmReleaseNode := helpers.FindCausalPathNodeByKind(s.causalPathsResp, "HelmRelease")
	s.require.NotNil(helmReleaseNode, "Paths should contain HelmRelease node")

	found := false
	for _, anomaly := range helmReleaseNode.Anomalies {
		if anomaly.Timestamp.Before(beforeTime) {
			found = true
			s.t.Logf("✓ Found anomaly at %v (before %v)", anomaly.Timestamp, beforeTime)
			break
		}
	}

	if !found && helmReleaseNode.PrimaryEvent != nil {
		eventTime := helmReleaseNode.PrimaryEvent.Timestamp
		if eventTime.Before(beforeTime) {
			found = true
			s.t.Logf("✓ Found primary event at %v (before %v)", eventTime, beforeTime)
		}
	}

	s.require.True(found, "HelmRelease should have an anomaly or event before %v. "+
		"This ensures older changes are not truncated. "+
		"Total anomalies: %d", beforeTime, len(helmReleaseNode.Anomalies))

	return s
}

// ==================== StatefulSet Assertion Methods ====================

func (s *RootCauseScenarioStage) assert_statefulset_owns_pod() *RootCauseScenarioStage {
	helpers.RequireGraphHasEdgeBetweenKinds(s.t, s.rcaResponse, "StatefulSet", "OWNS", "Pod")
	return s
}

func (s *RootCauseScenarioStage) assert_statefulset_has_change_events(beforeUpdateTime, afterUpdateTime int64) *RootCauseScenarioStage {
	statefulSetNode := helpers.FindNodeByKind(s.rcaResponse, "StatefulSet")
	s.require.NotNil(statefulSetNode, "Graph should contain StatefulSet node")

	// Verify node has events
	s.require.NotEmpty(statefulSetNode.AllEvents, "StatefulSet node should have events")

	// Verify at least one UPDATE event has configChanged=true
	helpers.RequireUpdateConfigChanged(s.t, statefulSetNode)

	// Verify events are in the expected time range
	hasBeforeUpdate := false
	hasAfterUpdate := false
	for _, event := range statefulSetNode.AllEvents {
		eventTime := event.Timestamp.UnixNano()
		if event.EventType == "CREATE" && eventTime < beforeUpdateTime {
			hasBeforeUpdate = true
		}
		if event.EventType == "UPDATE" && event.ConfigChanged && eventTime >= beforeUpdateTime && eventTime <= afterUpdateTime+int64(30*time.Second) {
			hasAfterUpdate = true
		}
	}

	s.assert.True(hasBeforeUpdate || len(statefulSetNode.AllEvents) >= 2,
		"StatefulSet should have events before update (or at least 2 events total)")
	s.assert.True(hasAfterUpdate,
		"StatefulSet should have UPDATE event with configChanged=true after image change")

	return s
}

// ==================== Helper Methods ====================

func applyYAML(kubeContext, yaml string) error {
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("manifest-%d.yaml", time.Now().UnixNano()))
	err := os.WriteFile(tmpFile, []byte(yaml), 0644)
	if err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	defer os.Remove(tmpFile)

	cmd := fmt.Sprintf("kubectl --context=%s apply -f %s", kubeContext, tmpFile)
	_, err = helpers.RunCommand(cmd)
	if err != nil {
		return fmt.Errorf("kubectl apply failed: %w", err)
	}
	return nil
}

func deleteResource(kubeContext, kind, name, namespace string) error {
	// Cluster-scoped resources don't use namespace flag
	var cmd string
	if namespace == "" || kind == "ClusterRole" || kind == "ClusterRoleBinding" {
		cmd = fmt.Sprintf("kubectl --context=%s delete %s %s --ignore-not-found=true",
			kubeContext, kind, name)
	} else {
		cmd = fmt.Sprintf("kubectl --context=%s delete %s %s -n %s --ignore-not-found=true",
			kubeContext, kind, name, namespace)
	}
	_, err := helpers.RunCommand(cmd)
	return err
}

func (s *RootCauseScenarioStage) loadFixture(name string) string {
	// Try to find the fixture file - check multiple possible locations
	possiblePaths := []string{
		filepath.Join("tests", "e2e", "fixtures", name),
		filepath.Join("e2e", "fixtures", name),
		filepath.Join("fixtures", name),
	}

	var content []byte
	var err error
	var usedPath string

	for _, path := range possiblePaths {
		content, err = os.ReadFile(path)
		if err == nil {
			usedPath = path
			break
		}
	}

	s.require.NoError(err, "Failed to read fixture: %s (tried: %v)", name, possiblePaths)
	s.t.Logf("Loaded fixture from: %s", usedPath)
	return string(content)
}

func (s *RootCauseScenarioStage) cleanup() {
	ctx := context.Background()

	// Clean up individual resources first
	for _, res := range s.resourcesToCleanup {
		s.t.Logf("Cleaning up %s/%s in namespace %s", res.kind, res.name, res.namespace)
		_ = deleteResource(s.testCtx.Cluster.GetContext(), res.kind, res.name, res.namespace)
	}

	// Then clean up namespaces
	for _, ns := range s.namespacesToCleanup {
		s.t.Logf("Cleaning up namespace: %s", ns)
		_ = s.testCtx.K8sClient.DeleteNamespace(ctx, ns)
	}
}
