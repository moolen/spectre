# Feature Specification: Audit Timeline UI

**Feature Branch**: `004-audit-timeline-ui`
**Created**: 2025-11-25
**Status**: Draft
**Input**: Build a comprehensive audit data visualization dashboard with interactive timeline, filtering, and detailed audit event inspection for Kubernetes resources.

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

### User Story 1 - Explore Resource History Timeline (Priority: P1)

Platform operators need to understand what happened to a Kubernetes resource over time by viewing a visual timeline of state changes and audit events.

**Why this priority**: Core value of the feature - users cannot use the system without viewing the timeline.

**Independent Test**: User can load the UI, see a timeline of resources with color-coded status segments, and understand resource states without needing additional tools.

**Acceptance Scenarios**:

1. **Given** audit data exists for resources, **When** the UI loads, **Then** the timeline displays resources as rows with colored status segments
2. **Given** the timeline is displayed, **When** user views the Y-axis, **Then** each resource shows name, kind, and namespace
3. **Given** the timeline is displayed, **When** user views the X-axis, **Then** time scale is continuous and sticky at the top
4. **Given** status segments are visible, **When** user observes colors, **Then** Ready=Green, Warning=Yellow, Error=Red, Terminating/Unknown=Gray
5. **Given** audit events exist within a segment, **When** user looks at the timeline, **Then** small dots appear on segments representing discrete events

---

### User Story 2 - Filter Resources by Namespace and Kind (Priority: P1)

Platform operators need to focus on specific resources by filtering the timeline view by Kubernetes namespaces and resource kinds to reduce cognitive load.

**Why this priority**: Essential filtering capability that makes the timeline usable for large deployments.

**Independent Test**: User can select namespaces and kinds from dropdowns and the timeline updates to show only matching resources.

**Acceptance Scenarios**:

1. **Given** the UI is loaded, **When** user looks at the top bar, **Then** a namespace dropdown and kind dropdown are visible
2. **Given** multiple namespaces exist in the audit data, **When** user opens the namespace dropdown, **Then** all available namespaces are listed
3. **Given** multiple kinds exist in the audit data, **When** user opens the kind dropdown, **Then** all available kinds are listed (e.g., Pod, Deployment)
4. **Given** the dropdowns are open, **When** user selects one or more items, **Then** the timeline updates to show only matching resources
5. **Given** filters are applied, **When** user clears a filter, **Then** the timeline updates to include previously filtered resources
6. **Given** the dropdowns are visible, **When** user presses Arrow Keys, **Then** focus moves through options
7. **Given** focus is on an option, **When** user presses Enter or Space, **Then** option is toggled
8. **Given** dropdown is open, **When** user presses Escape, **Then** dropdown closes

---

### User Story 3 - Search Resources by Name (Priority: P1)

Platform operators need to quickly find specific resources by name without manually scrolling through the entire timeline.

**Why this priority**: Essential usability feature for large deployments with many resources.

**Independent Test**: User can type in a search box and the timeline filters to show only matching resource names in real-time.

**Acceptance Scenarios**:

1. **Given** the UI is loaded, **When** user looks at the top bar, **Then** a search input field is visible
2. **Given** the search field is focused, **When** user types characters, **Then** timeline updates in real-time to show only resources matching the search term
3. **Given** search results are displayed, **When** user clears the search field, **Then** timeline shows all resources again
4. **Given** search is active, **When** no resources match, **Then** timeline shows empty state with appropriate message

---

### User Story 4 - Inspect Selected Segment Details (Priority: P1)

Platform operators need to see detailed information about a specific resource state change by selecting a timeline segment to understand what changed and why.

**Why this priority**: Critical for troubleshooting - operators must be able to examine specific state changes in detail.

**Independent Test**: User can click a timeline segment and a detail panel appears showing status, time range, and relevant audit events.

**Acceptance Scenarios**:

1. **Given** timeline is displayed, **When** user clicks a resource segment, **Then** segment highlights with white border
2. **Given** segment is selected, **When** detail panel slides in from right, **Then** panel shows resource name, kind, namespace
3. **Given** detail panel is open, **When** user views status section, **Then** color-coded status, start time, end time, and message are visible
4. **Given** detail panel shows a segment, **When** user looks at audit events, **Then** only events within the segment's time window are listed
5. **Given** detail panel is open, **When** user clicks close button, **Then** panel slides out and timeline returns to normal
6. **Given** segment is selected, **When** user presses Escape, **Then** detail panel closes

---

### User Story 5 - Compare Configuration Changes (Priority: P2)

Platform operators need to understand what configuration changed between two resource states by seeing a diff of the JSON configuration.

**Why this priority**: Valuable for troubleshooting configuration drift and understanding the impact of changes.

**Independent Test**: User can select a segment and see a diff showing added (green), removed (red), and modified (yellow) configuration changes compared to the previous state.

**Acceptance Scenarios**:

1. **Given** detail panel is open for a segment, **When** user views configuration diff section, **Then** changes are highlighted with colors (Added=Green, Removed=Red, Modified=Yellow)
2. **Given** a segment has a previous state, **When** user views the diff, **Then** the comparison shows configuration changes between consecutive states
3. **Given** a segment is the initial state, **When** user views the diff, **Then** an appropriate message indicates this is the first state with no previous version

---

### User Story 6 - Navigate Resource History with Keyboard (Priority: P2)

Platform operators need to efficiently navigate between historical segments of a resource using keyboard shortcuts to inspect changes sequentially.

**Why this priority**: Improves workflow efficiency for operators analyzing multiple state changes.

**Independent Test**: User can press arrow keys to move between segments of the selected resource and detail panel updates accordingly.

**Acceptance Scenarios**:

1. **Given** a resource segment is selected, **When** user presses Right Arrow, **Then** the next historical segment of the same resource is selected
2. **Given** a resource segment is selected, **When** user presses Left Arrow, **Then** the previous historical segment of the same resource is selected
3. **Given** the first segment is selected, **When** user presses Left Arrow, **Then** no navigation occurs (boundary condition)
4. **Given** the last segment is selected, **When** user presses Right Arrow, **Then** no navigation occurs (boundary condition)
5. **Given** the first segment is selected, **When** user presses Right Arrow, **Then** detail panel updates to show next segment

---

### User Story 7 - Interactive Timeline Navigation (Priority: P2)

Platform operators need to zoom and pan across the timeline to view different time ranges and densities of resource changes.

**Why this priority**: Enables exploration of data across various time scales and focuses on specific time periods of interest.

**Independent Test**: User can drag on the timeline to zoom horizontally, use Shift+Scroll to pan, and see the view adjust accordingly.

**Acceptance Scenarios**:

1. **Given** timeline is displayed, **When** user drags horizontally on canvas, **Then** timeline zooms in/out in time dimension
2. **Given** zoomed timeline, **When** user holds Shift and scrolls horizontally, **Then** timeline pans left/right
3. **Given** timeline is displayed, **When** user scrolls vertically, **Then** resource list scrolls to reveal more resources
4. **Given** segment is selected, **When** user performs any navigation action, **Then** timeline auto-centers selected segment in viewport when possible
5. **Given** selected segment is wider than viewport, **When** timeline attempts to fit it, **Then** view automatically zooms out with padding around segment

---

### User Story 8 - View Branding and Navigation (Priority: P3)

Platform operators should see the "Spectre" branding with ghost icon in the UI to establish application identity and professionalism.

**Why this priority**: Important for UX polish and brand recognition, but not core functionality.

**Independent Test**: UI displays "Spectre" branding with ghost icon in the top bar.

**Acceptance Scenarios**:

1. **Given** the UI is loaded, **When** user looks at the top left, **Then** "Spectre" branding with ghost icon is visible

### Edge Cases

- What happens when no audit data exists for the selected filters?
- How does the system handle resources with extremely long time spans (days/months)?
- How does the system handle rapid state changes (many segments in short time)?
- What happens when a user selects multiple conflicting filters that result in no data?
- How should the UI handle timezone differences in audit timestamps?
- What happens when configuration diff is too large to render efficiently?

## Requirements *(mandatory)*

<!--
  ACTION REQUIRED: The content in this section represents placeholders.
  Fill them out with the right functional requirements.
-->

### Functional Requirements

- **FR-001**: System MUST fetch available namespaces, resource kinds, and resource counts from the backend on initial page load
- **FR-002**: System MUST display an interactive timeline with resources as rows and time on the X-axis
- **FR-003**: Each timeline segment MUST be color-coded based on resource status (Green=Ready, Yellow=Warning, Red=Error, Gray=Terminating/Unknown)
- **FR-004**: System MUST display small dots on timeline segments representing discrete audit events (create, patch, delete, etc.)
- **FR-005**: Users MUST be able to select one or multiple namespaces from a dropdown filter
- **FR-006**: Users MUST be able to select one or multiple resource kinds from a dropdown filter
- **FR-007**: System MUST support real-time text search to filter resources by name
- **FR-008**: Filters (namespaces, kinds, search) MUST update the timeline in real-time as user changes selections
- **FR-009**: Users MUST be able to click a timeline segment to select it and open a detail panel
- **FR-010**: The detail panel MUST display the resource name, kind, namespace, status, start time, end time, and message for the selected segment
- **FR-011**: The detail panel MUST show a configuration diff comparing JSON configuration of current segment against the previous segment
- **FR-012**: Configuration diff MUST highlight additions (green), removals (red), and modifications (yellow)
- **FR-013**: The detail panel MUST list all audit events that occurred strictly within the selected segment's time window
- **FR-014**: Audit events in the detail panel MUST also be highlighted on the main timeline (yellow, larger size)
- **FR-015**: Users MUST be able to close the detail panel via close button or Escape key
- **FR-016**: Users MUST be able to navigate between segments of the selected resource using Left/Right arrow keys
- **FR-017**: Users MUST be able to toggle dropdown items using Enter/Space keys
- **FR-018**: Users MUST be able to navigate dropdown items using Arrow keys
- **FR-019**: Users MUST be able to close dropdowns using Escape key
- **FR-020**: Users MUST be able to zoom timeline horizontally by dragging on the canvas
- **FR-021**: Users MUST be able to pan timeline horizontally using Shift+Scroll
- **FR-022**: Users MUST be able to scroll vertically through the resource list
- **FR-023**: Selected segments MUST be highlighted with a white border
- **FR-024**: When a segment is selected, the timeline MUST auto-center it in the viewport when possible
- **FR-025**: If a selected segment is wider than the viewport, the timeline MUST zoom out automatically to fit with padding
- **FR-026**: The time scale header MUST remain sticky at the top while scrolling resources
- **FR-027**: The UI MUST display "Spectre" branding with ghost icon in the top bar
- **FR-028**: System MUST efficiently handle large datasets (100+ resources, 1000+ segments) without significant performance degradation

### Key Entities

- **Resource**: Represents a Kubernetes resource being audited (Pod, Deployment, StatefulSet, etc.)
  - Attributes: name, kind, namespace, creation time, termination time
  - Relationships: has multiple segments across time, generates audit events

- **Segment**: Represents a continuous period during which a resource maintained the same status
  - Attributes: status (Ready/Warning/Error/Terminating/Unknown), start time, end time, message
  - Relationships: belongs to one resource, has associated audit events, has previous/next segments

- **Audit Event**: Represents a discrete action performed on or affecting a resource
  - Attributes: type (create, patch, delete, etc.), timestamp, user, details/changes
  - Relationships: occurs within a segment, appears as dot on timeline

- **Namespace**: Kubernetes namespace grouping resources
  - Attributes: name
  - Relationships: contains multiple resources

- **Kind**: Kubernetes resource type
  - Attributes: name, group, version
  - Relationships: categorizes multiple resources

## Success Criteria *(mandatory)*

<!--
  ACTION REQUIRED: Define measurable success criteria.
  These must be technology-agnostic and measurable.
-->

### Measurable Outcomes

- **SC-001**: Users can load the UI and see timeline visualization within 3 seconds for datasets with up to 500 resources
- **SC-002**: Users can apply filters (namespace, kind, search) and see timeline updates in real-time with under 500ms response time
- **SC-003**: Users can select a segment and see detail panel appear within 200ms with all information (status, diff, events) populated
- **SC-004**: Timeline supports interactive navigation (zoom, pan, scroll) with smooth 60 FPS animations during interaction
- **SC-005**: System successfully displays 100+ resources and 1000+ timeline segments without visual lag or performance degradation
- **SC-006**: All keyboard shortcuts (arrow keys, enter, space, escape) work correctly and are discoverable to users
- **SC-007**: Configuration diff renders correctly for changes up to 10MB JSON payloads within 1 second
- **SC-008**: 95% of user filter operations result in updated timeline view within 500ms
- **SC-009**: Selected segment auto-centering works correctly in 95% of cases across various viewport sizes
- **SC-010**: Audit event highlighting (yellow dots on timeline) visually matches and highlights listed events in detail panel with 100% accuracy

## Assumptions

- Audit data is already collected and stored in a persistent backend
- Backend API exists at `/internal/api` and can be extended with new endpoints for metadata
- The application is built with TypeScript/React (based on existing ui/ directory structure)
- D3.js is available for timeline visualization
- Modern browser features (flexbox, CSS grid, ES2020+) can be used
- Users have basic familiarity with Kubernetes concepts (namespaces, kinds, resources)
- Configuration diffs are meaningful only between consecutive segments of the same resource
- Timezone handling uses the user's local browser timezone (no explicit timezone picker needed)
