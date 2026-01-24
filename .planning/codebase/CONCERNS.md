# Codebase Concerns

**Analysis Date:** 2026-01-20

## Tech Debt

**Storage Package Removal - Incomplete Migration:**
- Issue: Storage package removed but migration to graph-based implementation incomplete
- Files: `internal/importexport/json_import_test.go:322-332`, `tests/e2e/demo_mode_test.go:8`, `chart/values.yaml:201`
- Impact: Multiple tests skipped, demo mode removed, persistence configuration deprecated but still present in Helm chart
- Fix approach: Complete graph-based import implementation to replace storage-backed functionality, remove deprecated configuration from chart

**Search Handler ResourceBuilder Missing:**
- Issue: ResourceBuilder functionality not yet reimplemented for graph-based queries
- Files: `internal/api/handlers/search_handler.go:58`
- Impact: Simplified resource building from events instead of proper graph traversal; may lose resource metadata richness
- Fix approach: Implement proper ResourceBuilder that queries graph for complete resource state and metadata

**Mock Data in UI:**
- Issue: UI implementation summary notes mock data still in use for development
- Files: `ui/IMPLEMENTATION_SUMMARY.md:277-279`
- Impact: Indicates frontend development may not be fully tested against real backend
- Fix approach: Remove mock data, ensure all UI components tested against live API

**Documentation Placeholders:**
- Issue: Multiple documentation pages are TODO stubs with no content
- Files: `docs/docs/operations/troubleshooting.md:3`, `docs/docs/operations/performance-tuning.md:3`, `docs/docs/operations/deployment.md:3`, `docs/docs/operations/backup-recovery.md:3`, `docs/docs/installation/local-development.md:9`, `docs/docs/operations/monitoring.md:3`, `docs/docs/operations/storage-management.md:3`, `docs/docs/development/contributing.md:3`, `docs/docs/development/building.md:3`, `docs/docs/development/release-process.md:3`, `docs/docs/development/development-setup.md:3`, `docs/docs/development/code-structure.md:3`
- Impact: Incomplete documentation prevents users from self-service troubleshooting and operations
- Fix approach: Migrate content from source files (docs/OPERATIONS.md) to individual pages, remove TODO markers

**Deprecated Import/Export API:**
- Issue: Old import/export API functions marked deprecated but still in codebase
- Files: `internal/importexport/MIGRATION_GUIDE.md:18-272`, `internal/importexport/REFACTORING_SUMMARY.md:144-331`
- Impact: Increased maintenance burden, potential confusion for developers
- Fix approach: Remove deprecated functions after confirming all callers migrated to new API

## Known Bugs

**Empty Catch Block:**
- Issue: Silent exception swallowing in RootCauseView component
- Files: `ui/src/components/RootCauseView.tsx:1337`
- Impact: Errors suppressed without logging, makes debugging difficult
- Trigger: Unknown - no context for what error is being caught
- Fix approach: Add error logging or handle error appropriately

## Security Considerations

**Environment Files in Repository:**
- Risk: `.env` and `.env.local` files exist but are gitignored; risk of accidental secret commits
- Files: `.gitignore:35-37`, `ui/.env`, `ui/.env.local`, `.auto-claude/.env`
- Current mitigation: Files properly gitignored
- Recommendations: Add pre-commit hooks to prevent .env file commits; document required environment variables in README without actual secrets

**No API Authentication Patterns Detected:**
- Risk: No visible authentication/authorization middleware in API handlers
- Files: `internal/api/handlers/search_handler.go`
- Current mitigation: May be handled at ingress/proxy level
- Recommendations: Document authentication architecture; add handler-level auth if missing

## Performance Bottlenecks

**Large Frontend Components:**
- Problem: Several components exceed 700+ lines, indicating complexity
- Files: `ui/src/components/RootCauseView.tsx:1719`, `ui/src/components/Timeline.tsx:953`, `ui/src/components/NamespaceGraph/NamespaceGraph.tsx:754`
- Cause: Monolithic components combining layout logic, rendering, and state management
- Improvement path: Extract sub-components, separate layout algorithms into pure functions, use composition

**Complex Graph Layout Algorithms:**
- Problem: Custom orthogonal routing with A* pathfinding may be CPU-intensive
- Files: `ui/src/utils/rootCauseLayout/route.ts:493`, `ui/src/utils/rootCauseLayout/place.ts:479`, `ui/src/utils/rootCauseLayout/force.ts:282`
- Cause: Real-time graph visualization with obstacle avoidance
- Improvement path: Consider Web Workers for layout computation, memoize layout results, add progressive rendering for large graphs

**Timeline Pagination Complexity:**
- Problem: Custom streaming/batching implementation with abort controllers and timeouts
- Files: `ui/src/hooks/useTimeline.ts:56-112`
- Cause: Large resource datasets requiring incremental loading
- Improvement path: Already optimized with viewport culling per IMPLEMENTATION_SUMMARY.md; monitor memory usage with 100K+ resources

**Generated Protobuf Files:**
- Problem: Large generated files may slow build/development
- Files: `ui/src/generated/timeline.ts:1432`, `ui/src/generated/internal/api/proto/timeline.ts:1250`
- Cause: Code generation from proto definitions
- Improvement path: Exclude from linting, use code splitting to lazy-load if not immediately needed

## Fragile Areas

**RootCauseView Component:**
- Files: `ui/src/components/RootCauseView.tsx`
- Why fragile: 1719 lines, complex D3 manipulation, graph layout coordination, multiple state sources
- Safe modification: Extract smaller components (SignificanceBadge already extracted), test D3 interactions separately, add integration tests
- Test coverage: No test file detected (`ui/src/components/RootCauseView.test.tsx` does not exist)

**Timeline Component:**
- Files: `ui/src/components/Timeline.tsx:953`
- Why fragile: Direct D3 DOM manipulation, zoom/pan coordination, event handling
- Safe modification: Change only in isolated feature branches, test zoom/pan interactions manually
- Test coverage: No test file detected

**Graph Import/Export System:**
- Files: `internal/importexport/json_import.go`, `internal/importexport/enrichment/`
- Why fragile: Multiple skipped tests indicate incomplete migration from storage to graph
- Safe modification: Ensure graph connection available, test with small datasets first
- Test coverage: Many tests skipped (`t.Skip`) in `json_import_test.go`

## Scaling Limits

**Metadata Cache Refresh:**
- Current capacity: 30-second refresh interval (configurable)
- Limit: With very large clusters (1000+ namespaces/kinds), metadata queries may become expensive
- Scaling path: Increase refresh interval, implement incremental cache updates, add memory-based cache layer

**Timeline Query Performance:**
- Current capacity: Optimized for ~500 resources per IMPLEMENTATION_SUMMARY.md
- Limit: UI targets <3s initial load for 500 resources; performance degrades with 100K+ resources
- Scaling path: Virtual scrolling already mentioned as future optimization, server-side aggregation for large time ranges

**FalkorDB Graph Database:**
- Current capacity: Unknown - performance benchmarks skipped in short mode
- Limit: Graph query performance depends on relationship density
- Scaling path: Monitor query execution times in `internal/graph/timeline_benchmark_test.go`, add indexes for common query patterns

## Dependencies at Risk

**ESLint Config Array Deprecated:**
- Risk: `@eslint/eslintrc` package shows deprecation warning
- Files: `ui/package-lock.json:1126`
- Impact: Future ESLint versions may break linting
- Migration plan: Migrate to flat config (`eslint.config.js`) per ESLint 9+ standards

**React 19 and Playwright Compatibility:**
- Risk: Using React 19.2.0 (very recent) with Playwright experimental CT
- Files: `ui/package.json:26-33`
- Impact: Experimental features may have undiscovered issues
- Migration plan: Monitor Playwright CT stability, pin versions to avoid breaking changes

**Dagre Layout Library:**
- Risk: Dagre library (0.8.5) last updated several years ago
- Files: `ui/package.json:22`, `ui/src/utils/graphLayout.ts:6`
- Impact: May lack modern React/TypeScript support, potential security issues
- Migration plan: Evaluate alternatives (react-flow, elkjs) for graph layout

## Missing Critical Features

**No Component-Level Error Boundaries:**
- Problem: Only app-level ErrorBoundary detected
- Files: `ui/src/components/Common/ErrorBoundary.tsx` (referenced in IMPLEMENTATION_SUMMARY.md but not verified in large components)
- Blocks: Graceful degradation when individual widgets fail

**No Backend Health Monitoring:**
- Problem: `/api/health` endpoint exists but no visible alerting/monitoring integration
- Files: API client references health endpoint per IMPLEMENTATION_SUMMARY.md
- Blocks: Proactive detection of backend failures

**No User Authentication System:**
- Problem: No authentication layer visible in frontend or backend handlers
- Files: No auth middleware detected in `internal/api/handlers/`
- Blocks: Multi-user deployments, audit trails

## Test Coverage Gaps

**UI Components:**
- What's not tested: Large visualization components (RootCauseView, Timeline, NamespaceGraph)
- Files: No `*.test.tsx` files found for: `ui/src/components/RootCauseView.tsx`, `ui/src/components/Timeline.tsx`, `ui/src/components/NamespaceGraph/NamespaceGraph.tsx`
- Risk: Visual regressions, interaction bugs in critical user-facing features
- Priority: High - these are primary user interaction surfaces

**Import/Export Graph Migration:**
- What's not tested: Graph-based import functionality
- Files: `internal/importexport/json_import_test.go:326-332` (multiple skipped tests)
- Risk: Data import failures, data loss during migration
- Priority: High - critical for data persistence

**E2E Tests Conditional:**
- What's not tested: Many e2e tests only run in long mode (`if testing.Short() { t.Skip() }`)
- Files: `tests/e2e/flux_helmrelease_integration_test.go:21`, `tests/e2e/root_cause_endpoint_flux_test.go:88`, `tests/e2e/default_resources_test.go:9`, `tests/e2e/mcp_stdio_test.go:9`, `tests/e2e/import_export_test.go:16-128`, `tests/e2e/config_reload_test.go:9`, `tests/e2e/mcp_failure_scenarios_test.go` (multiple)
- Risk: Integration failures only discovered in CI, not during local development
- Priority: Medium - CI should catch these, but slows development feedback

**Frontend Test Infrastructure Underutilized:**
- What's not tested: Vitest and Playwright CT configured but only 5 test files detected
- Files: Only `ui/src/utils/timeParsing.test.ts`, `ui/src/components/FilterBar.test.tsx`, `ui/src/components/TimeRangeDropdown.test.tsx`, and Playwright layout tests
- Risk: Regression bugs in filtering, state management, API integration
- Priority: Medium - infrastructure ready, needs test authoring

**Generated Code Type Safety:**
- What's not tested: Generated protobuf code uses `any` types extensively
- Files: `ui/src/generated/timeline.ts:255-1296`, `ui/src/generated/internal/api/proto/timeline.ts:193-1170`
- Risk: Type errors not caught at compile time in proto message handling
- Priority: Low - generated code, but could add runtime validation tests

---

*Concerns audit: 2026-01-20*
