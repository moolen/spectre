package e2e

import (
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	appsv1 "k8s.io/api/apps/v1"
)

type DefaultResourcesStage struct {
	*helpers.BaseContext

	t *testing.T

	testNamespace1 string
	testNamespace2 string
	deployment     *appsv1.Deployment
	resourceID     string

	// Helper managers
	nsManager *helpers.NamespaceManager
	ctxHelper *helpers.ContextHelper
}

func NewDefaultResourcesStage(t *testing.T) (*DefaultResourcesStage, *DefaultResourcesStage, *DefaultResourcesStage) {
	s := &DefaultResourcesStage{
		t: t,
	}
	return s, s, s
}

func (s *DefaultResourcesStage) and() *DefaultResourcesStage {
	return s
}

func (s *DefaultResourcesStage) a_test_environment() *DefaultResourcesStage {
	testCtx := helpers.SetupE2ETestShared(s.t)
	s.BaseContext = helpers.NewBaseContext(s.t, testCtx)

	// Initialize helper managers
	s.nsManager = helpers.NewNamespaceManager(s.T, s.K8sClient)
	s.ctxHelper = helpers.NewContextHelper(s.T)

	return s
}

func (s *DefaultResourcesStage) two_test_namespaces() *DefaultResourcesStage {
	ctx, cancel := s.ctxHelper.WithDefaultTimeout()
	defer cancel()

	version, err := s.K8sClient.GetClusterVersion(ctx)
	s.Require.NoError(err, "failed to get cluster version")
	s.T.Logf("Kubernetes version: %s", version)

	// Use namespace manager to create multiple namespaces with automatic cleanup
	namespaces, err := s.nsManager.CreateMultipleNamespaces(ctx, []string{"test-default", "test-alternate"})
	s.Require.NoError(err, "failed to create namespaces")

	s.testNamespace1 = namespaces["test-default"]
	s.testNamespace2 = namespaces["test-alternate"]

	return s
}

func (s *DefaultResourcesStage) deployment_is_created_in_first_namespace() *DefaultResourcesStage {
	ctx, cancel := s.ctxHelper.WithDefaultTimeout()
	defer cancel()

	deployment, err := helpers.CreateTestDeployment(ctx, s.T, s.K8sClient, s.testNamespace1)
	s.Require.NoError(err, "failed to create test deployment")
	s.Require.NotNil(deployment)
	s.deployment = deployment

	s.T.Logf("Deployment created: %s/%s (UID: %s)", deployment.Namespace, deployment.Name, deployment.UID)

	// Wait for deployment to be ready with all replicas available
	err = s.K8sClient.WaitForDeploymentReady(ctx, s.testNamespace1, deployment.Name, 2*time.Minute)
	if err != nil {
		s.T.Logf("Warning: deployment ready check failed (expected in e2e): %v", err)
	}

	return s
}

func (s *DefaultResourcesStage) deployment_is_indexed() *DefaultResourcesStage {
	resource := helpers.EventuallyResourceCreated(s.T, s.APIClient, s.testNamespace1, "Deployment", s.deployment.Name, helpers.SlowEventuallyOption)
	s.Require.NotNil(resource)
	s.Assert.Equal(s.deployment.Name, resource.Name)
	s.Assert.Equal("Deployment", resource.Kind, "Resource kind mismatch")
	s.Assert.Equal(s.testNamespace1, resource.Namespace, "Resource namespace mismatch")
	s.Assert.NotEmpty(resource.ID, "Resource ID should not be empty")
	s.Assert.NotEmpty(resource.Name, "Resource name should not be empty")
	s.resourceID = resource.ID
	s.T.Logf("✓ Resource found in API: %s/%s (ID: %s)", resource.Namespace, resource.Kind, resource.ID)
	return s
}

func (s *DefaultResourcesStage) namespace_filter_works() *DefaultResourcesStage {
	ctx, cancel := s.ctxHelper.WithDefaultTimeout()
	defer cancel()

	searchResp, err := s.APIClient.Search(ctx, time.Now().Unix()-90, time.Now().Unix()+10, s.testNamespace1, "Deployment")
	s.Require.NoError(err)
	s.Assert.Greater(searchResp.Count, 0, "Should find deployments in test-default namespace")

	foundInNamespace := false
	for _, r := range searchResp.Resources {
		if r.Name == s.deployment.Name && r.Namespace == s.testNamespace1 {
			foundInNamespace = true
			break
		}
	}
	s.Assert.True(foundInNamespace, "Deployment should be found with namespace filter")
	s.T.Logf("✓ Namespace filter works: Found %d resources", searchResp.Count)
	return s
}

func (s *DefaultResourcesStage) unfiltered_query_returns_all_namespaces() *DefaultResourcesStage {
	ctx, cancel := s.ctxHelper.WithDefaultTimeout()
	defer cancel()

	searchRespAll, err := s.APIClient.Search(ctx, time.Now().Unix()-90, time.Now().Unix()+10, "", "Deployment")
	s.Require.NoError(err)
	s.Assert.Greater(searchRespAll.Count, 0, "Should find deployments across all namespaces")
	s.T.Logf("✓ Unfiltered query works: Found %d total resources", searchRespAll.Count)
	return s
}

func (s *DefaultResourcesStage) wrong_namespace_filter_returns_no_results() *DefaultResourcesStage {
	ctx, cancel := s.ctxHelper.WithDefaultTimeout()
	defer cancel()

	searchRespWrong, err := s.APIClient.Search(ctx, time.Now().Unix()-90, time.Now().Unix()+10, s.testNamespace2, "Deployment")
	s.Require.NoError(err)

	foundInWrongNamespace := false
	for _, r := range searchRespWrong.Resources {
		if r.Name == s.deployment.Name {
			foundInWrongNamespace = true
			break
		}
	}
	s.Assert.False(foundInWrongNamespace, "Should NOT find test-default deployment when filtering by test-alternate namespace")
	return s
}

func (s *DefaultResourcesStage) metadata_contains_expected_data() *DefaultResourcesStage {
	ctx, cancel := s.ctxHelper.WithDefaultTimeout()
	defer cancel()

	metadata, err := s.APIClient.GetMetadata(ctx, nil, nil)
	s.Require.NoError(err)

	helpers.AssertNamespaceInMetadata(s.T, metadata, s.testNamespace1)
	helpers.AssertKindInMetadata(s.T, metadata, "Deployment")

	s.T.Log("✓ Default resources scenario completed successfully!")
	return s
}
