package e2e

import (
	"testing"
)

// TestCLIImportOnStartup validates the CLI import on startup functionality:
// 1. Create a test cluster
// 2. Generate test events and store them in a ConfigMap
// 3. Deploy Spectre with import configuration (mount ConfigMap and use --import flag)
// 4. Wait for Spectre to become ready
// 5. Verify imported data is present via metadata and search APIs
// 6. Verify import report in logs
func TestCLIImportOnStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	t.Parallel()

	given, when, then := NewImportExportStage(t)

	given.a_test_cluster().and().
		generated_test_events_stored_in_configmap()

	when.spectre_is_deployed_with_import_on_startup().and().
		wait_for_spectre_to_become_ready().and().
		port_forward_to_spectre()

	then.verify_imported_data_is_present_via_metadata_api().and().
		verify_resources_can_be_queried_via_search_api().and().
		specific_resources_are_present_by_name_for_cli_import().and().
		verify_import_report_in_logs()
}

// TestImportExportRoundTrip validates the full import/export workflow:
// 1. Deploy Spectre via Helm
// 2. Generate test data in two namespaces (import-1, import-2)
// 3. Export the data to a file
// 4. Uninstall Spectre and ensure no PV/PVC remains
// 5. Delete the generated resources and namespaces
// 6. Redeploy Spectre
// 7. Ensure old data is gone
// 8. Import the previously exported data
// 9. Verify the imported data is present
func TestImportExportRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	t.Parallel()

	given, when, then := NewImportExportStage(t)

	given.a_test_environment().and().
		test_data_in_two_namespaces().and().
		resources_are_indexed()

	when.data_is_exported_to_file().and().
		spectre_is_uninstalled().and().
		test_resources_are_deleted().and().
		spectre_is_redeployed().and().
		old_data_is_not_present().and().
		data_is_imported_from_binary_file()

	then.namespaces_appear_in_metadata().and().
		deployments_can_be_queried().and().
		specific_deployment_is_present()
}

// TestJSONEventBatchImport validates the JSON event batch import functionality:
// 1. Deploy Spectre via Helm
// 2. Generate 30-40 test events of different kinds across multiple namespaces
// 3. Call the JSON import endpoint
// 4. Verify imported events are present via search and metadata APIs
func TestJSONEventBatchImport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	t.Parallel()

	given, when, then := NewImportExportStage(t)

	given.a_test_environment().and().
		generated_test_events_for_multiple_namespaces()

	when.events_are_imported_via_json().and().
		wait_for_data_indexing()

	then.namespaces_appear_in_metadata().and().
		expected_resource_kinds_are_present().and().
		all_resources_are_queryable().and().
		specific_resources_are_present_by_name()
}

// TestBatchImportWithResourceTimeline validates batch import with multiple update events for a single resource:
// 1. Deploy Spectre via Helm
// 2. Generate a single Service resource with 10 update events in short succession (5-30 seconds apart)
// 3. Push the JSON data to the batch import endpoint
// 4. Verify the data is available through the timeline API
func TestBatchImportWithResourceTimeline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	t.Parallel()

	given, when, then := NewImportExportStage(t)

	given.a_test_environment().and().
		generated_service_with_timeline_events()

	when.events_are_imported_via_json().and().
		wait_for_data_indexing()

	then.namespaces_appear_in_metadata().and().
		service_kind_is_present().and().
		service_is_found_via_search().and().
		timeline_shows_status_segments().and().
		status_segments_are_ordered()
}

// TestJSONEventBatchImportWithKubernetesEvents validates the JSON event batch import functionality with Kubernetes Event resources:
// 1. Deploy Spectre via Helm
// 2. Generate test events including Kubernetes Events (Kind=Event) with involvedObject references
// 3. Call the JSON import endpoint
// 4. Verify imported events are present via search and metadata APIs, including Kind=Event
// 5. Verify that Kubernetes Events have the InvolvedObjectUID properly populated
func TestJSONEventBatchImportWithKubernetesEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	t.Parallel()

	given, when, then := NewImportExportStage(t)

	given.a_test_environment().and().
		generated_test_events_with_kubernetes_events()

	when.events_are_imported_via_json().and().
		wait_for_data_indexing()

	then.namespaces_appear_in_metadata().and().
		kubernetes_event_kind_is_present().and().
		kubernetes_events_can_be_queried().and().
		specific_kubernetes_event_is_present()
}
