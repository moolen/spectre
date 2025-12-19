/**
 * TimeRangeDropdown Component Tests
 *
 * Tests for the TimeRangeDropdown component focusing on:
 * 1. Date/time input fields with Enter to apply
 * 2. Time picker interactions
 * 3. Preset selections
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { userEvent } from '@testing-library/user-event';
import { TimeRangeDropdown } from './TimeRangeDropdown';

// Mock child components
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

// Mock hooks
vi.mock('../hooks/useSettings', () => ({
  useSettings: () => ({ timeFormat: '24h' }),
}));

vi.mock('../hooks/usePersistedQuickPreset', () => ({
  usePersistedQuickPreset: () => ({ preset: null, savePreset: vi.fn() }),
}));

describe('TimeRangeDropdown', () => {
  const mockOnConfirm = vi.fn();

  const defaultProps = {
    currentRange: {
      start: new Date('2025-01-01T10:00:00Z'),
      end: new Date('2025-01-01T11:00:00Z'),
    },
    onConfirm: mockOnConfirm,
  };

  beforeEach(() => {
    mockOnConfirm.mockClear();
  });

  it('should render the time range button with formatted label', () => {
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    expect(button).toBeInTheDocument();
    // Button should show the time range
    expect(button.textContent).toContain('Jan 1');
  });

  it('should open dropdown when button is clicked', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Dropdown should be visible with time inputs
    const timeInputs = screen.getAllByPlaceholderText('Time input');
    expect(timeInputs).toHaveLength(2); // Start and End inputs
  });

  it('should display preset buttons', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Preset buttons should be visible
    expect(screen.getByText('Last 30min')).toBeInTheDocument();
    expect(screen.getByText('Last 6h')).toBeInTheDocument();
    expect(screen.getByText('Last 24h')).toBeInTheDocument();
    expect(screen.getByText('Last 48h')).toBeInTheDocument();
  });

  it('should apply preset when preset button is clicked', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Click on "Last 30min" preset
    const presetButton = screen.getByText('Last 30min');
    await user.click(presetButton);

    // onConfirm should be called with the preset time range
    expect(mockOnConfirm).toHaveBeenCalled();
    const [range, rawStart, rawEnd] = mockOnConfirm.mock.calls[0];
    expect(rawStart).toBe('now-30m');
    expect(rawEnd).toBe('now');
    expect(range.start).toBeInstanceOf(Date);
    expect(range.end).toBeInstanceOf(Date);
  });

  it('should update start time input when changed', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Find start time input
    const startInput = screen.getByLabelText('Start Time');

    // Clear and type new value
    await user.clear(startInput);
    await user.type(startInput, '2025-01-01 09:00');

    // Input value should be updated
    expect(startInput).toHaveValue('2025-01-01 09:00');
  });

  it('should update end time input when changed', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Find end time input
    const endInput = screen.getByLabelText('End Time');

    // Clear and type new value
    await user.clear(endInput);
    await user.type(endInput, '2025-01-01 12:00');

    // Input value should be updated
    expect(endInput).toHaveValue('2025-01-01 12:00');
  });

  it('should apply changes when Apply button is clicked', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Change start time
    const startInput = screen.getByLabelText('Start Time');
    await user.clear(startInput);
    await user.type(startInput, '2025-01-01 09:00');

    // Click Apply button
    const applyButton = screen.getByRole('button', { name: /apply/i });
    await user.click(applyButton);

    // onConfirm should be called with the new time range
    expect(mockOnConfirm).toHaveBeenCalled();
    const [range, rawStart, rawEnd] = mockOnConfirm.mock.calls[0];
    expect(rawStart).toBe('2025-01-01 09:00');
  });

  it('should apply time range when Enter is pressed in start time input', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Change start time
    const startInput = screen.getByLabelText('Start Time');
    await user.clear(startInput);
    await user.type(startInput, '2025-01-01 09:00');

    // Press Enter
    await user.keyboard('{Enter}');

    // onConfirm should be called
    expect(mockOnConfirm).toHaveBeenCalled();
    const [range, rawStart, rawEnd] = mockOnConfirm.mock.calls[0];
    expect(rawStart).toBe('2025-01-01 09:00');
    expect(range.start).toBeInstanceOf(Date);
    expect(range.end).toBeInstanceOf(Date);
  });

  it('should apply time range when Enter is pressed in end time input', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Change end time
    const endInput = screen.getByLabelText('End Time');
    await user.clear(endInput);
    await user.type(endInput, '2025-01-01 12:00');

    // Press Enter
    await user.keyboard('{Enter}');

    // onConfirm should be called
    expect(mockOnConfirm).toHaveBeenCalled();
    const [range, rawStart, rawEnd] = mockOnConfirm.mock.calls[0];
    expect(rawEnd).toBe('2025-01-01 12:00');
    expect(range.start).toBeInstanceOf(Date);
    expect(range.end).toBeInstanceOf(Date);
  });

  it('should show validation error for invalid time range when applying with Enter', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Set start time after end time (invalid)
    const startInput = screen.getByLabelText('Start Time');
    await user.clear(startInput);
    await user.type(startInput, '2025-01-01 13:00{Enter}'); // After end time

    // Error message should be displayed
    const errorMessage = screen.getByText(/before/i);
    expect(errorMessage).toBeInTheDocument();

    // onConfirm should NOT be called
    expect(mockOnConfirm).not.toHaveBeenCalled();
  });

  it('should show validation error for invalid time range with Apply button', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Set start time after end time (invalid)
    const startInput = screen.getByLabelText('Start Time');
    await user.clear(startInput);
    await user.type(startInput, '2025-01-01 13:00'); // After end time

    // Click Apply
    const applyButton = screen.getByRole('button', { name: /apply/i });
    await user.click(applyButton);

    // Error message should be displayed
    const errorMessage = screen.getByText(/before/i);
    expect(errorMessage).toBeInTheDocument();

    // onConfirm should NOT be called
    expect(mockOnConfirm).not.toHaveBeenCalled();
  });

  it('should show validation error for invalid date format when using Enter', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Enter invalid date format
    const startInput = screen.getByLabelText('Start Time');
    await user.clear(startInput);
    await user.type(startInput, 'invalid-date{Enter}');

    // Error message should be displayed (checking for common error text)
    // The exact error message may vary, so we check for any error-like text
    expect(screen.getByText(/start|end|parse|invalid/i)).toBeInTheDocument();

    // onConfirm should NOT be called
    expect(mockOnConfirm).not.toHaveBeenCalled();
  });

  it('should show validation error for invalid date format with Apply button', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Enter invalid date format
    const startInput = screen.getByLabelText('Start Time');
    await user.clear(startInput);
    await user.type(startInput, 'invalid-date');

    // Click Apply
    const applyButton = screen.getByRole('button', { name: /apply/i });
    await user.click(applyButton);

    // Error message should be displayed (checking for common error text)
    // The exact error message may vary, so we check for any error-like text
    expect(screen.getByText(/start|end|parse|invalid/i)).toBeInTheDocument();

    // onConfirm should NOT be called
    expect(mockOnConfirm).not.toHaveBeenCalled();
  });

  it('should close dropdown after applying valid time range with Enter', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Dropdown should be visible
    expect(screen.getAllByPlaceholderText('Time input')).toHaveLength(2);

    // Change time and press Enter
    const startInput = screen.getByLabelText('Start Time');
    await user.clear(startInput);
    await user.type(startInput, '2025-01-01 09:00{Enter}');

    // Dropdown should be closed
    expect(screen.queryByPlaceholderText('Time input')).not.toBeInTheDocument();
  });

  it('should close dropdown after applying valid time range with Apply button', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Dropdown should be visible
    expect(screen.getAllByPlaceholderText('Time input')).toHaveLength(2);

    // Click Apply with valid inputs
    const applyButton = screen.getByRole('button', { name: /apply/i });
    await user.click(applyButton);

    // Dropdown should be closed
    expect(screen.queryByPlaceholderText('Time input')).not.toBeInTheDocument();
  });

  it('should apply time range with Enter key for human-friendly expressions', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Enter human-friendly expressions
    const startInput = screen.getByLabelText('Start Time');
    const endInput = screen.getByLabelText('End Time');
    await user.clear(startInput);
    await user.type(startInput, '2h ago');
    await user.clear(endInput);
    await user.type(endInput, 'now');

    // Press Enter in end input
    await user.keyboard('{Enter}');

    // onConfirm should be called with the expressions
    expect(mockOnConfirm).toHaveBeenCalled();
    const [range, rawStart, rawEnd] = mockOnConfirm.mock.calls[0];
    expect(rawStart).toBe('2h ago');
    expect(rawEnd).toBe('now');
    expect(range.start).toBeInstanceOf(Date);
    expect(range.end).toBeInstanceOf(Date);
  });

  it('should support human-friendly time expressions with Apply button', async () => {
    const user = userEvent.setup();
    render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Enter human-friendly expressions
    const startInput = screen.getByLabelText('Start Time');
    const endInput = screen.getByLabelText('End Time');
    await user.clear(startInput);
    await user.type(startInput, '2h ago');
    await user.clear(endInput);
    await user.type(endInput, 'now');

    // Click Apply
    const applyButton = screen.getByRole('button', { name: /apply/i });
    await user.click(applyButton);

    // onConfirm should be called with the expressions
    expect(mockOnConfirm).toHaveBeenCalled();
    const [range, rawStart, rawEnd] = mockOnConfirm.mock.calls[0];
    expect(rawStart).toBe('2h ago');
    expect(rawEnd).toBe('now');
    expect(range.start).toBeInstanceOf(Date);
    expect(range.end).toBeInstanceOf(Date);
  });

  it('should display raw time expressions in button label', () => {
    const propsWithRawExpressions = {
      ...defaultProps,
      rawStart: 'now-1h',
      rawEnd: 'now',
    };

    render(<TimeRangeDropdown {...propsWithRawExpressions} />);

    const button = screen.getByRole('button');
    expect(button.textContent).toContain('now-1h to now');
  });

  it('should close dropdown when clicking outside', async () => {
    const user = userEvent.setup();
    const { container } = render(<TimeRangeDropdown {...defaultProps} />);

    const button = screen.getByRole('button');
    await user.click(button);

    // Dropdown should be visible
    expect(screen.getAllByPlaceholderText('Time input')).toHaveLength(2);

    // Click outside the dropdown
    await user.click(container);

    // Dropdown should be closed
    expect(screen.queryByPlaceholderText('Time input')).not.toBeInTheDocument();
  });
});
