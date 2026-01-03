package e2e

import (
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	appsv1 "k8s.io/api/apps/v1"
)

type PodRestartStage struct {
	helpers.BaseStage

	testNamespace string
	deployment1   *appsv1.Deployment
	deployment2   *appsv1.Deployment

	// Helper managers
	nsManager *helpers.NamespaceManager
	ctxHelper *helpers.ContextHelper
}

func NewPodRestartStage(t *testing.T) (*PodRestartStage, *PodRestartStage, *PodRestartStage) {
	s := &PodRestartStage{
		BaseStage: helpers.NewBaseStage(t),
	}
	return s, s, s
}

func (s *PodRestartStage) and() *PodRestartStage {
	return s
}

func (s *PodRestartStage) a_test_environment() *PodRestartStage {
	s.BaseStage.SetupTestEnvironment()

	// Initialize helper managers
	s.nsManager = helpers.NewNamespaceManager(s.T, s.K8sClient)
	s.ctxHelper = helpers.NewContextHelper(s.T)

	return s
}

func (s *PodRestartStage) a_test_namespace_with_deployment() *PodRestartStage {
	ctx, cancel := s.ctxHelper.WithDefaultTimeout()
	defer cancel()

	// Use namespace manager to create namespace with automatic cleanup
	namespace, err := s.nsManager.CreateNamespaceWithRandomSuffix(ctx, "test-restart")
	s.Require.NoError(err, "failed to create namespace")
	s.testNamespace = namespace

	deployment, err := helpers.CreateTestDeployment(ctx, s.T, s.K8sClient, s.testNamespace)
	s.Require.NoError(err, "failed to create first deployment")
	s.deployment1 = deployment
	return s
}

func (s *PodRestartStage) deployment_is_indexed() *PodRestartStage {
	resource1 := helpers.EventuallyResourceCreated(s.T, s.APIClient, s.testNamespace, "Deployment", s.deployment1.Name, helpers.DefaultEventuallyOption)
	s.Require.NotNil(resource1)
	return s
}

func (s *PodRestartStage) spectre_pod_is_restarted() *PodRestartStage {
	ctx, cancel := s.ctxHelper.WithDefaultTimeout()
	defer cancel()

	podList, err := s.TestCtx.K8sClient.ListPods(ctx, s.TestCtx.Namespace, "app.kubernetes.io/instance="+s.TestCtx.ReleaseName)
	s.Require.NoError(err, "failed to list pods")
	s.Require.Greater(len(podList.Items), 0, "should have at least one pod")

	podName := podList.Items[0].Name
	if err := s.TestCtx.K8sClient.DeletePod(ctx, s.TestCtx.Namespace, podName); err != nil {
		s.T.Logf("Warning: failed to delete pod %s: %v", podName, err)
	}
	return s
}

func (s *PodRestartStage) wait_for_spectre_to_be_ready() *PodRestartStage {
	ctx, cancel := s.ctxHelper.WithLongTimeout()
	defer cancel()

	err := helpers.WaitForAppReady(ctx, s.TestCtx.K8sClient, s.TestCtx.Namespace, s.TestCtx.ReleaseName)
	s.Require.NoError(err, "failed to wait for app to be ready")
	return s
}

func (s *PodRestartStage) port_forward_is_reconnected() *PodRestartStage {
	err := s.TestCtx.ReconnectPortForward()
	s.Require.NoError(err, "failed to reconnect port-forward after pod restart")
	s.APIClient = s.TestCtx.APIClient
	return s
}

func (s *PodRestartStage) first_deployment_is_still_present() *PodRestartStage {
	resource1 := helpers.EventuallyResourceCreated(s.T, s.APIClient, s.testNamespace, "Deployment", s.deployment1.Name, helpers.DefaultEventuallyOption)
	s.Require.NotNil(resource1)
	return s
}

func (s *PodRestartStage) second_deployment_is_created() *PodRestartStage {
	ctx, cancel := s.ctxHelper.WithDefaultTimeout()
	defer cancel()

	deployment2Builder := helpers.NewDeploymentBuilder(s.T, "test-deployment-2", s.testNamespace)
	deployment2 := deployment2Builder.WithReplicas(1).Build()
	deployment2Created, err := s.K8sClient.CreateDeployment(ctx, s.testNamespace, deployment2)
	s.Require.NoError(err, "failed to create second deployment")
	s.deployment2 = deployment2Created
	return s
}

func (s *PodRestartStage) second_deployment_is_indexed() *PodRestartStage {
	resource2 := helpers.EventuallyResourceCreated(s.T, s.APIClient, s.testNamespace, "Deployment", s.deployment2.Name, helpers.DefaultEventuallyOption)
	s.Require.NotNil(resource2)
	return s
}

func (s *PodRestartStage) both_deployments_are_searchable() *PodRestartStage {
	ctx, cancel := s.ctxHelper.WithDefaultTimeout()
	defer cancel()

	searchResp, err := s.APIClient.Search(ctx, time.Now().Unix()-120, time.Now().Unix()+10, s.testNamespace, "Deployment")
	s.Require.NoError(err)

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

	s.Assert.True(foundDeployment1, "First deployment should still be found in search")
	s.Assert.True(foundDeployment2, "Second deployment should be found in search")

	s.T.Log("✓ Pod restart scenario completed successfully!")
	return s
}
