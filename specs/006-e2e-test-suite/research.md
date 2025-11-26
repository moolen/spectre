# Research Summary: E2E Test Suite Architecture

**Date**: 2025-11-26
**Feature**: 006-e2e-test-suite
**Status**: Complete

## Key Research Findings

### 1. Cluster Management: Kind

**Decision**: Use Kind (Kubernetes in Docker) for test cluster provisioning

**Rationale**:
- Single-node clusters start in 30-60 seconds
- Native Docker-based (no VMs, lighter weight)
- Perfect for local CI and test environments
- Automatic cleanup possible (single container)
- Industry standard for Kubernetes testing (used by K8s project itself)

**Implementation Pattern**:
```go
// Pseudocode - actual implementation in tests/e2e/helpers/cluster.go
cluster, err := CreateKindCluster("kem-e2e-test")
defer cluster.Delete()  // Cleanup on test exit
```

**Alternatives Considered**:
- **Kubeadm**: Requires more setup, harder to cleanup, overkill for tests
- **Minikube**: Slower startup, VM overhead, not ideal for CI
- **Actual K8s cluster**: Would require external resource, can't be isolated per test

---

### 2. Kubernetes Automation: client-go

**Decision**: Use official Kubernetes Go client-go library for automation

**Rationale**:
- Official library with K8s community support
- Supports all Kubernetes operations needed for tests
- Native Go - no subprocess calls or shell scripting
- Type-safe with comprehensive API coverage
- Well-documented with extensive examples

**Implementation Pattern**:
```go
// Pseudocode - creating deployment programmatically
clientset := kubernetes.NewForConfig(config)
deployment := &appsv1.Deployment{...}
clientset.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
```

**Alternatives Considered**:
- **kubectl subprocess**: Would require parsing output, harder to test, shell-dependent
- **REST API calls**: Would reinvent Kubernetes client logic
- **Third-party wrappers**: Less mature than official client-go

---

### 3. Helm Integration: Helm Go Client

**Decision**: Use official Helm Go client for chart deployments

**Rationale**:
- Helm is the standard for K8s deployments
- Official Go client avoids subprocess complexity
- Supports values overrides programmatically
- Handles chart dependencies and validation
- Integrates cleanly with client-go authentication

**Implementation Pattern**:
```go
// Pseudocode - deploying KEM chart
actionConfig := &action.Configuration{}
install := action.NewInstall(actionConfig)
install.ReleaseName = "kem"
// Deploy with custom values
chartVals := map[string]interface{}{"image.tag": "test-build"}
release, err := install.Run(chart, chartVals)
```

**Alternatives Considered**:
- **`helm install` subprocess**: Would require output parsing, environment setup
- **Manual kubectl apply**: Would lose Helm benefits (lifecycle, upgrades, values)
- **Kustomize**: Less mature for this use case, not the standard for KEM

---

### 4. Assertions & Retries: testify/assert

**Decision**: Use testify library with Eventually for async assertions

**Rationale**:
- `assert.Eventually()` handles polling with timeout
- Built-in backoff and configurable retry intervals
- Works with existing Go testing framework (no test framework change)
- Clear assertion failures with helpful messages
- Widely used in Go testing ecosystem

**Implementation Pattern**:
```go
// Pseudocode - eventually assertion
assert.Eventually(t, func() bool {
  events, err := apiClient.GetEvents(ctx, namespace)
  require.NoError(t, err)
  return len(events) >= expectedCount
}, 10*time.Second, 500*time.Millisecond)
```

**Alternatives Considered**:
- **Manual retry loops**: Boilerplate-heavy, inconsistent error handling
- **ginkgo/gomega**: Heavier BDD framework, overkill for this test suite
- **Custom polling**: Would reinvent Eventually's logic

---

### 5. Port Forwarding: Client-go Exec

**Decision**: Use client-go port-forward capability for API access

**Rationale**:
- Native Go support in client-go for port-forward
- Works within test process (no subprocess overhead)
- Automatic cleanup on context cancel
- Integrates with auth/kubeconfig from deployment

**Implementation Pattern**:
```go
// Pseudocode - port-forward to service
forwarder := portforward.New(...)
go forwarder.ForwardPorts()
// API accessible on localhost:localPort
```

**Alternatives Considered**:
- **kubectl port-forward subprocess**: Would require process management, harder cleanup
- **Direct pod IP**: Not reliable, K8s networking complexity
- **LoadBalancer service**: overkill, Kind doesn't support in same way

---

## Decision Dependencies

1. **Kind** (cluster creation) enables everything else
2. **client-go** (must work with Kind kubeconfig) - established pattern
3. **Helm client** (depends on client-go config) - standard deployment
4. **testify** (works with any test structure) - orthogonal
5. **Port-forward** (depends on client-go) - dependent on client-go choice

---

## Risk Analysis

**Medium Risk**: Event propagation timing
- Events may take 2-5 seconds to appear in API
- Configuration reload may take up to 30 seconds
- **Mitigation**: Use `assert.Eventually` with appropriate timeouts (10-30 seconds)

**Low Risk**: Kind cluster cleanup
- Container-based approach ensures automatic cleanup
- Docker guarantees container isolation

**Low Risk**: Authentication/RBAC
- Kind clusters have no RBAC by default
- Test setup uses system:admin context

---

## Technology Stack Summary

| Component | Technology | Version | Reason |
|-----------|-----------|---------|--------|
| **Cluster** | Kind | >= 0.17 | Fast, isolated, Docker-based |
| **API Automation** | client-go | v0.28+ | Official, comprehensive |
| **Helm Deployment** | Helm client | >= 3.0 | Standard, programmatic |
| **Assertions** | testify | latest | Async-aware, clean API |
| **Language** | Go | 1.21+ | Matches KEM codebase |
| **Port Forwarding** | client-go exec | built-in | Native, no subprocess |

---

## Recommendations for Implementation

1. **Cluster Lifecycle**: Create reusable cluster helper with setup/teardown
2. **API Client Wrapper**: Encapsulate port-forward + API calls
3. **Eventually Patterns**: Create assertion helpers to reduce test boilerplate
4. **Logging**: Use `t.Logf()` for test-aware logging
5. **Fixtures**: Keep YAML files in dedicated directory for clarity
6. **Scenario Independence**: Each test scenario creates its own cluster instance
