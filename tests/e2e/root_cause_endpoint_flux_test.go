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
	given, when, then := NewRootCauseScenarioStage(t)
	defer given.cleanup()

	// Setup: Install Flux BEFORE Spectre so that Spectre can watch Flux CRDs
	given.a_test_environment_with_flux().and().
		spectre_is_deployed()

	// Deploy HelmRelease from external chart repository (podinfo)
	when.flux_external_helmrelease_is_deployed("podinfo", "6.5.4", "https://stefanprodan.github.io/podinfo")

	// Update HelmRelease with invalid image tag
	when.flux_helmrelease_image_is_updated("does-not-exist-v999").and().
		wait_for_pod_failure("ImagePullBackOff", 120*time.Second)

	// Call root cause endpoint (with RBAC testing for Flux)
	when.failed_pod_is_identified().and().
		create_rbac_resources_for_testing().and().
		root_cause_endpoint_is_called()

	// Strict assertions
	then.assert_graph_has_required_kinds().and().
		assert_graph_has_required_edges()
}

// TestRootCause_FluxHelmReleaseValuesFrom_Endpoint_E2E validates root cause analysis
// for a HelmRelease that uses valuesFrom to reference a ConfigMap.
//
// Scenario: A HelmRelease is deployed with valuesFrom pointing to a ConfigMap
// that provides Helm values. The test validates that the root cause analysis graph
// includes the ConfigMap node and a REFERENCES_SPEC edge from the HelmRelease
// to the ConfigMap.
func TestRootCause_FluxHelmReleaseValuesFrom_Endpoint_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewRootCauseScenarioStage(t)
	defer given.cleanup()

	// Setup: Install Flux BEFORE Spectre so that Spectre can watch Flux CRDs
	given.a_test_environment_with_flux().and().
		spectre_is_deployed()

	// Deploy HelmRelease with valuesFrom referencing a ConfigMap
	when.flux_helmrelease_with_values_configmap_is_deployed("podinfo", "6.5.4", "https://stefanprodan.github.io/podinfo")

	// Update HelmRelease with invalid image tag to cause pod failure
	when.flux_helmrelease_with_values_configmap_image_is_updated("does-not-exist-v999").and().
		wait_for_pod_failure("ImagePullBackOff", 120*time.Second)

	// Call root cause endpoint
	when.failed_pod_is_identified().and().
		root_cause_endpoint_is_called()

	// Assertions: verify ConfigMap node and REFERENCES_SPEC edge exist
	then.assert_graph_has_helmrelease_manages_deployment().and().
		assert_graph_has_configmap_reference()
}
