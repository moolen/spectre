package e2e

import (
	"testing"
)

func TestScenarioDynamicConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewConfigReloadStage(t)

	given.a_test_environment().and().
		a_test_namespace().and().
		statefulset_is_created()

	when.statefulset_is_not_found_with_default_config().and().
		watcher_config_is_updated_to_include_statefulset().and().
		wait_for_hot_reload()

	then.statefulset_is_found_after_reload().and().
		deployment_can_still_be_captured().and().
		metadata_includes_both_resource_kinds()
}
