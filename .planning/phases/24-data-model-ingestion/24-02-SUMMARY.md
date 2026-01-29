---
phase: 24-data-model-ingestion
plan: 02
subsystem: observatory
tags: [grafana, signals, prometheus, kubernetes, promql, classification]

# Dependency graph
requires:
  - phase: 24-01
    provides: SignalAnchor types, 5-layer classifier, quality scorer
provides:
  - Signal extraction from Grafana panels to SignalAnchor instances
  - K8s workload inference from PromQL label selectors with priority
  - Deduplication by composite key (metric + namespace + workload)
affects: [24-03, 25, 26]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Panel-to-signal extraction with multi-query support"
    - "Workload inference with label priority (deployment > app.kubernetes.io/name > app > service > job > pod)"
    - "Namespace-only signals for unlinked metrics"
    - "Dashboard-level deduplication with quality-based winner selection"

key-files:
  created:
    - internal/integration/grafana/signal_extractor.go
    - internal/integration/grafana/signal_extractor_test.go
    - internal/integration/grafana/workload_linker.go
    - internal/integration/grafana/workload_linker_test.go
  modified: []

key-decisions:
  - "Namespace-only inference for signals with namespace but no workload labels (confidence 0.7)"
  - "Low-confidence threshold (< 0.5) filters out unclassifiable metrics"
  - "Composite key for deduplication: metric_name|namespace|workload_name"
  - "Highest quality signal wins on duplicates, preserving FirstSeen timestamp"
  - "7-day TTL via expires_at = last_seen + 7 days"

patterns-established:
  - "Signal extraction handles multi-query panels (golden signals dashboards)"
  - "Graceful degradation: skip unparseable queries without failing entire panel"
  - "Workload linker returns nil only for completely unlinked signals (no labels at all)"
  - "Integration between extractor, classifier, and linker via function composition"

# Metrics
duration: 4min
completed: 2026-01-29
---

# Phase 24 Plan 02: Signal Extraction & Workload Linkage Summary

**Panel-to-signal extraction with 5-layer classification, K8s workload inference via label priority, and dashboard-level deduplication by composite key**

## Performance

- **Duration:** 4 minutes
- **Started:** 2026-01-29T21:26:17Z
- **Completed:** 2026-01-29T21:30:26Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Signal extractor transforms Grafana panel queries into SignalAnchor instances with role classification
- Workload linker infers K8s namespace and workload from PromQL label selectors using priority order
- Dashboard-level deduplication by composite key with quality-based winner selection
- Comprehensive test coverage (20 test cases across extractor and linker)

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement signal extractor with multi-role support** - `1babed5` (feat)
2. **Task 2: Implement K8s workload linker with label priority** - `48eee9c` (feat)

## Files Created/Modified

- `internal/integration/grafana/signal_extractor.go` - Panel-to-signal transformation with classification and deduplication
- `internal/integration/grafana/signal_extractor_test.go` - 13 test cases covering single/multi-query, deduplication, quality inheritance
- `internal/integration/grafana/workload_linker.go` - K8s workload inference from PromQL labels with priority
- `internal/integration/grafana/workload_linker_test.go` - 11 test cases covering label priority, namespace inference, edge cases

## Decisions Made

**Namespace-only signal inference**
- Workload linker returns WorkloadInference with empty workload name when namespace exists but no workload labels
- Confidence 0.7 for namespace-only inference
- Enables tracking namespace-scoped metrics even without workload linkage

**Low-confidence filtering threshold**
- Signals with confidence < 0.5 are filtered out during extraction
- Prevents Unknown-role signals (confidence 0) from polluting graph
- Layer 4 (panel title) classifications at 0.5 are included as minimum viable

**Composite key deduplication strategy**
- Key format: `metric_name|namespace|workload_name`
- Handles same metric across multiple panels in dashboard
- Highest quality signal wins, preserving FirstSeen timestamp from earliest occurrence
- LastSeen updated on every dashboard sync

**Label priority hierarchy**
- deployment (0.9) > app.kubernetes.io/name (0.85) > app (0.7) > service (0.75) > job (0.8) > pod (0.6)
- Reflects K8s naming conventions and reliability of inference
- Confidence boosted to 0.9 when namespace present

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - implementation proceeded smoothly with all tests passing on first verification.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for Phase 24-03 (Graph Integration)**
- Signal extraction complete with full test coverage
- Workload inference ready for linking to ResourceIdentity nodes
- Deduplication logic ensures clean signal graph
- Awaits GraphBuilder integration to create SignalAnchor nodes and edges

**No blockers**
- All 24 test cases passing
- Integration points clearly defined (ClassifyMetric, InferWorkloadFromLabels)
- TTL calculation follows v1.4 pattern (7-day expires_at)

---
*Phase: 24-data-model-ingestion*
*Completed: 2026-01-29*
