# Graph Integration Test Suite

This directory contains integration tests for the graph database layer. The tests use testcontainers to spin up isolated FalkorDB instances for each test, ensuring fast, reliable, and parallelizable tests.

## Overview

The test suite validates the full stack from event ingestion through the graph pipeline to API endpoints:

- **Pipeline Tests**: Verify events are correctly transformed into graph nodes and edges
- **Query Executor Tests**: Verify Cypher queries return correct results
- **Timeline Endpoint Tests**: Verify timeline API endpoint behavior
- **Root Cause Endpoint Tests**: Verify root cause analysis functionality
- **Scenario Tests**: Test real-world failure scenarios
- **Performance Tests**: Ensure tests remain fast with larger datasets

## Running Tests

### Prerequisites

- Docker must be running (testcontainers requirement)
- Go 1.24+

### Run All Tests

```bash
make test-graph-integration-suite
```

### Run with Coverage

```bash
make test-graph-integration-coverage
```

### Run Specific Test

```bash
make test-graph-integration-single TEST=TestPipeline_ProcessSingleEvent
```

Or directly:

```bash
go test -v -tags=integration ./tests/integration/graph/... -run TestPipeline_ProcessSingleEvent
```

## Test Structure

### Core Infrastructure

- `harness.go`: Test harness with testcontainers integration
- `fixtures.go`: Helper functions to create test events
- `helpers.go`: Assertion utilities and test helpers
- `audit_log.go`: Utilities for loading events from audit log files (JSONL format)

### Test Files

- `pipeline_test.go`: Pipeline integration tests
- `query_executor_test.go`: Query executor tests
- `timeline_test.go`: Timeline endpoint tests
- `root_cause_test.go`: Root cause endpoint tests
- `scenarios_test.go`: Real-world scenario tests
- `performance_test.go`: Performance tests

## Test Harness

The `TestHarness` manages:

- FalkorDB container lifecycle (start/stop via testcontainers)
- Graph schema initialization
- Pipeline setup and teardown
- Test data seeding
- Cleanup between tests

Each test gets a fresh graph database instance with a unique graph name, ensuring complete isolation.

## Writing Tests

### Basic Test Pattern

```go
func TestMyFeature(t *testing.T) {
    harness, err := NewTestHarness(t)
    require.NoError(t, err)
    defer harness.Cleanup(context.Background())

    ctx := context.Background()
    client := harness.GetClient()

    // Create test events
    event := CreatePodEvent(uid, name, namespace, timestamp, eventType, status, nil)

    // Process events
    err = harness.SeedEvent(ctx, event)
    require.NoError(t, err)

    // Assert results
    AssertResourceExists(t, client, uid)
    AssertEventCount(t, client, uid, 1)
}
```

### Creating Events

Use the fixture functions in `fixtures.go`:

- `CreatePodEvent()` - Create Pod events
- `CreateDeploymentEvent()` - Create Deployment events
- `CreateServiceEvent()` - Create Service events
- `CreateHelmReleaseEvent()` - Create HelmRelease events
- `CreateOwnershipChain()` - Create ownership chains
- `CreateFailureScenario()` - Create failure scenarios

### Assertions

Use the helper functions in `helpers.go`:

- `AssertResourceExists()` - Verify resource exists
- `AssertEventCount()` - Verify event count
- `AssertEdgeExists()` - Verify edge exists
- `CountResources()` - Count total resources
- `CountEdges()` - Count edges by type

### Using Audit Logs

Audit logs provide a way to use real event data from live clusters in tests. This is useful for testing with actual production scenarios.

#### Capturing Events

To capture events from a live cluster, run Spectre with the `--audit-log` flag:

```bash
./spectre server \
    --watcher-enabled \
    --audit-log=/tmp/spectre-audit-log.jsonl \
    --graph-enabled \
    --graph-host=localhost
```

This will write all processed events to the specified JSONL file.

#### Using in Tests

Load events from an audit log file:

```go
func TestTimelineEndpoint_WithRealEvents(t *testing.T) {
    harness, err := NewTestHarness(t)
    require.NoError(t, err)
    defer harness.Cleanup(context.Background())

    ctx := context.Background()

    // Load events from captured audit log
    err := harness.SeedEventsFromAuditLog(ctx, "testdata/crashloop-scenario.jsonl")
    require.NoError(t, err)

    // Run tests with real event data
    executor := graph.NewQueryExecutor(harness.GetClient())
    // ... test queries ...
}
```

#### Filtering Events

Load only specific events:

```go
// Load only Pod events
podEvents, err := LoadAuditLogByResource("testdata/events.jsonl", "Pod")
require.NoError(t, err)

// Load events from specific namespace
namespaceEvents, err := LoadAuditLogByNamespace("testdata/events.jsonl", "default")
require.NoError(t, err)

// Load with custom filter
filteredEvents, err := LoadAuditLogFiltered("testdata/events.jsonl", func(e models.Event) bool {
    return e.Resource.Kind == "Pod" && e.Type == models.EventTypeCreate
})
require.NoError(t, err)

// Seed filtered events
err = harness.SeedEventsFromAuditLogFiltered(ctx, "testdata/events.jsonl", func(e models.Event) bool {
    return e.Resource.Kind == "Pod"
})
require.NoError(t, err)
```

#### Test Fixtures

Store audit log files in the `fixtures/` directory:

- `fixtures/crashloop-scenario.jsonl` - Pod crash loop scenario
- `fixtures/deployment-rollout.jsonl` - Deployment rollout events
- `fixtures/resource-cleanup.jsonl` - Resource deletion events

These fixtures can be version controlled and provide reproducible test scenarios.

## Best Practices

1. **One Test Per Scenario**: Each test should verify one specific behavior
2. **Descriptive Names**: Test names should clearly describe what they test
3. **Setup/Teardown**: Always use `defer harness.Cleanup()` for cleanup
4. **Isolation**: Each test gets a fresh graph database
5. **Batch Events**: Use `SeedEvents()` for multiple events to improve performance

## Troubleshooting

### Container Startup Failures

- Check Docker is running: `docker ps`
- Verify port 6379 is not in use
- Increase container startup timeout in `harness.go` if needed

### Test Timeouts

- Check for slow queries
- Verify test data size is reasonable
- Check for resource leaks

### Flaky Tests

- Ensure proper cleanup between tests
- Use unique graph names per test (handled by harness)
- Check for race conditions in concurrent tests

## Performance

Tests are designed to run quickly:

- Individual tests should complete in < 1 second
- Full test suite should complete in < 30 seconds
- Tests can run in parallel (testcontainers handles isolation)

## CI/CD Integration

The test suite is designed to run in CI/CD pipelines. See `.github/workflows/test-graph-integration.yml` for GitHub Actions integration.
