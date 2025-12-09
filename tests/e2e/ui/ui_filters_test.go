package e2e

import (
	"testing"
	"time"
)

// TestUIFilterByNamespace tests namespace filtering functionality
func TestUIFilterByNamespace(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewUIStage(t)

	namespace1 := "test-ns-1"
	namespace2 := "test-ns-2"

	given.a_test_environment().and().
		browser_is_initialized().and().
		deployments_are_created_in_namespaces([]string{namespace1, namespace2}).and().
		resources_are_available().and().
		navigated_to_timeline(10 * time.Minute).and().
		page_loads_completely().and().
		wait_for_text_in_timeline(namespace1).and().
		wait_for_text_in_timeline(namespace2)

	then.text_is_visible(namespace1).and().
		text_is_visible(namespace2)

	when.namespace_filter_is_set(namespace1)

	then.text_is_visible(namespace1).and().
		text_is_not_visible(namespace2)
}

// TestUIFilterByKind tests resource kind filtering functionality
func TestUIFilterByKind(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewUIStage(t)

	given.a_test_environment().and().
		browser_is_initialized().and().
		deployment_is_created("", "").and().
		deployment_is_created("test-deployment-2", "").and().
		resources_are_available().and().
		navigated_to_timeline(10 * time.Minute).and().
		page_loads_completely().and().
		wait_for_text_in_timeline("test-deployment")

	when.kind_dropdown_is_opened()

	then.kind_options_are_visible([]string{"Deployment", "Pod"})

	when.kind_option_is_selected("Deployment")
}

// TestUISearchFilter tests the search filtering functionality
func TestUISearchFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewUIStage(t)

	targetName := "search-target-1"
	otherName := "other-resource"

	given.a_test_environment().and().
		browser_is_initialized().and().
		deployment_is_created(targetName, "").and().
		deployment_is_created(otherName, "").and().
		resources_are_available().and().
		navigated_to_timeline(10 * time.Minute).and().
		page_loads_completely().and().
		wait_for_text_in_timeline(targetName).and().
		wait_for_text_in_timeline(otherName)

	then.text_is_visible(targetName).and().
		text_is_visible(otherName)

	when.search_filter_is_set(targetName)

	then.text_is_visible(targetName).and().
		text_is_not_visible(otherName)

	when.search_filter_is_cleared()

	then.text_is_visible(targetName).and().
		text_is_visible(otherName)
}
