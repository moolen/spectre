package e2e

import (
	"testing"
)

// TestRootCause_NetworkPolicy_SameNamespace_Endpoint_E2E validates root cause analysis
// via HTTP endpoint for a NetworkPolicy selecting pods in the same namespace.
//
// Scenario: A Deployment creates pods with specific labels. A NetworkPolicy in the
// same namespace selects those pods using label selectors. The test validates that
// the root cause analysis graph includes the NetworkPolicy node and SELECTS edge
// from the NetworkPolicy to the Pod.
func TestRootCause_NetworkPolicy_SameNamespace_Endpoint_E2E(t *testing.T) {
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
		"app":  "test-netpol-app",
		"tier": "frontend",
	}
	when.deployment_with_labels_is_deployed("test-netpol-deployment", podLabels)

	// Create NetworkPolicy in the same namespace that selects the pods
	selectorLabels := map[string]string{
		"app": "test-netpol-app",
	}
	when.networkpolicy_selecting_pods_is_created("test-network-policy", selectorLabels)

	// Call root cause endpoint
	when.running_pod_is_identified().and().
		root_cause_endpoint_is_called()

	// Assertions
	then.assert_graph_has_deployment_owns_pod().and().
		assert_graph_has_networkpolicy().and().
		assert_graph_has_selects_edge()
}

// TestRootCause_NetworkPolicy_CrossNamespace_Endpoint_E2E validates that NetworkPolicies
// do NOT select pods in different namespaces.
//
// Scenario: A Deployment creates pods in namespace A. A NetworkPolicy in namespace B
// has a selector that would match the pod labels, but since NetworkPolicies only
// select pods in their own namespace, the graph should NOT include the NetworkPolicy
// or a SELECTS edge.
func TestRootCause_NetworkPolicy_CrossNamespace_Endpoint_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewRootCauseScenarioStage(t)
	defer given.cleanup()

	// Setup: Deploy Spectre
	given.a_test_environment().and().
		spectre_is_deployed()

	// Deploy Deployment with specific labels in namespace A
	podLabels := map[string]string{
		"app":  "test-netpol-app",
		"tier": "frontend",
	}
	when.deployment_with_labels_is_deployed("test-netpol-deployment", podLabels)

	// Create NetworkPolicy in a DIFFERENT namespace (namespace B) that tries to select the pods
	// This should NOT work because NetworkPolicies only select pods in their own namespace
	selectorLabels := map[string]string{
		"app": "test-netpol-app",
	}
	when.networkpolicy_in_different_namespace_is_created("test-network-policy", selectorLabels)

	// Call root cause endpoint
	when.running_pod_is_identified().and().
		root_cause_endpoint_is_called()

	// Assertions: The NetworkPolicy should NOT appear in the graph because
	// it cannot select pods across namespaces
	then.assert_graph_has_deployment_owns_pod().and().
		assert_graph_has_no_networkpolicy()
}
