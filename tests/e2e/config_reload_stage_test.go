package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigReloadStage struct {
	t         *testing.T
	require   *require.Assertions
	assert    *assert.Assertions
	testCtx   *helpers.TestContext
	k8sClient *helpers.K8sClient
	apiClient *helpers.APIClient

	testNamespace      string
	statefulSet        *appsv1.StatefulSet
	deployment         *appsv1.Deployment
	foundAfterReload   bool
	newWatcherConfig   string
	configMapName      string
}

func NewConfigReloadStage(t *testing.T) (*ConfigReloadStage, *ConfigReloadStage, *ConfigReloadStage) {
	s := &ConfigReloadStage{
		t:       t,
		require: require.New(t),
		assert:  assert.New(t),
	}
	return s, s, s
}

func (s *ConfigReloadStage) and() *ConfigReloadStage {
	return s
}

func (s *ConfigReloadStage) a_test_environment() *ConfigReloadStage {
	s.testCtx = helpers.SetupE2ETest(s.t)
	s.k8sClient = s.testCtx.K8sClient
	s.apiClient = s.testCtx.APIClient
	return s
}

func (s *ConfigReloadStage) a_test_namespace() *ConfigReloadStage {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	s.testNamespace = "test-config"
	err := s.k8sClient.CreateNamespace(ctx, s.testNamespace)
	s.require.NoError(err, "failed to create namespace")
	s.t.Cleanup(func() {
		if err := s.k8sClient.DeleteNamespace(context.Background(), s.testNamespace); err != nil {
			s.t.Logf("Warning: failed to delete namespace: %v", err)
		}
	})

	return s
}

func (s *ConfigReloadStage) statefulset_is_created() *ConfigReloadStage {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ssBuilder := helpers.NewStatefulSetBuilder(s.t, "test-statefulset", s.testNamespace)
	statefulSet := ssBuilder.WithReplicas(1).Build()

	ssCreated, err := s.k8sClient.Clientset.AppsV1().StatefulSets(s.testNamespace).Create(ctx, statefulSet, metav1.CreateOptions{})
	s.require.NoError(err, "failed to create StatefulSet")
	s.statefulSet = ssCreated

	s.t.Logf("StatefulSet created: %s/%s", ssCreated.Namespace, ssCreated.Name)

	// Give it time to generate events
	time.Sleep(30 * time.Second)

	return s
}

func (s *ConfigReloadStage) statefulset_is_not_found_with_default_config() *ConfigReloadStage {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	searchResp, err := s.apiClient.Search(ctx, time.Now().Unix()-60, time.Now().Unix()+10, s.testNamespace, "StatefulSet")
	s.require.NoError(err)

	foundStatefulSet := false
	for _, r := range searchResp.Resources {
		if r.Name == s.statefulSet.Name && r.Kind == "StatefulSet" {
			foundStatefulSet = true
			break
		}
	}
	s.assert.False(foundStatefulSet, "StatefulSet should NOT be found with default watch config")

	return s
}

func (s *ConfigReloadStage) watcher_config_is_updated_to_include_statefulset() *ConfigReloadStage {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	s.configMapName = fmt.Sprintf("%s-spectre", s.testCtx.ReleaseName)
	s.newWatcherConfig = `resources:
  - group: "apps"
    version: "v1"
    kind: "StatefulSet"
  - group: "apps"
    version: "v1"
    kind: "Deployment"
`
	err := s.k8sClient.UpdateConfigMap(ctx, s.testCtx.Namespace, s.configMapName, map[string]string{
		"watcher.yaml": s.newWatcherConfig,
	})
	s.require.NoError(err, "failed to update watcher ConfigMap")
	s.t.Logf("Waiting for ConfigMap propagation and hot-reload (up to 90 seconds)...")

	return s
}

func (s *ConfigReloadStage) wait_for_hot_reload() *ConfigReloadStage {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Poll for the StatefulSet to appear in the API, which indicates hot-reload worked
	pollTimeout := time.After(90 * time.Second)
	pollTicker := time.NewTicker(5 * time.Second)
	defer pollTicker.Stop()

pollLoop:
	for {
		select {
		case <-pollTimeout:
			s.t.Logf("Timeout waiting for StatefulSet to appear after config reload")
			break pollLoop
		case <-pollTicker.C:
			searchRespAfter, err := s.apiClient.Search(ctx, time.Now().Unix()-500, time.Now().Unix()+10, s.testNamespace, "StatefulSet")
			if err != nil {
				s.t.Logf("Search error: %v", err)
				continue
			}
			for _, r := range searchRespAfter.Resources {
				if r.Name == s.statefulSet.Name && r.Kind == "StatefulSet" {
					s.foundAfterReload = true
					s.t.Logf("✓ StatefulSet found in API after config reload!")
					break pollLoop
				}
			}
			s.t.Logf("  StatefulSet not yet visible, waiting...")
		}
	}

	return s
}

func (s *ConfigReloadStage) statefulset_is_found_after_reload() *ConfigReloadStage {
	s.require.True(s.foundAfterReload, "StatefulSet should be found after config reload - hot-reload may not be working")
	return s
}

func (s *ConfigReloadStage) deployment_can_still_be_captured() *ConfigReloadStage {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	deployment, err := helpers.CreateTestDeployment(ctx, s.t, s.k8sClient, s.testNamespace)
	s.require.NoError(err, "failed to create deployment")
	s.deployment = deployment

	depResource := helpers.EventuallyResourceCreated(s.t, s.apiClient, s.testNamespace, "Deployment", deployment.Name, helpers.DefaultEventuallyOption)
	s.require.NotNil(depResource)
	s.t.Logf("✓ Deployment also captured after config reload")

	return s
}

func (s *ConfigReloadStage) metadata_includes_both_resource_kinds() *ConfigReloadStage {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	metadataStart := time.Now().Unix() - 500
	metadataEnd := time.Now().Unix() + 10
	metadata, err := s.apiClient.GetMetadata(ctx, &metadataStart, &metadataEnd)
	s.require.NoError(err)

	s.assert.Contains(metadata.Namespaces, s.testNamespace)
	s.assert.Contains(metadata.Kinds, "StatefulSet", "StatefulSet should be in metadata kinds")
	s.assert.Contains(metadata.Kinds, "Deployment", "Deployment should be in metadata kinds")
	s.t.Logf("✓ Metadata contains both StatefulSet and Deployment kinds")

	s.t.Log("✓ Dynamic config reload scenario completed successfully!")
	return s
}
