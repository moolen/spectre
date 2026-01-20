# Testing Patterns

**Analysis Date:** 2026-01-20

## Test Framework

**Runner:**
- Vitest 4.0.16
- Config: `/home/moritz/dev/spectre-via-ssh/ui/vitest.config.ts`

**Assertion Library:**
- Vitest built-in assertions (extended with @testing-library/jest-dom matchers)

**Run Commands:**
```bash
npm run test              # Run all tests once
npm run test:watch        # Watch mode for development
npm run test:ct           # Run Playwright component tests
npm run test:ct:ui        # Playwright component tests with UI
```

**Coverage:**
```bash
# Coverage configured in vitest.config.ts
# Provider: v8
# Reporters: text, json, html
# Excludes: node_modules/, dist/, **/*.d.ts, src/test/**
```

## Test File Organization

**Location:**
- Unit tests: Co-located with source files
  - `/home/moritz/dev/spectre-via-ssh/ui/src/utils/timeParsing.test.ts`
  - `/home/moritz/dev/spectre-via-ssh/ui/src/components/TimeRangeDropdown.test.tsx`
  - `/home/moritz/dev/spectre-via-ssh/ui/src/components/FilterBar.test.tsx`
- Component tests (Playwright): Separate directory
  - `/home/moritz/dev/spectre-via-ssh/ui/playwright/tests/layout-behavior.spec.tsx`

**Naming:**
- Unit tests: `*.test.ts` or `*.test.tsx`
- Playwright component tests: `*.spec.tsx`
- Test file mirrors source file name: `timeParsing.ts` → `timeParsing.test.ts`

**Structure:**
```
ui/src/
├── utils/
│   ├── timeParsing.ts
│   └── timeParsing.test.ts        # Co-located unit test
├── components/
│   ├── FilterBar.tsx
│   └── FilterBar.test.tsx         # Co-located component test
└── test/
    └── setup.ts                   # Global test setup

ui/playwright/
└── tests/
    └── layout-behavior.spec.tsx   # E2E-style component tests
```

## Test Structure

**Suite Organization:**
```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { userEvent } from '@testing-library/user-event';

describe('ComponentName', () => {
  const mockCallback = vi.fn();

  const defaultProps = {
    // ... props
  };

  beforeEach(() => {
    mockCallback.mockClear();
  });

  it('should describe expected behavior', () => {
    // Arrange
    render(<Component {...defaultProps} />);

    // Act
    const button = screen.getByRole('button');

    // Assert
    expect(button).toBeInTheDocument();
  });
});
```

**Patterns:**
- `describe` blocks for component/function grouping
- Nested `describe` blocks for feature grouping (e.g., "MultiSelectDropdown (Namespace Filter)")
- `it` blocks for individual test cases
- `beforeEach` for test isolation
- AAA pattern: Arrange, Act, Assert (implicit in test body)

**Setup/Teardown:**
- Global setup: `/home/moritz/dev/spectre-via-ssh/ui/src/test/setup.ts`
  - Extends Vitest expect with jest-dom matchers
  - Cleanup after each test with `@testing-library/react`
  - Mocks browser APIs: `window.matchMedia`, `IntersectionObserver`, `ResizeObserver`
- Per-test setup: `beforeEach` hooks
- No explicit teardown needed (automatic cleanup)

**Assertion Pattern:**
```typescript
// DOM presence
expect(element).toBeInTheDocument();
expect(element).not.toBeInTheDocument();

// Text content
expect(button.textContent).toContain('Expected Text');
expect(button).toHaveTextContent('Exact Text');

// CSS classes
expect(element).toHaveClass('className');

// Input values
expect(input).toHaveValue('value');

// Function calls
expect(mockFn).toHaveBeenCalled();
expect(mockFn).toHaveBeenCalledTimes(3);
expect(mockFn).toHaveBeenCalledWith(expectedArgs);

// Type checks
expect(result).toBeInstanceOf(Date);
expect(result).toBeNull();

// Comparisons
expect(value).toBe(expected);
expect(value).toEqual(expected); // Deep equality
```

## Mocking

**Framework:** Vitest `vi` module

**Patterns:**

**Mocking child components:**
```typescript
vi.mock('./TimeInputWithCalendar', () => ({
  TimeInputWithCalendar: ({ value, onChange, onEnter, label }: any) => (
    <input
      data-testid={`time-input-${label || value}`}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' && onEnter) {
          e.preventDefault();
          onEnter();
        }
      }}
      placeholder="Time input"
      aria-label={label}
    />
  ),
}));
```

**Mocking hooks:**
```typescript
vi.mock('../hooks/useSettings', () => ({
  useSettings: () => ({ timeFormat: '24h' }),
}));

vi.mock('../hooks/usePersistedQuickPreset', () => ({
  usePersistedQuickPreset: () => ({ preset: null, savePreset: vi.fn() }),
}));
```

**Mocking functions:**
```typescript
const mockOnConfirm = vi.fn();

beforeEach(() => {
  mockOnConfirm.mockClear();
});

// Later in test:
expect(mockOnConfirm).toHaveBeenCalled();
const [arg1, arg2] = mockOnConfirm.mock.calls[0];
```

**What to Mock:**
- Child components not under test (reduce complexity)
- External dependencies (API clients, browser APIs)
- Custom hooks when testing components
- Third-party libraries that don't work in test environment

**What NOT to Mock:**
- The component being tested
- Simple utilities (test them directly)
- React itself
- Testing library utilities

## Fixtures and Factories

**Test Data:**
```typescript
// Inline fixtures
const defaultProps = {
  currentRange: {
    start: new Date('2025-01-01T10:00:00Z'),
    end: new Date('2025-01-01T11:00:00Z'),
  },
  onConfirm: mockOnConfirm,
};

// Fixed dates for time-based tests
const fixedNow = new Date('2025-12-02T13:00:00Z');

// Variation with spread
const propsWithSelection = {
  ...defaultProps,
  filters: {
    ...defaultProps.filters,
    namespaces: ['default', 'production'],
  },
};
```

**Location:**
- Fixtures defined inline in test files (no separate fixture directory)
- Constants at top of `describe` block
- Shared fixtures reused via spread operator

## Coverage

**Requirements:** No enforced coverage threshold

**View Coverage:**
```bash
npm run test             # Runs with coverage
# Opens: coverage/index.html
```

**Exclusions:**
- `node_modules/`
- `dist/`
- `**/*.d.ts` (type definitions)
- `src/test/**` (test utilities)
- Generated code (protobuf)

## Test Types

**Unit Tests:**
- Scope: Individual functions and utilities
- Location: `/home/moritz/dev/spectre-via-ssh/ui/src/utils/timeParsing.test.ts`
- Approach: Pure function testing with various inputs
- Example: `parseTimeExpression('2h ago', fixedNow)` returns expected Date

**Component Tests (Vitest):**
- Scope: React components with React Testing Library
- Location: `/home/moritz/dev/spectre-via-ssh/ui/src/components/FilterBar.test.tsx`
- Approach: Render component, simulate user interactions, assert DOM state
- Libraries: `@testing-library/react`, `@testing-library/user-event`
- Example tests:
  - User interactions (clicking, typing)
  - Conditional rendering
  - Prop changes
  - Callback invocations

**Component Tests (Playwright):**
- Scope: Layout behavior and visual tests in real browser
- Location: `/home/moritz/dev/spectre-via-ssh/ui/playwright/tests/layout-behavior.spec.tsx`
- Config: `/home/moritz/dev/spectre-via-ssh/ui/playwright-ct.config.ts`
- Approach: Mount React components in Chromium, test CSS, layout, animations
- Example tests:
  - Sidebar expansion CSS transitions
  - Scroll behavior
  - ResizeObserver behavior
  - CSS measurements (`toHaveCSS('margin-left', '64px')`)

**Integration Tests:**
- Scope: Component + hook interactions
- Location: Component test files
- Approach: Test component with real hooks (not mocked)
- Example: `FilterBar` with `useFilters` hook

**E2E Tests:**
- Framework: Not used (Playwright used for component testing only)

## Common Patterns

**Async Testing:**
```typescript
it('should handle async operations', async () => {
  const user = userEvent.setup();
  render(<Component />);

  const button = screen.getByRole('button');
  await user.click(button);

  // Wait for async state update
  expect(await screen.findByText('Success')).toBeInTheDocument();
});
```

**User Event Testing:**
```typescript
import { userEvent } from '@testing-library/user-event';

it('should handle user input', async () => {
  const user = userEvent.setup();
  render(<Component />);

  const input = screen.getByPlaceholderText('Search...');
  await user.type(input, 'query');
  await user.keyboard('{Enter}');

  expect(mockCallback).toHaveBeenCalled();
});
```

**Error Testing:**
```typescript
it('should show validation error for invalid input', async () => {
  const user = userEvent.setup();
  render(<Component />);

  const input = screen.getByLabelText('Start Time');
  await user.clear(input);
  await user.type(input, 'invalid-date{Enter}');

  // Error message should be displayed
  expect(screen.getByText(/start|end|parse|invalid/i)).toBeInTheDocument();

  // Callback should NOT be called
  expect(mockOnConfirm).not.toHaveBeenCalled();
});
```

**State Update Testing:**
```typescript
it('should update state correctly', async () => {
  const user = userEvent.setup();

  // Mock that captures state updates
  let currentFilters = { search: 'nginx' };
  const mockSetFilters = vi.fn((updater) => {
    if (typeof updater === 'function') {
      currentFilters = updater(currentFilters);
    } else {
      currentFilters = updater;
    }
  });

  const { rerender } = render(
    <Component filters={currentFilters} setFilters={mockSetFilters} />
  );

  const input = screen.getByPlaceholderText(/search/i);
  await user.clear(input);

  expect(mockSetFilters).toHaveBeenCalled();

  // Rerender with updated state
  rerender(<Component filters={currentFilters} setFilters={mockSetFilters} />);
  expect(input).toHaveValue('');
});
```

**Playwright Component Testing:**
```typescript
import { test, expect } from '@playwright/experimental-ct-react';

test('should measure CSS properties', async ({ mount, page }) => {
  await mount(<App />);

  const main = page.locator('main');
  await expect(main).toBeVisible();

  // Verify CSS property
  await expect(main).toHaveCSS('margin-left', '64px');

  // Trigger hover
  const sidebar = page.locator('.sidebar-container');
  await sidebar.hover();
  await page.waitForTimeout(350); // Wait for transition

  // Verify CSS changed
  await expect(main).toHaveCSS('margin-left', '220px');
});
```

**Testing Dropdown/Select Components:**
```typescript
it('should filter options when typing in search box', async () => {
  const user = userEvent.setup();
  render(<Component {...defaultProps} />);

  // Open dropdown
  const button = screen.getByRole('button', { name: /all namespaces/i });
  await user.click(button);

  // Type in search
  const searchInput = screen.getByPlaceholderText('Search...');
  await user.type(searchInput, 'kube');

  // Assert filtered results
  expect(screen.getByText('kube-system')).toBeInTheDocument();
  expect(screen.queryByText('default')).not.toBeInTheDocument();
});
```

## Test Best Practices

**Accessibility Testing:**
- Use `screen.getByRole()` over `querySelector`
- Use `getByLabelText()` for form inputs
- Use `getByPlaceholderText()` as fallback

**Query Priority (from Testing Library):**
1. `getByRole` (preferred)
2. `getByLabelText`
3. `getByPlaceholderText`
4. `getByText`
5. `getByTestId` (last resort)

**Async Queries:**
- `findBy*` for elements that appear asynchronously
- `queryBy*` for elements that may not exist
- `getBy*` for elements that should exist

**Test Independence:**
- Each test should be independent
- Use `beforeEach` to reset mocks
- Don't rely on test execution order

---

*Testing analysis: 2026-01-20*
