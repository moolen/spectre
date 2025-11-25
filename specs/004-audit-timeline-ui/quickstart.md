# Quickstart: Audit Timeline UI Development

**Feature**: Audit Timeline UI (`004-audit-timeline-ui`)
**Date**: 2025-11-25

## Overview

This guide gets you started developing the Audit Timeline UI. The frontend is a React/TypeScript application that visualizes Kubernetes audit data in an interactive timeline.

## Prerequisites

- Node.js 18+ with npm
- Knowledge of React, TypeScript, and D3.js (basic level sufficient to start)
- Running backend API at `http://localhost:8080/api` (or configured via environment)

## Project Setup

### 1. Install Dependencies

From the `ui/` directory:

```bash
cd ui
npm install
```

Key dependencies that will be installed:
- `react@18.x` - UI framework
- `d3@7.x` - Timeline visualization
- `typescript@5.x` - Type safety
- `vite` - Build tool (dev server, bundling)
- `vitest` - Unit testing
- `@testing-library/react` - Component testing

### 2. Configure Environment

Create `.env.local` in the `ui/` directory:

```bash
# Backend API endpoint
VITE_API_BASE=http://localhost:8080/api

# Optional: Feature flags
VITE_ENABLE_VERBOSE_LOGGING=false
VITE_DEBUG_TIMELINE=false
```

### 3. Start Development Server

```bash
npm run dev
```

The app will be available at `http://localhost:5173` (Vite's default). The server hot-reloads on file changes.

### 4. Build for Production

```bash
npm run build
```

Output is in `ui/dist/` for deployment.

## Architecture Overview

### Directory Structure

```
ui/src/
├── components/        # React components organized by feature
│   ├── Timeline/     # D3-based timeline visualization
│   ├── Sidebar/      # Detail panel (right side)
│   ├── TopBar/       # Navigation, search, filters (top)
│   └── Common/       # Shared UI elements
├── pages/            # Full-page components
├── services/         # API client and utilities
├── types/            # TypeScript interfaces
├── hooks/            # Custom React hooks for state management
├── constants.ts      # Colors, enums, configuration
├── App.tsx           # Root component
└── index.tsx         # Entry point
```

### Key Components

#### 1. Timeline Component (`Timeline.tsx`)

The main visualization using D3.js. Responsibilities:
- Render resources as rows on Y-axis
- Render status segments as colored bars
- Render audit events as small dots
- Handle zoom/pan interactions
- Highlight selected segment

**Props**:
```typescript
interface TimelineProps {
  segments: Segment[];
  selectedSegmentId?: string;
  onSegmentClick: (segmentId: string) => void;
  timeRange: { start: Date; end: Date };
}
```

**Key Methods**:
- `updateScale()` - Recalculate D3 scales when data changes
- `handleDrag()` - Process zoom/pan gestures
- `highlightSegment()` - Visual feedback for selection
- `renderEvents()` - Overlay event dots

#### 2. DetailPanel Component (`DetailPanel.tsx`)

Right-side panel showing segment details. Includes:
- Resource metadata
- Status and time range
- Configuration diff (vs. previous state)
- List of audit events in segment window

**Props**:
```typescript
interface DetailPanelProps {
  segment: Segment;
  resource: Resource;
  events: AuditEvent[];
  previousConfiguration?: Record<string, any>;
  onClose: () => void;
}
```

#### 3. TopBar Component (`TopBar.tsx`)

Navigation and filtering controls:
- Spectre branding with ghost icon
- Search input for resource name
- Namespace dropdown (multi-select)
- Kind dropdown (multi-select)

**Props**:
```typescript
interface TopBarProps {
  namespaces: string[];
  kinds: string[];
  selectedNamespaces: Set<string>;
  selectedKinds: Set<string>;
  searchTerm: string;
  onNamespaceChange: (selected: Set<string>) => void;
  onKindChange: (selected: Set<string>) => void;
  onSearchChange: (term: string) => void;
}
```

### State Management

Custom hooks manage three state domains:

#### `useTimeline()`
- Fetches resources/segments from API
- Maintains timeline data in memory
- Handles data transformation

```typescript
const { resources, segments, loading, error } = useTimeline();
```

#### `useFilters()`
- Manages filter selections (namespaces, kinds, search)
- Computes filtered segment list
- No API calls (filtering happens client-side)

```typescript
const {
  selectedNamespaces,
  selectedKinds,
  searchTerm,
  filteredSegments,
  setNamespaceFilter,
  setKindFilter,
  setSearchTerm
} = useFilters();
```

#### `useSelection()`
- Tracks selected segment
- Manages detail panel visibility
- Handles keyboard navigation state

```typescript
const {
  selectedSegmentId,
  detailPanelOpen,
  selectSegment,
  closeDetailPanel,
  nextSegment,
  previousSegment
} = useSelection();
```

#### `useKeyboard()`
- Registers keyboard event listeners
- Manages focus and shortcut handling
- Provides accessibility support

```typescript
const { isPressed, registerShortcut } = useKeyboard();
```

### Services

#### `api.ts` - Backend Communication

```typescript
// Fetch metadata (namespaces, kinds) on page load
export const fetchMetadata = async (): Promise<Metadata> => { ... }

// Fetch timeline data
export const fetchResources = async (filters?: FilterParams): Promise<Resource[]> => { ... }

// Fetch events for a specific resource
export const fetchEvents = async (resourceId: string): Promise<AuditEvent[]> => { ... }
```

All requests use error handling with exponential backoff. Responses are cached.

#### `dataTransform.ts` - Data Wrangling

Transforms backend responses into frontend data structures:
```typescript
export const normalizeResources = (raw: any[]): Map<string, Segment[]> => { ... }
export const filterSegments = (segments: Segment[], filters: Filters): Segment[] => { ... }
```

#### `timelineUtils.ts` - D3 Calculations

D3 scale management and coordinate calculations:
```typescript
export const createTimeScale = (start: Date, end: Date, width: number) => { ... }
export const createBandScale = (resources: Resource[], height: number) => { ... }
```

#### `diffCalculator.ts` - JSON Diff Logic

Compares two JSON objects and categorizes changes:
```typescript
export const calculateDiff = (
  current: Record<string, any>,
  previous: Record<string, any>
): DiffResult => { ... }

interface DiffResult {
  added: Record<string, any>;
  removed: Record<string, any>;
  modified: Record<string, any>;
}
```

### Types

TypeScript interfaces are in `types/` directory:

```typescript
// types/Resource.ts
export interface Resource {
  id: string;
  name: string;
  kind: string;
  namespace: string;
  createdAt: Date;
  deletedAt?: Date;
}

// types/Segment.ts
export interface Segment {
  id: string;
  resourceId: string;
  status: 'Ready' | 'Warning' | 'Error' | 'Terminating' | 'Unknown';
  startTime: Date;
  endTime: Date;
  message?: string;
  configuration: Record<string, any>;
}

// types/AuditEvent.ts
export interface AuditEvent {
  id: string;
  resourceId: string;
  timestamp: Date;
  eventType: string;
  user?: string;
  changes?: Record<string, any>;
  message?: string;
}
```

## Development Workflow

### 1. Adding a New Feature

Example: Adding a status filter dropdown

1. **Create component**: `components/TopBar/StatusFilterDropdown.tsx`
   - Use `FilterDropdown.tsx` as template
   - Define props interface
   - Implement render and event handlers

2. **Update state**: Extend `useFilters()` hook
   - Add status selection state
   - Add computed filter function
   - Export filtered results

3. **Integrate**: Add to `TopBar.tsx`
   - Pass props from parent
   - Wire event handlers
   - Update layout

4. **Test**: Write `tests/unit/StatusFilterDropdown.test.tsx`
   - Test render, interaction, state changes
   - Use React Testing Library

### 2. Modifying Timeline Visualization

1. Update `components/Timeline/Timeline.tsx` for visual changes
2. Update `services/timelineUtils.ts` for calculation changes
3. Test with `tests/integration/Timeline.integration.test.tsx`
4. Verify performance with React DevTools Profiler

### 3. Updating API Integration

1. Modify endpoint URL in `services/api.ts`
2. Update response type in `types/ApiResponse.ts`
3. Adjust data transformation in `services/dataTransform.ts`
4. Write contract test in `tests/integration/api.test.ts`

## Testing

### Run All Tests

```bash
npm test
```

### Run Specific Test Suite

```bash
npm test -- Timeline.test.tsx
```

### Watch Mode

```bash
npm test -- --watch
```

### Coverage Report

```bash
npm test -- --coverage
```

### E2E Tests (Playwright)

```bash
npm run test:e2e
```

## Performance Debugging

### React DevTools Profiler

1. Install React DevTools browser extension
2. Open DevTools → Profiler tab
3. Click "Record" and interact with timeline
4. Analyze component render times and re-renders

### Timeline Performance

For large datasets, check:
1. **Segment rendering**: Should only render visible segments
2. **Event dot rendering**: Should cull dots outside viewport
3. **Filter updates**: Should be <500ms for 1000 segments
4. **Zoom/pan**: Should maintain 60 FPS

### Memory Profiling

1. DevTools → Memory tab
2. Take heap snapshot before and after interaction
3. Check for memory leaks (growing heap over time)

## Connecting to Backend API

The app expects a backend API at `VITE_API_BASE` (default: `http://localhost:8080/api`).

### Required Endpoints

- `GET /metadata` - Fetch namespaces and kinds
- `GET /resources` - Fetch timeline data
- `GET /events/{resourceId}` - Fetch audit events

See [contracts/api.openapi.yaml](contracts/api.openapi.yaml) for full API specification.

### Mock Data for Development

If backend isn't available, use mock data in `services/api.ts`:

```typescript
// Uncomment mock provider
import { createMockServer } from './mocks/server';
createMockServer(); // MSW intercepts API calls
```

Mock data includes:
- 10 sample resources across 2 namespaces
- 20+ segments with various statuses
- 50+ audit events

## Common Tasks

### Add New Status Color

1. Update `STATUS_COLORS` in `constants.ts`
2. Update `Segment` type in `types/Segment.ts` if adding new status
3. Update D3 color scale in `timelineUtils.ts`
4. Add tests in `tests/unit/constants.test.ts`

### Optimize Performance

1. Add React.memo to low-change components
2. Use useCallback for event handlers
3. Implement virtualization for long lists
4. Profile with React DevTools Profiler

### Add Keyboard Shortcut

1. Define shortcut in `useKeyboard()` hook
2. Register handler in relevant component
3. Add keyboard event listener in `useKeyboard`
4. Test with keyboard event simulator

## Troubleshooting

### "Cannot find module" Errors

Check that:
- All imports use correct relative paths
- TypeScript strict mode is configured (`tsconfig.json`)
- Files are in correct directories

### API Connection Issues

- Check `VITE_API_BASE` environment variable
- Verify backend is running and accessible
- Check browser console for CORS errors
- Use Firefox/Chrome DevTools Network tab to inspect requests

### Timeline Not Rendering

- Check browser console for errors
- Verify D3 scales are created before rendering
- Check that segments have valid time ranges
- Verify SVG elements are being appended to DOM

### Performance Issues

- Profile with React DevTools Profiler
- Check that virtualization is working (only visible segments rendered)
- Verify memoization is preventing unnecessary re-renders
- Check that filter computations aren't blocking rendering

## Next Steps

After setting up:
1. Review [data-model.md](data-model.md) to understand data structures
2. Study [contracts/api.openapi.yaml](contracts/api.openapi.yaml) for API details
3. Look at existing components in `src/components/` for patterns
4. Start with a small feature (e.g., add a new filter dropdown)
5. Run tests frequently to catch issues early

## Additional Resources

- [React Documentation](https://react.dev)
- [TypeScript Handbook](https://www.typescriptlang.org/docs)
- [D3.js Documentation](https://d3js.org)
- [Vite Guide](https://vitejs.dev/guide/)
- [Testing Library Best Practices](https://testing-library.com)
