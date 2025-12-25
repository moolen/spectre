package e2e

import (
	"testing"
	"time"
)

// TestRootCause_FluxHelmRelease_Endpoint_E2E validates root cause analysis
// via HTTP endpoint for a Flux-managed HelmRelease scenario.
//
// Scenario: A HelmRelease is deployed via Flux using an external Helm repository,
// then updated with a non-existent image tag, causing pods to fail with ImagePullBackOff.
// The test validates that the root cause analysis correctly identifies the HelmRelease
// and constructs the complete causal graph with all required resource kinds and
// relationship types.
func TestRootCause_FluxHelmRelease_Endpoint_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewRootCauseScenarioStage(t)
	defer given.cleanup()

	// Setup: Install Flux and deploy Spectre
	given.a_test_environment().and().
		flux_is_installed().and().
		spectre_is_deployed()

	// Deploy HelmRelease from external chart repository (external-secrets)
	when.flux_external_helmrelease_is_deployed("external-secrets", "0.9.9", "https://charts.external-secrets.io")

	// Update HelmRelease with invalid image tag
	when.flux_helmrelease_image_is_updated("v6.6.6").and().
		wait_for_pod_failure("ImagePullBackOff", 120*time.Second)

	// Call root cause endpoint
	when.failed_pod_is_identified().and().
		root_cause_endpoint_is_called()

	// Strict assertions
	then.assert_graph_has_required_kinds().and().
		assert_graph_has_required_edges()
}
