# E2E Test Suite for Kubernetes Event Monitor (KEM)

This directory contains end-to-end tests for the Kubernetes Event Monitor application.

## Overview

The e2e test suite validates KEM's ability to:

1. **Capture Kubernetes audit events** in default configuration
2. **Persist events across pod restarts** (durability)
3. **Dynamically reload watch configuration** (operational flexibility)

## Test Scenarios

### Scenario 1: Default Resource Event Capture (TestScenarioDefaultResources)

Validates default KEM configuration watches Deployments and Pods correctly.

**Key validations:**
- Deployment creation events are captured
- Namespace filtering works (matching, all, cross-namespace)
- Metadata endpoint returns correct namespaces and kinds
- Event details contain required fields

**Expected execution time:** ~2 minutes

### Scenario 2: Pod Restart Durability (TestScenarioPodRestart)

Validates that KEM persists events across pod restarts.

**Key validations:**
- Events persist after KEM pod restart
- Original events remain accessible
- New events are captured after restart
- Data integrity maintained across restart

**Expected execution time:** ~2 minutes

### Scenario 3: Dynamic Configuration Reload (TestScenarioDynamicConfig)

Validates KEM's ability to dynamically reload watch configuration.

**Key validations:**
- Default config excludes unconfigured resource types (StatefulSet)
- Configuration updates trigger new resource watching
- Metadata endpoint reflects configuration changes
- Multiple resource types coexist

**Expected execution time:** ~1.5 minutes

## Running Tests

### Prerequisites

```bash
# Kubernetes tools
kind --version        # Kubernetes in Docker
kubectl --version     # Kubernetes CLI
helm --version        # Helm (for KEM deployment)

# Go
go version            # Go 1.21 or later

# Docker
docker ps             # Docker daemon running
```

### Run All Tests

```bash
cd /home/moritz/dev/rpk
go test -v ./tests/e2e
```

### Run Specific Test

```bash
go test -v -run TestScenarioDefaultResources ./tests/e2e
go test -v -run TestScenarioPodRestart ./tests/e2e
go test -v -run TestScenarioDynamicConfig ./tests/e2e
```

### Run Without E2E

To skip e2e tests in CI:

```bash
go test -short -v ./tests/e2e
```

## Project Structure

```
tests/e2e/
├── main_test.go                          # Test entry points
├── helpers/                              # Reusable test infrastructure
│   ├── cluster.go                        # Kind cluster management
│   ├── k8s.go                           # Kubernetes operations
│   ├── api.go                           # KEM API client
│   ├── assertions.go                    # Eventually assertions
│   ├── deployment.go                    # Deployment builders
│   ├── helm.go                          # Helm deployment
│   └── portforward.go                   # Port-forward utilities
├── scenarios/                            # Test scenario implementations
│   └── doc.go                           # Package documentation
├── fixtures/                             # Test data and configs
│   ├── nginx-deployment.yaml
│   ├── test-statefulset.yaml
│   ├── watch-config-default.yaml
│   ├── watch-config-extended.yaml
│   └── helm-values-test.yaml
└── .gitignore
```

## Test Infrastructure

### Cluster Management (cluster.go)

- **CreateKindCluster()**: Creates isolated Kind clusters for each test
- **Delete()**: Cleans up clusters and kubeconfig files
- Automatic context and kubeconfig management

### Kubernetes Operations (k8s.go)

- Namespace management (create, delete, list)
- Deployment management (create, get, delete, search)
- Pod operations (get, list, delete, wait for ready)
- Cluster version queries

### API Client (api.go)

Implements all 5 KEM API endpoints:
- **GET /v1/search** - Resource discovery with filtering
- **GET /v1/metadata** - Aggregated metadata
- **GET /v1/resources/{id}** - Single resource details
- **GET /v1/resources/{id}/segments** - Status timeline
- **GET /v1/resources/{id}/events** - Audit events

### Eventually Assertions (assertions.go)

Async-aware test assertions with configurable timeouts:
- **EventuallyAPIAvailable()** - Wait for API readiness
- **EventuallyResourceCreated()** - Wait for resource in API
- **EventuallyEventCreated()** - Wait for specific event
- **EventuallyEventCount()** - Wait for event threshold
- **EventuallyCondition()** - Custom condition polling

### Builders (deployment.go)

Fluent API for creating test resources:
- **DeploymentBuilder** - Customizable Kubernetes Deployments
- **StatefulSetBuilder** - Customizable StatefulSets

### Helm Deployment (helm.go)

- InstallChart() - Deploy Helm charts
- UpgradeChart() - Update deployments
- UninstallChart() - Clean up

### Port-Forward (portforward.go)

- Port-forward to services for API access
- Automatic port allocation
- Health check on startup

## Test Execution Flow

Each test follows this pattern:

```
1. Create Kind cluster (30-60s)
2. Create Kubernetes client
3. Verify cluster connectivity
4. Create test namespace
5. Create test resources (deployments, statefulsets)
6. Verify API access via port-forward
7. Query API and validate results
8. [Scenario-specific operations]
9. Cleanup all resources
10. Delete cluster
```

## Timing Expectations

| Operation | Duration |
|-----------|----------|
| Kind cluster creation | 30-60s |
| Docker image build | 20-40s |
| Helm deployment | 30-45s |
| Test scenario execution | 30-60s |
| Cluster cleanup | 10-20s |
| **Total per test** | **2-4 minutes** |

## Event Propagation

- **Event capture**: 2-5 seconds (async processing)
- **API query timeout**: 10 seconds (with retries)
- **Configuration reload**: Up to 30 seconds

All tests use `assert.Eventually()` with appropriate timeouts to handle these delays.

## Debugging

### Monitor During Tests

```bash
# Watch cluster resources
watch kubectl --context kind-kem-e2e get all -A

# Follow KEM logs
kubectl --context kind-kem-e2e logs -f -l app=kem

# Check specific pod
kubectl --context kind-kem-e2e describe pod -l app=kem

# Manual API testing
kubectl --context kind-kem-e2e port-forward svc/kem-api 8080:8080 &
curl -s http://localhost:8080/v1/search | jq .
```

### Common Issues

**"kind: command not found"**
```bash
go install sigs.k8s.io/kind@latest
```

**"Cannot connect to Docker daemon"**
```bash
# Start Docker daemon
sudo systemctl start docker  # Linux
open -a Docker              # macOS
```

**"Test hangs on cluster creation"**
- Check Docker resources: `docker system df`
- Free space if needed: `docker system prune`

**"API connection timeout"**
- KEM may not be ready yet
- Tests retry automatically
- Check logs for errors

## Performance Baselines

Expected response times (p95):
- /v1/search: < 5 seconds
- /v1/metadata: < 5 seconds
- /v1/resources/{id}: < 5 seconds
- /v1/resources/{id}/segments: < 5 seconds
- /v1/resources/{id}/events: < 5 seconds

## Design Decisions

### Why Kind?
- Fast cluster startup (30-60s)
- Lightweight (single Docker container)
- Easy cleanup and isolation per test
- Standard for Kubernetes testing

### Why Eventually Assertions?
- Events have async propagation delay (2-5s)
- Configuration reload takes time (30s)
- Poll-based assertions handle variability
- Configurable timeouts for different operations

### Why Separate Namespaces?
- Test isolation prevents cross-contamination
- Namespace filtering validation requires multiple namespaces
- Easy cleanup via namespace deletion
- Matches real-world usage patterns

## Known Limitations

1. **KEM Deployment**: Tests expect KEM to be deployed via Helm
   - Currently placeholders for when KEM chart is available
   - Tests can run against pre-deployed KEM at localhost:8080

2. **Pod Restart**: Simulated via sleep
   - Real implementation would delete and recreate pod
   - Requires KEM deployment in cluster

3. **Configuration Reload**: Simulated via sleep
   - Real implementation would update ConfigMap and trigger annotation
   - Requires KEM to support configuration reloading

## Next Steps

1. Deploy KEM in test cluster via Helm
2. Implement actual pod restart and config reload logic
3. Add performance benchmarks
4. Add failure scenario tests (network issues, timeouts)
5. Add stress testing (large number of resources)

## References

- [KEM Specification](../../specs/006-e2e-test-suite/spec.md)
- [API Contract](../../specs/006-e2e-test-suite/contracts/k8s-event-monitor-api.md)
- [Data Model](../../specs/006-e2e-test-suite/data-model.md)
- [Quickstart Guide](../../specs/006-e2e-test-suite/quickstart.md)
