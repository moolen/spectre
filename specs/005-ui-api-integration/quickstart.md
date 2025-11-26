# Quick Start: UI-API Integration

**Phase**: 1 - Design & Contracts
**Date**: 2025-11-26

This document provides a quick walkthrough of the integration from end-to-end.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     React UI Browser                         │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │  App.tsx                                                │ │
│  │  - useTimeline hook                                     │ │
│  │  - Timeline, FilterBar, DetailPanel components        │ │
│  └─────────────────────────────────────────────────────────┘ │
│                              ↓                                │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │  services/api.ts (apiClient)                           │ │
│  │  - searchResources(startTime, endTime, filters)        │ │
│  │  - transformSearchResponse(backendData) → K8sResource[]│ │
│  └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              ↓ HTTP GET
                  /v1/search?start=X&end=Y
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                        Go Backend HTTP                        │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │  api/server.go                                          │ │
│  │  - registerHandlers()                                   │ │
│  │  - handleSearch() → SearchHandler.Handle()             │ │
│  └─────────────────────────────────────────────────────────┘ │
│                              ↓                                │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │  api/search_handler.go                                  │ │
│  │  - parseQuery(request)                                  │ │
│  │  - queryExecutor.Execute(query)                        │ │
│  │  - buildSearchResponse(results) → SearchResponse JSON │ │
│  └─────────────────────────────────────────────────────────┘ │
│                              ↓                                │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │  storage/query_executor.go                              │ │
│  │  - Query block storage for resources & events          │ │
│  │  - Return structured results                           │ │
│  └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## User Scenario: View Timeline

### Step-by-Step Flow

**Step 1: User Opens Application**
```
Browser loads http://localhost:3000
↓
React App mounts
↓
useTimeline hook initializes in App.tsx
```

**Step 2: Fetch Data**
```
useTimeline.fetchData() executes
↓
Calls: apiClient.searchResources(
  startTime: Date.now() - 2*60*60*1000,  // 2 hours ago
  endTime: Date.now(),
  filters: {} // No filters initially
)
↓
Shows loading state to user
```

**Step 3: API Request**
```
apiClient builds query:
  start = 1700000000
  end   = 1700007200
  (no filter params)
↓
Sends HTTP GET:
  /v1/search?start=1700000000&end=1700007200
↓
Backend receives and processes
```

**Step 4: Backend Processing**
```
search_handler.parseQuery()
  ↓ Validates timestamps
  ↓ Creates QueryRequest with filters
  ↓
queryExecutor.Execute(query)
  ↓ Queries block storage
  ↓ Returns resources with events
  ↓
buildSearchResponse()
  ↓ Wraps results in SearchResponse
  ↓ Includes metadata (count, executionTime)
  ↓
Returns JSON response with SearchResponse
```

**Step 5: Frontend Transform**
```
apiClient receives response as JSON
  ↓
transformSearchResponse(jsonData)
  ↓ For each resource in response:
    ↓ Convert timestamps (Unix seconds → JavaScript Date)
    ↓ Validate required fields
    ↓ Create K8sResource object
  ↓ Return K8sResource[]
```

**Step 6: State Update**
```
useTimeline sets state:
  - resources = transformedData
  - loading = false
  - error = null
  ↓
React re-renders with new data
```

**Step 7: Display Timeline**
```
App.tsx renders:
  - FilterBar (allows changing filters)
  - Timeline (displays resources visually)
  - DetailPanel (shows selected resource details)
  ↓
User sees Kubernetes resources with status timelines
```

## User Scenario: Apply Filters

### Filtering Flow

**Step 1: User Selects Namespace Filter**
```
FilterBar component
  ↓ User clicks "default" namespace
  ↓ Component calls setFilters({namespace: "default"})
```

**Step 2: Filter Change Triggers Fetch**
```
useFilters hook detects filter change
  ↓
useTimeline.fetchData() called with filters
  ↓
Calls: apiClient.searchResources(
  startTime: <same as before>,
  endTime: <same as before>,
  filters: {namespace: "default"}
)
```

**Step 3: API Request with Filters**
```
apiClient builds query:
  start = 1700000000
  end   = 1700007200
  namespace = default
  ↓
Sends: /v1/search?start=1700000000&end=1700007200&namespace=default
```

**Step 4: Backend Filtering**
```
search_handler.parseQuery()
  ↓ Extracts namespace=default from query
  ↓
queryExecutor.Execute(query)
  ↓ Filters storage results by namespace
  ↓ Returns only resources in "default" namespace
```

**Step 5: Frontend Update**
```
Transform response
  ↓
Update state with filtered resources
  ↓
Re-render timeline with filtered data
  ↓
User sees only resources in "default" namespace
```

## Error Scenario: API Timeout

### Error Handling Flow

**Step 1: Request Sent**
```
apiClient.searchResources() called
↓
Timeout set to 30 seconds
```

**Step 2: Timeout Exceeded**
```
No response after 30 seconds
↓
AbortController cancels request
↓
Fetch throws error
```

**Step 3: Error Caught**
```
useTimeline catch block:
  ↓ Detects timeout error
  ↓ Sets error state with message:
    "Service is temporarily unavailable. Please try again."
  ↓ Sets loading = false
```

**Step 4: Error Display**
```
App.tsx renders ErrorBoundary
↓
Shows error message to user
↓
User can:
  - Click "Retry" button
  - Adjust filters and try again
  - Check service status
```

## Data Transformation Example

### Backend Response → Frontend Format

**Backend Response (JSON)**:
```json
{
  "resources": [
    {
      "id": "pod-12345",
      "group": "core",
      "version": "v1",
      "kind": "Pod",
      "namespace": "default",
      "name": "my-app-xyz",
      "statusSegments": [
        {
          "startTime": 1700000000,
          "endTime": 1700003600,
          "status": "Ready",
          "message": "Pod is running",
          "config": {"image": "my-app:1.0.0"}
        }
      ],
      "events": [
        {
          "id": "evt-001",
          "timestamp": 1700000100,
          "verb": "create",
          "user": "kubelet",
          "message": "Pod created",
          "details": null
        }
      ]
    }
  ],
  "count": 1,
  "executionTimeMs": 245
}
```

**Transform Process**:
```typescript
function transformSearchResponse(response: any): K8sResource[] {
  return response.resources.map(resource => {
    // Convert status segments
    const statusSegments = resource.statusSegments.map(seg => ({
      start: new Date(seg.startTime * 1000),      // Unix seconds → Date
      end: new Date(seg.endTime * 1000),
      status: seg.status as ResourceStatus,
      message: seg.message,
      config: seg.config
    }));

    // Convert events
    const events = resource.events.map(evt => ({
      id: evt.id,
      timestamp: new Date(evt.timestamp * 1000),  // Unix seconds → Date
      verb: evt.verb,
      user: evt.user,
      message: evt.message,
      details: evt.details
    }));

    // Return transformed resource
    return {
      id: resource.id,
      group: resource.group,
      version: resource.version,
      kind: resource.kind,
      namespace: resource.namespace,
      name: resource.name,
      statusSegments,
      events
    };
  });
}
```

**Frontend Result (TypeScript)**:
```typescript
const resource: K8sResource = {
  id: "pod-12345",
  group: "core",
  version: "v1",
  kind: "Pod",
  namespace: "default",
  name: "my-app-xyz",
  statusSegments: [
    {
      start: Date(Wed Nov 15 2023 04:26:40 GMT+0000),
      end: Date(Wed Nov 15 2023 05:26:40 GMT+0000),
      status: ResourceStatus.Ready,
      message: "Pod is running",
      config: {image: "my-app:1.0.0"}
    }
  ],
  events: [
    {
      id: "evt-001",
      timestamp: Date(Wed Nov 15 2023 04:28:20 GMT+0000),
      verb: "create",
      user: "kubelet",
      message: "Pod created",
      details: null
    }
  ]
}
```

## Configuration

### Environment Variables

**Frontend** (ui/.env):
```
VITE_API_BASE=http://localhost:8080/v1
```

**Backend** (main.go):
```go
API_PORT=8080       // Port for HTTP API
API_TIMEOUT=30s     // Request timeout
```

## Testing the Integration

### Manual Testing

**1. Start Backend**:
```bash
go run main.go
```

**2. Start Frontend Dev Server**:
```bash
cd ui
npm run dev
```

**3. Open Browser**:
```
http://localhost:5173
```

**4. Verify API Calls**:
- Open DevTools (F12)
- Go to Network tab
- Interact with UI (change filters, etc.)
- Observe `/v1/search` requests
- Check response data in Network tab

### Expected Results

- ✓ Page loads without errors
- ✓ Resources display in timeline
- ✓ Filters work (namespace, kind, etc.)
- ✓ Detail panel shows events
- ✓ No console errors
- ✓ API requests appear in Network tab
- ✓ Response times < 3 seconds for typical data

## Next Steps

1. Implement backend response formatting (Phase 2)
2. Implement frontend API service (Phase 2)
3. Remove mock data from codebase (Phase 2)
4. Test integration (Phase 2)
5. Handle edge cases and errors (Phase 2)
