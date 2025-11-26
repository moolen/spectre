# Feature Specification: UI-API Integration

**Feature Branch**: `005-ui-api-integration`
**Created**: 2025-11-26
**Status**: Draft
**Input**: User description: "the ui in @ui/ should make use of the API provided in @internal/api/. please remove the mocks. extend the API if needed to match the request pattern the UI expects."

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

### User Story 1 - View Kubernetes Resources Timeline (Priority: P1)

Users need to view a timeline visualization of Kubernetes resources and their status changes over time. This is the core audit timeline feature that requires fetching real data from the backend.

**Why this priority**: This is the fundamental use case for the application. Without this, the UI cannot function. Users expect to see actual Kubernetes event data, not mock data.

**Independent Test**: Users can load the UI and immediately see a list of Kubernetes resources with their status timelines populated from the actual backend API, without any mocked data.

**Acceptance Scenarios**:

1. **Given** the user opens the application, **When** the page loads, **Then** the UI fetches resources from the `/v1/search` API endpoint and displays them in the timeline view
2. **Given** the user has the application loaded, **When** they interact with the timeline, **Then** the resource data refreshes from the backend and the display updates accurately
3. **Given** the user closes and reopens the application, **When** the page reloads, **Then** fresh data is fetched from the backend API

---

### User Story 2 - Filter Resources with Real Data (Priority: P2)

Users need to filter the timeline by namespace, resource kind, and search terms. These filters should work with actual backend data, not mocks.

**Why this priority**: Filtering is essential for users to find relevant resources in large datasets. This builds on the core timeline feature.

**Independent Test**: Users can apply filters (namespace, kind, search) and see the filtered results immediately reflect what the backend API returns based on those filter parameters.

**Acceptance Scenarios**:

1. **Given** the user selects a namespace filter, **When** they apply it, **Then** the API is called with the namespace parameter and only matching resources are displayed
2. **Given** the user enters a search term, **When** they submit the search, **Then** the backend is queried and results are filtered accordingly
3. **Given** the user combines multiple filters, **When** they apply them, **Then** the backend search query includes all filter parameters and correct results are returned

---

### User Story 3 - View Audit Events for Resources (Priority: P2)

Users need to see detailed audit events (Kubernetes API audit logs) associated with specific resources, including timestamps, user actions, and event details.

**Why this priority**: Audit events provide the detailed audit trail that users need to understand what happened to resources. This is important but depends on viewing resources first.

**Independent Test**: Users can select a resource and see detailed audit events fetched from the backend API for that resource.

**Acceptance Scenarios**:

1. **Given** the user has selected a resource in the timeline, **When** they view the detail panel, **Then** audit events for that resource are fetched from the backend
2. **Given** the user specifies a time range by selecting a segment, **When** the detail panel opens, **Then** events within that time range are displayed from the backend

---

### User Story 4 - Handle Real-Time Status Segments (Priority: P3)

The UI should correctly handle resource status segments (Ready, Warning, Error, Terminating) as they come from the real backend, maintaining proper state visualization.

**Why this priority**: Status visualization is important for understanding resource health, but the core functionality works without this specialized handling.

**Independent Test**: Status segments for resources are correctly fetched from the backend and displayed with appropriate visual indicators.

**Acceptance Scenarios**:

1. **Given** a resource has multiple status segments, **When** the resource is fetched from the backend, **Then** all status segments are properly parsed and displayed on the timeline
2. **Given** a status segment has configuration data, **When** the user views the segment details, **Then** the configuration is accurately displayed

### Edge Cases

- What happens when the API is unreachable or returns an error? The UI should display an appropriate error message to the user.
- What happens when the user applies filters but no resources match? The UI should show an empty state indicating no results.
- What happens when an API response takes longer than the configured timeout? The request should be cancelled and the user should see a timeout error message.
- What happens when the backend returns resources with unexpected field formats? The UI should gracefully handle missing or malformed data without crashing.

## Requirements *(mandatory)*

<!--
  ACTION REQUIRED: The content in this section represents placeholders.
  Fill them out with the right functional requirements.
-->

### Functional Requirements

- **FR-001**: The UI MUST fetch Kubernetes resources from the `/v1/search` API endpoint using query parameters for time range (start, end) and optional filters (namespace, kind, group, version)
- **FR-002**: The UI MUST parse the API response and convert backend resource/event data structures into the UI's internal `K8sResource` type
- **FR-003**: The UI MUST remove all mock data generation code and mock data imports from the codebase
- **FR-004**: The API MUST extend or provide response formats that include all required data for the UI: resources with their status segments, events, and metadata
- **FR-005**: The UI MUST handle API errors gracefully, displaying user-friendly error messages when requests fail
- **FR-006**: The UI MUST support filtering by namespace, kind, group, and version by passing these as query parameters to the `/v1/search` API
- **FR-007**: The UI MUST display loading states while fetching data from the API
- **FR-008**: The API response MUST include event data (audit events) for resources with timestamps, verbs (create, update, patch, delete, get, list), user information, and event details
- **FR-009**: The UI's `useTimeline` hook MUST fetch data from the real API instead of generating mock data
- **FR-010**: The API response structure MUST be compatible with the UI's expected data format (K8sResource with statusSegments and events arrays)

### Key Entities *(include if feature involves data)*

- **K8sResource**: Represents a Kubernetes resource with its audit history. Contains id, group, version, kind, namespace, name, status segments, and associated events.
- **ResourceStatusSegment**: Represents a period during which a resource maintained a specific status. Contains start time, end time, status value (Ready/Warning/Error/Terminating), optional message, and configuration snapshot.
- **K8sEvent**: Represents a Kubernetes audit event or API activity. Contains id, timestamp, verb (action type), user, message, and optional details.
- **QueryRequest**: Request parameters for the search API. Contains start timestamp, end timestamp, and optional filters (namespace, kind, group, version).
- **QueryResponse**: Response from the search API containing matched resources, event count, and execution metadata.

## Success Criteria *(mandatory)*

<!--
  ACTION REQUIRED: Define measurable success criteria.
  These must be technology-agnostic and measurable.
-->

### Measurable Outcomes

- **SC-001**: The UI successfully fetches and displays real Kubernetes resource data from the backend API instead of mock data, verified by loading the application and confirming API calls are made
- **SC-002**: All mock data generation code (mockData.ts and imports of generateMockData) is removed from the UI codebase
- **SC-003**: Users can filter resources using namespace, kind, and search parameters, and the API is called with the correct query parameters
- **SC-004**: The API responds with properly formatted resource and event data that matches the UI's expected data structure within 3 seconds for typical datasets
- **SC-005**: The UI handles API errors gracefully, displaying error messages instead of crashing when the API is unavailable or returns errors
- **SC-006**: No type errors or runtime errors occur when the UI processes real API responses
- **SC-007**: The application remains functional when the user refreshes the page, with data re-fetched from the API

## Assumptions

- The backend API at `/v1/search` will be extended or modified to return response data in a format compatible with UI expectations (resources with embedded events and status segments)
- The UI will use the existing `apiClient` service for making HTTP requests, which is already configured for the backend API
- The time range filter (start/end timestamps) is required for all API searches, and the UI will provide sensible defaults (e.g., last 2 hours of data)
- Status values in the API response will use the same enumeration as the UI expects (Ready, Warning, Error, Terminating, Unknown)
- The API backend has the capability to return event/audit data alongside resource data, or this capability will be added during this feature
