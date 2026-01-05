package e2e

import (
	"testing"
)

func TestScenarioDefaultResources(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	t.Parallel()

	given, when, then := NewDefaultResourcesStage(t)

	given.a_test_environment().and().
		two_test_namespaces().and().
		deployment_is_created_in_first_namespace()

	when.deployment_is_indexed()

	then.namespace_filter_works().and().
		unfiltered_query_returns_all_namespaces().and().
		wrong_namespace_filter_returns_no_results().and().
		metadata_contains_expected_data()
}
