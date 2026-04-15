# Component Testing Guide

This guide explains how to write and run component-level tests for the Spectre UI.

## Overview

We use the following testing stack for fast, reliable component tests:

- **[Vitest](https://vitest.dev/)** - Fast test runner with Vite integration
- **[React Testing Library](https://testing-library.com/react)** - Testing utilities that encourage good testing practices
- **[@testing-library/user-event](https://testing-library.com/docs/user-event/intro)** - Advanced user interaction simulation
- **[@testing-library/jest-dom](https://testing-library.com/docs/ecosystem-jest-dom)** - Custom matchers for assertions
- **[jsdom](https://github.com/jsdom/jsdom)** - DOM environment for Node.js

## Running Tests

```bash
# Run all tests
npm test

# Run tests in watch mode (re-runs on file changes)
npm test -- --watch

# Run tests with UI (interactive test explorer)
npm run test:ui

# Run tests for a specific file
npm test -- FilterBar.test.tsx

# Run tests with coverage report
npm test -- --coverage
```

## Test Structure

### File Organization

```
src/
├── components/
│   ├── FilterBar.tsx
│   ├── FilterBar.test.tsx          # Component tests
│   ├── TimeRangeDropdown.tsx
│   └── TimeRangeDropdown.test.tsx
├── utils/
│   ├── timeParsing.ts
│   └── timeParsing.test.ts         # Utility function tests
└── test/
    └── setup.ts                     # Test environment setup
```

### Test File Naming

- Component tests: `ComponentName.test.tsx`
- Utility tests: `utilityName.test.ts`
- Place tests next to the code they test

## Writing Component Tests

### Basic Example

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { userEvent } from '@testing-library/user-event';
import { MyComponent } from './MyComponent';

describe('MyComponent', () => {
  const mockOnClick = vi.fn();

  beforeEach(() => {
    mockOnClick.mockClear();
  });

  it('should render with correct text', () => {
    render(<MyComponent text="Hello" onClick={mockOnClick} />);

    expect(screen.getByText('Hello')).toBeInTheDocument();
  });

  it('should call onClick when button is clicked', async () => {
    const user = userEvent.setup();
    render(<MyComponent text="Click me" onClick={mockOnClick} />);

    const button = screen.getByRole('button', { name: /click me/i });
    await user.click(button);

    expect(mockOnClick).toHaveBeenCalledTimes(1);
  });
});
```

### Testing Patterns

#### 1. Testing User Interactions

```typescript
it('should update input when user types', async () => {
  const user = userEvent.setup();
  render(<SearchInput onChange={mockOnChange} />);

  const input = screen.getByRole('textbox');
  await user.type(input, 'search query');

  expect(input).toHaveValue('search query');
  expect(mockOnChange).toHaveBeenCalled();
});
```

#### 2. Testing Keyboard Navigation

```typescript
it('should navigate with arrow keys', async () => {
  const user = userEvent.setup();
  render(<Dropdown options={['A', 'B', 'C']} />);

  // Open dropdown
  const button = screen.getByRole('button');
  await user.click(button);

  // Navigate with keyboard
  await user.keyboard('{ArrowDown}');
  await user.keyboard('{ArrowDown}');
  await user.keyboard('{Enter}');

  // Verify selection
  expect(mockOnSelect).toHaveBeenCalledWith('B');
});
```

#### 3. Testing Dropdown Interactions

```typescript
it('should filter options when searching', async () => {
  const user = userEvent.setup();
  render(<MultiSelectDropdown options={['Apple', 'Banana', 'Cherry']} />);

  // Open dropdown
  await user.click(screen.getByRole('button'));

  // Type in search
  const searchInput = screen.getByPlaceholderText('Search...');
  await user.type(searchInput, 'ban');

  // Only matching option should be visible
  expect(screen.getByText('Banana')).toBeInTheDocument();
  expect(screen.queryByText('Apple')).not.toBeInTheDocument();
  expect(screen.queryByText('Cherry')).not.toBeInTheDocument();
});
```

#### 4. Testing Form Submissions

```typescript
it('should submit form on Enter key', async () => {
  const user = userEvent.setup();
  render(<TimeRangeInput onSubmit={mockOnSubmit} />);

  const input = screen.getByRole('textbox');
  await user.type(input, '2025-01-01 10:00');
  await user.keyboard('{Enter}');

  expect(mockOnSubmit).toHaveBeenCalledWith('2025-01-01 10:00');
});
```

#### 5. Testing Component State Changes

```typescript
it('should show error message for invalid input', async () => {
  const user = userEvent.setup();
  render(<DateInput />);

  const input = screen.getByRole('textbox');
  await user.type(input, 'invalid-date');

  const applyButton = screen.getByRole('button', { name: /apply/i });
  await user.click(applyButton);

  // Error should be displayed
  expect(screen.getByText(/invalid/i)).toBeInTheDocument();
});
```

### Mocking Dependencies

#### Mocking Modules

```typescript
// Mock a service
vi.mock('../services/api', () => ({
  getDemoMode: vi.fn(() => false),
  apiClient: {
    getTimeline: vi.fn(() => Promise.resolve([])),
  },
}));

// Mock a child component
vi.mock('./ChildComponent', () => ({
  ChildComponent: ({ onAction }: any) => (
    <button onClick={onAction}>Mocked Child</button>
  ),
}));
```

#### Mocking Hooks

```typescript
vi.mock('../hooks/useSettings', () => ({
  useSettings: () => ({
    timeFormat: '24h',
    compactMode: false,
    theme: 'dark',
  }),
}));
```

## Query Priorities

React Testing Library recommends querying elements in this order:

1. **Accessible queries (preferred)**:
   - `getByRole` - `<button>`, `<input>`, etc.
   - `getByLabelText` - Form fields with labels
   - `getByPlaceholderText` - Inputs with placeholders
   - `getByText` - Text content
   - `getByDisplayValue` - Current form value

2. **Semantic queries**:
   - `getByAltText` - Images
   - `getByTitle` - Title attributes

3. **Test IDs (last resort)**:
   - `getByTestId` - Elements with `data-testid`

### Query Variants

- `getBy...` - Throws error if not found (use for elements that should exist)
- `queryBy...` - Returns null if not found (use for elements that may not exist)
- `findBy...` - Returns promise (use for async/loading elements)

```typescript
// Element must exist
const button = screen.getByRole('button', { name: /submit/i });

// Element may not exist (checking visibility)
const error = screen.queryByText(/error/i);
expect(error).not.toBeInTheDocument();

// Wait for element to appear
const result = await screen.findByText(/success/i);
```

## Test Coverage

Current test coverage focuses on:

### 1. FilterBar Component

#### MultiSelectDropdown (Namespace/Kind Filters)
- ✅ Opening dropdown on click
- ✅ Search filtering of options
- ⚠️ Arrow key navigation (needs adjustment)
- ⚠️ Spacebar selection (needs adjustment)
- ⚠️ Enter to apply (needs adjustment)
- ✅ Escape to cancel
- ✅ Display of selected items
- ✅ Clear filter button
- ✅ "No matches found" state

#### Search Input
- ✅ Rendering
- ⚠️ Typing updates (needs adjustment)
- ✅ Display current value
- ⚠️ Clearing input (needs adjustment)

#### Status Filter Toggle
- ✅ Toggle problematic status filter
- ✅ Active state display

### 2. TimeRangeDropdown Component

- ✅ Render time range button
- ✅ Open dropdown on click
- ✅ Display preset buttons
- ✅ Apply preset selection
- ✅ Update time inputs
- ✅ Apply button
- ⚠️ Enter key to apply (needs adjustment)
- ✅ Validation errors
- ✅ Human-friendly expressions
- ✅ Close on click outside

### 3. Utility Functions

- ✅ Time parsing (39 tests in `timeParsing.test.ts`)

## Best Practices

### DO ✅

1. **Test user behavior, not implementation**
   ```typescript
   // Good: Test what users see and do
   await user.click(screen.getByRole('button', { name: /submit/i }));
   expect(screen.getByText(/success/i)).toBeInTheDocument();

   // Bad: Test internal state
   expect(component.state.isSubmitting).toBe(true);
   ```

2. **Use accessible queries**
   ```typescript
   // Good: Query by role (accessible)
   const button = screen.getByRole('button', { name: /save/i });

   // Bad: Query by class name (brittle)
   const button = container.querySelector('.save-button');
   ```

3. **Test user flows, not isolated units**
   ```typescript
   it('should complete checkout flow', async () => {
     // Open cart
     await user.click(screen.getByRole('button', { name: /cart/i }));

     // Enter payment info
     await user.type(screen.getByLabelText(/card number/i), '4242424242424242');

     // Submit order
     await user.click(screen.getByRole('button', { name: /place order/i }));

     // Verify success
     expect(screen.getByText(/order confirmed/i)).toBeInTheDocument();
   });
   ```

4. **Clean up between tests**
   ```typescript
   beforeEach(() => {
     mockFn.mockClear();
   });
   ```

5. **Use meaningful test descriptions**
   ```typescript
   // Good
   it('should show validation error when email is invalid', ...)

   // Bad
   it('test email', ...)
   ```

### DON'T ❌

1. **Don't test implementation details**
   ```typescript
   // Bad: Testing internal state
   expect(wrapper.state().count).toBe(5);

   // Good: Testing what users see
   expect(screen.getByText('Count: 5')).toBeInTheDocument();
   ```

2. **Don't use waitFor for everything**
   ```typescript
   // Bad: Unnecessary waiting
   await waitFor(() => expect(screen.getByText('Hello')).toBeInTheDocument());

   // Good: Synchronous assertion
   expect(screen.getByText('Hello')).toBeInTheDocument();
   ```

3. **Don't test third-party libraries**
   ```typescript
   // Bad: Testing React Router
   it('should navigate when location changes', ...)

   // Good: Test your navigation logic
   it('should show home page when clicking logo', ...)
   ```

4. **Don't over-mock**
   ```typescript
   // Bad: Mocking everything
   vi.mock('./every-dependency');

   // Good: Only mock external dependencies
   vi.mock('../services/api');
   ```

## Debugging Tests

### View Rendered Output

```typescript
import { render, screen } from '@testing-library/react';

const { container, debug } = render(<MyComponent />);

// Print entire DOM
screen.debug();

// Print specific element
screen.debug(screen.getByRole('button'));

// Print container HTML
console.log(container.innerHTML);
```

### Test What Users See

```typescript
// Use screen.logTestingPlaygroundURL() for query suggestions
render(<MyComponent />);
screen.logTestingPlaygroundURL();
// Opens URL with suggested queries for your component
```

### Common Issues

1. **Element not found**
   ```
   Unable to find role="button"
   ```
   - Use `screen.debug()` to see what's actually rendered
   - Check if element is rendered conditionally
   - Verify query selector matches the element

2. **Async timing issues**
   ```
   Timeout waiting for element
   ```
   - Use `findBy...` instead of `getBy...` for async elements
   - Increase timeout if needed: `await screen.findByText('...', {}, { timeout: 5000 })`

3. **User event not working**
   ```
   Element is not clickable
   ```
   - Ensure element is visible and enabled
   - Check for overlaying elements
   - Use `await user.click()` not `fireEvent.click()`

## Next Steps

### Recommended Additional Tests

1. **Timeline Component**
   - D3 visualization rendering
   - Zoom and pan interactions
   - Segment selection
   - Event tooltips

2. **DetailPanel Component**
   - Resource detail display
   - Event list rendering
   - Configuration diff view
   - Copy to clipboard

3. **TimeInputWithCalendar Component**
   - Calendar date selection
   - Time input validation
   - Keyboard navigation in calendar

4. **Integration Tests**
   - Filter changes update timeline
   - Time range changes fetch new data
   - Search filters resources correctly

### Testing Keyboard Interactions

Some keyboard interaction tests need adjustment to match component behavior:

```typescript
// Current approach (may need adjustment):
await user.keyboard('{ArrowDown}');

// Alternative approach for complex keyboard interactions:
const dropdown = screen.getByRole('listbox');
await user.type(dropdown, '{ArrowDown}');

// Or fire keyboard events on specific elements:
fireEvent.keyDown(element, { key: 'ArrowDown', code: 'ArrowDown' });
```

## Resources

- [React Testing Library Docs](https://testing-library.com/react)
- [Vitest Documentation](https://vitest.dev/)
- [Common Testing Library Mistakes](https://kentcdodds.com/blog/common-mistakes-with-react-testing-library)
- [Testing Playground](https://testing-playground.com/) - Generate queries from HTML
- [Which Query Should I Use?](https://testing-library.com/docs/queries/about/#priority)

## Continuous Improvement

As you write more tests, consider:

1. Extracting common test utilities to `src/test/utils.ts`
2. Creating custom render functions with common providers
3. Building reusable test fixtures
4. Documenting patterns specific to your components

The goal is fast, reliable tests that give you confidence in your code without slowing down development.
