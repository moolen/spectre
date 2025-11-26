# Quickstart: E2E Test Suite

Get the end-to-end test suite running in 10 minutes.

## Prerequisites

Verify you have these tools installed:

```bash
# Kind for cluster creation
kind --version
# Expected output: kind version vX.XX.X

# Kubernetes CLI
kubectl --version
# Expected output: Client Version: vX.XX.X

# Helm for deployments
helm --version
# Expected output: version.BuildInfo{Version:"vX.X.X",...}

# Docker (required by Kind)
docker ps
# Should return without error (daemon running)

# Go (for running tests)
go version
# Expected output: go version go1.21 (or later)
```

If any tool is missing, install it first before proceeding.

## Quick Start: 5 Minutes

### 1. Clone and Navigate

```bash
cd /path/to/rpk
cd tests/e2e
```

### 2. Run All Tests

```bash
go test -v ./...
```

Expected output:
```
=== RUN   TestScenarioDefaultResources
...
=== RUN   TestScenarioPodRestart
...
=== RUN   TestScenarioDynamicConfig
...
--- PASS: all tests (4m32s)
ok      github.com/moritz/rpk/tests/e2e
```

### 3. That's It!

The test suite will:
- ✅ Create Kind cluster
- ✅ Build KEM binary/image
- ✅ Deploy via Helm
- ✅ Run all 3 test scenarios
- ✅ Clean up cluster automatically

---

## Running Specific Scenarios

Run one scenario at a time:

### Scenario 1: Default Resource Event Capture

```bash
go test -v -run TestScenarioDefaultResources ./scenarios
```

Tests:
- ✓ Capture Deployment create event
- ✓ Filter by namespace
- ✓ Query all namespaces
- ✓ Verify cross-namespace filtering

**Expected time**: 2 minutes

### Scenario 2: Pod Restart Durability

```bash
go test -v -run TestScenarioPodRestart ./scenarios
```

Tests:
- ✓ Capture events before restart
- ✓ Restart KEM Pod
- ✓ Verify events still accessible

**Expected time**: 2 minutes

### Scenario 3: Dynamic Configuration Reload

```bash
go test -v -run TestScenarioDynamicConfig ./scenarios
```

Tests:
- ✓ Update watch configuration
- ✓ Trigger reload via annotation
- ✓ Verify new resource type is watched
- ✓ Capture events from new resource type

**Expected time**: 1.5 minutes

---

## Debugging: Monitor During Tests

In a separate terminal, watch the cluster while tests run:

```bash
# See all resources in all namespaces
watch kubectl --context kind-kem-e2e get all -A

# Watch KEM Pod logs
kubectl --context kind-kem-e2e logs -f -l app=kem

# Describe a specific Pod (when something fails)
kubectl --context kind-kem-e2e describe pod -l app=kem

# Manual API testing (in another terminal)
kubectl --context kind-kem-e2e port-forward svc/kem-api 8080:8080 &
curl -s http://localhost:8080/v1/search | jq .
curl -s http://localhost:8080/v1/metadata | jq .
```

---

## Verifying the Test Setup

Before running full tests, verify individual components:

### Test 1: Kind Works

```bash
kind get clusters
# Should show: kind-kem-e2e (if test is running)
```

### Test 2: Helm Repository Access

```bash
helm repo list
# Should show kem repository configured
```

### Test 3: Docker Image Available

```bash
docker image ls | grep kem
# Should show kem:latest or kem:test-build
```

---

## Troubleshooting

### Issue: "kind: command not found"

**Solution**: Install Kind
```bash
go install sigs.k8s.io/kind@latest
```

### Issue: "Cannot connect to the Docker daemon"

**Solution**: Start Docker daemon
```bash
# On macOS
open -a Docker

# On Linux
sudo systemctl start docker

# Then verify
docker ps
```

### Issue: "Test hangs on cluster creation"

**Solution**: Check Docker resources
```bash
docker system df  # Show usage
docker system prune  # Free up space if needed
```

### Issue: "API connection timeout"

**Solution**: The API may not be ready yet
- By design, tests use `assert.Eventually` with retries
- If timeout is too short, increase wait time in test
- Check KEM logs: `kubectl logs -l app=kem`

### Issue: "Configuration reload times out"

**Solution**: The system may need more time
- Check annotation was applied: `kubectl get pod -o jsonpath='{.items[0].metadata.annotations}'`
- Verify config file was updated: `kubectl get configmap kem-watch-config -o yaml`
- Increase timeout in test if needed (default: 30 seconds)

---

## Output: What to Expect

### Successful Test Run

```
=== RUN   TestScenarioDefaultResources
    helpers.go:42: Creating Kind cluster: kem-e2e-test-001
    helpers.go:58: Building KEM Docker image
    helpers.go:75: Deploying KEM via Helm
    helpers.go:92: Waiting for KEM Pod ready
    scenario_default_resources_test.go:31: Creating test Deployment in 'test-default' namespace
    scenario_default_resources_test.go:48: Querying API for deployment events
    scenario_default_resources_test.go:52: ✓ Found 5 events for deployment
    scenario_default_resources_test.go:65: Testing namespace filter
    scenario_default_resources_test.go:72: ✓ Namespace filter returned 5 events
    scenario_default_resources_test.go:85: Testing cross-namespace filtering
    scenario_default_resources_test.go:91: ✓ Cross-namespace filter returned 0 events
    helpers.go:115: Deleting Kind cluster
--- PASS: TestScenarioDefaultResources (120.50s)

PASS
ok    github.com/moritz/rpk/tests/e2e/scenarios  120.50s
```

### Test Failure Output

```
    scenario_default_resources_test.go:52: Assertion failed
        Expected: 5 events
        Actual: 0 events
        Query: namespace=test-default, kind=Deployment
        Time range: 2025-11-26T10:15:00Z to 2025-11-26T10:16:00Z

    Debugging tips:
        - Check KEM logs: kubectl logs -l app=kem -c kem
        - Check Deployment exists: kubectl get deployment -n test-default
        - Verify events manually: kubectl get events -n test-default
```

---

## Next Steps

After tests pass:

1. **Read the specification**: `cat ../spec.md`
2. **Review the data model**: `cat ../data-model.md`
3. **Check API contracts**: `ls ../contracts/`
4. **Explore the implementation**: `ls helpers/ scenarios/`

---

## Common Patterns

### Add a New Test

1. Create file: `scenarios/scenario_mytest_test.go`
2. Use helpers: `cluster.CreateCluster()`, `cluster.DeployKEM()`
3. Query API: `apiClient.Query(ctx, query)`
4. Assert: `assert.Eventually(t, checkFunc, timeout, interval)`
5. Cleanup: Use `defer cluster.Delete()`

### Custom Assertions

```go
// Check event exists with specific properties
assert.Eventually(t, func() bool {
    events, err := apiClient.GetEvents(ctx, query)
    require.NoError(t, err)

    for _, evt := range events {
        if evt.Kind == "Deployment" && evt.Verb == "create" {
            return true  // Found it
        }
    }
    return false  // Not found yet
}, 10*time.Second, 500*time.Millisecond)
```

### Port-Forward Without Tests

```bash
# Keep cluster running
kubectl --context kind-kem-e2e get nodes

# In another terminal, port-forward
kubectl --context kind-kem-e2e port-forward svc/kem-api 8080:8080

# Query API
curl http://localhost:8080/v1/search
```

---

## Performance Baselines

Typical timing on standard hardware:

| Operation | Time |
|-----------|------|
| Kind cluster creation | 30-60s |
| Docker image build | 20-40s |
| Helm deployment | 30-45s |
| Test scenario (default resources) | 30-60s |
| Test scenario (pod restart) | 45-90s |
| Test scenario (dynamic config) | 60-120s |
| Cluster cleanup | 10-20s |
| **Total** | **4-6 minutes** |

If any operation significantly exceeds these times, check:
- Docker image availability (pre-build vs on-demand)
- Network connectivity (pulling images)
- System resources (CPU, disk space)

---

## Getting Help

**Test fails?**
1. Run with more logging: `go test -v -run TestName`
2. Keep cluster running: Comment out `cluster.Delete()` to inspect
3. Check logs: `kubectl logs --tail=100 -l app=kem`
4. Check events: `kubectl get events -A`

**Want to understand internals?**
1. Read `../spec.md` for requirements
2. Read `../data-model.md` for entity design
3. Read helper code: `helpers/cluster.go`, `helpers/api.go`
4. Read test code: `scenarios/scenario_*.go`

**Found a bug?**
1. Reduce to minimal test case
2. Document expected vs actual behavior
3. Save cluster state for debugging: `kubectl config view > /tmp/debug-kubeconfig`
