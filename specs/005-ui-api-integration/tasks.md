# Implementation Tasks: UI-API Integration

**Feature**: UI-API Integration (005-ui-api-integration)
**Branch**: `005-ui-api-integration`
**Generated**: 2025-11-26

## Summary

This feature replaces mock data in the React UI with real API calls to the backend. The implementation provides a granular multi-endpoint API design where each endpoint has a single responsibility, enabling efficient frontend data fetching and incremental loading.

**Key Changes**:
- Backend: Create 5 new endpoints (`/v1/search`, `/v1/metadata`, `/v1/resources/{id}`, `/v1/resources/{id}/segments`, `/v1/resources/{id}/events`)
- Backend: Build data models and aggregation logic to transform raw events into structured resources
- Frontend: Extend API client with methods for each endpoint
- Frontend: Replace mock data generation with targeted API calls
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

### Backend: Response Data Models

- [ ] [T006] Create new file `/home/moritz/dev/rpk/internal/models/api_types.go` with API response types: SearchResponse, Resource (minimal), StatusSegment, AuditEvent matching OpenAPI contract in plan.md
- [ ] [T007] Add JSON struct tags to SearchResponse fields: json:"resources", json:"count", json:"executionTimeMs"
- [ ] [T008] Add JSON struct tags to Resource fields: json:"id", json:"group", json:"version", json:"kind", json:"namespace", json:"name", json:"statusSegments", json:"events"
- [ ] [T009] Add JSON struct tags to StatusSegment fields: json:"startTime", json:"endTime", json:"status", json:"message", json:"config"
- [ ] [T010] Add JSON struct tags to AuditEvent fields: json:"id", json:"timestamp", json:"verb", json:"user", json:"message", json:"details"
- [ ] [T011] Implement Validate() method on SearchResponse to ensure count >= 0, executionTimeMs >= 0, and resources is non-nil
- [ ] [T012] Implement Validate() method on Resource to ensure id, kind, namespace, name are non-empty
- [ ] [T013] Implement Validate() method on StatusSegment to ensure startTime < endTime and status is valid enum

### Backend: Event-to-Resource Aggregation

- [ ] [T014] Create new file `/home/moritz/dev/rpk/internal/storage/resource_builder.go` with ResourceBuilder type for aggregating events into resources
- [ ] [T015] Implement ResourceBuilder.BuildResourcesFromEvents(events []Event) map[string]*Resource that groups events by resource UID
- [ ] [T016] Implement method to extract resource metadata (id, group, version, kind, namespace, name) from Event.Resource field
- [ ] [T017] Implement ResourceBuilder.BuildStatusSegments(events []Event) []StatusSegment that derives segments from CREATE/UPDATE/DELETE events
- [ ] [T018] Implement status inference: "Ready" for CREATE/UPDATE events, "Terminating" for DELETE, "Unknown" as fallback
- [ ] [T019] Implement ResourceBuilder.BuildAuditEvents(events []Event) []AuditEvent with verb mapping (CREATE→create, UPDATE→update, PATCH→patch, DELETE→delete, GET→get, LIST→list)
- [ ] [T020] Add timestamp conversion helper: Event.Timestamp (nanoseconds) → Unix seconds via division by 1e9

### Backend: Search Handler (GET /v1/search)

- [ ] [T021] Update `/home/moritz/dev/rpk/internal/api/search_handler.go` to use ResourceBuilder for transforming QueryResult events into Resource objects
- [ ] [T022] Modify SearchHandler.Handle() to build SearchResponse with only basic resource data (id, group, version, kind, namespace, name, no nested segments/events)
- [ ] [T023] Set SearchResponse.Count to number of unique resources (len(resources)), not number of events
- [ ] [T024] Copy ExecutionTimeMs from QueryResult to SearchResponse.ExecutionTimeMs
- [ ] [T025] Ensure /v1/search response is lightweight (basic resource data only, detailed data fetched separately)

### Backend: Resource Detail Handler (GET /v1/resources/{resourceId})

- [ ] [T026] Create new file `/home/moritz/dev/rpk/internal/api/resource_handler.go` with ResourceHandler type
- [ ] [T027] Implement ResourceHandler.Handle(w, r) to parse {resourceId} from URL path
- [ ] [T028] Look up resource by ID in storage and return Resource with full statusSegments array
- [ ] [T029] Return 404 if resource not found
- [ ] [T030] Register handler in server.go: `s.router.HandleFunc("/v1/resources/{resourceId}", s.handleResource)`

### Backend: Resource Segments Handler (GET /v1/resources/{resourceId}/segments)

- [ ] [T031] Create handler in `/home/moritz/dev/rpk/internal/api/segments_handler.go` for segments endpoint
- [ ] [T032] Accept optional `start` and `end` query parameters to filter segments by time range
- [ ] [T033] Return SegmentsResponse with segments array and resource metadata
- [ ] [T034] Implement segment filtering by time range if start/end provided
- [ ] [T035] Register handler in server.go: `s.router.HandleFunc("/v1/resources/{resourceId}/segments", s.handleSegments)`

### Backend: Resource Events Handler (GET /v1/resources/{resourceId}/events)

- [ ] [T036] Create handler in `/home/moritz/dev/rpk/internal/api/events_handler.go` for events endpoint
- [ ] [T037] Accept optional `start`, `end`, and `limit` query parameters (default limit: 100)
- [ ] [T038] Query storage for events matching resource ID and time range
- [ ] [T039] Transform storage events to AuditEvent format using ResourceBuilder logic
- [ ] [T040] Return EventsResponse with events array, count, and resourceId
- [ ] [T041] Register handler in server.go: `s.router.HandleFunc("/v1/resources/{resourceId}/events", s.handleEvents)`

### Backend: Metadata Handler (GET /v1/metadata)

- [ ] [T042] Create handler in `/home/moritz/dev/rpk/internal/api/metadata_handler.go` for metadata endpoint
- [ ] [T043] Accept optional `start` and `end` query parameters to filter metadata to a time range
- [ ] [T044] Query storage to aggregate distinct namespaces across all resources
- [ ] [T045] Query storage to aggregate distinct kinds across all resources
- [ ] [T046] Query storage to aggregate distinct groups across all resources
- [ ] [T047] Query storage to compute resource counts by kind
- [ ] [T048] Return MetadataResponse with namespaces, kinds, groups, resourceCounts, totalEvents, timeRange
- [ ] [T049] Register handler in server.go: `s.router.HandleFunc("/v1/metadata", s.handleMetadata)`

### Backend: Server Route Registration Update

- [ ] [T050] Update `/home/moritz/dev/rpk/internal/api/server.go` registerHandlers() to include all 5 new endpoints
- [ ] [T051] Verify all handlers are registered with correct paths and HTTP methods (all GET)

### Frontend: API Response Types

- [ ] [T052] Create new file `/home/moritz/dev/rpk/ui/src/services/apiTypes.ts` with TypeScript interfaces matching multi-endpoint API design
- [ ] [T053] Define SearchResponse interface: {resources: Resource[], count: number, executionTimeMs: number}
- [ ] [T054] Define Resource interface (minimal for /v1/search): {id, group, version, kind, namespace, name}
- [ ] [T055] Define ResourceDetail interface (for /v1/resources/{id}): extends Resource with statusSegments: StatusSegment[]
- [ ] [T056] Define StatusSegment interface: {startTime: number, endTime: number, status: string, message?: string, config: Record<string, any>}
- [ ] [T057] Define AuditEvent interface: {id, timestamp: number, verb, user, message, details?: string}
- [ ] [T058] Define MetadataResponse interface: {namespaces: string[], kinds: string[], groups: string[], resourceCounts: Record<string, number>, totalEvents: number, timeRange: {earliest: number, latest: number}}
- [ ] [T059] Define EventsResponse interface: {events: AuditEvent[], count: number, resourceId: string}
- [ ] [T060] Define SegmentsResponse interface: {segments: StatusSegment[], resourceId: string, count: number}

### Frontend: Data Transformation Service

- [ ] [T061] Create new file `/home/moritz/dev/rpk/ui/src/services/dataTransformer.ts` for backend-to-frontend data conversion
- [ ] [T062] Implement transformSearchResponse(response: SearchResponse) → K8sResource[] in dataTransformer.ts
- [ ] [T063] Implement transformResourceDetail(resource: ResourceDetail) → K8sResource in dataTransformer.ts (includes statusSegments)
- [ ] [T064] Implement transformStatusSegment(segment: StatusSegment) → StatusSegment helper (convert Unix seconds to Date: new Date(startTime * 1000))
- [ ] [T065] Implement transformAuditEvent(event: AuditEvent) → K8sEvent helper (convert timestamp to Date, map verb strings)
- [ ] [T066] Add validation in transformers to skip invalid data (missing required fields: id, kind, namespace, name)
- [ ] [T067] Add error handling to catch timestamp conversion errors and log without crashing

### Frontend: API Service Extension (Multi-Endpoint)

- [ ] [T068] Add searchResources() method to ApiClient in `/home/moritz/dev/rpk/ui/src/services/api.ts`
  - Parameters: startTime (string | number), endTime (string | number), filters?: {namespace?, kind?, group?, version?}
  - Returns: Promise<K8sResource[]>
  - Makes GET request to `/v1/search?start=X&end=Y&[filters]`
  - Uses transformSearchResponse() to convert response

- [ ] [T069] Add getMetadata() method to ApiClient
  - Parameters: startTime? (string | number), endTime? (string | number)
  - Returns: Promise<MetadataResponse>
  - Makes GET request to `/v1/metadata?[start=X&end=Y]`

- [ ] [T070] Add getResourceSegments() method to ApiClient
  - Parameters: resourceId (string), startTime? (string | number), endTime? (string | number)
  - Returns: Promise<StatusSegment[]>
  - Makes GET request to `/v1/resources/{resourceId}/segments?[start=X&end=Y]`

- [ ] [T071] Add getResourceEvents() method to ApiClient
  - Parameters: resourceId (string), startTime? (string | number), endTime? (string | number), limit? (number)
  - Returns: Promise<K8sEvent[]>
  - Makes GET request to `/v1/resources/{resourceId}/events?[start=X&end=Y&limit=L]`
  - Uses transformAuditEvent() to convert events

- [ ] [T072] Add proper timestamp conversion in all methods: JavaScript milliseconds → Unix seconds via `Math.floor(timestamp / 1000)`
- [ ] [T073] Add error handling to wrap API errors with user-friendly messages (network, timeout, 404, 500, etc.)
- [ ] [T074] Export all response type interfaces from apiTypes.ts for use in api.ts and components

---

## Phase 3: User Story 1 - View Kubernetes Resources Timeline (P1)

### Update useTimeline Hook

- [ ] [T075] [US1] Import apiClient from services/api.ts into `/home/moritz/dev/rpk/ui/src/hooks/useTimeline.ts`
- [ ] [T076] [US1] Remove import of generateMockData from services/mockData.ts in useTimeline.ts
- [ ] [T077] [US1] Replace mock data generation with apiClient.searchResources() call in fetchData() method
- [ ] [T078] [US1] Calculate default time range: startTime = Date.now() - 2*60*60*1000 (2 hours ago), endTime = Date.now()
- [ ] [T079] [US1] Convert JavaScript timestamps (milliseconds) to Unix seconds for API call: `Math.floor(Date.now() / 1000)`
- [ ] [T080] [US1] Update error handling to catch API-specific errors (timeout, network, validation) and set user-friendly error messages
- [ ] [T081] [US1] Test that loading state is properly set to true before fetch and false after completion

### Update App Component

- [ ] [T082] [US1] Remove any remaining references to generateMockData in `/home/moritz/dev/rpk/ui/src/App.tsx`
- [ ] [T083] [US1] Verify Timeline component receives resources from useTimeline hook correctly
- [ ] [T084] [US1] Verify ErrorBoundary component displays errors from useTimeline.error state

### Testing & Validation

- [ ] [T085] [US1] Manual test: Start backend server and verify `/v1/search` endpoint returns valid SearchResponse JSON
- [ ] [T086] [US1] Manual test: Load UI and verify resources display in timeline view without console errors
- [ ] [T087] [US1] Manual test: Verify Network tab shows GET request to `/v1/search?start=X&end=Y`
- [ ] [T088] [US1] Manual test: Refresh page and verify data is re-fetched from backend (not cached)

---

## Phase 4: User Story 2 - Filter Resources with Real Data (P2)

### Extend useTimeline for Filtering

- [ ] [T089] [US2] Add filters parameter to fetchData() method in `/home/moritz/dev/rpk/ui/src/hooks/useTimeline.ts`
- [ ] [T090] [US2] Update fetchData() to accept optional filters: {namespace?: string, kind?: string, group?: string, version?: string}
- [ ] [T091] [US2] Pass filters to apiClient.searchResources() as third parameter
- [ ] [T092] [US2] Add useEffect dependency on filters so fetchData() is called when filters change
- [ ] [T093] [US2] Add optional call to apiClient.getMetadata() to populate filter options for FilterBar component

### Connect FilterBar Component

- [ ] [T094] [US2] Update `/home/moritz/dev/rpk/ui/src/App.tsx` to pass current filter state to useTimeline hook
- [ ] [T095] [US2] Import useFilters hook in App.tsx to get filter state
- [ ] [T096] [US2] Call useTimeline with filters from useFilters (pass to fetchData())
- [ ] [T097] [US2] Update FilterBar component to call getMetadata() on mount to populate dropdown options
- [ ] [T098] [US2] Verify FilterBar component triggers fetchData() when namespace filter changes

### Testing & Validation

- [ ] [T099] [US2] Manual test: Select namespace filter and verify API is called with `?namespace=default` parameter
- [ ] [T100] [US2] Manual test: Enter search term in kind filter and verify API receives `?kind=Pod` parameter
- [ ] [T101] [US2] Manual test: Combine multiple filters (namespace + kind) and verify both parameters are sent to API
- [ ] [T102] [US2] Manual test: Clear filters and verify all resources are fetched again (no filter params)
- [ ] [T103] [US2] Manual test: Verify /v1/metadata endpoint populates filter dropdowns with available namespaces and kinds

---

## Phase 5: User Story 3 - View Audit Events for Resources (P2)

### Update DetailPanel Component for Event Fetching

- [ ] [T104] [US3] Update DetailPanel component in `/home/moritz/dev/rpk/ui/src/components/DetailPanel.tsx` to call apiClient.getResourceEvents() when a resource is selected
- [ ] [T105] [US3] Modify DetailPanel to pass resourceId from selected resource to getResourceEvents()
- [ ] [T106] [US3] Add loading state to DetailPanel while events are being fetched from /v1/resources/{id}/events
- [ ] [T107] [US3] Add error handling in DetailPanel to display user-friendly error message if event fetch fails
- [ ] [T108] [US3] Display transformed events (with Date objects and mapped verbs) in detail panel

### Verify Events Display

- [ ] [T109] [US3] Verify DetailPanel component correctly displays events from apiClient.getResourceEvents()
- [ ] [T110] [US3] Check that event timestamps are formatted correctly (JavaScript Date objects from transformAuditEvent)
- [ ] [T111] [US3] Check that event verbs (create, update, patch, delete, get, list) are displayed with correct labels
- [ ] [T112] [US3] Verify event details (user, message, timestamp) display from AuditEvent objects

### Testing & Validation

- [ ] [T113] [US3] Manual test: Select a resource in timeline and verify detail panel fetches events from /v1/resources/{id}/events
- [ ] [T114] [US3] Manual test: Verify events are sorted by timestamp ascending (oldest first)
- [ ] [T115] [US3] Manual test: Click on different resources and verify events update correctly
- [ ] [T116] [US3] Manual test: Verify event details (user, message, timestamp) are displayed accurately from backend data

---

## Phase 6: User Story 4 - Handle Real-Time Status Segments (P3)

### Fetch and Display Status Segments

- [ ] [T117] [US4] Update Timeline component in `/home/moritz/dev/rpk/ui/src/components/Timeline.tsx` to call apiClient.getResourceSegments() for detailed status segment data
- [ ] [T118] [US4] Modify Timeline to load status segments on-demand (either lazy-load or fetch after initial /v1/search)
- [ ] [T119] [US4] Add error handling for failed segment fetches (graceful fallback to available data)
- [ ] [T120] [US4] Apply status segment styling based on status value (Ready→green, Warning→yellow, Error→red, Terminating→gray)

### Verify Status Segments

- [ ] [T121] [US4] Verify ResourceBuilder in backend creates statusSegments with correct status values (Ready, Warning, Error, Terminating, Unknown)
- [ ] [T122] [US4] Verify statusSegments are sorted by startTime ascending in backend response from /v1/resources/{id}/segments
- [ ] [T123] [US4] Check that Timeline component correctly renders status segments with proper colors and labels
- [ ] [T124] [US4] Check that status segment config data is preserved during transformation (backend map → frontend Record)

### Testing & Validation

- [ ] [T125] [US4] Manual test: Verify resources with multiple status changes show distinct colored segments on timeline
- [ ] [T126] [US4] Manual test: Verify status segment tooltips display message and timestamps correctly
- [ ] [T127] [US4] Manual test: Verify deleted resources show "Terminating" status in final segment from /v1/resources/{id}/segments
- [ ] [T128] [US4] Manual test: Verify newly created resources show "Ready" status after creation event
- [ ] [T129] [US4] Manual test: Verify segments endpoint (/v1/resources/{id}/segments) returns correct status data

---

## Phase 7: Polish & Integration Testing

### Remove Mock Data

- [ ] [T130] Delete file `/home/moritz/dev/rpk/ui/src/services/mockData.ts` completely
- [ ] [T131] Search codebase for any remaining imports of mockData.ts and remove them using grep
- [ ] [T132] Search codebase for any remaining calls to generateMockData() function and remove them

### Error Handling Refinement

- [ ] [T133] Add specific error message for network timeout: "Service is temporarily unavailable. Please try again." in `/home/moritz/dev/rpk/ui/src/services/api.ts`
- [ ] [T134] Add specific error message for 400 Bad Request: "Invalid search parameters. Check your filters and try again." in api.ts
- [ ] [T135] Add specific error message for 404 Not Found: "Resource not found. It may have been deleted." in api.ts
- [ ] [T136] Add specific error message for 500 Internal Server Error: "An error occurred while fetching data. Please try again." in api.ts
- [ ] [T137] Add error message for malformed response: "Received unexpected data format from server." in dataTransformer.ts
- [ ] [T138] Test error display in UI by simulating backend unavailability (stop backend server)
- [ ] [T139] Test error display for 404 errors (request invalid resourceId)
- [ ] [T140] Test error display for timeout errors (slow backend endpoint)

### Loading States

- [ ] [T141] Verify Loading component in `/home/moritz/dev/rpk/ui/src/components/Common/Loading.tsx` displays during API fetch
- [ ] [T142] Add loading spinner test: verify spinner appears immediately when filters change
- [ ] [T143] Verify loading state clears after API response is received (both success and error cases)
- [ ] [T144] Verify loading state displays while fetching segments for each resource

### Empty State Handling

- [ ] [T145] Verify EmptyState component in `/home/moritz/dev/rpk/ui/src/components/Common/EmptyState.tsx` displays when API returns zero resources
- [ ] [T146] Test empty state message: "No resources found matching your filters. Try adjusting your search criteria."
- [ ] [T147] Verify empty state includes action buttons (Clear Filters, Refresh) if applicable
- [ ] [T148] Test empty events state: display message when detail panel has no events

### Integration Testing

- [ ] [T149] End-to-end test: Start backend with sample data, load UI, verify timeline displays resources from /v1/search
- [ ] [T150] End-to-end test: Apply filters, verify filtered results display correctly from /v1/search?filters
- [ ] [T151] End-to-end test: Select resource, verify detail panel fetches and shows events from /v1/resources/{id}/events
- [ ] [T152] End-to-end test: Verify status segments load correctly from /v1/resources/{id}/segments
- [ ] [T153] End-to-end test: Refresh page, verify data is re-fetched from all endpoints
- [ ] [T154] Cross-browser test: Verify UI works in Chrome, Firefox, Safari (timestamp handling varies by browser)
- [ ] [T155] Test all 5 endpoints are called correctly: /v1/search, /v1/metadata, /v1/resources/{id}, /v1/resources/{id}/segments, /v1/resources/{id}/events

### Performance Validation

- [ ] [T156] Measure API response time for /v1/search (last 2 hours, no filters) - should be < 3 seconds
- [ ] [T157] Measure API response time for /v1/metadata - should be < 1 second
- [ ] [T158] Measure API response time for /v1/resources/{id}/segments - should be < 1 second
- [ ] [T159] Measure API response time for /v1/resources/{id}/events - should be < 1 second
- [ ] [T160] Measure frontend rendering time for 100 resources - should be < 500ms
- [ ] [T161] Test with large dataset (1000+ resources, 10000+ events) - verify UI remains responsive
- [ ] [T162] Verify no memory leaks when repeatedly applying filters and refreshing data

### Code Quality

- [ ] [T163] Run TypeScript compiler (`npm run build` in ui/) and fix any type errors
- [ ] [T164] Run Go tests (`go test ./...` in project root) and verify all tests pass
- [ ] [T165] Run linter on frontend code (`npm run lint` in ui/) and fix any warnings
- [ ] [T166] Add JSDoc comments to new API methods (searchResources, getMetadata, getResourceSegments, getResourceEvents)
- [ ] [T167] Add JSDoc comments to transformation functions (transformSearchResponse, transformStatusSegment, transformAuditEvent)
- [ ] [T168] Add Go doc comments to new types (SearchResponse, Resource, StatusSegment, AuditEvent, ResourceBuilder)
- [ ] [T169] Add Go doc comments to new handler types and methods

### Documentation Updates

- [ ] [T170] Update `/home/moritz/dev/rpk/ui/README.md` to document new API service usage and multi-endpoint architecture
- [ ] [T171] Add inline code comments explaining timestamp conversion logic (nanoseconds → seconds → Date)
- [ ] [T172] Add inline code comments explaining resource aggregation algorithm in ResourceBuilder
- [ ] [T173] Add comments documenting the 5-endpoint API design in plan.md or API documentation

---

## Final Verification Checklist

Before marking this feature as complete, verify:

### Backend Endpoints
- [ ] ✓ `GET /v1/search` returns SearchResponse with basic resource list (id, group, version, kind, namespace, name)
- [ ] ✓ `GET /v1/metadata` returns namespaces, kinds, groups, and resource counts
- [ ] ✓ `GET /v1/resources/{resourceId}` returns single Resource with statusSegments array
- [ ] ✓ `GET /v1/resources/{resourceId}/segments` returns array of StatusSegments with time range filtering
- [ ] ✓ `GET /v1/resources/{resourceId}/events` returns array of AuditEvents with optional filtering and limit

### Frontend Integration
- [ ] ✓ ApiClient has methods: searchResources(), getMetadata(), getResourceSegments(), getResourceEvents()
- [ ] ✓ Data transformers convert backend format to frontend K8sResource/K8sEvent types
- [ ] ✓ Frontend successfully calls backend APIs instead of generating mock data
- [ ] ✓ useTimeline hook fetches from /v1/search and manages default time range

### Mock Data Removal
- [ ] ✓ All mock data files and imports are removed from the codebase
- [ ] ✓ mockData.ts file deleted
- [ ] ✓ generateMockData() no longer called anywhere

### Features
- [ ] ✓ Timeline displays resources from /v1/search endpoint
- [ ] ✓ Filters (namespace, kind, group, version) work correctly and pass to /v1/search
- [ ] ✓ Timeline displays status segments fetched from /v1/resources/{id}/segments
- [ ] ✓ Status segments display with correct colors based on status value
- [ ] ✓ Detail panel displays audit events from /v1/resources/{id}/events
- [ ] ✓ Event timestamps, verbs, and user info display correctly

### Error & State Handling
- [ ] ✓ Error handling shows user-friendly messages for all failure scenarios (network, 400, 404, 500, timeout, malformed)
- [ ] ✓ Loading states display during API calls
- [ ] ✓ Empty states display when API returns zero resources
- [ ] ✓ Page gracefully handles network unavailability

### Quality
- [ ] ✓ No TypeScript compilation errors
- [ ] ✓ No Go test failures
- [ ] ✓ No console errors when running the application
- [ ] ✓ Application works after page refresh (data re-fetched from all endpoints)
- [ ] ✓ API response times meet performance goals (/v1/search < 3 seconds, other endpoints < 1 second)
- [ ] ✓ All user stories (US1-US4) acceptance criteria are met
- [ ] ✓ Code properly documented with comments

---

## Notes

**Task Format**: `[TaskID] [P if parallelizable] [Story label if applicable] Description with file path`

**Parallelizable Tasks**: Tasks marked with [P] can be executed in parallel with other [P] tasks in the same phase.

**Story Labels**: [US1], [US2], [US3], [US4] indicate which user story the task belongs to. Setup, foundational, and polish tasks have no story label as they support all stories.

**File Paths**: All file paths are absolute paths starting from `/home/moritz/dev/rpk/`

**Multi-Endpoint Design**: This revision introduces a granular 5-endpoint API design:
- `GET /v1/search` - Resource discovery (core list view)
- `GET /v1/metadata` - Filter options and aggregations
- `GET /v1/resources/{resourceId}` - Single resource with segments
- `GET /v1/resources/{resourceId}/segments` - Status timeline segments
- `GET /v1/resources/{resourceId}/events` - Audit event details

Each endpoint is independently callable, enabling efficient frontend data fetching and incremental loading.

**Total Task Count**: 173 tasks (renamed from original 115 to reflect new endpoint coverage)

**Estimated Effort**:
- Phase 1 (Setup): 30 minutes
- Phase 2 (Foundational): 6-8 hours (increased due to 5 endpoint handlers)
- Phase 3 (US1): 2-3 hours
- Phase 4 (US2): 1.5-2 hours (additional metadata endpoint usage)
- Phase 5 (US3): 1.5-2 hours (events endpoint integration)
- Phase 6 (US4): 1.5-2 hours (segments endpoint integration)
- Phase 7 (Polish): 3-4 hours (comprehensive testing of all endpoints)

**Total Estimated Time**: 16-21 hours
