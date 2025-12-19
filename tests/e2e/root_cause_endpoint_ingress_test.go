package e2e

import (
	"testing"
)

// TestRootCause_Ingress_SameNamespace_Endpoint_E2E validates root cause analysis
// via HTTP endpoint for an Ingress routing to a Service that selects pods.
//
// Scenario: A Deployment creates pods with specific labels. A Service selects those
// pods. An Ingress references the Service. The test validates that the root cause
// analysis graph includes the complete chain: Ingress → Service → Pod with
// appropriate edges (REFERENCES_SPEC and SELECTS).
func TestRootCause_Ingress_SameNamespace_Endpoint_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewRootCauseScenarioStage(t)
	defer given.cleanup()

	// Setup: Deploy Spectre
	given.a_test_environment().and().
		spectre_is_deployed()

	// Deploy Deployment with specific labels
	podLabels := map[string]string{
		"app":  "test-ingress-app",
		"tier": "backend",
	}
	when.deployment_with_labels_is_deployed("test-ingress-deployment", podLabels)

	// Create Service that selects the pods
	selectorLabels := map[string]string{
		"app": "test-ingress-app",
	}
	when.service_selecting_pods_is_created("test-ingress-service", selectorLabels, 80)

	// Create Ingress that references the Service
	when.ingress_referencing_service_is_created("test-ingress", "test-ingress-service")

	// Call root cause endpoint
	when.running_pod_is_identified().and().
		root_cause_endpoint_is_called()

	// Assertions: verify Ingress → Service → Pod path
	then.assert_graph_has_deployment_owns_pod().and().
		assert_graph_has_service().and().
		assert_graph_has_service_selects_pod().and().
		assert_graph_has_ingress().and().
		assert_graph_has_ingress_references_service()
}

