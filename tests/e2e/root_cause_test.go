package e2e

import (
	"testing"
)

// TestRootCauseHelmRelease runs all root cause analysis scenarios
// using the shared cluster for efficiency.
func TestRootCauseHelmRelease(t *testing.T) {
	t.Run("HelmReleaseImageChange", TestRootCause_HelmReleaseImageChange_E2E)
	t.Run("MissingManagesEdgeFallback", TestRootCause_MissingManagesEdge_FallbackToOwnership_E2E)
}

// TestRootCauseEndpoint runs root cause analysis endpoint tests
// using the HTTP /v1/root-cause endpoint (not MCP tool).
func TestRootCauseEndpoint(t *testing.T) {
	t.Run("FluxHelmRelease", TestRootCause_FluxHelmRelease_Endpoint_E2E)
	t.Run("StatefulSetImageChange", TestRootCause_StatefulSetImageChange_Endpoint_E2E)
}
