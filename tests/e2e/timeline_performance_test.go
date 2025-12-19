package e2e

import (
	"testing"
)

// TestTimelinePerformanceWith10Files validates timeline query performance with 10 storage files.
// This test imports events spanning 10 hours and queries a fixed 1-hour window.
func TestTimelinePerformanceWith10Files(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewTimelinePerformanceStage(t)

	given.a_test_environment().and().
		events_spanning_hours(10)

	when.events_are_imported().and().
		timeline_is_queried_for_last_hour()

	then.query_performance_is_acceptable().and().
		baseline_performance_is_recorded()
}

// TestTimelinePerformanceWith25Files validates timeline query performance with 25 storage files.
// This test imports events spanning 25 hours and queries a fixed 1-hour window.
func TestTimelinePerformanceWith25Files(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewTimelinePerformanceStage(t)

	given.a_test_environment().and().
		events_spanning_hours(25)

	when.events_are_imported().and().
		timeline_is_queried_for_last_hour()

	then.query_performance_is_acceptable()
}

// TestTimelinePerformanceWith100Files validates timeline query performance with 100 storage files.
// This test imports events spanning 100 hours (~4 days) and queries a fixed 1-hour window.
func TestTimelinePerformanceWith100Files(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewTimelinePerformanceStage(t)

	given.a_test_environment().and().
		events_spanning_hours(100)

	when.events_are_imported().and().
		timeline_is_queried_for_last_hour()

	then.query_performance_is_acceptable()
}

// TestTimelinePerformanceWith500Files validates timeline query performance with 500 storage files.
// This test imports events spanning 500 hours (~21 days) and queries a fixed 1-hour window.
// This test verifies that the regression is fixed where query performance degrades with many files.
func TestTimelinePerformanceWith500Files(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewTimelinePerformanceStage(t)

	given.a_test_environment().and().
		events_spanning_hours(500)

	when.events_are_imported().and().
		timeline_is_queried_for_last_hour()

	then.query_performance_is_acceptable()
}
