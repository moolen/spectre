package e2e

import (
	"fmt"
	"testing"
	"time"
)

// TestUIRootCauseAnalysis tests the root cause analysis view in the UI
func TestUIRootCauseAnalysis(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewUIStage(t)

	// Use a unique deployment name for this test
	deploymentName := fmt.Sprintf("test-faulty-deploy-%d", time.Now().Unix())

	given.a_test_environment().and().
		browser_is_initialized().and().
		healthy_deployment_is_created(deploymentName).and().
		deployment_resources_are_available().and().
		deployment_image_is_changed_to_faulty(deploymentName).and().
		pod_with_image_pull_error_exists(deploymentName).and().
		pod_error_is_indexed(deploymentName).and().
		navigated_to_timeline(15 * time.Minute).and().
		scroll_timeline_to_load_all_resources().and().
		page_loads_completely()

	when.namespace_filter_is_set(given.testCtx.Namespace).and().
		kind_dropdown_is_opened().and().
		kind_option_is_selected("Pod").and().
		wait_for_text_in_timeline(deploymentName).and().
		erroneous_timeline_segment_is_clicked(deploymentName)

	then.detail_panel_is_visible().and().
		analyze_root_cause_button_exists()

	when.analyze_root_cause_button_is_clicked()

	then.root_cause_page_is_loaded().and().
		root_cause_graph_is_visible().and().
		root_cause_graph_has_expected_nodes().and().
		root_cause_graph_has_expected_edges()

	when.root_cause_lookback_is_changed_to("1 hour")

	then.root_cause_graph_is_visible().and().
		root_cause_graph_has_expected_nodes().and().
		root_cause_graph_has_expected_edges()
}
