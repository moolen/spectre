package e2e

import (
	"testing"
)

func TestScenarioPodRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewPodRestartStage(t)

	given.a_test_environment().and().
		a_test_namespace_with_deployment().and().
		deployment_is_indexed()

	when.spectre_pod_is_restarted().and().
		wait_for_spectre_to_be_ready().and().
		port_forward_is_reconnected()

	then.first_deployment_is_still_present().and().
		second_deployment_is_created().and().
		second_deployment_is_indexed().and().
		both_deployments_are_searchable()
}
