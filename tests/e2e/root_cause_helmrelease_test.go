package e2e

import (
	"testing"
	"time"
)

// TestRootCause_HelmReleaseImageChange_E2E validates full root cause path
// from HelmRelease configuration change to pod failure with ImagePullBackOff.
//
// Scenario: HelmRelease values are updated with a non-existent image tag,
// causing the managed Deployment to create a new ReplicaSet with pods that
// fail to pull the image. The test validates that the root cause analysis
// correctly identifies the HelmRelease as the root cause and constructs
// the complete causal chain.
func TestRootCause_HelmReleaseImageChange_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewRootCauseScenarioStage(t)
	defer given.cleanup()

	// Setup: Install Flux and deploy Spectre with MCP
	given.a_test_environment().and().
		flux_is_installed().and().
		spectre_is_deployed().and().
		mcp_client_is_connected()

	// Deploy healthy HelmRelease with valid nginx image
	when.helmrelease_is_deployed("helmrelease-valid-image.yaml").and().
		wait_for_healthy_deployment(120 * time.Second)

	// Update HelmRelease with non-existent image tag
	when.helmrelease_is_updated("helmrelease-invalid-image.yaml").and().
		wait_for_pod_failure("ImagePullBackOff", 120*time.Second)

	// Call root cause analysis MCP tool
	when.failed_pod_is_identified().and().
		find_root_cause_tool_is_called()

	// Validate: Root cause should be HelmRelease
	then.root_cause_is_helmrelease().and().
		observed_symptom_is("ImagePullError").and().
		// Causal chain: HelmRelease -> Deployment -> ReplicaSet -> Pod
		causal_chain_includes_all_steps(4).and().
		causal_chain_has_step("HelmRelease", "MANAGES", "Deployment").and().
		causal_chain_has_step("Deployment", "OWNS", "ReplicaSet").and().
		causal_chain_has_step("ReplicaSet", "OWNS", "Pod").and().
		causal_chain_has_step("Pod", "SYMPTOM", "").and().
		// Confidence should be high due to MANAGES relationship
		confidence_score_exceeds(0.75).and().
		confidence_factors_are_valid().and().
		supporting_evidence_includes_flux_labels().and().
		temporal_proximity_is_high()
}

// TestRootCause_MissingManagesEdge_FallbackToOwnership_E2E validates that
// when no MANAGES relationship exists (direct Deployment update), the root
// cause analysis correctly falls back to ownership chain and identifies
// the Deployment as root cause with appropriate confidence score.
func TestRootCause_MissingManagesEdge_FallbackToOwnership_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewRootCauseScenarioStage(t)
	defer given.cleanup()

	// Setup
	given.a_test_environment().and().
		spectre_is_deployed().and().
		mcp_client_is_connected()

	// Deploy Deployment directly (no HelmRelease/Flux)
	when.deployment_is_deployed("deployment-direct.yaml").and().
		wait_for_healthy_pods(60 * time.Second)

	// Update Deployment with invalid image
	when.deployment_is_updated("deployment-direct-invalid.yaml").and().
		wait_for_pod_failure("ImagePullBackOff", 90*time.Second)

	// Call root cause analysis
	when.failed_pod_is_identified().and().
		find_root_cause_tool_is_called()

	// Validate: Root cause should be Deployment (no MANAGES edge)
	then.root_cause_is_deployment().and().
		observed_symptom_is("ImagePullError").and().
		// Causal chain: Deployment -> ReplicaSet -> Pod (no HelmRelease)
		causal_chain_includes_all_steps(3).and().
		causal_chain_has_step("Deployment", "OWNS", "ReplicaSet").and().
		causal_chain_has_step("ReplicaSet", "OWNS", "Pod").and().
		causal_chain_has_step("Pod", "SYMPTOM", "").and().
		// Confidence should be moderate (no MANAGES relationship)
		confidence_score_in_range(0.55, 0.80).and().
		confidence_factors_are_valid()
}
