# Implementation Tasks: UI-API Integration

**Feature**: UI-API Integration (005-ui-api-integration)
**Branch**: `005-ui-api-integration`
**Generated**: 2025-11-26

## Summary

This feature replaces mock data in the React UI with real API calls to the backend `/v1/search` endpoint. The implementation connects the existing UI components to the backend storage layer through a newly designed SearchResponse API contract.

**Key Changes**:
- Backend: Extend `/v1/search` to return structured resources with status segments and events
- Frontend: Replace mock data generation with API service calls
- Remove: All mock data files and imports from the UI codebase

## Dependencies

User stories have the following completion order:

```
Phase 1 (Setup)
    ↓
Phase 2 (Foundational) ← Required by ALL user stories
    ↓
    ├─→ Phase 3 (US1: View Timeline) ← P1: MUST complete first
    ├─→ Phase 4 (US2: Filter Resources) ← P2: Depends on US1
    ├─→ Phase 5 (US3: View Audit Events) ← P2: Depends on US1
    └─→ Phase 6 (US4: Handle Status Segments) ← P3: Depends on US1
    ↓
Phase 7 (Polish & Testing)
```

**Critical Path**: Phase 1 → Phase 2 → Phase 3 (US1) → Phase 7

---

## Phase 1: Setup

### Environment & Configuration

- [ ] [T001] [P] Verify Go backend dependencies are installed (go-dateparser, existing internal packages) in `/home/moritz/dev/rpk/go.mod`
- [ ] [T002] [P] Verify frontend dependencies are installed (React, TypeScript, Vite) in `/home/moritz/dev/rpk/ui/package.json`
- [ ] [T003] Create environment configuration file `/home/moritz/dev/rpk/ui/.env` with `VITE_API_BASE=/v1` if not exists
- [ ] [T004] [P] Verify backend server starts successfully and `/healthz` endpoint responds at `http://localhost:8080/healthz`
- [ ] [T005] [P] Verify frontend dev server starts successfully at `http://localhost:5173`

---

## Phase 2: Foundational Infrastructure

### Backend: SearchResponse Data Models

- [ ] [T006] Create new file `/home/moritz/dev/rpk/internal/models/search_response.go` with SearchResponse, Resource, StatusSegment, and AuditEvent types matching the OpenAPI contract
- [ ] [T007] Add JSON struct tags to all SearchResponse model fields (json:"resources", json:"count", json:"executionTimeMs")
- [ ] [T008] Implement Validate() method on SearchResponse type to ensure count matches len(resources) and executionTimeMs >= 0
- [ ] [T009] Implement Validate() method on Resource type to ensure required fields (id, kind, namespace, name) are non-empty
- [ ] [T010] Implement Validate() method on StatusSegment type to ensure startTime < endTime and status is valid enum value
- [ ] [T011] Implement Validate() method on AuditEvent type to ensure required fields (id, timestamp, verb, user, message) are non-empty

### Backend: Event-to-Resource Aggregation

- [ ] [T012] Create new file `/home/moritz/dev/rpk/internal/storage/resource_builder.go` with ResourceBuilder type
- [ ] [T013] Implement ResourceBuilder.BuildResourcesFromEvents() method that groups events by resource UID in `/home/moritz/dev/rpk/internal/storage/resource_builder.go`
- [ ] [T014] Implement ResourceBuilder.CreateResource() method that extracts metadata (group, version, kind, namespace, name) from Event.Resource field
- [ ] [T015] Implement ResourceBuilder.BuildStatusSegments() method that derives status segments from event timeline (CREATE→Ready, DELETE→Terminating)
- [ ] [T016] Implement ResourceBuilder.BuildAuditEvents() method that converts storage Event objects to AuditEvent objects with verb mapping (CREATE→create, UPDATE→update, DELETE→delete)
- [ ] [T017] Add timestamp conversion logic: Event.Timestamp (nanoseconds) → AuditEvent.Timestamp (seconds) via `timestamp / 1e9`
- [ ] [T018] Add status inference logic: use "Ready" for resources with last event = CREATE/UPDATE, "Terminating" for DELETE events, "Unknown" otherwise

### Backend: SearchResponse Builder

- [ ] [T019] Create new file `/home/moritz/dev/rpk/internal/api/search_response_builder.go` with SearchResponseBuilder type
- [ ] [T020] Implement SearchResponseBuilder.BuildFromQueryResult() method that takes QueryResult and returns SearchResponse in `/home/moritz/dev/rpk/internal/api/search_response_builder.go`
- [ ] [T021] Integrate ResourceBuilder into SearchResponseBuilder to transform []Event into []Resource
- [ ] [T022] Set SearchResponse.Count to number of unique resources (not number of events)
- [ ] [T023] Set SearchResponse.ExecutionTimeMs from QueryResult.ExecutionTimeMs (already in milliseconds)
- [ ] [T024] Add deduplication logic to ensure each resource appears only once in the response (group by UID)

### Backend: Update Search Handler

- [ ] [T025] Update `/home/moritz/dev/rpk/internal/api/search_handler.go` Handle() method to use SearchResponseBuilder instead of returning QueryResult directly
- [ ] [T026] Import models.SearchResponse and api.SearchResponseBuilder in search_handler.go
- [ ] [T027] Replace `writeJSON(w, result)` with `writeJSON(w, searchResponse)` where searchResponse is built from QueryResult

### Frontend: API Response Types

- [ ] [T028] Create new file `/home/moritz/dev/rpk/ui/src/services/apiTypes.ts` with TypeScript interfaces matching OpenAPI contract
- [ ] [T029] Define SearchResponse interface with resources: Resource[], count: number, executionTimeMs: number in apiTypes.ts
- [ ] [T030] Define Resource interface with id, group, version, kind, namespace, name, statusSegments, events in apiTypes.ts
- [ ] [T031] Define StatusSegment interface with startTime: number, endTime: number, status: string, message?: string, config: Record<string, any> in apiTypes.ts
- [ ] [T032] Define AuditEvent interface with id, timestamp: number, verb, user, message, details? in apiTypes.ts

### Frontend: Data Transformation Service

- [ ] [T033] Create new file `/home/moritz/dev/rpk/ui/src/services/dataTransformer.ts` for backend-to-frontend data conversion
- [ ] [T034] Implement transformSearchResponse() function that converts SearchResponse (backend) to K8sResource[] (frontend) in dataTransformer.ts
- [ ] [T035] Implement transformStatusSegment() helper that converts Unix seconds to JavaScript Date objects: `new Date(startTime * 1000)` in dataTransformer.ts
- [ ] [T036] Implement transformAuditEvent() helper that converts Unix seconds to JavaScript Date objects and maps verbs (create, update, delete) in dataTransformer.ts
- [ ] [T037] Add validation in transformSearchResponse() to skip resources with missing required fields (id, kind, namespace, name)
- [ ] [T038] Add error handling in transformSearchResponse() to catch and log timestamp conversion errors without crashing

### Frontend: API Service Extension

- [ ] [T039] Add searchResources() method to ApiClient class in `/home/moritz/dev/rpk/ui/src/services/api.ts`
- [ ] [T040] Implement searchResources() with parameters: startTime (string | number), endTime (string | number), filters?: {namespace, kind, group, version}
- [ ] [T041] Build query string in searchResources() using URLSearchParams with start, end, and optional filter parameters
- [ ] [T042] Make GET request to `/v1/search` endpoint with constructed query parameters
- [ ] [T043] Import and use transformSearchResponse() to convert API response to K8sResource[] before returning
- [ ] [T044] Add error handling in searchResources() to wrap API errors with user-friendly messages
- [ ] [T045] Export SearchResponse type from apiTypes.ts for use in api.ts

---

## Phase 3: User Story 1 - View Kubernetes Resources Timeline (P1)

### Update useTimeline Hook

- [ ] [T046] [US1] Import apiClient and searchResources from services/api.ts into `/home/moritz/dev/rpk/ui/src/hooks/useTimeline.ts`
- [ ] [T047] [US1] Remove import of generateMockData from services/mockData.ts in useTimeline.ts
- [ ] [T048] [US1] Replace mock data generation with apiClient.searchResources() call in fetchData() method
- [ ] [T049] [US1] Calculate default time range: startTime = Date.now() - 2*60*60*1000 (2 hours ago), endTime = Date.now()
- [ ] [T050] [US1] Convert JavaScript timestamps (milliseconds) to Unix seconds for API call: `Math.floor(Date.now() / 1000)`
- [ ] [T051] [US1] Update error handling to catch API-specific errors (timeout, network, validation) and set user-friendly error messages
- [ ] [T052] [US1] Test that loading state is properly set to true before fetch and false after completion

### Update App Component

- [ ] [T053] [US1] Remove any remaining references to generateMockData in `/home/moritz/dev/rpk/ui/src/App.tsx`
- [ ] [T054] [US1] Verify Timeline component receives resources from useTimeline hook correctly
- [ ] [T055] [US1] Verify ErrorBoundary component displays errors from useTimeline.error state

### Testing & Validation

- [ ] [T056] [US1] Manual test: Start backend server and verify `/v1/search` endpoint returns valid SearchResponse JSON
- [ ] [T057] [US1] Manual test: Load UI and verify resources display in timeline view without console errors
- [ ] [T058] [US1] Manual test: Verify Network tab shows GET request to `/v1/search?start=X&end=Y`
- [ ] [T059] [US1] Manual test: Refresh page and verify data is re-fetched from backend (not cached)

---

## Phase 4: User Story 2 - Filter Resources with Real Data (P2)

### Extend useTimeline for Filtering

- [ ] [T060] [US2] Add filters parameter to fetchData() method in `/home/moritz/dev/rpk/ui/src/hooks/useTimeline.ts`
- [ ] [T061] [US2] Update fetchData() to accept optional filters: {namespace?: string, kind?: string, group?: string, version?: string}
- [ ] [T062] [US2] Pass filters to apiClient.searchResources() as third parameter
- [ ] [T063] [US2] Add useEffect dependency on filters so fetchData() is called when filters change

### Connect FilterBar Component

- [ ] [T064] [US2] Update `/home/moritz/dev/rpk/ui/src/App.tsx` to pass current filter state to useTimeline hook
- [ ] [T065] [US2] Import useFilters hook in App.tsx to get filter state
- [ ] [T066] [US2] Call useTimeline with filters from useFilters: `useTimeline({filters: filterState})`
- [ ] [T067] [US2] Verify FilterBar component triggers fetchData() when namespace filter changes

### Testing & Validation

- [ ] [T068] [US2] Manual test: Select namespace filter and verify API is called with `?namespace=default` parameter
- [ ] [T069] [US2] Manual test: Enter search term in kind filter and verify API receives `?kind=Pod` parameter
- [ ] [T070] [US2] Manual test: Combine multiple filters (namespace + kind) and verify both parameters are sent to API
- [ ] [T071] [US2] Manual test: Clear filters and verify all resources are fetched again (no filter params)

---

## Phase 5: User Story 3 - View Audit Events for Resources (P2)

### Verify Events Display

- [ ] [T072] [US3] Verify Resource objects returned from backend include non-empty events array in SearchResponse
- [ ] [T073] [US3] Verify DetailPanel component in `/home/moritz/dev/rpk/ui/src/components/DetailPanel.tsx` correctly displays events from selected resource
- [ ] [T074] [US3] Check that event timestamps are formatted correctly (JavaScript Date objects from transformed data)
- [ ] [T075] [US3] Check that event verbs (create, update, delete) are displayed with correct labels

### Testing & Validation

- [ ] [T076] [US3] Manual test: Select a resource in timeline and verify detail panel shows audit events
- [ ] [T077] [US3] Manual test: Verify events are sorted by timestamp ascending (oldest first)
- [ ] [T078] [US3] Manual test: Click on different resources and verify events update correctly
- [ ] [T079] [US3] Manual test: Verify event details (user, message, timestamp) are displayed accurately

---

## Phase 6: User Story 4 - Handle Real-Time Status Segments (P3)

### Verify Status Segments

- [ ] [T080] [US4] Verify ResourceBuilder in backend creates statusSegments with correct status values (Ready, Warning, Error, Terminating, Unknown)
- [ ] [T081] [US4] Verify statusSegments are sorted by startTime ascending in backend response
- [ ] [T082] [US4] Check that Timeline component in `/home/moritz/dev/rpk/ui/src/components/Timeline.tsx` renders status segments with correct colors
- [ ] [T083] [US4] Check that status segment config data is preserved during transformation (backend map → frontend Record)

### Testing & Validation

- [ ] [T084] [US4] Manual test: Verify resources with multiple status changes show distinct colored segments on timeline
- [ ] [T085] [US4] Manual test: Verify status segment tooltips display message and timestamps correctly
- [ ] [T086] [US4] Manual test: Verify deleted resources show "Terminating" status in final segment
- [ ] [T087] [US4] Manual test: Verify newly created resources show "Ready" status after creation event

---

## Phase 7: Polish & Integration Testing

### Remove Mock Data

- [ ] [T088] Delete file `/home/moritz/dev/rpk/ui/src/services/mockData.ts` completely
- [ ] [T089] Search codebase for any remaining imports of mockData.ts and remove them using grep
- [ ] [T090] Search codebase for any remaining calls to generateMockData() function and remove them

### Error Handling Refinement

- [ ] [T091] Add specific error message for network timeout: "Service is temporarily unavailable. Please try again." in `/home/moritz/dev/rpk/ui/src/services/api.ts`
- [ ] [T092] Add specific error message for 400 Bad Request: "Invalid search parameters. Check your filters and try again." in api.ts
- [ ] [T093] Add specific error message for 500 Internal Server Error: "An error occurred while fetching data. Please try again." in api.ts
- [ ] [T094] Add error message for malformed response: "Received unexpected data format from server." in dataTransformer.ts
- [ ] [T095] Test error display in UI by simulating backend unavailability (stop backend server)

### Loading States

- [ ] [T096] Verify Loading component in `/home/moritz/dev/rpk/ui/src/components/Common/Loading.tsx` displays during API fetch
- [ ] [T097] Add loading spinner test: verify spinner appears immediately when filters change
- [ ] [T098] Verify loading state clears after API response is received (both success and error cases)

### Empty State Handling

- [ ] [T099] Verify EmptyState component in `/home/moritz/dev/rpk/ui/src/components/Common/EmptyState.tsx` displays when API returns zero resources
- [ ] [T100] Test empty state message: "No resources found matching your filters. Try adjusting your search criteria."
- [ ] [T101] Verify empty state includes action buttons (Clear Filters, Refresh) if applicable

### Integration Testing

- [ ] [T102] End-to-end test: Start backend with sample data, load UI, verify timeline displays resources
- [ ] [T103] End-to-end test: Apply filters, verify filtered results display correctly
- [ ] [T104] End-to-end test: Select resource, verify detail panel shows events and status segments
- [ ] [T105] End-to-end test: Refresh page, verify data is re-fetched and display updates
- [ ] [T106] Cross-browser test: Verify UI works in Chrome, Firefox, Safari (timestamp handling varies by browser)

### Performance Validation

- [ ] [T107] Measure API response time for typical query (last 2 hours, no filters) - should be < 3 seconds
- [ ] [T108] Measure frontend rendering time for 100 resources - should be < 500ms
- [ ] [T109] Test with large dataset (1000+ resources, 10000+ events) - verify UI remains responsive
- [ ] [T110] Verify no memory leaks when repeatedly applying filters and refreshing data

### Code Quality

- [ ] [T111] Run TypeScript compiler (`npm run build` in ui/) and fix any type errors
- [ ] [T112] Run Go tests (`go test ./...` in project root) and verify all tests pass
- [ ] [T113] Run linter on frontend code (`npm run lint` in ui/) and fix any warnings
- [ ] [T114] Add JSDoc comments to new functions in api.ts and dataTransformer.ts
- [ ] [T115] Add Go doc comments to new types in search_response.go and resource_builder.go

### Documentation Updates

- [ ] [T116] Update `/home/moritz/dev/rpk/ui/README.md` to document new API service usage (if README exists)
- [ ] [T117] Add inline code comments explaining timestamp conversion logic (nanoseconds → seconds → Date)
- [ ] [T118] Add inline code comments explaining resource aggregation algorithm in ResourceBuilder

---

## Final Verification Checklist

Before marking this feature as complete, verify:

- [ ] ✓ Backend `/v1/search` endpoint returns SearchResponse with resources, statusSegments, and events
- [ ] ✓ Frontend successfully calls backend API instead of generating mock data
- [ ] ✓ All mock data files and imports are removed from the codebase
- [ ] ✓ Filters (namespace, kind) work correctly and pass parameters to API
- [ ] ✓ Timeline displays resources with correct status segments and colors
- [ ] ✓ Detail panel displays audit events with timestamps, verbs, and users
- [ ] ✓ Error handling shows user-friendly messages for all failure scenarios
- [ ] ✓ Loading states display during API calls
- [ ] ✓ Empty states display when no resources match filters
- [ ] ✓ No TypeScript compilation errors
- [ ] ✓ No Go test failures
- [ ] ✓ No console errors when running the application
- [ ] ✓ Application works after page refresh (data re-fetched from API)
- [ ] ✓ API response times meet performance goals (< 3 seconds typical)
- [ ] ✓ All user stories (US1-US4) acceptance criteria are met

---

## Notes

**Task Format**: `[TaskID] [P if parallelizable] [Story label if applicable] Description with file path`

**Parallelizable Tasks**: Tasks marked with [P] can be executed in parallel with other [P] tasks in the same phase.

**Story Labels**: [US1], [US2], [US3], [US4] indicate which user story the task belongs to. Setup, foundational, and polish tasks have no story label as they support all stories.

**File Paths**: All file paths are absolute paths starting from `/home/moritz/dev/rpk/`

**Estimated Effort**:
- Phase 1 (Setup): 30 minutes
- Phase 2 (Foundational): 4-6 hours
- Phase 3 (US1): 2-3 hours
- Phase 4 (US2): 1-2 hours
- Phase 5 (US3): 1 hour
- Phase 6 (US4): 1 hour
- Phase 7 (Polish): 2-3 hours

**Total Estimated Time**: 12-17 hours
