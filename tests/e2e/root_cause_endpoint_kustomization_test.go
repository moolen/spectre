package e2e

import (
	"testing"
	"time"
)

// TestRootCause_FluxKustomization_Endpoint_E2E validates root cause analysis
// via HTTP endpoint for a Flux Kustomization managing a Deployment.
//
// Scenario: A Kustomization resource is created along with a Deployment that
// has the Kustomize labels (kustomize.toolkit.fluxcd.io/name and
// kustomize.toolkit.fluxcd.io/namespace). The test validates that the root
// cause analysis graph includes the Kustomization node and a MANAGES edge
// from the Kustomization to the Deployment.
func TestRootCause_FluxKustomization_Endpoint_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewRootCauseScenarioStage(t)
	defer given.cleanup()

	// Setup: Install Flux BEFORE Spectre so that Spectre can watch Flux CRDs
	given.a_test_environment_with_flux().and().
		spectre_is_deployed()

	// Deploy Kustomization with a labeled Deployment
	when.flux_kustomization_with_labeled_deployment_is_deployed("test-kustomization")

	// Update Deployment with invalid image to cause pod failure
	when.deployment_image_is_updated_to_invalid("nginx:does-not-exist-v999").and().
		wait_for_pod_failure("ImagePullBackOff", 120*time.Second)

	// Call root cause endpoint
	when.failed_pod_is_identified().and().
		root_cause_endpoint_is_called()

	// Assertions: verify Kustomization node and MANAGES edge exist
	then.assert_graph_has_kustomization_manages_deployment()
}
