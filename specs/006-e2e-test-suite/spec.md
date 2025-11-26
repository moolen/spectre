# Feature Specification: End-to-End Test Suite for Kubernetes Event Monitor

**Feature Branch**: `006-e2e-test-suite`
**Created**: 2025-11-26
**Status**: Draft
**Input**: Comprehensive e2e test suite for KEM with Kind cluster, Docker image deployment, and multi-scenario testing

## User Scenarios & Testing *(mandatory)*

<!--
  IMPORTANT: User stories should be PRIORITIZED as user journeys ordered by importance.
  Each user story/journey must be INDEPENDENTLY TESTABLE - meaning if you implement just ONE of them,
  you should still have a viable MVP (Minimum Viable Product) that delivers value.
  
  Assign priorities (P1, P2, P3, etc.) to each story, where P1 is the most critical.
  Think of each story as a standalone slice of functionality that can be:
  - Developed independently
  - Tested independently
  - Deployed independently
  - Demonstrated to users independently
-->

### User Story 1 - Verify Event Capture for Default Watched Resources (Priority: P1)

Platform engineers need to validate that the Kubernetes Event Monitor correctly captures audit events for the default watched resource types (Deployments and Pods) when deployed via Helm chart. This is the foundation test that verifies basic functionality works as expected.

**Why this priority**: This is the core functionality - if the system doesn't capture events for the default resource types, nothing else matters. This must work before any other testing.

**Independent Test**: Can be fully tested by deploying a Deployment via kubectl and verifying API returns events within the expected time window, then demonstrating filtering by namespace works correctly.

**Acceptance Scenarios**:

1. **Given** a Kind cluster with KEM deployed via Helm with default configuration, **When** a Deployment with nginx image is created in a namespace, **Then** audit events for that Deployment appear in API responses within the expected time frame
2. **Given** events exist in the system, **When** API is queried with a specific namespace filter, **Then** only events from that namespace are returned
3. **Given** events exist from multiple namespaces, **When** API is queried without filters, **Then** all events are returned
4. **Given** events exist in other namespaces, **When** API is queried with a filter for a different namespace, **Then** no events from the current namespace are returned

---

### User Story 2 - Verify Event Persistence Through Pod Restarts (Priority: P1)

Platform engineers need assurance that captured events remain accessible and queryable even after Pod restarts. This validates data durability and confirms that the event storage mechanism survives component failures.

**Why this priority**: Data loss on Pod restart is a critical issue. Teams must trust that events won't disappear after infrastructure restarts. This is foundational for production use.

**Independent Test**: Can be fully tested by capturing events from a Pod modification, restarting the Pod, then re-querying the API and confirming the same events are still accessible.

**Acceptance Scenarios**:

1. **Given** audit events have been captured for a Pod, **When** that Pod is restarted, **Then** the Pod regains connectivity to the storage backend
2. **Given** a Pod has been restarted, **When** API is queried for previously captured events, **Then** all events remain accessible with correct metadata

---

### User Story 3 - Verify Dynamic Configuration Reloading (Priority: P2)

Platform engineers need the ability to extend the watched resource types by updating configuration and having the system automatically pick up the changes without full cluster restart. This enables operational flexibility and gradual expansion of monitoring coverage.

**Why this priority**: While important for operational flexibility, this is less critical than capturing default resources or data persistence. Teams can work around this by redeploying, but dynamic reconfiguration is a significant operational improvement.

**Independent Test**: Can be fully tested by updating watch config to include a new resource type, triggering a remount via annotation, creating that resource type, and verifying events appear in the API.

**Acceptance Scenarios**:

1. **Given** the watch configuration includes only default resources, **When** configuration is updated to add a new resource type not in defaults, **Then** the configuration change is persisted
2. **Given** configuration has been updated, **When** watcher Pod is annotated with please-remount trigger, **Then** the configuration is reloaded within a defined window (allowing for propagation delays)
3. **Given** configuration has been reloaded, **When** the newly-watched resource type is created, **Then** audit events appear in the API, confirming dynamic watch capability

### Edge Cases

- What happens when API queries span time windows where events don't exist?
- How does the system behave when filtering for namespaces that don't exist?
- How does event availability change during the remount/reload window (before it completes)?
- What happens if Pod restart occurs while events are being written?
- How does the system handle rapid successive configuration changes?

## Requirements *(mandatory)*

<!--
  ACTION REQUIRED: The content in this section represents placeholders.
  Fill them out with the right functional requirements.
-->

### Functional Requirements

- **FR-001**: Test suite MUST programmatically create a Kind cluster from scratch
- **FR-002**: Test suite MUST build the KEM binary/container image
- **FR-003**: Test suite MUST deploy KEM into the Kind cluster using Helm chart
- **FR-004**: Test suite MUST verify API accessibility via port-forwarding to the KEM service
- **FR-005**: Test suite MUST validate that default Helm chart configuration watches Deployments and Pods
- **FR-006**: Test suite MUST create a test Deployment with nginx image and verify event capture
- **FR-007**: Test suite MUST query the API with time-range filtering to retrieve events within expected window
- **FR-008**: Test suite MUST verify namespace-based filtering returns only matching namespace events
- **FR-009**: Test suite MUST verify unfiltered API queries return all events from all namespaces
- **FR-010**: Test suite MUST verify filtered queries for non-existent namespaces return no events
- **FR-011**: Test suite MUST restart a Pod and verify previously captured events remain queryable
- **FR-012**: Test suite MUST update the watch configuration to add a resource type not in defaults
- **FR-013**: Test suite MUST trigger configuration reload via Pod annotation with timestamp marker
- **FR-014**: Test suite MUST wait for configuration propagation (15 seconds recommended) before testing new resource
- **FR-015**: Test suite MUST create the newly-watched resource and verify events appear in API
- **FR-016**: Test suite MUST implement retry logic with eventual consistency assertions for all API queries due to potential propagation delays
- **FR-017**: Test suite MUST be independently runnable and require no manual setup beyond cluster prerequisites

### Key Entities

- **Audit Event**: Represents a Kubernetes API action (create, update, patch, delete) with timestamp, user, resource reference, and action details
- **Watch Configuration**: Defines which resource types (Kind, API Group, namespace) are monitored by the event capture system
- **Test Cluster**: Ephemeral Kind cluster created during test execution, destroyed after test completion
- **API Response**: JSON object containing events array, count, and execution metadata for a query

## Success Criteria *(mandatory)*

<!--
  ACTION REQUIRED: Define measurable success criteria.
  These must be technology-agnostic and measurable.
-->

### Measurable Outcomes

- **SC-001**: Test suite completes full setup (cluster creation, build, deployment) in under 5 minutes for standard hardware
- **SC-002**: All three test scenarios (default resources, restart durability, dynamic config) pass with 100% success rate
- **SC-003**: Event API queries complete and return results within 5 seconds, with 95th percentile under 10 seconds even with retry logic
- **SC-004**: Configuration reload is detected and active within 30 seconds of annotation trigger (allowing for 15s propagation + 15s buffer)
- **SC-005**: Test suite provides clear pass/fail status for each scenario with descriptive error messages on failure
- **SC-006**: Test suite can be re-run against the same cluster or cleaned up and recreated without manual intervention
- **SC-007**: All assertions use eventual consistency patterns to handle asynchronous event propagation (max 10 retries with 1s interval)

## Assumptions

- **Test Environment**: Kind is installed and available on test machine; Docker daemon is running
- **Helm**: Helm 3.x is installed and has access to KEM Helm chart repository
- **kubectl Access**: kubectl is configured with access to create and manage resources in Kind cluster
- **Timing**: Event propagation through storage layer takes under 5 seconds; configuration reload via annotation takes under 30 seconds
- **Port Forwarding**: kubectl port-forward is the stable mechanism for accessing KEM API during tests
- **Data Storage**: KEM uses a durable storage backend that persists across Pod restarts in Kind cluster
- **Resource Cleanup**: Test suite performs cleanup (cluster deletion) after completion to avoid resource exhaustion
