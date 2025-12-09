package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
)

type DefaultResourcesStage struct {
	t         *testing.T
	require   *require.Assertions
	assert    *assert.Assertions
	testCtx   *helpers.TestContext
	k8sClient *helpers.K8sClient
	apiClient *helpers.APIClient

	testNamespace1 string
	testNamespace2 string
	deployment     *appsv1.Deployment
	resourceID     string
}

func NewDefaultResourcesStage(t *testing.T) (*DefaultResourcesStage, *DefaultResourcesStage, *DefaultResourcesStage) {
	s := &DefaultResourcesStage{
		t:       t,
		require: require.New(t),
		assert:  assert.New(t),
	}
	return s, s, s
}

func (s *DefaultResourcesStage) and() *DefaultResourcesStage {
	return s
}

func (s *DefaultResourcesStage) a_test_environment() *DefaultResourcesStage {
	s.testCtx = helpers.SetupE2ETest(s.t)
	s.k8sClient = s.testCtx.K8sClient
	s.apiClient = s.testCtx.APIClient
	return s
}

func (s *DefaultResourcesStage) two_test_namespaces() *DefaultResourcesStage {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	version, err := s.k8sClient.GetClusterVersion(ctx)
	s.require.NoError(err, "failed to get cluster version")
	s.t.Logf("Kubernetes version: %s", version)

	s.testNamespace1 = "test-default"
	s.testNamespace2 = "test-alternate"

	for _, ns := range []string{s.testNamespace1, s.testNamespace2} {
		ns := ns // capture loop variable for closure
		err := s.k8sClient.CreateNamespace(ctx, ns)
		s.require.NoError(err, "failed to create namespace %s", ns)
		s.t.Cleanup(func() {
			if err := s.k8sClient.DeleteNamespace(context.Background(), ns); err != nil {
				s.t.Logf("Warning: failed to delete namespace %s: %v", ns, err)
			}
		})
	}

	return s
}

func (s *DefaultResourcesStage) deployment_is_created_in_first_namespace() *DefaultResourcesStage {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	deployment, err := helpers.CreateTestDeployment(ctx, s.t, s.k8sClient, s.testNamespace1)
	s.require.NoError(err, "failed to create test deployment")
	s.require.NotNil(deployment)
	s.deployment = deployment

	s.t.Logf("Deployment created: %s/%s (UID: %s)", deployment.Namespace, deployment.Name, deployment.UID)

	// Wait for deployment to be ready with all replicas available
	err = s.k8sClient.WaitForDeploymentReady(ctx, s.testNamespace1, deployment.Name, 2*time.Minute)
	if err != nil {
		s.t.Logf("Warning: deployment ready check failed (expected in e2e): %v", err)
	}

	return s
}

func (s *DefaultResourcesStage) deployment_is_indexed() *DefaultResourcesStage {
	resource := helpers.EventuallyResourceCreated(s.t, s.apiClient, s.testNamespace1, "Deployment", s.deployment.Name, helpers.SlowEventuallyOption)
	s.require.NotNil(resource)
	s.assert.Equal(s.deployment.Name, resource.Name)
	s.assert.Equal("Deployment", resource.Kind, "Resource kind mismatch")
	s.assert.Equal(s.testNamespace1, resource.Namespace, "Resource namespace mismatch")
	s.assert.NotEmpty(resource.ID, "Resource ID should not be empty")
	s.assert.NotEmpty(resource.Name, "Resource name should not be empty")
	s.resourceID = resource.ID
	s.t.Logf("✓ Resource found in API: %s/%s (ID: %s)", resource.Namespace, resource.Kind, resource.ID)
	return s
}

func (s *DefaultResourcesStage) namespace_filter_works() *DefaultResourcesStage {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	searchResp, err := s.apiClient.Search(ctx, time.Now().Unix()-90, time.Now().Unix()+10, s.testNamespace1, "Deployment")
	s.require.NoError(err)
	s.assert.Greater(searchResp.Count, 0, "Should find deployments in test-default namespace")

	foundInNamespace := false
	for _, r := range searchResp.Resources {
		if r.Name == s.deployment.Name && r.Namespace == s.testNamespace1 {
			foundInNamespace = true
			break
		}
	}
	s.assert.True(foundInNamespace, "Deployment should be found with namespace filter")
	s.t.Logf("✓ Namespace filter works: Found %d resources", searchResp.Count)
	return s
}

func (s *DefaultResourcesStage) unfiltered_query_returns_all_namespaces() *DefaultResourcesStage {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	searchRespAll, err := s.apiClient.Search(ctx, time.Now().Unix()-90, time.Now().Unix()+10, "", "Deployment")
	s.require.NoError(err)
	s.assert.Greater(searchRespAll.Count, 0, "Should find deployments across all namespaces")
	s.t.Logf("✓ Unfiltered query works: Found %d total resources", searchRespAll.Count)
	return s
}

func (s *DefaultResourcesStage) wrong_namespace_filter_returns_no_results() *DefaultResourcesStage {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	searchRespWrong, err := s.apiClient.Search(ctx, time.Now().Unix()-90, time.Now().Unix()+10, s.testNamespace2, "Deployment")
	s.require.NoError(err)

	foundInWrongNamespace := false
	for _, r := range searchRespWrong.Resources {
		if r.Name == s.deployment.Name {
			foundInWrongNamespace = true
			break
		}
	}
	s.assert.False(foundInWrongNamespace, "Should NOT find test-default deployment when filtering by test-alternate namespace")
	return s
}

func (s *DefaultResourcesStage) metadata_contains_expected_data() *DefaultResourcesStage {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	metadata, err := s.apiClient.GetMetadata(ctx, nil, nil)
	s.require.NoError(err)

	helpers.AssertNamespaceInMetadata(s.t, metadata, s.testNamespace1)
	helpers.AssertKindInMetadata(s.t, metadata, "Deployment")

	s.t.Log("✓ Default resources scenario completed successfully!")
	return s
}
