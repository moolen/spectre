package e2e

import (
	"testing"
	"time"
)

// TestTimelineBarHighlighting tests that timeline bars show visual highlighting (outline) when selected
// via clicking and arrow key navigation
func TestTimelineBarHighlighting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewUIStage(t)

	given.a_test_environment().and().
		browser_is_initialized().and().
		deployment_is_created("", "").and().
		resources_are_available().and().
		navigated_to_timeline(10 * time.Minute).and().
		scroll_timeline_to_load_all_resources()

	when.resource_label_is_clicked()

	then.timeline_segment_is_highlighted()

	when.arrow_key_is_pressed("ArrowRight")

	then.timeline_segment_is_highlighted()

	when.arrow_key_is_pressed("ArrowLeft")

	then.timeline_segment_is_highlighted()
}

// TestTimelineHighlightingWithThemeSwitch tests highlighting works when switching themes
func TestTimelineHighlightingWithThemeSwitch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewUIStage(t)

	given.a_test_environment().and().
		browser_is_initialized().and().
		deployment_is_created("", "").and().
		resources_are_available().and().
		navigated_to_timeline(10 * time.Minute).and().
		scroll_timeline_to_load_all_resources()

	when.resource_label_is_clicked()

	then.timeline_segment_is_highlighted()

	when.theme_is_switched_to("light")

	then.timeline_segment_is_highlighted()

	when.theme_is_switched_to("dark")

	then.timeline_segment_is_highlighted()
}
