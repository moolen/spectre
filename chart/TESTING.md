# Helm Chart Testing Guide

This document describes how to test the Spectre Helm chart at multiple levels: unit tests, integration tests, and local deployment tests.

## Testing Levels

The chart has three levels of testing:

1. **Unit Tests** - Fast template rendering tests using helm-unittest
2. **Integration Tests** - Full deployment tests on Kind cluster
3. **CI Tests** - Automated testing in GitHub Actions

## Unit Tests (helm-unittest)

### Quick Start

```bash
# Install plugin and run tests
make helm-unittest

# Or manually
helm plugin install https://github.com/helm-unittest/helm-unittest.git
helm unittest ./chart --color
```

### What Gets Tested

Unit tests validate template rendering without requiring a Kubernetes cluster:
- ✅ Templates render correctly with default values
- ✅ Conditional resources (ingress, monitoring, PDB, etc.)
- ✅ Value overrides and customization
- ✅ Labels, annotations, and selectors
- ✅ Security contexts and RBAC permissions
- ✅ Resource configurations (probes, volumes, env vars)

### Test Coverage

- **10 test files** covering all templates
- **100+ test cases** covering various scenarios
- **All major features** tested

See `chart/tests/README.md` for detailed documentation.

### Running Unit Tests

```bash
# Run all tests
make helm-unittest

# Run specific test file
helm unittest ./chart -f tests/deployment_test.yaml

# Generate JUnit XML for CI
helm unittest ./chart --output-type JUnit --output-file test-results.xml

# Run with verbose output
helm unittest ./chart -v
```

### Writing New Unit Tests

Create a test file in `chart/tests/`:

```yaml
suite: test my-feature
templates:
  - my-template.yaml
tests:
  - it: should render correctly
    set:
      myFeature:
        enabled: true
    asserts:
      - isKind:
          of: MyResourceKind
      - equal:
          path: spec.myField
          value: expected-value
```

See `chart/tests/README.md` for examples and best practices.

## Integration Tests

Integration tests deploy the chart to a real Kubernetes cluster (Kind) and verify functionality.

### Prerequisites

- Docker
- Helm 3.x
- kubectl
- Kind (Kubernetes in Docker)

### Quick Start

Run complete integration test with a single command:

```bash
make helm-test-local
```

This will:
1. Build the Docker image
2. Create a Kind cluster named `helm-test`
3. Load the image into the cluster
4. Install the Helm chart
5. Run all Helm tests (connection, readiness, RBAC, PVC)
6. Display results

### Manual Integration Testing

#### 1. Lint the Chart

```bash
make helm-lint
```

Or directly with Helm:

```bash
helm lint ./chart
```

#### 2. Template and Validate

```bash
helm template test-release ./chart --debug
```

Test with different configurations:

```bash
# With Ingress
helm template test ./chart --set ingress.enabled=true

# With Autoscaling
helm template test ./chart --set autoscaling.enabled=true

# With Monitoring
helm template test ./chart \
  --set serviceMonitor.enabled=true \
  --set podDisruptionBudget.enabled=true
```

#### 3. Create a Test Cluster

```bash
kind create cluster --name helm-test
```

#### 4. Build and Load Image

```bash
# Build image
make docker-build

# Load into Kind
kind load docker-image spectre:latest --name helm-test
```

#### 5. Install Chart

```bash
helm install spectre-test ./chart \
  --namespace monitoring \
  --create-namespace \
  --set image.repository=spectre \
  --set image.tag=latest \
  --set image.pullPolicy=IfNotPresent \
  --wait
```

#### 6. Run Integration Tests

```bash
helm test spectre-test --namespace monitoring --logs
```

These tests validate:
- Health endpoint (`/health`) is accessible
- Readiness endpoint (`/ready`) is accessible
- RBAC permissions are correctly configured
- PVC is writable

#### 7. Cleanup

```bash
# Remove Helm release
helm uninstall spectre-test --namespace monitoring

# Delete Kind cluster
kind delete cluster --name helm-test
```

Or use the cleanup target:

```bash
make helm-clean
```

## CI Testing

### GitHub Actions Workflow

Tests run automatically on every PR that modifies the chart.

**Workflow: `.github/workflows/helm-tests.yml`**

### CI Pipeline Jobs

#### 1. Unit Tests (`unittest`)
**Duration**: ~1-2 minutes

- Installs helm-unittest plugin
- Runs all 100+ unit tests
- Generates JUnit test reports
- Publishes test results to PR

#### 2. Lint and Validate (`lint-and-validate`)
**Duration**: ~2-3 minutes
**Depends on**: unittest

- Runs `ct lint` (chart-testing)
- Runs `helm lint`
- Templates with default values
- Templates with ingress enabled
- Templates with autoscaling enabled
- Templates with monitoring enabled

#### 3. Kubernetes Integration Test (`kubernetes-test`)
**Duration**: ~8-12 minutes
**Depends on**: unittest, lint-and-validate

- Creates Kind cluster (k8s v1.28)
- Builds application and Docker image
- Loads image into cluster
- Installs chart
- Waits for deployment to be ready
- Runs Helm integration tests
- Collects logs on failure

#### 4. Upgrade Testing (`test-upgrades`)
**Duration**: ~10-15 minutes
**Depends on**: unittest, lint-and-validate

- Installs chart v1
- Runs tests
- Upgrades to v2 with modified values
- Verifies upgrade succeeded
- Runs tests again

#### 5. Security Scanning (`security-scan`)
**Duration**: ~3-5 minutes
**Depends on**: unittest, lint-and-validate

- Templates chart for static analysis
- Runs Checkov security scanner
- Verifies security contexts
- Checks non-root user configuration
- Validates capability dropping

**Total CI Time**: ~25-35 minutes (jobs run in parallel)

### Triggering CI Tests

Tests run automatically when:
- PR is created/updated with changes to `chart/**`
- Changes pushed to `master` affecting `chart/**`
- Workflow file `.github/workflows/helm-tests.yml` is modified

### Viewing CI Results

1. Navigate to GitHub Actions tab
2. Select "Helm Chart Tests" workflow
3. View individual job results and logs
4. Check test result summaries on PR

## Test Coverage Matrix

| Component | Unit Tests | Integration Tests | Security Scan |
|-----------|------------|------------------|---------------|
| Deployment | ✅ 40+ tests | ✅ | ✅ |
| Service | ✅ 14 tests | ✅ | ✅ |
| ServiceAccount | ✅ 7 tests | ✅ | ✅ |
| RBAC | ✅ 10 tests | ✅ | ✅ |
| ConfigMap | ✅ 4 tests | ✅ | ✅ |
| PVC | ✅ 11 tests | ✅ | ✅ |
| Ingress | ✅ 10 tests | ✅ | ✅ |
| ServiceMonitor | ✅ 10 tests | ✅ | ✅ |
| PodDisruptionBudget | ✅ 7 tests | ✅ | ✅ |

## Common Issues

### Unit Test Failures

**Problem**: Test fails with "Path not found"

**Solution**: Verify path exists in template:
```bash
helm template test ./chart | grep -A 10 "spec.myField"
```

**Problem**: helm-unittest plugin not installed

**Solution**: Install plugin:
```bash
make helm-unittest-install
```

### Integration Test Failures

**Problem**: Cannot pull Docker image in cluster

**Solution**: Ensure image is loaded into Kind cluster:
```bash
kind load docker-image spectre:latest --name helm-test
```

**Problem**: Deployment not ready timeout

**Solution**: Check pod logs for errors:
```bash
kubectl logs -n monitoring -l app.kubernetes.io/name=spectre
kubectl describe pod -n monitoring -l app.kubernetes.io/name=spectre
```

**Problem**: PVC test fails

**Solution**: Verify PVC is bound:
```bash
kubectl get pvc -n monitoring
kubectl describe pvc -n monitoring
```

**Problem**: Service account test fails

**Solution**: Verify RBAC:
```bash
kubectl get clusterrole spectre-test-spectre
kubectl get clusterrolebinding spectre-test-spectre
```

## Advanced Testing

### Testing with Custom Values

Create a values file (`test-values.yaml`):
```yaml
replicaCount: 2
persistence:
  size: 5Gi
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: spectre-test.local
      paths:
        - path: /
          pathType: Prefix
```

Install with custom values:
```bash
helm install spectre-test ./chart \
  --namespace monitoring \
  --create-namespace \
  -f test-values.yaml
```

### Testing Upgrades

```bash
# Initial install
helm install spectre-test ./chart --namespace monitoring

# Make changes to chart

# Upgrade
helm upgrade spectre-test ./chart --namespace monitoring

# Run tests again
helm test spectre-test --namespace monitoring
```

### Debugging Failed Tests

View test pod logs:
```bash
kubectl logs -n monitoring spectre-test-spectre-test-connection
kubectl logs -n monitoring spectre-test-spectre-test-readiness
kubectl logs -n monitoring spectre-test-spectre-test-sa
kubectl logs -n monitoring spectre-test-spectre-test-pvc
```

Describe test pods:
```bash
kubectl describe pod -n monitoring -l "helm.sh/hook=test"
```

Keep test pods for debugging:
```bash
helm test spectre-test --namespace monitoring --logs --debug
```

## Makefile Targets Reference

| Target | Description | Duration |
|--------|-------------|----------|
| `make helm-unittest` | Run unit tests | ~5-10s |
| `make helm-unittest-install` | Install helm-unittest plugin | ~5s |
| `make helm-lint` | Lint the chart | ~1-2s |
| `make helm-template` | Template with default values | ~1-2s |
| `make helm-test` | Run tests on existing cluster | ~5-10min |
| `make helm-test-local` | Create Kind cluster and test | ~8-15min |
| `make helm-clean` | Clean up test resources | ~10-30s |

## Best Practices

### Before Committing

1. **Run unit tests**: `make helm-unittest`
2. **Lint chart**: `make helm-lint`
3. **Template verification**: `make helm-template`

### Local Development

1. **Test locally first**: `make helm-test-local`
2. **Test different configurations**: Enable ingress, monitoring, etc.
3. **Check logs on failure**: Review pod logs for errors
4. **Clean up after testing**: `make helm-clean`

### Writing Tests

1. **Unit tests for template logic**: Test conditionals and value overrides
2. **Integration tests for functionality**: Test actual deployment
3. **Meaningful test names**: Clearly describe what is being tested
4. **Test edge cases**: Empty values, null values, max values
5. **Keep tests focused**: One concept per test

### CI/CD

1. **All tests must pass**: No merging with failing tests
2. **Review test output**: Check CI logs for warnings
3. **Update tests with changes**: Keep tests in sync with templates
4. **Monitor test duration**: Keep tests fast

## Contributing

When adding new features to the chart:

1. Update templates
2. Add unit tests to `chart/tests/`
3. Run `make helm-unittest` locally
4. Run `make helm-test-local` for integration test
5. Ensure all CI tests pass on PR
6. Update `chart/README.md` with new parameters
7. Update this TESTING.md if needed

## Resources

- **Unit Tests**: `chart/tests/README.md`
- **Workflow Docs**: `.github/workflows/README.md`
- **Chart Docs**: `chart/README.md`
- **helm-unittest**: https://github.com/helm-unittest/helm-unittest
- **Helm Best Practices**: https://helm.sh/docs/chart_best_practices/

## Quick Reference

```bash
# Unit tests (fast, no cluster needed)
make helm-unittest

# Integration tests (requires cluster)
make helm-test-local

# Just lint
make helm-lint

# Cleanup
make helm-clean
```

The Spectre Helm chart has comprehensive testing at all levels to ensure reliability and quality!
