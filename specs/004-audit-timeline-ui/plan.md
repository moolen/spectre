# Implementation Plan: Audit Timeline UI

**Branch**: `004-audit-timeline-ui` | **Date**: 2025-11-25 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/004-audit-timeline-ui/spec.md`

## Summary

Build an interactive web-based audit data visualization dashboard that displays Kubernetes resource state changes over time using a D3.js-based Gantt-style timeline. The UI provides multi-dimensional filtering (namespace, kind, search), segment-level detail inspection with configuration diffs, and keyboard-accessible controls for efficient operator workflows. Integrates with backend API at `/internal/api` to fetch metadata and audit event data.

## Technical Context

**Language/Version**: TypeScript 5.x with React 18+
**Primary Dependencies**:
- React 18+ (UI framework)
- D3.js 7.x (timeline visualization)
- TypeScript 5.x (type safety)
- React Query or similar (data fetching & caching)
- CSS-in-JS solution (Tailwind CSS or styled-components)
- Vite (build tool - evident from existing ui/vite.config.ts)

**Storage**: N/A (frontend only - data sourced from backend API at `/internal/api`)
**Testing**: Vitest + React Testing Library (unit/integration), Cypress/Playwright (E2E)
**Target Platform**: Modern browsers (Chrome, Firefox, Safari, Edge - ES2020+ support required)
**Project Type**: Web application (single-page application)
**Performance Goals**:
- 3-second initial load for 500 resources
- <500ms filter response time
- 200ms detail panel appearance
- 60 FPS smooth interactions
- 95% filter operation success within 500ms

**Constraints**:
- Real-time responsiveness (<500ms for filter updates)
- Large dataset handling (100+ resources, 1000+ segments)
- No external dependencies for core visualization
- Accessible keyboard navigation (WCAG 2.1 AA)

**Scale/Scope**:
- 100+ resources displayed simultaneously
- 1000+ timeline segments
- 5-7 major UI components
- 20+ user interactions
- 10 success criteria with measurable targets

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Constitution Status**: No constitution file detected. Proceeding with standard web application best practices:
- Component-based architecture with clear separation of concerns
- Type-safe development with TypeScript
- Unit and integration testing for critical paths
- E2E testing for user workflows
- Accessibility compliance (WCAG 2.1 AA minimum)
- Performance budgets and monitoring
- API contract testing with backend

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
ui/
├── src/
│   ├── components/
│   │   ├── Timeline/
│   │   │   ├── Timeline.tsx          # Main D3-based timeline component
│   │   │   ├── Timeline.module.css   # Timeline styles
│   │   │   └── TimelineTooltip.tsx   # Hover/selection tooltips
│   │   ├── Sidebar/
│   │   │   ├── DetailPanel.tsx       # Right-side detail view
│   │   │   ├── ConfigDiff.tsx        # JSON diff viewer
│   │   │   └── EventList.tsx         # Audit events list
│   │   ├── TopBar/
│   │   │   ├── TopBar.tsx            # Navigation & branding
│   │   │   ├── SearchInput.tsx       # Real-time search box
│   │   │   ├── FilterDropdown.tsx    # Multi-select dropdown
│   │   │   └── Branding.tsx          # Spectre logo & icon
│   │   └── Common/
│   │       ├── Loading.tsx
│   │       ├── ErrorBoundary.tsx
│   │       └── EmptyState.tsx
│   ├── pages/
│   │   └── Dashboard.tsx             # Main dashboard page
│   ├── services/
│   │   ├── api.ts                    # Backend API client
│   │   ├── dataTransform.ts          # Resource/segment data transformation
│   │   ├── timelineUtils.ts          # D3 and timeline calculations
│   │   └── diffCalculator.ts         # JSON diff logic
│   ├── types/
│   │   ├── Resource.ts
│   │   ├── Segment.ts
│   │   ├── AuditEvent.ts
│   │   ├── Namespace.ts
│   │   └── ApiResponse.ts
│   ├── hooks/
│   │   ├── useTimeline.ts            # Timeline state & logic
│   │   ├── useFilters.ts             # Filter state management
│   │   ├── useSelection.ts           # Segment selection logic
│   │   └── useKeyboard.ts            # Keyboard shortcut handling
│   ├── constants.ts                  # Color mappings, status types
│   ├── App.tsx                       # Root component
│   └── index.tsx                     # Entry point
├── tests/
│   ├── unit/
│   │   ├── services/
│   │   ├── hooks/
│   │   └── utils/
│   ├── integration/
│   │   ├── Timeline.integration.test.tsx
│   │   └── Filters.integration.test.tsx
│   └── e2e/
│       ├── timeline.spec.ts
│       ├── filtering.spec.ts
│       └── keyboard.spec.ts
├── package.json
├── tsconfig.json
├── vite.config.ts
└── README.md
```

**Structure Decision**: Selected web application structure (Option 2) with the existing `ui/` directory as the sole frontend. The component structure reflects the three main UI regions (TopBar filtering, Timeline canvas, DetailPanel sidebar) plus shared utilities for API communication, D3 visualization, and state management. This aligns with React best practices and the existing ui/ directory structure.

## Complexity Tracking

No violations to standard web application practices. Component-based structure aligns with React best practices and scalability requirements for timeline visualization and interactive filtering.

## Phase 0: Research (To Be Generated)

Research phase will address:
- D3.js timeline implementation patterns for large datasets (1000+ segments)
- JSON diff visualization libraries or custom implementation
- Keyboard accessibility patterns for dropdown and timeline navigation
- Performance optimization techniques for Gantt-style visualizations
- State management approach (Context API vs. external library)

## Phase 1: Design Deliverables (To Be Generated)

Phase 1 will produce:
1. **data-model.md** - Entity definitions with validation rules
2. **contracts/** - API contract specifications
3. **quickstart.md** - Development setup guide
4. Agent context updated with TypeScript/React/D3.js technologies
