# Tasks: Audit Timeline UI

**Input**: Design documents from `/specs/004-audit-timeline-ui/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/api.openapi.yaml, quickstart.md

**Organization**: Tasks grouped by user story priority (P1, P2, P3) to enable independent implementation and parallel development.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1-US8)
- Include exact file paths in all descriptions

## Path Conventions

- **Frontend**: `ui/src/` for React components and services
- **Tests**: `ui/tests/` for all test files
- **Types**: `ui/src/types/` for TypeScript interfaces
- **API**: Backend at `/internal/api` (external dependency)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic development environment

- [x] T001 Verify existing project structure in `ui/` directory (vite.config.ts, tsconfig.json, package.json)
- [x] T002 Install additional dependencies (D3.js 7.x, React Query, Tailwind CSS or styled-components) in `ui/package.json`
- [x] T003 [P] Create TypeScript interfaces for all entities in `ui/src/types/` (Resource.ts, Segment.ts, AuditEvent.ts, Namespace.ts, Kind.ts)
- [x] T004 [P] Create constants file (`ui/src/constants.ts`) with status colors, enum mappings, and UI configuration
- [x] T005 Configure environment variables for API endpoint (`ui/.env.local`) with VITE_API_BASE
- [x] T006 Setup vitest configuration (`ui/vitest.config.ts`) with React Testing Library setup
- [x] T007 [P] Create ESLint and Prettier configuration for TypeScript/React consistency

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that blocks all user stories

**⚠️ CRITICAL**: No user story work can begin until this phase completes

- [x] T008 Create API client service (`ui/src/services/api.ts`) with error handling and request/response types
  - Implements: GET /api/metadata, GET /api/resources, GET /api/events/{resourceId}
  - Uses React Query for caching and state management
- [x] T009 Create data transformation utilities (`ui/src/services/dataTransform.ts`)
  - Transforms backend responses into frontend data structures
  - Normalizes resources and segments for efficient lookup
  - Handles filtering and sorting operations
- [x] T010 [P] Create D3.js timeline utilities (`ui/src/services/timelineUtils.ts`)
  - Time scale and band scale creation
  - Coordinate calculations for segments and resources
  - Viewport culling for rendering optimization
- [x] T011 [P] Create JSON diff calculator (`ui/src/services/diffCalculator.ts`)
  - Compares current vs previous configuration
  - Categorizes changes as added/removed/modified
  - Handles deeply nested object comparison
- [x] T012 [P] Create custom hooks for state management (`ui/src/hooks/`)
  - `useTimeline.ts`: Fetch and manage timeline data
  - `useFilters.ts`: Manage filter selections and compute filtered results
  - `useSelection.ts`: Track selected segment and detail panel state
  - `useKeyboard.ts`: Handle keyboard shortcuts and accessibility
- [x] T013 Create main App component (`ui/src/App.tsx`) with context providers and error boundaries
- [x] T014 Create mock API response data (`ui/src/mocks/`) for development and testing without backend

**Checkpoint**: Foundation ready - all user stories can begin implementation in parallel

---

## Phase 3: User Story 1 - Explore Resource History Timeline (Priority: P1) 🎯 MVP

**Goal**: Display an interactive D3-based timeline showing Kubernetes resources with color-coded status segments and audit events

**Independent Test**: User can load the dashboard, see timeline with resource rows, colored segments representing status periods, and small dots representing audit events. Timeline is responsive to viewport changes.

### Implementation for User Story 1

- [x] T015 [P] [US1] Create Timeline component (`ui/src/components/Timeline/Timeline.tsx`)
  - Accepts resources and segments from props
  - Renders D3 SVG-based Gantt chart
  - Implements viewport culling (only render visible segments)
  - Handles zoom/pan interactions via D3 behavior

- [x] T016 [P] [US1] Create TimelineTooltip component (`ui/src/components/Timeline/TimelineTooltip.tsx`)
  - Shows resource metadata on hover
  - Displays segment details and duration

- [x] T017 [P] [US1] Create segment rendering styles (`ui/src/components/Timeline/Timeline.module.css`)
  - Color-coded status segments (Ready=Green, Warning=Yellow, Error=Red, Terminating/Unknown=Gray)
  - Sticky time scale header at top
  - Smooth transitions for color changes

- [x] T018 [US1] Implement Timeline.tsx D3 integration
  - Create time scale (scaleTime) for X-axis
  - Create band scale (scaleBand) for Y-axis resources
  - Render resource rows with segmented bars
  - Overlay audit event dots on segments (depends on T015, T016, T017)

- [x] T019 [P] [US1] Create Dashboard page (`ui/src/pages/Dashboard.tsx`)
  - Layout: TopBar (top), Timeline (center), DetailPanel (right when selected)
  - Fetches initial metadata and resources on mount
  - Manages overall data flow

- [x] T020 [P] [US1] Create Empty State component (`ui/src/components/Common/EmptyState.tsx`)
  - Displays when no resources match filters
  - Prompts user to adjust filters or check data availability

- [x] T021 [P] [US1] Create Loading State component (`ui/src/components/Common/Loading.tsx`)
  - Shows spinner while fetching metadata and resources
  - Non-blocking (allows interaction with existing data)

- [x] T022 [US1] Create integration test for timeline rendering (`ui/tests/integration/Timeline.integration.test.tsx`)
  - Mocks API responses with sample resources and segments
  - Verifies segments render with correct colors
  - Tests viewport culling (only visible segments rendered)

- [x] T023 [US1] Create unit tests for D3 utilities (`ui/tests/unit/services/timelineUtils.test.ts`)
  - Test scale creation for various data ranges
  - Test coordinate calculations
  - Test viewport culling logic

**Checkpoint**: User Story 1 complete - timeline displays and is interactive

---

## Phase 4: User Story 2 - Filter Resources by Namespace and Kind (Priority: P1)

**Goal**: Allow users to select namespaces and resource kinds from dropdown filters to focus timeline on relevant resources

**Independent Test**: User can open filter dropdowns, select multiple namespaces/kinds, timeline updates to show only matching resources. Keyboard navigation (arrows, enter, escape) works in dropdowns.

### Implementation for User Story 2

- [x] T024 [P] [US2] Create FilterDropdown component (`ui/src/components/TopBar/FilterDropdown.tsx`)
  - Multi-select dropdown with checkboxes
  - Keyboard navigation support (Arrow Keys, Enter/Space, Escape)
  - Accessibility attributes (ARIA labels, roles)
  - Optional search within dropdown

- [x] T025 [P] [US2] Create TopBar component (`ui/src/components/TopBar/TopBar.tsx`)
  - Layout: Branding (left), Search input (center), Dropdowns (right)
  - Passes filter state to parent Dashboard
  - Responsive layout for small screens

- [x] T026 [P] [US2] Create Branding component (`ui/src/components/TopBar/Branding.tsx`)
  - Displays "Spectre" text with ghost icon
  - Positioned in top-left of TopBar

- [x] T027 [US2] Integrate filter dropdowns into Dashboard (`ui/src/pages/Dashboard.tsx`)
  - Wire namespace and kind dropdowns to useFilters hook
  - Recompute filtered segments when filters change
  - Update timeline with filtered data (depends on T024, T025)

- [x] T028 [P] [US2] Create keyboard handler in useKeyboard hook (`ui/src/hooks/useKeyboard.ts`)
  - Intercept Arrow Keys for dropdown navigation
  - Handle Enter/Space for selection toggle
  - Handle Escape for dropdown close

- [x] T029 [P] [US2] Create unit tests for FilterDropdown (`ui/tests/unit/components/FilterDropdown.test.tsx`)
  - Test render of available options
  - Test multi-select functionality
  - Test keyboard navigation behavior

- [x] T030 [US2] Create integration test for filtering (`ui/tests/integration/Filters.integration.test.tsx`)
  - User selects namespace filter
  - Timeline updates to show only matching resources
  - Verify filter persistence across interactions

**Checkpoint**: User Story 2 complete - filtering by namespace and kind works

---

## Phase 5: User Story 3 - Search Resources by Name (Priority: P1)

**Goal**: Enable users to quickly find resources by typing in a search box that filters timeline in real-time

**Independent Test**: User types in search box, timeline updates instantly to show only resources with matching names. Clearing search shows all resources again. Empty search shows appropriate message.

### Implementation for User Story 3

- [x] T031 [P] [US3] Create SearchInput component (`ui/src/components/TopBar/SearchInput.tsx`)
  - Text input field with placeholder "Search resources by name..."
  - Debounced input handling (avoid excessive recomputation)
  - Clear button to reset search term
  - Enter key to trigger search

- [x] T032 [US3] Integrate SearchInput into TopBar (`ui/src/components/TopBar/TopBar.tsx`)
  - Add SearchInput next to or above filter dropdowns
  - Pass search term to parent Dashboard via callback

- [x] T033 [US3] Implement search filtering in useFilters hook (`ui/src/hooks/useFilters.ts`)
  - Filter segments by resource name (case-insensitive substring match)
  - Combine search with namespace/kind filters
  - Debounce with 300ms delay to avoid performance issues

- [x] T034 [P] [US3] Create unit tests for SearchInput (`ui/tests/unit/components/SearchInput.test.tsx`)
  - Test input value updates
  - Test debounce behavior
  - Test clear button functionality

- [x] T035 [US3] Create integration test for search filtering (`ui/tests/integration/Search.integration.test.tsx`)
  - User types search term
  - Timeline updates in real-time with matching resources
  - Clearing search shows all resources

**Checkpoint**: User Story 3 complete - real-time resource name search works

---

## Phase 6: User Story 4 - Inspect Selected Segment Details (Priority: P1)

**Goal**: Allow users to click timeline segments and see a detail panel with status, timestamps, configuration, and relevant audit events

**Independent Test**: User clicks a timeline segment, detail panel slides in from right showing all segment details. Clicking close button or pressing Escape closes panel. Panel displays correct resource metadata, segment status/timestamps, and filtered events.

### Implementation for User Story 4

- [x] T036 [P] [US4] Create DetailPanel component (`ui/src/components/Sidebar/DetailPanel.tsx`)
  - Slide-in panel from right side
  - Header: Resource name, kind, namespace, close button
  - Status section: Color-coded status, timestamps, message
  - Event list (delegated to EventList component)
  - Config diff section (delegated to ConfigDiff component)

- [x] T037 [P] [US4] Create EventList component (`ui/src/components/Sidebar/EventList.tsx`)
  - Displays audit events for segment's time window
  - Shows event type, timestamp, user, message
  - Handles empty state (no events in window)
  - Optional: Virtual scrolling for many events

- [x] T038 [P] [US4] Create Sidebar styles (`ui/src/components/Sidebar/Sidebar.module.css`)
  - Slide-in animation from right
  - Scrollable content area
  - Sticky header
  - Responsive width (min-width on small screens)

- [x] T039 [US4] Implement segment selection in Timeline component (`ui/src/components/Timeline/Timeline.tsx`)
  - Click handler on segment bars
  - Call onSegmentClick callback with segment ID
  - Highlight selected segment with white border
  - Update timeline center/zoom on selected segment (depends on T015)

- [x] T040 [US4] Integrate DetailPanel into Dashboard (`ui/src/pages/Dashboard.tsx`)
  - Show/hide panel based on selectedSegmentId state
  - Fetch events for selected resource via API
  - Pass segment and events data to DetailPanel
  - Handle Escape key to close panel (depends on T036, T039)

- [x] T041 [P] [US4] Create Keyboard handler for detail panel escape (`ui/src/hooks/useKeyboard.ts`)
  - Escape key closes detail panel when open
  - Does not interfere with other Escape listeners

- [x] T042 [P] [US4] Create unit tests for DetailPanel (`ui/tests/unit/components/DetailPanel.test.tsx`)
  - Test render of resource metadata
  - Test display of segment status and timestamps
  - Test event list rendering

- [x] T043 [US4] Create integration test for segment selection (`ui/tests/integration/SegmentSelection.integration.test.tsx`)
  - Click timeline segment
  - Verify detail panel appears with correct data
  - Click close button
  - Verify panel closes

**Checkpoint**: User Story 4 complete - segment inspection with detail panel works

---

## Phase 7: User Story 5 - Compare Configuration Changes (Priority: P2)

**Goal**: Show JSON diff of configuration changes between consecutive segment states in the detail panel

**Independent Test**: Detail panel displays configuration diff section with added (green), removed (red), and modified (yellow) changes highlighted. First segment shows "no previous version" message. Diff renders correctly for large payloads.

### Implementation for User Story 5

- [x] T044 [P] [US5] Create ConfigDiff component (`ui/src/components/Sidebar/ConfigDiff.tsx`)
  - Displays current and previous configurations side-by-side
  - Highlights additions in green, removals in red, modifications in yellow
  - Handles first segment (no previous state) gracefully
  - Scrollable for large diffs

- [x] T045 [P] [US5] Create diff visualization helper (`ui/src/components/Sidebar/DiffViewer.tsx`)
  - Renders JSON with syntax highlighting
  - Marks diff regions with color background
  - Handles collapsible sections for nested objects

- [x] T046 [US5] Integrate ConfigDiff into DetailPanel (`ui/src/components/Sidebar/DetailPanel.tsx`)
  - Fetch previous segment's configuration from API or local state
  - Call diffCalculator to compute differences
  - Pass current and diff result to ConfigDiff component
  - Handle error case (configuration unavailable)

- [x] T047 [P] [US5] Create unit tests for diffCalculator (`ui/tests/unit/services/diffCalculator.test.ts`)
  - Test detection of added keys
  - Test detection of removed keys
  - Test detection of modified values
  - Test nested object comparison
  - Test large payload handling

- [x] T048 [P] [US5] Create unit tests for ConfigDiff component (`ui/tests/unit/components/ConfigDiff.test.tsx`)
  - Test rendering of diff with color highlighting
  - Test "no previous version" message for first segment
  - Test handling of large diffs

- [x] T049 [US5] Create integration test for configuration diff (`ui/tests/integration/ConfigDiff.integration.test.tsx`)
  - Select segment with previous state
  - Verify diff shows added/removed/modified changes with correct colors
  - Select first segment
  - Verify appropriate "no previous version" message

**Checkpoint**: User Story 5 complete - configuration diff visualization works

---

## Phase 8: User Story 6 - Navigate Resource History with Keyboard (Priority: P2)

**Goal**: Allow users to press Left/Right arrow keys to navigate between historical segments of a selected resource

**Independent Test**: User selects a segment, presses Right Arrow to move to next segment, presses Left Arrow to move to previous segment. Detail panel updates instantly. Navigation stops at first/last segment (boundary conditions).

### Implementation for User Story 6

- [x] T050 [US6] Enhance useSelection hook to track segment history (`ui/src/hooks/useSelection.ts`)
  - Get current resource segments list
  - Implement nextSegment() and previousSegment() functions
  - Check boundaries (first/last segment)
  - Update selected segment on keyboard input

- [x] T051 [P] [US6] Create keyboard navigation handler (`ui/src/hooks/useKeyboard.ts`)
  - Intercept Left/Right arrow keys
  - Prevent default arrow key behavior (page scroll)
  - Dispatch nextSegment/previousSegment actions
  - Only active when segment is selected

- [x] T052 [US6] Integrate arrow key navigation into Dashboard (`ui/src/pages/Dashboard.tsx`)
  - Register keyboard listeners via useKeyboard hook
  - Call nextSegment/previousSegment on arrow key press
  - Update detail panel with new segment data (depends on T050)

- [x] T053 [P] [US6] Create unit tests for segment history navigation (`ui/tests/unit/hooks/useSelection.test.ts`)
  - Test nextSegment() moves to next chronological segment
  - Test previousSegment() moves to previous segment
  - Test boundary conditions (first/last segment returns false)

- [x] T054 [US6] Create integration test for arrow key navigation (`ui/tests/integration/KeyboardNavigation.integration.test.tsx`)
  - Select segment
  - Press Right Arrow, verify next segment selected
  - Press Left Arrow, verify previous segment selected
  - Verify boundaries (no action when at first/last)

**Checkpoint**: User Story 6 complete - keyboard navigation between segments works

---

## Phase 9: User Story 7 - Interactive Timeline Navigation (Priority: P2)

**Goal**: Allow users to zoom and pan across the timeline using mouse drag and Shift+Scroll for exploring different time ranges

**Independent Test**: User can drag horizontally on timeline to zoom in/out. Shift+Click/Scroll pans left/right. Vertical scrolling shows more resources. Selected segment auto-centers in viewport when possible. Large segments zoom out automatically.

### Implementation for User Story 7

- [x] T055 [US7] Enhance Timeline component with D3 zoom/pan behavior (`ui/src/components/Timeline/Timeline.tsx`)
  - Implement d3.zoom() for drag-to-zoom behavior
  - Constrain zoom to reasonable levels (prevent over-zooming)
  - Update scales on zoom/pan
  - Re-render only visible segments on interaction
  - Disable CSS transitions during interaction for performance

- [x] T056 [US7] Implement Shift+Scroll pan behavior (`ui/src/components/Timeline/Timeline.tsx`)
  - Listen for Shift+Wheel events
  - Adjust x-axis pan offset
  - Smooth scroll behavior with requestAnimationFrame

- [x] T057 [US7] Implement vertical scrolling for resources (`ui/src/components/Timeline/Timeline.tsx`)
  - Allow mouse wheel scroll to reveal more resources
  - Virtual scrolling: only render visible resource rows
  - Smooth scroll without jumping

- [x] T058 [P] [US7] Enhance Timeline auto-centering on segment selection (`ui/src/services/timelineUtils.ts`)
  - When segment selected, calculate optimal zoom/pan to center it
  - If segment wider than viewport, zoom out to fit with padding
  - Animate transition to new view

- [x] T059 [US7] Integrate auto-centering into Timeline component (`ui/src/components/Timeline/Timeline.tsx`)
  - Call centerSegment utility when selectedSegmentId changes
  - Smooth D3 transition to new view
  - Depends on T055, T058

- [x] T060 [P] [US7] Create performance tests for zoom/pan (`ui/tests/integration/Performance.performance.test.tsx`)
  - Measure FPS during drag-to-zoom interaction
  - Verify 60 FPS maintained during pan
  - Test with large dataset (1000+ segments)

- [x] T061 [US7] Create integration test for timeline navigation (`ui/tests/integration/TimelineNavigation.integration.test.tsx`)
  - User drags on timeline to zoom in/out
  - Shift+Scroll pans timeline
  - Vertical scroll reveals more resources
  - Selected segment auto-centers
  - Large segment zooms out to fit

**Checkpoint**: User Story 7 complete - interactive timeline zoom/pan works smoothly

---

## Phase 10: User Story 8 - View Branding and Navigation (Priority: P3)

**Goal**: Display "Spectre" branding with ghost icon in the top bar

**Independent Test**: UI displays "Spectre" text with ghost icon in top-left of application. Branding is visible and styled consistently.

### Implementation for User Story 8

- [x] T062 [P] [US8] Create Branding component with ghost icon (`ui/src/components/TopBar/Branding.tsx`)
  - Display "Spectre" text
  - Render ghost icon (SVG or icon library)
  - Link to home/dashboard on click

- [x] T063 [P] [US8] Create branding styles (`ui/src/components/TopBar/TopBar.module.css`)
  - Position branding in top-left
  - Icon sizing and spacing
  - Hover/active states

- [x] T064 [US8] Integrate Branding into TopBar (`ui/src/components/TopBar/TopBar.tsx`)
  - Add Branding component to left side
  - Ensure responsive layout on mobile

- [x] T065 [P] [US8] Create unit tests for Branding component (`ui/tests/unit/components/Branding.test.tsx`)
  - Test render of "Spectre" text
  - Test icon is rendered
  - Test click handler

**Checkpoint**: User Story 8 complete - branding displayed

---

## Phase 11: Polish & Cross-Cutting Concerns

**Purpose**: Performance optimization, accessibility, error handling, documentation

- [x] T066 [P] Create error handling and Error Boundary (`ui/src/components/Common/ErrorBoundary.tsx`)
  - Catches React errors in component tree
  - Displays user-friendly error message
  - Includes error details in development mode
  - Provides recovery action (reload, navigate home)

- [x] T067 [P] Implement global error handling in API client (`ui/src/services/api.ts`)
  - Exponential backoff for failed requests
  - User-friendly error messages
  - Logging of errors for debugging
  - Network error vs. server error handling

- [x] T068 [P] Create accessibility audit and fixes
  - Verify WCAG 2.1 AA compliance
  - Add missing ARIA labels and roles
  - Test with screen readers (NVDA, JAWS)
  - Ensure keyboard-only navigation works

- [x] T069 [P] Performance optimization
  - Profile with React DevTools Profiler
  - Ensure no unnecessary re-renders
  - Verify virtualization working for large datasets
  - Measure load time for 500+ resources

- [x] T070 [P] Create Loading states and Skeletons
  - Skeleton screens while data loads
  - Loading indicators during interactions
  - Avoid blank screens

- [x] T071 Create API documentation in README (`ui/README.md`)
  - Setup instructions from quickstart
  - Component overview
  - Available hooks documentation
  - API contract reference

- [x] T072 Create troubleshooting guide (`ui/TROUBLESHOOTING.md`)
  - Common issues and solutions
  - Browser console error reference
  - Performance debugging tips

- [x] T073 [P] Create visual regression test suite (`ui/tests/e2e/visual.spec.ts`)
  - Screenshot testing for all major components
  - Baseline comparison for future changes
  - Run with Playwright

- [x] T074 Create end-to-end test suite (`ui/tests/e2e/full-flow.spec.ts`)
  - Complete user journey from load to detail inspection
  - Test all filter combinations
  - Test keyboard navigation paths

- [x] T075 Create performance benchmarks (`ui/tests/performance/benchmarks.test.ts`)
  - Measure filter response time (<500ms)
  - Measure detail panel appearance (<200ms)
  - Measure initial load time (<3s for 500 resources)
  - Measure memory usage over time

**Checkpoint**: All user stories complete, optimized, tested, and documented

---

## Implementation Strategy

### MVP Scope (Phase 3 - User Story 1)
Start with User Story 1 only:
- Timeline visualization with color-coded segments
- Audit events as dots
- Basic layout and styling
- Verify performance with mock data

### Phase 2: Add Filtering (Phases 4-5 - User Stories 2-3)
- Namespace and Kind filtering
- Search by resource name
- All three filters work together

### Phase 3: Add Details & Interaction (Phases 6-7 - User Stories 4-5)
- Detail panel on segment click
- Configuration diff visualization
- Event list in detail panel

### Phase 4: Enhance Navigation (Phases 8-9 - User Stories 6-7)
- Keyboard shortcuts for segment navigation
- Zoom/pan/scroll interactions
- Auto-centering selected segment

### Phase 5: Polish (Phases 10-11 - User Story 8 + Polish)
- Branding and header
- Error handling and loading states
- Performance optimization
- Accessibility improvements
- Comprehensive testing

---

## Dependencies & Parallel Opportunities

### Can start in parallel:
- **Phase 1 Setup tasks** (T001-T007): All independent file creation
- **Phase 2 Foundational** (T008-T014): All independent service creation
- **Within each user story**: Component creation (T***) tasks are independent until integration

### Must complete before proceeding:
- **Phase 1** before Phase 2
- **Phase 2** before any User Story (Phase 3+)
- Within User Stories: Component creation before integration

### Recommended delivery order:
1. Complete Phase 1 + Phase 2 (foundation)
2. Complete Phase 3 (US1 - MVP timeline)
3. Complete Phase 4 + 5 (US2-3 - filtering)
4. Complete Phase 6 + 7 (US4-5 - details)
5. Complete Phase 8 + 9 (US6-7 - navigation)
6. Complete Phase 10 + 11 (US8 + polish)

Each phase is independently testable and deliverable as a complete feature increment.

---

## Task Counts Summary

| Phase | Phase Name | Task Count | User Story Coverage |
|-------|-----------|-----------|-------------------|
| 1 | Setup | 7 | Foundational |
| 2 | Foundational | 7 | Foundational |
| 3 | US1 Timeline | 9 | Explore timeline (P1) |
| 4 | US2 Filtering | 7 | Filter by namespace/kind (P1) |
| 5 | US3 Search | 5 | Search by name (P1) |
| 6 | US4 Details | 8 | Inspect segment details (P1) |
| 7 | US5 Config Diff | 6 | Compare configuration (P2) |
| 8 | US6 Keyboard Nav | 5 | Navigate with keyboard (P2) |
| 9 | US7 Timeline Nav | 7 | Zoom/pan/scroll timeline (P2) |
| 10 | US8 Branding | 4 | Display branding (P3) |
| 11 | Polish | 10 | Error handling, A11y, performance |
| | **TOTAL** | **75** | **All 8 user stories** |

---

## Acceptance Criteria Validation

Each task can be validated against success criteria:

**User Story 1**: Timeline renders in <3s with 500 resources ✓
**User Story 2**: Filters update in <500ms ✓
**User Story 3**: Search updates in real-time (<500ms) ✓
**User Story 4**: Detail panel appears in <200ms ✓
**User Story 5**: Config diff renders in <1s ✓
**User Story 6**: Keyboard navigation instant ✓
**User Story 7**: Timeline zoom/pan maintains 60 FPS ✓
**User Story 8**: Branding always visible ✓

All tasks complete when each user story independently passes its acceptance scenarios.
