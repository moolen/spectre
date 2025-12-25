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
	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type RootCauseScenarioStage struct {
	t         *testing.T
	require   *require.Assertions
	assert    *assert.Assertions
	testCtx   *helpers.TestContext
	mcpClient *helpers.MCPClient

	// Test state
	helmReleaseName   string
	helmReleaseNs     string
	helmRepoName      string
	deploymentName    string
	deploymentNs      string
	statefulSetName   string
	statefulSetNs     string
	targetNamespace   string
	failedPodUID      string
	failedPodName     string
	failureTimestamp  int64
	beforeUpdateTime  int64
	afterUpdateTime   int64
	rcaResponse       *analysis.RootCauseAnalysisV2

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
	s.testCtx = helpers.SetupE2ETest(s.t)
	s.t.Logf("✓ Test environment setup completed (took %v)", time.Since(startTime))
	return s
}

func (s *RootCauseScenarioStage) flux_is_installed() *RootCauseScenarioStage {
	startTime := time.Now()
	err := helpers.EnsureFluxInstalled(s.t, s.testCtx.K8sClient, s.testCtx.Cluster.GetKubeConfig())
	s.require.NoError(err, "Failed to ensure Flux is installed")
	s.t.Logf("✓ Flux installation check completed (took %v)", time.Since(startTime))
	return s
}

func (s *RootCauseScenarioStage) spectre_is_deployed() *RootCauseScenarioStage {
	startTime := time.Now()
	// Spectre is already deployed by SetupE2ETest
	// Just verify it's ready
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	err := helpers.WaitForAppReady(ctx, s.testCtx.K8sClient, s.testCtx.Namespace, s.testCtx.ReleaseName)
	s.require.NoError(err, "Spectre deployment not ready")

	s.t.Logf("✓ Spectre is deployed and ready (took %v)", time.Since(startTime))
	return s
}

func (s *RootCauseScenarioStage) mcp_client_is_connected() *RootCauseScenarioStage {
	startTime := time.Now()
	// Create port-forward for MCP server
	serviceName := s.testCtx.ReleaseName + "-spectre"
	mcpPortForward, err := helpers.NewPortForwarder(s.t, s.testCtx.Cluster.GetKubeConfig(), s.testCtx.Namespace, serviceName, 8082)
	s.require.NoError(err, "Failed to create MCP port-forward")

	err = mcpPortForward.WaitForReady(30 * time.Second)
	s.require.NoError(err, "MCP server not reachable via port-forward")

	s.mcpClient = helpers.NewMCPClient(s.t, mcpPortForward.GetURL())
	s.t.Logf("✓ MCP client connected (took %v)", time.Since(startTime))

	return s
}

// ==================== HelmRelease Deployment Stages ====================

func (s *RootCauseScenarioStage) helmrelease_is_deployed(fixture string) *RootCauseScenarioStage {
	startTime := time.Now()
	fixtureContent := s.loadFixture(fixture)

	// Apply the manifest
	s.applyManifest(fixtureContent)

	// Extract HelmRelease name and namespace
	s.extractHelmReleaseInfo(fixtureContent)

	// Track namespace for cleanup
	if !contains(s.namespacesToCleanup, s.helmReleaseNs) {
		s.namespacesToCleanup = append(s.namespacesToCleanup, s.helmReleaseNs)
	}

	s.t.Logf("✓ Deployed HelmRelease: %s/%s (took %v)", s.helmReleaseNs, s.helmReleaseName, time.Since(startTime))
	return s
}

func (s *RootCauseScenarioStage) wait_for_healthy_deployment(timeout time.Duration) *RootCauseScenarioStage {
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// First wait for HelmRelease to be ready
	err := helpers.WaitForHelmReleaseReady(ctx, s.t, s.helmReleaseNs, s.helmReleaseName, timeout)
	s.require.NoError(err, "HelmRelease did not become ready")

	// Wait for pods to be running
	s.t.Logf("Waiting for pods to be running in namespace %s...", s.helmReleaseNs)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pods, err := s.testCtx.K8sClient.Clientset.CoreV1().Pods(s.helmReleaseNs).List(ctx, metav1.ListOptions{})
		if err == nil && len(pods.Items) > 0 {
			allRunning := true
			for _, pod := range pods.Items {
				if pod.Status.Phase != corev1.PodRunning {
					allRunning = false
					break
				}
			}
			if allRunning {
				s.t.Logf("✓ All pods are running in namespace %s (total wait: %v)", s.helmReleaseNs, time.Since(startTime))
				return s
			}
		}
		time.Sleep(3 * time.Second)
	}

	s.require.Fail("Timeout waiting for pods to be running")
	return s
}

func (s *RootCauseScenarioStage) helmrelease_is_updated(fixture string) *RootCauseScenarioStage {
	fixtureContent := s.loadFixture(fixture)

	// Apply the updated manifest
	s.applyManifest(fixtureContent)

	// Wait for reconciliation
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	err := helpers.WaitForHelmReleaseReconciled(ctx, s.t, s.helmReleaseNs, s.helmReleaseName, 90*time.Second)
	s.require.NoError(err, "HelmRelease did not reconcile after update")

	s.t.Logf("✓ HelmRelease %s/%s updated and reconciled", s.helmReleaseNs, s.helmReleaseName)
	return s
}

// ==================== Direct Deployment Stages ====================

func (s *RootCauseScenarioStage) deployment_is_deployed(fixture string) *RootCauseScenarioStage {
	fixtureContent := s.loadFixture(fixture)

	// Apply the manifest
	s.applyManifest(fixtureContent)

	// Extract deployment info
	s.extractDeploymentInfo(fixtureContent)

	// Track namespace for cleanup
	if !contains(s.namespacesToCleanup, s.deploymentNs) {
		s.namespacesToCleanup = append(s.namespacesToCleanup, s.deploymentNs)
	}

	s.t.Logf("✓ Deployed Deployment: %s/%s", s.deploymentNs, s.deploymentName)
	return s
}

func (s *RootCauseScenarioStage) wait_for_healthy_pods(timeout time.Duration) *RootCauseScenarioStage {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns := s.deploymentNs
	if ns == "" {
		ns = s.helmReleaseNs
	}

	s.t.Logf("Waiting for pods to be running in namespace %s...", ns)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pods, err := s.testCtx.K8sClient.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err == nil && len(pods.Items) > 0 {
			allRunning := true
			for _, pod := range pods.Items {
				if pod.Status.Phase != corev1.PodRunning {
					allRunning = false
					break
				}
			}
			if allRunning {
				s.t.Logf("✓ All pods are running in namespace %s", ns)
				return s
			}
		}
		time.Sleep(3 * time.Second)
	}

	s.require.Fail("Timeout waiting for pods to be running")
	return s
}

func (s *RootCauseScenarioStage) deployment_is_updated(fixture string) *RootCauseScenarioStage {
	fixtureContent := s.loadFixture(fixture)

	// Apply the updated manifest
	s.applyManifest(fixtureContent)

	s.t.Logf("✓ Deployment %s/%s updated", s.deploymentNs, s.deploymentName)
	return s
}

// ==================== Flux External HelmRelease Stages ====================

func (s *RootCauseScenarioStage) flux_external_helmrelease_is_deployed(chartName, chartVersion, chartURL string) *RootCauseScenarioStage {
	startTime := time.Now()
	ctx := context.Background()

	// Create unique test identifiers
	testID := fmt.Sprintf("rca-test-%d", time.Now().UnixNano()%1000000)
	s.helmRepoName = fmt.Sprintf("%s-repo-%s", chartName, testID)
	s.helmReleaseName = fmt.Sprintf("%s-%s", chartName, testID)
	s.targetNamespace = fmt.Sprintf("%s-%s", chartName, testID)

	// Create target namespace
	err := s.testCtx.K8sClient.CreateNamespace(ctx, s.targetNamespace)
	s.require.NoError(err, "Failed to create target namespace")
	s.namespacesToCleanup = append(s.namespacesToCleanup, s.targetNamespace)

	// Create HelmRepository in flux-system
	helmRepoYAML := fmt.Sprintf(`apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: HelmRepository
metadata:
  name: %s
  namespace: flux-system
spec:
  interval: 1h
  url: %s`, s.helmRepoName, chartURL)

	err = applyYAML(s.testCtx.Cluster.GetKubeConfig(), helmRepoYAML)
	s.require.NoError(err, "Failed to create HelmRepository")
	s.resourcesToCleanup = append(s.resourcesToCleanup, resourceCleanup{
		kind:      "helmrepository",
		name:      s.helmRepoName,
		namespace: "flux-system",
	})

	// Wait for HelmRepository to be ready
	s.t.Logf("Waiting for HelmRepository %s/%s to be ready...", "flux-system", s.helmRepoName)
	s.waitForHelmRepositoryReady("flux-system", s.helmRepoName, 60*time.Second)

	// Create HelmRelease in flux-system
	helmReleaseYAML := fmt.Sprintf(`apiVersion: helm.toolkit.fluxcd.io/v2beta2
kind: HelmRelease
metadata:
  name: %s
  namespace: flux-system
spec:
  interval: 5m
  chart:
    spec:
      chart: %s
      version: "%s"
      sourceRef:
        kind: HelmRepository
        name: %s
        namespace: flux-system
  values:
    replicaCount: 1
    resources:
      limits:
        cpu: 200m
        memory: 256Mi
      requests:
        cpu: 100m
        memory: 128Mi
    serviceAccount:
      create: true
      name: %s
  targetNamespace: %s
  install:
    createNamespace: false`, s.helmReleaseName, chartName, chartVersion, s.helmRepoName, chartName, s.targetNamespace)

	err = applyYAML(s.testCtx.Cluster.GetKubeConfig(), helmReleaseYAML)
	s.require.NoError(err, "Failed to create HelmRelease")
	s.resourcesToCleanup = append(s.resourcesToCleanup, resourceCleanup{
		kind:      "helmrelease",
		name:      s.helmReleaseName,
		namespace: "flux-system",
	})

	// Store HelmRelease info for assertions
	s.helmReleaseNs = "flux-system"

	// Wait for HelmRelease to be ready
	hrCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	err = helpers.WaitForHelmReleaseReady(hrCtx, s.t, "flux-system", s.helmReleaseName, 120*time.Second)
	s.require.NoError(err, "HelmRelease did not become ready")

	// Wait for pods to be running in target namespace
	s.t.Logf("Waiting for pods to be running in namespace %s...", s.targetNamespace)
	s.waitForPodsRunningInNamespace(s.targetNamespace, 120*time.Second)

	s.t.Logf("✓ Flux external HelmRelease deployed successfully (took %v)", time.Since(startTime))
	return s
}

func (s *RootCauseScenarioStage) flux_helmrelease_image_is_updated(imageTag string) *RootCauseScenarioStage {
	ctx := context.Background()

	// For simplicity, we'll create a new manifest with the updated values
	updateYAML := fmt.Sprintf(`apiVersion: helm.toolkit.fluxcd.io/v2beta2
kind: HelmRelease
metadata:
  name: %s
  namespace: flux-system
spec:
  interval: 5m
  chart:
    spec:
      chart: external-secrets
      version: "0.9.9"
      sourceRef:
        kind: HelmRepository
        name: %s
        namespace: flux-system
  values:
    replicaCount: 1
    image:
      tag: "%s"
    resources:
      limits:
        cpu: 200m
        memory: 256Mi
      requests:
        cpu: 100m
        memory: 128Mi
    serviceAccount:
      create: true
      name: external-secrets
  targetNamespace: %s
  install:
    createNamespace: false`, s.helmReleaseName, s.helmRepoName, imageTag, s.targetNamespace)

	err := applyYAML(s.testCtx.Cluster.GetKubeConfig(), updateYAML)
	s.require.NoError(err, "Failed to update HelmRelease")

	// Wait for reconciliation
	reconcileCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	err = helpers.WaitForHelmReleaseReconciled(reconcileCtx, s.t, "flux-system", s.helmReleaseName, 90*time.Second)
	s.require.NoError(err, "HelmRelease did not reconcile after update")

	// Update the stage's namespace tracking for pod failure detection
	s.helmReleaseNs = s.targetNamespace

	s.t.Logf("✓ HelmRelease image updated to: %s", imageTag)
	return s
}

func (s *RootCauseScenarioStage) waitForHelmRepositoryReady(namespace, name string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		cmd := fmt.Sprintf("kubectl --kubeconfig %s get helmrepository %s -n %s -o jsonpath='{.status.conditions[?(@.type==\"Ready\")].status}'",
			s.testCtx.Cluster.GetKubeConfig(), name, namespace)
		output, err := helpers.RunCommand(cmd)
		if err == nil && strings.TrimSpace(output) == "True" {
			s.t.Logf("✓ HelmRepository %s/%s is ready", namespace, name)
			return
		}
		time.Sleep(5 * time.Second)
	}
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
			for _, pod := range pods.Items {
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

	cmd := fmt.Sprintf("kubectl --kubeconfig=%s apply -f %s", s.testCtx.Cluster.GetKubeConfig(), tmpFile)
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
	cmd := fmt.Sprintf("kubectl --kubeconfig=%s get statefulset %s -n %s -o json",
		s.testCtx.Cluster.GetKubeConfig(), s.statefulSetName, s.statefulSetNs)
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

	cmd = fmt.Sprintf("kubectl --kubeconfig=%s apply -f %s", s.testCtx.Cluster.GetKubeConfig(), tmpFile)
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
		cmd := fmt.Sprintf("kubectl --kubeconfig=%s get statefulset %s -n %s -o json",
			s.testCtx.Cluster.GetKubeConfig(), name, namespace)
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
							for _, pod := range pods.Items {
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

		for _, pod := range pods.Items {
			// Check container statuses for the symptom
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.State.Waiting != nil {
					if strings.Contains(containerStatus.State.Waiting.Reason, symptom) {
						s.failedPodUID = string(pod.UID)
						s.failedPodName = pod.Name
						s.failureTimestamp = time.Now().UnixNano()
						s.t.Logf("✓ Found failed pod: %s (UID: %s) with symptom: %s (waited %v)",
							s.failedPodName, s.failedPodUID, symptom, time.Since(startTime))
						return s
					}
				}
			}
		}

		time.Sleep(3 * time.Second)
	}

	// List pods for debugging
	pods, _ := s.testCtx.K8sClient.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	s.t.Logf("Current pods in namespace %s:", ns)
	for _, pod := range pods.Items {
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

func (s *RootCauseScenarioStage) root_cause_endpoint_is_called() *RootCauseScenarioStage {
	startTime := time.Now()
	// Wait for Spectre to index the failure
	s.t.Log("Waiting 15 seconds for Spectre to index the failure...")
	waitStart := time.Now()
	time.Sleep(15 * time.Second)
	s.t.Logf("Waited %v for indexing", time.Since(waitStart))

	// Call HTTP endpoint
	s.t.Logf("Calling /v1/root-cause with resourceUID=%s, timestamp=%d",
		s.failedPodUID, s.failureTimestamp)

	callStart := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use default lookback of 10 minutes, maxDepth 5, minConfidence 0.6
	lookback := 10 * time.Minute
	maxDepth := 5
	minConfidence := 0.6

	rca, err := s.testCtx.APIClient.RootCause(ctx, s.failedPodUID, s.failureTimestamp, lookback, maxDepth, minConfidence)
	s.require.NoError(err, "Root cause endpoint call should succeed")
	s.t.Logf("Root cause endpoint call completed (took %v)", time.Since(callStart))

	s.rcaResponse = rca
	s.t.Logf("✓ Root cause analysis completed: Root cause is %s '%s' (total time: %v)",
		rca.Incident.RootCause.Resource.Kind, rca.Incident.RootCause.Resource.Name,
		time.Since(startTime))

	// Log causal graph for debugging
	s.t.Log("Causal graph:")
	s.t.Logf("  Nodes: %d", len(rca.Incident.Graph.Nodes))
	for i, node := range rca.Incident.Graph.Nodes {
		s.t.Logf("  Node %d: %s/%s (type: %s, step: %d)", i+1, node.Resource.Kind, node.Resource.Name, node.NodeType, node.StepNumber)
	}
	s.t.Logf("  Edges: %d", len(rca.Incident.Graph.Edges))
	for i, edge := range rca.Incident.Graph.Edges {
		s.t.Logf("  Edge %d: %s -[%s]-> %s", i+1, edge.From, edge.RelationshipType, edge.To)
	}

	return s
}

func (s *RootCauseScenarioStage) find_root_cause_tool_is_called() *RootCauseScenarioStage {
	startTime := time.Now()
	// Wait for Spectre to index the failure
	s.t.Log("Waiting 15 seconds for Spectre to index the failure...")
	waitStart := time.Now()
	time.Sleep(15 * time.Second)
	s.t.Logf("Waited %v for indexing", time.Since(waitStart))

	// Call MCP tool
	request := map[string]interface{}{
		"resource_uid":      s.failedPodUID,
		"failure_timestamp": s.failureTimestamp,
	}

	s.t.Logf("Calling find_root_cause with resource_uid=%s, timestamp=%d",
		s.failedPodUID, s.failureTimestamp)

	callStart := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := s.mcpClient.CallTool(ctx, "find_root_cause", request)
	s.require.NoError(err, "MCP tool call should succeed")
	s.t.Logf("MCP tool call completed (took %v)", time.Since(callStart))

	// Extract content from response
	// MCP tools/call response has structure: { "content": [ { "type": "text", "text": "..." } ] }
	parseStart := time.Now()
	contentArray, ok := response["content"].([]interface{})
	s.require.True(ok, "Response should have 'content' array")
	s.require.NotEmpty(contentArray, "Content array should not be empty")

	firstContent, ok := contentArray[0].(map[string]interface{})
	s.require.True(ok, "First content should be a map")

	textContent, ok := firstContent["text"].(string)
	s.require.True(ok, "First content should have 'text' field")

	// Log the raw response for debugging
	s.t.Logf("Raw MCP response text (first 500 chars): %s", truncateString(textContent, 500))

	// Parse the JSON text content into RootCauseAnalysisV2
	var rca analysis.RootCauseAnalysisV2
	err = json.Unmarshal([]byte(textContent), &rca)
	s.require.NoError(err, "Response should be valid RootCauseAnalysisV2. Raw text: %s", truncateString(textContent, 200))

	s.rcaResponse = &rca
	s.t.Logf("✓ Root cause analysis completed: Root cause is %s '%s' (total time: %v, parse time: %v)",
		rca.Incident.RootCause.Resource.Kind, rca.Incident.RootCause.Resource.Name,
		time.Since(startTime), time.Since(parseStart))

	// Log causal graph for debugging
	s.t.Log("Causal graph:")
	s.t.Logf("  Nodes: %d", len(rca.Incident.Graph.Nodes))
	for i, node := range rca.Incident.Graph.Nodes {
		s.t.Logf("  Node %d: %s (type: %s, step: %d)", i+1, node.Resource.Kind, node.NodeType, node.StepNumber)
	}
	s.t.Logf("  Edges: %d", len(rca.Incident.Graph.Edges))
	for i, edge := range rca.Incident.Graph.Edges {
		s.t.Logf("  Edge %d: %s -[%s]-> %s", i+1, edge.From, edge.RelationshipType, edge.To)
	}

	return s
}

// ==================== Assertion Stages ====================

func (s *RootCauseScenarioStage) root_cause_is_helmrelease() *RootCauseScenarioStage {
	s.assert.Equal("HelmRelease", s.rcaResponse.Incident.RootCause.Resource.Kind,
		"Root cause should be HelmRelease")
	s.assert.Equal(s.helmReleaseName, s.rcaResponse.Incident.RootCause.Resource.Name,
		"Root cause should be the deployed HelmRelease")
	return s
}

func (s *RootCauseScenarioStage) root_cause_is_deployment() *RootCauseScenarioStage {
	s.assert.Equal("Deployment", s.rcaResponse.Incident.RootCause.Resource.Kind,
		"Root cause should be Deployment")
	return s
}

func (s *RootCauseScenarioStage) causal_chain_includes_all_steps(expectedSteps int) *RootCauseScenarioStage {
	// Count spine nodes (which represent the causal chain steps)
	spineNodes := 0
	for _, node := range s.rcaResponse.Incident.Graph.Nodes {
		if node.NodeType == "SPINE" {
			spineNodes++
		}
	}
	s.assert.Equal(expectedSteps, spineNodes,
		"Causal graph should have %d spine nodes", expectedSteps)
	return s
}

func (s *RootCauseScenarioStage) causal_chain_has_step(resourceKind, relType, targetKind string) *RootCauseScenarioStage {
	// Find a node with the given resource kind
	var sourceNode *analysis.GraphNode
	for i := range s.rcaResponse.Incident.Graph.Nodes {
		if s.rcaResponse.Incident.Graph.Nodes[i].Resource.Kind == resourceKind {
			sourceNode = &s.rcaResponse.Incident.Graph.Nodes[i]
			break
		}
	}

	if sourceNode == nil {
		s.assert.Fail("Node with kind %s not found in graph", resourceKind)
		return s
	}

	// If no target kind specified, just check that the node exists
	if targetKind == "" {
		return s
	}

	// Find an edge from this node with the given relationship type
	found := false
	for _, edge := range s.rcaResponse.Incident.Graph.Edges {
		if edge.From == sourceNode.ID && edge.RelationshipType == relType {
			// Find the target node
			for _, node := range s.rcaResponse.Incident.Graph.Nodes {
				if node.ID == edge.To && node.Resource.Kind == targetKind {
					found = true
					break
				}
			}
		}
		if found {
			break
		}
	}

	s.assert.True(found, "Causal graph should include edge %s -[%s]-> %s", resourceKind, relType, targetKind)
	return s
}

func (s *RootCauseScenarioStage) confidence_score_exceeds(threshold float64) *RootCauseScenarioStage {
	s.assert.GreaterOrEqual(s.rcaResponse.Incident.Confidence.Score, threshold,
		"Confidence score should be >= %.2f (got %.2f)", threshold, s.rcaResponse.Incident.Confidence.Score)
	return s
}

func (s *RootCauseScenarioStage) confidence_score_in_range(min, max float64) *RootCauseScenarioStage {
	s.assert.GreaterOrEqual(s.rcaResponse.Incident.Confidence.Score, min,
		"Confidence score should be >= %.2f", min)
	s.assert.LessOrEqual(s.rcaResponse.Incident.Confidence.Score, max,
		"Confidence score should be <= %.2f", max)
	return s
}

func (s *RootCauseScenarioStage) confidence_factors_are_valid() *RootCauseScenarioStage {
	factors := s.rcaResponse.Incident.Confidence.Factors

	s.assert.GreaterOrEqual(factors.DirectSpecChange, 0.0)
	s.assert.LessOrEqual(factors.DirectSpecChange, 1.0)

	s.assert.GreaterOrEqual(factors.TemporalProximity, 0.0)
	s.assert.LessOrEqual(factors.TemporalProximity, 1.0)

	s.assert.GreaterOrEqual(factors.RelationshipStrength, 0.0)
	s.assert.LessOrEqual(factors.RelationshipStrength, 1.0)

	return s
}

func (s *RootCauseScenarioStage) supporting_evidence_includes_flux_labels() *RootCauseScenarioStage {
	foundFluxEvidence := false
	for _, evidence := range s.rcaResponse.SupportingEvidence {
		if evidence.Type == "RELATIONSHIP" &&
			(strings.Contains(evidence.Description, "helm.toolkit.fluxcd.io") ||
				strings.Contains(evidence.Description, "MANAGES")) {
			foundFluxEvidence = true
			break
		}
	}
	s.assert.True(foundFluxEvidence, "Supporting evidence should include Flux label matching")
	return s
}

func (s *RootCauseScenarioStage) temporal_proximity_is_high() *RootCauseScenarioStage {
	s.assert.GreaterOrEqual(s.rcaResponse.Incident.Confidence.Factors.TemporalProximity, 0.5,
		"Temporal proximity should be >= 0.5")
	return s
}

func (s *RootCauseScenarioStage) observed_symptom_is(symptomType string) *RootCauseScenarioStage {
	s.assert.Equal(symptomType, s.rcaResponse.Incident.ObservedSymptom.SymptomType,
		"Observed symptom should be %s", symptomType)
	return s
}

func (s *RootCauseScenarioStage) assert_graph_has_required_kinds() *RootCauseScenarioStage {
	helpers.RequireGraphNonEmpty(s.t, s.rcaResponse)

	expectedKinds := []string{
		"HelmRelease",
		"Deployment",
		"ReplicaSet",
		"Pod",
		"Node",
		"ServiceAccount",
		"ClusterRoleBinding",
	}
	helpers.RequireGraphHasKinds(s.t, s.rcaResponse, expectedKinds)
	return s
}

func (s *RootCauseScenarioStage) assert_graph_has_required_edges() *RootCauseScenarioStage {
	// Verify core ownership chain
	helpers.RequireGraphHasEdgeBetweenKinds(s.t, s.rcaResponse, "HelmRelease", "MANAGES", "Deployment")
	helpers.RequireGraphHasEdgeBetweenKinds(s.t, s.rcaResponse, "Deployment", "OWNS", "ReplicaSet")
	helpers.RequireGraphHasEdgeBetweenKinds(s.t, s.rcaResponse, "ReplicaSet", "OWNS", "Pod")

	// Verify attachment relationships
	helpers.RequireGraphHasEdgeBetweenKinds(s.t, s.rcaResponse, "Pod", "SCHEDULED_ON", "Node")
	helpers.RequireGraphHasEdgeBetweenKinds(s.t, s.rcaResponse, "Pod", "USES_SERVICE_ACCOUNT", "ServiceAccount")
	helpers.RequireGraphHasEdgeBetweenKinds(s.t, s.rcaResponse, "ServiceAccount", "GRANTS_TO", "ClusterRoleBinding")

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

	// Verify node has at least CREATE and UPDATE events
	helpers.RequireNodeHasEventTypes(s.t, statefulSetNode, []string{"CREATE", "UPDATE"})

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

func applyYAML(kubeconfig, yaml string) error {
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("manifest-%d.yaml", time.Now().UnixNano()))
	err := os.WriteFile(tmpFile, []byte(yaml), 0644)
	if err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	defer os.Remove(tmpFile)

	cmd := fmt.Sprintf("kubectl --kubeconfig=%s apply -f %s", kubeconfig, tmpFile)
	_, err = helpers.RunCommand(cmd)
	if err != nil {
		return fmt.Errorf("kubectl apply failed: %w", err)
	}
	return nil
}

func deleteResource(kubeconfig, kind, name, namespace string) error {
	cmd := fmt.Sprintf("kubectl --kubeconfig=%s delete %s %s -n %s --ignore-not-found=true",
		kubeconfig, kind, name, namespace)
	_, err := helpers.RunCommand(cmd)
	return err
}

func (s *RootCauseScenarioStage) loadFixture(name string) string {
	// Try to find the fixture file - check multiple possible locations
	possiblePaths := []string{
		filepath.Join("tests/e2e/fixtures", name),
		filepath.Join("e2e/fixtures", name),
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

func (s *RootCauseScenarioStage) applyManifest(manifestContent string) {
	// Write to temp file
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("manifest-%d.yaml", time.Now().UnixNano()))
	err := os.WriteFile(tmpFile, []byte(manifestContent), 0644)
	s.require.NoError(err, "Failed to write temp manifest")
	defer os.Remove(tmpFile)

	// Apply with kubectl using the correct kubeconfig
	kubeconfig := s.testCtx.Cluster.GetKubeConfig()
	cmd := fmt.Sprintf("kubectl --kubeconfig=%s apply -f %s", kubeconfig, tmpFile)
	output, err := helpers.RunCommand(cmd)
	s.require.NoError(err, "Failed to apply manifest: %s\nOutput: %s", cmd, output)
}

func (s *RootCauseScenarioStage) extractHelmReleaseInfo(manifestContent string) {
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifestContent), 4096)

	for {
		var obj unstructured.Unstructured
		err := decoder.Decode(&obj)
		if err != nil {
			break
		}

		if obj.GetKind() == "HelmRelease" {
			s.helmReleaseName = obj.GetName()
			s.helmReleaseNs = obj.GetNamespace()
			return
		}
	}
}

func (s *RootCauseScenarioStage) extractDeploymentInfo(manifestContent string) {
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifestContent), 4096)

	for {
		var obj unstructured.Unstructured
		err := decoder.Decode(&obj)
		if err != nil {
			break
		}

		if obj.GetKind() == "Deployment" {
			s.deploymentName = obj.GetName()
			s.deploymentNs = obj.GetNamespace()
			return
		}
	}
}

func (s *RootCauseScenarioStage) cleanup() {
	ctx := context.Background()

	// Clean up individual resources first
	for _, res := range s.resourcesToCleanup {
		s.t.Logf("Cleaning up %s/%s in namespace %s", res.kind, res.name, res.namespace)
		_ = deleteResource(s.testCtx.Cluster.GetKubeConfig(), res.kind, res.name, res.namespace)
	}

	// Then clean up namespaces
	for _, ns := range s.namespacesToCleanup {
		s.t.Logf("Cleaning up namespace: %s", ns)
		_ = s.testCtx.K8sClient.DeleteNamespace(ctx, ns)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
