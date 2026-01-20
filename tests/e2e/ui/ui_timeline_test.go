package e2e

import (
	"testing"
	"time"
)

// TestUITimelinePageNavigation tests the timeline page loads correctly and navigation works
func TestUITimelinePageNavigation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, _, then := NewUIStage(t)

	given.a_test_environment().and().
		browser_is_initialized().and().
		navigated_to_root()

	then.filter_bar_is_visible().and().
		page_has_content()
}

// TestUITimelineDataLoading tests that the timeline loads and displays data
func TestUITimelineDataLoading(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, _, then := NewUIStage(t)

	given.a_test_environment().and().
		browser_is_initialized().and().
		deployment_is_created("", "").and().
		resources_are_available().and().
		navigated_to_timeline(10 * time.Minute)

	then.filter_bar_is_visible().and().
		page_has_content()
}
