package e2e

import (
	"testing"
)

// TestRootCauseEndpoint runs root cause analysis endpoint tests
// using the HTTP /v1/root-cause endpoint (not MCP tool).
func TestRootCauseEndpoint(t *testing.T) {
	t.Run("FluxHelmRelease", TestRootCause_FluxHelmRelease_Endpoint_E2E)
	t.Run("FluxHelmReleaseValuesFrom", TestRootCause_FluxHelmReleaseValuesFrom_Endpoint_E2E)
	t.Run("FluxKustomization", TestRootCause_FluxKustomization_Endpoint_E2E)
	t.Run("StatefulSetImageChange", TestRootCause_StatefulSetImageChange_Endpoint_E2E)
	t.Run("NetworkPolicySameNamespace", TestRootCause_NetworkPolicy_SameNamespace_Endpoint_E2E)
	t.Run("NetworkPolicyCrossNamespace", TestRootCause_NetworkPolicy_CrossNamespace_Endpoint_E2E)
	t.Run("IngressSameNamespace", TestRootCause_Ingress_SameNamespace_Endpoint_E2E)
}
