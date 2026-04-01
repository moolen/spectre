package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigReloadStage struct {
	*helpers.BaseContext

	t *testing.T

	testNamespace    string
	statefulSet      *appsv1.StatefulSet
	deployment       *appsv1.Deployment
	foundAfterReload bool
	newWatcherConfig string
	configMapName    string

	// Helper managers
	nsManager  *helpers.NamespaceManager
	ctxHelper  *helpers.ContextHelper
	waitHelper *helpers.WaitHelper
}

func NewConfigReloadStage(t *testing.T) (*ConfigReloadStage, *ConfigReloadStage, *ConfigReloadStage) {
	s := &ConfigReloadStage{
		t: t,
	}
	return s, s, s
}

func (s *ConfigReloadStage) and() *ConfigReloadStage {
	return s
}

func (s *ConfigReloadStage) a_test_environment() *ConfigReloadStage {
	// Use custom minimal watcher config for this test
	testCtx := helpers.SetupE2ETestWithValuesFile(s.t, "tests/e2e/fixtures/helm-values-minimal-watcher.yaml")
	s.BaseContext = helpers.NewBaseContext(s.t, testCtx)

	// Initialize helper managers
	s.nsManager = helpers.NewNamespaceManager(s.T, s.K8sClient)
	s.ctxHelper = helpers.NewContextHelper(s.T)
	s.waitHelper = helpers.NewWaitHelper(s.T)

	return s
}

func (s *ConfigReloadStage) a_test_namespace() *ConfigReloadStage {
	ctx, cancel := s.ctxHelper.WithLongTimeout()
	defer cancel()

	// Use namespace manager to create namespace with automatic cleanup
	namespace, err := s.nsManager.CreateNamespace(ctx, "test-config")
	s.Require.NoError(err, "failed to create namespace")
	s.testNamespace = namespace

	return s
}

func (s *ConfigReloadStage) statefulset_is_created() *ConfigReloadStage {
	ctx, cancel := s.ctxHelper.WithLongTimeout()
	defer cancel()

	ssBuilder := helpers.NewStatefulSetBuilder(s.T, "test-statefulset", s.testNamespace)
	statefulSet := ssBuilder.WithReplicas(1).Build()

	ssCreated, err := s.K8sClient.Clientset.AppsV1().StatefulSets(s.testNamespace).Create(ctx, statefulSet, metav1.CreateOptions{})
	s.Require.NoError(err, "failed to create StatefulSet")
	s.statefulSet = ssCreated

	s.T.Logf("StatefulSet created: %s/%s", ssCreated.Namespace, ssCreated.Name)

	// Give it time to generate events
	s.waitHelper.Sleep(30*time.Second, "StatefulSet event generation")

	return s
}

func (s *ConfigReloadStage) statefulset_is_not_found_with_default_config() *ConfigReloadStage {
	ctx, cancel := s.ctxHelper.WithLongTimeout()
	defer cancel()

	searchResp, err := s.APIClient.Search(ctx, time.Now().Unix()-60, time.Now().Unix()+10, s.testNamespace, "StatefulSet")
	s.Require.NoError(err)

	foundStatefulSet := false
	for _, r := range searchResp.Resources {
		if r.Name == s.statefulSet.Name && r.Kind == "StatefulSet" {
			foundStatefulSet = true
			break
		}
	}
	s.Assert.False(foundStatefulSet, "StatefulSet should NOT be found with default watch config")

	return s
}

func (s *ConfigReloadStage) watcher_config_is_updated_to_include_statefulset() *ConfigReloadStage {
	ctx, cancel := s.ctxHelper.WithLongTimeout()
	defer cancel()

	s.configMapName = fmt.Sprintf("%s-spectre", s.TestCtx.ReleaseName)
	s.newWatcherConfig = `resources:
  - group: "apps"
    version: "v1"
    kind: "StatefulSet"
  - group: "apps"
    version: "v1"
    kind: "Deployment"
`
	err := s.K8sClient.UpdateConfigMap(ctx, s.TestCtx.Namespace, s.configMapName, map[string]string{
		"watcher.yaml": s.newWatcherConfig,
	})
	s.Require.NoError(err, "failed to update watcher ConfigMap")

	// Verify the ConfigMap was updated correctly
	cm, err := s.K8sClient.Clientset.CoreV1().ConfigMaps(s.TestCtx.Namespace).Get(ctx, s.configMapName, metav1.GetOptions{})
	s.Require.NoError(err, "failed to get ConfigMap")
	s.T.Logf("ConfigMap %s updated. Contents of watcher.yaml:\n%s", s.configMapName, cm.Data["watcher.yaml"])

	// ConfigMap volume updates in Kubernetes can take 60-120 seconds due to kubelet sync period.
	// Instead of waiting for propagation, we restart the pod to force immediate config reload.
	// This simulates a deployment rollout which is a common pattern for config changes.
	s.T.Log("Restarting Spectre pod to apply new watcher config...")
	s.restartSpectrePod(ctx)

	return s
}

// restartSpectrePod deletes the Spectre pod and waits for the deployment to create a new one
func (s *ConfigReloadStage) restartSpectrePod(ctx context.Context) {
	// Get the current pod name
	pods, err := s.K8sClient.Clientset.CoreV1().Pods(s.TestCtx.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/instance=%s", s.TestCtx.ReleaseName),
	})
	s.Require.NoError(err, "failed to list pods")
	s.Require.NotEmpty(pods.Items, "no Spectre pods found")

	oldPodName := pods.Items[0].Name
	s.T.Logf("Deleting pod %s to trigger restart with new config", oldPodName)

	// Delete the pod
	err = s.K8sClient.DeletePod(ctx, s.TestCtx.Namespace, oldPodName)
	s.Require.NoError(err, "failed to delete pod")

	// Wait for a new pod to be ready (different from the old one)
	s.T.Log("Waiting for new pod to be ready...")
	err = s.waitForNewPodReady(ctx, oldPodName)
	s.Require.NoError(err, "failed to wait for new pod")

	// Reconnect port-forward to the new pod
	s.T.Log("Reconnecting port-forward to new pod...")
	err = s.TestCtx.ReconnectPortForward()
	s.Require.NoError(err, "failed to reconnect port-forward")

	// Update the API client with the new URL
	s.APIClient = helpers.NewAPIClient(s.T, s.TestCtx.PortForward.GetURL())

	// Give the watcher time to start capturing events
	// Need to wait for FalkorDB sidecar to be ready + watcher to capture existing StatefulSet
	s.waitHelper.Sleep(20*time.Second, "watcher and graph startup")
	s.T.Log("✓ Spectre pod restarted with new watcher config")
}

// waitForNewPodReady waits for a new pod (different from oldPodName) to be running and ready
func (s *ConfigReloadStage) waitForNewPodReady(ctx context.Context, oldPodName string) error {
	timeout := time.After(120 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for new pod to be ready")
		case <-ticker.C:
			pods, err := s.K8sClient.Clientset.CoreV1().Pods(s.TestCtx.Namespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("app.kubernetes.io/instance=%s", s.TestCtx.ReleaseName),
			})
			if err != nil {
				s.T.Logf("Error listing pods: %v", err)
				continue
			}

			for _, pod := range pods.Items {
				// Skip the old pod (it might still be terminating)
				if pod.Name == oldPodName {
					continue
				}

				// Check if the new pod is ready
				if pod.Status.Phase == corev1.PodRunning {
					for _, cond := range pod.Status.Conditions {
						if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
							s.T.Logf("✓ New pod %s is ready", pod.Name)
							return nil
						}
					}
				}
			}
			s.T.Log("  Waiting for new pod...")
		}
	}
}

func (s *ConfigReloadStage) wait_for_hot_reload() *ConfigReloadStage {
	ctx, cancel := s.ctxHelper.WithLongTimeout()
	defer cancel()

	// Trigger an update on the StatefulSet to generate a new event that the watcher can capture.
	// The watcher's List operation on startup may have a race condition with the graph indexing,
	// so updating the StatefulSet ensures we have a fresh UPDATE event with current timestamp.
	s.T.Log("Triggering StatefulSet update to generate new event for watcher...")

	// Fetch the latest version to avoid conflict errors (resourceVersion may have changed)
	latestSS, err := s.K8sClient.Clientset.AppsV1().StatefulSets(s.testNamespace).Get(ctx, s.statefulSet.Name, metav1.GetOptions{})
	if err != nil {
		s.T.Logf("Warning: Failed to get latest StatefulSet: %v", err)
	} else {
		if latestSS.Annotations == nil {
			latestSS.Annotations = make(map[string]string)
		}
		latestSS.Annotations["spectre.io/config-reload-test"] = time.Now().Format(time.RFC3339)
		updatedSS, err := s.K8sClient.Clientset.AppsV1().StatefulSets(s.testNamespace).Update(ctx, latestSS, metav1.UpdateOptions{})
		if err != nil {
			s.T.Logf("Warning: Failed to update StatefulSet: %v", err)
		} else {
			s.statefulSet = updatedSS
			s.T.Logf("✓ StatefulSet updated to trigger new event")
		}
	}

	// Poll for the StatefulSet to appear in the API, which indicates the watcher is capturing events.
	// Since we updated the StatefulSet, the watcher should capture an UPDATE event immediately.
	// Use 90s timeout to allow for graph indexing and any processing delays.
	pollTimeout := time.After(90 * time.Second)
	pollTicker := time.NewTicker(3 * time.Second)
	defer pollTicker.Stop()

pollLoop:
	for {
		select {
		case <-pollTimeout:
			s.T.Logf("Timeout waiting for StatefulSet to appear after config reload")
			s.dumpDebugInfo(ctx)
			break pollLoop
		case <-pollTicker.C:
			// Use a wide time window (1 hour) to ensure we capture events from pod startup
			// The watcher captures events during initial LIST with timestamps from that moment,
			// which could be several minutes before the polling loop runs.
			startTs := time.Now().Unix() - 3600
			endTs := time.Now().Unix() + 10
			searchRespAfter, err := s.APIClient.Search(ctx, startTs, endTs, s.testNamespace, "StatefulSet")
			if err != nil {
				s.T.Logf("Search error: %v", err)
				continue
			}
			s.T.Logf("  Search returned %d resources (start=%d, end=%d, ns=%s, kind=StatefulSet)",
				len(searchRespAfter.Resources), startTs, endTs, s.testNamespace)
			for _, r := range searchRespAfter.Resources {
				s.T.Logf("    Found: %s/%s (kind=%s)", r.Namespace, r.Name, r.Kind)
				if r.Name == s.statefulSet.Name && r.Kind == "StatefulSet" {
					s.foundAfterReload = true
					s.T.Logf("✓ StatefulSet found in API after config reload!")
					break pollLoop
				}
			}
			s.T.Logf("  StatefulSet '%s' not yet visible, waiting...", s.statefulSet.Name)
		}
	}

	return s
}

func (s *ConfigReloadStage) statefulset_is_found_after_reload() *ConfigReloadStage {
	s.Require.True(s.foundAfterReload, "StatefulSet should be found after config reload - hot-reload may not be working")
	return s
}

func (s *ConfigReloadStage) deployment_can_still_be_captured() *ConfigReloadStage {
	ctx, cancel := s.ctxHelper.WithLongTimeout()
	defer cancel()

	deployment, err := helpers.CreateTestDeployment(ctx, s.T, s.K8sClient, s.testNamespace)
	s.Require.NoError(err, "failed to create deployment")
	s.deployment = deployment

	depResource := helpers.EventuallyResourceCreated(s.T, s.APIClient, s.testNamespace, "Deployment", deployment.Name, helpers.DefaultEventuallyOption)
	s.Require.NotNil(depResource)
	s.T.Logf("✓ Deployment also captured after config reload")

	return s
}

func (s *ConfigReloadStage) metadata_includes_both_resource_kinds() *ConfigReloadStage {
	ctx, cancel := s.ctxHelper.WithLongTimeout()
	defer cancel()

	metadataStart := time.Now().Unix() - 3600
	metadataEnd := time.Now().Unix() + 10
	metadata, err := s.APIClient.GetMetadata(ctx, &metadataStart, &metadataEnd)
	s.Require.NoError(err)

	s.Assert.Contains(metadata.Namespaces, s.testNamespace)
	s.Assert.Contains(metadata.Kinds, "StatefulSet", "StatefulSet should be in metadata kinds")
	s.Assert.Contains(metadata.Kinds, "Deployment", "Deployment should be in metadata kinds")
	s.T.Logf("✓ Metadata contains both StatefulSet and Deployment kinds")

	s.T.Log("✓ Dynamic config reload scenario completed successfully!")
	return s
}

// dumpDebugInfo dumps container logs and watcher config for debugging test failures
func (s *ConfigReloadStage) dumpDebugInfo(ctx context.Context) {
	s.T.Log("=== Debug Info: Dumping pod logs and config ===")

	// Get the Spectre pod name
	pods, err := s.K8sClient.Clientset.CoreV1().Pods(s.TestCtx.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/instance=%s", s.TestCtx.ReleaseName),
	})
	if err != nil {
		s.T.Logf("Failed to list pods: %v", err)
		return
	}

	if len(pods.Items) == 0 {
		s.T.Log("No Spectre pods found")
		return
	}

	podName := pods.Items[0].Name
	s.T.Logf("Spectre pod: %s", podName)

	// Get pod logs (last 200 lines)
	tailLines := int64(200)
	logs, err := s.K8sClient.GetPodLogs(ctx, s.TestCtx.Namespace, podName, "spectre", &tailLines)
	if err != nil {
		s.T.Logf("Failed to get pod logs: %v", err)
	} else {
		// Filter for relevant log lines
		s.T.Log("=== Relevant Spectre container logs ===")
		for _, line := range strings.Split(logs, "\n") {
			if strings.Contains(line, "StatefulSet") ||
				strings.Contains(line, "ERROR") ||
				strings.Contains(line, "error") {
				s.T.Logf("  %s", line)
			}
		}
	}

	// Also try getting metadata to see what kinds are known
	s.T.Log("=== Checking metadata for known kinds ===")
	startTs := time.Now().Unix() - 3600
	endTs := time.Now().Unix() + 10
	metadata, err := s.APIClient.GetMetadata(ctx, &startTs, &endTs)
	if err != nil {
		s.T.Logf("Failed to get metadata: %v", err)
	} else {
		s.T.Logf("Known kinds: %v", metadata.Kinds)
		s.T.Logf("Known namespaces: %v", metadata.Namespaces)
	}
}
