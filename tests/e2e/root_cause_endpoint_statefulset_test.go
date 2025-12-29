package e2e

import (
	"testing"
	"time"
)

// TestRootCause_StatefulSetImageChange_Endpoint_E2E validates root cause analysis
// via HTTP endpoint for a StatefulSet image change scenario.
//
// Scenario: A StatefulSet is deployed with a valid image, then updated with a
// non-existent image tag, causing pods to fail with ImagePullBackOff. The test
// validates that the root cause analysis correctly identifies the StatefulSet
// and includes change events before and after the image change.
func TestRootCause_StatefulSetImageChange_Endpoint_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewRootCauseScenarioStage(t)
	defer given.cleanup()

	// Setup: Deploy Spectre (StatefulSet watching is enabled by default)
	given.a_test_environment().and().
		spectre_is_deployed()

	// Deploy healthy StatefulSet with valid nginx image
	when.statefulset_is_deployed("test-statefulset.yaml")

	// Update StatefulSet with invalid image
	when.statefulset_image_is_updated("nginx:v6.6.6").and().
		wait_for_pod_failure("ImagePullBackOff", 120*time.Second)

	// Call root cause endpoint
	when.failed_pod_is_identified().and().
		root_cause_endpoint_is_called()

	// Strict assertions
	then.assert_statefulset_owns_pod().and().
		assert_statefulset_has_change_events(given.beforeUpdateTime, given.afterUpdateTime)
}
