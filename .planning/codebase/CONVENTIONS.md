# Coding Conventions

**Analysis Date:** 2026-01-20

## Naming Patterns

**Files:**
- Components: PascalCase - `FilterBar.tsx`, `TimeRangeDropdown.tsx`, `ErrorBoundary.tsx`
- Hooks: camelCase with `use` prefix - `useFilters.ts`, `useSelection.ts`, `useTimeline.ts`
- Services: camelCase - `api.ts`, `geminiService.ts`, `dataTransformer.ts`
- Types: camelCase - `types.ts`, `apiTypes.ts`, `namespaceGraph.ts`
- Utilities: camelCase - `timeParsing.ts`, `toast.ts`, `jsonDiff.ts`
- Test files: `.test.ts` or `.test.tsx` for unit tests, `.spec.tsx` for Playwright component tests
- Pages: PascalCase with `Page` suffix - `TimelinePage.tsx`, `SettingsPage.tsx`

**Functions:**
- Regular functions: camelCase - `parseTimeExpression`, `transformSearchResponse`, `normalizeToSeconds`
- React components: PascalCase - `FilterBar`, `TimeRangeDropdown`, `ErrorBoundary`
- Custom hooks: camelCase with `use` prefix - `useFilters`, `useSelection`, `useTimeline`
- Event handlers: camelCase with `handle` prefix - `handleSearchChange`, `handleNamespacesChange`, `handleReset`

**Variables:**
- Constants: camelCase - `baseUrl`, `defaultProps`, `fixedNow`
- React state: camelCase - `sidebarExpanded`, `filters`, `resources`
- Component props: camelCase - `onTimeRangeChange`, `availableNamespaces`, `setFilters`

**Types:**
- Interfaces: PascalCase - `FilterState`, `K8sResource`, `TimeRange`, `ApiClientConfig`
- Enums: PascalCase - `ResourceStatus`
- Type aliases: PascalCase - `TimelineFilters`, `ApiMetadata`
- Props interfaces: PascalCase with component name + `Props` suffix - `FilterBarProps`, `ErrorBoundaryProps`

## Code Style

**Formatting:**
- Tool: Prettier 3.2.0
- Config: `/home/moritz/dev/spectre-via-ssh/ui/.prettierrc.json`
- Settings:
  - Semi-colons: Required (`"semi": true`)
  - Quotes: Single quotes (`"singleQuote": true`)
  - Trailing commas: ES5 style (`"trailingComma": "es5"`)
  - Print width: 100 characters
  - Tab width: 2 spaces
  - Arrow parens: Always (`"arrowParens": "always"`)

**Linting:**
- Tool: ESLint 8.57.0
- Config: `/home/moritz/dev/spectre-via-ssh/ui/.eslintrc.json`
- Key rules:
  - `react/react-in-jsx-scope`: Off (React 19 auto-import)
  - `react/prop-types`: Off (TypeScript types used)
  - `no-unused-vars`: Warn
  - `no-console`: Off (console.log allowed)
  - `no-undef`: Off (TypeScript handles this)
- Extends: `eslint:recommended`, `plugin:react/recommended`, `plugin:react-hooks/recommended`
- Disable comments used sparingly: Only in generated files (`/home/moritz/dev/spectre-via-ssh/ui/src/generated/timeline.ts`)

## Import Organization

**Order:**
1. External libraries - React, third-party packages
2. Internal modules - Services, hooks, types
3. Relative imports - Components, utilities

**Examples:**
```typescript
// External
import React, { useState, useEffect } from 'react';
import { Routes, Route } from 'react-router-dom';
import { Toaster } from 'sonner';

// Internal services/types
import { K8sResource, FilterState } from '../types';
import { apiClient } from '../services/api';

// Relative components
import TimelinePage from './pages/TimelinePage';
import Sidebar from './components/Sidebar';
```

**Path Aliases:**
- `@/*` maps to `./src/*` (configured in `tsconfig.json` and Vite)
- Usage: Prefer relative imports for nearby files, use `@/` for cross-directory imports

## Error Handling

**Patterns:**
- API errors: Try-catch blocks with structured error messages
- Error extraction:
  ```typescript
  catch (error) {
    if (error instanceof Error) {
      if (error.name === 'AbortError') {
        throw new Error(`Request timeout...`);
      }
      throw error;
    }
    throw new Error('Unknown error occurred');
  }
  ```
- User-facing errors: Use toast notifications via `/home/moritz/dev/spectre-via-ssh/ui/src/utils/toast.ts`
- Component errors: React ErrorBoundary in `/home/moritz/dev/spectre-via-ssh/ui/src/components/Common/ErrorBoundary.tsx`
- Development vs production: Check `process.env.NODE_ENV === 'development'` for detailed error display

**Toast Error Pattern:**
```typescript
import { toast } from '../utils/toast';

// Generic error
toast.error('Failed to load data', error.message);

// API-specific error (auto-categorizes network/timeout errors)
toast.apiError(error, 'Loading timeline');

// Promise-based error
toast.promise(apiCall(), {
  loading: 'Loading...',
  success: 'Success!',
  error: (err) => err.message
});
```

## Logging

**Framework:** Native `console` methods

**Patterns:**
- Development logging: `console.log`, `console.error` allowed
- Error logging: `console.error('Error Boundary caught:', error, errorInfo)`
- Debug logging: `console.log(result, transformed)` in development
- Production: No automatic stripping (errors still logged to console)

## Comments

**When to Comment:**
- File-level JSDoc headers explaining purpose:
  ```typescript
  /**
   * API Client Service
   * Communicates with the backend API at /v1
   */
  ```
- Function-level JSDoc for public APIs:
  ```typescript
  /**
   * Get timeline data using gRPC streaming
   * Returns timeline data in batches for progressive rendering
   */
  async getTimelineGrpc(...) { }
  ```
- Complex logic explanation:
  ```typescript
  // Parse apiVersion to extract group and version
  const [groupVersion, version] = grpcResource.apiVersion.includes('/')
    ? grpcResource.apiVersion.split('/')
    : ['', grpcResource.apiVersion];
  ```
- Test descriptions in comments:
  ```typescript
  /**
   * TimeRangeDropdown Component Tests
   *
   * Tests for the TimeRangeDropdown component focusing on:
   * 1. Date/time input fields with Enter to apply
   * 2. Time picker interactions
   * 3. Preset selections
   */
  ```

**JSDoc/TSDoc:**
- Used for public APIs and exported functions
- Parameter descriptions in complex functions
- Not used for simple getters/setters
- Return type descriptions when non-obvious

## Function Design

**Size:**
- Keep functions focused on single responsibility
- API client methods: 50-150 lines typical
- React components: 50-200 lines typical
- Utility functions: 10-50 lines typical
- Extract complex logic into separate functions

**Parameters:**
- Use interfaces for multiple related parameters:
  ```typescript
  async getTimeline(
    startTime: string | number,
    endTime: string | number,
    filters?: TimelineFilters
  ): Promise<K8sResource[]>
  ```
- Optional parameters at the end
- Use destructuring for component props:
  ```typescript
  export const FilterBar: React.FC<FilterBarProps> = ({
    filters,
    setFilters,
    timeRange,
    onTimeRangeChange
  }) => {
  ```

**Return Values:**
- Explicit return types on public APIs
- Async functions return Promise<T>
- React components return JSX.Element (implicit)
- Utility functions return primitives or structured types
- Early returns for error cases:
  ```typescript
  if (!ai) return "API Key not configured...";
  ```

## Module Design

**Exports:**
- Named exports preferred over default exports for utilities/hooks:
  ```typescript
  export const apiClient = new ApiClient({ ... });
  export { ApiClient };
  ```
- Default exports for React components:
  ```typescript
  export default App;
  ```
- Export interfaces/types alongside implementations
- Re-export from index files where appropriate

**Barrel Files:**
- Not heavily used
- Types consolidated in `/home/moritz/dev/spectre-via-ssh/ui/src/types.ts`
- Components exported individually from their files
- Services have single-file exports

## React-Specific Conventions

**Component Structure:**
1. Imports
2. Type/interface definitions
3. Component function
4. Event handlers (can be inside or outside component)
5. Default export

**Hooks Usage:**
- Custom hooks in `/home/moritz/dev/spectre-via-ssh/ui/src/hooks/`
- Use `useMemo` for expensive computations
- Use `useCallback` for stable function references
- Use `useState` for local state
- Use `useEffect` for side effects

**Props:**
- Always use TypeScript interfaces
- Destructure in function signature
- Optional props with `?` suffix
- Event handlers: `onEventName` pattern

**State Management:**
- Local component state with `useState`
- Context for settings: `/home/moritz/dev/spectre-via-ssh/ui/src/hooks/useSettings.ts`
- Props drilling for simple cases
- Callback props for state updates from children

## TypeScript Usage

**Type Safety:**
- Strict mode enabled (`tsconfig.json`)
- Explicit return types on public APIs
- Interface over type for object shapes
- Enum for fixed sets of values (`ResourceStatus`)
- `any` used sparingly (mostly in generated code or protobuf handling)

**Type Assertions:**
- Used when necessary: `seg.status as any as ResourceStatus`
- Prefer type guards over assertions when possible
- Document why assertion is safe

---

*Convention analysis: 2026-01-20*
