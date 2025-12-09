package e2e

import (
	"testing"
	"time"
)

// TestUIDetailPanelInteraction tests detail panel open/close and resource display
func TestUIDetailPanelInteraction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewUIStage(t)

	given.a_test_environment().and().
		browser_is_initialized().and().
		deployment_is_created("", "").and().
		resources_are_available().and().
		navigated_to_timeline(10 * time.Minute)

	when.resource_label_is_clicked()

	then.detail_panel_is_visible().and().
		escape_key_is_pressed()

	then.detail_panel_is_not_visible()
}
