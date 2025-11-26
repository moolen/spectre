# Implementation Plan: UI-API Integration

**Branch**: `005-ui-api-integration` | **Date**: 2025-11-26 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/005-ui-api-integration/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Replace mock data in the React UI with real API calls to the backend `/v1/search` endpoint. The implementation involves: (1) extending the Go backend API to return resource and event data in the format the UI expects, (2) updating React hooks to fetch from the real API instead of generating mock data, (3) removing all mock data code from the codebase, and (4) implementing proper error handling and loading states. This enables the Kubernetes event audit timeline UI to work with real backend data.

## Technical Context

**Backend Language/Version**: Go 1.21+ (existing in internal/storage, internal/api)
**Frontend Language/Version**: TypeScript 5.x with React 18+
**Primary Dependencies**:
  - Backend: Go standard library (net/http, encoding/json), existing QueryExecutor in internal/storage
  - Frontend: React, TypeScript, Vite, fetch API
**Storage**: Block-based storage backend (existing in internal/storage)
**Testing**:
  - Backend: Go testing package, integration tests
  - Frontend: Vitest (configured in vitest.config.ts)
**Target Platform**: Web application (browser-based UI communicating with HTTP API server)
**Project Type**: Web - separate frontend and backend
**Performance Goals**: API responds within 3 seconds for typical datasets; UI renders updates instantly
**Constraints**: Time range filters (start/end) required for all searches; graceful error handling for unavailable API
**Scale/Scope**: Support Kubernetes cluster audit data spanning multiple namespaces and resource kinds

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution file appears to be a template (not filled in). Proceeding with standard development practices:
- Tests will be included for API response handling and data transformation
- Integration tests will verify API contract compliance
- Code will maintain separation between UI and backend concerns
- Error handling will be comprehensive and user-friendly

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
# Backend API (Go)
internal/api/
├── server.go          # HTTP server (already exists)
├── search_handler.go  # /v1/search handler (needs extension)
├── response.go        # Response formatting (already exists)
├── errors.go          # Error handling (already exists)
└── validators.go      # Validation (already exists)

internal/models/
└── query.go           # QueryRequest/Response models (may need extension)

internal/storage/
└── query_executor.go  # Query execution (provides data)

# Frontend UI (TypeScript/React)
ui/src/
├── services/
│   ├── api.ts         # API client (exists, needs update)
│   ├── mockData.ts    # REMOVE this file
│   └── [other services]
├── hooks/
│   ├── useTimeline.ts # REFACTOR to use real API
│   └── [other hooks]
├── components/        # UI components (no changes needed)
├── types.ts           # Type definitions (may need extension)
└── App.tsx            # REFACTOR to remove mock data

ui/src/
└── [existing component structure - no changes]

tests/
├── integration/       # New integration tests for API contract
└── unit/             # New unit tests for data transformation
```

**Structure Decision**: Web application with separate Go backend and React frontend.
- Backend extensions are limited to the `/v1/search` endpoint and response data model
- Frontend changes are concentrated in: services/api.ts, hooks/useTimeline.ts, and App.tsx
- Mock data layer (mockData.ts) will be completely removed
- Existing component structure remains unchanged

## Complexity Tracking

No constitution violations. Implementation follows clean separation of concerns:
- Backend: Minimal changes - extend existing query executor and response formatting
- Frontend: Isolated to data-fetching layer (hooks, services) and mock removal
- No changes to UI components - they already expect K8sResource format

## Phase 0: Research & Unknowns Resolution

### Key Research Areas

1. **Backend Response Format**: Confirm how to structure `/v1/search` responses to include:
   - Resources with embedded statusSegments array
   - Resources with embedded events array
   - QueryResponse wrapper with metadata

2. **Data Transformation Patterns**: Research efficient conversion from backend format to K8sResource type

3. **Error Handling Strategy**: Define user-friendly error messages for common failures:
   - Network unavailability
   - API timeouts
   - Malformed responses
   - Empty result sets

4. **Time Range Defaults**: Determine reasonable default time ranges when no explicit range provided

**Decisions Made** (from specification):
- Use existing `/v1/search` endpoint (no new endpoints needed)
- Leverage existing QueryExecutor for data retrieval
- Use standard REST query parameters for filtering
- Implement timeouts in fetch requests (30s default already exists in api.ts)

## Phase 1: Design & API Contracts

### 1.1 Data Model Design

Backend response structure for `/v1/search`:

```go
type SearchResponse struct {
  Resources      []Resource     `json:"resources"`
  Count          int           `json:"count"`
  ExecutionTimeMs int64        `json:"executionTimeMs"`
}

type Resource struct {
  ID              string            `json:"id"`
  Group           string            `json:"group"`
  Version         string            `json:"version"`
  Kind            string            `json:"kind"`
  Namespace       string            `json:"namespace"`
  Name            string            `json:"name"`
  StatusSegments  []StatusSegment   `json:"statusSegments"`
  Events          []AuditEvent      `json:"events"`
}

type StatusSegment struct {
  StartTime  int64                  `json:"startTime"`
  EndTime    int64                  `json:"endTime"`
  Status     string                 `json:"status"` // Ready, Warning, Error, Terminating
  Message    string                 `json:"message,omitempty"`
  Config     map[string]interface{} `json:"config"`
}

type AuditEvent struct {
  ID        string `json:"id"`
  Timestamp int64  `json:"timestamp"`
  Verb      string `json:"verb"` // create, update, patch, delete, get, list
  User      string `json:"user"`
  Message   string `json:"message"`
  Details   string `json:"details,omitempty"`
}
```

Frontend will convert this to K8sResource (already defined in ui/src/types.ts).

### 1.2 API Contracts

API is divided into multiple endpoints for separation of concerns. Frontend will make targeted requests based on what data it needs.

#### Core Search Endpoint (Resource Discovery)

**Endpoint**: `GET /v1/search`

**Purpose**: Return list of resources matching filters and time range (minimal data for timeline view)

**Query Parameters**:
- `start` (required): Unix timestamp or human-readable date
- `end` (required): Unix timestamp or human-readable date
- `namespace` (optional): Filter by Kubernetes namespace
- `kind` (optional): Filter by resource kind
- `group` (optional): Filter by API group
- `version` (optional): Filter by API version

**Response**: SearchResponse with basic resource data
```json
{
  "resources": [
    {
      "id": "unique-resource-id",
      "group": "v1",
      "version": "Pod",
      "kind": "Pod",
      "namespace": "default",
      "name": "my-pod"
    }
  ],
  "count": 42,
  "executionTimeMs": 1234
}
```

**Error Responses**:
- 400: Invalid request (missing required params, invalid timestamps)
- 500: Internal server error (query execution failed)

---

#### Resource Detail Endpoint (Status Segments + Metadata)

**Endpoint**: `GET /v1/resources/{resourceId}`

**Purpose**: Return detailed resource with status segments and configuration

**Response**: Single Resource with statusSegments
```json
{
  "id": "unique-resource-id",
  "group": "v1",
  "version": "Pod",
  "kind": "Pod",
  "namespace": "default",
  "name": "my-pod",
  "statusSegments": [
    {
      "startTime": 1700000000,
      "endTime": 1700001000,
      "status": "Ready",
      "message": "Pod is running",
      "config": {"replicas": 1}
    }
  ]
}
```

**Error Responses**:
- 404: Resource not found
- 500: Internal server error

---

#### Resource Events Endpoint (Audit Trail)

**Endpoint**: `GET /v1/resources/{resourceId}/events`

**Purpose**: Return all audit events for a specific resource within optional time range

**Query Parameters**:
- `start` (optional): Unix timestamp
- `end` (optional): Unix timestamp
- `limit` (optional, default: 100): Maximum number of events to return

**Response**: Array of AuditEvents
```json
{
  "events": [
    {
      "id": "event-123",
      "timestamp": 1700000000,
      "verb": "create",
      "user": "system:admin",
      "message": "Pod created",
      "details": "{\"reason\": \"Triggered\", \"...\"}"
    }
  ],
  "count": 15,
  "resourceId": "unique-resource-id"
}
```

**Error Responses**:
- 404: Resource not found
- 400: Invalid query parameters
- 500: Internal server error

---

#### Status Segments Endpoint (Timeline Visualization)

**Endpoint**: `GET /v1/resources/{resourceId}/segments`

**Purpose**: Return status segments for a resource (for timeline visualization)

**Query Parameters**:
- `start` (optional): Unix timestamp
- `end` (optional): Unix timestamp

**Response**: Array of StatusSegments with config snapshots
```json
{
  "segments": [
    {
      "startTime": 1700000000,
      "endTime": 1700001000,
      "status": "Ready",
      "message": "Pod is running",
      "config": {"replicas": 1, "image": "nginx:latest"}
    }
  ],
  "resourceId": "unique-resource-id",
  "count": 5
}
```

**Error Responses**:
- 404: Resource not found
- 500: Internal server error

---

#### Metadata Endpoint (Filters + Aggregation)

**Endpoint**: `GET /v1/metadata`

**Purpose**: Return available namespaces, kinds, groups, and resource counts for filter UI

**Query Parameters**:
- `start` (optional): Unix timestamp (filter to events in this range)
- `end` (optional): Unix timestamp (filter to events in this range)

**Response**: Metadata for filter UI
```json
{
  "namespaces": ["default", "kube-system", "monitoring"],
  "kinds": ["Pod", "Service", "Deployment", "StatefulSet"],
  "groups": ["", "apps", "batch"],
  "resourceCounts": {
    "Pod": 42,
    "Service": 15,
    "Deployment": 8
  },
  "totalEvents": 5000,
  "timeRange": {
    "earliest": 1700000000,
    "latest": 1700086400
  }
}
```

**Error Responses**:
- 500: Internal server error

---

### Endpoint Usage Strategy

**Timeline View (Initial Load)**:
1. Call `GET /v1/search?start=X&end=Y` → Get resource list
2. For each resource, optionally call `GET /v1/resources/{id}/segments` → Get status segments
3. Display timeline immediately, load details on-demand

**Filter Bar**:
1. Call `GET /v1/metadata?start=X&end=Y` → Get available filters
2. User selects filter, triggers `GET /v1/search?start=X&end=Y&namespace=default`

**Detail Panel (On Resource Click)**:
1. Call `GET /v1/resources/{id}/events` → Get audit events
2. Display events in detail panel

**Incremental Loading**:
- `/v1/search` returns basic resource data quickly
- Status segments loaded on-demand per resource (`/v1/resources/{id}/segments`)
- Events loaded only when detail panel opens (`/v1/resources/{id}/events`)

### 1.3 Frontend Data Service

The existing `apiClient` in ui/src/services/api.ts will be extended with targeted methods for each endpoint:

```typescript
// Search endpoint - get initial resource list
async searchResources(
  startTime: string | number,
  endTime: string | number,
  filters?: {
    namespace?: string;
    kind?: string;
    group?: string;
    version?: string;
  }
): Promise<K8sResource[]>

// Metadata endpoint - get filter options
async getMetadata(
  startTime?: string | number,
  endTime?: string | number
): Promise<{
  namespaces: string[];
  kinds: string[];
  groups: string[];
  resourceCounts: Record<string, number>;
}>

// Resource detail - get status segments
async getResourceSegments(
  resourceId: string,
  startTime?: string | number,
  endTime?: string | number
): Promise<StatusSegment[]>

// Resource events - get audit trail
async getResourceEvents(
  resourceId: string,
  startTime?: string | number,
  endTime?: string | number,
  limit?: number
): Promise<K8sEvent[]>

// Data transformation helpers
private transformSearchResponse(response: SearchResponse): K8sResource[]
private transformStatusSegment(segment: ApiStatusSegment): StatusSegment
private transformEvent(event: ApiEvent): K8sEvent
```

### 1.4 Frontend Hook Implementation

`useTimeline` hook will:
1. Call `searchResources` with default time range (last 2 hours)
2. Handle loading state during fetch
3. Handle errors with user-friendly messages
4. Store resources in state
5. Support refresh functionality

**Quick Start**: See quickstart.md for end-to-end flow example.

## Phase 2: Implementation Tasks

*Detailed task list generated by /speckit.tasks*

The following phases will be executed:

### Backend Tasks
1. Extend QueryExecutor or create new search method to return formatted resources with events
2. Update search_handler.go to build SearchResponse with embedded resource data
3. Implement response transformation layer

### Frontend Tasks
1. Create new `searchResources` method in API service
2. Implement data transformer (backend → K8sResource)
3. Update useTimeline hook to use real API
4. Remove mockData.ts and all generateMockData imports
5. Update App.tsx to remove mock data initialization
6. Implement error boundaries and error display components
7. Add loading state UI feedback

### Testing Tasks
1. Integration tests for /v1/search endpoint
2. Unit tests for data transformation logic
3. Contract tests between frontend/backend
4. Error handling tests
