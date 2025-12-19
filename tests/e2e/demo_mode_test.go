package e2e

import (
	"testing"
)

func TestScenarioDemoMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewDemoModeStage(t)

	given.a_demo_mode_environment()

	when.spectre_server_starts_with_demo_flag().and().
		ui_loads_successfully()

	then.demo_mode_indicator_is_visible().and().
		timeline_data_is_displayed().and().
		resources_are_visible_in_timeline().and().
		metadata_endpoint_returns_demo_data()
}
