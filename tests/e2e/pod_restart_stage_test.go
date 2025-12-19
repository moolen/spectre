package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
)

type PodRestartStage struct {
	t         *testing.T
	require   *require.Assertions
	assert    *assert.Assertions
	testCtx   *helpers.TestContext
	k8sClient *helpers.K8sClient
	apiClient *helpers.APIClient

	testNamespace string
	deployment1   *appsv1.Deployment
	deployment2   *appsv1.Deployment
}

func NewPodRestartStage(t *testing.T) (*PodRestartStage, *PodRestartStage, *PodRestartStage) {
	s := &PodRestartStage{
		t:       t,
		require: require.New(t),
		assert:  assert.New(t),
	}
	return s, s, s
}

func (s *PodRestartStage) and() *PodRestartStage {
	return s
}

func (s *PodRestartStage) a_test_environment() *PodRestartStage {
	s.testCtx = helpers.SetupE2ETest(s.t)
	s.k8sClient = s.testCtx.K8sClient
	s.apiClient = s.testCtx.APIClient
	return s
}

func (s *PodRestartStage) a_test_namespace_with_deployment() *PodRestartStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 2*time.Minute)
	defer cancel()

	// Generate unique namespace name to avoid collisions with cluster reuse
	suffix := rand.Intn(999999)
	s.testNamespace = fmt.Sprintf("test-restart-%d", suffix)
	err := s.k8sClient.CreateNamespace(ctx, s.testNamespace)
	s.require.NoError(err, "failed to create namespace")
	s.t.Cleanup(func() {
		if err := s.k8sClient.DeleteNamespace(s.t.Context(), s.testNamespace); err != nil {
			s.t.Logf("Warning: failed to delete namespace: %v", err)
		}
	})

	s.deployment1, err = helpers.CreateTestDeployment(ctx, s.t, s.k8sClient, s.testNamespace)
	s.require.NoError(err, "failed to create first deployment")
	return s
}

func (s *PodRestartStage) deployment_is_indexed() *PodRestartStage {
	resource1 := helpers.EventuallyResourceCreated(s.t, s.apiClient, s.testNamespace, "Deployment", s.deployment1.Name, helpers.DefaultEventuallyOption)
	s.require.NotNil(resource1)
	return s
}

func (s *PodRestartStage) spectre_pod_is_restarted() *PodRestartStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 2*time.Minute)
	defer cancel()

	podList, err := s.testCtx.K8sClient.ListPods(ctx, s.testCtx.Namespace, "app.kubernetes.io/instance="+s.testCtx.ReleaseName)
	s.require.NoError(err, "failed to list pods")
	s.require.Greater(len(podList.Items), 0, "should have at least one pod")

	podName := podList.Items[0].Name
	if err := s.testCtx.K8sClient.DeletePod(ctx, s.testCtx.Namespace, podName); err != nil {
		s.t.Logf("Warning: failed to delete pod %s: %v", podName, err)
	}
	return s
}

func (s *PodRestartStage) wait_for_spectre_to_be_ready() *PodRestartStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 5*time.Minute)
	defer cancel()

	err := helpers.WaitForAppReady(ctx, s.testCtx.K8sClient, s.testCtx.Namespace, s.testCtx.ReleaseName)
	s.require.NoError(err, "failed to wait for app to be ready")
	return s
}

func (s *PodRestartStage) port_forward_is_reconnected() *PodRestartStage {
	err := s.testCtx.ReconnectPortForward()
	s.require.NoError(err, "failed to reconnect port-forward after pod restart")
	s.apiClient = s.testCtx.APIClient
	return s
}

func (s *PodRestartStage) first_deployment_is_still_present() *PodRestartStage {
	resource1 := helpers.EventuallyResourceCreated(s.t, s.apiClient, s.testNamespace, "Deployment", s.deployment1.Name, helpers.DefaultEventuallyOption)
	s.require.NotNil(resource1)
	return s
}

func (s *PodRestartStage) second_deployment_is_created() *PodRestartStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 2*time.Minute)
	defer cancel()

	deployment2Builder := helpers.NewDeploymentBuilder(s.t, "test-deployment-2", s.testNamespace)
	deployment2 := deployment2Builder.WithReplicas(1).Build()
	deployment2Created, err := s.k8sClient.CreateDeployment(ctx, s.testNamespace, deployment2)
	s.require.NoError(err, "failed to create second deployment")
	s.deployment2 = deployment2Created
	return s
}

func (s *PodRestartStage) second_deployment_is_indexed() *PodRestartStage {
	resource2 := helpers.EventuallyResourceCreated(s.t, s.apiClient, s.testNamespace, "Deployment", s.deployment2.Name, helpers.DefaultEventuallyOption)
	s.require.NotNil(resource2)
	return s
}

func (s *PodRestartStage) both_deployments_are_searchable() *PodRestartStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 2*time.Minute)
	defer cancel()

	searchResp, err := s.apiClient.Search(ctx, time.Now().Unix()-120, time.Now().Unix()+10, s.testNamespace, "Deployment")
	s.require.NoError(err)

	foundDeployment1 := false
	foundDeployment2 := false

	for _, r := range searchResp.Resources {
		if r.Name == s.deployment1.Name {
			foundDeployment1 = true
		}
		if r.Name == s.deployment2.Name {
			foundDeployment2 = true
		}
	}

	s.assert.True(foundDeployment1, "First deployment should still be found in search")
	s.assert.True(foundDeployment2, "Second deployment should be found in search")

	s.t.Log("✓ Pod restart scenario completed successfully!")
	return s
}
