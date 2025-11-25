# Research Phase: Audit Timeline UI

**Feature**: Audit Timeline UI (`004-audit-timeline-ui`)
**Date**: 2025-11-25
**Status**: Complete

## D3.js Timeline Implementation for Large Datasets

### Decision: D3.js 7.x with virtualization techniques

### Rationale
D3.js is the industry standard for interactive data visualization, particularly Gantt-style timelines. For 1000+ segments across 100+ resources:
- D3's scale and axis systems handle continuous time scales efficiently
- Brush/zoom interactions provide the drag-to-zoom and pan functionality required
- Custom rendering can implement viewport-based virtualization to handle large datasets
- Transition system supports smooth 60 FPS animations for geometry and color changes
- Strong community support with proven patterns for timeline visualizations

### Alternatives Considered
1. **ECharts** - Excellent charting library but less flexible for custom Gantt implementations and D3 integration
2. **Chart.js** - Simpler but lacks the interactive control needed for this complex timeline
3. **Custom Canvas/SVG** - Would require building rendering and interaction handling from scratch, much higher complexity
4. **Recharts** - Built on D3 but less suitable for the specific Gantt use case and custom event overlays

### Implementation Pattern
- Use D3 scales (`scaleTime` for X-axis, `scaleBand` for Y-axis resources)
- Implement viewport culling: only render segments/resources currently visible
- Use `requestAnimationFrame` for smooth panning/zooming
- Separate CSS transitions for color/stroke changes (kept on) from geometry transitions (turned off) to prevent lag during zoom

---

## JSON Diff Visualization

### Decision: Custom diff calculator with color-coded highlighting

### Rationale
- Configuration diffs are between consecutive states (single-level comparison)
- JSON payloads can be up to 10MB (large structured objects)
- Need to highlight additions (green), removals (red), modifications (yellow)
- Custom solution provides:
  - Full control over rendering performance
  - Clean integration with React component lifecycle
  - Ability to handle deeply nested objects efficiently
  - No dependency on external diff library

### Alternatives Considered
1. **diff-match-patch** - Designed for text-level diffs, not ideal for structured JSON comparison
2. **json-diff library** - Good but adds external dependency and may not match exact color scheme
3. **Draft.js or similar** - Overly complex for this use case, brings in rich text editor complexity

### Implementation Pattern
- Two-pass comparison: first collect all paths that changed, then traverse both objects to categorize changes
- For large diffs (>1000 changes), implement collapsible sections to prevent DOM explosion
- Use a specialized React component that handles scrolling and highlights matching fields in both old and new versions side-by-side

---

## Keyboard Accessibility Patterns

### Decision: Custom keyboard handler with focus management

### Rationale
- Dropdown navigation (Arrow Keys, Enter/Space, Escape)
- Timeline segment navigation (Left/Right arrows)
- Detail panel dismiss (Escape)
- These interactions require precise focus management and event handling
- Custom implementation allows:
  - Consistent behavior across all interactive elements
  - Integration with React hooks for cleaner code
  - Ability to prevent default browser behaviors where needed
  - Clear state management for active focus

### Alternatives Considered
1. **Headless UI library (Radix UI)** - Excellent but adds dependency and bundle size
2. **WAI-ARIA patterns manual implementation** - Fine-grained control but error-prone
3. **Browser native controls** - Limited to standard HTML elements, insufficient for custom timeline

### Implementation Pattern
- Custom `useKeyboard` hook that:
  - Registers global keyboard listeners during component mount
  - Manages focus state in React context
  - Dispatches actions based on key combinations
  - Prevents default behavior for specific keys (arrows, escape)
- Implement `aria-*` attributes for screen reader support
- Visual focus indicators on all interactive elements

---

## Performance Optimization for Gantt Visualization

### Decision: Viewport-based rendering + React.memo + useCallback memoization

### Rationale
- 1000+ segments can mean 1000s of DOM nodes if all rendered
- Viewport-based rendering only creates DOM nodes for visible resources/segments
- React optimization prevents unnecessary re-renders of unchanged segments
- D3 transitions use CSS transforms (GPU-accelerated) rather than forcing layout recalculations
- Combined approach achieves 60 FPS target

### Optimization Strategy
1. **Rendering**: Only render resources and segments currently in viewport
2. **Memoization**: Wrap segment, resource, and event components with React.memo
3. **Event Handlers**: Wrap timeline interaction handlers with useCallback to prevent unnecessary re-renders
4. **Data Structure**: Store filtered results in separate memoized selector to avoid recomputing on every render
5. **CSS**: Use `will-change: transform` on animated elements; disable `pointer-events` on non-visible segments
6. **Virtual Scrolling**: For resource list (Y-axis), implement virtual scrolling to handle 100+ resources

### Performance Budgets
- Initial load: 3s for 500 resources (measured: Time to Interactive)
- Filter update: <500ms (from filter change to rendered update visible to user)
- Detail panel: <200ms (from click to panel fully visible with data)
- Animation frame rate: 60 FPS during zoom/pan (measured: no dropped frames >16ms)

---

## State Management Approach

### Decision: React Context API + custom hooks (no external library)

### Rationale
- Feature complexity doesn't warrant Redux or MobX
- Context API is built-in and familiar to React developers
- Custom hooks provide clean separation of concerns
- Three main state concerns:
  1. Timeline data (resources, segments, events) - data fetching via API
  2. Filter state (selected namespaces, kinds, search term)
  3. UI state (selected segment, detail panel visibility, zoom level)
- Custom hooks allow independent testing of each state domain

### Context Structure
```
- DataContext: Timeline resources, segments, audit events (fetched from backend)
- FilterContext: Active namespace/kind filters, search term, filtered results
- SelectionContext: Selected segment ID, timeline zoom/pan state, detail panel visibility
```

### Alternatives Considered
1. **Redux** - Overkill for 3 simple state domains; adds boilerplate
2. **Zustand or Jotai** - Lightweight alternatives but adds external dependency
3. **useReducer only** - Possible but Context API + hooks provides better composition

### Hook Pattern
- `useTimeline()` - Fetch and manage timeline data from API
- `useFilters()` - Manage filter selections and compute filtered results
- `useSelection()` - Manage segment selection and detail panel state
- `useKeyboard()` - Manage keyboard shortcuts for navigation

---

## API Contract Definition

### Decision: RESTful API with JSON responses

### Rationale
- Backend at `/internal/api` already exists with REST pattern
- Feature spec assumes backend API is extensible
- Simple HTTP GET/POST for metadata and audit data fetching
- Stateless endpoints allow client-side caching/state management
- JSON response format matches frontend data structures

### Endpoints Required
1. **GET /api/metadata** - Returns available namespaces, kinds, and resource counts
2. **GET /api/resources** - Returns timeline segments with status, timestamps, messages
3. **GET /api/events/:resourceId** - Returns audit events for a specific resource
4. **GET /api/config/:resourceId/:segmentId** - Returns configuration snapshot for comparison

### Response Formats
- Standard HTTP status codes (200, 400, 404, 500)
- Consistent JSON structure with `data` field containing results
- Error responses with `error` field containing message
- Pagination support for large result sets (limit/offset)

---

## Testing Strategy

### Decision: Multi-level testing pyramid

### Rationale
- Unit tests for utilities (diff calculation, data transformation, time calculations)
- Integration tests for React components with mock API
- E2E tests for critical user workflows (timeline display → filter → select → detail panel)

### Test Framework Choices
- **Unit/Integration**: Vitest (fast, native ESM, built for Vite projects) + React Testing Library (component testing best practices)
- **E2E**: Playwright (recommended for visual testing; Cypress alternative acceptable)

### Test Coverage Targets
- Service/utility functions: 90%+ coverage
- Component logic: 80%+ coverage (excluding purely presentational components)
- Critical user paths: 100% E2E coverage
- Performance: Lighthouse metrics and custom benchmarks

---

## Summary of Decisions

| Area | Decision | Key Reason |
|------|----------|-----------|
| Timeline Viz | D3.js 7.x with viewport culling | Industry standard, flexible, performance optimization possible |
| JSON Diff | Custom calculator + React component | Control over rendering, handles large payloads efficiently |
| Keyboard A11y | Custom useKeyboard hook | Full control, integrates cleanly with React, WAI-ARIA compliant |
| Performance | Viewport rendering + React.memo + useCallback | Achieves 60 FPS target with 1000+ segments |
| State Mgmt | Context API + custom hooks | Simple, no external dependencies, good composition |
| API | RESTful with JSON | Matches existing backend pattern, extensible design |
| Testing | Vitest + RTL + Playwright | Aligned with Vite/React stack, comprehensive coverage |

All research items complete. Ready for Phase 1 design.
