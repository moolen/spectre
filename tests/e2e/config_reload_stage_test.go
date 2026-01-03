package e2e

import (
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigReloadStage struct {
	helpers.BaseStage

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
		BaseStage: helpers.NewBaseStage(t),
	}
	return s, s, s
}

func (s *ConfigReloadStage) and() *ConfigReloadStage {
	return s
}

func (s *ConfigReloadStage) a_test_environment() *ConfigReloadStage {
	// Use custom minimal watcher config for this test
	s.BaseStage.SetupTestEnvironmentWithValues("tests/e2e/fixtures/helm-values-minimal-watcher.yaml")

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
	namespace, err := s.nsManager.CreateNamespaceWithRandomSuffix(ctx, "test-config")
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

	s.configMapName = s.TestCtx.ReleaseName + "-spectre"
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
	s.T.Logf("Waiting for ConfigMap propagation and hot-reload (up to 90 seconds)...")

	return s
}

func (s *ConfigReloadStage) wait_for_hot_reload() *ConfigReloadStage {
	ctx, cancel := s.ctxHelper.WithLongTimeout()
	defer cancel()

	// Poll for the StatefulSet to appear in the API, which indicates hot-reload worked
	pollTimeout := time.After(90 * time.Second)
	pollTicker := time.NewTicker(5 * time.Second)
	defer pollTicker.Stop()

pollLoop:
	for {
		select {
		case <-pollTimeout:
			s.T.Logf("Timeout waiting for StatefulSet to appear after config reload")
			break pollLoop
		case <-pollTicker.C:
			searchRespAfter, err := s.APIClient.Search(ctx, time.Now().Unix()-500, time.Now().Unix()+10, s.testNamespace, "StatefulSet")
			if err != nil {
				s.T.Logf("Search error: %v", err)
				continue
			}
			for _, r := range searchRespAfter.Resources {
				if r.Name == s.statefulSet.Name && r.Kind == "StatefulSet" {
					s.foundAfterReload = true
					s.T.Logf("✓ StatefulSet found in API after config reload!")
					break pollLoop
				}
			}
			s.T.Logf("  StatefulSet not yet visible, waiting...")
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

	metadataStart := time.Now().Unix() - 500
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
