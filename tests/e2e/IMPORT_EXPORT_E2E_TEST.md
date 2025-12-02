# Import/Export E2E Test

## Overview

The `TestImportExportRoundTrip` test validates the complete import/export workflow for Spectre's storage system in a real Kubernetes environment.

## Test Scenario

The test follows a 9-step process to ensure data portability:

### 1. Deploy Spectre via Helm
- Uses `helpers.SetupE2ETest(t)` to create a Kind cluster
- Deploys Spectre via Helm chart into the monitoring namespace
- Waits for deployment readiness
- Sets up port-forward and API client

### 2. Generate Test Data
- Creates two namespaces: `import-1` and `import-2`
- Creates 25 Deployments in each namespace (50 total resources)
- Uses `helpers.NewDeploymentBuilder` to create nginx deployments
- Waits for resources to be indexed by Spectre
- Verifies data is queryable via the API

### 3. Export Data
- Computes a time range covering the last 15 minutes
- Calls `GET /v1/storage/export?from=...&to=...&include_open_hour=true&compression=true`
- Streams the response to a temporary file (`export.tar.gz`)
- Verifies the export file is non-empty

### 4. Uninstall Spectre
- Uses `helpers.HelmDeployer.UninstallChart()` to remove the Helm release
- Verifies that the PersistentVolumeClaim is deleted
- Ensures no persistent volumes remain

### 5. Delete Test Resources
- Deletes both test namespaces (`import-1` and `import-2`)
- Waits for namespaces to be fully removed
- All test deployments are cleaned up

### 6. Redeploy Spectre
- Loads Helm values using `helpers.LoadHelmValues()`
- Rebuilds and loads the Docker image using `helpers.BuildAndLoadTestImage()`
- Reinstalls the Helm chart
- Waits for deployment readiness
- Reconnects port-forward using `testCtx.ReconnectPortForward()`

### 7. Verify Old Data is Gone
- Calls `APIClient.GetMetadata()` and verifies `import-1` and `import-2` are NOT present
- Searches for Deployments in both namespaces and verifies 0 results
- Confirms the storage is clean before import

### 8. Import Exported Data
- Opens the previously exported archive file
- POSTs it to `POST /v1/storage/import?validate=true&overwrite=true`
- Verifies HTTP 200 response
- Parses the import report and checks for 0 failed files
- Logs the number of imported events

### 9. Verify Imported Data
- Uses `helpers.EventuallyCondition` to wait for namespaces to appear in metadata
- Searches for Deployments in both namespaces and verifies results > 0
- Spot-checks for a specific deployment (`import-deploy-0`) to ensure it's queryable
- Confirms the import/export round-trip was successful

## Test Duration

Expected duration: ~5-10 minutes (depending on cluster creation and image building)

## Prerequisites

- Docker (for building images)
- Kind (for creating Kubernetes clusters)
- kubectl (configured in PATH)
- Sufficient disk space for Kind cluster and export archives

## Running the Test

```bash
# Run the e2e test
go test -v ./tests/e2e -run TestImportExportRoundTrip -timeout 15m

# Skip in short mode
go test -short ./tests/e2e  # This test will be skipped
```

## Test Helpers Used

### From `helpers` package:
- `SetupE2ETest(t)` - Creates Kind cluster and deploys Spectre
- `K8sClient` - Kubernetes client for creating/deleting resources
- `APIClient` - HTTP client for Spectre's API
- `HelmDeployer` - Helm operations (install/uninstall)
- `NewDeploymentBuilder` - Creates test Deployment objects
- `EventuallyCondition` - Waits for async conditions
- `LoadHelmValues()` - Loads test Helm values
- `BuildAndLoadTestImage()` - Builds and loads Docker image
- `RepoPath()` - Gets absolute path to repo files
- `ReconnectPortForward()` - Reconnects port-forward after pod restart

## What This Test Validates

✅ **Export Functionality**
- Export API endpoint works correctly
- Data is properly serialized to tar.gz archive
- Time-range filtering works
- Include open hour option works

✅ **Import Functionality**
- Import API endpoint works correctly
- Archive validation works
- Logical merge at hour level works
- Data is properly restored

✅ **Data Portability**
- Events survive Spectre teardown
- Events survive PVC/PV deletion
- Events survive namespace deletion
- Events are queryable after import

✅ **Helm Chart Behavior**
- Uninstall properly cleans up PVCs
- Reinstall creates fresh storage
- Port-forward reconnection works

✅ **API Consistency**
- Metadata endpoint reflects imported data
- Search endpoint works with imported data
- Time-range queries work correctly
- Namespace filtering works

## Known Limitations

1. **Test Duration**: The test takes several minutes due to cluster creation and image building
2. **Resource Requirements**: Requires Docker and Kind to be available
3. **Cleanup**: Test cleanup happens via `t.Cleanup()`, but manual cleanup may be needed if test panics
4. **Timing**: Uses `time.Sleep()` in some places for stability; could be replaced with more robust polling

## Troubleshooting

### Test Fails at Step 2 (Generate Test Data)
- Check that the Kind cluster has sufficient resources
- Verify that nginx images can be pulled
- Check Spectre logs for indexing errors

### Test Fails at Step 3 (Export)
- Verify port-forward is working
- Check Spectre logs for export errors
- Ensure sufficient disk space for export file

### Test Fails at Step 6 (Redeploy)
- Verify Docker image builds successfully
- Check Helm chart for errors
- Ensure Kind cluster is still running

### Test Fails at Step 9 (Verify Import)
- Check import report for errors
- Verify Spectre logs for import processing
- Ensure sufficient time for data to be indexed

## Future Improvements

1. **Parallel Resource Creation**: Create deployments in parallel to speed up test
2. **More Resource Types**: Test with StatefulSets, DaemonSets, etc.
3. **Larger Data Sets**: Test with more resources to validate performance
4. **Error Injection**: Test import with corrupted archives
5. **Incremental Import**: Test importing into non-empty storage
6. **Multiple Exports**: Test exporting and importing multiple times

## Related Documentation

- [Export/Import User Guide](../../docs/EXPORT_IMPORT.md)
- [Export/Import Implementation](../../docs/EXPORT_IMPORT_IMPLEMENTATION.md)
- [E2E Test Suite Overview](../../specs/006-e2e-test-suite/quickstart.md)

