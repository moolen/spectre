---
phase: 24-data-model-ingestion
plan: 04
subsystem: signal-ingestion-verification
tags: [grafana, signals, integration-test, end-to-end, verification]

requires: ["24-01-signal-types-classifier", "24-02-signal-extractor", "24-03-signal-graph-integration"]
provides: ["verified-signal-pipeline", "integration-test-coverage"]
affects: ["25-baseline-storage", "26-observatory-api"]

tech-stack:
  added: []
  patterns: ["end-to-end-integration-testing", "mock-graph-client", "subtest-organization"]

key-files:
  created:
    - path: "internal/integration/grafana/signal_integration_test.go"
      lines: 543
      description: "End-to-end signal ingestion test with 10 test cases covering full pipeline"
  modified: []

decisions:
  - id: "integration-test-with-mocks"
    choice: "Use mockGraphClient instead of testcontainers for signal integration tests"
    rationale: "Follows existing test patterns in dashboard_syncer_test.go and graph_builder_test.go"
    impact: "Faster test execution, no container overhead, validates graph query structure"

  - id: "subtest-organization"
    choice: "Single TestSignalIngestionEndToEnd with 8 subtests, plus 2 separate test functions"
    rationale: "Group related pipeline tests together, isolate specific behavior tests"
    impact: "Clear test output hierarchy, easier to identify failure points"

metrics:
  duration: "11m"
  completed: "2026-01-29"
  tasks: 2
  commits: 1
  tests-added: 10
  lines-created: 543
---

# Phase 24 Plan 04: Signal Ingestion Integration Test Summary

**One-liner:** End-to-end signal ingestion pipeline verified through 10 integration test cases covering classification, quality scoring, graph persistence, TTL, and relationships

## What Was Built

### TestSignalIngestionEndToEnd Integration Test

Created comprehensive integration test covering signal extraction, classification, quality propagation, and graph persistence through the DashboardSyncer:

**Test Structure:**
- Single test function with 8 subtests using table-driven patterns
- Uses mockGraphClient following existing test conventions
- Validates full pipeline: GrafanaDashboard → syncDashboard → SignalAnchor nodes in graph

**8 Subtests Covering:**

1. **Known metrics Layer 1 classification (0.95 confidence)**
   - Tests: `kube_pod_status_phase` → Availability, `container_cpu_usage_seconds_total` → Saturation
   - Validates hardcoded metric classification with highest confidence
   - Verifies quality score propagation from dashboard (freshness-based)

2. **PromQL structure Layer 2 classification (0.9 confidence)**
   - Tests: `histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))` → Latency
   - Validates PromQL AST pattern detection (histogram_quantile function)
   - Extracts base metric name `http_request_duration_seconds` from bucket metric

3. **Quality score propagation from dashboard to signals**
   - Tests dashboard with alert rule → quality boost (+0.2)
   - Validates quality scoring factors: freshness (1.0 for recent), alerts (boost)
   - Verifies signals inherit computed quality score

4. **TTL expiration (7 days) via expires_at timestamp**
   - Creates expired signal with `expires_at` in past
   - Queries with `WHERE expires_at > $now` filter
   - Validates expired signals excluded from results

5. **Signal relationships (SOURCED_FROM, REPRESENTS)**
   - Verifies `(SignalAnchor)-[:SOURCED_FROM]->(Dashboard)` edge
   - Verifies `(SignalAnchor)-[:REPRESENTS]->(Metric)` edge
   - Validates relationship counts match signal counts

6. **Unlinked signals with empty workload fields**
   - Tests signals with no workload namespace/name
   - Validates empty workload strings don't cause errors
   - Verifies signal stored without MONITORS relationship

7. **Multi-query panel creating multiple signals**
   - Dashboard panel with 2 targets (multi-query)
   - Validates both signals extracted and stored
   - Verifies correct role classification for each metric

8. **Idempotency via MERGE upsert**
   - Syncs same dashboard twice
   - Validates signal updated (not duplicated)
   - Verifies ON MATCH preserves `first_seen`, updates other fields

### Additional Test Functions

**TestSignalIngestion_LowConfidenceFiltering:**
- Tests signals with confidence < 0.5 are excluded
- Validates Unknown role (confidence 0) not stored
- Verifies Layer 4 panel title classification (0.5 confidence) IS stored

**TestSignalIngestion_NamespaceOnlyInference:**
- Tests signals with namespace but no workload name
- Validates namespace-only inference (confidence 0.7)
- Verifies empty workload_name handled gracefully

## Task Breakdown

| Task | Description | Commit | Files | Duration |
|------|-------------|--------|-------|----------|
| 1 | Create end-to-end signal ingestion integration test | 836e0e2 | signal_integration_test.go | ~9m |
| 2 | Human verification checkpoint | APPROVED | - | ~2m |

Total execution time: 11 minutes

## Verification Results

**Automated Tests:**
- All 10 integration test cases passing
- 543 lines of test coverage
- Validates signal extraction, classification, quality scoring, graph persistence

**Human Verification (APPROVED):**
- Signal ingestion pipeline confirmed working end-to-end
- SignalAnchor nodes queryable with correct properties
- Signal relationships exist: SignalAnchor→Dashboard, SignalAnchor→Metric
- Classification produces expected roles with correct confidence
- Quality scores propagate from dashboard to signals
- TTL expiration works via expires_at query-time filtering
- Unlinked signals stored without errors

## Test Coverage Details

### End-to-End Pipeline Coverage

**Classification Layers Tested:**
- Layer 1 (0.95 confidence): Known metrics (kube_pod_status_phase, container_cpu_usage_seconds_total)
- Layer 2 (0.9 confidence): PromQL structure (histogram_quantile)
- Layer 4 (0.5 confidence): Panel title patterns (tested in low confidence filtering)
- Layer 5 (0 confidence): Unknown classification (filtered out)

**Quality Scoring Factors Tested:**
- Freshness: Recent dashboard (0 days old) → 1.0
- Freshness: Old dashboard (30 days old) → decay calculation
- Alert boost: Dashboard with 1 alert rule → +0.2 quality
- Base quality computation: avg(freshness, usage, ownership, completeness)

**Graph Operations Tested:**
- MERGE upsert with composite key (metric_name + namespace + workload + integration)
- ON CREATE: Sets all fields including first_seen
- ON MATCH: Updates fields, preserves first_seen
- Relationship creation: SOURCED_FROM, REPRESENTS
- Optional MONITORS relationship (only when workload exists)

**Edge Cases Tested:**
- Empty workload namespace and name
- Namespace-only signals (no workload name)
- Expired signals (past expires_at timestamp)
- Low confidence signals (<0.5 threshold)
- Multi-query panels (multiple targets per panel)
- Dashboard sync idempotency

### Mock Graph Client Validation

Integration tests validate query structure without running FalkorDB:
- MERGE queries with correct composite key
- Relationship queries with correct edge types
- WHERE clause filtering (expires_at, confidence threshold)
- OPTIONAL MATCH for conditional relationships

## Deviations from Plan

None - plan executed exactly as written.

## Implementation Notes

**Test Pattern Choice:**
- Follows existing test patterns in `dashboard_syncer_test.go` and `graph_builder_test.go`
- Uses `mockGraphClient` instead of testcontainers (as in `integration_lifecycle_test.go`)
- Faster test execution, no container startup overhead
- Validates query structure and graph operations logic

**Subtest Organization:**
- Main function `TestSignalIngestionEndToEnd` groups pipeline tests
- Separate functions for specific behaviors (low confidence, namespace-only)
- Table-driven approach for clarity and maintainability

**Dashboard Test Data:**
- Realistic dashboard structures with panels, targets, PromQL queries
- Varied freshness values (0 days, 30 days) for quality scoring
- Mix of Layer 1 and Layer 2 metrics for classification coverage

**Mock Query Validation:**
- Verifies correct MERGE query syntax
- Validates composite key fields in query
- Checks relationship query structure
- Confirms WHERE clause filtering logic

## Next Phase Readiness

**Phase 25 (Baseline Storage) Requirements:**
- ✅ Signal ingestion pipeline verified end-to-end
- ✅ SignalAnchor nodes persisted with correct properties
- ✅ Classification confidence levels validated
- ✅ Quality scores available for signal prioritization
- ✅ TTL mechanism tested and working
- ✅ Integration test coverage for regression prevention

**Phase 26 (Observatory API) Requirements:**
- ✅ Signal query patterns validated
- ✅ Relationship traversal tested (SignalAnchor→Dashboard, SignalAnchor→Metric)
- ✅ Workload filtering patterns verified
- ✅ Confidence-based signal filtering tested

**Blockers:**
- None - all Phase 25/26 requirements met

**Confidence Level:**
- High - 10 integration tests covering all major pipeline components
- Human verification confirmed end-to-end functionality
- Ready for baseline storage implementation

## Performance Characteristics

**Test Execution:**
- All 10 tests: <100ms (mock-based, no I/O)
- No container startup overhead
- Suitable for CI/CD pipeline

**Pipeline Validation:**
- 8 subtests cover common scenarios
- 2 separate tests cover edge cases
- Table-driven patterns enable easy expansion

**Coverage Gaps:**
- No testcontainers for real FalkorDB validation (intentional, follows existing patterns)
- Stats API stubs (alert count, views) return 0 (quality scoring partially limited)

## Success Criteria

All success criteria from plan met:

1. ✅ Integration test verifies signal ingestion from dashboard sync to graph
2. ✅ SignalAnchor nodes queryable with correct properties
3. ✅ Relationships exist: SignalAnchor→Dashboard, SignalAnchor→Metric
4. ✅ Classification produces expected roles with correct confidence
5. ✅ Quality scores propagate from dashboard to signals
6. ✅ TTL expiration works via expires_at filtering
7. ✅ Unlinked signals stored without errors
8. ✅ Human verification confirms graph queries work correctly

**All criteria verified through automated tests and human approval.**

## Commit History

| Commit | Description | Files | Tests |
|--------|-------------|-------|-------|
| `836e0e2` | test(24-04): add signal ingestion end-to-end integration test | signal_integration_test.go | +10 |

## Files Created

```
internal/integration/grafana/
└── signal_integration_test.go (+543 lines)
    ├── TestSignalIngestionEndToEnd (8 subtests)
    │   ├── Known metrics Layer 1 classification
    │   ├── PromQL structure Layer 2 classification
    │   ├── Quality score propagation
    │   ├── TTL expiration
    │   ├── Signal relationships
    │   ├── Unlinked signals
    │   ├── Multi-query panel
    │   └── Idempotency
    ├── TestSignalIngestion_LowConfidenceFiltering
    └── TestSignalIngestion_NamespaceOnlyInference
```

## Phase 24 Summary

With completion of Plan 04, Phase 24 (Data Model & Ingestion) is now complete:

**Phase 24 Accomplishments:**
- **Plan 01:** Signal types, layered classifier (5 layers), dashboard quality scorer (5 factors)
- **Plan 02:** Signal extractor, K8s workload linker, deduplication logic
- **Plan 03:** BuildSignalGraph method, DashboardSyncer integration, graph relationships
- **Plan 04:** End-to-end integration test, human verification, pipeline validation

**Phase 24 Deliverables:**
- ✅ SignalAnchor data model with 7 roles (Availability, Latency, Errors, Traffic, Saturation, Churn, Novelty)
- ✅ Classification engine with confidence decay (0.95 → 0.85-0.9 → 0.7-0.8 → 0.5 → 0)
- ✅ Dashboard quality scoring with alert boost (5 factors)
- ✅ Signal extraction from Grafana dashboards (PromQL parsing)
- ✅ K8s workload inference from PromQL labels (6 label priority)
- ✅ Graph persistence with MERGE upsert (composite key deduplication)
- ✅ Signal relationships: SOURCED_FROM, REPRESENTS, MONITORS
- ✅ TTL mechanism (7 days, query-time filtering)
- ✅ Integration test coverage (10 tests, 543 lines)

**Ready for Phase 25:** Baseline storage and anomaly detection
